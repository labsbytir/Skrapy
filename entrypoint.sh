#!/bin/sh
set -e

print_boot() {
  cat <<'LOGO'
 _   _                    _
| \ | | _____      _____| | __
|  \| |/ _ \ \ /\ / / __| |/ /
| |\  |  __/\ V  V /\__ \   <
|_| \_|\___| \_/\_/ |___/_|\_\

 (c) 2026 tir labs inc. - MIT - github.com/labsbytir/Skrapy
LOGO
}
print_boot

# Resolve proxy URL (never embed secrets in the image).
# PROXY_URLS (comma list) + PROXY_MODE round-robin|sticky + PROXY_PICK N
# select the pool; Lightpanda gets one URL per run (rotation across runs).
PX=""
if [ -n "$PROXY_URLS" ]; then
  LIST="$PROXY_URLS"
  if [ "${PROXY_PICK:-0}" -gt 0 ] 2>/dev/null; then
    LIST=$(printf '%s' "$PROXY_URLS" | cut -d, -f1-"${PROXY_PICK}")
  fi
  if [ "${PROXY_MODE:-round-robin}" = "sticky" ]; then
    PX=$(printf '%s' "$LIST" | cut -d, -f1)
  else
    N=$(printf '%s' "$LIST" | tr ',' '\n' | grep -c .)
    IDX=$(( ( $(date +%s) % N ) + 1 ))
    PX=$(printf '%s' "$LIST" | cut -d, -f"$IDX")
  fi
elif [ -n "$WEBSHARE_TOKEN" ] && [ -x /usr/bin/resolve-proxy ]; then
  PX=$(/usr/bin/resolve-proxy 2>/dev/null || true)
elif [ -n "$PROXY_USERNAME" ] && [ "$PROXY_USERNAME" != "USERNAME" ] && \
     [ -n "$PROXY_PASSWORD" ] && [ "$PROXY_PASSWORD" != "PASSWORD" ]; then
  PX="http://${PROXY_USERNAME}:${PROXY_PASSWORD}@${PROXY_HOST:-isp.decodo.com}:${PROXY_PORT:-10000}"
fi
if [ -n "$PX" ]; then export HTTP_PROXY="$PX" HTTPS_PROXY="$PX"; fi

# Bright Data CDP (secret-injected at runtime, not built in).
if [ -n "$BRIGHTDATA_CDP_URL" ]; then
  LIGHTPANDA_CDP="$BRIGHTDATA_CDP_URL"
elif [ -n "$BRIGHTDATA_CUSTOMER" ] && [ -n "$BRIGHTDATA_ZONE" ] && [ -n "$BRIGHTDATA_PASSWORD" ]; then
  LIGHTPANDA_CDP="wss://brd-customer-${BRIGHTDATA_CUSTOMER}-zone-${BRIGHTDATA_ZONE}:${BRIGHTDATA_PASSWORD}@brd.superproxy.io:9222"
else
  LIGHTPANDA_CDP="${LIGHTPANDA_CDP:-ws://127.0.0.1:9222/}"
fi
export LIGHTPANDA_CDP
export SOURCES_CSV="${SOURCES_CSV:-sources.csv}"
export OUTPUT="${OUTPUT:-results.jsonl}"
export RECONCILED="${RECONCILED:-reconciled.jsonl}"
export UPLOAD_URL="${UPLOAD_URL:-}"
export API_KEY="${API_KEY:-}"
export API_TOKEN="${API_TOKEN:-}"
export BATCH_MB="${BATCH_MB:-50}"
export RUN_CAP_MB="${RUN_CAP_MB:-1024}"
export S3_BUCKET="${S3_BUCKET:-}"
export S3_PREFIX="${S3_PREFIX:-}"
export S3_ENDPOINT="${S3_ENDPOINT:-}"
export BLOCKLIST="${BLOCKLIST:-/opt/skrapy/blocklist.txt}"
export DEPTH="${DEPTH:-2}"
export OBEY_ROBOTS="${OBEY_ROBOTS:-1}"
export DEDUPE="${DEDUPE:-1}"
export CONCURRENCY="${CONCURRENCY:-4}"
export PROXY_MODE="${PROXY_MODE:-round-robin}"
export PROXY_PICK="${PROXY_PICK:-0}"
export LOOP="${LOOP:-0}"
export COLLECT_INTERVAL="${COLLECT_INTERVAL:-${SCRAPE_INTERVAL:-1h}}"
# (SCRAPE_INTERVAL kept as a legacy fallback; set COLLECT_INTERVAL going forward.)

# Preflight: S3 uploader + history plotters (best-effort pip, bars stay default).
if ! command -v s5cmd >/dev/null 2>&1 && ! command -v rclone >/dev/null 2>&1; then
  echo "skrapy: WARNING: neither s5cmd nor rclone on PATH; S3 upload will fail" >&2
fi
if ! python3 -c "import plotext" 2>/dev/null; then
  pip install --quiet --target /opt/skrapy/pytools termgraph plotext youplot 2>/dev/null || \
    echo "skrapy: WARNING: pip plotters unavailable (offline?); use zero-dep bars" >&2
fi
case ":$PYTHONPATH:" in
  *:/opt/skrapy/pytools:*) ;;
  *) export PYTHONPATH="/opt/skrapy/pytools${PYTHONPATH:+:$PYTHONPATH}" ;;
esac

# Optional first-boot embed pack installer.
if [ "${SKRAPY_FIRST_BOOT:-${GO_NEWSK_FIRST_BOOT:-0}}" = "1" ] && [ -x /usr/bin/skrapy-first-boot ]; then
  /usr/bin/skrapy-first-boot
fi

# Start token API alongside Lightpanda (single active job model).
if [ -n "$API_TOKEN" ] && [ -x /usr/bin/skrapy-api ]; then
  /usr/bin/skrapy-api &
  API_PID=$!
fi

# TLS via Caddy (public access): DOMAIN enables Automatic HTTPS (needs
# :80/:443, i.e. --cap-add=NET_BIND_SERVICE or root); TLS_CERT/TLS_KEY
# serve bring-your-own cert on :8443 (nonroot-safe, map host 443->8443).
# Neither set: plain HTTP on :8080 (dev).
export DOMAIN="${DOMAIN:-}"
export ACME_EMAIL="${ACME_EMAIL:-}"
export TLS_CERT="${TLS_CERT:-}"
export TLS_KEY="${TLS_KEY:-}"
if { [ -n "$DOMAIN" ] || { [ -n "$TLS_CERT" ] && [ -n "$TLS_KEY" ]; }; } && command -v caddy >/dev/null 2>&1; then
  {
    if [ -n "$TLS_CERT" ]; then
      printf ':8443 {\n\ttls %s %s\n\treverse_proxy 127.0.0.1:8080\n}\n' "$TLS_CERT" "$TLS_KEY"
    else
      printf '%s {\n\treverse_proxy 127.0.0.1:8080\n}\n' "$DOMAIN"
    fi
    [ -n "$ACME_EMAIL" ] && printf '{\n\temail %s\n}\n' "$ACME_EMAIL"
  } > /tmp/Caddyfile
  caddy run --config /tmp/Caddyfile --adapter caddyfile &
  CADDY_PID=$!
fi

# Start Lightpanda with the proxy (if configured).
ARGS="--host 0.0.0.0 --advertise-host "${ADVERTISE_HOST:-127.0.0.1}" --port 9222"
if [ -n "$PX" ]; then
  ARGS="$ARGS --http-proxy $PX"
fi
/usr/bin/lightpanda serve $ARGS &
LP_PID=$!

# Wait for CDP.
for i in $(seq 1 30); do
  if wget -qO- "http://127.0.0.1:9222/json/version" >/dev/null 2>&1; then break; fi
  sleep 1
done

# Run collector (+ reconciliation + upload if configured).
/usr/bin/skrapy
RC=$?
kill $LP_PID 2>/dev/null || true
if [ -n "${API_PID:-}" ]; then kill $API_PID 2>/dev/null || true; fi
if [ -n "${CADDY_PID:-}" ]; then kill $CADDY_PID 2>/dev/null || true; fi
exit $RC