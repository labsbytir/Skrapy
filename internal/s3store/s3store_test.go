package s3store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// shim writes an executable shell script that echoes progress then succeeds.
func shim(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestUploadZipS5cmdPrimary(t *testing.T) {
	dir := t.TempDir()
	shim(t, dir, "s5cmd", `echo "10%"; echo "55.5%"; echo done`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("S3_BUCKET", "bkt")
	t.Setenv("S3_PREFIX", "pre/")

	st, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	var pcts []float64
	done := false
	err = st.UploadZip(context.Background(), "/tmp/x.zip", func(p float64, d bool) {
		pcts = append(pcts, p)
		done = done || d
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pcts) < 2 || pcts[len(pcts)-1] != 100 || !done {
		t.Fatalf("progress %+v done=%v", pcts, done)
	}
}

func TestUploadZipRcloneFallback(t *testing.T) {
	dir := t.TempDir()
	shim(t, dir, "s5cmd", `echo boom >&2; exit 1`)
	shim(t, dir, "rclone", `echo "Transferred: 50%"; echo "Transferred: 100%"`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("S3_BUCKET", "bkt")

	st, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	var last float64
	done := false
	if err := st.UploadZip(context.Background(), "/tmp/x.zip", func(p float64, d bool) {
		last, done = p, d
	}); err != nil {
		t.Fatal(err)
	}
	if last != 100 || !done {
		t.Fatalf("last=%v done=%v", last, done)
	}
}

func TestUploadZipNoBucket(t *testing.T) {
	t.Setenv("S3_BUCKET", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected bucket error")
	}
}
