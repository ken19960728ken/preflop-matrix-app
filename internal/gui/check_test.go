package gui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
)

func TestCheckRangesCompleteFolder(t *testing.T) {
	e := newEnv(t, withCorpus(true))
	v := e.svc.CheckRanges()
	if !v.OK || v.Error != "" || !v.Complete || v.SlotsFilled != v.SlotsTotal || v.SlotsTotal != 95 || v.StandardCount != 108 || v.Mapped != 108 {
		t.Errorf("check = %+v", v)
	}
	if len(v.MissingNodes)+len(v.MissingActions)+len(v.Unreadable)+len(v.Ignored) != 0 {
		t.Errorf("complete folder lists problems: %+v", v)
	}
	if v.MissingNodes == nil || v.Unreadable == nil || v.UnfilledMissing == nil {
		t.Error("lists must be empty arrays, not nil")
	}
}

// writeProblemFolder 建立一個有各種問題的範圍資料夾：缺格、忽略、無法讀取、未填。
func writeProblemFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"UTG open.txt": "AsAh: 1", "BB vs UTG open call.txt": "AsAh: 1", "BB vs UTG open 3bet.txt": "AsAh: 1",
		"UTG vs BB 3bet call.txt": "", "HJ vs BB 3bet call.txt": "AsAh: 1",
		"notes.txt": "hello", "UTG open copy.txt": "x", "BTN vs SB 3bet call.txt": "\xff\xfe",
		"UTG vs HJ open call.txt": "AsAh: 1", "SB vs BTN open call.txt": "AsAh: 1",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckRangesProblems(t *testing.T) {
	e := newEnv(t, func(st *config.Settings) { st.RangesDir = writeProblemFolder(t) })
	v := e.svc.CheckRanges()
	if !v.OK || v.Complete || v.SlotsFilled == 0 || v.SlotsFilled >= v.SlotsTotal {
		t.Fatalf("check = %+v", v)
	}
	if len(v.Ignored) == 0 {
		t.Errorf("ignored = %v", v.Ignored)
	}
	if len(v.Unreadable) != 1 || v.Unreadable[0].Name != "BTN vs SB 3bet call.txt" || v.Unreadable[0].Reason == "" {
		t.Errorf("unreadable = %+v", v.Unreadable)
	}
	if len(v.UnfilledMissing)+len(v.UnfilledRedundant) != 1 {
		t.Errorf("unfilled = %v / %v", v.UnfilledMissing, v.UnfilledRedundant)
	}
}

func TestCheckRangesMissingNodesListFileNames(t *testing.T) {
	e := newEnv(t, func(st *config.Settings) { st.RangesDir = writeProblemFolder(t) })
	v := e.svc.CheckRanges()
	if len(v.MissingNodes) == 0 || v.MissingNodes[0].NodeLabel == "" || len(v.MissingNodes[0].Cells) == 0 {
		t.Fatalf("missing nodes = %+v", v.MissingNodes)
	}
	for _, n := range v.MissingNodes {
		for _, c := range n.Cells {
			if c.Actions == nil {
				t.Errorf("cell %+v has nil actions", c)
			}
			for _, name := range append([]string{c.Base}, c.Actions...) {
				if name != "" && !strings.HasSuffix(name, ".txt") {
					t.Errorf("file name without .txt: %q", name)
				}
			}
		}
	}
}

func TestCheckRangesMissingActionTextAndNames(t *testing.T) {
	ranges := t.TempDir()
	for _, name := range []string{"UTG open", "BB vs UTG open call"} {
		if err := os.WriteFile(filepath.Join(ranges, name+".txt"), []byte("AsAh: 1"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e := newEnv(t, func(st *config.Settings) { st.RangesDir = ranges })
	v := e.svc.CheckRanges()
	if len(v.MissingActions) != 1 || v.MissingActions[0].Text == "" || len(v.MissingActions[0].Names) == 0 ||
		v.MissingActions[0].Names[0] != "BB vs UTG open 3bet.txt" {
		t.Errorf("missing actions = %+v", v.MissingActions)
	}
}

func TestCheckRangesErrors(t *testing.T) {
	empty := t.TempDir()
	cases := []struct {
		name string
		mut  func(*config.Settings)
		want string
	}{
		{"not selected", func(st *config.Settings) {}, "尚未選擇理論範圍資料夾"},
		{"missing", func(st *config.Settings) { st.RangesDir = filepath.Join(empty, "gone") }, "已選擇的理論範圍資料夾不存在"},
		{"no txt", func(st *config.Settings) { st.RangesDir = empty }, "底下沒有 .txt 範圍檔"},
	}
	for _, c := range cases {
		v := newEnv(t, c.mut).svc.CheckRanges()
		if v.OK || !strings.Contains(v.Error, c.want) || v.MissingNodes == nil {
			t.Errorf("%s: %+v", c.name, v)
		}
	}
}

func TestInitRangeTemplateCancelAndErrors(t *testing.T) {
	e := newEnv(t, nil)
	if v := e.svc.InitRangeTemplate(); !v.Cancelled || v.OK || v.Error != "" || v.Mapped == nil {
		t.Errorf("cancel = %+v", v)
	}
	e.host.pickErr = errors.New("no window")
	if v := e.svc.InitRangeTemplate(); v.OK || !strings.Contains(v.Error, "no window") {
		t.Errorf("dialog error = %+v", v)
	}
	e.host.pickErr = nil
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	e.host.pickPath = filepath.Join(blocker, "sub")
	if v := e.svc.InitRangeTemplate(); v.OK || v.Error == "" || v.Dir != e.host.pickPath {
		t.Errorf("write failure = %+v", v)
	}
}

func TestInitRangeTemplateCreatesStandardSet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "ranges")
	e := newEnv(t, nil)
	e.host.pickPath = dir

	v := e.svc.InitRangeTemplate()
	if !v.OK || v.Dir != dir || v.Created != len(engine.StandardSet()) || v.Skipped != 0 || v.HowTo != "created" || len(v.Mapped) != 0 {
		t.Fatalf("first run = %+v", v)
	}
	if _, err := os.Stat(filepath.Join(dir, engine.HowToFillName)); err != nil {
		t.Error(err)
	}
	if got := e.settings(t).RangesDir; got != "" {
		t.Errorf("InitRangeTemplate must not change the ranges folder, got %q", got)
	}

	v = e.svc.InitRangeTemplate()
	if !v.OK || v.Created != 0 || v.Skipped != len(engine.StandardSet()) || v.HowTo != "exists" {
		t.Errorf("second run = %+v", v)
	}
}

func TestInitRangeTemplateReportsMappedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "utg open.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, func(st *config.Settings) { st.RangesDir = dir })
	e.host.pickPath = dir
	v := e.svc.InitRangeTemplate()
	if !v.OK || len(v.Mapped) != 1 || v.Mapped[0].Name != "UTG open.txt" || v.Mapped[0].Existing != "utg open.txt" {
		t.Errorf("mapped = %+v", v.Mapped)
	}
	if got := e.host.picks[0].start; got != dir {
		t.Errorf("dialog start = %q, want the current ranges folder", got)
	}
}
