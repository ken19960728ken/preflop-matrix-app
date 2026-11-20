package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func settingsFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "preflop-matrix", "settings.json")
}

func TestSettingsRoundTrip(t *testing.T) {
	path := settingsFile(t)
	want := Settings{Lang: "zh-Hans", HandsDir: "/h", RangesDir: "/r", Hero: "Ken", TopN: 0, LastSaveDir: "/s"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, warn := Load(path)
	if warn != nil || got != want {
		t.Errorf("got %+v warn=%v, want %+v", got, warn, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 && filepath.Separator == '/' {
		t.Errorf("settings file perm %o is too open", perm)
	}
}

func TestSaveOverwritesAndLeavesNoTempFiles(t *testing.T) {
	path := settingsFile(t)
	for _, hero := range []string{"A", "B"} {
		s := DefaultSettings()
		s.Hero = hero
		if err := Save(path, s); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := Load(path)
	if got.Hero != "B" {
		t.Errorf("hero = %q", got.Hero)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("expected only settings.json, got %v", entries)
	}
}

func TestLoadMissingFileGivesDefaultsWithoutWarning(t *testing.T) {
	got, warn := Load(settingsFile(t))
	if warn != nil || got != DefaultSettings() {
		t.Errorf("got %+v warn=%v", got, warn)
	}
}

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	path := settingsFile(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadCorruptFileIsBackedUpAndGivesDefaults(t *testing.T) {
	for name, content := range map[string]string{"garbage": "{not json", "empty": "", "array": "[1]", "null": "null"} {
		path := writeSettings(t, content)
		got, warn := Load(path)
		if !errors.Is(warn, ErrCorruptSettings) || got != DefaultSettings() {
			t.Errorf("%s: got %+v warn=%v", name, got, warn)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: original should be moved away: %v", name, err)
		}
		backups, _ := filepath.Glob(path + ".corrupt-*")
		if len(backups) != 1 {
			t.Fatalf("%s: backups = %v", name, backups)
		}
		if saved, _ := os.ReadFile(backups[0]); string(saved) != content {
			t.Errorf("%s: backup content %q", name, saved)
		}
		if _, warn := Load(path); warn != nil {
			t.Errorf("%s: second load should be quiet: %v", name, warn)
		}
	}
}

func TestLoadCorruptBackupFailureIsReported(t *testing.T) {
	path := writeSettings(t, "{bad")
	// 備份目標是既有的非空資料夾 → rename 失敗
	orig := timeNow
	timeNow = func() int64 { return 42 }
	t.Cleanup(func() { timeNow = orig })
	if err := os.MkdirAll(filepath.Join(path+".corrupt-42", "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, warn := Load(path)
	if !errors.Is(warn, ErrCorruptSettings) || got != DefaultSettings() {
		t.Fatalf("got %+v warn=%v", got, warn)
	}
	if warn.Error() == "" || len(warn.Error()) < 30 {
		t.Errorf("warning should mention the rename failure: %v", warn)
	}
}

func TestLoadKeepsGoodFieldsWhenOneFieldIsBad(t *testing.T) {
	path := writeSettings(t, `{"top_n":"x","hands_dir":"/a","hero":"Ken","lang":"zh-TW"}`)
	got, warn := Load(path)
	want := DefaultSettings()
	want.HandsDir, want.Hero, want.Lang = "/a", "Ken", "zh-Hant"
	if !errors.Is(warn, ErrCorruptSettings) || got != want {
		t.Errorf("got %+v warn=%v, want %+v", got, warn, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("a file with only bad fields must not be moved: %v", err)
	}
}

func TestLoadUnreadablePathGivesDefaultsAndWarning(t *testing.T) {
	dir := t.TempDir() // 是資料夾，ReadFile 會失敗
	got, warn := Load(dir)
	if warn == nil || errors.Is(warn, ErrCorruptSettings) || got != DefaultSettings() {
		t.Errorf("got %+v warn=%v", got, warn)
	}
}

func TestLoadMissingAndUnknownFields(t *testing.T) {
	path := writeSettings(t, `{"lang":"en","future_field":{"a":1}}`)
	got, warn := Load(path)
	want := DefaultSettings()
	want.Lang = "en"
	if warn != nil || got != want {
		t.Errorf("got %+v warn=%v, want %+v", got, warn, want)
	}
}

func TestLoadNormalizesLangAndDefaultsBlankHero(t *testing.T) {
	for in, want := range map[string]string{"zh-TW": "zh-Hant", "zh_CN": "zh-Hans", "en-US": "en", "": ""} {
		path := writeSettings(t, `{"lang":"`+in+`","hero":" \t "}`)
		got, warn := Load(path)
		if warn != nil || got.Lang != want || got.Hero != DefaultHero {
			t.Errorf("lang %q: got %+v warn=%v", in, got, warn)
		}
	}
}

func TestLoadTopNOutOfRangeFallsBackWithWarning(t *testing.T) {
	for _, v := range []string{"99", "-1"} {
		path := writeSettings(t, `{"top_n":`+v+`,"hands_dir":"/keep"}`)
		got, warn := Load(path)
		if !errors.Is(warn, ErrCorruptSettings) || got.TopN != DefaultTopN || got.HandsDir != "/keep" {
			t.Errorf("top_n %s: got %+v warn=%v", v, got, warn)
		}
	}
	path := writeSettings(t, `{"top_n":20}`)
	if got, warn := Load(path); warn != nil || got.TopN != 20 {
		t.Errorf("20 is valid: %+v %v", got, warn)
	}
}

func TestSaveCreatesDirWith0700(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions only")
	}
	path := filepath.Join(t.TempDir(), "new", "preflop-matrix", "settings.json")
	if err := Save(path, DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}
}

func TestSaveFailsWhenParentIsAFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(blocker, "sub", "settings.json"), DefaultSettings()); err == nil {
		t.Error("expected error")
	}
}

func TestSaveFailsWhenTargetIsADirectoryAndCleansTemp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.MkdirAll(filepath.Join(target, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Save(target, DefaultSettings()); err == nil {
		t.Error("expected rename error")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp file left behind: %v", entries)
	}
}

func TestSettingsPathIsUnderUserConfigDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	p, err := SettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "settings.json" || filepath.Base(filepath.Dir(p)) != "preflop-matrix" {
		t.Errorf("unexpected path %q", p)
	}
}
