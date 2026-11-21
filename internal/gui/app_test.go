package gui

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
)

func TestGetStateDefaultsAndSystemLanguage(t *testing.T) {
	e := newEnv(t, nil)
	st := e.svc.GetState()
	want := State{Lang: "zh-Hant", LangSource: "system", Hero: "Hero", TopN: 5, Version: Version}
	if st != want {
		t.Errorf("state = %+v, want %+v", st, want)
	}
}

func TestSetLanguage(t *testing.T) {
	e := newEnv(t, nil)
	if r := e.svc.SetLanguage("fr"); r.OK || r.Error == "" {
		t.Errorf("unsupported language accepted: %+v", r)
	}
	if r := e.svc.SetLanguage("zh-TW"); r.OK {
		t.Errorf("non-canonical tag accepted: %+v", r)
	}
	if r := e.svc.SetLanguage("en"); !r.OK || r.Error != "" {
		t.Fatalf("SetLanguage(en) = %+v", r)
	}
	if st := e.svc.GetState(); st.Lang != "en" || st.LangSource != "saved" {
		t.Errorf("state = %+v", st)
	}
	if got := e.settings(t).Lang; got != "en" {
		t.Errorf("saved lang = %q", got)
	}
	if r := e.svc.SetLanguage("xx"); !strings.Contains(r.Error, "Unsupported language: xx") {
		t.Errorf("error is not in the current language: %q", r.Error)
	}
}

func TestSetHero(t *testing.T) {
	e := newEnv(t, nil)
	if r := e.svc.SetHero("  Ken  "); !r.OK {
		t.Fatal(r)
	}
	if got := e.svc.GetState().Hero; got != "Ken" {
		t.Errorf("hero = %q", got)
	}
	e.svc.SetHero("   ")
	if got := e.svc.GetState().Hero; got != config.DefaultHero {
		t.Errorf("blank hero = %q, want default", got)
	}
}

func TestSetTopNValidation(t *testing.T) {
	e := newEnv(t, nil)
	for _, n := range []int{-1, 21} {
		if r := e.svc.SetTopN(n); r.OK || !strings.Contains(r.Error, "20") {
			t.Errorf("SetTopN(%d) = %+v", n, r)
		}
	}
	if r := e.svc.SetTopN(0); !r.OK {
		t.Fatal(r)
	}
	if got := e.svc.GetState().TopN; got != 0 {
		t.Errorf("topN = %d", got)
	}
	if r := e.svc.SetTopN(20); !r.OK || e.settings(t).TopN != 20 {
		t.Errorf("SetTopN(20) = %+v", r)
	}
}

func TestPickDirs(t *testing.T) {
	hands, ranges := t.TempDir(), t.TempDir()
	e := newEnv(t, nil)

	e.host.pickPath = ""
	if r := e.svc.PickHandsDir(); !r.Cancelled || r.OK || r.Error != "" {
		t.Errorf("cancelled pick = %+v", r)
	}
	if got := e.settings(t).HandsDir; got != "" {
		t.Errorf("cancel changed settings: %q", got)
	}

	e.host.pickErr = errors.New("boom")
	if r := e.svc.PickRangesDir(); r.OK || !strings.Contains(r.Error, "boom") {
		t.Errorf("dialog error = %+v", r)
	}
	e.host.pickErr = nil

	e.host.pickPath = hands
	if r := e.svc.PickHandsDir(); !r.OK || r.Path != hands {
		t.Errorf("PickHandsDir = %+v", r)
	}
	e.host.pickPath = ranges
	if r := e.svc.PickRangesDir(); !r.OK || r.Path != ranges {
		t.Errorf("PickRangesDir = %+v", r)
	}
	st := e.svc.GetState()
	if st.HandsDir != hands || st.Ranges != (RangesState{Path: ranges, Exists: true, Picked: true}) {
		t.Errorf("state = %+v", st)
	}
	if last := e.host.picks[len(e.host.picks)-1]; last.start != "" {
		t.Errorf("start dir for a first pick = %q", last.start)
	}
	e.svc.PickRangesDir()
	if last := e.host.picks[len(e.host.picks)-1]; last.start != ranges {
		t.Errorf("second pick should start at the saved folder, got %q", last.start)
	}

	if r := e.svc.ClearRangesDir(); !r.OK {
		t.Fatal(r)
	}
	if st := e.svc.GetState(); st.Ranges != (RangesState{}) {
		t.Errorf("after clear: %+v", st.Ranges)
	}
}

func TestStateRangesPickedButMissing(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	e := newEnv(t, func(st *config.Settings) { st.RangesDir = gone })
	if got := e.svc.GetState().Ranges; got != (RangesState{Path: gone, Picked: true}) {
		t.Errorf("ranges = %+v", got)
	}
}

func TestSettingsSaveFailureIsReported(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewAppService(Options{SettingsPath: filepath.Join(blocker, "settings.json"), DetectLang: func() string { return "en" }})
	for name, r := range map[string]Result{
		"SetLanguage": svc.SetLanguage("en"), "SetHero": svc.SetHero("x"), "SetTopN": svc.SetTopN(3), "Clear": svc.ClearRangesDir(),
	} {
		if r.OK || !strings.Contains(r.Error, "Cannot save settings") {
			t.Errorf("%s = %+v", name, r)
		}
	}
}

func TestSettingsMutexUnderConcurrency(t *testing.T) {
	e := newEnv(t, nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				e.svc.SetHero("h")
				e.svc.SetTopN(j)
				e.svc.SetLanguage("en")
				e.svc.ClearRangesDir()
				e.svc.GetState()
				e.svc.GetReportView()
			}
		}()
	}
	wg.Wait()
	st, err := config.Load(e.settingsPath)
	if err != nil {
		t.Fatalf("settings file damaged: %v", err)
	}
	if st.Hero != "h" || st.Lang != "en" {
		t.Errorf("settings = %+v", st)
	}
}

func TestMessagesReadsUINamespace(t *testing.T) {
	e := newEnv(t, nil)
	var all []map[string]string
	for _, lang := range i18n.Languages {
		m := e.svc.Messages(lang)
		if m == nil {
			t.Fatalf("Messages(%q) is nil", lang)
		}
		for k, v := range m {
			if !strings.HasPrefix(k, "ui.") || strings.TrimSpace(v) == "" {
				t.Errorf("Messages(%q): bad entry %q = %q", lang, k, v)
			}
		}
		all = append(all, m)
	}
	if len(all[0]) == 0 { // ui.json 還沒有（由前端任務加入）
		return
	}
	for i, m := range all[1:] {
		if len(m) != len(all[0]) {
			t.Errorf("%s has %d keys, %s has %d", i18n.Languages[i+1], len(m), i18n.Languages[0], len(all[0]))
		}
		for k := range all[0] {
			if _, ok := m[k]; !ok {
				t.Errorf("%s lacks %q", i18n.Languages[i+1], k)
			}
		}
	}
	if got := e.svc.Messages("zh-TW"); len(got) != len(all[0]) {
		t.Errorf("Messages(zh-TW) has %d keys", len(got))
	}
}

func TestSetHeroLengthLimit(t *testing.T) {
	e := newEnv(t, nil)
	if r := e.svc.SetHero(strings.Repeat("字", maxHeroRunes)); !r.OK {
		t.Errorf("64 characters rejected: %+v", r)
	}
	if r := e.svc.SetHero(strings.Repeat("字", maxHeroRunes+1)); r.OK || !strings.Contains(r.Error, "64") {
		t.Errorf("65 characters accepted: %+v", r)
	}
	if got := []rune(e.svc.GetState().Hero); len(got) != maxHeroRunes {
		t.Errorf("hero changed by the rejected call: %d runes", len(got))
	}
}

func TestCorruptSettingsWarningUntilNextSave(t *testing.T) {
	e := newEnv(t, nil)
	if err := os.MkdirAll(filepath.Dir(e.settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.settingsPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	st := e.svc.GetState()
	if st.Hero != "Hero" || !strings.Contains(st.SettingsWarning, "設定檔損毀") {
		t.Fatalf("state = %+v", st)
	}
	if again := e.svc.GetState(); again.SettingsWarning != st.SettingsWarning {
		t.Errorf("warning must persist until a save: %q", again.SettingsWarning)
	}
	e.svc.SetLanguage("en")
	if w := e.svc.GetState().SettingsWarning; w != "" {
		t.Errorf("warning after a successful save: %q", w)
	}
}

func TestBadFieldWarningIsLoggedOnce(t *testing.T) {
	e := newEnv(t, nil)
	if err := os.MkdirAll(filepath.Dir(e.settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.settingsPath, []byte(`{"top_n": "many", "hero": "Ken"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	for i := 0; i < 5; i++ {
		e.svc.GetState()
	}
	if n := strings.Count(buf.String(), "settings:"); n != 1 {
		t.Errorf("logged %d times:\n%s", n, buf.String())
	}
	if st := e.svc.GetState(); st.Hero != "Ken" || st.TopN != 5 || st.SettingsWarning == "" {
		t.Errorf("state = %+v", st)
	}
}

func TestUnreadableSettingsAreNotOverwritten(t *testing.T) {
	e := newEnv(t, nil)
	if err := os.MkdirAll(e.settingsPath, 0o700); err != nil { // 目錄無法當檔案讀：非損壞的讀取失敗
		t.Fatal(err)
	}
	for name, r := range map[string]Result{"SetHero": e.svc.SetHero("x"), "SetTopN": e.svc.SetTopN(2), "Language": e.svc.SetLanguage("en")} {
		if r.OK || r.Error == "" {
			t.Errorf("%s = %+v", name, r)
		}
	}
	if info, err := os.Stat(e.settingsPath); err != nil || !info.IsDir() {
		t.Errorf("settings path was replaced: %v", err)
	}
}

func TestVersionMatchesBuildConfig(t *testing.T) {
	raw, err := os.ReadFile("../../build/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^  version: "([^"]+)"`).FindSubmatch(raw)
	if m == nil || string(m[1]) != Version {
		t.Errorf("build/config.yml version %q, gui.Version %q", m, Version)
	}
}
