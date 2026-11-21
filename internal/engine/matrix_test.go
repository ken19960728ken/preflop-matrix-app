package engine

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"reflect"
	"strings"
	"testing"
)

const share = 1326.0

func build(t *testing.T, fsys fs.FS, hands []string, hero ...string) *Data {
	t.Helper()
	h := "Hero"
	if len(hero) > 0 {
		h = hero[0]
	}
	var parsed []Hand
	for _, text := range hands {
		parsed = append(parsed, mustParse(t, text))
	}
	return BuildMatrix(parsed, mustRanges(t, fsys), h)
}

func rfs(t *testing.T, files map[string]W) fs.FS { return writeRanges(t, files) }

func th(t *testing.T, d *Data, path ...string) any { return mustAt(t, asJSON(t, d), path...) }

func approxMap(c, r float64) map[string]float64 { return map[string]float64{"c": c, "r": r} }

func TestLayoutHasEveryCell(t *testing.T) { // test_layout_has_every_cell
	d := build(t, rfs(t, nil), nil)
	if !reflect.DeepEqual(d.Open.Keys(), []string{"UTG", "HJ", "CO", "BTN", "SB"}) {
		t.Errorf("open keys %v", d.Open.Keys())
	}
	j := asJSON(t, d)
	for _, node := range []string{"vs_open", "vs_3bet", "vs_4bet", "vs_5bet"} {
		cells := mustAt(t, j, node).(map[string]any)
		_, a := cells["UTG>HJ"]
		_, b := cells["SB>BB"]
		_, c := cells["BB>UTG"]
		if len(cells) != 15 || !a || !b || c {
			t.Errorf("%s: %d cells", node, len(cells))
		}
	}
	nulls := map[string]any{"th": nil, "me": nil, "pool": nil}
	wantEqual(t, mustAt(t, j, "open", "UTG"), nulls)
	wantEqual(t, mustAt(t, j, "vs_5bet", "UTG>BB"), nulls)
}

func topLevelKeys(t *testing.T, b []byte) []string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(string(b)))
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

func TestJSONTopLevelOrderAndShape(t *testing.T) { // P8
	b, err := build(t, rfs(t, nil), nil).MarshalWith(pyText)
	if err != nil {
		t.Fatal(err)
	}
	got := topLevelKeys(t, b)
	if !reflect.DeepEqual(got, []string{"open", "vs_open", "vs_3bet", "vs_4bet", "vs_5bet", "absolute", "meta"}) {
		t.Errorf("order %v", got)
	}
	if !strings.Contains(string(b), `"warnings":[]`) || !strings.Contains(string(b), `"open":{"UTG":{"th":null,"me":null,"pool":null}`) ||
		!strings.Contains(string(b), `"absolute":{"vs_3bet":{"UTG\u003eHJ":{}`) || !strings.Contains(string(b), `"date_from":""`) {
		t.Errorf("shape: %s", b)
	}
}

func TestOpenTheoryIsTheComboShare(t *testing.T) { // test_open_theory_is_the_combo_share
	d := build(t, rfs(t, map[string]W{"UTG Open 2bb": {"AA": 1, "KK": 0.5}}), nil)
	wantApprox(t, th(t, d, "open", "UTG", "th"), map[string]float64{"r": 9 / share})
	wantEqual(t, th(t, d, "open", "HJ", "th"), nil)
}

func TestVsOpenTheory(t *testing.T) { // test_vs_open_theory
	d := build(t, rfs(t, map[string]W{"BB vs UTG open 2bb call": {"AA": 0.5}, "BB vs UTG open 2bb 3bet 9bb": {"AA": 0.5, "KK": 1}}), nil)
	wantApprox(t, th(t, d, "vs_open", "UTG>BB", "th"), approxMap(3/share, 9/share))
}

func TestVsOpenMissingActionCountsAsZero(t *testing.T) { // test_vs_open_missing_action_counts_as_zero
	d := build(t, rfs(t, map[string]W{"BB vs UTG open 2bb call": {"AA": 0.5}}), nil)
	wantApprox(t, th(t, d, "vs_open", "UTG>BB", "th"), approxMap(3/share, 0))
	d = build(t, rfs(t, map[string]W{"BB vs UTG open 2bb 3bet 9bb": {"AA": 0.5}}), nil)
	wantApprox(t, th(t, d, "vs_open", "UTG>BB", "th"), approxMap(0, 3/share))
	wantEqual(t, th(t, build(t, rfs(t, nil), nil), "vs_open", "UTG>BB", "th"), nil)
}

func TestVs3betTheoryIsConditionalOnTheOpenRange(t *testing.T) { // test_vs_3bet_theory_is_conditional_on_the_open_range
	d := build(t, rfs(t, map[string]W{"UTG Open 2bb": {"AA": 1, "KK": 1}, "UTG vs BB 3bet 10bb call": {"AA": 0.5},
		"UTG vs BB 3bet 10bb 4bet 23bb": {"AA": 0.5, "KK": 1}}), nil)
	wantApprox(t, th(t, d, "vs_3bet", "UTG>BB", "th"), approxMap(3.0/12, 9.0/12))
}

func TestLaterStreetWeightsAreJointAndCappedAtTheBase(t *testing.T) { // test_later_street_weights_are_joint_and_capped_at_the_base
	d := build(t, rfs(t, map[string]W{"UTG Open 2bb": {"AA": 0.5, "KK": 0.5}, "UTG vs BB 3bet 10bb call": {"AA": 0.9},
		"UTG vs BB 3bet 10bb 4bet 23bb": {"KK": 0.25}}), nil)
	wantApprox(t, th(t, d, "vs_3bet", "UTG>BB", "th"), approxMap(0.5, 0.25))
}

func TestVs3betMissingActionCountsAsZeroButTheBaseIsRequired(t *testing.T) { // test_vs_3bet_missing_action_counts_as_zero_but_the_base_is_required
	base := W{"AA": 1, "KK": 1}
	d := build(t, rfs(t, map[string]W{"UTG Open 2bb": base, "UTG vs BB 3bet 10bb call": {"AA": 1}}), nil)
	wantApprox(t, th(t, d, "vs_3bet", "UTG>BB", "th"), approxMap(0.5, 0))
	d = build(t, rfs(t, map[string]W{"UTG Open 2bb": base, "UTG vs BB 3bet 10bb 4bet 23bb": {"KK": 1}}), nil)
	wantApprox(t, th(t, d, "vs_3bet", "UTG>BB", "th"), approxMap(0, 0.5))
	wantEqual(t, th(t, build(t, rfs(t, map[string]W{"UTG vs BB 3bet 10bb call": {"AA": 1}}), nil), "vs_3bet", "UTG>BB", "th"), nil)
	wantEqual(t, th(t, build(t, rfs(t, map[string]W{"UTG Open 2bb": base}), nil), "vs_3bet", "UTG>BB", "th"), nil)
}

func TestVs3bet4betAllInFileIsAddedToThe4betRange(t *testing.T) { // test_vs_3bet_4bet_all_in_file_is_added_to_the_4bet_range
	files := map[string]W{"UTG Open 2bb": {"AA": 1, "KK": 1}, "UTG vs BB 3bet 10bb call": {"KK": 1},
		"UTG vs BB 3bet 10bb 4bet 23bb": {"AA": 0.25}, "UTG vs BB 3bet 10bb 4bet all in": {"AA": 0.5}}
	wantApprox(t, th(t, build(t, rfs(t, files), nil), "vs_3bet", "UTG>BB", "th"), approxMap(0.5, 0.375))
	delete(files, "UTG vs BB 3bet 10bb 4bet 23bb")
	r := mustAt(t, asJSON(t, build(t, rfs(t, files), nil)), "vs_3bet", "UTG>BB", "th", "r").(float64)
	if !approx(r, 0.25) {
		t.Errorf("r = %v", r)
	}
}

func TestMissingActionsReport(t *testing.T) { // test_missing_actions_report
	book := mustRanges(t, rfs(t, map[string]W{
		"BB vs UTG open 2bb call": {"AA": 1},
		"UTG Open 2bb":            {"AA": 1}, "UTG vs HJ 3bet 6.5bb 4bet 18bb": {"AA": 1},
		"BB vs HJ open 2bb 3bet 10bb": {"AA": 1}, "BB vs HJ 4bet 23bb call": {"AA": 1},
		"HJ vs CO 3bet 6.5bb call": {"AA": 1},
		"BB vs CO open 2bb call":   {"AA": 1}, "BB vs CO open 2bb 3bet 10bb": {"AA": 1},
	}))
	got := MissingActions(book)
	want := []Message{
		newMsg(MsgMatrixMissingAction, "node", "vs_open", "cell", "UTG>BB", "action", "3bet"),
		newMsg(MsgMatrixMissingAction, "node", "vs_open", "cell", "HJ>BB", "action", "call"),
		newMsg(MsgMatrixMissingAction, "node", "vs_3bet", "cell", "UTG>HJ", "action", "call"),
		newMsg(MsgMatrixMissingAction, "node", "vs_4bet", "cell", "HJ>BB", "action", "5bet"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	zh := []string{"vs_open UTG>BB 缺 3bet，已當 0", "vs_open HJ>BB 缺 call，已當 0", "vs_3bet UTG>HJ 缺 call，已當 0", "vs_4bet HJ>BB 缺 5bet，已當 0"}
	for i, m := range got {
		if text := fmt.Sprintf("%s %s 缺 %s，已當 0", m.Args["node"], m.Args["cell"], m.Args["action"]); text != zh[i] {
			t.Errorf("python text: %s", text)
		}
	}
}

func TestVs4betTheoryUsesThe3betRangeAsBase(t *testing.T) { // test_vs_4bet_theory_uses_the_3bet_range_as_base_and_adds_the_all_in_range
	d := build(t, rfs(t, map[string]W{"BB vs UTG open 2bb 3bet 9bb": {"AA": 1, "KK": 1}, "BB vs UTG 4bet 23bb call": {"KK": 1},
		"BB vs UTG 4bet 23bb 5bet 48bb": {"AA": 0.75}, "BB vs UTG 4bet 23bb 5bet all in": {"AA": 0.75}}), nil)
	wantApprox(t, th(t, d, "vs_4bet", "UTG>BB", "th"), approxMap(6.0/12, 6.0/12))
}

func TestVs4betTheoryWithOnlyOne5betFileOrNone(t *testing.T) { // test_vs_4bet_theory_with_only_one_5bet_file_or_none
	files := map[string]W{"BB vs UTG open 2bb 3bet 9bb": {"AA": 1, "KK": 1}, "BB vs UTG 4bet 23bb call": {"KK": 1},
		"BB vs UTG 4bet 23bb 5bet all in": {"AA": 0.5}}
	r := mustAt(t, asJSON(t, build(t, rfs(t, files), nil)), "vs_4bet", "UTG>BB", "th", "r").(float64)
	if !approx(r, 3.0/12) {
		t.Errorf("r = %v", r)
	}
	delete(files, "BB vs UTG 4bet 23bb 5bet all in")
	wantApprox(t, th(t, build(t, rfs(t, files), nil), "vs_4bet", "UTG>BB", "th"), approxMap(0.5, 0))
}

func TestTheoryAboveOneIsClampedWithAWarning(t *testing.T) { // test_theory_above_one_is_clamped_with_a_warning
	everything := W{}
	for _, h := range handClasses {
		everything[h] = 1
	}
	d := build(t, rfs(t, map[string]W{"BB vs UTG open 2bb call": everything, "BB vs UTG open 2bb 3bet 9bb": everything}), nil)
	j := asJSON(t, d)
	c, r := mustAt(t, j, "vs_open", "UTG>BB", "th", "c").(float64), mustAt(t, j, "vs_open", "UTG>BB", "th", "r").(float64)
	if c+r > 1+1e-9 || !approx(c, r) {
		t.Errorf("c=%v r=%v", c, r)
	}
	if len(d.Meta.Warnings) != 1 {
		t.Fatalf("warnings %v", d.Meta.Warnings)
	}
	m := d.Meta.Warnings[0]
	if m.Key != MsgMatrixTheoryOverOne || m.Args["node"] != "vs_open" || m.Args["cell"] != "UTG>BB" || m.Args["total"] != "2.0000" {
		t.Errorf("%+v", m)
	}
	if text := fmt.Sprintf("%s %s 理論的跟注 + 加注 = %s 超過 1，已按比例縮到 1", m.Args["node"], m.Args["cell"], m.Args["total"]); text != "vs_open UTG>BB 理論的跟注 + 加注 = 2.0000 超過 1，已按比例縮到 1" {
		t.Error(text)
	}
}

func TestTheoryWithinToleranceIsLeftAlone(t *testing.T) { // test_theory_within_tolerance_is_left_alone
	d := build(t, rfs(t, map[string]W{"UTG Open 2bb": {"AA": 1}, "UTG vs BB 3bet 10bb call": {"AA": 0.5}, "UTG vs BB 3bet 10bb 4bet 23bb": {"AA": 0.5}}), nil)
	wantApprox(t, th(t, d, "vs_3bet", "UTG>BB", "th"), approxMap(0.5, 0.5))
	if len(d.Meta.Warnings) != 0 {
		t.Error("no warnings")
	}
}

func TestObservedFrequenciesSplitMeAndPool(t *testing.T) { // test_observed_frequencies_split_me_and_pool
	rest := seq(folds("HJ"), folds("CO"), folds("BTN"), folds("SB"))
	hands := []string{
		makeHand(acts(seq(raises("UTG", 2)), rest, seq(calls("BB"))), H{ID: "A"}),
		makeHand(acts(seq(raises("UTG", 2)), rest, seq(raises("BB", 9), folds("UTG"))), H{ID: "B"}),
		makeHand(acts(seq(raises("UTG", 2)), rest, seq(folds("BB"))), H{ID: "C", Hero: "SB"}),
	}
	j := asJSON(t, build(t, rfs(t, nil), hands))
	wantEqual(t, mustAt(t, j, "vs_open", "UTG>BB", "me"), map[string]any{"c": 0.5, "r": 0.5, "n": 2.0})
	wantEqual(t, mustAt(t, j, "vs_open", "UTG>BB", "pool"), map[string]any{"c": 0.0, "r": 0.0, "n": 1.0})
	wantEqual(t, mustAt(t, j, "open", "UTG", "pool"), map[string]any{"r": 1.0, "n": 3.0})
	wantEqual(t, mustAt(t, j, "open", "UTG", "me"), nil)
	wantEqual(t, mustAt(t, j, "vs_3bet", "UTG>BB", "pool"), map[string]any{"c": 0.0, "r": 0.0, "n": 1.0})
	wantEqual(t, mustAt(t, j, "vs_open", "UTG>BB", "th"), nil)
}

func TestOpenRateCountsLimpsInTheDenominator(t *testing.T) { // test_open_rate_counts_limps_in_the_denominator
	f4 := seq(folds("UTG"), folds("HJ"), folds("CO"), folds("BTN"))
	hands := []string{
		makeHand(acts(f4, seq(calls("SB"), checks("BB"))), H{ID: "A"}),
		makeHand(acts(f4, seq(raises("SB", 3), folds("BB"))), H{ID: "B"}),
		makeHand(acts(f4, seq(folds("SB"))), H{ID: "C"}),
	}
	pool := mustAt(t, asJSON(t, build(t, rfs(t, nil), hands)), "open", "SB", "pool").(map[string]any)
	if !approx(pool["r"].(float64), 1.0/3) || pool["n"] != 3.0 {
		t.Errorf("%v", pool)
	}
}

func TestVs4betAndVs5betCellsAreKeyedOpenerFirst(t *testing.T) { // test_vs_4bet_and_vs_5bet_cells_are_keyed_opener_first
	hand := makeHand(seq(folds("UTG"), folds("HJ"), raises("CO", 2), raises("BTN", 7), folds("SB"), folds("BB"),
		raises("CO", 18), raises("BTN", 40), calls("CO")), H{Hero: "CO"})
	j := asJSON(t, build(t, rfs(t, nil), []string{hand}))
	wantEqual(t, mustAt(t, j, "vs_3bet", "CO>BTN", "me"), map[string]any{"c": 0.0, "r": 1.0, "n": 1.0})
	wantEqual(t, mustAt(t, j, "vs_4bet", "CO>BTN", "pool"), map[string]any{"c": 0.0, "r": 1.0, "n": 1.0})
	wantEqual(t, mustAt(t, j, "vs_5bet", "CO>BTN", "me"), map[string]any{"c": 1.0, "r": 0.0, "n": 1.0})
}

func TestMeta(t *testing.T) { // test_meta
	hands := []string{makeHand(nil, H{ID: "A", Day: "2026/09/30"}), makeHand(nil, H{ID: "B", Day: "2026/10/06", Hero: "SB"})}
	wantEqual(t, mustAt(t, asJSON(t, build(t, rfs(t, nil), hands)), "meta"), map[string]any{
		"hands": 2.0, "hero": "Hero", "hero_hands": 2.0, "warnings": []any{}, "date_from": "2026-09-30", "date_to": "2026-10-06"})
	if got := build(t, rfs(t, nil), hands, "Nobody").Meta.HeroHands; got != 0 {
		t.Errorf("hero_hands %d", got)
	}
	reversed := []string{hands[1], hands[0]}
	if m := build(t, rfs(t, nil), reversed).Meta; m.DateFrom != "2026-09-30" || m.DateTo != "2026-10-06" {
		t.Errorf("%+v", m)
	}
}

func chainHands() []string {
	rest := seq(folds("UTG"), folds("HJ"))
	full := acts(rest, seq(raises("CO", 2), raises("BTN", 7), folds("SB"), folds("BB"), raises("CO", 18), raises("BTN", 40), calls("CO")))
	other := acts(rest, seq(raises("CO", 2), folds("BTN"), folds("SB"), folds("BB")))
	return []string{makeHand(full, H{ID: "A", Hero: "CO"}), makeHand(other, H{ID: "B", Hero: "CO"}),
		makeHand(other, H{ID: "C", Hero: "SB"}), makeHand(other, H{ID: "D", Hero: "SB"})}
}

var chainFiles = map[string]W{"CO Open 2bb": {"AA": 1, "KK": 1}, "CO vs BTN 3bet 7bb call": {"AA": 1}, "CO vs BTN 3bet 7bb 4bet 20bb": {"KK": 1}}

func TestAbsoluteChainsTheConditionalFrequencies(t *testing.T) { // test_absolute_chains_the_conditional_frequencies
	j := asJSON(t, build(t, rfs(t, chainFiles), chainHands()))
	wantEqual(t, mustAt(t, j, "absolute", "vs_3bet", "CO>BTN", "me"), map[string]any{"reach": 1.0, "f": 0.0, "c": 0.0, "r": 1.0, "n": 1.0})
	thAbs := mustAt(t, j, "absolute", "vs_3bet", "CO>BTN", "th").(map[string]any)
	delete(thAbs, "n")
	wantApprox(t, thAbs, map[string]float64{"reach": 12 / share, "f": 0, "c": 0.5 * 12 / share, "r": 0.5 * 12 / share})
}

func TestAbsoluteThNIsNull(t *testing.T) {
	j := asJSON(t, build(t, rfs(t, chainFiles), chainHands()))
	wantEqual(t, mustAt(t, j, "absolute", "vs_3bet", "CO>BTN", "th", "n"), nil)
	p := mustAt(t, j, "absolute", "vs_4bet", "CO>BTN", "pool").(map[string]any)
	if !approx(p["reach"].(float64), 0.25) || !approx(p["r"].(float64), 0.25) {
		t.Errorf("%v", p)
	}
	wantEqual(t, mustAt(t, j, "absolute", "vs_5bet", "CO>BTN", "me"), map[string]any{"reach": 1.0, "f": 0.0, "c": 1.0, "r": 0.0, "n": 1.0})
}

func TestAbsoluteHasEverythingNeededAndPartsAddUp(t *testing.T) { // test_absolute_has_everything_needed_and_parts_add_up
	j := asJSON(t, build(t, rfs(t, chainFiles), chainHands()))
	for _, node := range []string{"vs_3bet", "vs_4bet", "vs_5bet"} {
		cells := mustAt(t, j, "absolute", node).(map[string]any)
		if len(cells) != 15 {
			t.Errorf("%s: %d", node, len(cells))
		}
		for _, row := range cells {
			for who, e := range row.(map[string]any) {
				if who != "th" && who != "me" && who != "pool" {
					t.Errorf("who %s", who)
				}
				m := e.(map[string]any)
				if !approx(m["f"].(float64)+m["c"].(float64)+m["r"].(float64), m["reach"].(float64)) {
					t.Errorf("%s %v", node, m)
				}
			}
		}
	}
	if _, ok := at(j, "absolute", "vs_5bet", "CO>BTN", "th"); ok {
		t.Error("vs_5bet has no th")
	}
	wantEqual(t, mustAt(t, j, "absolute", "vs_3bet", "UTG>BB"), map[string]any{})
}

func TestAbsoluteNeedsThePreviousStreetToHaveData(t *testing.T) { // test_absolute_needs_the_previous_street_to_have_data
	hand := makeHand(seq(folds("UTG"), folds("HJ"), raises("CO", 2), raises("BTN", 7), folds("SB"), folds("BB"), calls("CO")), H{Hero: "CO"})
	j := asJSON(t, build(t, rfs(t, nil), []string{hand}))
	if _, ok := at(j, "absolute", "vs_3bet", "CO>BTN", "me"); !ok {
		t.Error("vs_3bet me")
	}
	if _, ok := at(j, "absolute", "vs_4bet", "CO>BTN", "me"); ok {
		t.Error("vs_4bet me must be absent")
	}
}

func TestVs5betNeverHasTheoryWithoutATableRow(t *testing.T) { // test_vs_5bet_never_has_theory_even_with_vs_4bet_files
	d := build(t, rfs(t, map[string]W{"BB vs UTG open 2bb 3bet 9bb": {"AA": 1, "KK": 1}, "BB vs UTG 4bet 23bb call": {"KK": 1}}), nil)
	j := asJSON(t, d)
	wantApprox(t, mustAt(t, j, "vs_4bet", "UTG>BB", "th"), approxMap(0.5, 0))
	wantEqual(t, mustAt(t, j, "vs_5bet", "UTG>BB", "th"), nil)
	for _, cell := range mustAt(t, j, "vs_5bet").(map[string]any) {
		wantEqual(t, cell.(map[string]any)["th"], nil)
	}
	if len(d.Meta.Warnings) != 0 {
		t.Error("warnings")
	}
}

func TestAbsoluteVs5betCarriesNoTheory(t *testing.T) { // test_absolute_vs_5bet_carries_no_theory_even_when_its_reach_chain_exists
	d := build(t, rfs(t, map[string]W{"UTG Open 2bb": {"AA": 1, "KK": 1}, "UTG vs BB 3bet 9bb call": {"AA": 0.5},
		"UTG vs BB 3bet 9bb 4bet 22bb": {"AA": 0.5}, "BB vs UTG open 2bb 3bet 9bb": {"AA": 1, "KK": 1}, "BB vs UTG 4bet 23bb call": {"KK": 1}}), nil)
	j := asJSON(t, d)
	if _, ok := at(j, "absolute", "vs_3bet", "UTG>BB", "th"); !ok {
		t.Error("vs_3bet th")
	}
	if _, ok := at(j, "absolute", "vs_4bet", "UTG>BB", "th"); !ok {
		t.Error("vs_4bet th")
	}
	for _, row := range mustAt(t, j, "absolute", "vs_5bet").(map[string]any) {
		if _, ok := row.(map[string]any)["th"]; ok {
			t.Error("vs_5bet th")
		}
	}
}

func TestVs5betTheoryIsNotScaledOrWarnedAbout(t *testing.T) { // test_vs_5bet_theory_is_not_scaled_or_warned_about
	everything := W{}
	for _, h := range handClasses {
		everything[h] = 1
	}
	d := build(t, rfs(t, map[string]W{"BB vs UTG open 2bb 3bet 9bb": everything, "BB vs UTG 4bet 23bb call": everything, "BB vs UTG 4bet 23bb 5bet 48bb": everything}), nil)
	wantEqual(t, mustAt(t, asJSON(t, d), "vs_5bet", "UTG>BB", "th"), nil)
	for _, w := range d.Meta.Warnings {
		if w.Args["node"] == "vs_5bet" {
			t.Error("vs_5bet warning")
		}
	}
}

// withTheoryRow adds a row to the theory table for one test.
func withTheoryRow(t *testing.T, row TheoryNode) {
	t.Helper()
	saved := theoryNodes
	theoryNodes = append(append([]TheoryNode(nil), saved...), row)
	t.Cleanup(func() { theoryNodes = saved })
}

var fakeVs5bet = TheoryNode{
	Node:      NodeVs5bet,
	FileRegex: `^({POS}) vs ({POS}) 5bet (call|6bet)$`,
	Map:       func(raw string) string { return raw },
	Base:      &BaseRef{NodeVs3bet, true, "4bet"},
	Actions:   [2]ActionSpec{{"call", []string{"call"}}, {"6bet", []string{"6bet"}}},
}

func TestVs5betTheoryAppearsFromATableRowAlone(t *testing.T) { // extensibility: one more table row, no matrix change
	files := map[string]W{"UTG Open 2bb": {"AA": 1, "KK": 1}, "UTG vs BB 3bet 10bb call": {"KK": 1}, "UTG vs BB 3bet 10bb 4bet 23bb": {"AA": 1},
		"UTG vs BB 5bet call": {"AA": 0.5}, "UTG vs BB 5bet 6bet": {"AA": 0.25}}
	before := asJSON(t, build(t, rfs(t, files), nil))
	wantEqual(t, mustAt(t, before, "vs_5bet", "UTG>BB", "th"), nil)
	if _, ok := ParseFilename("UTG vs BB 5bet call"); ok {
		t.Fatal("no row, no mapping")
	}
	withTheoryRow(t, fakeVs5bet)
	if k, ok := ParseFilename("utg VS bb 5BET 6bet"); !ok || k != rk("vs_5bet", "UTG", "BB", "6bet") {
		t.Errorf("%v %v", k, ok)
	}
	if _, ok := ParseFilename("BB vs UTG 5bet call"); ok {
		t.Error("the opener must be the earlier position")
	}
	after := asJSON(t, build(t, rfs(t, files), nil))
	wantApprox(t, mustAt(t, after, "vs_5bet", "UTG>BB", "th"), approxMap(0.5, 0.25))
	wantEqual(t, mustAt(t, after, "vs_4bet"), mustAt(t, before, "vs_4bet"))
	wantEqual(t, mustAt(t, after, "vs_3bet"), mustAt(t, before, "vs_3bet"))
	wantEqual(t, mustAt(t, after, "open"), mustAt(t, before, "open"))
	abs := mustAt(t, after, "absolute", "vs_5bet", "UTG>BB", "th").(map[string]any)
	if !approx(abs["reach"].(float64), 12/share*0.5) || abs["n"] != nil {
		t.Errorf("absolute th %v", abs)
	}
	missing := MissingActions(mustRanges(t, rfs(t, map[string]W{"UTG Open 2bb": {"AA": 1}, "UTG vs BB 3bet 10bb 4bet 23bb": {"AA": 1}, "UTG vs BB 5bet call": {"AA": 1}})))
	found := false
	for _, m := range missing {
		found = found || (m.Args["node"] == "vs_5bet" && m.Args["action"] == "6bet" && m.Args["cell"] == "UTG>BB")
	}
	if !found {
		t.Errorf("missing %v", missing)
	}
}

func TestOMapAndMarshalErrors(t *testing.T) {
	o := newOMap[int]()
	o.Set("b", 1)
	o.Set("a", 2)
	o.Set("b", 3)
	b, _ := json.Marshal(o) // pointer and value both marshal in insertion order
	bv, _ := json.Marshal(*o)
	if string(b) != `{"b":3,"a":2}` || string(bv) != string(b) || o.Len() != 2 {
		t.Errorf("%s %s", b, bv)
	}
	if _, ok := o.Get("zz"); ok {
		t.Error("Get")
	}
	bad := newOMap[float64]()
	bad.Set("x", math.NaN())
	if _, err := json.Marshal(bad); err == nil {
		t.Error("NaN cannot be marshalled")
	}
	nan := math.NaN()
	empty := func() Data {
		return Data{Open: newOMap[OpenCell](), Pairs: map[string]*OMap[PairCell]{}, Absolute: newOMap[*OMap[AbsRow]]()}
	}
	d := empty()
	if _, err := d.MarshalWith(pyText); err != nil {
		t.Error(err)
	}
	d = empty()
	d.Open.Set("UTG", OpenCell{Me: &OpenObs{R: nan}})
	if _, err := d.MarshalWith(pyText); err == nil {
		t.Error("NaN in open")
	}
	d = empty()
	c := newOMap[PairCell]()
	c.Set("x", PairCell{Th: &PairTheory{C: nan}})
	d.Pairs[NodeVsOpen] = c
	if _, err := d.MarshalWith(pyText); err == nil {
		t.Error("NaN in pair")
	}
	d = empty()
	abs := newOMap[AbsRow]()
	abs.Set("k", AbsRow{Th: &AbsEntry{Reach: nan}})
	d.Absolute.Set("y", abs)
	if _, err := d.MarshalWith(pyText); err == nil {
		t.Error("NaN in absolute")
	}
	d = empty()
	d.Meta.Warnings = []Message{newMsg(MsgParseDuplicatePlayer)}
	if b, err := d.MarshalWith(pyText); err != nil || !strings.Contains(string(b), `"warnings":["duplicate player name"]`) {
		t.Errorf("%s %v", b, err)
	}
}

func TestMarshalJSONIsRefused(t *testing.T) { // misuse guard: warnings are messages, not text
	d := build(t, rfs(t, nil), nil)
	if _, err := json.Marshal(d); err == nil {
		t.Error("pointer")
	}
	if _, err := json.Marshal(*d); err == nil {
		t.Error("value")
	}
}

func TestMarshalWithGivesThePythonWarningShapeForValueAndPointer(t *testing.T) {
	everything := W{}
	for _, h := range handClasses {
		everything[h] = 1
	}
	d := build(t, rfs(t, map[string]W{"BB vs UTG open 2bb call": everything, "BB vs UTG open 2bb 3bet 9bb": everything,
		"UTG vs BB 3bet 10bb call": {"AA": 1}}), nil)
	fromPtr, err := d.MarshalWith(pyText)
	if err != nil {
		t.Fatal(err)
	}
	fromValue, err := (*d).MarshalWith(pyText)
	if err != nil || string(fromPtr) != string(fromValue) {
		t.Fatalf("value and pointer differ: %v", err)
	}
	var m struct {
		Meta struct {
			Warnings []string `json:"warnings"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(fromPtr, &m); err != nil {
		t.Fatal(err)
	}
	want := []string{"vs_open UTG>BB 理論的跟注 + 加注 = 2.0000 超過 1，已按比例縮到 1"}
	if !reflect.DeepEqual(m.Meta.Warnings, want) {
		t.Errorf("%q", m.Meta.Warnings)
	}
	book := mustRanges(t, rfs(t, map[string]W{"BB vs UTG open 2bb call": {"AA": 1}}))
	if got := pyText(MissingActions(book)[0]); got != "vs_open UTG>BB 缺 3bet，已當 0" {
		t.Errorf("%s", got)
	}
}

func TestHeroNameIsEscapedInJSON(t *testing.T) {
	b, _ := build(t, rfs(t, nil), nil, "<b>&").MarshalWith(pyText)
	if strings.Contains(string(b), "<b>") {
		t.Errorf("raw angle brackets in %s", b)
	}
}
