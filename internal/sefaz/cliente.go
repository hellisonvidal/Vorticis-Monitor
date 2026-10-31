package sefaz

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/fiscal"
)

const nsNFe = "http://www.portalfiscal.inf.br/nfe"

// Cliente executa chamadas SOAP 1.2 com TLS mútuo.
type Cliente struct {
	http     *http.Client
	Ambiente int
	Cert     *Certificado
}

// NovoCliente prepara o cliente HTTP com o certificado do contribuinte.
func NovoCliente(cert *Certificado, ambiente int, verificarTLS bool, cadeiaExtraPEM string) *Cliente {
	pool, _ := x509.SystemCertPool()
	if pool == nil {
		pool = x509.NewCertPool()
	}
	if cadeiaExtraPEM != "" {
		if b, err := os.ReadFile(cadeiaExtraPEM); err == nil {
			pool.AppendCertsFromPEM(b)
		}
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{cert.TLS},
			RootCAs:            pool,
			InsecureSkipVerify: !verificarTLS, // somente se configurado explicitamente
			MinVersion:         tls.VersionTLS12,
			Renegotiation:      tls.RenegotiateOnceAsClient,
		},
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 20 * time.Second,
		MaxIdleConnsPerHost: 4,
	}
	return &Cliente{http: &http.Client{Transport: tr, Timeout: 60 * time.Second}, Ambiente: ambiente, Cert: cert}
}

func (c *Cliente) amb() string { return strconv.Itoa(c.Ambiente) }

func (c *Cliente) url(autorizador, servico string) (string, error) {
	u := URLs[c.amb()][autorizador][servico]
	if u == "" {
		return "", fmt.Errorf("webservice %s não configurado para %s (ambiente %s) — ajuste webservices.json", servico, autorizador, c.amb())
	}
	return u, nil
}

// soap envia a mensagem e devolve o corpo da resposta.
func (c *Cliente) soap(ctx context.Context, url, wsdlNS, corpo string, envolverInteresse bool) ([]byte, error) {
	dados := `<nfeDadosMsg xmlns="` + wsdlNS + `">` + corpo + `</nfeDadosMsg>`
	if envolverInteresse {
		dados = `<nfeDistDFeInteresse xmlns="` + wsdlNS + `"><nfeDadosMsg>` + corpo + `</nfeDadosMsg></nfeDistDFeInteresse>`
	}
	env := `<?xml version="1.0" encoding="utf-8"?><soap12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:soap12="http://www.w3.org/2003/05/soap-envelope"><soap12:Body>` +
		dados + `</soap12:Body></soap12:Envelope>`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(env))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha de comunicação com %s: %w", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("SEFAZ respondeu HTTP %d: %s", resp.StatusCode, trunc(string(b), 300))
	}
	return b, nil
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// extrair localiza o elemento `nome` no envelope e o decodifica.
func extrair(b []byte, nome string, v any) error {
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("resposta sem <%s>: %w", nome, err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == nome {
			return dec.DecodeElement(v, &se)
		}
	}
}

// ---------- Status do serviço ----------

type RetStatus struct {
	Autorizador string `json:"autorizador"`
	CStat       int    `xml:"cStat" json:"c_stat"`
	XMotivo     string `xml:"xMotivo" json:"x_motivo"`
	TMed        string `xml:"tMed" json:"t_med"`
	DhRecbto    string `xml:"dhRecbto" json:"dh_recbto"`
	XObs        string `xml:"xObs" json:"x_obs"`
	Online      bool   `json:"online"`
}

func (c *Cliente) StatusServico(ctx context.Context, uf, tpEmis string) (*RetStatus, error) {
	aut := Autorizador(uf, tpEmis)
	u, err := c.url(aut, SvcStatus)
	if err != nil {
		return nil, err
	}
	corpo := `<consStatServ xmlns="` + nsNFe + `" versao="4.00"><tpAmb>` + c.amb() + `</tpAmb><cUF>` + fiscal.CodigoPorUF[uf] + `</cUF><xServ>STATUS</xServ></consStatServ>`
	b, err := c.soap(ctx, u, "http://www.portalfiscal.inf.br/nfe/wsdl/NFeStatusServico4", corpo, false)
	if err != nil {
		return nil, err
	}
	r := &RetStatus{Autorizador: aut}
	if err := extrair(b, "retConsStatServ", r); err != nil {
		return nil, err
	}
	r.Online = r.CStat == 107
	return r, nil
}

// ---------- Consulta protocolo ----------

type RetConsulta struct {
	CStat   int    `xml:"cStat" json:"c_stat"`
	XMotivo string `xml:"xMotivo" json:"x_motivo"`
	ChNFe   string `xml:"chNFe" json:"chave"`
	ProtNFe *struct {
		InfProt struct {
			NProt    string `xml:"nProt"`
			DhRecbto string `xml:"dhRecbto"`
			CStat    int    `xml:"cStat"`
			XMotivo  string `xml:"xMotivo"`
		} `xml:"infProt"`
	} `xml:"protNFe" json:"-"`
	ProcEventos []struct {
		RetEvento struct {
			InfEvento struct {
				TpEvento    string `xml:"tpEvento"`
				CStat       int    `xml:"cStat"`
				NProt       string `xml:"nProt"`
				DhRegEvento string `xml:"dhRegEvento"`
				XEvento     string `xml:"xEvento"`
			} `xml:"infEvento"`
		} `xml:"retEvento"`
	} `xml:"procEventoNFe" json:"-"`
	Protocolo string `json:"protocolo"`
	DhAut     string `json:"dh_autorizacao"`
}

func (c *Cliente) ConsultaProtocolo(ctx context.Context, chave string) (*RetConsulta, error) {
	k := fiscal.AnalisaChave(chave)
	if !k.Valida {
		return nil, fmt.Errorf("chave inválida: %s", k.Motivo)
	}
	aut := Autorizador(fiscal.UFPorCodigo[k.CUF], k.TpEmis)
	u, err := c.url(aut, SvcConsulta)
	if err != nil {
		return nil, err
	}
	corpo := `<consSitNFe xmlns="` + nsNFe + `" versao="4.00"><tpAmb>` + c.amb() + `</tpAmb><xServ>CONSULTAR</xServ><chNFe>` + chave + `</chNFe></consSitNFe>`
	b, err := c.soap(ctx, u, "http://www.portalfiscal.inf.br/nfe/wsdl/NFeConsultaProtocolo4", corpo, false)
	if err != nil {
		return nil, err
	}
	r := &RetConsulta{}
	if err := extrair(b, "retConsSitNFe", r); err != nil {
		return nil, err
	}
	if r.ProtNFe != nil {
		r.Protocolo, r.DhAut = r.ProtNFe.InfProt.NProt, r.ProtNFe.InfProt.DhRecbto
	}
	return r, nil
}

// ---------- Distribuição DF-e (Ambiente Nacional) ----------

type DocZip struct {
	NSU    string `xml:"NSU,attr" json:"nsu"`
	Schema string `xml:"schema,attr" json:"schema"`
	B64    string `xml:",chardata" json:"-"`
	XML    []byte `json:"-"`
}

type RetDist struct {
	CStat   int      `xml:"cStat" json:"c_stat"`
	XMotivo string   `xml:"xMotivo" json:"x_motivo"`
	UltNSU  string   `xml:"ultNSU" json:"ult_nsu"`
	MaxNSU  string   `xml:"maxNSU" json:"max_nsu"`
	Docs    []DocZip `xml:"loteDistDFeInt>docZip" json:"docs"`
}

func (c *Cliente) distDFe(ctx context.Context, cUFAutor, cnpj, filtro string) (*RetDist, error) {
	u, err := c.url("AN", SvcDistDFe)
	if err != nil {
		return nil, err
	}
	doc := "<CNPJ>" + cnpj + "</CNPJ>"
	if len(cnpj) == 11 {
		doc = "<CPF>" + cnpj + "</CPF>"
	}
	corpo := `<distDFeInt xmlns="` + nsNFe + `" versao="1.01"><tpAmb>` + c.amb() + `</tpAmb><cUFAutor>` + cUFAutor + `</cUFAutor>` + doc + filtro + `</distDFeInt>`
	b, err := c.soap(ctx, u, "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe", corpo, true)
	if err != nil {
		return nil, err
	}
	r := &RetDist{}
	if err := extrair(b, "retDistDFeInt", r); err != nil {
		return nil, err
	}
	for i := range r.Docs {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(r.Docs[i].B64))
		if err != nil {
			continue
		}
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			continue
		}
		r.Docs[i].XML, _ = io.ReadAll(zr)
		r.Docs[i].B64 = ""
	}
	return r, nil
}

// DistDFePorNSU consulta a partir do último NSU (cStat 137 = nada novo; aguardar 1h).
func (c *Cliente) DistDFePorNSU(ctx context.Context, cUFAutor, cnpj, ultNSU string) (*RetDist, error) {
	return c.distDFe(ctx, cUFAutor, cnpj, "<distNSU><ultNSU>"+fmt.Sprintf("%015s", ultNSU)+"</ultNSU></distNSU>")
}

// DistDFePorChave baixa o XML completo de uma chave (após ciência da operação).
func (c *Cliente) DistDFePorChave(ctx context.Context, cUFAutor, cnpj, chave string) (*RetDist, error) {
	return c.distDFe(ctx, cUFAutor, cnpj, "<consChNFe><chNFe>"+chave+"</chNFe></consChNFe>")
}

// ---------- Manifestação do destinatário ----------

// Tipos de evento de manifestação (NT 2012.002 / 2020.001).
var EventosManifestacao = map[string]string{
	"210210": "Ciencia da Operacao",
	"210200": "Confirmacao da Operacao",
	"210220": "Desconhecimento da Operacao",
	"210240": "Operacao nao Realizada",
}

type RetEvento struct {
	CStat        int    `xml:"cStat" json:"c_stat"`
	XMotivo      string `xml:"xMotivo" json:"x_motivo"`
	EventoCStat  int    `json:"evento_c_stat"`
	EventoMotivo string `json:"evento_x_motivo"`
	Protocolo    string `json:"protocolo"`
	DhRegistro   string `json:"dh_registro"`
	RetEvento    []struct {
		InfEvento struct {
			CStat       int    `xml:"cStat"`
			XMotivo     string `xml:"xMotivo"`
			NProt       string `xml:"nProt"`
			DhRegEvento string `xml:"dhRegEvento"`
		} `xml:"infEvento"`
	} `xml:"retEvento" json:"-"`
	XMLEnviado string `json:"-"`
}

// MontarEventoManifestacao gera o <envEvento> assinado.
func MontarEventoManifestacao(cert *Certificado, ambiente int, cnpj, chave, tpEvento, xJust string, dh time.Time) (string, error) {
	desc, ok := EventosManifestacao[tpEvento]
	if !ok {
		return "", fmt.Errorf("tipo de evento de manifestação inválido: %s", tpEvento)
	}
	if tpEvento == "210240" && len([]rune(xJust)) < 15 {
		return "", fmt.Errorf("operação não realizada exige justificativa com 15 a 255 caracteres")
	}
	id := "ID" + tpEvento + chave + "01"
	dhs := dh.In(time.FixedZone("BRT", -3*3600)).Format("2006-01-02T15:04:05-07:00")
	det := `<descEvento>` + desc + `</descEvento>`
	if tpEvento == "210240" {
		det += `<xJust>` + EscaparTexto(xJust) + `</xJust>`
	}
	inf := `<infEvento xmlns="` + nsNFe + `" Id="` + id + `"><cOrgao>91</cOrgao><tpAmb>` + strconv.Itoa(ambiente) + `</tpAmb><CNPJ>` + cnpj + `</CNPJ><chNFe>` + chave +
		`</chNFe><dhEvento>` + dhs + `</dhEvento><tpEvento>` + tpEvento + `</tpEvento><nSeqEvento>1</nSeqEvento><verEvento>1.00</verEvento>` +
		`<detEvento versao="1.00">` + det + `</detEvento></infEvento>`
	sig, err := AssinarXMLDSig(inf, id, cert)
	if err != nil {
		return "", err
	}
	// No documento final o xmlns é herdado de <envEvento>; removemos a redeclaração.
	infDoc := strings.Replace(inf, ` xmlns="`+nsNFe+`"`, "", 1)
	return `<envEvento xmlns="` + nsNFe + `" versao="1.00"><idLote>` + strconv.FormatInt(dh.UnixNano()%1e15, 10) + `</idLote><evento versao="1.00">` + infDoc + sig + `</evento></envEvento>`, nil
}

// Manifestar envia o evento de manifestação do destinatário ao Ambiente Nacional.
func (c *Cliente) Manifestar(ctx context.Context, cnpj, chave, tpEvento, xJust string) (*RetEvento, error) {
	u, err := c.url("AN", SvcEvento)
	if err != nil {
		return nil, err
	}
	env, err := MontarEventoManifestacao(c.Cert, c.Ambiente, cnpj, chave, tpEvento, xJust, time.Now())
	if err != nil {
		return nil, err
	}
	b, err := c.soap(ctx, u, "http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4", env, false)
	if err != nil {
		return nil, err
	}
	r := &RetEvento{XMLEnviado: env}
	if err := extrair(b, "retEnvEvento", r); err != nil {
		return nil, err
	}
	if len(r.RetEvento) > 0 {
		i := r.RetEvento[0].InfEvento
		r.EventoCStat, r.EventoMotivo, r.Protocolo, r.DhRegistro = i.CStat, i.XMotivo, i.NProt, i.DhRegEvento
	}
	return r, nil
}
