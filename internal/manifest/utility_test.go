package manifest

import (
	"os"
	"testing"
)

func Test_cleanupOrphanedSSTs(t *testing.T) {
	cleanManifest()
	cleanSSTFiles()
	defer cleanManifest()
	defer cleanSSTFiles()

	level := spec.SSTLevel(0)
	manifest[level] = []*spec.SSTMetaData{{Name: "sst-0.json"}}

	tmpFile := level.GetSSTPath("test.tmp")
	os.WriteFile(tmpFile, []byte("test"), 0o644)

	compactionOrphanedSST := level.GetSSTPath("sst-32.json")
	os.WriteFile(compactionOrphanedSST, []byte("test"), 0o644)

	cleanupOrphanedSSTs()

	if _, err := os.Stat(tmpFile); err == nil {
		t.Errorf("expected .tmp file to be removed")
	}
}
