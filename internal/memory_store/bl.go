package memorystore

import (
	"encoding/json"
	"fmt"
	"kv_store/internal/spec"
	"kv_store/internal/utility"
	"os"
	"sort"
)

type negativeCacheKey struct {
	key string
	man int
}

const (
	negativeCacheLimit = 100
)

var (
	memStore             = make(map[string]string)
	manifest             = make([]string, 0)
	negativeCache        = make([]negativeCacheKey, negativeCacheLimit)
	negativeCachePointer = 0
)

// handlePut sets the value for the provided key directly in the in-memory map.
// If the map size grows beyond the max table size, the map is flushed out.
func handlePut(r *spec.PutRequest) bool {
	memStore[r.Key] = r.Value

	if len(memStore) >= utility.MemTableSize {
		flushMemTable()
	}

	return true
}

// handleGet searchs the provided key in the in-memory map, and if it fails to find it
// there, it will search for it within the SSTs.
func handleGet(key string) (string, bool) {
	value, exists := memStore[key]

	if !exists {
		// check the sst tables, and ensure it doesn't error out.
		value, exists, err := checkSST(key)
		if err != nil {
			fmt.Println("failed to checkSST: ", err.Error())
		}
		return value, exists
	}
	return value, true
}

// since negative cache is supposed to be small(100) and we can iterate through it rather quickly,
// we use an array instead of a map in this case.
func fetchNegativeCache(key string) negativeCacheKey {
	for _, k := range negativeCache {
		if k.key == key {
			return k
		}
	}
	return negativeCacheKey{key: key, man: -1}
}

// putNegativeCache searches through the existing cache to see if the key is present and updates it if it is.
// If not, it places(or replaces) it at the current index of this negative cache.
func putNegativeCache(key string, man int) {
	for idx, k := range negativeCache {
		if k.key == key {
			negativeCache[idx] = negativeCacheKey{key, man}
			return
		}
	}

	negativeCache[negativeCachePointer%negativeCacheLimit] = negativeCacheKey{key, man}
	negativeCachePointer++
}

// checkSST searches for the keys in the SST files created by the flush operations on the local storage.
func checkSST(key string) (string, bool, error) {

	// Search through the negative cache before
	cacheOut := fetchNegativeCache(key)
	searchEndIndex := max(-1, cacheOut.man)

	// begin looking for the key from the latest page.
	index := len(manifest) - 1
	for index > searchEndIndex {
		sstTable, err := loadSST(manifest[index])
		if err != nil {
			return "", false, err
		}
		for _, item := range sstTable {
			if item.Key == key {
				return item.Value, true, nil
			}
		}
		index--
	}

	putNegativeCache(key, len(manifest)-1)

	return "", false, nil
}

// if the sst file exists, loadSST returns all of the key value pairs present in it.
func loadSST(filename string) ([]spec.PutRequest, error) {
	sstFile, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("unable to read SST-File - %s, err: %v", filename, err.Error())
		return nil, err
	}

	sstMap := []spec.PutRequest{}

	err = json.Unmarshal(sstFile, &sstMap)
	if err != nil {
		fmt.Printf("unable to unmarshal SST-File - %s, err: %v", filename, err.Error())
		return nil, err
	}

	return sstMap, nil
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

	// TODO: We can probably reserve this page.
	flushOut := make([]spec.PutRequest, 0, len(memStore))
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
	// need to test if reallocating is faster or clearing each entry is faster.
	manifest = append(manifest, sstName)

	return true
}
