package memorystore

import (
	"kv_store/internal/spec"
	"kv_store/internal/utility"
	"testing"
)

func Test_loadSST(t *testing.T) {

	val1 := "val1"

	tests := []struct {
		name     string // description of this test case
		filename string
		want     []spec.SSTEntry
		wantErr  bool
		setup    func()
	}{
		{
			name:     "T1-CleanSST",
			filename: "sst-0.json",
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
				loadManifest()
				memStore = memTableGenerator(2)
				entry2 := memStore["key2"]
				entry2.Tombstone = true
				memStore["key2"] = entry2
				flushMemTable()
			},
		},
		{
			name:     "T2-NoSST",
			filename: "sst-0.json",
			wantErr:  true,
			setup: func() {
				cleanManifest()
				cleanSSTFiles()
				loadManifest()
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
				t.Fatal("loadSST() output length does not match with expected output")
			}
			for index, value := range tt.want {
				if got[index].Key != value.Key || !utility.CheckPtrStringsEqual(got[index].Value, value.Value) || got[index].Tombstone != value.Tombstone {
					t.Errorf("loadSST() output does not match with expected output, got: %+v, want: %+v", got[index], value)
				}
			}
		})
	}
}
