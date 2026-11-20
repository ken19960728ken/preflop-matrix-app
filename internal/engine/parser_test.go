package engine

import (
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func fiveHanded(id string) string {
	var keep []string
	for _, l := range strings.Split(makeHand(nil, H{ID: id}), "\n") {
		if !strings.Contains(l, "p_UTG (") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

func errKey(err error) string {
	var pe *ParseError
	if errors.As(err, &pe) {
		return pe.Msg.Key
	}
	return ""
}

func TestParseHandFields(t *testing.T) { // test_parse_hand_fields
	h := mustParse(t, makeHand(acts(seq(folds("UTG"), raises("HJ", 2), folds("CO"), calls("BTN"), folds("SB"), folds("BB"))), H{ID: "RC42", Day: "2026/09/30"}))
	if h.HandID != "RC42" || h.Date != "2026-09-30" || h.BB.Cmp(rat("0.1")) != 0 {
		t.Fatalf("%+v", h)
	}
	if h.Positions["Hero"] != "BB" || h.Positions["p_HJ"] != "HJ" {
		t.Error("positions")
	}
	var got [][2]string
	for _, a := range h.Actions {
		got = append(got, [2]string{a.Player, a.Kind})
	}
	want := [][2]string{{"p_UTG", "fold"}, {"p_HJ", "raise"}, {"p_CO", "fold"}, {"p_BTN", "call"}, {"p_SB", "fold"}, {"Hero", "fold"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("actions %v", got)
	}
	if h.Actions[1].ToAmount.Cmp(rat("0.20")) != 0 {
		t.Error("to amount")
	}
}

func TestBBIsTheHeaderBigBlind(t *testing.T) { // test_bb_is_the_header_big_blind_not_the_posted_amount
	text := strings.Replace(makeHand(seq(folds("UTG"))), "Hero: posts big blind $0.1", "Hero: posts big blind $0.04 and is all-in", 1)
	if mustParse(t, text).BB.Cmp(rat("0.1")) != 0 {
		t.Error("bb")
	}
}

func TestTrailingWhitespaceIsIgnored(t *testing.T) { // test_trailing_whitespace_is_ignored
	text := strings.Replace(makeHand(seq(folds("UTG"), raises("HJ", 2))), "p_UTG: folds", "p_UTG: folds   ", 1)
	text = strings.ReplaceAll(text, "\n", " \n")
	h := mustParse(t, text)
	if len(h.Actions) != 2 || h.Actions[0].Kind != "fold" || h.Actions[1].Kind != "raise" {
		t.Errorf("%+v", h.Actions)
	}
}

func TestEmptyButtonSeatHasItsOwnError(t *testing.T) { // test_empty_button_seat_has_its_own_error
	_, err := ParseHand(makeHand(seq(folds("UTG")), H{Button: 9}))
	var size TableSizeError
	if errKey(err) != MsgParseButtonEmpty || errors.As(err, &size) {
		t.Fatalf("%v", err)
	}
	var pe *ParseError
	errors.As(err, &pe)
	if pe.Msg.Args["seat"] != "9" {
		t.Error("seat arg")
	}
}

func TestWrongTableSizeNamesThePlayerCount(t *testing.T) { // test_wrong_table_size_message_names_the_player_count
	_, err := ParseHand(fiveHanded("RC1"))
	var size TableSizeError
	if !errors.As(err, &size) || size.Msg.Key != MsgParseTableSize || size.Msg.Args["count"] != "5" || size.Msg.Args["size"] != "6" {
		t.Fatalf("%v", err)
	}
}

func TestPositionsMustAgreeWithBlinds(t *testing.T) { // test_positions_must_agree_with_who_posted_the_blinds
	swapped := strings.Replace(makeHand(seq(folds("UTG"))), "p_SB: posts small blind", "p_UTG: posts small blind", 1)
	if _, err := ParseHand(swapped); errKey(err) != MsgParseBlindsMismatch {
		t.Errorf("swapped: %v", err)
	}
	var keep []string
	for _, l := range strings.Split(makeHand(seq(folds("UTG"))), "\n") {
		if !strings.Contains(l, "small blind") {
			keep = append(keep, l)
		}
	}
	if _, err := ParseHand(strings.Join(keep, "\n")); errKey(err) != MsgParseBlindsMismatch {
		t.Errorf("no sb: %v", err)
	}
}

func TestAllInFlagAndCheckAreKept(t *testing.T) { // test_allin_flag_and_check_are_kept
	h := mustParse(t, makeHand(seq(raisesAllIn("UTG", 100), checks("BB"))))
	if !h.Actions[0].AllIn || h.Actions[1].Kind != "check" {
		t.Errorf("%+v", h.Actions)
	}
}

func TestPostflopLinesAreNotRead(t *testing.T) { // test_postflop_lines_are_not_read
	text := makeHand(seq(raises("CO", 2), folds("BTN"), folds("SB"), calls("BB")),
		H{Extra: []string{"*** FLOP *** [2c 7d 9h]", "garbage that is not a real action"}})
	if len(mustParse(t, text).Actions) != 4 {
		t.Error("actions")
	}
}

func TestUnparseableHandRaisesParseError(t *testing.T) { // test_unparseable_hand_raises_parse_error[<lambda>0..5]
	base := makeHand(seq(folds("UTG"), raises("HJ", 2)))
	for i, mangle := range []func(string) string{
		func(s string) string { return strings.Replace(s, "Poker Hand #", "Poker Game #", 1) },
		func(s string) string { return strings.Replace(s, "*** HOLE CARDS ***\n", "", 1) },
		func(s string) string { return strings.Replace(s, "*** SUMMARY ***", "*** SUMMARY **", 1) },
		func(s string) string { return strings.Replace(s, "p_UTG: folds", "p_UTG: does a handstand", 1) },
		func(s string) string { return strings.Replace(s, "p_UTG: folds", "ghost: folds", 1) },
		func(s string) string { return strings.Replace(s, "big blind", "huge blind", 1) },
	} {
		_, err := ParseHand(mangle(base))
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("lambda%d: %v", i, err)
		}
	}
}

func TestMangledKeys(t *testing.T) {
	base := makeHand(seq(folds("UTG"), raises("HJ", 2)))
	for repl, key := range map[string]string{
		"p_UTG: folds|p_UTG: does a handstand": MsgParseUnrecognisedPreflop,
		"p_UTG: folds|ghost: folds":            MsgParsePlayerNotSeated,
		"big blind|huge blind":                 MsgParseUnrecognisedBeforeDeal,
		"Poker Hand #|Poker Game #":            MsgParseBadHeader,
		"*** HOLE CARDS ***\n|":                MsgParseMissingSections,
		"Seat 3: Hero|Seat 3: p_SB":            MsgParseDuplicatePlayer,
	} {
		parts := strings.SplitN(repl, "|", 2)
		if _, err := ParseHand(strings.Replace(base, parts[0], parts[1], 1)); errKey(err) != key {
			t.Errorf("%s: %v", repl, err)
		}
	}
	ghost := strings.Replace(base, "p_UTG: folds", "ghost: folds", 1)
	_, err := ParseHand(ghost)
	var pe *ParseError
	errors.As(err, &pe)
	if pe.Msg.Args["player"] != "'ghost'" {
		t.Errorf("repr arg: %v", pe.Msg.Args)
	}
	if !strings.Contains(err.Error(), MsgParsePlayerNotSeated) {
		t.Error("Error text")
	}
}

func TestBadAmountsAndSeats(t *testing.T) {
	if _, err := parseAmount(",,"); errKey(err) != MsgParseBadAmount {
		t.Errorf("%v", err)
	}
	if r, err := parseAmount("1,234.50"); err != nil || r.Cmp(rat("1234.5")) != 0 {
		t.Errorf("%v %v", r, err)
	}
	big := strings.Replace(makeHand(seq(folds("UTG"))), "Seat 2:", "Seat 99999999999999999999:", 1)
	if _, err := ParseHand(big); errKey(err) != MsgParseBadSeat {
		t.Errorf("%v", err)
	}
	bigButton := strings.Replace(makeHand(seq(folds("UTG"))), "Seat #1 is", "Seat #99999999999999999999 is", 1)
	if _, err := ParseHand(bigButton); errKey(err) != MsgParseBadSeat {
		t.Errorf("%v", err)
	}
	badRaise := strings.Replace(makeHand(seq(raises("HJ", 2))), "raises $0.20 to $0.20", "raises $0.20 to $,", 1)
	if _, err := ParseHand(badRaise); errKey(err) != MsgParseBadAmount {
		t.Errorf("%v", err)
	}
}

func TestNotSixPlayersIsTableSizeError(t *testing.T) { // test_not_six_players_is_table_size_error
	_, err := ParseHand(fiveHanded("RC1"))
	var size TableSizeError
	if !errors.As(err, &size) {
		t.Error("want TableSizeError")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Error("TableSizeError is also a ParseError")
	}
}

func TestSplitHands(t *testing.T) { // test_split_hands
	text := makeHand(nil, H{ID: "RC1"}) + makeHand(nil, H{ID: "RC2"})
	if len(SplitHands(text)) != 2 {
		t.Error("two hands")
	}
	if got := SplitHands("junk\nPoker Hand #A: x\nPoker Hand #B: y\r\n\r\n"); len(got) != 3 || got[0] != "junk" {
		t.Errorf("preamble: %q", got)
	}
	if got := SplitHands("\xEF\xBB\xBF\xEF\xBB\xBFPoker Hand #A: x"); len(got) != 1 || got[0] != "Poker Hand #A: x" {
		t.Errorf("all leading BOMs are stripped: %q", got)
	}
	if got := SplitHands("x Poker Hand #A: not at line start"); len(got) != 1 {
		t.Errorf("mid-line marker does not split: %q", got)
	}
	if len(SplitHands("  \n ")) != 0 {
		t.Error("blank text")
	}
}

func TestLoadHandsRecursesDedupesAndCounts(t *testing.T) { // test_load_hands_recurses_dedupes_and_counts
	ok := makeHand(seq(folds("UTG")), H{ID: "RC1"})
	dir := writeTree(t, map[string][]byte{
		"gg/a.txt":     []byte(ok + makeHand(seq(folds("UTG")), H{ID: "RC2"})),
		"gg/sub/b.txt": []byte(ok),
		"gg/sub/c.txt": []byte(strings.Replace(makeHand(nil, H{ID: "RC3"}), "big blind", "huge blind", 1)),
		"d.txt":        []byte(fiveHanded("RC4")),
		"ignored.csv":  []byte(ok),
	})
	r := mustHands(t, os.DirFS(dir))
	var ids []string
	for _, h := range r.Hands {
		ids = append(ids, h.HandID)
	}
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, []string{"RC1", "RC2"}) {
		t.Errorf("ids %v", ids)
	}
	if r.Files != 4 || r.Duplicates != 1 || r.Unparseable != 1 || r.WrongTableSize != 1 || r.UnreadableFiles != 0 {
		t.Errorf("%+v", r)
	}
	if len(r.ErrorSamples) != 1 || r.ErrorSamples[0].File != "c.txt" {
		t.Errorf("samples %+v", r.ErrorSamples)
	}
}

func TestDuplicateUnparseableHandIsCountedOnce(t *testing.T) { // test_duplicate_unparseable_hand_is_counted_once
	bad := []byte(strings.Replace(makeHand(nil, H{ID: "RC7"}), "big blind", "huge blind", 1))
	r := mustHands(t, os.DirFS(writeTree(t, map[string][]byte{"a.txt": bad, "b.txt": bad})))
	if r.Unparseable != 1 || r.Duplicates != 1 || len(r.ErrorSamples) != 1 {
		t.Errorf("%+v", r)
	}
}

func TestLoadHandsReadsAByteOrderMark(t *testing.T) { // test_load_hands_reads_a_byte_order_mark
	data := append([]byte("\xEF\xBB\xBF"), makeHand(seq(folds("UTG")), H{ID: "RC1"})...)
	r := mustHands(t, os.DirFS(writeTree(t, map[string][]byte{"a.txt": data})))
	if len(r.Hands) != 1 || r.Hands[0].HandID != "RC1" {
		t.Errorf("%+v", r)
	}
}

func TestLoadHandsEmptyFolder(t *testing.T) { // test_load_hands_empty_folder
	r := mustHands(t, os.DirFS(t.TempDir()))
	if len(r.Hands) != 0 || r.Files != 0 {
		t.Errorf("%+v", r)
	}
}

func TestLoadHandsUnreadableAndSampleCap(t *testing.T) {
	tree := map[string][]byte{"dir.txt/x.md": []byte("x")}
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		tree[n+".txt"] = []byte(strings.Replace(makeHand(nil, H{ID: "ID" + n}), "big blind", "huge blind", 1))
	}
	r := mustHands(t, os.DirFS(writeTree(t, tree)))
	if r.UnreadableFiles != 1 || r.Unparseable != 7 || len(r.ErrorSamples) != 5 {
		t.Errorf("%+v", r)
	}
	if r.ErrorSamples[0].File != "a.txt" || r.ErrorSamples[0].Msg.Key != MsgParseUnrecognisedBeforeDeal {
		t.Errorf("%+v", r.ErrorSamples)
	}
}

func TestInvalidBytesInHandFilesAreReplaced(t *testing.T) {
	data := []byte(makeHand(seq(folds("UTG")), H{ID: "RC1", Extra: []string{"Dealt to junk \xff\xfe"}}))
	r := mustHands(t, os.DirFS(writeTree(t, map[string][]byte{"a.txt": data})))
	if len(r.Hands) != 1 {
		t.Errorf("%+v", r)
	}
}
