package cli

import (
	"context"
	"os"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
)

// templateName 是標準檔名加上 .txt（檔案實際的名稱）。
func templateName(stem string) string { return stem + ".txt" }

func templateNames(stems []string) string {
	names := make([]string, len(stems))
	for i, s := range stems {
		names[i] = templateName(s)
	}
	return strings.Join(names, ", ")
}

// runCheck 只檢查範圍資料夾的完整度：完整 0、不完整 1（含沒有 .txt）、資料夾不存在 2。
func (t term) runCheck(o options) int {
	src, ok := t.resolveRangesDir(o)
	if !ok {
		return exitBadInput
	}
	book, err := engine.LoadRanges(context.Background(), os.DirFS(src.Path))
	if err != nil {
		t.fail("cli.error.read_failed", "path", src.Path, "err", err.Error())
		return exitFailed
	}
	if book.Files == 0 {
		t.fail("cli.error.no_ranges_files", "dir", src.Path)
		return exitFailed
	}
	check := engine.CheckRanges(book)
	t.printCheck(check)
	if !check.Complete {
		return exitFailed
	}
	return exitOK
}

// printCheck 依分組印出檢查結果：必要動作統計、缺的節點、缺的動作檔、未填、忽略、重複、無法讀取，最後是結論。
func (t term) printCheck(c engine.CheckReport) {
	t.say("cli.check.summary", "have", itoa(c.SlotsFilled), "required", itoa(c.SlotsTotal),
		"nodes", itoa(c.Mapped), "standard", itoa(c.StandardCount))
	for _, n := range c.MissingCells {
		for _, cell := range n.Cells {
			stems := cell.Actions
			if cell.Base != "" {
				stems = append([]string{cell.Base}, stems...)
			}
			t.say("cli.check.missing_node", "item", n.Node+" "+cell.Cell+": "+templateNames(stems))
		}
	}
	for _, m := range c.MissingActions {
		t.say("cli.check.missing_action", "item", t.text(m.Msg)+" -> "+templateNames(m.CanonicalNames))
	}
	t.printItems("cli.check.unfilled_redundant", c.UnfilledRedundant)
	t.printItems("cli.check.unfilled_missing", c.UnfilledMissing)
	t.printItems("cli.ranges_ignored", c.Ignored)
	t.printItems("cli.ranges_duplicate", c.Duplicates)
	t.printUnreadable(c.Unreadable)
	if c.Complete {
		t.say("cli.check.complete")
	} else {
		t.say("cli.check.incomplete")
	}
}
