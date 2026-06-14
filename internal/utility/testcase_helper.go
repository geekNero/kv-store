package utility

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"kv_store/internal/common"
	"kv_store/internal/config"
	"kv_store/internal/spec"
)

func CleanStart() {
	os.Remove("config.json")
	config.LoadConfig()
	// Ensure directories exist
	for l := spec.SSTLevel(0); l <= spec.DefaultMaxLevel; l++ {
		os.MkdirAll(l.FolderString(), 0o755)
	}
}

func CleanSSTFiles() {
	// Cleanup any sst-files in all levels
	for l := spec.SSTLevel(0); l <= spec.DefaultMaxLevel; l++ {
		files, err := filepath.Glob(l.GetSSTPath("sst*.json"))
		if err != nil {
			fmt.Printf("Failed to cleanup sst-files in %s post test case execution\n", l.FolderString())
			continue
		}

		for _, f := range files {
			_ = os.Remove(f)
		}
	}
}

func cleanWAL() {
	if wal.fileHandle != nil {
		wal.fileHandle.Close()
		wal.fileHandle = nil
	}
	_ = os.Remove(common.WALName)
}

func cleanManifest() {
	manifest = make(map[spec.SSTLevel][]*spec.SSTMetaData)
	_ = os.Remove(common.ManifestName)
	if config.Conf.NextFileID != nil {
		for i := range config.Conf.NextFileID {
			config.Conf.NextFileID[i] = 0
		}
	}
}

func memTableGenerator(num int) map[string]Value {
	testMemTable := map[string]Value{}
	for i := 1; i <= num; i++ {
		testMemTable[fmt.Sprintf("key%d", i)] = Value{Value: fmt.Sprintf("val%d", i)}
	}
	return testMemTable
}

func sstDataGenerator(start int, end int, level spec.SSTLevel) []*spec.SSTEntry {
	entries := []*spec.SSTEntry{}
	for i := start; i <= end; i++ {
		key := fmt.Sprintf("key%05d", i)
		value := fmt.Sprintf("val%d", level)

		entry := spec.SSTEntry{
			Key: key,
		}
		if i%3 == 0 {
			entry.Tombstone = true
		} else {
			entry.Value = &value
		}
		entries = append(entries, &entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries
}

func sstGenerator(start int, end int, level spec.SSTLevel) (*spec.SSTMetaData, error) {
	return writeSST(sstDataGenerator(start, end, level), level)
}
