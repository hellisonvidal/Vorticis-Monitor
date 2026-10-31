package fiscal

import "sort"

// Rejeicao descreve um código de retorno (cStat) da SEFAZ em linguagem clara.
type Rejeicao struct {
	Codigo     int    `json:"codigo"`
	Oficial    string `json:"descricao_oficial"`
	Explicacao string `json:"explicacao"`
	Sugestao   string `json:"sugestao"`
	// AcaoAutomatica identifica uma correção que o Monitor pode executar/propor
	// automaticamente (ver servicos.CorrecaoAutomatica).
	AcaoAutomatica string `json:"acao_automatica,omitempty"`
	Categoria      string `json:"categoria"`
}

// CatalogoRejeicoes contém os códigos mais frequentes (MOC NF-e 7.0 / NT vigentes).
var CatalogoRejeicoes = map[int]Rejeicao{}

func add(c int, cat, of, exp, sug, acao string) {
	CatalogoRejeicoes[c] = Rejeicao{Codigo: c, Categoria: cat, Oficial: of, Explicacao: exp, Sugestao: sug, AcaoAutomatica: acao}
}

func init() {
	// Sucesso / situações
	add(100, "sucesso", "Autorizado o uso da NF-e", "Documento autorizado.", "Nenhuma ação necessária.", "")
	add(101, "sucesso", "Cancelamento de NF-e homologado", "Documento cancelado com sucesso.", "Nenhuma ação necessária.", "")
	add(102, "sucesso", "Inutilização de número homologado", "Faixa de numeração inutilizada.", "Nenhuma ação necessária.", "")
	add(135, "sucesso", "Evento registrado e vinculado a NF-e", "Evento (cancelamento, CC-e, manifestação) registrado.", "Nenhuma ação necessária.", "")
	add(150, "sucesso", "Autorizado o uso da NF-e, autorização fora de prazo", "Autorizada, mas transmitida após o prazo (comum em contingência).", "Verifique a rotina de transmissão de contingência.", "")
	add(137, "distribuicao", "Nenhum documento localizado para o destinatário", "Não há novos documentos no Ambiente Nacional.", "Aguardar 1 hora antes de nova consulta (regra de consumo indevido).", "agendar_proxima_consulta")
	add(138, "distribuicao", "Documento localizado para o destinatário", "Há documentos a processar.", "Processamento automático.", "")
	add(108, "indisponibilidade", "Serviço Paralisado Momentaneamente (curto prazo)", "SEFAZ autorizadora fora do ar temporariamente.", "Ativar contingência SVC da UF ou aguardar alguns minutos.", "ativar_contingencia")
	add(109, "indisponibilidade", "Serviço Paralisado sem Previsão", "SEFAZ autorizadora indisponível sem previsão de retorno.", "Emitir em contingência SVC-AN/SVC-RS conforme a UF.", "ativar_contingencia")
	add(110, "denegacao", "Uso Denegado", "A SEFAZ denegou o uso do documento por irregularidade cadastral.", "O número fica consumido; regularize o cadastro antes de reemitir com novo número.", "")
	add(301, "denegacao", "Uso Denegado: Irregularidade fiscal do emitente", "A IE do emitente está irregular (suspensa, baixada, etc).", "Regularizar a IE do emitente junto à SEFAZ.", "")
	add(302, "denegacao", "Uso Denegado: Irregularidade fiscal do destinatário", "A IE do destinatário está irregular.", "Consultar o cadastro do cliente (CadConsultaCadastro) e regularizar antes de nova venda.", "consultar_cadastro")
	add(303, "denegacao", "Uso Denegado: Destinatário não habilitado a operar na UF", "O destinatário não está habilitado na UF.", "Verificar inscrição do destinatário.", "consultar_cadastro")
	// Rejeições frequentes
	add(204, "duplicidade", "Duplicidade de NF-e", "Já existe NF-e autorizada com a mesma chave.", "Consultar o protocolo pela chave e recuperar o XML autorizado (não reenviar).", "consultar_protocolo")
	add(539, "duplicidade", "Duplicidade de NF-e com diferença na Chave de Acesso", "Existe NF-e com mesmo número/série, mas com chave diferente (cNF ou data distintos).", "Consultar a chave informada no xMotivo; se autorizada, recuperar o XML; caso contrário, gerar novo número.", "consultar_protocolo")
	add(205, "situacao", "NF-e está denegada na base de dados da SEFAZ", "O documento já consta como denegado.", "Não é possível reutilizar o número.", "")
	add(206, "situacao", "NF-e já está inutilizada na Base de dados da SEFAZ", "Número inutilizado anteriormente.", "Emitir com o próximo número disponível.", "")
	add(207, "cadastro", "CNPJ do emitente inválido", "O CNPJ do emitente não passou na validação do DV.", "Corrigir o CNPJ no cadastro da empresa.", "")
	add(208, "cadastro", "CNPJ do destinatário inválido", "O CNPJ do cliente é inválido.", "Corrigir o cadastro do cliente.", "")
	add(209, "cadastro", "IE do emitente inválida", "A Inscrição Estadual do emitente é inválida.", "Corrigir a IE no cadastro da empresa.", "")
	add(210, "cadastro", "IE do destinatário inválida", "A IE do cliente é inválida para a UF.", "Validar via CadConsultaCadastro; informar ISENTO ou indIEDest=9 se não contribuinte.", "consultar_cadastro")
	add(213, "certificado", "CNPJ-Base do Emitente difere do CNPJ-Base do Certificado Digital", "O certificado usado não pertence à empresa emitente.", "Selecionar o certificado da empresa correta.", "")
	add(215, "schema", "Falha no schema XML", "O XML não respeita o leiaute oficial.", "Verificar campos obrigatórios/formato; validar contra o XSD da versão vigente.", "")
	add(225, "schema", "Falha no Schema XML da NF-e", "Estrutura da NF-e fora do leiaute.", "Validar o XML contra o XSD; verificar caracteres especiais e casas decimais.", "")
	add(217, "situacao", "NF-e não consta na base de dados da SEFAZ", "A chave consultada não existe na SEFAZ.", "Se emitida em contingência, transmitir; senão verificar a chave.", "transmitir_contingencia")
	add(218, "situacao", "NF-e já está cancelada na base de dados da SEFAZ", "Documento cancelado anteriormente.", "Atualizar o status no ERP.", "sincronizar_status")
	add(228, "data", "Data de Emissão muito atrasada", "dhEmi anterior ao limite aceito (em geral 30 dias).", "Emitir novo documento com data atual.", "")
	add(233, "cadastro", "IE do destinatário não cadastrada", "A IE informada não existe no cadastro da SEFAZ.", "Consultar cadastro do cliente.", "consultar_cadastro")
	add(236, "chave", "Chave de Acesso com dígito verificador inválido", "O DV da chave não confere.", "Recalcular o cDV (módulo 11).", "recalcular_dv")
	add(252, "ambiente", "Ambiente informado diverge do Ambiente de recebimento", "tpAmb diferente do endpoint (produção x homologação).", "Ajustar tpAmb ou a URL do webservice.", "")
	add(280, "certificado", "Certificado Transmissor inválido", "Certificado vencido, revogado ou cadeia não reconhecida.", "Instalar certificado válido e verificar a cadeia ICP-Brasil.", "")
	add(297, "assinatura", "Assinatura difere do calculado", "O XML foi alterado após a assinatura.", "Reassinar o XML após qualquer alteração.", "")
	add(321, "devolucao", "NF-e de devolução de mercadoria não possui documento fiscal referenciado", "finNFe=4 exige NFref.", "Informar a chave da NF-e original no grupo NFref.", "incluir_referencia")
	add(327, "cfop", "CFOP inválido para NF-e com finalidade de devolução", "Devolução com CFOP que não é de devolução.", "Usar CFOP de devolução (ex.: 5202/6202, 5411/6411, 1202/2202).", "sugerir_cfop")
	add(501, "prazo", "Prazo de cancelamento superior ao previsto na Legislação", "O cancelamento foi solicitado fora do prazo.", "Avaliar cancelamento extemporâneo junto à SEFAZ ou emitir NF-e de estorno/entrada.", "")
	add(531, "totais", "Total da BC ICMS difere do somatório dos itens", "ICMSTot/vBC ≠ soma de vBC dos itens.", "Recalcular totais.", "recalcular_totais")
	add(532, "totais", "Total do ICMS difere do somatório dos itens", "ICMSTot/vICMS ≠ soma dos itens.", "Recalcular totais.", "recalcular_totais")
	add(533, "totais", "Total da BC ICMS-ST difere do somatório dos itens", "ICMSTot/vBCST ≠ soma dos itens.", "Recalcular totais.", "recalcular_totais")
	add(534, "totais", "Total do ICMS-ST difere do somatório dos itens", "ICMSTot/vST ≠ soma dos itens.", "Recalcular totais.", "recalcular_totais")
	add(564, "totais", "Total do Produto / Serviço difere do somatório dos itens", "ICMSTot/vProd ≠ soma de vProd.", "Recalcular totais.", "recalcular_totais")
	add(602, "totais", "Total do PIS difere do somatório dos itens sujeitos ao ICMS", "ICMSTot/vPIS ≠ soma dos itens.", "Recalcular totais de PIS.", "recalcular_totais")
	add(603, "totais", "Total do COFINS difere do somatório dos itens sujeitos ao ICMS", "ICMSTot/vCOFINS ≠ soma dos itens.", "Recalcular totais de COFINS.", "recalcular_totais")
	add(610, "totais", "Total da NF difere do somatório dos Valores compõe o valor Total da NF", "vNF ≠ vProd - vDesc - vICMSDeson + vST + vFCPST + vFrete + vSeg + vOutro + vII + vIPI + vIPIDevol.", "Recalcular vNF.", "recalcular_totais")
	add(656, "distribuicao", "Consumo Indevido", "Consultas excessivas ao webservice (ex.: distDFe sem respeitar ultNSU/1h).", "Respeitar intervalo de 1 hora e sempre usar o ultNSU retornado.", "agendar_proxima_consulta")
	add(694, "difal", "Não informado o grupo de ICMS para a UF de destino", "Venda interestadual a consumidor final não contribuinte sem o grupo ICMSUFDest (DIFAL).", "Incluir o grupo ICMSUFDest com vBCUFDest, pICMSUFDest, pICMSInter e vICMSUFDest.", "calcular_difal")
	add(696, "difal", "Operação com não contribuinte deve indicar operação com consumidor final", "indIEDest=9 exige indFinal=1.", "Ajustar indFinal=1.", "ajustar_indfinal")
	add(778, "ncm", "Informado NCM inexistente", "O NCM não consta na tabela TIPI vigente.", "Atualizar o NCM do produto conforme TIPI.", "sugerir_ncm")
	add(806, "st", "Operação com ICMS-ST sem informação do CEST", "Produto com ST precisa do código CEST.", "Informar o CEST compatível com o NCM (Convênio ICMS 142/2018).", "sugerir_cest")
	add(573, "evento", "Duplicidade de Evento", "Evento com mesmo tipo e sequência já registrado.", "Consultar eventos da chave e incrementar nSeqEvento se aplicável.", "consultar_protocolo")
	add(999, "outros", "Erro não catalogado", "Erro interno da SEFAZ.", "Tentar novamente; persistindo, contatar a SEFAZ.", "")
}

// DescreveRejeicao retorna a descrição do código (ou genérica).
func DescreveRejeicao(c int) Rejeicao {
	if r, ok := CatalogoRejeicoes[c]; ok {
		return r
	}
	return Rejeicao{Codigo: c, Categoria: "outros", Oficial: "Código não catalogado", Explicacao: "Consulte o MOC/NT vigente para o código.", Sugestao: "Verificar xMotivo retornado pela SEFAZ."}
}

// ListaRejeicoes devolve o catálogo ordenado.
func ListaRejeicoes() []Rejeicao {
	out := make([]Rejeicao, 0, len(CatalogoRejeicoes))
	for _, r := range CatalogoRejeicoes {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Codigo < out[j].Codigo })
	return out
}
