package cli

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
)

// 旗標的標準名稱（flagSpec.key）。
const (
	keyHands  = "hands"
	keyRanges = "ranges"
	keyOut    = "out"
	keyHero   = "hero"
	keyTitle  = "title"
	keyJSON   = "json"
	keyLang   = "lang"
	keyTop    = "top"
	keyCheck  = "check-ranges"
	keyInit   = "init-ranges"
	keyHelp   = "help"
)

const (
	defaultOut  = "preflop_matrix.html"
	defaultHero = "Hero"
	defaultTop  = 5
)

// flagSpec 是一個旗標：所有名稱（含簡寫）、標準名稱、是否帶值。解析器、IsCLIArgs 與說明文字都由 flagSpecs 產生。
type flagSpec struct {
	names []string
	key   string
	value bool
}

var flagSpecs = []flagSpec{
	{[]string{"--hands"}, keyHands, true},
	{[]string{"--ranges"}, keyRanges, true},
	{[]string{"-o", "--out"}, keyOut, true},
	{[]string{"--hero"}, keyHero, true},
	{[]string{"--title"}, keyTitle, true},
	{[]string{"--json"}, keyJSON, true},
	{[]string{"--lang"}, keyLang, true},
	{[]string{"--top"}, keyTop, true},
	{[]string{"--check-ranges"}, keyCheck, false},
	{[]string{"--init-ranges"}, keyInit, true},
	{[]string{"-h", "--help"}, keyHelp, false},
}

// knownFlags 是命令列模式認得的旗標名稱；GUI App 只在第一個參數屬於此集合時才走 CLI，
// 其餘參數（-AppleLanguages、-NS*、-psn_*、檔案關聯路徑…）一律開 GUI。
var knownFlags = func() map[string]*flagSpec {
	m := map[string]*flagSpec{}
	for i := range flagSpecs {
		for _, n := range flagSpecs[i].names {
			m[n] = &flagSpecs[i]
		}
	}
	return m
}()

// lookup 找出 arg 對應的旗標：--x=v、-o=v、-oVALUE 都會回傳內嵌的值（hasVal 為 true）。
func lookup(arg string) (spec *flagSpec, val string, hasVal bool) {
	name, val, hasVal := strings.Cut(arg, "=")
	if spec = knownFlags[name]; spec != nil {
		return spec, val, hasVal
	}
	if rest, ok := strings.CutPrefix(arg, "-o"); ok && rest != "" && !strings.HasPrefix(arg, "--") {
		return knownFlags["-o"], rest, true
	}
	return nil, "", false
}

// IsCLIArgs 回報 args 是否為命令列呼叫：第一個參數是已知旗標（含 --x=value、-oVALUE 形式）。
func IsCLIArgs(args []string) bool {
	if len(args) == 0 {
		return false
	}
	spec, _, _ := lookup(args[0])
	return spec != nil
}

var negativeNumber = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

// looksLikeFlag 同 argparse：以 - 開頭（且不是單獨的 -）又不是負數的字串不能當作以空白分隔的值。
func looksLikeFlag(s string) bool {
	return len(s) > 1 && s[0] == '-' && !negativeNumber.MatchString(s)
}

// usageError 是參數錯誤：i18n key 與佔位符，由呼叫端以選定的語言印出。
type usageError struct {
	key  string
	args map[string]string
}

// options 是解析後的命令列：vals 是帶值旗標最後一次的值，seen 是出現過的旗標。
type options struct {
	vals map[string]string
	seen map[string]bool
}

func (o options) has(key string) bool { return o.seen[key] }

func (o options) get(key, def string) string {
	if o.seen[key] {
		return o.vals[key]
	}
	return def
}

// parseArgs 解析 args（支援 --x v 與 --x=v，後者覆蓋前者）。遇到錯誤仍掃完全部參數，回傳第一個錯誤，
// 讓呼叫端能先從 --lang 決定錯誤訊息的語言。
func parseArgs(args []string) (options, *usageError) {
	o := options{vals: map[string]string{}, seen: map[string]bool{}}
	var first *usageError
	fail := func(key string, kv ...string) {
		if first == nil {
			first = &usageError{key, map[string]string{}}
			for i := 0; i+1 < len(kv); i += 2 {
				first.args[kv[i]] = kv[i+1]
			}
		}
	}
	for i := 0; i < len(args); i++ {
		spec, val, hasVal := lookup(args[i])
		switch {
		case spec == nil || (!spec.value && hasVal):
			fail("cli.error.unknown_flag", "arg", args[i])
		case !spec.value:
			o.seen[spec.key] = true
		case hasVal:
			o.seen[spec.key], o.vals[spec.key] = true, val
		case i+1 >= len(args) || looksLikeFlag(args[i+1]):
			fail("cli.error.missing_value", "flag", args[i])
		default:
			i++
			o.seen[spec.key], o.vals[spec.key] = true, args[i]
		}
	}
	return o, first
}

// langAliases 把 --lang 接受的寫法（不分大小寫）對應到語言代碼。
var langAliases = map[string]string{
	"zh-hant": i18n.ZhHant, "zh-tw": i18n.ZhHant,
	"zh-hans": i18n.ZhHans, "zh-cn": i18n.ZhHans,
	"en": i18n.En,
}

// resolveLang 回傳輸出語言：--lang 的值，未給用系統語言。--lang 值不認得時回傳 ok=false（語言退回系統語言）。
func resolveLang(o options) (lang string, ok bool) {
	if !o.has(keyLang) {
		return i18n.Detect(), true
	}
	lang, ok = langAliases[strings.ToLower(o.vals[keyLang])]
	if !ok {
		return i18n.Detect(), false
	}
	return lang, true
}

// parseTop 解析 --top（預設 defaultTop）；不是 0 到 engine.MaxTopN 的整數回傳 ok=false。
func parseTop(o options) (n int, ok bool) {
	if !o.has(keyTop) {
		return defaultTop, true
	}
	n, err := strconv.Atoi(strings.TrimSpace(o.vals[keyTop]))
	return n, err == nil && engine.ValidTopN(n)
}

// conflictKeys 是與 --check-ranges／--init-ranges 互斥的產生報告旗標。
var conflictKeys = []string{keyHands, keyOut, keyJSON, keyHero, keyTitle, keyTop}

// modeConflict 回報 --check-ranges／--init-ranges 是否與彼此或產生報告的旗標併用。
func modeConflict(o options) bool {
	if !o.has(keyCheck) && !o.has(keyInit) {
		return false
	}
	if o.has(keyCheck) && o.has(keyInit) || o.has(keyInit) && o.has(keyRanges) {
		return true
	}
	for _, k := range conflictKeys {
		if o.has(k) {
			return true
		}
	}
	return false
}
