package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Build the position-vs-position frequency matrices (theory / Hero / pool).
//
// JSON layout (same as the HTML template expects), keys in this order:
//
//	open:      {pos: {th, me, pool}}                      th={r}, me/pool={r, n}
//	vs_*:      {"<earlier>><later>": {th, me, pool}}      th={c, r}, me/pool={c, r, n}; fold = 1 - c - r
//	absolute:  {vs_3bet|vs_4bet|vs_5bet: {key: {th|me|pool: {reach, f, c, r, n}}}}
//	           the same frequencies as a share of all dealt hands (the conditional frequency times the
//	           probability of reaching the node); a who without data has no key, th has n = null
//	meta:      hands, hero, hero_hands, warnings, date_from, date_to
//
// A missing theory or no observations is null. A node without a row in theoryNodes has no theory.

// The three sources of a cell.
const (
	WhoTh   = "th"
	WhoMe   = "me"
	WhoPool = "pool"
)

// sumTolerance is a variable so that 1+sumTolerance is rounded in float64 like Python, not as an exact constant.
var sumTolerance = 1e-6

var whoOrder = []string{WhoTh, WhoMe, WhoPool}

// OMap is a string-keyed map that keeps insertion order and marshals as a JSON object in that order.
type OMap[V any] struct {
	keys []string
	m    map[string]V
}

func newOMap[V any]() *OMap[V] { return &OMap[V]{m: map[string]V{}} }

// Set adds or replaces a key.
func (o *OMap[V]) Set(key string, v V) {
	if _, ok := o.m[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.m[key] = v
}

// Get returns the value of a key.
func (o *OMap[V]) Get(key string) (V, bool) {
	v, ok := o.m[key]
	return v, ok
}

// Keys lists the keys in insertion order.
func (o *OMap[V]) Keys() []string { return append([]string(nil), o.keys...) }

// Len is the number of keys.
func (o *OMap[V]) Len() int { return len(o.keys) }

// MarshalJSON writes the keys in insertion order.
func (o OMap[V]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		vb, err := json.Marshal(o.m[k])
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// OpenTheory is the theory of an open cell.
type OpenTheory struct {
	R float64 `json:"r"`
}

// OpenObs is an observed open frequency.
type OpenObs struct {
	R float64 `json:"r"`
	N int     `json:"n"`
}

// PairTheory is the theory of a two-player cell.
type PairTheory struct {
	C float64 `json:"c"`
	R float64 `json:"r"`
}

// PairObs is an observed two-player frequency.
type PairObs struct {
	C float64 `json:"c"`
	R float64 `json:"r"`
	N int     `json:"n"`
}

// OpenCell is a cell of the open table; a nil field is JSON null.
type OpenCell struct {
	Th   *OpenTheory `json:"th"`
	Me   *OpenObs    `json:"me"`
	Pool *OpenObs    `json:"pool"`
}

// PairCell is a cell of a two-player table; a nil field is JSON null.
type PairCell struct {
	Th   *PairTheory `json:"th"`
	Me   *PairObs    `json:"me"`
	Pool *PairObs    `json:"pool"`
}

// AbsEntry is one source's frequencies as a share of all dealt hands. N is nil (null) for theory.
type AbsEntry struct {
	Reach float64 `json:"reach"`
	F     float64 `json:"f"`
	C     float64 `json:"c"`
	R     float64 `json:"r"`
	N     *int    `json:"n"`
}

// AbsRow holds the sources that have data; a missing source has no key.
type AbsRow struct {
	Th   *AbsEntry `json:"th,omitempty"`
	Me   *AbsEntry `json:"me,omitempty"`
	Pool *AbsEntry `json:"pool,omitempty"`
}

// Meta is the "meta" object.
type Meta struct {
	Hands     int       `json:"hands"`
	Hero      string    `json:"hero"`
	HeroHands int       `json:"hero_hands"`
	Warnings  []Message `json:"warnings"`
	DateFrom  string    `json:"date_from"`
	DateTo    string    `json:"date_to"`
}

// Data is the matrix payload of the report (DATA).
type Data struct {
	Open     *OMap[OpenCell]
	Pairs    map[string]*OMap[PairCell] // by node name, every node but open
	Absolute *OMap[*OMap[AbsRow]]
	Meta     Meta
}

// errUseMarshalWith is returned by Data.MarshalJSON: the warnings are messages, not text.
var errUseMarshalWith = errors.New("engine: Data must be serialised with MarshalWith so that warnings become text")

// MarshalJSON always fails; use MarshalWith.
func (d Data) MarshalJSON() ([]byte, error) { return nil, errUseMarshalWith }

// pythonMeta is Meta with the warnings already formatted, the shape the template reads.
type pythonMeta struct {
	Hands     int      `json:"hands"`
	Hero      string   `json:"hero"`
	HeroHands int      `json:"hero_hands"`
	Warnings  []string `json:"warnings"`
	DateFrom  string   `json:"date_from"`
	DateTo    string   `json:"date_to"`
}

// MarshalWith writes the payload in the Python shape: open, vs_open, vs_3bet, vs_4bet, vs_5bet, absolute, meta
// in this order, and meta.warnings as an array of strings made by format.
func (d Data) MarshalWith(format func(Message) string) ([]byte, error) {
	warnings := make([]string, len(d.Meta.Warnings))
	for i, m := range d.Meta.Warnings {
		warnings[i] = format(m)
	}
	meta := pythonMeta{d.Meta.Hands, d.Meta.Hero, d.Meta.HeroHands, warnings, d.Meta.DateFrom, d.Meta.DateTo}
	var buf bytes.Buffer
	buf.WriteByte('{')
	put := func(key string, v any) error {
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(key)
		vb, err := json.Marshal(v)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
		return err
	}
	for _, n := range nodes {
		var v any = d.Pairs[n.Name]
		if !n.Pair {
			v = d.Open
		}
		if err := put(n.Name, v); err != nil {
			return nil, err
		}
	}
	if err := put("absolute", d.Absolute); err != nil {
		return nil, err
	}
	if err := put("meta", meta); err != nil {
		return nil, err
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// cellKey is "<earlier>><later>" for a decision (the position alone for open).
func cellKey(node, pos, vs string) string {
	info := nodeInfo(node)
	switch {
	case !info.Pair:
		return pos
	case info.EarlierIsVs:
		return vs + ">" + pos
	}
	return pos + ">" + vs
}

type tallyKey struct{ node, cell, who string }

type counter struct{ fold, call, raise int }

func (c *counter) n() int { return c.fold + c.call + c.raise }

// tally counts decisions per (node, cell, me|pool); it also returns the number of hands the hero played in.
func tally(hands []Hand, hero string) (map[tallyKey]*counter, int) {
	counts := map[tallyKey]*counter{}
	heroHands := 0
	for _, hand := range hands {
		if _, ok := hand.Positions[hero]; ok {
			heroHands++
		}
		for _, d := range PreflopDecisions(hand) {
			who := WhoPool
			if d.Player == hero {
				who = WhoMe
			}
			k := tallyKey{d.Node, cellKey(d.Node, d.Pos, d.Vs), who}
			c := counts[k]
			if c == nil {
				c = &counter{}
				counts[k] = c
			}
			switch d.Action {
			case ActFold:
				c.fold++
			case ActCall:
				c.call++
			default:
				c.raise++
			}
		}
	}
	return counts, heroHands
}

// union adds up the ranges (capped at 1); nil when none exists.
func union(ranges ...Range) Range {
	var present []Range
	for _, r := range ranges {
		if r != nil {
			present = append(present, r)
		}
	}
	if len(present) == 0 {
		return nil
	}
	out := make(Range, len(handClasses))
	for _, h := range handClasses {
		vals := make([]float64, len(present))
		for i, r := range present {
			vals[i] = r[h]
		}
		out[h] = pyMin(1.0, pySum(vals))
	}
	return out
}

func (n *TheoryNode) actionRange(book *RangeBook, spec ActionSpec, actor, vs string) Range {
	if len(spec.Keys) == 1 {
		return book.Get(n.Node, actor, vs, spec.Keys[0])
	}
	ranges := make([]Range, len(spec.Keys))
	for i, k := range spec.Keys {
		ranges[i] = book.Get(n.Node, actor, vs, k)
	}
	return union(ranges...)
}

// inputs is the base range and the ranges of the two actions for the cell (earlier, later).
func (n *TheoryNode) inputs(book *RangeBook, earlier, later string) (Range, [2]Range) {
	actor, vs := n.actorVs(earlier, later)
	var base Range
	if n.Base != nil {
		baseVs := ""
		if n.Base.WithVs {
			baseVs = vs
		}
		base = book.Get(n.Base.Node, actor, baseVs, n.Base.Action)
	}
	return base, [2]Range{n.actionRange(book, n.Actions[0], actor, vs), n.actionRange(book, n.Actions[1], actor, vs)}
}

// MissingActions lists the theory cells that have only one of their two actions (the other counts as 0).
func MissingActions(book *RangeBook) []Message {
	var out []Message
	for _, m := range missingActions(book) {
		out = append(out, m.Msg)
	}
	return out
}

// conditional is the call / raise share of the hands in base.
//
// The call / raise range files of a later street give a hand's weight over all deals (the base
// range already filtered the hands), so each is capped at the base weight before it is divided by it.
func conditional(base, call, raise Range) *PairTheory {
	size := pySum(comboWeights(base))
	if size == 0 {
		return nil
	}
	capped := func(rng Range) float64 {
		vals := make([]float64, len(handClasses))
		for i, h := range handClasses {
			vals[i] = float64(pyMin(rng[h], base[h]) * float64(Combos(h)))
		}
		return pySum(vals) / size
	}
	return &PairTheory{C: capped(call), R: capped(raise)}
}

var zeroRange = func() Range {
	r := make(Range, len(handClasses))
	for _, h := range handClasses {
		r[h] = 0.0
	}
	return r
}()

// theory is the cell's theory. A missing action counts as 0 when the other action and the base range exist.
func (n *TheoryNode) theory(book *RangeBook, earlier, later string) *PairTheory {
	base, acts := n.inputs(book, earlier, later)
	if (acts[0] == nil && acts[1] == nil) || (n.Base != nil && base == nil) {
		return nil
	}
	first, second := acts[0], acts[1]
	if first == nil {
		first = zeroRange
	}
	if second == nil {
		second = zeroRange
	}
	if n.Base == nil {
		return &PairTheory{C: TotalShare(first), R: TotalShare(second)}
	}
	return conditional(base, first, second)
}

// clamped scales c and r down proportionally when they add up to more than 1.
func clamped(th *PairTheory, node, cell string, warnings *[]Message) *PairTheory {
	if th == nil || th.C+th.R <= 1+sumTolerance {
		return th
	}
	total := th.C + th.R
	*warnings = append(*warnings, newMsg(MsgMatrixTheoryOverOne, "node", node, "cell", cell,
		"total", strconv.FormatFloat(total, 'f', 4, 64)))
	return &PairTheory{C: th.C / total, R: th.R / total}
}

// observed returns the counts of a source in a cell, or nil when there are none.
func observed(counts map[tallyKey]*counter, node, cell, who string) *counter {
	c := counts[tallyKey{node, cell, who}]
	if c == nil || c.n() == 0 {
		return nil
	}
	return c
}

func openObs(counts map[tallyKey]*counter, pos, who string) *OpenObs {
	c := observed(counts, NodeOpen, pos, who)
	if c == nil {
		return nil
	}
	return &OpenObs{R: float64(c.raise) / float64(c.n()), N: c.n()}
}

func pairObs(counts map[tallyKey]*counter, node, cell, who string) *PairObs {
	c := observed(counts, node, cell, who)
	if c == nil {
		return nil
	}
	n := float64(c.n())
	return &PairObs{C: float64(c.call) / n, R: float64(c.raise) / n, N: c.n()}
}

func openCell(pos string, counts map[tallyKey]*counter, book *RangeBook) OpenCell {
	cell := OpenCell{Me: openObs(counts, pos, WhoMe), Pool: openObs(counts, pos, WhoPool)}
	if row := theoryRow(NodeOpen); row != nil {
		if rng := book.Get(NodeOpen, pos, "", row.Actions[0].Keys[0]); rng != nil {
			cell.Th = &OpenTheory{R: TotalShare(rng)}
		}
	}
	return cell
}

func pairCell(node string, p Pair, counts map[tallyKey]*counter, book *RangeBook, warnings *[]Message) PairCell {
	cell := PairCell{}
	cellID := p.Earlier + ">" + p.Later
	if row := theoryRow(node); row != nil {
		cell.Th = clamped(row.theory(book, p.Earlier, p.Later), node, cellID, warnings)
	}
	cell.Me = pairObs(counts, node, cellID, WhoMe)
	cell.Pool = pairObs(counts, node, cellID, WhoPool)
	return cell
}

// pickRatio selects the raise frequency of a source from the three optional values.
func pickRatio(who string, th, me, pool *float64) (float64, bool) {
	var p *float64
	switch who {
	case WhoTh:
		p = th
	case WhoMe:
		p = me
	case WhoPool:
		p = pool
	}
	if p == nil {
		return 0, false
	}
	return *p, true
}

// ratio is the raise frequency of a source, if it has data.
func (c OpenCell) ratio(who string) (float64, bool) {
	var th, me, pool *float64
	if c.Th != nil {
		th = &c.Th.R
	}
	if c.Me != nil {
		me = &c.Me.R
	}
	if c.Pool != nil {
		pool = &c.Pool.R
	}
	return pickRatio(who, th, me, pool)
}

func (c PairCell) ratio(who string) (float64, bool) {
	var th, me, pool *float64
	if c.Th != nil {
		th = &c.Th.R
	}
	if c.Me != nil {
		me = &c.Me.R
	}
	if c.Pool != nil {
		pool = &c.Pool.R
	}
	return pickRatio(who, th, me, pool)
}

// absoluteNodes lists the nodes of the "absolute" table with the nodes whose raise frequencies
// multiply into the probability of reaching them.
var absoluteNodes = []struct {
	Node  string
	Chain []string
}{
	{NodeVs3bet, []string{NodeOpen}},
	{NodeVs4bet, []string{NodeVsOpen}},
	{NodeVs5bet, []string{NodeOpen, NodeVs3bet}},
}

// reach is the probability (over all dealt hands) that the responder of the cell gets to decide.
func (d *Data) reach(chain []string, key, who string) (float64, bool) {
	opener, _, _ := strings.Cut(key, ">")
	product := 0.0
	for i, node := range chain {
		var r float64
		var ok bool
		if nodeInfo(node).Pair {
			cell, found := d.Pairs[node].Get(key)
			r, ok = cell.ratio(who)
			ok = ok && found
		} else {
			cell, found := d.Open.Get(opener)
			r, ok = cell.ratio(who)
			ok = ok && found
		}
		if !ok {
			return 0, false
		}
		if i == 0 {
			product = r
		} else {
			product *= r
		}
	}
	return product, true
}

func absoluteEntry(reach, c, r float64, n *int) *AbsEntry {
	return &AbsEntry{Reach: reach, F: reach * (1 - c - r), C: reach * c, R: reach * r, N: n}
}

func (d *Data) absoluteRow(chain []string, key string, cell PairCell) AbsRow {
	var row AbsRow
	for _, who := range whoOrder {
		reach, ok := d.reach(chain, key, who)
		if !ok {
			continue
		}
		switch {
		case who == WhoTh && cell.Th != nil:
			row.Th = absoluteEntry(reach, cell.Th.C, cell.Th.R, nil)
		case who == WhoMe && cell.Me != nil:
			n := cell.Me.N
			row.Me = absoluteEntry(reach, cell.Me.C, cell.Me.R, &n)
		case who == WhoPool && cell.Pool != nil:
			n := cell.Pool.N
			row.Pool = absoluteEntry(reach, cell.Pool.C, cell.Pool.R, &n)
		}
	}
	return row
}

func (d *Data) buildAbsolute() *OMap[*OMap[AbsRow]] {
	out := newOMap[*OMap[AbsRow]]()
	for _, an := range absoluteNodes {
		rows := newOMap[AbsRow]()
		cells := d.Pairs[an.Node]
		for _, key := range cells.Keys() {
			cell, _ := cells.Get(key)
			rows.Set(key, d.absoluteRow(an.Chain, key, cell))
		}
		out.Set(an.Node, rows)
	}
	return out
}

// BuildMatrix builds the whole report payload.
func BuildMatrix(hands []Hand, book *RangeBook, hero string) *Data {
	format := DefaultFormat()
	counts, heroHands := tally(hands, hero)
	warnings := []Message{}
	data := &Data{Open: newOMap[OpenCell](), Pairs: map[string]*OMap[PairCell]{}}
	for _, pos := range format.Openers() {
		data.Open.Set(pos, openCell(pos, counts, book))
	}
	for _, n := range nodes {
		if !n.Pair {
			continue
		}
		cells := newOMap[PairCell]()
		for _, p := range format.Pairs() {
			cells.Set(p.Earlier+">"+p.Later, pairCell(n.Name, p, counts, book, &warnings))
		}
		data.Pairs[n.Name] = cells
	}
	data.Absolute = data.buildAbsolute()
	meta := Meta{Hands: len(hands), Hero: hero, HeroHands: heroHands, Warnings: warnings}
	for i, h := range hands {
		if i == 0 || h.Date < meta.DateFrom {
			meta.DateFrom = h.Date
		}
		if i == 0 || h.Date > meta.DateTo {
			meta.DateTo = h.Date
		}
	}
	data.Meta = meta
	return data
}
