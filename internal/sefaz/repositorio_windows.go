//go:build windows

package sefaz

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Acesso ao repositório "Pessoal" (CurrentUser\My) do Windows. Certificados A1
// instalados com chave exportável são copiados para o cofre do monitor como
// .pfx; os demais continuam no Windows e as assinaturas (TLS e XML) são feitas
// pela própria API de criptografia do Windows (CNG), sem expor a chave.

var (
	modCrypt32         = windows.NewLazySystemDLL("crypt32.dll")
	procPFXExport      = modCrypt32.NewProc("PFXExportCertStoreEx")
	modNCrypt          = windows.NewLazySystemDLL("ncrypt.dll")
	procNCryptSignHash = modNCrypt.NewProc("NCryptSignHash")
)

const (
	exportPrivateKeys       = 0x0004
	reportNoPrivateKey      = 0x0001
	reportNotAbleToExport   = 0x0002
	bcryptPadPKCS1          = 0x00000002
	bcryptPadPSS            = 0x00000008
	ntePermDenied           = 0x80090010
	nteNotSupported         = 0x80090029
	scardWUserCancelled     = 0x8010006E
	errCanceladoPeloUsuario = 0x800704C7
)

type certEncontrado struct {
	ctx  *windows.CertContext
	leaf *x509.Certificate
}

// percorrer visita os certificados do repositório Pessoal; parar=true encerra
// mantendo o contexto atual (o chamador libera com CertFreeCertificateContext).
func percorrer(f func(ctx *windows.CertContext, leaf *x509.Certificate) (parar bool)) (*certEncontrado, error) {
	st, err := windows.CertOpenSystemStore(0, windows.StringToUTF16Ptr("MY"))
	if err != nil {
		return nil, fmt.Errorf("não foi possível abrir os certificados do Windows: %w", err)
	}
	defer windows.CertCloseStore(st, 0)
	var ctx *windows.CertContext
	for {
		ctx, err = windows.CertEnumCertificatesInStore(st, ctx)
		if ctx == nil {
			return nil, nil
		}
		der := make([]byte, ctx.Length)
		copy(der, unsafe.Slice(ctx.EncodedCert, ctx.Length))
		leaf, err := x509.ParseCertificate(der)
		if err != nil {
			continue
		}
		if f(ctx, leaf) {
			return &certEncontrado{ctx: windows.CertDuplicateCertificateContext(ctx), leaf: leaf}, nil
		}
	}
}

// ListarRepositorio lista os e-CNPJ instalados no Windows do usuário.
func ListarRepositorio() ([]CertRepositorio, error) {
	out := []CertRepositorio{}
	agora := time.Now()
	_, err := percorrer(func(_ *windows.CertContext, leaf *x509.Certificate) bool {
		cnpj := extrairCNPJ(leaf)
		if len(cnpj) != 14 || leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
			return false
		}
		out = append(out, CertRepositorio{Impressao: Impressao(leaf.Raw), Titular: leaf.Subject.CommonName, CNPJ: cnpj,
			Emissor: leaf.Issuer.CommonName, ValidoDe: leaf.NotBefore, Validade: leaf.NotAfter,
			Valido: agora.After(leaf.NotBefore) && agora.Before(leaf.NotAfter)})
		return false
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Validade.After(out[j].Validade) })
	return out, err
}

func achar(impressao string) (*certEncontrado, error) {
	impressao = strings.ToUpper(strings.ReplaceAll(impressao, " ", ""))
	c, err := percorrer(func(_ *windows.CertContext, leaf *x509.Certificate) bool { return Impressao(leaf.Raw) == impressao })
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, errors.New("certificado não encontrado no Windows deste usuário (foi removido ou instalado em outro usuário)")
	}
	return c, nil
}

// ExportarRepositorio exporta certificado + chave para .pfx protegido por senha.
// Falha quando a chave foi instalada como "não exportável".
func ExportarRepositorio(impressao, senha string) ([]byte, error) {
	c, err := achar(impressao)
	if err != nil {
		return nil, err
	}
	defer windows.CertFreeCertificateContext(c.ctx)
	mem, err := windows.CertOpenStore(windows.CERT_STORE_PROV_MEMORY, 0, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CertCloseStore(mem, 0)
	if err := windows.CertAddCertificateContextToStore(mem, c.ctx, windows.CERT_STORE_ADD_ALWAYS, nil); err != nil {
		return nil, err
	}
	pw, _ := windows.UTF16PtrFromString(senha)
	var blob windows.CryptDataBlob
	flags := uintptr(exportPrivateKeys | reportNoPrivateKey | reportNotAbleToExport)
	if r, _, e := procPFXExport.Call(uintptr(mem), uintptr(unsafe.Pointer(&blob)), uintptr(unsafe.Pointer(pw)), 0, flags); r == 0 {
		return nil, fmt.Errorf("chave privada não exportável: %v", e)
	}
	buf := make([]byte, blob.Size)
	blob.Data = &buf[0]
	if r, _, e := procPFXExport.Call(uintptr(mem), uintptr(unsafe.Pointer(&blob)), uintptr(unsafe.Pointer(pw)), 0, flags); r == 0 {
		return nil, fmt.Errorf("chave privada não exportável: %v", e)
	}
	return buf[:blob.Size], nil
}

// CarregarRepositorio usa o certificado instalado sem exportar a chave.
func CarregarRepositorio(impressao string) (*Certificado, error) {
	c, err := achar(impressao)
	if err != nil {
		return nil, err
	}
	defer windows.CertFreeCertificateContext(c.ctx)
	var h windows.Handle
	var spec uint32
	var liberar bool
	err = windows.CryptAcquireCertificatePrivateKey(c.ctx, windows.CRYPT_ACQUIRE_ONLY_NCRYPT_KEY_FLAG, nil, &h, &spec, &liberar)
	if err != nil {
		return nil, fmt.Errorf("não foi possível acessar a chave privada do certificado instalado (%v). Exporte o certificado para .pfx e inclua pelo arquivo", err)
	}
	pub, ok := c.leaf.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("chave do certificado não é RSA")
	}
	// ACs da cadeia montada pelo Windows (quando ele a conhece) + as baixadas pelo AIA.
	var extras []*x509.Certificate
	if cadeias, err := c.leaf.Verify(x509.VerifyOptions{KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err == nil && len(cadeias) > 0 {
		extras = cadeias[0][1:]
	}
	return montarCertificado(c.leaf, completarCadeia(c.leaf, extras), &assinadorCNG{h: h, pub: pub}, "REPOSITORIO"), nil
}

// assinadorCNG implementa crypto.Signer com NCryptSignHash.
type assinadorCNG struct {
	h   windows.Handle
	pub *rsa.PublicKey
}

type bcryptPKCS1Padding struct{ alg *uint16 }
type bcryptPSSPadding struct {
	alg  *uint16
	salt uint32
}

func (a *assinadorCNG) Public() crypto.PublicKey { return a.pub }

func (a *assinadorCNG) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	nomes := map[crypto.Hash]string{crypto.SHA1: "SHA1", crypto.SHA256: "SHA256", crypto.SHA384: "SHA384", crypto.SHA512: "SHA512"}
	nome, ok := nomes[opts.HashFunc()]
	if !ok {
		return nil, fmt.Errorf("algoritmo de hash %v não suportado", opts.HashFunc())
	}
	alg, _ := windows.UTF16PtrFromString(nome)
	var pad unsafe.Pointer
	var flags uintptr
	pkcs := bcryptPKCS1Padding{alg: alg}
	pss := bcryptPSSPadding{alg: alg}
	if o, ok := opts.(*rsa.PSSOptions); ok {
		pss.salt = uint32(opts.HashFunc().Size())
		if o.SaltLength > 0 {
			pss.salt = uint32(o.SaltLength)
		}
		pad, flags = unsafe.Pointer(&pss), bcryptPadPSS
	} else {
		pad, flags = unsafe.Pointer(&pkcs), bcryptPadPKCS1
	}
	var n uint32
	sig := make([]byte, a.pub.Size())
	r, _, _ := procNCryptSignHash.Call(uintptr(a.h), uintptr(pad), uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		uintptr(unsafe.Pointer(&sig[0])), uintptr(len(sig)), uintptr(unsafe.Pointer(&n)), flags)
	runtime.KeepAlive(alg)
	runtime.KeepAlive(&pkcs)
	runtime.KeepAlive(&pss)
	if r != 0 {
		switch uint32(r) {
		case scardWUserCancelled, errCanceladoPeloUsuario:
			return nil, errors.New("assinatura cancelada pelo usuário")
		case ntePermDenied:
			return nil, errors.New("o Windows negou o uso da chave privada do certificado")
		case nteNotSupported:
			return nil, errors.New("o provedor da chave não suporta esta assinatura — inclua o certificado pelo arquivo .pfx")
		}
		return nil, fmt.Errorf("falha ao assinar com o certificado do Windows (0x%08X)", uint32(r))
	}
	return sig[:n], nil
}
