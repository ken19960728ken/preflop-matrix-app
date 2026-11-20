package paritydiff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// specialsFile 是 posix_extras 語料裡 specials/specials.json 的格式：
// repo 內不能原樣存放的東西（檔名含 tab、symlink）以佔位檔名存放，使用前由 Materialize 還原。
type specialsFile struct {
	Rename []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"rename"`
	Symlink []struct {
		Path   string `json:"path"`
		Target string `json:"target"`
	} `json:"symlink"`
}

// Materialize 把語料資料夾 src 複製到 dst，再套用 src/specials/specials.json（改名、建 symlink）。
// 路徑皆相對於語料資料夾，用 / 分隔。Python 端對應實作：tools/parity/materialize_extras.py。
func Materialize(src, dst string) error {
	data, err := os.ReadFile(filepath.Join(src, "specials", "specials.json"))
	if err != nil {
		return err
	}
	var sp specialsFile
	if err := json.Unmarshal(data, &sp); err != nil {
		return fmt.Errorf("specials.json: %w", err)
	}
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		return err
	}
	for _, r := range sp.Rename {
		if err := os.Rename(filepath.Join(dst, filepath.FromSlash(r.From)), filepath.Join(dst, filepath.FromSlash(r.To))); err != nil {
			return err
		}
	}
	for _, l := range sp.Symlink {
		link := filepath.Join(dst, filepath.FromSlash(l.Path))
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(filepath.FromSlash(l.Target), link); err != nil {
			return err
		}
	}
	return nil
}
