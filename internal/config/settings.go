package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
)

const (
	DefaultHero = "Hero"
	DefaultTopN = 5
	MaxTopN     = 20
)

// Settings 是 GUI 記住的選擇。Lang 為空代表使用者還沒選過（改跟系統語言）。
type Settings struct {
	Lang        string `json:"lang"`
	HandsDir    string `json:"hands_dir"`
	RangesDir   string `json:"ranges_dir"`
	Hero        string `json:"hero"`
	TopN        int    `json:"top_n"`
	LastSaveDir string `json:"last_save_dir"`
}

// DefaultSettings 是沒有設定檔（或欄位缺漏）時的值。
func DefaultSettings() Settings {
	return Settings{Hero: DefaultHero, TopN: DefaultTopN}
}

// ErrCorruptSettings 表示設定檔存在但無法解析；Load 仍會回傳可用的預設值。
var ErrCorruptSettings = errors.New("settings file is corrupt")

// timeNow 回傳 unix 秒；測試可替換。
var timeNow = func() int64 { return time.Now().Unix() }

// SettingsPath 是 os.UserConfigDir()/preflop-matrix/settings.json。
func SettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "preflop-matrix", "settings.json"), nil
}

// Load 讀取 path。
//   - 檔案不存在：預設值、無 warning。
//   - 整檔不是 JSON 物件：先把原檔改名為 <path>.corrupt-<unix秒>，回傳預設值＋包著 ErrCorruptSettings 的 warning。
//   - 個別欄位型別錯誤或 top_n 超出 0..20：只有該欄位退回預設，其他欄位保留，warning 逐欄說明。
//   - 缺欄位補預設、未知欄位忽略；lang 先 Normalize（zh-TW→zh-Hant），空字串代表未選過；hero 空白退回預設。
//   - 其他讀取錯誤（權限…）：預設值＋該錯誤。
func Load(path string) (Settings, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return DefaultSettings(), err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		if err == nil {
			err = errors.New("not a JSON object")
		}
		warn := fmt.Errorf("%w: %v", ErrCorruptSettings, err)
		backup := fmt.Sprintf("%s.corrupt-%d", path, timeNow())
		if rerr := os.Rename(path, backup); rerr != nil {
			warn = errors.Join(warn, rerr)
		}
		return DefaultSettings(), warn
	}
	s := DefaultSettings()
	var bad []error
	for _, f := range []struct {
		key string
		dst any
	}{
		{"lang", &s.Lang}, {"hands_dir", &s.HandsDir}, {"ranges_dir", &s.RangesDir},
		{"hero", &s.Hero}, {"top_n", &s.TopN}, {"last_save_dir", &s.LastSaveDir},
	} {
		v, ok := fields[f.key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(v, f.dst); err != nil {
			bad = append(bad, fmt.Errorf("%w: field %s: %v", ErrCorruptSettings, f.key, err))
		}
	}
	if s.Lang != "" {
		s.Lang = i18n.Normalize(s.Lang)
	}
	if strings.TrimSpace(s.Hero) == "" {
		s.Hero = DefaultHero
	}
	if s.TopN < 0 || s.TopN > MaxTopN {
		s.TopN = DefaultTopN
		bad = append(bad, fmt.Errorf("%w: field top_n out of range 0..%d", ErrCorruptSettings, MaxTopN))
	}
	return s, errors.Join(bad...)
}

// Save 以暫存檔＋rename 寫入 path（目錄 0700、檔案 0600），中途失敗不會留下半個設定檔。
func Save(path string, s Settings) (err error) {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
