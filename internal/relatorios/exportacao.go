package relatorios

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/store"
)

// ExportarZIP grava em ZIP os documentos selecionados, em XML e/ou PDF,
// organizados em pastas por modelo e movimento. Retorna a quantidade exportada.
func ExportarZIP(w io.Writer, s *store.Store, chaves []string, comXML, comPDF bool) (int, error) {
	zw := zip.NewWriter(w)
	n := 0
	usados := map[string]int{}
	// unico evita nomes repetidos no ZIP (ex.: mesmo número de séries diferentes).
	unico := func(pasta, nome string) string {
		k := strings.ToLower(pasta + nome)
		usados[k]++
		if c := usados[k]; c > 1 {
			ext := path.Ext(nome)
			nome = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(nome, ext), c, ext)
		}
		return pasta + nome
	}
	for _, ch := range chaves {
		d, err := s.ObterDocumento(ch)
		if err != nil {
			continue
		}
		pasta := fmt.Sprintf("%s/%s", d.Modelo, map[string]string{"E": "recebidos", "S": "emitidos"}[d.Direcao])
		// PDF + XML: ambos pela chave de acesso; só PDF: "NF 123 - Fornecedor.pdf".
		nomePDF := NomePDF(d)
		if comXML {
			nomePDF = NomeChave(d, "pdf")
		}
		nomePDF = unico(pasta+"/pdf/", nomePDF)
		if comXML && d.XML != "" {
			fw, err := zw.CreateHeader(&zip.FileHeader{Name: unico(pasta+"/xml/", NomeChave(d, "xml")), Method: zip.Deflate, Modified: time.Now()})
			if err != nil {
				return n, err
			}
			io.WriteString(fw, d.XML)
		}
		if comPDF && !d.Resumo {
			its, _ := s.Itens(d.ID)
			fw, err := zw.CreateHeader(&zip.FileHeader{Name: nomePDF, Method: zip.Deflate, Modified: time.Now()})
			if err != nil {
				return n, err
			}
			if err := PDF(fw, d, its); err != nil {
				return n, err
			}
		}
		n++
	}
	return n, zw.Close()
}
