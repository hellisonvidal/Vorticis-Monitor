//go:build !windows

package keystore

func proteger(b []byte) ([]byte, error)    { return b, nil }
func desproteger(b []byte) ([]byte, error) { return b, nil }
