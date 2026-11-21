package gui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
)

var keyLiteral = regexp.MustCompile(`"((?:gui|report)\.[A-Za-z0-9_.]*[A-Za-z0-9_])"`)

// 程式碼裡用到的 gui.* 與 report.* 訊息 key 在三個語言都必須存在。
func TestEveryMessageKeyExistsInAllLanguages(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range keyLiteral.FindAllStringSubmatch(string(src), -1) {
			keys[m[1]] = true
		}
	}
	delete(keys, "report.html") // 暫存報告的檔名，不是訊息 key
	for _, node := range []string{"open", "vs_open", "vs_3bet", "vs_4bet"} {
		keys["report.dev.node."+node] = true
	}
	if len(keys) < 25 {
		t.Fatalf("found only %d keys, the scan is broken", len(keys))
	}
	for key := range keys {
		for _, lang := range i18n.Languages {
			if got := i18n.T(lang, key, nil); got == key {
				t.Errorf("key %q missing for %s", key, lang)
			}
		}
	}
}
