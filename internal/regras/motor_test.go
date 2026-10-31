package regras

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/xmlfiscal"
)

type paramTeste struct{}

func (paramTeste) UF(uf string) (fiscal.ParametroUF, bool) {
	for _, p := range fiscal.ParametrosUFPadrao {
		if p.UF == uf {
			return p, true
		}
	}
	return fiscal.ParametroUF{}, false
}
func (paramTeste) MVA(ncm, cest, uf string, _ time.Time) (float64, bool) {
	if ncm == "87083090" && uf == "PR" {
		return 71.78, true
	}
	return 0, false
}
func (paramTeste) NCMExiste(string) (bool, bool) { return true, false }

func avaliar(t *testing.T, arquivo string, perfil *Perfil, direcao string) Resultado {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "samples", arquivo))
	if err != nil {
		t.Skip("amostra ausente; execute samples/gerar_amostras.py")
	}
	d, err := xmlfiscal.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	c := &Contexto{NFe: d.NFe, Prot: d.ProtNFe, NFSe: d.NFSe, Params: paramTeste{}, Empresa: perfil, Direcao: direcao}
	if d.CTe != nil {
		c.CTe = &d.CTe.CTe.InfCte
	}
	return Novo().Executar(c)
}

func temRegra(r Resultado, regra string, sev Severidade) bool {
	for _, o := range r.Ocorrencias {
		if o.Regra == regra && o.Severidade == sev {
			return true
		}
	}
	return false
}

var empresa = &Perfil{CNPJ: "11222333000181", UF: "SP", Regime: "LR"}

func TestVendaDIFALCorreta(t *testing.T) {
	r := avaliar(t, "nfe_venda_difal_ok.xml", empresa, "S")
	if r.Erros != 0 {
		t.Fatalf("esperava 0 erros, obtive %+v", r.Ocorrencias)
	}
}

func TestSTComErros(t *testing.T) {
	r := avaliar(t, "nfe_st_com_erros.xml", empresa, "S")
	for _, c := range []struct {
		regra string
		sev   Severidade
	}{{"NFE021", Erro}, {"NFE050", Alerta}, {"NFE080", Erro}} {
		if !temRegra(r, c.regra, c.sev) {
			t.Errorf("esperava %s/%s em %+v", c.regra, c.sev, r.Ocorrencias)
		}
	}
	if !r.Bloqueante {
		t.Error("documento deveria ser bloqueante")
	}
}

func TestContingenciaSVCErrada(t *testing.T) {
	r := avaliar(t, "nfe_contingencia_svc.xml", empresa, "S")
	if !temRegra(r, "NFE090", Erro) {
		t.Fatalf("esperava erro de contingência: %+v", r.Ocorrencias)
	}
}

func TestDevolucaoIncorreta(t *testing.T) {
	r := avaliar(t, "nfe_devolucao_incorreta.xml", empresa, "S")
	found321, found327 := false, false
	for _, o := range r.Ocorrencias {
		found321 = found321 || o.CodRejeicao == 321
		found327 = found327 || o.CodRejeicao == 327
	}
	if !found321 || !found327 {
		t.Fatalf("esperava rejeições 321 e 327: %+v", r.Ocorrencias)
	}
}

func TestEntradaFornecedor(t *testing.T) {
	r := avaliar(t, "nfe_entrada_fornecedor_rs.xml", empresa, "E")
	if r.Erros != 0 {
		t.Fatalf("entrada deveria estar ok: %+v", r.Ocorrencias)
	}
}

func TestNFSeRetencoes(t *testing.T) {
	r := avaliar(t, "nfse_nacional_tomada.xml", empresa, "E")
	if !temRegra(r, "NFS020", Alerta) { // 7.10 sem INSS
		t.Fatalf("esperava alerta de INSS: %+v", r.Ocorrencias)
	}
	if r.Erros != 0 {
		t.Fatalf("NFS-e sem erros esperados: %+v", r.Ocorrencias)
	}
}

func TestCTe(t *testing.T) {
	r := avaliar(t, "cte_frete_compra.xml", empresa, "E")
	if r.Erros != 0 {
		t.Fatalf("CT-e ok esperado: %+v", r.Ocorrencias)
	}
}

func TestCalculos(t *testing.T) {
	d := CalculaDIFAL(1000, 7, 20.5, 0, "BASE_DUPLA")
	if d.BaseDestino != 1169.81 || d.ValorDIFAL != 169.81 {
		t.Fatalf("difal base dupla: %+v", d)
	}
	if u := CalculaDIFAL(1000, 12, 18, 0, "BASE_UNICA"); u.ValorDIFAL != 60 {
		t.Fatalf("difal base única: %+v", u)
	}
	if m := MVAAjustada(40, 12, 19.5); m != 53.04 {
		t.Fatalf("MVA ajustada: %v", m)
	}
	st := CalculaICMSST(500, 40, 0, 19.5, 60)
	if st.BaseST != 700 || st.ValorST != 76.5 {
		t.Fatalf("ST: %+v", st)
	}
}

func TestValidadores(t *testing.T) {
	if !fiscal.ValidaCNPJ("11.222.333/0001-81") || fiscal.ValidaCNPJ("11222333000182") {
		t.Fatal("CNPJ numérico")
	}
	if !fiscal.ValidaCNPJ("12ABC34501DE35") { // exemplo oficial RFB de CNPJ alfanumérico
		t.Fatal("CNPJ alfanumérico")
	}
	if !fiscal.ValidaCPF("529.982.247-25") {
		t.Fatal("CPF")
	}
	if ok, local := fiscal.ValidaIE("110.042.490.114", "SP"); !ok || !local {
		t.Fatal("IE SP")
	}
}
