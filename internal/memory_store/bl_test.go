package memorystore

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_flushMemTable(t *testing.T) {

	type args struct {
		inputManifest []string
		sstFileName   string
	}

	memTableGenerator := func() map[string]string {

		testMemTable := map[string]string{
			"key1": "val1",
			"key2": "val2",
		}
		return testMemTable
	}

	tests := []struct {
		name string // description of this test case
		args
		want bool
	}{
		{
			name: "T1-No_Manifest_Files",
			args: args{inputManifest: []string{}, sstFileName: "sst-0.json"},
			want: true,
		},
		{
			name: "T2-Few_Manifest_Files",
			args: args{inputManifest: []string{
				"sst-0.json",
				"sst-1.json",
			}, sstFileName: "sst-2.json"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memStore = memTableGenerator()
			manifest = tt.inputManifest
			got := flushMemTable()
			if got == tt.want == true {
				sstFile, err := os.ReadFile(tt.sstFileName)
				if err != nil {
					t.Errorf("failed: unable to open sstFile - %s", tt.sstFileName)
				}
				sstData := make(map[string]string)
				err = json.Unmarshal(sstFile, sstData)

				if err != nil {
					t.Errorf("failed: unable to unmarshal sstFile - %s", tt.sstFileName)
				}

				if diff := cmp.Diff(sstData, memTableGenerator()); diff != "" {
					t.Errorf("failed: data don't match\n %s", diff)
				}
				if len(memStore) != 0 {
					t.Errorf("failed: memStore ain't empty")
				}
			}

		})
	}
}
