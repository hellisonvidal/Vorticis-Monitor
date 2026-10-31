// Package cadastro preenche o cadastro da empresa a partir do CNPJ: dados da
// Receita Federal (via serviços públicos BrasilAPI, CNPJ.ws e ReceitaWS, com
// fallback entre eles) e Inscrição Estadual pela SEFAZ da UF com o certificado
// digital (CadConsultaCadastro4) ou pelos dados do Sintegra agregados no CNPJ.ws.
package cadastro

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/fiscal"
)

// Inscricao é uma Inscrição Estadual encontrada para o CNPJ.
type Inscricao struct {
	IE     string `json:"ie"`
	UF     string `json:"uf"`
	Ativa  bool   `json:"ativa"`
	Fonte  string `json:"fonte"`
	Regime string `json:"regime_apuracao,omitempty"`
}

// Dados é o resultado consolidado da consulta.
type Dados struct {
	CNPJ         string      `json:"cnpj"`
	RazaoSocial  string      `json:"razao_social"`
	NomeFantasia string      `json:"nome_fantasia"`
	Situacao     string      `json:"situacao"`
	UF           string      `json:"uf"`
	Municipio    string      `json:"municipio"`
	CodMun       string      `json:"cod_mun"`
	Logradouro   string      `json:"logradouro"`
	Numero       string      `json:"numero"`
	Complemento  string      `json:"complemento"`
	Bairro       string      `json:"bairro"`
	CEP          string      `json:"cep"`
	Telefone     string      `json:"telefone"`
	Email        string      `json:"email"`
	CNAE         string      `json:"cnae"`
	CNAEDescr    string      `json:"cnae_descricao"`
	Simples      bool        `json:"simples"`
	MEI          bool        `json:"mei"`
	Regime       string      `json:"regime_sugerido"` // SN | MEI | LP (Receita não informa LP/LR)
	Inscricoes   []Inscricao `json:"inscricoes"`
	IE           string      `json:"ie"` // IE ativa da UF da empresa (sugestão)
	Fontes       []string    `json:"fontes"`
	Avisos       []string    `json:"avisos"`
}

// URLs dos provedores (variáveis para permitir testes).
var (
	URLBrasilAPI = "https://brasilapi.com.br/api/cnpj/v1/"
	URLCNPJws    = "https://publica.cnpj.ws/cnpj/"
	URLReceitaWS = "https://receitaws.com.br/v1/cnpj/"
	Cliente      = &http.Client{Timeout: 15 * time.Second}
)

func obterJSON(ctx context.Context, url string, v any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "VorticisMonitor/1.0")
	resp, err := Cliente.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("CNPJ não encontrado")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.Unmarshal(b, v)
}

func vazio(s *string, v string) {
	if *s == "" {
		*s = strings.TrimSpace(v)
	}
}

func brasilAPI(ctx context.Context, cnpj string, d *Dados) error {
	var r struct {
		RazaoSocial  string `json:"razao_social"`
		NomeFantasia string `json:"nome_fantasia"`
		Situacao     string `json:"descricao_situacao_cadastral"`
		UF           string `json:"uf"`
		Municipio    string `json:"municipio"`
		CodIBGE      any    `json:"codigo_municipio_ibge"`
		Logradouro   string `json:"logradouro"`
		TipoLogr     string `json:"descricao_tipo_de_logradouro"`
		Numero       string `json:"numero"`
		Complemento  string `json:"complemento"`
		Bairro       string `json:"bairro"`
		CEP          any    `json:"cep"`
		Telefone     string `json:"ddd_telefone_1"`
		Email        string `json:"email"`
		CNAE         any    `json:"cnae_fiscal"`
		CNAEDescr    string `json:"cnae_fiscal_descricao"`
		Simples      *bool  `json:"opcao_pelo_simples"`
		MEI          *bool  `json:"opcao_pelo_mei"`
	}
	if err := obterJSON(ctx, URLBrasilAPI+cnpj, &r); err != nil {
		return err
	}
	logr := strings.TrimSpace(r.TipoLogr + " " + r.Logradouro)
	vazio(&d.RazaoSocial, r.RazaoSocial)
	vazio(&d.NomeFantasia, r.NomeFantasia)
	vazio(&d.Situacao, r.Situacao)
	vazio(&d.UF, r.UF)
	vazio(&d.Municipio, r.Municipio)
	vazio(&d.CodMun, texto(r.CodIBGE))
	vazio(&d.Logradouro, logr)
	vazio(&d.Numero, r.Numero)
	vazio(&d.Complemento, r.Complemento)
	vazio(&d.Bairro, r.Bairro)
	vazio(&d.CEP, texto(r.CEP))
	vazio(&d.Telefone, r.Telefone)
	vazio(&d.Email, r.Email)
	vazio(&d.CNAE, texto(r.CNAE))
	vazio(&d.CNAEDescr, r.CNAEDescr)
	if r.Simples != nil && *r.Simples {
		d.Simples = true
	}
	if r.MEI != nil && *r.MEI {
		d.MEI = true
	}
	return nil
}

func cnpjWS(ctx context.Context, cnpj string, d *Dados) error {
	var r struct {
		RazaoSocial     string `json:"razao_social"`
		Estabelecimento struct {
			NomeFantasia string `json:"nome_fantasia"`
			Situacao     string `json:"situacao_cadastral"`
			TipoLogr     string `json:"tipo_logradouro"`
			Logradouro   string `json:"logradouro"`
			Numero       string `json:"numero"`
			Complemento  string `json:"complemento"`
			Bairro       string `json:"bairro"`
			CEP          string `json:"cep"`
			Email        string `json:"email"`
			DDD1         string `json:"ddd1"`
			Tel1         string `json:"telefone1"`
			Cidade       struct {
				Nome   string `json:"nome"`
				IBGEID any    `json:"ibge_id"`
			} `json:"cidade"`
			Estado struct {
				Sigla string `json:"sigla"`
			} `json:"estado"`
			Atividade struct {
				ID        string `json:"id"`
				Descricao string `json:"descricao"`
			} `json:"atividade_principal"`
			Inscricoes []struct {
				IE     string `json:"inscricao_estadual"`
				Ativo  bool   `json:"ativo"`
				Estado struct {
					Sigla string `json:"sigla"`
				} `json:"estado"`
			} `json:"inscricoes_estaduais"`
		} `json:"estabelecimento"`
		Simples *struct {
			Simples string `json:"simples"`
			MEI     string `json:"mei"`
		} `json:"simples"`
	}
	if err := obterJSON(ctx, URLCNPJws+cnpj, &r); err != nil {
		return err
	}
	e := r.Estabelecimento
	vazio(&d.RazaoSocial, r.RazaoSocial)
	vazio(&d.NomeFantasia, e.NomeFantasia)
	vazio(&d.Situacao, e.Situacao)
	vazio(&d.UF, e.Estado.Sigla)
	vazio(&d.Municipio, e.Cidade.Nome)
	vazio(&d.CodMun, texto(e.Cidade.IBGEID))
	vazio(&d.Logradouro, strings.TrimSpace(e.TipoLogr+" "+e.Logradouro))
	vazio(&d.Numero, e.Numero)
	vazio(&d.Complemento, e.Complemento)
	vazio(&d.Bairro, e.Bairro)
	vazio(&d.CEP, e.CEP)
	vazio(&d.Email, e.Email)
	vazio(&d.Telefone, e.DDD1+e.Tel1)
	vazio(&d.CNAE, e.Atividade.ID)
	vazio(&d.CNAEDescr, e.Atividade.Descricao)
	if r.Simples != nil {
		d.Simples = d.Simples || strings.EqualFold(r.Simples.Simples, "Sim")
		d.MEI = d.MEI || strings.EqualFold(r.Simples.MEI, "Sim")
	}
	for _, i := range e.Inscricoes {
		d.Inscricoes = append(d.Inscricoes, Inscricao{IE: fiscal.SoDigitos(i.IE), UF: i.Estado.Sigla, Ativa: i.Ativo, Fonte: "Sintegra (via CNPJ.ws)"})
	}
	return nil
}

func receitaWS(ctx context.Context, cnpj string, d *Dados) error {
	var r struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		Nome        string `json:"nome"`
		Fantasia    string `json:"fantasia"`
		Situacao    string `json:"situacao"`
		UF          string `json:"uf"`
		Municipio   string `json:"municipio"`
		Logradouro  string `json:"logradouro"`
		Numero      string `json:"numero"`
		Complemento string `json:"complemento"`
		Bairro      string `json:"bairro"`
		CEP         string `json:"cep"`
		Telefone    string `json:"telefone"`
		Email       string `json:"email"`
		Simples     struct {
			Optante bool `json:"optante"`
		} `json:"simples"`
		Simei struct {
			Optante bool `json:"optante"`
		} `json:"simei"`
	}
	if err := obterJSON(ctx, URLReceitaWS+cnpj, &r); err != nil {
		return err
	}
	if strings.EqualFold(r.Status, "ERROR") {
		return fmt.Errorf("%s", r.Message)
	}
	vazio(&d.RazaoSocial, r.Nome)
	vazio(&d.NomeFantasia, r.Fantasia)
	vazio(&d.Situacao, r.Situacao)
	vazio(&d.UF, r.UF)
	vazio(&d.Municipio, r.Municipio)
	vazio(&d.Logradouro, r.Logradouro)
	vazio(&d.Numero, r.Numero)
	vazio(&d.Complemento, r.Complemento)
	vazio(&d.Bairro, r.Bairro)
	vazio(&d.CEP, r.CEP)
	vazio(&d.Telefone, r.Telefone)
	vazio(&d.Email, r.Email)
	d.Simples = d.Simples || r.Simples.Optante
	d.MEI = d.MEI || r.Simei.Optante
	return nil
}

func texto(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	}
	return fmt.Sprint(v)
}

// ConsultarCNPJ busca os dados públicos do CNPJ na Receita Federal (com fallback entre provedores).
func ConsultarCNPJ(ctx context.Context, cnpj string) (*Dados, error) {
	cnpj = fiscal.NormalizaCNPJ(cnpj)
	if !fiscal.ValidaCNPJ(cnpj) {
		return nil, fmt.Errorf("CNPJ inválido")
	}
	d := &Dados{CNPJ: cnpj}
	provedores := []struct {
		nome string
		f    func(context.Context, string, *Dados) error
	}{{"Receita Federal (BrasilAPI)", brasilAPI}, {"Receita Federal + Sintegra (CNPJ.ws)", cnpjWS}, {"Receita Federal (ReceitaWS)", receitaWS}}
	var erros []string
	for _, p := range provedores {
		c, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := p.f(c, cnpj, d)
		cancel()
		if err != nil {
			erros = append(erros, p.nome+": "+err.Error())
			continue
		}
		d.Fontes = append(d.Fontes, p.nome)
		// Com razão social e IE já obtidas não é preciso consultar os demais.
		if d.RazaoSocial != "" && d.CodMun != "" && len(d.Inscricoes) > 0 {
			break
		}
	}
	if d.RazaoSocial == "" {
		return nil, fmt.Errorf("não foi possível consultar o CNPJ (%s)", strings.Join(erros, "; "))
	}
	d.CEP = fiscal.SoDigitos(d.CEP)
	d.UF = strings.ToUpper(d.UF)
	d.Normalizar()
	return d, nil
}

// Normalizar define o regime sugerido e a IE principal da UF da empresa.
func (d *Dados) Normalizar() {
	switch {
	case d.MEI:
		d.Regime = "MEI"
	case d.Simples:
		d.Regime = "SN"
	default:
		d.Regime = "LP"
		d.Avisos = appendUnico(d.Avisos, "A Receita não informa se a empresa é do Lucro Presumido ou Real: confira o regime.")
	}
	d.IE = ""
	for _, i := range d.Inscricoes {
		if i.UF == d.UF && i.Ativa {
			d.IE = i.IE
			break
		}
	}
	if d.IE == "" && len(d.Inscricoes) == 0 {
		d.Avisos = appendUnico(d.Avisos, "Inscrição Estadual não localizada nas fontes públicas; inclua o certificado digital para consultar a SEFAZ-"+d.UF+" ou consulte o Sintegra.")
	}
}

// MesclarSEFAZ acrescenta as IEs retornadas pela SEFAZ (prioridade sobre as demais fontes).
func (d *Dados) MesclarSEFAZ(uf string, ies []Inscricao) {
	var outras []Inscricao
	for _, i := range d.Inscricoes {
		dup := false
		for _, n := range ies {
			dup = dup || n.IE == i.IE
		}
		if !dup {
			outras = append(outras, i)
		}
	}
	d.Inscricoes = append(ies, outras...)
	d.Fontes = appendUnico(d.Fontes, "SEFAZ-"+uf+" (certificado digital)")
	var avisos []string
	for _, a := range d.Avisos {
		if !strings.HasPrefix(a, "Inscrição Estadual não localizada") {
			avisos = append(avisos, a)
		}
	}
	d.Avisos = avisos
	d.Normalizar()
}

func appendUnico(l []string, s string) []string {
	for _, x := range l {
		if x == s {
			return l
		}
	}
	return append(l, s)
}

// URLSintegra devolve o portal do Sintegra (consulta manual, com captcha) para conferência.
func URLSintegra(uf string) string {
	return "http://www.sintegra.gov.br/"
}
