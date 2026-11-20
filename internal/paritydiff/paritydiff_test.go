package paritydiff

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustCompare(t *testing.T, a, b string) []Diff {
	t.Helper()
	diffs, err := Compare([]byte(a), []byte(b))
	if err != nil {
		t.Fatalf("Compare(%s, %s): %v", a, b, err)
	}
	return diffs
}

func TestCompareRules(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want []Diff
	}{
		{"identical", `{"a":1,"b":[1,2],"c":null}`, `{"c":null,"b":[1,2],"a":1}`, nil},
		{"key order is irrelevant", `{"a":1,"b":2}`, `{"b":2,"a":1}`, nil},
		{"negative zero equals zero", `{"x":-0.0}`, `{"x":0}`, nil},
		{"negative zero int form", `{"x":-0}`, `{"x":0.0}`, nil},
		{"1 equals 1.0", `{"x":1}`, `{"x":1.0}`, nil},
		{"exponent form equals", `{"x":1e0}`, `{"x":1}`, nil},
		{"zero tolerance", `{"x":0.1}`, `{"x":0.10000000000000002}`,
			[]Diff{{"$.x", KindValue, "0.1", "0.10000000000000002"}}},
		{"same float spelled differently", `{"x":0.30000000000000004}`, `{"x":3.0000000000000004e-1}`, nil},
		{"big integers compare exactly", `{"x":9007199254740993}`, `{"x":9007199254740992}`,
			[]Diff{{"$.x", KindValue, "9007199254740993", "9007199254740992"}}},
		{"big integers equal", `{"x":12345678901234567890123}`, `{"x":12345678901234567890123}`, nil},
		{"negative zero integer literal", `{"x":-0}`, `{"x":0}`, nil},
		{"integer vs float falls back to float64", `{"x":9007199254740993}`, `{"x":9007199254740992.0}`, nil},
		{"show does not HTML-escape", `{"s":"<a&b>"}`, `{"s":"c"}`, []Diff{{"$.s", KindValue, `"<a&b>"`, `"c"`}}},
		{"string differs", `{"s":"a"}`, `{"s":"b"}`, []Diff{{"$.s", KindValue, `"a"`, `"b"`}}},
		{"string vs number", `{"s":"1"}`, `{"s":1}`, []Diff{{"$.s", KindType, `"1"`, "1"}}},
		{"bool differs", `{"b":true}`, `{"b":false}`, []Diff{{"$.b", KindValue, "true", "false"}}},
		{"bool vs number", `{"b":true}`, `{"b":1}`, []Diff{{"$.b", KindType, "true", "1"}}},
		{"null vs number", `{"x":null}`, `{"x":0}`, []Diff{{"$.x", KindType, "null", "0"}}},
		{"null vs missing key (a has null)", `{"x":null}`, `{}`, []Diff{{"$.x", KindMissingInB, "null", ""}}},
		{"null vs missing key (b has null)", `{}`, `{"x":null}`, []Diff{{"$.x", KindMissingInA, "", "null"}}},
		{"null equals null", `{"x":null}`, `{"x":null}`, nil},
		{"missing key in b", `{"a":1,"b":2}`, `{"a":1}`, []Diff{{"$.b", KindMissingInB, "2", ""}}},
		{"missing key in a", `{"a":1}`, `{"a":1,"b":2}`, []Diff{{"$.b", KindMissingInA, "", "2"}}},
		{"extra keys reported in sorted order", `{"z":1,"a":1}`, `{"m":1}`, []Diff{
			{"$.a", KindMissingInB, "1", ""}, {"$.m", KindMissingInA, "", "1"}, {"$.z", KindMissingInB, "1", ""}}},
		{"object vs array", `{"x":{}}`, `{"x":[]}`, []Diff{{"$.x", KindType, "{}", "[]"}}},
		{"array element differs", `{"w":["a","b"]}`, `{"w":["a","c"]}`, []Diff{{"$.w[1]", KindValue, `"b"`, `"c"`}}},
		{"array longer in a", `[1,2,3]`, `[1,2]`, []Diff{{"$[2]", KindMissingInB, "3", ""}}},
		{"array longer in b", `[1]`, `[1,{"k":2}]`, []Diff{{"$[1]", KindMissingInA, "", `{"k":2}`}}},
		{"empty array vs empty object", `[]`, `{}`, []Diff{{"$", KindType, "[]", "{}"}}},
		{"nested path with identifier keys", `{"open":{"UTG":{"th":{"r":0.5}}}}`, `{"open":{"UTG":{"th":{"r":0.6}}}}`,
			[]Diff{{"$.open.UTG.th.r", KindValue, "0.5", "0.6"}}},
		{"path quotes keys that are not identifiers", `{"vs_3bet":{"CO>BTN":{"th":{"c":1}}}}`, `{"vs_3bet":{"CO>BTN":{"th":{"c":2}}}}`,
			[]Diff{{"$.vs_3bet['CO>BTN'].th.c", KindValue, "1", "2"}}},
		{"path escapes quotes and backslashes", `{"a'b\\c":1}`, `{"a'b\\c":2}`,
			[]Diff{{`$['a\'b\\c']`, KindValue, "1", "2"}}},
		{"path of a key with a leading digit", `{"1a":1}`, `{"1a":2}`, []Diff{{"$['1a']", KindValue, "1", "2"}}},
		{"top level scalar", `1`, `2`, []Diff{{"$", KindValue, "1", "2"}}},
		{"top level types", `null`, `1`, []Diff{{"$", KindType, "null", "1"}}},
		{"long values are truncated", `{"s":"` + strings.Repeat("x", 100) + `"}`, `{"s":"y"}`,
			[]Diff{{"$.s", KindValue, `"` + strings.Repeat("x", 79) + "…", `"y"`}}},
		{"unicode strings compare exactly", `{"s":"警告"}`, `{"s":"警告"}`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mustCompare(t, tc.a, tc.b)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestCompareErrors(t *testing.T) {
	tests := []struct{ name, a, b string }{
		{"a is not JSON", `{`, `{}`},
		{"b is not JSON", `{}`, `nope`},
		{"empty input", ``, `{}`},
		{"whitespace only", " \n", `{}`},
		{"duplicate key", `{"a":1,"a":2}`, `{}`},
		{"nested duplicate key", `{}`, `{"o":{"k":[{"x":1,"x":1}]}}`},
		{"invalid UTF-8", "{\"s\":\"\xff\"}", `{}`},
		{"lone high surrogate escape", `{"s":"\ud800"}`, `{}`},
		{"surrogate pair escape", `{"s":"\ud83d\ude00"}`, `{}`},
		{"uppercase surrogate escape", `{}`, `{"s":"\uDFFF"}`},
		{"trailing data", `{} {}`, `{}`},
		{"NaN", `{"x":NaN}`, `{"x":1}`},
		{"Infinity", `{"x":1}`, `{"x":Infinity}`},
		{"number beyond float64", `{"x":1e999}`, `{"x":1}`},
		{"nested number beyond float64", `{"x":[{"y":-1e999}]}`, `{"x":[{"y":1}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if diffs, err := Compare([]byte(tc.a), []byte(tc.b)); err == nil {
				t.Errorf("want error, got diffs %+v", diffs)
			}
		})
	}
}

func TestCompareAcceptsEscapedBackslashThatLooksLikeSurrogate(t *testing.T) {
	if diffs := mustCompare(t, `{"s":"\\ud800","t":"\u00e9"}`, `{"s":"\\ud800","t":"\u00e9"}`); len(diffs) != 0 {
		t.Errorf("unexpected diffs %+v", diffs)
	}
}

func TestEmptyInputMessage(t *testing.T) {
	_, err := Compare(nil, []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "empty input") {
		t.Errorf("got %v", err)
	}
}

func TestDiffString(t *testing.T) {
	tests := []struct {
		d    Diff
		want string
	}{
		{Diff{"$.a", KindValue, "1", "2"}, "$.a: value a=1 b=2"},
		{Diff{"$.a", KindType, "null", "1"}, "$.a: type a=null b=1"},
		{Diff{"$.a", KindMissingInA, "", "1"}, "$.a: missing_in_a only in b: 1"},
		{Diff{"$.a", KindMissingInB, "1", ""}, "$.a: missing_in_b only in a: 1"},
	}
	for _, tc := range tests {
		if got := tc.d.String(); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}

// 每份 golden 與自己比對必須完全一致，且確實能解析。
func TestGoldenFilesSelfCompare(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/parity/*/golden.json")
	if err != nil || len(paths) < 2 {
		t.Fatalf("want at least 2 golden files, got %v (err %v)", paths, err)
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if diffs := mustCompare(t, string(data), string(data)); len(diffs) != 0 {
			t.Errorf("%s differs from itself: %+v", p, diffs)
		}
	}
}
