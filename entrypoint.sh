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
export BATCH_MB="${BATCH_MB:-50}"
export LOOP="${LOOP:-0}"
export COLLECT_INTERVAL="${COLLECT_INTERVAL:-${SCRAPE_INTERVAL:-1h}}"
# (SCRAPE_INTERVAL kept as a legacy fallback; set COLLECT_INTERVAL going forward.)

# Optional first-boot embed pack installer.
if [ "${SKRAPY_FIRST_BOOT:-${GO_NEWSK_FIRST_BOOT:-0}}" = "1" ] && [ -x /usr/bin/skrapy-first-boot ]; then
  /usr/bin/skrapy-first-boot
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
exit $RC