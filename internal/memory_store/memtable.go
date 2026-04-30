package memorystore

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

	"kv_store/internal/config"
	"kv_store/internal/spec"
)

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

	l0 := spec.SSTLevel(0)

	sst, err := writeSST(flushOut, l0)
	if err != nil {
		return false
	}

	memStore = make(map[string]Value)
	// need to test if reallocating is faster or clearing each entry is faster.
	manifest[l0] = append(manifest[l0], sst)

	if len(manifest[l0]) > config.Conf.LevelSize[int(l0)] {
		log.Println(" triggered l0 compaction")
		err = triggerL0Compaction()
		if err != nil {
			fmt.Println("l0 compaction failed, error: ", err.Error())
			return false
		}
	}

	return true
}

func writeSST(data []*spec.SSTEntry, level spec.SSTLevel) (*spec.SSTMetaData, error) {
	sstName := getNextSSTName(level)

	sstPath := level.GetSSTPath(sstName)
	sstInfo := spec.SSTMetaData{
		Name: sstName,
	}
	if len(data) > 0 {
		sstInfo.FirstKey = data[0].Key
		sstInfo.LastKey = data[len(data)-1].Key
	}

	f, err := os.Create(sstPath + ".tmp")
	if err != nil {
		fmt.Printf("failed to create temp sst file: %v\n", err.Error())
		return nil, err
	}
	defer f.Close()

	marshalledOut, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		fmt.Println("failed to marshal output to temp sst during flush: ", err.Error())
		return nil, err
	}
	_, err = f.Write(marshalledOut)
	if err != nil {
		fmt.Printf("failed to write to sst file during flush: %s, err: %v\n", sstName, err.Error())
		return nil, err
	}

	err = os.Rename(sstPath+".tmp", sstPath)
	if err != nil {
		log.Printf("failed to rename temp sst file with actual path: %s", err.Error())
	}

	err = f.Sync()
	if err != nil {
		log.Printf("failed to sync SSTFile - %s, error: %s", sstName, err.Error())
		return nil, err
	}

	// ensure the file rename persists.
	dir, _ := os.Open(level.FolderString())
	dir.Sync()
	dir.Close()

	return &sstInfo, nil
}

func getNextSSTName(level spec.SSTLevel) string {
	if level > spec.SSTLevel(config.Conf.MaxLevels) {
		return ""
	}

	sstName := fmt.Sprintf(sstTemplate, config.Conf.NextFileID[level])
	config.Conf.NextFileID[level]++
	// TODO: Avoid flushing config so frequently
	config.FlushConfig()

	return sstName
}
