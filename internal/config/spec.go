package config

type Config struct {
	// Accessing the NextFileID using the level as index
	NextFileID []uint64 `json:"next_file_id_per_level"`
}

const configPath string = "config.json"
