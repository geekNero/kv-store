package manifest

import (
	"log"
	"os"
	"path/filepath"

	"kv_store/internal/utility"
)

func cleanupOrphanedSSTs() {
	for key := range manifest {
		entries, err := os.ReadDir(key.FolderString())
		if err != nil {
			log.Printf("failed to read %s directory, error: %s", key.FolderString(), err.Error())
		}

		// delete all .tmp files and keep track of all ssts files to weed out orphaned ones later on.
		// keys are file names and value is whether the sst is to be deleted or not.
		ssts := map[string]bool{}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".tmp" {
				os.Remove(key.GetSSTPath(entry.Name()))
			} else if utility.IsSSTFile(entry.Name()) {
				// mark each sst as to be deleted initially
				ssts[entry.Name()] = true
			}
		}

		// delete the files that are not present in the manifest.
		// such files can be created when the server crashes mid-compaction
		for _, entry := range manifest[key] {
			ssts[entry.Name] = false
		}

		for filename, toBeDeleted := range ssts {
			if toBeDeleted {
				os.Remove(key.GetSSTPath(filename))
			}
		}
	}
}
