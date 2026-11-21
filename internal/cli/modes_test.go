package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// flags_test.go 之外的新功能表格測試：旗標解析、--lang、--top、互斥、--check-ranges、--init-ranges。

func TestExitCodesAndMessages(t *testing.T) {
	isolateEnv(t)
	hands, ranges := handsDir(t), standardRanges(t)
	out := func() string { return filepath.Join(t.TempDir(), "o.html") }
	tests := []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"hands required", []string{"--ranges", ranges}, 2, "產生報告需要 --hands"},
		{"no args at all", nil, 2, "產生報告需要 --hands"},
		{"unknown flag", []string{"--bogus"}, 2, "不認得的參數：--bogus"},
		{"positional", []string{"x.txt"}, 2, "不認得的參數：x.txt"},
		{"bool flag with value", []string{"--check-ranges=1"}, 2, "不認得的參數：--check-ranges=1"},
		{"missing value at end", []string{"--hands"}, 2, "參數 --hands 需要一個值"},
		{"missing value before flag", []string{"--hands", "--ranges", ranges}, 2, "參數 --hands 需要一個值"},
		{"lang invalid", []string{"--hands", hands, "--ranges", ranges, "--lang", "fr"}, 2, "--lang 只接受 zh-Hant、zh-Hans、en"},
		{"lang empty", []string{"--hands", hands, "--ranges", ranges, "--lang="}, 2, "--lang 只接受"},
		{"top 21", []string{"--hands", hands, "--ranges", ranges, "--top", "21"}, 2, "--top 必須是 0 到 20 的整數：21"},
		{"top -1", []string{"--hands", hands, "--ranges", ranges, "--top", "-1"}, 2, "--top 必須是 0 到 20 的整數：-1"},
		{"top text", []string{"--hands", hands, "--ranges", ranges, "--top=abc"}, 2, "--top 必須是 0 到 20 的整數：abc"},
		{"top empty", []string{"--hands", hands, "--ranges", ranges, "--top="}, 2, "--top 必須是"},
		{"check + init", []string{"--check-ranges", "--init-ranges", t.TempDir()}, 2, "不能彼此併用"},
		{"check + hands", []string{"--check-ranges", "--hands", hands}, 2, "不能彼此併用"},
		{"check + out", []string{"--check-ranges", "-o", out()}, 2, "不能彼此併用"},
		{"check + json", []string{"--check-ranges", "--json=x"}, 2, "不能彼此併用"},
		{"check + hero", []string{"--check-ranges", "--hero", "x"}, 2, "不能彼此併用"},
		{"check + title", []string{"--check-ranges", "--title", "x"}, 2, "不能彼此併用"},
		{"check + top", []string{"--check-ranges", "--top", "3"}, 2, "不能彼此併用"},
		{"init + ranges", []string{"--init-ranges", t.TempDir(), "--ranges", ranges}, 2, "不能彼此併用"},
		{"init + top", []string{"--init-ranges", t.TempDir(), "--top=3"}, 2, "不能彼此併用"},
		{"init target is a file", []string{"--init-ranges", filepath.Join(ranges, "UTG Open 2bb.txt")}, 2, "不是資料夾："},
		{"hero value looks like a flag", []string{"--hands", hands, "--hero", "-x"}, 2, "參數 --hero 需要一個值"},
		{"title value looks like a flag", []string{"--hands", hands, "--title", "--ranges"}, 2, "參數 --title 需要一個值"},
		{"init + hands", []string{"--init-ranges", t.TempDir(), "--hands", hands}, 2, "不能彼此併用"},
		{"init empty dir", []string{"--init-ranges="}, 2, "參數 --init-ranges 需要一個值"},
		{"check folder missing", []string{"--check-ranges", "--ranges", filepath.Join(t.TempDir(), "nope")}, 2, "不存在"},
		{"top boundary 0 ok", []string{"--hands", hands, "--ranges", ranges, "--top", "0", "-o", out()}, 0, ""},
		{"top boundary 20 ok", []string{"--hands", hands, "--ranges", ranges, "--top=20", "-o", out()}, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runCLI(t, tt.args...)
			if r.code != tt.code {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, tt.code, r.stdout, r.err)
			}
			if tt.stderr != "" {
				mustContain(t, "stderr", r.err, "錯誤：", tt.stderr)
			}
		})
	}
}

func TestParseErrorsStillWinOverLaterModes(t *testing.T) {
	isolateEnv(t)
	r := runCLI(t, "--bogus", "-h")
	if r.code != 2 || r.stdout != "" {
		t.Errorf("exit %d, stdout %q", r.code, r.stdout)
	}
}

func TestLangAliasesAndEqualsForm(t *testing.T) {
	isolateEnv(t)
	hands, ranges := handsDir(t), standardRanges(t)
	for lang, want := range map[string]string{
		"zh-Hant": "手牌：掃描", "zh-TW": "手牌：掃描", "ZH-tw": "手牌：掃描",
		"zh-Hans": "手牌：扫描", "zh-CN": "手牌：扫描",
		"en": "Hands: scanned", "EN": "Hands: scanned",
	} {
		for _, form := range [][]string{{"--lang", lang}, {"--lang=" + lang}} {
			args := append([]string{"--hands=" + hands, "--ranges=" + ranges, "--out=" + filepath.Join(t.TempDir(), "o.html")}, form...)
			r := runCLI(t, args...)
			if r.code != 0 {
				t.Fatalf("%v: exit %d: %s", form, r.code, r.err)
			}
			mustContain(t, lang+" stdout", r.stdout, want)
		}
	}
}

func TestLangDefaultsToSystemLanguage(t *testing.T) {
	isolateEnv(t)
	var out, errb strings.Builder
	if code := run([]string{"--hands", handsDir(t), "--ranges", standardRanges(t), "-o", filepath.Join(t.TempDir(), "o.html")}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if out.Len() == 0 {
		t.Error("no output")
	}
}

func TestErrorsUseTheChosenLanguage(t *testing.T) {
	isolateEnv(t)
	r := runCLI(t, "--lang", "en", "--ranges", "x")
	mustContain(t, "stderr", r.err, "Error: ")
	r = runCLI(t, "--lang", "zh-Hans", "--top", "99")
	mustContain(t, "stderr", r.err, "错误：")
	r = runCLI(t, "--lang", "fr", "--top", "99")
	mustContain(t, "invalid lang is reported before top", r.err, "--lang")
}

func TestHelp(t *testing.T) {
	isolateEnv(t)
	for _, args := range [][]string{{"-h"}, {"--help"}} {
		r := runCLI(t, args...)
		if r.code != 0 || r.err != "" {
			t.Errorf("%v: exit %d, stderr %q", args, r.code, r.err)
		}
		mustContain(t, "help", r.stdout, "用法：", "--hands DIR", "--check-ranges", "--init-ranges DIR", "--top N", "--lang LANG", "-o, --out FILE", "預設 5")
	}
	en := runCLI(t, "--lang", "en", "--help")
	mustContain(t, "en help", en.stdout, "Usage:", "Options:")
}

// TestEveryFlagIsDocumentedAndRecognised 防止旗標表、說明與 IsCLIArgs 漂移。
func TestEveryFlagIsDocumentedAndRecognised(t *testing.T) {
	isolateEnv(t)
	help := runCLI(t, "-h").stdout
	for _, spec := range flagSpecs {
		for _, name := range spec.names {
			if !IsCLIArgs([]string{name}) || !IsCLIArgs([]string{name + "=x"}) {
				t.Errorf("IsCLIArgs rejects %s", name)
			}
			if !strings.Contains(help, name) {
				t.Errorf("help does not mention %s", name)
			}
		}
	}
	documented := 0
	for _, e := range helpEntries {
		documented += strings.Count(e.usage, ",") + 1
	}
	known := 0
	for _, spec := range flagSpecs {
		known += len(spec.names)
	}
	if documented != known || len(knownFlags) != known {
		t.Errorf("help documents %d flag names, flagSpecs has %d, knownFlags %d", documented, known, len(knownFlags))
	}
}

func TestLastValueWins(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	jsonPath := filepath.Join(tmp, "m.json")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "--hero", "Nobody", "--hero=p_UTG", "-o", filepath.Join(tmp, "o.html"), "--json", jsonPath)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	if hero := nested(readJSON(t, jsonPath), "meta", "hero"); hero != "p_UTG" {
		t.Errorf("hero = %v", hero)
	}
}

func TestEqualsValueMayContainEqualsSign(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "--title=a=b", "-o", filepath.Join(tmp, "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	page, _ := os.ReadFile(filepath.Join(tmp, "o.html"))
	mustContain(t, "html", string(page), `const USER_TITLE = "a=b"`)
}

func TestReportCarriesNoticeAndDeviations(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	out := filepath.Join(tmp, "o.html")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", out)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	page, _ := os.ReadFile(out)
	mustContain(t, "html", string(page), `"key":"report.notice.incomplete"`, `const DEFAULT_LANG = "zh-Hant"`, `const POS = ["UTG","HJ","CO","BTN","SB","BB"]`)
	mustNotContain(t, "html", string(page), "/*DATA*/", "/*NOTICE*/", "/*DEVIATIONS*/")
}

func TestCompleteRangesHaveNoNotice(t *testing.T) {
	isolateEnv(t)
	dir := filepath.Join(t.TempDir(), "r")
	if r := runCLI(t, "--init-ranges", dir); r.code != 0 {
		t.Fatal(r.err)
	}
	fillAll(t, dir)
	out := filepath.Join(t.TempDir(), "o.html")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", dir, "-o", out)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	page, _ := os.ReadFile(out)
	mustContain(t, "html", string(page), "const NOTICE = null;")
}

func TestTopZeroOmitsDeviationsAndTopControlsRows(t *testing.T) {
	isolateEnv(t)
	hands, ranges := handsDir(t), standardRanges(t)
	out := filepath.Join(t.TempDir(), "o.html")
	r := runCLI(t, "--hands", hands, "--ranges", ranges, "-o", out, "--top", "0")
	if r.code != 0 {
		t.Fatal(r.err)
	}
	page, _ := os.ReadFile(out)
	mustContain(t, "html", string(page), "const DEV = null;")
	mustNotContain(t, "stdout", r.stdout, "偏離摘要")
	r = runCLI(t, "--hands", hands, "--ranges", ranges, "-o", out)
	mustNotContain(t, "stdout", r.stdout, "應該", "建議")
}

func TestDeviationSummaryIsPrintedAfterTheOutputLine(t *testing.T) {
	isolateEnv(t)
	root := t.TempDir()
	hands := filepath.Join(root, "h", "a.txt")
	var text strings.Builder
	for i := 0; i < 40; i++ {
		text.WriteString(makeHand(append([]act{folds("UTG"), folds("HJ"), folds("CO"), folds("BTN")}, raises("SB", 3), folds("BB")), "D"+strconv.Itoa(i), "SB"))
	}
	write(t, hands, text.String())
	ranges := writeRanges(t, map[string]map[string]float64{"SB Open 3bb": {"AA": 1}})
	r := runCLI(t, "--hands", filepath.Dir(hands), "--ranges", ranges, "-o", filepath.Join(root, "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	i, j := strings.Index(r.stdout, "已輸出："), strings.Index(r.stdout, "偏離摘要")
	if i < 0 || j < i {
		t.Fatalf("summary not after the output line:\n%s", r.stdout)
	}
	mustContain(t, "stdout", r.stdout[j:], "我（Hero）", "位置對抗 | 節點 | 動作 | 實際 | 理論 | 差（百分點） | n | z", "SB | 開池 | 開池 | 100.0% | ")
}

func TestCheckRangesGroupsTheReport(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "UTG open.txt"), rangeText(map[string]float64{"AA": 1}))
	write(t, filepath.Join(dir, "BB vs UTG open call.txt"), rangeText(map[string]float64{"AA": 1}))
	write(t, filepath.Join(dir, "HJ open.txt"), "\n")
	write(t, filepath.Join(dir, "UTG open dup.txt"), "")
	write(t, filepath.Join(dir, "UTG open 2bb.txt"), rangeText(map[string]float64{"KK": 1}))
	write(t, filepath.Join(dir, "CO open.txt"), "garbage")
	write(t, filepath.Join(dir, "notes.txt"), "x")
	r := runCLI(t, "--check-ranges", "--ranges", dir)
	if r.code != 1 {
		t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.err)
	}
	mustContain(t, "stdout", r.stdout,
		"理論範圍來源：參數 --ranges → "+dir+"\n",
		"必要動作 2／95；對應到節點檔 2（標準組 108）",
		"  缺的節點：open BTN: BTN open.txt\n",
		"  缺的節點：vs_open UTG>HJ: HJ vs UTG open call.txt, HJ vs UTG open 3bet.txt",
		"缺的動作檔（當 0）：vs_open UTG>BB 缺 3bet，已當 0 -> BB vs UTG open 3bet.txt",
		"  未填（空白檔，該位置仍缺）：HJ open.txt",
		"  忽略（多人或不認得）：notes.txt",
		"  重複的節點檔（只用第一個）：UTG open.txt",
		"  無法讀取：CO open.txt: ",
		"理論範圍不完整\n")
	if strings.Index(r.stdout, "缺的節點") > strings.Index(r.stdout, "缺的動作檔") {
		t.Error("missing nodes must come before missing actions")
	}
}

func TestCheckRangesCompleteIsExit0(t *testing.T) {
	isolateEnv(t)
	dir := filepath.Join(t.TempDir(), "r")
	if r := runCLI(t, "--init-ranges", dir); r.code != 0 {
		t.Fatal(r.err)
	}
	fillAll(t, dir)
	r := runCLI(t, "--check-ranges", "--ranges", dir)
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.stdout)
	}
	mustContain(t, "stdout", r.stdout, "必要動作 95／95；對應到節點檔 108（標準組 108）", "理論範圍完整\n")
	mustNotContain(t, "stdout", r.stdout, "缺的")
}

func TestCheckRangesRedundantBlankFilesAreSeparated(t *testing.T) {
	isolateEnv(t)
	dir := filepath.Join(t.TempDir(), "r")
	if r := runCLI(t, "--init-ranges", dir); r.code != 0 {
		t.Fatal(r.err)
	}
	fillAll(t, dir)
	write(t, filepath.Join(dir, "UTG open 2bb.txt"), "")
	r := runCLI(t, "--check-ranges", "--ranges", dir)
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.stdout)
	}
	mustContain(t, "stdout", r.stdout, "  未填（空白檔，可刪除）：UTG open 2bb.txt\n")
	mustNotContain(t, "stdout", r.stdout, "仍缺")
}

func TestCheckRangesIncompleteCases(t *testing.T) {
	isolateEnv(t)
	onlyBlank := t.TempDir()
	write(t, filepath.Join(onlyBlank, "UTG open.txt"), "  \n")
	noTxt := t.TempDir()
	write(t, filepath.Join(noTxt, "readme.md"), "x")
	for name, tc := range map[string]struct {
		dir  string
		want string
	}{
		"only blank": {onlyBlank, "未填（空白檔，該位置仍缺）：UTG open.txt"},
		"no .txt":    {noTxt, "沒有 .txt 範圍檔"},
		"empty":      {t.TempDir(), "沒有 .txt 範圍檔"},
	} {
		r := runCLI(t, "--check-ranges", "--ranges", tc.dir)
		if r.code != 1 {
			t.Errorf("%s: exit %d", name, r.code)
		}
		mustContain(t, name, r.stdout+r.err, tc.want)
	}
}

func TestCheckRangesUsesEnvAndDefault(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "UTG open.txt"), rangeText(map[string]float64{"AA": 1}))
	t.Setenv("PREFLOP_MATRIX_RANGES", dir)
	r := runCLI(t, "--check-ranges")
	if r.code != 1 {
		t.Fatalf("exit %d: %s", r.code, r.err)
	}
	mustContain(t, "stdout", r.stdout, "環境變數 PREFLOP_MATRIX_RANGES")
	t.Setenv("PREFLOP_MATRIX_RANGES", "")
	if r := runCLI(t, "--check-ranges"); r.code != 2 {
		t.Errorf("missing default folder: exit %d", r.code)
	}
}

func TestInitRanges(t *testing.T) {
	isolateEnv(t)
	dir := filepath.Join(t.TempDir(), "new", "ranges")
	r := runCLI(t, "--init-ranges", dir)
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.err)
	}
	mustContain(t, "stdout", r.stdout, "範圍檔名樣板：新增 108 個 .txt、略過 0 個、已有對應檔 0 個（"+dir+"）", "已建立說明檔 HOW-TO-FILL.md\n")
	if !fileExists(filepath.Join(dir, "UTG open.txt")) || !fileExists(filepath.Join(dir, "HOW-TO-FILL.md")) {
		t.Error("template files missing")
	}
	again := runCLI(t, "--init-ranges="+dir)
	if again.code != 0 {
		t.Fatalf("second run exit %d: %s", again.code, again.err)
	}
	mustContain(t, "stdout", again.stdout, "新增 0 個 .txt、略過 108 個、已有對應檔 0 個", "說明檔 HOW-TO-FILL.md 已存在，未覆蓋")
}

func TestInitRangesListsMappedFiles(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "utg Open 2bb.txt"), "")
	r := runCLI(t, "--init-ranges", dir)
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.err)
	}
	mustContain(t, "stdout", r.stdout, "新增 107 個 .txt、略過 0 個、已有對應檔 1 個", "  已有對應檔（未建立 UTG open.txt）：utg Open 2bb.txt\n")
}

func TestInitRangesWriteFailureIsExit1(t *testing.T) {
	isolateEnv(t)
	blocker := filepath.Join(t.TempDir(), "blocker")
	write(t, blocker, "x")
	r := runCLI(t, "--init-ranges", filepath.Join(blocker, "r"))
	if r.code != 1 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "錯誤：無法建立範圍檔名樣板：")
}

// fillAll 把 dir 內所有 .txt 樣板檔填成同一個有效範圍。
func fillAll(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	body := rangeText(map[string]float64{"AA": 1})
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".txt") {
			write(t, filepath.Join(dir, e.Name()), body)
		}
	}
}
