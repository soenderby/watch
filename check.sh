#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_GO="$SCRIPT_DIR/.tools/go/bin/go"

GO_BIN="${GO_BIN:-go}"
if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  if [ -x "$LOCAL_GO" ]; then
    GO_BIN="$LOCAL_GO"
  else
    echo "go not found on PATH and local toolchain missing at $LOCAL_GO" >&2
    exit 1
  fi
fi

echo "Using: $GO_BIN"
"$GO_BIN" version

echo "=== vet ==="
"$GO_BIN" vet ./...

echo "=== test ==="
"$GO_BIN" test ./...

echo "=== build ==="
"$GO_BIN" build -o watch ./cmd/watch/

echo ""
echo "All checks passed"
