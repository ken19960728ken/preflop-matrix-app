package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// EnvVar 是 CLI 範圍資料夾的環境變數名稱。
const EnvVar = "PREFLOP_MATRIX_RANGES"

// ErrNoUserDir 表示需要使用者家目錄或設定資料夾，但作業系統查不到（HOME 或 %AppData% 未設定）。
// CLI 以 exit 2 報出（i18n key cli.error.no_home）。
var ErrNoUserDir = errors.New("cannot determine the user home or config directory")

// Origin 說明範圍資料夾從哪裡來；顯示文字由 i18n（cli.origin.*）決定，這裡只放穩定的代碼。
type Origin string

const (
	OriginFlag    Origin = "flag"
	OriginEnv     Origin = "env"
	OriginDefault Origin = "default"
)

// Source 是解析後的範圍資料夾與其來源。
type Source struct {
	Path   string
	Origin Origin
}

// Env 把作業系統相關的查詢集中起來，測試可以注入。
type Env struct {
	GOOS          string
	Getenv        func(string) string
	UserHomeDir   func() (string, error)
	UserConfigDir func() (string, error)
}

// SystemEnv 是真實環境。
func SystemEnv() Env {
	return Env{GOOS: runtime.GOOS, Getenv: os.Getenv, UserHomeDir: os.UserHomeDir, UserConfigDir: os.UserConfigDir}
}

func (e Env) windows() bool { return e.GOOS == "windows" }

// pyNormalize 照 Python pathlib（POSIX）的規則正規化：去掉空段與 "."、保留 ".."、
// 開頭剛好兩個 "/" 時保留 "//"、去尾斜線、空字串變 "."。
func pyNormalize(p string) string {
	root := ""
	if strings.HasPrefix(p, "/") {
		root = "/"
		if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
			root = "//"
		}
	}
	var parts []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" && seg != "." {
			parts = append(parts, seg)
		}
	}
	if joined := strings.Join(parts, "/"); joined != "" {
		return root + joined
	}
	if root != "" {
		return root
	}
	return "."
}

// clean 依平台正規化路徑：Windows 用 filepath.Clean，其餘照 Python Path。
func (e Env) clean(p string) string {
	if e.windows() {
		return filepath.Clean(p)
	}
	return pyNormalize(p)
}

// join 把 elem 接在 base 後面再正規化（base 為 "/" 或 "//" 時不會產生多餘斜線）。
func (e Env) join(base string, elem ...string) string {
	base = e.clean(base)
	if e.windows() {
		return filepath.Join(append([]string{base}, elem...)...)
	}
	for _, el := range elem {
		if strings.HasSuffix(base, "/") {
			base = pyNormalize(base + el)
		} else {
			base = pyNormalize(base + "/" + el)
		}
	}
	return base
}

// ExpandUser 只處理開頭的 "~" 與 "~/"（Windows 另外接受 "~\"）。
// 刻意偏離 Python：不展開 "~user"（需要查使用者資料庫），原樣保留。
// 需要家目錄但查不到時回傳 ErrNoUserDir。
func (e Env) ExpandUser(p string) (string, error) {
	rest, ok := strings.CutPrefix(p, "~")
	if !ok || (rest != "" && rest[0] != '/' && !(e.windows() && rest[0] == '\\')) {
		return p, nil
	}
	home, err := e.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("%w: expanding %q", ErrNoUserDir, p)
	}
	if e.windows() {
		return filepath.Join(home, strings.ReplaceAll(rest, `\`, "/")), nil
	}
	if home = strings.TrimRight(home, "/"); home == "" && rest == "" {
		return "/", nil
	}
	return pyNormalize(home + rest), nil
}

// DefaultRangesDir：mac/linux 只在 $XDG_CONFIG_HOME（展開 ~ 後）是絕對路徑時使用，否則 ~/.config；
// windows 用 os.UserConfigDir()，忽略 XDG。查不到家目錄／設定資料夾時回傳 ErrNoUserDir。
func (e Env) DefaultRangesDir() (string, error) {
	const sub = "preflop-matrix"
	if e.windows() {
		base, err := e.UserConfigDir()
		if err != nil || base == "" {
			return "", fmt.Errorf("%w: %v", ErrNoUserDir, err)
		}
		return e.join(base, sub, "ranges"), nil
	}
	if xdg, err := e.ExpandUser(e.Getenv("XDG_CONFIG_HOME")); err == nil && strings.HasPrefix(xdg, "/") {
		return e.join(xdg, sub, "ranges"), nil
	}
	home, err := e.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("%w: %v", ErrNoUserDir, err)
	}
	return e.join(home, ".config", sub, "ranges"), nil
}

// ResolveCLI：--ranges > $PREFLOP_MATRIX_RANGES > 預設資料夾。先設定者勝，不往下找；
// flag 為空字串視為已設定（路徑 "."）；環境變數為空字串視為未設定。
// 需要家目錄（~ 或預設資料夾）但查不到時回傳 ErrNoUserDir。
func (e Env) ResolveCLI(flag *string) (Source, error) {
	origin, raw := OriginDefault, ""
	switch v := e.Getenv(EnvVar); {
	case flag != nil:
		origin, raw = OriginFlag, *flag
	case v != "":
		origin, raw = OriginEnv, v
	default:
		p, err := e.DefaultRangesDir()
		return Source{p, OriginDefault}, err
	}
	p, err := e.ExpandUser(raw)
	if err != nil {
		return Source{}, err
	}
	return Source{e.clean(p), origin}, nil
}

// DefaultRangesDir 使用真實環境。
func DefaultRangesDir() (string, error) { return SystemEnv().DefaultRangesDir() }

// ExpandUser 使用真實環境。
func ExpandUser(p string) (string, error) { return SystemEnv().ExpandUser(p) }

// ResolveCLI 使用真實環境。
func ResolveCLI(flag *string) (Source, error) { return SystemEnv().ResolveCLI(flag) }

// CleanPath 照 Python Path 的規則（Windows 用 filepath.Clean）正規化路徑，只用於顯示；不解開 symlink，也不處理 ".."。
func CleanPath(p string) string { return SystemEnv().clean(p) }
