package sefaz

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Servicos identifica os webservices utilizados pelo Monitor.
const (
	SvcStatus   = "NFeStatusServico4"
	SvcConsulta = "NFeConsultaProtocolo4"
	SvcDistDFe  = "NFeDistribuicaoDFe"
	SvcEvento   = "NFeRecepcaoEvento4"
	SvcCadastro = "CadConsultaCadastro4"
)

// autorizadorPorUF: SEFAZ autorizadora de NF-e de cada UF.
var autorizadorPorUF = map[string]string{
	"AM": "AM", "BA": "BA", "GO": "GO", "MG": "MG", "MS": "MS", "MT": "MT", "PE": "PE", "PR": "PR", "RS": "RS", "SP": "SP",
	"MA": "SVAN",
	"AC": "SVRS", "AL": "SVRS", "AP": "SVRS", "CE": "SVRS", "DF": "SVRS", "ES": "SVRS", "PA": "SVRS", "PB": "SVRS", "PI": "SVRS",
	"RJ": "SVRS", "RN": "SVRS", "RO": "SVRS", "RR": "SVRS", "SC": "SVRS", "SE": "SVRS", "TO": "SVRS",
}

// URLs[ambiente][autorizador][servico]. Valores de referência (Portal Nacional da NF-e).
// Podem ser sobrescritos por <dados>/webservices.json sem recompilar.
var URLs = map[string]map[string]map[string]string{
	"1": {
		"AN":     {SvcDistDFe: "https://www1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx", SvcEvento: "https://www.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx"},
		"SVRS":   {SvcStatus: "https://nfe.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx", SvcConsulta: "https://nfe.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx"},
		"SVAN":   {SvcStatus: "https://www.sefazvirtual.fazenda.gov.br/NFeStatusServico4/NFeStatusServico4.asmx", SvcConsulta: "https://www.sefazvirtual.fazenda.gov.br/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx"},
		"SVC-AN": {SvcStatus: "https://www.svc.fazenda.gov.br/NFeStatusServico4/NFeStatusServico4.asmx", SvcConsulta: "https://www.svc.fazenda.gov.br/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx"},
		"SVC-RS": {SvcStatus: "https://nfe.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx", SvcConsulta: "https://nfe.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx"},
		"SP":     {SvcStatus: "https://nfe.fazenda.sp.gov.br/ws/nfestatusservico4.asmx", SvcConsulta: "https://nfe.fazenda.sp.gov.br/ws/nfeconsultaprotocolo4.asmx"},
		"RS":     {SvcStatus: "https://nfe.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx", SvcConsulta: "https://nfe.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx"},
		"MG":     {SvcStatus: "https://nfe.fazenda.mg.gov.br/nfe2/services/NFeStatusServico4", SvcConsulta: "https://nfe.fazenda.mg.gov.br/nfe2/services/NFeConsultaProtocolo4"},
		"PR":     {SvcStatus: "https://nfe.sefa.pr.gov.br/nfe/NFeStatusServico4", SvcConsulta: "https://nfe.sefa.pr.gov.br/nfe/NFeConsultaProtocolo4"},
		"GO":     {SvcStatus: "https://nfe.sefaz.go.gov.br/nfe/services/NFeStatusServico4", SvcConsulta: "https://nfe.sefaz.go.gov.br/nfe/services/NFeConsultaProtocolo4"},
		"BA":     {SvcStatus: "https://nfe.sefaz.ba.gov.br/webservices/NFeStatusServico4/NFeStatusServico4.asmx", SvcConsulta: "https://nfe.sefaz.ba.gov.br/webservices/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx"},
		"MT":     {SvcStatus: "https://nfe.sefaz.mt.gov.br/nfews/v2/services/NfeStatusServico4", SvcConsulta: "https://nfe.sefaz.mt.gov.br/nfews/v2/services/NfeConsulta4"},
		"MS":     {SvcStatus: "https://nfe.sefaz.ms.gov.br/ws/NFeStatusServico4", SvcConsulta: "https://nfe.sefaz.ms.gov.br/ws/NFeConsultaProtocolo4"},
		"PE":     {SvcStatus: "https://nfe.sefaz.pe.gov.br/nfe-service/services/NFeStatusServico4", SvcConsulta: "https://nfe.sefaz.pe.gov.br/nfe-service/services/NFeConsultaProtocolo4"},
		"AM":     {SvcStatus: "https://nfe.sefaz.am.gov.br/services2/services/NfeStatusServico4", SvcConsulta: "https://nfe.sefaz.am.gov.br/services2/services/NfeConsulta4"},
	},
	"2": {
		"AN":     {SvcDistDFe: "https://hom1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx", SvcEvento: "https://hom1.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx"},
		"SVRS":   {SvcStatus: "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx", SvcConsulta: "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx"},
		"SVAN":   {SvcStatus: "https://hom.sefazvirtual.fazenda.gov.br/NFeStatusServico4/NFeStatusServico4.asmx", SvcConsulta: "https://hom.sefazvirtual.fazenda.gov.br/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx"},
		"SVC-AN": {SvcStatus: "https://hom.svc.fazenda.gov.br/NFeStatusServico4/NFeStatusServico4.asmx", SvcConsulta: "https://hom.svc.fazenda.gov.br/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx"},
		"SVC-RS": {SvcStatus: "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx", SvcConsulta: "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx"},
		"SP":     {SvcStatus: "https://homologacao.nfe.fazenda.sp.gov.br/ws/nfestatusservico4.asmx", SvcConsulta: "https://homologacao.nfe.fazenda.sp.gov.br/ws/nfeconsultaprotocolo4.asmx"},
		"RS":     {SvcStatus: "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx", SvcConsulta: "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx"},
		"MG":     {SvcStatus: "https://hnfe.fazenda.mg.gov.br/nfe2/services/NFeStatusServico4", SvcConsulta: "https://hnfe.fazenda.mg.gov.br/nfe2/services/NFeConsultaProtocolo4"},
		"PR":     {SvcStatus: "https://homologacao.nfe.sefa.pr.gov.br/nfe/NFeStatusServico4", SvcConsulta: "https://homologacao.nfe.sefa.pr.gov.br/nfe/NFeConsultaProtocolo4"},
		"GO":     {SvcStatus: "https://homolog.sefaz.go.gov.br/nfe/services/NFeStatusServico4", SvcConsulta: "https://homolog.sefaz.go.gov.br/nfe/services/NFeConsultaProtocolo4"},
		"BA":     {SvcStatus: "https://hnfe.sefaz.ba.gov.br/webservices/NFeStatusServico4/NFeStatusServico4.asmx", SvcConsulta: "https://hnfe.sefaz.ba.gov.br/webservices/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx"},
		"MT":     {SvcStatus: "https://homologacao.sefaz.mt.gov.br/nfews/v2/services/NfeStatusServico4", SvcConsulta: "https://homologacao.sefaz.mt.gov.br/nfews/v2/services/NfeConsulta4"},
		"MS":     {SvcStatus: "https://hom.nfe.sefaz.ms.gov.br/ws/NFeStatusServico4", SvcConsulta: "https://hom.nfe.sefaz.ms.gov.br/ws/NFeConsultaProtocolo4"},
		"PE":     {SvcStatus: "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services/NFeStatusServico4", SvcConsulta: "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services/NFeConsultaProtocolo4"},
		"AM":     {SvcStatus: "https://homnfe.sefaz.am.gov.br/services2/services/NfeStatusServico4", SvcConsulta: "https://homnfe.sefaz.am.gov.br/services2/services/NfeConsulta4"},
	},
}

// CarregarOverrides aplica <dados>/webservices.json (mesma estrutura de URLs).
func CarregarOverrides(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, "webservices.json"))
	if err != nil {
		return nil
	}
	var o map[string]map[string]map[string]string
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	for amb, auts := range o {
		if URLs[amb] == nil {
			URLs[amb] = map[string]map[string]string{}
		}
		for aut, svcs := range auts {
			if URLs[amb][aut] == nil {
				URLs[amb][aut] = map[string]string{}
			}
			for s, u := range svcs {
				URLs[amb][aut][s] = u
			}
		}
	}
	return nil
}

// Autorizador retorna o autorizador da UF (ou SVC conforme tpEmis 6/7).
func Autorizador(uf, tpEmis string) string {
	switch tpEmis {
	case "6":
		return "SVC-AN"
	case "7":
		return "SVC-RS"
	}
	if a, ok := autorizadorPorUF[uf]; ok {
		return a
	}
	return "SVRS"
}

// URLsCadastro: CadConsultaCadastro4 (consulta de contribuinte com certificado) por UF.
// UFs que não oferecem o serviço ficam sem URL; é possível incluí-las em webservices.json
// (chave "CAD" do ambiente "1").
var URLsCadastro = map[string]string{
	"SP": "https://nfe.fazenda.sp.gov.br/ws/cadconsultacadastro4.asmx",
	"MG": "https://nfe.fazenda.mg.gov.br/nfe2/services/CadConsultaCadastro4",
	"PR": "https://nfe.sefa.pr.gov.br/nfe/CadConsultaCadastro4",
	"RS": "https://cad.sefazrs.rs.gov.br/ws/cadconsultacadastro/cadconsultacadastro4.asmx",
	"BA": "https://nfe.sefaz.ba.gov.br/webservices/CadConsultaCadastro4/CadConsultaCadastro4.asmx",
	"GO": "https://nfe.sefaz.go.gov.br/nfe/services/CadConsultaCadastro4",
	"MT": "https://nfe.sefaz.mt.gov.br/nfews/v2/services/CadConsultaCadastro4",
	"MS": "https://nfe.sefaz.ms.gov.br/ws/CadConsultaCadastro4",
	"PE": "https://nfe.sefaz.pe.gov.br/nfe-service/services/CadConsultaCadastro4",
	"AM": "https://nfe.sefaz.am.gov.br/services2/services/cadconsultacadastro4",
	"AC": "https://cad.svrs.rs.gov.br/ws/cadconsultacadastro/cadconsultacadastro4.asmx",
	"PB": "https://cad.svrs.rs.gov.br/ws/cadconsultacadastro/cadconsultacadastro4.asmx",
	"RN": "https://cad.svrs.rs.gov.br/ws/cadconsultacadastro/cadconsultacadastro4.asmx",
	"SC": "https://cad.svrs.rs.gov.br/ws/cadconsultacadastro/cadconsultacadastro4.asmx",
}

func urlCadastro(uf string) string {
	if u := URLs["1"]["CAD"][uf]; u != "" {
		return u
	}
	return URLsCadastro[uf]
}
