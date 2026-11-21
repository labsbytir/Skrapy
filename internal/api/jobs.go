package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Job is one crawl request: single active job at a time (global FIFO).
type Job struct {
	ID      string   `json:"jobId"`
	Seeds   []string `json:"seeds"`
	Status  string   `json:"status"` // queued | running | done | error
	Done    int      `json:"done"`
	Total   int      `json:"total"`
	Created string   `json:"created"`
}

// JobStore is an in-memory job registry.
type JobStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
	n    int
}

// NewJobStore returns an empty registry.
func NewJobStore() *JobStore { return &JobStore{jobs: map[string]*Job{}} }

// Submit queues seeds and returns the new job.
func (s *JobStore) Submit(seeds []string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	j := &Job{
		ID:      fmt.Sprintf("job-%s-%04d", time.Now().UTC().Format("20060102-150405"), s.n),
		Seeds:   seeds,
		Status:  "queued",
		Total:   len(seeds),
		Created: time.Now().UTC().Format(time.RFC3339),
	}
	s.jobs[j.ID] = j
	return j
}

// Get returns a copy of the job, if present.
func (s *JobStore) Get(id string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *j, true
}

func bearerOK(r *http.Request) bool {
	want := os.Getenv("API_TOKEN")
	if want == "" {
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// JobsHandler serves POST /api/jobs and GET /api/jobs/{id} behind Bearer auth.
func JobsHandler(store *JobStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !bearerOK(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Seeds []string `json:"seeds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		j := store.Submit(body.Seeds)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"jobId": j.ID})
	})
	mux.HandleFunc("/api/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if !bearerOK(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
		j, ok := store.Get(id)
		if !ok {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(j)
	})
	return mux
}
