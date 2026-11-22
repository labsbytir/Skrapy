#!/bin/sh
# Example: run the collector against one source through the proxy.
export PROXY_HOST="${PROXY_HOST:-isp.decodo.com}"
export PROXY_PORT="${PROXY_PORT:-10000}"
export PROXY_USERNAME="${PROXY_USERNAME:-USERNAME}"
export PROXY_PASSWORD="${PROXY_PASSWORD:-PASSWORD}"
export SOURCES_CSV="${SOURCES_CSV:-sources.csv}"
export OUTPUT="${OUTPUT:-results.jsonl}"
./skrapy