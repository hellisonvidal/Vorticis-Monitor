package relatorios

import (
	"archive/zip"
	"fmt"
	"io"
	"time"

	"vorticis.com.br/monitor/internal/store"
)

// ExportarZIP grava em ZIP os documentos selecionados, em XML e/ou PDF,
// organizados em pastas por modelo e movimento. Retorna a quantidade exportada.
func ExportarZIP(w io.Writer, s *store.Store, chaves []string, comXML, comPDF bool) (int, error) {
	zw := zip.NewWriter(w)
	n := 0
	for _, ch := range chaves {
		d, err := s.ObterDocumento(ch)
		if err != nil {
			continue
		}
		pasta := fmt.Sprintf("%s/%s", d.Modelo, map[string]string{"E": "recebidos", "S": "emitidos"}[d.Direcao])
		nome := d.Chave
		if d.Numero != "" {
			nome = d.Numero + "_" + d.Chave
		}
		if comXML && d.XML != "" {
			fw, err := zw.CreateHeader(&zip.FileHeader{Name: pasta + "/xml/" + nome + ".xml", Method: zip.Deflate, Modified: time.Now()})
			if err != nil {
				return n, err
			}
			io.WriteString(fw, d.XML)
		}
		if comPDF && !d.Resumo {
			its, _ := s.Itens(d.ID)
			fw, err := zw.CreateHeader(&zip.FileHeader{Name: pasta + "/pdf/" + nome + ".pdf", Method: zip.Deflate, Modified: time.Now()})
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
