package config

import (
	"errors"
	"path/filepath"
	"testing"
)

// isolate 隔離 HOME、XDG、環境變數，回傳假的 home。測試不得碰真實使用者目錄。
func isolate(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv(EnvVar, "")
	return home
}

func strp(s string) *string { return &s }

func mustDefault(t *testing.T) string {
	t.Helper()
	p, err := DefaultRangesDir()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustResolve(t *testing.T, flag *string) Source {
	t.Helper()
	src, err := ResolveCLI(flag)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func TestDefaultFallsBackToHomeConfig(t *testing.T) {
	home := isolate(t)
	want := filepath.Join(home, ".config", "preflop-matrix", "ranges")
	if got := mustDefault(t); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEmptyOrRelativeXDGIsIgnored(t *testing.T) {
	home := isolate(t)
	want := filepath.Join(home, ".config", "preflop-matrix", "ranges")
	for _, v := range []string{"", "relative/dir"} {
		t.Setenv("XDG_CONFIG_HOME", v)
		if got := mustDefault(t); got != want {
			t.Errorf("XDG=%q: got %q, want %q", v, got, want)
		}
	}
}

func TestAbsoluteXDGIsUsed(t *testing.T) {
	home := isolate(t)
	xdg := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, want := mustDefault(t), filepath.Join(xdg, "preflop-matrix", "ranges"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	t.Setenv("XDG_CONFIG_HOME", "~/xdg")
	if got, want := mustDefault(t), filepath.Join(home, "xdg", "preflop-matrix", "ranges"); got != want {
		t.Errorf("tilde XDG (Python parity): got %q, want %q", got, want)
	}
}

func TestOrderFlagThenEnvThenDefault(t *testing.T) {
	home := isolate(t)
	if src := mustResolve(t, nil); src.Origin != OriginDefault || src.Path != mustDefault(t) {
		t.Errorf("default: %+v", src)
	}
	t.Setenv(EnvVar, "~/env-ranges")
	if src := mustResolve(t, nil); src.Origin != OriginEnv || src.Path != filepath.Join(home, "env-ranges") {
		t.Errorf("env: %+v", src)
	}
	if src := mustResolve(t, strp("/flag")); src.Origin != OriginFlag || src.Path != "/flag" {
		t.Errorf("flag: %+v", src)
	}
}

func TestEmptyFlagIsNotTreatedAsMissing(t *testing.T) {
	isolate(t)
	t.Setenv(EnvVar, "/env")
	if src := mustResolve(t, strp("")); src.Origin != OriginFlag || src.Path != "." {
		t.Errorf("got %+v, want flag origin and path \".\"", src)
	}
}

func TestEmptyEnvCountsAsUnset(t *testing.T) {
	isolate(t)
	if src := mustResolve(t, nil); src.Origin != OriginDefault {
		t.Errorf("got %+v", src)
	}
}

func TestExpandUser(t *testing.T) {
	home := isolate(t)
	cases := map[string]string{
		"~":          home,
		"~/":         home + "/",
		"~/a/b":      filepath.Join(home, "a", "b"),
		"a/~":        "a/~",
		"/abs/~/x":   "/abs/~/x",
		"":           "",
		"relative/x": "relative/x",
	}
	for in, want := range cases {
		got, err := ExpandUser(in)
		if err != nil {
			t.Fatal(err)
		}
		if in == "~/" {
			want = home
		}
		if got != want {
			t.Errorf("ExpandUser(%q) = %q, want %q", in, got, want)
		}
	}
}

// ~user 刻意不展開（與 Python 不同）：需要查使用者資料庫，且 GUI／CLI 都用不到。
func TestExpandUserLeavesTildeUserAlone(t *testing.T) {
	isolate(t)
	for _, in := range []string{"~bob/x", "~bob", "~root"} {
		if got, err := ExpandUser(in); err != nil || got != in {
			t.Errorf("ExpandUser(%q) = %q, %v; want unchanged", in, got, err)
		}
	}
	if src := mustResolve(t, strp("~bob/x")); src.Path != "~bob/x" {
		t.Errorf("ResolveCLI kept %q", src.Path)
	}
}

func TestPyNormalize(t *testing.T) {
	cases := map[string]string{
		"a/../b":    "a/../b",
		"//foo":     "//foo",
		"///foo":    "/foo",
		"////":      "/",
		"/":         "/",
		"//":        "//",
		"/..":       "/..",
		"../x/..":   "../x/..",
		"./a//b/":   "a/b",
		"":          ".",
		".":         ".",
		"./.":       ".",
		"a/./b/../": "a/b/..",
		"/a//b":     "/a/b",
	}
	for in, want := range cases {
		if got := pyNormalize(in); got != want {
			t.Errorf("pyNormalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveUsesPythonNormalization(t *testing.T) {
	isolate(t)
	for in, want := range map[string]string{"./a//b/": "a/b", "//foo": "//foo", "a/../b": "a/../b", "": "."} {
		if src := mustResolve(t, strp(in)); src.Path != want {
			t.Errorf("ResolveCLI(%q).Path = %q, want %q", in, src.Path, want)
		}
	}
}

func fakeEnv(goos string, vars map[string]string) Env {
	return Env{
		GOOS:          goos,
		Getenv:        func(k string) string { return vars[k] },
		UserHomeDir:   func() (string, error) { return "/h", nil },
		UserConfigDir: func() (string, error) { return "/h/AppData/Roaming", nil },
	}
}

func TestWindowsIgnoresXDGAndUsesUserConfigDir(t *testing.T) {
	e := fakeEnv("windows", map[string]string{"XDG_CONFIG_HOME": "/xdg"})
	want := filepath.Join("/h/AppData/Roaming", "preflop-matrix", "ranges")
	if got, err := e.DefaultRangesDir(); err != nil || got != want {
		t.Errorf("got %q, %v, want %q", got, err, want)
	}
}

func TestWindowsExpandsBackslashTilde(t *testing.T) {
	win, unix := fakeEnv("windows", nil), fakeEnv("linux", nil)
	for in, want := range map[string]string{`~\foo`: filepath.Join("/h", "foo"), "~/foo": filepath.Join("/h", "foo"), "~": "/h"} {
		if got, err := win.ExpandUser(in); err != nil || got != want {
			t.Errorf("windows ExpandUser(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if got, _ := unix.ExpandUser(`~\foo`); got != `~\foo` {
		t.Errorf("non-windows ~\\ must stay: %q", got)
	}
}

func TestUnixInjectedEnv(t *testing.T) {
	e := fakeEnv("linux", map[string]string{"XDG_CONFIG_HOME": "/xdg"})
	if got, _ := e.DefaultRangesDir(); got != "/xdg/preflop-matrix/ranges" {
		t.Errorf("got %q", got)
	}
	e = fakeEnv("darwin", nil)
	if got, _ := e.DefaultRangesDir(); got != "/h/.config/preflop-matrix/ranges" {
		t.Errorf("got %q", got)
	}
	e = fakeEnv("linux", map[string]string{"XDG_CONFIG_HOME": "//x/"})
	if got, _ := e.DefaultRangesDir(); got != "//x/preflop-matrix/ranges" {
		t.Errorf("double-slash XDG: got %q", got)
	}
	e.UserHomeDir = func() (string, error) { return "/", nil }
	e.Getenv = func(string) string { return "" }
	if got, _ := e.DefaultRangesDir(); got != "/.config/preflop-matrix/ranges" {
		t.Errorf("root home: got %q", got)
	}
	if got, _ := e.ExpandUser("~"); got != "/" {
		t.Errorf("root home ~: got %q", got)
	}
}

func TestMissingUserDirIsAnError(t *testing.T) {
	boom := func() (string, error) { return "", errors.New("boom") }
	empty := func() (string, error) { return "", nil }
	for name, mutate := range map[string]func(*Env){
		"home error":        func(e *Env) { e.UserHomeDir = boom },
		"home empty":        func(e *Env) { e.UserHomeDir = empty },
		"windows config":    func(e *Env) { e.GOOS = "windows"; e.UserConfigDir = boom },
		"windows config ''": func(e *Env) { e.GOOS = "windows"; e.UserConfigDir = empty },
	} {
		e := fakeEnv("linux", nil)
		mutate(&e)
		if _, err := e.DefaultRangesDir(); !errors.Is(err, ErrNoUserDir) {
			t.Errorf("%s: DefaultRangesDir err = %v", name, err)
		}
		if _, err := e.ResolveCLI(nil); !errors.Is(err, ErrNoUserDir) {
			t.Errorf("%s: ResolveCLI err = %v", name, err)
		}
	}
}

func TestExpandUserWithoutHomeIsAnError(t *testing.T) {
	e := fakeEnv("linux", map[string]string{EnvVar: "~/x"})
	e.UserHomeDir = func() (string, error) { return "", errors.New("no home") }
	if _, err := e.ExpandUser("~/x"); !errors.Is(err, ErrNoUserDir) {
		t.Errorf("err = %v", err)
	}
	if _, err := e.ResolveCLI(nil); !errors.Is(err, ErrNoUserDir) {
		t.Errorf("env ~: err = %v", err)
	}
	if _, err := e.ResolveCLI(strp("~")); !errors.Is(err, ErrNoUserDir) {
		t.Errorf("flag ~: err = %v", err)
	}
	if src, err := e.ResolveCLI(strp("/abs")); err != nil || src.Path != "/abs" {
		t.Errorf("absolute flag must not need home: %+v %v", src, err)
	}
}

func TestWindowsCleansWithFilepath(t *testing.T) {
	e := fakeEnv("windows", nil)
	src, err := e.ResolveCLI(strp("a//b"))
	if err != nil || src.Path != filepath.Clean("a//b") {
		t.Errorf("got %+v %v", src, err)
	}
}
