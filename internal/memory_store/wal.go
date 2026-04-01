package memorystore

import (
	"encoding/json"
	"errors"
	"io"
	"kv_store/internal/spec"
	"kv_store/internal/utility"
	"log"
	"os"
)

type WAL struct {
	fileHandle *os.File
	walEncoder *json.Encoder
	// lock       sync.Mutex
	// syncCounter int
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
	err = wal.walEncoder.Encode(r)
	if err != nil {
		log.Println("failed to write WAL req to FS, error: ", err.Error())
		return
	}
	// wal.syncCounter++
	// // if wal.syncCounter == 10 {
	// 	wal.syncCounter = 0
	err = wal.fileHandle.Sync()
	if err != nil {
		log.Println("failed to sync WAL to FS, error: ", err.Error())
		return
	}
	// }
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

func loadWAL() error {
	replay := true

	f, err := os.Open(utility.WALName)
	if err != nil {
		if os.IsNotExist(err) {
			replay = false
		} else {
			log.Println("unable to read from wal.db, error: ", err.Error())
			return err
		}
	}
	defer f.Close()

	if replay {
		decoder := json.NewDecoder(f)
		for {
			var kv spec.WALRequest
			offset := decoder.InputOffset()
			err := decoder.Decode(&kv)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				log.Println("decode failed: ", err.Error())
				f.Truncate(offset)
				break
			}

			diskHash := kv.Hash

			kv.Hash = 0

			hash, err := utility.HashStruct(kv)
			if err != nil {
				log.Println("unable to calculate hash for wal request present on disk, assuming corruption from this point, error: ", err.Error())
				f.Truncate(offset)
				break
			}

			if hash != diskHash {
				log.Println("hash for wal request present on disk does not match with data, skipping loading of further entries from disk")
				f.Truncate(offset)
				break
			}

			if kv.Operation == utility.PUT {
				// we cannot use handlePut as handlePut also writes to WAL and we enter a loop.
				// handlePut(&spec.PutRequest{Key: kv.Key, Value: kv.Value})
				memStore[kv.Key] = Value{Value: kv.Value, Tombstone: false}
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

func closeWAL() {
	// wal.lock.Lock()
	// defer wal.lock.Unlock()

	wal.fileHandle.Close()
}
