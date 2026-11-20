// paritydiff-cli 比對兩份 JSON（Python golden 與 Go 輸出）。
//
//	go run ./tools/paritydiff-cli a.json b.json
//	go run ./tools/paritydiff-cli --summary a.stdout.txt b.stdout.txt   # 分類摘要（stdout）逐行比對
//
// exit 0：完全一致；1：有差異（逐行印出）；2：參數或檔案錯誤。
package main

import (
	"fmt"
	"os"

	"github.com/ken19960728ken/preflop-matrix-app/internal/paritydiff"
)

const maxPrinted = 200

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	summary := len(args) > 0 && args[0] == "--summary"
	if summary {
		args = args[1:]
	}
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: paritydiff-cli [--summary] a b")
		return 2
	}
	a, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	b, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if summary {
		return report(paritydiff.CompareSummary(string(a), string(b)))
	}
	diffs, err := paritydiff.Compare(a, b)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	lines := make([]string, len(diffs))
	for i, d := range diffs {
		lines[i] = d.String()
	}
	return report(lines)
}

func report(diffs []string) int {
	for i, d := range diffs {
		if i == maxPrinted {
			fmt.Printf("... %d more\n", len(diffs)-maxPrinted)
			break
		}
		fmt.Println(d)
	}
	if len(diffs) > 0 {
		fmt.Printf("%d difference(s)\n", len(diffs))
		return 1
	}
	fmt.Println("identical")
	return 0
}
