package sefaz

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func emitir(t *testing.T, cn string, pai *x509.Certificate, chavePai *rsa.PrivateKey, ca bool, aia string) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, IsCA: ca,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	if aia != "" {
		tpl.IssuingCertificateURL = []string{aia}
	}
	if pai == nil {
		pai, chavePai = tpl, k
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, pai, &k.PublicKey, chavePai)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	return c, k
}

// p7b monta um PKCS#7 "certs-only" (como os .p7b publicados pelas ACs ICP-Brasil).
func p7b(certs ...*x509.Certificate) []byte {
	var raw []byte
	for _, c := range certs {
		raw = append(raw, c.Raw...)
	}
	sd, _ := asn1.Marshal(struct {
		Versao   int
		Digests  asn1.RawValue
		Conteudo struct{ Tipo asn1.ObjectIdentifier }
		Certs    asn1.RawValue
	}{1, asn1.RawValue{Class: 0, Tag: 17, IsCompound: true}, struct{ Tipo asn1.ObjectIdentifier }{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}},
		asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: raw}})
	ci, _ := asn1.Marshal(struct {
		Tipo     asn1.ObjectIdentifier
		Conteudo asn1.RawValue
	}{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}, asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: sd}})
	return ci
}

// Reproduz o ADN: o servidor exige certificado de cliente e anuncia só a raiz; o
// certificado da empresa vem sem a AC intermediária, que é obtida pelo AIA (.p7b).
func TestCadeiaCompletadaPeloAIAeCertificadoEnviado(t *testing.T) {
	raiz, kRaiz := emitir(t, "Autoridade Certificadora Raiz Brasileira v5", nil, nil, true, "")
	var inter *x509.Certificate
	aia := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(p7b(inter)) }))
	defer aia.Close()
	inter, kInter := emitir(t, "AC TESTE RFB v5", raiz, kRaiz, true, "")
	leaf, kLeaf := emitir(t, "EMPRESA TESTE:11222333000181", inter, kInter, false, aia.URL+"/ac.p7b")

	if got := LerCertificados(p7b(inter, raiz)); len(got) != 2 {
		t.Fatalf("PKCS#7 com %d certificados", len(got))
	}
	cadeia := completarCadeia(leaf, nil)
	if len(cadeia) != 2 {
		t.Fatalf("cadeia com %d certificados, esperado certificado + AC intermediária", len(cadeia))
	}

	pool := x509.NewCertPool()
	pool.AddCert(raiz)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.TLS.PeerCertificates[0].Subject.CommonName)
	}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}
	srv.StartTLS()
	defer srv.Close()

	c := &Cliente{tr: novosTransportes(tls.Certificate{Certificate: cadeia, PrivateKey: kLeaf}, nil, false), Ambiente: 1}
	resp, err := c.do(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("conexão recusada: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "EMPRESA TESTE:11222333000181" {
		t.Fatalf("servidor recebeu %q", b)
	}

	// Só o certificado final (sem a intermediária): o servidor recusa e a mensagem explica.
	c2 := &Cliente{tr: novosTransportes(tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: kLeaf}, nil, false), Ambiente: 1}
	srv.Config.ErrorLog = nil
	if _, err := c2.do(context.Background(), http.MethodGet, srv.URL, nil, nil); err == nil {
		t.Fatal("sem a AC intermediária o servidor deveria recusar")
	} else {
		t.Logf("erro sem a cadeia: %v", err)
	}
}
