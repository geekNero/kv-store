package memorystore

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"kv_store/internal/spec"
	"kv_store/internal/utility"
)

var sstTemplate = "sst-%d.json"

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
func checkSST(key string) (*string, error) {
	// Search through the negative cache before
	cacheOut := fetchNegativeCache(key)
	searchEndIndex := cacheOut.sstNum

	// begin looking for the key in l0.
	found := false
	l0 := manifest[SSTLevel(0)]
	index := len(l0) - 1
outer:
	for index > searchEndIndex {
		if l0[index] == nil {
			log.Panicf("manifest index: %d, is a nil pointer", index)
		}

		// TODO: instead of loading the entire file, we can stream records to check through them
		sstTable, err := loadSST(l0[index].Name)
		if err != nil {
			return nil, err
		}
		for _, item := range sstTable {
			if item.Key == key {
				if item.Tombstone {
					found = true
					break outer
				}
				return item.Value, nil
			}
		}
		index--
	}

	if !found {
		// check higher levels
		level := SSTLevel(1)

		for level <= MaxLevel {
			value := searchOrderedSSTs(key, level)
			if value != nil {
				if value.Tombstone {
					break
				}
				return value.Value, nil
			}

			level++
		}
	}

	putNegativeCache(key, len(manifest)-1)

	return nil, nil
}

func searchOrderedSSTs(key string, level SSTLevel) *spec.SSTEntry {
	// need to lock on the SST with the range via binary search

	levelFolder := fmt.Sprintf("l%d", int(level))

	targetSST := utility.FindKeyContainingSST(key, manifest[level])
	if targetSST == nil {
		return nil
	}

	entries, err := loadSST(filepath.Join(levelFolder, targetSST.Name))
	if err != nil {
		log.Printf("failed to load sst of level: %d, sst name: %s, error: %s", int(level), targetSST.Name, err.Error())
		return nil
	}

	for _, entry := range entries {
		if entry.Key == key {
			return &entry
		}
	}

	return nil
}

// since negative cache is supposed to be small(100) and we can iterate through it rather quickly,
// we use an array instead of a map in this case.
func fetchNegativeCache(key string) negativeCacheKey {
	for _, k := range negativeCache {
		if k.key == key {
			return k
		}
	}
	return negativeCacheKey{key: key, sstNum: -1}
}

// putNegativeCache searches through the existing cache to see if the key is present and updates it if it is.
// If not, it places(or replaces) it at the current index of this negative cache.
func putNegativeCache(key string, sstNum int) {
	for idx, k := range negativeCache {
		if k.key == key {
			negativeCache[idx] = negativeCacheKey{key, sstNum}
			return
		}
	}

	negativeCache[negativeCachePointer%negativeCacheLimit] = negativeCacheKey{key, sstNum}
	negativeCachePointer++
}

func resetNegativeCache() {
	negativeCache = make([]negativeCacheKey, negativeCacheLimit)
}

func cleanupOrphanedSSTs() {
	for key := range manifest {
		levelName := fmt.Sprintf("l%d", key)
		entries, err := os.ReadDir(levelName)
		if err != nil {
			log.Printf("failed to read %s directory, error: %s", levelName, err.Error())
		}

		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".tmp" {
				os.Remove(entry.Name())
			}
		}
	}
}
