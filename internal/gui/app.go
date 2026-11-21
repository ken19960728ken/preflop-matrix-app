package gui

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
	"github.com/ken19960728ken/preflop-matrix-app/internal/pipeline"
)

// Version 是顯示在 State.Version 的版本；預設值與 build/config.yml 的 version 相同（有測試比對），
// 建置時由 Taskfile 以 -ldflags "-X" 注入。
var Version = "0.0.1"

const (
	maxHeroRunes = 64

	langSaved  = "saved"
	langSystem = "system"
	uiNS       = "ui"
)

// Options 是 NewAppService 的參數。
type Options struct {
	SettingsPath string        // settings.json 的完整路徑
	CacheDir     string        // OpenInBrowser 寫報告的資料夾（使用者專屬的快取目錄底下）
	DetectLang   func() string // 系統語言（i18n.Detect）
}

// Result 是只回報成敗的方法結果；Error 為已翻成當前語言的字串（空＝無）。
type Result struct {
	OK        bool   `json:"ok"`
	Cancelled bool   `json:"cancelled"`
	Error     string `json:"error"`
	Path      string `json:"path"`
}

// RangesState 是範圍資料夾的狀態：Picked 表示使用者選過，Exists 表示選過的資料夾目前還在。
type RangesState struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Picked bool   `json:"picked"`
}

// State 是前端畫面需要的全部設定與狀態。
type State struct {
	Lang       string      `json:"lang"`
	LangSource string      `json:"langSource"`
	HandsDir   string      `json:"handsDir"`
	Ranges     RangesState `json:"ranges"`
	Hero       string      `json:"hero"`
	TopN       int         `json:"topN"`
	Version    string      `json:"version"`
	HasReport  bool        `json:"hasReport"`
	Busy       bool        `json:"busy"`
	// SettingsWarning 非空表示設定檔損壞或欄位無效而改用預設值；保留到下次成功存檔為止。
	SettingsWarning string `json:"settingsWarning"`
}

// report 是最後一次 Generate 的結果與其渲染快取。
type report struct {
	res     *pipeline.Result
	topN    int
	seq     int
	changed bool              // 產生期間設定（資料夾或 Hero）被改過
	html    map[string][]byte // lang → 報告 HTML
}

// AppService 是 Wails 綁定的服務，同時是 /report 路由的 http.Handler。
// mu 同時序列化設定檔的 Load→改→Save 與報告狀態；耗時的載入與建矩陣不持鎖。
type AppService struct {
	host         host
	settingsPath string
	cacheDir     string
	systemLang   string

	mu   sync.Mutex
	ctx  context.Context
	busy bool
	rep  report
	// beforeFinish 只給測試用：在 Generate 載入完成、存入結果之前呼叫（模擬載入期間發生的事）。
	beforeFinish func()
	warn         error // 第一次 Load 遇到的損壞警告，成功 Save 後清除
}

// NewAppService 建立服務；host 在 app 建好之後由 Run 指定。
func NewAppService(o Options) *AppService {
	detect := o.DetectLang
	if detect == nil {
		detect = i18n.Detect
	}
	return &AppService{
		settingsPath: o.SettingsPath, cacheDir: o.CacheDir, systemLang: detect(),
		ctx: context.Background(),
	}
}

// ServiceStartup 取得 App 的 context：App 關閉前會被取消，進行中的 Generate 隨之中止。
func (s *AppService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx = ctx
	return nil
}

// snapshot 是一次操作開始時看到的設定、語言與 context。
type snapshot struct {
	st   config.Settings
	lang string
	ctx  context.Context
}

// loadChecked 讀設定；損壞（含欄位無效）時回傳預設值並記下警告（只記錄與記 log 一次），讀取失敗（權限等）回 error。
func (s *AppService) loadChecked() (config.Settings, error) {
	st, err := config.Load(s.settingsPath)
	switch {
	case err == nil:
	case errors.Is(err, config.ErrCorruptSettings):
		if s.warn == nil {
			s.warn = err
			log.Println("settings:", err)
		}
	default:
		return st, err
	}
	return st, nil
}

// loadLocked 同 loadChecked，但讀取失敗時只記 log 並回傳預設值（唯讀的呼叫者用）。
func (s *AppService) loadLocked() config.Settings {
	st, err := s.loadChecked()
	if err != nil {
		log.Println("settings:", err)
	}
	return st
}

func (s *AppService) langOf(st config.Settings) string {
	if st.Lang != "" {
		return st.Lang
	}
	return s.systemLang
}

func (s *AppService) snap() snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.loadLocked()
	return snapshot{st, s.langOf(st), s.ctx}
}

// update 在鎖內 Load→fn→Save；回傳存檔後的設定與語言。讀取或存檔失敗回 error 且不改動設定檔。
func (s *AppService) update(fn func(*config.Settings)) (snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateLocked(fn)
}

func (s *AppService) updateLocked(fn func(*config.Settings)) (snapshot, error) {
	st, err := s.loadChecked()
	before := s.langOf(st)
	if err != nil {
		return snapshot{st, before, s.ctx}, err
	}
	fn(&st)
	if err := config.Save(s.settingsPath, st); err != nil {
		return snapshot{st, before, s.ctx}, err
	}
	s.warn = nil
	return snapshot{st, s.langOf(st), s.ctx}, nil
}

// tr 翻譯 gui（與其他）命名空間的字串；kv 是成對的 {佔位符, 值}。
func tr(lang, key string, kv ...string) string {
	var args map[string]string
	if len(kv) > 0 {
		args = make(map[string]string, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			args[kv[i]] = kv[i+1]
		}
	}
	return i18n.T(lang, key, args)
}

func fail(lang, key string, kv ...string) Result { return Result{Error: tr(lang, key, kv...)} }

func itoa(n int) string { return strconv.Itoa(n) }

func cancelled() Result { return Result{Cancelled: true} }

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// GetState 回傳畫面所需的全部狀態。
func (s *AppService) GetState() State {
	s.mu.Lock()
	st := s.loadLocked()
	lang := s.langOf(st)
	out := State{
		Lang: lang, LangSource: langSystem, HandsDir: st.HandsDir,
		Ranges: RangesState{Path: st.RangesDir, Picked: st.RangesDir != ""},
		Hero:   st.Hero, TopN: st.TopN, Version: Version, HasReport: s.rep.res != nil, Busy: s.busy,
	}
	if st.Lang != "" {
		out.LangSource = langSaved
	}
	if s.warn != nil {
		out.SettingsWarning = tr(lang, "gui.warning.settings_corrupt", "err", s.warn.Error())
	}
	s.mu.Unlock()
	out.Ranges.Exists = out.Ranges.Picked && isDir(out.Ranges.Path)
	return out
}

// saved 把 update 的結果轉成 Result。
func saved(sn snapshot, err error) Result {
	if err != nil {
		return fail(sn.lang, "gui.error.settings_save", "err", err.Error())
	}
	return Result{OK: true}
}

// SetLanguage 記住語言；只接受三個支援的語言代碼。
func (s *AppService) SetLanguage(lang string) Result {
	if !i18n.IsSupported(lang) {
		return fail(s.snap().lang, "gui.error.bad_lang", "lang", lang)
	}
	return saved(s.update(func(st *config.Settings) { st.Lang = lang }))
}

// SetHero 記住 Hero 名稱；空白退回預設。
func (s *AppService) SetHero(name string) Result {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxHeroRunes {
		return fail(s.snap().lang, "gui.error.hero_too_long", "max", itoa(maxHeroRunes))
	}
	if name == "" {
		name = config.DefaultHero
	}
	return saved(s.update(func(st *config.Settings) { st.Hero = name }))
}

// SetTopN 記住偏離摘要的列數，並只重算目前報告的偏離摘要（不重新載入檔案）。
func (s *AppService) SetTopN(n int) Result {
	if !engine.ValidTopN(n) {
		return fail(s.snap().lang, "gui.error.bad_top_n", "max", itoa(engine.MaxTopN))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, err := s.updateLocked(func(st *config.Settings) { st.TopN = n })
	if err != nil {
		return fail(sn.lang, "gui.error.settings_save", "err", err.Error())
	}
	if s.rep.res != nil {
		if err := s.rep.res.SetTopN(n); err != nil {
			return fail(sn.lang, "gui.error.internal", "err", err.Error())
		}
		s.rep.topN = n
		s.rep.seq++
		s.rep.html = nil
	}
	return Result{OK: true}
}

// pick 開資料夾對話框並把選到的路徑交給 store；取消回 cancelled。
func (s *AppService) pick(titleKey, startDir string, store func(*config.Settings, string)) Result {
	sn := s.snap()
	dir, err := s.host.PickDir(tr(sn.lang, titleKey), validDir(startDir))
	if err != nil {
		return fail(sn.lang, "gui.error.dialog", "err", err.Error())
	}
	if dir == "" {
		return cancelled()
	}
	r := saved(s.update(func(st *config.Settings) { store(st, dir) }))
	r.Path = dir
	return r
}

// validDir 只在 dir 是現存資料夾時回傳它，否則空字串（交給系統決定對話框起點）。
func validDir(dir string) string {
	if dir != "" && isDir(dir) {
		return dir
	}
	return ""
}

// PickHandsDir 開對話框選手牌資料夾並記住。
func (s *AppService) PickHandsDir() Result {
	return s.pick("gui.dialog.pick_hands", s.snap().st.HandsDir, func(st *config.Settings, dir string) { st.HandsDir = dir })
}

// PickRangesDir 開對話框選理論範圍資料夾並記住。
func (s *AppService) PickRangesDir() Result {
	return s.pick("gui.dialog.pick_ranges", s.snap().st.RangesDir, func(st *config.Settings, dir string) { st.RangesDir = dir })
}

// ClearRangesDir 清除範圍資料夾（回到「未選擇」）。
func (s *AppService) ClearRangesDir() Result {
	return saved(s.update(func(st *config.Settings) { st.RangesDir = "" }))
}

// Messages 回傳前端介面文字（ui 命名空間）；ui.json 不存在時回空 map。
func (s *AppService) Messages(lang string) map[string]string {
	if !i18n.IsSupported(lang) {
		lang = i18n.Normalize(lang)
	}
	out := map[string]string{}
	raw, err := fs.ReadFile(i18n.Locales(), path.Join(lang, uiNS+".json"))
	if err != nil {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		log.Println("messages:", err)
		return map[string]string{}
	}
	return out
}
