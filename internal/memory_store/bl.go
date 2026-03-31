package memorystore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"slices"
	"sort"
	// "sync"

	"kv_store/internal/spec"
	"kv_store/internal/utility"
)

type negativeCacheKey struct {
	key string
	man int
}

type WAL struct {
	fileHandle *os.File
	walEncoder *json.Encoder
	// lock       sync.Mutex
	// syncCounter int
}

const (
	negativeCacheLimit = 100
)

var (
	memStore             = make(map[string]string)
	manifest             = make([]string, 0)
	negativeCache        = make([]negativeCacheKey, negativeCacheLimit)
	negativeCachePointer = 0
	wal                  WAL
)

// handlePut sets the value for the provided key directly in the in-memory map.
// If the map size grows beyond the max table size, the map is flushed out.
func handlePut(r *spec.PutRequest) bool {
	memStore[r.Key] = r.Value

	if len(memStore) >= utility.MemTableSize {
		flushMemTable()
		flushWAL()
	} else {
		walWrite(&spec.WALRequest{
			Key:       r.Key,
			Value:     r.Value,
			Operation: utility.PUT,
		})
	}

	return true
}

func walWrite(r *spec.WALRequest) {
	hash, err := utility.HashStruct(r)
	if err != nil {
		log.Println("failed to calculate hash for WAL entry, error: ", err.Error())
		return
	}

	r.Hash = hash

	// wal.lock.Lock()
	// defer wal.lock.Unlock()
	wal.walEncoder.Encode(r)
	// wal.syncCounter++
	// // if wal.syncCounter == 10 {
	// 	wal.syncCounter = 0
	err = wal.fileHandle.Sync()
	if err != nil {
		log.Println("failed to sync WAL to FS, error: ", err.Error())
	}
	// }
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

	defer f.Close()

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
	err = f.Sync()
	if err != nil {
		log.Printf("failed to sync SSTFile - %s, error: %s", sstName, err.Error())
	}

	memStore = make(map[string]string)
	// need to test if reallocating is faster or clearing each entry is faster.
	manifest = append(manifest, sstName)

	return true
}

func flushWAL() {
	// wal.lock.Lock()
	// defer wal.lock.Unlock()
	err := wal.fileHandle.Truncate(0)
	if err != nil {
		log.Println("failed to truncate WAL file, error: ", err.Error())
	}
	wal.fileHandle.Sync()
	_, err = wal.fileHandle.Seek(0, 0)
	if err != nil {
		log.Println("failed to sync after truncating WAL file, error: ", err.Error())
	}

	// wal.syncCounter = 0
}

// Section of functions that contain code to clean the memory store setup
func closeWAL() {
	// wal.lock.Lock()
	// defer wal.lock.Unlock()

	wal.fileHandle.Close()
}

func flushManifest() error {
	var f *os.File
	var err error
	// handle file create
	f, err = os.Create(utility.ManifestName)
	if err != nil {
		return err
	}
	defer f.Close()
	marshalledOut, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		return err
	}
	_, err = f.Write(marshalledOut)
	if err != nil {
		return err
	}

	f.Sync()

	return nil
}

// Section of functions that contain code to setup the memory store
func loadManifest() error {
	bytes, err := os.ReadFile(utility.ManifestName)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		entries, err := os.ReadDir(".")
		if err != nil {
			return err
		}

		re := regexp.MustCompile(`^sst-\d+\.json$`)
		for _, entry := range entries {
			isSSTFile := re.MatchString(entry.Name())
			if isSSTFile {
				manifest = append(manifest, entry.Name())
			}
		}
		slices.SortFunc(manifest, func(a string, b string) int {
			numA := utility.ExtractSSTFileNumber(a)
			numB := utility.ExtractSSTFileNumber(b)

			return numA - numB
		})

	} else {
		err = json.Unmarshal(bytes, &manifest)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadWAL() error {
	replay := true

	f, err := os.ReadFile(utility.WALName)
	if err != nil {
		if os.IsNotExist(err) {
			replay = false
		} else {
			log.Println("unable to read from wal.db, error: ", err.Error())
			return err
		}
	}

	if replay {
		decoder := json.NewDecoder(bytes.NewReader(f))
		for {
			var kv spec.WALRequest

			err := decoder.Decode(&kv)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Println("decode failed: ", err.Error())
				break
			}

			diskHash := kv.Hash

			kv.Hash = 0

			hash, err := utility.HashStruct(kv)
			if err != nil {
				log.Println("unable to calculate hash for wal request present on disk, assuming corruption from this point, error: ", err.Error())
				break
			}

			if hash != diskHash {
				log.Println("hash for wal request present on disk does not match with data, skipping loading of further entries from disk")
				break
			}

			if kv.Operation == utility.PUT {
				// we cannot use handlePut as handlePut also writes to WAL and we enter a loop.
				// handlePut(&spec.PutRequest{Key: kv.Key, Value: kv.Value})
				memStore[kv.Key] = kv.Value
			}
		}
	}

	// open the file handle to WAL
	wal.fileHandle, err = os.OpenFile(utility.WALName, os.O_TRUNC|os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Println("failed to create a new wal file in append mode, error: ", err.Error())
		return err
	}
	wal.walEncoder = json.NewEncoder(wal.fileHandle)
	// wal.lock = sync.Mutex{}
	// wal.syncCounter = 0
	return nil
}
