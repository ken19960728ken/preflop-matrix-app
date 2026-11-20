//go:build !windows && !(darwin && cgo)

package i18n

import "os"

// Detect 回傳系統介面語言（其他平台：LC_ALL、LC_MESSAGES、LANG）。
func Detect() string {
	return detectFromEnv(os.Getenv)
}
