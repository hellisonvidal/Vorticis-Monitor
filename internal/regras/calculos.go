package regras

// Cálculos tributários expostos também pela API de simulação.

// DIFAL é o resultado do cálculo do diferencial de alíquota.
type DIFAL struct {
	Metodo      string  `json:"metodo"`
	BaseOrigem  float64 `json:"base_origem"`
	BaseDestino float64 `json:"base_destino"`
	AliqInter   float64 `json:"aliq_inter"`
	AliqIntra   float64 `json:"aliq_intra"`
	AliqFCP     float64 `json:"aliq_fcp"`
	ValorDIFAL  float64 `json:"valor_difal"`
	ValorFCP    float64 `json:"valor_fcp"`
}

// CalculaDIFAL calcula o DIFAL (EC 87/2015, LC 190/2022).
//
//	Base única: DIFAL = BC × (Aint − Ainter)
//	Base dupla: BCdest = (BC − BC×Ainter) / (1 − Aint);  DIFAL = BCdest×Aint − BC×Ainter
func CalculaDIFAL(base, aliqInter, aliqIntra, aliqFCP float64, metodo string) DIFAL {
	d := DIFAL{Metodo: metodo, BaseOrigem: r2(base), AliqInter: aliqInter, AliqIntra: aliqIntra, AliqFCP: aliqFCP}
	ai, ad := aliqInter/100, (aliqIntra+aliqFCP)/100
	if metodo == "BASE_UNICA" {
		d.BaseDestino = r2(base)
		d.ValorDIFAL = r2(base * (aliqIntra/100 - ai))
	} else {
		d.BaseDestino = r2((base - base*ai) / (1 - ad))
		d.ValorDIFAL = r2(d.BaseDestino*aliqIntra/100 - base*ai)
	}
	if d.ValorDIFAL < 0 {
		d.ValorDIFAL = 0
	}
	d.ValorFCP = r2(d.BaseDestino * aliqFCP / 100)
	return d
}

// MVAAjustada aplica a fórmula do Convênio ICMS 142/2018 (cláusula 10ª):
//
//	MVA ajustada = [(1 + MVA-ST original) × (1 − ALQ inter) / (1 − ALQ intra)] − 1
func MVAAjustada(mvaOriginal, aliqInter, aliqIntra float64) float64 {
	if aliqInter >= aliqIntra {
		return mvaOriginal
	}
	v := ((1+mvaOriginal/100)*(1-aliqInter/100)/(1-aliqIntra/100) - 1) * 100
	return float64(int(v*100+0.5)) / 100
}

// ICMSST é o resultado do cálculo da substituição tributária.
type ICMSST struct {
	BaseOperacao float64 `json:"base_operacao"`
	MVA          float64 `json:"mva"`
	RedBC        float64 `json:"reducao_bc"`
	BaseST       float64 `json:"base_st"`
	AliqIntra    float64 `json:"aliq_intra"`
	ICMSProprio  float64 `json:"icms_proprio"`
	ValorST      float64 `json:"valor_st"`
}

// CalculaICMSST calcula a base e o ICMS-ST retido.
//
//	BCST = (vProd + vFrete + vSeg + vOutro − vDesc + vIPI) × (1 + MVA) × (1 − pRedBCST)
//	ICMS-ST = BCST × Aint − ICMS próprio
func CalculaICMSST(baseOperacao, mva, redBC, aliqIntra, icmsProprio float64) ICMSST {
	bst := baseOperacao * (1 + mva/100)
	if redBC > 0 {
		bst *= 1 - redBC/100
	}
	st := bst*aliqIntra/100 - icmsProprio
	if st < 0 {
		st = 0
	}
	return ICMSST{BaseOperacao: r2(baseOperacao), MVA: mva, RedBC: redBC, BaseST: r2(bst), AliqIntra: aliqIntra, ICMSProprio: r2(icmsProprio), ValorST: r2(st)}
}

// Retencoes calcula as retenções federais esperadas para uma NFS-e.
type Retencoes struct {
	IRRF          float64 `json:"irrf"`
	PIS           float64 `json:"pis"`
	COFINS        float64 `json:"cofins"`
	CSLL          float64 `json:"csll"`
	INSS          float64 `json:"inss"`
	PCCDispensado bool    `json:"pcc_dispensado"`
	IRDispensado  bool    `json:"ir_dispensado"`
}

// CalculaRetencoes aplica as alíquotas padrão: IRRF 1,5% (ou 1% p/ limpeza,
// conservação, segurança e locação de mão de obra), PCC 4,65% (0,65+3+1),
// INSS 11% em cessão de mão de obra. Valores ≤ R$ 10,00 são dispensados
// (Lei 9.430/96 art. 67; Lei 10.833/03 art. 31 §3º, red. Lei 13.137/15).
func CalculaRetencoes(base float64, subitem string, mesmoGrupoIR1 bool, cessaoMaoObra bool) Retencoes {
	r := Retencoes{}
	aliqIR := 1.5
	if mesmoGrupoIR1 {
		aliqIR = 1.0
	}
	r.IRRF = r2(base * aliqIR / 100)
	if r.IRRF <= 10 {
		r.IRRF, r.IRDispensado = 0, true
	}
	r.PIS, r.COFINS, r.CSLL = r2(base*0.0065), r2(base*0.03), r2(base*0.01)
	if r.PIS+r.COFINS+r.CSLL <= 10 {
		r.PIS, r.COFINS, r.CSLL, r.PCCDispensado = 0, 0, 0, true
	}
	if cessaoMaoObra {
		r.INSS = r2(base * 0.11)
	}
	return r
}
