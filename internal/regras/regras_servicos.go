package regras

import (
	"fmt"
	"strings"

	"vorticis.com.br/monitor/internal/fiscal"
)

func regrasNFSe() []Regra {
	return []Regra{
		{ID: "NFS001", Grupo: "cadastro", Descricao: "Documentos do prestador e tomador", Aplica: ehNFSe, Avaliar: sDocumentos},
		{ID: "NFS010", Grupo: "iss", Descricao: "Código de tributação do serviço (LC 116/2003 e cTribNac)", Aplica: ehNFSe, Avaliar: sCodigo},
		{ID: "NFS011", Grupo: "iss", Descricao: "Local de incidência do ISS (LC 116/2003 art. 3º)", Aplica: ehNFSe, Avaliar: sLocalIncidencia},
		{ID: "NFS012", Grupo: "iss", Descricao: "Alíquota (2%–5%) e valor do ISS", Aplica: ehNFSe, Avaliar: sAliquotaISS},
		{ID: "NFS020", Grupo: "retencoes", Descricao: "Retenções federais: IRRF, PIS/COFINS/CSLL e INSS", Aplica: ehNFSe, Avaliar: sRetencoes},
		{ID: "NFS030", Grupo: "totais", Descricao: "Valor líquido da NFS-e", Aplica: ehNFSe, Avaliar: sLiquido},
	}
}

func sDocumentos(c *Contexto) []Ocorrencia {
	n := c.NFSe
	var out []Ocorrencia
	if !fiscal.ValidaDocumento(n.PrestadorDoc) {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "prestador", Mensagem: "CNPJ/CPF do prestador inválido", Informado: n.PrestadorDoc})
	}
	if n.TomadorDoc != "" && !fiscal.ValidaDocumento(n.TomadorDoc) {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "tomador", Mensagem: "CNPJ/CPF do tomador inválido", Informado: n.TomadorDoc})
	}
	return out
}

func subitem(n string, ctrib string) string {
	if n != "" {
		return fiscal.CodigoServicoNormalizado(n)
	}
	return fiscal.CTribNacParaLC116(ctrib)
}

func sCodigo(c *Contexto) []Ocorrencia {
	n := c.NFSe
	var out []Ocorrencia
	if n.Padrao == "NACIONAL" {
		if d := fiscal.SoDigitos(n.CTribNac); len(d) != 6 {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "cServ/cTribNac", Mensagem: "Código de tributação nacional deve ter 6 dígitos (item+subitem+desdobro)", Informado: n.CTribNac})
		}
	}
	si := subitem(n.CodigoServico, n.CTribNac)
	p := strings.Split(si, ".")
	if len(p) != 2 || p[0] == "" || len(p[1]) != 2 {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "ItemListaServico", Mensagem: "Subitem da lista de serviços (LC 116/2003) inválido", Informado: si})
	}
	return out
}

func sLocalIncidencia(c *Contexto) []Ocorrencia {
	n := c.NFSe
	si := subitem(n.CodigoServico, n.CTribNac)
	if n.CodMunIncidencia == "" {
		return []Ocorrencia{{Severidade: Alerta, Campo: "cLocIncid", Mensagem: "Município de incidência do ISS não informado"}}
	}
	esperado, regra := n.PrestadorMun, "regra geral: domicílio do prestador"
	if fiscal.LC116LocalPrestacao[si] && n.CodMunPrestacao != "" {
		esperado, regra = n.CodMunPrestacao, "exceção do art. 3º: local da prestação"
	}
	if esperado != "" && esperado != n.CodMunIncidencia {
		return []Ocorrencia{{Severidade: Erro, Campo: "cLocIncid", Mensagem: "Município de incidência do ISS incorreto para o subitem " + si + " (" + regra + ")",
			Esperado: esperado, Informado: n.CodMunIncidencia, Sugestao: "Risco de bitributação/ISS recolhido ao município errado."}}
	}
	return nil
}

func sAliquotaISS(c *Contexto) []Ocorrencia {
	n := c.NFSe
	var out []Ocorrencia
	if !n.PrestadorSimples && n.AliquotaISS > 0 && (n.AliquotaISS < 2 || n.AliquotaISS > 5) {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "pAliq", Mensagem: "Alíquota de ISS fora do intervalo 2%–5% (LC 116/2003 art. 8º-A, LC 157/2016)", Informado: fmtv(n.AliquotaISS)})
	}
	if n.BaseCalculo > 0 && n.AliquotaISS > 0 {
		esp := r2(n.BaseCalculo * n.AliquotaISS / 100)
		if difere(esp, n.ValorISS) {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "vISSQN", Mensagem: "Valor do ISS diverge de base × alíquota", Esperado: fmtv(esp), Informado: fmtv(n.ValorISS)})
		}
	}
	return out
}

func sRetencoes(c *Contexto) []Ocorrencia {
	n := c.NFSe
	var out []Ocorrencia
	tomadorPJ := len(fiscal.NormalizaCNPJ(n.TomadorDoc)) == 14
	si := subitem(n.CodigoServico, n.CTribNac)
	base := n.ValorServicos
	pcc := n.ValorPIS + n.ValorCOFINS + n.ValorCSLL
	if n.PrestadorSimples {
		if n.ValorIR > 0 || pcc > 0 {
			out = append(out, Ocorrencia{Severidade: Alerta, Campo: "retencoes", Mensagem: "Retenção de IRRF/PCC sobre prestador optante do Simples Nacional",
				Sugestao: "Optantes do Simples são dispensados (IN RFB 1.234/2012 art. 4º, XI; IN SRF 459/2004 art. 3º). Verificar declaração do prestador."})
		}
	} else if tomadorPJ {
		esp := CalculaRetencoes(base, si, fiscal.LC116RetencaoIR1Pct[si], fiscal.LC116CessaoMaoObra[si])
		if n.ValorIR > 0 && difere(n.ValorIR, r2(base*0.015)) && difere(n.ValorIR, r2(base*0.01)) {
			out = append(out, Ocorrencia{Severidade: Alerta, Campo: "vRetIRRF", Mensagem: "IRRF retido diverge das alíquotas usuais (1,5% ou 1%)", Esperado: fmtv(esp.IRRF), Informado: fmtv(n.ValorIR)})
		}
		if n.ValorIR > 0 && n.ValorIR <= 10 {
			out = append(out, Ocorrencia{Severidade: Alerta, Campo: "vRetIRRF", Mensagem: "IRRF ≤ R$ 10,00 é dispensado de retenção", Informado: fmtv(n.ValorIR)})
		}
		if pcc > 0 {
			if pcc <= 10 {
				out = append(out, Ocorrencia{Severidade: Alerta, Campo: "PCC", Mensagem: "PIS/COFINS/CSLL retidos ≤ R$ 10,00 são dispensados (Lei 13.137/2015)", Informado: fmtv(pcc)})
			}
			for _, x := range []struct {
				nome     string
				v, aliq  float64
			}{{"PIS", n.ValorPIS, 0.65}, {"COFINS", n.ValorCOFINS, 3}, {"CSLL", n.ValorCSLL, 1}} {
				if e := r2(base * x.aliq / 100); x.v > 0 && difere(e, x.v) {
					out = append(out, Ocorrencia{Severidade: Alerta, Campo: "ret" + x.nome, Mensagem: fmt.Sprintf("Retenção de %s diverge de %.2f%%", x.nome, x.aliq), Esperado: fmtv(e), Informado: fmtv(x.v)})
				}
			}
		}
		if fiscal.LC116CessaoMaoObra[si] && n.ValorINSS == 0 {
			out = append(out, Ocorrencia{Severidade: Alerta, Campo: "vRetCP", Mensagem: "Serviço " + si + " tipicamente sujeito a retenção de INSS (11%) por cessão de mão de obra/empreitada", Esperado: fmtv(esp.INSS),
				Sugestao: "Confirmar enquadramento (Lei 8.212/91 art. 31; IN RFB 2.110/2022) e informar na EFD-Reinf R-2010."})
		}
		if n.ValorINSS > 0 && difere(n.ValorINSS, r2(base*0.11)) && difere(n.ValorINSS, r2(base*0.035)) {
			out = append(out, Ocorrencia{Severidade: Alerta, Campo: "vRetCP", Mensagem: "INSS retido diverge de 11% (ou 3,5% CPRB)", Esperado: fmtv(r2(base * 0.11)), Informado: fmtv(n.ValorINSS)})
		}
	}
	return out
}

func sLiquido(c *Contexto) []Ocorrencia {
	n := c.NFSe
	if n.ValorLiquido == 0 {
		return nil
	}
	esp := n.ValorServicos - n.ValorIR - n.ValorPIS - n.ValorCOFINS - n.ValorCSLL - n.ValorINSS
	if n.ISSRetido {
		esp -= n.ValorISS
	}
	if difere(r2(esp), n.ValorLiquido) {
		return []Ocorrencia{{Severidade: Alerta, Campo: "vLiq", Mensagem: "Valor líquido diverge de serviços − retenções", Esperado: fmtv(r2(esp)), Informado: fmtv(n.ValorLiquido)}}
	}
	return nil
}

// ---------- CT-e ----------

func regrasCTe() []Regra {
	return []Regra{
		{ID: "CTE001", Grupo: "estrutura", Descricao: "Chave de acesso do CT-e", Aplica: ehCTe, Avaliar: tChave},
		{ID: "CTE010", Grupo: "classificacao", Descricao: "CFOP de prestação de transporte", Aplica: ehCTe, Avaliar: tCFOP},
		{ID: "CTE020", Grupo: "totais", Descricao: "Valor da prestação x valor a receber e ICMS", Aplica: ehCTe, Avaliar: tValores},
		{ID: "CTE030", Grupo: "vinculo", Descricao: "Documentos transportados", Aplica: ehCTe, Avaliar: tDocs},
	}
}

func tChave(c *Contexto) []Ocorrencia {
	ch := strings.TrimPrefix(c.CTe.ID, "CTe")
	if k := fiscal.AnalisaChave(ch); !k.Valida {
		return []Ocorrencia{{Severidade: Erro, Campo: "infCte/@Id", Mensagem: "Chave do CT-e inválida: " + k.Motivo, CodRejeicao: 236}}
	}
	return nil
}

func tCFOP(c *Contexto) []Ocorrencia {
	cf := c.CTe.Ide.CFOP
	if !fiscal.EhCFOPTransporte(cf) && cf != "5932" && cf != "6932" && cf != "7358" {
		return []Ocorrencia{{Severidade: Erro, Campo: "ide/CFOP", Mensagem: "CFOP não é de prestação de serviço de transporte (x351–x360, x932)", Informado: cf}}
	}
	inter := c.CTe.Ide.UFIni != c.CTe.Ide.UFFim
	if inter && strings.HasPrefix(cf, "5") {
		return []Ocorrencia{{Severidade: Erro, Campo: "ide/CFOP", Mensagem: "Prestação interestadual com CFOP de operação interna", Informado: cf}}
	}
	return nil
}

func tValores(c *Contexto) []Ocorrencia {
	var out []Ocorrencia
	v := c.CTe.VPrest
	if v.VRec.F() > v.VTPrest.F()+tolerancia {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "vPrest/vRec", Mensagem: "Valor a receber maior que o valor total da prestação"})
	}
	g := c.CTe.Imp.ICMS.Primeiro()
	if g.VBC.F() > 0 && g.PICMS.F() > 0 && difere(r2(g.VBC.F()*g.PICMS.F()/100), g.VICMS.F()) {
		out = append(out, Ocorrencia{Severidade: Erro, Campo: "imp/ICMS/vICMS", Mensagem: "ICMS do CT-e diverge de vBC × pICMS", Esperado: fmtv(r2(g.VBC.F() * g.PICMS.F() / 100)), Informado: fmtv(g.VICMS.F())})
	}
	return out
}

func tDocs(c *Contexto) []Ocorrencia {
	var out []Ocorrencia
	for _, n := range c.CTe.InfCTeNorm.InfDoc.InfNFe {
		if !fiscal.AnalisaChave(n.Chave).Valida {
			out = append(out, Ocorrencia{Severidade: Erro, Campo: "infDoc/infNFe/chave", Mensagem: "Chave de NF-e transportada inválida", Informado: n.Chave})
		}
	}
	return out
}
