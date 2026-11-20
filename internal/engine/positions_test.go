package engine

import (
	"reflect"
	"testing"
)

var six = []Seat{{1, "a"}, {2, "b"}, {3, "c"}, {4, "d"}, {5, "e"}, {6, "f"}}

func TestAssignPositions(t *testing.T) {
	odd := []Seat{{9, "f"}, {2, "a"}, {5, "c"}, {3, "b"}, {7, "e"}, {6, "d"}}
	for _, tc := range []struct { // tests/test_positions.py
		name   string // python test it ports
		seats  []Seat
		button int
		want   map[string]string
	}{
		{"test_button_seat_one", six, 1, map[string]string{"b": "SB", "c": "BB", "d": "UTG", "e": "HJ", "f": "CO", "a": "BTN"}},
		{"test_button_seat_in_the_middle_wraps_around", six, 4, map[string]string{"e": "SB", "f": "BB", "a": "UTG", "b": "HJ", "c": "CO", "d": "BTN"}},
		{"test_button_on_last_seat", six, 6, map[string]string{"a": "SB", "b": "BB", "c": "UTG", "d": "HJ", "e": "CO", "f": "BTN"}},
		{"test_seat_numbers_need_not_be_contiguous_or_sorted", odd, 5, map[string]string{"d": "SB", "e": "BB", "f": "UTG", "a": "HJ", "b": "CO", "c": "BTN"}},
		{"test_not_six_players_is_none (5)", six[:5], 1, nil},
		{"test_not_six_players_is_none (2)", six[:2], 1, nil},
		{"test_button_seat_must_be_taken", six, 7, nil},
	} {
		got, ok := SixMax().AssignPositions(tc.seats, tc.button)
		if (tc.want != nil) != ok || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v %v, want %v", tc.name, got, ok, tc.want)
		}
	}
}

func TestAssignPositionsDoesNotReorderTheCaller(t *testing.T) {
	seats := []Seat{{9, "f"}, {2, "a"}, {5, "c"}, {3, "b"}, {7, "e"}, {6, "d"}}
	before := append([]Seat(nil), seats...)
	SixMax().AssignPositions(seats, 5)
	if !reflect.DeepEqual(seats, before) {
		t.Error("seats were reordered")
	}
}

func TestTableFormatQueries(t *testing.T) {
	if len(Formats()) != 1 || !reflect.DeepEqual(Formats()[0], SixMax()) || DefaultFormat().Size != 6 {
		t.Fatal("only SixMax is registered")
	}
	if i, ok := SixMax().Index("BTN"); !ok || i != 3 {
		t.Errorf("Index(BTN) = %d %v", i, ok)
	}
	if _, ok := SixMax().Index("MP"); ok {
		t.Error("MP is not a position")
	}
	if !SixMax().Earlier("UTG", "BB") || SixMax().Earlier("BB", "UTG") || SixMax().Earlier("BTN", "BTN") || SixMax().Earlier("MP", "BB") || SixMax().Earlier("UTG", "MP") {
		t.Error("Earlier")
	}
	if got := SixMax().Openers(); !reflect.DeepEqual(got, []string{"UTG", "HJ", "CO", "BTN", "SB"}) {
		t.Errorf("Openers = %v", got)
	}
	pairs := SixMax().Pairs()
	if len(pairs) != 15 || pairs[0] != (Pair{"UTG", "HJ"}) || pairs[4] != (Pair{"UTG", "BB"}) || pairs[14] != (Pair{"SB", "BB"}) {
		t.Errorf("Pairs = %v", pairs)
	}
	if SixMax().PosAlternation() != "UTG|HJ|CO|BTN|SB|BB" {
		t.Error("PosAlternation")
	}
	if SixMax().PosJSON() != `["UTG","HJ","CO","BTN","SB","BB"]` {
		t.Errorf("PosJSON = %s", SixMax().PosJSON())
	}
	got := SixMax().Openers()
	got[0] = "X"
	f := SixMax()
	f.Positions[1] = "X"
	if SixMax().Positions[0] != "UTG" || SixMax().Positions[1] != "HJ" || DefaultFormat().Positions[1] != "HJ" || Formats()[0].Positions[1] != "HJ" {
		t.Error("the registered format must not be reachable through returned values")
	}
}
