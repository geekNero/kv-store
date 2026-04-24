package config

import (
	"os"
	"testing"
	"kv_store/internal/spec"
)

func TestConfig(t *testing.T) {
	// Use a custom config path for testing if needed, but here we just ensure we cleanup
	defer os.Remove(configPath)
	os.Remove(configPath)

	t.Run("LoadNonExistent", func(t *testing.T) {
		err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}
		if Conf.CompactionBatchSize != 2 {
			t.Errorf("expected default CompactionBatchSize 2, got %d", Conf.CompactionBatchSize)
		}
		if len(Conf.LevelSize) != int(spec.MaxLevel)+1 {
			t.Errorf("expected LevelSize length %d, got %d", int(spec.MaxLevel)+1, len(Conf.LevelSize))
		}
	})

	t.Run("FlushAndLoad", func(t *testing.T) {
		Conf.CompactionBatchSize = 10
		Conf.NextFileID[0] = 100
		err := FlushConfig()
		if err != nil {
			t.Fatalf("FlushConfig failed: %v", err)
		}

		// Reset Conf
		Conf = Config{}
		err = LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if Conf.CompactionBatchSize != 10 {
			t.Errorf("expected CompactionBatchSize 10, got %d", Conf.CompactionBatchSize)
		}
		if Conf.NextFileID[0] != 100 {
			t.Errorf("expected NextFileID[0] 100, got %d", Conf.NextFileID[0])
		}
	})
}
