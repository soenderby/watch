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

echo "=== checks ==="
GO_BIN="$GO_BIN" "$SCRIPT_DIR/check.sh"

echo "=== install ==="
cd "$SCRIPT_DIR"
"$GO_BIN" install ./cmd/watch/

GOBIN_PATH="$($GO_BIN env GOBIN)"
if [ -z "$GOBIN_PATH" ]; then
  GOBIN_PATH="$($GO_BIN env GOPATH)/bin"
fi
WATCH_BIN="$GOBIN_PATH/watch"

if [ ! -x "$WATCH_BIN" ]; then
  echo "installed binary not found at $WATCH_BIN" >&2
  exit 1
fi

echo "Installed: $WATCH_BIN"

echo "=== smoke ==="
if ! "$WATCH_BIN" help | grep -q "identity"; then
  echo "smoke check failed: identity command missing from help output" >&2
  exit 1
fi
"$WATCH_BIN" version

echo ""
echo "Sync complete."
echo "If a shell still resolves an old cached command, run: hash -r"
