package memorystore

import (
	"fmt"
	"log"

	"kv_store/internal/spec"
	"kv_store/internal/utility"
)

type negativeCacheKey struct {
	key    string
	sstNum int
}

type Value struct {
	Value     string
	Tombstone bool
}

const (
	negativeCacheLimit = 100
)

var (
	memStore             = make(map[string]Value)
	manifest             = make(map[spec.SSTLevel][]*spec.SSTMetaData)
	negativeCache        = make([]negativeCacheKey, negativeCacheLimit)
	negativeCachePointer = 0
	wal                  WAL
)

// handlePut sets the value for the provided key directly in the in-memory map.
// If the map size grows beyond the max table size, the map is flushed out.
func handlePut(r *spec.PutRequest) bool {
	memStore[r.Key] = Value{
		Value:     r.Value,
		Tombstone: false,
	}

	if len(memStore) >= utility.MemTableSize {
		flushMemTable()
		flushWAL()
	} else {
		// WAL writes cannot be called concurrently because when the goroutine waits on the lock,
		// the scheduler does not guarantee FIFO, meaning a newer PUT request can get the lock
		// before an older PUT request.
		// TODO: Find a way to use a FIFO queue for these writes.
		walWrite(&spec.WALRequest{
			Key:       r.Key,
			Value:     r.Value,
			Operation: utility.PUT,
		})
	}

	return true
}

// handleGet searchs the provided key in the in-memory map, and if it fails to find it
// there, it will search for it within the SSTs.
func handleGet(key string) *string {
	value, exists := memStore[key]

	if !exists {
		// check the sst tables, and ensure it doesn't error out.
		value, err := checkSST(key)
		if err != nil {
			fmt.Println("failed to checkSST: ", err.Error())
		}
		return value
	}
	if value.Tombstone {
		return nil
	}

	return &value.Value

}

func handleDelete(key string) (bool, error) {
	// first check if the entry is present in memStore
	if !deleteMemtableEntry(key) {

		value, err := checkSST(key)
		if err != nil {
			log.Println("failed to check SST while serving delete request, error: ", err.Error())
			return false, err
		}

		if value == nil {
			return false, nil
		}

		memStore[key] = Value{
			Tombstone: true,
		}

	}

	walWrite(&spec.WALRequest{
		Key:       key,
		Operation: utility.DELETE,
	})

	return true, nil
}
