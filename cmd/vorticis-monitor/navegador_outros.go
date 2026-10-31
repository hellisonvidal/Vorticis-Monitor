//go:build !windows

package main

import "os/exec"

func abrirNavegador(url string) { exec.Command("xdg-open", url).Start() }
