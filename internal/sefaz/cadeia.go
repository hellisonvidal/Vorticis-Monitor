package sefaz

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Cadeia do certificado enviada no TLS mútuo.
//
// O Ambiente Nacional da NFS-e (ADN) em produção só aceita certificados cuja cadeia
// chegue a uma das raízes da ICP-Brasil (v5, v10, v11…) e anuncia apenas essas
// raízes como "ACs aceitas". Se o monitor enviar só o certificado final — comum em
// .pfx exportados sem a cadeia ou em certificados instalados no Windows sem as ACs
// intermediárias —, o servidor recusa a conexão: em TLS 1.3 o alerta chega como
// "tls: bad record MAC" e em TLS 1.2 como "handshake failure". Por isso a cadeia é
// ordenada (certificado → AC final → AC intermediária) e completada pelas ACs
// emissoras publicadas no próprio certificado (Authority Information Access).

// raizICP reconhece o emissor das ACs de 1º nível (a raiz não precisa ser enviada).
func raizICP(c *x509.Certificate) bool {
	return bytes.Equal(c.RawIssuer, c.RawSubject) || strings.HasPrefix(c.Issuer.CommonName, "Autoridade Certificadora Raiz Brasileira")
}

// ordenarCadeia monta leaf → emissor → … com os certificados disponíveis, sem a raiz.
func ordenarCadeia(leaf *x509.Certificate, extras []*x509.Certificate) []*x509.Certificate {
	cadeia := []*x509.Certificate{leaf}
	usados := map[string]bool{string(leaf.Raw): true}
	for atual := leaf; len(cadeia) < 8 && !bytes.Equal(atual.RawIssuer, atual.RawSubject); {
		var prox *x509.Certificate
		for _, c := range extras {
			if !usados[string(c.Raw)] && bytes.Equal(c.RawSubject, atual.RawIssuer) && atual.CheckSignatureFrom(c) == nil {
				prox = c
				break
			}
		}
		if prox == nil || bytes.Equal(prox.RawIssuer, prox.RawSubject) { // fim ou raiz (não enviada)
			break
		}
		usados[string(prox.Raw)] = true
		cadeia = append(cadeia, prox)
		atual = prox
	}
	return cadeia
}

var (
	cacheAIA   sync.Map // URL → []*x509.Certificate
	clienteAIA = &http.Client{Timeout: 12 * time.Second}
)

// baixarEmissor obtém as ACs publicadas no endereço AIA (DER, PEM ou PKCS#7 .p7b/.p7c).
func baixarEmissor(ctx context.Context, url string) ([]*x509.Certificate, error) {
	if v, ok := cacheAIA.Load(url); ok {
		return v.([]*x509.Certificate), nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, errors.New("endereço não suportado")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := clienteAIA.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	certs := LerCertificados(b)
	if len(certs) == 0 {
		return nil, errors.New("nenhum certificado no arquivo")
	}
	cacheAIA.Store(url, certs)
	return certs, nil
}

// LerCertificados decodifica certificados em DER, PEM ou PKCS#7 (SignedData).
func LerCertificados(b []byte) []*x509.Certificate {
	var out []*x509.Certificate
	if bytes.Contains(b, []byte("-----BEGIN")) {
		for {
			var blk *pem.Block
			blk, b = pem.Decode(b)
			if blk == nil {
				break
			}
			out = append(out, LerCertificados(blk.Bytes)...)
		}
		return out
	}
	if c, err := x509.ParseCertificate(b); err == nil {
		return []*x509.Certificate{c}
	}
	if cs, err := x509.ParseCertificates(b); err == nil && len(cs) > 0 {
		return cs
	}
	return lerPKCS7(b)
}

// lerPKCS7 extrai o campo certificates de um ContentInfo/SignedData (RFC 2315).
func lerPKCS7(b []byte) []*x509.Certificate {
	var ci struct {
		Tipo     asn1.ObjectIdentifier
		Conteudo asn1.RawValue `asn1:"explicit,tag:0"`
	}
	if _, err := asn1.Unmarshal(b, &ci); err != nil {
		return nil
	}
	var sd struct {
		Versao      int
		Digests     asn1.RawValue
		Conteudo    asn1.RawValue
		Certificado asn1.RawValue `asn1:"optional,tag:0"`
	}
	if _, err := asn1.Unmarshal(ci.Conteudo.Bytes, &sd); err != nil {
		return nil
	}
	cs, _ := x509.ParseCertificates(sd.Certificado.Bytes)
	return cs
}

// completarCadeia ordena a cadeia e busca as ACs que faltam pelo AIA do certificado.
// É tolerante a falhas: sem internet ou sem AIA, segue com o que tiver.
func completarCadeia(leaf *x509.Certificate, extras []*x509.Certificate) [][]byte {
	cadeia := ordenarCadeia(leaf, extras)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	for len(cadeia) < 8 {
		ult := cadeia[len(cadeia)-1]
		if raizICP(ult) || len(ult.IssuingCertificateURL) == 0 {
			break
		}
		var achou *x509.Certificate
		for _, url := range ult.IssuingCertificateURL {
			cs, err := baixarEmissor(ctx, url)
			if err != nil {
				log.Printf("cadeia do certificado: AC emissora em %s: %v", url, err)
				continue
			}
			extras = append(extras, cs...)
			for _, c := range cs {
				if bytes.Equal(c.RawSubject, ult.RawIssuer) && ult.CheckSignatureFrom(c) == nil {
					achou = c
				}
			}
			if achou != nil {
				break
			}
		}
		if achou == nil {
			break
		}
		if n := ordenarCadeia(leaf, extras); len(n) > len(cadeia) {
			cadeia = n
			continue
		}
		break // emissor encontrado é a raiz
	}
	out := make([][]byte, len(cadeia))
	for i, c := range cadeia {
		out[i] = c.Raw
	}
	return out
}
