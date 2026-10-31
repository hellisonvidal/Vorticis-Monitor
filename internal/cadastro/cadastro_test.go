package cadastro

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConsultarCNPJComFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/brasil/"):
			w.Write([]byte(`{"razao_social":"VORTICIS DEMO COMERCIO LTDA","nome_fantasia":"VORTICIS","descricao_situacao_cadastral":"ATIVA",
			"uf":"SP","municipio":"SAO PAULO","codigo_municipio_ibge":3550308,"descricao_tipo_de_logradouro":"RUA","logradouro":"DAS FLORES",
			"numero":"100","bairro":"CENTRO","cep":"01001000","cnae_fiscal":4751201,"cnae_fiscal_descricao":"Comércio","opcao_pelo_simples":false,"opcao_pelo_mei":false}`))
		case strings.HasPrefix(r.URL.Path, "/ws/"):
			w.Write([]byte(`{"razao_social":"VORTICIS DEMO COMERCIO LTDA","estabelecimento":{"estado":{"sigla":"SP"},"cidade":{"nome":"São Paulo","ibge_id":3550308},
			"inscricoes_estaduais":[{"inscricao_estadual":"110.042.490.114","ativo":true,"estado":{"sigla":"SP"}},{"inscricao_estadual":"0961234567","ativo":false,"estado":{"sigla":"RS"}}]},
			"simples":{"simples":"Não","mei":"Não"}}`))
		default:
			http.Error(w, "x", 500)
		}
	}))
	defer srv.Close()
	URLBrasilAPI, URLCNPJws, URLReceitaWS = srv.URL+"/brasil/", srv.URL+"/ws/", srv.URL+"/rws/"
	d, err := ConsultarCNPJ(context.Background(), "11.222.333/0001-81")
	if err != nil {
		t.Fatal(err)
	}
	if d.RazaoSocial != "VORTICIS DEMO COMERCIO LTDA" || d.CodMun != "3550308" || d.Logradouro != "RUA DAS FLORES" {
		t.Fatalf("dados: %+v", d)
	}
	if d.IE != "110042490114" || len(d.Inscricoes) != 2 || d.Regime != "LP" {
		t.Fatalf("IE/regime: %+v", d)
	}
	d.MesclarSEFAZ("SP", []Inscricao{{IE: "110042490114", UF: "SP", Ativa: true, Fonte: "SEFAZ-SP"}})
	if d.Inscricoes[0].Fonte != "SEFAZ-SP" || len(d.Inscricoes) != 2 {
		t.Fatalf("mescla: %+v", d.Inscricoes)
	}
	if _, err := ConsultarCNPJ(context.Background(), "11222333000182"); err == nil {
		t.Fatal("CNPJ inválido deveria falhar")
	}
}
