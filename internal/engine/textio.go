package engine

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	bom             = []byte{0xEF, 0xBB, 0xBF}
	errInvalidUTF8  = errors.New("invalid utf-8")
	utf8Replacement = string(utf8.RuneError)
)

// invalidRunLen is the length of the maximal subpart of an ill-formed UTF-8 sequence starting at b[0]
// (Unicode "best practice" U+FFFD substitution, the rule CPython's errors="replace" follows).
func invalidRunLen(b []byte) int {
	lo, hi := byte(0x80), byte(0xBF)
	var need int
	switch c := b[0]; {
	case 0xC2 <= c && c <= 0xDF:
		need = 1
	case c == 0xE0:
		need, lo = 2, 0xA0
	case c == 0xED:
		need, hi = 2, 0x9F
	case 0xE1 <= c && c <= 0xEF:
		need = 2
	case c == 0xF0:
		need, lo = 3, 0x90
	case c == 0xF4:
		need, hi = 3, 0x8F
	case 0xF1 <= c && c <= 0xF3:
		need = 3
	default:
		return 1
	}
	n := 1
	for n <= need && n < len(b) && lo <= b[n] && b[n] <= hi {
		lo, hi = 0x80, 0xBF
		n++
	}
	return n
}

// decodeReplace is bytes.decode("utf-8", errors="replace"): one U+FFFD per maximal ill-formed subpart.
func decodeReplace(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			sb.WriteString(utf8Replacement)
			i += invalidRunLen(b[i:])
			continue
		}
		sb.Write(b[i : i+size])
		i += size
	}
	return sb.String()
}

// universalNewlines turns "\r\n" and a lone "\r" into "\n", as Python's text mode does.
func universalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// decodeHandText is read_text(encoding="utf-8-sig", errors="replace"): one BOM removed, bad bytes replaced.
func decodeHandText(b []byte) string {
	return universalNewlines(decodeReplace(bytes.TrimPrefix(b, bom)))
}

// decodeRangeText is read_text(encoding="utf-8-sig"): one BOM removed, any invalid byte is an error.
func decodeRangeText(b []byte) (string, error) {
	b = bytes.TrimPrefix(b, bom)
	if !utf8.Valid(b) {
		return "", errInvalidUTF8
	}
	return universalNewlines(string(b)), nil
}

// readHandFile reads a hand history file.
func readHandFile(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	return decodeHandText(b), nil
}

// readRangeFile reads a range file.
func readRangeFile(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	return decodeRangeText(b)
}

// txtFiles is sorted(root.rglob("*.txt")): every entry whose name ends in .txt (folders and symlinks too),
// never descending into a symlink, in path-component order.
func txtFiles(ctx context.Context, fsys fs.FS) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if err != nil || p == "." {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".txt") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return pathLess(out[i], out[j]) })
	return out, nil
}

// pathLess compares path component by component (case-sensitive, by code point), like sorted(Path).
func pathLess(a, b string) bool {
	pa, pb := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}

// baseName is the last path component.
func baseName(p string) string {
	return p[strings.LastIndexByte(p, '/')+1:]
}

// fileStem is Path.stem of a name that ends in ".txt".
func fileStem(name string) string {
	if name == ".txt" {
		return name
	}
	return strings.TrimSuffix(name, ".txt")
}
