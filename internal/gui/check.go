package gui

import (
	"os"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
	"github.com/ken19960728ken/preflop-matrix-app/internal/pipeline"
)

// MissingCell 是一個缺的格子；Base 是缺的底範圍檔名（沒缺時為空），Actions 是缺的動作檔名（含 .txt）。
type MissingCell struct {
	Cell    string   `json:"cell"`
	Base    string   `json:"base"`
	Actions []string `json:"actions"`
}

// MissingNode 是同一個節點的缺格。
type MissingNode struct {
	Node      string        `json:"node"`
	NodeLabel string        `json:"nodeLabel"`
	Cells     []MissingCell `json:"cells"`
}

// MissingActionView 是只有一個動作的格子；缺的那個動作當 0。Names 是可補的檔名（含 .txt）。
type MissingActionView struct {
	Text  string   `json:"text"`
	Names []string `json:"names"`
}

// UnreadableView 是讀不了的範圍檔與原因。
type UnreadableView struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// CheckView 是範圍完整度檢查的結果；Error 非空時其他欄位無意義。
type CheckView struct {
	OK                bool                `json:"ok"`
	Error             string              `json:"error"`
	SlotsFilled       int                 `json:"slotsFilled"`
	SlotsTotal        int                 `json:"slotsTotal"`
	Mapped            int                 `json:"mapped"`
	StandardCount     int                 `json:"standardCount"`
	Complete          bool                `json:"complete"`
	MissingNodes      []MissingNode       `json:"missingNodes"`
	MissingActions    []MissingActionView `json:"missingActions"`
	UnfilledRedundant []string            `json:"unfilledRedundant"`
	UnfilledMissing   []string            `json:"unfilledMissing"`
	Ignored           []string            `json:"ignored"`
	Duplicates        []string            `json:"duplicates"`
	Unreadable        []UnreadableView    `json:"unreadable"`
}

// MappedSkip 是因為已有對應檔而沒有建立的樣板檔。
type MappedSkip struct {
	Name     string `json:"name"`
	Existing string `json:"existing"`
}

// InitView 是建立範圍檔名樣板的結果。Created、Skipped 是 .txt 的數量；HowTo 是 "created"、"exists" 或空字串。
type InitView struct {
	OK        bool         `json:"ok"`
	Cancelled bool         `json:"cancelled"`
	Error     string       `json:"error"`
	Dir       string       `json:"dir"`
	Created   int          `json:"created"`
	Skipped   int          `json:"skipped"`
	Mapped    []MappedSkip `json:"mapped"`
	HowTo     string       `json:"howTo"`
}

// emptyCheck 的清單都是空陣列（不是 nil），前端不必處理 null。
func emptyCheck() CheckView {
	return CheckView{MissingNodes: []MissingNode{}, MissingActions: []MissingActionView{},
		UnfilledRedundant: []string{}, UnfilledMissing: []string{}, Ignored: []string{}, Duplicates: []string{},
		Unreadable: []UnreadableView{}}
}

func checkErr(lang, key string, kv ...string) CheckView {
	v := emptyCheck()
	v.Error = tr(lang, key, kv...)
	return v
}

// CheckRanges 檢查設定裡的理論範圍資料夾（只讀）；沒選或資料夾不存在回 Error。
func (s *AppService) CheckRanges() CheckView {
	sn := s.snap()
	dir := sn.st.RangesDir
	switch {
	case dir == "":
		return checkErr(sn.lang, "gui.error.no_ranges_selected")
	case !isDir(dir):
		return checkErr(sn.lang, "gui.error.ranges_missing", "path", dir)
	}
	book, err := engine.LoadRanges(sn.ctx, os.DirFS(dir))
	if err != nil {
		return checkErr(sn.lang, "gui.error.read_failed", "path", dir, "err", err.Error())
	}
	if book.Files == 0 {
		return checkErr(sn.lang, "gui.error.no_ranges_files", "dir", dir)
	}
	return checkView(sn.lang, engine.CheckRanges(book))
}

func checkView(lang string, c engine.CheckReport) CheckView {
	t := pipeline.Translator(lang)
	v := emptyCheck()
	v.OK = true
	v.SlotsFilled, v.SlotsTotal, v.Mapped, v.StandardCount, v.Complete = c.SlotsFilled, c.SlotsTotal, c.Mapped, c.StandardCount, c.Complete
	for _, n := range c.MissingCells {
		node := MissingNode{Node: n.Node, NodeLabel: i18n.T(lang, "report.dev.node."+n.Node, nil), Cells: []MissingCell{}}
		for _, cell := range n.Cells {
			base := ""
			if cell.Base != "" {
				base = fileName(cell.Base)
			}
			node.Cells = append(node.Cells, MissingCell{Cell: cell.Cell, Base: base, Actions: fileNames(cell.Actions)})
		}
		v.MissingNodes = append(v.MissingNodes, node)
	}
	for _, m := range c.MissingActions {
		v.MissingActions = append(v.MissingActions, MissingActionView{Text: t(m.Msg), Names: fileNames(m.CanonicalNames)})
	}
	v.UnfilledRedundant = append(v.UnfilledRedundant, c.UnfilledRedundant...)
	v.UnfilledMissing = append(v.UnfilledMissing, c.UnfilledMissing...)
	v.Ignored = append(v.Ignored, c.Ignored...)
	v.Duplicates = append(v.Duplicates, c.Duplicates...)
	for _, u := range c.Unreadable {
		v.Unreadable = append(v.Unreadable, UnreadableView{Name: u.Name, Reason: t(u.Msg)})
	}
	return v
}

// fileName 是標準檔名加上 .txt（檔案實際的名稱）。
func fileName(stem string) string { return stem + ".txt" }

func fileNames(stems []string) []string {
	out := make([]string, len(stems))
	for i, s := range stems {
		out[i] = fileName(s)
	}
	return out
}

func initErr(lang, key string, kv ...string) InitView {
	return InitView{Error: tr(lang, key, kv...), Mapped: []MappedSkip{}}
}

// InitRangeTemplate 開資料夾對話框，在選到的資料夾建立範圍檔名樣板（不覆蓋既有檔案）。
func (s *AppService) InitRangeTemplate() InitView {
	sn := s.snap()
	dir, err := s.host.PickDir(tr(sn.lang, "gui.dialog.init_ranges"), validDir(sn.st.RangesDir))
	if err != nil {
		return initErr(sn.lang, "gui.error.dialog", "err", err.Error())
	}
	if dir == "" {
		return InitView{Cancelled: true, Mapped: []MappedSkip{}}
	}
	created, skipped, mapped, err := engine.InitTemplate(dir)
	if err != nil {
		v := initErr(sn.lang, "gui.error.init_failed", "err", err.Error())
		v.Dir = dir
		return v
	}
	v := InitView{OK: true, Dir: dir, Created: countTxt(created), Skipped: countTxt(skipped), Mapped: []MappedSkip{}}
	for _, m := range mapped {
		v.Mapped = append(v.Mapped, MappedSkip{Name: m.Name, Existing: m.Existing})
	}
	switch {
	case contains(created, engine.HowToFillName):
		v.HowTo = "created"
	case contains(skipped, engine.HowToFillName):
		v.HowTo = "exists"
	}
	return v
}

func countTxt(names []string) int {
	n := 0
	for _, name := range names {
		if strings.HasSuffix(name, ".txt") {
			n++
		}
	}
	return n
}

func contains(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}
