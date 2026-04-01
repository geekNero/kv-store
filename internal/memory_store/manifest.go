package memorystore

import (
	"encoding/json"
	"kv_store/internal/utility"
	"log"
	"os"
	"slices"
)

func flushManifest() error {
	var f *os.File
	var err error
	// handle file create
	f, err = os.Create(utility.ManifestName)
	if err != nil {
		log.Println("failed to create manifest file")
		return err
	}
	defer f.Close()

	marshalledOut, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		log.Println("failed to marshal manifest content")
		return err
	}
	_, err = f.Write(marshalledOut)
	if err != nil {
		log.Println("failed to write marshalled content to disk")
		return err
	}

	return f.Sync()
}

// Section of functions that contain code to setup the memory store
func loadManifest() error {
	entries, err := os.ReadDir(".")
	if err != nil {
		log.Println("failed to list files in current directory")
		return err
	}
	bytes, err := os.ReadFile(utility.ManifestName)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("failed to read manifest file")
			return err
		}

		for _, entry := range entries {
			if utility.IsSSTFile(entry.Name()) {
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
			log.Println("failed to unmarshal manifest file")
			return err
		}
		for _, entry := range entries {
			if utility.IsSSTFile(entry.Name()) && utility.ExtractSSTFileNumber(entry.Name()) >= len(manifest) {
				err = os.Remove(entry.Name())
				if err != nil {
					log.Printf("failed to clean up dangling sst file - %s, error: %s\n", entry.Name(), err.Error())
				}
			}
		}
	}
	return nil
}
