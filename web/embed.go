// Package web embute o painel (HTML/JS/CSS) no executável.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var arquivos embed.FS

// Static devolve o sistema de arquivos do painel.
func Static() fs.FS {
	f, _ := fs.Sub(arquivos, "static")
	return f
}
