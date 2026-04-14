package config

type Config struct {
	// Accessing the NextFileID using the level as index
	NextFileID []uint64 `json:"next_file_id_per_level"`

	// Used to determine batches for multi-level compaction
	CompactionBatchSize uint16 `json:"compaction_batch_size"`

	//Used for compaction trigger
	LevelSize []int `json:"level_file_size"`
}

const configPath string = "config.json"
