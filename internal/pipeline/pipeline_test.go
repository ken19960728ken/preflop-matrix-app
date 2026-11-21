package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/ken19960728ken/preflop-matrix-app/internal/engine"
)

// 合成假資料：UTG 開池 40 次、SB 開池 40 次。

var seats = []string{"BTN", "SB", "BB", "UTG", "HJ", "CO"}

func hand(id string, actions []string) string {
	name := func(pos string) string {
		if pos == "BB" {
			return "Hero"
		}
		return "p_" + pos
	}
	lines := []string{
		fmt.Sprintf("Poker Hand #%s: Hold'em No Limit ($0.05/$0.1) - 2026/10/06 06:55:22", id),
		"Table 'RushAndCash1' 6-max Seat #1 is the button",
	}
	for i, pos := range seats {
		lines = append(lines, fmt.Sprintf("Seat %d: %s ($10 in chips)", i+1, name(pos)))
	}
	lines = append(lines, name("SB")+": posts small blind $0.05", name("BB")+": posts big blind $0.1", "*** HOLE CARDS ***")
	for _, pos := range seats {
		lines = append(lines, "Dealt to "+name(pos)+" ")
	}
	for _, a := range actions {
		pos, verb, _ := strings.Cut(a, " ")
		lines = append(lines, name(pos)+": "+verb)
	}
	lines = append(lines, "*** SUMMARY ***", "Total pot $0.5 | Rake $0.02 | Jackpot $0 | Bingo $0 | Fortune $0 | Tax $0")
	return strings.Join(lines, "\n") + "\n\n"
}

func handsFS() fstest.MapFS {
	var text strings.Builder
	for i := 0; i < 40; i++ {
		text.WriteString(hand("U"+strconv.Itoa(i), []string{"UTG raises $0.20 to $0.20", "HJ folds", "CO folds", "BTN folds", "SB folds", "BB folds"}))
		text.WriteString(hand("S"+strconv.Itoa(i), []string{"UTG folds", "HJ folds", "CO folds", "BTN folds", "SB raises $0.30 to $0.30", "BB folds"}))
	}
	return fstest.MapFS{"gg/a.txt": {Data: []byte(text.String())}}
}

// aces 是只有 AA 六個組合、權重 1 的範圍檔。
const aces = "AsAh: 1,AsAd: 1,AsAc: 1,AhAd: 1,AhAc: 1,AdAc: 1"

// everything 是全部 1326 個組合、權重 1 的範圍檔。
var everything = func() string {
	var cards []string
	for _, r := range "AKQJT98765432" {
		for _, su := range "shdc" {
			cards = append(cards, string(r)+string(su))
		}
	}
	var parts []string
	for i := range cards {
		for j := i + 1; j < len(cards); j++ {
			parts = append(parts, cards[i]+cards[j]+": 1")
		}
	}
	return strings.Join(parts, ",")
}()

func rangesFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for name, body := range files {
		m[name+".txt"] = &fstest.MapFile{Data: []byte(body)}
	}
	return m
}

func build(t *testing.T, ranges fs.FS, top int) *Result {
	t.Helper()
	r, err := Build(context.Background(), Input{Hands: handsFS(), Ranges: ranges, Hero: "Hero", TopN: top})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNoRangesGivesNoticeAndEmptyTheory(t *testing.T) {
	r := build(t, nil, 5)
	if r.Book != nil || r.Check != nil {
		t.Fatalf("Book %v, Check %v", r.Book, r.Check)
	}
	if n := Notice(r); n == nil || n.Key != "report.notice.noRanges" {
		t.Errorf("notice = %+v", n)
	}
	data, html, err := Render(r, "zh-Hant", nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	open := decode(t, data)["open"].(map[string]any)
	for pos, cell := range open {
		if cell.(map[string]any)["th"] != nil {
			t.Errorf("open %s has theory %v", pos, cell)
		}
	}
	if !strings.Contains(string(html), `{"key":"report.notice.noRanges"}`) {
		t.Error("html has no noRanges notice")
	}
	if len(r.Deviations.Pool) != 0 || len(r.Deviations.Me) != 0 {
		t.Errorf("deviations without theory: %+v", r.Deviations)
	}
}

func TestIncompleteRangesNotice(t *testing.T) {
	r := build(t, rangesFS(map[string]string{"UTG open": aces, "BB vs UTG open call": aces}), 5)
	n := Notice(r)
	if n == nil || n.Key != "report.notice.incomplete" || n.Args["nodes"] != "48" || n.Args["actions"] != "1" {
		t.Errorf("notice = %+v", n)
	}
	if Notice(&Result{Check: &engine.CheckReport{Complete: true}}) != nil {
		t.Error("complete ranges must have no notice")
	}
}

type countingFS struct {
	fs.FS
	opens *atomic.Int64
}

func (c countingFS) Open(name string) (fs.File, error) {
	c.opens.Add(1)
	return c.FS.Open(name)
}

func TestSetTopNOnlyRecomputesDeviations(t *testing.T) {
	var opens atomic.Int64
	ranges := rangesFS(map[string]string{"UTG open": aces, "SB open": aces})
	r, err := Build(context.Background(), Input{Hands: countingFS{handsFS(), &opens}, Ranges: ranges, Hero: "Hero", TopN: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Deviations.Pool) != 2 {
		t.Fatalf("pool rows = %d", len(r.Deviations.Pool))
	}
	before := opens.Load()
	if err := r.SetTopN(1); err != nil || len(r.Deviations.Pool) != 1 {
		t.Fatalf("SetTopN(1): %v, rows %d", err, len(r.Deviations.Pool))
	}
	if err := r.SetTopN(0); err != nil || len(r.Deviations.Pool) != 0 {
		t.Fatalf("SetTopN(0): %v", err)
	}
	if err := r.SetTopN(engine.MaxTopN + 1); err == nil {
		t.Error("SetTopN accepted a value above the maximum")
	}
	if err := r.SetTopN(-1); err == nil {
		t.Error("SetTopN accepted a negative value")
	}
	if opens.Load() != before {
		t.Errorf("SetTopN reloaded files: %d opens after %d", opens.Load(), before)
	}
}

func TestBuildSentinels(t *testing.T) {
	ctx := context.Background()
	r, err := Build(ctx, Input{Hands: fstest.MapFS{}, Hero: "Hero", TopN: 5})
	if !errors.Is(err, ErrNoHands) || r == nil || r.Loaded == nil || r.Data != nil {
		t.Errorf("no hands: %v, %+v", err, r)
	}
	r, err = Build(ctx, Input{Hands: handsFS(), Ranges: fstest.MapFS{}, Hero: "Hero", TopN: 5})
	if !errors.Is(err, ErrNoRanges) || r == nil || r.Loaded == nil || r.Book == nil || r.Data != nil {
		t.Errorf("no ranges: %v, %+v", err, r)
	}
	if _, err = Build(ctx, Input{Hands: handsFS(), Hero: "Hero", TopN: 99}); err == nil {
		t.Error("TopN 99 accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = Build(cancelled, Input{Hands: handsFS(), Hero: "Hero"}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}

func TestNoHeroAndWarnings(t *testing.T) {
	r, err := Build(context.Background(), Input{Hands: handsFS(), Hero: "Nobody", TopN: 5,
		Ranges: rangesFS(map[string]string{"BB vs UTG open call": everything, "BB vs UTG open 3bet": everything})})
	if err != nil {
		t.Fatal(err)
	}
	if !r.NoHero {
		t.Error("NoHero = false")
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Key != engine.MsgMatrixTheoryOverOne {
		t.Fatalf("warnings = %+v", r.Warnings)
	}
	zh, en := Translator("zh-Hant")(r.Warnings[0]), Translator("en")(r.Warnings[0])
	if zh == en || !strings.Contains(zh, "超過 1") || strings.Contains(en, "超過") {
		t.Errorf("zh %q, en %q", zh, en)
	}
	data, _, err := Render(r, "en", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	warnings := decode(t, data)["meta"].(map[string]any)["warnings"].([]any)
	if len(warnings) != 1 || warnings[0] != en {
		t.Errorf("json warnings = %v", warnings)
	}
}

func TestRenderOptions(t *testing.T) {
	r := build(t, rangesFS(map[string]string{"UTG open": aces}), 5)
	title := "標題"
	_, html, err := Render(r, "en", &title, 0)
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	for _, want := range []string{`const USER_TITLE = "標題"`, "const DEV = null;", `const DEFAULT_LANG = "en"`} {
		if !strings.Contains(page, want) {
			t.Errorf("html lacks %s", want)
		}
	}
	_, html, err = Render(r, "zh-Hant", nil, 3)
	if err != nil || !strings.Contains(string(html), `const DEV = {"pool":[{`) || !strings.Contains(string(html), "const USER_TITLE = null;") {
		t.Errorf("top 3: %v", err)
	}
	if _, _, err = Render(r, "fr", nil, 3); err == nil {
		t.Error("unsupported language accepted")
	}
	if _, _, err = Render(r, "en", nil, 21); err == nil {
		t.Error("topN 21 accepted")
	}
}
