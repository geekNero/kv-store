package memorystore

import (
	"kv_store/internal/utility"
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
