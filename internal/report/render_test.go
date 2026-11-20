package report

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

var sixMax = []string{"UTG", "HJ", "CO", "BTN", "SB", "BB"}

const chromeBin = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

func testDict(t *testing.T) map[string]map[string]string {
	t.Helper()
	dict, err := LoadDict(os.DirFS("../i18n/locales"))
	if err != nil {
		t.Fatal(err)
	}
	return dict
}

func pairs(pos []string) [][2]string {
	var out [][2]string
	for i, a := range pos[:len(pos)-1] {
		for _, b := range pos[i+1:] {
			out = append(out, [2]string{a, b})
		}
	}
	return out
}

// syntheticData 產生版面完整的合成 DATA；th5 為 true 時 vs_5bet 也有理論。
func syntheticData(pos []string, hero string, th5 bool) []byte {
	freq := func(i int, withN bool) map[string]any {
		c, r := 0.1+0.03*float64(i%10), 0.05+0.02*float64(i%7)
		m := map[string]any{"c": c, "r": r}
		if withN {
			m["n"] = 10 + 40*i
		}
		return m
	}
	data := map[string]any{"open": map[string]any{}}
	for i, p := range pos[:len(pos)-1] {
		data["open"].(map[string]any)[p] = map[string]any{
			"th":   map[string]any{"r": 0.1 + 0.05*float64(i)},
			"me":   map[string]any{"r": 0.12 + 0.05*float64(i), "n": 50 * (i + 1)},
			"pool": map[string]any{"r": 0.08 + 0.05*float64(i), "n": 500 * (i + 1)},
		}
	}
	for _, sec := range []string{"vs_open", "vs_3bet", "vs_4bet", "vs_5bet"} {
		cells := map[string]any{}
		for i, pr := range pairs(pos) {
			cell := map[string]any{"th": nil, "me": freq(i+1, true), "pool": freq(i+4, true)}
			if sec != "vs_5bet" || th5 {
				cell["th"] = freq(i+2, false)
			}
			if i%4 == 3 {
				cell["me"] = nil
			}
			cells[pr[0]+">"+pr[1]] = cell
		}
		data[sec] = cells
	}
	data["absolute"] = map[string]any{}
	data["meta"] = map[string]any{"hands": 12345, "hero": hero, "hero_hands": 99, "warnings": []string{},
		"date_from": "2026/10/01", "date_to": "2026/10/06"}
	raw, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return raw
}

func baseOptions(t *testing.T) Options {
	return Options{Lang: "zh-Hant", I18N: testDict(t), Positions: sixMax}
}

func constant(t *testing.T, page, name string) json.RawMessage {
	t.Helper()
	m := regexp.MustCompile(`(?m)^const `+name+` = (.*);$`).FindAllStringSubmatch(page, -1)
	if len(m) != 1 {
		t.Fatalf("const %s appears %d times", name, len(m))
	}
	return json.RawMessage(m[0][1])
}

func decode(t *testing.T, page, name string, into any) {
	t.Helper()
	if err := json.Unmarshal(constant(t, page, name), into); err != nil {
		t.Fatalf("const %s: %v", name, err)
	}
}

func render(t *testing.T, data []byte, opt Options) string {
	t.Helper()
	out, err := RenderHTML(data, opt)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func sameJSON(t *testing.T, got json.RawMessage, want []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("payload differs:\n got %s\nwant %s", got, want)
	}
}

func TestRenderEmbedsDataAndFillsMarks(t *testing.T) {
	data := syntheticData(sixMax, "Hero", false)
	title := "我的 <矩陣> & 測試"
	opt := baseOptions(t)
	opt.Title, opt.Lang = &title, "en"
	page := render(t, data, opt)
	sameJSON(t, constant(t, page, "D"), data)
	var gotTitle string
	decode(t, page, "USER_TITLE", &gotTitle)
	if gotTitle != title {
		t.Fatalf("title = %q", gotTitle)
	}
	var gotLang string
	decode(t, page, "DEFAULT_LANG", &gotLang)
	if gotLang != "en" {
		t.Fatalf("lang = %q", gotLang)
	}
	var pos []string
	decode(t, page, "POS", &pos)
	if !slices.Equal(pos, sixMax) {
		t.Fatalf("POS = %v", pos)
	}
	for _, name := range []string{"NOTICE", "DEV"} {
		if got := string(constant(t, page, name)); got != "null" {
			t.Fatalf("%s = %s, want null", name, got)
		}
	}
	for _, mark := range marks {
		if strings.Contains(page, mark) {
			t.Fatalf("mark %q left in page", mark)
		}
	}
	if !strings.HasPrefix(page, "<!doctype html>") {
		t.Fatal("page does not start with doctype")
	}
}

func TestNilTitleIsNull(t *testing.T) {
	page := render(t, syntheticData(sixMax, "Hero", false), baseOptions(t))
	if got := string(constant(t, page, "USER_TITLE")); got != "null" {
		t.Fatalf("USER_TITLE = %s", got)
	}
}

func TestMarksOrPlaceholdersInUserTextDoNotLeak(t *testing.T) {
	hero := "{{TITLE}} /*I18N*/null"
	data := syntheticData(sixMax, hero, false)
	title := "/*DATA*/null {{FOOTNOTE}} /*I18N*/null /*POS*/null"
	opt := baseOptions(t)
	opt.Title = &title
	opt.Notice = &Notice{Key: "report.notice.incomplete", Args: map[string]string{"nodes": "/*DEVIATIONS*/null", "actions": "{{x}}"}}
	opt.Deviations = &Deviations{Pool: []Deviation{{Node: "open", Decider: "/*TITLE*/null", AnchorID: "{{a}}"}}}
	page := render(t, data, opt)
	sameJSON(t, constant(t, page, "D"), data)
	for _, name := range []string{"D", "I18N", "DEFAULT_LANG", "USER_TITLE", "NOTICE", "POS", "DEV"} {
		constant(t, page, name)
	}
	var gotTitle string
	decode(t, page, "USER_TITLE", &gotTitle)
	if gotTitle != title {
		t.Fatalf("title = %q", gotTitle)
	}
	var pos []string
	decode(t, page, "POS", &pos)
	if !slices.Equal(pos, sixMax) {
		t.Fatalf("POS = %v", pos)
	}
}

func TestOriginalSpaceSectionWithoutCannedCommentary(t *testing.T) {
	page := render(t, syntheticData(sixMax, "Hero", false), baseOptions(t))
	for _, want := range []string{`id="a-3b"`, `id="a-4b"`, `id="a-5b"`, "awall("} {
		if !strings.Contains(page, want) {
			t.Fatalf("page lacks %s", want)
		}
	}
	for _, banned := range []string{"ka-3b", "ka-4b", "k-open"} {
		if strings.Contains(page, banned) {
			t.Fatalf("page contains %s", banned)
		}
	}
}

// 每個注入點都帶終止字元：</script>、<!--、U+2028/2029、注入點字串。
func TestEveryInjectionPointIsScriptSafe(t *testing.T) {
	evil := "x</script><b><!-- \u2028 \u2029 /*DATA*/null {{"
	data := syntheticData(sixMax, evil, false)
	opt := baseOptions(t)
	opt.I18N = map[string]map[string]string{}
	for lang, dict := range testDict(t) {
		opt.I18N[lang] = map[string]string{}
		for k, v := range dict {
			opt.I18N[lang][k] = v
		}
		opt.I18N[lang]["report.intro"] = evil
	}
	opt.Title = &evil
	opt.Notice = &Notice{Key: evil, Args: map[string]string{"nodes": evil}}
	opt.Deviations = &Deviations{Pool: []Deviation{{Node: evil, Decider: evil, Opponent: evil, Action: evil, AnchorID: evil}}, Me: []Deviation{}}
	page := render(t, data, opt)
	if n := strings.Count(page, "</script>"); n != 1 {
		t.Fatalf("page has %d </script>", n)
	}
	if strings.Contains(page, "\u2028") || strings.Contains(page, "\u2029") || strings.Contains(page, "<b>") || strings.Contains(page, "<!--") {
		t.Fatal("unescaped terminator in page")
	}
	var title string
	decode(t, page, "USER_TITLE", &title)
	var pos []string
	decode(t, page, "POS", &pos)
	var notice Notice
	decode(t, page, "NOTICE", &notice)
	var dev Deviations
	decode(t, page, "DEV", &dev)
	var i18n map[string]map[string]string
	decode(t, page, "I18N", &i18n)
	if title != evil || !slices.Equal(pos, sixMax) || notice.Key != evil || notice.Args["nodes"] != evil ||
		dev.Pool[0].AnchorID != evil || i18n["en"]["report.intro"] != evil {
		t.Fatal("payload did not round-trip")
	}
	sameJSON(t, constant(t, page, "D"), data)
}

func TestRenderRejectsBadInput(t *testing.T) {
	data := syntheticData(sixMax, "Hero", false)
	good := baseOptions(t)
	cases := map[string]struct {
		data []byte
		mod  func(*Options)
	}{
		"bad lang":        {data, func(o *Options) { o.Lang = "fr" }},
		"empty lang":      {data, func(o *Options) { o.Lang = "" }},
		"missing dict":    {data, func(o *Options) { o.I18N = map[string]map[string]string{"en": {"a": "b"}} }},
		"few positions":   {data, func(o *Options) { o.Positions = []string{"A"} }},
		"invalid json":    {[]byte("{"), func(*Options) {}},
		"non-object json": {[]byte("[1]"), func(*Options) {}},
		"padded array":    {[]byte(" \n[{}]"), func(*Options) {}},
		"bad position":    {data, func(o *Options) { o.Positions = []string{"A", "B C"} }},
		"markup position": {data, func(o *Options) { o.Positions = []string{"A", "</script>"} }},
	}
	for name, c := range cases {
		opt := good
		c.mod(&opt)
		if _, err := RenderHTML(c.data, opt); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestTemplateMarksAppearExactlyOnceInOrder(t *testing.T) {
	last := -1
	for _, mark := range marks {
		if n := strings.Count(templateHTML, mark); n != 1 {
			t.Fatalf("%s appears %d times", mark, n)
		}
		idx := strings.Index(templateHTML, mark)
		if idx < last {
			t.Fatalf("%s is out of order", mark)
		}
		last = idx
	}
}

func TestTemplateHasNoPositionLiterals(t *testing.T) {
	re := regexp.MustCompile(`UTG|HJ|CO|BTN|SB|BB`)
	if m := re.FindAllString(templateHTML, -1); m != nil {
		t.Fatalf("position literals in template: %v", m)
	}
}

var placeholderRe = regexp.MustCompile(`\{(\w+)\}`)

func placeholders(s string) []string {
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func TestDictionariesHaveSameKeysAndPlaceholders(t *testing.T) {
	dict := testDict(t)
	ref := dict["zh-Hant"]
	for _, lang := range Langs() {
		d := dict[lang]
		if len(d) != len(ref) {
			t.Errorf("%s has %d keys, zh-Hant has %d", lang, len(d), len(ref))
		}
		for key, want := range ref {
			got, ok := d[key]
			switch {
			case !ok:
				t.Errorf("%s lacks %s", lang, key)
			case strings.TrimSpace(got) == "":
				t.Errorf("%s: %s is empty", lang, key)
			case !slices.Equal(placeholders(got), placeholders(want)):
				t.Errorf("%s: %s placeholders %v != %v", lang, key, placeholders(got), placeholders(want))
			}
			if !strings.HasPrefix(key, "report.") {
				t.Errorf("key %s lacks the report. prefix", key)
			}
		}
	}
}

func TestDictionariesAvoidAdvisoryWords(t *testing.T) {
	re := regexp.MustCompile(`(?i)應該|建議|应该|建议|should|recommend|suggest|advise`)
	for lang, d := range testDict(t) {
		for key, text := range d {
			if re.MatchString(text) {
				t.Errorf("%s: %s contains advisory wording", lang, key)
			}
		}
	}
}

// template 用到的 key（字面值或動態前綴）都在字典裡，字典裡的 key 也都有被用到（report.notice.* 由呼叫端提供）。
func TestTemplateKeysMatchDictionary(t *testing.T) {
	dict := testDict(t)["zh-Hant"]
	used := map[string]bool{}
	var prefixes []string
	for _, m := range regexp.MustCompile(`report\.[A-Za-z0-9_.]+`).FindAllString(templateHTML, -1) {
		if strings.HasSuffix(m, ".") {
			prefixes = append(prefixes, m)
			continue
		}
		used[m] = true
		if _, ok := dict[m]; !ok {
			t.Errorf("template key %s is not in the dictionary", m)
		}
	}
	dynamic := map[string][]string{
		"report.dev.node.":   {"open", "vs_open", "vs_3bet", "vs_4bet"},
		"report.dev.col.":    {"spot", "node", "action", "obs", "theory", "diff", "n", "z"},
		"report.who.":        {"theory", "me", "pool"},
		"report.sec.vs5bet.": {"cap", "capTheory", "capMixed"},
		"report.abs.vs5bet.": {"cap", "capTheory", "capMixed"},
	}
	for _, p := range prefixes {
		suffixes, ok := dynamic[p]
		if !ok {
			t.Errorf("unexpected dynamic key prefix %s", p)
		}
		for _, s := range suffixes {
			if _, ok := dict[p+s]; !ok {
				t.Errorf("dynamic key %s is not in the dictionary", p+s)
			}
			used[p+s] = true
		}
	}
	for key := range dict {
		if !used[key] && !strings.HasPrefix(key, "report.notice.") {
			t.Errorf("dictionary key %s is unused", key)
		}
	}
}

func TestScriptSafeReplacer(t *testing.T) {
	bs := string(rune(92))
	for in, want := range map[string]string{
		"<":                          bs + "u003c",
		"</script>":                  bs + "u003c/script>",
		string(rune(0x2028)):         bs + "u2028",
		string(rune(0x2029)):         bs + "u2029",
		"plain & > text":             "plain & > text",
		"a<b" + string(rune(0x2028)): "a" + bs + "u003cb" + bs + "u2028",
	} {
		if got := scriptSafe.Replace(in); got != want {
			t.Errorf("scriptSafe.Replace(%q) = %q, want %q", in, got, want)
		}
	}
	if scriptSafe.Replace("<") != "\\u003c" {
		t.Error("replacement must be a literal backslash-u sequence")
	}
}

func TestLangsReturnsACopy(t *testing.T) {
	Langs()[0] = "x"
	if Langs()[0] != "zh-Hant" {
		t.Fatal("Langs() exposes shared state")
	}
}

func TestEmptyTitleIsNull(t *testing.T) {
	empty := ""
	opt := baseOptions(t)
	opt.Title = &empty
	page := render(t, syntheticData(sixMax, "Hero", false), opt)
	if got := string(constant(t, page, "USER_TITLE")); got != "null" {
		t.Fatalf("USER_TITLE = %s", got)
	}
}

func TestPaddedObjectIsAccepted(t *testing.T) {
	data := append([]byte(" \n\t"), syntheticData(sixMax, "Hero", false)...)
	render(t, data, baseOptions(t))
}

// 手寫的原始 DATA bytes（不經 json.Marshal）：含 </script>、<!--、實體 U+2028/2029。
func TestRawDataBytesAreScriptSafe(t *testing.T) {
	evil := "a</script><script>alert(1)</script><!--" + string(rune(0x2028)) + string(rune(0x2029))
	raw := []byte(`{"open":{},"meta":{"hero":"` + evil + `","hands":1,"date_from":"","date_to":""}}`)
	page := render(t, raw, baseOptions(t))
	if n := strings.Count(page, "</script>"); n != 1 {
		t.Fatalf("page has %d </script>", n)
	}
	for _, bad := range []string{"<!--", "<script>alert", string(rune(0x2028)), string(rune(0x2029))} {
		if strings.Contains(page, bad) {
			t.Fatalf("page contains raw %q", bad)
		}
	}
	sameJSON(t, constant(t, page, "D"), raw)
}

// partial5 拿掉 vs_5bet 一半格子的理論（依 key 排序後的偶數位）。
func partial5(t *testing.T, data []byte) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	cells := doc["vs_5bet"].(map[string]any)
	keys := make([]string, 0, len(cells))
	for k := range cells {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for i, k := range keys {
		if i%2 == 0 {
			cells[k].(map[string]any)["th"] = nil
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func chromeSkipUnlessAvailable(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode: skipping Chrome tests")
	}
	if _, err := os.Stat(chromeBin); err != nil {
		if os.Getenv("PMA_REQUIRE_CHROME") == "1" {
			t.Fatal("PMA_REQUIRE_CHROME=1 but Google Chrome is not installed")
		}
		t.Skip("Google Chrome not installed")
	}
}

// 同時最多 4 個 Chrome，避免平行測試把機器吃滿。
var chromeSlots = make(chan struct{}, 4)

func chromeDump(t *testing.T, html, query string) string {
	t.Helper()
	chromeSlots <- struct{}{}
	defer func() { <-chromeSlots }()
	dir := t.TempDir()
	file := filepath.Join(dir, "report.html")
	if err := os.WriteFile(file, []byte(html), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, chromeBin, "--headless=new", "--disable-gpu", "--no-first-run",
		"--host-resolver-rules=MAP * ~NOTFOUND",
		"--user-data-dir="+filepath.Join(dir, "profile"), "--virtual-time-budget=3000", "--dump-dom",
		"file://"+file+query)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("chrome: %v", err)
	}
	return string(out)
}

// 殘留的 i18n key（"in the report." 這種句尾不算）。
var leftoverKey = regexp.MustCompile(`\breport\.[A-Za-z]\w*`)

// 去掉 <script> 與 data-i 屬性後的可見 DOM。
var scriptRe = regexp.MustCompile(`(?s)<script.*?</script>|\sdata-i="[^"]*"`)

func idsIn(dom string) map[string]bool {
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`\sid="(cell-[^"]*)"`).FindAllStringSubmatch(dom, -1) {
		ids[m[1]] = true
	}
	return ids
}

// 由 POS 推導所有可能的 AnchorID。
func expectedIDs(pos []string, th5 bool) []string {
	var out []string
	secs := []string{"vs_open", "vs_3bet", "vs_4bet"}
	if th5 {
		secs = append(secs, "vs_5bet")
	}
	for _, sec := range secs {
		for _, pr := range pairs(pos) {
			for _, who := range []string{"me", "pool"} {
				for _, k := range []string{"f", "c", "r"} {
					out = append(out, fmt.Sprintf("cell-%s-%s-%s-%s-%s", sec, pr[0], pr[1], who, k))
				}
			}
		}
	}
	for _, p := range pos[:len(pos)-1] {
		out = append(out, "cell-open-"+p+"-me", "cell-open-"+p+"-pool")
	}
	return out
}

func TestRenderedDOMPerLanguage(t *testing.T) {
	chromeSkipUnlessAvailable(t)
	t.Parallel()
	titles := map[string]string{"zh-Hant": "翻前位置矩陣", "zh-Hans": "翻前位置矩阵", "en": "Preflop Position Matrix"}
	for _, lang := range Langs() {
		for _, mode := range []string{"none", "all", "mixed"} {
			t.Run(lang+"/5bet-theory="+mode, func(t *testing.T) {
				t.Parallel()
				data := syntheticData(sixMax, "Hero", mode != "none")
				if mode == "mixed" {
					data = partial5(t, data)
				}
				dom := chromeDump(t, render(t, data, baseOptions(t)), "?lang="+lang)
				visible := scriptRe.ReplaceAllString(dom, "")
				if leftoverKey.MatchString(visible) {
					t.Fatalf("untranslated key left in DOM: %s", leftoverKey.FindString(visible))
				}
				if !strings.Contains(dom, `<html lang="`+lang+`"`) {
					t.Errorf("html lang is not %s", lang)
				}
				if !strings.Contains(visible, "<title>"+titles[lang]+"</title>") {
					t.Errorf("title is not %s", titles[lang])
				}
				if !strings.Contains(visible, `id="notice" hidden=""></div>`) {
					t.Error("notice should be hidden and empty when null")
				}
				if !strings.Contains(visible, `id="dev" hidden=""></div>`) {
					t.Error("deviation summary should be hidden when null")
				}
				ids := idsIn(visible)
				for _, id := range expectedIDs(sixMax, mode != "none") {
					if !ids[id] {
						t.Errorf("anchor %s missing from DOM", id)
					}
				}
				if mode == "none" {
					for id := range ids {
						if strings.HasPrefix(id, "cell-vs_5bet-") {
							t.Errorf("unexpected %s without 5bet theory", id)
						}
					}
				}
				if mode == "mixed" && lang == "en" {
					for _, want := range []string{"Spots with theory use the three rows above", "Pool − Theory", "Me − Pool"} {
						if !strings.Contains(visible, want) {
							t.Errorf("mixed 5bet layout lacks %q", want)
						}
					}
				}
			})
		}
	}
}

func noticeAndDeviationsOptions(t *testing.T) Options {
	opt := baseOptions(t)
	opt.Notice = &Notice{Key: "report.notice.incomplete", Args: map[string]string{"nodes": "3", "actions": "7"}}
	opt.Deviations = &Deviations{
		Pool: []Deviation{
			{Node: "vs_3bet", Decider: "CO", Opponent: "BTN", Action: "r", Obs: 0.31, Theory: 0.2, DiffPP: 11, N: 120, Z: 3.1, AnchorID: "cell-vs_3bet-CO-BTN-pool-r"},
			{Node: "vs_open", Decider: "BB", Opponent: "CO", Action: "c", Obs: 0.3, Theory: 0.3, DiffPP: -0.04, N: 50, Z: -0.04, AnchorID: "cell-vs_open-CO-BB-pool-c"},
			{Node: "open", Decider: "SB", Action: "r", Obs: 0.3, Theory: 0.3, DiffPP: 0.04, N: 50, Z: 0.04, AnchorID: `bad id"><b>`},
		},
		Me: []Deviation{{Node: "open", Decider: "UTG", Action: "r", Obs: 0.2, Theory: 0.001, DiffPP: 19.9, N: 40, Z: 2.2, Clamped: true, AnchorID: "cell-open-UTG-me"}},
	}
	return opt
}

func TestNoticeAndDeviationsRenderInEveryLanguage(t *testing.T) {
	chromeSkipUnlessAvailable(t)
	t.Parallel()
	dict := testDict(t)
	page := render(t, syntheticData(sixMax, "Hero", false), noticeAndDeviationsOptions(t))
	for _, lang := range Langs() {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			dom := scriptRe.ReplaceAllString(chromeDump(t, page, "?lang="+lang), "")
			if leftoverKey.MatchString(dom) {
				t.Errorf("untranslated key %s", leftoverKey.FindString(dom))
			}
			notice := strings.NewReplacer("{nodes}", "3", "{actions}", "7").Replace(dict[lang]["report.notice.incomplete"])
			wants := []string{notice, `href="#cell-vs_3bet-CO-BTN-pool-r"`, `href="#cell-open-UTG-me"`, "CO vs BTN", "+11.0", "31.0%", "20.0%", "0.1%", "<td>0.0</td>"}
			for _, want := range wants {
				if !strings.Contains(dom, want) {
					t.Errorf("DOM lacks %q", want)
				}
			}
			for _, bad := range []string{"-0.0", "+0.0", "bad id", "<b>"} {
				if strings.Contains(dom, bad) {
					t.Errorf("DOM contains %q", bad)
				}
			}
			if strings.Contains(dom, `id="notice" hidden`) || strings.Contains(dom, `id="dev" hidden`) {
				t.Error("notice or deviations hidden")
			}
			if !strings.Contains(dom, `<th class="l"`) {
				t.Error("node/action headers lack the left-align class")
			}
		})
	}
}

func TestDeviationRowClickJumpsToCell(t *testing.T) {
	chromeSkipUnlessAvailable(t)
	t.Parallel()
	page := render(t, syntheticData(sixMax, "Hero", false), noticeAndDeviationsOptions(t))
	page = strings.Replace(page, "</body>", `<script>document.querySelector('#dev tr[data-anchor] td.l').click(); document.body.dataset.hash = location.hash</script></body>`, 1)
	dom := chromeDump(t, page, "")
	if !strings.Contains(dom, `data-hash="#cell-vs_3bet-CO-BTN-pool-r"`) {
		t.Fatal("clicking a row did not set the hash to its cell")
	}
}

func TestEmbedHidesSwitcherAndDeviationsButKeepsNotice(t *testing.T) {
	chromeSkipUnlessAvailable(t)
	t.Parallel()
	dom := scriptRe.ReplaceAllString(chromeDump(t, render(t, syntheticData(sixMax, "Hero", false), noticeAndDeviationsOptions(t)), "?embed=1"), "")
	if !strings.Contains(dom, `data-embed="1"`) {
		t.Error("embed flag not set on <html>")
	}
	if !strings.Contains(dom, `id="dev" hidden=""></div>`) {
		t.Error("deviation summary should be hidden in embed mode")
	}
	if strings.Contains(dom, `id="notice" hidden`) || !strings.Contains(dom, "3") {
		t.Error("notice should stay visible in embed mode")
	}
}

func TestLanguageSwitcherRerendersWithoutKeys(t *testing.T) {
	chromeSkipUnlessAvailable(t)
	t.Parallel()
	page := render(t, syntheticData(sixMax, "Hero", false), baseOptions(t))
	page = strings.Replace(page, "</body>", `<script>document.querySelector('#langsw [data-lang="en"]').click()</script></body>`, 1)
	dom := chromeDump(t, page, "")
	visible := scriptRe.ReplaceAllString(dom, "")
	if !strings.Contains(visible, "<title>Preflop Position Matrix</title>") || !strings.Contains(dom, `<html lang="en"`) || leftoverKey.MatchString(visible) {
		t.Fatal("switching to en did not re-render the page")
	}
}

// DATA 與使用者文字裡的 HTML 不得變成 DOM 元素（critic 實證過的 XSS 路徑）。
func TestMarkupInDataAndTextDoesNotBecomeDOM(t *testing.T) {
	chromeSkipUnlessAvailable(t)
	t.Parallel()
	hero := "</script><img src=x onerror=document.title='pwned'>"
	for name, data := range map[string][]byte{
		"marshaled": syntheticData(sixMax, hero, false),
		"raw bytes": []byte(`{"open":{},"vs_open":{},"vs_3bet":{},"vs_4bet":{},"vs_5bet":{},"meta":{"hero":"` + strings.ReplaceAll(hero, "'", "\\u0027") + `","hands":1,"date_from":"","date_to":""}}`),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			opt := noticeAndDeviationsOptions(t)
			title := hero
			opt.Title = &title
			dom := chromeDump(t, render(t, data, opt), "")
			if strings.Contains(dom, "<img") || strings.Contains(dom, "pwned</title>") {
				t.Fatal("injected markup reached the DOM")
			}
		})
	}
}
