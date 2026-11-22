package proxy

import (
	"testing"
)

func TestURLPlaceholder(t *testing.T) {
	c := Config{Host: "isp.decodo.com", Port: "10000", Username: "USERNAME", Password: "PASSWORD"}
	if got := c.URL(); got != "" {
		t.Fatalf("expected empty URL for placeholder creds, got %q", got)
	}
}

func TestURLReal(t *testing.T) {
	c := Config{Host: "isp.decodo.com", Port: "10000", Username: "u", Password: "p"}
	want := "http://u:p@isp.decodo.com:10000"
	if got := c.URL(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}