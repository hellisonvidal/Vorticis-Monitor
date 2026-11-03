package sefaz

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Distribuição de NFS-e do Padrão Nacional pelo ADN (Ambiente de Dados Nacional).
// GET {base}/contribuintes/DFe/{NSU}?lote=true&cnpjConsulta={CNPJ}, com TLS mútuo.
// O certificado pode ser o da própria empresa (ou de outro CNPJ da mesma raiz) ou,
// a partir de 01/11/2026, o de um procurador com procuração eletrônica NFS-e
// concedida pela empresa — nesse caso cnpjConsulta indica a empresa representada.
var URLsADN = map[int]string{
	1: "https://adn.nfse.gov.br",
	2: "https://adn.producaorestrita.nfse.gov.br",
}

// ItemADN é um documento distribuído (NFS-e ou evento).
type ItemADN struct {
	NSU             json.Number `json:"NSU"`
	ChaveAcesso     string      `json:"ChaveAcesso"`
	TipoDocumento   string      `json:"TipoDocumento"`
	TipoEvento      string      `json:"TipoEvento"`
	ArquivoXml      string      `json:"ArquivoXml"`
	DataHoraGeracao string      `json:"DataHoraGeracao"`
	XML             []byte      `json:"-"`
}

// RetADN é o resultado de uma página da distribuição.
type RetADN struct {
	HTTP        int
	Status      string
	Itens       []ItemADN
	Erros       []string
	RetryAfter  time.Duration
	SemNovos    bool // 404 ou lote vazio: não há documentos após o NSU informado
	NegadoAcess bool // 401/403: certificado sem permissão (sem procuração / sem vínculo)
}

// Mensagem resume o retorno para exibição.
func (r *RetADN) Mensagem() string {
	switch {
	case len(r.Erros) > 0:
		return strings.Join(r.Erros, "; ")
	case r.HTTP == 429:
		return "limite de consultas do ADN atingido (HTTP 429)"
	case r.SemNovos:
		return "nenhuma NFS-e nova no Ambiente Nacional"
	case r.NegadoAcess:
		return fmt.Sprintf("acesso negado pelo Ambiente Nacional (HTTP %d): o certificado não tem procuração eletrônica NFS-e ou vínculo com este CNPJ", r.HTTP)
	}
	return fmt.Sprintf("%d documento(s) recebido(s)", len(r.Itens))
}

// DistNFSe busca até 50 documentos a partir do NSU.
func (c *Cliente) DistNFSe(ctx context.Context, nsu, cnpjConsulta string) (*RetADN, error) {
	base := URLsADN[c.Ambiente]
	if base == "" {
		base = URLsADN[1]
	}
	n, _ := strconv.ParseInt(strings.TrimLeft(nsu, "0"), 10, 64)
	u := fmt.Sprintf("%s/contribuintes/DFe/%d?lote=true", base, n)
	if cnpjConsulta != "" {
		u += "&cnpjConsulta=" + cnpjConsulta
	}
	resp, err := c.do(ctx, http.MethodGet, u, nil, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, fmt.Errorf("falha de comunicação com o Ambiente Nacional da NFS-e: %w", err)
	}
	defer resp.Body.Close()
	corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	r := &RetADN{HTTP: resp.StatusCode}
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
		r.RetryAfter = time.Duration(s) * time.Second
	}
	var b struct {
		StatusProcessamento string          `json:"StatusProcessamento"`
		LoteDFe             json.RawMessage `json:"LoteDFe"`
		Erros               []struct {
			Codigo    string `json:"Codigo"`
			Descricao string `json:"Descricao"`
		} `json:"Erros"`
		Mensagem string `json:"mensagem"`
	}
	json.Unmarshal(corpo, &b)
	r.Status = b.StatusProcessamento
	for _, e := range b.Erros {
		r.Erros = append(r.Erros, strings.TrimSpace(e.Codigo+" "+e.Descricao))
	}
	if b.Mensagem != "" && len(r.Erros) == 0 && resp.StatusCode >= 400 {
		r.Erros = append(r.Erros, b.Mensagem)
	}
	switch {
	case resp.StatusCode == 404:
		r.SemNovos = true
		r.Erros = nil
		return r, nil
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		r.NegadoAcess = true
		return r, nil
	case resp.StatusCode == 429:
		return r, nil
	case resp.StatusCode >= 400:
		if len(r.Erros) == 0 {
			r.Erros = append(r.Erros, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(corpo[:min(len(corpo), 300)]))))
		}
		return r, nil
	}
	if len(b.LoteDFe) > 0 && b.LoteDFe[0] == '{' {
		var it ItemADN
		json.Unmarshal(b.LoteDFe, &it)
		r.Itens = []ItemADN{it}
	} else {
		json.Unmarshal(b.LoteDFe, &r.Itens)
	}
	for i := range r.Itens {
		r.Itens[i].XML = DecodificarXMLADN(r.Itens[i].ArquivoXml)
	}
	r.SemNovos = len(r.Itens) == 0
	return r, nil
}

// DecodificarXMLADN converte o ArquivoXml (Base64 de GZip, Base64 simples ou XML) em bytes.
func DecodificarXMLADN(v string) []byte {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if strings.HasPrefix(v, "<") {
		return []byte(v)
	}
	raw, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil
	}
	if len(raw) > 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil
		}
		raw, err = io.ReadAll(zr)
		if err != nil {
			return nil
		}
	}
	return raw
}
