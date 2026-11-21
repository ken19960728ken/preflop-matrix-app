package engine

// CellMissing is a theory cell the book cannot compute (its theory is "–"): the base range is missing, or
// no action of the cell is there. Base is the standard name of the missing base range ("" when it is there),
// Actions are the standard names of the unfilled action slots.
type CellMissing struct {
	Cell    string
	Base    string
	Actions []string
}

// NodeMissing groups the missing cells of one node.
type NodeMissing struct {
	Node  string
	Cells []CellMissing
}

// MissingAction is a cell that has one action but not the other (the missing one counts as 0).
// CanonicalNames are the standard names of the missing action's files.
type MissingAction struct {
	Msg            Message
	Node           string
	Cell           string
	CanonicalNames []string
}

// CheckReport is the completeness of a range folder.
//
// A required action slot is one action of a cell (open: the raise; the two-player nodes: both actions, each
// satisfied by any of its keys). Complete means every slot is filled. A base range is itself a required
// slot (the open raise, the vs_open 3bet), so a complete book always has the base ranges.
//
// MissingCells are in theory-table order. UnfilledRedundant are blank files whose action slot another file
// already fills (they can be deleted); UnfilledMissing are blank files whose slot is still empty.
// AllMissingNames are the standard names of every file named in MissingCells and MissingActions, without
// duplicates, in StandardSet order.
type CheckReport struct {
	Mapped            int // range files mapped to a node
	StandardCount     int
	SlotsFilled       int
	SlotsTotal        int
	MissingCells      []NodeMissing
	MissingActions    []MissingAction
	AllMissingNames   []string
	UnfilledRedundant []string
	UnfilledMissing   []string
	Ignored           []string
	Duplicates        []string
	Unreadable        []UnreadableFile
	Complete          bool
}

// cellNames are the standard names of the files of the cell that fill the given action slot.
func (n *TheoryNode) cellNames(f TableFormat, earlier, later string, spec ActionSpec) []string {
	var out []string
	for _, k := range n.standardKeys(f, earlier, later) {
		if containsString(spec.Keys, k.Action) {
			out = append(out, CanonicalName(k))
		}
	}
	return out
}

// missingActions lists the cells that have one action but not the other.
func missingActions(book *RangeBook) []MissingAction {
	f := DefaultFormat()
	var out []MissingAction
	for i := range theoryNodes {
		row := &theoryNodes[i]
		if !row.pairNode() {
			continue
		}
		for _, c := range row.cells(f) {
			base, acts := row.inputs(book, c[0], c[1])
			if (row.Base != nil && base == nil) || (acts[0] == nil && acts[1] == nil) {
				continue
			}
			for j, rng := range acts {
				if rng == nil {
					out = append(out, MissingAction{
						Msg:            newMsg(MsgMatrixMissingAction, "node", row.Node, "cell", c[0]+">"+c[1], "action", row.Actions[j].Name),
						Node:           row.Node,
						Cell:           c[0] + ">" + c[1],
						CanonicalNames: row.cellNames(f, c[0], c[1], row.Actions[j]),
					})
				}
			}
		}
	}
	return out
}

// slotFilled reports whether the action slot of the key is filled by any of its keys.
func slotFilled(book *RangeBook, k RangeKey) bool {
	row := theoryRow(k.Node)
	for _, spec := range row.specs() {
		if !containsString(spec.Keys, k.Action) {
			continue
		}
		for _, a := range spec.Keys {
			if book.Ranges[RangeKey{k.Node, k.Pos, k.Vs, a}] != nil {
				return true
			}
		}
	}
	return false
}

// CheckRanges checks a loaded range folder against the theory table.
func CheckRanges(book *RangeBook) CheckReport {
	f := DefaultFormat()
	r := CheckReport{
		Mapped:         len(book.Ranges),
		StandardCount:  len(StandardSet()),
		MissingActions: missingActions(book),
		Ignored:        append([]string(nil), book.Ignored...),
		Duplicates:     append([]string(nil), book.Duplicates...),
		Unreadable:     append([]UnreadableFile(nil), book.Unreadable...),
	}
	named := map[string]bool{}
	for i := range theoryNodes {
		row := &theoryNodes[i]
		var missing []CellMissing
		for _, c := range row.cells(f) {
			actor, vs := row.actorVs(c[0], c[1])
			base, acts := row.inputs(book, c[0], c[1])
			var names []string
			filled := 0
			for j, spec := range row.specs() {
				r.SlotsTotal++
				if acts[j] != nil {
					filled++
				} else {
					names = append(names, row.cellNames(f, c[0], c[1], spec)...)
				}
			}
			r.SlotsFilled += filled
			baseMissing := row.Base != nil && base == nil
			if !baseMissing && filled > 0 {
				continue
			}
			cell := CellMissing{Cell: cellKey(row.Node, actor, vs), Actions: names}
			if baseMissing {
				baseVs := ""
				if row.Base.WithVs {
					baseVs = vs
				}
				cell.Base = CanonicalName(RangeKey{row.Base.Node, actor, baseVs, row.Base.Action})
				named[cell.Base] = true
			}
			for _, n := range names {
				named[n] = true
			}
			missing = append(missing, cell)
		}
		if missing != nil {
			r.MissingCells = append(r.MissingCells, NodeMissing{row.Node, missing})
		}
	}
	for _, m := range r.MissingActions {
		for _, n := range m.CanonicalNames {
			named[n] = true
		}
	}
	for _, k := range StandardSet() {
		if name := CanonicalName(k); named[name] {
			r.AllMissingNames = append(r.AllMissingNames, name)
		}
	}
	for _, name := range book.Unfilled {
		if key, ok := ParseFilename(fileStem(name)); ok && slotFilled(book, key) {
			r.UnfilledRedundant = append(r.UnfilledRedundant, name)
		} else {
			r.UnfilledMissing = append(r.UnfilledMissing, name)
		}
	}
	r.Complete = r.SlotsFilled == r.SlotsTotal
	return r
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
