package engine

import (
	"sort"
	"strings"
)

// TableFormat describes one table size. Every position name used anywhere in the program
// is spelled here and nowhere else; internal/guard enforces it.
type TableFormat struct {
	Size       int
	Positions  []string // in preflop action order: the opener always comes before the players who answer it
	FromButton []string // dealing order, starting with the seat left of the button
	// FiveBetAllInOnly are the cells (earlier, later) whose 5bet has only an all in file in the template. In the
	// reference range set this template targets, the 5bet of these vs_4bet cells is always all in (there is no smaller size),
	// so no plain 5bet file is offered; completeness still accepts either file.
	FiveBetAllInOnly []Pair
}

// sixMax is the only registered format.
var sixMax = TableFormat{
	Size:       6,
	Positions:  []string{"UTG", "HJ", "CO", "BTN", "SB", "BB"},
	FromButton: []string{"SB", "BB", "UTG", "HJ", "CO", "BTN"},

	FiveBetAllInOnly: []Pair{{"SB", "BB"}, {"CO", "BTN"}},
}

var formats = []TableFormat{sixMax}

func (f TableFormat) clone() TableFormat {
	return TableFormat{f.Size, append([]string(nil), f.Positions...), append([]string(nil), f.FromButton...),
		append([]Pair(nil), f.FiveBetAllInOnly...)}
}

// SixMax returns the 6-max format.
func SixMax() TableFormat { return sixMax.clone() }

// Formats returns the registered table formats.
func Formats() []TableFormat {
	out := make([]TableFormat, len(formats))
	for i, f := range formats {
		out[i] = f.clone()
	}
	return out
}

// DefaultFormat is the format hands and ranges are read for.
func DefaultFormat() TableFormat { return formats[0].clone() }

// Pair is two positions of a cell: Earlier acts before Later.
type Pair struct{ Earlier, Later string }

// Seat is one "Seat N: name" line.
type Seat struct {
	Num  int
	Name string
}

// Index is the position's place in action order.
func (f TableFormat) Index(pos string) (int, bool) {
	for i, p := range f.Positions {
		if p == pos {
			return i, true
		}
	}
	return 0, false
}

// Earlier reports whether first acts before second; unknown positions are never earlier.
func (f TableFormat) Earlier(first, second string) bool {
	i, ok1 := f.Index(first)
	j, ok2 := f.Index(second)
	return ok1 && ok2 && i < j
}

// Openers are the positions that can open the pot (everybody but the last to act).
func (f TableFormat) Openers() []string {
	return append([]string(nil), f.Positions[:len(f.Positions)-1]...)
}

// Pairs lists every (earlier, later) combination in the order the matrices list their cells.
func (f TableFormat) Pairs() []Pair {
	var out []Pair
	for i, a := range f.Positions[:len(f.Positions)-1] {
		for _, b := range f.Positions[i+1:] {
			out = append(out, Pair{a, b})
		}
	}
	return out
}

// PosAlternation is the regular-expression alternation of all position names.
func (f TableFormat) PosAlternation() string { return strings.Join(f.Positions, "|") }

// PosJSON is the position list as a JSON array, for the template's /*POS*/ injection point.
func (f TableFormat) PosJSON() string {
	quoted := make([]string, len(f.Positions))
	for i, p := range f.Positions {
		quoted[i] = `"` + p + `"`
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// AssignPositions returns {player: position}, or false unless exactly Size players sit and the button seat is taken.
func (f TableFormat) AssignPositions(seats []Seat, buttonSeat int) (map[string]string, bool) {
	if len(seats) != f.Size {
		return nil, false
	}
	buttonTaken := false
	for _, s := range seats {
		buttonTaken = buttonTaken || s.Num == buttonSeat
	}
	if !buttonTaken {
		return nil, false
	}
	ordered := append([]Seat(nil), seats...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Num != ordered[j].Num {
			return ordered[i].Num < ordered[j].Num
		}
		return ordered[i].Name < ordered[j].Name
	})
	var after, upto []Seat
	for _, s := range ordered {
		if s.Num > buttonSeat {
			after = append(after, s)
		} else {
			upto = append(upto, s)
		}
	}
	out := make(map[string]string, f.Size)
	for i, s := range append(after, upto...) {
		out[s.Name] = f.FromButton[i]
	}
	return out, true
}
