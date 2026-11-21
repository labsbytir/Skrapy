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
	mux.HandleFunc("/api/items", api.RestHandler)
	mux.HandleFunc("/rpc", api.JSONRPCHandler)
	mux.Handle("/api/jobs", jobs)
	mux.Handle("/api/jobs/", jobs)
	log.Println("API listening on :8080 (REST + JSON-RPC + jobs)")
	log.Fatal(http.ListenAndServe(":8080", mux))
}