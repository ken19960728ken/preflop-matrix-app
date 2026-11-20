package main

import (
	"embed"
	"os"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/cli"
	"github.com/ken19960728ken/preflop-matrix-app/internal/gui"
)

//go:embed all:frontend/dist
var assets embed.FS

// macOS 從 Finder 啟動時可能附帶 -psn_ 開頭的參數，不算命令列參數。
const finderArgPrefix = "-psn_"

func main() {
	var args []string
	for _, a := range os.Args[1:] {
		if !strings.HasPrefix(a, finderArgPrefix) {
			args = append(args, a)
		}
	}
	if len(args) > 0 {
		os.Exit(cli.Run(args))
	}
	os.Exit(gui.Run(assets))
}
