package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
)

// text 把引擎訊息翻成當前語言（命名空間 engine.*）。
func (t term) text(m engine.Message) string { return t.t(m.Key, flatten(m.Args)...) }

// printHandsSummary 印出手牌載入摘要與無法解析的範例（同 Python _hands_summary）。
func (t term) printHandsSummary(r *engine.LoadResult) {
	t.say("cli.hands_summary", "files", itoa(r.Files), "hands", itoa(len(r.Hands)), "duplicates", itoa(r.Duplicates),
		"not_six_max", itoa(r.WrongTableSize), "unparseable", itoa(r.Unparseable), "unreadable", itoa(r.UnreadableFiles))
	for _, s := range r.ErrorSamples {
		t.say("cli.hands_error_sample", "sample", s.File+": "+t.text(s.Msg))
	}
}

// printRangesSummary 印出範圍載入摘要與逐項清單（同 Python _ranges_summary；忽略的檔案只列數量）。
func (t term) printRangesSummary(book *engine.RangeBook) {
	t.say("cli.ranges_summary", "files", itoa(book.Files), "nodes", itoa(len(book.Ranges)), "ignored", itoa(len(book.Ignored)),
		"duplicates", itoa(len(book.Duplicates)), "unreadable", itoa(len(book.Unreadable)), "unfilled", itoa(len(book.Unfilled)))
	t.printUnreadable(book.Unreadable)
	t.printItems("cli.ranges_unfilled", book.Unfilled)
	for _, m := range engine.MissingActions(book) {
		t.say("cli.ranges_incomplete", "item", t.text(m))
	}
	t.printItems("cli.ranges_duplicate", book.Duplicates)
}

func (t term) printUnreadable(files []engine.UnreadableFile) {
	for _, f := range files {
		t.say("cli.ranges_unreadable", "item", f.Name+": "+t.text(f.Msg))
	}
}

// printItems 每個項目印一行 key（佔位符 {item}）。
func (t term) printItems(key string, items []string) {
	for _, s := range items {
		t.say(key, "item", s)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// devActionLabel 是偏離摘要的動作名稱（同報告 template 的 actionLabel）。
func (t term) devActionLabel(d engine.Deviation) string {
	switch {
	case d.Action == engine.DevFold:
		return t.t("report.act.fold")
	case d.Action == engine.DevCall:
		return t.t("report.act.call")
	case d.Node == "open":
		return t.t("report.act.open")
	}
	return map[string]string{"vs_open": "3bet", "vs_3bet": "4bet", "vs_4bet": "5bet"}[d.Node]
}

// devRow 是一列偏離：位置對抗（決策者在前）、節點、動作、實際、理論、差、n、z；只有數字，沒有建議。
func (t term) devRow(d engine.Deviation) string {
	spot := d.Decider
	if d.Opponent != "" {
		spot += " vs " + d.Opponent
	}
	theory := fmt.Sprintf("%.1f%%", d.Theory*100)
	if d.Clamped {
		theory += t.t("cli.dev.clamped_tag")
	}
	cols := []string{spot, t.t("report.dev.node." + d.Node), t.devActionLabel(d),
		fmt.Sprintf("%.1f%%", d.Obs*100), theory, signed(d.DiffPP), itoa(d.N), signed(d.Z)}
	return "  " + strings.Join(cols, " | ")
}

// printDeviations 在報告完成後印出偏離摘要；兩組都沒有列時整段不印。
func (t term) printDeviations(rep engine.DeviationReport, hero string) {
	if len(rep.Pool) == 0 && len(rep.Me) == 0 {
		return
	}
	fmt.Fprintln(t.stdout, t.t("report.dev.title"))
	t.printDevGroup(t.t("report.dev.pool"), rep.Pool)
	t.printDevGroup(t.t("report.dev.me", "hero", hero), rep.Me)
}

func (t term) printDevGroup(title string, rows []engine.Deviation) {
	if len(rows) == 0 {
		return
	}
	cols := []string{"spot", "node", "action", "obs", "theory", "diff", "n", "z"}
	heads := make([]string, len(cols))
	for i, c := range cols {
		heads[i] = t.t("report.dev.col." + c)
	}
	fmt.Fprintf(t.stdout, " %s\n  %s\n", title, strings.Join(heads, " | "))
	for _, d := range rows {
		fmt.Fprintln(t.stdout, t.devRow(d))
	}
}

// signed 是帶正負號的一位小數；四捨五入成 0 時不帶號（同報告的 signed()）。
func signed(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	switch {
	case s == "0.0" || s == "-0.0":
		return "0.0"
	case strings.HasPrefix(s, "-"):
		return s
	}
	return "+" + s
}
