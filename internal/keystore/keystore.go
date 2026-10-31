// Package keystore cifra segredos (PFX e senha do certificado digital) com AES-256-GCM.
// A chave-mestra fica em <dados>/chave.mestra; no Windows ela é protegida pela
// DPAPI no escopo da máquina (somente o próprio computador consegue abri-la).
package keystore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
)

type Cofre struct{ aead cipher.AEAD }

// SemProtecaoMaquina desativa a DPAPI (modo portátil): a chave-mestra fica apenas
// no arquivo, permitindo levar a pasta para outro computador. Proteja a pasta.
var SemProtecaoMaquina bool

// prefixoPlano marca chaves-mestras gravadas sem DPAPI.
var prefixoPlano = []byte("VTXK1:")

func Abrir(dir string) (*Cofre, error) {
	p := filepath.Join(dir, "chave.mestra")
	var chave []byte
	if b, err := os.ReadFile(p); err == nil {
		if bytes.HasPrefix(b, prefixoPlano) {
			chave = b[len(prefixoPlano):]
		} else if chave, err = desproteger(b); err != nil {
			return nil, err
		}
	} else {
		chave = make([]byte, 32)
		if _, err := rand.Read(chave); err != nil {
			return nil, err
		}
		prot := append(append([]byte{}, prefixoPlano...), chave...)
		if !SemProtecaoMaquina {
			var err error
			if prot, err = proteger(chave); err != nil {
				return nil, err
			}
		}
		if err := os.WriteFile(p, prot, 0o600); err != nil {
			return nil, err
		}
	}
	if len(chave) != 32 {
		return nil, errors.New("chave-mestra inválida")
	}
	blk, err := aes.NewCipher(chave)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	return &Cofre{aead: g}, nil
}

func (c *Cofre) Cifrar(p []byte) ([]byte, error) {
	n := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(n); err != nil {
		return nil, err
	}
	return c.aead.Seal(n, n, p, nil), nil
}

func (c *Cofre) Decifrar(b []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(b) < ns {
		return nil, errors.New("dado cifrado inválido")
	}
	return c.aead.Open(nil, b[:ns], b[ns:], nil)
}
