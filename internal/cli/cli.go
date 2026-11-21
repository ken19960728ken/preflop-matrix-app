package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
)

// Exit codes（與 Python 版相同）。
const (
	exitOK       = 0
	exitFailed   = 1
	exitBadInput = 2
)

// term 是一次執行的輸出管道與語言。
type term struct {
	stdout, stderr io.Writer
	lang           string
}

// t 取得 key 的當前語言文字；kv 是成對的佔位符名稱與值。
func (t term) t(key string, kv ...string) string {
	args := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		args[kv[i]] = kv[i+1]
	}
	return i18n.T(t.lang, key, args)
}

// say 把 key 的文字印到 stdout。
func (t term) say(key string, kv ...string) { fmt.Fprintln(t.stdout, t.t(key, kv...)) }

// fail 把 key 的文字加上「錯誤：」前綴印到 stderr。
func (t term) fail(key string, kv ...string) { t.failText(t.t(key, kv...)) }

// failText 把已翻譯好的訊息加上「錯誤：」前綴印到 stderr。
func (t term) failText(message string) {
	fmt.Fprintln(t.stderr, t.t("cli.error_prefix", "message", message))
}

// warn 把已翻譯好的訊息加上「警告：」前綴印到 stderr。
func (t term) warn(message string) {
	fmt.Fprintln(t.stderr, t.t("cli.warning_prefix", "message", message))
}

// Run 執行命令列模式並回傳 exit code。
func Run(args []string) int { return run(args, os.Stdout, os.Stderr) }

func run(args []string, stdout, stderr io.Writer) int {
	o, perr := parseArgs(args)
	lang, langOK := resolveLang(o)
	t := term{stdout, stderr, lang}
	top, topOK := parseTop(o)
	switch {
	case perr != nil:
		t.fail(perr.key, flatten(perr.args)...)
	case !langOK:
		t.fail("cli.error.lang_invalid", "value", o.vals[keyLang])
	case !topOK:
		t.fail("cli.error.top_range", "value", o.vals[keyTop])
	case modeConflict(o):
		t.fail("cli.error.mode_conflict")
	case o.has(keyHelp):
		t.printHelp()
		return exitOK
	case o.has(keyCheck):
		return t.runCheck(o)
	case o.has(keyInit):
		return t.runInit(o.vals[keyInit])
	case !o.has(keyHands):
		t.fail("cli.error.hands_required")
	default:
		return t.runReport(o, top)
	}
	return exitBadInput
}

func flatten(args map[string]string) []string {
	var kv []string
	for k, v := range args {
		kv = append(kv, k, v)
	}
	return kv
}
