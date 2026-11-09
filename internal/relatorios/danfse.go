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
	"vorticis.com.br/monitor/internal/regras"
	"vorticis.com.br/monitor/internal/store"
)

// DANFSe v2.0 (NT SE/CGNFS-e nº 008/2026), lido direto do XML da NFS-e
// (Padrão Nacional, com os grupos de IBS/CBS da Reforma Tributária, ou ABRASF):
// identificação, prestador/fornecedor, tomador/adquirente, destinatário,
// intermediário, serviço, ISSQN, tributos federais, IBS/CBS, valor total,
// informações complementares e canhoto, em uma única página A4.

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
	return strings.Replace(strconv.FormatFloat(f, 'f', 2, 64), ".", ",", 1) + "%"
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

// Descrições das opções do leiaute da NFS-e, como pede a NT 008/2026.
var (
	tabTribISSQN = map[string]string{"1": "Operação Tributável", "2": "Imunidade", "3": "Exportação de Serviço", "4": "Não Incidência"}
	tabRetISSQN  = map[string]string{"1": "Não Retido", "2": "Retido pelo Tomador", "3": "Retido pelo Intermediário"}
	tabSimples   = map[string]string{"1": "Não Optante", "2": "Optante - Microempreendedor Individual (MEI)", "3": "Optante - Microempresa ou Empresa de Pequeno Porte (ME/EPP)"}
	tabRegApSN   = map[string]string{"1": "Regime de apuração dos tributos federais e municipal pelo Simples Nacional",
		"2": "Regime de apuração dos tributos federais pelo SN e o ISSQN pela NFS-e conforme respectiva legislação municipal do tributo",
		"3": "Regime de apuração dos tributos federais e municipal pela NFS-e conforme respectivas legislações federal e municipal de cada tributo"}
	tabRegEsp = map[string]string{"0": "Nenhum", "1": "Ato Cooperado (Cooperativa)", "2": "Estimativa", "3": "Microempresa Municipal", "4": "Notário ou Registrador",
		"5": "Profissional Autônomo", "6": "Sociedade de Profissionais", "9": "Outros"}
	tabRetPC = map[string]string{"0": "PIS/COFINS/CSLL Não Retidos", "1": "PIS/COFINS Retido", "2": "PIS/COFINS Não Retido", "3": "PIS/COFINS/CSLL Retidos",
		"4": "PIS/COFINS Retidos, CSLL Não Retido", "5": "PIS Retido, COFINS/CSLL Não Retido", "6": "COFINS Retido, PIS/CSLL Não Retido",
		"7": "PIS Não Retido, COFINS/CSLL Retidos", "8": "PIS/COFINS Não Retidos, CSLL Retido", "9": "COFINS Não Retido, PIS/CSLL Retidos"}
	tabExigISS   = map[string]string{"1": "Exigível", "2": "Não Incidência", "3": "Isenção", "4": "Exportação", "5": "Imunidade", "6": "Exigibilidade Suspensa por Decisão Judicial", "7": "Exigibilidade Suspensa por Processo Administrativo"}
	tabSusp      = map[string]string{"1": "Exigibilidade Suspensa por Decisão Judicial", "2": "Exigibilidade Suspensa por Processo Administrativo"}
	tabImunidade = map[string]string{"0": "Imunidade (tipo não informado)", "1": "Patrimônio, renda ou serviços, uns dos outros", "2": "Templos de qualquer culto",
		"3": "Patrimônio, renda ou serviços dos partidos políticos, sindicatos e instituições sem fins lucrativos", "4": "Livros, jornais, periódicos e o papel destinado a sua impressão",
		"5": "Fonogramas e videofonogramas musicais produzidos no Brasil"}
	tabBM     = map[string]string{"1": "Isenção", "2": "Redução da BC em Percentual", "3": "Redução da BC em Valor", "4": "Alíquota Diferenciada"}
	tabEmit   = map[string]string{"1": "Prestador", "2": "Tomador", "3": "Intermediário"}
	tabStat   = map[string]string{"100": "NFS-e Gerada", "101": "NFS-e de Substituição Gerada", "102": "NFS-e de Decisão Judicial ou Administrativa", "103": "NFS-e Avulsa"}
	tabFin    = map[string]string{"0": "NFS-e regular"}
	tabAmbGer = map[string]string{"1": "Prefeitura", "2": "Sistema Nacional NFS-e"}
	tabTpAmb  = map[string]string{"1": "Produção", "2": "Homologação"}
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

// parte é um participante da operação (prestador, tomador, destinatário, intermediário).
type parte struct{ doc, im, nome, ender, municipio, ibgeCep, fone, email string }

func (p parte) vazia() bool { return p.doc == "" && p.nome == "" }

// dadosDANFSe é o conteúdo do DANFSe, independente do padrão do XML. Os
// campos já vêm prontos para impressão (descrições das opções, máscaras).
type dadosDANFSe struct {
	padrao string
	// cabeçalho e identificação
	municipioEmissor, ambGer, tpAmb, homologacao                                string
	chave, numero, competencia, emissaoNFSe, numDPS, serieDPS, emissaoDPS       string
	emitente, situacao, finalidade, codVerif                                    string
	anoCompet                                                                   int
	prest, toma, dest, interm                                                   parte
	destTomador                                                                 bool // destinatário é o próprio tomador
	simples, regApSN                                                            string
	codTrib, nbs, localPrestacao, descCodTrib, descricao                        string
	item99                                                                      bool // código de tributação nacional 99: sem município no cabeçalho
	semISSQN                                                                    bool
	tribISSQN, incidencia, regEsp, tpImunidade, exig, nProcSusp, retISS         string
	beneficio, vCalcBM, vDedRed, vDescIncond, vDescCond, vBC, aliq, vISS, vServ string
	vIR, vCP, vCSLL, vPIS, vCOFINS, tpRetPC                                     string
	vTotRet, vLiq                                                               string
	// IBS/CBS
	ibscbs                                                        bool
	cst, cClassTrib, cIndOp, cLocIncidIBS, xLocIncidIBS           string
	vReeRepRes, vBCIBS, pRedUF, pRedMun, pRedCBS, pIBSUF, pIBSMun string
	pEfetMun, vIBSMun, pEfetUF, vIBSUF, vIBSTot, pCBS, pEfetCBS   string
	vCBS, vTotNF                                                  string
	// informações complementares
	infCompl []string
	totais   string
}

// filho devolve o filho direto com o nome (sem descer na árvore).
func (n *no) filho(nome string) *no {
	if n == nil {
		return nil
	}
	for _, f := range n.filhos {
		if f.nome == nome {
			return f
		}
	}
	return nil
}

func ufDoMunicipio(cMun string) string {
	if len(cMun) >= 2 {
		return fiscal.UFPorCodigo[cMun[:2]]
	}
	return ""
}

func codTrib(nac, mun string) string {
	if len(nac) == 6 {
		nac = nac[:2] + "." + nac[2:4] + "." + nac[4:]
	}
	return juntar(" / ", nac, mun)
}

func mascaraNBS(c string) string {
	if len(c) == 9 {
		return c[:1] + "." + c[1:5] + "." + c[5:7] + "." + c[7:]
	}
	return c
}

func dataHora(s string) string {
	t := regras.ParseData(s)
	if t.IsZero() {
		return s
	}
	return t.Format("02/01/2006 15:04:05")
}

func numf(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	return f
}

func valor(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }

func extrairNacional(r *no) dadosDANFSe {
	inf := r.em("infNFSe")
	dps := inf.em("infDPS")
	d := dadosDANFSe{padrao: "NACIONAL"}
	d.chave = strings.TrimPrefix(inf.v("@Id"), "NFS")
	d.numero = inf.v("nNFSe")
	d.competencia = dataBR(dps.v("dCompet"))
	d.anoCompet, _ = strconv.Atoi(ou(dps.v("dCompet"), "0000")[:4])
	d.emissaoNFSe = dataHora(inf.v("dhProc"))
	d.numDPS, d.serieDPS, d.emissaoDPS = dps.v("nDPS"), dps.v("serie"), dataHora(dps.v("dhEmi"))
	emit := inf.filho("emit")
	ufEmi := ou(emit.v("enderNac/UF"), ufDoMunicipio(dps.v("cLocEmi")))
	if m := inf.v("xLocEmi"); m != "" {
		d.municipioEmissor = juntar(" - ", m, strings.Trim(ufEmi, "-"))
	}
	d.ambGer, d.tpAmb = tab(tabAmbGer, inf.v("ambGer")), tab(tabTpAmb, dps.v("tpAmb"))
	if dps.v("tpAmb") == "2" {
		d.homologacao = "NFS-e SEM VALIDADE JURÍDICA"
	}
	d.emitente, d.situacao = tab(tabEmit, dps.v("tpEmit")), tab(tabStat, inf.v("cStat"))
	ibsDPS := dps.filho("IBSCBS")
	d.finalidade = tab(tabFin, ibsDPS.v("finNFSe"))

	// Nomes dos municípios citados na própria NFS-e (emissão, prestação e incidências).
	ibsN := inf.filho("IBSCBS")
	nomes := map[string]string{dps.v("cLocEmi"): inf.v("xLocEmi"), dps.v("locPrest/cLocPrestacao"): inf.v("xLocPrestacao"),
		inf.v("cLocIncid"): inf.v("xLocIncid"), ibsN.v("cLocalidadeIncid"): ibsN.v("xLocalidadeIncid")}
	end := func(p *parte, e *no) {
		if e == nil {
			return
		}
		if c := e.v("cMun"); c != "" {
			p.municipio = juntar(" / ", ou(nomes[c]), ou(e.v("UF"), ufDoMunicipio(c)))
			p.ibgeCep = juntar(" / ", c, cep(e.v("CEP")))
		} else if e.v("cEndPost", "xCidade", "cPais") != "" {
			p.municipio = juntar(" / ", e.v("xCidade"), e.v("xEstProvReg"), e.v("cPais"))
			p.ibgeCep = juntar(" / ", "-", e.v("cEndPost"))
		}
		p.ender = juntar(", ", e.v("xLgr"), e.v("nro"), e.v("xCpl"), e.v("xBairro"))
	}
	pessoa := func(n *no) parte {
		return parte{doc: formatarDoc(n.v("CNPJ", "CPF", "NIF")), im: n.v("IM"), nome: n.v("xNome"), fone: n.v("fone"), email: n.v("email")}
	}
	prestDPS := dps.filho("prest")
	d.prest = pessoa(emit)
	if emit == nil {
		d.prest = pessoa(prestDPS)
	}
	if d.prest.im == "" {
		d.prest.im = prestDPS.v("IM")
	}
	if d.prest.fone == "" {
		d.prest.fone, d.prest.email = prestDPS.v("fone"), prestDPS.v("email")
	}
	if e := emit.filho("enderNac"); e != nil {
		end(&d.prest, e)
	} else {
		end(&d.prest, prestDPS.filho("end"))
	}
	if t := dps.filho("toma"); t != nil {
		d.toma = pessoa(t)
		end(&d.toma, t.filho("end"))
	}
	if t := ibsDPS.filho("dest"); t != nil {
		d.dest = pessoa(t)
		end(&d.dest, t.filho("end"))
	} else {
		d.destTomador = !d.toma.vazia()
	}
	if t := dps.filho("interm"); t != nil {
		d.interm = pessoa(t)
		end(&d.interm, t.filho("end"))
	}
	reg := prestDPS.filho("regTrib")
	d.simples, d.regApSN = tab(tabSimples, reg.v("opSimpNac")), tab(tabRegApSN, reg.v("regApTribSN"))
	d.regEsp = tab(tabRegEsp, reg.v("regEspTrib"))

	serv := dps.filho("serv")
	cNac := serv.v("cServ/cTribNac")
	d.item99 = strings.HasPrefix(cNac, "99")
	d.codTrib = codTrib(cNac, serv.v("cServ/cTribMun"))
	d.nbs = mascaraNBS(serv.v("cServ/cNBS"))
	d.descCodTrib = ou(inf.v("xTribMun"), inf.v("xTribNac"))
	if c := serv.v("locPrest/cLocPrestacao"); c != "" {
		d.localPrestacao = juntar(" / ", ou(inf.v("xLocPrestacao")), ufDoMunicipio(c), "BR")
	} else if p := serv.v("locPrest/cPaisPrestacao"); p != "" {
		d.localPrestacao = juntar(" / ", inf.v("xLocPrestacao"), p)
	}
	d.descricao = serv.v("cServ/xDescServ")

	vals := dps.filho("valores")
	valN := inf.filho("valores")
	tm := vals.em("trib/tribMun")
	d.semISSQN = tm == nil
	d.tribISSQN = tab(tabTribISSQN, tm.v("tribISSQN"))
	if c := inf.v("cLocIncid"); c != "" {
		d.incidencia = juntar(" / ", ou(inf.v("xLocIncid")), ufDoMunicipio(c), "BR")
	} else if p := tm.v("cPaisResult"); p != "" {
		d.incidencia = juntar(" / ", inf.v("xLocIncid"), p)
	}
	if c := tm.v("tpImunidade"); c != "" {
		d.tpImunidade = tab(tabImunidade, c)
	}
	if s := tm.v("exigSusp/tpSusp"); s != "" {
		d.exig, d.nProcSusp = tab(tabSusp, s), tm.v("exigSusp/nProcesso")
	}
	if c := tm.v("BM/tpBM"); c != "" {
		d.beneficio = tab(tabBM, c)
	}
	d.vCalcBM = valN.v("vCalcBM", "")
	if d.vCalcBM == "" {
		d.vCalcBM = tm.v("BM/vRedBCBM")
	}
	if dr := ou(vals.v("vDedRed/vDR"), valN.v("vCalcDR")); dr != "-" || ibsN.v("valores/vCalcReeRepRes") != "" {
		d.vDedRed = valor(numf(dr) + numf(ibsN.v("valores/vCalcReeRepRes")))
	}
	d.retISS = tab(tabRetISSQN, tm.v("tpRetISSQN"))
	d.vServ = vals.v("vServPrest/vServ")
	d.vDescIncond, d.vDescCond = vals.v("vDescCondIncond/vDescIncond"), vals.v("vDescCondIncond/vDescCond")
	d.vBC, d.vISS = valN.v("vBC"), valN.v("vISSQN")
	if d.aliq = valN.v("pAliqAplic"); d.aliq == "" {
		d.aliq = tm.v("pAliq")
	}
	tf := vals.em("trib/tribFed")
	d.vIR, d.vCP, d.vCSLL = tf.v("vRetIRRF"), tf.v("vRetCP"), tf.v("vRetCSLL")
	d.vPIS, d.vCOFINS, d.tpRetPC = tf.v("piscofins/vPis"), tf.v("piscofins/vCofins"), tf.v("piscofins/tpRetPisCofins")
	d.vTotRet, d.vLiq = valN.v("vTotalRet"), valN.v("vLiq")

	// IBS/CBS (Reforma Tributária)
	if ibsN != nil || ibsDPS != nil {
		d.ibscbs = true
		g := ibsDPS.em("valores/trib/gIBSCBS")
		d.cst, d.cClassTrib, d.cIndOp = g.v("CST"), g.v("cClassTrib"), ibsDPS.v("cIndOp")
		d.cLocIncidIBS, d.xLocIncidIBS = ibsN.v("cLocalidadeIncid"), ibsN.v("xLocalidadeIncid")
		v := ibsN.filho("valores")
		d.vReeRepRes, d.vBCIBS = v.v("vCalcReeRepRes"), v.v("vBC")
		d.pRedUF, d.pRedMun, d.pRedCBS = v.v("uf/pRedAliqUF"), v.v("mun/pRedAliqMun"), v.v("fed/pRedAliqCBS")
		d.pIBSUF, d.pIBSMun, d.pCBS = v.v("uf/pIBSUF"), v.v("mun/pIBSMun"), v.v("fed/pCBS")
		d.pEfetUF, d.pEfetMun, d.pEfetCBS = v.v("uf/pAliqEfetUF"), v.v("mun/pAliqEfetMun"), v.v("fed/pAliqEfetCBS")
		t := ibsN.filho("totCIBS")
		d.vTotNF = t.v("vTotNF")
		d.vIBSTot, d.vIBSUF, d.vIBSMun = t.v("gIBS/vIBSTot"), t.v("gIBS/gIBSUFTot/vIBSUF"), t.v("gIBS/gIBSMunTot/vIBSMun")
		d.vCBS = t.v("gCBS/vCBS")
	}

	// Informações complementares, na ordem da NT 008 (separadas por "|").
	ic := serv.filho("infoCompl")
	add := func(rot, v string) {
		if v = strings.TrimSpace(v); v != "" {
			d.infCompl = append(d.infCompl, rot+" "+v)
		}
	}
	add("Inf. Cont.:", ic.v("xInfComp"))
	add("NFS-e Subst.:", dps.v("subst/chSubstda"))
	add("Doc. Ref.:", ic.v("docRef"))
	add("Cod. Obra:", serv.v("obra/cObra"))
	add("Insc. Imob.:", serv.v("obra/inscImobFisc")+ibsDPS.v("imovel/inscImobFisc"))
	add("Cod. Evt.:", serv.v("atvEvento/idAtvEvt"))
	add("Doc. Tec.:", ic.v("idDocTec"))
	add("Núm. Ped.:", ic.v("xPed"))
	var itens []string
	if g := ic.filho("gItemPed"); g != nil {
		for _, it := range g.filhos {
			if s := strings.TrimSpace(it.texto); s != "" {
				itens = append(itens, s)
			}
		}
	}
	add("Item Ped.:", strings.Join(itens, ", "))
	add("Inf. A. T. Mun.:", inf.v("xOutInf"))
	if tt := vals.em("trib/totTrib"); tt != nil {
		switch {
		case tt.filho("vTotTrib") != nil:
			d.totais = fmt.Sprintf("Federais: %s ; Estaduais: %s ; Municipais: %s", moeda(tt.v("vTotTrib/vTotTribFed")), moeda(tt.v("vTotTrib/vTotTribEst")), moeda(tt.v("vTotTrib/vTotTribMun")))
		case tt.filho("pTotTrib") != nil:
			d.totais = fmt.Sprintf("Federais: %s ; Estaduais: %s ; Municipais: %s", pct(tt.v("pTotTrib/pTotTribFed")), pct(tt.v("pTotTrib/pTotTribEst")), pct(tt.v("pTotTrib/pTotTribMun")))
		case tt.v("pTotTribSN") != "":
			d.totais = "Simples Nacional: " + pct(tt.v("pTotTribSN"))
		}
	}
	return d
}

func extrairABRASF(r *no) dadosDANFSe {
	inf := r.em("InfNfse")
	decl := inf.em("InfDeclaracaoPrestacaoServico")
	if decl == nil {
		decl = r.em("InfDeclaracaoPrestacaoServico")
	}
	d := dadosDANFSe{padrao: "ABRASF", finalidade: "-", situacao: "-", emitente: "Prestador"}
	d.numero, d.codVerif = inf.v("Numero"), inf.v("CodigoVerificacao")
	d.emissaoNFSe = dataHora(inf.v("DataEmissao"))
	comp := ou(decl.v("Competencia"), inf.v("Competencia"))
	d.competencia = dataBR(comp)
	if len(comp) >= 4 {
		d.anoCompet, _ = strconv.Atoi(comp[:4])
	}
	d.numDPS, d.serieDPS, d.emissaoDPS = decl.v("IdentificacaoRps/Numero"), decl.v("IdentificacaoRps/Serie"), dataHora(decl.v("Rps/DataEmissao"))
	if c := inf.v("OrgaoGerador/CodigoMunicipio"); c != "" {
		d.municipioEmissor = juntar(" - ", "IBGE "+c, ou(inf.v("OrgaoGerador/Uf"), ufDoMunicipio(c)))
	}
	d.ambGer = "Prefeitura (padrão ABRASF)"
	ender := func(p *parte, e *no) {
		if e == nil {
			return
		}
		c := e.v("CodigoMunicipio")
		p.ender = juntar(", ", e.v("Endereco"), e.v("Numero"), e.v("Complemento"), e.v("Bairro"))
		p.municipio = juntar(" / ", ou(""), ou(e.v("Uf"), ufDoMunicipio(c)))
		p.ibgeCep = juntar(" / ", c, cep(e.v("Cep")))
	}
	ps := inf.em("PrestadorServico")
	d.prest = parte{doc: formatarDoc(ou(ps.v("Cnpj", "Cpf"), decl.v("Prestador/Cnpj", "Prestador/Cpf"))), im: ou(ps.v("InscricaoMunicipal"), decl.v("Prestador/InscricaoMunicipal")),
		nome: ps.v("RazaoSocial"), fone: ps.v("Contato/Telefone"), email: ps.v("Contato/Email")}
	if d.prest.doc == "-" {
		d.prest.doc = ""
	}
	if d.prest.im == "-" {
		d.prest.im = ""
	}
	ender(&d.prest, ps.em("Endereco"))
	t := decl.em("TomadorServico")
	if t == nil {
		t = decl.em("Tomador")
	}
	if t != nil {
		d.toma = parte{doc: formatarDoc(t.v("Cnpj", "Cpf")), im: t.v("InscricaoMunicipal"), nome: t.v("RazaoSocial"), fone: t.v("Contato/Telefone"), email: t.v("Contato/Email")}
		ender(&d.toma, t.em("Endereco"))
		d.destTomador = true
	}
	if t := decl.em("Intermediario"); t != nil {
		d.interm = parte{doc: formatarDoc(t.v("Cnpj", "Cpf")), im: t.v("InscricaoMunicipal"), nome: t.v("RazaoSocial")}
	}
	s := decl.em("Servico")
	d.codTrib = juntar(" / ", s.v("ItemListaServico"), s.v("CodigoTributacaoMunicipio"))
	d.descricao = s.v("Discriminacao")
	if c := s.v("CodigoMunicipio"); c != "" {
		d.localPrestacao = juntar(" / ", "IBGE "+c, ufDoMunicipio(c), "BR")
	}
	if c := ou(s.v("MunicipioIncidencia"), s.v("CodigoMunicipio")); c != "-" {
		d.incidencia = juntar(" / ", "IBGE "+c, ufDoMunicipio(c), "BR")
	}
	d.tribISSQN = tab(tabExigISS, s.v("ExigibilidadeISS"))
	d.retISS = map[string]string{"1": "Retido", "2": "Não Retido"}[s.v("IssRetido")]
	d.simples = map[string]string{"1": "Optante", "2": "Não Optante"}[decl.v("OptanteSimplesNacional")]
	d.regEsp = tab(tabRegEsp, decl.v("RegimeEspecialTributacao"))
	v := s.em("Valores")
	d.vServ, d.vDedRed, d.vDescIncond, d.vDescCond = v.v("ValorServicos"), v.v("ValorDeducoes"), v.v("DescontoIncondicionado"), v.v("DescontoCondicionado")
	d.vBC = ou(inf.v("ValoresNfse/BaseCalculo"), v.v("BaseCalculo"))
	al := ou(inf.v("ValoresNfse/Aliquota"), v.v("Aliquota"))
	if f, err := strconv.ParseFloat(al, 64); err == nil && f > 0 && f < 1 {
		al = strconv.FormatFloat(f*100, 'f', -1, 64)
	}
	d.aliq = strings.Trim(al, "-")
	d.vISS = ou(inf.v("ValoresNfse/ValorIss"), v.v("ValorIss"))
	d.vIR, d.vCP, d.vCSLL, d.vPIS, d.vCOFINS = v.v("ValorIr"), v.v("ValorInss"), v.v("ValorCsll"), v.v("ValorPis"), v.v("ValorCofins")
	// no ABRASF, PIS/COFINS informados são retenções
	d.tpRetPC = "2"
	if numf(d.vPIS)+numf(d.vCOFINS) > 0 {
		d.tpRetPC = "1"
	}
	ret := numf(d.vIR) + numf(d.vCP) + numf(d.vCSLL) + numf(d.vPIS) + numf(d.vCOFINS)
	if s.v("IssRetido") == "1" {
		ret += numf(d.vISS)
	}
	d.vTotRet = valor(ret)
	d.vLiq = ou(inf.v("ValoresNfse/ValorLiquidoNfse"), v.v("ValorLiquidoNfse"))
	for _, x := range []string{inf.v("OutrasInformacoes"), decl.v("InformacoesComplementares")} {
		if x != "" {
			d.infCompl = append(d.infCompl, "Inf. Cont.: "+x)
		}
	}
	if d.codVerif != "" {
		d.infCompl = append(d.infCompl, "Cód. Verificação: "+d.codVerif)
	}
	return d
}

func dataBR(s string) string {
	if len(s) >= 10 && s[4] == '-' {
		return s[8:10] + "/" + s[5:7] + "/" + s[:4]
	}
	return s
}

// DANFSe gera o PDF da NFS-e no leiaute oficial do DANFSe v2.0 (Anexo I da Nota
// Técnica SE/CGNFS-e nº 008/2026): página única A4 em retrato, cabeçalho com a
// logomarca da NFS-e, blocos em grade de quatro colunas nas posições sugeridas
// pela NT, QR Code de consulta pública, bloco de tributação IBS/CBS, valor
// líquido + IBS/CBS, informações complementares e canhoto. Campos sem
// informação no XML saem com "-".
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
	if d.prest.vazia() {
		d.prest.nome, d.prest.doc = doc.EmitNome, formatarDoc(doc.EmitDoc)
	}
	if d.toma.vazia() && doc.DestNome != "" {
		d.toma.nome, d.toma.doc = doc.DestNome, formatarDoc(doc.DestDoc)
		d.destTomador = d.dest.vazia()
	}
	if d.vLiq == "" {
		d.vLiq = valor(doc.ValorTotal)
	}
	// Retenções federais: com tpRetPisCofins = 1 (PIS/COFINS retido), PIS e
	// COFINS somam-se às contribuições retidas e o débito próprio fica zerado.
	contribRet, pisProprio, cofProprio := d.vCSLL, d.vPIS, d.vCOFINS
	if d.tpRetPC == "1" {
		contribRet = valor(numf(d.vCSLL) + numf(d.vPIS) + numf(d.vCOFINS))
		pisProprio, cofProprio = "0", "0"
	}
	totIBSCBS, exclusoes := "", ""
	if d.ibscbs {
		if d.vIBSTot != "" || d.vCBS != "" {
			totIBSCBS = valor(numf(d.vIBSTot) + numf(d.vCBS))
		}
		exclusoes = valor(numf(d.vDescIncond) + numf(d.vReeRepRes) + numf(d.vISS) + numf(d.vPIS) + numf(d.vCOFINS))
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(tr("DANFSe "+d.numero), false)
	pdf.SetCreator("Vorticis Monitor", false)
	pdf.AddPage()

	// Marca d'água (por baixo do conteúdo) de NFS-e cancelada ou substituída (item 2.5): diagonal,
	// mínimo 50 pt, cinza K35.
	marca := map[string]string{"CANCELADA": "CANCELADA", "SUBSTITUIDA": "SUBSTITUÍDA"}[doc.Status]
	if marca != "" {
		pdf.SetTextColor(166, 166, 166)
		pdf.SetFont("Helvetica", "", 64)
		pdf.TransformBegin()
		pdf.TransformRotate(58, 105, 150)
		pdf.SetXY(0, 140)
		pdf.CellFormat(210, 20, tr(marca), "", 0, "C", false, 0, "")
		pdf.TransformEnd()
		pdf.SetTextColor(0, 0, 0)
	}

	// Medidas da NT 008 (item 2.4.5), em mm.
	const (
		esq, larg, dir = 3.0, 204.0, 207.0
		topo           = 3.0
		hl             = 6.3   // altura das linhas dos blocos
		linha          = 3.0   // entrelinha do conteúdo (7 pt)
		yCanhoto       = 281.0 // canhoto (opcional) no rodapé
	)
	cols := [4]float64{3, 54.1, 105.1, 156.2}
	sombra := func() { pdf.SetFillColor(242, 242, 242) }                 // cinza claro, 5%
	fino := func() { pdf.SetDrawColor(0, 0, 0); pdf.SetLineWidth(0.18) } // 0,5 pt
	fino()

	// cabe corta o texto com reticências quando não cabe na largura.
	cabe := func(s string, w float64) string {
		s = tr(s)
		if pdf.GetStringWidth(s) <= w {
			return s
		}
		for len(s) > 0 && pdf.GetStringWidth(s+"...") > w {
			s = s[:len(s)-1]
		}
		return strings.TrimRight(s, " ") + "..."
	}
	largura := func(i, span int) float64 {
		if i+span >= 4 {
			return dir - cols[i]
		}
		return cols[i+span] - cols[i]
	}
	// campo: rótulo (6 pt negrito) e conteúdo (7 pt) numa célula da grade.
	campo := func(i, span int, y, h float64, rot, val string, destaque bool) {
		x, w := cols[i], largura(i, span)
		if destaque {
			sombra()
			pdf.Rect(x, y, w, h, "F")
		}
		pdf.SetFont("Helvetica", "B", 6)
		pdf.SetXY(x+0.8, y+0.5)
		pdf.CellFormat(w-1.6, 2.4, cabe(rot, w-1.6), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetXY(x+0.8, y+3)
		pdf.CellFormat(w-1.6, 3, cabe(ou(val), w-1.6), "", 0, "L", false, 0, "")
	}
	// identificação: rótulos em 7 pt negrito, caixa alta.
	ident := func(i, span int, y, h float64, rot, val string, destaque bool) {
		x, w := cols[i], largura(i, span)
		if destaque {
			sombra()
			pdf.Rect(x, y+0.3, w-1, h-0.6, "F")
		}
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetXY(x+0.8, y+0.6)
		pdf.CellFormat(w-1.6, 2.8, tr(rot), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetXY(x+0.8, y+3.5)
		pdf.CellFormat(w-1.6, 3, cabe(ou(val), w-1.6), "", 0, "L", false, 0, "")
	}
	// bloco: linha divisória e título (7 pt, negrito, caixa alta, fundo cinza) na 1ª coluna.
	bloco := func(y float64, titulo string) {
		fino()
		pdf.Line(esq, y, dir, y)
		sombra()
		pdf.Rect(esq+0.4, y+0.6, cols[1]-esq-1.6, 3.4, "F")
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetXY(esq+0.8, y+0.6)
		pdf.CellFormat(cols[1]-esq-2, 3.4, cabe(titulo, cols[1]-esq-2.4), "", 0, "L", false, 0, "")
	}
	// aviso: bloco reduzido a uma linha centralizada (itens 2.3.1 e 2.3.2).
	aviso := func(y float64, texto string) float64 {
		fino()
		pdf.Line(esq, y, dir, y)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetXY(esq, y+0.1)
		pdf.CellFormat(larg, 3.1, tr(texto), "", 0, "C", false, 0, "")
		return y + 3.2
	}
	texto := func(s string, w float64) []string {
		pdf.SetFont("Helvetica", "", 7)
		var out []string
		for _, l := range pdf.SplitLines([]byte(tr(s)), w) {
			out = append(out, string(l))
		}
		return out
	}
	cortar := func(ls []string, n int, w float64) []string {
		if len(ls) <= n {
			return ls
		}
		ls = append([]string(nil), ls[:max(n, 0)]...)
		if n > 0 {
			s := ls[n-1]
			for len(s) > 0 && pdf.GetStringWidth(s+"...") > w {
				s = s[:len(s)-1]
			}
			ls[n-1] = s + "..."
		}
		return ls
	}

	// ── Cabeçalho ──────────────────────────────────────────────────────────
	sombra()
	pdf.Rect(esq, topo, larg, 11.6, "F")
	pdf.RegisterImageOptionsReader("logo-nfse", fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, bytes.NewReader(logoNFSe))
	desenharLogo(pdf, "logo-nfse", 4.9, topo+0.6, 40, 10.4)
	pdf.SetFont("Helvetica", "B", 9)
	yTit := topo + 1.9
	if d.homologacao != "" {
		yTit = topo + 0.3
	}
	pdf.SetXY(cols[1], yTit)
	pdf.CellFormat(101.9, 3.6, "DANFSe v2.0", "", 2, "C", false, 0, "")
	pdf.CellFormat(101.9, 3.6, tr("Documento Auxiliar da NFS-e"), "", 2, "C", false, 0, "")
	if d.homologacao != "" {
		pdf.SetTextColor(255, 0, 0)
		pdf.CellFormat(101.9, 3.4, tr(d.homologacao), "", 2, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.SetXY(cols[3], topo+1)
	pdf.SetFont("Helvetica", "", 8)
	if !d.item99 {
		pdf.CellFormat(50, 3.6, cabe("Município: "+ou(d.municipioEmissor), 50), "", 2, "L", false, 0, "")
	} else {
		pdf.Ln(3.6)
	}
	pdf.SetX(cols[3])
	pdf.SetFont("Helvetica", "", 6)
	pdf.CellFormat(50, 2.6, cabe("Ambiente Gerador: "+ou(d.ambGer), 50), "", 2, "L", false, 0, "")
	pdf.SetX(cols[3])
	pdf.CellFormat(50, 2.6, cabe("Tipo de Ambiente: "+ou(d.tpAmb), 50), "", 2, "L", false, 0, "")

	// ── Dados da NFS-e ────────────────────────────────────────────────────
	y := 14.8
	fino()
	pdf.Line(esq, y, dir, y)
	ident(0, 3, y, 7.7, "CHAVE DE ACESSO DA NFS-E", d.chave, false)
	y += 7.9
	ident(0, 1, y, 6.7, "NÚMERO DA NFS-E", d.numero, false)
	ident(1, 1, y, 6.7, "COMPETÊNCIA DA NFS-E", d.competencia, false)
	ident(2, 1, y, 6.7, "DATA E HORA DA EMISSÃO DA NFS-E", d.emissaoNFSe, false)
	y += 6.9
	ident(0, 1, y, 6.7, "NÚMERO DA DPS", d.numDPS, false)
	ident(1, 1, y, 6.7, "SÉRIE DA DPS", d.serieDPS, false)
	ident(2, 1, y, 6.7, "DATA E HORA DA EMISSÃO DA DPS", d.emissaoDPS, false)
	y += 6.9
	ident(0, 1, y, 6.7, "EMITENTE DA NFS-E", d.emitente, true)
	ident(1, 1, y, 6.7, "SITUAÇÃO DA NFS-E", d.situacao, false)
	ident(2, 1, y, 6.7, "FINALIDADE", d.finalidade, false)
	if c := fiscal.SoDigitos(d.chave); len(c) >= 44 && c == d.chave {
		if q, err := qr.Encode("https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave="+c, qr.M); err == nil {
			lado := 15.2
			m := lado / float64(q.Size)
			pdf.SetFillColor(0, 0, 0)
			for yy := 0; yy < q.Size; yy++ {
				for xx := 0; xx < q.Size; xx++ {
					if q.Black(xx, yy) {
						pdf.Rect(174.8+float64(xx)*m, 16.7+float64(yy)*m, m+0.01, m+0.01, "F")
					}
				}
			}
		}
	}
	pdf.SetFont("Helvetica", "", 6)
	pdf.SetXY(158, 33.2)
	pdf.MultiCell(47.2, 2.3, tr("A autenticidade desta NFS-e pode ser verificada pela leitura deste código QR ou pela consulta da chave de acesso no portal nacional da NFS-e"), "", "L", false)
	y += 6.9

	// ── Participantes ─────────────────────────────────────────────────────
	participante := func(titulo string, p parte, comIM bool) {
		bloco(y, titulo)
		campo(1, 1, y, hl, "CNPJ / CPF / NIF", p.doc, false)
		if comIM {
			campo(2, 1, y, hl, "Indicador Municipal (Inscrição)", p.im, false)
		}
		campo(3, 1, y, hl, "Telefone", fone(p.fone), false)
		y += hl
		campo(0, 2, y, hl, "Nome / Nome Empresarial", p.nome, false)
		campo(2, 1, y, hl, "Município / Sigla UF", p.municipio, false)
		campo(3, 1, y, hl, "Código IBGE / CEP", p.ibgeCep, false)
		y += hl
		campo(0, 2, y, hl, "Endereço", p.ender, false)
		campo(2, 2, y, hl, "E-mail", p.email, false)
		y += hl
	}
	participante("PRESTADOR / FORNECEDOR", d.prest, true)
	campo(0, 1, y, hl, "Simples Nacional na Data de Competência", d.simples, false)
	campo(1, 3, y, hl, "Regime de Apuração Tributária pelo SN", d.regApSN, false)
	y += hl
	if d.toma.vazia() {
		y = aviso(y, "TOMADOR/ADQUIRENTE DA OPERAÇÃO NÃO IDENTIFICADO NA NFS-e")
	} else {
		participante("TOMADOR / ADQUIRENTE", d.toma, true)
	}
	switch {
	case !d.dest.vazia():
		participante("DESTINATÁRIO DA OPERAÇÃO", d.dest, false)
	case d.destTomador:
		y = aviso(y, "O DESTINATÁRIO É O PRÓPRIO TOMADOR/ADQUIRENTE DA OPERAÇÃO")
	default:
		y = aviso(y, "DESTINATÁRIO DA OPERAÇÃO NÃO IDENTIFICADO NA NFS-e")
	}
	if d.interm.vazia() {
		y = aviso(y, "INTERMEDIÁRIO DA OPERAÇÃO NÃO IDENTIFICADO NA NFS-e")
	} else {
		participante("INTERMEDIÁRIO DA OPERAÇÃO", d.interm, true)
	}

	// ── Serviço prestado ──────────────────────────────────────────────────
	bloco(y, "SERVIÇO PRESTADO")
	campo(1, 1, y, hl, "Código de Tributação Nacional / Municipal", d.codTrib, false)
	campo(2, 1, y, hl, "Código da NBS", d.nbs, false)
	campo(3, 1, y, hl, "Local da Prestação / Sigla UF / País", d.localPrestacao, false)
	y += hl
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetXY(esq+0.8, y+0.3)
	pdf.CellFormat(larg-1.6, 3, cabe(ou(d.descCodTrib), larg-1.6), "", 0, "L", false, 0, "")
	y += 3.8
	yDesc := y

	// Altura dos blocos seguintes, para que tudo caiba numa única página.
	// Linhas do ISSQN que podem ser suprimidas quando não há dado algum (nota 5).
	linhaRegime := d.regEsp != "-" && d.regEsp != "Nenhum" || d.tpImunidade != "" || d.exig != "" || d.nProcSusp != ""
	linhaBM := d.beneficio != "" || numf(d.vCalcBM) != 0 || numf(d.vDedRed) != 0 || numf(d.vDescIncond) != 0
	linhasISS := 0
	if !d.semISSQN {
		linhasISS = 2
		for _, l := range []bool{linhaRegime, linhaBM} {
			if l {
				linhasISS++
			}
		}
	}
	pisCofinsProprio := d.anoCompet <= 2026 // item 2.1.9, nota 6
	altISS := 3.2
	if linhasISS > 0 {
		altISS = float64(linhasISS) * hl
	}
	altFed := hl
	if pisCofinsProprio {
		altFed += hl
	}
	resto := altISS + altFed + 4*hl + 2*6.7 + 3.9 // ISSQN, federal, IBS/CBS, valor total, título das inf. compl.

	infos := texto(strings.Join(d.infCompl, " | "), larg-1.6)
	if len(d.infCompl) == 0 {
		infos = nil
	}
	totais := "Totais Aproximados dos Tributos cfe. Lei nº 12.741/2012: " + ou(d.totais, "Federais: - ; Estaduais: - ; Municipais: -")
	desc := texto(ou(d.descricao), larg-1.6)
	livres := int((yCanhoto - 1.5 - yDesc - resto - 3.4 /*rótulo da descrição*/ - linha /*linha dos totais*/) / linha)
	nInf := min(len(infos), max(livres/3, livres-len(desc)))
	nDesc := livres - nInf
	desc = cortar(desc, nDesc, larg-1.6)
	infos = cortar(infos, nInf, larg-1.6)

	pdf.SetFont("Helvetica", "B", 6)
	pdf.SetXY(esq+0.8, y)
	pdf.CellFormat(60, 2.6, tr("Descrição do Serviço"), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 7)
	for i, l := range desc {
		pdf.SetXY(esq+0.8, y+3+float64(i)*linha)
		pdf.CellFormat(larg-1.6, linha, l, "", 0, "L", false, 0, "")
	}
	y += 3.4 + float64(nDesc)*linha

	// ── Tributação municipal (ISSQN) ──────────────────────────────────────
	if d.semISSQN {
		y = aviso(y, "TRIBUTAÇÃO MUNICIPAL (ISSQN) - OPERAÇÃO NÃO SUJEITA AO ISSQN")
	} else {
		bloco(y, "TRIBUTAÇÃO MUNICIPAL (ISSQN)")
		campo(1, 1, y, hl, "Tipo de Tributação do ISSQN", d.tribISSQN, false)
		campo(2, 2, y, hl, "Município / Sigla UF / País de Incidência do ISSQN", d.incidencia, false)
		y += hl
		if linhaRegime {
			campo(0, 1, y, hl, "Regime Especial de Tributação do ISSQN", d.regEsp, false)
			campo(1, 1, y, hl, "Tipo de Imunidade do ISSQN", d.tpImunidade, false)
			campo(2, 1, y, hl, "Suspensão da Exigibilidade do ISSQN", d.exig, false)
			campo(3, 1, y, hl, "Número Processo Suspensão", d.nProcSusp, false)
			y += hl
		}
		if linhaBM {
			campo(0, 1, y, hl, "Benefício Municipal", d.beneficio, false)
			campo(1, 1, y, hl, "Cálculo do BM", moeda(d.vCalcBM), false)
			campo(2, 1, y, hl, "Total Deduções/Reduções", moeda(d.vDedRed), false)
			campo(3, 1, y, hl, "Desconto Incondicionado", moeda(d.vDescIncond), false)
			y += hl
		}
		campo(0, 1, y, hl, "BC ISSQN", moeda(d.vBC), false)
		campo(1, 1, y, hl, "Alíquota Aplicada", pct(d.aliq), false)
		campo(2, 1, y, hl, "Retenção do ISSQN", d.retISS, false)
		campo(3, 1, y, hl, "ISSQN Apurado", moeda(d.vISS), false)
		y += hl
	}

	// ── Tributação federal (exceto CBS) ───────────────────────────────────
	bloco(y, "TRIBUTAÇÃO FEDERAL (EXCETO CBS)")
	campo(1, 1, y, hl, "IRRF", moeda(d.vIR), false)
	campo(2, 1, y, hl, "Contribuição Previdenciária - Retida", moeda(d.vCP), false)
	campo(3, 1, y, hl, "Contribuições Sociais - Retidas", moeda(contribRet), false)
	y += hl
	if pisCofinsProprio {
		campo(0, 1, y, hl, "PIS - Débito Apuração Própria", moeda(pisProprio), false)
		campo(1, 1, y, hl, "COFINS - Débito Apuração Própria", moeda(cofProprio), false)
		campo(2, 2, y, hl, "Descrição Contrib. Sociais - Retidas", tab(tabRetPC, d.tpRetPC), false)
		y += hl
	}

	// ── Tributação IBS / CBS ──────────────────────────────────────────────
	bloco(y, "TRIBUTAÇÃO IBS / CBS")
	cstClass := ""
	if d.cst != "" || d.cClassTrib != "" {
		cstClass = juntar(" / ", ou(d.cst), ou(d.cClassTrib))
	}
	operacao := ""
	if d.ibscbs && (d.cIndOp != "" || d.cLocIncidIBS != "") {
		operacao = juntar(" / ", ou(d.cIndOp), ou(d.cLocIncidIBS), ou(d.xLocIncidIBS), ou(ufDoMunicipio(d.cLocIncidIBS)))
	}
	pp := func(v ...string) string {
		if juntar("", v...) == "" {
			return "-"
		}
		var p []string
		for _, s := range v {
			p = append(p, pct(s))
		}
		return strings.Join(p, " / ")
	}
	campo(1, 1, y, hl, "CST / cClassTrib", cstClass, false)
	campo(2, 2, y, hl, "Indicador de Operação / Código IBGE Incidência / Município Incidência / Sigla UF", operacao, false)
	y += hl
	campo(0, 1, y, hl, "Exclusões e Reduções da Base de Cálculo", moeda(exclusoes), false)
	campo(1, 1, y, hl, "Base de Cálculo Após Exclusões e Reduções", moeda(d.vBCIBS), false)
	campo(2, 1, y, hl, "Red. Alíquota IBS / Red. Alíquota CBS", pp(d.pRedUF, d.pRedMun, d.pRedCBS), false)
	campo(3, 1, y, hl, "Alíquota - IBS UF / IBS Mun", pp(d.pIBSUF, d.pIBSMun), false)
	y += hl
	campo(0, 1, y, hl, "Alíq. Efetiva Municipal - IBS", pct(d.pEfetMun), false)
	campo(1, 1, y, hl, "Valor Apurado Municipal - IBS", moeda(d.vIBSMun), false)
	campo(2, 1, y, hl, "Alíq. Efetiva Estadual - IBS", pct(d.pEfetUF), false)
	campo(3, 1, y, hl, "Valor Apurado Estadual - IBS", moeda(d.vIBSUF), false)
	y += hl
	campo(0, 1, y, hl, "Valor Total Apurado - IBS", moeda(d.vIBSTot), false)
	campo(1, 1, y, hl, "Alíquota - CBS", pct(d.pCBS), false)
	campo(2, 1, y, hl, "Alíquota Efetiva - CBS", pct(d.pEfetCBS), false)
	campo(3, 1, y, hl, "Valor Total Apurado - CBS", moeda(d.vCBS), false)
	y += hl

	// ── Valor total da NFS-e ──────────────────────────────────────────────
	const ht = 6.7
	bloco(y, "VALOR TOTAL DA NFS-E")
	destaque := func(i int, y float64, rot, val string, fundo bool) {
		x, w := cols[i], largura(i, 1)
		if fundo {
			sombra()
			pdf.Rect(x, y, w, ht, "F")
		}
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetXY(x+0.8, y+0.5)
		pdf.CellFormat(w-1.6, 2.8, cabe(rot, w-1.6), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetXY(x+0.8, y+3.4)
		pdf.CellFormat(w-1.6, 3, cabe(val, w-1.6), "", 0, "L", false, 0, "")
	}
	destaque(1, y, "VALOR DA OPERAÇÃO / SERVIÇO", moeda(d.vServ), false)
	campo(2, 1, y, ht, "Desconto Incondicionado", moeda(d.vDescIncond), false)
	campo(3, 1, y, ht, "Desconto Condicionado", moeda(d.vDescCond), false)
	y += ht
	campo(0, 1, y, ht, "Total das Retenções (ISSQN / Federais)", moeda(d.vTotRet), false)
	destaque(1, y, "VALOR LÍQUIDO DA NFS-e", moeda(d.vLiq), false)
	campo(2, 1, y, ht, "Total do IBS/CBS", moeda(totIBSCBS), false)
	destaque(3, y, "VALOR LÍQUIDO DA NFS-e + IBS/CBS", moeda(d.vTotNF), true)
	y += ht

	// ── Informações complementares ────────────────────────────────────────
	bloco(y, "INFORMAÇÕES COMPLEMENTARES")
	y += 3.9
	pdf.SetFont("Helvetica", "", 7)
	for _, l := range infos {
		pdf.SetXY(esq+0.8, y)
		pdf.CellFormat(larg-1.6, linha, l, "", 0, "L", false, 0, "")
		y += linha
	}
	pdf.SetXY(esq+0.8, y)
	pdf.CellFormat(larg-1.6, linha, cabe(totais, larg-1.6), "", 0, "L", false, 0, "")

	// ── Canhoto (opcional) ────────────────────────────────────────────────
	fino()
	pdf.SetDashPattern([]float64{0.8, 0.6}, 0)
	pdf.Line(esq, yCanhoto-1, dir, yCanhoto-1)
	pdf.SetDashPattern(nil, 0)
	pdf.Rect(esq+0.4, yCanhoto, larg-0.8, 6.7, "D")
	for _, x := range cols[1:3] {
		pdf.Line(x, yCanhoto, x, yCanhoto+6.7)
	}
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetXY(cols[0]+0.8, yCanhoto+0.5)
	pdf.CellFormat(50, 2.8, tr("DATA CIENTIFICAÇÃO:"), "", 0, "L", false, 0, "")
	pdf.SetXY(cols[1]+0.8, yCanhoto+0.5)
	pdf.CellFormat(50, 2.8, tr("IDENTIFICAÇÃO E ASSINATURA"), "", 0, "L", false, 0, "")
	pdf.SetXY(cols[2]+0.8, yCanhoto+0.5)
	pdf.CellFormat(100, 2.8, tr("Nº NFS-e / CHAVE NFS-e"), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetXY(cols[2]+0.8, yCanhoto+3.4)
	pdf.CellFormat(100, 3, cabe(juntar(" / ", d.numero, d.chave), largura(2, 2)-1.6), "", 0, "L", false, 0, "")

	// Borda da página (1 pt).
	pdf.SetLineWidth(0.35)
	pdf.Rect(esq, topo, larg, yCanhoto+7.7-topo, "D")

	return pdf.Output(w)
}
