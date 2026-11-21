package s3store

// S3 multipart upload via s5cmd (primary) with rclone fallback.
// Progress parses any NN% tokens on child output; tools that stay
// quiet report 0 at start and 100 on success.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ProgressFunc receives upload percent (0-100); done is true on completion.
type ProgressFunc func(pct float64, done bool)

// Store holds the S3 destination resolved from the environment.
type Store struct {
	Bucket   string
	Prefix   string
	Endpoint string
}

// FromEnv builds a Store from S3_BUCKET (required), S3_PREFIX, S3_ENDPOINT.
func FromEnv() (*Store, error) {
	b := strings.Trim(os.Getenv("S3_BUCKET"), "/")
	if b == "" {
		return nil, errors.New("s3store: S3_BUCKET not set")
	}
	return &Store{
		Bucket:   b,
		Prefix:   strings.Trim(os.Getenv("S3_PREFIX"), "/"),
		Endpoint: strings.TrimRight(os.Getenv("S3_ENDPOINT"), "/"),
	}, nil
}

// Key maps a zip path to its object key.
func (s *Store) Key(zipPath string) string {
	base := filepath.Base(zipPath)
	if s.Prefix == "" {
		return base
	}
	return s.Prefix + "/" + base
}

var pctRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

func emitLines(out string, cb ProgressFunc) float64 {
	last := -1.0
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		for _, m := range pctRe.FindAllStringSubmatch(sc.Text(), -1) {
			var v float64
			fmt.Sscanf(m[1], "%f", &v)
			if v > last {
				last = v
				if cb != nil {
					cb(v, false)
				}
			}
		}
	}
	return last
}

// run executes cmd, streaming NN% progress to cb; 100 on clean exit.
func run(ctx context.Context, name string, args []string, cb ProgressFunc) error {
	cmd := exec.CommandContext(ctx, name, args...)
	b, err := cmd.CombinedOutput()
	last := emitLines(string(b), cb)
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(b)))
	}
	if last < 100 && cb != nil {
		cb(100, false)
	}
	if cb != nil {
		cb(100, true)
	}
	return nil
}

// UploadZip uploads zipPath, trying s5cmd first then rclone.
func (s *Store) UploadZip(ctx context.Context, zipPath string, cb ProgressFunc) error {
	dst := "s3://" + s.Bucket + "/" + s.Key(zipPath)
	rcloneArgs := []string{"copyto", zipPath, ":s3:" + s.Bucket + "/" + s.Key(zipPath), "--progress"}
	if s.Endpoint != "" {
		rcloneArgs = append(rcloneArgs, "--s3-endpoint", s.Endpoint)
	}
	if _, err := exec.LookPath("s5cmd"); err == nil {
		args := []string{}
		if s.Endpoint != "" {
			args = append(args, "--endpoint-url", s.Endpoint)
		}
		args = append(args, "cp", zipPath, dst)
		if err := run(ctx, "s5cmd", args, cb); err == nil {
			return nil
		} else if _, err2 := exec.LookPath("rclone"); err2 == nil {
			if err2 := run(ctx, "rclone", rcloneArgs, cb); err2 == nil {
				return nil
			}
			return err
		} else {
			return err
		}
	}
	if _, err := exec.LookPath("rclone"); err == nil {
		return run(ctx, "rclone", rcloneArgs, cb)
	}
	return errors.New("s3store: neither s5cmd nor rclone on PATH")
}
