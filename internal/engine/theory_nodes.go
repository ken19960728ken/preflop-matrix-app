package engine

import (
	"regexp"
	"strings"
	"sync"
)

// Range keys' action names.
const (
	ActRangeRaise = "raise" // the open-raising range
	ActRangeCall  = "call"
)

// BaseRef names the range a node's frequencies are conditional on: the range of an earlier node,
// of the same decider (WithVs adds the opponent to the key).
type BaseRef struct {
	Node   string
	WithVs bool
	Action string
}

// ActionSpec is one of the two actions a theory cell is made of. Keys are the range-key actions whose
// ranges are added up (union, capped at 1).
type ActionSpec struct {
	Name string
	Keys []string
}

// TheoryNode is one row of the theory table: how range files map to a node and how its theory is built.
// File names, the inputs of a cell, the missing-action check and the action names all come from here.
//
// FileRegex is matched case-insensitively against the whitespace-normalised file stem; {POS} and {SIZE}
// are replaced by the position alternation and the bet-size token. The pair nodes capture
// (decider, opponent, action); open captures (position). Map turns the lower-cased action group into the
// RangeKey action. Base nil means the actions are shares of all hands (no conditioning); open has a single
// action (Actions[0]) and no opponent.
type TheoryNode struct {
	Node      string
	FileRegex string
	Map       func(rawAction string) string
	Base      *BaseRef
	Actions   [2]ActionSpec
	// Stem is the standard file name of the node (without .txt and without bet sizes), with {ACTOR}, {VS} and
	// {ACTION} (the range-key action, "_allin" spelled " all in"); it must parse back to the same key.
	Stem string
	// AllInFile adds a standard file for each further key of the second action (the all in spelling).
	AllInFile bool
}

const sizePattern = `\d+(?:\.\d+)?bb`

func raiseAction(name string) func(string) string {
	return func(raw string) string {
		switch {
		case raw == ActRangeCall:
			return raw
		case strings.Contains(raw, "all"):
			return name + "_allin"
		}
		return name
	}
}

// theoryNodes is the theory table. A vs_5bet row would be one more entry here.
var theoryNodes = []TheoryNode{
	{
		Node:      NodeOpen,
		FileRegex: `^({POS}) open(?: {SIZE})?$`,
		Map:       func(string) string { return ActRangeRaise },
		Actions:   [2]ActionSpec{{"raise", []string{ActRangeRaise}}},
		Stem:      "{ACTOR} open",
	},
	{
		Node:      NodeVsOpen,
		FileRegex: `^({POS}) vs ({POS}) open(?: {SIZE})? (call|3bet|3b)(?: {SIZE})?$`,
		Map: func(raw string) string {
			if raw == "3b" {
				return "3bet"
			}
			return raw
		},
		Actions: [2]ActionSpec{{"call", []string{"call"}}, {"3bet", []string{"3bet"}}},
		Stem:    "{ACTOR} vs {VS} open {ACTION}",
	},
	{
		Node:      NodeVs3bet,
		FileRegex: `^({POS}) vs ({POS}) 3bet(?: {SIZE})? (call|4bet(?: 4bet)?(?: {SIZE})?(?: all ?in)?)(?: {SIZE})?$`,
		Map:       raiseAction("4bet"),
		Base:      &BaseRef{NodeOpen, false, ActRangeRaise},
		Actions:   [2]ActionSpec{{"call", []string{"call"}}, {"4bet", []string{"4bet", "4bet_allin"}}},
		Stem:      "{ACTOR} vs {VS} 3bet {ACTION}",
	},
	{
		Node:      NodeVs4bet,
		FileRegex: `^({POS}) vs ({POS}) 4bet(?: {SIZE})? (call|5bet(?: 5bet)?(?: {SIZE})?(?: all ?in)?)$`,
		Map:       raiseAction("5bet"),
		Base:      &BaseRef{NodeVsOpen, true, "3bet"},
		Actions:   [2]ActionSpec{{"call", []string{"call"}}, {"5bet", []string{"5bet", "5bet_allin"}}},
		Stem:      "{ACTOR} vs {VS} 4bet {ACTION}",
		AllInFile: true,
	},
}

// TheoryNodes returns a copy of the theory table.
func TheoryNodes() []TheoryNode { return append([]TheoryNode(nil), theoryNodes...) }

// theoryRow is the table row of a node, or nil when the node has no theory.
func theoryRow(node string) *TheoryNode {
	for i := range theoryNodes {
		if theoryNodes[i].Node == node {
			return &theoryNodes[i]
		}
	}
	return nil
}

var (
	regexCache   = map[string]*regexp.Regexp{}
	regexCacheMu sync.Mutex
)

func (n *TheoryNode) regex() *regexp.Regexp {
	regexCacheMu.Lock()
	defer regexCacheMu.Unlock()
	if rx, ok := regexCache[n.FileRegex]; ok {
		return rx
	}
	pattern := strings.NewReplacer("{POS}", DefaultFormat().PosAlternation(), "{SIZE}", sizePattern).Replace(n.FileRegex)
	rx := regexp.MustCompile("(?i)" + pattern)
	regexCache[n.FileRegex] = rx
	return rx
}

// pairNode reports whether the node's theory is made of two actions between two players.
func (n *TheoryNode) pairNode() bool { return nodeInfo(n.Node).Pair }

// actorVs gives the decider and the opponent of the pair (earlier, later) in this node.
func (n *TheoryNode) actorVs(earlier, later string) (actor, vs string) {
	if nodeInfo(n.Node).EarlierIsVs {
		return later, earlier
	}
	return earlier, later
}
