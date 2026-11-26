//go:build windows

package credentials

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// keyEntropy is a fixed, public distinguishing constant for the DPAPI blob; the protection itself is the
// current user account, so any program running as that user can unprotect the key.
var keyEntropy = []byte("router-tools vault key v1")

// protectKey wraps the raw key with DPAPI for the current user, without any UI.
func protectKey(raw []byte) ([]byte, error) {
	return dpapi(raw, true)
}

func unprotectKey(blob []byte) ([]byte, error) {
	return dpapi(blob, false)
}

func dpapi(data []byte, protect bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, windows.ERROR_INVALID_DATA
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	entropy := windows.DataBlob{Size: uint32(len(keyEntropy)), Data: &keyEntropy[0]}
	var out windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&in, nil, &entropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, &entropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) }()
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
