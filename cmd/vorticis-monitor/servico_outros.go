//go:build !windows

package main

import "errors"

func executandoComoServico() bool { return false }
func executarServico()            {}
func controlarServico(string) error {
	return errors.New("serviço do Windows indisponível nesta plataforma; o Vorticis Monitor é distribuído para Windows")
}
