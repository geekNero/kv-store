package memorystore

import (
	"testing"

	"kv_store/internal/spec"
)

// TODO: Complete this test function after I am done with all the work surrounding multilevel compaction
func Test_compact(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		targetLevel spec.SSTLevel
		wantErr     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := compact(tt.targetLevel)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("compact() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("compact() succeeded unexpectedly")
			}
		})
	}
}
