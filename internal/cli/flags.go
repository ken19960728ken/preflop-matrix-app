package cli

import "strings"

// knownFlags 是命令列模式認得的旗標；GUI App 只在第一個參數屬於此集合時才走 CLI，
// 其餘參數（-AppleLanguages、-NS*、-psn_*、檔案關聯路徑…）一律開 GUI。
var knownFlags = map[string]bool{
	"--hands":        true,
	"--ranges":       true,
	"-o":             true,
	"--out":          true,
	"--hero":         true,
	"--title":        true,
	"--json":         true,
	"--lang":         true,
	"--top":          true,
	"--check-ranges": true,
	"--init-ranges":  true,
	"-h":             true,
	"--help":         true,
}

// IsCLIArgs 回報 args 是否為命令列呼叫：第一個參數是已知旗標（含 --x=value 形式）。
func IsCLIArgs(args []string) bool {
	if len(args) == 0 {
		return false
	}
	name, _, _ := strings.Cut(args[0], "=")
	return knownFlags[name]
}
