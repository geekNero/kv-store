package memorystore

import "testing"

func Test_deleteMemtableEntry(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		key  string
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deleteMemtableEntry(tt.key)
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("deleteMemtableEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}
