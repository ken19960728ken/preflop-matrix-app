package engine

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func rk(node, pos, vs, action string) RangeKey { return RangeKey{node, pos, vs, action} }

func mustRange(t *testing.T, text string) Range {
	t.Helper()
	r, err := ParseRangeText(text)
	if err != nil {
		t.Fatalf("ParseRangeText(%q): %v", text, err)
	}
	return r
}

func TestHandClasses(t *testing.T) { // test_hand_classes
	seen := map[string]bool{}
	total := 0
	for _, h := range handClasses {
		seen[h] = true
		total += Combos(h)
	}
	if len(handClasses) != 169 || len(seen) != 169 || total != 1326 {
		t.Fatal("169 classes / 1326 combos")
	}
	if handClasses[0] != "AA" || handClasses[1] != "AKs" || handClasses[13] != "AKo" || handClasses[168] != "22" {
		t.Errorf("row-major order: %v %v %v %v", handClasses[0], handClasses[1], handClasses[13], handClasses[168])
	}
	for _, tc := range [][3]string{{"Kh", "As", "AKo"}, {"Ks", "As", "AKs"}, {"9d", "9c", "99"}} {
		if got, err := HandClass(tc[0], tc[1]); err != nil || got != tc[2] {
			t.Errorf("HandClass(%s,%s) = %q %v", tc[0], tc[1], got, err)
		}
	}
	for _, bad := range [][2]string{{"A", "Ah"}, {"Zs", "Ah"}, {"Ax", "Ah"}, {"é1", "Ah"}, {"Ah", "Aé"}} {
		var re *RangeError
		if _, err := HandClass(bad[0], bad[1]); !errors.As(err, &re) || re.Msg.Key != MsgRangeBadCard {
			t.Errorf("%v: %v", bad, err)
		}
	}
	if _, err := HandClass("As", "As"); err == nil {
		t.Error("same card twice")
	}
}

func TestParseRangeTextFractionsPerClass(t *testing.T) { // test_parse_range_text_fractions_per_class
	r := mustRange(t, rangeText(W{"AA": 1, "AKs": 0.5, "AKo": 0.25, "72o": 1}))
	if r["AA"] != 1 || r["AKs"] != 0.5 || r["AKo"] != 0.25 || r["72o"] != 1 || r["KK"] != 0 || len(r) != 169 {
		t.Errorf("%v", r)
	}
}

func TestMissingWeightDefaultsToOneAndClassWeightIsCapped(t *testing.T) { // test_missing_weight_defaults_to_one_and_class_weight_is_capped
	if !approx(mustRange(t, "AsAh,AsAd")["AA"], 2.0/6) {
		t.Error("default weight")
	}
	if mustRange(t, rangeText(W{"AA": 2}))["AA"] != 1 {
		t.Error("cap")
	}
}

func TestBadRangeTextIsAnError(t *testing.T) { // test_bad_range_text_is_a_value_error[...]
	for text, key := range map[string]string{
		"AsA: 1": MsgRangeBadCombo, "ZsAh: 1": MsgRangeBadCard, "AsAh: lots": MsgRangeNotAFloat, "AsAh: nan": MsgRangeBadWeight,
		"AsAh: inf": MsgRangeBadWeight, "AsAh: -0.5": MsgRangeBadWeight, "AxAh: 1": MsgRangeBadCard, "AsAs: 1": MsgRangeSameCard,
		"AsAh: 1,AhAs: 0.5": MsgRangeDuplicateCombo, "AsAh: 1,AsAh: 1": MsgRangeDuplicateCombo,
		"AsAh: 1e400": MsgRangeBadWeight, "AsAh: 1:2": MsgRangeNotAFloat,
	} {
		_, err := ParseRangeText(text)
		var re *RangeError
		if !errors.As(err, &re) || re.Msg.Key != key {
			t.Errorf("%q: %v (want %s)", text, err, key)
		}
	}
	_, err := ParseRangeText("AsAh: lots")
	var re *RangeError
	errors.As(err, &re)
	if re.Msg.Args["value"] != `' lots'` || err.Error() == "" {
		t.Errorf("value arg is the raw text: %v", re.Msg.Args)
	}
}

func TestRangeTextEdgeCases(t *testing.T) {
	if r := mustRange(t, "AsAh: -0"); r["AA"] != 0 {
		t.Error("-0 is a valid zero weight")
	}
	if r := mustRange(t, "AsAh: 1_0 ,\r\n AsAd:\t0.5　"); r["AA"] != 1 {
		t.Error("underscore, universal newline and Unicode space")
	}
	if r := mustRange(t, " , ,"); r["AA"] != 0 {
		t.Error("only separators")
	}
}

func TestTotalShare(t *testing.T) { // test_total_share
	if !approx(TotalShare(mustRange(t, rangeText(W{"AA": 1, "KK": 0.5}))), 9.0/1326) {
		t.Error("share")
	}
}

func TestTwoPlayerFileNamesMapToNodes(t *testing.T) { // test_two_player_file_names_map_to_nodes[...]
	for stem, key := range map[string]RangeKey{
		"BTN Open 2bb":                          rk("open", "BTN", "", "raise"),
		"btn open 2BB":                          rk("open", "BTN", "", "raise"),
		"bb VS btn OPEN 2bb CALL":               rk("vs_open", "BB", "BTN", "call"),
		"Btn vs Bb 3Bet 9.5bb 4Bet 22bb":        rk("vs_3bet", "BTN", "BB", "4bet"),
		"Bb vs Btn 4Bet 22bb 5Bet All In":       rk("vs_4bet", "BB", "BTN", "5bet_allin"),
		"BTN vs BB 3bet 9.5bb 4bet all in":      rk("vs_3bet", "BTN", "BB", "4bet_allin"),
		"BTN vs BB 3bet 9.5bb 4bet allin":       rk("vs_3bet", "BTN", "BB", "4bet_allin"),
		"BTN vs BB 3bet 9.5bb 4bet 22bb all in": rk("vs_3bet", "BTN", "BB", "4bet_allin"),
		"SB Open 3bb":                           rk("open", "SB", "", "raise"),
		"BB vs BTN open 2bb call":               rk("vs_open", "BB", "BTN", "call"),
		"BB vs BTN open 2bb 3bet 9.5bb":         rk("vs_open", "BB", "BTN", "3bet"),
		"BB vs HJ open 2bb 3b 10bb":             rk("vs_open", "BB", "HJ", "3bet"),
		"BB vs UTG open call":                   rk("vs_open", "BB", "UTG", "call"),
		"BB vs UTG open 3bet 10bb":              rk("vs_open", "BB", "UTG", "3bet"),
		"SB vs CO open 2bb call":                rk("vs_open", "SB", "CO", "call"),
		"BTN vs BB 3bet 9.5bb call":             rk("vs_3bet", "BTN", "BB", "call"),
		"BTN vs BB 3bet 9.5bb 4bet 22bb":        rk("vs_3bet", "BTN", "BB", "4bet"),
		"BB vs BTN 4bet 22bb call":              rk("vs_4bet", "BB", "BTN", "call"),
		"BB vs BTN 4bet 22bb 5bet 5bet 46bb":    rk("vs_4bet", "BB", "BTN", "5bet"),
		"BB vs UTG 4bet 23bb 5bet 48.5bb":       rk("vs_4bet", "BB", "UTG", "5bet"),
		"BTN vs UTG 4bet 5bet 40bb":             rk("vs_4bet", "BTN", "UTG", "5bet"),
		"BB vs BTN 4bet 22bb 5bet all in":       rk("vs_4bet", "BB", "BTN", "5bet_allin"),
		"BTN  Open   2bb":                       rk("open", "BTN", "", "raise"),
		"BTN Open":                              rk("open", "BTN", "", "raise"), // test_open_file_without_a_size_still_maps
		"BTN vs BB 3bet 9.5bb 4bet 4bet":        rk("vs_3bet", "BTN", "BB", "4bet"),
		"BTN vs BB 3bet 4bet 22bb 9bb":          rk("vs_3bet", "BTN", "BB", "4bet"),
	} {
		if got, ok := ParseFilename(stem); !ok || got != key {
			t.Errorf("%q: %v %v, want %v", stem, got, ok, key)
		}
	}
}

func TestMultiwayAndUnknownFileNamesAreIgnored(t *testing.T) { // test_multiway_and_unknown_file_names_are_ignored[...]
	for _, stem := range []string{
		"BB vs BTN open 2bb SB call call", "BB vs BTN open 2bb SB call squeeze 12.5bb", "BB vs BTN open 2bb SB 3bet 8.5bb 4bet 19.5bb",
		"BB vs UTG open 2bb HJ call CO call squeeze 12.5bb", "BTN vs SB 3bet 8.5bb BB 4bet 19.5bb 5bet 39bb", "CO vs HJ 3bet UTG 6.5bb 4bet 15bb",
		"UTG vs HJ 3bet CO 4bet BB 5bet all in", "BTN vs SB call BB squeeze 12.5bb call", "BTN vs UTG open HJ 3bet CO 4bet 5bet 30bb",
		"BTN vs BB 5bet 46bb all in", "CO vs BB 5bet all in call", "SB vs BTN 6bet all in call", "UTG vs BB open 2bb call",
		"BTN vs BTN open 2bb call", "BB vs UTG 4bet 23bb 5bet 48.5bb extra", "notes",
		"BB vs UTG 3bet call", "UTG vs BB 4bet 5bet", "MP open 2bb", "",
	} {
		if got, ok := ParseFilename(stem); ok {
			t.Errorf("%q should be ignored, got %v", stem, got)
		}
	}
}

func ranges(t *testing.T, files map[string][]byte) *RangeBook {
	t.Helper()
	return mustRanges(t, os.DirFS(writeTree(t, files)))
}

func TestLoadRangesBucketsEveryFile(t *testing.T) { // test_load_ranges_buckets_every_file
	files := map[string]string{"UTG Open 2bb": "AsAh: 1", "BB vs UTG open 2bb call": "KsKh: 1", "BB vs UTG open call": "QsQh: 1",
		"BB vs UTG open 2bb SB call call": "JsJh: 1", "BTN Open 2bb": "AsA: 1"}
	tree := map[string][]byte{"sub/readme.md": []byte("not a range")}
	for stem, text := range files {
		tree["sub/"+stem+".txt"] = []byte(text)
	}
	book := ranges(t, tree)
	if book.Files != 5 {
		t.Errorf("files %d", book.Files)
	}
	var keys []RangeKey
	for k := range book.Ranges {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Node < keys[j].Node })
	if !reflect.DeepEqual(keys, []RangeKey{rk("open", "UTG", "", "raise"), rk("vs_open", "BB", "UTG", "call")}) {
		t.Errorf("keys %v", keys)
	}
	if !approx(book.Get("vs_open", "BB", "UTG", "call")["KK"], 1.0/6) {
		t.Error("KK")
	}
	if !reflect.DeepEqual(book.Duplicates, []string{"BB vs UTG open call.txt"}) || !reflect.DeepEqual(book.Ignored, []string{"BB vs UTG open 2bb SB call call.txt"}) {
		t.Errorf("%v %v", book.Duplicates, book.Ignored)
	}
	if len(book.Unreadable) != 1 || book.Unreadable[0].Name != "BTN Open 2bb.txt" || book.Unreadable[0].Msg.Key != MsgRangeBadCombo {
		t.Errorf("%+v", book.Unreadable)
	}
	if book.Get("open", "BTN", "", "raise") != nil {
		t.Error("BTN must be absent")
	}
}

func TestFilesWithAByteOrderMarkAreRead(t *testing.T) { // test_files_with_a_byte_order_mark_are_read
	book := ranges(t, map[string][]byte{"UTG Open 2bb.txt": []byte("\xEF\xBB\xBFAsAh: 1")})
	if !approx(book.Get("open", "UTG", "", "raise")["AA"], 1.0/6) {
		t.Error("AA")
	}
}

func TestBadWeightIsUnreadableNotFatal(t *testing.T) { // test_a_file_with_a_bad_weight_is_unreadable_not_fatal
	book := ranges(t, map[string][]byte{"UTG Open 2bb.txt": []byte("AsAh: nan"), "HJ Open 2bb.txt": []byte("AsAh: 1")})
	if len(book.Ranges) != 1 || book.Get("open", "HJ", "", "raise") == nil || len(book.Unreadable) != 1 {
		t.Errorf("%+v", book)
	}
}

func TestBlankFileIsUnfilled(t *testing.T) { // test_blank_file_is_unfilled_not_unreadable_or_duplicate[6 cases]
	for _, data := range []string{"", "\xEF\xBB\xBF", " \n\t\r\n ", "\xEF\xBB\xBF \n", " 　 \u0085", "\x1c\x1f"} {
		book := ranges(t, map[string][]byte{"UTG Open 2bb.txt": []byte(data)})
		if book.Files != 1 || len(book.Ranges) != 0 || !reflect.DeepEqual(book.Unfilled, []string{"UTG Open 2bb.txt"}) ||
			len(book.Unreadable) != 0 || len(book.Duplicates) != 0 || len(book.Ignored) != 0 {
			t.Errorf("%q: %+v", data, book)
		}
	}
}

func TestBlankFileBeforeAndAfter(t *testing.T) {
	book := ranges(t, map[string][]byte{"a/UTG Open.txt": []byte("\n"), "b/UTG Open 2bb.txt": []byte("AsAh: 1")}) // test_blank_file_in_an_earlier_folder_does_not_take_the_node
	if !approx(book.Get("open", "UTG", "", "raise")["AA"], 1.0/6) || !reflect.DeepEqual(book.Unfilled, []string{"UTG Open.txt"}) || len(book.Duplicates) != 0 {
		t.Errorf("earlier: %+v", book)
	}
	book = ranges(t, map[string][]byte{"a/UTG Open.txt": []byte("AsAh: 1"), "b/UTG Open 2bb.txt": []byte(" ")}) // test_blank_file_in_a_later_folder_is_unfilled_not_a_duplicate
	if !approx(book.Get("open", "UTG", "", "raise")["AA"], 1.0/6) || !reflect.DeepEqual(book.Unfilled, []string{"UTG Open 2bb.txt"}) || len(book.Duplicates) != 0 {
		t.Errorf("later: %+v", book)
	}
}

func TestZeroWidthSpaceAndTwoBOMsAreUnreadable(t *testing.T) {
	book := ranges(t, map[string][]byte{"UTG Open 2bb.txt": []byte("​")}) // test_zero_width_space_is_not_blank_so_the_file_is_unreadable
	if len(book.Unfilled) != 0 || len(book.Unreadable) != 1 {
		t.Errorf("zwsp: %+v", book)
	}
	book = ranges(t, map[string][]byte{"UTG Open 2bb.txt": []byte("\xEF\xBB\xBF\xEF\xBB\xBF")}) // test_two_byte_order_marks_are_unreadable_not_unfilled
	if len(book.Unfilled) != 0 || len(book.Unreadable) != 1 || len(book.Ranges) != 0 {
		t.Errorf("two BOMs: %+v", book)
	}
}

func TestBlankMultiwayFileIsIgnored(t *testing.T) { // test_blank_multiway_file_is_ignored_not_unfilled
	book := ranges(t, map[string][]byte{"BB vs BTN open 2bb SB call call.txt": []byte("\n")})
	if !reflect.DeepEqual(book.Ignored, []string{"BB vs BTN open 2bb SB call call.txt"}) || len(book.Unfilled) != 0 {
		t.Errorf("%+v", book)
	}
}

func TestUnreadableFileAfterTheWinnerIsADuplicate(t *testing.T) { // test_unreadable_file_after_the_winner_is_a_duplicate
	book := ranges(t, map[string][]byte{"a/UTG Open.txt": []byte("AsAh: 1"), "b/UTG Open 2bb.txt": []byte("\xff")})
	if !reflect.DeepEqual(book.Duplicates, []string{"UTG Open 2bb.txt"}) || len(book.Unreadable) != 0 {
		t.Errorf("%+v", book)
	}
}

func TestInvalidUTF8IsUnreadable(t *testing.T) {
	book := ranges(t, map[string][]byte{"UTG Open 2bb.txt": []byte("AsAh: 1\xff")})
	if len(book.Unreadable) != 1 || book.Unreadable[0].Msg.Key != MsgRangeInvalidUTF8 || len(book.Ranges) != 0 {
		t.Errorf("%+v", book)
	}
}

func TestTwoBlankFilesForOneNodeAreBothUnfilled(t *testing.T) { // test_two_blank_files_for_one_node_are_both_unfilled
	book := ranges(t, map[string][]byte{"UTG Open.txt": nil, "UTG Open 2bb.txt": []byte("\n")})
	got := append([]string(nil), book.Unfilled...)
	sort.Strings(got)
	if len(book.Ranges) != 0 || !reflect.DeepEqual(got, []string{"UTG Open 2bb.txt", "UTG Open.txt"}) || len(book.Duplicates) != 0 {
		t.Errorf("%+v", book)
	}
}

func TestFoldersSortByPathComponent(t *testing.T) { // test_folders_sort_by_path_component_so_a_beats_a_space_b
	book := ranges(t, map[string][]byte{"a b/UTG Open.txt": []byte("KsKh: 1"), "a/UTG Open.txt": []byte("AsAh: 1")})
	r := book.Get("open", "UTG", "", "raise")
	if !approx(r["AA"], 1.0/6) || r["KK"] != 0.0 || !reflect.DeepEqual(book.Duplicates, []string{"UTG Open.txt"}) {
		t.Errorf("%+v", book)
	}
}

func TestAFolderNamedLikeARangeFileIsUnreadable(t *testing.T) { // test_a_folder_named_like_a_range_file_is_unreadable
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "UTG Open 2bb.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	book := mustRanges(t, os.DirFS(dir))
	if len(book.Unreadable) != 1 || len(book.Ranges) != 0 || book.Unreadable[0].Msg.Key != MsgLoadFileUnreadable {
		t.Errorf("%+v", book)
	}
}

func TestOnlyCommasIsStillAValidZeroWeightRange(t *testing.T) { // test_only_commas_is_still_a_valid_zero_weight_range
	book := ranges(t, map[string][]byte{"UTG Open 2bb.txt": []byte(",,,")})
	r := book.Get("open", "UTG", "", "raise")
	if len(book.Unfilled) != 0 || len(book.Unreadable) != 0 || len(r) != 169 {
		t.Fatalf("%+v", book)
	}
	for _, h := range handClasses {
		if r[h] != 0 {
			t.Fatal(h)
		}
	}
}

func TestRangeLoadingDoesNotFollowFolderSymlinks(t *testing.T) {
	outside := writeTree(t, map[string][]byte{"UTG Open.txt": []byte("AsAh: 1")})
	dir := writeTree(t, map[string][]byte{"HJ Open.txt": []byte("AsAh: 1")})
	if err := os.Symlink(outside, filepath.Join(dir, "linked")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := os.Symlink(filepath.Join(outside, "UTG Open.txt"), filepath.Join(dir, "CO Open.txt")); err != nil {
		t.Fatal(err)
	}
	book := mustRanges(t, os.DirFS(dir))
	if book.Get("open", "UTG", "", "raise") != nil || book.Get("open", "HJ", "", "raise") == nil || book.Get("open", "CO", "", "raise") == nil || book.Files != 2 {
		t.Errorf("%+v", book)
	}
}
