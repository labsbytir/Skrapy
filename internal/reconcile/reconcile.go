package reconcile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Result mirrors the collector output line.
type Result struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Fetched string `json:"fetched"`
}

// Dedup by URL, drop empty titles, normalize whitespace. Writes cleaned lines.
func Run(inPath, outPath string) (int, int, error) {
	inF, err := os.Open(inPath)
	if err != nil {
		return 0, 0, err
	}
	defer inF.Close()

	outF, err := os.Create(outPath)
	if err != nil {
		return 0, 0, err
	}
	defer outF.Close()

	seen := map[string]bool{}
	total := 0
	kept := 0
	scanner := bufio.NewScanner(inF)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	w := bufio.NewWriter(outF)
	defer w.Flush()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		total++
		var r Result
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue // skip malformed lines -> effectively quarantined
		}
		if r.URL == "" || strings.TrimSpace(r.Title) == "" {
			continue
		}
		if seen[r.URL] {
			continue
		}
		seen[r.URL] = true
		r.Title = strings.Join(strings.Fields(r.Title), " ")
		r.Text = strings.Join(strings.Fields(r.Text), " ")
		out, _ := json.Marshal(r)
		fmt.Fprintln(w, string(out))
		kept++
	}
	return kept, total, scanner.Err()
}