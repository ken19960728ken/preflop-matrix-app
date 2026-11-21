package engine

import (
	"context"
	"errors"
	"io/fs"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Python's \S: anything that is not str.isspace.
const nonSpace = `[^\t-\r\x1c-\x20\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

const amountPattern = `\$([\d,]+(?:\.\d+)?)`

var (
	reHeader = regexp.MustCompile(`^Poker Hand #(` + nonSpace + `+): Hold'em No Limit \(` + amountPattern + `/` + amountPattern +
		`\) - (\d{4})/(\d{2})/(\d{2}) \d{2}:\d{2}:\d{2}$`)
	reTable     = regexp.MustCompile(`^Table '[^']+' \d+-max Seat #(\d+) is the button$`)
	reSeat      = regexp.MustCompile(`^Seat (\d+): (.+?) \(` + amountPattern + ` in chips\)$`)
	rePost      = regexp.MustCompile(`^(.+?): posts (small|big) blind ` + amountPattern + `(?: and is all-in)?$`)
	reHandID    = regexp.MustCompile(`^Poker Hand #(` + nonSpace + `+):`)
	reFoldCheck = regexp.MustCompile(`^(.+?): (folds|checks)$`)
	reCall      = regexp.MustCompile(`^(.+?): calls ` + amountPattern + `( and is all-in)?$`)
	reRaise     = regexp.MustCompile(`^(.+?): raises ` + amountPattern + ` to ` + amountPattern + `( and is all-in)?$`)
	reIgnored   = regexp.MustCompile(`^(Dealt to .*|Uncalled bet \(.*|Cash Drop to Pot .*|.+: shows \[.*|.+: Bet & Muck folds|.*All-in Insurance.*)$`)
)

const (
	holeCards  = "*** HOLE CARDS ***"
	summary    = "*** SUMMARY ***"
	handStart  = "Poker Hand #"
	maxSamples = 5
)

// ParseError: the text is not a hand this parser understands.
type ParseError struct{ Msg Message }

func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	return messageString(e.Msg)
}

// TableSizeError: a well-formed hand that does not have exactly the table's number of players.
type TableSizeError struct{ *ParseError }

func (e TableSizeError) Unwrap() error {
	if e.ParseError == nil {
		return nil
	}
	return e.ParseError
}

func messageString(m Message) string {
	keys := make([]string, 0, len(m.Args))
	for k := range m.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{m.Key}
	for _, k := range keys {
		parts = append(parts, k+"="+m.Args[k])
	}
	return strings.Join(parts, " ")
}

func parseErr(key string, kv ...string) error { return &ParseError{newMsg(key, kv...)} }

// Action is one preflop voluntary action.
type Action struct {
	Player   string
	Kind     string   // fold | check | call | raise
	ToAmount *big.Rat // raise-to size, raises only
	AllIn    bool
}

// Hand is the preflop part of one hand.
type Hand struct {
	HandID    string
	Date      string // YYYY-MM-DD
	BB        *big.Rat
	Positions map[string]string // player name -> position
	Actions   []Action          // preflop voluntary actions in order (blinds excluded)
}

func parseAmount(text string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(strings.ReplaceAll(text, ",", ""))
	if !ok {
		return nil, parseErr(MsgParseBadAmount, "text", pyRepr(text))
	}
	return r, nil
}

// SplitHands splits a file's text into hand blocks.
func SplitHands(text string) []string {
	text = strings.TrimLeft(strings.ReplaceAll(text, "\r\n", "\n"), "\xEF\xBB\xBF")
	var starts []int
	for i := 0; i < len(text); i++ {
		if (i == 0 || text[i-1] == '\n') && strings.HasPrefix(text[i:], handStart) {
			starts = append(starts, i)
		}
	}
	var pieces []string
	prev := 0
	for _, s := range starts {
		pieces = append(pieces, text[prev:s])
		prev = s
	}
	pieces = append(pieces, text[prev:])
	var out []string
	for _, p := range pieces {
		if b := pyStrip(p); b != "" {
			out = append(out, b)
		}
	}
	return out
}

func parseAction(line string) (Action, error) {
	if m := reFoldCheck.FindStringSubmatch(line); m != nil {
		kind := "check"
		if m[2] == "folds" {
			kind = "fold"
		}
		return Action{Player: m[1], Kind: kind}, nil
	}
	if m := reCall.FindStringSubmatch(line); m != nil {
		return Action{Player: m[1], Kind: "call", AllIn: m[3] != ""}, nil
	}
	if m := reRaise.FindStringSubmatch(line); m != nil {
		to, err := parseAmount(m[3])
		if err != nil {
			return Action{}, err
		}
		return Action{Player: m[1], Kind: "raise", ToAmount: to, AllIn: m[4] != ""}, nil
	}
	return Action{}, parseErr(MsgParseUnrecognisedPreflop, "line", pyRepr(line))
}

func indexOfLine(lines []string, want string) int {
	for i, l := range lines {
		if l == want {
			return i
		}
	}
	return -1
}

// handLines splits a block into right-stripped lines.
func handLines(block string) []string {
	raw := strings.Split(pyStrip(strings.ReplaceAll(block, "\r\n", "\n")), "\n")
	lines := make([]string, len(raw))
	for i, l := range raw {
		lines[i] = pyRStrip(l)
	}
	return lines
}

// parseHeader checks the two header lines and the section markers; it returns the header and button matches
// and the index of the HOLE CARDS line.
func parseHeader(lines []string) (header, table []string, hole int, err error) {
	header = reHeader.FindStringSubmatch(lines[0])
	if len(lines) > 1 {
		table = reTable.FindStringSubmatch(lines[1])
	}
	if header == nil || table == nil {
		return nil, nil, 0, parseErr(MsgParseBadHeader, "line", pyRepr(lines[0]))
	}
	hole, summ := indexOfLine(lines, holeCards), indexOfLine(lines, summary)
	if hole < 0 || summ < 0 {
		return nil, nil, 0, parseErr(MsgParseMissingSections)
	}
	return header, table, hole, nil
}

// parseSeats reads the lines between the header and the deal: seats, posted blinds, ignorable lines.
func parseSeats(lines []string) ([]Seat, map[string]string, error) {
	var seats []Seat
	posted := map[string]string{}
	for _, line := range lines {
		if m := reSeat.FindStringSubmatch(line); m != nil {
			num, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, nil, parseErr(MsgParseBadSeat, "seat", m[1])
			}
			seats = append(seats, Seat{num, m[2]})
		} else if m := rePost.FindStringSubmatch(line); m != nil {
			posted[m[2]] = m[1]
		} else if !reIgnored.MatchString(line) {
			return nil, nil, parseErr(MsgParseUnrecognisedBeforeDeal, "line", pyRepr(line))
		}
	}
	return seats, posted, nil
}

// parseActions reads the preflop actions after the deal, up to the next street.
func parseActions(lines []string) ([]Action, error) {
	var actions []Action
	for _, line := range lines {
		if strings.HasPrefix(line, "*** ") {
			break
		}
		if pyStrip(line) != "" && !reIgnored.MatchString(line) {
			a, err := parseAction(line)
			if err != nil {
				return nil, err
			}
			actions = append(actions, a)
		}
	}
	return actions, nil
}

// validateTable checks the table against the seats and actions and assigns the positions.
func validateTable(format TableFormat, buttonText string, seats []Seat, actions []Action, posted map[string]string) (map[string]string, error) {
	names := map[string]bool{}
	for _, s := range seats {
		names[s.Name] = true
	}
	if len(names) != len(seats) {
		return nil, parseErr(MsgParseDuplicatePlayer)
	}
	if len(seats) != format.Size {
		return nil, TableSizeError{&ParseError{newMsg(MsgParseTableSize,
			"count", strconv.Itoa(len(seats)), "size", strconv.Itoa(format.Size))}}
	}
	button, err := strconv.Atoi(buttonText)
	if err != nil {
		return nil, parseErr(MsgParseBadSeat, "seat", buttonText)
	}
	if !seatTaken(seats, button) {
		return nil, parseErr(MsgParseButtonEmpty, "seat", buttonText)
	}
	if unseated := firstUnseated(actions, names); unseated != nil {
		return nil, parseErr(MsgParsePlayerNotSeated, "player", pyRepr(*unseated))
	}
	positions, _ := format.AssignPositions(seats, button)
	if positions[posted["small"]] != format.FromButton[0] || positions[posted["big"]] != format.FromButton[1] {
		return nil, parseErr(MsgParseBlindsMismatch)
	}
	return positions, nil
}

// ParseHand parses the preflop part of one hand block.
func ParseHand(block string) (Hand, error) {
	lines := handLines(block)
	header, table, hole, err := parseHeader(lines)
	if err != nil {
		return Hand{}, err
	}
	seats, posted, err := parseSeats(lines[2:hole])
	if err != nil {
		return Hand{}, err
	}
	actions, err := parseActions(lines[hole+1:])
	if err != nil {
		return Hand{}, err
	}
	positions, err := validateTable(DefaultFormat(), table[1], seats, actions, posted)
	if err != nil {
		return Hand{}, err
	}
	bigBlind, err := parseAmount(header[3])
	if err != nil {
		return Hand{}, err
	}
	return Hand{HandID: header[1], Date: header[4] + "-" + header[5] + "-" + header[6],
		BB: bigBlind, Positions: positions, Actions: actions}, nil
}

func seatTaken(seats []Seat, num int) bool {
	for _, s := range seats {
		if s.Num == num {
			return true
		}
	}
	return false
}

// firstUnseated is min(unseated) of the Python original: the smallest action player not at the table.
func firstUnseated(actions []Action, names map[string]bool) *string {
	var min *string
	for i := range actions {
		p := actions[i].Player
		if !names[p] && (min == nil || p < *min) {
			min = &actions[i].Player
		}
	}
	return min
}

// ErrorSample is one example of a hand or file that could not be used.
type ErrorSample struct {
	File string
	Msg  Message
}

// LoadResult is what LoadHands found.
type LoadResult struct {
	Hands           []Hand
	Files           int
	Duplicates      int
	Unparseable     int
	WrongTableSize  int
	UnreadableFiles int
	ErrorSamples    []ErrorSample
}

func (r *LoadResult) noteError(file string, msg Message) {
	if len(r.ErrorSamples) < maxSamples {
		r.ErrorSamples = append(r.ErrorSamples, ErrorSample{file, msg})
	}
}

// LoadHands recursively reads every .txt under the root of fsys, skips duplicates by hand id
// and counts what cannot be used. It stops with ctx.Err() when ctx is cancelled.
func LoadHands(ctx context.Context, fsys fs.FS) (*LoadResult, error) {
	result := &LoadResult{}
	seen := map[string]bool{}
	paths, err := txtFiles(ctx, fsys)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result.Files++
		text, err := readHandFile(fsys, path)
		if err != nil {
			result.UnreadableFiles++
			result.noteError(baseName(path), newMsg(MsgLoadFileUnreadable, "reason", err.Error()))
			continue
		}
		result.addBlocks(baseName(path), text, seen)
	}
	return result, nil
}

// addBlocks parses the hands of one file's text into the result.
func (r *LoadResult) addBlocks(name, text string, seen map[string]bool) {
	for _, block := range SplitHands(text) {
		if m := reHandID.FindStringSubmatch(block); m != nil {
			if seen[m[1]] {
				r.Duplicates++
				continue
			}
			seen[m[1]] = true
		}
		hand, err := ParseHand(block)
		var sizeErr TableSizeError
		var parseErr *ParseError
		switch {
		case errors.As(err, &sizeErr):
			r.WrongTableSize++
		case errors.As(err, &parseErr):
			r.Unparseable++
			r.noteError(name, parseErr.Msg)
		default:
			r.Hands = append(r.Hands, hand)
		}
	}
}

func newRat() *big.Rat { return new(big.Rat) }
