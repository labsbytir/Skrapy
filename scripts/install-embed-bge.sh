#!/bin/sh
# skrapy-install-embed-bge - wire up BAAI/bge-small-en-v1.5 + pglite/pgvector
set -e

# --- low-memory guard ---
# Low-memory guard: pglite init needs ~2G. Override with FORCE_INSTALL=1.
if [ -z "$FORCE_INSTALL" ] && [ -r /proc/meminfo ]; then
  MEM_KB=$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)
  if [ -n "$MEM_KB" ] && [ "$MEM_KB" -lt 1800000 ]; then
    echo "Low memory ($((MEM_KB / 1024)) MB available, need ~2000 MB for pglite init." >&2
    echo "Set FORCE_INSTALL=1 to install anyway (server may OOM)." >&2
    exit 1
  fi
fi
MODEL_DIR="/opt/skrapy/embedding/bge-small-en-v1.5"

echo "Model weights: $MODEL_DIR"
ls -la "$MODEL_DIR" 2>/dev/null || echo "Model weights missing - will download on first run."

if [ ! -f "$MODEL_DIR/package.json" ]; then
  (cd "$MODEL_DIR" && npm init -y >/dev/null 2>&1)
fi
if [ ! -d "$MODEL_DIR/node_modules/@electric-sql/pglite" ]; then
  echo "Installing pglite + pgvector extension..."
  (cd "$MODEL_DIR" && npm install --no-audit --no-fund @electric-sql/pglite @electric-sql/pglite-pgvector)
fi

echo "Enable pgvector in pglite via:"
echo "  import { PGlite } from '@electric-sql/pglite';"
echo "  import { vector } from '@electric-sql/pglite-pgvector';"
echo "  const db = new PGlite({ extensions: { vector } });"
echo "  await db.exec('CREATE EXTENSION vector;');"
echo "BAAI/bge-small-en-v1.5 embedding pack ready (dims: 384)."