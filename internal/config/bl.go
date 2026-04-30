package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"kv_store/internal/spec"
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
			NextFileID: make([]uint64, spec.DefaultMaxLevel+1),
			// Level size is not required for the last level
			LevelSize: make([]int, spec.DefaultMaxLevel),
			MaxLevels: spec.DefaultMaxLevel,
		}

		// ensure folders for each level are created
		for l := 0; l <= Conf.MaxLevels; l++ {
			if l < Conf.MaxLevels {
				Conf.LevelSize[l] = spec.DefaultLevelCapacity
			}
			err := os.MkdirAll(spec.SSTLevel(l).FolderString(), 0o755)
			if err != nil {
				log.Printf("failed to create level %d folder, error: %s", l, err.Error())
				return err
			}
		}
		Conf.LevelSize[0] = 4

		return nil
	}

	err = json.Unmarshal(configFile, &Conf)
	if err != nil {
		log.Println("failed to unmarshal config file, error: ", err.Error())
		return err
	}

	if !validateConfig() {
		return fmt.Errorf("config.json is invalid")
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

func validateConfig() bool {
	if Conf.MaxLevels%2 == 1 {
		log.Println("last_sst_level should be a multiple of 2")
		return false
	}

	if len(Conf.LevelSize) != Conf.MaxLevels {
		log.Println("level_capacity is not defined for every level")
		return false
	}

	for _, element := range Conf.LevelSize {
		if element <= 0 {
			log.Println("level_capacity has to have natural numbers only")
			return false
		}
	}

	return true
}
