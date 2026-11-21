package guard

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Position names are data of the table format: only internal/engine/positions.go (and tests, locales,
// testdata, generated bindings, dependencies and build output) may spell them.
var positionToken = regexp.MustCompile(`(^|[^A-Za-z0-9_])(UTG|HJ|CO|BTN|SB|BB)([^A-Za-z0-9_]|$)`)

var (
	skipDirs  = map[string]bool{".git": true, "node_modules": true, "testdata": true, "bindings": true, "dist": true, "build": true, "locales": true}
	textExts  = map[string]bool{".ts": true, ".js": true, ".html": true, ".css": true}
	allowedGo = "internal/engine/positions.go"
	textRoots = []string{"frontend", "internal"}
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// goStringLiterals returns the string and character literals of Go source (comments are not literals).
func goStringLiterals(path string, src []byte) []string {
	fset := token.NewFileSet()
	file := fset.AddFile(path, fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, 0)
	var out []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.STRING || tok == token.CHAR {
			out = append(out, lit)
		}
	}
}

// violations lists the guarded files of the tree that spell a position name.
func violations(t *testing.T, root string) (offences []string, scanned int) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(mustRel(t, root, p))
		ext := filepath.Ext(p)
		isGo := ext == ".go" && !strings.HasSuffix(p, "_test.go") && rel != allowedGo
		isText := textExts[ext] && underAny(rel, textRoots)
		if !isGo && !isText {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		scanned++
		if isGo {
			for _, lit := range goStringLiterals(p, src) {
				if positionToken.MatchString(lit) {
					offences = append(offences, rel+": "+lit)
				}
			}
		} else if loc := positionToken.FindIndex(src); loc != nil {
			offences = append(offences, rel+": "+string(src[loc[0]:loc[1]]))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return offences, scanned
}

func mustRel(t *testing.T, root, p string) string {
	t.Helper()
	rel, err := filepath.Rel(root, p)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func underAny(rel string, roots []string) bool {
	for _, r := range roots {
		if strings.HasPrefix(rel, r+"/") {
			return true
		}
	}
	return false
}

func TestNoPositionLiteralsOutsidePositionsGo(t *testing.T) {
	offences, scanned := violations(t, repoRoot(t))
	if scanned == 0 {
		t.Fatal("nothing scanned")
	}
	for _, o := range offences {
		t.Errorf("position name outside positions.go: %s", o)
	}
}

func TestGuardFindsViolationsInEveryFileKind(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module x\n")
	write("a.go", "package a\nvar x = \"BTN\" // CO in a comment is fine\n")
	write("internal/engine/positions.go", "package engine\nvar x = \"BTN\"\n")
	write("internal/b_test.go", "package b\nvar x = \"BTN\"\n")
	write("frontend/src/a.ts", "const p = 'UTG'\n")
	write("frontend/src/a.js", "const p = 'HJ'\n")
	write("internal/report/t.html", "<td>SB</td>")
	write("internal/report/t.css", "/* BB */")
	write("internal/i18n/locales/en/x.js", "BB")
	write("frontend/node_modules/p/x.js", "BB")
	write("frontend/bindings/x.ts", "BB")
	write("frontend/dist/x.js", "BB")
	write("other/x.js", "BB")
	write("clean.go", "package c\nvar x = \"COMMON\" // BB\n")
	offences, scanned := violations(t, root)
	if scanned != 6 || len(offences) != 5 {
		t.Errorf("scanned %d, offences %v", scanned, offences)
	}
}

func TestPositionTokenBoundaries(t *testing.T) {
	for in, want := range map[string]bool{
		`"UTG"`: true, `"BB vs UTG open"`: true, `"UTG|HJ"`: true, `"CO>BTN"`: true, "SB": true,
		`"COMMON"`: false, `"CO2"`: false, `"sbb"`: false, `"BBQ"`: false, `"a_BB"`: false, `"hjk"`: false, `""`: false,
	} {
		if positionToken.MatchString(in) != want {
			t.Errorf("%s: want %v", in, want)
		}
	}
}
