# Skrapy — Wolfi image + crawl → queue → JSON → S3 — Design

Date: 2026-11-21. Approach picked: A (fork GoNewsk into fresh Skrapy repo). Uploader decision: both `s5cmd` + `rclone`.

## Intent / why

Seed-stage crawler appliance: Lightpanda crawls a seed list, discovers links first, queues them in 1,000-link batches, extracts values + text structure to JSON, blocks ads/trackers, zips each batch, multipart-uploads to S3 with progress, all behind a token-authed trigger endpoint with a live terminal dashboard. Reuses the tested GoNewsk collector/proxy/reconcile instead of rebuilding.

## Goals

- New Wolfi image + apko/melange configs named `skrapy` (fresh repo `labsbytir/Skrapy`, seeded from `Newsk/experiments/GoNewsk`, package renamed `gonewsk` → `skrapy`).
- Runtimes in image: `python3`, `nodejs-24` (+`npm`, TS via `typescript` npm), `go` toolchain libs as needed, `lightpanda` 1.0.4 pinned, `aws-cli`/`s5cmd`/`rclone` for S3.
- All egress via Decodo ISP proxy from env (`PROXY_USERNAME/PASSWORD/HOST/PORT` or `WEBSHARE_TOKEN`); Lightpanda started `serve --http-proxy`; `HTTP_PROXY/HTTPS_PROXY` exported for python/node/go fetches. Secrets never baked or committed.
- Flow: seed crawl → link discovery (same-domain, dedup) → durable FIFO queue → fetch+inspect (title, meta values, body text structure) → JSONL per link → batch closes at 1,000 links → zip batch → S3 multipart upload with % → continue queue.
- Token endpoint: `POST /api/jobs` (`Authorization: Bearer $API_TOKEN`) enqueues seeds; `GET /api/jobs/:id` status. Minimal Go REST in `cmd/api`, same auth pattern as existing `internal/api`.
- TUI (monospace-design-tui dashboard archetype, §1/§5/§8/§10/§11): header (job id, batch n/m), scrolling live link log (URL truncated + HTTP status / error code, color-role status), footer key strip (`q` quit, `p` pause, `j/k` scroll), storage-used bar (run dir bytes + cap), S3 upload % bar per batch. Dual render: full-screen when TTY, plain line log when piped (`--plain`), `--json` metrics preserved.
- Ad/tracker blocking: Lightpanda request interception against bundled EasyList-style blocklist + `chromedp` cookie-banner dismiss; tracking cookies dropped (no persistent jar).

## Non-goals / anti-goals

- Non-goals this pass: distributed workers, Redis/NATS, scheduler/cron UI, R2/MinIO-specific paths (S3-compatible env covers them), embedding/pgvector packs (stay in GoNewsk subpackages).
- Anti-goals: baking proxy creds/API tokens/S3 keys into image; adding apko/melange *into* the runtime image (they stay build-time); JS-heavy TUI (Go Bubble Tea / existing metrics.TUI extended).

## Constraints

- Wolfi (`https://packages.wolfi.dev/os`) + apko build; podman via `/opt/podman/bin/podman`; nonroot `65532`.
- License: MIT, `Copyright (c) 2026 tir labs inc.` (current year). SPDX `MIT` in melange `copyright`, `LICENSE` file at repo root, apko annotations `org.opencontainers.image.licenses: MIT`, `vendor: tir labs inc.`
- TUI theme (tir palette; terminals get 24-bit ANSI `ESC[38;2;R;G;Bm` / `48;2;...`, never raw hex in output):
  - `--digital-blue #256EFF` (37,110,255): header, links, focus border, progress fill.
  - `--indigo-velvet #46237A` (70,35,122): background / inactive panels.
  - `--emerald #3DDC97` (61,220,151): success / 2xx status.
  - `--white #FCFCFC` (252,252,252): primary text.
  - `--hot-fuchsia #FF495C` (255,73,92): errors / 4xx-5xx, never color-only (always paired with status-code text per monospace §5/§7).
- apko available (`~/.local/bin/apko`); melange missing locally — CI or ` следуйте GoNewsk/.github` pipeline builds.
- Batch rule hard: max 1,000 links scraped per batch file, then compile+zip+upload before next batch starts.
- Zip per batch: `<job>-batch-<NNNN>.zip` containing `batch-<NNNN>.jsonl`; upload via multipart (s5cmd primary, rclone fallback), progress callback → TUI.
- Storage accounting: `du`-style walk of run dir on each batch close + periodic tick; display `used / cap` (cap from env `RUN_CAP_MB`, default 1024).

## Approaches considered

- A — Fork-and-extend GoNewsk (PICKED): fastest, keeps tested `collector`, `proxy.Resolve`, `reconcile`, `upload.Split`, `entrypoint.sh` proxy wiring. Tradeoff: inherits GoNewsk quirks.
- B — Two-image builder+runner: smaller runtime (~200MB saved) but second apko config + publish pipeline; deferred to later.
- C — Python-first rewrite (FastAPI+Textual): nicer for Python hacking, but discards working Go pipeline; rejected.

## What the builder gets (decided)

- Seed from `/Newsk/experiments/GoNewsk` → Skrapy repo root: `apko.yaml` (packages: `wolfi-base`, `ca-certificates-bundle`, `python3`, `nodejs-24`, `aws-cli`, `s5cmd`, `rclone`, `skrapy`), `melange.yaml` (package `skrapy`, pipelines `go/build` for `./...` + `resolve-proxy`, fetch lightpanda, copy `entrypoint.sh`, blocklist asset).
- Queue: file FIFO `queue/` + `state.json` (pending/in-flight/done/error counts); crash-resume on restart. No external broker.
- S3: env `S3_ENDPOINT/S3_BUCKET/S3_PREFIX/AWS_*`; uploader tries `s5cmd cp --multipart` with `%` parse, falls back to `rclone copy --progress`; TUI shows active uploader name.
- API token from env `API_TOKEN`, constant-time compare; `POST /api/jobs {seeds:[...]}` → `{jobId}`.

## Decisions (partner-approved 2026-11-21)

- Record schema: maximal for AI — `url, finalUrl, status, title, meta{}, headings[], text, links[], forms[] {action, inputs[] {name, type, value}}, jsonld, error, fetched`. (partner: "all of them for the values the webpage generates, this is for AI")
- Bounds are CLI/env flags, not fixed: `--depth N` (default 2), `--obey-robots=true|false`, `--dedupe=true|false` (per-job scope), `--concurrency N` (default 4, 1 rps/domain throttle). (my call: depth counts link-hops from seeds; dedupe key = normalized URL.)
- Proxy pool: `PROXY_URLS` comma list (falls back to single `PROXY_*` set); `--proxy-mode round-robin|sticky` (default round-robin), `--proxy-pick N` limits how many pool entries are used (subset or all). Concurrent = parallel fetches multiplexed over the pool, one request per proxy at a time per slot. (my call: naming + defaults.)
- Job model: single active job, global FIFO queue. (partner picked.)
- Flow: pipelined — fetch continues while batch zips/uploads; on S3 OK delete local batch dir + zip. Retry 3x backoff, dead-letter to `state.json`; pause on `RUN_CAP_MB`. (partner picked delete-after-upload; pipelining + retry counts my call.)

## Left to builder (plan-time calls)

- Exact REST routes + job-id scheme; TUI key bindings final; blocklist source file + update cadence; S3 key layout; `RUN_CAP_MB` default; whether `s5cmd` exists in used Wolfi mirror (else `go install` in melange pipeline).
