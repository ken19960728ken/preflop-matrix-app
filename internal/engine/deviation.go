package engine

import (
	"fmt"
	"math"
	"sort"
)

// Deviation summary: cells whose observed frequency differs significantly from theory (n >= MinN and |z| >= ZMin),
// ranked by effect size (Cohen's h).

// Deviation limits and the theory clamp used for the z-score's standard error.
const (
	// MinN is the fewest observations a cell needs to be considered.
	MinN = 30
	// ZMin is the smallest |z| a cell needs to be listed.
	ZMin = 3
	// zTolerance absorbs floating-point error at the ZMin boundary (z = 3 computed as 2.9999999999999996 still counts).
	zTolerance = 1e-9
	// MaxTopN is the largest number of rows per table.
	MaxTopN = 20

	thetaMin = 0.005
	thetaMax = 0.995
)

// Deviation actions.
const (
	DevFold  = "f"
	DevCall  = "c"
	DevRaise = "r"
)

var devActions = []string{DevFold, DevCall, DevRaise}

// Deviation is one row of the summary. The JSON shape is the one internal/report reads (camelCase).
// Obs and Theory are 0-1 fractions, DiffPP = (Obs-Theory)*100.
type Deviation struct {
	Node     string  `json:"node"`
	Decider  string  `json:"decider"`
	Opponent string  `json:"opponent"` // "" for open
	Action   string  `json:"action"`   // f | c | r (open: r only)
	Obs      float64 `json:"obs"`
	Theory   float64 `json:"theory"`
	DiffPP   float64 `json:"diffPP"`
	N        int     `json:"n"`
	Z        float64 `json:"z"`
	Clamped  bool    `json:"clamped"` // theory lay outside [0.005, 0.995] and was clamped in the standard error
	AnchorID string  `json:"anchorID"`
}

// DeviationReport holds the top rows against the player pool and against Hero.
type DeviationReport struct {
	Pool []Deviation `json:"pool"`
	Me   []Deviation `json:"me"`
}

// ValidTopN reports whether n is a usable row count (0..MaxTopN).
func ValidTopN(n int) bool { return n >= 0 && n <= MaxTopN }

// Deviations returns the topN most deviating cells for the pool and for Hero. Callers validate topN with
// ValidTopN; an invalid value is a programming error and returns an error.
func Deviations(data Data, topN int) (DeviationReport, error) {
	if !ValidTopN(topN) {
		return DeviationReport{}, fmt.Errorf("engine: topN %d outside 0..%d", topN, MaxTopN)
	}
	cands := candidates(data)
	return DeviationReport{
		Pool: top(cands[WhoPool], topN),
		Me:   top(cands[WhoMe], topN),
	}, nil
}

// candidate is a Deviation with its place in the tie-break order.
type candidate struct {
	Deviation
	h                  float64 // Cohen's h of the observed vs the (unclamped) theory frequency
	node, cell, action int
}

// cohenH is |2*asin(sqrt(obs)) - 2*asin(sqrt(theory))|, both clamped to [0,1] against rounding noise.
func cohenH(obs, theory float64) float64 {
	return math.Abs(2*math.Asin(math.Sqrt(clamp(obs, 0, 1))) - 2*math.Asin(math.Sqrt(clamp(theory, 0, 1))))
}

func clamp(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

func (c candidate) before(o candidate) bool {
	if c.h != o.h {
		return c.h > o.h
	}
	if a, b := math.Abs(c.Z), math.Abs(o.Z); a != b {
		return a > b
	}
	if a, b := math.Abs(c.DiffPP), math.Abs(o.DiffPP); a != b {
		return a > b
	}
	if c.N != o.N {
		return c.N > o.N
	}
	if c.node != o.node {
		return c.node < o.node
	}
	if c.cell != o.cell {
		return c.cell < o.cell
	}
	return c.action < o.action
}

// top sorts, keeps the strongest action of each cell and returns the first n (never nil).
func top(cands []candidate, n int) []Deviation {
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].before(cands[j]) })
	out := make([]Deviation, 0, n)
	seen := map[[2]int]bool{}
	for _, c := range cands {
		if len(out) == n {
			break
		}
		cell := [2]int{c.node, c.cell}
		if seen[cell] {
			continue
		}
		seen[cell] = true
		out = append(out, c.Deviation)
	}
	return out
}

// candidates builds every eligible (cell, action) per source.
func candidates(data Data) map[string][]candidate {
	out := map[string][]candidate{WhoPool: nil, WhoMe: nil}
	for ni, info := range Nodes() {
		if !info.Deviation {
			continue
		}
		if info.Pair {
			pairCands(out, data.Pairs[info.Name], info, ni)
		} else {
			openCands(out, data.Open, info, ni)
		}
	}
	return out
}

// openCands adds the open cells (raise only).
func openCands(out map[string][]candidate, cells *OMap[OpenCell], info NodeInfo, ni int) {
	if cells == nil {
		return
	}
	for ci, pos := range DefaultFormat().Openers() {
		cell, ok := cells.Get(pos)
		if !ok || cell.Th == nil {
			continue
		}
		for _, who := range []string{WhoPool, WhoMe} {
			obs := cell.Pool
			if who == WhoMe {
				obs = cell.Me
			}
			if obs == nil {
				continue
			}
			d := Deviation{Node: info.Name, Decider: pos, Action: DevRaise, AnchorID: "cell-open-" + pos + "-" + who}
			out[who] = appendCand(out[who], d, obs.R, cell.Th.R, obs.N, ni, ci, len(devActions)-1)
		}
	}
}

// pairCands adds the f, c, r actions of every two-player cell.
func pairCands(out map[string][]candidate, cells *OMap[PairCell], info NodeInfo, ni int) {
	if cells == nil {
		return
	}
	for ci, p := range DefaultFormat().Pairs() {
		cell, ok := cells.Get(p.Earlier + ">" + p.Later)
		if !ok || cell.Th == nil {
			continue
		}
		decider, opponent := p.Earlier, p.Later
		if info.EarlierIsVs {
			decider, opponent = p.Later, p.Earlier
		}
		// BuildMatrix scales a theory with c+r above 1 down to 1, so this clamp only absorbs floating-point error.
		theory := [3]float64{clamp(1-cell.Th.C-cell.Th.R, 0, 1), cell.Th.C, cell.Th.R}
		for _, who := range []string{WhoPool, WhoMe} {
			obs := obsFor(cell, who)
			if obs == nil {
				continue
			}
			observed := [3]float64{clamp(1-obs.C-obs.R, 0, 1), obs.C, obs.R}
			for ai, act := range devActions {
				d := Deviation{
					Node: info.Name, Decider: decider, Opponent: opponent, Action: act,
					AnchorID: "cell-" + info.Name + "-" + p.Earlier + "-" + p.Later + "-" + who + "-" + act,
				}
				out[who] = appendCand(out[who], d, observed[ai], theory[ai], obs.N, ni, ci, ai)
			}
		}
	}
}

// obsFor returns the cell's observation of one source (nil when it has none).
func obsFor(cell PairCell, who string) *PairObs {
	if who == WhoMe {
		return cell.Me
	}
	return cell.Pool
}

// appendCand scores one action and appends it when the cell has enough observations and |z| reaches ZMin.
func appendCand(list []candidate, d Deviation, obs, theory float64, n, node, cell, action int) []candidate {
	if n < MinN {
		return list
	}
	tc := clamp(theory, thetaMin, thetaMax)
	d.Obs, d.Theory, d.N = obs, theory, n
	d.DiffPP = (obs - theory) * 100
	d.Z = (obs - theory) / math.Sqrt(tc*(1-tc)/float64(n))
	d.Clamped = tc != theory
	if !(math.Abs(d.Z) >= ZMin-zTolerance) {
		return list
	}
	return append(list, candidate{d, cohenH(obs, theory), node, cell, action})
}
