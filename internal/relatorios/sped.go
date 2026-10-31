// Package relatorios gera relatórios fiscais, arquivos de pré-escrituração SPED,
// exportações em lote e representações gráficas (DANFE/DACTE/DANFSe simplificados).
package relatorios

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/regras"
	"vorticis.com.br/monitor/internal/store"
)

// Periodo AAAA-MM.
type Periodo struct {
	Ini, Fim time.Time
}

func ParsePeriodo(s string) (Periodo, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return Periodo{}, fmt.Errorf("período inválido (use AAAA-MM): %s", s)
	}
	return Periodo{Ini: t, Fim: t.AddDate(0, 1, -1)}, nil
}

func (p Periodo) filtro(empresaID int64) store.FiltroDocumentos {
	return store.FiltroDocumentos{EmpresaID: empresaID, De: p.Ini.Format("2006-01-02"), Ate: p.Fim.Format("2006-01-02")}
}

// escritor acumula registros e contadores para os blocos 9.
type escritor struct {
	w     *bufio.Writer
	cont  map[string]int
	bloco map[byte]int
	ordem []string
	total int
}

func novoEscritor(w io.Writer) *escritor {
	return &escritor{w: bufio.NewWriter(w), cont: map[string]int{}, bloco: map[byte]int{}}
}

func (e *escritor) reg(campos ...string) {
	r := campos[0]
	if _, ok := e.cont[r]; !ok {
		e.ordem = append(e.ordem, r)
	}
	e.cont[r]++
	e.bloco[r[0]]++
	e.total++
	for i, c := range campos {
		campos[i] = strings.ReplaceAll(c, "|", " ")
	}
	e.w.WriteString("|" + strings.Join(campos, "|") + "|\r\n")
}

func (e *escritor) fechaBloco(b byte) {
	e.reg(string(b)+"990", strconv.Itoa(e.bloco[b]+1))
}

func (e *escritor) bloco9() error {
	e.reg("9001", "0")
	regs := append([]string{}, e.ordem...)
	n := len(regs) + 3 // + 9900 de 9900, 9990, 9999
	for _, r := range regs {
		c := e.cont[r]
		if r == "9001" {
			c = 1
		}
		e.reg("9900", r, strconv.Itoa(c))
	}
	e.reg("9900", "9900", strconv.Itoa(n))
	e.reg("9900", "9990", "1")
	e.reg("9900", "9999", "1")
	e.reg("9990", strconv.Itoa(e.bloco['9']+1))
	e.reg("9999", strconv.Itoa(e.total+1))
	return e.w.Flush()
}

func v(x float64) string { return strings.Replace(strconv.FormatFloat(x, 'f', 2, 64), ".", ",", 1) }
func q(x float64) string { return strings.Replace(strconv.FormatFloat(x, 'f', 3, 64), ".", ",", 1) }

func data(s string) string {
	t := regras.ParseData(s)
	if t.IsZero() {
		return ""
	}
	return t.Format("02012006")
}

func codSit(d *store.Documento) string {
	switch d.Status {
	case "CANCELADA":
		return "02"
	case "DENEGADA":
		return "04"
	case "INUTILIZADA":
		return "05"
	}
	if d.FinNFe == "2" {
		return "06" // complementar
	}
	return "00"
}

func cfopEntrada(c string) string {
	if len(c) != 4 {
		return c
	}
	switch c[0] {
	case '5':
		return "1" + c[1:]
	case '6':
		return "2" + c[1:]
	case '7':
		return "3" + c[1:]
	}
	return c
}

type participante struct{ cod, nome, doc, uf string }

type dadosSPED struct {
	emp   *store.Empresa
	docs  []*store.Documento
	itens map[int64][]store.Item
	parts map[string]participante
	prods map[string]store.Item
	unid  map[string]bool
}

func carregar(s *store.Store, empresaID int64, p Periodo) (*dadosSPED, error) {
	emp, err := s.ObterEmpresa(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa não encontrada")
	}
	docs, err := s.DocumentosComXML(p.filtro(empresaID))
	if err != nil {
		return nil, err
	}
	d := &dadosSPED{emp: emp, itens: map[int64][]store.Item{}, parts: map[string]participante{}, prods: map[string]store.Item{}, unid: map[string]bool{}}
	for _, doc := range docs {
		if doc.Resumo || doc.Status == "BLOQUEADA" || doc.Status == "PENDENTE" || doc.Status == "REJEITADA" {
			continue
		}
		d.docs = append(d.docs, doc)
		its, _ := s.Itens(doc.ID)
		d.itens[doc.ID] = its
		other, nome, uf := doc.DestDoc, doc.DestNome, doc.DestUF
		if doc.Direcao == "E" {
			other, nome, uf = doc.EmitDoc, doc.EmitNome, doc.EmitUF
		}
		if other != "" {
			d.parts[other] = participante{cod: other, nome: nome, doc: other, uf: uf}
		}
		if doc.Direcao == "E" {
			for _, i := range its {
				d.prods[doc.EmitDoc+":"+i.CodProd] = i
				d.unid[strings.ToUpper(i.Unidade)] = true
			}
		}
	}
	return d, nil
}

func codItem(fornecedor, cod string) string {
	return strings.TrimLeft(fornecedor, "0")[:min(8, len(strings.TrimLeft(fornecedor, "0")))] + "-" + cod
}

// EFDICMSIPI gera arquivo de pré-escrituração da EFD ICMS/IPI (blocos 0, B, C, D, E, G, H, K, 1, 9)
// a partir dos documentos monitorados. Deve ser importado/validado no PVA ou no sistema de escrituração.
func EFDICMSIPI(w io.Writer, s *store.Store, empresaID int64, p Periodo, versaoLeiaute string) error {
	d, err := carregar(s, empresaID, p)
	if err != nil {
		return err
	}
	if versaoLeiaute == "" {
		versaoLeiaute = "020"
	}
	e := novoEscritor(w)
	ini, fim := p.Ini.Format("02012006"), p.Fim.Format("02012006")
	perfil := "A"
	if d.emp.Regime == "LP" {
		perfil = "B"
	}
	e.reg("0000", versaoLeiaute, "0", ini, fim, d.emp.RazaoSocial, d.emp.CNPJ, "", d.emp.UF, d.emp.IE, d.emp.CodMun, "", "", perfil, "1")
	e.reg("0001", "0")
	e.reg("0005", d.emp.RazaoSocial, "", "", "", "", "", "", "", "")
	e.reg("0100", "CONTADOR RESPONSAVEL", "", "", "", "", "", "", "", "", "", "", "", "")
	keys := make([]string, 0, len(d.parts))
	for k := range d.parts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pt := d.parts[k]
		cnpj, cpf := pt.doc, ""
		if len(pt.doc) == 11 {
			cnpj, cpf = "", pt.doc
		}
		e.reg("0150", pt.cod, pt.nome, "01058", cnpj, cpf, "", "", "", "", "", "", "")
	}
	for u := range d.unid {
		e.reg("0190", u, u)
	}
	pk := make([]string, 0, len(d.prods))
	for k := range d.prods {
		pk = append(pk, k)
	}
	sort.Strings(pk)
	for _, k := range pk {
		i := d.prods[k]
		forn := strings.SplitN(k, ":", 2)[0]
		e.reg("0200", codItem(forn, i.CodProd), i.Descricao, "", "", strings.ToUpper(i.Unidade), "00", i.NCM, "", "", "", "", i.CEST)
	}
	e.fechaBloco('0')
	e.reg("B001", "1")
	e.fechaBloco('B')

	// Bloco C (NF-e/NFC-e)
	temC := false
	for _, doc := range d.docs {
		if doc.Modelo == "NFE" || doc.Modelo == "NFCE" {
			temC = true
		}
	}
	e.reg("C001", map[bool]string{true: "0", false: "1"}[temC])
	type ana struct {
		cst, cfop string
		aliq      float64
	}
	debitos, creditos := 0.0, 0.0
	difalUF := map[string][2]float64{}
	if temC {
		e.reg("C010", d.emp.CNPJ, "2")
	}
	for _, doc := range d.docs {
		if doc.Modelo != "NFE" && doc.Modelo != "NFCE" {
			continue
		}
		mod := "55"
		if doc.Modelo == "NFCE" {
			mod = "65"
		}
		indOper, indEmit, part := "1", "0", doc.DestDoc
		if doc.Direcao == "E" {
			indOper, indEmit, part = "0", "1", doc.EmitDoc
		}
		if doc.Modelo == "NFCE" {
			part = ""
		}
		sit := codSit(doc)
		if sit == "02" || sit == "04" || sit == "05" {
			e.reg("C100", indOper, indEmit, "", mod, sit, doc.Serie, doc.Numero, doc.Chave, "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "")
			continue
		}
		its := d.itens[doc.ID]
		var vMerc, vDesc, vBC, vBCST float64
		for _, i := range its {
			vMerc += i.ValorTotal
			vDesc += i.ValorDesconto
			vBC += i.VBC
			vBCST += i.VBCST
		}
		e.reg("C100", indOper, indEmit, part, mod, sit, doc.Serie, doc.Numero, doc.Chave, data(doc.DhEmissao), data(doc.DhEmissao), v(doc.ValorTotal), "0",
			v(vDesc), "0,00", v(vMerc), "9", "0,00", "0,00", "0,00", v(vBC), v(doc.ValorICMS), v(vBCST), v(doc.ValorST), v(doc.ValorIPI), v(doc.ValorPIS), v(doc.ValorCOFINS), "0,00", "0,00")
		if doc.ValorDIFAL > 0 && doc.Direcao == "S" {
			var fcp float64
			for _, i := range its {
				fcp += i.VFCPDest
			}
			e.reg("C101", v(fcp), v(doc.ValorDIFAL), "0,00")
			x := difalUF[doc.DestUF]
			difalUF[doc.DestUF] = [2]float64{x[0] + doc.ValorDIFAL, x[1] + fcp}
		}
		ag := map[ana]*[6]float64{}
		var ordem []ana
		for _, i := range its {
			cfop := i.CFOP
			if doc.Direcao == "E" && indEmit == "1" {
				cfop = cfopEntrada(cfop)
				e.reg("C170", strconv.Itoa(i.NItem), codItem(doc.EmitDoc, i.CodProd), "", q(i.Quantidade), strings.ToUpper(i.Unidade), v(i.ValorTotal), v(i.ValorDesconto), "0",
					i.CSTICMS, cfop, "", v(i.VBC), v(i.PICMS), v(i.VICMS), v(i.VBCST), v(i.PICMSST), v(i.VICMSST), "0", "", "", "", "", v(i.VIPI),
					i.CSTPIS, "", "", "", "", v(i.VPIS), i.CSTCOFINS, "", "", "", "", v(i.VCOFINS), "", "0,00")
			}
			k := ana{i.CSTICMS, cfop, i.PICMS}
			if ag[k] == nil {
				ag[k] = &[6]float64{}
				ordem = append(ordem, k)
			}
			a := ag[k]
			a[0] += i.ValorTotal - i.ValorDesconto + i.VICMSST + i.VIPI
			a[1] += i.VBC
			a[2] += i.VICMS
			a[3] += i.VBCST
			a[4] += i.VICMSST
			a[5] += i.VIPI
		}
		for _, k := range ordem {
			a := ag[k]
			e.reg("C190", k.cst, k.cfop, v(k.aliq), v(a[0]), v(a[1]), v(a[2]), v(a[3]), v(a[4]), "0,00", v(a[5]), "")
			if doc.Direcao == "S" {
				debitos += a[2]
			} else {
				creditos += a[2]
			}
		}
	}
	e.fechaBloco('C')

	// Bloco D (CT-e)
	temD := false
	for _, doc := range d.docs {
		if doc.Modelo == "CTE" {
			temD = true
		}
	}
	e.reg("D001", map[bool]string{true: "0", false: "1"}[temD])
	if temD {
		e.reg("D010", d.emp.CNPJ)
	}
	for _, doc := range d.docs {
		if doc.Modelo != "CTE" {
			continue
		}
		indOper, indEmit, part := "0", "1", doc.EmitDoc
		if doc.Direcao == "S" {
			indOper, indEmit, part = "1", "0", doc.DestDoc
		}
		e.reg("D100", indOper, indEmit, part, "57", codSit(doc), doc.Serie, "", doc.Numero, doc.Chave, data(doc.DhEmissao), data(doc.DhEmissao), "0", "", v(doc.ValorTotal), "0,00", "1",
			v(doc.ValorTotal), v(doc.ValorTotal), v(doc.ValorICMS), "0,00", "", "", "", "")
		cfop := doc.CFOPPrincipal
		if doc.Direcao == "E" {
			cfop = cfopEntrada(cfop)
			creditos += doc.ValorICMS
		} else {
			debitos += doc.ValorICMS
		}
		aliq := 0.0
		if doc.ValorTotal > 0 {
			aliq = doc.ValorICMS / doc.ValorTotal * 100
		}
		e.reg("D190", "000", cfop, v(aliq), v(doc.ValorTotal), v(doc.ValorTotal), v(doc.ValorICMS), "0,00", "")
	}
	e.fechaBloco('D')

	// Bloco E (apuração)
	e.reg("E001", "0")
	e.reg("E100", ini, fim)
	saldo := debitos - creditos
	recolher, credor := 0.0, 0.0
	if saldo > 0 {
		recolher = saldo
	} else {
		credor = -saldo
	}
	e.reg("E110", v(debitos), "0,00", "0,00", "0,00", v(creditos), "0,00", "0,00", "0,00", "0,00", v(max(0, saldo)), "0,00", v(recolher), v(credor), "0,00")
	ufs := make([]string, 0, len(difalUF))
	for uf := range difalUF {
		ufs = append(ufs, uf)
	}
	sort.Strings(ufs)
	for _, uf := range ufs {
		x := difalUF[uf]
		e.reg("E300", uf, ini, fim)
		e.reg("E310", "1", "0,00", v(x[0]), "0,00", "0,00", "0,00", "0,00", "0,00", v(x[0]), "0,00", "0,00",
			"0,00", v(x[1]), "0,00", "0,00", "0,00", "0,00", "0,00", v(x[1]), "0,00", "0,00")
	}
	e.fechaBloco('E')
	for _, b := range []string{"G", "H", "K", "1"} {
		e.reg(b+"001", "1")
		e.fechaBloco(b[0])
	}
	return e.bloco9()
}

// EFDContribuicoes gera a pré-escrituração da EFD-Contribuições (0, A, C, D, F, M, 1, 9).
// Créditos de PIS/COFINS são estimados (base = valor do item) para empresas não cumulativas.
func EFDContribuicoes(w io.Writer, s *store.Store, empresaID int64, p Periodo) error {
	d, err := carregar(s, empresaID, p)
	if err != nil {
		return err
	}
	e := novoEscritor(w)
	ini, fim := p.Ini.Format("02012006"), p.Fim.Format("02012006")
	e.reg("0000", "006", "0", "", "", ini, fim, d.emp.RazaoSocial, d.emp.CNPJ, d.emp.UF, d.emp.CodMun, "", "00", "1")
	e.reg("0001", "0")
	cod := "1" // não cumulativo
	aPis, aCof := 1.65, 7.6
	if d.emp.Regime == "LP" {
		cod, aPis, aCof = "2", 0.65, 3.0
	}
	e.reg("0110", cod, "1", "1", "")
	e.reg("0140", "", d.emp.RazaoSocial, d.emp.CNPJ, d.emp.UF, d.emp.IE, d.emp.CodMun, "", "")
	for _, pt := range d.parts {
		cnpj, cpf := pt.doc, ""
		if len(pt.doc) == 11 {
			cnpj, cpf = "", pt.doc
		}
		e.reg("0150", pt.cod, pt.nome, "01058", cnpj, cpf, "", "", "", "", "", "", "")
	}
	e.fechaBloco('0')

	// Bloco A — serviços (NFS-e)
	temA := false
	for _, doc := range d.docs {
		if doc.Modelo == "NFSE" {
			temA = true
		}
	}
	e.reg("A001", map[bool]string{true: "0", false: "1"}[temA])
	var debPis, debCof, credPis, credCof, retPis, retCof float64
	if temA {
		e.reg("A010", d.emp.CNPJ)
		for _, doc := range d.docs {
			if doc.Modelo != "NFSE" {
				continue
			}
			indOper, part := "0", doc.EmitDoc
			if doc.Direcao == "S" {
				indOper, part = "1", doc.DestDoc
			}
			pis, cof := doc.ValorTotal*aPis/100, doc.ValorTotal*aCof/100
			e.reg("A100", indOper, map[bool]string{true: "0", false: "1"}[doc.Direcao == "S"], part, "00", "", "", doc.Numero, "", data(doc.DhEmissao), "", v(doc.ValorTotal), "1", "0,00", v(doc.ValorTotal), v(pis), v(doc.ValorTotal), v(cof), v(doc.ValorPIS), v(doc.ValorCOFINS), v(doc.ValorISS))
			if doc.Direcao == "S" {
				debPis, debCof = debPis+pis, debCof+cof
				retPis, retCof = retPis+doc.ValorPIS, retCof+doc.ValorCOFINS
			} else if cod == "1" {
				credPis, credCof = credPis+pis, credCof+cof
			}
		}
	}
	e.fechaBloco('A')

	e.reg("C001", "0")
	e.reg("C010", d.emp.CNPJ, "2")
	for _, doc := range d.docs {
		if doc.Modelo != "NFE" {
			continue
		}
		indOper, indEmit, part := "1", "0", doc.DestDoc
		if doc.Direcao == "E" {
			indOper, indEmit, part = "0", "1", doc.EmitDoc
		}
		sit := codSit(doc)
		e.reg("C100", indOper, indEmit, part, "55", sit, doc.Serie, doc.Numero, doc.Chave, data(doc.DhEmissao), data(doc.DhEmissao), v(doc.ValorTotal), "0", "0,00", "0,00", v(doc.ValorTotal), "9", "0,00", "0,00", "0,00", "0,00", v(doc.ValorICMS), "0,00", v(doc.ValorST), v(doc.ValorIPI), v(doc.ValorPIS), v(doc.ValorCOFINS), "0,00", "0,00")
		if sit != "00" && sit != "06" {
			continue
		}
		for _, i := range d.itens[doc.ID] {
			cst, _ := strconv.Atoi(i.CSTPIS)
			if doc.Direcao == "S" {
				debPis += i.VPIS
				debCof += i.VCOFINS
			} else if cod == "1" && cst >= 50 && cst <= 56 || (doc.Direcao == "E" && cod == "1" && cst <= 2) {
				b := i.ValorTotal - i.ValorDesconto + i.VIPI
				credPis += b * aPis / 100
				credCof += b * aCof / 100
			}
			if doc.Direcao == "E" {
				e.reg("C170", strconv.Itoa(i.NItem), codItem(doc.EmitDoc, i.CodProd), "", q(i.Quantidade), strings.ToUpper(i.Unidade), v(i.ValorTotal), v(i.ValorDesconto), "0",
					i.CSTICMS, cfopEntrada(i.CFOP), "", v(i.VBC), v(i.PICMS), v(i.VICMS), v(i.VBCST), v(i.PICMSST), v(i.VICMSST), "0", i.CSTIPI, "", "", "", v(i.VIPI),
					"50", v(i.ValorTotal), v(aPis), "", "", v(i.ValorTotal*aPis/100), "50", v(i.ValorTotal), v(aCof), "", "", v(i.ValorTotal*aCof/100), "")
			}
		}
	}
	e.fechaBloco('C')
	for _, b := range []string{"D", "F"} {
		e.reg(b+"001", "1")
		e.fechaBloco(b[0])
	}
	e.reg("M001", "0")
	nc := cod == "1"
	pisDev, cofDev := max(0, debPis-credPis-retPis), max(0, debCof-credCof-retCof)
	if nc {
		e.reg("M200", v(debPis), v(credPis), "0,00", v(max(0, debPis-credPis)), v(retPis), "0,00", v(pisDev), "0,00", "0,00", "0,00", "0,00", v(pisDev))
		e.reg("M600", v(debCof), v(credCof), "0,00", v(max(0, debCof-credCof)), v(retCof), "0,00", v(cofDev), "0,00", "0,00", "0,00", "0,00", v(cofDev))
	} else {
		e.reg("M200", "0,00", "0,00", "0,00", "0,00", "0,00", "0,00", "0,00", v(debPis), v(retPis), "0,00", v(pisDev), v(pisDev))
		e.reg("M600", "0,00", "0,00", "0,00", "0,00", "0,00", "0,00", "0,00", v(debCof), v(retCof), "0,00", v(cofDev), v(cofDev))
	}
	e.fechaBloco('M')
	e.reg("1001", "1")
	e.fechaBloco('1')
	return e.bloco9()
}
