package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/labsbytir/Skrapy/internal/collector"
	"github.com/labsbytir/Skrapy/internal/metrics"
	"github.com/labsbytir/Skrapy/internal/proxy"
	"github.com/labsbytir/Skrapy/internal/queue"
	"github.com/labsbytir/Skrapy/internal/reconcile"
	"github.com/labsbytir/Skrapy/internal/s3store"
	"github.com/labsbytir/Skrapy/internal/upload"
)

// errRunCapExceeded stops the run when RUN_CAP_MB is hit; resume on next run.
var errRunCapExceeded = errors.New("skrapy: RUN_CAP_MB exceeded")

type config struct {
	cdpURL        string
	sourcesCSV    string
	output        string
	reconciled    string
	uploadURL     string
	apiKey        string
	batchMB       int64
	loop          bool
	interval      time.Duration
	brightData    bool
	plain       bool
	jsonMetrics bool
	depth       int
	obeyRobots  bool
	dedupe      bool
	concurrency int
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.cdpURL, "cdp-url", envOr("LIGHTPANDA_CDP", "ws://127.0.0.1:9222/"), "Lightpanda CDP URL")
	flag.StringVar(&cfg.sourcesCSV, "sources", envOr("SOURCES_CSV", "sources.csv"), "Input sources CSV")
	flag.StringVar(&cfg.output, "output", envOr("OUTPUT", "results.jsonl"), "Raw output JSONL")
	flag.StringVar(&cfg.reconciled, "reconciled", envOr("RECONCILED", "reconciled.jsonl"), "Reconciled output JSONL")
	flag.StringVar(&cfg.uploadURL, "upload-url", envOr("UPLOAD_URL", ""), "Cloudflare upload Worker URL (enables upload)")
	flag.StringVar(&cfg.apiKey, "api-key", envOr("API_KEY", ""), "API key for upload Worker")
	flag.Int64Var(&cfg.batchMB, "batch-mb", 50, "Max MB per upload chunk")
	flag.BoolVar(&cfg.loop, "loop", envOr("LOOP", "") == "1", "Continuous collect loop")
	flag.DurationVar(&cfg.interval, "interval", envOrDurationBoth("COLLECT_INTERVAL", "SCRAPE_INTERVAL", 1*time.Hour), "Loop interval")
	flag.BoolVar(&cfg.brightData, "bright-data", envOr("BRIGHT_DATA", "") == "1", "Use Bright Data CDP URL builder")
	flag.BoolVar(&cfg.plain, "plain", false, "Plain log output (no live dashboard)")
	flag.BoolVar(&cfg.jsonMetrics, "json", false, "Machine-readable metric snapshots on stdout")
	flag.IntVar(&cfg.depth, "depth", envOrInt("DEPTH", 2), "Link-follow depth (0 = seeds only)")
	flag.BoolVar(&cfg.obeyRobots, "obey-robots", envOrBool("OBEY_ROBOTS", true), "Obey robots.txt Disallow")
	flag.BoolVar(&cfg.dedupe, "dedupe", envOrBool("DEDUPE", true), "Dedupe URLs per job")
	flag.IntVar(&cfg.concurrency, "concurrency", envOrInt("CONCURRENCY", 4), "Parallel fetch workers")
	flag.Parse()

	if cfg.brightData {
		if bd := proxy.BrightDataCDPURL(); bd != "" {
			cfg.cdpURL = bd
			log.Printf("using Bright Data CDP")
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	runOnce := func() error {
		m := metrics.New()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if !cfg.plain && !cfg.jsonMetrics {
			tui := &metrics.TUI{}
			go func() {
				// §9 dual rendering: full-screen when TTY, line view otherwise.
				if err := tui.Run(m, cancel); err != nil {
					d := metrics.Start(m, true, false)
					defer d.Stop()
					<-ctx.Done()
				}
			}()
			_ = ctx
		} else {
			d := metrics.Start(m, cfg.plain, cfg.jsonMetrics)
			defer d.Stop()
		}
		log.Printf("collecting %s -> %s", cfg.sourcesCSV, cfg.output)
		if err := collectFile(ctx, cfg, cfg.output, m); err != nil {
			if errors.Is(err, errRunCapExceeded) {
				m.SetNotice("RUN_CAP_MB reached — resume frees space after upload")
				log.Printf("cap hit, stopping run (resume-safe)")
				return nil
			}
			return err
		}

		kept, total, err := reconcile.Run(cfg.output, cfg.reconciled)
		if err != nil {
			return err
		}
		log.Printf("reconciled %d/%d -> %s", kept, total, cfg.reconciled)

		if os.Getenv("S3_BUCKET") != "" {
			m.SetStorage(dirBytes(".skrapy-queue", cfg.output, batchDir(cfg.reconciled)), int64(envOrInt("RUN_CAP_MB", 1024))*1024*1024)
			if err := uploadBatches(cfg.reconciled, m); err != nil {
				return err
			}
			os.Remove(cfg.output)
		} else if cfg.uploadURL != "" && cfg.apiKey != "" {
			if err := uploadReconciled(cfg.uploadURL, cfg.apiKey, cfg.reconciled, cfg.batchMB); err != nil {
				return err
			}
		}
		return nil
	}

	if cfg.loop {
		for {
			select {
			case <-sig:
				log.Println("shutting down")
				return
			case <-time.After(cfg.interval):
				if err := runOnce(); err != nil {
					log.Printf("loop error: %v", err)
				}
			}
		}
	} else {
		if err := runOnce(); err != nil {
			log.Fatal(err)
		}
	}
}

func collectFile(parent context.Context, cfg config, output string, m *metrics.Metrics) error {
	f, err := os.Open(cfg.sourcesCSV)
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return err
	}

	var seeds []string
	for i, row := range rows {
		if i == 0 || len(row) < 2 || row[1] == "" {
			continue
		}
		seeds = append(seeds, row[1])
	}

	q, err := queue.Open(".skrapy-queue", cfg.dedupe)
	if err != nil {
		return err
	}
	if err := q.Enqueue(seeds, 0); err != nil {
		return err
	}

	out, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	enc := json.NewEncoder(out)

	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()

	var (
		mu      sync.Mutex
		encMu   sync.Mutex
		netMu   sync.Mutex // guards hosts + lastHit across workers
		hosts   = map[string][]string{} // host -> robots disallows
		lastHit = map[string]time.Time{}
		workers = cfg.concurrency
		pages   int
		capHit  error
	)
	if workers < 1 {
		workers = 1
	}
	runCap := int64(envOrInt("RUN_CAP_MB", 1024)) * 1024 * 1024
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				for m.Paused() {
					select {
					case <-ctx.Done():
						return
					case <-time.After(200 * time.Millisecond):
					}
				}
				mu.Lock()
				it, ok := q.Next()
				mu.Unlock()
				if !ok {
					return
				}
				netMu.Lock()
				blocked := cfg.obeyRobots && robotsBlocked(hosts, it.URL)
				netMu.Unlock()
				if blocked {
					mu.Lock()
					q.Dead(it.URL, "robots")
					mu.Unlock()
					m.AddFail(it.URL + ": robots disallowed")
					m.AddRow(metrics.Row{URL: it.URL, Status: "robots"})
					continue
				}
				netMu.Lock()
				throttle(lastHit, it.URL)
				netMu.Unlock()
				start := time.Now()
				res, ferr := collector.Run(ctx, cfg.cdpURL, it.URL)
				ms := time.Since(start).Milliseconds()
				if ferr != nil {
					mu.Lock()
					q.Dead(it.URL, ferr.Error())
					mu.Unlock()
					m.AddFail(it.URL + ": " + ferr.Error())
					m.AddRow(metrics.Row{URL: it.URL, Status: "fail", MS: ms})
					log.Printf("err %s: %v", it.URL, ferr)
					continue
				}
				encMu.Lock()
				werr := enc.Encode(res)
				encMu.Unlock()
				if werr != nil {
					log.Printf("encode err %s: %v", it.URL, werr)
					continue
				}
				// Follow same-root links while under depth.
				if it.Depth < cfg.depth {
					var next []string
					for _, l := range res.Links {
						if abs := resolveLink(it.URL, l); abs != "" && sameRoot(it.URL, abs) {
							next = append(next, abs)
						}
					}
					mu.Lock()
					_ = q.Enqueue(next, it.Depth+1)
					q.Ack(it.URL)
					mu.Unlock()
				} else {
					mu.Lock()
					q.Ack(it.URL)
					mu.Unlock()
				}
				status := "ok"
				if ms > 10000 {
					status = "slow"
				}
				m.AddRow(metrics.Row{URL: it.URL, Status: status, MS: ms, Title: res.Title})
				m.AddOK(int64(len(res.Text)))
				m.Sample(1)
				mu.Lock()
				pages++
				if pages%50 == 0 {
					m.SetStorage(dirBytes(".skrapy-queue", output, batchDir(output)), runCap)
					if runCap > 0 && dirBytes(".skrapy-queue", output, batchDir(output)) > runCap {
						mu.Unlock()
						log.Printf("RUN_CAP_MB reached, stopping (resume on next run)")
						mu.Lock()
						capHit = errRunCapExceeded
						mu.Unlock()
						cancel()
						return
					}
				}
				mu.Unlock()
				log.Printf("%s [%d]", res.Title, ms)
			}
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	return capHit
}

// batchDir holds per-run batch chunks + zips for an output file.
func batchDir(output string) string {
	return strings.TrimSuffix(output, ".jsonl") + ".batches"
}

// uploadBatches splits output into 1,000-link chunks, zips each, uploads via
// S3 with progress, and deletes local artifacts only after a clean upload.
// Fetch keeps running during other batches: this runs after the crawl pass
// while the queue stays open for the next run.
func uploadBatches(output string, m *metrics.Metrics) error {
	st, err := s3store.FromEnv()
	if err != nil {
		return err
	}
	jobID := "job-" + time.Now().UTC().Format("20060102-150405")
	chunks, err := upload.SplitByCount(output, batchDir(output), 1000)
	if err != nil {
		return err
	}
	for _, c := range chunks {
		zp, err := upload.ZipBatch(batchDir(output), jobID, c.Index)
		if err != nil {
			return err
		}
		m.SetS3(0, "s5cmd")
		zerr := st.UploadZip(context.Background(), zp, func(p float64, done bool) {
			m.SetS3(p, st.LastTool)
		})
		m.SetS3(100, st.LastTool)
		if zerr != nil {
			return zerr
		}
		os.Remove(c.Path)
		os.Remove(zp)
	}
	os.Remove(output)
	return nil
}
func dirBytes(paths ...string) int64 {
	var total int64
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !fi.IsDir() {
			total += fi.Size()
			continue
		}
		_ = filepath.Walk(p, func(_ string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				total += info.Size()
			}
			return nil
		})
	}
	return total
}

func uploadReconciled(endpoint, apiKey, path string, batchMB int64) error {
	u := upload.New(endpoint, apiKey)
	batchID := time.Now().UTC().Format("20060102-150405")
	chunks, err := upload.Split(path, ".skrapy-batches", batchMB*1024*1024)
	if err != nil {
		return err
	}
	log.Printf("uploading %d chunks (~%dMB each)", len(chunks), batchMB)
	for _, c := range chunks {
		if err := u.Upload(batchID, c); err != nil {
			return err
		}
		log.Printf("uploaded chunk %d/%d (%d bytes)", c.Index+1, c.Total, c.Bytes)
	}
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envOrInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envOrBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes":
			return true
		case "0", "false", "no":
			return false
		}
	}
	return def
}

// rootHost returns the last two labels (example.com) for same-domain checks.
func rootHost(h string) string {
	parts := strings.Split(h, ".")
	if len(parts) < 2 {
		return h
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// sameRoot reports whether link shares the seed's root domain (subdomains in).
func sameRoot(seedURL, link string) bool {
	su, err := url.Parse(seedURL)
	if err != nil {
		return false
	}
	lu, err := url.Parse(link)
	if err != nil || lu.Host == "" {
		return false
	}
	return rootHost(su.Hostname()) == rootHost(lu.Hostname())
}

// resolveLink absolutizes href against base; "" when unusable.
func resolveLink(base, href string) string {
	if href == "" || strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
		return ""
	}
	bu, err := url.Parse(base)
	if err != nil {
		return ""
	}
	lu, err := bu.Parse(href)
	if err != nil || (lu.Scheme != "http" && lu.Scheme != "https") {
		return ""
	}
	lu.Fragment = ""
	return lu.String()
}

var robotsHTTP = &http.Client{
	Timeout:   10 * time.Second,
	Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
}

// robotsBlocked fetches and caches host robots.txt Disallow prefixes.
func robotsBlocked(cache map[string][]string, rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	if _, ok := cache[u.Host]; !ok {
		cache[u.Host] = fetchDisallows(u.Scheme + "://" + u.Host + "/robots.txt")
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	for _, d := range cache[u.Host] {
		if d != "" && strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}

// fetchDisallows honors User-agent scoping: only the "*" group (plus
// rules before any User-agent line) applies to Skrapy.
func fetchDisallows(robotsURL string) []string {
	resp, err := robotsHTTP.Get(robotsURL)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	var out []string
	inScope := true // rules before any User-agent line apply globally
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if ua, ok := strings.CutPrefix(strings.ToLower(line), "user-agent:"); ok {
			inScope = strings.TrimSpace(ua) == "*"
			continue
		}
		if inScope && strings.HasPrefix(strings.ToLower(line), "disallow:") {
			out = append(out, strings.TrimSpace(line[len("disallow:"):]))
		}
	}
	return out
}

// throttle enforces ~1 rps per host.
func throttle(last map[string]time.Time, rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return
	}
	if d := time.Since(last[u.Host]); d < time.Second {
		time.Sleep(time.Second - d)
	}
	last[u.Host] = time.Now()
}

func envOrDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// envOrDurationBoth prefers the new key but honors the legacy key,
// so existing LOOP/SCRAPE_INTERVAL deployments keep working.
func envOrDurationBoth(newKey, legacyKey string, def time.Duration) time.Duration {
	if v := os.Getenv(newKey); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return envOrDuration(legacyKey, def)
}