package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRestSparseFieldset(t *testing.T) {
	Items = []Item{{ID: "1", Title: "T", URL: "http://x", Text: "body"}}
	req := httptest.NewRequest("GET", "/api/items?fields=id,title", nil)
	w := httptest.NewRecorder()
	RestHandler(w, req)
	var out []map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0]["id"] != "1" || out[0]["title"] != "T" {
		t.Fatalf("unexpected selective output: %v", out)
	}
	if _, ok := out[0]["text"]; ok {
		t.Fatalf("text should be excluded by fieldset: %v", out)
	}
}

func TestJSONRPCList(t *testing.T) {
	Items = []Item{{ID: "1", Title: "T"}}
	body := `{"jsonrpc":"2.0","method":"list","id":1}`
	req := httptest.NewRequest("POST", "/rpc", strings.NewReader(body))
	w := httptest.NewRecorder()
	JSONRPCHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("status %d", w.Result().StatusCode)
	}
}