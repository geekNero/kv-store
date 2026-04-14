package config

import (
	"encoding/json"
	"kv_store/internal/spec"
	"log"
	"os"
)

var Conf Config

func LoadConfig() error {

	configFile, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("failed to open config file, error: ", err.Error())
			return err
		}

		Conf = Config{
			NextFileID:          make([]uint64, int(spec.MaxLevel)+1),
			CompactionBatchSize: 2,
			LevelSize:           make([]int, int(spec.MaxLevel)+1),
		}

		Conf.LevelSize[0] = 4

		// ensure folders for each level are created
		for l := spec.SSTLevel(0); l <= spec.MaxLevel; l++ {
			if l > spec.SSTLevel(0) {
				Conf.LevelSize[int(l)] = 1000
			}

			err := os.MkdirAll(l.FolderString(), 0755)
			if err != nil {
				log.Printf("failed to create level %d folder, error: %s", int(l), err.Error())
				return err
			}
		}

		return nil
	}

	err = json.Unmarshal(configFile, &Conf)
	if err != nil {
		log.Println("failed to unmarshal config file, error: ", err.Error())
		return err
	}

	return nil
}

func FlushConfig() error {

	f, err := os.Create(configPath)
	if err != nil {
		log.Println("failed to create config file, error: ", err.Error())
		return err
	}

	data, err := json.Marshal(Conf)
	if err != nil {
		log.Println("failed to json encode config file, error: ", err.Error())
		return err
	}

	_, err = f.Write(data)
	if err != nil {
		log.Println("failed to write data to config file", err.Error())
		return err
	}

	return f.Sync()
}
