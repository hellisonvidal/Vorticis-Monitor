package servicos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vorticis.com.br/monitor/internal/config"
	"vorticis.com.br/monitor/internal/regras"
	"vorticis.com.br/monitor/internal/store"
)

// NF-e em que a empresa cadastrada é só a transportadora: não é dela.
func TestNFeDeTerceiroComoTransportadora(t *testing.T) {
	x, err := os.ReadFile("../../samples/nfe_venda_difal_ok.xml")
	if err != nil {
		t.Skip("sem amostra")
	}
	const transp = "17661871000159"
	xml := strings.Replace(string(x), "<transp>", "<transp><transporta><CNPJ>"+transp+"</CNPJ><xNome>PONTA SERVICE</xNome></transporta>", 1)
	if !strings.Contains(xml, transp) {
		t.Fatal("amostra sem <transp>")
	}
	if NFeDaEmpresa([]byte(xml), func(d string) bool { return d == transp }) {
		t.Fatal("empresa só transportadora não pode ficar com a nota")
	}
	if !NFeDaEmpresa([]byte(xml), func(d string) bool { return d == "11222333000181" }) {
		t.Fatal("emitente deve ficar com a nota")
	}

	dir := t.TempDir()
	st, _ := store.Open(filepath.Join(dir, "t.db"))
	m := &Monitor{Store: st, Cfg: &config.Config{DataDir: dir}, Motor: regras.Novo(), Hub: NovoHub(st), clientes: map[int64]*clienteCache{}}
	st.SalvarEmpresa(&store.Empresa{CNPJ: transp, RazaoSocial: "PONTA SERVICE", UF: "PR"})
	if _, err := m.Importar([]byte(xml), "DISTDFE"); err != nil {
		t.Fatal(err)
	}
	if n := m.LimparNotasDeTerceiros(); n != 1 {
		t.Fatalf("removidas %d, esperado 1", n)
	}
}
