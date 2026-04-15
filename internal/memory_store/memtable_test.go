package memorystore

import (
	"kv_store/internal/config"
	"kv_store/internal/spec"
	"os"
	"testing"
)

func Test_deleteMemtableEntry(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		setup func()
		want  bool
	}{
		{
			name: "KeyExists",
			key:  "k1",
			setup: func() {
				memStore["k1"] = Value{Value: "v1"}
			},
			want: true,
		},
		{
			name: "KeyDoesNotExist",
			key:  "k2",
			setup: func() {
				delete(memStore, "k2")
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got := deleteMemtableEntry(tt.key)
			if got != tt.want {
				t.Errorf("deleteMemtableEntry() = %v, want %v", got, tt.want)
			}
			if tt.want && !memStore[tt.key].Tombstone {
				t.Errorf("expected tombstone to be true for key %s", tt.key)
			}
		})
	}
}

func Test_writeSST(t *testing.T) {
	cleanManifest()
	cleanSSTFiles()
	defer cleanManifest()
	defer cleanSSTFiles()

	v1 := "v1"
	data := []*spec.SSTEntry{
		{Key: "k1", Value: &v1},
	}
	level := spec.SSTLevel(0)
	config.Conf.NextFileID[0] = 0

	meta, err := writeSST(data, level)
	if err != nil {
		t.Fatalf("writeSST failed: %v", err)
	}

	if meta.Name != "sst-0.json" {
		t.Errorf("expected sst-0.json, got %s", meta.Name)
	}

	path := level.GetSSTPath(meta.Name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("SST file not created at %s", path)
	}
}

func Test_getNextSSTName(t *testing.T) {
	config.Conf.NextFileID[0] = 5
	name := getNextSSTName(0)
	if name != "sst-5.json" {
		t.Errorf("expected sst-5.json, got %s", name)
	}
	if config.Conf.NextFileID[0] != 6 {
		t.Errorf("expected NextFileID to be 6, got %d", config.Conf.NextFileID[0])
	}
}
