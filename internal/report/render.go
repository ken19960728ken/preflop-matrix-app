// Package report: 渲染報告 HTML。
package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

//go:embed template.html
var templateHTML string

// Langs 回傳支援的報告語言（每次一份新複本），順序即頁內語言切換器的順序。
func Langs() []string { return []string{"zh-Hant", "zh-Hans", "en"} }

var positionName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// 注入點，依在 template 中出現的順序排列。
var marks = [...]string{
	"/*DATA*/null",
	"/*I18N*/null",
	`/*LANG*/"zh-Hant"`,
	"/*TITLE*/null",
	"/*NOTICE*/null",
	"/*POS*/null",
	"/*DEVIATIONS*/null",
}

// Options 是 RenderHTML 的非資料參數。
type Options struct {
	// Lang 是頁面預設語言（Langs 之一）；頁面可用 ?lang= 覆蓋。
	Lang string
	// I18N 是 lang -> key -> 文字 的 report 字典，三個語言都必須有。LoadDict 可從 locales 目錄讀取。
	I18N map[string]map[string]string
	// Title 為 nil 或空字串時頁面用翻譯過的預設標題。
	Title *string
	// Notice 為 nil 時不顯示提醒（頁面與無提醒版相同）。
	Notice *Notice
	// Positions 是依桌型排序的位置名稱（至少 2 個，只允許 [A-Za-z0-9_]）；template 內不含任何位置字面值。
	Positions []string
	// Deviations 為 nil 或兩表皆空時偏離摘要整區不顯示。
	Deviations *Deviations
}

// Notice 是頂部提醒：Key 為 report.notice.* 之一，Args 填入佔位符。
type Notice struct {
	Key  string            `json:"key"`
	Args map[string]string `json:"args,omitempty"`
}

// Deviation 是偏離摘要的一列；JSON 形狀即注入 /*DEVIATIONS*/ 的格式。
// 比例（Obs、Theory）為 0–1 小數，DiffPP 為百分點（(Obs−Theory)×100，帶正負由頁面顯示）。
// Node 為 open|vs_open|vs_3bet|vs_4bet；Action 為 f|c|r（open 只有 r）。
// Opponent 在 open 時為空字串。AnchorID 即報告格子的 id（cell-<sec>-<op>-<de>-<who>-<k>，
// open 為 cell-open-<pos>-<who>），op/de 取資料 key（"<earlier>><later>"）的順序，不是決策者順序。
type Deviation struct {
	Node     string  `json:"node"`
	Decider  string  `json:"decider"`
	Opponent string  `json:"opponent"`
	Action   string  `json:"action"`
	Obs      float64 `json:"obs"`
	Theory   float64 `json:"theory"`
	DiffPP   float64 `json:"diffPP"`
	N        int     `json:"n"`
	Z        float64 `json:"z"`
	Clamped  bool    `json:"clamped"`
	AnchorID string  `json:"anchorID"`
}

// Deviations 是 /*DEVIATIONS*/ 的整體形狀：玩家池與 Hero 兩張表。
type Deviations struct {
	Pool []Deviation `json:"pool"`
	Me   []Deviation `json:"me"`
}

// LoadDict 從 locales 根目錄（<lang>/report.json）讀出三語 report 字典。
func LoadDict(locales fs.FS) (map[string]map[string]string, error) {
	out := make(map[string]map[string]string, len(Langs()))
	for _, lang := range Langs() {
		raw, err := fs.ReadFile(locales, lang+"/report.json")
		if err != nil {
			return nil, err
		}
		var dict map[string]string
		if err := json.Unmarshal(raw, &dict); err != nil {
			return nil, fmt.Errorf("%s/report.json: %w", lang, err)
		}
		out[lang] = dict
	}
	return out, nil
}

// scriptSafe 讓 JSON 可安全放進 <script>：< 與 JS 行終止字元改成 \u 跳脫（它們只可能出現在 JSON 字串內）。
var scriptSafe = strings.NewReplacer(
	"<", "\\u003c",
	string(rune(0x2028)), "\\u2028",
	string(rune(0x2029)), "\\u2029",
)

func inject(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return scriptSafe.Replace(string(raw)), nil
}

// RenderHTML 把 DATA（matrix JSON）與 opt 填進報告 template。
// template 先依注入點切開，再依序接上 payload，所以 payload 內容不會被當成注入點再次展開。
func RenderHTML(data []byte, opt Options) ([]byte, error) {
	if !slices.Contains(Langs(), opt.Lang) {
		return nil, fmt.Errorf("report: unsupported language %q", opt.Lang)
	}
	for _, lang := range Langs() {
		if len(opt.I18N[lang]) == 0 {
			return nil, fmt.Errorf("report: missing dictionary for %q", lang)
		}
	}
	if len(opt.Positions) < 2 {
		return nil, fmt.Errorf("report: need at least 2 positions, got %d", len(opt.Positions))
	}
	for _, p := range opt.Positions {
		if !positionName.MatchString(p) {
			return nil, fmt.Errorf("report: invalid position name %q", p)
		}
	}
	if !json.Valid(data) || !bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("{")) {
		return nil, fmt.Errorf("report: data must be a valid JSON object")
	}

	var title any
	if opt.Title != nil && *opt.Title != "" {
		title = *opt.Title
	}
	values := []any{json.RawMessage(data), opt.I18N, opt.Lang, title, opt.Notice, opt.Positions, opt.Deviations}
	if len(values) != len(marks) {
		return nil, fmt.Errorf("report: %d payloads for %d injection points", len(values), len(marks))
	}

	var out strings.Builder
	rest := templateHTML
	for i, mark := range marks {
		payload, err := inject(values[i])
		if err != nil {
			return nil, fmt.Errorf("report: %s: %w", mark, err)
		}
		before, after, ok := strings.Cut(rest, mark)
		if !ok {
			return nil, fmt.Errorf("report: template is missing %s", mark)
		}
		out.WriteString(before)
		out.WriteString(payload)
		rest = after
	}
	out.WriteString(rest)
	return []byte(out.String()), nil
}
