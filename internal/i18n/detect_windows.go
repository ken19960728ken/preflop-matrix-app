//go:build windows

package i18n

import (
	"os"
	"syscall"
	"unsafe"
)

const muiLanguageName = 0x8

var procGetUserPreferredUILanguages = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserPreferredUILanguages")

// Detect 回傳系統介面語言（Windows：使用者偏好 UI 語言清單的第一個）。
func Detect() string {
	var count, size uint32
	if r, _, _ := procGetUserPreferredUILanguages.Call(muiLanguageName, uintptr(unsafe.Pointer(&count)), 0, uintptr(unsafe.Pointer(&size))); r == 0 || size == 0 {
		return detectFromEnv(os.Getenv)
	}
	buf := make([]uint16, size)
	if r, _, _ := procGetUserPreferredUILanguages.Call(muiLanguageName, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return detectFromEnv(os.Getenv)
	}
	return Normalize(syscall.UTF16ToString(buf))
}
