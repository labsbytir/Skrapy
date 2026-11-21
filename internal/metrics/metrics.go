// Package metrics tracks collect + inference usage and feeds both the
// line dashboard (clig.dev-friendly) and the full-screen Dashboard
// archetype (monospace-design-tui §11.1).
package metrics

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Counters struct {
	PagesOK   int64
	PagesFail int64
	Bytes     int64
	Started   time.Time
	// Inference / token usage (embedding packs, LLM calls).
	InferCalls  int64
	InferTokens int64
	// Proxy usage.
	ProxyRequests int64
}

// Row is one dashboard table entry.
type Row struct {
	URL    string
	Status string // ok | slow | fail
	MS     int64
	Title  string
}

type Metrics struct {
	mu     sync.Mutex
	c      Counters
	rates  []float64 // pages/sec samples for sparkline
	errors []string  // recent error tail
	rows   []Row     // table ring, cap 200
}

func New() *Metrics { return &Metrics{c: Counters{Started: time.Now()}} }

func (m *Metrics) AddRow(r Row) {
	m.mu.Lock()
	m.rows = append(m.rows, r)
	if len(m.rows) > 200 {
		m.rows = m.rows[len(m.rows)-200:]
	}
	m.mu.Unlock()
}

func (m *Metrics) AddOK(nbytes int64) {
	m.mu.Lock()
	m.c.PagesOK++
	m.c.Bytes += nbytes
	m.mu.Unlock()
}

func (m *Metrics) AddFail(msg string) {
	m.mu.Lock()
	m.c.PagesFail++
	m.errors = append(m.errors, msg)
	if len(m.errors) > 5 {
		m.errors = m.errors[len(m.errors)-5:]
	}
	m.mu.Unlock()
}

func (m *Metrics) AddInfer(tokens int64) {
	m.mu.Lock()
	m.c.InferCalls++
	m.c.InferTokens += tokens
	m.mu.Unlock()
}

func (m *Metrics) AddProxy() {
	m.mu.Lock()
	m.c.ProxyRequests++
	m.mu.Unlock()
}

// Sample records a pages/sec point for the sparkline.
func (m *Metrics) Sample(pps float64) {
	m.mu.Lock()
	m.rates = append(m.rates, pps)
	if len(m.rates) > 40 {
		m.rates = m.rates[len(m.rates)-40:]
	}
	m.mu.Unlock()
}

type Snapshot struct {
	Counters
	Elapsed  float64  `json:"elapsed_s"`
	PagesSec float64  `json:"pages_sec"`
	Recent   []string `json:"recent_errors"`
	Rows     []Row    `json:"rows,omitempty"`
	Spark    []float64 `json:"sparkline,omitempty"`
}

func (m *Metrics) Snap() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	el := time.Since(m.c.Started).Seconds()
	var pps float64
	if el > 0 {
		pps = float64(m.c.PagesOK+m.c.PagesFail) / el
	}
	return Snapshot{
		Counters: m.c, Elapsed: el, PagesSec: pps,
		Recent: append([]string(nil), m.errors...),
		Rows:   append([]Row(nil), m.rows...),
		Spark:  append([]float64(nil), m.rates...),
	}
}

// EmitJSON writes one machine-readable snapshot to stdout.
func (m *Metrics) EmitJSON() {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(m.Snap())
}
