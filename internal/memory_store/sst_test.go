package memorystore

import (
	"kv_store/internal/config"
	"kv_store/internal/spec"
	"kv_store/internal/utility"
	"os"
	"testing"
)

func Test_loadSST(t *testing.T) {

	val1 := "val1"

	tests := []struct {
		name     string
		filename string
		want     []spec.SSTEntry
		wantErr  bool
		setup    func()
	}{
		{
			name:     "T1-CleanSST",
			filename: "l0/sst-0.json",
			want: []spec.SSTEntry{
				{
					Key:   "key1",
					Value: &val1,
				},
				{
					Key:       "key2",
					Tombstone: true,
				},
			},
			setup: func() {
				cleanManifest()
				cleanSSTFiles()
				config.Conf.NextFileID[0] = 0
				memStore = memTableGenerator(2)
				entry2 := memStore["key2"]
				entry2.Tombstone = true
				memStore["key2"] = entry2
				flushMemTable()
			},
		},
		{
			name:     "T2-NoSST",
			filename: "l0/sst-99.json",
			wantErr:  true,
			setup: func() {
				cleanManifest()
				cleanSSTFiles()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanManifest()
			defer cleanSSTFiles()
			if tt.setup != nil {
				tt.setup()
			}
			got, gotErr := loadSST(tt.filename)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("loadSST() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("loadSST() succeeded unexpectedly")
			}
			if len(tt.want) != len(got) {
				t.Fatalf("loadSST() length mismatch: got %d, want %d", len(got), len(tt.want))
			}
			for index, value := range tt.want {
				if got[index].Key != value.Key || !utility.CheckPtrStringsEqual(got[index].Value, value.Value) || got[index].Tombstone != value.Tombstone {
					t.Errorf("loadSST() output mismatch at index %d, got: %+v, want: %+v", index, got[index], value)
				}
			}
		})
	}
}

func Test_resetNegativeCache(t *testing.T) {
	negativeCache[0] = negativeCacheKey{key: "k1", sstNum: 1}
	resetNegativeCache()
	for _, nc := range negativeCache {
		if nc.key != "" {
			t.Errorf("expected empty cache, got %+v", nc)
		}
	}
}

func Test_cleanupOrphanedSSTs(t *testing.T) {
	cleanManifest()
	cleanSSTFiles()
	defer cleanManifest()
	defer cleanSSTFiles()

	level := spec.SSTLevel(0)
	manifest[level] = []*spec.SSTMetaData{{Name: "sst-0.json"}}
	
	tmpFile := level.GetSSTPath("test.tmp")
	os.WriteFile(tmpFile, []byte("test"), 0644)

	cleanupOrphanedSSTs()

	if _, err := os.Stat(tmpFile); err == nil {
		t.Errorf("expected .tmp file to be removed")
	}
}

