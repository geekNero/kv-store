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
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		firstKey string
		lastKey  string
		sstSet   []*spec.SSTMetaData
		want     int
		want2    int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got2 := utility.FindSSTRange(tt.firstKey, tt.lastKey, tt.sstSet)
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("FindSSTRange() = %v, want %v", got, tt.want)
			}
			if true {
				t.Errorf("FindSSTRange() = %v, want %v", got2, tt.want2)
			}
		})
	}
}
