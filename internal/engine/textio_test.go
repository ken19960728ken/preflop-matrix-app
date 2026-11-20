package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDecodeReplaceMatchesPython(t *testing.T) {
	for _, v := range decodeReplaceVectors {
		if got := decodeReplace([]byte(v.in)); got != v.want {
			t.Errorf("decodeReplace(%q) = %q, want %q", v.in, got, v.want)
		}
	}
}

func TestDecodeHandText(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"one BOM removed":       {"\xEF\xBB\xBFabc", "abc"},
		"only one BOM removed":  {"\xEF\xBB\xBF\xEF\xBB\xBFabc", "\xEF\xBB\xBFabc"},
		"crlf":                  {"a\r\nb", "a\nb"},
		"lone cr":               {"a\rb\r\rc", "a\nb\n\nc"},
		"bad byte replaced":     {"a\xffb", "a�b"},
		"truncated BOM at end":  {"\xEF\xBB", "�"},
		"NEL is not a newline":  {"a\u0085b", "a\u0085b"},
		"line separator intact": {"a b", "a b"},
	} {
		if got := decodeHandText([]byte(tc.in)); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestDecodeRangeText(t *testing.T) {
	if got, err := decodeRangeText([]byte("\xEF\xBB\xBFAsAh: 1\r\nKsKh: 1\r")); err != nil || got != "AsAh: 1\nKsKh: 1\n" {
		t.Errorf("got %q, %v", got, err)
	}
	for _, bad := range []string{"\xff", "\xed\xa0\x80", "ok\xc0\xaf", "\xe2\x82"} {
		if _, err := decodeRangeText([]byte(bad)); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestPathLessComparesComponents(t *testing.T) {
	// sorted(Path): a/ < a b/ < a-x/, which a whole-string comparison would not give ("a b" < "a-x" < "a/y").
	paths := []string{"a b/x.txt", "a-x/x.txt", "a/x.txt", "a/y/z.txt", "B/x.txt", "a.txt"}
	want := []string{"B/x.txt", "a/x.txt", "a/y/z.txt", "a b/x.txt", "a-x/x.txt", "a.txt"}
	sorted := append([]string(nil), paths...)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if pathLess(sorted[j], sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if !reflect.DeepEqual(sorted, want) {
		t.Errorf("got %q, want %q", sorted, want)
	}
	if pathLess("a", "a") || !pathLess("a", "a/b") || pathLess("a/b", "a") {
		t.Error("a prefix comes first")
	}
}

func TestTxtFilesWalk(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a b/one.txt", "")
	write("a/two.txt", "")
	write("a-x/three.txt", "")
	write("UPPER.TXT", "")
	write("notes.md", "")
	write("dir.txt/inner.txt", "")
	write(".txt", "")
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, "linked.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(dir, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := os.Symlink(filepath.Join(elsewhere, "linked.txt"), filepath.Join(dir, "file_link.txt")); err != nil {
		t.Fatal(err)
	}
	got, _ := txtFiles(context.Background(), os.DirFS(dir))
	want := []string{".txt", "a/two.txt", "a b/one.txt", "a-x/three.txt", "dir.txt", "dir.txt/inner.txt", "file_link.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if text, err := readHandFile(os.DirFS(dir), "file_link.txt"); err != nil || text != "" {
		t.Errorf("a file symlink is read: %q %v", text, err)
	}
	if _, err := readHandFile(os.DirFS(dir), "dir.txt"); err == nil {
		t.Error("a folder named *.txt cannot be read")
	}
}

func TestFileStemAndBase(t *testing.T) {
	for in, want := range map[string]string{"a.txt": "a", "a.b.txt": "a.b", ".txt": ".txt", "..txt": ".", "UTG Open 2bb.txt": "UTG Open 2bb"} {
		if got := fileStem(in); got != want {
			t.Errorf("fileStem(%q) = %q, want %q", in, got, want)
		}
	}
	if baseName("a/b/c.txt") != "c.txt" || baseName("c.txt") != "c.txt" {
		t.Error("baseName")
	}
}
