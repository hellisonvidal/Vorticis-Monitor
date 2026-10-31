package relatorios

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/store"
)

// Tipos de coluna: T texto, D data, M moeda, N inteiro.
const (
	T = 'T'
	D = 'D'
	M = 'M'
	N = 'N'
)

// Coluna de relatório. Largura em mm no PDF (0 = só na planilha).
type Coluna struct {
	Titulo  string  `json:"titulo"`
	Tipo    rune    `json:"-"`
	TipoStr string  `json:"tipo"`
	Largura float64 `json:"-"`
}

// Relatorio é o resultado tabular, renderizado em PDF, XLSX, XLS ou CSV.
type Relatorio struct {
	ID      string   `json:"id"`
	Titulo  string   `json:"titulo"`
	Filtros string   `json:"filtros"`
	Colunas []Coluna `json:"colunas"`
	Linhas  [][]any  `json:"linhas"`
	Total   []any    `json:"total,omitempty"`
}

// Tipo de relatório disponível no menu.
type Tipo struct {
	ID        string `json:"id"`
	Nome      string `json:"nome"`
	Descricao string `json:"descricao"`
}

var Catalogo = []Tipo{
	{"documentos", "Relação de documentos", "Todos os documentos do período com participante, situação, valores e impostos"},
	{"tributos", "Resumo de tributos", "Totais de valor, ICMS, ICMS-ST, IPI, PIS, COFINS, DIFAL e ISS por tipo de documento e movimento (somente autorizados)"},
	{"participantes", "Por fornecedor / cliente", "Quantidade e valor por emitente (recebidos) ou destinatário (emitidos)"},
	{"cfop", "Resumo por CFOP", "Itens de NF-e/NFC-e agrupados por CFOP com base e impostos (somente autorizados)"},
	{"manifestacao", "Manifestação das NF-e recebidas", "NF-e destinadas à empresa e a situação da manifestação do destinatário"},
}

var nomesModelo = map[string]string{"NFE": "NF-e", "NFCE": "NFC-e", "CTE": "CT-e", "MDFE": "MDF-e", "NFSE": "NFS-e"}
var nomesManif = map[string]string{"CIENCIA": "Ciência", "CONFIRMADA": "Confirmada", "DESCONHECIDA": "Desconhecida", "NAO_REALIZADA": "Não realizada"}
var nomesStatus = map[string]string{"AUTORIZADA": "Autorizada", "CANCELADA": "Cancelada", "DENEGADA": "Denegada", "REJEITADA": "Rejeitada", "PENDENTE": "Pendente",
	"CONTINGENCIA": "Contingência", "BLOQUEADA": "Bloqueada", "RESUMO": "Resumo", "INUTILIZADA": "Inutilizada"}

func mov(d string) string {
	if d == "E" {
		return "Recebido"
	}
	return "Emitido"
}

func modelo(m string) string {
	if n := nomesModelo[m]; n != "" {
		return n
	}
	return m
}

func data(s string) any {
	for _, l := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(l, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t
		}
	}
	return s
}

// participante devolve a outra parte do documento conforme o movimento.
func participante(d *store.Documento) (nome, doc, uf string) {
	if d.Direcao == "E" {
		return d.EmitNome, d.EmitDoc, d.EmitUF
	}
	return d.DestNome, d.DestDoc, d.DestUF
}

// FormatarDoc aplica a máscara de CNPJ/CPF.
func FormatarDoc(s string) string {
	switch len(s) {
	case 14:
		return s[:2] + "." + s[2:5] + "." + s[5:8] + "/" + s[8:12] + "-" + s[12:]
	case 11:
		return s[:3] + "." + s[3:6] + "." + s[6:9] + "-" + s[9:]
	}
	return s
}

func cols(def ...any) []Coluna {
	out := []Coluna{}
	for i := 0; i < len(def); i += 3 {
		t := def[i+1].(rune)
		out = append(out, Coluna{Titulo: def[i].(string), Tipo: t, TipoStr: string(t), Largura: def[i+2].(float64)})
	}
	return out
}

// Gerar monta o relatório. itens devolve os itens de um documento (usado no CFOP).
func Gerar(id string, docs []*store.Documento, itens func(int64) []store.Item, filtros string) (*Relatorio, error) {
	r := &Relatorio{ID: id, Filtros: filtros}
	for _, t := range Catalogo {
		if t.ID == id {
			r.Titulo = t.Nome
		}
	}
	autorizado := func(d *store.Documento) bool { return d.Status == "AUTORIZADA" }
	switch id {
	case "documentos":
		r.Colunas = cols("Emissão", D, 19.0, "Modelo", T, 13.0, "Número", T, 18.0, "Série", T, 10.0, "Movimento", T, 17.0,
			"Participante", T, 56.0, "CNPJ/CPF", T, 32.0, "UF", T, 8.0, "Situação", T, 21.0, "Valor total", M, 24.0,
			"ICMS", M, 20.0, "ICMS-ST", M, 20.0, "IPI", M, 19.0, "PIS", M, 0.0, "COFINS", M, 0.0, "Chave de acesso", T, 0.0)
		var tv, ti, ts, tp, tpis, tcof float64
		for _, d := range docs {
			nome, doc, uf := participante(d)
			r.Linhas = append(r.Linhas, []any{data(d.DhEmissao), modelo(d.Modelo), d.Numero, d.Serie, mov(d.Direcao), nome, FormatarDoc(doc), uf,
				nomesStatus[d.Status], d.ValorTotal, d.ValorICMS, d.ValorST, d.ValorIPI, d.ValorPIS, d.ValorCOFINS, d.Chave})
			if autorizado(d) {
				tv, ti, ts, tp, tpis, tcof = tv+d.ValorTotal, ti+d.ValorICMS, ts+d.ValorST, tp+d.ValorIPI, tpis+d.ValorPIS, tcof+d.ValorCOFINS
			}
		}
		r.Total = []any{"Total", "", "", "", "", fmt.Sprintf("%d documento(s) · soma dos autorizados", len(docs)), "", "", "", tv, ti, ts, tp, tpis, tcof, ""}
	case "tributos":
		r.Colunas = cols("Documento", T, 26.0, "Movimento", T, 22.0, "Qtd.", N, 14.0, "Valor total", M, 30.0, "ICMS", M, 26.0, "ICMS-ST", M, 26.0,
			"IPI", M, 24.0, "PIS", M, 24.0, "COFINS", M, 24.0, "DIFAL", M, 24.0, "ISS", M, 24.0)
		type ac struct {
			q int
			v [8]float64
		}
		m := map[string]*ac{}
		for _, d := range docs {
			if !autorizado(d) {
				continue
			}
			k := modelo(d.Modelo) + "|" + mov(d.Direcao)
			if m[k] == nil {
				m[k] = &ac{}
			}
			a := m[k]
			a.q++
			for i, v := range []float64{d.ValorTotal, d.ValorICMS, d.ValorST, d.ValorIPI, d.ValorPIS, d.ValorCOFINS, d.ValorDIFAL, d.ValorISS} {
				a.v[i] += v
			}
		}
		tot := ac{}
		for _, k := range chavesOrdenadas(m) {
			a := m[k]
			p := strings.Split(k, "|")
			l := []any{p[0], p[1], a.q}
			tot.q += a.q
			for i, v := range a.v {
				l = append(l, v)
				tot.v[i] += v
			}
			r.Linhas = append(r.Linhas, l)
		}
		r.Total = []any{"Total", "", tot.q}
		for _, v := range tot.v {
			r.Total = append(r.Total, v)
		}
	case "participantes":
		r.Colunas = cols("Participante", T, 90.0, "CNPJ/CPF", T, 36.0, "UF", T, 10.0, "Movimento", T, 22.0, "Qtd.", N, 16.0,
			"Valor total", M, 34.0, "ICMS", M, 30.0, "ICMS-ST", M, 30.0)
		type ac struct {
			nome, doc, uf, mov string
			q                  int
			v, i, s            float64
		}
		m := map[string]*ac{}
		for _, d := range docs {
			if !autorizado(d) {
				continue
			}
			nome, doc, uf := participante(d)
			k := d.Direcao + doc + nome
			if m[k] == nil {
				m[k] = &ac{nome: nome, doc: doc, uf: uf, mov: mov(d.Direcao)}
			}
			a := m[k]
			a.q, a.v, a.i, a.s = a.q+1, a.v+d.ValorTotal, a.i+d.ValorICMS, a.s+d.ValorST
		}
		l := make([]*ac, 0, len(m))
		for _, a := range m {
			l = append(l, a)
		}
		sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v })
		var q int
		var tv, ti, ts float64
		for _, a := range l {
			r.Linhas = append(r.Linhas, []any{a.nome, FormatarDoc(a.doc), a.uf, a.mov, a.q, a.v, a.i, a.s})
			q, tv, ti, ts = q+a.q, tv+a.v, ti+a.i, ts+a.s
		}
		r.Total = []any{fmt.Sprintf("Total (%d participantes)", len(l)), "", "", "", q, tv, ti, ts}
	case "cfop":
		r.Colunas = cols("CFOP", T, 20.0, "Movimento", T, 24.0, "Itens", N, 18.0, "Valor dos itens", M, 36.0, "Base ICMS", M, 34.0,
			"ICMS", M, 30.0, "ICMS-ST", M, 30.0, "IPI", M, 30.0, "PIS", M, 27.0, "COFINS", M, 27.0)
		type ac struct {
			q int
			v [7]float64
		}
		m := map[string]*ac{}
		for _, d := range docs {
			if !autorizado(d) || (d.Modelo != "NFE" && d.Modelo != "NFCE") {
				continue
			}
			for _, it := range itens(d.ID) {
				k := it.CFOP + "|" + mov(d.Direcao)
				if m[k] == nil {
					m[k] = &ac{}
				}
				a := m[k]
				a.q++
				for i, v := range []float64{it.ValorTotal, it.VBC, it.VICMS, it.VICMSST, it.VIPI, it.VPIS, it.VCOFINS} {
					a.v[i] += v
				}
			}
		}
		tot := ac{}
		for _, k := range chavesOrdenadas(m) {
			a := m[k]
			p := strings.Split(k, "|")
			l := []any{p[0], p[1], a.q}
			tot.q += a.q
			for i, v := range a.v {
				l = append(l, v)
				tot.v[i] += v
			}
			r.Linhas = append(r.Linhas, l)
		}
		r.Total = []any{"Total", "", tot.q}
		for _, v := range tot.v {
			r.Total = append(r.Total, v)
		}
	case "manifestacao":
		r.Colunas = cols("Emissão", D, 20.0, "Número", T, 20.0, "Emitente", T, 76.0, "CNPJ/CPF", T, 34.0, "UF", T, 9.0, "Valor total", M, 28.0,
			"Situação", T, 24.0, "Manifestação", T, 26.0, "Data da manifestação", D, 30.0, "Chave de acesso", T, 0.0)
		var tv float64
		for _, d := range docs {
			if d.Direcao != "E" || d.Modelo != "NFE" {
				continue
			}
			man := nomesManif[d.Manifestacao]
			if man == "" {
				man = "Pendente"
			}
			var dm any = ""
			if d.DhManifestacao != "" {
				dm = data(d.DhManifestacao)
			}
			r.Linhas = append(r.Linhas, []any{data(d.DhEmissao), d.Numero, d.EmitNome, FormatarDoc(d.EmitDoc), d.EmitUF, d.ValorTotal,
				nomesStatus[d.Status], man, dm, d.Chave})
			tv += d.ValorTotal
		}
		r.Total = []any{"Total", fmt.Sprintf("%d NF-e", len(r.Linhas)), "", "", "", tv, "", "", "", ""}
	default:
		return nil, fmt.Errorf("relatório %q não existe", id)
	}
	if r.Linhas == nil {
		r.Linhas = [][]any{}
	}
	return r, nil
}

func chavesOrdenadas[V any](m map[string]V) []string {
	k := make([]string, 0, len(m))
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

// Texto converte um valor de célula para exibição (PDF/CSV).
func Texto(v any, tipo rune) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case time.Time:
		return x.Format("02/01/2006")
	case int:
		return fmt.Sprint(x)
	case float64:
		if tipo == N {
			return fmt.Sprintf("%.0f", x)
		}
		return Moeda(x)
	}
	return fmt.Sprint(v)
}

// Moeda formata 1234.5 como "1.234,50".
func Moeda(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%.2f", v)
	inteiro, dec := s[:len(s)-3], s[len(s)-2:]
	var b strings.Builder
	for i, c := range inteiro {
		if i > 0 && (len(inteiro)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	out := b.String() + "," + dec
	if neg {
		out = "-" + out
	}
	return out
}
