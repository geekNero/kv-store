package config

type Config struct {
	// Accessing the NextFileID using the level as index
	NextFileID []uint64 `json:"next_file_id_per_level"`

	// Used to determine batches for multi-level compaction
	CompactionBatchSize uint16
}

const configPath string = "config.json"
