package memorystore

import (
	"fmt"
	"kv_store/internal/utility"
	"os"
	"testing"
)

func TestNewFileIterator(t *testing.T) {

	cleanManifest()
	cleanSSTFiles()
	defer cleanManifest()
	defer cleanSSTFiles()

	loadManifest()
	memStore = memTableGenerator(4)
	memStore["key5"] = Value{
		Tombstone: true,
	}
	flushMemTable()

	sstFileName := "sst-0.json"

	fileIterator := NewFileIterator(sstFileName)
	if fileIterator == nil {
		t.Errorf("file iterator is nil\n")
	}

	// test fileIterator without opening it
	entry := fileIterator.nextItem()
	if entry != nil {
		t.Fatalf("fileIterator returned a entry without being opened, entry: %+v", *entry)
	}

	err := fileIterator.open()
	if err != nil {
		t.Fatal(err)
	}

	// test closing fileIterator without draining it
	err = fileIterator.close()
	if err == nil {
		t.Fatal("fileIterator closed without being drained")
	}

	// compare non-tombstone entries
	keys := []string{"key1", "key2", "key3", "key4"}
	for _, key := range keys {
		value := Value{Value: "val" + key[3:]}
		entry := fileIterator.nextItem()
		if entry == nil {
			t.Fatalf("o/p of nextItem is empty, key: %s", key)
		}

		if entry.Key != key || entry.Tombstone != value.Tombstone || !utility.CheckPtrStringsEqual(entry.Value, &value.Value) {
			t.Errorf("o/p of nextItem does not match, got: %+v, want: %+v, wantKey: %s\n", entry, value, key)
			if entry.Value != nil {
				t.Logf("entry.Value: %s", *entry.Value)
			}
		}
	}

	// compare tombstone entry
	entry = fileIterator.nextItem()
	if entry == nil {
		t.Fatalf("tombstone entry in the sst was not read and is nil")
	}

	if entry.Key != "key5" || entry.Value != nil || entry.Tombstone == false {
		t.Errorf("tombstone entry from the sst does not match, entry: %+v", entry)
	}

	entry = fileIterator.nextItem()
	if entry != nil {
		t.Fatalf("unexpected entry, entry: %+v", entry)
	}

	err = fileIterator.close()
	if err != nil {
		t.Fatal(err)
	}

}

func TestFileIterator_EdgeCases(t *testing.T) {
	cleanup := func() {
		cleanManifest()
		cleanSSTFiles()
	}
	defer cleanup()

	t.Run("EmptyFile", func(t *testing.T) {
		cleanup()
		filename := "empty.json"
		os.WriteFile(filename, []byte(""), 0644)
		defer os.Remove(filename)

		iter := NewFileIterator(filename)
		err := iter.open()
		if err == nil {
			t.Error("expected error when opening empty file")
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		cleanup()
		filename := "malformed.json"
		os.WriteFile(filename, []byte("[{ \"key\": \"missing_quote }"), 0644)
		defer os.Remove(filename)

		iter := NewFileIterator(filename)
		err := iter.open()
		if err != nil {
			t.Fatalf("did not expect error during open, got: %v", err)
		}

		entry := iter.nextItem()
		if entry != nil {
			t.Error("expected nil entry for malformed JSON item")
		}
	})

	t.Run("NotAJSONArray", func(t *testing.T) {
		cleanup()
		filename := "notarray.json"
		os.WriteFile(filename, []byte("{ \"key\": \"val\" }"), 0644)
		defer os.Remove(filename)

		iter := NewFileIterator(filename)
		err := iter.open()
		if err == nil {
			t.Error("expected error when file is not a JSON array")
		}
	})
}

func flushData(data map[string]Value) {
	memStore = data
	flushMemTable()
}

func Test_triggerCompaction(t *testing.T) {
	cleanup := func() {
		cleanManifest()
		cleanSSTFiles()
		memStore = make(map[string]Value)
	}

	tests := []struct {
		name    string
		wantErr bool
		setup   func()
		verify  func(t *testing.T)
	}{
		{
			name: "T1-BasicOverlapAndTombstones",
			setup: func() {
				// SST-0
				flushData(map[string]Value{
					"key1": {Value: "val0"},
					"key2": {Value: "val0"},
					"key3": {Tombstone: true},
				})
				// SST-1
				flushData(map[string]Value{
					"key1": {Value: "val1"},
					"key3": {Value: "val1"},
					"key4": {Value: "val1"},
				})
				// SST-2
				flushData(map[string]Value{
					"key2": {Tombstone: true},
					"key4": {Value: "val2"},
					"key5": {Value: "val2"},
				})
			},
			verify: func(t *testing.T) {
				if len(manifest) != 1 {
					t.Fatalf("expected 1 SST file in manifest, got %d", len(manifest))
				}

				entries, err := loadSST(manifest[0])
				if err != nil {
					t.Fatal(err)
				}

				expected := map[string]string{
					"key1": "val1",
					"key3": "val1",
					"key4": "val2",
					"key5": "val2",
				}

				if len(entries) != len(expected) {
					t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
				}

				for _, entry := range entries {
					val, ok := expected[entry.Key]
					if !ok {
						t.Errorf("unexpected key %s in compacted SST", entry.Key)
						continue
					}
					if entry.Tombstone {
						t.Errorf("key %s should not be a tombstone", entry.Key)
					}
					if *entry.Value != val {
						t.Errorf("key %s: expected value %s, got %s", entry.Key, val, *entry.Value)
					}
				}
			},
		},
		{
			name: "T2-MultipleOutputFiles",
			setup: func() {
				// We'll create two SSTs that when compacted will result in more than MemTableSize entries
				// Note: utility.MemTableSize is 2000
				data1 := make(map[string]Value)
				for i := range 1500 {
					data1[fmt.Sprintf("a%04d", i)] = Value{Value: "val"}
				}
				flushData(data1)

				data2 := make(map[string]Value)
				for i := range 1500 {
					data2[fmt.Sprintf("b%04d", i)] = Value{Value: "val"}
				}
				flushData(data2)
			},
			verify: func(t *testing.T) {
				// 1500 + 1500 = 3000 entries. MemTableSize = 2000.
				// Should result in 2 SSTs: one with 2000, one with 1000.
				if len(manifest) != 2 {
					t.Fatalf("expected 2 SST files in manifest, got %d", len(manifest))
				}

				entries1, _ := loadSST(manifest[0])
				entries2, _ := loadSST(manifest[1])

				if len(entries1) != utility.MemTableSize {
					t.Errorf("expected %d entries in first SST, got %d", utility.MemTableSize, len(entries1))
				}
				if len(entries2) != 1000 {
					t.Errorf("expected 1000 entries in second SST, got %d", len(entries2))
				}
			},
		},
		{
			name: "T3-EmptyResultAfterCompaction",
			setup: func() {
				flushData(map[string]Value{
					"key1": {Value: "val1"},
				})
				flushData(map[string]Value{
					"key1": {Tombstone: true},
				})
			},
			verify: func(t *testing.T) {
				if len(manifest) != 0 {
					t.Fatalf("expected 0 SST files in manifest, got %d", len(manifest))
				}
			},
		},
		{
			name: "T4-MissingFileInManifest",
			setup: func() {
				flushData(map[string]Value{"key1": {Value: "val1"}})
				flushData(map[string]Value{"key2": {Value: "val2"}})
				// Manually remove one file from disk but keep in manifest
				os.Remove(manifest[0])
			},
			verify: func(t *testing.T) {
				// It should skip the missing file and continue
				if len(manifest) != 1 {
					t.Fatalf("expected 1 SST file in manifest, got %d", len(manifest))
				}
				entries, _ := loadSST(manifest[0])
				if len(entries) != 1 || entries[0].Key != "key2" {
					t.Errorf("unexpected entries: %+v", entries)
				}
			},
		},
		{
			name: "T5-LargeCompaction",
			setup: func() {
				// Many small SSTs
				for i := range 10 {
					data := make(map[string]Value)
					data[fmt.Sprintf("key%d", i)] = Value{Value: "val"}
					flushData(data)
				}
			},
			verify: func(t *testing.T) {
				if len(manifest) != 1 {
					t.Fatalf("expected 1 SST file, got %d", len(manifest))
				}
				entries, _ := loadSST(manifest[0])
				if len(entries) != 10 {
					t.Errorf("expected 10 entries, got %d", len(entries))
				}
			},
		},
		{
			name: "T6-NegativeCacheReset",
			setup: func() {
				flushData(map[string]Value{"key1": {Value: "val1"}})
				// Populate negative cache
				handleGet("missing_key")
				found := false
				for _, nc := range negativeCache {
					if nc.key == "missing_key" {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("failed to populate negative cache")
				}
			},
			verify: func(t *testing.T) {
				// After compaction, negative cache should be reset
				for _, nc := range negativeCache {
					if nc.key != "" {
						t.Errorf("negative cache not reset: %+v", nc)
					}
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup()
			defer cleanup()

			if tt.setup != nil {
				tt.setup()
			}

			gotErr := triggerCompaction()
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("triggerCompaction() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			if tt.verify != nil {
				tt.verify(t)
			}
		})
	}
}
