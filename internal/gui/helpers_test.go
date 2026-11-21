package gui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
)

const (
	corpusHands  = "../../testdata/parity/random_s42/hands"
	corpusRanges = "../../testdata/parity/random_s42/ranges"
)

type pickCall struct{ title, start string }

type saveCall struct{ title, name, dir string }

type printCall struct{ title, url string }

// fakeHost 是 host 的假物件：回傳預先設定的值並記錄呼叫。
type fakeHost struct {
	mu       sync.Mutex
	pickPath string
	pickErr  error
	picks    []pickCall
	savePath string
	saveErr  error
	saves    []saveCall
	openErr  error
	opened   []string
	printErr error
	printed  []printCall
}

func (f *fakeHost) PickDir(title, start string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.picks = append(f.picks, pickCall{title, start})
	return f.pickPath, f.pickErr
}

func (f *fakeHost) SaveFile(title, name, dir string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves = append(f.saves, saveCall{title, name, dir})
	return f.savePath, f.saveErr
}

func (f *fakeHost) OpenFile(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, path)
	return f.openErr
}

func (f *fakeHost) PrintReport(title, url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.printed = append(f.printed, printCall{title, url})
	return f.printErr
}

type env struct {
	svc          *AppService
	host         *fakeHost
	settingsPath string
	cacheDir     string
}

// newEnv 建立使用 TempDir 設定檔的服務；系統語言固定為 zh-Hant，mut 可預先改設定。
func newEnv(t *testing.T, mut func(*config.Settings)) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{host: &fakeHost{}, settingsPath: filepath.Join(root, "cfg", "settings.json"), cacheDir: filepath.Join(root, "cache")}
	e.svc = NewAppService(Options{SettingsPath: e.settingsPath, CacheDir: e.cacheDir, DetectLang: func() string { return "zh-Hant" }})
	e.svc.host = e.host
	if mut != nil {
		st := config.DefaultSettings()
		mut(&st)
		if err := config.Save(e.settingsPath, st); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func (e *env) settings(t *testing.T) config.Settings {
	t.Helper()
	st, err := config.Load(e.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func withCorpus(withRanges bool) func(*config.Settings) {
	return func(st *config.Settings) {
		st.HandsDir = corpusHands
		if withRanges {
			st.RangesDir = corpusRanges
		}
	}
}

// applicationOptions 是 ServiceStartup 的第二個參數（服務不使用它）。
var applicationOptions application.ServiceOptions

// get 對 ServeHTTP 發出 GET 請求。
func (e *env) get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e.svc.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// lightHands 複製語料的前幾個手牌檔到暫存資料夾（完整語料在 -race 下太慢；只有需要偏離列的測試才用 corpusHands）。
func lightHands(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	n := 0
	err := fs.WalkDir(os.DirFS(corpusHands), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || n >= 4 {
			return err
		}
		n++
		b, err := os.ReadFile(filepath.Join(corpusHands, p))
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, strings.ReplaceAll(p, "/", "_")), b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// withLight 指定輕量手牌；ranges 為範圍資料夾（空字串＝不選）。
func withLight(t *testing.T, ranges string) func(*config.Settings) {
	hands := lightHands(t)
	return func(st *config.Settings) { st.HandsDir, st.RangesDir = hands, ranges }
}
