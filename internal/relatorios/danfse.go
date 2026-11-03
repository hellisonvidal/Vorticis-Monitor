package relatorios

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/store"
)

// DANFSe completo, lido direto do XML da NFS-e (Padrão Nacional — leiaute do
// DANFSe do Sistema Nacional NFS-e — ou ABRASF): prestador, tomador,
// intermediário, serviço com a descrição integral, tributação municipal e
// federal, retenções, valor líquido, tributos aproximados e informações
// complementares.

// no é um elemento do XML (nomes locais, sem namespace).
type no struct {
	nome   string
	texto  string
	filhos []*no
}

func lerArvore(b []byte) *no {
	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	raiz := &no{}
	pilha := []*no{raiz}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &no{nome: t.Name.Local}
			for _, a := range t.Attr {
				n.filhos = append(n.filhos, &no{nome: "@" + a.Name.Local, texto: a.Value})
			}
			p := pilha[len(pilha)-1]
			p.filhos = append(p.filhos, n)
			pilha = append(pilha, n)
		case xml.EndElement:
			if len(pilha) > 1 {
				pilha = pilha[:len(pilha)-1]
			}
		case xml.CharData:
			pilha[len(pilha)-1].texto += string(t)
		}
	}
	return raiz
}

// busca o primeiro descendente com o nome local.
func (n *no) busca(nome string) *no {
	if n == nil {
		return nil
	}
	for _, f := range n.filhos {
		if f.nome == nome {
			return f
		}
	}
	for _, f := range n.filhos {
		if r := f.busca(nome); r != nil {
			return r
		}
	}
	return nil
}

// em percorre o caminho "a/b/c" (cada parte é procurada entre os descendentes).
func (n *no) em(caminho string) *no {
	for _, p := range strings.Split(caminho, "/") {
		if n = n.busca(p); n == nil {
			return nil
		}
	}
	return n
}

// v devolve o texto do primeiro caminho existente.
func (n *no) v(caminhos ...string) string {
	for _, c := range caminhos {
		if x := n.em(c); x != nil {
			if t := strings.TrimSpace(x.texto); t != "" {
				return t
			}
		}
	}
	return ""
}

func moeda(s string) string {
	if s == "" {
		return "-"
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return s
	}
	return "R$ " + milhar(f)
}

func milhar(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	s := fmt.Sprintf("%.2f", f)
	inteiro, dec := s[:len(s)-3], s[len(s)-2:]
	var b strings.Builder
	for i, c := range inteiro {
		if i > 0 && (len(inteiro)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	r := b.String() + "," + dec
	if neg {
		r = "-" + r
	}
	return r
}

func pct(s string) string {
	if s == "" {
		return "-"
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return strings.Replace(strconv.FormatFloat(f, 'f', -1, 64), ".", ",", 1) + "%"
}

func ou(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return "-"
}

func juntar(sep string, v ...string) string {
	var p []string
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			p = append(p, s)
		}
	}
	return strings.Join(p, sep)
}

func cep(c string) string {
	c = fiscal.SoDigitos(c)
	if len(c) == 8 {
		return c[:5] + "-" + c[5:]
	}
	return c
}

var (
	tabTribISSQN = map[string]string{"1": "Operação tributável", "2": "Imunidade", "3": "Exportação de serviço", "4": "Não incidência"}
	tabRetISSQN  = map[string]string{"1": "Não retido", "2": "Retido pelo tomador", "3": "Retido pelo intermediário"}
	tabSimples   = map[string]string{"1": "Não optante", "2": "Optante - Microempreendedor Individual (MEI)", "3": "Optante - Microempresa ou Empresa de Pequeno Porte (ME/EPP)"}
	tabRegApSN   = map[string]string{"1": "Tributos federais e municipal pelo Simples Nacional", "2": "Tributos federais pelo SN e ISSQN fora do SN", "3": "Tributos federais e municipal fora do SN"}
	tabRegEsp    = map[string]string{"0": "Nenhum", "1": "Ato cooperado (cooperativa)", "2": "Estimativa", "3": "Microempresa municipal", "4": "Notário ou registrador", "5": "Profissional autônomo", "6": "Sociedade de profissionais"}
	tabRetPC     = map[string]string{"1": "Retido", "2": "Não retido"}
	tabExigISS   = map[string]string{"1": "Exigível", "2": "Não incidência", "3": "Isenção", "4": "Exportação", "5": "Imunidade", "6": "Exigibilidade suspensa por decisão judicial", "7": "Exigibilidade suspensa por processo administrativo"}
)

func tab(t map[string]string, c string) string {
	if c == "" {
		return "-"
	}
	if s, ok := t[c]; ok {
		return s
	}
	return c
}

// dadosDANFSe é o conteúdo do DANFSe, independente do padrão do XML.
type dadosDANFSe struct {
	padrao, chave, numero, competencia, emissaoNFSe, numDPS, serieDPS, emissaoDPS, municipioEmissor, homologacao, codVerif string
	prest, toma, interm                                                                                                    parte
	cTribNac, xTribNac, cTribMun, xTribMun, nbs, cnae, localPrestacao, descricao                                           string
	tribISSQN, incidencia, regEsp, exig, retISS, simples, regApSN                                                          string
	vServ, vDescIncond, vDescCond, vDedRed, vBC, aliq, vISS, vISSRet, vIR, vCP, vCSLL, vPIS, vCOFINS, retPC, vRetFed       string
	vTotRet, vLiq, totTrib, infCompl                                                                                       string
}

type parte struct{ doc, im, nome, ender, municipio, cep, fone, email string }

func extrairNacional(r *no) dadosDANFSe {
	inf := r.em("infNFSe")
	dps := inf.em("infDPS")
	d := dadosDANFSe{padrao: "NACIONAL"}
	d.chave = strings.TrimPrefix(inf.v("@Id"), "NFS")
	d.numero = inf.v("nNFSe")
	d.competencia = dataBR(dps.v("dCompet"))
	d.emissaoNFSe = formatarData(inf.v("dhProc"))
	d.numDPS, d.serieDPS, d.emissaoDPS = dps.v("nDPS"), dps.v("serie"), formatarData(dps.v("dhEmi"))
	d.municipioEmissor = inf.v("xLocEmi")
	if dps.v("tpAmb") == "2" {
		d.homologacao = "NFS-e SEM VALIDADE JURÍDICA (ambiente de homologação)"
	}
	// Nomes dos municípios citados na própria NFS-e (emissão, prestação e incidência).
	nomes := map[string]string{dps.v("cLocEmi"): inf.v("xLocEmi"), dps.v("locPrest/cLocPrestacao"): inf.v("xLocPrestacao"), inf.v("cLocIncid"): inf.v("xLocIncid")}
	end := func(e *no, municipio string) (string, string, string) {
		if e == nil {
			return "", "", ""
		}
		mun := municipio
		if c := e.v("cMun"); c != "" && mun == "" {
			if mun = nomes[c]; mun == "" {
				mun = "IBGE " + c
			}
		}
		if uf := e.v("UF"); uf != "" {
			mun = juntar(" / ", mun, uf)
		}
		if p := e.v("xPais", "cPais"); p != "" && e.v("cMun") == "" {
			mun = juntar(" / ", e.v("xCidade"), p)
		}
		return juntar(", ", e.v("xLgr"), e.v("nro"), e.v("xCpl"), e.v("xBairro")), mun, cep(e.v("CEP", "cEndPost"))
	}
	emit := inf.em("emit")
	d.prest = parte{doc: formatarDoc(emit.v("CNPJ", "CPF", "NIF")), im: emit.v("IM"), nome: juntar(" — ", emit.v("xNome"), emit.v("xFant")), fone: emit.v("fone"), email: emit.v("email")}
	d.prest.ender, d.prest.municipio, d.prest.cep = end(emit.em("enderNac"), inf.v("xLocEmi"))
	if d.prest.im == "" {
		d.prest.im = dps.v("prest/IM")
	}
	if d.prest.fone == "" {
		d.prest.fone, d.prest.email = dps.v("prest/fone"), dps.v("prest/email")
	}
	if t := dps.em("toma"); t != nil {
		d.toma = parte{doc: formatarDoc(t.v("CNPJ", "CPF", "NIF")), im: t.v("IM"), nome: t.v("xNome"), fone: t.v("fone"), email: t.v("email")}
		d.toma.ender, d.toma.municipio, d.toma.cep = end(t.em("end"), "")
	}
	if t := dps.em("interm"); t != nil {
		d.interm = parte{doc: formatarDoc(t.v("CNPJ", "CPF", "NIF")), im: t.v("IM"), nome: t.v("xNome"), fone: t.v("fone"), email: t.v("email")}
		d.interm.ender, d.interm.municipio, d.interm.cep = end(t.em("end"), "")
	}
	d.cTribNac, d.xTribNac = dps.v("cServ/cTribNac"), inf.v("xTribNac")
	d.cTribMun, d.xTribMun = dps.v("cServ/cTribMun"), inf.v("xTribMun")
	d.nbs = juntar(" — ", dps.v("cServ/cNBS"), inf.v("xNBS"))
	d.localPrestacao = ou(inf.v("xLocPrestacao"), dps.v("locPrest/cLocPrestacao"), dps.v("locPrest/cPaisPrestacao"))
	d.descricao = dps.v("cServ/xDescServ")
	tm := dps.em("tribMun")
	d.tribISSQN = tab(tabTribISSQN, tm.v("tribISSQN"))
	d.incidencia = ou(inf.v("xLocIncid"), inf.v("cLocIncid"))
	d.regEsp = tab(tabRegEsp, dps.v("regTrib/regEspTrib"))
	d.simples = tab(tabSimples, dps.v("regTrib/opSimpNac"))
	d.regApSN = tab(tabRegApSN, dps.v("regTrib/regApTribSN"))
	if s := tm.v("exigSusp/tpSusp"); s != "" {
		d.exig = "Exigibilidade suspensa (" + s + ") processo " + tm.v("exigSusp/nProcesso")
	}
	d.retISS = tab(tabRetISSQN, tm.v("tpRetISSQN"))
	vals := dps.em("valores")
	d.vServ = vals.v("vServPrest/vServ")
	d.vDescIncond, d.vDescCond = vals.v("vDescCondIncond/vDescIncond"), vals.v("vDescCondIncond/vDescCond")
	d.vDedRed = ou(inf.v("valores/vCalcDR"), vals.v("vDedRed/vDR"))
	if d.vDedRed == "-" {
		d.vDedRed = ""
	}
	d.vBC, d.aliq, d.vISS = inf.v("valores/vBC"), ou(inf.v("valores/pAliqAplic"), tm.v("pAliq")), inf.v("valores/vISSQN")
	if d.aliq == "-" {
		d.aliq = ""
	}
	if c := tm.v("tpRetISSQN"); c == "2" || c == "3" {
		d.vISSRet = d.vISS
	}
	tf := dps.em("tribFed")
	d.vIR, d.vCP, d.vCSLL = tf.v("vRetIRRF"), tf.v("vRetCP"), tf.v("vRetCSLL")
	d.vPIS, d.vCOFINS, d.retPC = tf.v("piscofins/vPis"), tf.v("piscofins/vCofins"), tab(tabRetPC, tf.v("piscofins/tpRetPisCofins"))
	d.vTotRet, d.vLiq = inf.v("valores/vTotalRet"), inf.v("valores/vLiq")
	if tt := dps.em("totTrib"); tt != nil {
		switch {
		case tt.em("vTotTrib") != nil:
			d.totTrib = fmt.Sprintf("Federais %s · Estaduais %s · Municipais %s", moeda(tt.v("vTotTribFed")), moeda(tt.v("vTotTribEst")), moeda(tt.v("vTotTribMun")))
		case tt.em("pTotTrib") != nil:
			d.totTrib = fmt.Sprintf("Federais %s · Estaduais %s · Municipais %s", pct(tt.v("pTotTribFed")), pct(tt.v("pTotTribEst")), pct(tt.v("pTotTribMun")))
		case tt.v("pTotTribSN") != "":
			d.totTrib = "Simples Nacional: " + pct(tt.v("pTotTribSN"))
		case tt.v("indTotTrib") == "0":
			d.totTrib = "Não informados (Decreto 8.264/2014)"
		}
	}
	d.infCompl = juntar(" · ", dps.v("infoCompl/xInfComp"), dps.v("infoCompl/xPed"), inf.v("xOutInf"))
	return d
}

func extrairABRASF(r *no) dadosDANFSe {
	inf := r.em("InfNfse")
	decl := inf.em("InfDeclaracaoPrestacaoServico")
	if decl == nil {
		decl = r.em("InfDeclaracaoPrestacaoServico")
	}
	d := dadosDANFSe{padrao: "ABRASF"}
	d.numero, d.codVerif = inf.v("Numero"), inf.v("CodigoVerificacao")
	d.emissaoNFSe = formatarData(inf.v("DataEmissao"))
	d.competencia = dataBR(ou(decl.v("Competencia"), inf.v("Competencia")))
	d.numDPS, d.serieDPS, d.emissaoDPS = decl.v("IdentificacaoRps/Numero"), decl.v("IdentificacaoRps/Serie"), formatarData(decl.v("Rps/DataEmissao"))
	d.municipioEmissor = juntar(" / ", inf.v("OrgaoGerador/CodigoMunicipio"), inf.v("OrgaoGerador/Uf"))
	if d.municipioEmissor != "" {
		d.municipioEmissor = "IBGE " + d.municipioEmissor
	}
	ender := func(e *no) (string, string, string) {
		if e == nil {
			return "", "", ""
		}
		mun := e.v("CodigoMunicipio")
		if mun != "" {
			mun = "IBGE " + mun
		}
		return juntar(", ", e.v("Endereco"), e.v("Numero"), e.v("Complemento"), e.v("Bairro")), juntar(" / ", mun, e.v("Uf")), cep(e.v("Cep"))
	}
	ps := inf.em("PrestadorServico")
	d.prest = parte{doc: formatarDoc(ou(ps.v("Cnpj", "Cpf"), decl.v("Prestador/Cnpj", "Prestador/Cpf"))), im: ou(ps.v("InscricaoMunicipal"), decl.v("Prestador/InscricaoMunicipal")),
		nome: juntar(" — ", ps.v("RazaoSocial"), ps.v("NomeFantasia")), fone: ps.v("Contato/Telefone"), email: ps.v("Contato/Email")}
	if ps != nil {
		d.prest.ender, d.prest.municipio, d.prest.cep = ender(ps.em("Endereco"))
	}
	t := decl.em("TomadorServico")
	if t == nil {
		t = decl.em("Tomador")
	}
	if t != nil {
		d.toma = parte{doc: formatarDoc(t.v("Cnpj", "Cpf")), im: t.v("InscricaoMunicipal"), nome: t.v("RazaoSocial"), fone: t.v("Contato/Telefone"), email: t.v("Contato/Email")}
		d.toma.ender, d.toma.municipio, d.toma.cep = ender(t.em("Endereco"))
	}
	if t := decl.em("Intermediario"); t != nil {
		d.interm = parte{doc: formatarDoc(t.v("Cnpj", "Cpf")), im: t.v("InscricaoMunicipal"), nome: t.v("RazaoSocial")}
	}
	s := decl.em("Servico")
	d.cTribNac, d.cTribMun, d.cnae = s.v("ItemListaServico"), s.v("CodigoTributacaoMunicipio"), s.v("CodigoCnae")
	d.descricao = s.v("Discriminacao")
	d.localPrestacao = ou(s.v("CodigoMunicipio"))
	if d.localPrestacao != "-" {
		d.localPrestacao = "IBGE " + d.localPrestacao
	}
	d.incidencia = ou(s.v("MunicipioIncidencia"), s.v("CodigoMunicipio"))
	d.exig = tab(tabExigISS, s.v("ExigibilidadeISS"))
	d.tribISSQN = d.exig
	d.exig = ""
	d.retISS = map[string]string{"1": "Retido", "2": "Não retido"}[s.v("IssRetido")]
	d.simples = map[string]string{"1": "Optante", "2": "Não optante"}[decl.v("OptanteSimplesNacional")]
	d.regEsp = tab(tabRegEsp, decl.v("RegimeEspecialTributacao"))
	v := s.em("Valores")
	d.vServ, d.vDedRed, d.vDescIncond, d.vDescCond = v.v("ValorServicos"), v.v("ValorDeducoes"), v.v("DescontoIncondicionado"), v.v("DescontoCondicionado")
	d.vBC = ou(inf.v("ValoresNfse/BaseCalculo"), v.v("BaseCalculo"))
	al := ou(inf.v("ValoresNfse/Aliquota"), v.v("Aliquota"))
	if f, err := strconv.ParseFloat(al, 64); err == nil && f > 0 && f < 1 {
		al = strconv.FormatFloat(f*100, 'f', -1, 64)
	}
	d.aliq = al
	d.vISS = ou(inf.v("ValoresNfse/ValorIss"), v.v("ValorIss"))
	if s.v("IssRetido") == "1" {
		d.vISSRet = d.vISS
	}
	d.vIR, d.vCP, d.vCSLL, d.vPIS, d.vCOFINS = v.v("ValorIr"), v.v("ValorInss"), v.v("ValorCsll"), v.v("ValorPis"), v.v("ValorCofins")
	d.retPC = "-"
	d.vLiq = ou(inf.v("ValoresNfse/ValorLiquidoNfse"), v.v("ValorLiquidoNfse"))
	d.infCompl = juntar(" · ", inf.v("OutrasInformacoes"), decl.v("InformacoesComplementares"))
	return d
}

func dataBR(s string) string {
	if len(s) >= 10 && s[4] == '-' {
		return s[8:10] + "/" + s[5:7] + "/" + s[:4]
	}
	return s
}

// DANFSe gera o PDF completo da NFS-e a partir do XML armazenado.
func DANFSe(w io.Writer, doc *store.Documento) error {
	r := lerArvore([]byte(doc.XML))
	var d dadosDANFSe
	if r.em("infNFSe") != nil {
		d = extrairNacional(r)
	} else {
		d = extrairABRASF(r)
	}
	if d.chave == "" && doc.Chave != "" && !strings.HasPrefix(doc.Chave, "NFSE") {
		d.chave = doc.Chave
	}
	if d.numero == "" {
		d.numero = doc.Numero
	}
	if d.prest.nome == "" {
		d.prest.nome, d.prest.doc = doc.EmitNome, formatarDoc(doc.EmitDoc)
	}
	if d.toma.nome == "" && doc.DestNome != "" {
		d.toma.nome, d.toma.doc = doc.DestNome, formatarDoc(doc.DestDoc)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	const x0, L = 8.0, 194.0
	pdf.SetMargins(x0, 8, x0)
	pdf.SetAutoPageBreak(true, 12)
	pdf.SetTitle(tr("NFS-e "+d.numero), false)
	pdf.SetCreator("Vorticis Monitor", false)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 6)
		pdf.CellFormat(L, 4, tr(fmt.Sprintf("DANFSe gerado pelo Vorticis Monitor a partir do XML da NFS-e - página %d", pdf.PageNo())), "", 0, "C", false, 0, "")
	})
	pdf.AddPage()

	// cel é um campo com rótulo e valor; a linha cresce até caber o maior valor.
	type cel struct {
		w             float64 // fração da largura
		rotulo, valor string
		negrito       bool
	}
	linha := func(cs ...cel) {
		const hr, hl = 3.0, 3.6
		alt := 0.0
		for _, c := range cs {
			pdf.SetFont("Helvetica", map[bool]string{true: "B", false: ""}[c.negrito], 7.5)
			n := len(pdf.SplitLines([]byte(tr(ou(c.valor))), c.w*L-2))
			alt = max(alt, hr+float64(max(1, n))*hl+1)
		}
		if pdf.GetY()+alt > 285 {
			pdf.AddPage()
		}
		y, x := pdf.GetY(), x0
		for _, c := range cs {
			cw := c.w * L
			pdf.Rect(x, y, cw, alt, "D")
			pdf.SetFont("Helvetica", "", 5.5)
			pdf.SetTextColor(70, 70, 70)
			pdf.SetXY(x+1, y+0.6)
			pdf.CellFormat(cw-2, 2.4, tr(strings.ToUpper(c.rotulo)), "", 0, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Helvetica", map[bool]string{true: "B", false: ""}[c.negrito], 7.5)
			pdf.SetXY(x+1, y+hr)
			pdf.MultiCell(cw-2, hl, tr(ou(c.valor)), "", "L", false)
			x += cw
		}
		pdf.SetXY(x0, y+alt)
	}
	secao := func(t string) {
		if pdf.GetY() > 270 {
			pdf.AddPage()
		}
		pdf.SetFillColor(225, 232, 238)
		pdf.SetFont("Helvetica", "B", 7.5)
		pdf.CellFormat(L, 5, tr(t), "1", 1, "L", true, 0, "")
	}

	// Cabeçalho
	if a := avisoStatus(doc); a != "" || d.homologacao != "" {
		pdf.SetTextColor(190, 20, 20)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(L, 6, tr(juntar(" · ", a, d.homologacao)), "", 1, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	y0 := pdf.GetY()
	pdf.Rect(x0, y0, L, 20, "D")
	pdf.SetXY(x0+2, y0+2)
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(60, 7, "DANFSe", "", 2, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 7)
	pdf.CellFormat(60, 3.5, tr("Documento Auxiliar da NFS-e"), "", 2, "L", false, 0, "")
	pdf.CellFormat(60, 3.5, tr(map[string]string{"NACIONAL": "Padrão Nacional (Sistema Nacional NFS-e)", "ABRASF": "Padrão ABRASF (prefeitura)"}[d.padrao]), "", 2, "L", false, 0, "")
	pdf.SetXY(x0+70, y0+2)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.CellFormat(122, 4, tr("Município emissor: "+ou(d.municipioEmissor)), "", 2, "R", false, 0, "")
	pdf.SetFont("Helvetica", "", 7)
	if len(d.chave) >= 44 {
		pdf.CellFormat(122, 4, tr("Consulta de autenticidade: https://www.nfse.gov.br/consultapublica"), "", 2, "R", false, 0, "")
	} else if d.codVerif != "" {
		pdf.CellFormat(122, 4, tr("Código de verificação: "+d.codVerif+" (consulte no site da prefeitura)"), "", 2, "R", false, 0, "")
	}
	if c := fiscal.SoDigitos(d.chave); len(c) >= 44 && c == d.chave {
		mods := barras128C(c)
		total := 0
		for _, m := range mods {
			total += m
		}
		escala, bx := 100.0/float64(total), x0+L-102
		pdf.SetFillColor(0, 0, 0)
		for i, m := range mods {
			if i%2 == 0 {
				pdf.Rect(bx, y0+10.5, float64(m)*escala, 8, "F")
			}
			bx += float64(m) * escala
		}
	}
	pdf.SetXY(x0, y0+20)
	if d.chave != "" {
		linha(cel{1, "Chave de acesso da NFS-e", formatarChave(d.chave), true})
	}
	linha(cel{0.2, "Número da NFS-e", d.numero, true}, cel{0.2, "Competência", d.competencia, false}, cel{0.2, "Data e hora da emissão da NFS-e", d.emissaoNFSe, false},
		cel{0.14, "Número da DPS / RPS", d.numDPS, false}, cel{0.1, "Série", d.serieDPS, false}, cel{0.16, "Emissão da DPS / RPS", d.emissaoDPS, false})

	partes := func(t string, p parte) {
		secao(t)
		linha(cel{0.24, "CNPJ / CPF / NIF", p.doc, true}, cel{0.18, "Inscrição municipal", p.im, false}, cel{0.58, "Nome / Nome empresarial", p.nome, true})
		linha(cel{0.5, "Endereço", p.ender, false}, cel{0.3, "Município", p.municipio, false}, cel{0.2, "CEP", p.cep, false})
		if p.fone != "" || p.email != "" {
			linha(cel{0.3, "Telefone", p.fone, false}, cel{0.7, "E-mail", p.email, false})
		}
	}
	partes("PRESTADOR DO SERVIÇO (EMITENTE DA NFS-e)", d.prest)
	linha(cel{0.4, "Simples Nacional na data de competência", d.simples, false}, cel{0.35, "Regime de apuração tributária pelo SN", ou(d.regApSN), false}, cel{0.25, "Regime especial de tributação", d.regEsp, false})
	if d.toma.nome != "" || d.toma.doc != "" {
		partes("TOMADOR DO SERVIÇO", d.toma)
	} else {
		secao("TOMADOR DO SERVIÇO")
		linha(cel{1, "Tomador", "Não identificado na NFS-e", false})
	}
	if d.interm.nome != "" || d.interm.doc != "" {
		partes("INTERMEDIÁRIO DO SERVIÇO", d.interm)
	}

	secao("SERVIÇO PRESTADO")
	linha(cel{0.5, "Código de tributação nacional", juntar(" - ", d.cTribNac, d.xTribNac), false}, cel{0.5, "Código de tributação municipal", juntar(" - ", d.cTribMun, d.xTribMun), false})
	linha(cel{0.4, "Local da prestação", d.localPrestacao, false}, cel{0.35, "NBS", d.nbs, false}, cel{0.25, "CNAE", d.cnae, false})
	linha(cel{1, "Descrição do serviço", d.descricao, false})

	secao("TRIBUTAÇÃO MUNICIPAL (ISSQN)")
	linha(cel{0.3, "Tributação do ISSQN", d.tribISSQN, false}, cel{0.3, "Município de incidência do ISSQN", d.incidencia, false}, cel{0.4, "Retenção do ISSQN", d.retISS, false})
	if d.exig != "" {
		linha(cel{1, "Exigibilidade", d.exig, false})
	}
	linha(cel{0.2, "Valor do serviço", moeda(d.vServ), true}, cel{0.2, "Desconto incondicionado", moeda(d.vDescIncond), false}, cel{0.2, "Deduções / reduções", moeda(d.vDedRed), false},
		cel{0.15, "Base de cálculo", moeda(d.vBC), false}, cel{0.1, "Alíquota", pct(d.aliq), false}, cel{0.15, "ISSQN apurado", moeda(d.vISS), true})

	secao("TRIBUTAÇÃO FEDERAL")
	linha(cel{0.16, "IRRF retido", moeda(d.vIR), false}, cel{0.16, "CP / INSS retido", moeda(d.vCP), false}, cel{0.16, "CSLL retida", moeda(d.vCSLL), false},
		cel{0.16, "PIS", moeda(d.vPIS), false}, cel{0.16, "COFINS", moeda(d.vCOFINS), false}, cel{0.2, "Retenção do PIS / COFINS", d.retPC, false})

	secao("VALOR TOTAL DA NFS-e")
	linha(cel{0.2, "Valor do serviço", moeda(d.vServ), false}, cel{0.2, "Desconto condicionado", moeda(d.vDescCond), false}, cel{0.2, "Desconto incondicionado", moeda(d.vDescIncond), false},
		cel{0.2, "ISSQN retido", moeda(d.vISSRet), false}, cel{0.2, "Total de retenções", moeda(d.vTotRet), false})
	linha(cel{0.6, "Valor aproximado dos tributos", d.totTrib, false}, cel{0.4, "Valor líquido da NFS-e", moeda(ou(d.vLiq, fmt.Sprintf("%.2f", doc.ValorTotal))), true})

	secao("INFORMAÇÕES COMPLEMENTARES")
	info := juntar(" · ", d.infCompl, "Situação no monitor: "+doc.Status)
	linha(cel{1, "Informações complementares", info, false})
	return pdf.Output(w)
}
