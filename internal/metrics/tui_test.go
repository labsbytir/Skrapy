package metrics

import (
	"os"
	"strings"
	"testing"
)

func TestTUIRender(t *testing.T) {
	m := New()
	m.AddRow(Row{URL: "https://example.com", Status: "ok", MS: 120, Title: "Example"})
	m.AddRow(Row{URL: "https://slow.test", Status: "slow", MS: 12000, Title: "Slow"})
	m.AddRow(Row{URL: "https://bad.test", Status: "fail", MS: 5})
	m.Sample(1.5)

	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	tui := &TUI{m: m, out: w, w: 100, h: 30}
	tui.render()
	w.Close()
	os.Stdout = old
	buf := make([]byte, 65536)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	for _, want := range []string{"Skrapy", "OK:", "example.com", "Filter", "Quit", "◉", "⚠", "✖"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q", want)
		}
	}
}

func TestTUISmall(t *testing.T) {
	m := New()
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	tui := &TUI{m: m, out: w, w: 40, h: 10}
	tui.render()
	w.Close()
	os.Stdout = old
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	if !strings.Contains(string(buf[:n]), "too small") {
		t.Fatal("expected too-small message")
	}
}