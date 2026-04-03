package utility

const (
	// Size Limits
	MemTableSize      = 2000
	CompactionTrigger = 10000

	// File Names
	ManifestName = "manifest.json"
	WALName      = "wal.db"

	// Operations
	PUT    = "put"
	DELETE = "deleted"
)
