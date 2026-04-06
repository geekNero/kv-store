package memorystore

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

	"kv_store/internal/spec"
)

var nextSSTID int

func deleteMemtableEntry(key string) bool {
	entry, exists := memStore[key]
	if !exists {
		return false
	}

	entry.Tombstone = true
	memStore[key] = entry

	return true
}

func flushMemTable() bool {

	// Convert the mem-table into a list of PutRequests and DeleteRequests, to be marshalled out.
	keys := make([]string, 0, len(memStore))
	for key := range memStore {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// TODO: We can probably reserve this page.
	flushOut := make([]*spec.SSTEntry, 0, len(memStore))
	for _, key := range keys {
		entry := memStore[key]
		sstEntry := spec.SSTEntry{
			Key:       key,
			Tombstone: entry.Tombstone,
		}

		if !entry.Tombstone {
			sstEntry.Value = &entry.Value
		}

		flushOut = append(flushOut, &sstEntry)
	}

	sstName, err := writeSST(flushOut)
	if err != nil {
		return false
	}

	memStore = make(map[string]Value)
	// need to test if reallocating is faster or clearing each entry is faster.
	manifest = append(manifest, sstName)

	return true
}

func writeSST(data []*spec.SSTEntry) (string, error) {
	sstid := nextSSTID
	nextSSTID++

	sstName := fmt.Sprintf("sst-%d.json", sstid)

	f, err := os.Create(sstName)
	if err != nil {
		fmt.Printf("failed to flush mem-table: %v\n", err.Error())
		return "", err
	}
	defer f.Close()

	marshalledOut, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		fmt.Println("failed to marshal output in flush: ", err.Error())
		return "", err
	}
	_, err = f.Write(marshalledOut)
	if err != nil {
		fmt.Printf("failed to write to sst file: %s, err: %v\n", sstName, err.Error())
		return "", err
	}
	err = f.Sync()
	if err != nil {
		log.Printf("failed to sync SSTFile - %s, error: %s", sstName, err.Error())
		return "", err
	}

	return sstName, nil
}
