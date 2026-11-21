package engine

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
)

func messageKeys(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "messages.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, _ := strconv.Unquote(lit.Value); strings.HasPrefix(s, "engine.") {
				keys = append(keys, s)
			}
		}
		return true
	})
	return keys
}

func TestEveryMessageKeyIsTranslated(t *testing.T) {
	keys := messageKeys(t)
	if len(keys) != 21 {
		t.Fatalf("%d message keys", len(keys))
	}
	for _, lang := range i18n.Languages {
		for _, k := range keys {
			if i18n.T(lang, k, nil) == k {
				t.Errorf("%s: %s not translated", lang, k)
			}
		}
	}
}

func TestMatrixMessagesMatchPythonInZhHant(t *testing.T) {
	for _, m := range []Message{
		newMsg(MsgMatrixMissingAction, "node", "vs_3bet", "cell", "UTG>BB", "action", "4bet"),
		newMsg(MsgMatrixTheoryOverOne, "node", "vs_open", "cell", "UTG>BB", "total", "1.2500"),
	} {
		if got, want := i18n.T(i18n.ZhHant, m.Key, m.Args), pyText(m); got != want {
			t.Errorf("%s: %q != %q", m.Key, got, want)
		}
	}
}

// extraArgs are the placeholders of the keys that are new in Go (no Python text in pyTemplates).
var extraArgs = map[string][]string{
	MsgParseBadSeat:       {"seat"},
	MsgLoadFileUnreadable: {"reason"},
	MsgRangeInvalidUTF8:   nil,
}

var placeholderRE = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

func placeholderSet(s string) []string {
	var out []string
	for _, m := range placeholderRE.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func TestLocaleFilesHoldExactlyTheMessageKeys(t *testing.T) {
	keys := messageKeys(t)
	want := append([]string(nil), keys...)
	sort.Strings(want)
	for _, lang := range i18n.Languages {
		raw, err := os.ReadFile(filepath.Join("..", "i18n", "locales", lang, "engine.json"))
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatal(err)
		}
		var got []string
		for k, v := range texts {
			got = append(got, k)
			expected, ok := extraArgs[k]
			if !ok {
				expected = placeholderSet(pyTemplates[k])
			}
			if g := placeholderSet(v); !reflect.DeepEqual(append([]string(nil), g...), append([]string(nil), expected...)) && !(len(g) == 0 && len(expected) == 0) {
				t.Errorf("%s %s: placeholders %v, want %v", lang, k, g, expected)
			}
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: keys differ from messages.go: %v", lang, got)
		}
	}
}
