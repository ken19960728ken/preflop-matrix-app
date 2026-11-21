package engine

import (
	"reflect"
	"testing"
)

type dec struct{ node, pos, vs, action string }

func decide(t *testing.T, actions []act, opts ...H) []dec {
	t.Helper()
	var out []dec
	for _, d := range PreflopDecisions(mustParse(t, makeHand(actions, opts...))) {
		out = append(out, dec{d.Node, d.Pos, d.Vs, d.Action})
	}
	return out
}

var (
	openUTGHJ = []dec{{"open", "UTG", "", "fold"}, {"open", "HJ", "", "fold"}}
	foldToCO  = seq(folds("UTG"), folds("HJ"))
)

func cat(parts ...[]dec) []dec {
	var out []dec
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func same(t *testing.T, name string, got, want []dec) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n got %v\nwant %v", name, got, want)
	}
}

func TestClassify(t *testing.T) {
	same(t, "test_open_then_vs_open_for_each_later_player",
		decide(t, acts(foldToCO, seq(raises("CO", 2), folds("BTN"), folds("SB"), calls("BB")))),
		cat(openUTGHJ, []dec{{"open", "CO", "", "raise"}, {"vs_open", "BTN", "CO", "fold"}, {"vs_open", "SB", "CO", "fold"}, {"vs_open", "BB", "CO", "call"}}))
	same(t, "test_a_cold_caller_removes_the_decisions_behind_it",
		decide(t, acts(foldToCO, seq(raises("CO", 2), calls("BTN"), calls("SB"), calls("BB")))),
		cat(openUTGHJ, []dec{{"open", "CO", "", "raise"}, {"vs_open", "BTN", "CO", "call"}}))
	same(t, "test_vs_open_3bet_and_opener_facing_it",
		decide(t, seq(folds("UTG"), raises("HJ", 2), folds("CO"), folds("BTN"), folds("SB"), raises("BB", 9), folds("HJ"))),
		[]dec{{"open", "UTG", "", "fold"}, {"open", "HJ", "", "raise"}, {"vs_open", "CO", "HJ", "fold"}, {"vs_open", "BTN", "HJ", "fold"},
			{"vs_open", "SB", "HJ", "fold"}, {"vs_open", "BB", "HJ", "raise"}, {"vs_3bet", "HJ", "BB", "fold"}})
	var walk []dec
	for _, p := range []string{"UTG", "HJ", "CO", "BTN", "SB"} {
		walk = append(walk, dec{"open", p, "", "fold"})
	}
	same(t, "test_walk_has_only_open_folds",
		decide(t, seq(folds("UTG"), folds("HJ"), folds("CO"), folds("BTN"), folds("SB"))), walk)
	same(t, "test_sb_limp_is_an_open_decision_and_the_bb_check_is_not_a_decision",
		decide(t, seq(folds("UTG"), folds("HJ"), folds("CO"), folds("BTN"), calls("SB"), checks("BB"))),
		append(walk[:4:4], dec{"open", "SB", "", "call"}))
	got := decide(t, seq(folds("UTG"), folds("HJ"), folds("CO"), folds("BTN"), raises("SB", 3), raises("BB", 10), calls("SB")))
	same(t, "test_blind_vs_blind_chain", got[len(got)-3:],
		[]dec{{"open", "SB", "", "raise"}, {"vs_open", "BB", "SB", "raise"}, {"vs_3bet", "SB", "BB", "call"}})
	same(t, "test_vs_3bet_heads_up_ignores_players_who_fold_in_between",
		decide(t, acts(foldToCO, seq(raises("CO", 2), raises("BTN", 7), folds("SB"), folds("BB"), calls("CO")))),
		cat(openUTGHJ, []dec{{"open", "CO", "", "raise"}, {"vs_open", "BTN", "CO", "raise"}, {"vs_3bet", "CO", "BTN", "call"}}))
}

func TestVs3betActions(t *testing.T) { // test_vs_3bet_actions[last0-call, last1-fold, last2-raise]
	for last, want := range map[string]struct {
		a      act
		action string
	}{"call": {calls("CO"), "call"}, "fold": {folds("CO"), "fold"}, "raise": {raisesAllIn("CO", 100), "raise"}} {
		got := decide(t, acts(foldToCO, seq(raises("CO", 2), raises("BTN", 7), folds("SB"), folds("BB"), want.a)))
		if got[len(got)-1] != (dec{"vs_3bet", "CO", "BTN", want.action}) {
			t.Errorf("%s: %v", last, got[len(got)-1])
		}
	}
}

func TestClassifyDeepChains(t *testing.T) {
	same(t, "test_vs_4bet_and_vs_5bet_chain",
		decide(t, acts(foldToCO, seq(raises("CO", 2), raises("BTN", 7), folds("SB"), folds("BB"), raises("CO", 18), raises("BTN", 40), raisesAllIn("CO", 100))))[3:],
		[]dec{{"vs_open", "BTN", "CO", "raise"}, {"vs_3bet", "CO", "BTN", "raise"}, {"vs_4bet", "BTN", "CO", "raise"}, {"vs_5bet", "CO", "BTN", "raise"}})
	base := acts(foldToCO, seq(raises("CO", 2), raises("BTN", 7), folds("SB"), folds("BB"), raises("CO", 18)))
	g := decide(t, acts(base, seq(folds("BTN"))))
	if g[len(g)-1] != (dec{"vs_4bet", "BTN", "CO", "fold"}) {
		t.Errorf("vs_4bet fold: %v", g[len(g)-1])
	}
	g = decide(t, acts(base, seq(raises("BTN", 40), calls("CO"))))
	if g[len(g)-1] != (dec{"vs_5bet", "CO", "BTN", "call"}) { // test_vs_4bet_fold_and_vs_5bet_call
		t.Errorf("vs_5bet call: %v", g[len(g)-1])
	}
}

func TestClassifyRemovals(t *testing.T) {
	same(t, "test_squeeze_removes_the_squeezer_and_the_opener_but_keeps_the_caller",
		decide(t, acts(foldToCO, seq(raises("CO", 2), calls("BTN"), raises("SB", 11), folds("BB"), folds("CO"), folds("BTN")))),
		cat(openUTGHJ, []dec{{"open", "CO", "", "raise"}, {"vs_open", "BTN", "CO", "call"}}))
	want := cat(openUTGHJ, []dec{{"open", "CO", "", "raise"}, {"vs_open", "BTN", "CO", "raise"}})
	same(t, "test_cold_call_of_the_3bet_removes_the_opener_decision",
		decide(t, acts(foldToCO, seq(raises("CO", 2), raises("BTN", 7), calls("SB"), folds("BB"), calls("CO")))), want)
	same(t, "test_cold_4bet_removes_both_original_players",
		decide(t, acts(foldToCO, seq(raises("CO", 2), raises("BTN", 7), raises("SB", 25), folds("BB"), folds("CO"), folds("BTN")))), want)
	for _, d := range decide(t, acts(foldToCO, seq(raises("CO", 2), raises("BTN", 7), calls("SB"), folds("BB"), raises("CO", 20), folds("SB"), folds("BTN")))) {
		if d.node == "vs_3bet" || d.node == "vs_4bet" || d.node == "vs_5bet" { // test_third_player_in_the_pot_removes_vs_4bet
			t.Errorf("unexpected %v", d)
		}
	}
	same(t, "test_limp_removes_every_later_node_even_if_the_limper_reraises",
		decide(t, seq(calls("UTG"), raises("HJ", 4), folds("CO"), folds("BTN"), folds("SB"), folds("BB"), raises("UTG", 12), calls("HJ"))),
		[]dec{{"open", "UTG", "", "call"}})
	same(t, "test_incomplete_all_in_raise_counts_as_a_call",
		decide(t, acts(foldToCO, seq(raises("CO", 2), raisesAllIn("BTN", 2.5), folds("SB"), folds("BB"), calls("CO")))),
		cat(openUTGHJ, []dec{{"open", "CO", "", "raise"}, {"vs_open", "BTN", "CO", "call"}}))
}

func TestDecisionCarriesThePlayerName(t *testing.T) { // test_decision_carries_the_player_name
	hand := mustParse(t, makeHand(seq(folds("UTG"), folds("HJ"), folds("CO"), folds("BTN"), raises("SB", 3), folds("BB")), H{Hero: "SB"}))
	var got [][2]string
	for _, d := range PreflopDecisions(hand)[4:] {
		got = append(got, [2]string{d.Player, d.Pos})
	}
	if !reflect.DeepEqual(got, [][2]string{{"Hero", "SB"}, {"p_BB", "BB"}}) {
		t.Errorf("%v", got)
	}
}

func TestNodeInfo(t *testing.T) {
	if nodeInfo("nope").Name != "" || !nodeInfo(NodeVs4bet).EarlierIsVs || nodeInfo(NodeOpen).Pair {
		t.Error("nodeInfo")
	}
}
