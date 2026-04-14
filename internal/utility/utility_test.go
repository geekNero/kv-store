package utility_test

import (
	"testing"

	"kv_store/internal/spec"
	"kv_store/internal/utility"
)

func TestExtractSSTFileNumber(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		sstName string
		want    int
	}{
		{
			name:    "T1-SingleDigit_File_Name",
			sstName: "sst-1.json",
			want:    1,
		},
		{
			name:    "T2-DoubleDigit_File_Name",
			sstName: "sst-20.json",
			want:    20,
		},
		{
			name:    "T3-IncorrectFileName",
			sstName: "sit-20.json",
			want:    -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utility.ExtractSSTFileNumber(tt.sstName)

			if got != tt.want {
				t.Errorf("ExtractSSTFileNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsASCII(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"ASCII", "hello", true},
		{"Non-ASCII", "héllo", false},
		{"Empty", "", true},
		{"Symbols", "!@#$%", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utility.IsASCII(tt.s); got != tt.want {
				t.Errorf("IsASCII() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashStruct(t *testing.T) {
	type testStruct struct {
		Key   string
		Value string
	}

	s1 := testStruct{Key: "key1", Value: "val1"}
	s2 := testStruct{Key: "key1", Value: "val1"}
	s3 := testStruct{Key: "key2", Value: "val2"}

	h1, err1 := utility.HashStruct(s1)
	h2, err2 := utility.HashStruct(s2)
	h3, err3 := utility.HashStruct(s3)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("HashStruct failed: %v, %v, %v", err1, err2, err3)
	}

	if h1 != h2 {
		t.Errorf("HashStruct(s1) != HashStruct(s2)")
	}

	if h1 == h3 {
		t.Errorf("HashStruct(s1) == HashStruct(s3)")
	}
}

func TestIsSSTFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"Valid", "sst-1.json", true},
		{"ValidMultiDigit", "sst-123.json", true},
		{"InvalidPrefix", "st-1.json", false},
		{"InvalidExtension", "sst-1.txt", false},
		{"NoNumber", "sst-.json", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utility.IsSSTFile(tt.path); got != tt.want {
				t.Errorf("IsSSTFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckPtrStringsEqual(t *testing.T) {
	s1 := "hello"
	s2 := "hello"
	s3 := "world"

	tests := []struct {
		name string
		a    *string
		b    *string
		want bool
	}{
		{"BothNil", nil, nil, true},
		{"OneNil", &s1, nil, false},
		{"OtherNil", nil, &s1, false},
		{"Equal", &s1, &s2, true},
		{"NotEqual", &s1, &s3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utility.CheckPtrStringsEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("CheckPtrStringsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindKeyContainingSST(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		key    string
		sstSet []*spec.SSTMetaData
		want   *spec.SSTMetaData
	}{
		{
			name: "T1_Key_Present_in_Middle_SST",
			key:  "json",
			sstSet: []*spec.SSTMetaData{
				{
					FirstKey: "abc",
					LastKey:  "beezlebub",
					Name:     "sst-1",
				},
				{
					FirstKey: "joker",
					LastKey:  "pharoh",
					Name:     "sst-2",
				},
				{
					FirstKey: "sleeper",
					LastKey:  "zof",
					Name:     "sst-3",
				},
			},
			want: &spec.SSTMetaData{
				Name: "sst-2",
			},
		},
		{
			name: "T2_Key_Present_in_LastSST",
			key:  "jsonk",
			sstSet: []*spec.SSTMetaData{
				{
					FirstKey: "abc",
					LastKey:  "beezlebub",
					Name:     "sst-1",
				},
				{
					FirstKey: "jesus",
					LastKey:  "joker",
					Name:     "sst-2",
				},
				{
					FirstKey: "jpmorgan",
					LastKey:  "zof",
					Name:     "sst-3",
				},
			},
			want: &spec.SSTMetaData{

				Name: "sst-3",
			},
		},
		{
			name: "T3_Key_Present_in_FirstSST",
			key:  "babe",
			sstSet: []*spec.SSTMetaData{
				{
					FirstKey: "abc",
					LastKey:  "beezlebub",
					Name:     "sst-1",
				},
				{
					FirstKey: "jesus",
					LastKey:  "joker",
					Name:     "sst-2",
				},
				{
					FirstKey: "jpmorgan",
					LastKey:  "zof",
					Name:     "sst-3",
				},
			},
			want: &spec.SSTMetaData{
				Name: "sst-1",
			},
		},
		{
			name: "T4_Key_Absent_P1",
			key:  "zzz",
			sstSet: []*spec.SSTMetaData{
				{
					FirstKey: "abc",
					LastKey:  "beezlebub",
					Name:     "sst-1",
				},
				{
					FirstKey: "jesus",
					LastKey:  "joker",
					Name:     "sst-2",
				},
				{
					FirstKey: "jpmorgan",
					LastKey:  "zof",
					Name:     "sst-3",
				},
			},
			want: nil,
		},
		{
			name: "T4_Key_Absent_P2",
			key:  "jpac",
			sstSet: []*spec.SSTMetaData{
				{
					FirstKey: "abc",
					LastKey:  "beezlebub",
					Name:     "sst-1",
				},
				{
					FirstKey: "jesus",
					LastKey:  "joker",
					Name:     "sst-2",
				},
				{
					FirstKey: "jpmorgan",
					LastKey:  "zof",
					Name:     "sst-3",
				},
			},
			want: nil,
		},
		{
			name: "T4_Key_Absent_P3",
			key:  "aao",
			sstSet: []*spec.SSTMetaData{
				{
					FirstKey: "abc",
					LastKey:  "beezlebub",
					Name:     "sst-1",
				},
				{
					FirstKey: "jesus",
					LastKey:  "joker",
					Name:     "sst-2",
				},
				{
					FirstKey: "jpmorgan",
					LastKey:  "zof",
					Name:     "sst-3",
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utility.FindKeyContainingSST(tt.key, tt.sstSet)
			if tt.want == nil && got != nil {
				t.Errorf("FindKeyContainingSST(), got %v, but expected nil", got)
			}

			if tt.want != nil && tt.want.Name != got.Name {
				t.Errorf("FindKeyContainingSST(), got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindSSTRange(t *testing.T) {
	sstSet := []*spec.SSTMetaData{
		{FirstKey: "apple", LastKey: "banana", Name: "sst-1"},
		{FirstKey: "cherry", LastKey: "date", Name: "sst-2"},
		{FirstKey: "elephant", LastKey: "fig", Name: "sst-3"},
		{FirstKey: "grape", LastKey: "honeydew", Name: "sst-4"},
	}

	tests := []struct {
		name      string
		firstKey  string
		lastKey   string
		wantStart int
		wantEnd   int
	}{
		{
			name:      "T1-NoOverlap-Before",
			firstKey:  "aardvark",
			lastKey:   "acorn",
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "T2-NoOverlap-Between",
			firstKey:  "blueberry",
			lastKey:   "cantaloupe",
			wantStart: 1,
			wantEnd:   1,
		},
		{
			name:      "T3-SingleOverlap",
			firstKey:  "avocado",
			lastKey:   "bayberry",
			wantStart: 0,
			wantEnd:   1,
		},
		{
			name:      "T4-MultipleOverlap",
			firstKey:  "avocado",
			lastKey:   "elderberry",
			wantStart: 0,
			wantEnd:   2,
		},
		{
			name:      "T5-ExactMatch",
			firstKey:  "cherry",
			lastKey:   "date",
			wantStart: 1,
			wantEnd:   2,
		},
		{
			name:      "T6-NoOverlap-After",
			firstKey:  "iris",
			lastKey:   "jackfruit",
			wantStart: 4,
			wantEnd:   4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := utility.FindSSTRange(tt.firstKey, tt.lastKey, sstSet)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Errorf("FindSSTRange() = (%v, %v), want (%v, %v)", gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
