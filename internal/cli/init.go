package cli

import (
	"os"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
)

// countTxt 數出 names 裡的 .txt 檔（說明檔不算樣板）。
func countTxt(names []string) int {
	n := 0
	for _, name := range names {
		if strings.HasSuffix(name, ".txt") {
			n++
		}
	}
	return n
}

func contains(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// runInit 在 dir 建立範圍檔名樣板：成功 0（全部略過也是 0）、寫入失敗 1；路徑不是資料夾或家目錄查不到為 2。
func (t term) runInit(dir string) int {
	if dir == "" {
		t.fail("cli.error.missing_value", "flag", "--init-ranges")
		return exitBadInput
	}
	target, err := config.ExpandUser(dir)
	if err != nil {
		t.failNoHome(err, false)
		return exitBadInput
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		t.fail("cli.error.not_a_dir", "path", shown(target))
		return exitBadInput
	}
	created, skipped, mapped, err := engine.InitTemplate(target)
	if err != nil {
		t.fail("cli.error.init_failed", "err", err.Error())
		return exitFailed
	}
	t.say("cli.init.summary", "created", itoa(countTxt(created)), "skipped", itoa(countTxt(skipped)),
		"mapped", itoa(len(mapped)), "dir", shown(target))
	for _, m := range mapped {
		t.say("cli.init.mapped", "name", m.Name, "existing", m.Existing)
	}
	if contains(created, engine.HowToFillName) {
		t.say("cli.init.howto_created", "name", engine.HowToFillName)
	} else if contains(skipped, engine.HowToFillName) {
		t.say("cli.init.howto_exists", "name", engine.HowToFillName)
	}
	return exitOK
}
