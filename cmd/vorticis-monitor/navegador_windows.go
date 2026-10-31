//go:build windows

package main

import "os/exec"

func abrirNavegador(url string) { exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() }
