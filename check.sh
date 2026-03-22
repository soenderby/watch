#!/usr/bin/env bash
set -euo pipefail

echo "=== vet ==="
go vet ./...

echo "=== test ==="
go test ./...

echo "=== build ==="
go build -o watch ./cmd/watch/

echo ""
echo "All checks passed"
