//go:build windows

package main

import (
	"log"
	"runtime"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Segundo plano na bandeja do Windows (área de notificação, perto do relógio).
// Com a opção ligada (Configurações › Sistema), fechar a janela apenas a esconde:
// o monitoramento automático continua e o ícone da bandeja reabre a janela ou
// encerra o programa. As notas novas apenas entram no sistema, sem avisos.

var (
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	procShellNotifyIcon    = shell32.NewProc("Shell_NotifyIconW")
	procRegisterClassEx    = user32.NewProc("RegisterClassExW")
	procCreateWindowEx     = user32.NewProc("CreateWindowExW")
	procDefWindowProc      = user32.NewProc("DefWindowProcW")
	procCallWindowProc     = user32.NewProc("CallWindowProcW")
	procSetWindowLongPtr   = user32.NewProc("SetWindowLongPtrW")
	procGetMessage         = user32.NewProc("GetMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessage    = user32.NewProc("DispatchMessageW")
	procPostMessage        = user32.NewProc("PostMessageW")
	procLoadImage          = user32.NewProc("LoadImageW")
	procLoadIcon           = user32.NewProc("LoadIconW")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	procCreatePopupMenu    = user32.NewProc("CreatePopupMenu")
	procAppendMenu         = user32.NewProc("AppendMenuW")
	procSetMenuDefaultItem = user32.NewProc("SetMenuDefaultItem")
	procTrackPopupMenu     = user32.NewProc("TrackPopupMenu")
	procDestroyMenu        = user32.NewProc("DestroyMenu")
	procGetCursorPos       = user32.NewProc("GetCursorPos")
	procIsWindowVisible    = user32.NewProc("IsWindowVisible")
	procIsIconic           = user32.NewProc("IsIconic")
	procRegisterWindowMsg  = user32.NewProc("RegisterWindowMessageW")
	procGetModuleHandle    = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")
	procPostQuitMessage    = user32.NewProc("PostQuitMessage")
)

const (
	wmClose        = 0x0010
	wmDestroy      = 0x0002
	wmNull         = 0x0000
	wmCommand      = 0x0111
	wmLButtonUp    = 0x0202
	wmLButtonDbl   = 0x0203
	wmRButtonUp    = 0x0205
	wmBandeja      = 0x8000 + 1 // WM_APP + 1
	ninBalloonClik = 0x0405

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2
	nifMsg    = 0x01
	nifIcon   = 0x02
	nifTip    = 0x04
	nifInfo   = 0x10
	niifInfo  = 0x01

	swHide    = 0
	swShow    = 5
	swRestore = 9

	cmdAbrir = 1
	cmdSair  = 2
)

type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type msgW struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
	Private uint32
}

// bandeja controla o ícone e a janela principal.
type bandeja struct {
	principal    uintptr // janela do painel (WebView2)
	hwnd         uintptr // janela oculta que recebe os eventos do ícone
	icone        uintptr
	procOriginal uintptr
	saindo       atomic.Bool
	avisouOculta atomic.Bool
	taskbarMsg   uint32
	segundoPlano func() bool
}

var bandejaAtiva *bandeja

func utf16Fixo(dst []uint16, s string) {
	u, _ := windows.UTF16FromString(s)
	if len(u) > len(dst) {
		u = append(u[:len(dst)-1], 0)
	}
	copy(dst, u)
}

func (b *bandeja) dados(flags uint32) *notifyIconData {
	n := &notifyIconData{HWnd: b.hwnd, UID: 1, UFlags: flags, UCallbackMessage: wmBandeja, HIcon: b.icone}
	n.CbSize = uint32(unsafe.Sizeof(*n))
	utf16Fixo(n.SzTip[:], "Vorticis Monitor — monitoramento ativo")
	return n
}

func (b *bandeja) adicionar() {
	procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(b.dados(nifMsg|nifIcon|nifTip))))
}

// Avisar mostra uma notificação do Windows a partir do ícone da bandeja.
func (b *bandeja) Avisar(titulo, texto string) {
	n := b.dados(nifInfo)
	n.DwInfoFlags = niifInfo
	utf16Fixo(n.SzInfoTitle[:], titulo)
	utf16Fixo(n.SzInfo[:], texto)
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(n)))
}

func (b *bandeja) visivel() bool {
	r, _, _ := procIsWindowVisible.Call(b.principal)
	return r != 0
}

// Mostrar traz a janela do painel de volta.
func (b *bandeja) Mostrar() {
	if r, _, _ := procIsIconic.Call(b.principal); r != 0 {
		procShowWindow.Call(b.principal, swRestore)
	} else {
		procShowWindow.Call(b.principal, swShow)
	}
	procSetForegroundWnd.Call(b.principal)
}

// Esconder deixa o programa só na bandeja.
func (b *bandeja) Esconder() {
	procShowWindow.Call(b.principal, swHide)
	if !b.avisouOculta.Swap(true) {
		b.Avisar("Vorticis Monitor continua em segundo plano", "O monitoramento das notas segue ativo. Clique no ícone perto do relógio para abrir ou encerrar o programa.")
	}
}

// Sair encerra o programa (fecha a janela de verdade).
func (b *bandeja) Sair() {
	b.saindo.Store(true)
	procPostMessage.Call(b.principal, wmClose, 0, 0)
}

func (b *bandeja) menu() {
	m, _, _ := procCreatePopupMenu.Call()
	if m == 0 {
		return
	}
	defer procDestroyMenu.Call(m)
	abrir, _ := windows.UTF16PtrFromString("Abrir o Vorticis Monitor")
	sair, _ := windows.UTF16PtrFromString("Sair (encerrar o monitoramento)")
	procAppendMenu.Call(m, 0, cmdAbrir, uintptr(unsafe.Pointer(abrir)))
	procAppendMenu.Call(m, 0x800, 0, 0) // MF_SEPARATOR
	procAppendMenu.Call(m, 0, cmdSair, uintptr(unsafe.Pointer(sair)))
	procSetMenuDefaultItem.Call(m, cmdAbrir, 0)
	var pt struct{ X, Y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWnd.Call(b.hwnd)                                                                  // necessário para o menu fechar ao clicar fora
	cmd, _, _ := procTrackPopupMenu.Call(m, 0x0100|0x0002, uintptr(pt.X), uintptr(pt.Y), 0, b.hwnd, 0) // TPM_RETURNCMD | TPM_RIGHTBUTTON
	procPostMessage.Call(b.hwnd, wmNull, 0, 0)
	switch cmd {
	case cmdAbrir:
		b.Mostrar()
	case cmdSair:
		b.Sair()
	}
}

// procBandeja trata os cliques no ícone.
func (b *bandeja) procBandeja(hwnd, msg, wp, lp uintptr) uintptr {
	switch {
	case msg == wmBandeja:
		switch lp & 0xFFFF {
		case wmLButtonUp, wmLButtonDbl, ninBalloonClik:
			b.Mostrar()
		case wmRButtonUp:
			b.menu()
		}
		return 0
	case b.taskbarMsg != 0 && uint32(msg) == b.taskbarMsg: // Explorer reiniciado: recoloca o ícone
		b.adicionar()
		return 0
	case msg == wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, msg, wp, lp)
	return r
}

// procPrincipal intercepta o "fechar" da janela do painel.
func (b *bandeja) procPrincipal(hwnd, msg, wp, lp uintptr) uintptr {
	if msg == wmClose && !b.saindo.Load() && b.segundoPlano() {
		b.Esconder()
		return 0
	}
	r, _, _ := procCallWindowProc.Call(b.procOriginal, hwnd, msg, wp, lp)
	return r
}

// iniciarBandeja cria o ícone (em thread própria) e passa a interceptar o fechar da janela.
// Deve ser chamada na thread da janela principal.
func iniciarBandeja(principal uintptr, segundoPlano func() bool) *bandeja {
	b := &bandeja{principal: principal, segundoPlano: segundoPlano}
	pronto := make(chan bool)
	go func() {
		runtime.LockOSThread()
		inst, _, _ := procGetModuleHandle.Call(0)
		cx, _, _ := procGetSystemMetrics.Call(49) // SM_CXSMICON
		cy, _, _ := procGetSystemMetrics.Call(50)
		b.icone, _, _ = procLoadImage.Call(inst, 1, 1, cx, cy, 0) // ícone #1 do executável
		if b.icone == 0 {
			b.icone, _, _ = procLoadIcon.Call(0, 32512) // IDI_APPLICATION
		}
		classe, _ := windows.UTF16PtrFromString("VorticisMonitorBandeja")
		wc := wndClassEx{WndProc: windows.NewCallback(b.procBandeja), Instance: inst, ClassName: classe}
		wc.Size = uint32(unsafe.Sizeof(wc))
		procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
		titulo, _ := windows.UTF16PtrFromString("Vorticis Monitor (bandeja)")
		b.hwnd, _, _ = procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(classe)), uintptr(unsafe.Pointer(titulo)), 0, 0, 0, 0, 0, 0, 0, inst, 0)
		if b.hwnd == 0 {
			log.Printf("bandeja: não foi possível criar a janela do ícone")
			pronto <- false
			return
		}
		tb, _ := windows.UTF16PtrFromString("TaskbarCreated")
		m, _, _ := procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(tb)))
		b.taskbarMsg = uint32(m)
		b.adicionar()
		pronto <- true
		var msg msgW
		for {
			r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	if !<-pronto {
		return nil
	}
	const gwlpWndProc = ^uintptr(3) // -4
	b.procOriginal, _, _ = procSetWindowLongPtr.Call(principal, gwlpWndProc, windows.NewCallback(b.procPrincipal))
	bandejaAtiva = b
	return b
}

// Encerrar remove o ícone da bandeja.
func (b *bandeja) Encerrar() {
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(b.dados(0))))
	procPostMessage.Call(b.hwnd, wmClose, 0, 0)
}
