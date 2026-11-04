package relatorios

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vorticis.com.br/monitor/internal/store"
)

func logoTeste() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 120, 60))
	for x := 0; x < 120; x++ {
		for y := 0; y < 60; y++ {
			img.Set(x, y, color.RGBA{uint8(20 + x), 90, uint8(160 - y), 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func TestDANFENFeCompleto(t *testing.T) {
	LogoPorCNPJ = func(c string) []byte {
		if c == "11222333000181" {
			return logoTeste()
		}
		return nil
	}
	defer func() { LogoPorCNPJ = func(string) []byte { return nil } }()
	arqs, _ := filepath.Glob("../../samples/nfe_*.xml")
	if len(arqs) == 0 {
		t.Skip("sem amostras")
	}
	for _, a := range arqs {
		x, _ := os.ReadFile(a)
		var b bytes.Buffer
		d := &store.Documento{Modelo: "NFE", XML: string(x), Status: "AUTORIZADA"}
		if strings.Contains(a, "contingencia") {
			d.Status = "CONTINGENCIA"
		}
		if err := PDF(&b, d, nil); err != nil || !bytes.HasPrefix(b.Bytes(), []byte("%PDF")) {
			t.Fatalf("%s: %v", a, err)
		}
		if dir := os.Getenv("DANFE_SAIDA"); dir != "" {
			os.WriteFile(filepath.Join(dir, strings.TrimSuffix(filepath.Base(a), ".xml")+".pdf"), b.Bytes(), 0o644)
		}
	}
}

func TestNomesDosArquivos(t *testing.T) {
	d := &store.Documento{Modelo: "NFE", Numero: "000101", EmitNome: "INDÚSTRIA GAÚCHA S/A", Chave: "43261087654321000198550010000558711100558714"}
	if n := NomePDF(d); n != "NF 101 - INDÚSTRIA GAÚCHA S A.pdf" {
		t.Fatalf("NF-e: %q", n)
	}
	d.Modelo, d.Numero, d.EmitNome = "NFSE", "66", "JOAO LUIS DOS SANTOS DA SILVA 72633034934"
	if n := NomePDF(d); n != "NFS 66 - JOAO LUIS DOS SANTOS DA SILVA 72633034934.pdf" {
		t.Fatalf("NFS-e: %q", n)
	}
	if n := NomeChave(d, "pdf"); n != d.Chave+".pdf" {
		t.Fatalf("chave: %q", n)
	}
	if cd := ContentDisposition("attachment", "NF 1 - AÇÃO.pdf"); !strings.Contains(cd, `filename="NF 1 - ACAO.pdf"`) || !strings.Contains(cd, "filename*=UTF-8''NF%201%20-%20A%C3%87%C3%83O.pdf") {
		t.Fatalf("content-disposition: %s", cd)
	}
}
