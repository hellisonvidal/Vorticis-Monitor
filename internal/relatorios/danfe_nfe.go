package relatorios

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/store"
)

// DANFE completo da NF-e (modelo 55), retrato A4, no leiaute do Manual de
// Orientação do Contribuinte: canhoto, emitente (com logotipo quando
// cadastrado), quadro DANFE, chave com código de barras, protocolo,
// destinatário, faturas/duplicatas, cálculo do imposto, transportador e volumes,
// produtos, ISSQN e dados adicionais. Lido direto do XML da nota.

var tabModFrete = map[string]string{"0": "0 - Emitente (CIF)", "1": "1 - Destinatário (FOB)", "2": "2 - Terceiros", "3": "3 - Próprio remetente", "4": "4 - Próprio destinatário", "9": "9 - Sem frete"}

func num(s string, casas int) string {
	if s == "" {
		return ""
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	if casas == 2 {
		return milhar(f)
	}
	t := strconv.FormatFloat(f, 'f', casas, 64)
	if i := strings.Index(t, "."); i >= 0 { // tira zeros finais, mantendo ao menos 2 casas
		for len(t) > i+3 && t[len(t)-1] == '0' {
			t = t[:len(t)-1]
		}
	}
	return strings.Replace(t, ".", ",", 1)
}

func fone(s string) string {
	s = fiscal.SoDigitos(s)
	switch len(s) {
	case 10:
		return "(" + s[:2] + ") " + s[2:6] + "-" + s[6:]
	case 11:
		return "(" + s[:2] + ") " + s[2:7] + "-" + s[7:]
	}
	return s
}

func numeroNF(n string) string {
	n = fmt.Sprintf("%09s", fiscal.SoDigitos(n))
	return n[:3] + "." + n[3:6] + "." + n[6:]
}

type itemDANFE struct {
	cod, desc, ncm, cst, cfop, un, qtd, vun, vtot, bc, vicms, vipi, aicms, aipi string
}

// DANFENFe gera o DANFE completo da NF-e a partir do XML armazenado.
func DANFENFe(w io.Writer, doc *store.Documento) error {
	r := lerArvore([]byte(doc.XML))
	inf := r.em("infNFe")
	if inf == nil {
		return fmt.Errorf("XML sem infNFe")
	}
	ide, emit, dest, tot := inf.em("ide"), inf.em("emit"), inf.em("dest"), inf.em("total/ICMSTot")
	transp, prot := inf.em("transp"), r.em("infProt")
	chave := strings.TrimPrefix(inf.v("@Id"), "NFe")
	if chave == "" {
		chave = doc.Chave
	}
	nNF, serie := ide.v("nNF"), ide.v("serie")

	var itens []itemDANFE
	for _, det := range inf.filhos {
		if det.nome != "det" {
			continue
		}
		p := det.em("prod")
		ic := det.em("imposto/ICMS")
		var icms *no
		if ic != nil && len(ic.filhos) > 0 {
			icms = ic.filhos[0]
		}
		cst := icms.v("orig") + icms.v("CST", "CSOSN")
		desc := p.v("xProd")
		if a := det.v("infAdProd"); a != "" {
			desc += "\n" + a
		}
		ipi := det.em("imposto/IPI/IPITrib")
		itens = append(itens, itemDANFE{cod: p.v("cProd"), desc: desc, ncm: p.v("NCM"), cst: cst, cfop: p.v("CFOP"), un: p.v("uCom"),
			qtd: num(p.v("qCom"), 4), vun: num(p.v("vUnCom"), 4), vtot: num(p.v("vProd"), 2),
			bc: num(icms.v("vBC"), 2), vicms: num(icms.v("vICMS"), 2), vipi: num(ipi.v("vIPI"), 2), aicms: num(icms.v("pICMS"), 2), aipi: num(ipi.v("pIPI"), 2)})
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	const x0, L, rodape = 7.0, 196.0, 288.0
	pdf.SetMargins(x0, 7, x0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AliasNbPages("{nb}")
	pdf.SetTitle(tr("DANFE NF-e "+nNF), false)
	pdf.SetCreator("Vorticis Monitor", false)
	logo := registrarLogo(pdf, emit.v("CNPJ", "CPF"))

	linhaCor := func() { pdf.SetDrawColor(90, 90, 90); pdf.SetLineWidth(0.2) }
	linhaCor()
	// campo: caixa com rótulo pequeno e valor (alinhamento a = L, C ou R).
	campo := func(x, y, cw, h float64, rot, val string, a string, tam float64, negrito bool) {
		pdf.Rect(x, y, cw, h, "D")
		pdf.SetTextColor(80, 80, 80)
		pdf.SetFont("Helvetica", "", 5)
		pdf.SetXY(x+0.8, y+0.5)
		pdf.CellFormat(cw-1.6, 2.2, tr(strings.ToUpper(rot)), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		st := ""
		if negrito {
			st = "B"
		}
		pdf.SetFont("Helvetica", st, tam)
		txt := tr(val)
		for pdf.GetStringWidth(txt) > cw-1.6 && len(txt) > 1 {
			txt = txt[:len(txt)-1]
		}
		pdf.SetXY(x+0.8, y+2.6)
		pdf.CellFormat(cw-1.6, h-3, txt, "", 0, a, false, 0, "")
	}
	titulo := func(y float64, t string) float64 {
		pdf.SetFont("Helvetica", "B", 6.5)
		pdf.SetTextColor(20, 60, 90)
		pdf.SetXY(x0, y)
		pdf.CellFormat(L, 3.4, tr(t), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		return y + 3.4
	}
	aviso := avisoStatus(doc)
	if ide.v("tpAmb") == "2" {
		aviso = juntar(" · ", aviso, "SEM VALOR FISCAL - EMITIDA EM HOMOLOGAÇÃO")
	}
	marca := map[string]string{"CANCELADA": "CANCELADA", "DENEGADA": "USO DENEGADO"}[doc.Status]
	if marca == "" && aviso != "" {
		marca = "SEM VALOR FISCAL"
	}
	marcaDagua := func() {
		if marca == "" {
			return
		}
		pdf.SetAlpha(0.13, "Normal")
		pdf.SetTextColor(200, 30, 30)
		pdf.SetFont("Helvetica", "B", 54)
		pdf.TransformBegin()
		pdf.TransformRotate(38, 105, 165)
		pdf.SetXY(5, 158)
		pdf.CellFormat(200, 20, tr(marca), "", 0, "C", false, 0, "")
		pdf.TransformEnd()
		pdf.SetAlpha(1, "Normal")
		pdf.SetTextColor(0, 0, 0)
	}

	// Quadro do emitente + DANFE + chave (repetido em todas as folhas).
	cabecalho := func(y float64) float64 {
		h := 32.0
		// emitente
		pdf.Rect(x0, y, 80, h, "D")
		tx, tw := x0+1.5, 77.0
		if logo != "" {
			desenharLogo(pdf, logo, x0+1.5, y+2, 24, h-4)
			tx, tw = x0+27, 51.5
		}
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetXY(tx, y+2.5)
		pdf.MultiCell(tw, 3.8, tr(emit.v("xNome")), "", "C", false)
		pdf.SetFont("Helvetica", "", 6.5)
		end := emit.em("enderEmit")
		for _, l := range []string{juntar(", ", end.v("xLgr"), end.v("nro"), end.v("xCpl")), juntar(" - ", end.v("xBairro"), cep(end.v("CEP"))),
			juntar(" - ", end.v("xMun"), end.v("UF")), juntar("  ", fone(end.v("fone")))} {
			if l != "" {
				pdf.SetX(tx)
				pdf.MultiCell(tw, 2.9, tr(l), "", "C", false)
			}
		}
		// DANFE
		xd := x0 + 80
		pdf.Rect(xd, y, 34, h, "D")
		pdf.SetXY(xd, y+1.5)
		pdf.SetFont("Helvetica", "B", 13)
		pdf.CellFormat(34, 5.5, "DANFE", "", 2, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 5.6)
		pdf.MultiCell(34, 2.5, tr("Documento Auxiliar da\nNota Fiscal Eletrônica"), "", "C", false)
		pdf.SetFont("Helvetica", "", 6.5)
		pdf.SetXY(xd+3, y+13)
		pdf.MultiCell(20, 2.9, tr("0 - ENTRADA\n1 - SAÍDA"), "", "L", false)
		pdf.Rect(xd+24, y+13, 6, 6, "D")
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetXY(xd+24, y+13)
		pdf.CellFormat(6, 6, ide.v("tpNF"), "", 0, "C", false, 0, "")
		pdf.SetXY(xd, y+20.5)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(34, 3.6, tr("Nº "+numeroNF(nNF)), "", 2, "C", false, 0, "")
		pdf.CellFormat(34, 3.6, tr("SÉRIE "+fmt.Sprintf("%03s", serie)), "", 2, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 6.5)
		pdf.CellFormat(34, 3.4, tr(fmt.Sprintf("FOLHA %d/{nb}", pdf.PageNo())), "", 2, "C", false, 0, "")
		// chave
		xc := x0 + 114
		pdf.Rect(xc, y, 82, h, "D")
		if c := fiscal.SoDigitos(chave); len(c) == 44 {
			mods := barras128C(c)
			total := 0
			for _, m := range mods {
				total += m
			}
			escala, bx := 76.0/float64(total), xc+3
			pdf.SetFillColor(0, 0, 0)
			for i, m := range mods {
				if i%2 == 0 {
					pdf.Rect(bx, y+1.8, float64(m)*escala, 11, "F")
				}
				bx += float64(m) * escala
			}
		}
		pdf.Line(xc, y+14.5, xc+82, y+14.5)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetFont("Helvetica", "", 5)
		pdf.SetXY(xc+0.8, y+15)
		pdf.CellFormat(80, 2.2, "CHAVE DE ACESSO", "", 2, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		tamChave := 8.5
		for pdf.SetFont("Helvetica", "B", tamChave); pdf.GetStringWidth(formatarChave(chave)) > 78 && tamChave > 5; pdf.SetFont("Helvetica", "B", tamChave) {
			tamChave -= 0.25
		}
		pdf.CellFormat(80, 4.2, formatarChave(chave), "", 2, "C", false, 0, "")
		pdf.Line(xc, y+22, xc+82, y+22)
		pdf.SetFont("Helvetica", "", 6.3)
		pdf.SetXY(xc+1, y+23.2)
		pdf.MultiCell(80, 2.8, tr("Consulta de autenticidade no portal nacional da NF-e www.nfe.fazenda.gov.br/portal ou no site da Sefaz Autorizadora"), "", "C", false)
		y += h
		protocolo := "-"
		if prot != nil {
			protocolo = juntar(" - ", prot.v("nProt"), formatarData(prot.v("dhRecbto")))
		}
		campo(x0, y, 114, 7.5, "Natureza da operação", ide.v("natOp"), "L", 8, true)
		campo(x0+114, y, 82, 7.5, "Protocolo de autorização de uso", protocolo, "C", 8, true)
		y += 7.5
		campo(x0, y, 65, 7.5, "Inscrição estadual", emit.v("IE"), "L", 8, false)
		campo(x0+65, y, 65, 7.5, "Inscrição estadual do subst. tributário", emit.v("IEST"), "L", 8, false)
		campo(x0+130, y, 66, 7.5, "CNPJ / CPF", formatarDoc(emit.v("CNPJ", "CPF")), "L", 8, false)
		return y + 7.5
	}

	colunas := []struct {
		t string
		w float64
		a string
	}{{"CÓDIGO", 15, "L"}, {"DESCRIÇÃO DO PRODUTO / SERVIÇO", 52, "L"}, {"NCM/SH", 12, "C"}, {"O/CST", 8, "C"}, {"CFOP", 8, "C"}, {"UN", 8, "C"},
		{"QUANT.", 13, "R"}, {"VALOR UNIT.", 14, "R"}, {"VALOR TOTAL", 14, "R"}, {"B.CÁLC. ICMS", 13, "R"}, {"VALOR ICMS", 11, "R"}, {"VALOR IPI", 10, "R"}, {"ALÍQ. ICMS", 9, "R"}, {"ALÍQ. IPI", 9, "R"}}
	cabItens := func(y float64) float64 {
		pdf.SetFillColor(234, 240, 245)
		pdf.SetFont("Helvetica", "B", 4.8)
		x := x0
		for _, c := range colunas {
			pdf.Rect(x, y, c.w, 5, "FD")
			pdf.SetXY(x, y+0.4)
			pdf.MultiCell(c.w, 2.1, tr(c.t), "", "C", false)
			x += c.w
		}
		return y + 5
	}

	pdf.AddPage()
	marcaDagua()
	y := 7.0
	if aviso != "" {
		pdf.SetTextColor(190, 20, 20)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetXY(x0, y)
		pdf.CellFormat(L, 4.5, tr(aviso), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 5
	}
	// Canhoto
	dataEmi := formatarData(ide.v("dhEmi", "dEmi"))
	pdf.Rect(x0, y, 160, 15, "D")
	pdf.SetFont("Helvetica", "", 6.2)
	pdf.SetXY(x0+1, y+1)
	pdf.MultiCell(158, 2.7, tr(fmt.Sprintf("RECEBEMOS DE %s OS PRODUTOS E/OU SERVIÇOS CONSTANTES DA NOTA FISCAL ELETRÔNICA INDICADA ABAIXO. EMISSÃO: %s  VALOR TOTAL: R$ %s  DESTINATÁRIO: %s",
		strings.ToUpper(emit.v("xNome")), dataEmi, num(tot.v("vNF"), 2), strings.ToUpper(dest.v("xNome")))), "", "L", false)
	campo(x0, y+7.5, 40, 7.5, "Data de recebimento", "", "L", 8, false)
	campo(x0+40, y+7.5, 120, 7.5, "Identificação e assinatura do recebedor", "", "L", 8, false)
	pdf.Rect(x0+160, y, 36, 15, "D")
	pdf.SetXY(x0+160, y+1.5)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(36, 5, "NF-e", "", 2, "C", false, 0, "")
	pdf.SetFont("Helvetica", "B", 8)
	pdf.CellFormat(36, 4, tr("Nº "+numeroNF(nNF)), "", 2, "C", false, 0, "")
	pdf.CellFormat(36, 4, tr("SÉRIE "+fmt.Sprintf("%03s", serie)), "", 2, "C", false, 0, "")
	y += 16.5
	pdf.SetDashPattern([]float64{1, 1}, 0)
	pdf.Line(x0, y, x0+L, y)
	pdf.SetDashPattern([]float64{}, 0)
	y += 1.5

	y = cabecalho(y)

	// Destinatário / remetente
	y = titulo(y+0.8, "DESTINATÁRIO / REMETENTE")
	de := dest.em("enderDest")
	campo(x0, y, 120, 7.5, "Nome / razão social", dest.v("xNome"), "L", 8, true)
	campo(x0+120, y, 46, 7.5, "CNPJ / CPF", formatarDoc(dest.v("CNPJ", "CPF", "idEstrangeiro")), "L", 8, false)
	campo(x0+166, y, 30, 7.5, "Data da emissão", dataBR(ide.v("dhEmi", "dEmi")), "C", 8, false)
	y += 7.5
	campo(x0, y, 95, 7.5, "Endereço", juntar(", ", de.v("xLgr"), de.v("nro"), de.v("xCpl")), "L", 8, false)
	campo(x0+95, y, 47, 7.5, "Bairro / distrito", de.v("xBairro"), "L", 8, false)
	campo(x0+142, y, 24, 7.5, "CEP", cep(de.v("CEP")), "C", 8, false)
	campo(x0+166, y, 30, 7.5, "Data da saída/entrada", dataBR(ide.v("dhSaiEnt", "dSaiEnt")), "C", 8, false)
	y += 7.5
	horaSai := ""
	if s := ide.v("dhSaiEnt"); len(s) >= 16 {
		horaSai = s[11:16]
	} else {
		horaSai = ide.v("hSaiEnt")
	}
	campo(x0, y, 72, 7.5, "Município", de.v("xMun"), "L", 8, false)
	campo(x0+72, y, 34, 7.5, "Fone / fax", fone(de.v("fone")), "L", 8, false)
	campo(x0+106, y, 12, 7.5, "UF", de.v("UF"), "C", 8, false)
	campo(x0+118, y, 48, 7.5, "Inscrição estadual", dest.v("IE"), "L", 8, false)
	campo(x0+166, y, 30, 7.5, "Hora da saída/entrada", horaSai, "C", 8, false)
	y += 7.5

	// Fatura / duplicatas
	var dups []*no
	if cobr := inf.em("cobr"); cobr != nil {
		for _, f := range cobr.filhos {
			if f.nome == "dup" {
				dups = append(dups, f)
			}
		}
		y = titulo(y+0.8, "FATURA / DUPLICATAS")
		if fat := cobr.em("fat"); fat != nil && len(dups) == 0 {
			campo(x0, y, L, 7.5, "Fatura", fmt.Sprintf("Nº %s · valor original R$ %s · desconto R$ %s · valor líquido R$ %s", fat.v("nFat"), num(fat.v("vOrig"), 2), num(fat.v("vDesc"), 2), num(fat.v("vLiq"), 2)), "L", 8, false)
			y += 7.5
		}
		const porLinha = 6
		cw := L / porLinha
		for i, d := range dups {
			if i > 0 && i%porLinha == 0 {
				y += 9
			}
			x := x0 + float64(i%porLinha)*cw
			pdf.Rect(x, y, cw, 9, "D")
			pdf.SetFont("Helvetica", "", 6.3)
			for k, l := range []string{"Nº " + d.v("nDup"), "Venc. " + dataBR(d.v("dVenc")), "R$ " + num(d.v("vDup"), 2)} {
				pdf.SetXY(x+1, y+0.6+float64(k)*2.7)
				pdf.CellFormat(cw-2, 2.7, tr(l), "", 0, "L", false, 0, "")
			}
		}
		if len(dups) > 0 {
			y += 9
		}
	}
	if pg := inf.em("pag"); pg != nil && len(dups) == 0 {
		var formas []string
		for _, dp := range pg.filhos {
			if dp.nome == "detPag" {
				formas = append(formas, juntar(" ", map[string]string{"01": "Dinheiro", "02": "Cheque", "03": "Cartão de crédito", "04": "Cartão de débito", "05": "Crédito loja", "10": "Vale alimentação", "11": "Vale refeição", "12": "Vale presente", "13": "Vale combustível", "15": "Boleto", "16": "Depósito", "17": "PIX", "18": "Transferência", "19": "Fidelidade", "90": "Sem pagamento", "99": "Outros"}[dp.v("tPag")], "R$ "+num(dp.v("vPag"), 2)))
			}
		}
		if len(formas) > 0 {
			y = titulo(y+0.8, "PAGAMENTO")
			campo(x0, y, L, 7, "Formas de pagamento", strings.Join(formas, " · "), "L", 8, false)
			y += 7
		}
	}

	// Cálculo do imposto
	y = titulo(y+0.8, "CÁLCULO DO IMPOSTO")
	l1 := [][2]string{{"Base de cálc. do ICMS", tot.v("vBC")}, {"Valor do ICMS", tot.v("vICMS")}, {"Base de cálc. ICMS ST", tot.v("vBCST")}, {"Valor do ICMS subst.", tot.v("vST")},
		{"V. imp. importação", tot.v("vII")}, {"V. ICMS UF remet.", tot.v("vICMSUFRemet")}, {"Valor do FCP", tot.v("vFCP")}, {"V. total produtos", tot.v("vProd")}}
	l2 := [][2]string{{"Valor do frete", tot.v("vFrete")}, {"Valor do seguro", tot.v("vSeg")}, {"Desconto", tot.v("vDesc")}, {"Outras despesas", tot.v("vOutro")},
		{"Valor total IPI", tot.v("vIPI")}, {"V. ICMS UF dest.", tot.v("vICMSUFDest")}, {"V. aprox. tributos", tot.v("vTotTrib")}, {"V. total da nota", tot.v("vNF")}}
	for k, ln := range [][][2]string{l1, l2} {
		cw := L / float64(len(ln))
		for i, c := range ln {
			v := num(c[1], 2)
			if v == "" {
				v = "0,00"
			}
			campo(x0+float64(i)*cw, y, cw, 7.5, c[0], v, "R", 8, k == 1 && i == len(ln)-1)
		}
		y += 7.5
	}

	// Transportador / volumes
	y = titulo(y+0.8, "TRANSPORTADOR / VOLUMES TRANSPORTADOS")
	ta, vei := transp.em("transporta"), transp.em("veicTransp")
	campo(x0, y, 70, 7.5, "Nome / razão social", ta.v("xNome"), "L", 7.5, false)
	campo(x0+70, y, 32, 7.5, "Frete", tab(tabModFrete, transp.v("modFrete")), "L", 7, false)
	campo(x0+102, y, 20, 7.5, "Código ANTT", vei.v("RNTC"), "C", 7.5, false)
	campo(x0+122, y, 22, 7.5, "Placa do veículo", vei.v("placa"), "C", 7.5, false)
	campo(x0+144, y, 10, 7.5, "UF", vei.v("UF"), "C", 7.5, false)
	campo(x0+154, y, 42, 7.5, "CNPJ / CPF", formatarDoc(ta.v("CNPJ", "CPF")), "L", 7.5, false)
	y += 7.5
	campo(x0, y, 92, 7.5, "Endereço", ta.v("xEnder"), "L", 7.5, false)
	campo(x0+92, y, 52, 7.5, "Município", ta.v("xMun"), "L", 7.5, false)
	campo(x0+144, y, 10, 7.5, "UF", ta.v("UF"), "C", 7.5, false)
	campo(x0+154, y, 42, 7.5, "Inscrição estadual", ta.v("IE"), "L", 7.5, false)
	y += 7.5
	vol := transp.em("vol")
	for i, c := range [][2]string{{"Quantidade", vol.v("qVol")}, {"Espécie", vol.v("esp")}, {"Marca", vol.v("marca")}, {"Numeração", vol.v("nVol")},
		{"Peso bruto", num(vol.v("pesoB"), 3)}, {"Peso líquido", num(vol.v("pesoL"), 3)}} {
		a := "L"
		if i == 0 || i >= 4 {
			a = "R"
		}
		campo(x0+float64(i)*L/6, y, L/6, 7.5, c[0], c[1], a, 7.5, false)
	}
	y += 7.5

	// Produtos
	y = titulo(y+0.8, "DADOS DOS PRODUTOS / SERVIÇOS")
	y = cabItens(y)
	issqn := inf.em("total/ISSQNtot")
	adic := juntar(" ", inf.v("infAdic/infCpl"))
	if f := inf.v("infAdic/infAdFisco"); f != "" {
		adic = juntar(" · ", adic, "Fisco: "+f)
	}
	if ia := inf.v("autXML/CNPJ", "autXML/CPF"); ia != "" {
		adic = juntar(" · ", adic, "Autorizado a obter o XML: "+formatarDoc(ia))
	}
	reservaFinal := 30.0
	if issqn != nil {
		reservaFinal += 11
	}
	pdf.SetFont("Helvetica", "", 6.2)
	for idx, it := range itens {
		dl := pdf.SplitLines([]byte(tr(it.desc)), colunas[1].w-1.4)
		h := float64(max(1, len(dl)))*2.7 + 1.2
		limite := rodape - 6
		if idx == len(itens)-1 {
			limite = rodape - reservaFinal
		}
		if y+h > limite {
			pdf.AddPage()
			marcaDagua()
			y = cabecalho(7)
			y = titulo(y+0.8, "DADOS DOS PRODUTOS / SERVIÇOS (continuação)")
			y = cabItens(y)
			pdf.SetFont("Helvetica", "", 6.2)
		}
		vals := []string{it.cod, "", it.ncm, it.cst, it.cfop, it.un, it.qtd, it.vun, it.vtot, it.bc, it.vicms, it.vipi, it.aicms, it.aipi}
		x := x0
		for i, c := range colunas {
			pdf.Rect(x, y, c.w, h, "D")
			if i == 1 {
				for k, l := range dl {
					pdf.SetXY(x+0.7, y+0.6+float64(k)*2.7)
					pdf.CellFormat(c.w-1.4, 2.7, string(l), "", 0, "L", false, 0, "")
				}
			} else {
				txt := tr(vals[i])
				for pdf.GetStringWidth(txt) > c.w-1.2 && len(txt) > 1 {
					txt = txt[:len(txt)-1]
				}
				pdf.SetXY(x+0.6, y+0.6)
				pdf.CellFormat(c.w-1.2, 2.7, txt, "", 0, c.a, false, 0, "")
			}
			x += c.w
		}
		y += h
	}

	// ISSQN e dados adicionais (sempre no fim, na mesma folha do último item)
	if y > rodape-reservaFinal {
		pdf.AddPage()
		marcaDagua()
		y = cabecalho(7)
	}
	if issqn != nil {
		y = titulo(y+0.8, "CÁLCULO DO ISSQN")
		campo(x0, y, 49, 7.5, "Inscrição municipal", emit.v("IM"), "L", 8, false)
		campo(x0+49, y, 49, 7.5, "Valor total dos serviços", num(issqn.v("vServ"), 2), "R", 8, false)
		campo(x0+98, y, 49, 7.5, "Base de cálculo do ISSQN", num(issqn.v("vBC"), 2), "R", 8, false)
		campo(x0+147, y, 49, 7.5, "Valor do ISSQN", num(issqn.v("vISS"), 2), "R", 8, false)
		y += 7.5
	}
	y = titulo(y+0.8, "DADOS ADICIONAIS")
	ha := min(max(22, rodape-y-1), 48.0)
	if n := len(pdf.SplitLines([]byte(tr(adic)), 128)); float64(n)*2.9+5 > ha {
		ha = min(rodape-y-1, float64(n)*2.9+5)
	}
	pdf.Rect(x0, y, 130, ha, "D")
	pdf.Rect(x0+130, y, 66, ha, "D")
	pdf.SetTextColor(80, 80, 80)
	pdf.SetFont("Helvetica", "", 5)
	pdf.SetXY(x0+0.8, y+0.5)
	pdf.CellFormat(100, 2.2, tr("INFORMAÇÕES COMPLEMENTARES"), "", 0, "L", false, 0, "")
	pdf.SetXY(x0+130.8, y+0.5)
	pdf.CellFormat(60, 2.2, "RESERVADO AO FISCO", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "", 6.5)
	pdf.SetXY(x0+0.8, y+3.2)
	pdf.MultiCell(128.4, 2.9, tr(ou(adic, "")), "", "L", false)
	pdf.SetFont("Helvetica", "", 5.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(x0, rodape+2)
	pdf.CellFormat(L, 3, tr("DANFE gerado pelo Vorticis Monitor a partir do XML da NF-e"), "", 0, "R", false, 0, "")
	return pdf.Output(w)
}
