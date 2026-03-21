#!/bin/bash
set -euo pipefail

VERSION=$(git describe --tags --always 2>/dev/null || echo dev)

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w -X main.Version=${VERSION}" \
  -o concave-tui ./cmd/concave-tui/

echo "Built: concave-tui ($(du -sh concave-tui | cut -f1))"
