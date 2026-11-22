package upload

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitByCountAndZip(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "results.jsonl")
	f, err := os.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2500; i++ {
		fmt.Fprintf(f, `{"url":"https://seed.example/p%d"}`+"\n", i)
	}
	f.Close()

	chunks, err := SplitByCount(in, filepath.Join(dir, "batches"), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunks %d, want 3", len(chunks))
	}

	zp, err := ZipBatch(filepath.Join(dir, "batches"), "job1", 0)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(zp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if len(zr.File) != 1 {
		t.Fatalf("zip entries %d, want 1", len(zr.File))
	}
	if zr.File[0].Name != "batch-0000.jsonl" {
		t.Fatalf("zip name %q", zr.File[0].Name)
	}
}
