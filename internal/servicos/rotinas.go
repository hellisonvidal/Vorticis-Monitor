package servicos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/sefaz"
	"vorticis.com.br/monitor/internal/store"
	"vorticis.com.br/monitor/internal/xmlfiscal"
)

// NFeDaEmpresa indica se a NF-e pertence à empresa (emitente ou destinatária).
// Notas em que ela só aparece como transportadora ou em autXML são de terceiros.
// XMLs que não são NF-e (resumos, eventos) são aceitos.
func NFeDaEmpresa(xml []byte, daEmpresa func(doc string) bool) bool {
	d, err := xmlfiscal.Parse(xml)
	if err != nil || d.NFe == nil {
		return true
	}
	return daEmpresa(d.NFe.Emit.Doc()) || daEmpresa(d.NFe.Dest.Doc())
}

// LimparNotasDeTerceiros remove as NF-e baixadas da SEFAZ em que nenhuma empresa
// cadastrada é emitente ou destinatária (ex.: a empresa era só a transportadora).
func (m *Monitor) LimparNotasDeTerceiros() int {
	emps, _ := m.Store.ListarEmpresas()
	cnpjs := map[string]bool{}
	for _, e := range emps {
		cnpjs[e.CNPJ] = true
	}
	docs, _, err := m.Store.ListarDocumentos(store.FiltroDocumentos{Modelo: "NFE,NFCE", Todos: true, SemEmpresa: true, Origem: "DISTDFE"})
	if err != nil {
		return 0
	}
	n := 0
	for _, d := range docs {
		if cnpjs[d.EmitDoc] || cnpjs[d.DestDoc] {
			continue
		}
		full, err := m.Store.ObterDocumento(d.Chave)
		if err != nil {
			continue
		}
		x, err := xmlfiscal.Parse([]byte(full.XML))
		if err != nil || x.NFe == nil {
			continue
		}
		// só remove se uma empresa cadastrada aparece como transportadora ou em autXML
		terceiro := cnpjs[x.NFe.Transp.Transporta.CNPJ]
		for _, a := range x.NFe.AutXML {
			terceiro = terceiro || cnpjs[a.CNPJ]
		}
		if terceiro && m.Store.ExcluirDocumento(d.Chave) == nil {
			n++
		}
	}
	if n > 0 {
		log.Printf("removidas %d NF-e de terceiros (empresa só como transportadora/autXML)", n)
	}
	return n
}

// ---------------- Captura (Distribuição DF-e) ----------------

// ResultadoCaptura descreve um ciclo de captura.
type ResultadoCaptura struct {
	EmpresaID  int64  `json:"empresa_id"`
	Documentos int    `json:"documentos"`
	UltNSU     string `json:"ult_nsu"`
	MaxNSU     string `json:"max_nsu"`
	CStat      int    `json:"c_stat"`
	Motivo     string `json:"motivo"`
	Proxima    string `json:"proxima_consulta"`
	Ignorado   bool   `json:"ignorado"`
}

// CapturarDistDFe busca os documentos destinados à empresa no Ambiente Nacional.
//
// Regras para conviver com outros sistemas que usam o mesmo CNPJ (outro monitor
// de notas, ERP, contador): a SEFAZ controla o consumo por CNPJ, não por
// programa. Quando não há documentos novos (cStat 137) ou quando o limite é
// excedido (656 — consumo indevido), uma nova consulta só pode ser feita após
// 1 hora. Por isso:
//   - o intervalo nunca é menor que 60 min, nem mesmo em "Buscar agora";
//   - em 656, o monitor aguarda 1 h (mais tempo se o bloqueio se repetir);
//   - se o 656 vier sem que ESTE monitor tenha consultado na última hora, outro
//     sistema está consultando o CNPJ: o intervalo passa a ser no mínimo 2 h,
//     com uma folga aleatória, para não disputar a mesma janela.
func (m *Monitor) CapturarDistDFe(ctx context.Context, empresaID int64, forcar bool) (*ResultadoCaptura, error) {
	if m.Cfg.NaoBuscarNFe {
		return &ResultadoCaptura{EmpresaID: empresaID, Ignorado: true, Motivo: "Busca de NF-e/NFC-e desativada em Configurações › Monitoramento"}, nil
	}
	return m.capturarDist(ctx, alvoDist{
		chaveNSU: empresaID, servico: "NFE_DISTDFE",
		abrir: func() (*sefaz.Cliente, string, string, error) {
			cli, emp, err := m.ClienteSefaz(empresaID)
			if err != nil {
				return nil, "", "", err
			}
			return cli, fiscal.CodigoPorUF[emp.UF], emp.CNPJ, nil
		},
		// A SEFAZ também entrega as NF-e em que a empresa é só transportadora ou
		// autorizada a obter o XML: essas não são da empresa e ficam de fora.
		aceitar: func(xml []byte) bool {
			emp, err := m.Store.ObterEmpresa(empresaID)
			return err != nil || NFeDaEmpresa(xml, func(doc string) bool { return doc == emp.CNPJ })
		},
		aposImportar: func(d sefaz.DocZip, ir *ResultadoImportacao) {
			emp, err := m.Store.ObterEmpresa(empresaID)
			if err == nil && strings.HasPrefix(d.Schema, "resNFe") && emp.CienciaAutomatica && ir.Status == StResumo {
				if _, err := m.Manifestar(ctx, ir.Chave, "210210", ""); err != nil {
					log.Printf("ciência automática %s: %v", ir.Chave, err)
				}
			}
		},
	}, forcar)
}

// alvoDist parametriza a Distribuição DF-e: a própria empresa ou um procurador
// (contador) que consulta as NF-e em que consta como autorizado (autXML).
type alvoDist struct {
	chaveNSU     int64  // empresa (positivo) ou procurador (negativo) no controle de NSU
	servico      string // NFE_DISTDFE | NFE_AUTXML
	abrir        func() (cli *sefaz.Cliente, cUF, doc string, err error)
	aceitar      func(xml []byte) bool // nil = importa tudo
	aposImportar func(d sefaz.DocZip, ir *ResultadoImportacao)
}

// IntervaloNFe é o intervalo do monitoramento da NF-e na SEFAZ (mínimo de 60 min — regra da SEFAZ).
func (m *Monitor) IntervaloNFe() time.Duration {
	return time.Duration(max(60, m.Cfg.IntervaloDistDFeMin)) * time.Minute
}

// IntervaloNFSe é o intervalo do monitoramento da NFS-e no Ambiente Nacional (mínimo de 10 min).
func (m *Monitor) IntervaloNFSe() time.Duration {
	return time.Duration(max(10, m.Cfg.IntervaloNFSeMin)) * time.Minute
}

// antecipar indica que o intervalo configurado já passou desde a última consulta
// (ex.: o usuário reduziu o intervalo), salvo quando o servidor exigiu a espera
// ou a última tentativa falhou (cStat 0: aguarda os 15 min de nova tentativa).
func antecipar(n store.NSU, intervalo time.Duration, bloqueios ...int) bool {
	for _, c := range bloqueios {
		if n.UltimoCStat == c {
			return false
		}
	}
	t, err := time.Parse(time.RFC3339, n.UltimaConsulta)
	return err == nil && time.Since(t) >= intervalo
}

func (m *Monitor) capturarDist(ctx context.Context, a alvoDist, forcar bool) (*ResultadoCaptura, error) {
	n := m.Store.ObterNSU(a.chaveNSU, a.servico)
	res := &ResultadoCaptura{EmpresaID: a.chaveNSU, UltNSU: n.UltNSU, Proxima: n.ProximaConsulta}
	intervalo := m.IntervaloNFe()
	if n.Concorrente && intervalo < 120*time.Minute {
		intervalo = 120 * time.Minute
	}
	if t, err := time.Parse(time.RFC3339, n.ProximaConsulta); err == nil && time.Now().Before(t) && !antecipar(n, intervalo, 0, 656) {
		// "Buscar agora" só antecipa a consulta quando a espera não é exigida pela SEFAZ.
		if !forcar || n.UltimoCStat == 137 || n.UltimoCStat == 656 {
			res.Ignorado = true
			res.Motivo = "A SEFAZ só permite nova consulta deste CNPJ a partir de " + t.Format("02/01 15:04") +
				" (regra de 1 hora; consultar antes gera bloqueio por consumo indevido)"
			return res, nil
		}
	}
	cli, cUF, docInteressado, err := a.abrir()
	if err != nil {
		// Certificado indisponível: registra e tenta de novo em 15 min (o monitoramento
		// automático verifica a cada minuto e não deve repetir o erro a cada volta).
		n.ProximaConsulta = time.Now().Add(15 * time.Minute).Format(time.RFC3339)
		n.UltimoCStat, n.UltimoMotivo = 0, err.Error()
		m.Store.SalvarNSU(n)
		return nil, err
	}
	folga := func() time.Duration {
		if !n.Concorrente {
			return 0
		}
		return time.Duration(rand.IntN(8)) * time.Minute
	}
	for i := 0; i < 20; i++ {
		anterior := n.UltimaConsulta
		n.UltimaConsulta = time.Now().Format(time.RFC3339)
		r, err := cli.DistDFePorNSU(ctx, cUF, docInteressado, n.UltNSU)
		if err != nil {
			n.ProximaConsulta = time.Now().Add(15 * time.Minute).Format(time.RFC3339)
			n.UltimoCStat, n.UltimoMotivo = 0, err.Error()
			m.Store.SalvarNSU(n)
			return nil, err
		}
		n.UltimoCStat, n.UltimoMotivo, res.CStat, res.Motivo = r.CStat, r.XMotivo, r.CStat, r.XMotivo
		if r.CStat == 656 {
			n.Bloqueios++
			t, errT := time.Parse(time.RFC3339, anterior)
			if i == 0 && (errT != nil || time.Since(t) >= 59*time.Minute) {
				n.Concorrente = true
			}
			espera := time.Duration(min(n.Bloqueios, 3)) * 61 * time.Minute
			if n.Concorrente && espera < 120*time.Minute {
				espera = 120 * time.Minute
			}
			n.ProximaConsulta = time.Now().Add(espera + folga()).Format(time.RFC3339)
			res.Motivo = r.XMotivo + " — nova tentativa automática em " + time.Now().Add(espera).Format("15:04")
			if n.Concorrente {
				res.Motivo += ". Outro sistema está consultando este CNPJ na SEFAZ (ex.: outro monitor de notas); o intervalo foi ampliado para evitar novos bloqueios"
			}
			break
		}
		n.Bloqueios = 0
		if r.UltNSU != "" {
			n.UltNSU = r.UltNSU
		}
		n.MaxNSU = r.MaxNSU
		for _, d := range r.Docs {
			if len(d.XML) == 0 || (a.aceitar != nil && !a.aceitar(d.XML)) {
				continue
			}
			ir, err := m.Importar(d.XML, "DISTDFE")
			if err != nil {
				log.Printf("distdfe NSU %s (%s): %v", d.NSU, d.Schema, err)
				continue
			}
			res.Documentos++
			if a.aposImportar != nil {
				a.aposImportar(d, ir)
			}
		}
		if r.CStat != 138 || n.UltNSU == n.MaxNSU {
			n.ProximaConsulta = time.Now().Add(intervalo + folga()).Format(time.RFC3339)
			break
		}
		// cStat 138 com mais documentos: segue no mesmo ciclo, com pausa curta.
		n.ProximaConsulta = ""
		select {
		case <-ctx.Done():
			i = 20
		case <-time.After(2 * time.Second):
		}
	}
	m.Store.SalvarNSU(n)
	res.UltNSU, res.MaxNSU, res.Proxima = n.UltNSU, n.MaxNSU, n.ProximaConsulta
	m.Hub.Publicar("captura.ciclo", res)
	return res, nil
}

// CapturarNFeProcurador busca, com o certificado do procurador (contador), as NF-e
// em que ele consta como autorizado a obter o XML (grupo autXML). Só importa
// documentos de empresas cadastradas no monitor (emitente ou destinatário).
func (m *Monitor) CapturarNFeProcurador(ctx context.Context, procID int64, forcar bool) (*ResultadoCaptura, error) {
	if m.Cfg.NaoBuscarNFe {
		return &ResultadoCaptura{EmpresaID: -procID, Ignorado: true, Motivo: "Busca de NF-e/NFC-e desativada em Configurações › Monitoramento"}, nil
	}
	p, err := m.Store.ObterProcurador(procID)
	if err != nil {
		return nil, fmt.Errorf("procurador %d não encontrado", procID)
	}
	return m.capturarDist(ctx, alvoDist{
		chaveNSU: -procID, servico: "NFE_AUTXML",
		abrir: func() (*sefaz.Cliente, string, string, error) {
			if p.UF == "" || fiscal.CodigoPorUF[p.UF] == "" {
				return nil, "", "", errors.New("informe a UF do procurador em Configurações › Procuradores")
			}
			cli, err := m.ClienteProcurador(p, 1)
			if err != nil {
				return nil, "", "", err
			}
			return cli, fiscal.CodigoPorUF[p.UF], p.Documento, nil
		},
		aceitar: func(xml []byte) bool {
			emps, _ := m.Store.ListarEmpresas()
			cadastrada := func(doc string) bool {
				for _, e := range emps {
					if e.Ativo && e.CNPJ == doc {
						return true
					}
				}
				return false
			}
			if d, err := xmlfiscal.Parse(xml); err == nil && d.NFe != nil {
				return NFeDaEmpresa(xml, cadastrada)
			}
			// eventos e resumos: CNPJ de empresa cadastrada embutido na chave
			doc := string(xml)
			for _, e := range emps {
				if e.Ativo && strings.Contains(doc, "<chNFe>") && strings.Contains(doc, e.CNPJ) {
					return true
				}
			}
			return false
		},
	}, forcar)
}

// CapturarNFSe busca as NFS-e do Padrão Nacional (prestadas e tomadas) no ADN.
// Com procuração, usa o certificado do procurador e informa o CNPJ da empresa
// em cnpjConsulta. Sem documentos novos, consulta de novo após o intervalo
// configurado; em HTTP 429 respeita o Retry-After.
func (m *Monitor) CapturarNFSe(ctx context.Context, empresaID int64, forcar bool) (*ResultadoCaptura, error) {
	if m.Cfg.NaoBuscarNFSe {
		return &ResultadoCaptura{EmpresaID: empresaID, Ignorado: true, Motivo: "Busca de NFS-e desativada em Configurações › Monitoramento"}, nil
	}
	n := m.Store.ObterNSU(empresaID, "NFSE_ADN")
	if n.UltNSU == "" || strings.Trim(n.UltNSU, "0") == "" {
		n.UltNSU = "0"
	}
	res := &ResultadoCaptura{EmpresaID: empresaID, UltNSU: n.UltNSU, Proxima: n.ProximaConsulta}
	intervalo := m.IntervaloNFSe()
	if t, err := time.Parse(time.RFC3339, n.ProximaConsulta); err == nil && time.Now().Before(t) && (!forcar || n.UltimoCStat == 429) && !antecipar(n, intervalo, 0, 429) {
		res.Ignorado = true
		res.Motivo = "Próxima consulta de NFS-e permitida a partir de " + t.Format("02/01 15:04")
		return res, nil
	}
	cli, emp, via, err := m.ClienteNFSe(empresaID)
	if err != nil {
		n.ProximaConsulta = time.Now().Add(15 * time.Minute).Format(time.RFC3339)
		n.UltimoCStat, n.UltimoMotivo = 0, err.Error()
		m.Store.SalvarNSU(n)
		return nil, err
	}
	salvar := func(prox time.Duration, cstat int, motivo string) {
		n.ProximaConsulta = time.Now().Add(prox).Format(time.RFC3339)
		n.UltimoCStat, n.UltimoMotivo = cstat, motivo+" · via "+via
		res.CStat, res.Motivo = cstat, n.UltimoMotivo
	}
	for pagina := 0; pagina < 20; pagina++ {
		n.UltimaConsulta = time.Now().Format(time.RFC3339)
		r, err := cli.DistNFSe(ctx, n.UltNSU, emp.CNPJ)
		if err != nil {
			salvar(15*time.Minute, 0, err.Error())
			m.Store.SalvarNSU(n)
			return nil, err
		}
		if r.HTTP == 429 {
			espera := r.RetryAfter
			if espera < 5*time.Minute {
				espera = 30 * time.Minute
			}
			salvar(espera, 429, r.Mensagem())
			break
		}
		if r.NegadoAcess || len(r.Erros) > 0 {
			salvar(intervalo, r.HTTP, r.Mensagem())
			break
		}
		maior := int64(0)
		for _, it := range r.Itens {
			if v, err := it.NSU.Int64(); err == nil && v > maior {
				maior = v
			}
			tipo := strings.ToUpper(it.TipoDocumento)
			if len(it.XML) == 0 || tipo == "DPS" || tipo == "CNC" || tipo == "NENHUM" {
				continue
			}
			if tipo == "EVENTO" || it.TipoEvento != "" || (!bytes.Contains(it.XML, []byte("<NFSe")) && bytes.Contains(it.XML, []byte("<evento"))) {
				if it.ChaveAcesso != "" && (strings.Contains(strings.ToUpper(it.TipoEvento), "CANCEL") || bytes.Contains(it.XML, []byte("e101101")) || bytes.Contains(it.XML, []byte("e105102"))) {
					if m.Store.AtualizarSituacao(it.ChaveAcesso, StCancelada, 0, "Cancelamento registrado no Ambiente Nacional da NFS-e", "", "") == nil {
						m.Hub.Publicar("documento.status", map[string]string{"chave": it.ChaveAcesso, "status": StCancelada})
					}
				}
				continue
			}
			if _, err := m.Importar(it.XML, "ADN"); err != nil {
				log.Printf("NFS-e ADN NSU %s: %v", it.NSU, err)
				continue
			}
			res.Documentos++
		}
		if maior > 0 {
			n.UltNSU = strconv.FormatInt(maior, 10)
		}
		if r.SemNovos || len(r.Itens) < 50 || maior == 0 {
			salvar(intervalo, 200, fmt.Sprintf("%d NFS-e recebida(s)", res.Documentos))
			if r.SemNovos && res.Documentos == 0 {
				salvar(intervalo, 404, "nenhuma NFS-e nova no Ambiente Nacional")
			}
			break
		}
		n.ProximaConsulta = ""
		select {
		case <-ctx.Done():
			pagina = 20
		case <-time.After(1500 * time.Millisecond):
		}
	}
	m.Store.SalvarNSU(n)
	res.UltNSU, res.Proxima = n.UltNSU, n.ProximaConsulta
	m.Hub.Publicar("captura.ciclo", res)
	return res, nil
}

// ConsultarPendentes atualiza a situação de documentos emitidos sem retorno.
func (m *Monitor) ConsultarPendentes(ctx context.Context, limite int) int {
	docs, _, _ := m.Store.ListarDocumentos(store.FiltroDocumentos{Status: "PENDENTE,CONTINGENCIA", Direcao: "S", PorPagina: limite})
	n := 0
	for _, d := range docs {
		if d.EmpresaID == nil || (d.Modelo != "NFE" && d.Modelo != "NFCE") {
			continue
		}
		if _, err := m.ConsultarSituacao(ctx, d.Chave); err == nil {
			n++
		} else if err == ErrSemCertificado {
			continue
		}
	}
	return n
}

// ---------------- Conciliação XML x Pedido de Compra ----------------

type Divergencia struct {
	Tipo      string  `json:"tipo"`
	NItem     int     `json:"n_item,omitempty"`
	ItemPed   string  `json:"item_pedido,omitempty"`
	Produto   string  `json:"produto,omitempty"`
	Mensagem  string  `json:"mensagem"`
	Esperado  float64 `json:"esperado,omitempty"`
	Informado float64 `json:"informado,omitempty"`
	Bloqueia  bool    `json:"bloqueia"`
}

// Conciliar cruza o XML de entrada com o pedido de compra (e ordem de recebimento informada).
func (m *Monitor) Conciliar(chave, ordemRecebimento string) (*store.Conciliacao, error) {
	d, err := m.Store.ObterDocumento(chave)
	if err != nil {
		return nil, fmt.Errorf("documento não encontrado")
	}
	if d.Direcao != "E" || d.EmpresaID == nil || d.Modelo != "NFE" {
		return nil, fmt.Errorf("conciliação aplica-se a NF-e de entrada de empresa cadastrada")
	}
	if d.Resumo {
		return nil, fmt.Errorf("documento possui apenas resumo; manifeste ciência e baixe o XML")
	}
	itens, _ := m.Store.Itens(d.ID)
	numPed := ""
	for _, i := range itens {
		if i.PedCompra != "" {
			numPed = i.PedCompra
			break
		}
	}
	var peds []*store.Pedido
	if numPed != "" {
		peds, _ = m.Store.Pedidos(*d.EmpresaID, "", numPed, false)
	}
	if len(peds) == 0 {
		peds, _ = m.Store.Pedidos(*d.EmpresaID, d.EmitDoc, "", true)
	}
	c := &store.Conciliacao{DocumentoID: d.ID, Chave: chave, OrdemRecebimento: ordemRecebimento}
	var divs []Divergencia
	if len(peds) == 0 {
		c.Status = "SEM_PEDIDO"
		divs = append(divs, Divergencia{Tipo: "SEM_PEDIDO", Mensagem: "Nenhum pedido de compra aberto para o fornecedor " + d.EmitDoc, Bloqueia: true})
		c.Divergencias = divs
		return c, m.Store.SalvarConciliacao(c)
	}
	p := peds[0]
	c.PedidoID, c.PedidoNumero = p.ID, p.Numero
	if fiscal.NormalizaCNPJ(p.FornecedorCNPJ) != fiscal.NormalizaCNPJ(d.EmitDoc) {
		divs = append(divs, Divergencia{Tipo: "FORNECEDOR_DIVERGENTE", Mensagem: "CNPJ do emitente difere do fornecedor do pedido " + p.Numero, Bloqueia: true})
	}
	usados := map[int64]float64{}
	finalidades := map[int]string{}
	tolP, tolQ := m.Cfg.ToleranciaPrecoPct, m.Cfg.ToleranciaQtdPct
	for _, it := range itens {
		pi := casarItem(p.Itens, it)
		if pi == nil {
			divs = append(divs, Divergencia{Tipo: "ITEM_NAO_PEDIDO", NItem: it.NItem, Produto: it.CodProd + " " + it.Descricao, Mensagem: "Item do XML não localizado no pedido", Informado: it.Quantidade, Bloqueia: true})
			continue
		}
		finalidades[it.NItem] = pi.Finalidade
		saldo := pi.Quantidade - pi.QuantidadeRecebida - usados[pi.ID]
		if it.Quantidade > saldo*(1+tolQ/100)+0.0001 {
			divs = append(divs, Divergencia{Tipo: "QUANTIDADE_EXCEDE_SALDO", NItem: it.NItem, ItemPed: pi.Item, Produto: it.Descricao, Mensagem: "Quantidade faturada excede o saldo do pedido", Esperado: saldo, Informado: it.Quantidade, Bloqueia: true})
		}
		usados[pi.ID] += it.Quantidade
		if pi.ValorUnitario > 0 {
			dif := (it.ValorUnit - pi.ValorUnitario) / pi.ValorUnitario * 100
			if math.Abs(dif) > tolP {
				divs = append(divs, Divergencia{Tipo: "PRECO_DIVERGENTE", NItem: it.NItem, ItemPed: pi.Item, Produto: it.Descricao, Mensagem: fmt.Sprintf("Preço unitário %.2f%% %s do pedido", math.Abs(dif), map[bool]string{true: "acima", false: "abaixo"}[dif > 0]), Esperado: pi.ValorUnitario, Informado: it.ValorUnit, Bloqueia: dif > 0})
			}
		}
		if pi.NCM != "" && pi.NCM != it.NCM {
			divs = append(divs, Divergencia{Tipo: "NCM_DIVERGENTE", NItem: it.NItem, ItemPed: pi.Item, Produto: it.Descricao, Mensagem: "NCM do XML (" + it.NCM + ") difere do cadastro do pedido (" + pi.NCM + ")"})
		}
	}
	for _, pi := range p.Itens {
		if usados[pi.ID] == 0 && pi.Quantidade-pi.QuantidadeRecebida > 0 {
			divs = append(divs, Divergencia{Tipo: "PENDENTE_ENTREGA", ItemPed: pi.Item, Produto: pi.Descricao, Mensagem: "Item do pedido não faturado nesta nota (entrega parcial)", Esperado: pi.Quantidade - pi.QuantidadeRecebida})
		}
	}
	c.Status = "CONCILIADO"
	for _, dv := range divs {
		if dv.Bloqueia {
			c.Status = "DIVERGENTE"
		}
	}
	if divs == nil {
		divs = []Divergencia{}
	}
	c.Divergencias = divs
	var jaConciliado string
	m.Store.DB.QueryRow(`SELECT status FROM conciliacao WHERE documento_id = ?`, d.ID).Scan(&jaConciliado)
	if err := m.Store.SalvarConciliacao(c); err != nil {
		return nil, err
	}
	if c.Status == "CONCILIADO" && jaConciliado != "CONCILIADO" {
		for id, q := range usados {
			m.Store.RegistrarRecebimento(id, q)
		}
		m.Store.AtualizarStatusPedido(p.ID)
	}
	// Revalida com a finalidade de cada item (DIFAL de uso/consumo e ativo).
	m.Revalidar(chave, finalidades)
	m.Hub.Publicar("documento.status", map[string]any{"chave": chave, "conciliacao": c.Status})
	return c, nil
}

func casarItem(itens []store.PedidoItem, it store.Item) *store.PedidoItem {
	for i := range itens {
		if it.ItemPed != "" && itens[i].Item == it.ItemPed {
			return &itens[i]
		}
	}
	for i := range itens {
		if itens[i].CodProdutoFornecedor != "" && itens[i].CodProdutoFornecedor == it.CodProd {
			return &itens[i]
		}
	}
	var melhor *store.PedidoItem
	menor := math.MaxFloat64
	for i := range itens {
		if itens[i].NCM == it.NCM && itens[i].NCM != "" {
			if d := math.Abs(itens[i].ValorUnitario - it.ValorUnit); d < menor {
				menor, melhor = d, &itens[i]
			}
		}
	}
	return melhor
}

// ---------------- Agendador ----------------

// Iniciar executa as rotinas periódicas até o contexto ser cancelado.
func (m *Monitor) Iniciar(ctx context.Context) {
	intervalo := time.Duration(max(15, m.Cfg.IntervaloVarreduraSeg)) * time.Second
	t := time.NewTicker(intervalo)
	defer t.Stop()
	m.LimparNotasDeTerceiros()
	ciclo := 0
	for {
		m.executarCiclo(ctx, ciclo)
		ciclo++
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (m *Monitor) executarCiclo(ctx context.Context, ciclo int) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("rotina: pânico recuperado: %v", r)
		}
	}()
	m.ProcessarPastaMonitorada()
	m.BackupSeVencido()
	if ciclo%5 == 0 && m.Cfg.ConsultarPendentesSefaz {
		m.ConsultarPendentes(ctx, 50)
	}
	// Monitoramento automático: verificado a cada varredura (1 min); cada empresa só
	// é consultada quando chega a hora definida em Configurações (proxima_consulta).
	if m.Cfg.MonitoramentoAutomatico {
		emps, _ := m.Store.ListarEmpresas()
		for _, e := range emps {
			if !m.Cfg.NaoBuscarNFe && e.Ativo && e.CapturaAutomatica && e.TemCertificado {
				if _, err := m.CapturarDistDFe(ctx, e.ID, false); err != nil {
					log.Printf("captura DF-e %s: %v", e.CNPJ, err)
				}
			}
			if !m.Cfg.NaoBuscarNFSe && e.Ativo && e.CapturaAutomatica && (e.TemCertificado || (e.NFSeModo == "PROCURACAO" && e.NFSeProcuradorID > 0)) {
				if _, err := m.CapturarNFSe(ctx, e.ID, false); err != nil {
					log.Printf("captura NFS-e %s: %v", e.CNPJ, err)
				}
			}
		}
		procs, _ := m.Store.ListarProcuradores()
		for _, p := range procs {
			if !m.Cfg.NaoBuscarNFe && p.NFeAutXML && p.UF != "" {
				if _, err := m.CapturarNFeProcurador(ctx, p.ID, false); err != nil {
					log.Printf("captura NF-e autXML do procurador %s: %v", p.Nome, err)
				}
			}
		}
	}
}
