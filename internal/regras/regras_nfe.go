package regras

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/xmlfiscal"
)

func fmtv(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func regrasNFe() []Regra {
	return []Regra{
		{ID: "NFE001", Grupo: "estrutura", Descricao: "Chave de acesso: DV e coerência com os campos do grupo ide", Aplica: ehNFe, Avaliar: rChave},
		{ID: "NFE002", Grupo: "cadastro", Descricao: "CNPJ/CPF do emitente e do destinatário", Aplica: ehNFe, Avaliar: rDocumentos},
		{ID: "NFE003", Grupo: "cadastro", Descricao: "Inscrição Estadual (validação local de DV)", Aplica: ehNFe, Avaliar: rIE},
		{ID: "NFE010", Grupo: "classificacao", Descricao: "CFOP x tipo de operação x destino", Aplica: ehNFe, Avaliar: rCFOP},
		{ID: "NFE011", Grupo: "operacao", Descricao: "Devolução: CFOP e documento referenciado", Aplica: ehNFe, Avaliar: rDevolucao},
		{ID: "NFE012", Grupo: "classificacao", Descricao: "Consumidor final / não contribuinte", Aplica: ehNFe, Avaliar: rConsumidorFinal},
		{ID: "NFE020", Grupo: "classificacao", Descricao: "NCM: formato e existência na TIPI", Aplica: ehNFe, Avaliar: rNCM},
		{ID: "NFE021", Grupo: "classificacao", Descricao: "CEST obrigatório em operações com ST", Aplica: ehNFe, Avaliar: rCEST},
		{ID: "NFE030", Grupo: "icms", Descricao: "CRT x CST/CSOSN", Aplica: ehNFe, Avaliar: rCRT},
		{ID: "NFE031", Grupo: "icms", Descricao: "Cálculo do ICMS próprio", Aplica: ehNFe, Avaliar: rICMSProprio},
		{ID: "NFE032", Grupo: "icms", Descricao: "Alíquota interestadual (4%/7%/12%)", Aplica: ehNFe, Avaliar: rAliqInter},
		{ID: "NFE040", Grupo: "difal", Descricao: "DIFAL EC 87/2015 – venda a consumidor final não contribuinte", Aplica: ehNFe, Avaliar: rDIFALSaida},
		{ID: "NFE041", Grupo: "difal", Descricao: "DIFAL do adquirente – entrada para uso/consumo ou ativo", Aplica: ehNFe, Avaliar: rDIFALEntrada},
		{ID: "NFE050", Grupo: "st", Descricao: "ICMS-ST: MVA, alíquota e valor retido", Aplica: ehNFe, Avaliar: rST},
		{ID: "NFE060", Grupo: "ipi", Descricao: "IPI: CST x tipo de operação e cálculo", Aplica: ehNFe, Avaliar: rIPI},
		{ID: "NFE070", Grupo: "pis_cofins", Descricao: "PIS/COFINS: CST x regime tributário e alíquotas", Aplica: ehNFe, Avaliar: rPISCOFINS},
		{ID: "NFE080", Grupo: "totais", Descricao: "Totalizadores do grupo ICMSTot", Aplica: ehNFe, Avaliar: rTotais},
		{ID: "NFE090", Grupo: "contingencia", Descricao: "Auditoria de emissão em contingência (SVC-AN, SVC-RS, FS-DA, off-line)", Aplica: ehNFe, Avaliar: rContingencia},
		{ID: "NFE095", Grupo: "reforma", Descricao: "Reforma Tributária: grupo IBS/CBS (LC 214/2025)", Aplica: ehNFe, Avaliar: rIBSCBS},
	}
}

func rChave(c *Contexto) []Ocorrencia {
	n := c.NFe
	ch := n.Chave()
	k := fiscal.AnalisaChave(ch)
	var out []Ocorrencia
	if !k.Valida {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "infNFe/@Id", Mensagem: "Chave de acesso inválida: " + k.Motivo, Informado: ch, CodRejeicao: 236})
		return out
	}
	chk := func(campo, esperado, inf string) {
		if strings.TrimLeft(esperado, "0") != strings.TrimLeft(inf, "0") {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: campo, Mensagem: "Campo diverge da chave de acesso", Esperado: esperado, Informado: inf, CodRejeicao: 236})
		}
	}
	chk("ide/cUF", k.CUF, n.Ide.CUF)
	chk("ide/mod", k.Modelo, n.Ide.Mod)
	chk("ide/serie", k.Serie, n.Ide.Serie)
	chk("ide/nNF", k.Numero, n.Ide.NNF)
	chk("ide/tpEmis", k.TpEmis, n.Ide.TpEmis)
	chk("emit/CNPJ", strings.TrimLeft(k.CNPJ, "0"), strings.TrimLeft(fiscal.NormalizaCNPJ(n.Emit.Doc()), "0"))
	if len(n.Ide.DhEmi) >= 7 {
		chk("ide/dhEmi (AAMM)", k.AAMM, n.Ide.DhEmi[2:4]+n.Ide.DhEmi[5:7])
	}
	return out
}

func rDocumentos(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	if !fiscal.ValidaDocumento(n.Emit.Doc()) {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "emit/CNPJ", Mensagem: "CNPJ/CPF do emitente inválido", Informado: n.Emit.Doc(), CodRejeicao: 207})
	}
	if d := n.Dest.Doc(); d != "" && !fiscal.ValidaDocumento(d) {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "dest/CNPJ", Mensagem: "CNPJ/CPF do destinatário inválido", Informado: d, CodRejeicao: 208})
	}
	if n.Ide.Mod == "55" && n.Dest.Doc() == "" && n.Dest.IdEstrangeiro == "" {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "dest", Mensagem: "NF-e modelo 55 sem identificação do destinatário"})
	}
	return out
}

func rIE(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	if ok, local := fiscal.ValidaIE(n.Emit.IE, n.Emit.EnderEmit.UF); !ok {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "emit/IE", Mensagem: "IE do emitente inválida para " + n.Emit.EnderEmit.UF, Informado: n.Emit.IE, CodRejeicao: 209})
	} else if !local {
		out = append(out, Ocorrencia{Severidade: Info, Campo: "emit/IE", Mensagem: "DV da IE não verificado localmente para " + n.Emit.EnderEmit.UF + "; validar via CadConsultaCadastro4"})
	}
	if n.Dest.IndIEDest == "1" {
		if ok, _ := fiscal.ValidaIE(n.Dest.IE, n.Dest.EnderDest.UF); !ok {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "dest/IE", Mensagem: "IE do destinatário inválida para " + n.Dest.EnderDest.UF, Informado: n.Dest.IE, CodRejeicao: 210})
		}
	}
	if n.Dest.IndIEDest == "1" && n.Dest.IE == "" {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "dest/IE", Mensagem: "indIEDest=1 (contribuinte) exige IE do destinatário", CodRejeicao: 210})
	}
	return out
}

func rCFOP(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	esperado := map[string]map[string]string{
		"0": {"1": "1", "2": "2", "3": "3"},
		"1": {"1": "5", "2": "6", "3": "7"},
	}
	for _, d := range n.Det {
		cfop := d.Prod.CFOP
		if len(cfop) != 4 {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "prod/CFOP", Mensagem: "CFOP inválido", Informado: cfop})
			continue
		}
		if e, ok := esperado[n.Ide.TpNF][n.Ide.IdDest]; ok && cfop[:1] != e {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "prod/CFOP",
				Mensagem: fmt.Sprintf("CFOP %s incompatível com tpNF=%s e idDest=%s", cfop, n.Ide.TpNF, n.Ide.IdDest),
				Esperado: e + "xxx", Informado: cfop, Sugestao: "Ajustar o primeiro dígito do CFOP: 1/5 interna, 2/6 interestadual, 3/7 exterior."})
		}
		if n.Ide.IdDest == "2" && n.Dest.EnderDest.UF == n.Emit.EnderEmit.UF {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "ide/idDest", Mensagem: "idDest=2 (interestadual) mas emitente e destinatário estão na mesma UF"})
			break
		}
	}
	return out
}

func rDevolucao(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	if n.Ide.FinNFe != "4" {
		for _, d := range n.Det {
			if fiscal.CFOPsDevolucao[d.Prod.CFOP] {
				out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "ide/finNFe", Mensagem: "CFOP de devolução/retorno em NF-e com finalidade diferente de devolução", Informado: d.Prod.CFOP, Sugestao: "Usar finNFe=4 e referenciar a NF-e original."})
				break
			}
		}
		return out
	}
	if len(n.Ide.NFref) == 0 {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "ide/NFref", Mensagem: "NF-e de devolução sem documento referenciado", CodRejeicao: 321})
	}
	for _, r := range n.Ide.NFref {
		if r.RefNFe != "" && !fiscal.AnalisaChave(r.RefNFe).Valida {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "NFref/refNFe", Mensagem: "Chave referenciada inválida", Informado: r.RefNFe})
		}
	}
	for _, d := range n.Det {
		if !fiscal.CFOPsDevolucao[d.Prod.CFOP] {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "prod/CFOP", Mensagem: "CFOP não é de devolução/retorno", Informado: d.Prod.CFOP, CodRejeicao: 327})
		}
	}
	return out
}

func rConsumidorFinal(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	if n.Dest.IndIEDest == "9" && n.Ide.IndFinal != "1" && n.Ide.Mod == "55" {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "ide/indFinal", Mensagem: "Destinatário não contribuinte exige indFinal=1", CodRejeicao: 696})
	}
	if n.Ide.IdDest == "2" && n.Dest.IndIEDest == "9" && n.Ide.TpNF == "1" {
		for _, d := range n.Det {
			switch d.Prod.CFOP {
			case "6102":
				out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "prod/CFOP", Mensagem: "Venda interestadual a não contribuinte com CFOP 6102", Esperado: "6108", Informado: "6102", Sugestao: "Para mercadoria de terceiros destinada a não contribuinte use 6108."})
			case "6101":
				out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "prod/CFOP", Mensagem: "Venda interestadual a não contribuinte com CFOP 6101", Esperado: "6107", Informado: "6101", Sugestao: "Para produção própria destinada a não contribuinte use 6107."})
			}
		}
	}
	return out
}

func rNCM(c *Contexto) []Ocorrencia {
	var out []Ocorrencia
	for _, d := range c.NFe.Det {
		ncm := d.Prod.NCM
		if ncm == "00" {
			continue // serviço em NF-e conjugada
		}
		if len(ncm) != 8 || fiscal.SoDigitos(ncm) != ncm {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "prod/NCM", Mensagem: "NCM deve ter 8 dígitos", Informado: ncm, CodRejeicao: 778})
			continue
		}
		if c.Params != nil {
			if existe, carregada := c.Params.NCMExiste(ncm); carregada && !existe {
				out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "prod/NCM", Mensagem: "NCM inexistente na TIPI vigente", Informado: ncm, CodRejeicao: 778})
			}
		}
	}
	return out
}

func rCEST(c *Contexto) []Ocorrencia {
	var out []Ocorrencia
	for _, d := range c.NFe.Det {
		g := d.Imposto.ICMSGrupo()
		cst := g.CSTouCSOSN()
		if fiscal.CSTsComST[cst] && d.Prod.CEST == "" {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "prod/CEST", Mensagem: "Operação com ICMS-ST (CST/CSOSN " + cst + ") sem CEST", CodRejeicao: 806})
		}
		if d.Prod.CEST != "" && (len(d.Prod.CEST) != 7 || fiscal.SoDigitos(d.Prod.CEST) != d.Prod.CEST) {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "prod/CEST", Mensagem: "CEST deve ter 7 dígitos", Informado: d.Prod.CEST})
		}
	}
	return out
}

func rCRT(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	simples := n.Emit.CRT == "1" || n.Emit.CRT == "4"
	for _, d := range n.Det {
		g := d.Imposto.ICMSGrupo()
		if g.XMLName.Local == "" {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "imposto/ICMS", Mensagem: "Grupo de ICMS não informado"})
			continue
		}
		if simples && g.CSOSN == "" {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMS/CSOSN", Mensagem: "Emitente do Simples Nacional (CRT=" + n.Emit.CRT + ") deve usar CSOSN", Informado: "CST " + g.CST})
		}
		if !simples && g.CSOSN != "" {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMS/CST", Mensagem: "Emitente do regime normal (CRT=" + n.Emit.CRT + ") deve usar CST", Informado: "CSOSN " + g.CSOSN})
		}
		if c.Empresa != nil && c.Direcao == "S" {
			emp := c.Empresa.Regime == "SN" || c.Empresa.Regime == "MEI"
			if emp != simples {
				out = append(out, Ocorrencia{Severidade: Alerta, Campo: "emit/CRT", Mensagem: "CRT do XML diverge do regime cadastrado da empresa (" + c.Empresa.Regime + ")", Informado: n.Emit.CRT})
				break
			}
		}
	}
	return out
}

func rICMSProprio(c *Contexto) []Ocorrencia {
	var out []Ocorrencia
	for _, d := range c.NFe.Det {
		g := d.Imposto.ICMSGrupo()
		if g.VBC.F() > 0 && g.PICMS.F() > 0 {
			esp := r2(g.VBC.F() * g.PICMS.F() / 100)
			if difere(esp, g.VICMS.F()) {
				out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMS/vICMS", Mensagem: "Valor do ICMS diverge de vBC × pICMS", Esperado: fmtv(esp), Informado: fmtv(g.VICMS.F())})
			}
		}
		if (g.CST == "00" || g.CST == "10") && g.PRedBC.F() == 0 {
			baseEsp := d.Prod.VProd.F() + d.Prod.VFrete.F() + d.Prod.VSeg.F() + d.Prod.VOutro.F() - d.Prod.VDesc.F()
			if g.VBC.F() > 0 && difere(baseEsp, g.VBC.F()) && difere(baseEsp+d.Imposto.IPIGrupo().VIPI.F(), g.VBC.F()) {
				out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "ICMS/vBC", Mensagem: "Base do ICMS difere do valor da operação sem redução informada", Esperado: fmtv(baseEsp), Informado: fmtv(g.VBC.F())})
			}
		}
	}
	return out
}

func rAliqInter(c *Contexto) []Ocorrencia {
	n := c.NFe
	if n.Ide.IdDest != "2" || n.Emit.CRT == "1" || n.Emit.CRT == "4" {
		return nil
	}
	var out []Ocorrencia
	for _, d := range n.Det {
		g := d.Imposto.ICMSGrupo()
		if g.PICMS.F() == 0 {
			continue
		}
		esp := fiscal.AliquotaInterestadual(n.Emit.EnderEmit.UF, n.Dest.EnderDest.UF, g.Orig)
		if difere(esp, g.PICMS.F()) {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMS/pICMS",
				Mensagem: fmt.Sprintf("Alíquota interestadual %s→%s (origem %s) incorreta", n.Emit.EnderEmit.UF, n.Dest.EnderDest.UF, g.Orig),
				Esperado: fmtv(esp), Informado: fmtv(g.PICMS.F()), Sugestao: "Res. SF 22/89 (7%/12%) e Res. SF 13/2012 (4% para importados)."})
		}
	}
	return out
}

func paramUF(c *Contexto, uf string) (fiscal.ParametroUF, bool) {
	if c.Params == nil {
		return fiscal.ParametroUF{}, false
	}
	return c.Params.UF(uf)
}

func rDIFALSaida(c *Contexto) []Ocorrencia {
	n := c.NFe
	if !(n.Ide.IdDest == "2" && n.Ide.IndFinal == "1" && n.Dest.IndIEDest == "9" && n.Ide.TpNF == "1") {
		return nil
	}
	var out []Ocorrencia
	pu, ok := paramUF(c, n.Dest.EnderDest.UF)
	for _, d := range n.Det {
		u := d.Imposto.ICMSUFDest
		if u == nil {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "imposto/ICMSUFDest", Mensagem: "Venda interestadual a consumidor final não contribuinte sem o grupo de DIFAL", CodRejeicao: 694})
			continue
		}
		g := d.Imposto.ICMSGrupo()
		pInter := fiscal.AliquotaInterestadual(n.Emit.EnderEmit.UF, n.Dest.EnderDest.UF, g.Orig)
		if difere(pInter, u.PICMSInter.F()) {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMSUFDest/pICMSInter", Mensagem: "Alíquota interestadual do DIFAL incorreta", Esperado: fmtv(pInter), Informado: fmtv(u.PICMSInter.F())})
		}
		if !ok {
			continue
		}
		if difere(pu.AliqInterna, u.PICMSUFDest.F()) {
			out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "ICMSUFDest/pICMSUFDest", Mensagem: "Alíquota interna da UF de destino diverge da tabela vigente", Esperado: fmtv(pu.AliqInterna), Informado: fmtv(u.PICMSUFDest.F())})
		}
		if u.VICMSUFRemet.F() > 0 {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMSUFDest/vICMSUFRemet", Mensagem: "Desde 2019 a partilha é 100% para a UF de destino (vICMSUFRemet deve ser 0)", Informado: fmtv(u.VICMSUFRemet.F())})
		}
		base := u.VBCUFDest.F()
		if pu.DifalMetodo == "BASE_DUPLA" {
			base = d.Prod.VProd.F() + d.Prod.VFrete.F() + d.Prod.VSeg.F() + d.Prod.VOutro.F() - d.Prod.VDesc.F() + d.Imposto.IPIGrupo().VIPI.F()
		}
		calc := CalculaDIFAL(base, pInter, pu.AliqInterna, pu.FCP, pu.DifalMetodo)
		if difere(calc.ValorDIFAL, u.VICMSUFDest.F()) {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMSUFDest/vICMSUFDest",
				Mensagem: "Valor do DIFAL divergente (" + pu.DifalMetodo + ")", Esperado: fmtv(calc.ValorDIFAL), Informado: fmtv(u.VICMSUFDest.F()),
				Sugestao: fmt.Sprintf("Base destino %s; alíquota interna %s%% − interestadual %s%%.", fmtv(calc.BaseDestino), fmtv(pu.AliqInterna), fmtv(pInter)), AcaoCorrecao: "calcular_difal"})
		}
		if pu.FCP > 0 && difere(calc.ValorFCP, u.VFCPUFDest.F()) {
			out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "ICMSUFDest/vFCPUFDest", Mensagem: "FCP da UF de destino divergente", Esperado: fmtv(calc.ValorFCP), Informado: fmtv(u.VFCPUFDest.F())})
		}
	}
	return out
}

func rDIFALEntrada(c *Contexto) []Ocorrencia {
	n := c.NFe
	if c.Direcao != "E" || n.Ide.IdDest != "2" {
		return nil
	}
	var out []Ocorrencia
	ufDest := n.Dest.EnderDest.UF
	pu, ok := paramUF(c, ufDest)
	for _, d := range n.Det {
		// Nota própria de entrada: o CFOP 2xxx define a finalidade. XML de terceiros:
		// a finalidade vem do pedido de compra conciliado (Contexto.FinalidadeItem).
		finalidade := ""
		if f, ok := fiscal.CFOPsUsoConsumoAtivo[d.Prod.CFOP]; ok {
			finalidade = f
		} else if f := c.FinalidadeItem[d.NItem]; f == "USO_CONSUMO" || f == "ATIVO" {
			finalidade = strings.ToLower(strings.ReplaceAll(f, "_", " "))
		}
		if finalidade == "" {
			continue
		}
		if !ok {
			continue
		}
		g := d.Imposto.ICMSGrupo()
		pInter := g.PICMS.F()
		if pInter == 0 {
			pInter = fiscal.AliquotaInterestadual(n.Emit.EnderEmit.UF, ufDest, g.Orig)
		}
		base := d.Prod.VProd.F() + d.Prod.VFrete.F() + d.Prod.VSeg.F() + d.Prod.VOutro.F() - d.Prod.VDesc.F() + d.Imposto.IPIGrupo().VIPI.F()
		calc := CalculaDIFAL(base, pInter, pu.AliqInterna, 0, pu.DifalMetodo)
		if calc.ValorDIFAL > 0 {
			out = append(out, Ocorrencia{Severidade: Info, Item: d.NItem, Campo: "DIFAL entrada",
				Mensagem: fmt.Sprintf("DIFAL a recolher pelo adquirente (%s): R$ %s", finalidade, fmtv(calc.ValorDIFAL)),
				Esperado: fmtv(calc.ValorDIFAL), Sugestao: "Lançar no registro E110/E111 (EFD ICMS/IPI) conforme legislação da UF."})
		}
	}
	return out
}

func rST(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	ufDest := n.Dest.EnderDest.UF
	pu, okUF := paramUF(c, ufDest)
	dh := parseData(n.Ide.DhEmi)
	for _, d := range n.Det {
		g := d.Imposto.ICMSGrupo()
		cst := g.CSTouCSOSN()
		if !fiscal.CSTsComST[cst] || fiscal.CSTsSTRetidaAnteriormente[cst] {
			continue
		}
		if g.VBCST.F() == 0 {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMS/vBCST", Mensagem: "CST/CSOSN com ST sem base de cálculo de ST"})
			continue
		}
		if g.ModBCST == "4" && c.Params != nil { // 4 = MVA
			if mva, ok := c.Params.MVA(d.Prod.NCM, d.Prod.CEST, ufDest, dh); ok {
				esperado := mva
				if n.Ide.IdDest == "2" && n.Emit.CRT == "3" && okUF {
					esperado = MVAAjustada(mva, fiscal.AliquotaInterestadual(n.Emit.EnderEmit.UF, ufDest, g.Orig), pu.AliqInterna)
				}
				if difere(esperado, g.PMVAST.F()) {
					out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "ICMS/pMVAST",
						Mensagem: "MVA divergente da tabela vigente (possível MVA defasada)", Esperado: fmtv(esperado), Informado: fmtv(g.PMVAST.F()),
						Sugestao: "Verificar protocolo/convênio vigente para NCM " + d.Prod.NCM + " em " + ufDest + "."})
				}
			}
		}
		if okUF && n.Ide.TpNF == "1" && g.PICMSST.F() > 0 && difere(pu.AliqInterna+pu.FCP, g.PICMSST.F()) && difere(pu.AliqInterna, g.PICMSST.F()) {
			out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "ICMS/pICMSST", Mensagem: "Alíquota interna de ST diverge da tabela da UF de destino", Esperado: fmtv(pu.AliqInterna), Informado: fmtv(g.PICMSST.F())})
		}
		baseOp := d.Prod.VProd.F() + d.Prod.VFrete.F() + d.Prod.VSeg.F() + d.Prod.VOutro.F() - d.Prod.VDesc.F() + d.Imposto.IPIGrupo().VIPI.F()
		calc := CalculaICMSST(baseOp, g.PMVAST.F(), g.PRedBCST.F(), g.PICMSST.F(), g.VICMS.F())
		if g.ModBCST == "4" && difere(calc.BaseST, g.VBCST.F()) {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMS/vBCST", Mensagem: "Base de cálculo do ICMS-ST divergente", Esperado: fmtv(calc.BaseST), Informado: fmtv(g.VBCST.F())})
		}
		espST := r2(g.VBCST.F()*g.PICMSST.F()/100 - g.VICMS.F())
		if espST < 0 {
			espST = 0
		}
		if difere(espST, g.VICMSST.F()) {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "ICMS/vICMSST", Mensagem: "Valor do ICMS-ST diverge de vBCST × pICMSST − vICMS próprio", Esperado: fmtv(espST), Informado: fmtv(g.VICMSST.F())})
		}
	}
	return out
}

func rIPI(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	for _, d := range n.Det {
		g := d.Imposto.IPIGrupo()
		if g.CST == "" {
			continue
		}
		cst, _ := strconv.Atoi(g.CST)
		if n.Ide.TpNF == "1" && cst < 50 {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "IPI/CST", Mensagem: "CST de IPI de entrada (00–49) em nota de saída", Informado: g.CST})
		}
		if n.Ide.TpNF == "0" && cst >= 50 {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "IPI/CST", Mensagem: "CST de IPI de saída (50–99) em nota de entrada", Informado: g.CST})
		}
		if (g.CST == "00" || g.CST == "50") && g.PIPI.F() > 0 && difere(r2(g.VBC.F()*g.PIPI.F()/100), g.VIPI.F()) {
			out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: "IPI/vIPI", Mensagem: "Valor do IPI diverge de vBC × pIPI", Esperado: fmtv(r2(g.VBC.F() * g.PIPI.F() / 100)), Informado: fmtv(g.VIPI.F())})
		}
	}
	return out
}

func rPISCOFINS(c *Contexto) []Ocorrencia {
	n := c.NFe
	var out []Ocorrencia
	simples := n.Emit.CRT == "1" || n.Emit.CRT == "4"
	regime := ""
	if c.Empresa != nil && c.Direcao == "S" {
		regime = c.Empresa.Regime
	}
	for _, d := range n.Det {
		for _, tr := range []struct {
			nome      string
			g         xmlfiscal.GrupoImposto
			aliq, val float64
			lr, lp    float64
		}{
			{"PIS", d.Imposto.PISGrupo(), d.Imposto.PISGrupo().PPIS.F(), d.Imposto.PISGrupo().VPIS.F(), 1.65, 0.65},
			{"COFINS", d.Imposto.COFINSGrupo(), d.Imposto.COFINSGrupo().PCOFINS.F(), d.Imposto.COFINSGrupo().VCOFINS.F(), 7.6, 3.0},
		} {
			if tr.g.CST == "" {
				out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: tr.nome + "/CST", Mensagem: "Grupo " + tr.nome + " não informado"})
				continue
			}
			cst, _ := strconv.Atoi(tr.g.CST)
			if n.Ide.TpNF == "1" && cst >= 50 && cst < 99 {
				out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: tr.nome + "/CST", Mensagem: "CST de entrada/crédito (50–98) em documento de saída", Informado: tr.g.CST})
			}
			if simples && n.Ide.TpNF == "1" && (cst == 1 || cst == 2) && tr.aliq > 0 {
				out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: tr.nome + "/CST", Mensagem: "Emitente do Simples Nacional destacando " + tr.nome + " com CST " + tr.g.CST, Esperado: "49 ou 99", Informado: tr.g.CST, Sugestao: "No Simples o PIS/COFINS é recolhido no DAS; usar CST 49/99 (ou 04/06 para monofásico/alíquota zero)."})
			}
			if n.Ide.TpNF == "1" && fiscal.EhMonofasico(d.Prod.NCM) && cst == 1 && !simples {
				out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: tr.nome + "/CST", Mensagem: "NCM " + d.Prod.NCM + " sujeito a tributação monofásica com CST 01", Sugestao: "Na revenda de monofásico use CST 04; na saída do fabricante/importador, CST 02."})
			}
			if cst == 1 && tr.aliq > 0 {
				switch regime {
				case "LR":
					if difere(tr.aliq, tr.lr) {
						out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: tr.nome + "/p" + tr.nome, Mensagem: "Alíquota de " + tr.nome + " não cumulativa (Lucro Real) divergente", Esperado: fmtv(tr.lr), Informado: fmtv(tr.aliq)})
					}
				case "LP":
					if difere(tr.aliq, tr.lp) {
						out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: tr.nome + "/p" + tr.nome, Mensagem: "Alíquota de " + tr.nome + " cumulativa (Lucro Presumido) divergente", Esperado: fmtv(tr.lp), Informado: fmtv(tr.aliq)})
					}
				}
			}
			if tr.g.VBC.F() > 0 && tr.aliq > 0 && difere(r2(tr.g.VBC.F()*tr.aliq/100), tr.val) {
				out = append(out, Ocorrencia{Severidade: Erro, Item: d.NItem, Campo: tr.nome + "/v" + tr.nome, Mensagem: "Valor de " + tr.nome + " diverge de vBC × alíquota", Esperado: fmtv(r2(tr.g.VBC.F() * tr.aliq / 100)), Informado: fmtv(tr.val)})
			}
		}
		if c.Empresa != nil && c.Empresa.Regime == "LP" && c.Direcao == "E" && n.Ide.TpNF == "0" {
			if cst, _ := strconv.Atoi(d.Imposto.PISGrupo().CST); cst >= 50 && cst <= 66 {
				out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "PIS/CST", Mensagem: "Empresa do Lucro Presumido (cumulativo) não apropria crédito de PIS/COFINS", Informado: d.Imposto.PISGrupo().CST, Esperado: "70–75 ou 98/99"})
			}
		}
	}
	return out
}

func rTotais(c *Contexto) []Ocorrencia {
	n := c.NFe
	t := n.Total.ICMSTot
	var s struct{ vProd, vBC, vICMS, vBCST, vST, vPIS, vCOFINS, vIPI, vDesc, vFrete, vSeg, vOutro, vFCPST, vDeson float64 }
	for _, d := range n.Det {
		if d.Prod.IndTot != "0" {
			s.vProd += d.Prod.VProd.F()
		}
		g := d.Imposto.ICMSGrupo()
		s.vBC += g.VBC.F()
		s.vICMS += g.VICMS.F()
		s.vBCST += g.VBCST.F()
		s.vST += g.VICMSST.F()
		s.vFCPST += g.VFCPST.F()
		s.vDeson += g.VICMSDeson.F()
		s.vPIS += d.Imposto.PISGrupo().VPIS.F()
		s.vCOFINS += d.Imposto.COFINSGrupo().VCOFINS.F()
		s.vIPI += d.Imposto.IPIGrupo().VIPI.F()
		s.vDesc += d.Prod.VDesc.F()
		s.vFrete += d.Prod.VFrete.F()
		s.vSeg += d.Prod.VSeg.F()
		s.vOutro += d.Prod.VOutro.F()
	}
	var out []Ocorrencia
	chk := func(campo string, soma, total float64, rej int) {
		if difere(r2(soma), total) {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "ICMSTot/" + campo, Mensagem: "Total difere do somatório dos itens", Esperado: fmtv(r2(soma)), Informado: fmtv(total), CodRejeicao: rej})
		}
	}
	chk("vProd", s.vProd, t.VProd.F(), 564)
	chk("vBC", s.vBC, t.VBC.F(), 531)
	chk("vICMS", s.vICMS, t.VICMS.F(), 532)
	chk("vBCST", s.vBCST, t.VBCST.F(), 533)
	chk("vST", s.vST, t.VST.F(), 534)
	chk("vPIS", s.vPIS, t.VPIS.F(), 602)
	chk("vCOFINS", s.vCOFINS, t.VCOFINS.F(), 603)
	vnf := t.VProd.F() - t.VDesc.F() - t.VICMSDeson.F() + t.VST.F() + t.VFCPST.F() + t.VFrete.F() + t.VSeg.F() + t.VOutro.F() + t.VII.F() + t.VIPI.F() + t.VIPIDevol.F()
	chk("vNF", vnf, t.VNF.F(), 610)
	return out
}

func rContingencia(c *Contexto) []Ocorrencia {
	n := c.NFe
	tp := n.Ide.TpEmis
	if tp == "" || tp == "1" {
		return nil
	}
	var out []Ocorrencia
	desc := fiscal.DescricaoTpEmis[tp]
	if tp == "6" || tp == "7" {
		ufAut := fiscal.UFPorCodigo[n.Ide.CUF]
		if esp := fiscal.SVCPorUF[ufAut]; esp != "" && esp != tp {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "ide/tpEmis", Mensagem: "SVC incorreta para a UF " + ufAut, Esperado: esp + " (" + fiscal.DescricaoTpEmis[esp] + ")", Informado: tp + " (" + desc + ")"})
		}
		if n.Ide.Mod == "65" {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "ide/tpEmis", Mensagem: "NFC-e não admite contingência SVC (usar tpEmis=9 off-line)"})
		}
	}
	if n.Ide.DhCont == "" {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "ide/dhCont", Mensagem: "Emissão em contingência sem data/hora de entrada em contingência"})
	}
	if l := len([]rune(n.Ide.XJust)); l < 15 || l > 256 {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "ide/xJust", Mensagem: "Justificativa de contingência deve ter entre 15 e 256 caracteres", Informado: fmt.Sprint(l)})
	}
	if c.Prot == nil {
		emi := parseData(n.Ide.DhEmi)
		horas := c.Agora.Sub(emi).Hours()
		sev := Alerta
		if horas > 24 {
			sev = Erro
		}
		out = append(out, Ocorrencia{Severidade: sev, Campo: "protNFe", Mensagem: fmt.Sprintf("Documento em %s sem autorização há %.1f h", desc, horas),
			Sugestao: "Transmitir à SEFAZ assim que o serviço normal for restabelecido (FS-DA/off-line: prazo legal de transmissão).", AcaoCorrecao: "transmitir_contingencia"})
	}
	if tp == "5" {
		out = append(out, Ocorrencia{Severidade: Info, Campo: "ide/tpEmis", Mensagem: "FS-DA: DANFE deve ser impresso em formulário de segurança (mín. 2 vias)"})
	}
	return out
}

func rIBSCBS(c *Contexto) []Ocorrencia {
	n := c.NFe
	emi := parseData(n.Ide.DhEmi)
	if emi.Year() < 2026 || n.Emit.CRT == "1" || n.Emit.CRT == "4" {
		return nil
	}
	var out []Ocorrencia
	ausentes := 0
	for _, d := range n.Det {
		x := d.Imposto.IBSCBS
		if x == nil {
			ausentes++
			continue
		}
		if emi.Year() == 2026 && x.G.GCBS.PCBS.F() > 0 && difere(x.G.GCBS.PCBS.F(), 0.9) {
			out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "IBSCBS/gCBS/pCBS", Mensagem: "Alíquota de teste da CBS em 2026 é 0,9%", Esperado: "0.90", Informado: fmtv(x.G.GCBS.PCBS.F())})
		}
		if emi.Year() == 2026 && x.G.GIBSUF.PIBSUF.F() > 0 && difere(x.G.GIBSUF.PIBSUF.F()+x.G.GIBSMun.PIBSMun.F(), 0.1) {
			out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "IBSCBS/gIBSUF", Mensagem: "Alíquota de teste do IBS em 2026 é 0,1% (UF + município)", Esperado: "0.10", Informado: fmtv(x.G.GIBSUF.PIBSUF.F() + x.G.GIBSMun.PIBSMun.F())})
		}
		if x.CClassTrib == "" {
			out = append(out, Ocorrencia{Severidade: Alerta, Item: d.NItem, Campo: "IBSCBS/cClassTrib", Mensagem: "cClassTrib não informado"})
		}
	}
	if ausentes > 0 {
		out = append(out, Ocorrencia{Severidade: Alerta, Campo: "imposto/IBSCBS", Mensagem: fmt.Sprintf("%d item(ns) sem grupo IBS/CBS (obrigatório para regime normal a partir de 2026, NT 2025.002)", ausentes)})
	}
	return out
}

func parseData(s string) time.Time {
	for _, l := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ParseData é exportado para uso em outros pacotes.
func ParseData(s string) time.Time { return parseData(s) }
