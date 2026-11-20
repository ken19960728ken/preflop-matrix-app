package main

import (
	"embed"
	"os"

	"github.com/ken19960728ken/preflop-matrix-app/internal/cli"
	"github.com/ken19960728ken/preflop-matrix-app/internal/gui"
)

//go:embed all:frontend/dist
var assets embed.FS

// GUI App 只在第一個參數是已知 CLI 旗標時走命令列；其餘（-psn_*、-AppleLanguages、-NS*、檔案關聯）一律開視窗。
// 純 console 入口見 cmd/preflop-matrix。
func main() {
	args := os.Args[1:]
	if cli.IsCLIArgs(args) {
		os.Exit(cli.Run(args))
	}
	os.Exit(gui.Run(assets))
}
