package memorystore

import (
	"encoding/json"
	"fmt"
	"kv_store/internal/spec"
	"os"
)

// if the sst file exists, loadSST returns all of the key value pairs present in it.
func loadSST(filename string) ([]spec.SSTEntry, error) {
	sstFile, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("unable to read SST-File - %s, err: %v", filename, err.Error())
		return nil, err
	}

	sstEntries := []spec.SSTEntry{}

	err = json.Unmarshal(sstFile, &sstEntries)
	if err != nil {
		fmt.Printf("unable to unmarshal SST-File - %s, err: %v", filename, err.Error())
		return nil, err
	}

	return sstEntries, nil
}

// checkSST searches for the keys in the SST files created by the flush operations on the local storage.
func checkSST(key string) (string, bool, error) {
	// Search through the negative cache before
	cacheOut := fetchNegativeCache(key)
	searchEndIndex := max(-1, cacheOut.man)

	// begin looking for the key from the latest page.
	index := len(manifest) - 1
outer:
	for index > searchEndIndex {
		// TODO: instead of loading the entire file, we can stream records to check through them
		sstTable, err := loadSST(manifest[index])
		if err != nil {
			return "", false, err
		}
		for _, item := range sstTable {
			if item.Key == key {
				if item.Tombstone {
					break outer
				}
				return *item.Value, true, nil
			}
		}
		index--
	}

	putNegativeCache(key, len(manifest)-1)

	return "", false, nil
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

func resetNegativeCache() {
	negativeCache = make([]negativeCacheKey, negativeCacheLimit)
}
