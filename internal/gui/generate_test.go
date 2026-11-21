package gui

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
)

func TestGenerateWithoutRangesGivesNotice(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	v := e.svc.Generate()
	if !v.OK || v.Error != "" {
		t.Fatalf("Generate = %+v", v)
	}
	if v.Notice != "未選擇理論範圍：理論值均顯示為 –" {
		t.Errorf("notice = %q", v.Notice)
	}
	if len(v.Summary) < 2 || !strings.HasPrefix(v.Summary[0], "手牌：掃描 4 個檔案，採用 ") {
		t.Errorf("summary = %q", v.Summary)
	}
	if len(v.Deviations.Pool)+len(v.Deviations.Me) != 0 {
		t.Errorf("deviations without theory: %+v", v.Deviations)
	}
	if v.Warnings == nil || v.Deviations.Pool == nil || v.Deviations.Me == nil {
		t.Error("lists must be empty arrays, not nil")
	}
	if v.ReportURL != "/report/current.html?embed=1&lang=zh-Hant&v=1" {
		t.Errorf("reportURL = %q", v.ReportURL)
	}
	if st := e.svc.GetState(); !st.HasReport || st.Busy {
		t.Errorf("state = %+v", st)
	}
}

func TestGenerateWithCompleteRanges(t *testing.T) {
	e := newEnv(t, withCorpus(true))
	v := e.svc.Generate()
	if !v.OK {
		t.Fatalf("Generate = %+v", v)
	}
	if v.Notice != "" {
		t.Errorf("complete ranges should have no notice: %q", v.Notice)
	}
	if last := v.Summary[len(v.Summary)-1]; !strings.Contains(last, "理論範圍：108 個檔案，對應到節點 108 個") {
		t.Errorf("ranges summary = %q", last)
	}
	if len(v.Deviations.Pool) == 0 || len(v.Deviations.Pool) > 5 || len(v.Deviations.Me) > 5 {
		t.Fatalf("deviation rows: pool %d, me %d", len(v.Deviations.Pool), len(v.Deviations.Me))
	}
	for _, r := range v.Deviations.Pool {
		if !strings.HasSuffix(r.Obs, "%") || !strings.HasSuffix(r.Theory, "%") || r.Z == "" || r.N == "" || !strings.HasPrefix(r.AnchorID, "cell-") {
			t.Errorf("row = %+v", r)
		}
		if strings.ContainsAny(r.Diff, "eE") || (r.Diff[0] != '+' && r.Diff[0] != '-' && r.Diff != "0.0") {
			t.Errorf("diff = %q", r.Diff)
		}
	}
}

func TestGenerateTranslatesEverything(t *testing.T) {
	light := withLight(t, "")
	zh := newEnv(t, light).svc.Generate()
	en := newEnv(t, func(st *config.Settings) { light(st); st.Lang = "en" }).svc.Generate()
	if zh.Summary[0] == en.Summary[0] || zh.Notice == en.Notice {
		t.Errorf("zh and en views are identical: %+v / %+v", zh.Summary[0], en.Summary[0])
	}
	if !strings.Contains(en.Summary[0], "Hands: scanned 4 file(s)") {
		t.Errorf("en summary = %q", en.Summary[0])
	}
	if !strings.Contains(en.ReportURL, "lang=en") {
		t.Errorf("reportURL = %q", en.ReportURL)
	}
}

func TestGenerateExpectedFailures(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	empty := t.TempDir()
	cases := []struct {
		name string
		mut  func(*config.Settings)
		want string
	}{
		{"no hands dir", func(st *config.Settings) {}, "尚未選擇手牌資料夾"},
		{"hands dir missing", func(st *config.Settings) { st.HandsDir = gone }, "手牌資料夾不存在"},
		{"hands dir is a file", func(st *config.Settings) { st.HandsDir = filepath.Join(corpusHands, "..", "golden.json") }, "手牌資料夾不存在或不是資料夾"},
		{"no hands inside", func(st *config.Settings) { st.HandsDir = empty }, "底下沒有可用的手牌"},
		{"ranges missing", func(st *config.Settings) { withCorpus(false)(st); st.RangesDir = gone }, "已選擇的理論範圍資料夾不存在"},
		{"ranges without txt", func(st *config.Settings) { withCorpus(false)(st); st.RangesDir = empty }, "底下沒有 .txt 範圍檔"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, c.mut)
			v := e.svc.Generate()
			if v.OK || !strings.Contains(v.Error, c.want) || v.ReportURL != "" {
				t.Errorf("Generate = %+v, want error containing %q", v, c.want)
			}
			if e.svc.GetState().HasReport {
				t.Error("a failed Generate must not produce a report")
			}
			if v.Summary == nil || v.Warnings == nil {
				t.Error("lists must be empty arrays, not nil")
			}
		})
	}
}

func TestFailedGenerateKeepsPreviousReport(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	if v := e.svc.Generate(); !v.OK {
		t.Fatal(v)
	}
	if _, err := e.svc.update(func(st *config.Settings) { st.RangesDir = filepath.Join(t.TempDir(), "gone") }); err != nil {
		t.Fatal(err)
	}
	if v := e.svc.Generate(); v.OK {
		t.Fatalf("expected an error: %+v", v)
	}
	if v := e.svc.GetReportView(); !v.OK || !strings.HasSuffix(v.ReportURL, "&v=1") {
		t.Errorf("previous report lost: %+v", v)
	}
}

func TestGenerateRejectsReentry(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	e.svc.busy = true
	v := e.svc.Generate()
	if v.OK || !strings.Contains(v.Error, "正在產生報告") {
		t.Errorf("busy Generate = %+v", v)
	}
	if !e.svc.GetState().Busy {
		t.Error("busy flag was cleared by the rejected call")
	}
	e.svc.busy = false
	if v := e.svc.Generate(); !v.OK {
		t.Errorf("Generate after busy = %+v", v)
	}
}

func TestGenerateConcurrentCalls(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	var wg sync.WaitGroup
	views := make([]ReportView, 6)
	for i := range views {
		wg.Add(1)
		go func() { defer wg.Done(); views[i] = e.svc.Generate() }()
	}
	wg.Wait()
	ok := 0
	for _, v := range views {
		switch {
		case v.OK:
			ok++
		case !strings.Contains(v.Error, "正在產生報告"):
			t.Errorf("unexpected error %q", v.Error)
		}
	}
	if ok == 0 {
		t.Error("no Generate succeeded")
	}
	if e.svc.GetState().Busy {
		t.Error("still busy after all calls returned")
	}
}

func TestGenerateStopsWhenContextIsCancelled(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	ctx, cancel := context.WithCancel(context.Background())
	if err := e.svc.ServiceStartup(ctx, applicationOptions); err != nil {
		t.Fatal(err)
	}
	cancel()
	v := e.svc.Generate()
	if v.OK || v.Error != "已取消" {
		t.Errorf("Generate = %+v", v)
	}
	if st := e.svc.GetState(); st.HasReport || st.Busy {
		t.Errorf("state = %+v", st)
	}
	if v := e.svc.GetReportView(); v.OK {
		t.Error("cancelled Generate produced a report")
	}
}

func TestSwitchingLanguageDoesNotReload(t *testing.T) {
	light := withLight(t, "")
	e := newEnv(t, light)
	hands := e.settings(t).HandsDir
	first := e.svc.Generate()
	if !first.OK {
		t.Fatal(first)
	}
	if err := os.RemoveAll(hands); err != nil {
		t.Fatal(err)
	}
	e.svc.SetLanguage("en")
	v := e.svc.GetReportView()
	if !v.OK || !strings.Contains(v.Notice, "No theory ranges selected") || !strings.Contains(v.Summary[0], "Hands: scanned") {
		t.Errorf("view after language switch = %+v", v)
	}
	if v.ReportURL != "/report/current.html?embed=1&lang=en&v=1" {
		t.Errorf("reportURL = %q", v.ReportURL)
	}
}

func TestGetReportViewWithoutReport(t *testing.T) {
	v := newEnv(t, nil).svc.GetReportView()
	if v.OK || v.Error != "尚未產生報告" {
		t.Errorf("view = %+v", v)
	}
}

func TestSetTopNOnlyRecomputesDeviations(t *testing.T) {
	hands := t.TempDir()
	if err := os.CopyFS(hands, os.DirFS(corpusHands)); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, func(st *config.Settings) { withCorpus(true)(st); st.HandsDir = hands; st.TopN = 20 })
	big := e.svc.Generate()
	if !big.OK || len(big.Deviations.Pool) < 3 {
		t.Fatalf("need at least 3 pool rows, got %+v", big)
	}
	if err := os.RemoveAll(hands); err != nil {
		t.Fatal(err)
	}
	html20 := e.get(t, "/current.html?lang=zh-Hant").Body.String()

	if r := e.svc.SetTopN(2); !r.OK {
		t.Fatal(r)
	}
	v := e.svc.GetReportView()
	if len(v.Deviations.Pool) != 2 || len(v.Deviations.Me) > 2 || !v.OK {
		t.Fatalf("topN 2: pool %d me %d", len(v.Deviations.Pool), len(v.Deviations.Me))
	}
	for i, r := range v.Deviations.Pool {
		if r != big.Deviations.Pool[i] {
			t.Errorf("row %d changed: %+v != %+v", i, r, big.Deviations.Pool[i])
		}
	}
	if !strings.HasSuffix(v.ReportURL, "&v=2") {
		t.Errorf("reportURL must change so the iframe reloads: %q", v.ReportURL)
	}
	if html2 := e.get(t, "/current.html?lang=zh-Hant").Body.String(); html2 == html20 {
		t.Error("the rendered report still has the old deviation summary")
	}

	e.svc.SetTopN(0)
	if v := e.svc.GetReportView(); len(v.Deviations.Pool)+len(v.Deviations.Me) != 0 {
		t.Errorf("topN 0 still has rows: %+v", v.Deviations)
	}
	if html0 := e.get(t, "/current.html?lang=zh-Hant").Body.String(); strings.Contains(html0, `"anchorID"`) {
		t.Error("topN 0 report still injects deviations")
	}
}

func TestNoHeroWarning(t *testing.T) {
	light := withLight(t, "")
	e := newEnv(t, func(st *config.Settings) { light(st); st.Hero = "NobodyHere" })
	v := e.svc.Generate()
	if !v.OK {
		t.Fatal(v)
	}
	found := false
	for _, w := range v.Warnings {
		found = found || strings.Contains(w, "NobodyHere")
	}
	if !found {
		t.Errorf("warnings = %q", v.Warnings)
	}
}

func TestUnreadableRangeFileIsAWarning(t *testing.T) {
	ranges := t.TempDir()
	if err := os.WriteFile(filepath.Join(ranges, "UTG open.txt"), []byte{0xff, 0xfe, 'x'}, 0o600); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, withLight(t, ranges))
	v := e.svc.Generate()
	if !v.OK {
		t.Fatal(v)
	}
	found := false
	for _, w := range v.Warnings {
		found = found || strings.Contains(w, "UTG open.txt")
	}
	if !found {
		t.Errorf("warnings = %q", v.Warnings)
	}
}

func TestIncompleteRangesNotice(t *testing.T) {
	ranges := t.TempDir()
	src := os.DirFS(corpusRanges)
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries[:20] {
		b, err := fs.ReadFile(src, ent.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ranges, ent.Name()), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e := newEnv(t, withLight(t, ranges))
	v := e.svc.Generate()
	if !v.OK || !strings.Contains(v.Notice, "理論範圍不完整：缺") {
		t.Errorf("view = %+v", v)
	}
}

func TestDevRowsFormatting(t *testing.T) {
	rows := devRows("zh-Hant", []engine.Deviation{
		{Node: "open", Decider: "UTG", Action: "r", Obs: 0.2, Theory: 0.1234, DiffPP: 7.66, N: 120, Z: 3.04, AnchorID: "cell-open-UTG-pool"},
		{Node: "vs_3bet", Decider: "CO", Opponent: "BTN", Action: "f", Obs: 0.5, Theory: 0.5, DiffPP: -0.04, N: 30, Z: -3.25, Clamped: true},
		{Node: "vs_open", Decider: "BB", Opponent: "CO", Action: "r", Obs: 0.1, Theory: 0.2, DiffPP: -10, N: 99, Z: 3},
		{Node: "vs_4bet", Decider: "BB", Opponent: "CO", Action: "c", Obs: 0.1, Theory: 0.2, DiffPP: 0.25, N: 99, Z: -0.04},
	})
	want := []DevRow{
		{"UTG", "開池", "開池", "20.0%", "12.3%", "+7.7", "120", "+3.0", false, "cell-open-UTG-pool"},
		{"CO vs BTN", "面對 3bet", "棄牌", "50.0%", "50.0%", "0.0", "30", "-3.3", true, ""},
		{"BB vs CO", "面對開池", "3bet", "10.0%", "20.0%", "-10.0", "99", "+3.0", false, ""},
		{"BB vs CO", "面對 4bet", "跟注", "10.0%", "20.0%", "+0.3", "99", "0.0", false, ""},
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
	if got := devRows("en", nil); got == nil || len(got) != 0 {
		t.Errorf("devRows(nil) = %#v", got)
	}
	if got := devRows("en", []engine.Deviation{{Node: "vs_4bet", Decider: "BB", Opponent: "CO", Action: "r"}}); got[0].Action != "5bet" {
		t.Errorf("action = %q", got[0].Action)
	}
}

func TestDevActionLabelUnknownKeepsCode(t *testing.T) {
	if got := devActionLabel("en", engine.Deviation{Node: "vs_5bet", Action: "r"}); got != "r" {
		t.Errorf("label = %q", got)
	}
}

func TestSettingsChangedDuringGenerateIsWarned(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	e.svc.beforeFinish = func() { e.svc.SetHero("Changed") }
	v := e.svc.Generate()
	if !v.OK || !slices.ContainsFunc(v.Warnings, func(w string) bool { return strings.Contains(w, "設定已在產生期間變更") }) {
		t.Errorf("view = %+v", v)
	}
	if again := e.svc.GetReportView(); !slices.Equal(again.Warnings, v.Warnings) {
		t.Errorf("GetReportView lost the warning: %q", again.Warnings)
	}
}

func TestConcurrentGenerateAndSettingsChanges(t *testing.T) {
	e := newEnv(t, withLight(t, ""))
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 6; i++ {
				fn()
			}
		}()
	}
	run(func() { e.svc.Generate() })
	run(func() { e.svc.SetTopN(2); e.svc.SetTopN(7) })
	run(func() { e.svc.SetLanguage("en"); e.svc.SetLanguage("zh-Hans") })
	run(func() { e.svc.ClearRangesDir(); e.svc.SetHero("Hero") })
	run(func() { e.get(t, "/current.html?lang=en"); e.get(t, "/current.html") })
	run(func() { e.svc.GetReportView(); e.svc.GetState(); e.svc.CheckRanges() })
	wg.Wait()
	if st := e.svc.GetState(); st.Busy || !st.HasReport {
		t.Errorf("state = %+v", st)
	}
	if rec := e.get(t, "/current.html?lang=zh-Hant"); rec.Code != 200 {
		t.Errorf("status = %d", rec.Code)
	}
}
