#!/usr/bin/env bash
# Compila o Vorticis Monitor e gera os pacotes Windows (requer Go 1.24+, NSIS 3 e zip).
#   dist/VorticisMonitor.exe                     executável Windows (duplo clique abre o painel)
#   dist/VorticisMonitor-Portatil-<v>.zip        versão portátil (sem instalação)
#   dist/VorticisMonitor-Setup-<v>.exe           instalador (serviço do Windows)
#   dist/vorticis-monitor-linux-amd64            modo servidor web
set -euo pipefail
cd "$(dirname "$0")"
VERSAO="${VERSAO:-$(grep -oP 'Versao = "\K[^"]+' internal/api/server.go)}"
GOBIN="$(go env GOPATH)/bin"
rm -rf dist && mkdir -p dist

echo "== testes"; go test ./...

echo "== recursos Windows (ícone, versão, manifesto)"
[ -x "$GOBIN/go-winres" ] || go install github.com/tc-hib/go-winres@v0.3.3
(cd cmd/vorticis-monitor && "$GOBIN/go-winres" simply --arch amd64 --icon ../../installer/vorticis.ico --manifest cli \
  --product-name "Vorticis Monitor" --file-description "Vorticis Monitor Fiscal" --copyright "(c) 2026 Vorticis" \
  --product-version "$VERSAO" --file-version "$VERSAO" --original-filename "VorticisMonitor.exe")

echo "== windows/amd64"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/VorticisMonitor.exe ./cmd/vorticis-monitor

echo "== portátil"
P="dist/VorticisMonitor-Portatil-$VERSAO"
mkdir -p "$P/dados/importar" "$P/exemplos"
cp dist/VorticisMonitor.exe "$P/VorticisMonitor-Portatil.exe"
echo "Arquivo marcador do modo portátil: os dados ficam na pasta 'dados'." > "$P/vorticis.portatil"
cp installer/LEIA-ME-PORTATIL.txt installer/LICENCA.txt "$P/"
cp samples/*.xml "$P/exemplos/"
cp docs/ARQUITETURA.md "$P/"
(cd dist && zip -qr "VorticisMonitor-Portatil-$VERSAO.zip" "VorticisMonitor-Portatil-$VERSAO")
rm -rf "$P"

echo "== instalador"
cp dist/VorticisMonitor.exe dist/vorticis-monitor.exe
(cd installer && makensis -V2 -DVERSAO="$VERSAO" vorticis-monitor.nsi)
rm dist/vorticis-monitor.exe

echo "== linux/amd64 (modo servidor web)"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/vorticis-monitor-linux-amd64 ./cmd/vorticis-monitor

(cd dist && sha256sum * > SHA256SUMS.txt)
ls -lh dist
