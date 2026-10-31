package sefaz

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// CertificadoTeste gera um PFX autoassinado (somente para testes).
func CertificadoTeste(t testing.TB) ([]byte, string) {
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "VORTICIS DEMO COMERCIO LTDA:11222333000181"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	pfx, err := pkcs12.Modern.Encode(k, c, nil, "1234")
	if err != nil {
		t.Fatal(err)
	}
	return pfx, "1234"
}

func TestEventoAssinado(t *testing.T) {
	pfx, senha := CertificadoTeste(t)
	cert, err := CarregarPFX(pfx, senha)
	if err != nil {
		t.Fatal(err)
	}
	if cert.CNPJ != "11222333000181" {
		t.Fatalf("CNPJ extraído: %q", cert.CNPJ)
	}
	env, err := MontarEventoManifestacao(cert, 2, "11222333000181", "35261011222333000181550010000001011100001014", "210210", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p := os.Getenv("VORTICIS_EVENTO_OUT"); p != "" {
		os.WriteFile(p, []byte(env), 0o644)
		os.WriteFile(p+".pem", cert.Leaf.Raw, 0o644)
	}
}
