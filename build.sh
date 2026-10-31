#!/usr/bin/env bash
# Compila o Vorticis Monitor e gera o instalador Windows (requer Go 1.24+ e NSIS 3).
set -euo pipefail
cd "$(dirname "$0")"
VERSAO="${VERSAO:-$(grep -oP 'Versao = "\K[^"]+' internal/api/server.go)}"
mkdir -p dist
echo "== testes"; go test ./...
echo "== windows/amd64"; GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/vorticis-monitor.exe ./cmd/vorticis-monitor
echo "== linux/amd64 (modo servidor web)"; GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/vorticis-monitor-linux-amd64 ./cmd/vorticis-monitor
echo "== instalador"; (cd installer && makensis -V2 -DVERSAO="$VERSAO" vorticis-monitor.nsi)
ls -lh dist
