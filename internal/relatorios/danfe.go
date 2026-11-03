// Package relatorios gera a representação em PDF dos documentos (DANFE/DACTE/
// DAMDFE/DANFSe simplificados) e a exportação em lote (ZIP) de XML e PDF.
package relatorios

import (
	"fmt"
	"io"
	"strings"

	"github.com/go-pdf/fpdf"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/regras"
	"vorticis.com.br/monitor/internal/store"
)

// code128 contém os padrões de largura (barra/espaço) dos símbolos 0–106.
var code128 = strings.Fields(`212222 222122 222221 121223 121322 131222 122213 122312 132212 221213
221312 231212 112232 122132 122231 113222 123122 123221 223211 221132 221231 213212 223112 312131
311222 321122 321221 312212 322112 322211 212123 212321 232121 111323 131123 131321 112313 132113
132311 211313 231113 231311 112133 112331 132131 113123 113321 133121 313121 211331 231131 213113
213311 213131 311123 311321 331121 312113 312311 332111 314111 221411 431111 111224 111422 121124
121421 141122 141221 112214 112412 122114 122411 142112 142211 241211 221114 413111 241112 134111
111242 121142 121241 114212 124112 124211 411212 421112 421211 212141 214121 412121 111143 111341
131141 114113 114311 411113 411311 113141 114131 311141 411131 211412 211214 211232 2331112`)

// barras128C devolve as larguras (em módulos) de barras e espaços do Code 128-C.
func barras128C(num string) []int {
	num = fiscal.SoDigitos(num)
	if len(num)%2 == 1 {
		num = "0" + num
	}
	codes := []int{105}
	soma := 105
	for i := 0; i < len(num); i += 2 {
		val := int(num[i]-'0')*10 + int(num[i+1]-'0')
		codes = append(codes, val)
		soma += val * (i/2 + 1)
	}
	codes = append(codes, soma%103, 106)
	var out []int
	for _, c := range codes {
		for _, ch := range code128[c] {
			out = append(out, int(ch-'0'))
		}
	}
	return out
}

func f2(x float64) string { return strings.Replace(fmt.Sprintf("%.2f", x), ".", ",", 1) }

func formatarChave(c string) string {
	var p []string
	for i := 0; i < len(c); i += 4 {
		p = append(p, c[i:min(i+4, len(c))])
	}
	return strings.Join(p, " ")
}

func formatarDoc(d string) string {
	d = fiscal.NormalizaCNPJ(d)
	switch len(d) {
	case 14:
		return d[0:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:]
	case 11:
		return d[0:3] + "." + d[3:6] + "." + d[6:9] + "-" + d[9:]
	}
	return d
}

func formatarData(s string) string {
	t := regras.ParseData(s)
	if t.IsZero() {
		return s
	}
	return t.Format("02/01/2006 15:04")
}

func titulos(modelo string) (string, string) {
	switch modelo {
	case "NFCE":
		return "DANFE NFC-e", "Documento Auxiliar da NFC-e"
	case "CTE":
		return "DACTE", "Documento Auxiliar do CT-e"
	case "MDFE":
		return "DAMDFE", "Documento Auxiliar do MDF-e"
	case "NFSE":
		return "DANFSe", "Documento Auxiliar da NFS-e"
	}
	return "DANFE", "Documento Auxiliar da Nota Fiscal Eletrônica"
}

func avisoStatus(d *store.Documento) string {
	switch d.Status {
	case "CANCELADA":
		return "DOCUMENTO CANCELADO"
	case "DENEGADA":
		return "USO DENEGADO"
	case "CONTINGENCIA":
		return "EMITIDO EM CONTINGÊNCIA - PENDENTE DE AUTORIZAÇÃO"
	case "PENDENTE", "BLOQUEADA", "REJEITADA":
		return "SEM VALOR FISCAL - DOCUMENTO NÃO AUTORIZADO"
	}
	return ""
}

// PDF escreve a representação gráfica simplificada do documento em PDF (A4).
func PDF(w io.Writer, d *store.Documento, itens []store.Item) error {
	if d.Modelo == "NFSE" && strings.TrimSpace(d.XML) != "" {
		return DANFSe(w, d) // leiaute próprio e completo da NFS-e
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	pdf.SetMargins(8, 8, 8)
	pdf.SetAutoPageBreak(true, 12)
	pdf.SetTitle(tr(d.Modelo+" "+d.Numero), false)
	pdf.SetCreator("Vorticis Monitor", false)
	titulo, sub := titulos(d.Modelo)
	aviso := avisoStatus(d)
	const L = 194.0 // largura útil

	campo := func(x, y, w, h float64, rotulo, valor string, tam float64) {
		pdf.Rect(x, y, w, h, "D")
		pdf.SetFont("Helvetica", "", 5.5)
		pdf.SetXY(x+1, y+0.6)
		pdf.CellFormat(w-2, 2.6, tr(strings.ToUpper(rotulo)), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", tam)
		pdf.SetXY(x+1, y+3.2)
		pdf.CellFormat(w-2, h-3.6, tr(valor), "", 0, "L", false, 0, "")
	}

	cabecalho := func() {
		x0, y0 := 8.0, pdf.GetY()
		if aviso != "" {
			pdf.SetTextColor(190, 20, 20)
			pdf.SetFont("Helvetica", "B", 11)
			pdf.CellFormat(L, 6, tr(aviso), "", 1, "C", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			y0 = pdf.GetY()
		}
		// emitente
		pdf.Rect(x0, y0, 78, 30, "D")
		pdf.SetFont("Helvetica", "", 5.5)
		pdf.SetXY(x0+1, y0+0.8)
		pdf.CellFormat(76, 2.6, "EMITENTE", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 9.5)
		pdf.SetXY(x0+1, y0+5)
		pdf.MultiCell(76, 4.2, tr(d.EmitNome), "", "L", false)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetX(x0 + 1)
		pdf.CellFormat(76, 4, tr("CNPJ/CPF: "+formatarDoc(d.EmitDoc)+"   UF: "+d.EmitUF), "", 1, "L", false, 0, "")
		// título
		pdf.Rect(x0+78, y0, 36, 30, "D")
		pdf.SetXY(x0+78, y0+2)
		pdf.SetFont("Helvetica", "B", 13)
		pdf.CellFormat(36, 6, tr(titulo), "", 2, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 6)
		pdf.MultiCell(36, 2.8, tr(sub), "", "C", false)
		pdf.SetX(x0 + 78)
		pdf.SetFont("Helvetica", "", 8)
		mov := "1 - SAÍDA"
		if d.Direcao == "E" {
			mov = "0 - ENTRADA"
		}
		pdf.CellFormat(36, 4.5, tr(mov), "", 2, "C", false, 0, "")
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(36, 4.5, tr("Nº "+d.Numero), "", 2, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(36, 4, tr("Série "+d.Serie), "", 2, "C", false, 0, "")
		// chave + barras
		xc := x0 + 114
		pdf.Rect(xc, y0, 80, 30, "D")
		if len(d.Chave) == 44 && fiscal.SoDigitos(d.Chave) == d.Chave {
			mods := barras128C(d.Chave)
			total := 0
			for _, m := range mods {
				total += m
			}
			escala := 74.0 / float64(total)
			bx := xc + 3
			pdf.SetFillColor(0, 0, 0)
			for i, m := range mods {
				if i%2 == 0 {
					pdf.Rect(bx, y0+2, float64(m)*escala, 12, "F")
				}
				bx += float64(m) * escala
			}
		}
		pdf.SetFont("Helvetica", "", 5.5)
		pdf.SetXY(xc+1, y0+15.5)
		pdf.CellFormat(78, 2.6, "CHAVE DE ACESSO", "", 2, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(78, 4, formatarChave(d.Chave), "", 2, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 6)
		pdf.MultiCell(78, 2.8, tr("Consulta de autenticidade no portal nacional da NF-e (www.nfe.fazenda.gov.br/portal) ou no site da SEFAZ autorizadora"), "", "C", false)
		y := y0 + 30
		campo(x0, y, 114, 9, "Natureza da operação", d.Natureza, 8)
		campo(x0+114, y, 80, 9, "Protocolo de autorização de uso", strings.TrimSpace(d.Protocolo+"  "+formatarData(d.DhAutorizacao)), 8)
		y += 9
		campo(x0, y, 104, 9, "Destinatário / Remetente", d.DestNome, 8)
		campo(x0+104, y, 40, 9, "CNPJ/CPF", formatarDoc(d.DestDoc), 8)
		campo(x0+144, y, 12, 9, "UF", d.DestUF, 8)
		campo(x0+156, y, 38, 9, "Data de emissão", formatarData(d.DhEmissao), 8)
		y += 9
		type par struct {
			r string
			v float64
		}
		vals := []par{{"Valor ICMS", d.ValorICMS}, {"Valor ICMS-ST", d.ValorST}, {"Valor IPI", d.ValorIPI}, {"PIS", d.ValorPIS}, {"COFINS", d.ValorCOFINS}}
		if d.ValorDIFAL > 0 {
			vals = append(vals, par{"ICMS UF destino", d.ValorDIFAL})
		}
		if d.ValorISS > 0 {
			vals = append(vals, par{"ISS", d.ValorISS})
		}
		vals = append(vals, par{"Valor total", d.ValorTotal})
		cw := L / float64(len(vals))
		for i, v := range vals {
			campo(x0+float64(i)*cw, y, cw, 9, v.r, f2(v.v), 8.5)
		}
		pdf.SetY(y + 11)
	}

	colunas := []struct {
		t string
		w float64
		a string
	}{{"CÓD.", 18, "L"}, {"DESCRIÇÃO", 56, "L"}, {"NCM", 14, "C"}, {"CST", 9, "C"}, {"CFOP", 9, "C"}, {"UN", 8, "C"},
		{"QTD", 14, "R"}, {"V. UNIT", 15, "R"}, {"V. TOTAL", 16, "R"}, {"BC ICMS", 14, "R"}, {"ICMS", 11, "R"}, {"% ICMS", 10, "R"}}
	cabItens := func() {
		pdf.SetFont("Helvetica", "B", 6)
		pdf.SetFillColor(230, 236, 240)
		for _, c := range colunas {
			pdf.CellFormat(c.w, 5, tr(c.t), "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() > 1 {
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(L, 5, tr(fmt.Sprintf("%s nº %s - continuação - chave %s", titulo, d.Numero, d.Chave)), "", 1, "L", false, 0, "")
			cabItens()
		}
	})
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 6)
		pdf.CellFormat(L, 4, tr(fmt.Sprintf("Gerado pelo Vorticis Monitor a partir do XML - representação simplificada - página %d", pdf.PageNo())), "", 0, "C", false, 0, "")
	})
	pdf.AddPage()
	cabecalho()
	if len(itens) > 0 {
		pdf.SetFont("Helvetica", "B", 7)
		pdf.CellFormat(L, 4, tr("DADOS DOS PRODUTOS / SERVIÇOS"), "", 1, "L", false, 0, "")
		cabItens()
		pdf.SetFont("Helvetica", "", 6.5)
		for _, it := range itens {
			vals := []string{it.CodProd, it.Descricao, it.NCM, it.CSTICMS, it.CFOP, it.Unidade,
				strings.Replace(fmt.Sprintf("%.4f", it.Quantidade), ".", ",", 1), strings.Replace(fmt.Sprintf("%.4f", it.ValorUnit), ".", ",", 1),
				f2(it.ValorTotal), f2(it.VBC), f2(it.VICMS), f2(it.PICMS)}
			for i, c := range colunas {
				txt := tr(vals[i])
				for pdf.GetStringWidth(txt) > c.w-1.5 && len(txt) > 1 {
					txt = txt[:len(txt)-1]
				}
				pdf.CellFormat(c.w, 4.5, txt, "1", 0, c.a, false, 0, "")
			}
			pdf.Ln(-1)
		}
	}
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.CellFormat(L, 4, "DADOS ADICIONAIS", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 7)
	info := "Situação: " + d.Status
	if d.XMotivo != "" {
		info += fmt.Sprintf(" (%d - %s)", d.CStat, d.XMotivo)
	}
	if d.TpEmis != "" && d.TpEmis != "1" {
		info += ". Forma de emissão: " + fiscal.DescricaoTpEmis[d.TpEmis]
	}
	pdf.MultiCell(L, 3.6, tr(info+"."), "1", "L", false)
	return pdf.Output(w)
}
