package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ken19960728ken/preflop-matrix-app/internal/report"
)

var anchorRe = regexp.MustCompile(`^cell-[A-Za-z0-9_-]+$`)

func pairData(node string, cells map[string]PairCell) Data {
	m := newOMap[PairCell]()
	for _, p := range DefaultFormat().Pairs() {
		k := p.Earlier + ">" + p.Later
		if c, ok := cells[k]; ok {
			m.Set(k, c)
		}
	}
	return Data{Pairs: map[string]*OMap[PairCell]{node: m}}
}

func openData(cells map[string]OpenCell) Data {
	m := newOMap[OpenCell]()
	for _, pos := range DefaultFormat().Openers() {
		if c, ok := cells[pos]; ok {
			m.Set(pos, c)
		}
	}
	return Data{Open: m}
}

func pobs(c, r float64, n int) *PairObs { return &PairObs{c, r, n} }
func pth(c, r float64) *PairTheory      { return &PairTheory{c, r} }

func mustDev(t *testing.T, d Data, n int) DeviationReport {
	t.Helper()
	r, err := Deviations(d, n)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMinNBoundary(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want int
	}{{29, 0}, {30, 1}, {31, 1}} {
		d := openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0.2}, Pool: &OpenObs{0.5, tc.n}}})
		if got := len(mustDev(t, d, 5).Pool); got != tc.want {
			t.Errorf("n=%d: got %d rows, want %d", tc.n, got, tc.want)
		}
	}
}

func TestNullsExcluded(t *testing.T) {
	d := openData(map[string]OpenCell{
		"UTG": {Pool: &OpenObs{0.4, 100}, Me: &OpenObs{0.4, 100}},
		"HJ":  {Th: &OpenTheory{0.2}},
		"CO":  {Th: &OpenTheory{0.2}, Me: &OpenObs{0.4, 100}},
	})
	r := mustDev(t, d, 5)
	if len(r.Pool) != 0 || len(r.Me) != 1 || r.Me[0].Decider != "CO" {
		t.Fatalf("got %+v", r)
	}
	d = pairData(NodeVsOpen, map[string]PairCell{"CO>BB": {Pool: pobs(0.1, 0.1, 100)}, "HJ>BB": {Th: pth(0.1, 0.1)}})
	if r := mustDev(t, d, 5); len(r.Pool)+len(r.Me) != 0 {
		t.Fatalf("got %+v", r)
	}
	if r := mustDev(t, Data{}, 5); len(r.Pool)+len(r.Me) != 0 {
		t.Fatalf("empty data: %+v", r)
	}
	d = Data{Pairs: map[string]*OMap[PairCell]{}, Open: newOMap[OpenCell]()}
	if r := mustDev(t, d, 5); len(r.Pool)+len(r.Me) != 0 {
		t.Fatalf("empty maps: %+v", r)
	}
}

func TestZByHand(t *testing.T) {
	d := openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0.3}, Pool: &OpenObs{0.5, 100}}})
	dv := mustDev(t, d, 1).Pool[0]
	want := 0.2 / math.Sqrt(0.21/100)
	if !near(dv.Z, want) || !near(dv.DiffPP, 20) || dv.Clamped || dv.N != 100 || !near(dv.Obs, 0.5) || !near(dv.Theory, 0.3) {
		t.Fatalf("got %+v want z=%v", dv, want)
	}
	if math.Round(dv.Z*10)/10 != 4.4 {
		t.Fatalf("z=%v", dv.Z)
	}
}

func TestZMinBoundary(t *testing.T) {
	// theta .5, n 64: standard error is exactly 0.0625.
	for _, tc := range []struct {
		obs  float64
		want int
	}{{0.5 + 2.99*0.0625, 0}, {0.6875, 1}, {0.5 + 3.01*0.0625, 1}, {0.3125, 1}, {0.5, 0}} {
		d := openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0.5}, Pool: &OpenObs{tc.obs, 64}}})
		if got := len(mustDev(t, d, 5).Pool); got != tc.want {
			t.Errorf("obs=%v: got %d rows, want %d", tc.obs, got, tc.want)
		}
	}
	d := openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0.5}, Pool: &OpenObs{0.6875, 64}}})
	if z := mustDev(t, d, 5).Pool[0].Z; z != ZMin {
		t.Fatalf("z=%v", z)
	}
}

func TestCohenHByHand(t *testing.T) {
	want := 2*math.Asin(math.Sqrt(0.5)) - 2*math.Asin(math.Sqrt(0.3))
	if got := cohenH(0.5, 0.3); !near(got, want) || !near(got, 0.4115168460674878) {
		t.Fatalf("h=%v want %v", got, want)
	}
	if got := cohenH(0.3, 0.5); !near(got, want) {
		t.Fatalf("not symmetric: %v", got)
	}
	if got := cohenH(0.2, 0); !near(got, 2*math.Asin(math.Sqrt(0.2))) {
		t.Fatalf("theta 0: %v", got)
	}
	if got := cohenH(1, 0); !near(got, math.Pi) {
		t.Fatalf("0 vs 1: %v", got)
	}
	if got := cohenH(1+1e-12, -1e-12); !near(got, math.Pi) || math.IsNaN(got) {
		t.Fatalf("rounding noise not clamped: %v", got)
	}
	if cohenH(0.4, 0.4) != 0 {
		t.Fatal("equal frequencies must give 0")
	}
}

func TestRankedByEffectSizeNotZ(t *testing.T) {
	// large n, small difference: z is large, h is small.
	big := OpenCell{Th: &OpenTheory{0.5}, Pool: &OpenObs{0.5 + 0.098, 5795}}
	// small n, large difference: z is smaller, h is large.
	small := OpenCell{Th: &OpenTheory{0.5}, Pool: &OpenObs{0.5 + 0.282, 614}}
	rows := mustDev(t, openData(map[string]OpenCell{"UTG": big, "HJ": small}), 5).Pool
	if len(rows) != 2 || rows[0].Decider != "HJ" || rows[1].Decider != "UTG" {
		t.Fatalf("got %+v", rows)
	}
	if math.Abs(rows[0].Z) >= math.Abs(rows[1].Z) {
		t.Fatalf("setup: z should favour the other order: %v vs %v", rows[0].Z, rows[1].Z)
	}
}

func TestEqualHBreaksTieByZ(t *testing.T) {
	cells := map[string]OpenCell{
		"UTG": {Th: &OpenTheory{0.3}, Pool: &OpenObs{0.5, 100}},
		"HJ":  {Th: &OpenTheory{0.3}, Pool: &OpenObs{0.5, 400}},
	}
	rows := mustDev(t, openData(cells), 5).Pool
	if len(rows) != 2 || rows[0].Decider != "HJ" || rows[0].N != 400 {
		t.Fatalf("got %+v", rows)
	}
}

func TestThetaZeroIsDefined(t *testing.T) {
	d := openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0}, Pool: &OpenObs{0.1, 100}}})
	rows := mustDev(t, d, 1).Pool
	if len(rows) != 1 || !near(rows[0].Z, 0.1/math.Sqrt(0.005*0.995/100)) {
		t.Fatalf("got %+v", rows)
	}
}

func TestThetaClamp(t *testing.T) {
	for _, tc := range []struct {
		theta, obs, tc float64
		clamped        bool
	}{
		{0, 0.1, 0.005, true},
		{1, 0.9, 0.995, true},
		{0.001, 0.1, 0.005, true},
		{0.999, 0.9, 0.995, true},
		{0.005, 0.1, 0.005, false},
		{0.995, 0.9, 0.995, false},
		{0.5, 0.8, 0.5, false},
	} {
		d := openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{tc.theta}, Me: &OpenObs{tc.obs, 50}}})
		dv := mustDev(t, d, 1).Me[0]
		want := (tc.obs - tc.theta) / math.Sqrt(tc.tc*(1-tc.tc)/50)
		if !near(dv.Z, want) || dv.Clamped != tc.clamped || dv.Theory != tc.theta {
			t.Errorf("theta=%v: got z=%v clamped=%v theory=%v, want z=%v clamped=%v", tc.theta, dv.Z, dv.Clamped, dv.Theory, want, tc.clamped)
		}
	}
}

func TestObsEqualsTheoryIsNotListed(t *testing.T) {
	d := openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0.25}, Pool: &OpenObs{0.25, 80}}})
	if rows := mustDev(t, d, 1).Pool; len(rows) != 0 {
		t.Fatalf("got %+v", rows)
	}
}

func TestTheoryFoldClampedToZero(t *testing.T) {
	d := pairData(NodeVsOpen, map[string]PairCell{"CO>BB": {Th: pth(0.6, 0.4+1e-12), Pool: pobs(0.1, 0.1, 100)}})
	r := mustDev(t, d, 5)
	if len(r.Pool) != 1 {
		t.Fatalf("got %+v", r)
	}
	// f is the strongest: obs 0.8 vs theory 0 (clamped from about -1e-12).
	dv := r.Pool[0]
	if dv.Action != DevFold || dv.Theory != 0 || !dv.Clamped || !near(dv.Obs, 0.8) {
		t.Fatalf("got %+v", dv)
	}
	d = pairData(NodeVsOpen, map[string]PairCell{"CO>BB": {Th: pth(0.5, 0.5), Pool: pobs(0.6, 0.4+1e-12, 1000)}})
	if dv := mustDev(t, d, 5).Pool; len(dv) != 1 || dv[0].Action == DevFold {
		t.Fatalf("obs fold not clamped: %+v", dv)
	}
}

func TestSameCellKeepsLargestAbsZ(t *testing.T) {
	// theory f=.2 c=.4 r=.4; observed f=0 c=.5 r=.5: f deviates most.
	d := pairData(NodeVs3bet, map[string]PairCell{"CO>BTN": {Th: pth(0.4, 0.4), Pool: pobs(0.5, 0.5, 100)}})
	r := mustDev(t, d, 20)
	if len(r.Pool) != 1 || r.Pool[0].Action != DevFold {
		t.Fatalf("got %+v", r.Pool)
	}
	// Raise is the strongest here.
	d = pairData(NodeVs3bet, map[string]PairCell{"CO>BTN": {Th: pth(0.4, 0.1), Pool: pobs(0.4, 0.4, 100)}})
	if r := mustDev(t, d, 20); len(r.Pool) != 1 || r.Pool[0].Action != DevRaise {
		t.Fatalf("got %+v", r.Pool)
	}
}

func TestOpenOnlyRaise(t *testing.T) {
	d := openData(map[string]OpenCell{"BTN": {Th: &OpenTheory{0.4}, Pool: &OpenObs{0.6, 100}}})
	dv := mustDev(t, d, 5).Pool
	if len(dv) != 1 || dv[0].Action != DevRaise || dv[0].Node != NodeOpen || dv[0].Opponent != "" || dv[0].Decider != "BTN" {
		t.Fatalf("got %+v", dv)
	}
}

func TestComparatorChain(t *testing.T) {
	base := candidate{Deviation: Deviation{Z: 2, DiffPP: 5, N: 100}, node: 1, cell: 1, action: 1}
	mod := func(f func(c *candidate)) candidate { c := base; f(&c); return c }
	for _, tc := range []struct {
		name string
		a, b candidate
	}{
		{"abs z", mod(func(c *candidate) { c.Z = -3 }), base},
		{"abs diff", mod(func(c *candidate) { c.DiffPP = -6 }), base},
		{"n", mod(func(c *candidate) { c.N = 101 }), base},
		{"node", mod(func(c *candidate) { c.node = 0 }), base},
		{"cell", mod(func(c *candidate) { c.cell = 0 }), base},
		{"action", mod(func(c *candidate) { c.action = 0 }), base},
	} {
		if !tc.a.before(tc.b) || tc.b.before(tc.a) {
			t.Errorf("%s: wrong order", tc.name)
		}
	}
	if base.before(base) {
		t.Error("equal candidates must not be before each other")
	}
}

func TestEndToEndTieOrder(t *testing.T) {
	obs := &OpenObs{0.65, 100}
	th := &OpenTheory{0.4}
	d := openData(map[string]OpenCell{"HJ": {Th: th, Pool: obs}, "UTG": {Th: th, Pool: obs}})
	cell := PairCell{Th: pth(0.4, 0.4), Pool: pobs(0.5, 0.5, 100)}
	d.Pairs = map[string]*OMap[PairCell]{
		NodeVs3bet: pairData(NodeVs3bet, map[string]PairCell{"CO>BB": cell, "CO>BTN": cell}).Pairs[NodeVs3bet],
		NodeVsOpen: pairData(NodeVsOpen, map[string]PairCell{"CO>BB": cell}).Pairs[NodeVsOpen],
	}
	got := []string{}
	for _, dv := range mustDev(t, d, 20).Pool {
		got = append(got, dv.AnchorID)
	}
	// Strongest first (f in the pair cells), then nodes in matrix order, cells in Pairs() order; open last (weaker z).
	want := []string{
		"cell-vs_open-CO-BB-pool-f", "cell-vs_3bet-CO-BTN-pool-f", "cell-vs_3bet-CO-BB-pool-f",
		"cell-open-UTG-pool", "cell-open-HJ-pool",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestTopN(t *testing.T) {
	cells := map[string]OpenCell{}
	for _, p := range DefaultFormat().Openers() {
		cells[p] = OpenCell{Th: &OpenTheory{0.2}, Pool: &OpenObs{0.4, 100}}
	}
	d := openData(cells)
	n := len(DefaultFormat().Openers())
	for _, tc := range []struct{ top, want int }{{0, 0}, {1, 1}, {3, 3}, {20, n}} {
		r := mustDev(t, d, tc.top)
		if len(r.Pool) != tc.want {
			t.Errorf("topN=%d: got %d want %d", tc.top, len(r.Pool), tc.want)
		}
		if r.Pool == nil || r.Me == nil {
			t.Errorf("topN=%d: nil slice", tc.top)
		}
	}
	for _, bad := range []int{-1, 21} {
		if _, err := Deviations(d, bad); err == nil {
			t.Errorf("topN=%d: want error", bad)
		}
	}
	b, _ := json.Marshal(mustDev(t, d, 0))
	if string(b) != `{"pool":[],"me":[]}` {
		t.Fatalf("got %s", b)
	}
}

func TestDeciderOpponentAndAnchors(t *testing.T) {
	cell := PairCell{Th: pth(0.3, 0.3), Pool: pobs(0.5, 0.2, 100), Me: pobs(0.2, 0.5, 100)}
	for _, tc := range []struct {
		node, dec, opp string
	}{
		{NodeVsOpen, "BB", "CO"}, {NodeVs3bet, "CO", "BB"}, {NodeVs4bet, "BB", "CO"},
	} {
		r := mustDev(t, pairData(tc.node, map[string]PairCell{"CO>BB": cell}), 5)
		for _, row := range append(append([]Deviation{}, r.Pool...), r.Me...) {
			if row.Decider != tc.dec || row.Opponent != tc.opp || row.Node != tc.node {
				t.Errorf("%s: got %+v", tc.node, row)
			}
			if !anchorRe.MatchString(row.AnchorID) || !strings.HasPrefix(row.AnchorID, "cell-"+tc.node+"-CO-BB-") {
				t.Errorf("anchor %q", row.AnchorID)
			}
		}
	}
	r := mustDev(t, pairData(NodeVs3bet, map[string]PairCell{"CO>BTN": cell}), 5)
	if r.Pool[0].Decider != "CO" || r.Pool[0].Opponent != "BTN" ||
		!strings.HasSuffix(r.Pool[0].AnchorID, "-pool-"+r.Pool[0].Action) || !strings.HasSuffix(r.Me[0].AnchorID, "-me-"+r.Me[0].Action) {
		t.Fatalf("got %+v", r)
	}
	o := mustDev(t, openData(map[string]OpenCell{"SB": {Th: &OpenTheory{0.3}, Pool: &OpenObs{0.6, 40}, Me: &OpenObs{0.05, 40}}}), 5)
	if o.Pool[0].AnchorID != "cell-open-SB-pool" || o.Me[0].AnchorID != "cell-open-SB-me" {
		t.Fatalf("got %+v", o)
	}
}

func TestPoolAndMeIndependent(t *testing.T) {
	d := openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0.2}, Pool: &OpenObs{0.5, 100}}})
	r := mustDev(t, d, 5)
	if len(r.Pool) != 1 || len(r.Me) != 0 {
		t.Fatalf("got %+v", r)
	}
	d = openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0.2}, Pool: &OpenObs{0.5, 10}, Me: &OpenObs{0.05, 100}}})
	r = mustDev(t, d, 5)
	if len(r.Pool) != 0 || len(r.Me) != 1 || r.Me[0].N != 100 {
		t.Fatalf("got %+v", r)
	}
}

func TestVs5betNeverListed(t *testing.T) {
	cell := PairCell{Th: nil, Pool: pobs(0.4, 0.4, 100), Me: pobs(0.4, 0.4, 100)}
	if r := mustDev(t, pairData(NodeVs5bet, map[string]PairCell{"CO>BTN": cell}), 5); len(r.Pool)+len(r.Me) != 0 {
		t.Fatalf("vs_5bet without th: %+v", r)
	}
	cell.Th = pth(0.1, 0.1)
	if r := mustDev(t, pairData(NodeVs5bet, map[string]PairCell{"CO>BTN": cell}), 5); len(r.Pool)+len(r.Me) != 0 {
		t.Fatalf("vs_5bet with th: %+v", r)
	}
	for _, info := range Nodes() {
		if want := info.Name != NodeVs5bet; info.Deviation != want {
			t.Errorf("%s: Deviation=%v", info.Name, info.Deviation)
		}
	}
}

func TestVs4betMinNPerSource(t *testing.T) {
	for _, tc := range []struct {
		poolN, meN       int
		wantPool, wantMe int
	}{{29, 30, 0, 1}, {30, 29, 1, 0}, {29, 29, 0, 0}, {30, 30, 1, 1}} {
		cell := PairCell{Th: pth(0.3, 0.3), Pool: pobs(0.6, 0.1, tc.poolN), Me: pobs(0.1, 0.6, tc.meN)}
		r := mustDev(t, pairData(NodeVs4bet, map[string]PairCell{"CO>BB": cell}), 5)
		if len(r.Pool) != tc.wantPool || len(r.Me) != tc.wantMe {
			t.Errorf("pool n=%d me n=%d: got %d/%d rows", tc.poolN, tc.meN, len(r.Pool), len(r.Me))
		}
		for _, dv := range append(append([]Deviation{}, r.Pool...), r.Me...) {
			want := tc.poolN
			if strings.Contains(dv.AnchorID, "-me-") {
				want = tc.meN
			}
			if dv.N != want || dv.Node != NodeVs4bet {
				t.Errorf("got %+v", dv)
			}
		}
	}
}

func TestValidTopN(t *testing.T) {
	for n, want := range map[int]bool{-1: false, 0: true, 5: true, 20: true, 21: false} {
		if ValidTopN(n) != want {
			t.Errorf("ValidTopN(%d) != %v", n, want)
		}
	}
}

func TestUnknownKeysIgnored(t *testing.T) {
	d := pairData(NodeVs3bet, nil)
	d.Pairs[NodeVs3bet].Set("XX>YY", PairCell{Th: pth(0.1, 0.1), Pool: pobs(0.5, 0.5, 100)})
	d.Open = newOMap[OpenCell]()
	d.Open.Set("XX", OpenCell{Th: &OpenTheory{0.1}, Pool: &OpenObs{0.5, 100}})
	if r := mustDev(t, d, 5); len(r.Pool) != 0 {
		t.Fatalf("got %+v", r)
	}
}

// The JSON of an engine result is read by internal/report without losing a field.
func TestJSONMatchesReportShape(t *testing.T) {
	cell := PairCell{Th: pth(0.3, 0.3), Pool: pobs(0.5, 0.2, 100), Me: pobs(0.2, 0.5, 100)}
	d := pairData(NodeVs3bet, map[string]PairCell{"CO>BTN": cell})
	d.Open = openData(map[string]OpenCell{"UTG": {Th: &OpenTheory{0.2}, Pool: &OpenObs{0.5, 100}}}).Open
	rep := mustDev(t, d, 5)
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var back report.Deviations
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&back); err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(back)
	if !bytes.Equal(raw, again) {
		t.Fatalf("lost fields:\n%s\n%s", raw, again)
	}
	if len(back.Pool) == 0 || len(back.Me) == 0 {
		t.Fatal("empty round trip")
	}
	if a, b := jsonTags(reflect.TypeOf(Deviation{})), jsonTags(reflect.TypeOf(report.Deviation{})); !reflect.DeepEqual(a, b) {
		t.Fatalf("tags differ: %v vs %v", a, b)
	}
}

func jsonTags(t reflect.Type) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		out = append(out, t.Field(i).Tag.Get("json"))
	}
	return out
}

// Parity corpus: every anchor of the summary is one the rendered report can produce.
func TestParityCorpusDeviationsAndAnchors(t *testing.T) {
	root := "../../testdata/parity/random_s42"
	if _, err := os.Stat(root); err != nil {
		t.Skip("corpus missing")
	}
	ctx := context.Background()
	hands, err := LoadHands(ctx, os.DirFS(root+"/hands"))
	if err != nil {
		t.Fatal(err)
	}
	book, err := LoadRanges(ctx, os.DirFS(root+"/ranges"))
	if err != nil {
		t.Fatal(err)
	}
	data := BuildMatrix(hands.Hands, book, "Hero")
	rep := mustDev(t, *data, MaxTopN)
	if len(rep.Pool) == 0 || len(rep.Me) == 0 {
		t.Fatalf("pool=%d me=%d", len(rep.Pool), len(rep.Me))
	}
	for i, tab := range [][]Deviation{rep.Pool, rep.Me} {
		for j, dv := range tab {
			if j < 5 {
				t.Logf("%s #%d %s %s>%s %s obs=%.4f th=%.4f diff=%+.1f n=%d z=%.1f clamped=%v %s",
					[]string{"pool", "me"}[i], j+1, dv.Node, dv.Decider, dv.Opponent, dv.Action, dv.Obs, dv.Theory, dv.DiffPP, dv.N, dv.Z, dv.Clamped, dv.AnchorID)
			}
			if dv.Node == NodeVs5bet || !anchorRe.MatchString(dv.AnchorID) || dv.N < MinN {
				t.Errorf("bad row %+v", dv)
			}
		}
	}

	raw, err := data.MarshalWith(pyText)
	if err != nil {
		t.Fatal(err)
	}
	dict, err := report.LoadDict(os.DirFS("../i18n/locales"))
	if err != nil {
		t.Fatal(err)
	}
	htmlOut, err := report.RenderHTML(raw, report.Options{Lang: "en", I18N: dict, Positions: DefaultFormat().Positions,
		Deviations: &report.Deviations{Pool: toReport(t, rep.Pool), Me: toReport(t, rep.Me)}})
	if err != nil {
		t.Fatal(err)
	}
	page := string(htmlOut)
	for _, lit := range []string{"cell-${sec}-${op}-${de}-${who}-${k}", "cell-open-${p}-me", "cell-open-${p}-pool"} {
		if !strings.Contains(page, lit) {
			t.Fatalf("template lacks id rule %q", lit)
		}
	}
	// The template gives an id to every cell of the me/pool rows that has theory and that source's data.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	parsed := map[string]map[string]map[string]json.RawMessage{}
	for _, info := range Nodes() {
		var cells map[string]map[string]json.RawMessage
		if err := json.Unmarshal(top[info.Name], &cells); err != nil {
			t.Fatal(err)
		}
		parsed[info.Name] = cells
	}
	for i, tab := range [][]Deviation{rep.Pool, rep.Me} {
		who := []string{WhoPool, WhoMe}[i]
		for _, dv := range tab {
			var key string
			if dv.Node == NodeOpen {
				key = dv.Decider
			} else if nodes := nodeInfo(dv.Node); nodes.EarlierIsVs {
				key = dv.Opponent + ">" + dv.Decider
			} else {
				key = dv.Decider + ">" + dv.Opponent
			}
			cell, ok := parsed[dv.Node][key]
			if !ok || string(cell["th"]) == "null" || string(cell[who]) == "null" || cell[who] == nil {
				t.Errorf("%s: cell %s/%s has no id in the report", dv.AnchorID, dv.Node, key)
			}
			if dv.Node != NodeOpen && !strings.HasPrefix(dv.AnchorID, "cell-"+dv.Node+"-"+strings.Replace(key, ">", "-", 1)+"-"+who+"-") {
				t.Errorf("anchor %s does not follow the data key %s", dv.AnchorID, key)
			}
		}
	}
}

func toReport(t *testing.T, rows []Deviation) []report.Deviation {
	t.Helper()
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var out []report.Deviation
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
