//go:build windows

package keystore

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const cryptprotectLocalMachine = 0x4

func blob(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

func copia(out windows.DataBlob) []byte {
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...)
}

func proteger(b []byte) ([]byte, error) {
	var out windows.DataBlob
	nome, _ := windows.UTF16PtrFromString("Vorticis Monitor")
	if err := windows.CryptProtectData(blob(b), nome, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN|cryptprotectLocalMachine, &out); err != nil {
		return nil, err
	}
	return copia(out), nil
}

func desproteger(b []byte) ([]byte, error) {
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(blob(b), nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	return copia(out), nil
}
