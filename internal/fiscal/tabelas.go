package fiscal

import "strings"

// UFPorCodigo mapeia o código IBGE da UF para a sigla.
var UFPorCodigo = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP", "17": "TO",
	"21": "MA", "22": "PI", "23": "CE", "24": "RN", "25": "PB", "26": "PE", "27": "AL",
	"28": "SE", "29": "BA", "31": "MG", "32": "ES", "33": "RJ", "35": "SP", "41": "PR",
	"42": "SC", "43": "RS", "50": "MS", "51": "MT", "52": "GO", "53": "DF",
}

// CodigoPorUF é o inverso de UFPorCodigo.
var CodigoPorUF = func() map[string]string {
	m := map[string]string{}
	for k, v := range UFPorCodigo {
		m[v] = k
	}
	return m
}()

// ParametroUF representa parâmetros de ICMS por UF.
// ATENÇÃO: valores de referência (modal 2026). Devem ser revisados pelo
// responsável fiscal; podem ser alterados via API /api/v1/parametros/uf.
type ParametroUF struct {
	UF          string  `json:"uf"`
	AliqInterna float64 `json:"aliq_interna"`
	FCP         float64 `json:"fcp"`
	DifalMetodo string  `json:"difal_metodo"` // BASE_DUPLA | BASE_UNICA
}

// ParametrosUFPadrao contém a tabela inicial carregada no primeiro uso.
var ParametrosUFPadrao = []ParametroUF{
	{"AC", 19, 0, "BASE_DUPLA"}, {"AL", 19, 1, "BASE_DUPLA"}, {"AM", 20, 0, "BASE_DUPLA"},
	{"AP", 18, 0, "BASE_UNICA"}, {"BA", 20.5, 0, "BASE_DUPLA"}, {"CE", 20, 0, "BASE_DUPLA"},
	{"DF", 20, 0, "BASE_DUPLA"}, {"ES", 17, 0, "BASE_UNICA"}, {"GO", 19, 0, "BASE_DUPLA"},
	{"MA", 23, 0, "BASE_DUPLA"}, {"MG", 18, 2, "BASE_DUPLA"}, {"MS", 17, 0, "BASE_DUPLA"},
	{"MT", 17, 0, "BASE_UNICA"}, {"PA", 19, 0, "BASE_DUPLA"}, {"PB", 20, 0, "BASE_DUPLA"},
	{"PE", 20.5, 0, "BASE_DUPLA"}, {"PI", 22.5, 0, "BASE_DUPLA"}, {"PR", 19.5, 0, "BASE_DUPLA"},
	{"RJ", 20, 2, "BASE_DUPLA"}, {"RN", 20, 0, "BASE_DUPLA"}, {"RO", 19.5, 0, "BASE_DUPLA"},
	{"RR", 20, 0, "BASE_DUPLA"}, {"RS", 17, 0, "BASE_DUPLA"}, {"SC", 17, 0, "BASE_DUPLA"},
	{"SE", 19, 1, "BASE_DUPLA"}, {"SP", 18, 0, "BASE_UNICA"}, {"TO", 20, 0, "BASE_DUPLA"},
}

var ufsSulSudesteSemES = map[string]bool{"SP": true, "RJ": true, "MG": true, "PR": true, "RS": true, "SC": true}

// AliquotaInterestadual retorna a alíquota interestadual (Res. SF 22/1989 e 13/2012).
func AliquotaInterestadual(ufOrig, ufDest string, origemMercadoria string) float64 {
	switch origemMercadoria {
	case "1", "2", "3", "8":
		return 4
	}
	if ufsSulSudesteSemES[ufOrig] && !ufsSulSudesteSemES[ufDest] {
		return 7
	}
	return 12
}

// SVCPorUF indica o ambiente de contingência SVC de cada UF autorizadora.
// tpEmis 6 = SVC-AN, 7 = SVC-RS.
var SVCPorUF = map[string]string{
	"AC": "6", "AL": "6", "AP": "6", "DF": "6", "ES": "6", "MG": "6", "PB": "6", "RJ": "6",
	"RN": "6", "RO": "6", "RR": "6", "RS": "6", "SC": "6", "SE": "6", "SP": "6", "TO": "6",
	"AM": "7", "BA": "7", "CE": "7", "GO": "7", "MA": "7", "MS": "7", "MT": "7", "PA": "7",
	"PE": "7", "PI": "7", "PR": "7",
}

// DescricaoTpEmis descreve a forma de emissão.
var DescricaoTpEmis = map[string]string{
	"1": "Normal", "2": "Contingência FS-IA", "4": "Contingência EPEC", "5": "Contingência FS-DA",
	"6": "Contingência SVC-AN", "7": "Contingência SVC-RS", "9": "Contingência off-line NFC-e",
}

// CFOPsDevolucao lista CFOPs de devolução/retorno aceitos com finNFe=4.
var CFOPsDevolucao = map[string]bool{}

func init() {
	for _, c := range []string{"201", "202", "203", "204", "208", "209", "210", "410", "411", "412", "413", "503", "504", "553", "555", "556", "660", "661", "662", "918", "919", "921"} {
		for _, p := range []string{"1", "2", "3", "5", "6", "7"} {
			CFOPsDevolucao[p+c] = true
		}
	}
}

// EhCFOPTransporte indica CFOP de prestação de serviço de transporte.
func EhCFOPTransporte(cfop string) bool {
	if len(cfop) != 4 {
		return false
	}
	s := cfop[1:]
	return s >= "351" && s <= "360"
}

// CFOPsUsoConsumoAtivo lista CFOPs de entrada interestadual sujeitos a DIFAL do adquirente contribuinte.
var CFOPsUsoConsumoAtivo = map[string]string{
	"2556": "uso e consumo", "2407": "uso e consumo (ST)", "2551": "ativo imobilizado", "2406": "ativo imobilizado (ST)",
}

// CSTsComST indica CST/CSOSN de ICMS com substituição tributária.
var CSTsComST = map[string]bool{"10": true, "30": true, "60": true, "70": true, "90": false, "201": true, "202": true, "203": true, "500": true}

// CSTsSTRetidaAnteriormente: ST já recolhida (exige CEST mas não cálculo).
var CSTsSTRetidaAnteriormente = map[string]bool{"60": true, "500": true}

// NCMMonofasicoPrefixos: prefixos de NCM com PIS/COFINS monofásico (referência
// Lei 10.147/2000, 10.485/2002, 10.833/2003). Lista resumida e configurável.
var NCMMonofasicoPrefixos = []string{"2710", "2711", "3003", "3004", "3303", "3304", "3305", "3307", "3401", "4011", "4013", "8708", "2201", "2202", "2203", "2204", "2205", "2206", "2207", "2208"}

// EhMonofasico verifica se o NCM se enquadra na lista de monofásicos.
func EhMonofasico(ncm string) bool {
	for _, p := range NCMMonofasicoPrefixos {
		if strings.HasPrefix(ncm, p) {
			return true
		}
	}
	return false
}

// LC116LocalPrestacao lista subitens da LC 116/2003 (art. 3º) cujo ISS é
// devido no local da prestação/estabelecimento do tomador, e não no domicílio do prestador.
var LC116LocalPrestacao = map[string]bool{
	"3.05": true, "7.02": true, "7.04": true, "7.05": true, "7.09": true, "7.10": true, "7.11": true,
	"7.12": true, "7.16": true, "7.17": true, "7.18": true, "7.19": true, "11.01": true, "11.02": true,
	"11.04": true, "12.01": true, "12.02": true, "12.03": true, "12.04": true, "12.05": true, "12.06": true,
	"12.07": true, "12.08": true, "12.09": true, "12.10": true, "12.11": true, "12.12": true, "12.13": true,
	"12.14": true, "12.15": true, "12.16": true, "12.17": true, "16.01": true, "16.02": true, "17.05": true,
	"17.10": true, "20.01": true, "20.02": true, "20.03": true, "4.22": true, "4.23": true, "5.09": true,
	"15.01": true, "15.09": true, "10.04": true, "1.03": true, "1.04": true, "1.09": true,
}

// LC116CessaoMaoObra: subitens tipicamente sujeitos a retenção de INSS (11%) – Lei 8.212/91 art. 31.
var LC116CessaoMaoObra = map[string]bool{"7.02": true, "7.05": true, "7.10": true, "11.02": true, "17.05": true, "7.11": true, "14.01": true}

// LC116RetencaoIR1Pct: serviços de limpeza, conservação, segurança e locação de mão de obra (IRRF 1%).
var LC116RetencaoIR1Pct = map[string]bool{"7.10": true, "11.02": true, "17.05": true, "7.11": true}

// CTribNacParaLC116 converte o código de tributação nacional (6 dígitos: IISSDD) em subitem "I.SS".
func CTribNacParaLC116(c string) string {
	c = SoDigitos(c)
	if len(c) < 4 {
		return ""
	}
	item := strings.TrimLeft(c[0:2], "0")
	return item + "." + c[2:4]
}

// CodigoServicoNormalizado normaliza "0702", "07.02", "7.02" → "7.02".
func CodigoServicoNormalizado(c string) string {
	c = strings.TrimSpace(c)
	if strings.Contains(c, ".") {
		p := strings.SplitN(c, ".", 2)
		return strings.TrimLeft(p[0], "0") + "." + p[1]
	}
	return CTribNacParaLC116(c)
}
