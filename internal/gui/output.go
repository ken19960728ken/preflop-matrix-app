package gui

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
	"github.com/ken19960728ken/preflop-matrix-app/internal/pipeline"
)

const (
	reportPath       = "/current.html"
	saveName         = "preflop_matrix.html"
	cachedReportName = "report.html"
	// sandbox iframe 仍可對 /wails/runtime 盲呼叫，所以報告頁一律禁止連線與表單，只留內嵌 script／style 與 Google Fonts。
	reportCSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' https://fonts.googleapis.com; " +
		"font-src https://fonts.gstatic.com; img-src data:; connect-src 'none'; form-action 'none'; base-uri 'none'"
)

// htmlLocked 回傳 lang 的報告 HTML（每個語言只渲染一次）；呼叫者持有 mu 且 s.rep.res 非 nil。
func (s *AppService) htmlLocked(lang string) ([]byte, error) {
	if b, ok := s.rep.html[lang]; ok {
		return b, nil
	}
	_, b, err := pipeline.Render(s.rep.res, lang, nil, s.rep.topN)
	if err != nil {
		return nil, err
	}
	if s.rep.html == nil {
		s.rep.html = map[string][]byte{}
	}
	s.rep.html[lang] = b
	return b, nil
}

// ServeHTTP 提供 /report/current.html（Wails 已剝掉 /report 前綴）。lang 參數選語言，缺省或不支援時用目前語言；
// embed=1 由頁面自己的 script 處理，伺服器端輸出相同。還沒有報告回 404。
func (s *AppService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Security-Policy", reportCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path != reportPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	lang := r.URL.Query().Get("lang")
	if !i18n.IsSupported(lang) {
		lang = s.langOf(s.loadLocked())
	}
	var body []byte
	var err error
	if s.rep.res != nil {
		body, err = s.htmlLocked(lang)
	}
	have := s.rep.res != nil
	s.mu.Unlock()
	switch {
	case !have:
		http.NotFound(w, r)
	case err != nil:
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	default:
		_, _ = w.Write(body)
	}
}

// currentHTML 回傳目前語言的報告 HTML；沒有報告或渲染失敗時回傳已翻譯的錯誤。
func (s *AppService) currentHTML() (snapshot, []byte, *Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.loadLocked()
	sn := snapshot{st, s.langOf(st), s.ctx}
	if s.rep.res == nil {
		r := fail(sn.lang, "gui.error.no_report")
		return sn, nil, &r
	}
	b, err := s.htmlLocked(sn.lang)
	if err != nil {
		r := fail(sn.lang, "gui.error.internal", "err", err.Error())
		return sn, nil, &r
	}
	return sn, b, nil
}

// SaveHTML 開存檔對話框，把目前語言的報告（含三語切換器）存成 HTML，並記住資料夾。
// 使用者沒給副檔名時補 .html；補完後的檔案若已存在則不覆蓋（對話框只確認過原本的檔名）。
func (s *AppService) SaveHTML() Result {
	sn, html, bad := s.currentHTML()
	if bad != nil {
		return *bad
	}
	path, err := s.host.SaveFile(tr(sn.lang, "gui.dialog.save_html"), saveName, validDir(sn.st.LastSaveDir))
	if err != nil {
		return fail(sn.lang, "gui.error.dialog", "err", err.Error())
	}
	if path == "" {
		return cancelled()
	}
	if filepath.Ext(path) == "" {
		path += ".html"
		if _, err := os.Lstat(path); err == nil {
			return fail(sn.lang, "gui.error.file_exists", "path", path)
		}
	}
	if err := os.WriteFile(path, html, 0o644); err != nil {
		return fail(sn.lang, "gui.error.write_file", "err", err.Error())
	}
	r := saved(s.update(func(st *config.Settings) { st.LastSaveDir = filepath.Dir(path) }))
	r.Path = path
	return r
}

// Print 開新視窗載入報告，載入完成後叫出系統列印面板。
func (s *AppService) Print() Result {
	sn, _, bad := s.currentHTML()
	if bad != nil {
		return *bad
	}
	if err := s.host.PrintReport(tr(sn.lang, "gui.print.window_title"), "/report"+reportPath+"?lang="+sn.lang); err != nil {
		return fail(sn.lang, "gui.error.print", "err", err.Error())
	}
	return Result{OK: true}
}

// OpenInBrowser 把目前語言的報告寫到使用者專屬的快取資料夾並用預設瀏覽器開啟。
// 資料夾必須是自己擁有、權限 0700 的真資料夾（不是 symlink），否則不寫入；
// 檔案先寫成同資料夾的暫存檔再 rename，瀏覽器不會讀到寫到一半的內容。
func (s *AppService) OpenInBrowser() Result {
	sn, html, bad := s.currentHTML()
	if bad != nil {
		return *bad
	}
	path, err := s.writeCachedReport(html)
	switch {
	case errors.Is(err, errUnsafeDir):
		return fail(sn.lang, "gui.error.cache_dir_unsafe", "path", s.cacheDir)
	case err != nil:
		return fail(sn.lang, "gui.error.write_file", "err", err.Error())
	}
	if err := s.host.OpenFile(path); err != nil {
		return fail(sn.lang, "gui.error.open_browser", "err", err.Error())
	}
	return Result{OK: true, Path: path}
}

var errUnsafeDir = errors.New("cache directory is not a private directory owned by the user")

func (s *AppService) writeCachedReport(html []byte) (string, error) {
	if err := os.MkdirAll(s.cacheDir, 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(s.cacheDir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 || !ownedByCurrentUser(info) {
		return "", errUnsafeDir
	}
	tmp, err := os.CreateTemp(s.cacheDir, ".report-*.tmp")
	if err != nil {
		return "", err
	}
	_, werr := tmp.Write(html)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
		return "", werr
	}
	path := filepath.Join(s.cacheDir, cachedReportName)
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return path, nil
}
