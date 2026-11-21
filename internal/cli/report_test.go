package cli

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 以下移植自 Python tests/test_cli.py（zh-Hant 下比對原文）。

func nested(v map[string]any, path ...string) any {
	var cur any = v
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func approx(a any, want float64) bool {
	f, ok := a.(float64)
	return ok && math.Abs(f-want) < 1e-12
}

func TestEndToEnd(t *testing.T) {
	isolateEnv(t)
	hands, ranges, tmp := handsDir(t), standardRanges(t), t.TempDir()
	out, jsonPath := filepath.Join(tmp, "out", "m.html"), filepath.Join(tmp, "m.json")
	r := runCLI(t, "--hands", hands, "--ranges", ranges, "-o", out, "--json", jsonPath, "--title", "測試")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.err)
	}
	mustContain(t, "stdout", r.stdout, "採用 3 手", "對應到節點 3 個", "忽略（多人或不認得）1", "已輸出：")
	page, _ := os.ReadFile(out)
	mustContain(t, "html", string(page), `const USER_TITLE = "測試"`, "const D = {")
	data := readJSON(t, jsonPath)
	if !approx(nested(data, "open", "UTG", "th", "r"), 9.0/1326) {
		t.Errorf("open UTG th.r = %v", nested(data, "open", "UTG", "th", "r"))
	}
	if !approx(nested(data, "open", "UTG", "pool", "r"), 2.0/3) || nested(data, "open", "UTG", "pool", "n") != 3.0 {
		t.Errorf("open UTG pool = %v", nested(data, "open", "UTG", "pool"))
	}
	me := nested(data, "vs_open", "UTG>BB", "me").(map[string]any)
	if me["c"] != 0.5 || me["r"] != 0.5 || me["n"] != 2.0 {
		t.Errorf("vs_open UTG>BB me = %v", me)
	}
	if !approx(nested(data, "vs_open", "UTG>BB", "th", "c"), 3.0/1326) || nested(data, "meta", "hands") != 3.0 {
		t.Errorf("th.c = %v, meta.hands = %v", nested(data, "vs_open", "UTG>BB", "th", "c"), nested(data, "meta", "hands"))
	}
}

func TestJSONUsesPythonIndent(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	jsonPath := filepath.Join(tmp, "m.json")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", filepath.Join(tmp, "o.html"), "--json", jsonPath)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	raw, _ := os.ReadFile(jsonPath)
	text := string(raw)
	mustContain(t, "json", text, "{\n \"open\": {\n  \"UTG\": {")
	if strings.HasSuffix(text, "\n") {
		t.Error("Python's json.dumps has no trailing newline")
	}
}

func TestJSONAndHTMLCannotBeTheSameFile(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	same := filepath.Join(tmp, "x.html")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", same, "--json", filepath.Join(tmp, ".", "x.html"))
	if r.code != 2 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "不能指向同一個檔案")
	if fileExists(same) {
		t.Error("html written")
	}
}

func TestJSONAndHTMLSameFileThroughSymlinkAndMissingParent(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(tmp, "real"), filepath.Join(tmp, "link")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ out, json string }{
		{filepath.Join(tmp, "real", "x.html"), filepath.Join(tmp, "link", "x.html")},
		{filepath.Join(tmp, "new", "sub", "x.html"), filepath.Join(tmp, "new", "sub", "..", "sub", "x.html")},
		{filepath.Join(tmp, "link", "new", "x.html"), filepath.Join(tmp, "real", "new", "x.html")},
	} {
		r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", tc.out, "--json", tc.json)
		if r.code != 2 {
			t.Errorf("%s vs %s: exit %d", tc.out, tc.json, r.code)
		}
	}
}

func TestSummaryDoesNotListCompleteCellAndJSONHasAbsolute(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	jsonPath := filepath.Join(tmp, "m.json")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", filepath.Join(tmp, "o.html"), "--json", jsonPath)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustNotContain(t, "stdout", r.stdout, "理論範圍不完整：vs_open UTG>BB 缺 3bet")
	abs, _ := readJSON(t, jsonPath)["absolute"].(map[string]any)
	if len(abs) != 3 || abs["vs_3bet"] == nil || abs["vs_4bet"] == nil || abs["vs_5bet"] == nil {
		t.Errorf("absolute keys = %v", abs)
	}
}

func TestSummaryReportsAMissingAction(t *testing.T) {
	isolateEnv(t)
	ranges := writeRanges(t, map[string]map[string]float64{"BB vs UTG open 2bb call": {"AA": 1}})
	r := runCLI(t, "--hands", handsDir(t), "--ranges", ranges, "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "理論範圍不完整：vs_open UTG>BB 缺 3bet，已當 0")
}

func TestSummaryListsUnfilledBlankFiles(t *testing.T) {
	isolateEnv(t)
	ranges := writeRanges(t, map[string]map[string]float64{"UTG Open 2bb": {"AA": 1}})
	write(t, filepath.Join(ranges, "HJ Open 2bb.txt"), " \n")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", ranges, "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "對應到節點 1 個", "無法讀取 0、未填（空白檔）1", "未填（空白檔）：HJ Open 2bb.txt")
}

func TestOnlyBlankRangeFilesStillOutputsWithExit0(t *testing.T) {
	isolateEnv(t)
	ranges := filepath.Join(t.TempDir(), "blank_ranges")
	write(t, filepath.Join(ranges, "UTG Open 2bb.txt"), "\n")
	out := filepath.Join(t.TempDir(), "o.html")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", ranges, "-o", out)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "未填（空白檔）1")
	if !fileExists(out) {
		t.Error("no html")
	}
}

func TestHeroOption(t *testing.T) {
	isolateEnv(t)
	hands, ranges, tmp := handsDir(t), standardRanges(t), t.TempDir()
	jsonPath := filepath.Join(tmp, "m.json")
	r := runCLI(t, "--hands", hands, "--ranges", ranges, "-o", filepath.Join(tmp, "o.html"), "--hero", "p_UTG", "--json", jsonPath)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	utg := nested(readJSON(t, jsonPath), "open", "UTG").(map[string]any)
	me := utg["me"].(map[string]any)
	if !approx(me["r"], 2.0/3) || me["n"] != 3.0 || utg["pool"] != nil {
		t.Errorf("UTG = %v", utg)
	}
	r = runCLI(t, "--hands", hands, "--ranges", ranges, "-o", filepath.Join(tmp, "o.html"), "--hero", "Nobody")
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stderr", r.err, "Nobody")
}

func TestNoHeroWarningUsesPythonRepr(t *testing.T) {
	isolateEnv(t)
	hands, ranges, tmp := handsDir(t), standardRanges(t), t.TempDir()
	for hero, want := range map[string]string{
		"Nobody": "名為 'Nobody' 的玩家",
		"O'Hara": `名為 "O'Hara" 的玩家`,
		`a'b"c`:  `名為 'a\'b"c' 的玩家`,
	} {
		r := runCLI(t, "--hands", hands, "--ranges", ranges, "-o", filepath.Join(tmp, "o.html"), "--hero", hero)
		if r.code != 0 {
			t.Fatal(r.err)
		}
		mustContain(t, "stderr for "+hero, r.err, "警告：所有手牌都沒有"+want+"，「我」欄會是空的（可用 --hero 指定）")
	}
}

func TestDefaultOutputNameIsInTheCurrentDirectory(t *testing.T) {
	isolateEnv(t)
	hands, ranges := handsDir(t), standardRanges(t)
	tmp := t.TempDir()
	t.Chdir(tmp)
	r := runCLI(t, "--hands", hands, "--ranges", ranges)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "preflop_matrix.html")); err != nil {
		t.Error(err)
	}
	mustContain(t, "stdout", r.stdout, "已輸出：preflop_matrix.html\n")
}

func TestUnparseableHandsAreCountedButNotFatal(t *testing.T) {
	isolateEnv(t)
	hands := handsDir(t)
	write(t, filepath.Join(hands, "bad.txt"), strings.Replace(makeHand(nil, "RC9", "BB"), "big blind", "huge blind", 1))
	r := runCLI(t, "--hands", hands, "--ranges", standardRanges(t), "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "無法解析略過 1", "bad.txt")
}

func TestRangesWithoutAMatchingFileStillRender(t *testing.T) {
	isolateEnv(t)
	out := filepath.Join(t.TempDir(), "o.html")
	ranges := writeRanges(t, map[string]map[string]float64{"BB vs UTG open 2bb SB call call": {"AA": 1}})
	if r := runCLI(t, "--hands", handsDir(t), "--ranges", ranges, "-o", out); r.code != 0 || !fileExists(out) {
		t.Errorf("exit %d, html %v: %s", r.code, fileExists(out), r.err)
	}
}
