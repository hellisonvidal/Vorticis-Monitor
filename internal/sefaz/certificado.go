// Package sefaz implementa a comunicação com os webservices da SEFAZ / Ambiente
// Nacional (SOAP 1.2 + TLS mútuo com certificado ICP-Brasil A1).
package sefaz

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// Certificado é um e-CNPJ/e-CPF A1 carregado em memória.
type Certificado struct {
	TLS  tls.Certificate
	Leaf *x509.Certificate
	// Assinador é a chave privada: *rsa.PrivateKey (arquivo .pfx) ou a chave
	// guardada no repositório do Windows (assinatura feita pelo próprio Windows).
	Assinador crypto.Signer
	// Origem: "ARQUIVO" ou "REPOSITORIO".
	Origem string
	// Impressao é a impressão digital SHA-1 (thumbprint) do certificado.
	Impressao string
	CNPJ      string
	Titular   string
	Emissor   string
	Serial    string
	ValidoDe  time.Time
	Validade  time.Time
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
	return montarCertificado(leaf, completarCadeia(leaf, cas), rk, "ARQUIVO"), nil
}

func montarCertificado(leaf *x509.Certificate, chain [][]byte, chave crypto.Signer, origem string) *Certificado {
	return &Certificado{
		TLS:  tls.Certificate{Certificate: chain, PrivateKey: chave, Leaf: leaf},
		Leaf: leaf, Assinador: chave, Origem: origem, Impressao: Impressao(leaf.Raw),
		Titular: leaf.Subject.CommonName, Emissor: leaf.Issuer.CommonName, CNPJ: extrairCNPJ(leaf),
		Serial: leaf.SerialNumber.Text(16), ValidoDe: leaf.NotBefore, Validade: leaf.NotAfter,
	}
}

// Impressao devolve o thumbprint SHA-1 (hexadecimal maiúsculo) do certificado DER.
func Impressao(der []byte) string {
	h := sha1.Sum(der)
	return strings.ToUpper(hex.EncodeToString(h[:]))
}

// CertRepositorio descreve um certificado instalado no Windows (Pessoal do usuário).
type CertRepositorio struct {
	Impressao string    `json:"impressao"`
	Titular   string    `json:"titular"`
	CNPJ      string    `json:"cnpj"`
	Emissor   string    `json:"emissor"`
	ValidoDe  time.Time `json:"valido_de"`
	Validade  time.Time `json:"valido_ate"`
	Valido    bool      `json:"valido"`
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
