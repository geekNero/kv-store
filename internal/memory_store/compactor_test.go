package memorystore

import (
	"fmt"
	"os"
	"sort"
	"testing"

	"kv_store/internal/config"
	"kv_store/internal/spec"

	"github.com/google/go-cmp/cmp"
)

func TestNewFileIterator(t *testing.T) {
	cleanManifest()
	cleanSSTFiles()
	defer cleanManifest()
	defer cleanSSTFiles()

	memStore = memTableGenerator(4)
	memStore["key5"] = Value{
		Tombstone: true,
	}
	flushMemTable()

	// flushMemTable adds to l0
	fileIterator := NewFileIterator(0, 0)
	if fileIterator == nil {
		t.Fatalf("file iterator is nil\n")
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

	// compare non-tombstone entries (keys are sorted)
	keys := []string{"key1", "key2", "key3", "key4"}
	for _, key := range keys {
		entry := fileIterator.nextItem()
		if entry == nil {
			t.Fatalf("o/p of nextItem is empty, key: %s", key)
		}

		if entry.Key != key {
			t.Errorf("o/p of nextItem does not match, got: %s, wantKey: %s\n", entry.Key, key)
		}
	}

	// compare tombstone entry
	entry = fileIterator.nextItem()
	if entry == nil {
		t.Fatalf("tombstone entry in the sst was not read and is nil")
	}

	if entry.Key != "key5" || entry.Tombstone == false {
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
		level := spec.SSTLevel(0)
		filename := level.GetSSTPath("sst-0.json")
		os.WriteFile(filename, []byte(""), 0o644)
		manifest[level] = append(manifest[level], &spec.SSTMetaData{Name: "sst-0.json"})

		iter := NewFileIterator(0, level)
		if iter == nil {
			return // expected
		}
		err := iter.open()
		if err == nil {
			t.Error("expected error when opening empty file")
		}
	})

	t.Run("NotAJSONArray", func(t *testing.T) {
		cleanup()
		level := spec.SSTLevel(0)
		filename := level.GetSSTPath("sst-0.json")
		os.WriteFile(filename, []byte("{ \"key\": \"val\" }"), 0o644)
		manifest[level] = append(manifest[level], &spec.SSTMetaData{Name: "sst-0.json"})

		iter := NewFileIterator(0, level)
		err := iter.open()
		if err == nil {
			t.Error("expected error when file is not a JSON array")
		}
	})
}

// Test_triggerL0Compaction isn't a table driven test as of now because we do not have stable compaction code. For now,
// we need to ensure there's one positive working path until we finish development.
// TODO: Cover more positive and negative scenarios once done.
func Test_triggerL0Compaction(t *testing.T) {
	cleanup := func() {
		cleanManifest()
		cleanSSTFiles()
	}

	t.Run("BasicL0ToL1", func(t *testing.T) {
		cleanup()
		defer cleanup()

		config.Conf.NextFileID[0] = 0
		config.Conf.NextFileID[1] = 0

		// l0 files
		data, _ := sstGenerator(1, 100, 0)
		manifest[0] = append(manifest[0], data)

		data, _ = sstGenerator(50, 150, 0)
		manifest[0] = append(manifest[0], data)

		data, _ = sstGenerator(100, 200, 0)
		manifest[0] = append(manifest[0], data)

		data, _ = sstGenerator(150, 170, 0)
		manifest[0] = append(manifest[0], data)

		// l1 files
		// completely intersects with l1
		data, _ = sstGenerator(110, 190, 1)
		manifest[1] = append(manifest[1], data)
		// partially intersects with l0
		data, _ = sstGenerator(199, 299, 1)
		manifest[1] = append(manifest[1], data)
		// no intersection with l0
		data, _ = sstGenerator(790, 899, 1)
		manifest[1] = append(manifest[1], data)

		err := triggerL0Compaction()
		if err != nil {
			t.Fatalf("triggerL0Compaction failed: %v", err)
		}

		if len(manifest[0]) != 0 {
			t.Errorf("expected 0 L0 files after compaction, got %d", len(manifest[0]))
		}

		// compare manifest data
		config.Conf.NextFileID[1] = 2
		wantManifest1 := make([]*spec.SSTMetaData, 0, 2)
		wantManifest1 = append(wantManifest1, &spec.SSTMetaData{
			Name:     getNextSSTName(1),
			FirstKey: fmt.Sprintf("key%05d", 790),
			LastKey:  fmt.Sprintf("key%05d", 899),
		},
			&spec.SSTMetaData{
				Name:     getNextSSTName(1),
				FirstKey: fmt.Sprintf("key%05d", 1),
				LastKey:  fmt.Sprintf("key%05d", 299),
			})

		sort.Slice(wantManifest1, func(i, j int) bool {
			return wantManifest1[i].FirstKey < wantManifest1[j].FirstKey
		})

		if diff := cmp.Diff(wantManifest1, manifest[1]); diff != "" {
			t.Errorf("L1 metadata is incorrect, diff: %s", diff)
		}

		// set the expected SST
		wantSST := sstDataGenerator(1, 200, 0)
		wantSST = append(wantSST, sstDataGenerator(201, 299, 1)...)
		compareWithSSTIndex := func(index int) {
			gotSST, err := loadSST(spec.SSTLevel(1).GetSSTPath(wantManifest1[index].Name))
			if err != nil {
				t.Fatalf("failed to loadSST for the test, sstname: %s", wantManifest1[index].Name)
			}

			if len(wantSST) != len(gotSST) {
				t.Fatalf("SST lenghts don't match, want: %d, got: %d", len(wantSST), len(gotSST))
			}

			for i, item := range wantSST {
				if diff := cmp.Diff(*item, gotSST[i]); diff != "" {
					t.Fatalf("want SST item: %+v, does not match got SST item: %+v", *item, gotSST[i])
				}
			}
		}

		compareWithSSTIndex(0)

		// reset the sst to compare it with next one
		wantSST = sstDataGenerator(790, 899, 1)
		compareWithSSTIndex(1)
	})
}
