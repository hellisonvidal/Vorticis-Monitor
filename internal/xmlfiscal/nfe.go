// Package xmlfiscal faz o parse dos documentos fiscais eletrônicos (NF-e, NFC-e,
// CT-e, MDF-e, NFS-e Nacional/ABRASF, eventos e resumos da Distribuição DF-e).
package xmlfiscal

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// Valor é um decimal tolerante a campos vazios.
type Valor float64

func (v *Valor) UnmarshalText(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" {
		*v = 0
		return nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return nil
	}
	*v = Valor(f)
	return nil
}

func (v Valor) F() float64 { return float64(v) }

type NFeProc struct {
	XMLName xml.Name `xml:"nfeProc"`
	NFe     NFe      `xml:"NFe"`
	ProtNFe *ProtNFe `xml:"protNFe"`
}

type NFe struct {
	InfNFe InfNFe `xml:"infNFe"`
}

type ProtNFe struct {
	InfProt struct {
		TpAmb    string `xml:"tpAmb"`
		ChNFe    string `xml:"chNFe"`
		DhRecbto string `xml:"dhRecbto"`
		NProt    string `xml:"nProt"`
		CStat    int    `xml:"cStat"`
		XMotivo  string `xml:"xMotivo"`
	} `xml:"infProt"`
}

type InfNFe struct {
	ID     string `xml:"Id,attr"`
	Versao string `xml:"versao,attr"`
	Ide    Ide    `xml:"ide"`
	Emit   Emit   `xml:"emit"`
	Dest   Dest   `xml:"dest"`
	Det    []Det  `xml:"det"`
	Total  struct {
		ICMSTot ICMSTot `xml:"ICMSTot"`
	} `xml:"total"`
	Transp struct {
		ModFrete   string `xml:"modFrete"`
		Transporta struct {
			CNPJ string `xml:"CNPJ"`
			CPF  string `xml:"CPF"`
		} `xml:"transporta"`
	} `xml:"transp"`
	AutXML []struct {
		CNPJ string `xml:"CNPJ"`
		CPF  string `xml:"CPF"`
	} `xml:"autXML"`
	InfAdic struct {
		InfCpl string `xml:"infCpl"`
	} `xml:"infAdic"`
	Compra struct {
		XPed string `xml:"xPed"`
	} `xml:"compra"`
}

// Chave retorna a chave de acesso (sem prefixo "NFe").
func (i *InfNFe) Chave() string { return strings.TrimPrefix(i.ID, "NFe") }

type Ide struct {
	CUF      string `xml:"cUF"`
	CNF      string `xml:"cNF"`
	NatOp    string `xml:"natOp"`
	Mod      string `xml:"mod"`
	Serie    string `xml:"serie"`
	NNF      string `xml:"nNF"`
	DhEmi    string `xml:"dhEmi"`
	DhSaiEnt string `xml:"dhSaiEnt"`
	TpNF     string `xml:"tpNF"`
	IdDest   string `xml:"idDest"`
	CMunFG   string `xml:"cMunFG"`
	TpEmis   string `xml:"tpEmis"`
	CDV      string `xml:"cDV"`
	TpAmb    string `xml:"tpAmb"`
	FinNFe   string `xml:"finNFe"`
	IndFinal string `xml:"indFinal"`
	IndPres  string `xml:"indPres"`
	DhCont   string `xml:"dhCont"`
	XJust    string `xml:"xJust"`
	NFref    []struct {
		RefNFe string `xml:"refNFe"`
	} `xml:"NFref"`
}

type Endereco struct {
	CMun string `xml:"cMun"`
	XMun string `xml:"xMun"`
	UF   string `xml:"UF"`
}

type Emit struct {
	CNPJ      string   `xml:"CNPJ"`
	CPF       string   `xml:"CPF"`
	XNome     string   `xml:"xNome"`
	EnderEmit Endereco `xml:"enderEmit"`
	IE        string   `xml:"IE"`
	CRT       string   `xml:"CRT"`
}

func (e Emit) Doc() string {
	if e.CNPJ != "" {
		return e.CNPJ
	}
	return e.CPF
}

type Dest struct {
	CNPJ          string   `xml:"CNPJ"`
	CPF           string   `xml:"CPF"`
	IdEstrangeiro string   `xml:"idEstrangeiro"`
	XNome         string   `xml:"xNome"`
	EnderDest     Endereco `xml:"enderDest"`
	IndIEDest     string   `xml:"indIEDest"`
	IE            string   `xml:"IE"`
}

func (d Dest) Doc() string {
	if d.CNPJ != "" {
		return d.CNPJ
	}
	return d.CPF
}

type Det struct {
	NItem   int     `xml:"nItem,attr"`
	Prod    Prod    `xml:"prod"`
	Imposto Imposto `xml:"imposto"`
}

type Prod struct {
	CProd    string `xml:"cProd"`
	CEAN     string `xml:"cEAN"`
	XProd    string `xml:"xProd"`
	NCM      string `xml:"NCM"`
	CEST     string `xml:"CEST"`
	CFOP     string `xml:"CFOP"`
	UCom     string `xml:"uCom"`
	QCom     Valor  `xml:"qCom"`
	VUnCom   Valor  `xml:"vUnCom"`
	VProd    Valor  `xml:"vProd"`
	VFrete   Valor  `xml:"vFrete"`
	VSeg     Valor  `xml:"vSeg"`
	VDesc    Valor  `xml:"vDesc"`
	VOutro   Valor  `xml:"vOutro"`
	IndTot   string `xml:"indTot"`
	XPed     string `xml:"xPed"`
	NItemPed string `xml:"nItemPed"`
}

// GrupoImposto captura qualquer subgrupo (ICMS00, ICMSSN102, PISAliq, IPITrib...).
type GrupoImposto struct {
	XMLName     xml.Name
	Orig        string `xml:"orig"`
	CST         string `xml:"CST"`
	CSOSN       string `xml:"CSOSN"`
	ModBC       string `xml:"modBC"`
	PRedBC      Valor  `xml:"pRedBC"`
	VBC         Valor  `xml:"vBC"`
	PICMS       Valor  `xml:"pICMS"`
	VICMS       Valor  `xml:"vICMS"`
	VICMSDeson  Valor  `xml:"vICMSDeson"`
	PFCP        Valor  `xml:"pFCP"`
	VFCP        Valor  `xml:"vFCP"`
	ModBCST     string `xml:"modBCST"`
	PMVAST      Valor  `xml:"pMVAST"`
	PRedBCST    Valor  `xml:"pRedBCST"`
	VBCST       Valor  `xml:"vBCST"`
	PICMSST     Valor  `xml:"pICMSST"`
	VICMSST     Valor  `xml:"vICMSST"`
	VFCPST      Valor  `xml:"vFCPST"`
	VBCSTRet    Valor  `xml:"vBCSTRet"`
	VICMSSTRet  Valor  `xml:"vICMSSTRet"`
	PCredSN     Valor  `xml:"pCredSN"`
	VCredICMSSN Valor  `xml:"vCredICMSSN"`
	// PIS/COFINS/IPI
	PPIS    Valor `xml:"pPIS"`
	VPIS    Valor `xml:"vPIS"`
	PCOFINS Valor `xml:"pCOFINS"`
	VCOFINS Valor `xml:"vCOFINS"`
	PIPI    Valor `xml:"pIPI"`
	VIPI    Valor `xml:"vIPI"`
}

// CSTouCSOSN retorna o código de situação tributária do grupo.
func (g GrupoImposto) CSTouCSOSN() string {
	if g.CSOSN != "" {
		return g.CSOSN
	}
	return g.CST
}

type grupoAny struct {
	Grupos []GrupoImposto `xml:",any"`
}

func (g grupoAny) Primeiro() GrupoImposto {
	for _, x := range g.Grupos {
		if x.XMLName.Local != "cEnq" && x.XMLName.Local != "CNPJProd" && x.XMLName.Local != "cSelo" && x.XMLName.Local != "qSelo" {
			return x
		}
	}
	return GrupoImposto{}
}

type Imposto struct {
	VTotTrib   Valor    `xml:"vTotTrib"`
	ICMS       grupoAny `xml:"ICMS"`
	IPI        grupoAny `xml:"IPI"`
	PIS        grupoAny `xml:"PIS"`
	COFINS     grupoAny `xml:"COFINS"`
	ICMSUFDest *struct {
		VBCUFDest      Valor `xml:"vBCUFDest"`
		VBCFCPUFDest   Valor `xml:"vBCFCPUFDest"`
		PFCPUFDest     Valor `xml:"pFCPUFDest"`
		PICMSUFDest    Valor `xml:"pICMSUFDest"`
		PICMSInter     Valor `xml:"pICMSInter"`
		PICMSInterPart Valor `xml:"pICMSInterPart"`
		VFCPUFDest     Valor `xml:"vFCPUFDest"`
		VICMSUFDest    Valor `xml:"vICMSUFDest"`
		VICMSUFRemet   Valor `xml:"vICMSUFRemet"`
	} `xml:"ICMSUFDest"`
	// Reforma Tributária (LC 214/2025, NT 2025.002): grupo IBS/CBS.
	IBSCBS *struct {
		CST        string `xml:"CST"`
		CClassTrib string `xml:"cClassTrib"`
		G          struct {
			VBC    Valor `xml:"vBC"`
			GIBSUF struct {
				PIBSUF Valor `xml:"pIBSUF"`
				VIBSUF Valor `xml:"vIBSUF"`
			} `xml:"gIBSUF"`
			GIBSMun struct {
				PIBSMun Valor `xml:"pIBSMun"`
				VIBSMun Valor `xml:"vIBSMun"`
			} `xml:"gIBSMun"`
			GCBS struct {
				PCBS Valor `xml:"pCBS"`
				VCBS Valor `xml:"vCBS"`
			} `xml:"gCBS"`
		} `xml:"gIBSCBS"`
	} `xml:"IBSCBS"`
}

func (i Imposto) ICMSGrupo() GrupoImposto   { return i.ICMS.Primeiro() }
func (i Imposto) IPIGrupo() GrupoImposto    { return i.IPI.Primeiro() }
func (i Imposto) PISGrupo() GrupoImposto    { return i.PIS.Primeiro() }
func (i Imposto) COFINSGrupo() GrupoImposto { return i.COFINS.Primeiro() }

type ICMSTot struct {
	VBC         Valor `xml:"vBC"`
	VICMS       Valor `xml:"vICMS"`
	VICMSDeson  Valor `xml:"vICMSDeson"`
	VFCPUFDest  Valor `xml:"vFCPUFDest"`
	VICMSUFDest Valor `xml:"vICMSUFDest"`
	VFCP        Valor `xml:"vFCP"`
	VBCST       Valor `xml:"vBCST"`
	VST         Valor `xml:"vST"`
	VFCPST      Valor `xml:"vFCPST"`
	VProd       Valor `xml:"vProd"`
	VFrete      Valor `xml:"vFrete"`
	VSeg        Valor `xml:"vSeg"`
	VDesc       Valor `xml:"vDesc"`
	VII         Valor `xml:"vII"`
	VIPI        Valor `xml:"vIPI"`
	VIPIDevol   Valor `xml:"vIPIDevol"`
	VPIS        Valor `xml:"vPIS"`
	VCOFINS     Valor `xml:"vCOFINS"`
	VOutro      Valor `xml:"vOutro"`
	VNF         Valor `xml:"vNF"`
}
