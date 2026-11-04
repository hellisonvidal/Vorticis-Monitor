package relatorios

import (
	"fmt"
	"net/url"
	"strings"

	"vorticis.com.br/monitor/internal/store"
)

// Nomes dos arquivos gerados:
//   - PDF: "NF 123 - Fornecedor.pdf" (NF-e), "NFS 66 - Fornecedor.pdf" (NFS-e) e o
//     mesmo padrão para os demais modelos (NFC, CT, MDF), com o nome do emitente;
//   - XML e PDF + XML: pela chave de acesso.
var prefixoPDF = map[string]string{"NFE": "NF", "NFCE": "NFC", "NFSE": "NFS", "CTE": "CT", "MDFE": "MDF"}

// NomePDF devolve o nome do PDF do documento no padrão "NF 123 - Fornecedor.pdf".
func NomePDF(d *store.Documento) string {
	p := prefixoPDF[d.Modelo]
	if p == "" {
		p = d.Modelo
	}
	n := strings.TrimLeft(d.Numero, "0")
	if n == "" {
		n = d.Numero
	}
	nome := strings.TrimSpace(p + " " + n)
	if f := strings.TrimSpace(d.EmitNome); f != "" {
		nome += " - " + f
	}
	return LimparNomeArquivo(nome) + ".pdf"
}

// NomeChave devolve "<chave>.<ext>".
func NomeChave(d *store.Documento, ext string) string { return LimparNomeArquivo(d.Chave) + "." + ext }

// LimparNomeArquivo remove caracteres que o Windows não aceita em nomes de arquivo.
func LimparNomeArquivo(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r < 32 || strings.ContainsRune(`\/:*?"<>|`, r):
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		s = strings.TrimSpace(string(r[:120]))
	}
	return strings.TrimRight(s, ". ")
}

var semAcentos = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "ê", "e", "è", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ü", "u", "ç", "c",
	"Á", "A", "À", "A", "Â", "A", "Ã", "A", "É", "E", "Ê", "E", "Í", "I", "Ó", "O", "Ô", "O", "Õ", "O", "Ú", "U", "Ü", "U", "Ç", "C", "º", "o", "ª", "a")

// ContentDisposition monta o cabeçalho com o nome em UTF-8 e uma versão sem acentos.
func ContentDisposition(tipo, nome string) string {
	ascii := strings.Map(func(r rune) rune {
		if r > 126 {
			return '_'
		}
		return r
	}, semAcentos.Replace(nome))
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, tipo, strings.ReplaceAll(ascii, `"`, ""), url.PathEscape(nome))
}
