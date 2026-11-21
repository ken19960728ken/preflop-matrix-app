package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
	"github.com/ken19960728ken/preflop-matrix-app/internal/pipeline"
)

// jsonIndent 是 --json 的縮排（同 Python json.dumps(indent=1)）。
const jsonIndent = " "

// outputs 是 -o 與 --json 的實際讀寫路徑（只展開 ~，不正規化；.. 要留給作業系統處理）。
type outputs struct{ html, json string }

// runReport 產生報告：驗證輸入、載入手牌與範圍、建矩陣、寫出 HTML／JSON，最後印偏離摘要。
func (t term) runReport(o options, top int) int {
	handsDir, err := config.ExpandUser(o.vals[keyHands])
	if err != nil {
		t.failNoHome(err, false)
		return exitBadInput
	}
	if !isDir(handsDir) {
		t.fail("cli.error.hands_dir", "path", shown(handsDir))
		return exitBadInput
	}
	src, ok := t.resolveRangesDir(o)
	if !ok {
		return exitBadInput
	}
	out, code := t.outputPaths(o)
	if code != exitOK {
		return code
	}
	res, code := t.build(o, handsDir, src.Path, top)
	if code != exitOK {
		return code
	}
	t.printDataWarnings(res, o.get(keyHero, defaultHero))
	return t.write(o, out, res, top)
}

// outputPaths 回傳 -o（預設 preflop_matrix.html）與 --json 的路徑，並確認兩者不是同一個檔案。
func (t term) outputPaths(o options) (outputs, int) {
	var out outputs
	var err error
	if out.html, err = config.ExpandUser(o.get(keyOut, defaultOut)); err == nil && o.vals[keyJSON] != "" {
		out.json, err = config.ExpandUser(o.vals[keyJSON])
	}
	if err != nil {
		t.failNoHome(err, false)
		return out, exitBadInput
	}
	if out.json != "" && resolveLoose(out.json) == resolveLoose(out.html) {
		t.fail("cli.error.same_output")
		return out, exitBadInput
	}
	return out, exitOK
}

// build 呼叫 pipeline 並印出手牌與範圍摘要；沒有可用手牌或沒有 .txt 範圍檔回傳 exit 1。
func (t term) build(o options, handsDir, rangesDir string, top int) (*pipeline.Result, int) {
	res, err := pipeline.Build(context.Background(), pipeline.Input{
		Hands: os.DirFS(handsDir), Ranges: os.DirFS(rangesDir), Hero: o.get(keyHero, defaultHero), TopN: top})
	if res != nil {
		t.printHandsSummary(res.Loaded)
	}
	if errors.Is(err, pipeline.ErrNoHands) {
		t.fail("cli.error.no_hands", "dir", shown(handsDir))
		return nil, exitFailed
	}
	if res != nil && res.Book != nil {
		t.printRangesSummary(res.Book)
	}
	if errors.Is(err, pipeline.ErrNoRanges) {
		t.fail("cli.error.no_ranges_files", "dir", rangesDir)
		return nil, exitFailed
	}
	if err != nil {
		t.fail("cli.error.internal", "err", err.Error())
		return nil, exitFailed
	}
	return res, exitOK
}

// printDataWarnings 把矩陣的警告與「沒有 Hero 手牌」警告印到 stderr。
func (t term) printDataWarnings(res *pipeline.Result, hero string) {
	for _, m := range res.Warnings {
		t.warn(t.text(m))
	}
	if res.NoHero {
		t.warn(t.t("cli.warning.no_hero", "hero", engine.PyRepr(hero)))
	}
}

// write 渲染並寫出 HTML（及 --json），印「已輸出」與偏離摘要。渲染失敗是內部錯誤，寫入失敗是 exit 1。
func (t term) write(o options, out outputs, res *pipeline.Result, top int) int {
	var title *string
	if o.has(keyTitle) {
		title = ptr(o.vals[keyTitle])
	}
	payload, html, err := pipeline.Render(res, t.lang, title, top)
	if err != nil {
		t.fail("cli.error.internal", "err", err.Error())
		return exitFailed
	}
	if err = writeFile(out.html, html); err == nil && out.json != "" {
		err = writeJSON(out.json, payload)
	}
	if err != nil {
		t.fail("cli.error.write_output", "err", err.Error())
		return exitFailed
	}
	t.printDone(shown(out.html), o.vals[keyJSON])
	t.printDeviations(res.Deviations, res.Data.Meta.Hero)
	return exitOK
}

func ptr(s string) *string { return &s }

func (t term) printDone(out, jsonPath string) {
	if jsonPath == "" {
		t.say("cli.done", "out", out)
		return
	}
	t.say("cli.done_with_json", "out", out, "json", jsonPath)
}

// parentDir 是 path 最後一個分隔符之前的部分（不正規化，x/.. 照原樣），同 Python Path.parent 對 .. 的處理。
func parentDir(path string) string {
	switch i := strings.LastIndexByte(path, os.PathSeparator); {
	case i < 0:
		return "."
	case i == 0:
		return path[:1]
	default:
		return path[:i]
	}
}

// writeFile 建立上層資料夾後寫檔。
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(parentDir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// writeJSON 以 Python 版的縮排（1 個空白）寫出矩陣資料。
func writeJSON(path string, payload []byte) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, payload, "", jsonIndent); err != nil {
		return err
	}
	return writeFile(path, buf.Bytes())
}
