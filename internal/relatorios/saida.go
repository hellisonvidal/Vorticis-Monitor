package relatorios

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/go-pdf/fpdf"
)

// Formatos de saída suportados e respectivos content-types.
var Formatos = map[string]string{
	"pdf":  "application/pdf",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"xls":  "application/vnd.ms-excel",
	"csv":  "text/csv; charset=utf-8",
}

// Escrever gera o relatório no formato pedido.
func (r *Relatorio) Escrever(w io.Writer, formato string) error {
	switch formato {
	case "pdf":
		return r.pdf(w)
	case "xlsx":
		return r.xlsx(w)
	case "xls":
		return r.xls(w)
	case "csv":
		return r.csv(w)
	}
	return fmt.Errorf("formato %q não suportado (use pdf, xlsx, xls ou csv)", formato)
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

// ---------------- XLS (Excel 97-2003, BIFF8 em Compound File) ----------------

type biff struct{ bytes.Buffer }

func (b *biff) rec(tipo uint16, dados ...any) {
	var p bytes.Buffer
	for _, d := range dados {
		binary.Write(&p, binary.LittleEndian, d)
	}
	binary.Write(&b.Buffer, binary.LittleEndian, tipo)
	binary.Write(&b.Buffer, binary.LittleEndian, uint16(p.Len()))
	b.Write(p.Bytes())
}

// strXL codifica XLUnicodeString (cch de 16 bits) em UTF-16, limitado a 255 caracteres.
func strXL(s string, cch8 bool) []byte {
	u := utf16.Encode([]rune(s))
	if len(u) > 255 {
		u = u[:255]
	}
	var p bytes.Buffer
	if cch8 {
		p.WriteByte(byte(len(u)))
	} else {
		binary.Write(&p, binary.LittleEndian, uint16(len(u)))
	}
	p.WriteByte(1) // fHighByte: caracteres de 16 bits
	binary.Write(&p, binary.LittleEndian, u)
	return p.Bytes()
}

func (r *Relatorio) xls(w io.Writer) error {
	if len(r.Linhas)+6 > 65536 {
		return errors.New("o formato XLS (Excel 97-2003) aceita até 65.536 linhas — use XLSX ou CSV")
	}
	bof := func(b *biff, dt uint16) {
		b.rec(0x0809, uint16(0x0600), dt, uint16(0x0DBB), uint16(0x07CC), uint32(0), uint32(0x06))
	}
	// ---- globais
	g := &biff{}
	bof(g, 0x0005)
	g.rec(0x0042, uint16(1200))                                                                                                          // CODEPAGE UTF-16
	g.rec(0x003D, uint16(0), uint16(0), uint16(0x4000), uint16(0x2000), uint16(0x0038), uint16(0), uint16(0), uint16(1), uint16(0x0258)) // WINDOW1
	g.rec(0x0022, uint16(0))                                                                                                             // DATEMODE 1900
	fonte := func(altura, peso uint16) {
		g.rec(0x0031, altura, uint16(0), uint16(0x7FFF), peso, uint16(0), uint8(0), uint8(0), uint8(0), uint8(0), strXL("Arial", true))
	}
	for i := 0; i < 4; i++ { // 0..3 (o índice 4 não existe no BIFF)
		fonte(200, 400)
	}
	fonte(200, 700) // 5 negrito
	fonte(280, 700) // 6 título
	xf := func(fnt, fmtID, flags uint16, usado uint8) {
		g.rec(0x00E0, fnt, fmtID, flags, uint8(0x20), uint8(0), uint8(0), usado, uint32(0), uint32(0), uint16(0x20C0))
	}
	for i := 0; i < 15; i++ { // XF de estilo 0..14
		xf(0, 0, 0xFFF5, 0)
	}
	xf(0, 0, 0x0001, 0) // 15 célula padrão
	const (
		xfMoeda = 16 + iota
		xfData
		xfNegrito
		xfNegritoMoeda
		xfTitulo
		xfInteiro
		xfNegritoInteiro
	)
	xf(0, 4, 0x0001, 0xFC)  // #,##0.00
	xf(0, 14, 0x0001, 0xFC) // data
	xf(5, 0, 0x0001, 0xFC)
	xf(5, 4, 0x0001, 0xFC)
	xf(6, 0, 0x0001, 0xFC)
	xf(0, 1, 0x0001, 0xFC)
	xf(5, 1, 0x0001, 0xFC)
	g.rec(0x0293, uint16(0x8000), uint8(0), uint8(0xFF)) // STYLE Normal
	nomeAba := []rune(r.Titulo)
	if len(nomeAba) > 31 {
		nomeAba = nomeAba[:31]
	}
	nome := strXL(strings.NewReplacer("/", "-", "?", "", "*", "", "[", "(", "]", ")", ":", "").Replace(string(nomeAba)), true)
	posBound := g.Len() + 4
	g.rec(0x0085, uint32(0), uint8(0), uint8(0), nome) // BOUNDSHEET (posição ajustada abaixo)
	g.rec(0x000A)
	// ---- planilha
	s := &biff{}
	bof(s, 0x0010)
	ultima := 4 + len(r.Linhas)
	if r.Total != nil {
		ultima++
	}
	s.rec(0x0200, uint32(0), uint32(ultima), uint16(0), uint16(len(r.Colunas)), uint16(0))
	for i, c := range r.Colunas {
		s.rec(0x007D, uint16(i), uint16(i), uint16(larguraPlanilha(c)*256), uint16(15), uint16(0), uint16(0))
	}
	label := func(l, c int, txt string, x uint16) { s.rec(0x0204, uint16(l), uint16(c), x, strXL(txt, false)) }
	numero := func(l, c int, v float64, x uint16) { s.rec(0x0203, uint16(l), uint16(c), x, v) }
	label(0, 0, r.Titulo, xfTitulo)
	label(1, 0, r.Filtros+" · gerado em "+time.Now().Format("02/01/2006 15:04"), 15)
	for i, c := range r.Colunas {
		label(3, i, c.Titulo, xfNegrito)
	}
	linha := func(n int, l []any, total bool) {
		for i, v := range l {
			switch x := v.(type) {
			case float64:
				numero(n, i, x, map[bool]uint16{false: xfMoeda, true: xfNegritoMoeda}[total])
			case int:
				numero(n, i, float64(x), map[bool]uint16{false: xfInteiro, true: xfNegritoInteiro}[total])
			case time.Time:
				numero(n, i, serialExcel(x), xfData)
			default:
				if t := Texto(v, T); t != "" {
					label(n, i, t, map[bool]uint16{false: 15, true: xfNegrito}[total])
				}
			}
		}
	}
	for n, l := range r.Linhas {
		linha(4+n, l, false)
	}
	if r.Total != nil {
		linha(4+len(r.Linhas), r.Total, true)
	}
	s.rec(0x023E, uint16(0x06B6), uint16(0), uint16(0), uint32(0x40), uint16(0), uint16(0), uint32(0)) // WINDOW2
	s.rec(0x000A)
	wb := g.Bytes()
	binary.LittleEndian.PutUint32(wb[posBound:], uint32(len(wb)))
	wb = append(wb, s.Bytes()...)
	return escreverCFB(w, wb)
}

// escreverCFB grava o fluxo "Workbook" num Compound File Binary (setores de 512 bytes).
func escreverCFB(w io.Writer, fluxo []byte) error {
	const setor = 512
	tam := len(fluxo)
	if len(fluxo) < 4096 { // evita o mini-stream
		fluxo = append(fluxo, make([]byte, 4096-len(fluxo))...)
		tam = 4096
	}
	nW := (len(fluxo) + setor - 1) / setor
	fluxo = append(fluxo, make([]byte, nW*setor-len(fluxo))...)
	nF := 1
	for (nW+1+nF+127)/128 > nF {
		nF++
	}
	if nF > 109 {
		return errors.New("relatório grande demais para XLS — use XLSX ou CSV")
	}
	const fim, livre, fatSect, nada = 0xFFFFFFFE, 0xFFFFFFFF, 0xFFFFFFFD, 0xFFFFFFFF
	dirSect := uint32(nW)
	fat := make([]uint32, nF*128)
	for i := range fat {
		fat[i] = livre
	}
	for i := 0; i < nW; i++ {
		fat[i] = uint32(i + 1)
	}
	fat[nW-1] = fim
	fat[dirSect] = fim
	for i := 0; i < nF; i++ {
		fat[nW+1+i] = fatSect
	}
	var h bytes.Buffer
	le := func(v any) { binary.Write(&h, binary.LittleEndian, v) }
	h.Write([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	h.Write(make([]byte, 16))
	le(uint16(0x003E))
	le(uint16(0x0003))
	le(uint16(0xFFFE))
	le(uint16(9))
	le(uint16(6))
	h.Write(make([]byte, 6))
	le(uint32(0))
	le(uint32(nF))
	le(dirSect)
	le(uint32(0))
	le(uint32(0x1000))
	le(uint32(fim))
	le(uint32(0))
	le(uint32(fim))
	le(uint32(0))
	for i := 0; i < 109; i++ {
		if i < nF {
			le(uint32(nW + 1 + i))
		} else {
			le(uint32(livre))
		}
	}
	entrada := func(nome string, tipo uint8, filho, inicio uint32, tamanho uint32) {
		u := utf16.Encode([]rune(nome))
		n := make([]uint16, 32)
		copy(n, u)
		le(n)
		if nome == "" {
			le(uint16(0))
		} else {
			le(uint16((len(u) + 1) * 2))
		}
		le(tipo)
		le(uint8(1))
		le(uint32(nada))
		le(uint32(nada))
		le(filho)
		h.Write(make([]byte, 16+4+8+8))
		le(inicio)
		le(tamanho)
		le(uint32(0))
	}
	if _, err := w.Write(h.Bytes()); err != nil {
		return err
	}
	if _, err := w.Write(fluxo); err != nil {
		return err
	}
	h.Reset()
	entrada("Root Entry", 5, 1, fim, 0)
	entrada("Workbook", 2, nada, 0, uint32(tam))
	entrada("", 0, nada, 0, 0)
	entrada("", 0, nada, 0, 0)
	le(fat)
	_, err := w.Write(h.Bytes())
	return err
}
