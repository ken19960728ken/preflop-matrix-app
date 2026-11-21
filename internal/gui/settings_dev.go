//go:build !production

package gui

import "os"

// settingsPathOverride 只在非 production 建置生效：PMA_SETTINGS_PATH 把設定檔指到別處（手動驗證用，避免動到真實設定）。
func settingsPathOverride() string { return os.Getenv("PMA_SETTINGS_PATH") }
