#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== gofmt =="
fmt_out="$(gofmt -l .)"
if [ -n "$fmt_out" ]; then
  echo "gofmt: files need formatting:"
  echo "$fmt_out"
  exit 1
fi

echo "== go vet =="
go vet ./...

echo "== test-depth contract + mutation tests + per-package coverage =="
python3 .github/workflows/test_depth_gate.py check

echo "== golangci-lint =="
golangci-lint run ./...

echo "ALL GATES GREEN"
