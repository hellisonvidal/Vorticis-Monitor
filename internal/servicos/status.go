// Package servicos orquestra ingestão, validação, captura, alertas e conciliação.
package servicos

// Status do documento no Monitor (ver docs/ARQUITETURA.md §3).
const (
	StPendente     = "PENDENTE"     // recebido do ERP, ainda sem retorno da SEFAZ
	StValidacao    = "BLOQUEADA"    // reprovado na pré-validação (não deve ser transmitido)
	StAutorizada   = "AUTORIZADA"
	StCancelada    = "CANCELADA"
	StDenegada     = "DENEGADA"
	StRejeitada    = "REJEITADA"
	StContingencia = "CONTINGENCIA" // emitido em contingência, aguardando transmissão/autorização
	StInutilizada  = "INUTILIZADA"
	StResumo       = "RESUMO"       // resNFe recebido; aguardando ciência/download do XML completo
)

// StatusPorCStat converte o código de retorno da SEFAZ em status do Monitor.
func StatusPorCStat(cstat int, tpEmis string) string {
	switch {
	case cstat == 100 || cstat == 150:
		return StAutorizada
	case cstat == 101 || cstat == 151 || cstat == 155 || cstat == 218:
		return StCancelada
	case cstat == 110 || cstat == 205 || cstat == 301 || cstat == 302 || cstat == 303:
		return StDenegada
	case cstat == 102 || cstat == 206:
		return StInutilizada
	case cstat == 0 || cstat == 103 || cstat == 105 || cstat == 217:
		if tpEmis != "" && tpEmis != "1" {
			return StContingencia
		}
		return StPendente
	case cstat >= 200:
		return StRejeitada
	}
	return StPendente
}

// Transicoes válidas da máquina de estados.
var Transicoes = map[string][]string{
	StPendente:     {StAutorizada, StRejeitada, StDenegada, StContingencia, StValidacao},
	StValidacao:    {StPendente},
	StContingencia: {StAutorizada, StRejeitada, StDenegada},
	StRejeitada:    {StPendente, StAutorizada, StInutilizada},
	StAutorizada:   {StCancelada},
	StResumo:       {StAutorizada, StCancelada, StDenegada},
	StCancelada:    {},
	StDenegada:     {},
	StInutilizada:  {},
}

// TransicaoValida verifica se a mudança de status é permitida.
func TransicaoValida(de, para string) bool {
	if de == para || de == "" {
		return true
	}
	for _, s := range Transicoes[de] {
		if s == para {
			return true
		}
	}
	return false
}
