#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="$ROOT/python/src/testscan_py/bin"
mkdir -p "$OUT_DIR"
if [[ "$(uname -s)" == MINGW* || "$(uname -s)" == MSYS* || "$(uname -s)" == CYGWIN* || "$(uname -s)" == Windows_NT ]]; then
  OUT="$OUT_DIR/testscan.exe"
else
  OUT="$OUT_DIR/testscan"
fi
(cd "$ROOT" && go build -o "$OUT" ./cmd/testscan)
echo "embedded: $OUT"
