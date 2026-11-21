package engine

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const someRange = "AsAh: 1"

func standardNames() []string {
	var out []string
	for _, k := range StandardSet() {
		out = append(out, CanonicalName(k))
	}
	return out
}

// filled writes the named files (with content) and loads them.
func filled(t *testing.T, names []string) *RangeBook {
	t.Helper()
	tree := map[string][]byte{}
	for _, n := range names {
		tree[n+".txt"] = []byte(someRange)
	}
	return mustRanges(t, os.DirFS(writeTree(t, tree)))
}

func without(names []string, drop ...string) []string {
	var out []string
	for _, n := range names {
		if !containsString(drop, n) {
			out = append(out, n)
		}
	}
	return out
}

func missingOf(r CheckReport, node string) []CellMissing {
	for _, n := range r.MissingCells {
		if n.Node == node {
			return n.Cells
		}
	}
	return nil
}

func cellOf(t *testing.T, r CheckReport, node, cell string) CellMissing {
	t.Helper()
	for _, c := range missingOf(r, node) {
		if c.Cell == cell {
			return c
		}
	}
	t.Fatalf("%s %s not reported: %v", node, cell, r.MissingCells)
	return CellMissing{}
}

func TestStandardSetRoundTripsAndHas108(t *testing.T) {
	set := StandardSet()
	if len(set) != 108 {
		t.Fatalf("standard set has %d files", len(set))
	}
	seen := map[string]bool{}
	for _, k := range set {
		name := CanonicalName(k)
		if seen[name] {
			t.Errorf("duplicate name %q", name)
		}
		seen[name] = true
		if back, ok := ParseFilename(name); !ok || back != k {
			t.Errorf("%q parses to %v %v, want %v", name, back, ok, k)
		}
	}
	for _, name := range []string{"BB vs SB 4bet 5bet", "BTN vs CO 4bet 5bet"} {
		if seen[name] {
			t.Errorf("%q must not be in the standard set", name)
		}
		if !seen[name+" all in"] {
			t.Errorf("%q missing", name+" all in")
		}
	}
	for _, name := range []string{"UTG open", "BB vs UTG open call", "BB vs UTG open 3bet", "UTG vs BB 3bet call",
		"UTG vs BB 3bet 4bet", "BB vs UTG 4bet call", "BB vs UTG 4bet 5bet", "BB vs UTG 4bet 5bet all in"} {
		if !seen[name] {
			t.Errorf("%q missing from the standard set", name)
		}
	}
	if CanonicalName(RangeKey{Node: "nope"}) != "" {
		t.Error("unknown node has no name")
	}
}

func TestEveryTableRowHasAStemThatRoundTrips(t *testing.T) {
	f := DefaultFormat()
	for i := range theoryNodes {
		row := &theoryNodes[i]
		if !strings.Contains(row.Stem, "{ACTOR}") || (row.pairNode() && !strings.Contains(row.Stem, "{VS}")) ||
			(len(row.specs()) > 1 && !strings.Contains(row.Stem, "{ACTION}")) {
			t.Errorf("%s: stem %q lacks a placeholder", row.Node, row.Stem)
		}
		for _, c := range row.cells(f) {
			keys := row.standardKeys(f, c[0], c[1])
			if len(keys) == 0 {
				t.Errorf("%s %v has no standard file", row.Node, c)
			}
			for _, k := range keys {
				if back, ok := ParseFilename(CanonicalName(k)); !ok || back != k {
					t.Errorf("%s: %q does not round-trip", row.Node, CanonicalName(k))
				}
			}
		}
	}
}

func TestFiveBetAllInOnlyCells(t *testing.T) {
	f := DefaultFormat()
	if len(f.FiveBetAllInOnly) != 2 {
		t.Fatalf("%v", f.FiveBetAllInOnly)
	}
	for _, p := range f.FiveBetAllInOnly {
		if !f.Earlier(p.Earlier, p.Later) {
			t.Errorf("%v: not a cell of the format", p)
		}
		removed := 0
		for i := range theoryNodes {
			row := &theoryNodes[i]
			if !row.AllInFile {
				continue
			}
			all := append([]string(nil), row.specs()[1].Keys...)
			got := row.fileActions(f, p.Earlier, p.Later)
			full := row.fileActions(f, "", "")
			if len(full) != 1+len(all) {
				t.Fatalf("%v", full)
			}
			removed += len(full) - len(got)
		}
		if removed != 1 {
			t.Errorf("%v removes %d keys, want exactly 1", p, removed)
		}
	}
	if len(StandardSet()) != 110-len(f.FiveBetAllInOnly) {
		t.Errorf("standard set %d", len(StandardSet()))
	}
	c := f.clone()
	c.FiveBetAllInOnly[0] = Pair{"x", "y"}
	if DefaultFormat().FiveBetAllInOnly[0] == c.FiveBetAllInOnly[0] {
		t.Error("clone shares the slice")
	}
}

func TestCompleteWithAll108(t *testing.T) {
	r := CheckRanges(filled(t, standardNames()))
	if !r.Complete || r.SlotsFilled != 95 || r.SlotsTotal != 95 || r.Mapped != 108 || r.StandardCount != 108 {
		t.Errorf("%+v", r)
	}
	if len(r.MissingCells) != 0 || len(r.MissingActions) != 0 || len(r.AllMissingNames) != 0 ||
		len(r.UnfilledRedundant)+len(r.UnfilledMissing)+len(r.Ignored)+len(r.Duplicates)+len(r.Unreadable) != 0 {
		t.Errorf("%+v", r)
	}
}

func TestMissingBaseRangeMakesItIncomplete(t *testing.T) {
	r := CheckRanges(filled(t, without(standardNames(), "UTG open")))
	if r.Complete || r.SlotsFilled != 94 {
		t.Errorf("%+v", r)
	}
	c := cellOf(t, r, "vs_3bet", "UTG>BB")
	if c.Base != "UTG open" || len(c.Actions) != 0 {
		t.Errorf("%+v", c)
	}
	if o := cellOf(t, r, "open", "UTG"); o.Base != "" || !reflect.DeepEqual(o.Actions, []string{"UTG open"}) {
		t.Errorf("%+v", o)
	}
}

func TestPlain5betIsOptionalWhenAllInIsThere(t *testing.T) {
	r := CheckRanges(filled(t, without(standardNames(), "BB vs UTG 4bet 5bet")))
	if !r.Complete || r.SlotsFilled != 95 || r.Mapped != 107 || len(r.MissingActions) != 0 {
		t.Errorf("%+v", r)
	}
	r = CheckRanges(filled(t, without(standardNames(), "BB vs UTG 4bet 5bet", "BB vs UTG 4bet 5bet all in")))
	if r.Complete || r.SlotsFilled != 94 || len(r.MissingActions) != 1 {
		t.Fatalf("%+v", r)
	}
	m := r.MissingActions[0]
	wantNames := []string{"BB vs UTG 4bet 5bet", "BB vs UTG 4bet 5bet all in"}
	if m.Msg.Args["action"] != "5bet" || m.Node != "vs_4bet" || m.Cell != "UTG>BB" || !reflect.DeepEqual(m.CanonicalNames, wantNames) {
		t.Errorf("%+v", m)
	}
	if !reflect.DeepEqual(r.AllMissingNames, wantNames) || len(r.MissingCells) != 0 {
		t.Errorf("%v %v", r.AllMissingNames, r.MissingCells)
	}
	if got := MissingActions(filled(t, without(standardNames(), "BB vs UTG 4bet 5bet", "BB vs UTG 4bet 5bet all in"))); !reflect.DeepEqual(got, []Message{m.Msg}) {
		t.Errorf("%v", got)
	}
}

func TestMissingCellsListStandardNames(t *testing.T) {
	r := CheckRanges(filled(t, []string{"UTG open", "BB vs UTG 4bet call"}))
	if r.Complete || r.SlotsFilled != 2 || r.SlotsTotal != 95 {
		t.Fatalf("%+v", r)
	}
	var order []string
	for _, n := range r.MissingCells {
		order = append(order, n.Node)
	}
	if !reflect.DeepEqual(order, []string{"open", "vs_open", "vs_3bet", "vs_4bet"}) {
		t.Errorf("node order %v", order)
	}
	if got := len(missingOf(r, "open")); got != 4 {
		t.Errorf("open: %d missing cells", got)
	}
	if got := len(missingOf(r, "vs_open")); got != 15 {
		t.Errorf("vs_open: %d", got)
	}
	want := CellMissing{Cell: "UTG>BB", Actions: []string{"UTG vs BB 3bet call", "UTG vs BB 3bet 4bet"}}
	if got := cellOf(t, r, "vs_3bet", "UTG>BB"); !reflect.DeepEqual(got, want) {
		t.Errorf("vs_3bet UTG>BB: %+v", got)
	}
	want = CellMissing{Cell: "UTG>BB", Base: "BB vs UTG open 3bet", Actions: []string{"BB vs UTG 4bet 5bet", "BB vs UTG 4bet 5bet all in"}}
	if got := cellOf(t, r, "vs_4bet", "UTG>BB"); !reflect.DeepEqual(got, want) {
		t.Errorf("vs_4bet UTG>BB: %+v", got)
	}
	if len(r.MissingActions) != 0 {
		t.Errorf("%v", r.MissingActions)
	}
	std := standardNames()
	pos := map[string]int{}
	for i, n := range std {
		pos[n] = i
	}
	last := -1
	for _, n := range r.AllMissingNames {
		if p, ok := pos[n]; !ok || p <= last {
			t.Errorf("AllMissingNames not unique and in standard order at %q", n)
		} else {
			last = p
		}
	}
	if !containsString(r.AllMissingNames, "BB vs UTG open 3bet") || containsString(r.AllMissingNames, "UTG open") {
		t.Errorf("%v", r.AllMissingNames)
	}
}

func TestMissingCellOfAllInOnlyCellNamesOnlyAllIn(t *testing.T) {
	r := CheckRanges(filled(t, without(standardNames(), "BB vs SB 4bet call", "BB vs SB 4bet 5bet all in")))
	want := CellMissing{Cell: "SB>BB", Actions: []string{"BB vs SB 4bet call", "BB vs SB 4bet 5bet all in"}}
	if got := cellOf(t, r, "vs_4bet", "SB>BB"); !reflect.DeepEqual(got, want) {
		t.Errorf("%+v", got)
	}
}

func TestReportCarriesBookInformation(t *testing.T) {
	dir := writeTree(t, map[string][]byte{
		"UTG open.txt":                  []byte(someRange),
		"UTG open 2bb.txt":              []byte(someRange),
		"BB vs UTG open call.txt":       []byte("  \n"),
		"BB vs UTG open 3bet.txt":       []byte("AsAh: x"),
		"UTG vs BB vs HJ open 3bet.txt": []byte(someRange),
	})
	book := mustRanges(t, os.DirFS(dir))
	r := CheckRanges(book)
	if len(r.Duplicates) != 1 || len(r.UnfilledMissing) != 1 || len(r.UnfilledRedundant) != 0 || len(r.Unreadable) != 1 || len(r.Ignored) != 1 || r.Mapped != 1 {
		t.Errorf("%+v", r)
	}
	r.Duplicates[0] = "changed"
	if book.Duplicates[0] == "changed" {
		t.Error("the report must copy the book's lists")
	}
}

func TestUnfilledRedundantWhenAnotherFileFillsTheSlot(t *testing.T) {
	dir := writeTree(t, map[string][]byte{
		"UTG open 2bb.txt":                    []byte(someRange),
		"UTG open.txt":                        []byte("\n"),
		"BB vs UTG 4bet call.txt":             []byte(someRange),
		"BB vs UTG 4bet 5bet all in.txt":      []byte(someRange),
		"BB vs UTG 4bet 5bet.txt":             nil,
		"UTG vs BB 3bet call.txt":             nil,
		"UTG vs BB 3bet 4bet all in.txt":      nil,
		"HJ open.txt":                         nil,
		"BB vs UTG open 3bet.txt":             []byte(someRange),
		"BB vs UTG open call.txt":             []byte(someRange),
		"UTG vs BB 3bet 4bet.txt":             []byte(someRange),
		"zzz/UTG vs BB 3bet 4bet all in.txt":  []byte(someRange),
		"CO vs BTN 3bet 4bet all in 10bb.txt": nil,
	})
	r := CheckRanges(mustRanges(t, os.DirFS(dir)))
	sort.Strings(r.UnfilledRedundant)
	sort.Strings(r.UnfilledMissing)
	wantRed := []string{"BB vs UTG 4bet 5bet.txt", "UTG open.txt", "UTG vs BB 3bet call.txt"}
	if reflect.DeepEqual(r.UnfilledRedundant, wantRed) {
		t.Fatalf("call slot of UTG vs BB is empty, so it must be missing: %v", r.UnfilledRedundant)
	}
	wantRed = []string{"BB vs UTG 4bet 5bet.txt", "UTG open.txt", "UTG vs BB 3bet 4bet all in.txt"}
	wantMissing := []string{"CO vs BTN 3bet 4bet all in 10bb.txt", "HJ open.txt", "UTG vs BB 3bet call.txt"}
	if !reflect.DeepEqual(r.UnfilledRedundant, wantRed) || !reflect.DeepEqual(r.UnfilledMissing, wantMissing) {
		t.Errorf("redundant %v missing %v", r.UnfilledRedundant, r.UnfilledMissing)
	}
}

func templateNames(t *testing.T) []string {
	t.Helper()
	return append(func() []string {
		var out []string
		for _, n := range standardNames() {
			out = append(out, n+".txt")
		}
		return out
	}(), HowToFillName)
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func TestInitTemplateThenCheck(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "ranges")
	created, skipped, mapped, err := InitTemplate(dir)
	if err != nil || len(skipped) != 0 || len(mapped) != 0 {
		t.Fatalf("%v %v %v %v", created, skipped, mapped, err)
	}
	want := templateNames(t)
	if !reflect.DeepEqual(sorted(created), sorted(want)) || len(created) != 109 || created[len(created)-1] != HowToFillName {
		t.Errorf("created %d files", len(created))
	}
	for _, name := range created {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || (name != HowToFillName && info.Size() != 0) {
			t.Errorf("%s: %v %v", name, info, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, HowToFillName))
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"繁體中文", "简体中文", "English"} {
		if !strings.Contains(string(data), h) {
			t.Errorf("%s section missing", h)
		}
	}
	r := CheckRanges(mustRanges(t, os.DirFS(dir)))
	if r.Complete || r.SlotsFilled != 0 || r.SlotsTotal != 95 || len(r.UnfilledMissing) != 108 || len(r.UnfilledRedundant) != 0 ||
		len(r.Unreadable) != 0 || len(r.Ignored) != 0 || r.Mapped != 0 {
		t.Errorf("%+v", r)
	}
	cells := 0
	for _, n := range r.MissingCells {
		cells += len(n.Cells)
	}
	if cells != 5+15*3 || len(r.AllMissingNames) != 108 {
		t.Errorf("%d missing cells, %d names", cells, len(r.AllMissingNames))
	}

	mine := filepath.Join(dir, "UTG open.txt")
	if err := os.WriteFile(mine, []byte(someRange), 0o644); err != nil {
		t.Fatal(err)
	}
	created, skipped, mapped, err = InitTemplate(dir)
	if err != nil || len(created) != 0 || len(mapped) != 0 || !reflect.DeepEqual(sorted(skipped), sorted(want)) {
		t.Fatalf("%d %d %v %v", len(created), len(skipped), mapped, err)
	}
	if got, _ := os.ReadFile(mine); string(got) != someRange {
		t.Errorf("existing content changed: %q", got)
	}
}

func TestInitTemplateDoesNotDuplicateAMappedNode(t *testing.T) {
	dir := writeTree(t, map[string][]byte{
		"UTG open 2bb.txt":         []byte(someRange),
		"bb  VS utg OPEN 3BET.txt": nil,
		"sub/HJ open.txt":          []byte(someRange),
	})
	created, skipped, mapped, err := InitTemplate(dir)
	if err != nil || len(skipped) != 0 {
		t.Fatal(created, skipped, err)
	}
	want := []MappedSkip{{"UTG open.txt", "UTG open 2bb.txt"}, {"HJ open.txt", "sub/HJ open.txt"}, {"BB vs UTG open 3bet.txt", "bb  VS utg OPEN 3BET.txt"}}
	sort.Slice(mapped, func(i, j int) bool { return mapped[i].Name < mapped[j].Name })
	sort.Slice(want, func(i, j int) bool { return want[i].Name < want[j].Name })
	if !reflect.DeepEqual(mapped, want) {
		t.Errorf("%v", mapped)
	}
	if len(created) != 109-3 {
		t.Errorf("%d created", len(created))
	}
	for _, name := range []string{"UTG open.txt", "HJ open.txt", "BB vs UTG open 3bet.txt"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s must not be created (%v)", name, err)
		}
	}
	r := CheckRanges(mustRanges(t, os.DirFS(dir)))
	if len(r.Duplicates) != 0 {
		t.Errorf("no duplicates expected: %v", r.Duplicates)
	}
}

func TestInitTemplateSkipsSymlinksWithoutFollowing(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"UTG open.txt": outside, "HJ open.txt": filepath.Join(dir, "nowhere")} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Skip("symlinks not available")
		}
	}
	created, skipped, _, err := InitTemplate(dir)
	if err != nil || len(created) != 107 || !reflect.DeepEqual(sorted(skipped), []string{"HJ open.txt", "UTG open.txt"}) {
		t.Fatalf("%d %v %v", len(created), skipped, err)
	}
	if got, _ := os.ReadFile(outside); string(got) != "keep" {
		t.Errorf("the link target was written: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "nowhere")); err == nil {
		t.Error("a dangling link was followed")
	}
}

func TestInitTemplateErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions are not enforced for root")
	}
	ro := t.TempDir()
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	if _, _, _, err := InitTemplate(ro); err == nil {
		t.Error("read-only folder: want an error")
	}
	if _, _, _, err := InitTemplate(filepath.Join(ro, "sub")); err == nil {
		t.Error("cannot create a folder inside a read-only one")
	}
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := InitTemplate(file); err == nil {
		t.Error("a file is not a folder")
	}
}

func TestInitTemplateFailureMidWayKeepsWhatWasDone(t *testing.T) {
	saved := openFile
	t.Cleanup(func() { openFile = saved })
	calls := 0
	openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		calls++
		if calls == 4 {
			return nil, errors.New("disk full")
		}
		return saved(name, flag, perm)
	}
	dir := t.TempDir()
	created, _, _, err := InitTemplate(dir)
	if err == nil || len(created) != 3 {
		t.Fatalf("%v %v", created, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 3 {
		t.Errorf("%d files on disk", len(entries))
	}
}

func TestInitTemplateRemovesAFileItCannotWrite(t *testing.T) {
	saved := openFile
	t.Cleanup(func() { openFile = saved })
	openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		f, err := saved(name, flag, perm)
		if err != nil || filepath.Base(name) != HowToFillName {
			return f, err
		}
		_ = f.Close()
		return os.Open(name)
	}
	dir := t.TempDir()
	created, _, _, err := InitTemplate(dir)
	if err == nil || len(created) != 108 {
		t.Fatalf("%d %v", len(created), err)
	}
	if _, err := os.Lstat(filepath.Join(dir, HowToFillName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the half-written file must be removed (%v)", err)
	}
}

func TestHowToFillMentionsTheNumbersAndTheAllInOnlyCells(t *testing.T) {
	text := string(howToFill)
	f := DefaultFormat()
	r := CheckRanges(filled(t, nil))
	for _, want := range []string{itoa(len(StandardSet())), itoa(r.SlotsTotal)} {
		if !strings.Contains(text, want) {
			t.Errorf("HOW-TO-FILL.md lacks %q", want)
		}
	}
	for _, p := range f.FiveBetAllInOnly {
		if prefix := p.Later + " vs " + p.Earlier + " 4bet"; !strings.Contains(text, prefix) {
			t.Errorf("HOW-TO-FILL.md lacks %q", prefix)
		}
	}
	if strings.Contains(text, "街") {
		t.Error("the word 街 is not used")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestSyntheticRandomRangesAreComplete(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "parity", "random_s42", "ranges")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("parity corpus not present")
	}
	r := CheckRanges(mustRanges(t, os.DirFS(dir)))
	if !r.Complete || r.SlotsFilled != 95 || r.Mapped != 108 || len(r.MissingActions) != 0 || len(r.MissingCells) != 0 {
		t.Errorf("%+v", r)
	}
}

// A vs_5bet row added to the table extends the check and the template without any other change.
func TestVs5betRowExtendsCheckAndTemplate(t *testing.T) {
	row := fakeVs5bet
	row.Stem = "{ACTOR} vs {VS} 5bet {ACTION}"
	withTheoryRow(t, row)

	set := StandardSet()
	if len(set) != 108+30 {
		t.Fatalf("standard set has %d files", len(set))
	}
	for _, k := range set {
		if back, ok := ParseFilename(CanonicalName(k)); !ok || back != k {
			t.Errorf("%q does not round-trip", CanonicalName(k))
		}
	}
	r := CheckRanges(filled(t, standardNames()))
	if !r.Complete || r.SlotsTotal != 95+30 || r.SlotsFilled != 125 {
		t.Errorf("%+v", r)
	}
	r = CheckRanges(filled(t, without(standardNames(), "UTG vs BB 5bet call", "UTG vs BB 5bet 6bet")))
	if r.Complete || r.SlotsFilled != 123 || len(missingOf(r, "vs_5bet")) != 1 {
		t.Errorf("%+v", r)
	}
	r = CheckRanges(filled(t, without(standardNames(), "UTG vs BB 5bet 6bet")))
	if len(r.MissingActions) != 1 || r.MissingActions[0].Node != "vs_5bet" || !reflect.DeepEqual(r.MissingActions[0].CanonicalNames, []string{"UTG vs BB 5bet 6bet"}) || missingOf(r, "vs_5bet") != nil {
		t.Errorf("%+v", r)
	}
	created, _, _, err := InitTemplate(t.TempDir())
	if err != nil || len(created) != 138+1 {
		t.Errorf("%d %v", len(created), err)
	}
}
