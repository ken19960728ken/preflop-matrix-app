package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingFolderIsExit2(t *testing.T) {
	isolateEnv(t)
	for _, which := range []string{"hands", "ranges"} {
		hands, ranges := handsDir(t), standardRanges(t)
		tmp := t.TempDir()
		if which == "hands" {
			hands = filepath.Join(tmp, "nope")
		} else {
			ranges = filepath.Join(tmp, "nope")
		}
		out := filepath.Join(tmp, "o.html")
		r := runCLI(t, "--hands", hands, "--ranges", ranges, "-o", out)
		if r.code != 2 {
			t.Errorf("%s: exit %d", which, r.code)
		}
		mustContain(t, which+" stderr", r.err, "不存在")
		if fileExists(out) {
			t.Errorf("%s: html written", which)
		}
	}
}

func TestHandsFolderIsReportedBeforeRangesFolder(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	r := runCLI(t, "--hands", filepath.Join(tmp, "h"), "--ranges", filepath.Join(tmp, "r"))
	mustContain(t, "stderr", r.err, "手牌資料夾不存在")
	mustNotContain(t, "stderr", r.err, "理論範圍資料夾")
	mustNotContain(t, "stdout", r.stdout, "理論範圍來源")
}

func TestAFileInsteadOfAFolderIsExit2(t *testing.T) {
	isolateEnv(t)
	notADir := filepath.Join(t.TempDir(), "file.txt")
	write(t, notADir, "x")
	if r := runCLI(t, "--hands", notADir, "--ranges", standardRanges(t)); r.code != 2 {
		t.Errorf("exit %d", r.code)
	}
}

func TestNoHandsIsExit1(t *testing.T) {
	isolateEnv(t)
	empty, out := t.TempDir(), filepath.Join(t.TempDir(), "o.html")
	r := runCLI(t, "--hands", empty, "--ranges", standardRanges(t), "-o", out)
	if r.code != 1 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "沒有可用的手牌")
	if fileExists(out) {
		t.Error("html written")
	}
}

func TestOnlyUnparseableHandsIsExit1WithSummary(t *testing.T) {
	isolateEnv(t)
	folder := t.TempDir()
	write(t, filepath.Join(folder, "x.txt"), strings.Replace(makeHand(nil, "RC1", "BB"), "big blind", "huge blind", 1))
	r := runCLI(t, "--hands", folder, "--ranges", standardRanges(t), "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 1 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stdout", r.stdout, "無法解析略過 1")
	mustContain(t, "stderr", r.err, "沒有可用的手牌")
}

func TestEmptyRangesFolderIsExit1(t *testing.T) {
	isolateEnv(t)
	r := runCLI(t, "--hands", handsDir(t), "--ranges", t.TempDir(), "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 1 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "沒有 .txt 範圍檔")
}

func TestUnwritableOutputIsExit1(t *testing.T) {
	isolateEnv(t)
	blocker := filepath.Join(t.TempDir(), "blocker")
	write(t, blocker, "x")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", filepath.Join(blocker, "o.html"))
	if r.code != 1 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "無法寫入")
	mustNotContain(t, "stdout", r.stdout, "已輸出")
}

func TestUnwritableJSONIsExit1(t *testing.T) {
	isolateEnv(t)
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	write(t, blocker, "x")
	r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", filepath.Join(tmp, "o.html"), "--json", filepath.Join(blocker, "m.json"))
	if r.code != 1 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "無法寫入")
}

func TestRangesDefaultFolderIsUsedWhenOmitted(t *testing.T) {
	isolateEnv(t)
	cfg := filepath.Join(t.TempDir(), "cfg")
	t.Setenv("XDG_CONFIG_HOME", cfg)
	standard := standardRanges(t)
	def := filepath.Join(cfg, "preflop-matrix", "ranges")
	if err := os.CopyFS(def, os.DirFS(standard)); err != nil {
		t.Fatal(err)
	}
	r := runCLI(t, "--hands", handsDir(t), "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "預設資料夾", def)
}

func TestEnvVarBeatsDefaultAndFlagBeatsEnv(t *testing.T) {
	isolateEnv(t)
	hands, ranges, tmp := handsDir(t), standardRanges(t), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "nothing-here"))
	t.Setenv("PREFLOP_MATRIX_RANGES", ranges)
	r := runCLI(t, "--hands", hands, "-o", filepath.Join(tmp, "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "環境變數 PREFLOP_MATRIX_RANGES")
	t.Setenv("PREFLOP_MATRIX_RANGES", filepath.Join(tmp, "missing"))
	r = runCLI(t, "--hands", hands, "--ranges", ranges, "-o", filepath.Join(tmp, "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "參數 --ranges")
}

func TestMissingDefaultFolderExplainsHowToSetItUp(t *testing.T) {
	isolateEnv(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "cfg"))
	r := runCLI(t, "--hands", handsDir(t), "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 2 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "預設資料夾", "docs/RANGES.md")
}

func TestEnvVarPointingNowhereSaysToFixTheEnvVar(t *testing.T) {
	isolateEnv(t)
	t.Setenv("PREFLOP_MATRIX_RANGES", filepath.Join(t.TempDir(), "missing"))
	r := runCLI(t, "--hands", handsDir(t), "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 2 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "unset PREFLOP_MATRIX_RANGES")
}

func TestNoHomeIsExit2(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("PREFLOP_MATRIX_RANGES", "")
	r := runCLI(t, "--hands", handsDir(t))
	if r.code != 2 {
		t.Errorf("exit %d", r.code)
	}
	mustContain(t, "stderr", r.err, "無法決定使用者的家目錄或設定資料夾", "--ranges")
	t.Setenv("PREFLOP_MATRIX_RANGES", "~/x")
	if r := runCLI(t, "--hands", handsDir(t)); r.code != 2 {
		t.Errorf("env ~: exit %d", r.code)
	}
	if r := runCLI(t, "--hands", "~/h", "--ranges", standardRanges(t)); r.code != 2 {
		t.Errorf("hands ~: exit %d", r.code)
	}
	if r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "-o", "~/o.html"); r.code != 2 {
		t.Errorf("out ~: exit %d", r.code)
	}
	if r := runCLI(t, "--hands", handsDir(t), "--ranges", standardRanges(t), "--json", "~/o.json", "-o", filepath.Join(t.TempDir(), "o.html")); r.code != 2 {
		t.Errorf("json ~: exit %d", r.code)
	}
	if r := runCLI(t, "--init-ranges", "~/ranges"); r.code != 2 {
		t.Errorf("init ~: exit %d", r.code)
	}
}

func TestEmptyRangesFlagMeansCurrentDirectory(t *testing.T) {
	isolateEnv(t)
	hands, ranges := handsDir(t), standardRanges(t)
	t.Chdir(ranges)
	r := runCLI(t, "--hands", hands, "--ranges", "", "-o", filepath.Join(t.TempDir(), "o.html"))
	if r.code != 0 {
		t.Fatal(r.err)
	}
	mustContain(t, "stdout", r.stdout, "理論範圍來源：參數 --ranges → .\n")
}
