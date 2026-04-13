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
			NextFileID: make([]uint64, int(spec.MaxLevel)),
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

	return nil
}
