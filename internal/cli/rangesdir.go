package cli

import (
	"os"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
)

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// failNoHome 印出 cli.error.no_home（config 與 ExpandUser 只會因 config.ErrNoUserDir 失敗）；
// hint 為 true 時附上「請改用 --ranges」的提示（只有範圍來源解析會加）。
func (t term) failNoHome(err error, hint bool) {
	msg := t.t("cli.error.no_home", "err", err.Error())
	if hint {
		msg += t.t("cli.hint.no_home_ranges")
	}
	t.failText(msg)
}

// originLabel 是範圍來源的在地化名稱（cli.origin.*）。
func (t term) originLabel(origin config.Origin) string {
	switch origin {
	case config.OriginFlag:
		return t.t("cli.origin.flag")
	case config.OriginEnv:
		return t.t("cli.origin.env", "env", config.EnvVar)
	default:
		return t.t("cli.origin.default")
	}
}

// rangesHint 是範圍資料夾不存在時附加的設定提示；來源是參數時沒有提示。
func (t term) rangesHint(src config.Source) string {
	switch src.Origin {
	case config.OriginDefault:
		return t.t("cli.hint.default", "env", config.EnvVar, "default", src.Path)
	case config.OriginEnv:
		return t.t("cli.hint.env", "env", config.EnvVar)
	}
	return ""
}

// resolveRangesDir 依 --ranges／環境變數／預設資料夾決定範圍資料夾並確認它存在，成功時印出來源行。
// 失敗時已印出錯誤，回傳 ok=false。
func (t term) resolveRangesDir(o options) (src config.Source, ok bool) {
	var flag *string
	if o.has(keyRanges) {
		v := o.vals[keyRanges]
		flag = &v
	}
	src, err := config.ResolveCLI(flag)
	if err != nil {
		t.failNoHome(err, true)
		return src, false
	}
	if !isDir(src.Path) {
		t.fail("cli.error.ranges_dir", "origin", t.originLabel(src.Origin), "path", src.Path, "hint", t.rangesHint(src))
		return src, false
	}
	t.say("cli.ranges_source", "origin", t.originLabel(src.Origin), "path", src.Path)
	return src, true
}
