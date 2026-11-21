package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ken19960728ken/preflop-matrix-app/internal/paritydiff"
)

// Parity：對 Python 原版產生的 golden 逐值比對。golden 是在 repo 根目錄用相對路徑
// （testdata/parity/<corpus>/hands）產生的，所以測試先把語料放到某個資料夾下的 testdata/parity/<corpus>，
// 再 chdir 到該資料夾，用同樣的相對路徑執行，輸出的路徑便與 golden 完全相同，不需要正規化。
// posix_extras 含檔名帶 tab 與 symlink，用 paritydiff.Materialize 還原到暫存資料夾。

const parityRoot = "../../testdata/parity"

// stageCorpus 回傳一個資料夾，其中 testdata/parity/<name> 是可執行的語料。
func stageCorpus(t *testing.T, name string) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join(parityRoot, name))
	if err != nil {
		t.Fatal(err)
	}
	if name != "posix_extras" {
		root, err := filepath.Abs(filepath.Join(parityRoot, "..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		return root
	}
	root := t.TempDir()
	if err := paritydiff.Materialize(src, filepath.Join(root, "testdata", "parity", name)); err != nil {
		t.Fatal(err)
	}
	return root
}

func golden(t *testing.T, name, file string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(parityRoot, name, file))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// untilOutputLine 回傳 stdout 到「已輸出：」行（含）為止的部分；其後是 Go 版新增的偏離摘要，Python 沒有。
func untilOutputLine(stdout string) string {
	i := strings.Index(stdout, "已輸出：")
	if i < 0 {
		return stdout
	}
	j := strings.Index(stdout[i:], "\n")
	return stdout[:i+j+1]
}

func TestParityWithPythonGolden(t *testing.T) {
	for _, name := range []string{"handcrafted", "random_s42", "posix_extras"} {
		t.Run(name, func(t *testing.T) {
			isolateEnv(t)
			goldenJSON, goldenStdout, goldenStderr := golden(t, name, "golden.json"), golden(t, name, "golden.stdout.txt"), golden(t, name, "golden.stderr.txt")
			t.Chdir(stageCorpus(t, name))
			base := filepath.Join("testdata", "parity", name)
			tmp := t.TempDir()
			jsonPath := filepath.Join(tmp, "out.json")
			args := []string{"--hands", filepath.Join(base, "hands"), "--ranges", filepath.Join(base, "ranges"),
				"-o", filepath.Join(tmp, "out.html"), "--json", jsonPath, "--lang", "zh-Hant"}

			r := runCLI(t, args...)
			if r.code != 0 {
				t.Fatalf("exit %d\nstderr: %s", r.code, r.err)
			}
			gotJSON, err := os.ReadFile(jsonPath)
			if err != nil {
				t.Fatal(err)
			}
			diffs, err := paritydiff.Compare(gotJSON, []byte(goldenJSON))
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range diffs {
				t.Errorf("json: %s", d)
			}
			for _, d := range paritydiff.CompareSummary(untilOutputLine(r.stdout), goldenStdout) {
				t.Errorf("stdout: %s", d)
			}
			if r.err != goldenStderr {
				t.Errorf("stderr differs\n got: %q\nwant: %q", r.err, goldenStderr)
			}

			top0 := runCLI(t, append(args, "--top", "0")...)
			for _, d := range paritydiff.CompareSummary(top0.stdout, goldenStdout) {
				t.Errorf("stdout with --top 0: %s", d)
			}
		})
	}
}
