package relatorios

import (
	"bytes"
	"testing"

	"vorticis.com.br/monitor/internal/store"
)

func TestMoeda(t *testing.T) {
	for v, s := range map[float64]string{0: "0,00", 1234.5: "1.234,50", -1000000: "-1.000.000,00", 12: "12,00"} {
		if Moeda(v) != s {
			t.Errorf("Moeda(%v) = %s, esperado %s", v, Moeda(v), s)
		}
	}
}

func TestFormatos(t *testing.T) {
	docs := []*store.Documento{{ID: 1, Modelo: "NFE", Direcao: "S", Status: "AUTORIZADA", Numero: "10", DhEmissao: "2026-10-01T10:00:00-03:00", DestNome: "Cliente; \"A\"", ValorTotal: 100.5}}
	for _, id := range []string{"documentos", "tributos", "participantes", "cfop", "manifestacao"} {
		r, err := Gerar(id, docs, func(int64) []store.Item { return []store.Item{{CFOP: "5102", ValorTotal: 100.5}} }, "teste")
		if err != nil {
			t.Fatal(err)
		}
		for f := range Formatos {
			var b bytes.Buffer
			if err := r.Escrever(&b, f); err != nil || b.Len() == 0 {
				t.Fatalf("%s/%s: %v", id, f, err)
			}
			if f == "xls" && (!bytes.HasPrefix(b.Bytes(), []byte{0xD0, 0xCF, 0x11, 0xE0}) || b.Len()%512 != 0) {
				t.Fatalf("xls inválido")
			}
		}
	}
}
