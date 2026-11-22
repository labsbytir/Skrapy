package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJobsAuth(t *testing.T) {
	t.Setenv("API_TOKEN", "secret123")
	srv := httptest.NewServer(JobsHandler(NewJobStore()))
	defer srv.Close()

	// no token -> 401
	resp, err := http.Post(srv.URL+"/api/jobs", "application/json", strings.NewReader(`{"seeds":["https://a.example"]}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("no-token status %d, want 401", resp.StatusCode)
	}

	// wrong token -> 401
	req, _ := http.NewRequest("POST", srv.URL+"/api/jobs", strings.NewReader(`{"seeds":[]}`))
	req.Header.Set("Authorization", "Bearer nope")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("bad-token status %d, want 401", resp.StatusCode)
	}

	// good token -> 201 + jobId
	req, _ = http.NewRequest("POST", srv.URL+"/api/jobs", strings.NewReader(`{"seeds":["https://a.example","https://b.example"]}`))
	req.Header.Set("Authorization", "Bearer secret123")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d, want 201", resp.StatusCode)
	}
	var created struct {
		JobID string `json:"jobId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || created.JobID == "" {
		t.Fatalf("jobId: %+v %v", created, err)
	}

	// status endpoint reflects seeds
	req, _ = http.NewRequest("GET", srv.URL+"/api/jobs/"+created.JobID, nil)
	req.Header.Set("Authorization", "Bearer secret123")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var st Job
	if err := json.NewDecoder(resp2.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.Total != 2 || st.Status != "queued" {
		t.Fatalf("status: %+v", st)
	}
}

func TestJobsDeniedWithoutTokenConfigured(t *testing.T) {
	t.Setenv("API_TOKEN", "")
	srv := httptest.NewServer(JobsHandler(NewJobStore()))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/api/jobs", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("status %d, want 401", resp.StatusCode)
	}
}
