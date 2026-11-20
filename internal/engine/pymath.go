package engine

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// pySum is Python 3.12's sum() over floats: CPython Python/bltinmodule.c builtin_sum_impl,
// float path (Neumaier compensated summation), starting from the int 0.
// Products that feed it must be wrapped in float64(...) by the caller so that arm64 cannot fuse
// them into a multiply-add, which would change the last bit.
func pySum(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	f := 0.0 + values[0]
	c := 0.0
	for _, x := range values[1:] {
		t := f + x
		if math.Abs(f) >= math.Abs(x) {
			c += (f - t) + x
		} else {
			c += (x - t) + f
		}
		f = t
	}
	if c != 0 && !math.IsInf(c, 0) && !math.IsNaN(c) {
		f += c
	}
	return f
}

// pyMin is Python's min(a, b): b only when it is strictly smaller.
func pyMin(a, b float64) float64 {
	if b < a {
		return b
	}
	return a
}

// isPySpace is str.isspace for one character (includes U+001C..U+001F, excludes U+200B).
func isPySpace(r rune) bool {
	return unicode.IsSpace(r) || (0x1c <= r && r <= 0x1f)
}

func pyStrip(s string) string  { return strings.TrimFunc(s, isPySpace) }
func pyRStrip(s string) string { return strings.TrimRightFunc(s, isPySpace) }

// pySplit is str.split() without arguments.
func pySplit(s string) []string { return strings.FieldsFunc(s, isPySpace) }

// decimalDigit is unicodedata.decimal for the Nd characters.
func decimalDigit(r rune) (int, bool) {
	if !unicode.Is(unicode.Nd, r) {
		return 0, false
	}
	for _, rg := range unicode.Nd.R16 {
		if lo, hi := rune(rg.Lo), rune(rg.Hi); lo <= r && r <= hi {
			return int(r-lo) % 10, true
		}
	}
	for _, rg := range unicode.Nd.R32 {
		if lo, hi := rune(rg.Lo), rune(rg.Hi); lo <= r && r <= hi {
			return int(r-lo) % 10, true
		}
	}
	return 0, false
}

// normalizeFloatText mirrors _PyUnicode_TransformDecimalAndSpaceToASCII: characters above U+007E
// become a space (if str.isspace), the ASCII digit of a decimal digit, or make the text invalid.
func normalizeFloatText(s string) (string, bool) {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r < 127:
			sb.WriteRune(r)
		case isPySpace(r):
			sb.WriteByte(' ')
		default:
			d, ok := decimalDigit(r)
			if !ok {
				return "", false
			}
			sb.WriteByte(byte('0' + d))
		}
	}
	return sb.String(), true
}

func isASCIIDigit(b byte) bool { return '0' <= b && b <= '9' }

// removeUnderscores applies PEP 515: an underscore is only allowed between two digits.
func removeUnderscores(s string) (string, bool) {
	if !strings.Contains(s, "_") {
		return s, true
	}
	var sb strings.Builder
	var prev byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			if !isASCIIDigit(prev) {
				return "", false
			}
		} else {
			if prev == '_' && !isASCIIDigit(c) {
				return "", false
			}
			sb.WriteByte(c)
		}
		prev = c
	}
	if prev == '_' {
		return "", false
	}
	return sb.String(), true
}

func isFloatSyntax(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := func() int {
		start := i
		for i < len(s) && isASCIIDigit(s[i]) {
			i++
		}
		return i - start
	}
	whole := digits()
	frac := 0
	if i < len(s) && s[i] == '.' {
		i++
		frac = digits()
	}
	if whole+frac == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if digits() == 0 {
			return false
		}
	}
	return i == len(s)
}

// parsePyFloat is Python's float(str): surrounding ASCII / Unicode whitespace, underscores between digits,
// inf / infinity / nan in any case with an optional sign, Unicode decimal digits; hex is rejected.
// A value too large for a float64 gives +-Inf (the caller's isfinite check rejects it, as in Python).
func parsePyFloat(s string) (float64, bool) {
	text, ok := normalizeFloatText(s)
	if !ok {
		return 0, false
	}
	text = strings.Trim(text, " \t\n\v\f\r")
	text, ok = removeUnderscores(text)
	if !ok {
		return 0, false
	}
	body, sign := text, 1.0
	if strings.HasPrefix(body, "+") {
		body = body[1:]
	} else if strings.HasPrefix(body, "-") {
		body, sign = body[1:], -1
	}
	switch strings.ToLower(body) {
	case "inf", "infinity":
		return math.Inf(int(sign)), true
	case "nan":
		return math.NaN(), true
	}
	if !isFloatSyntax(text) {
		return 0, false
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil && !math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// pyRepr is repr(str).
func pyRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = '"'
	}
	var sb strings.Builder
	sb.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote) || r == '\\':
			sb.WriteByte('\\')
			sb.WriteRune(r)
		case r == '\t':
			sb.WriteString(`\t`)
		case r == '\n':
			sb.WriteString(`\n`)
		case r == '\r':
			sb.WriteString(`\r`)
		case unicode.IsPrint(r):
			sb.WriteRune(r)
		case r < 0x100:
			sb.WriteString(`\x` + hex2(uint32(r), 2))
		case r < 0x10000:
			sb.WriteString(`\u` + hex2(uint32(r), 4))
		default:
			sb.WriteString(`\U` + hex2(uint32(r), 8))
		}
	}
	sb.WriteByte(quote)
	return sb.String()
}

func hex2(v uint32, width int) string {
	h := strconv.FormatUint(uint64(v), 16)
	return strings.Repeat("0", width-len(h)) + h
}
