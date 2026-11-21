package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 合成手牌與範圍檔（移植 Python tests/helpers.py；全部是假資料）。

var seatPositions = []string{"BTN", "SB", "BB", "UTG", "HJ", "CO"}

type act struct {
	pos, verb string
	to        float64
}

func raises(pos string, to float64) act { return act{pos, "raises", to} }
func calls(pos string) act              { return act{pos: pos, verb: "calls"} }
func folds(pos string) act              { return act{pos: pos, verb: "folds"} }

// rfiRest 是 UTG 開池後 HJ、CO、BTN、SB 都棄牌。
func rfiRest() []act { return []act{folds("HJ"), folds("CO"), folds("BTN"), folds("SB")} }

func join(parts ...[]act) []act {
	var out []act
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// makeHand 組一手牌；hero 坐的位置叫 Hero，其他人叫 p_<位置>。
func makeHand(actions []act, id, hero string) string {
	name := func(pos string) string {
		if pos == hero {
			return "Hero"
		}
		return "p_" + pos
	}
	lines := []string{
		fmt.Sprintf("Poker Hand #%s: Hold'em No Limit ($0.05/$0.1) - 2026/10/06 06:55:22", id),
		"Table 'RushAndCash1' 6-max Seat #1 is the button",
	}
	for i, pos := range seatPositions {
		lines = append(lines, fmt.Sprintf("Seat %d: %s ($10 in chips)", i+1, name(pos)))
	}
	lines = append(lines, name("SB")+": posts small blind $0.05", name("BB")+": posts big blind $0.1", "*** HOLE CARDS ***")
	for _, pos := range seatPositions {
		lines = append(lines, "Dealt to "+name(pos)+" ")
	}
	for _, a := range actions {
		switch a.verb {
		case "raises":
			to := a.to * 0.1
			lines = append(lines, fmt.Sprintf("%s: raises $%.2f to $%.2f", name(a.pos), to, to))
		case "calls":
			lines = append(lines, name(a.pos)+": calls $0.20")
		default:
			lines = append(lines, name(a.pos)+": "+a.verb)
		}
	}
	lines = append(lines, "*** SUMMARY ***", "Total pot $0.5 | Rake $0.02 | Jackpot $0 | Bingo $0 | Fortune $0 | Tax $0")
	return strings.Join(lines, "\n") + "\n\n"
}

func combos(hand string) []string {
	const suits = "shdc"
	r1, r2 := string(hand[0]), string(hand[1])
	var out []string
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			switch {
			case len(hand) == 2 && i < j, hand[len(hand)-1] == 's' && len(hand) == 3 && i == j,
				len(hand) == 3 && hand[2] == 'o' && i != j:
				out = append(out, r1+string(suits[i])+r2+string(suits[j]))
			}
		}
	}
	return out
}

// rangeText 把 {"AA": 1, "AKs": 0.5} 變成範圍檔內容（AsAh: 1,...）。
func rangeText(w map[string]float64) string {
	hands := make([]string, 0, len(w))
	for h := range w {
		hands = append(hands, h)
	}
	sort.Strings(hands)
	var parts []string
	for _, h := range hands {
		for _, c := range combos(h) {
			parts = append(parts, c+": "+strconv.FormatFloat(w[h], 'g', -1, 64))
		}
	}
	return strings.Join(parts, ",")
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// handsDir 是三手牌：兩手 UTG 開池（BB 一次跟注、一次 3bet）與一手 SB 的全棄牌（同 Python hands_dir）。
func handsDir(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "hands")
	write(t, filepath.Join(root, "gg", "a.txt"),
		makeHand(join([]act{raises("UTG", 2)}, rfiRest(), []act{calls("BB")}), "RC1", "BB")+
			makeHand(join([]act{raises("UTG", 2)}, rfiRest(), []act{raises("BB", 9), folds("UTG")}), "RC2", "BB"))
	write(t, filepath.Join(root, "gg", "b.txt"),
		makeHand(join([]act{folds("UTG")}, rfiRest()), "RC3", "SB"))
	return root
}

// writeRanges 把 {"UTG Open 2bb": {"AA": 1}} 寫成範圍檔，回傳資料夾。
func writeRanges(t *testing.T, files map[string]map[string]float64) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ranges")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, w := range files {
		write(t, filepath.Join(root, name+".txt"), rangeText(w))
	}
	return root
}

// standardRanges 是 Python 測試用的四個範圍檔（含一個多人節點會被忽略）。
func standardRanges(t *testing.T) string {
	t.Helper()
	return writeRanges(t, map[string]map[string]float64{
		"UTG Open 2bb":                    {"AA": 1, "KK": 0.5},
		"BB vs UTG open 2bb call":         {"AA": 0.5},
		"BB vs UTG open 2bb 3bet 9bb":     {"AA": 0.5, "KK": 1},
		"BB vs UTG open 2bb SB call call": {"AA": 1},
	})
}

// result 是一次執行的結果。
type result struct {
	code        int
	stdout, err string
}

// runCLI 以 zh-Hant 執行（測試比對的是 Python 原文）；args 自己帶 --lang 時不再加。
func runCLI(t *testing.T, args ...string) result {
	t.Helper()
	hasLang := false
	for _, a := range args {
		if a == "--lang" || strings.HasPrefix(a, "--lang=") {
			hasLang = true
		}
	}
	if !hasLang {
		args = append(args, "--lang", "zh-Hant")
	}
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return result{code, out.String(), errb.String()}
}

// isolateEnv 讓測試不受使用者的環境變數與家目錄影響。
func isolateEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("PREFLOP_MATRIX_RANGES", "")
	return home
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func mustContain(t *testing.T, label, text string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(text, s) {
			t.Errorf("%s lacks %q:\n%s", label, s, text)
		}
	}
}

func mustNotContain(t *testing.T, label, text string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(text, s) {
			t.Errorf("%s should not contain %q:\n%s", label, s, text)
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
