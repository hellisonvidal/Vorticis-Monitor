package relatorios

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	"rsc.io/qr"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/store"
)

// DANFSe completo, lido direto do XML da NFS-e (Padrão Nacional — leiaute do
// DANFSe do Sistema Nacional NFS-e — ou ABRASF): prestador, tomador,
// intermediário, serviço com a descrição integral, tributação municipal e
// federal, retenções, valor líquido, tributos aproximados e informações
// complementares.

// logoNFSe é o logotipo da NFS-e (Nota Fiscal de Serviço eletrônica) do Sistema
// Nacional, impresso no cabeçalho do DANFSe.
//
//go:embed img/logo_nfse.png
var logoNFSe []byte

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
	paisPrest, paisResult, tpImunidade, nProcSusp, beneficio, vCalcBM, totFed, totEst, totMun                              string
}

var tabImunidade = map[string]string{"0": "Imunidade (tipo não informado)", "1": "Patrimônio, renda ou serviços de entes federativos", "2": "Templos de qualquer culto",
	"3": "Partidos, sindicatos, educação e assistência social sem fins lucrativos", "4": "Livros, jornais, periódicos e papel", "5": "Fonogramas e videofonogramas musicais"}

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
		d.exig = map[string]string{"1": "Suspensa por decisão judicial", "2": "Suspensa por processo administrativo"}[s]
		d.nProcSusp = tm.v("exigSusp/nProcesso")
	}
	if c := tm.v("tpImunidade"); c != "" {
		d.tpImunidade = tab(tabImunidade, c)
	}
	d.paisPrest, d.paisResult = dps.v("locPrest/cPaisPrestacao"), tm.v("cPaisResult")
	d.beneficio = tm.v("BM/nBM")
	d.vCalcBM = inf.v("valores/vCalcBM")
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
			d.totFed, d.totEst, d.totMun = moeda(tt.v("vTotTribFed")), moeda(tt.v("vTotTribEst")), moeda(tt.v("vTotTribMun"))
		case tt.em("pTotTrib") != nil:
			d.totFed, d.totEst, d.totMun = pct(tt.v("pTotTribFed")), pct(tt.v("pTotTribEst")), pct(tt.v("pTotTribMun"))
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
	d.retPC = "Não retido"
	if f, _ := strconv.ParseFloat(d.vPIS, 64); f > 0 {
		d.retPC = "Retido" // no ABRASF, PIS/COFINS informados são retenções
	}
	if f, _ := strconv.ParseFloat(d.vCOFINS, 64); f > 0 {
		d.retPC = "Retido"
	}
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

// DANFSe gera o PDF da NFS-e no formato do DANFSe v1.0 do Portal Nacional da
// NFS-e: cabeçalho com selo, título e município; chave com QR Code de
// autenticidade; seções separadas por linhas finas (Emitente, Tomador,
// Intermediário, Serviço Prestado, Tributação Municipal, Tributação Federal,
// Valor Total, Totais Aproximados dos Tributos, Informações Complementares) com
// os campos em grade de quatro colunas. Lido direto do XML (Nacional ou ABRASF).
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
	f := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	fs := func(v float64) string {
		if v == 0 {
			return ""
		}
		return strconv.FormatFloat(v, 'f', 2, 64)
	}
	// Retenções federais e PIS/COFINS de apuração própria, como no DANFSe nacional.
	retido := d.retPC == "Retido"
	contribRet, pcProprio := f(d.vCSLL), 0.0
	var descRet []string
	if retido {
		contribRet += f(d.vPIS) + f(d.vCOFINS)
		if f(d.vPIS) > 0 {
			descRet = append(descRet, "PIS")
		}
		if f(d.vCOFINS) > 0 {
			descRet = append(descRet, "COFINS")
		}
	} else {
		pcProprio = f(d.vPIS) + f(d.vCOFINS)
	}
	if f(d.vCSLL) > 0 {
		descRet = append(descRet, "CSLL")
	}
	totalRetFed := f(d.vIR) + f(d.vCP) + contribRet

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	const x0, L, fim = 10.0, 190.0, 284.0
	pdf.SetMargins(x0, 9, x0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(tr("DANFSe "+d.numero), false)
	pdf.SetCreator("Vorticis Monitor", false)
	cinza := func() { pdf.SetDrawColor(150, 150, 150); pdf.SetLineWidth(0.25) }
	cinza()

	type campo struct {
		span     int // colunas ocupadas (de 4)
		rot, val string
	}
	y := 9.0
	novaPagina := func() {
		pdf.AddPage()
		y = 9
	}
	// grade desenha uma linha de campos (rótulo em negrito pequeno, valor abaixo).
	grade := func(x, largura float64, cols int, cs ...campo) {
		cw := largura / float64(cols)
		alt := 0.0
		for _, c := range cs {
			pdf.SetFont("Helvetica", "", 7.2)
			n := len(pdf.SplitLines([]byte(tr(ou(c.val))), cw*float64(c.span)-1.5))
			alt = max(alt, 3+float64(max(1, n))*3.2+1.2)
		}
		if y+alt > fim {
			novaPagina()
		}
		cx := x
		for _, c := range cs {
			w := cw * float64(c.span)
			pdf.SetFont("Helvetica", "B", 5.8)
			pdf.SetXY(cx, y+0.4)
			pdf.CellFormat(w-1, 2.6, tr(c.rot), "", 0, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 7.2)
			pdf.SetXY(cx, y+3)
			if c.rot != "" || c.val != "" { // campo vazio sem rótulo = espaço
				pdf.MultiCell(w-1.5, 3.2, tr(ou(c.val)), "", "L", false)
			}
			cx += w
		}
		y += alt
	}
	secao := func(t, sub string) {
		if y+12 > fim {
			novaPagina()
		}
		pdf.Line(x0, y, x0+L, y)
		y += 1
		pdf.SetFont("Helvetica", "B", 7.2)
		pdf.SetXY(x0, y)
		pdf.CellFormat(L, 3.6, tr(t), "", 0, "L", false, 0, "")
		if sub != "" {
			pdf.SetFont("Helvetica", "", 6.5)
			pdf.SetXY(x0+pdf.GetStringWidth(tr(t))+30, y)
			pdf.CellFormat(80, 3.6, tr(sub), "", 0, "L", false, 0, "")
		}
		y += 4
	}

	pdf.AddPage()
	// Marca d'água (cancelada / sem validade)
	marca := map[string]string{"CANCELADA": "CANCELADA", "DENEGADA": "DENEGADA"}[doc.Status]
	if marca == "" && d.homologacao != "" {
		marca = "SEM VALIDADE JURÍDICA"
	}
	if marca != "" {
		pdf.SetAlpha(0.12, "Normal")
		pdf.SetTextColor(200, 30, 30)
		pdf.SetFont("Helvetica", "B", 48)
		pdf.TransformBegin()
		pdf.TransformRotate(38, 105, 160)
		pdf.SetXY(0, 152)
		pdf.CellFormat(210, 20, tr(marca), "", 0, "C", false, 0, "")
		pdf.TransformEnd()
		pdf.SetAlpha(1, "Normal")
		pdf.SetTextColor(0, 0, 0)
	}

	// Cabeçalho: logotipo da NFS-e | logotipo do prestador | título | município emissor
	pdf.RegisterImageOptionsReader("logo-nfse", fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, bytes.NewReader(logoNFSe))
	desenharLogo(pdf, "logo-nfse", x0, y-1, 25, 18.5)
	if lg := registrarLogo(pdf, fiscal.SoDigitos(d.prest.doc)); lg != "" {
		desenharLogo(pdf, lg, x0+28, y, 26, 15)
	}
	pdf.SetXY(x0+55, y+3)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(80, 5.5, "DANFSe v1.0", "", 2, "C", false, 0, "")
	pdf.SetFont("Helvetica", "B", 8)
	pdf.CellFormat(80, 4, tr("Documento Auxiliar da NFS-e"), "", 2, "C", false, 0, "")
	pdf.SetXY(x0+135, y+3)
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.MultiCell(55, 3.4, tr(ou(d.municipioEmissor)), "", "R", false)
	pdf.SetFont("Helvetica", "", 6.3)
	pdf.SetX(x0 + 135)
	sub := "Município emissor da NFS-e"
	if d.padrao == "ABRASF" {
		sub = "Prefeitura emissora (padrão ABRASF)"
	}
	pdf.MultiCell(55, 3, tr(sub), "", "R", false)
	y += 18
	if aviso := juntar(" · ", avisoStatus(doc), d.homologacao); aviso != "" {
		pdf.SetTextColor(190, 20, 20)
		pdf.SetFont("Helvetica", "B", 8.5)
		pdf.SetXY(x0, y)
		pdf.CellFormat(L, 4.5, tr(aviso), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 5
	}

	// Identificação + QR Code
	pdf.Line(x0, y, x0+L, y)
	y += 1
	yIni := y
	larg := 150.0
	grade(x0, larg, 3, campo{3, "Chave de Acesso da NFS-e", formatarChave(d.chave)})
	grade(x0, larg, 3, campo{1, "Número da NFS-e", d.numero}, campo{1, "Competência da NFS-e", d.competencia}, campo{1, "Data e Hora da emissão da NFS-e", d.emissaoNFSe})
	grade(x0, larg, 3, campo{1, "Número da DPS", d.numDPS}, campo{1, "Série da DPS", d.serieDPS}, campo{1, "Data e Hora da emissão da DPS", d.emissaoDPS})
	if d.codVerif != "" {
		grade(x0, larg, 3, campo{3, "Código de verificação", d.codVerif})
	}
	if c := fiscal.SoDigitos(d.chave); len(c) >= 44 && c == d.chave {
		if q, err := qr.Encode("https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave="+c, qr.M); err == nil {
			lado := 19.0
			m := lado / float64(q.Size)
			qx, qy := x0+larg+(40-lado)/2, yIni+0.5
			pdf.SetFillColor(0, 0, 0)
			for yy := 0; yy < q.Size; yy++ {
				for xx := 0; xx < q.Size; xx++ {
					if q.Black(xx, yy) {
						pdf.Rect(qx+float64(xx)*m, qy+float64(yy)*m, m+0.01, m+0.01, "F")
					}
				}
			}
			pdf.SetFont("Helvetica", "", 4.6)
			pdf.SetXY(x0+larg, qy+lado+0.5)
			pdf.MultiCell(40, 1.9, tr("A autenticidade desta NFS-e pode ser verificada pela leitura deste código QR ou pela consulta da chave de acesso no portal nacional da NFS-e"), "", "C", false)
			y = max(y, pdf.GetY()+0.5)
		}
	}

	parteGrade := func(titulo string, p parte, emitente bool) {
		secao(titulo, map[bool]string{true: "Prestador do Serviço", false: ""}[emitente])
		grade(x0, L, 4, campo{2, "CNPJ / CPF / NIF", p.doc}, campo{1, "Inscrição Municipal", p.im}, campo{1, "Telefone", fone(p.fone)})
		grade(x0, L, 4, campo{2, "Nome / Nome Empresarial", p.nome}, campo{2, "E-mail", p.email})
		grade(x0, L, 4, campo{2, "Endereço", p.ender}, campo{1, "Município", p.municipio}, campo{1, "CEP", p.cep})
		if emitente {
			grade(x0, L, 4, campo{2, "Simples Nacional na Data de Competência", d.simples}, campo{2, "Regime de Apuração Tributária pelo SN", d.regApSN})
		}
	}
	parteGrade("EMITENTE DA NFS-e", d.prest, true)
	if d.toma.nome != "" || d.toma.doc != "" {
		parteGrade("TOMADOR DO SERVIÇO", d.toma, false)
	} else {
		secao("TOMADOR DO SERVIÇO", "NÃO IDENTIFICADO NA NFS-e")
	}
	if d.interm.nome != "" || d.interm.doc != "" {
		parteGrade("INTERMEDIÁRIO DO SERVIÇO", d.interm, false)
	} else {
		secao("INTERMEDIÁRIO DO SERVIÇO", "NÃO IDENTIFICADO NA NFS-e")
	}

	secao("SERVIÇO PRESTADO", "")
	grade(x0, L, 4, campo{2, "Código de Tributação Nacional", juntar(" - ", d.cTribNac, d.xTribNac)}, campo{1, "Código de Tributação Municipal", juntar(" - ", d.cTribMun, d.xTribMun)},
		campo{1, "Local da Prestação", d.localPrestacao})
	grade(x0, L, 4, campo{2, "País da Prestação", d.paisPrest}, campo{1, "Código NBS", d.nbs}, campo{1, "CNAE", d.cnae})
	grade(x0, L, 4, campo{4, "Descrição do Serviço", d.descricao})

	secao("TRIBUTAÇÃO MUNICIPAL", "")
	grade(x0, L, 4, campo{1, "Tributação do ISSQN", d.tribISSQN}, campo{1, "País Resultado da Prestação do Serviço", d.paisResult},
		campo{1, "Município de Incidência do ISSQN", d.incidencia}, campo{1, "Regime Especial de Tributação", d.regEsp})
	grade(x0, L, 4, campo{1, "Tipo de Imunidade", d.tpImunidade}, campo{1, "Suspensão da Exigibilidade do ISSQN", ou(d.exig, "Não")},
		campo{1, "Número Processo Suspensão", d.nProcSusp}, campo{1, "Benefício Municipal", d.beneficio})
	grade(x0, L, 4, campo{1, "Valor do Serviço", moeda(d.vServ)}, campo{1, "Desconto Incondicionado", moeda(d.vDescIncond)},
		campo{1, "Total Deduções/Reduções", moeda(d.vDedRed)}, campo{1, "Cálculo do BM", moeda(d.vCalcBM)})
	grade(x0, L, 4, campo{1, "BC ISSQN", moeda(d.vBC)}, campo{1, "Alíquota Aplicada", pct(d.aliq)},
		campo{1, "Retenção do ISSQN", d.retISS}, campo{1, "ISSQN Apurado", moeda(d.vISS)})

	secao("TRIBUTAÇÃO FEDERAL", "")
	grade(x0, L, 4, campo{1, "IRRF", moeda(d.vIR)}, campo{1, "Contribuição Previdenciária - Retida", moeda(d.vCP)},
		campo{1, "Contribuições Sociais - Retidas", moeda(fs(contribRet))}, campo{1, "Descrição Contrib. Sociais - Retidas", strings.Join(descRet, " / ")})
	pisP, cofP := "", ""
	if !retido {
		pisP, cofP = d.vPIS, d.vCOFINS
	}
	grade(x0, L, 4, campo{1, "PIS - Débito Apuração Própria", moeda(pisP)}, campo{1, "COFINS - Débito Apuração Própria", moeda(cofP)})

	secao("VALOR TOTAL DA NFS-E", "")
	grade(x0, L, 4, campo{1, "Valor do Serviço", moeda(d.vServ)}, campo{1, "Desconto Condicionado", moeda(d.vDescCond)},
		campo{1, "Desconto Incondicionado", moeda(d.vDescIncond)}, campo{1, "ISSQN Retido", moeda(d.vISSRet)})
	liq := d.vLiq
	if liq == "" {
		liq = fmt.Sprintf("%.2f", doc.ValorTotal)
	}
	grade(x0, L, 4, campo{1, "Total das Retenções Federais", moeda(fs(totalRetFed))}, campo{1, "PIS/COFINS - Débito Apur. Própria", moeda(fs(pcProprio))},
		campo{1, "", ""}, campo{1, "Valor Líquido da NFS-e", moeda(liq)})

	secao("TOTAIS APROXIMADOS DOS TRIBUTOS", "")
	grade(x0, L, 3, campo{1, "Federais", d.totFed}, campo{1, "Estaduais", d.totEst}, campo{1, "Municipais", d.totMun})
	if d.totTrib != "" && d.totFed == "" && d.totEst == "" && d.totMun == "" {
		grade(x0, L, 3, campo{3, "Observação", d.totTrib})
	}

	secao("INFORMAÇÕES COMPLEMENTARES", "")
	grade(x0, L, 4, campo{4, "", juntar(" · ", d.infCompl)})
	pdf.Line(x0, y, x0+L, y)

	pdf.SetFont("Helvetica", "", 5.5)
	pdf.SetTextColor(120, 120, 120)
	pdf.SetXY(x0, fim+4)
	pdf.CellFormat(L, 3, tr(fmt.Sprintf("DANFSe gerado pelo Vorticis Monitor a partir do XML da NFS-e · situação no monitor: %s", doc.Status)), "", 0, "C", false, 0, "")
	return pdf.Output(w)
}
