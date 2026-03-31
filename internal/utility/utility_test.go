package utility_test

import (
	"testing"

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
