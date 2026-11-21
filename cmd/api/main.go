package main

import (
	"log"
	"net/http"

	"github.com/labsbytir/Skrapy/internal/api"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/items", api.RestHandler)
	mux.HandleFunc("/rpc", api.JSONRPCHandler)
	log.Println("API listening on :8080 (REST + JSON-RPC)")
	log.Fatal(http.ListenAndServe(":8080", mux))
}