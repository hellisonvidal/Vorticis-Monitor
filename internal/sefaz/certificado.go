// Package sefaz implementa a comunicação com os webservices da SEFAZ / Ambiente
// Nacional (SOAP 1.2 + TLS mútuo com certificado ICP-Brasil A1).
package sefaz

import (
	"bytes"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// Certificado é um e-CNPJ/e-CPF A1 carregado em memória.
type Certificado struct {
	TLS      tls.Certificate
	Leaf     *x509.Certificate
	Chave    *rsa.PrivateKey
	CNPJ     string
	Titular  string
	Emissor  string
	Serial   string
	ValidoDe time.Time
	Validade time.Time
}

// oidCNPJ é o OtherName ICP-Brasil (2.16.76.1.3.3) que contém o CNPJ.
var oidCNPJ = []byte{0x06, 0x05, 0x60, 0x4c, 0x01, 0x03, 0x03}
var reDigitos14 = regexp.MustCompile(`[0-9A-Z]{12}[0-9]{2}`)

// CarregarPFX decodifica o arquivo .pfx/.p12.
func CarregarPFX(pfx []byte, senha string) (*Certificado, error) {
	key, leaf, cas, err := pkcs12.DecodeChain(pfx, senha)
	if err != nil {
		return nil, fmt.Errorf("não foi possível abrir o certificado (senha incorreta ou arquivo inválido): %w", err)
	}
	rk, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("chave privada do certificado não é RSA")
	}
	chain := [][]byte{leaf.Raw}
	for _, c := range cas {
		chain = append(chain, c.Raw)
	}
	c := &Certificado{
		TLS:  tls.Certificate{Certificate: chain, PrivateKey: rk, Leaf: leaf},
		Leaf: leaf, Chave: rk, Titular: leaf.Subject.CommonName, Emissor: leaf.Issuer.CommonName,
		Serial: leaf.SerialNumber.Text(16), ValidoDe: leaf.NotBefore, Validade: leaf.NotAfter,
	}
	c.CNPJ = extrairCNPJ(leaf)
	return c, nil
}

func extrairCNPJ(c *x509.Certificate) string {
	for _, ext := range c.Extensions {
		if ext.Id.String() == "2.5.29.17" {
			if i := bytes.Index(ext.Value, oidCNPJ); i >= 0 {
				if m := reDigitos14.Find(ext.Value[i:]); m != nil {
					return string(m)
				}
			}
		}
	}
	// Padrão do CN ICP-Brasil: "RAZAO SOCIAL:12345678000199"
	if p := strings.LastIndex(c.Subject.CommonName, ":"); p >= 0 {
		return strings.TrimSpace(c.Subject.CommonName[p+1:])
	}
	return ""
}

// Valido indica se o certificado está dentro da validade.
func (c *Certificado) Valido() bool {
	n := time.Now()
	return n.After(c.ValidoDe) && n.Before(c.Validade)
}
