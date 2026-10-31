#!/usr/bin/env bash
# Compila o Vorticis Monitor e gera os dois pacotes Windows, separados (requer Go 1.24+ e NSIS 3):
#   dist/instalador/VorticisMonitor-Setup-<v>.exe   instalador (instala para o usuário, sem administrador)
#   dist/portatil/VorticisMonitor-Portatil-<v>.exe   portátil (arquivo único, sem instalação; dados na pasta "dados" ao lado)
set -euo pipefail
cd "$(dirname "$0")"
VERSAO="${VERSAO:-$(grep -oP 'Versao = "\K[^"]+' internal/api/server.go)}"
GOBIN="$(go env GOPATH)/bin"
rm -rf dist && mkdir -p dist

echo "== testes"; go test ./...

echo "== recursos Windows (ícone, versão, manifesto)"
[ -x "$GOBIN/go-winres" ] || go install github.com/tc-hib/go-winres@v0.3.3
(cd cmd/vorticis-monitor && "$GOBIN/go-winres" make --arch amd64 --product-version "$VERSAO.0" --file-version "$VERSAO.0")

echo "== windows/amd64"
mkdir -p dist/build dist/instalador dist/portatil
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/build/VorticisMonitor.exe ./cmd/vorticis-monitor

echo "== portátil (o nome com \"Portatil\" ativa o modo portátil)"
cp dist/build/VorticisMonitor.exe "dist/portatil/VorticisMonitor-Portatil-$VERSAO.exe"

echo "== instalador"
(cd installer && makensis -V2 -DVERSAO="$VERSAO" vorticis-monitor.nsi)
rm -rf dist/build

(cd dist && sha256sum instalador/* portatil/* > SHA256SUMS.txt)
ls -lhR dist
