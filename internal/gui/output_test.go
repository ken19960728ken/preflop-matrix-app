package gui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertReportHeaders 檢查 /report 回應一律帶的 header；ok 為 true（200 的報告）時還要是 HTML（錯誤回應是純文字）。
func assertReportHeaders(t *testing.T, h http.Header, ok bool) {
	t.Helper()
	if ok && h.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", h.Get("Content-Type"))
	}
	want := map[string]string{
		"Cache-Control":           "no-store",
		"Content-Security-Policy": "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src data:; connect-src 'none'; form-action 'none'; base-uri 'none'",
		"X-Content-Type-Options":  "nosniff",
	}
	for k, v := range want {
		if got := h.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestServeHTTPBeforeReportIs404(t *testing.T) {
	e := newEnv(t, nil)
	rec := e.get(t, "/current.html")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
	assertReportHeaders(t, rec.Header(), false)
}

func TestServeHTTPReport(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	if v := e.svc.Generate(); !v.OK {
		t.Fatal(v)
	}
	rec := e.get(t, "/current.html?embed=1&lang=en&v=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	assertReportHeaders(t, rec.Header(), true)
	body := rec.Body.String()
	if !strings.HasPrefix(body, "<!") || !strings.Contains(body, `const DEFAULT_LANG = "en"`) {
		t.Errorf("body is not the English report (%d bytes)", len(body))
	}

	zh := e.get(t, "/current.html?lang=zh-Hans").Body.String()
	if !strings.Contains(zh, `const DEFAULT_LANG = "zh-Hans"`) {
		t.Error("lang=zh-Hans was ignored")
	}
	for _, bad := range []string{"/current.html", "/current.html?lang=fr", "/current.html?lang="} {
		if got := e.get(t, bad).Body.String(); !strings.Contains(got, `const DEFAULT_LANG = "zh-Hant"`) {
			t.Errorf("%s should fall back to the current language (zh-Hant)", bad)
		}
	}
	e.svc.SetLanguage("en")
	if got := e.get(t, "/current.html").Body.String(); !strings.Contains(got, `const DEFAULT_LANG = "en"`) {
		t.Error("fallback does not follow the saved language")
	}
	if again := e.get(t, "/current.html?lang=zh-Hans").Body.String(); again != zh {
		t.Error("the cached render differs from the first one")
	}
}

func TestServeHTTPRoutesAndMethods(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	e.svc.Generate()
	for _, p := range []string{"/", "/other.html", "/current.html/", "/report/current.html"} {
		rec := e.get(t, p)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d", p, rec.Code)
		}
		assertReportHeaders(t, rec.Header(), false)
	}
	rec := httptest.NewRecorder()
	e.svc.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/current.html", nil))
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST = %d, Allow %q", rec.Code, rec.Header().Get("Allow"))
	}
	assertReportHeaders(t, rec.Header(), false)
	head := httptest.NewRecorder()
	e.svc.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/current.html", nil))
	if head.Code != http.StatusOK {
		t.Errorf("HEAD = %d", head.Code)
	}
}

func TestSaveHTML(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	if r := e.svc.SaveHTML(); r.OK || r.Error != "尚未產生報告" || len(e.host.saves) != 0 {
		t.Errorf("no report: %+v, dialog calls %d", r, len(e.host.saves))
	}
	e.svc.Generate()

	if r := e.svc.SaveHTML(); !r.Cancelled || r.OK || r.Error != "" {
		t.Errorf("cancel = %+v", r)
	}
	if got := e.host.saves[0].name; got != "preflop_matrix.html" {
		t.Errorf("default name = %q", got)
	}

	e.host.saveErr = errors.New("sheet failed")
	if r := e.svc.SaveHTML(); r.OK || !strings.Contains(r.Error, "sheet failed") {
		t.Errorf("dialog error = %+v", r)
	}
	e.host.saveErr = nil

	dir := t.TempDir()
	out := filepath.Join(dir, "r.html")
	e.host.savePath = out
	r := e.svc.SaveHTML()
	if !r.OK || r.Path != out {
		t.Fatalf("save = %+v", r)
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), `const DEFAULT_LANG = "zh-Hant"`) {
		t.Errorf("saved file: %v", err)
	}
	if got := e.settings(t).LastSaveDir; got != dir {
		t.Errorf("last_save_dir = %q", got)
	}
	e.svc.SaveHTML()
	if got := e.host.saves[len(e.host.saves)-1].dir; got != dir {
		t.Errorf("second dialog starts at %q, want the last save dir", got)
	}

	e.host.savePath = filepath.Join(dir, "no", "such", "dir", "r.html")
	if r := e.svc.SaveHTML(); r.OK || r.Error == "" {
		t.Errorf("write failure = %+v", r)
	}
}

func TestPrint(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	if r := e.svc.Print(); r.OK || r.Error == "" || len(e.host.printed) != 0 {
		t.Errorf("no report: %+v", r)
	}
	e.svc.Generate()
	e.svc.SetLanguage("en")
	if r := e.svc.Print(); !r.OK {
		t.Fatalf("Print = %+v", r)
	}
	if got := e.host.printed[0]; got.url != "/report/current.html?lang=en" || got.title != "Print report" {
		t.Errorf("printed = %+v", got)
	}
	e.host.printErr = errors.New("no window")
	if r := e.svc.Print(); r.OK || !strings.Contains(r.Error, "no window") {
		t.Errorf("host error = %+v", r)
	}
}

func TestOpenInBrowser(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	if r := e.svc.OpenInBrowser(); r.OK || r.Error == "" {
		t.Errorf("no report: %+v", r)
	}
	e.svc.Generate()
	r := e.svc.OpenInBrowser()
	want := filepath.Join(e.cacheDir, "report.html")
	if !r.OK || r.Path != want || len(e.host.opened) != 1 || e.host.opened[0] != want {
		t.Fatalf("OpenInBrowser = %+v, opened %v", r, e.host.opened)
	}
	b, err := os.ReadFile(want)
	if err != nil || !strings.Contains(string(b), "DEFAULT_LANG") {
		t.Errorf("temp report: %v", err)
	}
	if info, _ := os.Stat(want); info.Mode().Perm() != 0o600 {
		t.Errorf("temp report mode = %v", info.Mode().Perm())
	}

	e.host.openErr = errors.New("no browser")
	if r := e.svc.OpenInBrowser(); r.OK || !strings.Contains(r.Error, "no browser") {
		t.Errorf("host error = %+v", r)
	}
	e.host.openErr = nil

}

func TestOpenInBrowserRejectsUnsafeCacheDir(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	e.svc.Generate()
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	open := filepath.Join(root, "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"symlink": link, "group-readable": open, "a file": file, "under a file": filepath.Join(file, "sub")} {
		e.svc.cacheDir = dir
		if r := e.svc.OpenInBrowser(); r.OK || r.Error == "" {
			t.Errorf("%s accepted: %+v", name, r)
		}
	}
	if len(e.host.opened) != 0 {
		t.Errorf("browser opened for an unsafe folder: %v", e.host.opened)
	}
	if entries, _ := os.ReadDir(real); len(entries) != 0 {
		t.Errorf("wrote through the symlink: %v", entries)
	}
}

func TestOpenInBrowserReplacesReportAtomically(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	e.svc.Generate()
	e.svc.OpenInBrowser()
	e.svc.SetLanguage("en")
	r := e.svc.OpenInBrowser()
	if !r.OK {
		t.Fatal(r)
	}
	entries, _ := os.ReadDir(e.cacheDir)
	if len(entries) != 1 || entries[0].Name() != "report.html" {
		t.Errorf("cache dir holds %v", entries)
	}
	if b, _ := os.ReadFile(r.Path); !strings.Contains(string(b), `const DEFAULT_LANG = "en"`) {
		t.Error("report.html was not replaced")
	}
}

func TestSaveHTMLAddsExtensionWithoutOverwriting(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	e.svc.Generate()
	dir := t.TempDir()
	e.host.savePath = filepath.Join(dir, "report")
	r := e.svc.SaveHTML()
	if !r.OK || r.Path != filepath.Join(dir, "report.html") {
		t.Fatalf("save = %+v", r)
	}
	if err := os.WriteFile(r.Path, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := e.svc.SaveHTML(); r.OK || !strings.Contains(r.Error, "report.html") {
		t.Errorf("overwrite = %+v", r)
	}
	if b, _ := os.ReadFile(r.Path); string(b) != "mine" {
		t.Error("existing file was overwritten")
	}
	e.host.savePath = filepath.Join(dir, "other.html")
	if r := e.svc.SaveHTML(); !r.OK {
		t.Errorf("explicit .html = %+v", r)
	}
}
