//go:build !windows

package gui

import (
	"io/fs"
	"os"
	"syscall"
)

// ownedByCurrentUser 回報檔案是否屬於目前使用者。
func ownedByCurrentUser(info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}
