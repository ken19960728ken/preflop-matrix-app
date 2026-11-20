package engine

import (
	"context"
	"errors"
	"io/fs"
	"strings"
)

// Theory range files: parsing and the file name -> node mapping.
//
// A range maps each of the 169 hand classes (AA, AKs, AKo, ...) to the fraction of its combos played.
// Only two-player nodes are mapped; any file naming a third player is ignored.

const (
	ranks       = "AKQJT98765432"
	suits       = "shdc"
	totalCombos = 1326
)

var handClasses = buildHands()

// HandClasses lists the 169 hand classes, 13x13 in row-major order.
func HandClasses() []string { return append([]string(nil), handClasses...) }

func buildHands() []string {
	out := make([]string, 0, 169)
	for i := 0; i < 13; i++ {
		for j := 0; j < 13; j++ {
			switch {
			case i == j:
				out = append(out, string([]byte{ranks[i], ranks[i]}))
			case i < j:
				out = append(out, string([]byte{ranks[i], ranks[j], 's'}))
			default:
				out = append(out, string([]byte{ranks[j], ranks[i], 'o'}))
			}
		}
	}
	return out
}

// Range maps every hand class to the fraction of its combos played.
type Range map[string]float64

// RangeError is a range file's text that cannot be parsed.
type RangeError struct{ Msg Message }

func (e *RangeError) Error() string { return messageString(e.Msg) }

func rangeErr(key string, kv ...string) error { return &RangeError{newMsg(key, kv...)} }

// Combos is the number of card combinations of a hand class.
func Combos(hand string) int {
	switch {
	case len(hand) == 2:
		return 6
	case hand[2] == 's':
		return 4
	}
	return 12
}

func rankIndex(c byte) int { return strings.IndexByte(ranks, c) }

func validCard(card []rune) bool {
	return len(card) == 2 && card[0] < 128 && card[1] < 128 &&
		rankIndex(byte(card[0])) >= 0 && strings.IndexByte(suits, byte(card[1])) >= 0
}

// HandClass maps two cards to their class: 'As', 'Kh' -> 'AKo'.
func HandClass(cardA, cardB string) (string, error) {
	for _, card := range []string{cardA, cardB} {
		if !validCard([]rune(card)) {
			return "", rangeErr(MsgRangeBadCard, "card", pyRepr(card))
		}
	}
	if cardA == cardB {
		return "", rangeErr(MsgRangeSameCard, "cards", cardA+cardB)
	}
	hi, lo := cardA, cardB
	if rankIndex(cardB[0]) < rankIndex(cardA[0]) {
		hi, lo = cardB, cardA
	}
	if hi[0] == lo[0] {
		return string([]byte{hi[0], hi[0]}), nil
	}
	kind := byte('o')
	if hi[1] == lo[1] {
		kind = 's'
	}
	return string([]byte{hi[0], lo[0], kind}), nil
}

// ParseRangeText parses 'AsAh: 1,KsKh: 0.5,...' into {hand class: fraction of its combos}, capped at 1.
func ParseRangeText(text string) (Range, error) {
	weight := map[string]float64{}
	seen := map[[2]string]bool{}
	for _, token := range strings.Split(strings.ReplaceAll(text, "\n", ","), ",") {
		if pyStrip(token) == "" {
			continue
		}
		cardsText, value, _ := strings.Cut(token, ":")
		cards := []rune(pyStrip(cardsText))
		if len(cards) != 4 {
			return nil, rangeErr(MsgRangeBadCombo, "token", pyRepr(pyStrip(token)))
		}
		a, b := string(cards[:2]), string(cards[2:])
		cls, err := HandClass(a, b)
		if err != nil {
			return nil, err
		}
		combo := [2]string{a, b}
		if b < a {
			combo = [2]string{b, a}
		}
		if seen[combo] {
			return nil, rangeErr(MsgRangeDuplicateCombo, "cards", string(cards))
		}
		seen[combo] = true
		w := 1.0
		if pyStrip(value) != "" {
			v, ok := parsePyFloat(value)
			if !ok {
				return nil, rangeErr(MsgRangeNotAFloat, "value", pyRepr(value))
			}
			w = v
		}
		if !isFinite(w) || w < 0 {
			return nil, rangeErr(MsgRangeBadWeight, "token", pyRepr(pyStrip(token)))
		}
		weight[cls] += w
	}
	out := make(Range, len(handClasses))
	for _, h := range handClasses {
		out[h] = pyMin(1.0, weight[h]/float64(Combos(h)))
	}
	return out, nil
}

func isFinite(f float64) bool { return f-f == 0 }

// comboWeights is rng[h] * combos(h) for every class, in handClasses order.
func comboWeights(rng Range) []float64 {
	out := make([]float64, len(handClasses))
	for i, h := range handClasses {
		out[i] = float64(rng[h] * float64(Combos(h)))
	}
	return out
}

// TotalShare is the fraction of all 1326 combos in the range.
func TotalShare(rng Range) float64 {
	return pySum(comboWeights(rng)) / totalCombos
}

// RangeKey identifies a range file's node. Vs is "" for open.
type RangeKey struct {
	Node   string // open | vs_open | vs_3bet | vs_4bet (and any node of the theory table)
	Pos    string // the acting position
	Vs     string // the opponent
	Action string // raise (open) | call | 3bet | 4bet | 4bet_allin | 5bet | 5bet_allin
}

// ParseFilename maps a range file name (without .txt) to its node, or false when it is not a two-player node.
func ParseFilename(stem string) (RangeKey, bool) {
	format := DefaultFormat()
	stem = strings.NewReplacer("\u0130", "i", "\u0131", "i").Replace(strings.Join(pySplit(stem), " "))
	for i := range theoryNodes {
		row := &theoryNodes[i]
		m := row.regex().FindStringSubmatch(stem)
		if m == nil {
			continue
		}
		if !row.pairNode() {
			return RangeKey{row.Node, strings.ToUpper(m[1]), "", row.Map("")}, true
		}
		actor, vs := strings.ToUpper(m[1]), strings.ToUpper(m[2])
		earlier, later := vs, actor
		if !nodeInfo(row.Node).EarlierIsVs {
			earlier, later = actor, vs
		}
		if !format.Earlier(earlier, later) {
			return RangeKey{}, false
		}
		return RangeKey{row.Node, actor, vs, row.Map(strings.ToLower(m[3]))}, true
	}
	return RangeKey{}, false
}

// UnreadableFile is a mapped range file that could not be read or parsed.
type UnreadableFile struct {
	Name string
	Msg  Message
}

// RangeBook is everything LoadRanges found.
type RangeBook struct {
	Ranges     map[RangeKey]Range
	Files      int
	Ignored    []string         // not a two-player node
	Duplicates []string         // a second file for an already mapped node
	Unreadable []UnreadableFile // mapped but could not be read or parsed
	Unfilled   []string         // mapped but blank (only whitespace / a BOM): not filled in yet
}

// Get returns the range of a node, or nil.
func (b *RangeBook) Get(node, pos, vs, action string) Range {
	return b.Ranges[RangeKey{node, pos, vs, action}]
}

func readFailure(err error) Message {
	if errors.Is(err, errInvalidUTF8) {
		return newMsg(MsgRangeInvalidUTF8)
	}
	return newMsg(MsgLoadFileUnreadable, "reason", err.Error())
}

// LoadRanges reads every .txt under the root of fsys and keeps the files that map to a two-player node.
//
// A blank file is "not filled in yet": it never takes its node, so a later file for the same node can.
func LoadRanges(ctx context.Context, fsys fs.FS) (*RangeBook, error) {
	book := &RangeBook{Ranges: map[RangeKey]Range{}}
	paths, err := txtFiles(ctx, fsys)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := baseName(path)
		book.Files++
		key, ok := ParseFilename(fileStem(name))
		if !ok {
			book.Ignored = append(book.Ignored, name)
			continue
		}
		text, readErr := readRangeFile(fsys, path)
		switch {
		case readErr == nil && pyStrip(text) == "":
			book.Unfilled = append(book.Unfilled, name)
		case book.Ranges[key] != nil:
			book.Duplicates = append(book.Duplicates, name)
		case readErr != nil:
			book.Unreadable = append(book.Unreadable, UnreadableFile{name, readFailure(readErr)})
		default:
			rng, err := ParseRangeText(text)
			var rangeError *RangeError
			if errors.As(err, &rangeError) {
				book.Unreadable = append(book.Unreadable, UnreadableFile{name, rangeError.Msg})
			} else {
				book.Ranges[key] = rng
			}
		}
	}
	return book, nil
}
