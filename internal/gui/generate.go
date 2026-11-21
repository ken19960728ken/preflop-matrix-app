package gui

import (
	"context"
	"errors"
	"os"

	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
	"github.com/ken19960728ken/preflop-matrix-app/internal/numfmt"
	"github.com/ken19960728ken/preflop-matrix-app/internal/pipeline"
)

// DevRow 是偏離摘要的一列；數字已格式化成與報告頁相同的字串（百分比與差、z 為一位小數）。
type DevRow struct {
	Spot     string `json:"spot"`
	Node     string `json:"node"`
	Action   string `json:"action"`
	Obs      string `json:"obs"`
	Theory   string `json:"theory"`
	Diff     string `json:"diff"`
	N        string `json:"n"`
	Z        string `json:"z"`
	Clamped  bool   `json:"clamped"`
	AnchorID string `json:"anchorID"`
}

// DevGroups 是玩家池與 Hero 兩組偏離列。
type DevGroups struct {
	Pool []DevRow `json:"pool"`
	Me   []DevRow `json:"me"`
}

// ReportView 是報告的畫面資料；Error 非空時其他欄位無意義。
type ReportView struct {
	OK         bool      `json:"ok"`
	Error      string    `json:"error"`
	Summary    []string  `json:"summary"`
	Warnings   []string  `json:"warnings"`
	Notice     string    `json:"notice"`
	Deviations DevGroups `json:"deviations"`
	ReportURL  string    `json:"reportURL"`
}

// failure 是預期內的失敗：訊息 key 與參數，等回傳時才用當時的語言翻譯。
type failure struct {
	key string
	kv  []string
}

func errView(lang string, f failure) ReportView {
	return ReportView{Error: tr(lang, f.key, f.kv...), Summary: []string{}, Warnings: []string{}, Deviations: noDevs()}
}

func noDevs() DevGroups { return DevGroups{Pool: []DevRow{}, Me: []DevRow{}} }

// Generate 用設定裡的資料夾產生報告並保存結果（供切語言、SetTopN、存檔使用）。同時間只有一個 Generate，
// 進行中再呼叫回傳 busy 錯誤。沒選範圍或範圍不完整仍產生報告；只有選過的範圍資料夾不存在才是錯誤。
func (s *AppService) Generate() ReportView {
	s.mu.Lock()
	if s.busy {
		lang := s.langOf(s.loadLocked())
		s.mu.Unlock()
		return errView(lang, failure{key: "gui.error.busy"})
	}
	s.busy = true
	st := s.loadLocked()
	ctx := s.ctx
	s.mu.Unlock()
	defer s.endBusy()

	res, fl := build(ctx, st.HandsDir, st.RangesDir, st.Hero, st.TopN)
	if s.beforeFinish != nil {
		s.beforeFinish()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.loadLocked()
	lang := s.langOf(cur)
	if fl != nil {
		return errView(lang, *fl)
	}
	if err := res.SetTopN(cur.TopN); err != nil {
		return errView(lang, failure{"gui.error.internal", []string{"err", err.Error()}})
	}
	changed := cur.HandsDir != st.HandsDir || cur.RangesDir != st.RangesDir || cur.Hero != st.Hero
	s.rep = report{res: res, topN: cur.TopN, seq: s.rep.seq + 1, changed: changed}
	return s.viewLocked(lang)
}

func (s *AppService) endBusy() {
	s.mu.Lock()
	s.busy = false
	s.mu.Unlock()
}

// build 載入手牌與範圍並建出結果；預期內的失敗以 failure 回傳。
func build(ctx context.Context, handsDir, rangesDir, hero string, topN int) (*pipeline.Result, *failure) {
	if handsDir == "" {
		return nil, &failure{key: "gui.error.no_hands_dir"}
	}
	if !isDir(handsDir) {
		return nil, &failure{"gui.error.hands_dir", []string{"path", handsDir}}
	}
	in := pipeline.Input{Hands: os.DirFS(handsDir), Hero: hero, TopN: topN}
	if rangesDir != "" {
		if !isDir(rangesDir) {
			return nil, &failure{"gui.error.ranges_missing", []string{"path", rangesDir}}
		}
		in.Ranges = os.DirFS(rangesDir)
	}
	res, err := pipeline.Build(ctx, in)
	switch {
	case err == nil:
		return res, nil
	case errors.Is(err, pipeline.ErrNoHands):
		return nil, &failure{"gui.error.no_hands", []string{"dir", handsDir}}
	case errors.Is(err, pipeline.ErrNoRanges):
		return nil, &failure{"gui.error.no_ranges_files", []string{"dir", rangesDir}}
	case errors.Is(err, context.Canceled):
		return nil, &failure{key: "gui.error.cancelled"}
	}
	return nil, &failure{"gui.error.internal", []string{"err", err.Error()}}
}

// GetReportView 以目前語言重新翻譯上次的報告結果（切語言後呼叫），不重新載入任何檔案。
func (s *AppService) GetReportView() ReportView {
	s.mu.Lock()
	defer s.mu.Unlock()
	lang := s.langOf(s.loadLocked())
	if s.rep.res == nil {
		return errView(lang, failure{key: "gui.error.no_report"})
	}
	return s.viewLocked(lang)
}

// viewLocked 由保存的結果組出畫面資料；呼叫者持有 mu 且 s.rep.res 非 nil。
func (s *AppService) viewLocked(lang string) ReportView {
	res := s.rep.res
	v := ReportView{
		OK:        true,
		Summary:   summaryLines(lang, res),
		Warnings:  warningLines(lang, res, s.rep.changed),
		ReportURL: "/report/current.html?embed=1&lang=" + lang + "&v=" + itoa(s.rep.seq),
		Deviations: DevGroups{
			Pool: devRows(lang, res.Deviations.Pool),
			Me:   devRows(lang, res.Deviations.Me),
		},
	}
	if n := pipeline.Notice(res); n != nil {
		v.Notice = i18n.T(lang, n.Key, n.Args)
	}
	return v
}

// summaryLines 是手牌與範圍的載入摘要。
func summaryLines(lang string, res *pipeline.Result) []string {
	l := res.Loaded
	out := []string{tr(lang, "gui.summary.hands",
		"files", itoa(l.Files), "hands", itoa(len(l.Hands)), "duplicates", itoa(l.Duplicates),
		"not_six_max", itoa(l.WrongTableSize), "unparseable", itoa(l.Unparseable), "unreadable", itoa(l.UnreadableFiles))}
	for _, e := range l.ErrorSamples {
		out = append(out, tr(lang, "gui.summary.error_sample", "sample", e.File+": "+pipeline.Translator(lang)(e.Msg)))
	}
	m := res.Data.Meta
	out = append(out, tr(lang, "gui.summary.matrix", "hands", itoa(m.Hands), "hero", m.Hero, "heroHands", itoa(m.HeroHands),
		"from", m.DateFrom, "to", m.DateTo))
	if b := res.Book; b != nil {
		out = append(out, tr(lang, "gui.summary.ranges", "files", itoa(b.Files), "nodes", itoa(len(b.Ranges)),
			"ignored", itoa(len(b.Ignored)), "duplicates", itoa(len(b.Duplicates)),
			"unreadable", itoa(len(b.Unreadable)), "unfilled", itoa(len(b.Unfilled))))
	}
	return out
}

// warningLines 是資料警告、沒有 Hero 手牌的提醒與讀不了的範圍檔。
func warningLines(lang string, res *pipeline.Result, changed bool) []string {
	out := []string{}
	t := pipeline.Translator(lang)
	for _, m := range res.Warnings {
		out = append(out, t(m))
	}
	if res.NoHero {
		out = append(out, tr(lang, "gui.warning.no_hero", "hero", res.Data.Meta.Hero))
	}
	if changed {
		out = append(out, tr(lang, "gui.warning.settings_changed"))
	}
	if res.Book != nil {
		for _, u := range res.Book.Unreadable {
			out = append(out, tr(lang, "gui.warning.unreadable", "name", u.Name, "reason", t(u.Msg)))
		}
	}
	return out
}

func devRows(lang string, rows []engine.Deviation) []DevRow {
	out := make([]DevRow, len(rows))
	for i, d := range rows {
		spot := d.Decider
		if d.Opponent != "" {
			spot += " vs " + d.Opponent
		}
		out[i] = DevRow{
			Spot: spot, Node: i18n.T(lang, "report.dev.node."+d.Node, nil), Action: devActionLabel(lang, d),
			Obs: numfmt.Percent(d.Obs), Theory: numfmt.Percent(d.Theory), Diff: numfmt.Signed(d.DiffPP), N: itoa(d.N), Z: numfmt.Signed(d.Z),
			Clamped: d.Clamped, AnchorID: d.AnchorID,
		}
	}
	return out
}

var raiseLabels = map[string]string{"vs_open": "3bet", "vs_3bet": "4bet", "vs_4bet": "5bet"}

// devActionLabel 同報告頁的 actionLabel。
func devActionLabel(lang string, d engine.Deviation) string {
	switch {
	case d.Action == engine.DevFold:
		return i18n.T(lang, "report.act.fold", nil)
	case d.Action == engine.DevCall:
		return i18n.T(lang, "report.act.call", nil)
	case d.Node == "open":
		return i18n.T(lang, "report.act.open", nil)
	}
	if label, ok := raiseLabels[d.Node]; ok {
		return label
	}
	return d.Action
}
