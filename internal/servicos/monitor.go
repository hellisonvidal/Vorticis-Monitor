package servicos

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"vorticis.com.br/monitor/internal/config"
	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/keystore"
	"vorticis.com.br/monitor/internal/regras"
	"vorticis.com.br/monitor/internal/sefaz"
	"vorticis.com.br/monitor/internal/store"
	"vorticis.com.br/monitor/internal/xmlfiscal"
)

// Monitor é o núcleo de serviços (compartilhado entre API, agendador e serviço Windows).
type Monitor struct {
	Store *store.Store
	Cfg   *config.Config
	Cofre *keystore.Cofre
	Motor *regras.Motor
	Hub   *Hub

	mu       sync.Mutex
	clientes map[int64]*clienteCache
}

type clienteCache struct {
	certID int64
	c      *sefaz.Cliente
}

func Novo(s *store.Store, cfg *config.Config, cofre *keystore.Cofre) *Monitor {
	return &Monitor{Store: s, Cfg: cfg, Cofre: cofre, Motor: regras.Novo(), Hub: NovoHub(s), clientes: map[int64]*clienteCache{}}
}

// ErrSemCertificado indica que a empresa ainda não possui certificado digital cadastrado.
var ErrSemCertificado = errors.New("empresa sem certificado digital A1 cadastrado — inclua o certificado em Configurações › Certificado Digital")

// ClienteSefaz devolve (com cache) o cliente SOAP com o certificado da empresa.
func (m *Monitor) ClienteSefaz(empresaID int64) (*sefaz.Cliente, *store.Empresa, error) {
	emp, err := m.Store.ObterEmpresa(empresaID)
	if err != nil {
		return nil, nil, fmt.Errorf("empresa %d não encontrada", empresaID)
	}
	ct, err := m.Store.CertificadoAtivo(empresaID)
	if err != nil {
		return nil, emp, ErrSemCertificado
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cc := m.clientes[empresaID]; cc != nil && cc.certID == ct.ID && cc.c.Ambiente == emp.Ambiente {
		return cc.c, emp, nil
	}
	pfx, err := m.Cofre.Decifrar(ct.PFX)
	if err != nil {
		return nil, emp, fmt.Errorf("falha ao abrir cofre do certificado: %w", err)
	}
	senha, err := m.Cofre.Decifrar(ct.Senha)
	if err != nil {
		return nil, emp, err
	}
	cert, err := sefaz.CarregarPFX(pfx, string(senha))
	if err != nil {
		return nil, emp, err
	}
	if !cert.Valido() {
		return nil, emp, fmt.Errorf("certificado digital vencido em %s", cert.Validade.Format("02/01/2006"))
	}
	c := sefaz.NovoCliente(cert, emp.Ambiente, m.Cfg.VerificarTLSSefaz, m.Cfg.CadeiaICPBrasil)
	m.clientes[empresaID] = &clienteCache{certID: ct.ID, c: c}
	return c, emp, nil
}

// InvalidarCliente descarta o cliente em cache (troca de certificado/ambiente).
func (m *Monitor) InvalidarCliente(empresaID int64) {
	m.mu.Lock()
	delete(m.clientes, empresaID)
	m.mu.Unlock()
}

// ResultadoImportacao resume o processamento de um XML.
type ResultadoImportacao struct {
	Arquivo     string              `json:"arquivo,omitempty"`
	Chave       string              `json:"chave"`
	Modelo      string              `json:"modelo"`
	Status      string              `json:"status"`
	Direcao     string              `json:"direcao"`
	Erros       int                 `json:"erros"`
	Alertas     int                 `json:"alertas"`
	Score       int                 `json:"score"`
	Mensagem    string              `json:"mensagem,omitempty"`
	Ocorrencias []regras.Ocorrencia `json:"ocorrencias,omitempty"`
}

func (m *Monitor) empresaPorDoc(docs ...string) *store.Empresa {
	for _, d := range docs {
		if d == "" {
			continue
		}
		if e, err := m.Store.EmpresaPorCNPJ(d); err == nil {
			return e
		}
	}
	return nil
}

func perfil(e *store.Empresa) *regras.Perfil {
	if e == nil {
		return nil
	}
	return &regras.Perfil{CNPJ: e.CNPJ, UF: e.UF, Regime: e.Regime}
}

// Validar executa o motor sem persistir (pré-emissão).
func (m *Monitor) Validar(xml []byte) (*xmlfiscal.Documento, regras.Resultado, error) {
	d, err := xmlfiscal.Parse(xml)
	if err != nil {
		return nil, regras.Resultado{}, err
	}
	ctx := m.contexto(d, nil)
	return d, m.Motor.Executar(ctx), nil
}

func (m *Monitor) contexto(d *xmlfiscal.Documento, finalidades map[int]string) *regras.Contexto {
	c := &regras.Contexto{Params: m.Store, Agora: time.Now(), FinalidadeItem: finalidades}
	switch {
	case d.NFe != nil:
		c.NFe, c.Prot = d.NFe, d.ProtNFe
		if e := m.empresaPorDoc(d.NFe.Emit.Doc()); e != nil {
			c.Empresa, c.Direcao = perfil(e), "S"
			if d.NFe.Ide.TpNF == "0" {
				c.Direcao = "E"
			}
		} else if e := m.empresaPorDoc(d.NFe.Dest.Doc()); e != nil {
			c.Empresa, c.Direcao = perfil(e), "E"
		} else {
			c.Direcao = "S"
		}
	case d.CTe != nil:
		c.CTe = &d.CTe.CTe.InfCte
		c.Direcao = "E"
		if e := m.empresaPorDoc(c.CTe.Emit.CNPJ); e != nil {
			c.Empresa, c.Direcao = perfil(e), "S"
		} else {
			c.Empresa = perfil(m.empresaPorDoc(c.CTe.Dest.CNPJ, c.CTe.Rem.CNPJ, c.CTe.Exped.CNPJ, c.CTe.Receb.CNPJ))
		}
	case d.NFSe != nil:
		c.NFSe = d.NFSe
		if e := m.empresaPorDoc(d.NFSe.PrestadorDoc); e != nil {
			c.Empresa, c.Direcao = perfil(e), "S"
		} else {
			c.Empresa, c.Direcao = perfil(m.empresaPorDoc(d.NFSe.TomadorDoc)), "E"
		}
	}
	return c
}

func hashXML(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func converterOcorrencias(ocs []regras.Ocorrencia) []store.Ocorrencia {
	out := make([]store.Ocorrencia, 0, len(ocs))
	for _, o := range ocs {
		out = append(out, store.Ocorrencia{Regra: o.Regra, Grupo: o.Grupo, Severidade: string(o.Severidade), NItem: o.Item, Campo: o.Campo,
			Mensagem: o.Mensagem, Sugestao: o.Sugestao, Esperado: o.Esperado, Informado: o.Informado, CodRejeicao: o.CodRejeicao, AcaoCorrecao: o.AcaoCorrecao})
	}
	return out
}

// Importar processa um XML (ERP, pasta monitorada, upload ou Distribuição DF-e).
func (m *Monitor) Importar(xml []byte, origem string) (*ResultadoImportacao, error) {
	return m.importar(xml, origem, nil)
}

func (m *Monitor) importar(xml []byte, origem string, finalidades map[int]string) (*ResultadoImportacao, error) {
	d, err := xmlfiscal.Parse(xml)
	if err != nil {
		return nil, err
	}
	switch d.Tipo {
	case xmlfiscal.TipoEvento:
		return m.importarEvento(d)
	case xmlfiscal.TipoResumo:
		return m.importarResumo(d, origem)
	}
	ctx := m.contexto(d, finalidades)
	res := m.Motor.Executar(ctx)
	doc := &store.Documento{Direcao: ctx.Direcao, Origem: origem, XML: string(d.Bruto), Hash: hashXML(d.Bruto)}
	var itens []store.Item
	var refs []string
	switch {
	case d.NFe != nil:
		itens, refs = m.preencherNFe(doc, d)
		res.Ocorrencias = append(res.Ocorrencias, m.validarDevolucao(d.NFe, refs)...)
	case d.CTe != nil:
		m.preencherCTe(doc, d)
		for _, n := range d.CTe.CTe.InfCte.InfCTeNorm.InfDoc.InfNFe {
			refs = append(refs, n.Chave)
		}
	case d.MDFe != nil:
		m.preencherMDFe(doc, d)
	case d.NFSe != nil:
		m.preencherNFSe(doc, d)
	}
	if ctx.Empresa != nil {
		if e, err := m.Store.EmpresaPorCNPJ(ctx.Empresa.CNPJ); err == nil {
			doc.EmpresaID = &e.ID
		}
	}
	// Recontagem após regras dependentes de base (devolução)
	res.Erros, res.Alertas = 0, 0
	for _, o := range res.Ocorrencias {
		switch o.Severidade {
		case regras.Erro:
			res.Erros++
		case regras.Alerta:
			res.Alertas++
		}
	}
	doc.Score, doc.Erros, doc.Alertas = max(0, 100-15*res.Erros-5*res.Alertas), res.Erros, res.Alertas
	if origem == "ERP" && res.Erros > 0 && doc.Status == StPendente {
		doc.Status = StValidacao
	}
	anterior, _ := m.Store.ObterDocumento(doc.Chave)
	if anterior != nil {
		if !TransicaoValida(anterior.Status, doc.Status) && doc.Status != anterior.Status {
			// Mantém status mais avançado (ex.: XML sem protocolo reimportado após autorização).
			doc.Status, doc.CStat, doc.XMotivo, doc.Protocolo = anterior.Status, anterior.CStat, anterior.XMotivo, anterior.Protocolo
		}
		doc.Manifestacao, doc.Conciliacao = anterior.Manifestacao, anterior.Conciliacao
	}
	if err := m.Store.SalvarDocumento(doc, itens, converterOcorrencias(res.Ocorrencias), refs); err != nil {
		return nil, err
	}
	m.arquivarXML(doc)
	tipoEv := "documento.novo"
	if anterior != nil {
		tipoEv = "documento.status"
	}
	m.Hub.Publicar(tipoEv, doc)
	return &ResultadoImportacao{Chave: doc.Chave, Modelo: doc.Modelo, Status: doc.Status, Direcao: doc.Direcao, Erros: res.Erros, Alertas: res.Alertas, Score: doc.Score, Ocorrencias: res.Ocorrencias}, nil
}

func tipoOperacao(direcao, fin string, cfop string) string {
	if fin == "4" {
		return "DEVOLUCAO"
	}
	if len(cfop) == 4 {
		switch cfop[1:] {
		case "151", "152", "153", "155", "156", "408", "409", "209", "552", "557":
			return "TRANSFERENCIA"
		}
		if fiscal.CFOPsDevolucao[cfop] {
			return "DEVOLUCAO"
		}
	}
	if direcao == "S" {
		return "VENDA"
	}
	return "COMPRA"
}

func (m *Monitor) preencherNFe(doc *store.Documento, d *xmlfiscal.Documento) ([]store.Item, []string) {
	n := d.NFe
	t := n.Total.ICMSTot
	doc.Chave, doc.Modelo = n.Chave(), string(d.Tipo)
	doc.Serie, doc.Numero, doc.DhEmissao = n.Ide.Serie, n.Ide.NNF, n.Ide.DhEmi
	doc.EmitDoc, doc.EmitNome, doc.EmitUF = n.Emit.Doc(), n.Emit.XNome, n.Emit.EnderEmit.UF
	doc.DestDoc, doc.DestNome, doc.DestUF = n.Dest.Doc(), n.Dest.XNome, n.Dest.EnderDest.UF
	doc.Natureza, doc.TpEmis, doc.FinNFe = n.Ide.NatOp, n.Ide.TpEmis, n.Ide.FinNFe
	doc.ValorTotal, doc.ValorICMS, doc.ValorST, doc.ValorIPI = t.VNF.F(), t.VICMS.F(), t.VST.F(), t.VIPI.F()
	doc.ValorPIS, doc.ValorCOFINS, doc.ValorDIFAL = t.VPIS.F(), t.VCOFINS.F(), t.VICMSUFDest.F()
	if len(n.Det) > 0 {
		doc.CFOPPrincipal = n.Det[0].Prod.CFOP
	}
	doc.TipoOperacao = tipoOperacao(doc.Direcao, n.Ide.FinNFe, doc.CFOPPrincipal)
	if d.ProtNFe != nil {
		p := d.ProtNFe.InfProt
		doc.CStat, doc.XMotivo, doc.Protocolo, doc.DhAutorizacao = p.CStat, p.XMotivo, p.NProt, p.DhRecbto
		doc.Status = StatusPorCStat(p.CStat, n.Ide.TpEmis)
	} else if n.Ide.TpEmis != "1" && n.Ide.TpEmis != "" {
		doc.Status = StContingencia
	} else {
		doc.Status = StPendente
	}
	var itens []store.Item
	for _, x := range n.Det {
		g, ipi, pis, cof := x.Imposto.ICMSGrupo(), x.Imposto.IPIGrupo(), x.Imposto.PISGrupo(), x.Imposto.COFINSGrupo()
		it := store.Item{NItem: x.NItem, CodProd: x.Prod.CProd, Descricao: x.Prod.XProd, NCM: x.Prod.NCM, CEST: x.Prod.CEST, CFOP: x.Prod.CFOP, Unidade: x.Prod.UCom,
			Quantidade: x.Prod.QCom.F(), ValorUnit: x.Prod.VUnCom.F(), ValorTotal: x.Prod.VProd.F(), ValorDesconto: x.Prod.VDesc.F(),
			CSTICMS: g.Orig + g.CSTouCSOSN(), VBC: g.VBC.F(), PICMS: g.PICMS.F(), VICMS: g.VICMS.F(), VBCST: g.VBCST.F(), PMVA: g.PMVAST.F(), PICMSST: g.PICMSST.F(), VICMSST: g.VICMSST.F(),
			CSTIPI: ipi.CST, VIPI: ipi.VIPI.F(), CSTPIS: pis.CST, VPIS: pis.VPIS.F(), CSTCOFINS: cof.CST, VCOFINS: cof.VCOFINS.F(),
			PedCompra: x.Prod.XPed, ItemPed: x.Prod.NItemPed}
		if it.PedCompra == "" {
			it.PedCompra = n.Compra.XPed
		}
		if u := x.Imposto.ICMSUFDest; u != nil {
			it.VDIFAL, it.VFCPDest = u.VICMSUFDest.F(), u.VFCPUFDest.F()
		}
		itens = append(itens, it)
	}
	var refs []string
	for _, r := range n.Ide.NFref {
		if r.RefNFe != "" {
			refs = append(refs, r.RefNFe)
		}
	}
	return itens, refs
}

// validarDevolucao confronta a devolução com o documento original (quando presente na base).
func (m *Monitor) validarDevolucao(n *xmlfiscal.InfNFe, refs []string) []regras.Ocorrencia {
	if n.Ide.FinNFe != "4" || len(refs) == 0 {
		return nil
	}
	var out []regras.Ocorrencia
	origQtd := map[string]float64{}
	encontrou := false
	for _, r := range refs {
		orig, err := m.Store.ObterDocumento(r)
		if err != nil {
			out = append(out, regras.Ocorrencia{Regra: "DEV001", Grupo: "operacao", Severidade: regras.Info, Campo: "NFref", Mensagem: "Documento original " + r + " não está na base do Monitor (não foi possível conferir quantidades)"})
			continue
		}
		encontrou = true
		if orig.Status == StCancelada || orig.Status == StDenegada {
			out = append(out, regras.Ocorrencia{Regra: "DEV002", Grupo: "operacao", Severidade: regras.Erro, Campo: "NFref", Mensagem: "Devolução referencia documento " + orig.Status, Informado: r})
		}
		its, _ := m.Store.Itens(orig.ID)
		for _, i := range its {
			origQtd[i.CodProd] += i.Quantidade
		}
	}
	if !encontrou {
		return out
	}
	for _, d := range n.Det {
		if q, ok := origQtd[d.Prod.CProd]; ok && d.Prod.QCom.F() > q+0.0001 {
			out = append(out, regras.Ocorrencia{Regra: "DEV003", Grupo: "operacao", Severidade: regras.Erro, Item: d.NItem, Campo: "prod/qCom",
				Mensagem: "Quantidade devolvida excede a quantidade do documento original", Esperado: strconv.FormatFloat(q, 'f', 4, 64), Informado: strconv.FormatFloat(d.Prod.QCom.F(), 'f', 4, 64)})
		} else if !ok {
			out = append(out, regras.Ocorrencia{Regra: "DEV004", Grupo: "operacao", Severidade: regras.Alerta, Item: d.NItem, Campo: "prod/cProd", Mensagem: "Produto devolvido não consta no documento original", Informado: d.Prod.CProd})
		}
	}
	return out
}

func (m *Monitor) preencherCTe(doc *store.Documento, d *xmlfiscal.Documento) {
	c := d.CTe.CTe.InfCte
	doc.Chave, doc.Modelo, doc.TipoOperacao = strings.TrimPrefix(c.ID, "CTe"), string(xmlfiscal.TipoCTe), "TRANSPORTE"
	doc.Serie, doc.Numero, doc.DhEmissao, doc.TpEmis = c.Ide.Serie, c.Ide.NCT, c.Ide.DhEmi, c.Ide.TpEmis
	doc.EmitDoc, doc.EmitNome, doc.EmitUF = c.Emit.CNPJ, c.Emit.XNome, c.Emit.EnderEmit.UF
	doc.DestDoc, doc.DestNome, doc.DestUF = first(c.Dest.CNPJ, c.Dest.CPF), c.Dest.XNome, c.Ide.UFFim
	doc.Natureza, doc.CFOPPrincipal, doc.ValorTotal = c.Ide.NatOp, c.Ide.CFOP, c.VPrest.VTPrest.F()
	doc.ValorICMS = c.Imp.ICMS.Primeiro().VICMS.F()
	doc.Status = StPendente
	if p := d.CTe.ProtCTe; p != nil {
		doc.CStat, doc.XMotivo, doc.Protocolo, doc.DhAutorizacao = p.InfProt.CStat, p.InfProt.XMotivo, p.InfProt.NProt, p.InfProt.DhRecbto
		doc.Status = StatusPorCStat(p.InfProt.CStat, c.Ide.TpEmis)
	}
}

func (m *Monitor) preencherMDFe(doc *store.Documento, d *xmlfiscal.Documento) {
	x := d.MDFe.MDFe.InfMDFe
	doc.Chave, doc.Modelo, doc.TipoOperacao, doc.Direcao = strings.TrimPrefix(x.ID, "MDFe"), string(xmlfiscal.TipoMDFe), "TRANSPORTE", "S"
	doc.Serie, doc.Numero, doc.DhEmissao, doc.TpEmis = x.Ide.Serie, x.Ide.NMDF, x.Ide.DhEmi, x.Ide.TpEmis
	doc.EmitDoc, doc.EmitNome, doc.EmitUF, doc.DestUF = x.Emit.CNPJ, x.Emit.XNome, x.Ide.UFIni, x.Ide.UFFim
	doc.ValorTotal = x.Tot.VCarga.F()
	doc.Status = StPendente
	if p := d.MDFe.ProtMDFe; p != nil {
		doc.CStat, doc.XMotivo, doc.Protocolo, doc.DhAutorizacao = p.InfProt.CStat, p.InfProt.XMotivo, p.InfProt.NProt, p.InfProt.DhRecbto
		doc.Status = StatusPorCStat(p.InfProt.CStat, x.Ide.TpEmis)
	}
	if e := m.empresaPorDoc(x.Emit.CNPJ); e != nil {
		doc.EmpresaID = &e.ID
	}
}

func (m *Monitor) preencherNFSe(doc *store.Documento, d *xmlfiscal.Documento) {
	n := d.NFSe
	doc.Chave = n.Chave
	if doc.Chave == "" {
		doc.Chave = "NFSE-" + fiscal.NormalizaCNPJ(n.PrestadorDoc) + "-" + n.Numero
	}
	doc.Modelo, doc.TipoOperacao, doc.Numero, doc.DhEmissao = string(xmlfiscal.TipoNFSe), "SERVICO", n.Numero, n.DataEmissao
	doc.EmitDoc, doc.EmitNome, doc.DestDoc, doc.DestNome = n.PrestadorDoc, n.PrestadorNome, n.TomadorDoc, n.TomadorNome
	doc.Natureza, doc.CFOPPrincipal = "Serviço "+first(n.CodigoServico, fiscal.CTribNacParaLC116(n.CTribNac)), ""
	doc.ValorTotal, doc.ValorISS = n.ValorServicos, n.ValorISS
	doc.ValorRetencoes = n.ValorIR + n.ValorPIS + n.ValorCOFINS + n.ValorCSLL + n.ValorINSS
	if n.ISSRetido {
		doc.ValorRetencoes += n.ValorISS
	}
	doc.ValorPIS, doc.ValorCOFINS = n.ValorPIS, n.ValorCOFINS
	doc.Status, doc.CStat = StAutorizada, 100
	if n.Cancelada {
		doc.Status = StCancelada
	}
}

func (m *Monitor) importarEvento(d *xmlfiscal.Documento) (*ResultadoImportacao, error) {
	e := d.Evento
	inf, ret := e.Evento.InfEvento, e.RetEvento.InfEvento
	seq, _ := strconv.Atoi(inf.NSeqEvento)
	ev := store.Evento{Chave: inf.ChNFe, TpEvento: inf.TpEvento, Descricao: inf.DetEvento.DescEvento, NSeq: seq, DhEvento: inf.DhEvento,
		Protocolo: ret.NProt, CStat: ret.CStat, XMotivo: ret.XMotivo, XML: string(d.Bruto)}
	if err := m.Store.SalvarEvento(ev); err != nil {
		return nil, err
	}
	ok := ret.CStat == 135 || ret.CStat == 136 || ret.CStat == 155 || ret.CStat == 0
	status := ""
	if ok {
		switch inf.TpEvento {
		case "110111", "110112": // cancelamento / cancelamento por substituição (NFC-e)
			status = StCancelada
			m.Store.AtualizarSituacao(inf.ChNFe, StCancelada, 101, "Cancelamento registrado ("+inf.DetEvento.XJust+")", "", "")
		case "210200":
			m.Store.AtualizarManifestacao(inf.ChNFe, "CONFIRMADA")
		case "210210":
			m.Store.AtualizarManifestacao(inf.ChNFe, "CIENCIA")
		case "210220":
			m.Store.AtualizarManifestacao(inf.ChNFe, "DESCONHECIDA")
		case "210240":
			m.Store.AtualizarManifestacao(inf.ChNFe, "NAO_REALIZADA")
		}
	}
	m.Hub.Publicar("documento.status", map[string]string{"chave": inf.ChNFe, "evento": inf.TpEvento, "status": status})
	return &ResultadoImportacao{Chave: inf.ChNFe, Modelo: "EVENTO", Status: status, Mensagem: "Evento " + inf.TpEvento + " " + inf.DetEvento.DescEvento + " registrado"}, nil
}

func (m *Monitor) importarResumo(d *xmlfiscal.Documento, origem string) (*ResultadoImportacao, error) {
	r := d.Resumo
	doc := &store.Documento{Chave: r.ChNFe, Modelo: "NFE", Direcao: "E", TipoOperacao: "COMPRA", DhEmissao: r.DhEmi, DhAutorizacao: r.DhRecbto,
		EmitDoc: first(r.CNPJ, r.CPF), EmitNome: r.XNome, ValorTotal: r.VNF.F(), Protocolo: r.NProt, Origem: origem, Resumo: true, Status: StResumo, CStat: 100,
		XML: string(d.Bruto), Hash: hashXML(d.Bruto), Score: 100}
	switch r.CSitNFe {
	case "2":
		doc.Status, doc.CStat = StDenegada, 110
	case "3":
		doc.Status, doc.CStat = StCancelada, 101
	}
	k := fiscal.AnalisaChave(r.ChNFe)
	doc.Serie, doc.Numero, doc.EmitUF = strings.TrimLeft(k.Serie, "0"), strings.TrimLeft(k.Numero, "0"), fiscal.UFPorCodigo[k.CUF]
	if err := m.Store.SalvarDocumento(doc, nil, nil, nil); err != nil {
		return nil, err
	}
	m.Hub.Publicar("documento.novo", doc)
	return &ResultadoImportacao{Chave: doc.Chave, Modelo: "RESUMO_NFE", Status: doc.Status, Direcao: "E", Mensagem: "Resumo recebido — manifeste ciência para baixar o XML completo"}, nil
}

func first(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// arquivarXML grava uma cópia em <dados>/xml/AAAAMM/<chave>.xml (backup e leitura por escrituração).
func (m *Monitor) arquivarXML(d *store.Documento) {
	mes := "000000"
	if len(d.DhEmissao) >= 7 {
		mes = strings.ReplaceAll(d.DhEmissao[:7], "-", "")
	}
	dir := filepath.Join(m.Cfg.DataDir, "xml", mes)
	if os.MkdirAll(dir, 0o700) == nil {
		os.WriteFile(filepath.Join(dir, d.Chave+".xml"), []byte(d.XML), 0o600)
	}
}

// Revalidar reprocessa o XML armazenado (ex.: após atualizar parâmetros de MVA/UF).
func (m *Monitor) Revalidar(chave string, finalidades map[int]string) (*ResultadoImportacao, error) {
	d, err := m.Store.ObterDocumento(chave)
	if err != nil {
		return nil, err
	}
	if d.Resumo {
		return nil, errors.New("documento possui apenas resumo; baixe o XML completo")
	}
	return m.importar([]byte(d.XML), d.Origem, finalidades)
}

// ImportarArquivo aceita XML individual ou ZIP com vários XMLs.
func (m *Monitor) ImportarArquivo(nome string, dados []byte, origem string) []ResultadoImportacao {
	var out []ResultadoImportacao
	if strings.HasSuffix(strings.ToLower(nome), ".zip") || bytes.HasPrefix(dados, []byte("PK\x03\x04")) {
		zr, err := zip.NewReader(bytes.NewReader(dados), int64(len(dados)))
		if err != nil {
			return []ResultadoImportacao{{Arquivo: nome, Status: "ERRO", Mensagem: err.Error()}}
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
				continue
			}
			if f.UncompressedSize64 > 50<<20 {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			b, _ := io.ReadAll(io.LimitReader(rc, 50<<20))
			rc.Close()
			out = append(out, m.importarUm(f.Name, b, origem))
		}
		return out
	}
	return []ResultadoImportacao{m.importarUm(nome, dados, origem)}
}

func (m *Monitor) importarUm(nome string, b []byte, origem string) ResultadoImportacao {
	r, err := m.Importar(b, origem)
	if err != nil {
		return ResultadoImportacao{Arquivo: nome, Status: "ERRO", Mensagem: err.Error()}
	}
	r.Arquivo = nome
	r.Ocorrencias = nil
	return *r
}

// ProcessarPastaMonitorada importa XML/ZIP gravados pelo ERP na pasta configurada.
func (m *Monitor) ProcessarPastaMonitorada() int {
	dir := m.Cfg.PastaMonitorada
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		nome := strings.ToLower(e.Name())
		if e.IsDir() || !(strings.HasSuffix(nome, ".xml") || strings.HasSuffix(nome, ".zip")) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) < 3*time.Second {
			continue // arquivo possivelmente ainda sendo gravado
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		res := m.ImportarArquivo(e.Name(), b, "PASTA")
		destino := "processados"
		for _, r := range res {
			if r.Status == "ERRO" {
				destino = "erros"
				log.Printf("pasta monitorada: %s: %s", r.Arquivo, r.Mensagem)
			}
		}
		os.MkdirAll(filepath.Join(dir, destino), 0o700)
		os.Rename(p, filepath.Join(dir, destino, e.Name()))
		n += len(res)
	}
	return n
}

// ConsultarSituacao consulta a SEFAZ e atualiza o status do documento.
func (m *Monitor) ConsultarSituacao(ctx context.Context, chave string) (*sefaz.RetConsulta, error) {
	d, err := m.Store.ObterDocumento(chave)
	if err != nil {
		return nil, fmt.Errorf("documento não encontrado")
	}
	if d.EmpresaID == nil {
		return nil, errors.New("documento não pertence a nenhuma empresa cadastrada")
	}
	cli, _, err := m.ClienteSefaz(*d.EmpresaID)
	if err != nil {
		return nil, err
	}
	r, err := cli.ConsultaProtocolo(ctx, chave)
	if err != nil {
		return nil, err
	}
	st := StatusPorCStat(r.CStat, d.TpEmis)
	for _, e := range r.ProcEventos {
		if (e.RetEvento.InfEvento.TpEvento == "110111" || e.RetEvento.InfEvento.TpEvento == "110112") && e.RetEvento.InfEvento.CStat == 135 {
			st = StCancelada
		}
	}
	if TransicaoValida(d.Status, st) || d.Status == StResumo {
		m.Store.AtualizarSituacao(chave, st, r.CStat, r.XMotivo, r.Protocolo, r.DhAut)
		d.Status, d.CStat, d.XMotivo = st, r.CStat, r.XMotivo
		m.Hub.Publicar("documento.status", map[string]any{"chave": chave, "status": st, "c_stat": r.CStat, "x_motivo": r.XMotivo})
	}
	return r, nil
}

// Manifestar registra a manifestação do destinatário e, na ciência, tenta baixar o XML completo.
func (m *Monitor) Manifestar(ctx context.Context, chave, tpEvento, justificativa string) (*sefaz.RetEvento, error) {
	d, err := m.Store.ObterDocumento(chave)
	if err != nil {
		return nil, fmt.Errorf("documento não encontrado")
	}
	if d.EmpresaID == nil {
		return nil, errors.New("documento sem empresa destinatária cadastrada")
	}
	cli, emp, err := m.ClienteSefaz(*d.EmpresaID)
	if err != nil {
		return nil, err
	}
	r, err := cli.Manifestar(ctx, emp.CNPJ, chave, tpEvento, justificativa)
	if err != nil {
		return nil, err
	}
	if r.EventoCStat == 135 || r.EventoCStat == 136 || r.EventoCStat == 573 {
		desc := sefaz.EventosManifestacao[tpEvento]
		m.Store.SalvarEvento(store.Evento{Chave: chave, TpEvento: tpEvento, Descricao: desc, NSeq: 1, DhEvento: time.Now().Format(time.RFC3339), Protocolo: r.Protocolo, CStat: r.EventoCStat, XMotivo: r.EventoMotivo, XML: r.XMLEnviado})
		mapa := map[string]string{"210210": "CIENCIA", "210200": "CONFIRMADA", "210220": "DESCONHECIDA", "210240": "NAO_REALIZADA"}
		m.Store.AtualizarManifestacao(chave, mapa[tpEvento])
		m.Store.Auditar("sistema", "MANIFESTACAO", "documento", chave, tpEvento+" "+desc)
		if tpEvento == "210210" && d.Resumo {
			go m.baixarXMLCompleto(context.Background(), *d.EmpresaID, chave)
		}
	}
	return r, nil
}

func (m *Monitor) baixarXMLCompleto(ctx context.Context, empresaID int64, chave string) {
	time.Sleep(5 * time.Second) // a disponibilização após a ciência não é instantânea
	cli, emp, err := m.ClienteSefaz(empresaID)
	if err != nil {
		return
	}
	r, err := cli.DistDFePorChave(ctx, fiscal.CodigoPorUF[emp.UF], emp.CNPJ, chave)
	if err != nil {
		log.Printf("download XML %s: %v", chave, err)
		return
	}
	for _, d := range r.Docs {
		if len(d.XML) > 0 {
			m.Importar(d.XML, "DISTDFE")
		}
	}
}
