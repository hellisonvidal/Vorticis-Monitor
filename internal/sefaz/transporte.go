package sefaz

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Perfis de TLS tentados em sequência quando o servidor encerra ou corrompe a
// conexão segura. Alguns servidores do governo (ex.: o ADN da NFS-e Nacional,
// atrás de balanceadores/WAF) falham com TLS 1.3 e certificado de cliente,
// com o key share pós-quântico enviado por padrão desde o Go 1.24 ou com
// cifras AES-GCM, devolvendo "tls: bad record MAC" (o ADN também responde assim,
// em TLS 1.3, quando recusa o certificado do cliente). O primeiro perfil que
// funcionar fica memorizado por servidor.
type perfilTLS struct {
	nome   string
	max    uint16
	curvas []tls.CurveID
	cifras []uint16
}

var perfisTLS = []perfilTLS{
	{nome: "padrão"},
	{nome: "TLS 1.2", max: tls.VersionTLS12, curvas: []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}},
	{nome: "TLS 1.2 compatível", max: tls.VersionTLS12, curvas: []tls.CurveID{tls.CurveP256, tls.CurveP384, tls.X25519}, cifras: []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384, tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA, tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}},
}

// transportes mantém um http.Client por perfil de TLS e o perfil que funciona em cada servidor.
type transportes struct {
	cert      tls.Certificate
	pool      *x509.CertPool
	verificar bool

	mu       sync.Mutex
	clientes []*http.Client
	porHost  map[string]int
}

func novosTransportes(cert tls.Certificate, pool *x509.CertPool, verificar bool) *transportes {
	return &transportes{cert: cert, pool: pool, verificar: verificar, clientes: make([]*http.Client, len(perfisTLS)), porHost: map[string]int{}}
}

func (t *transportes) cliente(i int) *http.Client {
	t.mu.Lock()
	defer t.mu.Unlock()
	if c := t.clientes[i]; c != nil {
		return c
	}
	p := perfisTLS[i]
	cfg := &tls.Config{
		// Envia sempre o certificado da empresa. Com "Certificates" o Go só o envia
		// quando a cadeia coincide com as ACs anunciadas pelo servidor; o ADN anuncia
		// apenas as raízes ICP-Brasil e, sem certificado, recusa a conexão.
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return &t.cert, nil },
		RootCAs:              t.pool,
		InsecureSkipVerify:   !t.verificar, // somente se configurado explicitamente
		MinVersion:           tls.VersionTLS12,
		MaxVersion:           p.max,
		CurvePreferences:     p.curvas,
		CipherSuites:         p.cifras,
		Renegotiation:        tls.RenegotiateOnceAsClient,
	}
	tr := &http.Transport{
		TLSClientConfig:     cfg,
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 20 * time.Second,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     50 * time.Second,
		// Perfis alternativos não reaproveitam conexões: evita reutilizar uma
		// sessão que o servidor já descartou.
		DisableKeepAlives: i > 0,
	}
	c := &http.Client{Transport: tr, Timeout: 60 * time.Second}
	t.clientes[i] = c
	return c
}

// erroTLS indica falha na camada segura (ou conexão derrubada durante o handshake),
// caso em que vale tentar outro perfil.
func erroTLS(err error) bool {
	if err == nil {
		return false
	}
	var rec tls.RecordHeaderError
	var alerta tls.AlertError
	if errors.As(err, &rec) || errors.As(err, &alerta) {
		return true
	}
	s := err.Error()
	for _, m := range []string{"tls:", "bad record MAC", "handshake", "remote error", "connection reset", "forcibly closed", "wsarecv", "unexpected EOF", ": EOF"} {
		if strings.Contains(s, m) {
			return true
		}
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

// Do executa a requisição montada por novo(), trocando de perfil de TLS quando
// o servidor rejeita a conexão segura.
func (t *transportes) Do(host string, novo func() (*http.Request, error)) (*http.Response, error) {
	t.mu.Lock()
	ini, ok := t.porHost[host]
	t.mu.Unlock()
	var primeiro, recusa error
	for k := 0; k < len(perfisTLS); k++ {
		i := (ini + k) % len(perfisTLS)
		req, err := novo()
		if err != nil {
			return nil, err
		}
		resp, err := t.cliente(i).Do(req)
		if err == nil {
			if !ok || i != ini {
				t.mu.Lock()
				t.porHost[host] = i
				t.mu.Unlock()
			}
			return resp, nil
		}
		if primeiro == nil {
			primeiro = err
		}
		if recusaCertificado(err) {
			recusa = err
		}
		if !erroTLS(err) || req.Context().Err() != nil {
			return nil, err
		}
	}
	if recusa != nil {
		return nil, fmt.Errorf("%w — o servidor %s recusou o certificado digital no início da conexão segura. Confira se o certificado é ICP-Brasil (e-CNPJ A1) válido; o monitor envia a cadeia completa (baixando as ACs intermediárias pela internet) — se o computador bloquear esse download, instale a cadeia da AC emissora no Windows ou inclua o .pfx exportado com todos os certificados do caminho", ErrCertificadoRecusado, host)
	}
	return nil, fmt.Errorf("%w — a conexão segura falhou em todos os modos de TLS testados; verifique se um antivírus/firewall com inspeção HTTPS (ou proxy) está interceptando o acesso a %s e libere o endereço", primeiro, host)
}

// ErrCertificadoRecusado: o servidor encerrou o handshake ao avaliar o certificado do cliente.
var ErrCertificadoRecusado = errors.New("certificado digital recusado pelo servidor")

// recusaCertificado identifica a recusa do certificado do cliente pelo servidor:
// em TLS 1.2 chega como alerta "handshake failure"/"bad certificate"; em TLS 1.3
// o alerta vem cifrado com outra chave e aparece como "bad record MAC".
func recusaCertificado(err error) bool {
	s := err.Error()
	for _, m := range []string{"remote error: tls: handshake failure", "remote error: tls: bad certificate", "remote error: tls: unknown certificate authority", "remote error: tls: certificate required", "remote error: tls: unsupported certificate", "remote error: tls: access denied"} {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
