// Package regras implementa o motor de regras fiscais do Vorticis Monitor.
//
// Pipeline: Parse XML → Contexto (perfil da empresa + parâmetros vigentes) →
// regras estruturais → cadastrais → classificação (CFOP/NCM/CEST) → tributos
// (ICMS, ST, DIFAL, IPI, PIS/COFINS, IBS/CBS) → totais → operação (contingência,
// devolução) → consolidação (score e bloqueio).
package regras

import (
	"math"
	"sort"
	"time"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/xmlfiscal"
)

type Severidade string

const (
	Erro   Severidade = "ERRO"   // causa rejeição/autuação – bloqueia pré-emissão
	Alerta Severidade = "ALERTA" // inconsistência que exige revisão
	Info   Severidade = "INFO"
)

// Ocorrencia é um achado do motor de regras.
type Ocorrencia struct {
	Regra         string     `json:"regra"`
	Grupo         string     `json:"grupo"`
	Severidade    Severidade `json:"severidade"`
	Item          int        `json:"item,omitempty"`
	Campo         string     `json:"campo,omitempty"`
	Mensagem      string     `json:"mensagem"`
	Sugestao      string     `json:"sugestao,omitempty"`
	Esperado      string     `json:"esperado,omitempty"`
	Informado     string     `json:"informado,omitempty"`
	CodRejeicao   int        `json:"cod_rejeicao,omitempty"`
	AcaoCorrecao  string     `json:"acao_correcao,omitempty"`
}

// Perfil é o enquadramento tributário da empresa monitorada.
type Perfil struct {
	CNPJ   string
	UF     string
	Regime string // SN, MEI, LP, LR
}

// Parametros fornece tabelas fiscais vigentes (UF, MVA, NCM).
type Parametros interface {
	UF(uf string) (fiscal.ParametroUF, bool)
	MVA(ncm, cest, ufDest string, data time.Time) (mva float64, ok bool)
	NCMExiste(ncm string) (existe bool, tabelaCarregada bool)
}

// Contexto agrega o documento e o ambiente de avaliação.
type Contexto struct {
	NFe     *xmlfiscal.InfNFe
	Prot    *xmlfiscal.ProtNFe
	NFSe    *xmlfiscal.NFSe
	CTe     *xmlfiscal.InfCte
	Empresa *Perfil
	Params  Parametros
	Agora   time.Time
	// Direcao em relação à empresa: "S" (saída/emitida) ou "E" (entrada/recebida).
	Direcao string
	// FinalidadeItem (nItem → REVENDA | INDUSTRIALIZACAO | USO_CONSUMO | ATIVO), vinda do pedido de compra.
	FinalidadeItem map[int]string
}

// Regra é uma unidade do motor.
type Regra struct {
	ID        string
	Grupo     string
	Descricao string
	Aplica    func(*Contexto) bool
	Avaliar   func(*Contexto) []Ocorrencia
}

// Resultado consolida a avaliação.
type Resultado struct {
	Ocorrencias []Ocorrencia `json:"ocorrencias"`
	Erros       int          `json:"erros"`
	Alertas     int          `json:"alertas"`
	Bloqueante  bool         `json:"bloqueante"`
	Score       int          `json:"score"` // 0–100 (100 = sem inconsistências)
	RegrasExec  []string     `json:"regras_executadas"`
}

// Motor executa um conjunto ordenado de regras.
type Motor struct{ regras []Regra }

// Novo cria o motor com todas as regras registradas.
func Novo() *Motor {
	m := &Motor{}
	m.regras = append(m.regras, regrasNFe()...)
	m.regras = append(m.regras, regrasNFSe()...)
	m.regras = append(m.regras, regrasCTe()...)
	return m
}

// Regras lista as regras para documentação/API.
func (m *Motor) Regras() []Regra { return m.regras }

// Executar avalia o contexto.
func (m *Motor) Executar(c *Contexto) Resultado {
	if c.Agora.IsZero() {
		c.Agora = time.Now()
	}
	res := Resultado{Ocorrencias: []Ocorrencia{}}
	for _, r := range m.regras {
		if r.Aplica != nil && !r.Aplica(c) {
			continue
		}
		res.RegrasExec = append(res.RegrasExec, r.ID)
		for _, o := range r.Avaliar(c) {
			o.Regra, o.Grupo = r.ID, r.Grupo
			if o.CodRejeicao != 0 && o.AcaoCorrecao == "" {
				o.AcaoCorrecao = fiscal.DescreveRejeicao(o.CodRejeicao).AcaoAutomatica
			}
			res.Ocorrencias = append(res.Ocorrencias, o)
		}
	}
	sort.SliceStable(res.Ocorrencias, func(i, j int) bool {
		return peso(res.Ocorrencias[i].Severidade) > peso(res.Ocorrencias[j].Severidade)
	})
	score := 100
	for _, o := range res.Ocorrencias {
		switch o.Severidade {
		case Erro:
			res.Erros++
			score -= 15
		case Alerta:
			res.Alertas++
			score -= 5
		}
	}
	res.Bloqueante = res.Erros > 0
	res.Score = int(math.Max(0, float64(score)))
	return res
}

func peso(s Severidade) int {
	switch s {
	case Erro:
		return 3
	case Alerta:
		return 2
	}
	return 1
}

// ---- helpers ----

const tolerancia = 0.02

func r2(v float64) float64 { return math.Round(v*100) / 100 }

func difere(a, b float64) bool { return math.Abs(a-b) > tolerancia }

func ehNFe(c *Contexto) bool  { return c.NFe != nil }
func ehNFSe(c *Contexto) bool { return c.NFSe != nil }
func ehCTe(c *Contexto) bool  { return c.CTe != nil }
