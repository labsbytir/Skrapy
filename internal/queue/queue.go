package queue

// FIFO link queue with crash-resume via state.json and optional dedupe.

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Item is one queued fetch with its crawl depth (seeds are 0).
type Item struct {
	URL   string `json:"url"`
	Depth int    `json:"depth"`
}

// Queue is a durable FIFO. Not safe for concurrent use; guard with a mutex.
type Queue struct {
	dir      string
	dedupe   bool
	pending  []Item
	inflight map[string]Item
	done     map[string]bool
	dead     map[string]string
}

type state struct {
	Pending  []Item            `json:"pending"`
	InFlight map[string]Item   `json:"inflight"`
	Done     map[string]bool   `json:"done"`
	Dead     map[string]string `json:"dead"`
}

// Open loads dir/state.json (or starts empty) with dedupe on/off.
func Open(dir string, dedupe bool) (*Queue, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	q := &Queue{dir: dir, dedupe: dedupe, inflight: map[string]Item{}, done: map[string]bool{}, dead: map[string]string{}}
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return q, nil
		}
		return nil, err
	}
	var st state
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	q.pending = st.Pending
	// Crash recovery: in-flight items go back to the head of pending.
	for _, it := range st.InFlight {
		if q.dedupe && (q.done[it.URL]) {
			continue
		}
		q.pending = append([]Item{it}, q.pending...)
	}
	if st.Done != nil {
		q.done = st.Done
	}
	if st.Dead != nil {
		q.dead = st.Dead
	}
	return q, nil
}

func (q *Queue) save() error {
	st := state{Pending: q.pending, InFlight: q.inflight, Done: q.done, Dead: q.dead}
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := filepath.Join(q.dir, "state.json.tmp")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(q.dir, "state.json"))
}

// Enqueue appends urls at depth, skipping done/pending when dedupe is on.
func (q *Queue) Enqueue(urls []string, depth int) error {
	seen := map[string]bool{}
	for _, it := range q.pending {
		seen[it.URL] = true
	}
	for _, u := range urls {
		if u == "" {
			continue
		}
		if q.dedupe && (q.done[u] || seen[u]) {
			continue
		}
		q.pending = append(q.pending, Item{URL: u, Depth: depth})
		seen[u] = true
	}
	return q.save()
}

// Next pops the head item, tracking it in-flight until Ack/Dead.
func (q *Queue) Next() (it Item, ok bool) {
	if len(q.pending) == 0 {
		return Item{}, false
	}
	it = q.pending[0]
	q.pending = q.pending[1:]
	q.inflight[it.URL] = it
	_ = q.save()
	return it, true
}

// Ack marks url done.
func (q *Queue) Ack(url string) {
	delete(q.inflight, url)
	q.done[url] = true
	_ = q.save()
}

// Dead records url with a failure reason after retries are exhausted.
func (q *Queue) Dead(url, reason string) {
	delete(q.inflight, url)
	q.dead[url] = reason
	_ = q.save()
}

// DoneCount reports completed fetches.
func (q *Queue) DoneCount() int { return len(q.done) }

// PendingCount reports queued fetches.
func (q *Queue) PendingCount() int { return len(q.pending) }
