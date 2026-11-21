package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestSameFileCheckFollowsSymlinkBeforeDotDot(t *testing.T) {
	isolateEnv(t)
	root := t.TempDir()
	cwd, sub := filepath.Join(root, "cwd"), filepath.Join(root, "D", "sub")
	for _, d := range []string{cwd, sub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join("..", "D", "sub"), filepath.Join(cwd, "link")); err != nil {
		t.Fatal(err)
	}
	hands, ranges := handsDir(t), standardRanges(t)
	t.Chdir(cwd)
	r := runCLI(t, "--hands", hands, "--ranges", ranges, "-o", "../D/b.html", "--json", "link/../b.html")
	if r.code != 2 || fileExists(filepath.Join(root, "D", "b.html")) {
		t.Errorf("link/../b.html is ../D/b.html: exit %d\n%s", r.code, r.err)
	}
	mustContain(t, "stderr", r.err, "不能指向同一個檔案")
	r = runCLI(t, "--hands", hands, "--ranges", ranges, "-o", "../D/b.html", "--json", "link/b.json")
	if r.code != 0 || !fileExists(filepath.Join(sub, "b.json")) {
		t.Errorf("different files: exit %d\n%s", r.code, r.err)
	}
}

func TestDotDotOutputPathsBehaveLikePython(t *testing.T) {
	isolateEnv(t)
	hands, ranges := handsDir(t), standardRanges(t)
	tmp := t.TempDir()
	t.Chdir(tmp)
	// Python 先建立 x（Path.parent 是 x/..），再寫入 ./o.html，並照原樣印出 x/../o.html。
	r := runCLI(t, "--hands", hands, "--ranges", ranges, "-o", "x/../o.html")
	if r.code != 0 || !fileExists("o.html") {
		t.Fatalf("exit %d: %s", r.code, r.err)
	}
	if !isDir("x") {
		t.Error("x was not created")
	}
	mustContain(t, "stdout", r.stdout, "已輸出：x/../o.html\n")
	r = runCLI(t, "--hands", hands, "--ranges", ranges, "-o", "y/../o.html", "--json", "o.html")
	if r.code != 2 {
		t.Errorf("same file through ..: exit %d", r.code)
	}
}

func TestOutputLineIsNormalisedLikePythonPath(t *testing.T) {
	isolateEnv(t)
	t.Chdir(t.TempDir())
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", "./a//b.html", "--json", "./m.json")
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "已輸出：a/b.html、./m.json\n")
}

func TestFlagValueForms(t *testing.T) {
	isolateEnv(t)
	hands, ranges, tmp := handsDir(t), standardRanges(t), t.TempDir()
	for name, args := range map[string][]string{
		"-oVALUE":      {"-o" + filepath.Join(tmp, "a.html")},
		"-o=VALUE":     {"-o=" + filepath.Join(tmp, "b.html")},
		"--out=VALUE":  {"--out=" + filepath.Join(tmp, "c.html")},
		"-o VALUE":     {"-o", filepath.Join(tmp, "d.html")},
		"--title=-x":   {"--title=-x", "-o", filepath.Join(tmp, "e.html")},
		"negative top": {"--top", "0", "-o", filepath.Join(tmp, "f.html")},
	} {
		r := runCLI(t, append([]string{"--hands", hands, "--ranges", ranges}, args...)...)
		if r.code != 0 {
			t.Errorf("%s: exit %d: %s", name, r.code, r.err)
		}
	}
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		if !fileExists(filepath.Join(tmp, name+".html")) {
			t.Errorf("%s.html missing", name)
		}
	}
	for _, first := range []string{"-ofile.html", "-o=x", "-h", "--top=1"} {
		if !IsCLIArgs([]string{first}) {
			t.Errorf("IsCLIArgs(%q) = false", first)
		}
	}
}

func TestNoticeCountsAreExact(t *testing.T) {
	isolateEnv(t)
	hands, tmp := handsDir(t), t.TempDir()
	for name, tc := range map[string]struct {
		files map[string]map[string]float64
		want  string
	}{
		"standard":       {map[string]map[string]float64{"UTG open": {"AA": 1}, "BB vs UTG open call": {"AA": 1}, "BB vs UTG open 3bet": {"AA": 1}}, `"actions":"0","nodes":"48"`},
		"missing action": {map[string]map[string]float64{"UTG open": {"AA": 1}, "BB vs UTG open call": {"AA": 1}}, `"actions":"1","nodes":"48"`},
	} {
		out := filepath.Join(tmp, name+".html")
		r := runCLI(t, "--hands", hands, "--ranges", writeRanges(t, tc.files), "-o", out)
		if r.code != 0 {
			t.Fatal(r.err)
		}
		page, _ := os.ReadFile(out)
		mustContain(t, name, string(page), `"key":"report.notice.incomplete"`, tc.want)
	}
}

// topHands: 40 手 UTG 開池（其他人棄牌）與 40 手 SB 開池；Hero 是 p_UTG，所以玩家池有 UTG、SB 兩列候選，Hero 有一列。
func topHands(t *testing.T) string {
	t.Helper()
	var text strings.Builder
	for i := 0; i < 40; i++ {
		text.WriteString(makeHand(join([]act{raises("UTG", 2)}, rfiRest(), []act{folds("BB")}), "U"+strconv.Itoa(i), "BB"))
		text.WriteString(makeHand(join([]act{folds("UTG"), folds("HJ"), folds("CO"), folds("BTN"), raises("SB", 3)}, []act{folds("BB")}), "S"+strconv.Itoa(i), "BB"))
	}
	dir := filepath.Join(t.TempDir(), "h")
	write(t, filepath.Join(dir, "a.txt"), text.String())
	return dir
}

var devRowPattern = regexp.MustCompile(`(?m)^  (UTG|SB) \| `)

func TestTopLimitsRowsPerGroup(t *testing.T) {
	isolateEnv(t)
	hands := topHands(t)
	ranges := writeRanges(t, map[string]map[string]float64{"UTG Open 2bb": {"AA": 1}, "SB Open 3bb": {"AA": 1}})
	tmp := t.TempDir()
	// 預設 Hero 坐 BB：玩家池有 UTG、SB 兩列候選，Hero 沒有。Hero 為 p_UTG：玩家池只剩 SB，Hero 有 UTG。
	for _, tc := range []struct {
		hero, top string
		want      int
	}{{"Hero", "1", 1}, {"Hero", "5", 2}, {"Hero", "0", 0}, {"p_UTG", "1", 2}, {"p_UTG", "5", 2}} {
		out := filepath.Join(tmp, "o.html")
		r := runCLI(t, "--hands", hands, "--ranges", ranges, "-o", out, "--hero", tc.hero, "--top", tc.top)
		if r.code != 0 {
			t.Fatal(r.err)
		}
		if got := len(devRowPattern.FindAllString(r.stdout, -1)); got != tc.want {
			t.Errorf("hero %s --top %s: %d rows, want %d\n%s", tc.hero, tc.top, got, tc.want, r.stdout)
		}
		if page, _ := os.ReadFile(out); tc.top != "0" {
			mustContain(t, "html", string(page), `const DEV = {"pool":[`)
		}
	}
}

func TestDeviationNumbersAreSignedLikeTheReport(t *testing.T) {
	for v, want := range map[float64]string{0: "0.0", 0.04: "0.0", -0.04: "0.0", 1.26: "+1.3", -1.26: "-1.3", 100: "+100.0"} {
		if got := signed(v); got != want {
			t.Errorf("signed(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestJSONKeepsHTMLCharactersLikePython(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	jsonPath := filepath.Join(tmp, "m.json")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", filepath.Join(tmp, "o.html"), "--hero", "A&B<x>", "--json", jsonPath)
	if r.code != 0 {
		t.Fatal(r.err)
	}
	raw, _ := os.ReadFile(jsonPath)
	mustContain(t, "json", string(raw), `"hero": "A&B<x>"`)
	page, _ := os.ReadFile(filepath.Join(tmp, "o.html"))
	mustNotContain(t, "html", string(page), "A&B<x>")
	mustContain(t, "html", string(page), `A\u0026B\u003cx\u003e`)
}

func TestClampedTagUsesFullWidthParentheses(t *testing.T) {
	isolateEnv(t)
	ranges := writeRanges(t, map[string]map[string]float64{"UTG Open 2bb": {"AA": 1}})
	r := runCLI(t, "--hands", topHands(t), "--ranges", ranges, "-o", filepath.Join(t.TempDir(), "o.html"))
	mustContain(t, "stdout", r.stdout, "（理論值已調整）")
	en := runCLI(t, "--lang", "en", "--hands", topHands(t), "--ranges", ranges, "-o", filepath.Join(t.TempDir(), "o.html"))
	mustContain(t, "en stdout", en.stdout, " (theory adjusted)")
}

func TestNoHomeHintOnlyForRangesSource(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("PREFLOP_MATRIX_RANGES", "")
	r := runCLI(t, "--hands", handsDir(t))
	mustContain(t, "ranges stderr", r.err, "（請改用 --ranges 指定範圍資料夾）")
	r = runCLI(t, "--init-ranges", "~/x")
	mustContain(t, "init stderr", r.err, "無法決定使用者的家目錄或設定資料夾")
	mustNotContain(t, "init stderr", r.err, "--ranges")
	r = runCLI(t, "--hands", "~/h", "--ranges", standardRanges(t))
	mustNotContain(t, "hands stderr", r.err, "--ranges")
}
