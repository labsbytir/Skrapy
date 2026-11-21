package metrics

import "testing"

func TestCounters(t *testing.T) {
	m := New()
	m.AddOK(100)
	m.AddOK(200)
	m.AddFail("boom")
	m.AddInfer(50)
	m.AddProxy()
	s := m.Snap()
	if s.PagesOK != 2 || s.PagesFail != 1 || s.Bytes != 300 {
		t.Fatalf("bad counters: %+v", s.Counters)
	}
	if s.InferCalls != 1 || s.InferTokens != 50 || s.ProxyRequests != 1 {
		t.Fatalf("bad usage: %+v", s.Counters)
	}
	if len(s.Recent) != 1 || s.Recent[0] != "boom" {
		t.Fatalf("bad errors: %v", s.Recent)
	}
}

func TestSparkline(t *testing.T) {
	if sparkline(nil) == "" {
		t.Fatal("empty sparkline")
	}
	if got := sparkline([]float64{0, 0}); got == "" {
		t.Fatal("empty zero sparkline")
	}
}
