package sefaz

import (
	"context"
	"fmt"
)

// Contribuinte é o retorno do CadConsultaCadastro4 (cadastro do contribuinte na SEFAZ).
type Contribuinte struct {
	IE       string `xml:"IE" json:"ie"`
	CNPJ     string `xml:"CNPJ" json:"cnpj"`
	UF       string `xml:"UF" json:"uf"`
	CSit     string `xml:"cSit" json:"situacao"` // 0 não habilitado | 1 habilitado
	XNome    string `xml:"xNome" json:"razao_social"`
	XFant    string `xml:"xFant" json:"nome_fantasia"`
	XRegApur string `xml:"xRegApur" json:"regime_apuracao"`
	CNAE     string `xml:"CNAE" json:"cnae"`
	Ender    struct {
		XLgr    string `xml:"xLgr" json:"logradouro"`
		Nro     string `xml:"nro" json:"numero"`
		XBairro string `xml:"xBairro" json:"bairro"`
		CMun    string `xml:"cMun" json:"cod_mun"`
		XMun    string `xml:"xMun" json:"municipio"`
		CEP     string `xml:"CEP" json:"cep"`
	} `xml:"ender" json:"endereco"`
}

type retConsCad struct {
	InfCons struct {
		CStat   int            `xml:"cStat"`
		XMotivo string         `xml:"xMotivo"`
		InfCad  []Contribuinte `xml:"infCad"`
	} `xml:"infCons"`
}

// ConsultaCadastro consulta o contribuinte pelo CNPJ na SEFAZ da UF (exige certificado digital).
// cStat 111 = uma ocorrência; 112 = várias (uma por IE); 259 = não cadastrado.
func (c *Cliente) ConsultaCadastro(ctx context.Context, uf, cnpj string) ([]Contribuinte, error) {
	u := urlCadastro(uf)
	if u == "" {
		return nil, fmt.Errorf("a SEFAZ-%s não oferece consulta de cadastro por webservice", uf)
	}
	corpo := `<ConsCad xmlns="` + nsNFe + `" versao="2.00"><infCons><xServ>CONS-CAD</xServ><UF>` + uf + `</UF><CNPJ>` + cnpj + `</CNPJ></infCons></ConsCad>`
	b, err := c.soap(ctx, u, "http://www.portalfiscal.inf.br/nfe/wsdl/CadConsultaCadastro4", corpo, false)
	if err != nil {
		return nil, err
	}
	var r retConsCad
	if err := extrair(b, "retConsCad", &r); err != nil {
		return nil, err
	}
	if r.InfCons.CStat != 111 && r.InfCons.CStat != 112 {
		return nil, fmt.Errorf("SEFAZ-%s: %d %s", uf, r.InfCons.CStat, r.InfCons.XMotivo)
	}
	return r.InfCons.InfCad, nil
}
