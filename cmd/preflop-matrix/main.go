// Command preflop-matrix 是純命令列入口（無 GUI 依賴的 console 程式）。
package main

import (
	"os"

	"github.com/ken19960728ken/preflop-matrix-app/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
