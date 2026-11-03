package relatorios

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Formatos de saída suportados e respectivos content-types.
var Formatos = map[string]string{
	"pdf":  "application/pdf",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"csv":  "text/csv; charset=utf-8",
}

// Escrever gera o relatório no formato pedido.
func (r *Relatorio) Escrever(w io.Writer, formato string) error {
	switch formato {
	case "pdf":
		return r.pdf(w)
	case "xlsx":
		return r.xlsx(w)
	case "csv":
		return r.csv(w)
	}
	return fmt.Errorf("formato %q não suportado (use pdf, xlsx ou csv)", formato)
}

// ---------------- PDF ----------------

func (r *Relatorio) pdf(w io.Writer) error {
	pdf := fpdf.New("L", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 12)
	pdf.SetTitle(tr(r.Titulo), false)
	pdf.SetCreator("Vorticis Monitor", false)
	pdf.AliasNbPages("{nb}")
	var vis []int
	soma := 0.0
	for i, c := range r.Colunas {
		if c.Largura > 0 {
			vis = append(vis, i)
			soma += c.Largura
		}
	}
	esc := 277 / soma
	larg := func(i int) float64 { return r.Colunas[i].Largura * esc }
	alinh := func(i int) string {
		if t := r.Colunas[i].Tipo; t == M || t == N {
			return "R"
		}
		return "L"
	}
	gerado := time.Now().Format("02/01/2006 15:04")
	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetTextColor(8, 47, 73)
		pdf.CellFormat(200, 7, tr(r.Titulo), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(90, 100, 110)
		pdf.CellFormat(77, 7, tr("Vorticis Monitor · "+gerado), "", 1, "R", false, 0, "")
		pdf.CellFormat(277, 5, tr(r.Filtros), "", 1, "L", false, 0, "")
		pdf.Ln(1)
		pdf.SetFillColor(13, 116, 144)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 7.5)
		for _, i := range vis {
			pdf.CellFormat(larg(i), 6, tr(r.Colunas[i].Titulo), "", 0, alinh(i), true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetTextColor(20, 30, 40)
	})
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(120, 130, 140)
		pdf.CellFormat(0, 5, tr(fmt.Sprintf("Página %d de {nb}", pdf.PageNo())), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	linha := func(l []any, negrito, zebra bool) {
		estilo := ""
		if negrito {
			estilo = "B"
		}
		pdf.SetFont("Helvetica", estilo, 7.2)
		if zebra {
			pdf.SetFillColor(240, 245, 248)
		} else if negrito {
			pdf.SetFillColor(220, 232, 238)
		}
		for _, i := range vis {
			txt := tr(Texto(l[i], r.Colunas[i].Tipo))
			for pdf.GetStringWidth(txt) > larg(i)-1.5 && len(txt) > 1 {
				txt = txt[:len(txt)-1]
			}
			pdf.CellFormat(larg(i), 5, txt, "", 0, alinh(i), zebra || negrito, 0, "")
		}
		pdf.Ln(-1)
	}
	for n, l := range r.Linhas {
		linha(l, false, n%2 == 1)
	}
	if len(r.Linhas) == 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.CellFormat(277, 10, tr("Nenhum registro no período e filtros informados."), "", 1, "C", false, 0, "")
	}
	if r.Total != nil {
		linha(r.Total, true, false)
	}
	return pdf.Output(w)
}

// ---------------- CSV (padrão do Excel em português: ";" e vírgula decimal) ----------------

func (r *Relatorio) csv(w io.Writer) error {
	var b bytes.Buffer
	b.WriteString("\uFEFF")
	campo := func(s string) string {
		if strings.ContainsAny(s, ";\"\n\r") {
			return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
		}
		return s
	}
	val := func(v any) string {
		switch x := v.(type) {
		case float64:
			return strings.Replace(strconv.FormatFloat(x, 'f', 2, 64), ".", ",", 1)
		case time.Time:
			return x.Format("02/01/2006")
		}
		return campo(Texto(v, T))
	}
	t := make([]string, len(r.Colunas))
	for i, c := range r.Colunas {
		t[i] = campo(c.Titulo)
	}
	b.WriteString(strings.Join(t, ";") + "\r\n")
	for _, l := range append(r.Linhas, r.Total) {
		if l == nil {
			continue
		}
		for i := range t {
			t[i] = val(l[i])
		}
		b.WriteString(strings.Join(t, ";") + "\r\n")
	}
	_, err := w.Write(b.Bytes())
	return err
}

// ---------------- XLSX (Office Open XML) ----------------

func serialExcel(t time.Time) float64 {
	return float64(t.Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)).Hours() / 24)
}

func colunaExcel(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

func escXML(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c == '&':
			b.WriteString("&amp;")
		case c == '<':
			b.WriteString("&lt;")
		case c == '>':
			b.WriteString("&gt;")
		case c == '"':
			b.WriteString("&quot;")
		case c < 0x20 && c != '\t' && c != '\n' && c != '\r':
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

func larguraPlanilha(c Coluna) float64 {
	switch {
	case c.Titulo == "Chave de acesso":
		return 48
	case c.Tipo == M:
		return 15
	case c.Tipo == D:
		return 12
	case c.Largura >= 50:
		return 45
	}
	return math.Max(10, float64(len(c.Titulo))+4)
}

func (r *Relatorio) xlsx(w io.Writer) error {
	z := zip.NewWriter(w)
	add := func(nome, conteudo string) error {
		f, err := z.CreateHeader(&zip.FileHeader{Name: nome, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, conteudo)
		return err
	}
	const cab = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
	add("[Content_Types].xml", cab+`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`)
	add("_rels/.rels", cab+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`)
	add("xl/_rels/workbook.xml.rels", cab+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`)
	nomeAba := r.Titulo
	if len([]rune(nomeAba)) > 31 {
		nomeAba = string([]rune(nomeAba)[:31])
	}
	nomeAba = strings.NewReplacer("/", "-", "\\", "-", "?", "", "*", "", "[", "(", "]", ")", ":", "").Replace(nomeAba)
	add("xl/workbook.xml", cab+`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="`+escXML(nomeAba)+`" sheetId="1" r:id="rId1"/></sheets><definedNames><definedName name="_xlnm._FilterDatabase" localSheetId="0" hidden="1">'`+strings.ReplaceAll(escXML(nomeAba), "'", "''")+`'!$A$4:$`+colunaExcel(len(r.Colunas)-1)+`$`+strconv.Itoa(4+len(r.Linhas))+`</definedName></definedNames></workbook>`)
	// Estilos: 0 padrão, 1 cabeçalho, 2 moeda, 3 data, 4 título, 5 total texto, 6 total moeda, 7 inteiro, 8 total inteiro, 9 filtros
	add("xl/styles.xml", cab+`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><numFmts count="2"><numFmt numFmtId="164" formatCode="#,##0.00"/><numFmt numFmtId="165" formatCode="dd/mm/yyyy"/></numFmts>`+
		`<fonts count="5"><font><sz val="10"/><name val="Calibri"/></font><font><b/><sz val="10"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font><font><b/><sz val="14"/><color rgb="FF082F49"/><name val="Calibri"/></font><font><b/><sz val="10"/><name val="Calibri"/></font><font><i/><sz val="9"/><color rgb="FF5A646E"/><name val="Calibri"/></font></fonts>`+
		`<fills count="4"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF0D7490"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFDCE8EE"/></patternFill></fill></fills>`+
		`<borders count="1"><border/></borders><cellStyleXfs count="1"><xf/></cellStyleXfs><cellXfs count="10">`+
		`<xf/><xf fontId="1" fillId="2" applyFont="1" applyFill="1"/><xf numFmtId="164" applyNumberFormat="1"/><xf numFmtId="165" applyNumberFormat="1"/><xf fontId="2" applyFont="1"/>`+
		`<xf fontId="3" fillId="3" applyFont="1" applyFill="1"/><xf numFmtId="164" fontId="3" fillId="3" applyNumberFormat="1" applyFont="1" applyFill="1"/><xf numFmtId="1" applyNumberFormat="1"/><xf numFmtId="1" fontId="3" fillId="3" applyNumberFormat="1" applyFont="1" applyFill="1"/><xf fontId="4" applyFont="1"/>`+
		`</cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`)
	var b strings.Builder
	b.WriteString(cab + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols>`)
	for i, c := range r.Colunas {
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, i+1, i+1, larguraPlanilha(c))
	}
	b.WriteString(`</cols><sheetData>`)
	str := func(ref, s string, st int) {
		fmt.Fprintf(&b, `<c r="%s" t="inlineStr" s="%d"><is><t xml:space="preserve">%s</t></is></c>`, ref, st, escXML(s))
	}
	b.WriteString(`<row r="1">`)
	str("A1", r.Titulo, 4)
	b.WriteString(`</row><row r="2">`)
	str("A2", r.Filtros+" · gerado em "+time.Now().Format("02/01/2006 15:04"), 9)
	b.WriteString(`</row><row r="4">`)
	for i, c := range r.Colunas {
		str(colunaExcel(i)+"4", c.Titulo, 1)
	}
	b.WriteString(`</row>`)
	linha := func(n int, l []any, total bool) {
		fmt.Fprintf(&b, `<row r="%d">`, n)
		for i, v := range l {
			ref := colunaExcel(i) + strconv.Itoa(n)
			stTxt, stNum, stInt := 0, 2, 7
			if total {
				stTxt, stNum, stInt = 5, 6, 8
			}
			switch x := v.(type) {
			case float64:
				fmt.Fprintf(&b, `<c r="%s" s="%d"><v>%s</v></c>`, ref, stNum, strconv.FormatFloat(x, 'f', -1, 64))
			case int:
				fmt.Fprintf(&b, `<c r="%s" s="%d"><v>%d</v></c>`, ref, stInt, x)
			case time.Time:
				fmt.Fprintf(&b, `<c r="%s" s="3"><v>%.0f</v></c>`, ref, serialExcel(x))
			default:
				if s := Texto(v, T); s != "" || total {
					str(ref, s, stTxt)
				}
			}
		}
		b.WriteString(`</row>`)
	}
	for n, l := range r.Linhas {
		linha(5+n, l, false)
	}
	if r.Total != nil {
		linha(5+len(r.Linhas), r.Total, true)
	}
	fmt.Fprintf(&b, `</sheetData><autoFilter ref="A4:%s%d"/></worksheet>`, colunaExcel(len(r.Colunas)-1), 4+max(1, len(r.Linhas)))
	if err := add("xl/worksheets/sheet1.xml", b.String()); err != nil {
		return err
	}
	return z.Close()
}
