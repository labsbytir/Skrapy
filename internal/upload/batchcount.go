package upload

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SplitByCount cuts a JSONL file into chunk files of at most n lines each.
func SplitByCount(inPath, dir string, n int) ([]Chunk, error) {
	if n <= 0 {
		n = 1000
	}
	inF, err := os.Open(inPath)
	if err != nil {
		return nil, err
	}
	defer inF.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	var (
		outF  *os.File
		w     *bufio.Writer
		count int
		index int
		paths []string
	)
	closeChunk := func() error {
		if w != nil {
			if err := w.Flush(); err != nil {
				return err
			}
			if err := outF.Close(); err != nil {
				return err
			}
			w = nil
		}
		return nil
	}

	scanner := bufio.NewScanner(inF)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if outF == nil || count >= n {
			if err := closeChunk(); err != nil {
				return nil, err
			}
			path := filepath.Join(dir, fmt.Sprintf("batch-%04d.jsonl", index))
			outF, err = os.Create(path)
			if err != nil {
				return nil, err
			}
			w = bufio.NewWriter(outF)
			count = 0
			index++
			paths = append(paths, path)
		}
		fmt.Fprintln(w, line)
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := closeChunk(); err != nil {
		return nil, err
	}

	out := make([]Chunk, 0, len(paths))
	for i, path := range paths {
		fi, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		out = append(out, Chunk{Path: path, Index: i, Total: len(paths), Bytes: fi.Size()})
	}
	return out, nil
}

// ZipBatch zips one batch chunk as <jobID>-batch-<NNNN>.zip (single entry).
// Partial zips are removed on any failure.
func ZipBatch(dir, jobID string, index int) (string, error) {
	src := filepath.Join(dir, fmt.Sprintf("batch-%04d.jsonl", index))
	dst := filepath.Join(dir, fmt.Sprintf("%s-batch-%04d.zip", jobID, index))
	zf, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	fail := func(err error) (string, error) {
		zf.Close()
		os.Remove(dst)
		return "", err
	}
	zw := zip.NewWriter(zf)
	sf, err := os.Open(src)
	if err != nil {
		return fail(err)
	}
	defer sf.Close()
	fi, err := sf.Stat()
	if err != nil {
		return fail(err)
	}
	hdr, err := zip.FileInfoHeader(fi)
	if err != nil {
		return fail(err)
	}
	hdr.Name = filepath.Base(src)
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return fail(err)
	}
	if _, err := io.Copy(w, sf); err != nil {
		return fail(err)
	}
	if err := zw.Close(); err != nil {
		return fail(err)
	}
	return dst, zf.Close()
}
