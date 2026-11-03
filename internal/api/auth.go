package api

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const iteracoes = 210000

// HashSenha gera "pbkdf2$iter$salt$hash".
func HashSenha(senha string) (string, error) {
	salt := make([]byte, 16)
	rand.Read(salt)
	h, err := pbkdf2.Key(sha256.New, senha, salt, iteracoes, 32)
	if err != nil {
		return "", err
	}
	return "pbkdf2$210000$" + hex.EncodeToString(salt) + "$" + hex.EncodeToString(h), nil
}

func conferirSenha(senha, armazenado string) bool {
	p := strings.Split(armazenado, "$")
	if len(p) != 4 {
		return false
	}
	salt, _ := hex.DecodeString(p[2])
	esperado, _ := hex.DecodeString(p[3])
	h, err := pbkdf2.Key(sha256.New, senha, salt, iteracoes, 32)
	return err == nil && subtle.ConstantTimeCompare(h, esperado) == 1
}

type sessao struct {
	Login  string `json:"u"`
	Perfil string `json:"p"`
	Exp    int64  `json:"e"`
	// Credencial salva ("lembrar senha"): Tipo "c" e Chk = resumo do hash da senha,
	// para que trocar a senha invalide as credenciais salvas.
	Tipo string `json:"t,omitempty"`
	Chk  string `json:"k,omitempty"`
}

func (s *Server) emitirToken(login, perfil string) string {
	return s.assinar(sessao{Login: login, Perfil: perfil, Exp: time.Now().Add(12 * time.Hour).Unix()})
}

func resumoSenha(hash string) string {
	h := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(h[:8])
}

// emitirCredencial gera a credencial de longa duração (180 dias) guardada no computador.
func (s *Server) emitirCredencial(login, senhaHash string) string {
	return s.assinar(sessao{Login: login, Exp: time.Now().Add(180 * 24 * time.Hour).Unix(), Tipo: "c", Chk: resumoSenha(senhaHash)})
}

func (s *Server) assinar(ss sessao) string {
	b, _ := json.Marshal(ss)
	p := base64.RawURLEncoding.EncodeToString(b)
	m := hmac.New(sha256.New, []byte(s.M.Cfg.SegredoSessao))
	m.Write([]byte(p))
	return p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (s *Server) lerAssinado(t string) (*sessao, error) {
	p := strings.SplitN(t, ".", 2)
	if len(p) != 2 {
		return nil, errors.New("token inválido")
	}
	m := hmac.New(sha256.New, []byte(s.M.Cfg.SegredoSessao))
	m.Write([]byte(p[0]))
	sig, _ := base64.RawURLEncoding.DecodeString(p[1])
	if !hmac.Equal(sig, m.Sum(nil)) {
		return nil, errors.New("assinatura do token inválida")
	}
	b, _ := base64.RawURLEncoding.DecodeString(p[0])
	var ss sessao
	if json.Unmarshal(b, &ss) != nil || time.Now().Unix() > ss.Exp {
		return nil, errors.New("sessão expirada")
	}
	return &ss, nil
}

func (s *Server) validarToken(t string) (*sessao, error) {
	ss, err := s.lerAssinado(t)
	if err == nil && ss.Tipo != "" {
		return nil, errors.New("token inválido")
	}
	return ss, err
}

type ctxKey struct{}

// autenticar aceita Bearer (painel), X-API-Key (ERP/escrituração) ou ?token= (SSE/downloads).
func (s *Server) autenticar(perfis ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var ss *sessao
			if k := r.Header.Get("X-API-Key"); k != "" {
				if subtle.ConstantTimeCompare([]byte(k), []byte(s.M.Cfg.APIKey)) == 1 {
					ss = &sessao{Login: "api", Perfil: "INTEGRACAO"}
				}
			} else {
				tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				if tok == "" {
					tok = r.URL.Query().Get("token")
				}
				ss, _ = s.validarToken(tok)
			}
			if ss == nil {
				erro(w, http.StatusUnauthorized, "não autenticado")
				return
			}
			if len(perfis) > 0 && ss.Perfil != "ADMIN" {
				ok := false
				for _, p := range perfis {
					ok = ok || p == ss.Perfil
				}
				if !ok {
					erro(w, http.StatusForbidden, "perfil sem permissão para esta operação")
					return
				}
			}
			if ss.Perfil == "CONSULTA" && r.Method != http.MethodGet && r.URL.Path != "/api/v1/auth/senha" && r.URL.Path != "/api/v1/auth/eu/empresa" {
				erro(w, http.StatusForbidden, "perfil de consulta não pode alterar dados")
				return
			}
			r.Header.Set("X-Vorticis-Usuario", ss.Login)
			next(w, r)
		}
	}
}

func usuario(r *http.Request) string { return r.Header.Get("X-Vorticis-Usuario") }
