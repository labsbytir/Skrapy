#!/bin/sh
# scripts/build-image.sh - build the Wolfi image locally (needs podman + melange + apko).
# CI (GitHub-hosted runners) cannot run melange builds; use this script on any
# Linux/macOS machine with podman, or the self-hosted build-image.yml workflow.
set -e
cd "$(dirname "$0")/.."

ARCHS="${ARCHS:-x86_64,aarch64}"
TAG="${TAG:-skrapy:latest}"
OUT="${OUT:-skrapy.tar}"
KEY="${KEY:-melange.rsa}"

[ -f "$KEY" ] || melange keygen
melange build melange.yaml --arch "$ARCHS" --signing-key "$KEY"
apko build apko.yaml "$TAG" "$OUT" \
  --repository-append packages --keyring-append melange.rsa.pub --arch "$ARCHS"
echo "Built $OUT ($TAG)"

# Push to GHCR (needs `podman login ghcr.io` with packages:write).
if [ "${PUSH:-0}" = "1" ]; then
  podman load -i "$OUT"
  podman push "$TAG" "ghcr.io/labsbytir/skrapy:${TAG##*:}"
  echo "Pushed ghcr.io/labsbytir/skrapy:${TAG##*:}"
fi