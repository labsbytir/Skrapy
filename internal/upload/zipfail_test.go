package upload

import (
	"os"
	"path/filepath"
	"testing"
)

func TestZipBatchCleansPartialOnFailure(t *testing.T) {
	dir := t.TempDir()
	bd := filepath.Join(dir, "batches")
	if err := os.Mkdir(bd, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ZipBatch(bd, "job1", 0); err == nil {
		t.Fatal("expected error for missing batch file")
	}
	matches, _ := filepath.Glob(filepath.Join(bd, "*.zip"))
	if len(matches) != 0 {
		t.Fatalf("partial zip left: %v", matches)
	}
}
