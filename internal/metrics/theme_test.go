package metrics

import (
	"strings"
	"testing"
)

func TestTirThemeCodes(t *testing.T) {
	if got := fg(Blue); !strings.Contains(got, "38;2;37;110;255") {
		t.Fatalf("blue fg %q", got)
	}
	if got := fg(Emerald); !strings.Contains(got, "38;2;61;220;151") {
		t.Fatalf("emerald fg %q", got)
	}
	if got := fg(Fuchsia); !strings.Contains(got, "38;2;255;73;92") {
		t.Fatalf("fuchsia fg %q", got)
	}
	if got := bg(Indigo); !strings.Contains(got, "48;2;70;35;122") {
		t.Fatalf("indigo bg %q", got)
	}
}

func TestBarWidth(t *testing.T) {
	b := Bar("Mon", 25, 40)
	// label + pct + space + 10 blocks (25% of 40)
	if !strings.Contains(b, "Mon") || !strings.Contains(b, "25%") {
		t.Fatalf("bar %q", b)
	}
	if strings.Count(b, "█") != 10 {
		t.Fatalf("blocks %d, want 10 in %q", strings.Count(b, "█"), b)
	}
	if strings.Count(Bar("Full", 100, 40), "█") != 40 {
		t.Fatal("full bar should be 40 blocks")
	}
}

func TestFrameStorageS3(t *testing.T) {
	m := New()
	m.SetStorage(256*1024*1024, 1024*1024*1024)
	m.SetS3(45, "s5cmd")
	m.AddRow(Row{URL: "https://a.example/x", Status: "ok", MS: 120, Title: "Hi"})
	tui := &TUI{m: m, w: 100, h: 30}
	f := tui.frame()
	for _, want := range []string{"Storage", "25%", "S3", "45%", "s5cmd", "https://a.example/x"} {
		if !strings.Contains(f, want) {
			t.Fatalf("frame missing %q\n%s", want, f)
		}
	}
}
