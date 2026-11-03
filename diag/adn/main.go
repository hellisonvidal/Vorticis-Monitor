// Diagnóstico temporário da conexão TLS com o ADN da NFS-e Nacional.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"
)

func certTeste() tls.Certificate {
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	t := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TESTE:11222333000181"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, _ := x509.CreateCertificate(rand.Reader, t, t, &k.PublicKey, k)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: k}
}

func main() {
	cert := certTeste()
	type perfil struct {
		nome     string
		min, max uint16
		cifras   []uint16
	}
	perfis := []perfil{
		{"padrao", tls.VersionTLS12, 0, nil},
		{"tls13", tls.VersionTLS13, 0, nil},
		{"tls12", tls.VersionTLS12, tls.VersionTLS12, nil},
		{"tls12-cbc-rsa", tls.VersionTLS12, tls.VersionTLS12, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, tls.TLS_RSA_WITH_AES_256_GCM_SHA384, tls.TLS_RSA_WITH_AES_128_CBC_SHA}},
	}
	for _, host := range []string{"adn.nfse.gov.br", "adn.producaorestrita.nfse.gov.br", "sefin.nfse.gov.br"} {
		for _, p := range perfis {
			for _, comCert := range []bool{false, true} {
				pedidos := 0
				cfg := &tls.Config{MinVersion: p.min, MaxVersion: p.max, CipherSuites: p.cifras, Renegotiation: tls.RenegotiateFreelyAsClient,
					GetClientCertificate: func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
						pedidos++
						fmt.Printf("    servidor pediu certificado (#%d): %d ACs aceitas, versão %x, esquemas %v\n", pedidos, len(cri.AcceptableCAs), cri.Version, cri.SignatureSchemes)
						if pedidos == 1 {
							for i, ca := range cri.AcceptableCAs {
								if i < 3 {
									var n pkix.RDNSequence
									if _, err := asn1Unmarshal(ca, &n); err == nil {
										fmt.Printf("      AC: %s\n", n.String())
									}
								}
							}
						}
						if comCert {
							return &cert, nil
						}
						return &tls.Certificate{}, nil
					},
					VerifyConnection: func(cs tls.ConnectionState) error {
						fmt.Printf("    handshake: versão %x cifra %s EMS? alpn=%q\n", cs.Version, tls.CipherSuiteName(cs.CipherSuite), cs.NegotiatedProtocol)
						return nil
					}}
				cli := &http.Client{Timeout: 25 * time.Second, Transport: &http.Transport{TLSClientConfig: cfg, DisableKeepAlives: true}}
				fmt.Printf("== %s perfil=%s cert=%v\n", host, p.nome, comCert)
				resp, err := cli.Get("https://" + host + "/contribuintes/DFe/0?lote=true&cnpjConsulta=11222333000181")
				if err != nil {
					fmt.Printf("    ERRO: %v\n", err)
					continue
				}
				b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
				resp.Body.Close()
				fmt.Printf("    HTTP %d %q\n", resp.StatusCode, b)
			}
		}
	}
}
