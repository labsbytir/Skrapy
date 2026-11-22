package main

import (
	"log"
	"net/http"

	"github.com/labsbytir/Skrapy/internal/api"
)

func main() {
	store := api.NewJobStore()
	jobs := api.JobsHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/api/items", api.RestHandler)
	mux.HandleFunc("/rpc", api.JSONRPCHandler)
	mux.Handle("/api/jobs", jobs)
	mux.Handle("/api/jobs/", jobs)
	log.Println("API listening on :8080 (REST + JSON-RPC + jobs)")
	log.Fatal(http.ListenAndServe(":8080", mux))
}