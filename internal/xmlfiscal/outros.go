package xmlfiscal

import "encoding/xml"

// ---------- CT-e ----------

type CTeProc struct {
	CTe struct {
		InfCte InfCte `xml:"infCte"`
	} `xml:"CTe"`
	ProtCTe *struct {
		InfProt struct {
			ChCTe    string `xml:"chCTe"`
			DhRecbto string `xml:"dhRecbto"`
			NProt    string `xml:"nProt"`
			CStat    int    `xml:"cStat"`
			XMotivo  string `xml:"xMotivo"`
		} `xml:"infProt"`
	} `xml:"protCTe"`
}

type InfCte struct {
	ID  string `xml:"Id,attr"`
	Ide struct {
		CUF    string `xml:"cUF"`
		CFOP   string `xml:"CFOP"`
		NatOp  string `xml:"natOp"`
		Mod    string `xml:"mod"`
		Serie  string `xml:"serie"`
		NCT    string `xml:"nCT"`
		DhEmi  string `xml:"dhEmi"`
		TpEmis string `xml:"tpEmis"`
		TpCTe  string `xml:"tpCTe"`
		Modal  string `xml:"modal"`
		UFIni  string `xml:"UFIni"`
		UFFim  string `xml:"UFFim"`
		Toma3  *struct {
			Toma string `xml:"toma"`
		} `xml:"toma3"`
		Toma4 *struct {
			Toma string `xml:"toma"`
			CNPJ string `xml:"CNPJ"`
		} `xml:"toma4"`
	} `xml:"ide"`
	Emit struct {
		CNPJ      string   `xml:"CNPJ"`
		IE        string   `xml:"IE"`
		XNome     string   `xml:"xNome"`
		EnderEmit Endereco `xml:"enderEmit"`
	} `xml:"emit"`
	Rem    struct{ CNPJ, CPF, XNome string } `xml:"rem"`
	Dest   struct{ CNPJ, CPF, XNome string } `xml:"dest"`
	Exped  struct{ CNPJ string }             `xml:"exped"`
	Receb  struct{ CNPJ string }             `xml:"receb"`
	VPrest struct {
		VTPrest Valor `xml:"vTPrest"`
		VRec    Valor `xml:"vRec"`
	} `xml:"vPrest"`
	Imp struct {
		ICMS grupoAny `xml:"ICMS"`
	} `xml:"imp"`
	InfCTeNorm struct {
		InfDoc struct {
			InfNFe []struct {
				Chave string `xml:"chave"`
			} `xml:"infNFe"`
		} `xml:"infDoc"`
	} `xml:"infCTeNorm"`
}

// ---------- MDF-e ----------

type MDFeProc struct {
	MDFe struct {
		InfMDFe struct {
			ID  string `xml:"Id,attr"`
			Ide struct {
				CUF    string `xml:"cUF"`
				Mod    string `xml:"mod"`
				Serie  string `xml:"serie"`
				NMDF   string `xml:"nMDF"`
				DhEmi  string `xml:"dhEmi"`
				TpEmis string `xml:"tpEmis"`
				UFIni  string `xml:"UFIni"`
				UFFim  string `xml:"UFFim"`
			} `xml:"ide"`
			Emit struct {
				CNPJ      string   `xml:"CNPJ"`
				XNome     string   `xml:"xNome"`
				EnderEmit Endereco `xml:"enderEmit"`
			} `xml:"emit"`
			Tot struct {
				VCarga Valor `xml:"vCarga"`
			} `xml:"tot"`
		} `xml:"infMDFe"`
	} `xml:"MDFe"`
	ProtMDFe *struct {
		InfProt struct {
			NProt    string `xml:"nProt"`
			CStat    int    `xml:"cStat"`
			XMotivo  string `xml:"xMotivo"`
			DhRecbto string `xml:"dhRecbto"`
		} `xml:"infProt"`
	} `xml:"protMDFe"`
}

// ---------- NFS-e ----------

// NFSe é o modelo normalizado de NFS-e (Padrão Nacional ou ABRASF).
type NFSe struct {
	Padrao           string  `json:"padrao"` // NACIONAL | ABRASF
	Chave            string  `json:"chave"`
	Numero           string  `json:"numero"`
	DataEmissao      string  `json:"data_emissao"`
	PrestadorDoc     string  `json:"prestador_doc"`
	PrestadorNome    string  `json:"prestador_nome"`
	PrestadorMun     string  `json:"prestador_mun"`
	PrestadorSimples bool    `json:"prestador_simples"`
	TomadorDoc       string  `json:"tomador_doc"`
	TomadorNome      string  `json:"tomador_nome"`
	CodigoServico    string  `json:"codigo_servico"` // subitem LC 116 normalizado (ex.: 7.02)
	CTribNac         string  `json:"ctrib_nac"`
	CodMunIncidencia string  `json:"cod_mun_incidencia"`
	CodMunPrestacao  string  `json:"cod_mun_prestacao"`
	ValorServicos    float64 `json:"valor_servicos"`
	ValorDeducoes    float64 `json:"valor_deducoes"`
	BaseCalculo      float64 `json:"base_calculo"`
	AliquotaISS      float64 `json:"aliquota_iss"`
	ValorISS         float64 `json:"valor_iss"`
	ISSRetido        bool    `json:"iss_retido"`
	ValorIR          float64 `json:"valor_ir"`
	ValorPIS         float64 `json:"valor_pis"`
	ValorCOFINS      float64 `json:"valor_cofins"`
	ValorCSLL        float64 `json:"valor_csll"`
	ValorINSS        float64 `json:"valor_inss"`
	ValorLiquido     float64 `json:"valor_liquido"`
	Cancelada        bool    `json:"cancelada"`
}

// nfseNacional mapeia o XML da NFS-e Padrão Nacional (leiaute SEFIN Nacional 1.00).
type nfseNacional struct {
	InfNFSe struct {
		ID        string `xml:"Id,attr"`
		XLocEmi   string `xml:"xLocEmi"`
		NNFSe     string `xml:"nNFSe"`
		CLocIncid string `xml:"cLocIncid"`
		DhProc    string `xml:"dhProc"`
		Emit      struct {
			CNPJ     string `xml:"CNPJ"`
			CPF      string `xml:"CPF"`
			XNome    string `xml:"xNome"`
			EnderNac struct {
				CMun string `xml:"cMun"`
			} `xml:"enderNac"`
		} `xml:"emit"`
		Valores struct {
			VBC        Valor `xml:"vBC"`
			PAliqAplic Valor `xml:"pAliqAplic"`
			VISSQN     Valor `xml:"vISSQN"`
			VLiq       Valor `xml:"vLiq"`
		} `xml:"valores"`
		DPS struct {
			InfDPS struct {
				DhEmi string `xml:"dhEmi"`
				Prest struct {
					RegTrib struct {
						OpSimpNac string `xml:"opSimpNac"`
					} `xml:"regTrib"`
				} `xml:"prest"`
				Toma struct {
					CNPJ  string `xml:"CNPJ"`
					CPF   string `xml:"CPF"`
					XNome string `xml:"xNome"`
				} `xml:"toma"`
				Serv struct {
					LocPrest struct {
						CLocPrestacao string `xml:"cLocPrestacao"`
					} `xml:"locPrest"`
					CServ struct {
						CTribNac string `xml:"cTribNac"`
					} `xml:"cServ"`
				} `xml:"serv"`
				Valores struct {
					VServPrest struct {
						VServ Valor `xml:"vServ"`
					} `xml:"vServPrest"`
					Trib struct {
						TribMun struct {
							TribISSQN  string `xml:"tribISSQN"`
							TpRetISSQN string `xml:"tpRetISSQN"`
							PAliq      Valor  `xml:"pAliq"`
						} `xml:"tribMun"`
						TribFed struct {
							PisCofins struct {
								VPis    Valor  `xml:"vPis"`
								VCofins Valor  `xml:"vCofins"`
								TpRet   string `xml:"tpRetPisCofins"`
							} `xml:"piscofins"`
							VRetCP   Valor `xml:"vRetCP"`
							VRetIRRF Valor `xml:"vRetIRRF"`
							VRetCSLL Valor `xml:"vRetCSLL"`
						} `xml:"tribFed"`
					} `xml:"trib"`
				} `xml:"valores"`
			} `xml:"infDPS"`
		} `xml:"DPS"`
	} `xml:"infNFSe"`
}

func (n nfseNacional) Normaliza() NFSe {
	i := n.InfNFSe
	d := i.DPS.InfDPS
	o := NFSe{
		Padrao: "NACIONAL", Chave: trimPrefixAny(i.ID, "NFS"), Numero: i.NNFSe, DataEmissao: d.DhEmi,
		PrestadorDoc: first(i.Emit.CNPJ, i.Emit.CPF), PrestadorNome: i.Emit.XNome, PrestadorMun: i.Emit.EnderNac.CMun,
		PrestadorSimples: d.Prest.RegTrib.OpSimpNac == "2" || d.Prest.RegTrib.OpSimpNac == "3",
		TomadorDoc:       first(d.Toma.CNPJ, d.Toma.CPF), TomadorNome: d.Toma.XNome,
		CTribNac: d.Serv.CServ.CTribNac, CodMunIncidencia: i.CLocIncid, CodMunPrestacao: d.Serv.LocPrest.CLocPrestacao,
		ValorServicos: d.Valores.VServPrest.VServ.F(), BaseCalculo: i.Valores.VBC.F(),
		AliquotaISS: first2(i.Valores.PAliqAplic.F(), d.Valores.Trib.TribMun.PAliq.F()), ValorISS: i.Valores.VISSQN.F(),
		ISSRetido: d.Valores.Trib.TribMun.TpRetISSQN == "2" || d.Valores.Trib.TribMun.TpRetISSQN == "3",
		ValorIR:   d.Valores.Trib.TribFed.VRetIRRF.F(), ValorCSLL: d.Valores.Trib.TribFed.VRetCSLL.F(),
		ValorINSS: d.Valores.Trib.TribFed.VRetCP.F(), ValorLiquido: i.Valores.VLiq.F(),
	}
	if t := d.Valores.Trib.TribFed.PisCofins.TpRet; t == "1" || t == "" {
		// tpRetPisCofins=1 → retido; demais valores apenas informativos
		o.ValorPIS = d.Valores.Trib.TribFed.PisCofins.VPis.F()
		o.ValorCOFINS = d.Valores.Trib.TribFed.PisCofins.VCofins.F()
	}
	return o
}

// nfseABRASF mapeia o InfNfse do padrão ABRASF 2.0x.
type nfseABRASF struct {
	Numero      string `xml:"Numero"`
	DataEmissao string `xml:"DataEmissao"`
	ValoresNfse struct {
		BaseCalculo  Valor `xml:"BaseCalculo"`
		Aliquota     Valor `xml:"Aliquota"`
		ValorIss     Valor `xml:"ValorIss"`
		ValorLiquido Valor `xml:"ValorLiquidoNfse"`
	} `xml:"ValoresNfse"`
	PrestadorServico struct {
		RazaoSocial string `xml:"RazaoSocial"`
		Endereco    struct {
			CodigoMunicipio string `xml:"CodigoMunicipio"`
		} `xml:"Endereco"`
	} `xml:"PrestadorServico"`
	Decl struct {
		Inf struct {
			Servico struct {
				Valores struct {
					ValorServicos Valor `xml:"ValorServicos"`
					ValorDeducoes Valor `xml:"ValorDeducoes"`
					ValorPis      Valor `xml:"ValorPis"`
					ValorCofins   Valor `xml:"ValorCofins"`
					ValorInss     Valor `xml:"ValorInss"`
					ValorIr       Valor `xml:"ValorIr"`
					ValorCsll     Valor `xml:"ValorCsll"`
					ValorIss      Valor `xml:"ValorIss"`
					Aliquota      Valor `xml:"Aliquota"`
				} `xml:"Valores"`
				IssRetido           string `xml:"IssRetido"`
				ItemListaServico    string `xml:"ItemListaServico"`
				CodigoMunicipio     string `xml:"CodigoMunicipio"`
				MunicipioIncidencia string `xml:"MunicipioIncidencia"`
			} `xml:"Servico"`
			Prestador struct {
				CpfCnpj struct{ Cnpj, Cpf string } `xml:"CpfCnpj"`
			} `xml:"Prestador"`
			Tomador struct {
				IdentificacaoTomador struct {
					CpfCnpj struct{ Cnpj, Cpf string } `xml:"CpfCnpj"`
				} `xml:"IdentificacaoTomador"`
				RazaoSocial string `xml:"RazaoSocial"`
			} `xml:"TomadorServico"`
			OptanteSimplesNacional string `xml:"OptanteSimplesNacional"`
		} `xml:"InfDeclaracaoPrestacaoServico"`
	} `xml:"DeclaracaoPrestacaoServico"`
}

func (n nfseABRASF) Normaliza() NFSe {
	s := n.Decl.Inf.Servico
	v := s.Valores
	o := NFSe{
		Padrao: "ABRASF", Numero: n.Numero, DataEmissao: n.DataEmissao,
		PrestadorDoc:  first(n.Decl.Inf.Prestador.CpfCnpj.Cnpj, n.Decl.Inf.Prestador.CpfCnpj.Cpf),
		PrestadorNome: n.PrestadorServico.RazaoSocial, PrestadorMun: n.PrestadorServico.Endereco.CodigoMunicipio,
		PrestadorSimples: n.Decl.Inf.OptanteSimplesNacional == "1",
		TomadorDoc:       first(n.Decl.Inf.Tomador.IdentificacaoTomador.CpfCnpj.Cnpj, n.Decl.Inf.Tomador.IdentificacaoTomador.CpfCnpj.Cpf),
		TomadorNome:      n.Decl.Inf.Tomador.RazaoSocial, CodigoServico: s.ItemListaServico,
		CodMunIncidencia: first(s.MunicipioIncidencia, s.CodigoMunicipio), CodMunPrestacao: s.CodigoMunicipio,
		ValorServicos: v.ValorServicos.F(), ValorDeducoes: v.ValorDeducoes.F(),
		BaseCalculo: n.ValoresNfse.BaseCalculo.F(), AliquotaISS: first2(n.ValoresNfse.Aliquota.F(), v.Aliquota.F()),
		ValorISS: first2(n.ValoresNfse.ValorIss.F(), v.ValorIss.F()), ISSRetido: s.IssRetido == "1",
		ValorIR: v.ValorIr.F(), ValorPIS: v.ValorPis.F(), ValorCOFINS: v.ValorCofins.F(), ValorCSLL: v.ValorCsll.F(),
		ValorINSS: v.ValorInss.F(), ValorLiquido: n.ValoresNfse.ValorLiquido.F(),
	}
	// ABRASF 1.0 informa alíquota como fração (0.05); normaliza para percentual.
	if o.AliquotaISS > 0 && o.AliquotaISS < 1 {
		o.AliquotaISS *= 100
	}
	if o.BaseCalculo == 0 {
		o.BaseCalculo = o.ValorServicos - o.ValorDeducoes
	}
	return o
}

// ---------- Eventos e resumos (Distribuição DF-e) ----------

type ProcEvento struct {
	Evento struct {
		InfEvento struct {
			CNPJ       string `xml:"CNPJ"`
			ChNFe      string `xml:"chNFe"`
			DhEvento   string `xml:"dhEvento"`
			TpEvento   string `xml:"tpEvento"`
			NSeqEvento string `xml:"nSeqEvento"`
			DetEvento  struct {
				DescEvento string `xml:"descEvento"`
				NProt      string `xml:"nProt"`
				XJust      string `xml:"xJust"`
				XCorrecao  string `xml:"xCorrecao"`
			} `xml:"detEvento"`
		} `xml:"infEvento"`
	} `xml:"evento"`
	RetEvento struct {
		InfEvento struct {
			CStat       int    `xml:"cStat"`
			XMotivo     string `xml:"xMotivo"`
			NProt       string `xml:"nProt"`
			DhRegEvento string `xml:"dhRegEvento"`
		} `xml:"infEvento"`
	} `xml:"retEvento"`
}

type ResNFe struct {
	XMLName  xml.Name `xml:"resNFe"`
	ChNFe    string   `xml:"chNFe"`
	CNPJ     string   `xml:"CNPJ"`
	CPF      string   `xml:"CPF"`
	XNome    string   `xml:"xNome"`
	IE       string   `xml:"IE"`
	DhEmi    string   `xml:"dhEmi"`
	TpNF     string   `xml:"tpNF"`
	VNF      Valor    `xml:"vNF"`
	DhRecbto string   `xml:"dhRecbto"`
	NProt    string   `xml:"nProt"`
	CSitNFe  string   `xml:"cSitNFe"` // 1=autorizada 2=denegada 3=cancelada
}

func first(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func first2(v ...float64) float64 {
	for _, s := range v {
		if s != 0 {
			return s
		}
	}
	return 0
}

func trimPrefixAny(s, p string) string {
	if len(s) > len(p) && s[:len(p)] == p {
		return s[len(p):]
	}
	return s
}
