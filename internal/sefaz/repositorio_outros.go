//go:build !windows

package sefaz

import "errors"

var errSemRepositorio = errors.New("o repositório de certificados do Windows só está disponível no Windows")

// ListarRepositorio lista os certificados e-CNPJ instalados (somente Windows).
func ListarRepositorio() ([]CertRepositorio, error) { return nil, errSemRepositorio }

// CarregarRepositorio abre um certificado instalado pelo thumbprint (somente Windows).
func CarregarRepositorio(impressao string) (*Certificado, error) { return nil, errSemRepositorio }

// ExportarRepositorio exporta o certificado instalado para .pfx (somente Windows).
func ExportarRepositorio(impressao, senha string) ([]byte, error) { return nil, errSemRepositorio }
