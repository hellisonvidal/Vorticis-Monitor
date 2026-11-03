package sefaz

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Servidor que só aceita cifra com troca de chaves RSA (TLS 1.2): os perfis
// padrão falham no handshake e o perfil compatível deve ser usado e memorizado.
func TestTrocaPerfilTLS(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	srv.TLS = &tls.Config{MaxVersion: tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_128_CBC_SHA}}
	srv.StartTLS()
	defer srv.Close()
	srv.Config.ErrorLog = nil
	c := &Cliente{tr: novosTransportes(tls.Certificate{}, nil, false), Ambiente: 1}
	for n := 0; n < 2; n++ {
		resp, err := c.do(context.Background(), http.MethodGet, srv.URL+"/x", nil, nil)
		if err != nil {
			t.Fatalf("tentativa %d: %v", n, err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(b) != "ok" {
			t.Fatalf("corpo = %q", b)
		}
	}
	host := strings.TrimPrefix(srv.URL, "https://")
	if got := c.tr.porHost[host]; got != 2 {
		t.Fatalf("perfil memorizado = %d, esperado 2", got)
	}
}

func TestErroTLS(t *testing.T) {
	if !erroTLS(errString("local error: tls: bad record MAC")) {
		t.Fatal("bad record MAC deve ser tratado como falha de TLS")
	}
	if erroTLS(errString("dial tcp: lookup adn.nfse.gov.br: no such host")) {
		t.Fatal("falha de DNS não é de TLS")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
