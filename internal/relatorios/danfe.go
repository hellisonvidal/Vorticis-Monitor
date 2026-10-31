package relatorios

import (
	"fmt"
	"html/template"
	"io"
	"strings"

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/regras"
	"vorticis.com.br/monitor/internal/store"
)

// code128 contém os padrões de largura (barra/espaço) dos símbolos 0–106.
var code128 = strings.Fields(`212222 222122 222221 121223 121322 131222 122213 122312 132212 221213
221312 231212 112232 122132 122231 113222 123122 123221 223211 221132 221231 213212 223112 312131
311222 321122 321221 312212 322112 322211 212123 212321 232121 111323 131123 131321 112313 132113
132311 211313 231113 231311 112133 112331 132131 113123 113321 133121 313121 211331 231131 213113
213311 213131 311123 311321 331121 312113 312311 332111 314111 221411 431111 111224 111422 121124
121421 141122 141221 112214 112412 122114 122411 142112 142211 241211 221114 413111 241112 134111
111242 121142 121241 114212 124112 124211 411212 421112 421211 212141 214121 412121 111143 111341
131141 114113 114311 411113 411311 113141 114131 311141 411131 211412 211214 211232 2331112`)

// Code128C gera o SVG do código de barras (subconjunto C, numérico de comprimento par).
func Code128C(num string, altura int) template.HTML {
	num = fiscal.SoDigitos(num)
	if len(num)%2 == 1 {
		num = "0" + num
	}
	codes := []int{105}
	soma := 105
	for i := 0; i < len(num); i += 2 {
		val := int(num[i]-'0')*10 + int(num[i+1]-'0')
		codes = append(codes, val)
		soma += val * (i/2 + 1)
	}
	codes = append(codes, soma%103, 106)
	var b strings.Builder
	x := 10 // zona de silêncio
	for _, c := range codes {
		for j, ch := range code128[c] {
			w := int(ch - '0')
			if j%2 == 0 {
				fmt.Fprintf(&b, `<rect x="%d" y="0" width="%d" height="%d"/>`, x, w, altura)
			}
			x += w
		}
	}
	return template.HTML(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="100%%" height="%d" preserveAspectRatio="none">%s</svg>`, x+10, altura, altura, b.String()))
}

func formatarChave(c string) string {
	var p []string
	for i := 0; i < len(c); i += 4 {
		p = append(p, c[i:min(i+4, len(c))])
	}
	return strings.Join(p, " ")
}

var tplDANFE = template.Must(template.New("danfe").Funcs(template.FuncMap{
	"v":     func(x float64) string { return f2(x) },
	"q":     func(x float64) string { return strings.Replace(fmt.Sprintf("%.4f", x), ".", ",", 1) },
	"chave": formatarChave,
	"data": func(s string) string {
		t := regras.ParseData(s)
		if t.IsZero() {
			return s
		}
		return t.Format("02/01/2006 15:04")
	},
	"doc": func(d string) string {
		d = fiscal.NormalizaCNPJ(d)
		if len(d) == 14 {
			return d[0:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:]
		}
		if len(d) == 11 {
			return d[0:3] + "." + d[3:6] + "." + d[6:9] + "-" + d[9:]
		}
		return d
	},
}).Parse(`<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><title>{{.Titulo}} {{.D.Numero}}</title>
<style>
body{font:10px Arial,Helvetica,sans-serif;margin:8mm;color:#000}
.box{border:1px solid #000;padding:3px 5px;margin:-1px 0 0 -1px}
.row{display:flex}.row>.box{flex:1}
.lbl{font-size:7px;text-transform:uppercase;color:#333}
.val{font-size:10px;font-weight:bold;min-height:12px}
h1{font-size:14px;margin:0}table{border-collapse:collapse;width:100%;margin-top:4px}
th,td{border:1px solid #000;padding:2px 3px;font-size:8px}th{background:#eee}
td.n{text-align:right}.aviso{color:#b00;font-weight:bold;text-align:center;font-size:12px;margin:4px 0}
@media print{body{margin:5mm}.np{display:none}}
</style></head><body>
<div class="np" style="text-align:right;margin-bottom:6px"><button onclick="print()">Imprimir / Salvar PDF</button></div>
{{if .Aviso}}<div class="aviso">{{.Aviso}}</div>{{end}}
<div class="row">
 <div class="box" style="flex:2"><div class="lbl">Emitente</div><div class="val" style="font-size:13px">{{.D.EmitNome}}</div>{{doc .D.EmitDoc}} — {{.D.EmitUF}}</div>
 <div class="box" style="flex:1;text-align:center"><h1>{{.Titulo}}</h1><div>{{.Subtitulo}}</div><div>{{if eq .D.Direcao "E"}}0 - ENTRADA{{else}}1 - SAÍDA{{end}}</div><div class="val">Nº {{.D.Numero}} Série {{.D.Serie}}</div></div>
 <div class="box" style="flex:2">{{if .Barras}}<div style="height:40px">{{.Barras}}</div>{{end}}<div class="lbl">Chave de acesso</div><div class="val">{{chave .D.Chave}}</div>
  <div class="lbl">Consulta de autenticidade no portal nacional da NF-e ou no site da SEFAZ autorizadora</div></div>
</div>
<div class="row"><div class="box" style="flex:2"><div class="lbl">Natureza da operação</div><div class="val">{{.D.Natureza}}</div></div>
 <div class="box"><div class="lbl">Protocolo de autorização de uso</div><div class="val">{{.D.Protocolo}} {{data .D.DhAutorizacao}}</div></div></div>
<div class="row"><div class="box" style="flex:3"><div class="lbl">Destinatário / Remetente</div><div class="val">{{.D.DestNome}}</div></div>
 <div class="box"><div class="lbl">CNPJ/CPF</div><div class="val">{{doc .D.DestDoc}}</div></div>
 <div class="box"><div class="lbl">UF</div><div class="val">{{.D.DestUF}}</div></div>
 <div class="box"><div class="lbl">Data de emissão</div><div class="val">{{data .D.DhEmissao}}</div></div></div>
<div class="row">
 <div class="box"><div class="lbl">Valor ICMS</div><div class="val">{{v .D.ValorICMS}}</div></div>
 <div class="box"><div class="lbl">Valor ICMS-ST</div><div class="val">{{v .D.ValorST}}</div></div>
 <div class="box"><div class="lbl">Valor IPI</div><div class="val">{{v .D.ValorIPI}}</div></div>
 <div class="box"><div class="lbl">PIS</div><div class="val">{{v .D.ValorPIS}}</div></div>
 <div class="box"><div class="lbl">COFINS</div><div class="val">{{v .D.ValorCOFINS}}</div></div>
 {{if .D.ValorDIFAL}}<div class="box"><div class="lbl">ICMS UF destino (DIFAL)</div><div class="val">{{v .D.ValorDIFAL}}</div></div>{{end}}
 {{if .D.ValorISS}}<div class="box"><div class="lbl">ISS</div><div class="val">{{v .D.ValorISS}}</div></div>{{end}}
 <div class="box"><div class="lbl">Valor total</div><div class="val" style="font-size:12px">{{v .D.ValorTotal}}</div></div>
</div>
{{if .Itens}}<table><thead><tr><th>Cód.</th><th>Descrição</th><th>NCM</th><th>CST</th><th>CFOP</th><th>Un</th><th>Qtd</th><th>V. Unit</th><th>V. Total</th><th>BC ICMS</th><th>ICMS</th><th>IPI</th><th>% ICMS</th></tr></thead><tbody>
{{range .Itens}}<tr><td>{{.CodProd}}</td><td>{{.Descricao}}</td><td>{{.NCM}}</td><td>{{.CSTICMS}}</td><td>{{.CFOP}}</td><td>{{.Unidade}}</td><td class="n">{{q .Quantidade}}</td><td class="n">{{q .ValorUnit}}</td><td class="n">{{v .ValorTotal}}</td><td class="n">{{v .VBC}}</td><td class="n">{{v .VICMS}}</td><td class="n">{{v .VIPI}}</td><td class="n">{{v .PICMS}}</td></tr>{{end}}
</tbody></table>{{end}}
<div class="box" style="margin-top:4px"><div class="lbl">Dados adicionais</div>Documento gerado pelo Vorticis Monitor a partir do XML autorizado — representação simplificada para conferência. Status: {{.D.Status}}{{if .D.XMotivo}} ({{.D.XMotivo}}){{end}}.</div>
</body></html>`))

// DANFE escreve a representação gráfica simplificada (HTML imprimível) do documento.
func DANFE(w io.Writer, d *store.Documento, itens []store.Item) error {
	titulo, sub := "DANFE", "Documento Auxiliar da Nota Fiscal Eletrônica"
	switch d.Modelo {
	case "NFCE":
		titulo, sub = "DANFE NFC-e", "Documento Auxiliar da NFC-e"
	case "CTE":
		titulo, sub = "DACTE", "Documento Auxiliar do Conhecimento de Transporte Eletrônico"
	case "MDFE":
		titulo, sub = "DAMDFE", "Documento Auxiliar do Manifesto Eletrônico de Documentos Fiscais"
	case "NFSE":
		titulo, sub = "DANFSe", "Documento Auxiliar da NFS-e"
	}
	aviso := ""
	switch d.Status {
	case "CANCELADA":
		aviso = "DOCUMENTO CANCELADO"
	case "DENEGADA":
		aviso = "USO DENEGADO"
	case "CONTINGENCIA":
		aviso = "EMITIDO EM CONTINGÊNCIA (" + fiscal.DescricaoTpEmis[d.TpEmis] + ") — PENDENTE DE AUTORIZAÇÃO"
	case "PENDENTE", "BLOQUEADA", "REJEITADA":
		aviso = "SEM VALOR FISCAL — DOCUMENTO NÃO AUTORIZADO"
	}
	var barras template.HTML
	if len(d.Chave) == 44 && fiscal.SoDigitos(d.Chave) == d.Chave {
		barras = Code128C(d.Chave, 40)
	}
	return tplDANFE.Execute(w, map[string]any{"D": d, "Itens": itens, "Titulo": titulo, "Subtitulo": sub, "Aviso": aviso, "Barras": barras})
}
