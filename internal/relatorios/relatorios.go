package relatorios

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/store"
	"vorticis.com.br/monitor/internal/xmlfiscal"
)

// Catalogo descreve os relatórios disponíveis (exibido no painel).
var Catalogo = []map[string]string{
	{"id": "livro-entradas-saidas", "nome": "Livro de Entradas e Saídas (resumo por CFOP/CST)", "formato": "csv", "uso": "Conferência com a escrituração e com o registro C190"},
	{"id": "apuracao-icms", "nome": "Apuração ICMS, ICMS-ST e DIFAL por UF", "formato": "csv", "uso": "Conferência do bloco E (E110/E210/E310) e guias GNRE"},
	{"id": "pis-cofins", "nome": "PIS/COFINS por CST e regime", "formato": "csv", "uso": "Conferência dos blocos C/M da EFD-Contribuições"},
	{"id": "retencoes", "nome": "Retenções na fonte de serviços (ISS, IRRF, PCC, INSS)", "formato": "csv", "uso": "EFD-Reinf R-2010/R-4020 e DCTFWeb"},
	{"id": "inconsistencias", "nome": "Inconsistências fiscais por regra", "formato": "csv", "uso": "Plano de correção de cadastros (NCM, CEST, CST, CFOP)"},
	{"id": "rejeicoes", "nome": "Rejeições e denegações SEFAZ", "formato": "csv", "uso": "Indicador de qualidade de emissão"},
	{"id": "manifestacao", "nome": "Pendências de manifestação do destinatário", "formato": "csv", "uso": "Evitar perda de prazo e notas frias"},
	{"id": "contingencia", "nome": "Documentos emitidos em contingência", "formato": "csv", "uso": "Auditoria de SVC-AN/SVC-RS/FS-DA/off-line"},
	{"id": "conciliacao", "nome": "Conciliação XML × Pedido de Compra", "formato": "csv", "uso": "Recebimento fiscal/compras"},
	{"id": "efd-icms-ipi", "nome": "SPED EFD ICMS/IPI (pré-escrituração)", "formato": "txt", "uso": "Importar no sistema de escrituração / validar no PVA"},
	{"id": "efd-contribuicoes", "nome": "SPED EFD-Contribuições (pré-escrituração)", "formato": "txt", "uso": "Importar no sistema de escrituração / validar no PVA"},
	{"id": "xml-lote", "nome": "Lote de XML e DANFE (ZIP)", "formato": "zip", "uso": "Envio ao contador / guarda legal de 5 anos"},
}

func novoCSV(w io.Writer) *csv.Writer {
	w.Write([]byte("\xef\xbb\xbf")) // BOM para Excel
	c := csv.NewWriter(w)
	c.Comma = ';'
	return c
}

func f2(x float64) string { return strings.Replace(strconv.FormatFloat(x, 'f', 2, 64), ".", ",", 1) }

// LivroEntradasSaidas: resumo por direção/CFOP/CST/alíquota.
func LivroEntradasSaidas(w io.Writer, s *store.Store, f store.FiltroDocumentos) error {
	docs, err := s.DocumentosComXML(f)
	if err != nil {
		return err
	}
	type k struct {
		dir, cfop, cst string
		aliq           float64
	}
	ag := map[k]*[7]float64{}
	for _, d := range docs {
		if d.Status != "AUTORIZADA" || d.Modelo == "NFSE" || d.Resumo {
			continue
		}
		its, _ := s.Itens(d.ID)
		for _, i := range its {
			cf := i.CFOP
			if d.Direcao == "E" && d.EmitDoc != "" {
				cf = cfopEntrada(cf)
			}
			kk := k{d.Direcao, cf, i.CSTICMS, i.PICMS}
			if ag[kk] == nil {
				ag[kk] = &[7]float64{}
			}
			a := ag[kk]
			a[0] += i.ValorTotal - i.ValorDesconto + i.VICMSST + i.VIPI
			a[1] += i.VBC
			a[2] += i.VICMS
			a[3] += i.VBCST
			a[4] += i.VICMSST
			a[5] += i.VIPI
			a[6]++
		}
	}
	keys := make([]k, 0, len(ag))
	for kk := range ag {
		keys = append(keys, kk)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].dir != keys[j].dir {
			return keys[i].dir < keys[j].dir
		}
		return keys[i].cfop+keys[i].cst < keys[j].cfop+keys[j].cst
	})
	c := novoCSV(w)
	c.Write([]string{"Movimento", "CFOP", "CST ICMS", "Alíq. ICMS", "Valor Contábil", "Base ICMS", "ICMS", "Base ST", "ICMS-ST", "IPI", "Itens"})
	for _, kk := range keys {
		a := ag[kk]
		c.Write([]string{map[string]string{"E": "Entrada", "S": "Saída"}[kk.dir], kk.cfop, kk.cst, f2(kk.aliq), f2(a[0]), f2(a[1]), f2(a[2]), f2(a[3]), f2(a[4]), f2(a[5]), strconv.Itoa(int(a[6]))})
	}
	c.Flush()
	return c.Error()
}

// ApuracaoICMS: débitos/créditos de ICMS, ST e DIFAL por UF.
func ApuracaoICMS(w io.Writer, s *store.Store, f store.FiltroDocumentos) error {
	docs, err := s.DocumentosComXML(f)
	if err != nil {
		return err
	}
	type tot struct {
		debICMS, credICMS, st, difal, fcp float64
		n                                 int
	}
	ag := map[string]*tot{}
	for _, d := range docs {
		if d.Status != "AUTORIZADA" || d.Resumo || d.Modelo == "NFSE" {
			continue
		}
		uf := d.DestUF
		if d.Direcao == "E" {
			uf = d.EmitUF
		}
		if ag[uf] == nil {
			ag[uf] = &tot{}
		}
		t := ag[uf]
		t.n++
		if d.Direcao == "S" {
			t.debICMS += d.ValorICMS
			t.st += d.ValorST
			t.difal += d.ValorDIFAL
			its, _ := s.Itens(d.ID)
			for _, i := range its {
				t.fcp += i.VFCPDest
			}
		} else {
			t.credICMS += d.ValorICMS
		}
	}
	ufs := make([]string, 0, len(ag))
	for u := range ag {
		ufs = append(ufs, u)
	}
	sort.Strings(ufs)
	c := novoCSV(w)
	c.Write([]string{"UF", "Documentos", "Débito ICMS (saídas)", "Crédito ICMS (entradas)", "ICMS-ST retido", "DIFAL UF destino", "FCP destino"})
	for _, u := range ufs {
		t := ag[u]
		c.Write([]string{u, strconv.Itoa(t.n), f2(t.debICMS), f2(t.credICMS), f2(t.st), f2(t.difal), f2(t.fcp)})
	}
	c.Flush()
	return c.Error()
}

// PISCOFINS por direção e CST.
func PISCOFINS(w io.Writer, s *store.Store, f store.FiltroDocumentos) error {
	docs, err := s.DocumentosComXML(f)
	if err != nil {
		return err
	}
	type k struct{ dir, cst string }
	ag := map[k]*[3]float64{}
	for _, d := range docs {
		if d.Status != "AUTORIZADA" || d.Resumo || d.Modelo == "NFSE" {
			continue
		}
		its, _ := s.Itens(d.ID)
		for _, i := range its {
			kk := k{d.Direcao, i.CSTPIS}
			if ag[kk] == nil {
				ag[kk] = &[3]float64{}
			}
			ag[kk][0] += i.ValorTotal
			ag[kk][1] += i.VPIS
			ag[kk][2] += i.VCOFINS
		}
	}
	c := novoCSV(w)
	c.Write([]string{"Movimento", "CST PIS/COFINS", "Valor dos itens", "PIS", "COFINS"})
	keys := make([]k, 0, len(ag))
	for kk := range ag {
		keys = append(keys, kk)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].dir+keys[i].cst < keys[j].dir+keys[j].cst })
	for _, kk := range keys {
		a := ag[kk]
		c.Write([]string{map[string]string{"E": "Entrada", "S": "Saída"}[kk.dir], kk.cst, f2(a[0]), f2(a[1]), f2(a[2])})
	}
	c.Flush()
	return c.Error()
}

// Retencoes lista as NFS-e com retenções (base para EFD-Reinf).
func Retencoes(w io.Writer, s *store.Store, f store.FiltroDocumentos) error {
	f.Modelo = "NFSE"
	docs, err := s.DocumentosComXML(f)
	if err != nil {
		return err
	}
	c := novoCSV(w)
	c.Write([]string{"Movimento", "Número", "Emissão", "Prestador", "Tomador", "Padrão", "Serviço", "Mun. incidência", "Valor serviços", "ISS", "ISS retido", "IRRF", "PIS", "COFINS", "CSLL", "INSS", "Líquido"})
	for _, d := range docs {
		x, err := xmlfiscal.Parse([]byte(d.XML))
		if err != nil || x.NFSe == nil {
			continue
		}
		n := x.NFSe
		c.Write([]string{map[string]string{"E": "Tomado", "S": "Prestado"}[d.Direcao], n.Numero, d.DhEmissao, n.PrestadorDoc + " " + n.PrestadorNome, n.TomadorDoc + " " + n.TomadorNome,
			n.Padrao, servicoLC116(n),
			n.CodMunIncidencia, f2(n.ValorServicos), f2(n.ValorISS), map[bool]string{true: "Sim", false: "Não"}[n.ISSRetido], f2(n.ValorIR), f2(n.ValorPIS), f2(n.ValorCOFINS), f2(n.ValorCSLL), f2(n.ValorINSS), f2(n.ValorLiquido)})
	}
	c.Flush()
	return c.Error()
}

// Documentos gera um CSV genérico de documentos (rejeições, manifestação, contingência).
func Documentos(w io.Writer, s *store.Store, f store.FiltroDocumentos, comRejeicao bool) error {
	f.PorPagina = 5000
	docs, _, err := s.ListarDocumentos(f)
	if err != nil {
		return err
	}
	c := novoCSV(w)
	cab := []string{"Chave", "Modelo", "Movimento", "Número", "Série", "Emissão", "Emitente", "Destinatário", "Valor", "Status", "cStat", "Motivo", "Forma emissão", "Manifestação", "Erros", "Alertas", "Score"}
	if comRejeicao {
		cab = append(cab, "Explicação", "Sugestão de correção")
	}
	c.Write(cab)
	for _, d := range docs {
		l := []string{d.Chave, d.Modelo, d.Direcao, d.Numero, d.Serie, d.DhEmissao, d.EmitDoc + " " + d.EmitNome, d.DestDoc + " " + d.DestNome, f2(d.ValorTotal), d.Status,
			strconv.Itoa(d.CStat), d.XMotivo, fiscal.DescricaoTpEmis[d.TpEmis], d.Manifestacao, strconv.Itoa(d.Erros), strconv.Itoa(d.Alertas), strconv.Itoa(d.Score)}
		if comRejeicao {
			r := fiscal.DescreveRejeicao(d.CStat)
			l = append(l, r.Explicacao, r.Sugestao)
		}
		c.Write(l)
	}
	c.Flush()
	return c.Error()
}

// Inconsistencias lista todas as ocorrências do período.
func Inconsistencias(w io.Writer, s *store.Store, f store.FiltroDocumentos) error {
	wh := []string{"o.severidade <> 'INFO'"}
	var a []any
	if f.EmpresaID > 0 {
		wh, a = append(wh, "d.empresa_id = ?"), append(a, f.EmpresaID)
	}
	if f.De != "" {
		wh, a = append(wh, "substr(d.dh_emissao,1,10) >= ?"), append(a, f.De)
	}
	if f.Ate != "" {
		wh, a = append(wh, "substr(d.dh_emissao,1,10) <= ?"), append(a, f.Ate)
	}
	rows, err := s.DB.Query(`SELECT d.chave, d.modelo, d.numero, d.direcao, o.regra, o.grupo, o.severidade, COALESCE(o.n_item,0), o.campo, o.mensagem, COALESCE(o.esperado,''), COALESCE(o.informado,''), COALESCE(o.sugestao,'')
		FROM ocorrencia o JOIN documento d ON d.id = o.documento_id WHERE `+strings.Join(wh, " AND ")+` ORDER BY o.severidade, o.regra`, a...)
	if err != nil {
		return err
	}
	defer rows.Close()
	c := novoCSV(w)
	c.Write([]string{"Chave", "Modelo", "Número", "Movimento", "Regra", "Grupo", "Severidade", "Item", "Campo", "Mensagem", "Esperado", "Informado", "Sugestão"})
	for rows.Next() {
		var r [13]string
		var item int
		rows.Scan(&r[0], &r[1], &r[2], &r[3], &r[4], &r[5], &r[6], &item, &r[8], &r[9], &r[10], &r[11], &r[12])
		r[7] = strconv.Itoa(item)
		c.Write(r[:])
	}
	c.Flush()
	return c.Error()
}

// ConciliacaoCSV exporta as conciliações.
func ConciliacaoCSV(w io.Writer, s *store.Store) error {
	cs, err := s.ListarConciliacoes(5000)
	if err != nil {
		return err
	}
	c := novoCSV(w)
	c.Write([]string{"Chave", "Pedido", "Ordem recebimento", "Status", "Divergências", "Data"})
	for _, x := range cs {
		n := 0
		if l, ok := x.Divergencias.([]any); ok {
			n = len(l)
		}
		c.Write([]string{x.Chave, x.PedidoNumero, x.OrdemRecebimento, x.Status, strconv.Itoa(n), x.CreatedAt})
	}
	c.Flush()
	return c.Error()
}

// LoteZIP exporta XMLs (e opcionalmente DANFEs) em ZIP organizados por modelo/movimento.
func LoteZIP(w io.Writer, s *store.Store, f store.FiltroDocumentos, comDanfe bool) (int, error) {
	docs, err := s.DocumentosComXML(f)
	if err != nil {
		return 0, err
	}
	zw := zip.NewWriter(w)
	n := 0
	for _, d := range docs {
		if d.XML == "" {
			continue
		}
		pasta := fmt.Sprintf("%s/%s", d.Modelo, map[string]string{"E": "entradas", "S": "saidas"}[d.Direcao])
		fw, err := zw.Create(pasta + "/xml/" + d.Chave + ".xml")
		if err != nil {
			return n, err
		}
		fw.Write([]byte(d.XML))
		if comDanfe && !d.Resumo {
			its, _ := s.Itens(d.ID)
			if fw, err := zw.Create(pasta + "/danfe/" + d.Chave + ".html"); err == nil {
				DANFE(fw, d, its)
			}
		}
		n++
	}
	return n, zw.Close()
}

func servicoLC116(n *xmlfiscal.NFSe) string {
	if n.CodigoServico != "" {
		return fiscal.CodigoServicoNormalizado(n.CodigoServico)
	}
	return fiscal.CTribNacParaLC116(n.CTribNac)
}
