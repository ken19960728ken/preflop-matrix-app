package engine

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestDottedAndDotlessIInAllIn(t *testing.T) { // Python's re.I matches both for the "i" of "all in"
	for stem, key := range map[string]RangeKey{
		"UTG vs BB 3bet 9bb 4bet all ın":     rk("vs_3bet", "UTG", "BB", "4bet_allin"),
		"UTG vs BB 3bet 9bb 4bet all İn":     rk("vs_3bet", "UTG", "BB", "4bet_allin"),
		"BTN vs BB 3bet 9bb 4bet ALL İN":     rk("vs_3bet", "BTN", "BB", "4bet_allin"),
		"BB vs BTN 4bet 5bet allın":          rk("vs_4bet", "BB", "BTN", "5bet_allin"),
		"BB vs BTN 4bet 22bb 5bet all ın":    rk("vs_4bet", "BB", "BTN", "5bet_allin"),
		"BB vs BTN 4bet 22bb 5bet all in":    rk("vs_4bet", "BB", "BTN", "5bet_allin"),
		"BTN vs BB 3bet 9bb 4bet 22bb all ı": rk("vs_3bet", "BTN", "BB", "4bet"),
	} {
		got, ok := ParseFilename(stem)
		if stem == "BTN vs BB 3bet 9bb 4bet 22bb all ı" {
			if ok {
				t.Errorf("%q must not match, got %v", stem, got)
			}
			continue
		}
		if !ok || got != key {
			t.Errorf("%q: %v %v, want %v", stem, got, ok, key)
		}
	}
	if _, ok := ParseFilename("UTG vs BB 3bet 9bb 4bet all ſn"); ok {
		t.Error("long s is not an i")
	}
}

func TestClassifySameRaiserTwiceUsesSetEquality(t *testing.T) { // expectations from Python preflop_decisions
	utg := func(node, vs, action string) dec { return dec{node, "UTG", vs, action} }
	same(t, "A: one player raises three times alone",
		decide(t, seq(raises("UTG", 2), raises("UTG", 6), raises("UTG", 18))),
		[]dec{{"open", "UTG", "", "raise"}, utg("vs_3bet", "UTG", "raise")})
	same(t, "B: a second player is in the pot, so {opener, 3bettor} is not the active set",
		decide(t, seq(raises("UTG", 2), calls("HJ"), raises("UTG", 6), raises("UTG", 18))),
		[]dec{{"open", "UTG", "", "raise"}, {"vs_open", "HJ", "UTG", "call"}})
	same(t, "C: the chain continues",
		decide(t, seq(raises("UTG", 2), raises("UTG", 6), raises("UTG", 18), raises("UTG", 40), raises("UTG", 90))),
		[]dec{{"open", "UTG", "", "raise"}, utg("vs_3bet", "UTG", "raise"), utg("vs_4bet", "UTG", "raise"), utg("vs_5bet", "UTG", "raise")})
	same(t, "D: caller then another raise",
		decide(t, seq(raises("UTG", 2), calls("HJ"), raises("UTG", 6), raises("UTG", 18), raises("HJ", 40))),
		[]dec{{"open", "UTG", "", "raise"}, {"vs_open", "HJ", "UTG", "call"}})
}

func TestZeroValuesDoNotPanic(t *testing.T) {
	if (TableSizeError{}).Error() != "" || (&ParseError{}).Error() == "x" || (*ParseError)(nil).Error() != "" {
		t.Error("Error on zero values")
	}
	var pe *ParseError
	if errors.As(TableSizeError{}, &pe) {
		t.Error("a zero TableSizeError wraps nothing")
	}
	if got := PreflopDecisions(Hand{}); got != nil {
		t.Errorf("no BB: %v", got)
	}
	if got := PreflopDecisions(Hand{BB: rat("0.1"), Actions: []Action{{Player: "a", Kind: "raise"}}}); got != nil {
		t.Errorf("raise without an amount: %v", got)
	}
}

func TestReturnedTablesAreCopies(t *testing.T) {
	h := HandClasses()
	h[0] = "XX"
	n := Nodes()
	n[0].Name = "x"
	rows := TheoryNodes()
	rows[0].Node = "x"
	if handClasses[0] != "AA" || nodes[0].Name != NodeOpen || theoryNodes[0].Node != NodeOpen {
		t.Error("tables were modified through accessors")
	}
	if len(HandClasses()) != 169 || len(Nodes()) != 5 || len(TheoryNodes()) != 4 {
		t.Error("sizes")
	}
}

func TestEveryBaseIsTheLastStepOfTheReachChain(t *testing.T) {
	withTheoryRow(t, fakeVs5bet) // the extension row obeys the same rule
	for _, row := range TheoryNodes() {
		if row.Base == nil {
			continue
		}
		found := false
		for _, an := range absoluteNodes {
			if an.Node == row.Node {
				found = true
				if last := an.Chain[len(an.Chain)-1]; last != row.Base.Node {
					t.Errorf("%s: reach chain ends at %s but the base is %s", row.Node, last, row.Base.Node)
				}
			}
		}
		if !found {
			t.Errorf("%s has a base range but no reach chain", row.Node)
		}
	}
}

// flipCtx reports cancellation from the n-th call of Err on.
type flipCtx struct {
	context.Context
	left int
}

func (c *flipCtx) Err() error {
	if c.left <= 0 {
		return context.Canceled
	}
	c.left--
	return nil
}

func TestLoadingCanBeCancelled(t *testing.T) {
	dir := writeTree(t, map[string][]byte{
		"UTG Open.txt": []byte("AsAh: 1"), "HJ Open.txt": []byte("AsAh: 1"), "CO Open.txt": []byte("AsAh: 1"),
		"a.txt": []byte(makeHand(nil)),
	})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := LoadHands(cancelled, os.DirFS(dir)); !errors.Is(err, context.Canceled) || r != nil {
		t.Errorf("hands: %v %v", r, err)
	}
	if b, err := LoadRanges(cancelled, os.DirFS(dir)); !errors.Is(err, context.Canceled) || b != nil {
		t.Errorf("ranges: %v %v", b, err)
	}
	// let the walk finish, then cancel between files
	for left := 1; left < 12; left++ {
		_, errH := LoadHands(&flipCtx{context.Background(), left}, os.DirFS(dir))
		_, errR := LoadRanges(&flipCtx{context.Background(), left}, os.DirFS(dir))
		if left < 3 && (errH == nil || errR == nil) {
			t.Errorf("left=%d: should be cancelled (%v %v)", left, errH, errR)
		}
	}
	if _, err := LoadRanges(&flipCtx{context.Background(), 1000}, os.DirFS(dir)); err != nil {
		t.Error(err)
	}
	var cut bool
	for left := 1; left < 40 && !cut; left++ {
		_, err := LoadRanges(&flipCtx{context.Background(), left}, os.DirFS(dir))
		cut = err != nil
	}
	if !cut {
		t.Error("never cancelled")
	}
}
