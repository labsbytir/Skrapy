package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labsbytir/Skrapy/internal/collector"
	"github.com/labsbytir/Skrapy/internal/metrics"
	"github.com/labsbytir/Skrapy/internal/proxy"
	"github.com/labsbytir/Skrapy/internal/reconcile"
	"github.com/labsbytir/Skrapy/internal/upload"
)

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
		if err := collectFile(ctx, cfg.cdpURL, cfg.sourcesCSV, cfg.output, m); err != nil {
			return err
		}

		kept, total, err := reconcile.Run(cfg.output, cfg.reconciled)
		if err != nil {
			return err
		}
		log.Printf("reconciled %d/%d -> %s", kept, total, cfg.reconciled)

		if cfg.uploadURL != "" && cfg.apiKey != "" {
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

func collectFile(parent context.Context, cdpURL, sourcesCSV, output string, m *metrics.Metrics) error {
	f, err := os.Open(sourcesCSV)
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return err
	}

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()
	enc := json.NewEncoder(out)

	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()

	for i, row := range rows {
		if i == 0 || len(row) < 2 || row[1] == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		start := time.Now()
		res, err := collector.Run(ctx, cdpURL, row[1])
		ms := time.Since(start).Milliseconds()
		if err != nil {
			m.AddFail(row[1] + ": " + err.Error())
			m.AddRow(metrics.Row{URL: row[1], Status: "fail", MS: ms})
			log.Printf("err %s: %v", row[1], err)
			continue
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
		status := "ok"
		if ms > 10000 {
			status = "slow"
		}
		m.AddRow(metrics.Row{URL: row[1], Status: status, MS: ms, Title: res.Title})
		m.AddOK(int64(len(res.Text)))
		m.Sample(1)
		log.Printf("[%d/%d] %s", i, len(rows), res.Title)
	}
	return nil
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