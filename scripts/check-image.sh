#!/bin/sh
# Failing-first check for Skrapy image configs (Task 2).
# Asserts runtime packages, Lightpanda pin, and OCI annotations.
set -e
fail=0
need_apko="wolfi-base ca-certificates-bundle python-3.13 py3.13-pip nodejs-24 aws-cli-v2 s5cmd rclone btop htop caddy skrapy"
for p in $need_apko; do
  grep -q "^\s*-\s*$p\s*$" apko.yaml || { echo "MISSING apko package: $p"; fail=1; }
done
grep -q 'lightpanda-.*-linux' melange.yaml || { echo "MISSING lightpanda pin in melange.yaml"; fail=1; }
grep -q 'releases/download/1.0.4' melange.yaml || { echo "MISSING lightpanda 1.0.4 pin"; fail=1; }
grep -q 'org.opencontainers.image.title: "Skrapy"' apko.yaml || { echo "MISSING title Skrapy"; fail=1; }
grep -q 'vendor: "tir labs inc."' apko.yaml || { echo "MISSING vendor tir labs inc."; fail=1; }
grep -q 'licenses: MIT' apko.yaml || { echo "MISSING licenses MIT"; fail=1; }
grep -q 'url: "https://github.com/labsbytir/Skrapy"' apko.yaml || { echo "MISSING url Skrapy"; fail=1; }
if [ "$fail" -ne 0 ]; then echo "CHECK-FAIL"; exit 1; fi
echo "CHECK-PASS"
