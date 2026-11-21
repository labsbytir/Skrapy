package proxy

import (
	"os"
	"strings"
	"sync/atomic"
)

// Pool rotates egress across proxy URLs (round-robin) or pins one (sticky).
type Pool struct {
	URLs []string
	Mode string // "round-robin" (default) or "sticky"
	n    uint64
}

// Next returns the proxy URL for the next fetch.
func (p *Pool) Next() string {
	if len(p.URLs) == 0 {
		return ""
	}
	if p.Mode == "sticky" {
		return p.URLs[0]
	}
	i := atomic.AddUint64(&p.n, 1) - 1
	return p.URLs[i%uint64(len(p.URLs))]
}

// PoolFromEnv builds a Pool from PROXY_URLS (comma list of full URLs),
// falling back to the single PROXY_HOST/PORT/USERNAME/PASSWORD set.
// pick caps the pool to the first N entries (0 = all).
func PoolFromEnv(mode string, pick int) *Pool {
	var urls []string
	if list := os.Getenv("PROXY_URLS"); list != "" {
		for _, u := range strings.Split(list, ",") {
			if u = strings.TrimSpace(u); u != "" {
				urls = append(urls, u)
			}
		}
	} else if u := FromEnv().URL(); u != "" {
		urls = []string{u}
	}
	if pick > 0 && len(urls) > pick {
		urls = urls[:pick]
	}
	if mode != "sticky" {
		mode = "round-robin"
	}
	return &Pool{URLs: urls, Mode: mode}
}
