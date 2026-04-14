package memorystore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"kv_store/internal/config"
	"kv_store/internal/spec"
	"kv_store/internal/utility"

	"github.com/google/go-cmp/cmp"
)

func TestMain(m *testing.M) {
	config.LoadConfig()
	// Ensure directories exist
	for l := spec.SSTLevel(0); l <= spec.MaxLevel; l++ {
		os.MkdirAll(l.FolderString(), 0755)
	}
	os.Exit(m.Run())
}

func cleanSSTFiles() {
	// Cleanup any sst-files in all levels
	for l := spec.SSTLevel(0); l <= spec.MaxLevel; l++ {
		files, err := filepath.Glob(l.GetSSTPath("sst*.json"))
		if err != nil {
			fmt.Printf("Failed to cleanup sst-files in %s post test case execution\n", l.FolderString())
			continue
		}

		for _, f := range files {
			_ = os.Remove(f)
		}
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
	manifest = make(map[spec.SSTLevel][]*spec.SSTMetaData)
	_ = os.Remove(utility.ManifestName)
	if config.Conf.NextFileID != nil {
		for i := range config.Conf.NextFileID {
			config.Conf.NextFileID[i] = 0
		}
	}
}

func memTableGenerator(num int) map[string]Value {

	testMemTable := map[string]Value{}
	for i := 1; i <= num; i++ {
		testMemTable[fmt.Sprintf("key%d", i)] = Value{Value: fmt.Sprintf("val%d", i)}
	}
	return testMemTable
}

func sstGenerator(start int, end int, level spec.SSTLevel) (*spec.SSTMetaData, error) {
	entries := []*spec.SSTEntry{}
	for i := start; i <= end; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("val%d", i)

		entry := spec.SSTEntry{
			Key: key,
		}
		if i%3 == 0 {
			entry.Tombstone = true
		} else {
			entry.Value = &value
		}
		entries = append(entries, &entry)
	}

	return writeSST(entries, level)
}

func Test_flushMemTable(t *testing.T) {
	type args struct {
		inputNextSSTID uint64
		sstFileName    string
	}

	tests := []struct {
		name string // description of this test case
		args
		want             bool
		setManifest      map[spec.SSTLevel][]*spec.SSTMetaData
		expectedManifest map[spec.SSTLevel][]*spec.SSTMetaData
	}{
		{
			name: "T1-No_Manifest_Files",
			args: args{inputNextSSTID: 0, sstFileName: spec.SSTLevel(0).GetSSTPath("sst-0.json")},
			want: true,
			setManifest: map[spec.SSTLevel][]*spec.SSTMetaData{
				spec.SSTLevel(0): {},
			},
			expectedManifest: map[spec.SSTLevel][]*spec.SSTMetaData{
				spec.SSTLevel(0): {
					{
						Name:     "sst-0.json",
						FirstKey: "key1",
						LastKey:  "key2",
					},
				},
			},
		},
		{
			name: "T2-Few_Manifest_Files",
			args: args{inputNextSSTID: 2, sstFileName: spec.SSTLevel(0).GetSSTPath("sst-2.json")},
			want: true,
			setManifest: map[spec.SSTLevel][]*spec.SSTMetaData{
				spec.SSTLevel(0): {
					{
						Name: "sst-0.json",
					},
					{
						Name: "sst-1.json",
					},
				},
			},
			expectedManifest: map[spec.SSTLevel][]*spec.SSTMetaData{
				spec.SSTLevel(0): {
					{
						Name: "sst-0.json",
					},
					{
						Name: "sst-1.json",
					},
					{
						Name:     "sst-2.json",
						FirstKey: "key1",
						LastKey:  "key2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest = tt.setManifest
			memStore = memTableGenerator(2)
			config.Conf.NextFileID[0] = tt.inputNextSSTID
			got := flushMemTable()
			if got == tt.want {
				sstFile, err := os.ReadFile(tt.sstFileName)
				if err != nil {
					t.Errorf("failed: unable to open sstFile - %s, err: %v", tt.sstFileName, err)
				}
				sstData := []spec.SSTEntry{}
				err = json.Unmarshal(sstFile, &sstData)
				if err != nil {
					t.Errorf("failed: unable to unmarshal sstFile - %s, err: %v", tt.sstFileName, err)
				}

				// Convert map to slice of SSTEntry for comparison
				keys := make([]string, 0, 2)
				for k := range memTableGenerator(2) {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				memTableSlice := make([]spec.SSTEntry, 0, len(keys))
				for _, k := range keys {
					val := memTableGenerator(2)[k].Value
					memTableSlice = append(memTableSlice, spec.SSTEntry{Key: k, Value: &val})
				}
				if diff := cmp.Diff(sstData, memTableSlice); diff != "" {
					t.Errorf("failed: data don't match\n %s", diff)
				}

				if len(memStore) != 0 {
					t.Errorf("failed: memStore ain't empty")
				}

				if diff := cmp.Diff(manifest, tt.expectedManifest); diff != "" {
					t.Errorf("failed: mainfest not as expected, diff: %s", diff)
				}
			}
			cleanSSTFiles()
		})
	}
}

func Test_checkSST(t *testing.T) {
	cleanupFunc := func() {
		memStore = make(map[string]Value)
		negativeCache = make([]negativeCacheKey, negativeCacheLimit)
		negativeCachePointer = 0
		cleanManifest()
		cleanSSTFiles()
	}

	tests := []struct {
		name          string
		key           string
		want          string
		wantPresent   bool
		wantErr       bool
		prepareTest   func()
		postTestCheck func()
	}{
		{
			name:        "T1_Key_Present_in_L0",
			key:         "key2",
			want:        "val2",
			wantPresent: true,
			wantErr:     false,
			prepareTest: func() {

				config.Conf.NextFileID[0] = 0
				config.Conf.NextFileID[1] = 1
				metadata, _ := sstGenerator(1, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(90, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(105, 200, spec.SSTLevel(1))
				manifest[spec.SSTLevel(1)] = append(manifest[spec.SSTLevel(1)], metadata)
			},
		},
		{
			name:        "T2_Key_Not_Present",
			key:         "txt",
			want:        "",
			wantPresent: false,
			wantErr:     false,
			prepareTest: func() {

				config.Conf.NextFileID[0] = 0
				config.Conf.NextFileID[1] = 1
				metadata, _ := sstGenerator(1, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(90, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(105, 200, spec.SSTLevel(1))
				manifest[spec.SSTLevel(1)] = append(manifest[spec.SSTLevel(1)], metadata)
			},
		},
		{
			name:        "T3_Key_Not_Present_Negative_Cache",
			key:         "txt",
			want:        "",
			wantPresent: false,
			wantErr:     false,
			prepareTest: func() {

				config.Conf.NextFileID[0] = 0
				config.Conf.NextFileID[1] = 1
				metadata, _ := sstGenerator(1, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(90, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(105, 200, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)
				putNegativeCache("txt", 1)
			},
			postTestCheck: func() {
				val := fetchNegativeCache("txt")
				if val.sstNum != 2 {
					t.Errorf("T3: sstNum value not updated in negative cache: expected: %d, got: %d", 2, val.sstNum)
				}
			},
		},
		{
			name:        "T4_Key_Present_in_l1",
			key:         "key110",
			want:        "val110",
			wantPresent: true,
			wantErr:     false,
			prepareTest: func() {

				config.Conf.NextFileID[0] = 0
				config.Conf.NextFileID[1] = 1
				metadata, _ := sstGenerator(1, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(90, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(105, 200, spec.SSTLevel(1))
				manifest[spec.SSTLevel(1)] = append(manifest[spec.SSTLevel(1)], metadata)
			},
		},
		{
			name:        "T5_Key_Deleted",
			key:         "90",
			want:        "",
			wantPresent: false,
			wantErr:     false,
			prepareTest: func() {
				config.Conf.NextFileID[0] = 0
				config.Conf.NextFileID[1] = 1
				metadata, _ := sstGenerator(1, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(90, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(105, 200, spec.SSTLevel(1))
				manifest[spec.SSTLevel(1)] = append(manifest[spec.SSTLevel(1)], metadata)
			},
			postTestCheck: func() {
				val := fetchNegativeCache("90")
				if val.sstNum != 1 {
					t.Errorf("T4: sstNum value not updated in negative cache: expected: %d, got: %d", 2, val.sstNum)
				}
			},
		},
		{
			name:        "T6_Key_Delete_in_l2",
			key:         "key300",
			want:        "",
			wantPresent: false,
			wantErr:     false,
			prepareTest: func() {

				config.Conf.NextFileID[0] = 0
				config.Conf.NextFileID[1] = 1
				config.Conf.NextFileID[2] = 0
				metadata, _ := sstGenerator(1, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(90, 100, spec.SSTLevel(0))
				manifest[spec.SSTLevel(0)] = append(manifest[spec.SSTLevel(0)], metadata)

				metadata, _ = sstGenerator(105, 200, spec.SSTLevel(1))
				manifest[spec.SSTLevel(1)] = append(manifest[spec.SSTLevel(1)], metadata)

				metadata, _ = sstGenerator(300, 400, spec.SSTLevel(2))
				manifest[spec.SSTLevel(2)] = append(manifest[spec.SSTLevel(2)], metadata)
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
			got, gotErr := checkSST(tt.key)
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("checkSST() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			if got != nil && *got != tt.want {
				t.Errorf("checkSST() = %v, want %v", *got, tt.want)
			}
			if (got != nil) != tt.wantPresent {
				t.Errorf("checkSST() presence = %v, want %v", got != nil, tt.wantPresent)
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
			want: negativeCacheKey{key: "json", sstNum: 2},
			prepareTest: func() {
				negativeCache[0] = negativeCacheKey{key: "json", sstNum: 2}
			},
		},
		{
			name: "T2_Key_Not_Present",
			key:  "txt",
			want: negativeCacheKey{key: "txt", sstNum: -1},
			prepareTest: func() {
				negativeCache[2] = negativeCacheKey{key: "json", sstNum: 2}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			negativeCache = make([]negativeCacheKey, negativeCacheLimit)
			negativeCachePointer = 0
			tt.prepareTest()
			got := fetchNegativeCache(tt.key)
			if got.key != tt.want.key || got.sstNum != tt.want.sstNum {
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
		sstNum       int
		cacheCounter int
		index        int
		prepareTest  func()
	}{
		{
			name:         "T1-No_Wrap",
			key:          "json",
			sstNum:       3,
			index:        3,
			cacheCounter: 3,
		},
		{
			name:         "T2-Wrap",
			key:          "json",
			sstNum:       3,
			index:        0,
			cacheCounter: negativeCacheLimit,
		},
		{
			name:         "T3-Update_Key",
			key:          "json",
			sstNum:       2,
			cacheCounter: 2,
			index:        0,
			prepareTest: func() {
				negativeCache[0] = negativeCacheKey{key: "json", sstNum: 1}
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
			putNegativeCache(tt.key, tt.sstNum)
			checkVal := negativeCache[tt.index]
			if checkVal.key != tt.key || checkVal.sstNum != tt.sstNum {
				t.Errorf("incorrect value, expected: %s, %d; got: %v", tt.key, tt.sstNum, checkVal)
			}
		})
	}
}

func Test_flushManifest(t *testing.T) {
	cleanupFunc := func() {
		manifest = make(map[spec.SSTLevel][]*spec.SSTMetaData)
		_ = os.Remove(utility.ManifestName)
	}

	tests := []struct {
		name          string
		inputManifest map[spec.SSTLevel][]*spec.SSTMetaData
		wantErr       bool
		prepareTest   func()
	}{
		{
			name:          "T1-Empty_Manifest",
			inputManifest: make(map[spec.SSTLevel][]*spec.SSTMetaData),
			wantErr:       false,
		},
		{
			name: "T2-Populated_Manifest",
			inputManifest: map[spec.SSTLevel][]*spec.SSTMetaData{
				0: {{Name: "l0/sst-0.json", FirstKey: "a", LastKey: "b"}},
			},
			wantErr: false,
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
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("flushManifest() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			// Verify file content
			fileContent, err := os.ReadFile(utility.ManifestName)
			if err != nil {
				t.Fatalf("failed to read manifest file: %v", err)
			}

			var gotManifest map[spec.SSTLevel][]*spec.SSTMetaData
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
		{
			name: "T3-BasicDelete",
			r: []*spec.WALRequest{
				{
					Key:       "key1",
					Value:     "garbage",
					Operation: utility.DELETE,
				},
			},
		},
		{
			name: "T4-MixedPutAndDelete",
			r: []*spec.WALRequest{
				{
					Key:       "key1",
					Value:     "val1",
					Operation: utility.PUT,
				},
				{
					Key:       "key2",
					Value:     "garbage",
					Operation: utility.DELETE,
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

				if got.Key != req.Key || (req.Operation == utility.PUT && req.Value != got.Value) {
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
		want    map[string]Value
		wantErr bool
	}{
		{
			name: "T1-Replay",
			setup: func() {
				cleanWAL()
				loadWAL()
				walWrite(&spec.WALRequest{Key: "k1", Value: "v1", Operation: utility.PUT})
				walWrite(&spec.WALRequest{Key: "k2", Value: "v2", Operation: utility.PUT})
				walWrite(&spec.WALRequest{Key: "k2", Value: "", Operation: utility.DELETE})
				closeWAL()
			},
			want: map[string]Value{"k1": {Value: "v1", Tombstone: false}, "k2": {Value: "", Tombstone: true}},
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
			want: map[string]Value{"k1": {Value: "v1", Tombstone: false}}, // Should stop at corruption or skip the corrupt entry
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memStore = make(map[string]Value)
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
	memStore = make(map[string]Value)
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

	if diff := cmp.Diff(memStore, map[string]Value{}); diff != "" {
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
		want      map[spec.SSTLevel][]*spec.SSTMetaData
		postCheck func(t *testing.T)
	}{
		{
			name:    "T1-CleanManifest",
			wantErr: false,
			setup: func() {
				manifest = map[spec.SSTLevel][]*spec.SSTMetaData{
					0: {{Name: "sst-0.json", FirstKey: "key1", LastKey: "key2"}},
					2: {{Name: "sst-2.json", FirstKey: "key3", LastKey: "key4"}},
				}
				flushManifest()
			},
			want: map[spec.SSTLevel][]*spec.SSTMetaData{
				0: {{Name: "sst-0.json", FirstKey: "key1", LastKey: "key2"}},
				2: {{Name: "sst-2.json", FirstKey: "key3", LastKey: "key4"}},
			},
		},
		{
			name: "T2-CleanManfist_With_Orphaned_SST",
			setup: func() {
				manifest = map[spec.SSTLevel][]*spec.SSTMetaData{
					0: {{Name: "sst-0.json", FirstKey: "key1", LastKey: "key2"}},
					2: {{Name: "sst-2.json", FirstKey: "key3", LastKey: "key4"}},
				}
				flushManifest()
				level1 := spec.SSTLevel(1)
				sstName := level1.GetSSTPath("sst-1.json")
				sstName = sstName + ".tmp"
				f, err := os.Create(sstName)
				if err != nil {
					t.Fatalf("failed to create orphaned sst file: %v", err)
				}
				f.Close()
			},
			want: map[spec.SSTLevel][]*spec.SSTMetaData{
				0: {{Name: "sst-0.json", FirstKey: "key1", LastKey: "key2"}},
				2: {{Name: "sst-2.json", FirstKey: "key3", LastKey: "key4"}},
			},
			postCheck: func(t *testing.T) {
				level1 := spec.SSTLevel(1)
				sstName := level1.GetSSTPath("sst-1.json")
				sstName = sstName + ".tmp"
				if _, err := os.Stat(sstName); !os.IsNotExist(err) {
					t.Errorf("orphaned SST file was not cleaned up: %s", sstName)
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
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("loadManifest() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			if diff := cmp.Diff(manifest, tt.want); diff != "" {
				t.Errorf("manifest mismatch (-got +want):\n%s", diff)
			}

			if tt.postCheck != nil {
				tt.postCheck(t)
			}
		})
	}
}
