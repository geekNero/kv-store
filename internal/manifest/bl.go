package manifest

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"kv_store/internal/common"
	"kv_store/internal/spec"
)

var (
	manifest map[spec.SSTLevel][]*spec.SSTMetaData
	mutex    sync.RWMutex
)

func flushManifest() error {
	var f *os.File
	var err error
	tempFilename := "temp_" + common.ManifestName
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

	err = os.Rename(tempFilename, common.ManifestName)
	if err != nil {
		log.Println("failed to rename temporary manifest, error: ", err.Error())
		return err
	}
	return nil
}

// Section of functions that contain code to setup the memory store
func LoadManifest() error {
	bytes, err := os.ReadFile(common.ManifestName)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("failed to read manifest file, error: ", err.Error())
			return err
		}
		manifest = make(map[spec.SSTLevel][]*spec.SSTMetaData)
		// mutex =
	} else {
		err = json.Unmarshal(bytes, &manifest)
		if err != nil {
			log.Println("failed to unmarshal manifest file", err.Error())
			return err
		}
	}
	cleanupOrphanedSSTs()
	mutex = sync.RWMutex{}

	return nil
}
