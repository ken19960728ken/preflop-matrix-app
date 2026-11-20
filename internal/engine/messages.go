package engine

// Message is a warning or error the engine reports without any translated text.
// Args hold values already formatted the way the Python original printed them
// (numbers with %.4f, strings through pyRepr), so a translation only substitutes {name}.
type Message struct {
	Key  string            `json:"key"`
	Args map[string]string `json:"args"`
}

func newMsg(key string, kv ...string) Message {
	args := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		args[kv[i]] = kv[i+1]
	}
	return Message{Key: key, Args: args}
}

// Message keys. Each comment gives the argument names and the original Python text.
// The matrix messages are zh-Hant in the Python original; the parse and range messages
// were English there, and the engine.range.invalid_utf8 / engine.load.file_unreadable
// reasons were Python exception texts (not reproduced; parity does not compare them).
const (
	// {text}  "bad amount: {text}"
	MsgParseBadAmount = "engine.parse.bad_amount"
	// {line}  "unsupported game or bad header: {line}"
	MsgParseBadHeader = "engine.parse.bad_header"
	// (none)  "missing HOLE CARDS or SUMMARY section"
	MsgParseMissingSections = "engine.parse.missing_sections"
	// {line}  "unrecognised line before the deal: {line}"
	MsgParseUnrecognisedBeforeDeal = "engine.parse.unrecognised_before_deal"
	// {line}  "unrecognised preflop line: {line}"
	MsgParseUnrecognisedPreflop = "engine.parse.unrecognised_preflop"
	// (none)  "duplicate player name"
	MsgParseDuplicatePlayer = "engine.parse.duplicate_player"
	// {count, size}  "{count} players (only {size}-handed hands are used)"
	MsgParseTableSize = "engine.parse.table_size"
	// {seat}  "button seat {seat} is empty"
	MsgParseButtonEmpty = "engine.parse.button_empty"
	// {player}  "player not seated: {player}"
	MsgParsePlayerNotSeated = "engine.parse.player_not_seated"
	// (none)  "positions from the button disagree with the players who posted the blinds"
	MsgParseBlindsMismatch = "engine.parse.blinds_mismatch"
	// {seat}  new in Go: a seat number too large for an int (Python accepted any integer)
	MsgParseBadSeat = "engine.parse.bad_seat"
	// {reason}  Python: the OSError text; Go: the I/O error text
	MsgLoadFileUnreadable = "engine.load.file_unreadable"

	// {card}  "bad card: {card}"
	MsgRangeBadCard = "engine.range.bad_card"
	// {cards}  "the same card twice: {cards}"
	MsgRangeSameCard = "engine.range.same_card"
	// {token}  "bad combo: {token}"
	MsgRangeBadCombo = "engine.range.bad_combo"
	// {cards}  "duplicate combo: {cards}"
	MsgRangeDuplicateCombo = "engine.range.duplicate_combo"
	// {token}  "bad weight: {token}"
	MsgRangeBadWeight = "engine.range.bad_weight"
	// {value}  "could not convert string to float: {value}"
	MsgRangeNotAFloat = "engine.range.not_a_float"
	// (none)  Python: the UnicodeDecodeError text of the strict utf-8-sig decode
	MsgRangeInvalidUTF8 = "engine.range.invalid_utf8"

	// {node, cell, action}  "{node} {cell} 缺 {action}，已當 0"
	MsgMatrixMissingAction = "engine.matrix.missing_action"
	// {node, cell, total}  "{node} {cell} 理論的跟注 + 加注 = {total} 超過 1，已按比例縮到 1"  (total is %.4f)
	MsgMatrixTheoryOverOne = "engine.matrix.theory_over_one"
)
