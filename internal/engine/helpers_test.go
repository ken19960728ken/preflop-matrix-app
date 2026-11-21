package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Synthetic GG hands and range files; nothing here comes from real data (port of tests/helpers.py).

const bbSize = 0.1

// Button is seat 1 (as in real files); seats 2.. follow clockwise.
var seatPositions = []string{"BTN", "SB", "BB", "UTG", "HJ", "CO"}

const testSuits = "shdc"

// act is one action of a synthetic hand.
type act struct {
	pos, verb string
	to        float64
	allin     bool
}

func raises(pos string, to float64) act      { return act{pos, "raises", to, false} }
func raisesAllIn(pos string, to float64) act { return act{pos, "raises", to, true} }
func calls(pos string) act                   { return act{pos: pos, verb: "calls"} }
func folds(pos string) act                   { return act{pos: pos, verb: "folds"} }
func checks(pos string) act                  { return act{pos: pos, verb: "checks"} }
func acts(parts ...[]act) []act {
	var out []act
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
func seq(a ...act) []act { return a }

// H are the options of makeHand; zero values mean the defaults of the Python helper.
type H struct {
	ID     string
	Hero   string
	Day    string
	Seats  []string
	Button int
	Extra  []string
}

// makeHand builds a hand text. The player sitting at Hero is called Hero, everybody else p_<position>.
func makeHand(actions []act, opts ...H) string {
	h := H{}
	if len(opts) > 0 {
		h = opts[0]
	}
	if h.ID == "" {
		h.ID = "RC1"
	}
	if h.Hero == "" {
		h.Hero = "BB"
	}
	if h.Day == "" {
		h.Day = "2026/10/06"
	}
	if h.Seats == nil {
		h.Seats = seatPositions
	}
	if h.Button == 0 {
		h.Button = 1
	}
	name := func(pos string) string {
		if pos == h.Hero {
			return "Hero"
		}
		return "p_" + pos
	}
	lines := []string{
		fmt.Sprintf("Poker Hand #%s: Hold'em No Limit ($0.05/$0.1) - %s 06:55:22", h.ID, h.Day),
		fmt.Sprintf("Table 'RushAndCash1' 6-max Seat #%d is the button", h.Button),
	}
	for i, pos := range h.Seats {
		lines = append(lines, fmt.Sprintf("Seat %d: %s ($10 in chips)", i+1, name(pos)))
	}
	lines = append(lines, name("SB")+": posts small blind $0.05", name("BB")+": posts big blind $0.1", "*** HOLE CARDS ***")
	for _, pos := range h.Seats {
		lines = append(lines, "Dealt to "+name(pos)+" ")
	}
	for _, a := range actions {
		switch a.verb {
		case "raises":
			to := a.to * bbSize
			line := fmt.Sprintf("%s: raises $%.2f to $%.2f", name(a.pos), to, to)
			if a.allin {
				line += " and is all-in"
			}
			lines = append(lines, line)
		case "calls":
			lines = append(lines, name(a.pos)+": calls $0.20")
		default:
			lines = append(lines, name(a.pos)+": "+a.verb)
		}
	}
	lines = append(lines, h.Extra...)
	lines = append(lines, "*** SUMMARY ***", "Total pot $0.5 | Rake $0.02 | Jackpot $0 | Bingo $0 | Fortune $0 | Tax $0")
	return strings.Join(lines, "\n") + "\n\n"
}

func mustParse(t *testing.T, text string) Hand {
	t.Helper()
	h, err := ParseHand(text)
	if err != nil {
		t.Fatalf("ParseHand: %v", err)
	}
	return h
}

func rat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("bad rat " + s)
	}
	return r
}

func comboTexts(hand string) []string {
	r1, r2 := string(hand[0]), string(hand[1])
	var out []string
	switch {
	case len(hand) == 2:
		for i := 0; i < 4; i++ {
			for j := i + 1; j < 4; j++ {
				out = append(out, r1+string(testSuits[i])+r2+string(testSuits[j]))
			}
		}
	case hand[2] == 's':
		for i := 0; i < 4; i++ {
			out = append(out, r1+string(testSuits[i])+r2+string(testSuits[i]))
		}
	default:
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				if i != j {
					out = append(out, r1+string(testSuits[i])+r2+string(testSuits[j]))
				}
			}
		}
	}
	return out
}

// W are range weights by hand class.
type W map[string]float64

// rangeText is {'AA': 1, 'AKs': 0.5} -> a range file body ('AsAh: 1,...').
func rangeText(w W) string {
	hands := make([]string, 0, len(w))
	for h := range w {
		hands = append(hands, h)
	}
	sort.Strings(hands)
	var parts []string
	for _, h := range hands {
		for _, c := range comboTexts(h) {
			parts = append(parts, c+": "+strconv.FormatFloat(w[h], 'g', -1, 64))
		}
	}
	return strings.Join(parts, ",")
}

// writeTree writes name -> bytes below a fresh folder and returns it.
func writeTree(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// writeRanges writes {file stem: weights} as range files into a fresh folder (the write_ranges fixture).
func writeRanges(t *testing.T, files map[string]W) fs.FS {
	t.Helper()
	tree := map[string][]byte{}
	for stem, w := range files {
		tree[stem+".txt"] = []byte(rangeText(w))
	}
	return os.DirFS(writeTree(t, tree))
}

func approx(got, want float64) bool {
	return math.Abs(got-want) <= math.Max(1e-6*math.Max(math.Abs(got), math.Abs(want)), 1e-12)
}

// asJSON is the JSON round trip of a value: what the template receives.
func asJSON(t *testing.T, d *Data) map[string]any {
	t.Helper()
	b, err := d.MarshalWith(pyText)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

// at walks nested JSON objects; ok is false when a key is absent (null is present with a nil value).
func at(v any, path ...string) (any, bool) {
	for _, k := range path {
		m, isMap := v.(map[string]any)
		if !isMap {
			return nil, false
		}
		var ok bool
		if v, ok = m[k]; !ok {
			return nil, false
		}
	}
	return v, true
}

func mustAt(t *testing.T, v any, path ...string) any {
	t.Helper()
	got, ok := at(v, path...)
	if !ok {
		t.Fatalf("missing key %v", path)
	}
	return got
}

// wantApprox asserts that got is an object with exactly the keys of want, each approximately equal.
func wantApprox(t *testing.T, got any, want map[string]float64) {
	t.Helper()
	m, ok := got.(map[string]any)
	if !ok || len(m) != len(want) {
		t.Fatalf("got %v, want keys of %v", got, want)
	}
	for k, w := range want {
		g, ok := m[k].(float64)
		if !ok || !approx(g, w) {
			t.Fatalf("got %v, want %v (key %s)", got, want, k)
		}
	}
}

func wantEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// pyTemplates are the texts of the Python original (English for parse and range errors, zh-Hant for the matrix).
var pyTemplates = map[string]string{
	MsgParseBadAmount:              "bad amount: {text}",
	MsgParseBadHeader:              "unsupported game or bad header: {line}",
	MsgParseMissingSections:        "missing HOLE CARDS or SUMMARY section",
	MsgParseUnrecognisedBeforeDeal: "unrecognised line before the deal: {line}",
	MsgParseUnrecognisedPreflop:    "unrecognised preflop line: {line}",
	MsgParseDuplicatePlayer:        "duplicate player name",
	MsgParseTableSize:              "{count} players (only {size}-handed hands are used)",
	MsgParseButtonEmpty:            "button seat {seat} is empty",
	MsgParsePlayerNotSeated:        "player not seated: {player}",
	MsgParseBlindsMismatch:         "positions from the button disagree with the players who posted the blinds",
	MsgRangeBadCard:                "bad card: {card}",
	MsgRangeSameCard:               "the same card twice: {cards}",
	MsgRangeBadCombo:               "bad combo: {token}",
	MsgRangeDuplicateCombo:         "duplicate combo: {cards}",
	MsgRangeBadWeight:              "bad weight: {token}",
	MsgRangeNotAFloat:              "could not convert string to float: {value}",
	MsgMatrixMissingAction:         "{node} {cell} 缺 {action}，已當 0",
	MsgMatrixTheoryOverOne:         "{node} {cell} 理論的跟注 + 加注 = {total} 超過 1，已按比例縮到 1",
}

// pyText formats a message like the Python original.
func pyText(m Message) string {
	text, ok := pyTemplates[m.Key]
	if !ok {
		return m.Key
	}
	for k, v := range m.Args {
		text = strings.ReplaceAll(text, "{"+k+"}", v)
	}
	return text
}

func mustHands(t *testing.T, fsys fs.FS) *LoadResult {
	t.Helper()
	r, err := LoadHands(context.Background(), fsys)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mustRanges(t *testing.T, fsys fs.FS) *RangeBook {
	t.Helper()
	b, err := LoadRanges(context.Background(), fsys)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
