package store

import (
	"path/filepath"
	"testing"
)

func TestFilialUsaCertificadoDaMatriz(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	matriz := &Empresa{CNPJ: "11222333000181", RazaoSocial: "Matriz", UF: "SP", UsarCertMatriz: true}
	filial := &Empresa{CNPJ: "11222333000262", RazaoSocial: "Filial", UF: "RJ", UsarCertMatriz: true}
	outra := &Empresa{CNPJ: "99888777000100", RazaoSocial: "Outra", UF: "SP", UsarCertMatriz: true}
	for _, e := range []*Empresa{matriz, filial, outra} {
		if err := s.SalvarEmpresa(e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CertificadoEfetivo(filial.ID); err == nil {
		t.Fatal("sem certificado na matriz a filial não tem certificado")
	}
	if err := s.SalvarCertificado(&Certificado{EmpresaID: matriz.ID, Tipo: "A1", Titular: "MATRIZ", PFX: []byte("x"), Senha: []byte("y")}); err != nil {
		t.Fatal(err)
	}
	c, err := s.CertificadoEfetivo(filial.ID)
	if err != nil || c.EmpresaID != matriz.ID || c.HerdadoDe == "" {
		t.Fatalf("filial deve usar o certificado da matriz: %+v %v", c, err)
	}
	e, _ := s.ObterEmpresa(filial.ID)
	if !e.TemCertificado || e.CertProprio || e.CertEmpresaID != matriz.ID {
		t.Fatalf("empresa filial: %+v", e)
	}
	if _, err := s.CertificadoEfetivo(outra.ID); err == nil {
		t.Fatal("empresa de outra raiz não pode usar o certificado")
	}
	filial.UsarCertMatriz = false
	filial.Ativo = true
	s.SalvarEmpresa(filial)
	if _, err := s.CertificadoEfetivo(filial.ID); err == nil {
		t.Fatal("com a opção desligada a filial não usa o certificado da matriz")
	}
}
