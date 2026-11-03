package relatorios

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"vorticis.com.br/monitor/internal/store"
)

func TestDANFSeNacionalCompleto(t *testing.T) {
	x, err := os.ReadFile("testdata/nfse_nacional_completa.xml")
	if err != nil {
		t.Fatal(err)
	}
	r := lerArvore(x)
	d := extrairNacional(r)
	espera := map[string][2]string{
		"chave":     {d.chave, "41190522414259560001120000000000006626115189169448"},
		"numero":    {d.numero, "66"},
		"prestador": {d.prest.nome, "JOAO LUIS DOS SANTOS DA SILVA 72633034934"},
		"tomador":   {d.toma.nome, "PONTA SERVICE TRANSPORTES RODOVIARIOS LTDA"},
		"tomEnd":    {d.toma.ender, "AVENIDA DAS CATARATAS, 5000, JARDIM"},
		"simples":   {d.simples, "Optante - Microempreendedor Individual (MEI)"},
		"trib":      {d.tribISSQN, "Operação tributável"},
		"ret":       {d.retISS, "Não retido"},
		"liq":       {moeda(d.vLiq), "R$ 2.375,00"},
		"iss":       {moeda(d.vISS), "R$ 47,50"},
		"tot":       {d.totTrib, "Simples Nacional: 6%"},
	}
	for k, v := range espera {
		if v[0] != v[1] {
			t.Errorf("%s = %q, esperado %q", k, v[0], v[1])
		}
	}
	if !strings.Contains(d.descricao, "deslocamento técnico até a garagem do tomador") {
		t.Errorf("descrição incompleta: %q", d.descricao)
	}
	var b bytes.Buffer
	if err := PDF(&b, &store.Documento{Modelo: "NFSE", XML: string(x), Status: "AUTORIZADA", Direcao: "E"}, nil); err != nil || !bytes.HasPrefix(b.Bytes(), []byte("%PDF")) {
		t.Fatalf("PDF: %v", err)
	}
	if p := os.Getenv("DANFSE_SAIDA"); p != "" {
		os.WriteFile(p, b.Bytes(), 0o644)
	}
}

func TestDANFSeABRASF(t *testing.T) {
	x := []byte(`<CompNfse><Nfse><InfNfse><Numero>123</Numero><CodigoVerificacao>AB12</CodigoVerificacao><DataEmissao>2026-10-10T10:00:00</DataEmissao>
<ValoresNfse><BaseCalculo>1000.00</BaseCalculo><Aliquota>5.00</Aliquota><ValorIss>50.00</ValorIss><ValorLiquidoNfse>950.00</ValorLiquidoNfse></ValoresNfse>
<PrestadorServico><RazaoSocial>PRESTADORA ABRASF LTDA</RazaoSocial><Endereco><Endereco>Rua X</Endereco><Numero>1</Numero><Bairro>Centro</Bairro><CodigoMunicipio>3550308</CodigoMunicipio><Uf>SP</Uf><Cep>01000000</Cep></Endereco></PrestadorServico>
<DeclaracaoPrestacaoServico><InfDeclaracaoPrestacaoServico><Competencia>2026-10-01</Competencia><Servico><Valores><ValorServicos>1000.00</ValorServicos><ValorIr>15.00</ValorIr><ValorIss>50.00</ValorIss></Valores><IssRetido>1</IssRetido><ItemListaServico>17.01</ItemListaServico><Discriminacao>Consultoria em gestão fiscal</Discriminacao><CodigoMunicipio>3550308</CodigoMunicipio><ExigibilidadeISS>1</ExigibilidadeISS></Servico>
<Prestador><CpfCnpj><Cnpj>22334455000186</Cnpj></CpfCnpj><InscricaoMunicipal>998877</InscricaoMunicipal></Prestador>
<TomadorServico><IdentificacaoTomador><CpfCnpj><Cnpj>11222333000181</Cnpj></CpfCnpj></IdentificacaoTomador><RazaoSocial>TOMADORA SA</RazaoSocial></TomadorServico><OptanteSimplesNacional>2</OptanteSimplesNacional></InfDeclaracaoPrestacaoServico></DeclaracaoPrestacaoServico></InfNfse></Nfse></CompNfse>`)
	d := extrairABRASF(lerArvore(x))
	if d.numero != "123" || d.prest.nome != "PRESTADORA ABRASF LTDA" || d.prest.doc != "22.334.455/0001-86" || d.toma.nome != "TOMADORA SA" ||
		d.descricao != "Consultoria em gestão fiscal" || d.retISS != "Retido" || moeda(d.vISSRet) != "R$ 50,00" || moeda(d.vLiq) != "R$ 950,00" || d.prest.im != "998877" {
		t.Fatalf("ABRASF: %+v", d)
	}
	var b bytes.Buffer
	if err := DANFSe(&b, &store.Documento{Modelo: "NFSE", XML: string(x), Status: "AUTORIZADA"}); err != nil {
		t.Fatal(err)
	}
}
