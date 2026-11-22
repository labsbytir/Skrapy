package metrics

import (
	"fmt"
	"os"
	"strings"
	"time"
)

var bars = []rune(" ▁▂▃▄▅▆▇█")

func sparkline(vals []float64) string {
	if len(vals) == 0 {
		return "(warming up)"
	}
	max := vals[0]
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}
	var b strings.Builder
	for _, v := range vals {
		b.WriteRune(bars[int(v/max*8)])
	}
	return b.String()
}

// Dashboard renders to stderr on a ticker. No-ops unless TTY and enabled.
type Dashboard struct {
	m      *Metrics
	plain  bool // --plain or not a TTY: periodic log lines instead
	quit   chan struct{}
	jsonOut bool // --json: machine-readable snapshots to stdout
}

func isTTY() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func noColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return true
	}
	for _, a := range os.Args {
		if a == "--no-color" {
			return true
		}
	}
	return false
}

// Start begins rendering every 2s. Caller must Stop().
func Start(m *Metrics, plain, jsonOut bool) *Dashboard {
	d := &Dashboard{m: m, plain: plain || !isTTY(), jsonOut: jsonOut, quit: make(chan struct{})}
	go d.loop()
	return d
}

func (d *Dashboard) Stop() { close(d.quit) }

func (d *Dashboard) loop() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-d.quit:
			return
		case <-t.C:
			d.render()
		}
	}
}

func (d *Dashboard) render() {
	s := d.m.Snap()
	if d.jsonOut {
		d.m.EmitJSON()
		return
	}
	if d.plain {
		fmt.Fprintf(os.Stderr, "ok=%d fail=%d %.1f pages/s infer_calls=%d infer_tokens=%d proxy_req=%d\n",
			s.PagesOK, s.PagesFail, s.PagesSec, s.InferCalls, s.InferTokens, s.ProxyRequests)
		return
	}
	c := ""
	if !noColor() {
		c = "\033[2K\r"
	}
	fmt.Fprintf(os.Stderr, "%s[collect] ok=%d fail=%d %.1f/s | %s\n[infer] calls=%d tokens=%d | [proxy] req=%d\n",
		c, s.PagesOK, s.PagesFail, s.PagesSec, sparkline(s.Spark),
		s.InferCalls, s.InferTokens, s.ProxyRequests)
	for _, e := range s.Recent {
		fmt.Fprintf(os.Stderr, "  ! %s\n", e)
	}
}
