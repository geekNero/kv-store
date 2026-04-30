package config

type Config struct {
	// Accessing the NextFileID using the level as index
	NextFileID []uint64 `json:"next_file_id_per_level"`

	// Used for compaction trigger
	LevelSize []int `json:"level_file_size"`

	// Maximum number of SST Levels to have
	MaxLevels int
}

const configPath string = "config.json"
