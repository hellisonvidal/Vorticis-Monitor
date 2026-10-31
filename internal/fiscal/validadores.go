// Package fiscal reúne validadores, tabelas de referência e o catálogo de
// rejeições utilizados pelo motor de regras do Vorticis Monitor.
package fiscal

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// SoDigitos remove qualquer caractere que não seja dígito.
func SoDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NormalizaCNPJ remove máscara preservando letras (CNPJ alfanumérico - IN RFB 2.229/2024).
func NormalizaCNPJ(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// valorCNPJ converte um caractere do CNPJ para o valor usado no cálculo do DV
// (código ASCII - 48), o que mantém compatibilidade com o CNPJ numérico.
func valorCNPJ(r byte) int { return int(r) - 48 }

func dvCNPJ(base string, pesos []int) int {
	soma := 0
	for i := 0; i < len(base); i++ {
		soma += valorCNPJ(base[i]) * pesos[i]
	}
	r := soma % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

// ValidaCNPJ valida CNPJ numérico ou alfanumérico (vigente desde jul/2026).
func ValidaCNPJ(c string) bool {
	c = NormalizaCNPJ(c)
	if len(c) != 14 {
		return false
	}
	for i := 0; i < 12; i++ {
		if !unicode.IsDigit(rune(c[i])) && !(c[i] >= 'A' && c[i] <= 'Z') {
			return false
		}
	}
	if !unicode.IsDigit(rune(c[12])) || !unicode.IsDigit(rune(c[13])) {
		return false
	}
	if strings.Count(c, string(c[0])) == 14 {
		return false
	}
	p1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	p2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	d1 := dvCNPJ(c[:12], p1)
	d2 := dvCNPJ(c[:12]+strconv.Itoa(d1), p2)
	return c[12:] == fmt.Sprintf("%d%d", d1, d2)
}

// ValidaCPF valida CPF.
func ValidaCPF(c string) bool {
	c = SoDigitos(c)
	if len(c) != 11 || strings.Count(c, string(c[0])) == 11 {
		return false
	}
	calc := func(n int) int {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(c[i]-'0') * (n + 1 - i)
		}
		r := (soma * 10) % 11
		if r == 10 {
			return 0
		}
		return r
	}
	return calc(9) == int(c[9]-'0') && calc(10) == int(c[10]-'0')
}

// ValidaDocumento valida CPF ou CNPJ conforme o tamanho.
func ValidaDocumento(d string) bool {
	d = NormalizaCNPJ(d)
	switch len(d) {
	case 11:
		return ValidaCPF(d)
	case 14:
		return ValidaCNPJ(d)
	}
	return false
}

// Chave representa a chave de acesso de 44 posições (NF-e, NFC-e, CT-e, MDF-e).
type Chave struct {
	CUF     string
	AAMM    string
	CNPJ    string
	Modelo  string
	Serie   string
	Numero  string
	TpEmis  string
	Codigo  string
	DV      string
	Valida  bool
	Motivo  string
	Formato string
}

// DVChave calcula o dígito verificador (módulo 11, pesos 2..9) da chave.
// Caracteres alfanuméricos do CNPJ usam valor ASCII-48 (NT 2025.001).
func DVChave(chave43 string) int {
	soma, peso := 0, 2
	for i := len(chave43) - 1; i >= 0; i-- {
		soma += (int(chave43[i]) - 48) * peso
		peso++
		if peso > 9 {
			peso = 2
		}
	}
	r := soma % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

// AnalisaChave decompõe e valida uma chave de acesso.
func AnalisaChave(s string) Chave {
	s = NormalizaCNPJ(s)
	k := Chave{}
	if len(s) != 44 {
		k.Motivo = fmt.Sprintf("chave com %d posições (esperado 44)", len(s))
		return k
	}
	k.CUF, k.AAMM, k.CNPJ, k.Modelo = s[0:2], s[2:6], s[6:20], s[20:22]
	k.Serie, k.Numero, k.TpEmis, k.Codigo, k.DV = s[22:25], s[25:34], s[34:35], s[35:43], s[43:44]
	if _, ok := UFPorCodigo[k.CUF]; !ok && k.CUF != "91" {
		k.Motivo = "código de UF inválido: " + k.CUF
		return k
	}
	mes, _ := strconv.Atoi(k.AAMM[2:])
	if mes < 1 || mes > 12 {
		k.Motivo = "mês de emissão inválido na chave"
		return k
	}
	if !ValidaDocumento(k.CNPJ) && !ValidaCPF(k.CNPJ[3:]) {
		k.Motivo = "CNPJ/CPF do emitente inválido na chave"
		return k
	}
	if strconv.Itoa(DVChave(s[:43])) != k.DV {
		k.Motivo = fmt.Sprintf("dígito verificador inválido (esperado %d)", DVChave(s[:43]))
		return k
	}
	k.Valida = true
	return k
}

// ValidaIE valida a Inscrição Estadual. Retorna (valida, verificadaLocalmente).
// Para UFs sem algoritmo implementado retorna verificada=false, indicando que a
// validação deve ser feita pelo serviço CadConsultaCadastro4 da SEFAZ.
func ValidaIE(ie, uf string) (bool, bool) {
	up := strings.ToUpper(strings.TrimSpace(ie))
	if up == "ISENTO" || up == "ISENTA" {
		return true, true
	}
	d := SoDigitos(ie)
	if d == "" {
		return false, true
	}
	n := func(i int) int { return int(d[i] - '0') }
	mod11 := func(pesos []int, ate int) int {
		s := 0
		for i := 0; i < ate; i++ {
			s += n(i) * pesos[i]
		}
		r := s % 11
		if r < 2 {
			return 0
		}
		return 11 - r
	}
	switch strings.ToUpper(uf) {
	case "SP":
		if len(d) != 12 {
			return false, true
		}
		p1 := []int{1, 3, 4, 5, 6, 7, 8, 10}
		s := 0
		for i := 0; i < 8; i++ {
			s += n(i) * p1[i]
		}
		if (s%11)%10 != n(8) {
			return false, true
		}
		p2 := []int{3, 2, 10, 9, 8, 7, 6, 5, 4, 3, 2}
		s = 0
		for i := 0; i < 11; i++ {
			s += n(i) * p2[i]
		}
		return (s%11)%10 == n(11), true
	case "RS":
		if len(d) != 10 {
			return false, true
		}
		dv := mod11([]int{2, 9, 8, 7, 6, 5, 4, 3, 2}, 9)
		return dv == n(9), true
	case "SC":
		if len(d) != 9 {
			return false, true
		}
		return mod11([]int{9, 8, 7, 6, 5, 4, 3, 2}, 8) == n(8), true
	case "PR":
		if len(d) != 10 {
			return false, true
		}
		return mod11([]int{3, 2, 7, 6, 5, 4, 3, 2}, 8) == n(8) &&
			mod11([]int{4, 3, 2, 7, 6, 5, 4, 3, 2}, 9) == n(9), true
	case "RJ":
		if len(d) != 8 {
			return false, true
		}
		return mod11([]int{2, 7, 6, 5, 4, 3, 2}, 7) == n(7), true
	}
	return len(d) >= 8 && len(d) <= 14, false
}
