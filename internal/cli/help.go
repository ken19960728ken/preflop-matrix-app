package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
)

// helpEntries 依顯示順序列出說明的旗標寫法與對應的 i18n key。
var helpEntries = []struct{ usage, key string }{
	{"--hands DIR", "cli.help.hands"},
	{"--ranges DIR", "cli.help.ranges"},
	{"-o, --out FILE", "cli.help.out"},
	{"--hero NAME", "cli.help.hero"},
	{"--title TEXT", "cli.help.title"},
	{"--json PATH", "cli.help.json"},
	{"--lang LANG", "cli.help.lang"},
	{"--top N", "cli.help.top"},
	{"--check-ranges", "cli.help.check_ranges"},
	{"--init-ranges DIR", "cli.help.init_ranges"},
	{"-h, --help", "cli.help.help"},
}

const helpIndent = 22

// printHelp 把用法與各旗標說明印到 stdout。
func (t term) printHelp() {
	defaultDir, err := config.DefaultRangesDir()
	if err != nil {
		defaultDir = "~/.config/preflop-matrix/ranges"
	}
	fmt.Fprintln(t.stdout, t.t("cli.help.usage"))
	fmt.Fprintln(t.stdout)
	fmt.Fprintln(t.stdout, t.t("cli.description"))
	fmt.Fprintln(t.stdout)
	fmt.Fprintln(t.stdout, t.t("cli.help.options"))
	for _, e := range helpEntries {
		text := t.t(e.key, "env", config.EnvVar, "default", t.helpDefault(e.key, defaultDir))
		pad := strings.Repeat(" ", max(helpIndent-len(e.usage)-2, 1))
		fmt.Fprintf(t.stdout, "  %s%s%s\n", e.usage, pad, text)
	}
}

// helpDefault 是說明文字裡 {default} 的值：--ranges 是預設資料夾，--title 是在地化的預設標題，--top 是預設筆數。
func (t term) helpDefault(key, defaultDir string) string {
	switch key {
	case "cli.help.ranges":
		return defaultDir
	case "cli.help.title":
		return t.t("report.title.default")
	default:
		return strconv.Itoa(defaultTop)
	}
}
