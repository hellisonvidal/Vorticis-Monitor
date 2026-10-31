package relatorios

import "testing"

func TestTabelaCode128(t *testing.T) {
	if len(code128) != 107 {
		t.Fatalf("esperava 107 símbolos, obtive %d", len(code128))
	}
	for i, p := range code128 {
		s := 0
		for _, c := range p {
			s += int(c - '0')
		}
		if (i < 106 && s != 11) || (i == 106 && s != 13) {
			t.Fatalf("símbolo %d (%s) soma %d", i, p, s)
		}
	}
}
