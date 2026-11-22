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
	"time"
)

// ProgressFunc receives upload percent (0-100); done is true on completion.
type ProgressFunc func(pct float64, done bool)

// Store holds the S3 destination resolved from the environment.
type Store struct {
	Bucket   string
	Prefix   string
	Endpoint string
	// LastTool names the uploader used by the most recent attempt.
	LastTool string
	// Retries caps attempts per tool (default 3); Backoff slots sleep seconds.
	Retries int
	Backoff []int
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

// UploadZip uploads zipPath, trying s5cmd first then rclone, each with
// retries and backoff. On total failure both errors are joined.
func (s *Store) UploadZip(ctx context.Context, zipPath string, cb ProgressFunc) error {
	dst := "s3://" + s.Bucket + "/" + s.Key(zipPath)
	rcloneArgs := []string{"copyto", zipPath, ":s3:" + s.Bucket + "/" + s.Key(zipPath), "--progress"}
	if s.Endpoint != "" {
		rcloneArgs = append(rcloneArgs, "--s3-endpoint", s.Endpoint)
	}
	retries := s.Retries
	if retries < 1 {
		retries = 3
	}
	backoff := s.Backoff
	if len(backoff) == 0 {
		backoff = []int{1, 2, 4}
	}
	if _, err := exec.LookPath("s5cmd"); err == nil {
		args := []string{}
		if s.Endpoint != "" {
			args = append(args, "--endpoint-url", s.Endpoint)
		}
		args = append(args, "cp", zipPath, dst)
		s.LastTool = "s5cmd"
		if err := retry(ctx, retries, backoff, func() error { return run(ctx, "s5cmd", args, cb) }); err == nil {
			return nil
		} else if _, err2 := exec.LookPath("rclone"); err2 == nil {
			s.LastTool = "rclone"
			if err3 := retry(ctx, retries, backoff, func() error { return run(ctx, "rclone", rcloneArgs, cb) }); err3 == nil {
				return nil
			} else {
				return fmt.Errorf("s3store: s5cmd: %v; rclone: %v", err, err3)
			}
		} else {
			return err
		}
	}
	if _, err := exec.LookPath("rclone"); err == nil {
		s.LastTool = "rclone"
		return retry(ctx, retries, backoff, func() error { return run(ctx, "rclone", rcloneArgs, cb) })
	}
	return errors.New("s3store: neither s5cmd nor rclone on PATH")
}

// retry runs fn up to n times, sleeping backoff slots (seconds) between tries.
func retry(ctx context.Context, n int, backoff []int, fn func() error) error {
	var err error
	for i := 0; i < n; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < n-1 {
			secs := 1
			if i < len(backoff) {
				secs = backoff[i]
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(secs) * time.Second):
			}
		}
	}
	return err
}
