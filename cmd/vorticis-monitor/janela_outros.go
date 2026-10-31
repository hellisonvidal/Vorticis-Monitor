//go:build !windows

package main

import (
	"context"
	"log"
)

func erroFatal(err error) { log.Fatal(err) }

// executarComJanela: fora do Windows não há janela; apenas o servidor interno.
func executarComJanela(ctx context.Context, _ context.CancelFunc) error {
	return executar(ctx, func(url, _ string) { log.Printf("painel disponível em %s", url) })
}
