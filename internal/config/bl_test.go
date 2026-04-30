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
		if len(Conf.LevelSize) != int(spec.DefaultMaxLevel) {
			t.Errorf("expected LevelSize length %d, got %d", int(spec.DefaultMaxLevel), len(Conf.LevelSize))
		}
		if Conf.MaxLevels != spec.DefaultMaxLevel {
			t.Errorf("expected MaxLevels %d, got %d", spec.DefaultMaxLevel, Conf.MaxLevels)
		}
	})

	t.Run("FlushAndLoad", func(t *testing.T) {
		Conf.NextFileID[0] = 100
		Conf.MaxLevels = 10
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

		if Conf.NextFileID[0] != 100 {
			t.Errorf("expected NextFileID[0] 100, got %d", Conf.NextFileID[0])
		}
		if Conf.MaxLevels != 10 {
			t.Errorf("expected Maxlevels 10, got: %d", Conf.MaxLevels)
		}
	})
}
