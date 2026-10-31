package servicos

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/store"
)

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

// CapturarDistDFe busca os documentos destinados à empresa no Ambiente Nacional,
// respeitando ultNSU e o intervalo mínimo de 1 hora após cStat 137/656.
func (m *Monitor) CapturarDistDFe(ctx context.Context, empresaID int64, forcar bool) (*ResultadoCaptura, error) {
	n := m.Store.ObterNSU(empresaID, "NFE_DISTDFE")
	res := &ResultadoCaptura{EmpresaID: empresaID, UltNSU: n.UltNSU, Proxima: n.ProximaConsulta}
	if !forcar && n.ProximaConsulta != "" {
		if t, err := time.Parse(time.RFC3339, n.ProximaConsulta); err == nil && time.Now().Before(t) {
			res.Ignorado, res.Motivo = true, "aguardando intervalo mínimo da SEFAZ"
			return res, nil
		}
	}
	cli, emp, err := m.ClienteSefaz(empresaID)
	if err != nil {
		return nil, err
	}
	intervalo := time.Duration(max(60, m.Cfg.IntervaloDistDFeMin)) * time.Minute
	for i := 0; i < 20; i++ {
		r, err := cli.DistDFePorNSU(ctx, fiscal.CodigoPorUF[emp.UF], emp.CNPJ, n.UltNSU)
		if err != nil {
			n.ProximaConsulta = time.Now().Add(15 * time.Minute).Format(time.RFC3339)
			n.UltimoMotivo = err.Error()
			m.Store.SalvarNSU(n)
			return nil, err
		}
		n.UltimoCStat, n.UltimoMotivo, res.CStat, res.Motivo = r.CStat, r.XMotivo, r.CStat, r.XMotivo
		if r.UltNSU != "" {
			n.UltNSU = r.UltNSU
		}
		n.MaxNSU = r.MaxNSU
		for _, d := range r.Docs {
			if len(d.XML) == 0 {
				continue
			}
			ir, err := m.Importar(d.XML, "DISTDFE")
			if err != nil {
				log.Printf("distdfe NSU %s (%s): %v", d.NSU, d.Schema, err)
				continue
			}
			res.Documentos++
			if strings.HasPrefix(d.Schema, "resNFe") && emp.CienciaAutomatica && ir.Status == StResumo {
				if _, err := m.Manifestar(ctx, ir.Chave, "210210", ""); err != nil {
					log.Printf("ciência automática %s: %v", ir.Chave, err)
				}
			}
		}
		if r.CStat == 137 || r.CStat == 656 || n.UltNSU == n.MaxNSU || r.CStat != 138 {
			n.ProximaConsulta = time.Now().Add(intervalo).Format(time.RFC3339)
			break
		}
	}
	m.Store.SalvarNSU(n)
	res.UltNSU, res.MaxNSU, res.Proxima = n.UltNSU, n.MaxNSU, n.ProximaConsulta
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
	if ciclo%5 == 0 {
		if m.Cfg.ConsultarPendentesSefaz {
			m.ConsultarPendentes(ctx, 50)
		}
		emps, _ := m.Store.ListarEmpresas()
		for _, e := range emps {
			if e.Ativo && e.CapturaAutomatica && e.TemCertificado {
				if _, err := m.CapturarDistDFe(ctx, e.ID, false); err != nil {
					log.Printf("captura DF-e %s: %v", e.CNPJ, err)
				}
			}
		}
	}
}
