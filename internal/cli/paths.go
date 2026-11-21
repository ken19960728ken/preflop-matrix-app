package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
)

// resolveLoose 回傳絕對路徑並解開已存在部分的 symlink，不存在的尾段原樣接回（Python Path.resolve() 非嚴格模式）。
// 路徑不先 Clean：.. 要在 symlink 解開之後才由作業系統語意處理（link/../b.html 指向 link 目標的上一層）。
func resolveLoose(p string) string {
	if !filepath.IsAbs(p) {
		wd, err := os.Getwd()
		if err != nil {
			return filepath.Clean(p)
		}
		p = wd + string(os.PathSeparator) + p
	}
	return evalLoose(p)
}

func evalLoose(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	i := strings.LastIndexByte(p, os.PathSeparator)
	if i <= 0 {
		return p
	}
	parent, base := p[:i], p[i+1:]
	return filepath.Join(evalLoose(parent), base)
}

// shown 是路徑的顯示形式（同 Python Path 的正規化：去掉 . 與重複斜線，保留 ..）；實際讀寫用未正規化的字串。
func shown(p string) string { return config.CleanPath(p) }
