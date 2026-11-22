# GoNewsk

![banner](./assets/banner.png)

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev)
[![Lightpanda](https://img.shields.io/badge/Lightpanda-1.0.4-orange)](https://github.com/lightpanda-io/browser)
[![Wolfi](https://img.shields.io/badge/Wolfi-Chainguard-blue)](https://www.chainguard.dev)
[![melange](https://img.shields.io/badge/melange-apko-green)](https://github.com/chainguard-dev/melange)
[![chromedp](https://img.shields.io/badge/chromedp-CDP-purple)](https://github.com/chromedp/chromedp)
[![License](https://img.shields.io/badge/License-MIT-yellow)](LICENSE)

> The world's media coverage in data.

A headless news collector that turns a curated list of news sources into structured JSON.
The seed catalog comes from [NewskList](https://github.com/labsbytir/NewskList) (28k+
MediaCloud sources) and is then extended by hand. Built on
[Lightpanda](https://github.com/lightpanda-io/browser) (Zig, ~10x lighter than Chrome),
packaged on Wolfi Linux with melange + apko, and exposed over REST, JSON-RPC, gRPC
and tRPC.

Experiment By: [github.com/labsbytir](https://github.com/labsbytir)
Built by: tir labs inc.
Creator & contributor: [github.com/pennytir](https://github.com/pennytir)
Built for: [tirPLM](https://github.com/labsbytir) — minifying large processes into small machines.

## Quick start

```sh
# 1. Get the binary (private repo — needs a token)
TOKEN=$(gh auth token)
ID=$(curl -s -H "Authorization: Bearer $TOKEN" https://api.github.com/repos/labsbytir/GoNewsk/releases/latest \
  | python3 -c "import json,sys; print([a['id'] for a in json.load(sys.stdin)['assets'] if a['name']=='gonewsk-linux-amd64'][0])")
curl -L -H "Authorization: Bearer $TOKEN" -H "Accept: application/octet-stream" \
  https://api.github.com/repos/labsbytir/GoNewsk/releases/assets/$ID -o gonewsk
chmod +x gonewsk

# 2. Start Lightpanda (advertise-host matters when collector runs in another container)
lightpanda serve --host 0.0.0.0 --advertise-host 127.0.0.1 --port 9222 &

# 3. Run
SOURCES_CSV=sources.csv LIGHTPANDA_CDP=ws://127.0.0.1:9222/ ./gonewsk
# → results.jsonl
```

Or run the whole thing in a container:

```sh
podman load -i newsk.tar
podman run --rm -v "$PWD:/data" -e SOURCES_CSV=/data/sources.csv -e OUTPUT=/data/results.jsonl newsk:latest
```

## What it does

- Reads a curated CSV of sources (seeded from NewskList, then added to by hand).
- Drives Lightpanda over CDP (chromedp) to fetch each source's homepage.
- Writes one JSON line per source: `{ url, title, text, fetched }`.
- Serves the collected data over four APIs (below).

## Proxy

Two providers supported. Pick one via env:

- **Decodo** (default): set `PROXY_USERNAME`/`PROXY_PASSWORD` (leave as `USERNAME`/`PASSWORD` to disable).
- **Bright Data**: set `BRIGHTDATA_API_KEY`/`BRIGHTDATA_API_TOKEN` for SDK access, or `BRIGHTDATA_CUSTOMER`+`BRIGHTDATA_ZONE`+`BRIGHTDATA_PASSWORD` (or `BRIGHTDATA_CDP_URL`) to route CDP through Bright Data's Browser API.

Proxy is applied to Lightpanda via `--http-proxy`. Examples: `examples/collect.sh`, `examples/webshare.py`.

## Configuration

| Env var | Default | Notes |
|---|---|---|
| `SOURCES_CSV` | `sources.csv` | Input catalog (curated) |
| `OUTPUT` | `results.jsonl` | Output path |
| `LIGHTPANDA_CDP` | `ws://127.0.0.1:9222` | Lightpanda CDP URL |
| `PROXY_HOST` / `PROXY_PORT` | `isp.decodo.com` / `10000` | Proxy host/port |
| `PROXY_USERNAME` / `PROXY_PASSWORD` | `USERNAME` / `PASSWORD` | Leave as placeholders to disable |

## APIs

Run `go run ./cmd/api` (or the `gonewsk` binary with `SERVE=1`) to expose:

- **REST (sparse fieldsets):** `GET /api/items?fields=id,title`
- **JSON-RPC 2.0:** `POST /rpc` with `{"jsonrpc":"2.0","method":"list","id":1}`
- **gRPC / ConnectRPC:** see `proto/gonewsk.proto` (compiled Go or Rust binary, <15MB)
- **tRPC:** see `api/trpc.example.ts` (requires a JS runtime in the image)

## Flavors (two — same Newsk core)

- **Lite**: headless collector + MiniLM embeddings + pglite. For Pi/servers.
- **Full**: Openbox desktop + EmDash + Harper + collector + embeddings.
- Flavors live in the core repo: [labsbytir/Newsk](https://github.com/labsbytir/Newsk) (`os/`).
  LXQt is not available in Wolfi, so Full is Openbox-based until the LXQt stack lands.

## Container manager

See [docs/dockerman.md](docs/dockerman.md) and https://dockerman.app/en/docs.

## Embedding packs (first boot)

Four containers in total — one base plus three optional embedding packs.
On first boot, run `gonewsk-first-boot` (set `GO_NEWSK_FIRST_BOOT=1`) to choose:

| Pack | Model | Vector DB |
|---|---|---|
| `none` (base) | — | — |
| `minilm` | all-MiniLM-L6-v2 | pglite + pgvector |
| `bge` | BAAI/bge-small-en-v1.5 | pglite + pgvector |
| `snowflake` | Snowflake/snowflake-arctic-embed-m-v2.0 (quantized ONNX, ~300MB) | pglite + pgvector |

```sh
GO_NEWSK_FIRST_BOOT=1 podman run --rm newsk:latest
# pick 1-4 at the prompt
```

Each pack installs `nodejs-24`, `npm`, `@electric-sql/pglite` + `@electric-sql/pglite/vector`
and downloads the model weights. pgvector is enabled via `CREATE EXTENSION IF NOT EXISTS vector;`.
(pglite, the WASM Postgres, can be added via npm if preferred.)

```sh
# Install a pack manually after boot:
apk add --no-cache gonewsk-embed-minilm
gonewsk-install-embed-minilm
```

## Platform (Cloudflare Workers)

- `workers/upload/` — API-key-gated upload endpoint. Validates keys against a KV store, accepts batched JSONL chunks (<90MB each due to Worker limits), and writes them to an R2 bucket. S3/R2 credentials live in Cloudflare secrets, never in the container.
- `workers/verify/` — Verification/quarantine worker. Consumes uploads, validates JSONL, scans for PII, routes clean records to `verified/` and suspicious ones to `quarantined/`.
- `embedding/` — Python container that reads `verified/` JSONL, embeds title+text with a small sentence-transformers model, and writes embedding JSONL.

See each worker's `wrangler.toml` for bindings (KV namespace, R2 bucket, Queue).

## Batch & continuous mode

The container reconciles (dedup, normalize) before uploading, splits the output into chunks, and posts each chunk to the upload Worker with an `x-api-key`. Run continuously with `LOOP=1 COLLECT_INTERVAL=1h` (`SCRAPE_INTERVAL` still honored as a legacy fallback).

```sh
LOOP=1 COLLECT_INTERVAL=30m \
  UPLOAD_URL=https://gonewsk-upload.example.com \
  API_KEY=gk_... \
  ./gonewsk
```

## Collect on a remote machine (base image, no embedding)

The base image excludes all embedding packs. On any Linux host with podman:

```sh
git clone https://github.com/labsbytir/GoNewsk.git && cd GoNewsk
# build tools: podman, melange, apko (see scripts/build-image.sh)
./scripts/build-image.sh            # builds Newsk base image (no embedding models)
podman load -i gonewsk.tar
podman run -d --name gonewsk \
  -v "$PWD:/data" \
  -e SOURCES_CSV=/data/sources.csv \
  -e OUTPUT=/data/results.jsonl \
  -e RECONCILED=/data/reconciled.jsonl \
  gonewsk:latest
```

Or pull the prebuilt tar from the GitHub release and `podman load` it — no build needed.

## Registry (GHCR)

The image is not on GHCR yet. To register it from any host with the image loaded:

```sh
podman login ghcr.io   # token needs packages:write
PUSH=1 TAG=ghcr.io/labsbytir/newsk:v0.1.1 ./scripts/build-image.sh
# then elsewhere: podman pull ghcr.io/labsbytir/newsk:v0.1.1
```

## Build from source

```sh
# Go binary
go build -o gonewsk .

# Wolfi image
melange build melange.yaml --arch x86_64,aarch64 --signing-key melange.rsa
apko build apko.yaml gonewsk:latest gonewsk.tar \
  --repository-append packages --keyring-append melange.rsa.pub --arch x86_64,aarch64
podman load -i gonewsk.tar
```

Lightpanda is built from source in CI (`.github/workflows/build-lightpanda.yml`); the
melange pipeline pins a released binary (bumped by `update-packages.yml`).

## License

MIT. See [ATTRIBUTION.md](ATTRIBUTION.md) for upstream credits (Lightpanda, Wolfi).