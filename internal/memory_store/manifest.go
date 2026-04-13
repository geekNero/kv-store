package memorystore

import (
	"encoding/json"
	"log"
	"os"

	"kv_store/internal/utility"
)

func flushManifest() error {
	var f *os.File
	var err error
	tempFilename := "temp_" + utility.ManifestName
	// handle file create
	f, err = os.Create(tempFilename)
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

	err = f.Sync()
	if err != nil {
		log.Println("failed to sync temporary manifest to disk, error: ", err.Error())
		return err
	}

	err = os.Rename(tempFilename, utility.ManifestName)
	if err != nil {
		log.Println("failed to rename temporary manifest, error: ", err.Error())
		return err
	}
	return nil
}

// Section of functions that contain code to setup the memory store
func loadManifest() error {
	bytes, err := os.ReadFile(utility.ManifestName)
	if err != nil {
		log.Println("failed to read manifest file, error: ", err.Error())
		return err
	}
	err = json.Unmarshal(bytes, &manifest)
	if err != nil {
		log.Println("failed to unmarshal manifest file")
		return err
	}

	cleanupOrphanedSSTs()

	return nil
}
