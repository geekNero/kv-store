package memorystore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"kv_store/internal/spec"
	"kv_store/internal/utility"

	"github.com/google/go-cmp/cmp"
)

func cleanSSTFiles() {
	// Cleanup any sst-files
	files, err := filepath.Glob("sst*")
	if err != nil {
		fmt.Println("Failed to cleanup sst-files post test case execution")
	}

	for _, f := range files {
		_ = os.Remove(f)
	}
}

func cleanWAL() {
	if wal.fileHandle != nil {
		wal.fileHandle.Close()
		wal.fileHandle = nil
	}
	_ = os.Remove(utility.WALName)
}

func cleanManifest() {
	manifest = []string{}
	_ = os.Remove(utility.ManifestName)
}

func Test_flushMemTable(t *testing.T) {
	type args struct {
		inputManifest []string
		sstFileName   string
	}

	memTableGenerator := func() map[string]value {
		testMemTable := map[string]value{
			"key1": {value: "val1"},
			"key2": {value: "val2"},
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
				keys := make([]string, 0, len(memTableGenerator()))
				for k := range memTableGenerator() {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				memTableSlice := make([]spec.PutRequest, 0, len(keys))
				for _, k := range keys {
					memTableSlice = append(memTableSlice, spec.PutRequest{Key: k, Value: memTableGenerator()[k].value})
				}
				if diff := cmp.Diff(sstData, memTableSlice); diff != "" {
					t.Errorf("failed: data don't match\n %s", diff)
				}

				if len(memStore) != 0 {
					t.Errorf("failed: memStore ain't empty")
				}
			}
			cleanSSTFiles()
		})
	}
}

func Test_checkSST(t *testing.T) {
	cleanupFunc := func() {
		memStore = make(map[string]value)
		manifest = make([]string, 0)
		negativeCache = make([]negativeCacheKey, negativeCacheLimit)
		negativeCachePointer = 0
		cleanSSTFiles()
	}

	tests := []struct {
		name          string
		key           string
		want          string
		want2         bool
		wantErr       bool
		prepareTest   func()
		postTestCheck func()
	}{
		{
			name:    "T1_Key_Present",
			key:     "json",
			want:    "yay",
			want2:   true,
			wantErr: false,
			prepareTest: func() {
				memStore = map[string]value{
					"key1": "val1",
					"key2": "val2",
					"json": "yay",
				}
				flushMemTable()
				memStore = map[string]value{
					"key3": "val3",
					"key1": "Val1",
				}
				flushMemTable()
			},
			postTestCheck: cleanSSTFiles,
		},
		{
			name:    "T2_Key_Not_Present",
			key:     "txt",
			want:    "",
			want2:   false,
			wantErr: false,
			prepareTest: func() {
				memStore = map[string]value{
					"key1": "val1",
					"key2": "val2",
					"json": "yay",
				}
				flushMemTable()
				memStore = map[string]value{
					"key3": "val3",
					"key1": "Val1",
				}
				flushMemTable()
			},
			postTestCheck: cleanSSTFiles,
		},
		{
			name:    "T3_Key_Not_Present_Negative_Cache",
			key:     "txt",
			want:    "",
			want2:   false,
			wantErr: false,
			prepareTest: func() {
				memStore = map[string]value{
					"key1": "val1",
					"key2": "val2",
					"json": "yay",
				}
				flushMemTable()
				memStore = map[string]value{
					"key3": "val3",
					"key1": "Val1",
					"txt":  "val-txt", // adding for verification
				}
				flushMemTable()
				memStore = map[string]value{
					"key3": "val3",
					"key1": "Val1",
				}
				flushMemTable()
				putNegativeCache("txt", 1)
			},
			postTestCheck: func() {
				val := fetchNegativeCache("txt")
				if val.man != 2 {
					t.Errorf("T3: manifest value not updated in negative cache: expected: %d, got: %d", 2, val.man)
				}
				cleanupFunc()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupFunc()
			if tt.prepareTest != nil {
				tt.prepareTest()
			}
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

			if got != tt.want {
				t.Errorf("checkSST() = %v, want %v", got, tt.want)
			}
			if got2 != tt.want2 {
				t.Errorf("checkSST() = %v, want %v", got2, tt.want2)
			}
			if tt.postTestCheck != nil {
				tt.postTestCheck()
			}
		})
	}
}

func Test_fetchNegativeCache(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		key         string
		want        negativeCacheKey
		prepareTest func()
	}{
		{
			name: "T1_Key_Present",
			key:  "json",
			want: negativeCacheKey{key: "json", man: 2},
			prepareTest: func() {
				negativeCache[0] = negativeCacheKey{key: "json", man: 2}
			},
		},
		{
			name: "T2_Key_Not_Present",
			key:  "txt",
			want: negativeCacheKey{key: "txt", man: -1},
			prepareTest: func() {
				negativeCache[2] = negativeCacheKey{key: "json", man: 2}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			negativeCache = make([]negativeCacheKey, negativeCacheLimit)
			negativeCachePointer = 0
			tt.prepareTest()
			got := fetchNegativeCache(tt.key)
			if got.key != tt.want.key || got.man != tt.want.man {
				t.Errorf("fetchNegativeCache() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_putNegativeCache(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		key          string
		man          int
		cacheCounter int
		index        int
		prepareTest  func()
	}{
		{
			name:         "T1-No_Wrap",
			key:          "json",
			man:          3,
			index:        3,
			cacheCounter: 3,
		},
		{
			name:         "T2-Wrap",
			key:          "json",
			man:          3,
			index:        0,
			cacheCounter: negativeCacheLimit,
		},
		{
			name:         "T3-Update_Key",
			key:          "json",
			man:          2,
			cacheCounter: 2,
			index:        0,
			prepareTest: func() {
				negativeCache[0] = negativeCacheKey{key: "json", man: 1}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			negativeCache = make([]negativeCacheKey, negativeCacheLimit)
			if tt.prepareTest != nil {
				tt.prepareTest()
			}
			negativeCachePointer = tt.cacheCounter
			putNegativeCache(tt.key, tt.man)
			checkVal := negativeCache[tt.index]
			if checkVal.key != tt.key || checkVal.man != tt.man {
				t.Errorf("incorrect value, expected: %s, %d; got: %v", tt.key, tt.man, checkVal)
			}
		})
	}
}

func Test_flushManifest(t *testing.T) {
	cleanupFunc := func() {
		manifest = make([]string, 0)
		_ = os.Remove(utility.ManifestName)
	}

	tests := []struct {
		name          string
		inputManifest []string
		wantErr       bool
		prepareTest   func()
	}{
		{
			name:          "T1-Empty_Manifest",
			inputManifest: []string{},
			wantErr:       false,
		},
		{
			name:          "T2-Populated_Manifest",
			inputManifest: []string{"sst-0.json", "sst-1.json"},
			wantErr:       false,
		},
		{
			name:          "T3-Overwrite_Existing_Manifest",
			inputManifest: []string{"sst-new.json"},
			wantErr:       false,
			prepareTest: func() {
				// Pre-create manifest with some old data
				oldManifest := []string{"sst-old.json"}
				data, _ := json.MarshalIndent(oldManifest, "", " ")
				_ = os.WriteFile(utility.ManifestName, data, 0o644)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupFunc()
			defer cleanupFunc()

			if tt.prepareTest != nil {
				tt.prepareTest()
			}

			manifest = tt.inputManifest
			gotErr := flushManifest()
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("flushManifest() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("flushManifest() succeeded unexpectedly")
			}

			// Verify file content
			fileContent, err := os.ReadFile(utility.ManifestName)
			if err != nil {
				t.Fatalf("failed to read manifest file: %v", err)
			}

			var gotManifest []string
			err = json.Unmarshal(fileContent, &gotManifest)
			if err != nil {
				t.Fatalf("failed to unmarshal manifest file: %v", err)
			}

			if diff := cmp.Diff(gotManifest, tt.inputManifest); diff != "" {
				t.Errorf("manifest data mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func Test_walWrite(t *testing.T) {
	tests := []struct {
		name string
		r    []*spec.WALRequest
	}{
		{
			name: "T1-BasicPut",
			r: []*spec.WALRequest{
				{
					Key:       "key1",
					Value:     "val1",
					Operation: utility.PUT,
				},
			},
		},
		{
			name: "T2-MultiplePut",
			r: []*spec.WALRequest{
				{
					Key:       "key1",
					Value:     "val1",
					Operation: utility.PUT,
				},
				{
					Key:       "key2",
					Value:     "val2",
					Operation: utility.PUT,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanWAL()
			// defer cleanWAL()
			err := loadWAL()
			if err != nil {
				t.Fatalf("loadWAL failed: %v", err)
			}

			for _, req := range tt.r {
				walWrite(req)
			}

			// Read back and verify
			f, err := os.Open(utility.WALName)
			if err != nil {
				t.Fatalf("failed to open WAL: %v", err)
			}
			defer f.Close()

			decoder := json.NewDecoder(f)
			for _, req := range tt.r {
				var got spec.WALRequest
				err = decoder.Decode(&got)
				if err != nil {
					t.Fatalf("failed to decode WAL entry: %v", err)
				}

				if got.Key != req.Key || got.Value != req.Value || got.Operation != req.Operation {
					t.Errorf("walWrite() = %v, want %v", got, req)
				}
				if got.Hash == 0 {
					t.Errorf("walWrite() hash is 0")
				}
			}
		})
	}
}

func Test_flushWAL(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "T1-Flush"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanWAL()
			defer cleanWAL()
			loadWAL()

			walWrite(&spec.WALRequest{Key: "k1", Value: "v1", Operation: utility.PUT})
			flushWAL()

			info, err := os.Stat(utility.WALName)
			if err != nil {
				t.Fatalf("failed to stat WAL: %v", err)
			}
			if info.Size() != 0 {
				t.Errorf("flushWAL() did not truncate file, size = %d", info.Size())
			}
		})
	}
}

func Test_loadWAL(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		want    map[string]string
		wantErr bool
	}{
		{
			name: "T1-Replay",
			setup: func() {
				cleanWAL()
				loadWAL()
				walWrite(&spec.WALRequest{Key: "k1", Value: "v1", Operation: utility.PUT})
				walWrite(&spec.WALRequest{Key: "k2", Value: "v2", Operation: utility.PUT})
				closeWAL()
			},
			want: map[string]string{"k1": "v1", "k2": "v2"},
		},
		{
			name: "T2-CorruptHash",
			setup: func() {
				cleanWAL()
				loadWAL()
				walWrite(&spec.WALRequest{Key: "k1", Value: "v1", Operation: utility.PUT})
				walWrite(&spec.WALRequest{Key: "k2", Value: "v2", Operation: utility.PUT})
				closeWAL()

				// Manually corrupt the file
				f, _ := os.OpenFile(utility.WALName, os.O_RDWR, 0o644)
				f.Seek(-5, 2) // go back a bit and change something
				f.Write([]byte("corruption"))
				f.Close()
			},
			want: map[string]string{"k1": "v1"}, // Should stop at corruption or skip the corrupt entry
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memStore = make(map[string]string)
			if tt.setup != nil {
				tt.setup()
			}

			err := loadWAL()
			if (err != nil) != tt.wantErr {
				t.Errorf("loadWAL() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(memStore, tt.want); diff != "" {
				t.Errorf("memStore mismatch (-got +want):\n%s", diff)
			}
			cleanWAL()
		})
	}
}

// Single scenario test
func Test_loadWALTruncation(t *testing.T) {
	memStore = make(map[string]string)
	cleanWAL()
	loadWAL()
	walWrite(&spec.WALRequest{Key: "k1", Value: "v1", Operation: utility.PUT})
	closeWAL()

	// Manually corrupt the file
	f, _ := os.OpenFile(utility.WALName, os.O_RDWR, 0o644)
	f.Seek(-5, 2) // go back a bit and change something
	f.Write([]byte("corruption"))
	f.Close()

	err := loadWAL()
	if err != nil {
		t.Errorf("loadWAL() error = %v", err)
	}

	if diff := cmp.Diff(memStore, map[string]string{}); diff != "" {
		t.Errorf("memStore mismatch (-got +want):\n%s", diff)
	}

	info, err := os.Stat(utility.WALName)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() != 0 {
		t.Errorf("WAL size should be 0 after truncating")
	}

	cleanWAL()
}

func Test_loadManifest(t *testing.T) {
	tests := []struct {
		name      string // description of this test case
		wantErr   bool
		setup     func()
		want      []string
		postCheck func(t *testing.T)
	}{
		{
			name:    "T1-CleanManifest_NoDanglingSSTs",
			wantErr: false,
			setup: func() {
				memStore = map[string]string{
					"key1": "val1",
					"key2": "val2",
					"json": "yay",
				}
				flushMemTable()
				memStore = map[string]string{
					"key3": "val3",
					"key1": "Val1",
					"txt":  "val-txt",
				}
				flushMemTable()
				flushManifest()
			},
			want: []string{
				"sst-0.json",
				"sst-1.json",
			},
		},
		{
			name:    "T2-NoManifest_WithDanglingSSTs",
			wantErr: false,
			setup: func() {
				memStore = map[string]string{
					"key1": "val1",
					"key2": "val2",
					"json": "yay",
				}
				flushMemTable()
				memStore = map[string]string{
					"key3": "val3",
					"key1": "Val1",
					"txt":  "val-txt",
				}
				flushMemTable()
				cleanManifest()
			},
			// In case of no manifest, the SSTs are considered as source of truth
			want: []string{
				"sst-0.json",
				"sst-1.json",
			},
		},
		{
			name:    "T3-CleanManifest_WithDanglingSSTs",
			wantErr: false,
			setup: func() {
				memStore = map[string]string{
					"key1": "val1",
					"key2": "val2",
					"json": "yay",
				}
				flushMemTable()
				memStore = map[string]string{
					"key3": "val3",
					"key1": "Val1",
					"txt":  "val-txt",
				}
				flushMemTable()
				flushManifest()
				memStore = map[string]string{
					"key3": "val3",
					"key1": "Val1",
				}
				flushMemTable()
				manifest = []string{}
			},
			want: []string{
				"sst-0.json",
				"sst-1.json",
			},
			postCheck: func(t *testing.T) {
				_, err := os.Stat("sst-2.json")

				if err == nil {
					t.Error("dangling sst present")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// start with clean manifest and sst files
			cleanManifest()
			cleanSSTFiles()
			defer cleanManifest()
			defer cleanSSTFiles()
			defer cleanWAL()

			if tt.setup != nil {
				tt.setup()
			}
			gotErr := loadManifest()
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("loadManifest() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("loadManifest() succeeded unexpectedly")
			}

			if len(tt.want) != len(manifest) {
				t.Errorf("manifest lengths don't match, got: %d, want: %d", len(tt.want), len(manifest))
			}

			for index, entry := range tt.want {
				if entry != manifest[index] {
					t.Errorf("entry in manifest does not match expected entry, found: %s, expected: %s", manifest[index], entry)
				}
			}

			if tt.postCheck != nil {
				tt.postCheck(t)
			}
		})
	}
}
