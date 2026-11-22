package s3store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadZipRetriesThenSucceeds(t *testing.T) {
	dir := t.TempDir()
	// s5cmd fails twice, succeeds on third; counter in file.
	shim(t, dir, "s5cmd", `n=$(cat "`+dir+`/n" 2>/dev/null || echo 0); n=$((n+1)); echo $n > "`+dir+`/n"; if [ $n -lt 3 ]; then echo flaky >&2; exit 1; fi; echo "100%"`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("S3_BUCKET", "bkt")
	st, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	done := false
	if err := st.UploadZip(context.Background(), "/tmp/x.zip", func(p float64, d bool) { calls++; done = done || d }); err != nil {
		t.Fatalf("should succeed after retries: %v", err)
	}
	if !done || calls == 0 {
		t.Fatalf("calls=%d done=%v", calls, done)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "n"))
	if string(raw) != "3\n" {
		t.Fatalf("attempts %q, want 3", raw)
	}
}
