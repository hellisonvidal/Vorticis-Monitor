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

func TestExcluirEmpresa(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, apagar := range []bool{false, true} {
		e := &Empresa{CNPJ: "11222333000181", RazaoSocial: "Empresa", UF: "SP"}
		if err := s.SalvarEmpresa(e); err != nil {
			t.Fatal(err)
		}
		s.SalvarCertificado(&Certificado{EmpresaID: e.ID, Tipo: "A1", PFX: []byte("x"), Senha: []byte("y")})
		s.SalvarNSU(NSU{EmpresaID: e.ID, Servico: "NFSE_ADN", UltNSU: "5"})
		chave := "3526101122233300018155001000000101110000101" + map[bool]string{false: "4", true: "5"}[apagar]
		if err := s.SalvarDocumento(&Documento{EmpresaID: &e.ID, Chave: chave, Modelo: "NFE", Direcao: "S", Status: "AUTORIZADA"}, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		if n := s.ContarDocumentosEmpresa(e.ID); n != 1 {
			t.Fatalf("documentos = %d", n)
		}
		if err := s.ExcluirEmpresa(e.ID, apagar); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ObterEmpresa(e.ID); err == nil {
			t.Fatal("empresa não foi excluída")
		}
		d, err := s.ObterDocumento(chave)
		if apagar && err == nil {
			t.Fatal("documento deveria ter sido excluído")
		}
		if !apagar && (err != nil || d.EmpresaID != nil) {
			t.Fatalf("documento deveria ficar sem empresa: %+v %v", d, err)
		}
	}
}

func TestUltimaEmpresaDoUsuario(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	e := &Empresa{CNPJ: "11222333000181", RazaoSocial: "Empresa", UF: "SP"}
	s.SalvarEmpresa(e)
	s.CriarUsuario(&Usuario{Login: "Maria", SenhaHash: "x", Perfil: "FISCAL"})
	if err := s.SalvarUltimaEmpresa("maria", e.ID); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.UsuarioPorLogin("maria"); u.UltimaEmpresaID != e.ID {
		t.Fatalf("última empresa = %d", u.UltimaEmpresaID)
	}
	s.ExcluirEmpresa(e.ID, false)
	if u, _ := s.UsuarioPorLogin("maria"); u.UltimaEmpresaID != 0 {
		t.Fatal("empresa excluída deve voltar para todas as empresas")
	}
}
