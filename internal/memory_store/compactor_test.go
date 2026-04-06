package memorystore

import (
	"encoding/json"
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
	for key, value := range memTableGenerator(4) {
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

func Test_triggerCompaction(t *testing.T) {
	cleanup := func() {
		cleanManifest()
		cleanSSTFiles()
	}

	tests := []struct {
		name    string // description of this test case
		wantErr bool
		setup   func()
		verify  func(t *testing.T)
	}{
		{
			name: "T1-Successful",
			setup: func() {
				loadManifest()

				temp := map[string]Value{
					"key1": {
						Value: "val2",
					},
					"ddd": {
						Value: "zizk",
					},
					"kiki": {
						Tombstone: true,
					},
					"zara": {
						Value: "zzz",
					},
					"ebd": {
						Value: "nono",
					},
				}
				memStore = temp
				flushMemTable()

				temp = map[string]Value{
					"key1": {
						Value: "val3",
					},
					"ebd": {
						Value: "ekd",
					},
					"abc": {
						Tombstone: true,
					},
					"dd": {
						Value: "dd",
					},
				}
				memStore = temp
				flushMemTable()

				temp = map[string]Value{
					"key1": {
						Value: "val1",
					},
					"abc": {
						Value: "lol",
					},
					"dd": {
						Value: "ken",
					},
					"ddd": {
						Value: "kenithra",
					},
					"zara": {
						Tombstone: true,
					},
				}
				memStore = temp
				flushMemTable()

			},
			verify: func(t *testing.T) {
				if len(manifest) != 1 {
					t.Fatalf("length of manifest not equal to 1, manifest: %+v\n", manifest)
				}

				if manifest[0] != "sst-3.json" {
					t.Fatalf("sst name not equal to sst-3.json, actual name: %s", manifest[0])
				}

				type entry struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}

				expected := []entry{
					{Key: "abc", Value: "lol"},
					{Key: "dd", Value: "ken"},
					{Key: "ddd", Value: "kenithra"},
					{Key: "ebd", Value: "ekd"},
					{Key: "key1", Value: "val1"},
				}

				got := []entry{}

				gotData, err := os.ReadFile("sst-3.json")
				if err != nil {
					t.Fatal(err)
				}

				json.Unmarshal(gotData, &got)

				if len(expected) != len(got) {
					t.Fatalf("data length does not match, got length: %d, expected length: %d", len(got), len(expected))
				}

				for key, value := range expected {
					if got[key] != value {
						t.Errorf("unexpected value for index: %d, got: %s, expected: %s\n", key, got[key], value)
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
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("triggerCompaction() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("triggerCompaction() succeeded unexpectedly")
			}

			if tt.verify != nil {
				tt.verify(t)
			}

		})
	}
}
