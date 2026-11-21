// Package pipeline: 手牌與範圍 → 矩陣資料、偏離摘要、報告 HTML。CLI 與 GUI 共用；不印東西、不碰路徑與參數。
package pipeline

import (
	"context"
	"errors"
	"io/fs"
	"strconv"
	"sync"

	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
	"github.com/ken19960728ken/preflop-matrix-app/internal/report"
)

var (
	// ErrNoHands 表示手牌資料夾底下沒有可用的手牌；Build 仍回傳帶有 Loaded 的 Result。
	ErrNoHands = errors.New("pipeline: no usable hands")
	// ErrNoRanges 表示範圍資料夾底下沒有 .txt；Build 仍回傳帶有 Loaded、Book 的 Result。
	ErrNoRanges = errors.New("pipeline: no .txt range files")
)

// Input 是 Build 的輸入。Ranges 為 nil 表示沒有選範圍（理論全空）。
type Input struct {
	Hands  fs.FS
	Ranges fs.FS
	Hero   string
	TopN   int
}

// Result 是 Build 的結果。Book 與 Check 在 Ranges 為 nil 時是 nil。
type Result struct {
	Loaded     *engine.LoadResult
	Book       *engine.RangeBook
	Data       *engine.Data
	Check      *engine.CheckReport
	Deviations engine.DeviationReport
	Warnings   []engine.Message
	NoHero     bool
}

// Build 載入手牌與範圍並建出矩陣與偏離摘要。
// 沒有可用手牌回 ErrNoHands（Result.Loaded 有值）；範圍資料夾沒有 .txt 回 ErrNoRanges（Loaded、Book 有值）。
// 其他錯誤（讀取失敗、取消、TopN 不合法）回 nil Result 之外的 error；Result 可能為 nil 或只有已載入的部分。
func Build(ctx context.Context, in Input) (*Result, error) {
	loaded, err := engine.LoadHands(ctx, in.Hands)
	if err != nil {
		return nil, err
	}
	r := &Result{Loaded: loaded}
	if len(loaded.Hands) == 0 {
		return r, ErrNoHands
	}
	book := &engine.RangeBook{Ranges: map[engine.RangeKey]engine.Range{}}
	if in.Ranges != nil {
		if r.Book, err = engine.LoadRanges(ctx, in.Ranges); err != nil {
			return r, err
		}
		if r.Book.Files == 0 {
			return r, ErrNoRanges
		}
		book = r.Book
	}
	r.Data = engine.BuildMatrix(loaded.Hands, book, in.Hero)
	if r.Book != nil {
		check := engine.CheckRanges(r.Book)
		r.Check = &check
	}
	r.Warnings = r.Data.Meta.Warnings
	r.NoHero = r.Data.Meta.HeroHands == 0
	return r, r.SetTopN(in.TopN)
}

// SetTopN 只重算偏離摘要（不重新載入任何檔案）；n 不在 0..engine.MaxTopN 回 error 且不改動 Result。
func (r *Result) SetTopN(n int) error {
	devs, err := engine.Deviations(*r.Data, n)
	if err != nil {
		return err
	}
	r.Deviations = devs
	return nil
}

// Translator 回傳把引擎訊息翻成 lang 的函式（engine 命名空間）。
func Translator(lang string) func(engine.Message) string {
	return func(m engine.Message) string { return i18n.T(lang, m.Key, m.Args) }
}

var dict = sync.OnceValues(func() (map[string]map[string]string, error) {
	return report.LoadDict(i18n.Locales())
})

// Notice 是報告頂部的提醒：沒有範圍 → noRanges；範圍不完整 → incomplete（缺的節點格數與動作檔數）；完整 → nil。
func Notice(r *Result) *report.Notice {
	if r.Check == nil {
		return &report.Notice{Key: "report.notice.noRanges"}
	}
	if r.Check.Complete {
		return nil
	}
	nodes := 0
	for _, n := range r.Check.MissingCells {
		nodes += len(n.Cells)
	}
	return &report.Notice{Key: "report.notice.incomplete", Args: map[string]string{
		"nodes": strconv.Itoa(nodes), "actions": strconv.Itoa(len(r.Check.MissingActions))}}
}

// Render 產生 DATA JSON（meta.warnings 已翻成 lang）與報告 HTML。title 為 nil 時用預設標題；
// topN 為 0 時報告不含偏離摘要（偏離摘要以 topN 重新計算，不使用 r.Deviations）。
func Render(r *Result, lang string, title *string, topN int) (dataJSON, html []byte, err error) {
	devs, err := engine.Deviations(*r.Data, topN)
	if err != nil {
		return nil, nil, err
	}
	d, err := dict()
	if err != nil {
		return nil, nil, err
	}
	if dataJSON, err = r.Data.MarshalWith(Translator(lang)); err != nil {
		return nil, nil, err
	}
	opt := report.Options{Lang: lang, I18N: d, Title: title, Notice: Notice(r), Positions: engine.DefaultFormat().Positions}
	if topN > 0 {
		opt.Deviations = reportDeviations(devs)
	}
	html, err = report.RenderHTML(dataJSON, opt)
	return dataJSON, html, err
}

func reportDeviations(rep engine.DeviationReport) *report.Deviations {
	convert := func(rows []engine.Deviation) []report.Deviation {
		out := make([]report.Deviation, len(rows))
		for i, d := range rows {
			out[i] = report.Deviation(d)
		}
		return out
	}
	return &report.Deviations{Pool: convert(rep.Pool), Me: convert(rep.Me)}
}
