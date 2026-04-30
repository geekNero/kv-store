package config

type Config struct {
	// Accessing the NextFileID using the level as index
	NextFileID []uint64 `json:"next_file_id_per_level"`

	// Used for compaction trigger
	LevelSize []int `json:"level_capacity"`

	// Maximum number of SST Levels to have
	MaxLevels int `json:"last_sst_level"`
}

const configPath string = "config.json"
