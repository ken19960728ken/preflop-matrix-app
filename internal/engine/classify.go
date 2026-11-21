package engine

// Classify preflop decisions into two-player nodes.
//
// A decision belongs to a node only when nobody besides the two players of that node has acted
// voluntarily so far (folds do not count). Limps, cold calls, squeezes and cold 4bets therefore
// drop the decision from every node.

// Node names.
const (
	NodeOpen   = "open"
	NodeVsOpen = "vs_open"
	NodeVs3bet = "vs_3bet"
	NodeVs4bet = "vs_4bet"
	NodeVs5bet = "vs_5bet"
)

// Decision actions.
const (
	ActFold  = "fold"
	ActCall  = "call"
	ActRaise = "raise"
)

// NodeInfo describes how a node's cell is keyed.
type NodeInfo struct {
	Name string
	// Pair is false for open (a cell is one position) and true for the two-player nodes.
	Pair bool
	// EarlierIsVs: the player deciding acts later than the opponent (so the cell key is "<opponent>><decider>").
	EarlierIsVs bool
}

var nodes = []NodeInfo{
	{NodeOpen, false, false},
	{NodeVsOpen, true, true},
	{NodeVs3bet, true, false},
	{NodeVs4bet, true, true},
	{NodeVs5bet, true, false},
}

// Nodes lists the five nodes in matrix order.
func Nodes() []NodeInfo { return append([]NodeInfo(nil), nodes...) }

func nodeInfo(name string) NodeInfo {
	for _, n := range nodes {
		if n.Name == name {
			return n
		}
	}
	return NodeInfo{}
}

// Decision is one classified preflop decision.
type Decision struct {
	Node   string
	Player string
	Pos    string // position of the player deciding
	Vs     string // position of the opponent whose raise is faced ("" for open)
	Action string // fold | call | raise (the next raise level: open / 3bet / 4bet / 5bet / 6bet)
}

// decisionNode returns the node (and the opponent player; "" for open) the player now decides in.
func decisionNode(raisers []string, active map[string]bool, player string, limped bool) (node, opponent string, ok bool) {
	n := len(raisers)
	if n == 0 {
		return NodeOpen, "", len(active) == 0
	}
	if limped {
		return "", "", false
	}
	opener := raisers[0]
	if n == 1 {
		if player != opener && len(active) == 1 && active[opener] {
			return NodeVsOpen, opener, true
		}
		return "", "", false
	}
	threeBettor := raisers[1]
	want := map[string]bool{opener: true, threeBettor: true}
	if len(active) != len(want) {
		return "", "", false
	}
	for p := range want {
		if !active[p] {
			return "", "", false
		}
	}
	switch {
	case n == 2 && player == opener:
		return NodeVs3bet, threeBettor, true
	case n == 3 && player == threeBettor && raisers[2] == opener:
		return NodeVs4bet, opener, true
	case n == 4 && player == opener && raisers[3] == threeBettor:
		return NodeVs5bet, threeBettor, true
	}
	return "", "", false
}

// PreflopDecisions returns every decision of the hand that falls into one of the five nodes.
func PreflopDecisions(hand Hand) []Decision {
	if hand.BB == nil {
		return nil
	}
	for _, a := range hand.Actions {
		if a.Kind == ActRaise && a.ToAmount == nil {
			return nil
		}
	}
	var out []Decision
	var raisers []string
	active := map[string]bool{}
	limped := false
	currentTo, lastSize := hand.BB, hand.BB
	for _, a := range hand.Actions {
		if a.Kind == "check" {
			continue
		}
		kind := a.Kind
		if kind == ActRaise {
			// An all-in raise smaller than the last raise does not reopen the betting: it counts as a call.
			size := newRat().Sub(a.ToAmount, currentTo)
			if a.AllIn && size.Cmp(lastSize) < 0 {
				kind = ActCall
			} else {
				lastSize = size
			}
			currentTo = a.ToAmount
		}
		if node, opponent, ok := decisionNode(raisers, active, a.Player, limped); ok {
			vs := ""
			if opponent != "" {
				vs = hand.Positions[opponent]
			}
			out = append(out, Decision{node, a.Player, hand.Positions[a.Player], vs, kind})
		}
		if kind == ActCall && len(raisers) == 0 {
			limped = true
		}
		if kind != ActFold {
			active[a.Player] = true
		}
		if kind == ActRaise {
			raisers = append(raisers, a.Player)
		}
	}
	return out
}
