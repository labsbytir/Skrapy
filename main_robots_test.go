package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRobotsGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User-agent: Googlebot\nDisallow: /g/\n\nUser-agent: *\nDisallow: /p/\n"))
	}))
	defer srv.Close()
	host := srv.Listener.Addr().String()
	_ = host
	u := srv.URL + "/p/x"
	if !robotsBlocked(map[string][]string{}, u) {
		t.Fatal("/p/x should be blocked by * group")
	}
	cache := map[string][]string{}
	robotsBlocked(cache, u)
	if robotsBlocked(cache, srv.URL+"/g/y") {
		t.Fatal("/g/y is Googlebot-only, should not block skrapy")
	}
	if robotsBlocked(cache, srv.URL+"/open") {
		t.Fatal("/open should be allowed")
	}
}
