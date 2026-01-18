package memorystore

import (
	"encoding/json"
	"fmt"
	"kv_store/internal/spec"
	"os"
	"path/filepath"
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
				sstData := []spec.PutRequest{}
				err = json.Unmarshal(sstFile, &sstData)

				if err != nil {
					t.Errorf("failed: unable to unmarshal sstFile - %s", tt.sstFileName)
				}

				// Convert map to slice of PutRequest for comparison
				memTableSlice := make([]spec.PutRequest, 0, len(memTableGenerator()))
				for k, v := range memTableGenerator() {
					memTableSlice = append(memTableSlice, spec.PutRequest{Key: k, Value: v})
				}
				if diff := cmp.Diff(sstData, memTableSlice); diff != "" {
					t.Errorf("failed: data don't match\n %s", diff)
				}

				if len(memStore) != 0 {
					t.Errorf("failed: memStore ain't empty")
				}
			}

			// Cleanup any sst-files
			files, err := filepath.Glob("sst*")
			if err != nil {
				fmt.Println("Failed to cleanup sst-files post test case execution")
			}

			for _, f := range files {
				_ = os.Remove(f)
			}

		})
	}
}

func Test_checkSST(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		key     string
		want    string
		want2   bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got2, gotErr := checkSST(tt.key)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("checkSST() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("checkSST() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("checkSST() = %v, want %v", got, tt.want)
			}
			if true {
				t.Errorf("checkSST() = %v, want %v", got2, tt.want2)
			}
		})
	}
}
