package engine

import "strings"

func (n *TheoryNode) specs() []ActionSpec {
	if n.pairNode() {
		return n.Actions[:]
	}
	return n.Actions[:1]
}

// cells lists the (earlier, later) positions of the node's cells; later is "" for open.
func (n *TheoryNode) cells(f TableFormat) [][2]string {
	var out [][2]string
	if !n.pairNode() {
		for _, p := range f.Openers() {
			out = append(out, [2]string{p, ""})
		}
		return out
	}
	for _, p := range f.Pairs() {
		out = append(out, [2]string{p.Earlier, p.Later})
	}
	return out
}

// fileActions are the range-key actions that have a standard file in the cell (earlier, later), in file order.
// The first key of every action has a file; with AllInFile the further keys of the second action (the all in
// spelling) have one too, and the plain key is left out in the cells of TableFormat.FiveBetAllInOnly.
func (n *TheoryNode) fileActions(f TableFormat, earlier, later string) []string {
	specs := n.specs()
	var out []string
	for j, spec := range specs {
		switch {
		case j < len(specs)-1 || !n.AllInFile:
			out = append(out, spec.Keys[0])
		case containsPair(f.FiveBetAllInOnly, Pair{earlier, later}):
			out = append(out, spec.Keys[1:]...)
		default:
			out = append(out, spec.Keys...)
		}
	}
	return out
}

func containsPair(list []Pair, p Pair) bool {
	for _, x := range list {
		if x == p {
			return true
		}
	}
	return false
}

// standardKeys are the keys of the standard files of the cell (earlier, later).
func (n *TheoryNode) standardKeys(f TableFormat, earlier, later string) []RangeKey {
	actor, vs := n.actorVs(earlier, later)
	var out []RangeKey
	for _, a := range n.fileActions(f, earlier, later) {
		out = append(out, RangeKey{n.Node, actor, vs, a})
	}
	return out
}

// CanonicalName is the standard file name (without .txt, no bet sizes) of a range key, or "" for a node
// that is not in the theory table.
func CanonicalName(k RangeKey) string {
	row := theoryRow(k.Node)
	if row == nil {
		return ""
	}
	return strings.NewReplacer("{ACTOR}", k.Pos, "{VS}", k.Vs,
		"{ACTION}", strings.ReplaceAll(k.Action, "_allin", " all in")).Replace(row.Stem)
}

// StandardSet lists the keys of the standard range files (108 for 6-max), derived from the theory table
// and the table format: nodes in table order, cells in matrix order.
func StandardSet() []RangeKey {
	f := DefaultFormat()
	var out []RangeKey
	for i := range theoryNodes {
		row := &theoryNodes[i]
		for _, c := range row.cells(f) {
			out = append(out, row.standardKeys(f, c[0], c[1])...)
		}
	}
	return out
}
