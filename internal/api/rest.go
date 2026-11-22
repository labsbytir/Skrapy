package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RESTHandler serves /api/items?fields=id,title
type Item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text"`
}

var Items []Item

func RestHandler(w http.ResponseWriter, r *http.Request) {
	fields := r.URL.Query().Get("fields")
	sel := map[string]bool{}
	if fields != "" {
		for _, f := range strings.Split(fields, ",") {
			sel[strings.TrimSpace(f)] = true
		}
	}
	out := []map[string]any{}
	for _, it := range Items {
		m := map[string]any{}
		if len(sel) == 0 || sel["id"] {
			m["id"] = it.ID
		}
		if len(sel) == 0 || sel["title"] {
			m["title"] = it.Title
		}
		if len(sel) == 0 || sel["url"] {
			m["url"] = it.URL
		}
		if len(sel) == 0 || sel["text"] {
			m["text"] = it.Text
		}
		out = append(out, m)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}