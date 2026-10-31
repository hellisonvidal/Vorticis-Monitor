package sefaz

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"strings"
)

// EscaparTexto aplica o escape de texto da Canonical XML 1.0.
func EscaparTexto(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#xD;")
	return r.Replace(s)
}

// AssinarXMLDSig assina um elemento já serializado na forma canônica
// (C14N 1.0 inclusiva — atributos ordenados, namespace herdado declarado no
// elemento, sem auto-fechamento e sem espaços entre tags), retornando o
// bloco <Signature> no padrão exigido pela SEFAZ (RSA-SHA1, enveloped).
func AssinarXMLDSig(elementoCanonico, id string, cert *Certificado) (string, error) {
	dg := sha1.Sum([]byte(elementoCanonico))
	digest := base64.StdEncoding.EncodeToString(dg[:])
	signedInfo := `<SignedInfo xmlns="http://www.w3.org/2000/09/xmldsig#">` +
		`<CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"></CanonicalizationMethod>` +
		`<SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"></SignatureMethod>` +
		`<Reference URI="#` + id + `"><Transforms>` +
		`<Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"></Transform>` +
		`<Transform Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"></Transform>` +
		`</Transforms><DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"></DigestMethod>` +
		`<DigestValue>` + digest + `</DigestValue></Reference></SignedInfo>`
	h := sha1.Sum([]byte(signedInfo))
	sig, err := rsa.SignPKCS1v15(rand.Reader, cert.Chave, crypto.SHA1, h[:])
	if err != nil {
		return "", err
	}
	// Dentro de <Signature xmlns=dsig>, o SignedInfo herda o namespace; serializamos sem redeclarar.
	si := strings.Replace(signedInfo, ` xmlns="http://www.w3.org/2000/09/xmldsig#"`, "", 1)
	return `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#">` + si +
		`<SignatureValue>` + base64.StdEncoding.EncodeToString(sig) + `</SignatureValue>` +
		`<KeyInfo><X509Data><X509Certificate>` + base64.StdEncoding.EncodeToString(cert.Leaf.Raw) +
		`</X509Certificate></X509Data></KeyInfo></Signature>`, nil
}
