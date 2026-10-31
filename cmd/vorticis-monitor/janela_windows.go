//go:build windows

package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"

	"vorticis.com.br/monitor/internal/config"
)

const tituloJanela = "Vorticis Monitor"

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW      = user32.NewProc("MessageBoxW")
	procFindWindowW      = user32.NewProc("FindWindowW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procSetForegroundWnd = user32.NewProc("SetForegroundWindow")
	procAttachConsole    = windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
)

func init() {
	// A janela (WebView2) precisa rodar sempre na mesma thread do sistema.
	runtime.LockOSThread()
	// O executável usa o subsistema gráfico; comandos de linha (version, reset-senha,
	// service) reaproveitam o console de quem chamou.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "--") {
		const attachParentProcess = ^uintptr(0)
		if r, _, _ := procAttachConsole.Call(attachParentProcess); r != 0 {
			if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
				os.Stdout, os.Stderr = f, f
			}
		}
	}
}

func caixaMensagem(titulo, texto string, flags uintptr) {
	t, _ := windows.UTF16PtrFromString(titulo)
	m, _ := windows.UTF16PtrFromString(texto)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
}

func erroFatal(err error) {
	caixaMensagem(tituloJanela, "Não foi possível iniciar o Vorticis Monitor:\n\n"+err.Error(), 0x10) // MB_ICONERROR
	os.Exit(1)
}

// trazerInstanciaAberta mostra a janela de uma instância já em execução.
func trazerInstanciaAberta() {
	t, _ := windows.UTF16PtrFromString(tituloJanela)
	if h, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(t))); h != 0 {
		procShowWindow.Call(h, 3) // SW_MAXIMIZE
		procSetForegroundWnd.Call(h)
	}
}

// executarComJanela inicia o monitor e abre o painel em janela própria do Windows.
// Fechar a janela encerra o programa.
func executarComJanela(ctx context.Context, cancel context.CancelFunc) error {
	// Uma instância por pasta de dados: abrir o atalho de novo apenas traz a janela.
	h := sha1.Sum([]byte(strings.ToLower(config.DiretorioDados())))
	nome, _ := windows.UTF16PtrFromString(`Local\VorticisMonitor-` + hex.EncodeToString(h[:8]))
	mtx, err := windows.CreateMutex(nil, false, nome)
	if err == windows.ERROR_ALREADY_EXISTS {
		trazerInstanciaAberta()
		return nil
	}
	if mtx != 0 {
		defer windows.CloseHandle(mtx)
	}
	if semJanela {
		return executar(ctx, nil)
	}

	type inicio struct{ url, dados string }
	pronto := make(chan inicio, 1)
	fim := make(chan error, 1)
	go func() { fim <- executar(ctx, func(u, d string) { pronto <- inicio{u, d} }) }()
	var ini inicio
	select {
	case ini = <-pronto:
	case err := <-fim:
		return err
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(ini.dados, "janela"),
		WindowOptions: webview2.WindowOptions{
			Title: tituloJanela, Width: 1366, Height: 860, Center: true, IconId: 1,
		},
	})
	if w == nil {
		cancel()
		<-fim
		return fmt.Errorf("o componente Microsoft Edge WebView2 Runtime não foi encontrado.\n\n" +
			"Ele já vem no Windows 10 e 11 atualizados. Instale-o gratuitamente em\n" +
			"https://go.microsoft.com/fwlink/p/?LinkId=2124703 e abra o programa novamente")
	}
	// Janela padrão do Windows (minimizar, maximizar/restaurar e fechar), sempre iniciando
	// maximizada; na inicialização automática fica minimizada e volta maximizada.
	hwnd := uintptr(w.Window())
	procShowWindow.Call(hwnd, 3) // SW_MAXIMIZE
	if minimizado {
		procShowWindow.Call(hwnd, 6) // SW_MINIMIZE
	}
	w.Navigate(ini.url)
	w.Run()
	w.Destroy()
	cancel()
	return <-fim
}
