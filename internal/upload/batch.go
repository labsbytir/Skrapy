package upload

import (
	"bufio"
	"fmt"
	"os"
)

// Chunk describes one upload slice.
type Chunk struct {
	Path   string
	Index  int
	Total  int
	Bytes  int64
}

// Split cuts a JSONL file into chunk files of at most maxBytes each.
func Split(inPath, dir string, maxBytes int64) ([]Chunk, error) {
	inF, err := os.Open(inPath)
	if err != nil {
		return nil, err
	}
	defer inF.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	var (
		outF   *os.File
		w      *bufio.Writer
		cur    int64
		index  int
		paths  []string
	)
	closeChunk := func() error {
		if w != nil {
			if err := w.Flush(); err != nil {
				return err
			}
			if err := outF.Close(); err != nil {
				return err
			}
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
		lineBytes := int64(len(line) + 1)
		if outF == nil || cur+lineBytes > maxBytes {
			if err := closeChunk(); err != nil {
				return nil, err
			}
			path := fmt.Sprintf("%s/chunk-%04d.jsonl", dir, index)
			outF, err = os.Create(path)
			if err != nil {
				return nil, err
			}
			w = bufio.NewWriter(outF)
			cur = 0
			index++
			paths = append(paths, path)
		}
		fmt.Fprintln(w, line)
		cur += lineBytes
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