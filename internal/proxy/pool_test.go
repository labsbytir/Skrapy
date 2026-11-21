package proxy

import (
	"testing"
)

func TestPoolRoundRobin(t *testing.T) {
	p := &Pool{URLs: []string{"http://p1", "http://p2", "http://p3"}}
	var got []string
	for i := 0; i < 6; i++ {
		got = append(got, p.Next())
	}
	want := []string{"http://p1", "http://p2", "http://p3", "http://p1", "http://p2", "http://p3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("seq %+v, want %+v", got, want)
		}
	}
}

func TestPoolSticky(t *testing.T) {
	p := &Pool{URLs: []string{"http://p1", "http://p2"}, Mode: "sticky"}
	for i := 0; i < 3; i++ {
		if p.Next() != "http://p1" {
			t.Fatal("sticky should return first URL")
		}
	}
}

func TestPoolFromEnvList(t *testing.T) {
	t.Setenv("PROXY_URLS", "http://a:1,http://b:2")
	p := PoolFromEnv("round-robin", 0)
	if len(p.URLs) != 2 {
		t.Fatalf("urls %+v", p.URLs)
	}
	p2 := PoolFromEnv("round-robin", 1)
	if len(p2.URLs) != 1 {
		t.Fatalf("pick-1 urls %+v", p2.URLs)
	}
}
