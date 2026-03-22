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
