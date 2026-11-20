package paritydiff

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Kind 是差異的種類。
type Kind string

const (
	KindValue      Kind = "value"        // 同型別但值不同（數值以 float64 完全相等為準）
	KindType       Kind = "type"         // 型別不同（含 null 對非 null）
	KindMissingInA Kind = "missing_in_a" // key（或陣列元素）只存在於 b
	KindMissingInB Kind = "missing_in_b" // key（或陣列元素）只存在於 a
)

// Diff 描述一處差異。Path 是 JSON path（$.open.UTG.th.r、$.vs_3bet['CO>BTN'].th.c、$.meta.warnings[0]）。
// A、B 是兩邊的值（緊湊 JSON，過長截斷）；缺的那一邊為空字串。
type Diff struct {
	Path string
	Kind Kind
	A, B string
}

func (d Diff) String() string {
	switch d.Kind {
	case KindMissingInA:
		return fmt.Sprintf("%s: %s only in b: %s", d.Path, d.Kind, d.B)
	case KindMissingInB:
		return fmt.Sprintf("%s: %s only in a: %s", d.Path, d.Kind, d.A)
	}
	return fmt.Sprintf("%s: %s a=%s b=%s", d.Path, d.Kind, d.A, d.B)
}

// Compare 比對兩份 JSON。數值以 float64 完全相等（0 容差；-0 == 0、1 == 1.0）；
// 物件 key 集合必須相同；null 與缺 key 是不同的差異；字串與布林完全相等。
// 任何一份不是合法 JSON（含尾端多餘內容、NaN/Infinity、超出 float64 的數字）回傳 error。
func Compare(a, b []byte) ([]Diff, error) {
	va, err := decode(a)
	if err != nil {
		return nil, fmt.Errorf("a: %w", err)
	}
	vb, err := decode(b)
	if err != nil {
		return nil, fmt.Errorf("b: %w", err)
	}
	var diffs []Diff
	compare("$", va, vb, &diffs)
	return diffs, nil
}

func decode(data []byte) (any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("empty input")
	}
	if !utf8.Valid(data) {
		return nil, errors.New("input is not valid UTF-8")
	}
	if err := rejectSurrogateEscapes(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec, "$")
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the top-level JSON value")
	}
	return v, nil
}

// parseValue 以 Token 走訪建樹，偵測重複 key，並檢查每個數字都能放進 float64。
func parseValue(dec *json.Decoder, path string) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Number:
		if _, err := strconv.ParseFloat(t.String(), 64); err != nil {
			return nil, fmt.Errorf("number %s at %s: %w", t, path, err)
		}
		return t, nil
	case json.Delim:
		if t == '[' {
			arr := []any{}
			for dec.More() {
				e, err := parseValue(dec, fmt.Sprintf("%s[%d]", path, len(arr)))
				if err != nil {
					return nil, err
				}
				arr = append(arr, e)
			}
			_, err := dec.Token()
			return arr, err
		}
		obj := map[string]any{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			k := kt.(string)
			if _, dup := obj[k]; dup {
				return nil, fmt.Errorf("duplicate key %q in object at %s", k, path)
			}
			e, err := parseValue(dec, path+keyPath(k))
			if err != nil {
				return nil, err
			}
			obj[k] = e
		}
		_, err = dec.Token()
		return obj, err
	}
	return tok, nil
}

// rejectSurrogateEscapes 拒絕字串裡的 \ud800-\udfff escape（Go 會悄悄換成 U+FFFD，Python 則保留孤立代理）。
func rejectSurrogateEscapes(data []byte) error {
	inString := false
	for i := 0; i < len(data); i++ {
		switch c := data[i]; {
		case !inString:
			inString = c == '"'
		case c == '"':
			inString = false
		case c == '\\':
			if i+5 < len(data) && data[i+1] == 'u' && (data[i+2] == 'd' || data[i+2] == 'D') && isSurrogateHex(data[i+3]) {
				return fmt.Errorf("surrogate escape %s at byte %d", data[i:i+6], i)
			}
			i++
		}
	}
	return nil
}

func isSurrogateHex(c byte) bool {
	return c >= '8' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case json.Number:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	default:
		return "object"
	}
}

func compare(path string, a, b any, out *[]Diff) {
	if typeName(a) != typeName(b) {
		*out = append(*out, Diff{path, KindType, show(a), show(b)})
		return
	}
	switch av := a.(type) {
	case json.Number:
		bn := b.(json.Number)
		if !numbersEqual(av, bn) {
			*out = append(*out, Diff{path, KindValue, av.String(), bn.String()})
		}
	case []any:
		bv := b.([]any)
		for i := 0; i < len(av) || i < len(bv); i++ {
			p := fmt.Sprintf("%s[%d]", path, i)
			switch {
			case i >= len(bv):
				*out = append(*out, Diff{p, KindMissingInB, show(av[i]), ""})
			case i >= len(av):
				*out = append(*out, Diff{p, KindMissingInA, "", show(bv[i])})
			default:
				compare(p, av[i], bv[i], out)
			}
		}
	case map[string]any:
		bv := b.(map[string]any)
		for _, k := range sortedKeys(av, bv) {
			p := path + keyPath(k)
			x, inA := av[k]
			y, inB := bv[k]
			switch {
			case !inB:
				*out = append(*out, Diff{p, KindMissingInB, show(x), ""})
			case !inA:
				*out = append(*out, Diff{p, KindMissingInA, "", show(y)})
			default:
				compare(p, x, y, out)
			}
		}
	default:
		if a != b {
			*out = append(*out, Diff{path, KindValue, show(a), show(b)})
		}
	}
}

var integerLiteral = regexp.MustCompile(`^-?[0-9]+$`)

// numbersEqual：兩邊都是整數字面量時用 big.Int 精確比；否則以 float64 完全相等（-0 == 0、1 == 1.0）。
func numbersEqual(a, b json.Number) bool {
	if integerLiteral.MatchString(a.String()) && integerLiteral.MatchString(b.String()) {
		x, _ := new(big.Int).SetString(a.String(), 10)
		y, _ := new(big.Int).SetString(b.String(), 10)
		return x.Cmp(y) == 0
	}
	fa, _ := strconv.ParseFloat(a.String(), 64)
	fb, _ := strconv.ParseFloat(b.String(), 64)
	return fa == fb
}

func sortedKeys(a, b map[string]any) []string {
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func keyPath(k string) string {
	if identifier.MatchString(k) {
		return "." + k
	}
	return "['" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(k) + "']"
}

const maxShown = 80

func show(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	s := strings.TrimSuffix(buf.String(), "\n")
	if r := []rune(s); len(r) > maxShown {
		return string(r[:maxShown]) + "…"
	}
	return s
}
