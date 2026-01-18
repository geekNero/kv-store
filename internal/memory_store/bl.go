package memorystore

import (
	"encoding/json"
	"fmt"
	"kv_store/internal/spec"
	"kv_store/internal/utility"
	"os"
	"sort"
)

var memStore = make(map[string]string)

var manifest = make([]string, 0)

func handlePut(r *spec.PutRequest) bool {
	memStore[r.Key] = r.Value

	if len(memStore) >= utility.MemTableSize {
		flushMemTable()
	}

	return true
}

func handleGet(key string) (string, bool) {
	value, exists := memStore[key]

	if !exists {
		return "", false
	}
	return value, true
}

func flushMemTable() bool {
	sstid := len(manifest)
	sstName := fmt.Sprintf("sst-%d.json", sstid)

	f, err := os.Create(sstName)
	if err != nil {
		fmt.Printf("failed to flush mem-table: %v\n", err.Error())
		return false
	}

	// Convert the mem-table into a list of PutRequests, to be marshalled out.
	keys := make([]string, 0, len(memStore))
	for key := range memStore {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	flushOut := make([]spec.PutRequest, len(memStore))
	for _, key := range keys {
		flushOut = append(flushOut, spec.PutRequest{Key: key, Value: memStore[key]})
	}

	marshalledOut, err := json.MarshalIndent(flushOut, "", " ")
	if err != nil {
		fmt.Println("failed to marshal output in flush: ", err.Error())
		return false
	}
	_, err = f.Write(marshalledOut)
	if err != nil {
		fmt.Printf("failed to write to sst file: %s, err: %v\n", sstName, err.Error())
		return false
	}

	memStore = make(map[string]string)
	manifest = append(manifest, sstName)

	return true
}
