#!/bin/sh
# skrapy-first-boot - offered on first container boot.
# User picks which embedding package to install on top of the base image.
# Base (no embedding) is always available; the three embedding packs add
# a sentence-transformers model + pglite + pgvector.

set -e

echo "GoNewsk first boot - choose an embedding pack:"
echo "  1) none    - base collector only (no embedding)"
echo "  2) minilm  - all-MiniLM-L6-v2 + pglite + pgvector"
echo "  3) bge     - BAAI/bge-small-en-v1.5 + pglite + pgvector"
echo "  4) snowflake - Snowflake/snowflake-arctic-embed-m-v2.0 + pglite + pgvector"
echo ""
read -r -p "Choice [1-4]: " choice

case "$choice" in
  1|none)
    echo "Base collector selected. No embedding pack installed."
    ;;
  2|minilm)
    echo "Installing all-MiniLM-L6-v2 embedding pack..."
    apk add --no-cache skrapy-embed-minilm
    skrapy-install-embed-minilm
    ;;
  3|bge)
    echo "Installing BAAI/bge-small-en-v1.5 embedding pack..."
    apk add --no-cache skrapy-embed-bge
    skrapy-install-embed-bge
    ;;
  4|snowflake)
    echo "Installing Snowflake/snowflake-arctic-embed-m-v2.0 embedding pack..."
    apk add --no-cache skrapy-embed-snowflake
    skrapy-install-embed-snowflake
    ;;
  *)
    echo "Invalid choice. Base collector only."
    ;;
esac

echo "First boot complete."