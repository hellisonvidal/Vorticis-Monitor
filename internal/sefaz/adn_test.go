package sefaz

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDistNFSe(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write([]byte(`<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse"/>`))
	zw.Close()
	arq := base64.StdEncoding.EncodeToString(gz.Bytes())
	var consulta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		consulta = r.URL.String()
		switch r.URL.Path {
		case "/contribuintes/DFe/0":
			fmt.Fprintf(w, `{"StatusProcessamento":"DOCUMENTOS_LOCALIZADOS","LoteDFe":[{"NSU":7,"ChaveAcesso":"X","TipoDocumento":"NFSE","ArquivoXml":"%s"}]}`, arq)
		case "/contribuintes/DFe/7":
			w.WriteHeader(404)
		default:
			w.WriteHeader(403)
		}
	}))
	defer srv.Close()
	URLsADN[9] = srv.URL
	tr := novosTransportes(tls.Certificate{}, nil, false)
	tr.clientes[0] = srv.Client()
	c := &Cliente{tr: tr, Ambiente: 9}
	r, err := c.DistNFSe(context.Background(), "000", "11222333000181")
	if err != nil || len(r.Itens) != 1 || !bytes.HasPrefix(r.Itens[0].XML, []byte("<NFSe")) {
		t.Fatalf("lote: %+v %v", r, err)
	}
	if consulta != "/contribuintes/DFe/0?lote=true&cnpjConsulta=11222333000181" {
		t.Fatalf("consulta = %s", consulta)
	}
	if r, _ = c.DistNFSe(context.Background(), "7", ""); !r.SemNovos {
		t.Fatal("404 deve indicar sem documentos novos")
	}
	if r, _ = c.DistNFSe(context.Background(), "8", ""); !r.NegadoAcess {
		t.Fatal("403 deve indicar acesso negado")
	}
}
