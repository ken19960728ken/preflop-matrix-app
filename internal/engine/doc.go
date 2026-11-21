// Package engine implements the range parsing, matrix computation and (later) deviation statistics of the
// preflop matrix. It does not import Wails and contains no translated text: warnings and errors are Message
// values (key plus pre-formatted arguments); see messages.go.
//
// The computation is a one-to-one port of the Python original (preflop_matrix 7354dff); the DATA payload is
// compared with the Python output with zero tolerance. Float code must keep the Python evaluation order and
// wrap every product that is added to something in float64(...), otherwise arm64 fuses it into a multiply-add
// and the last bit changes.
//
// Data must be serialised with Data.MarshalWith (Data.MarshalJSON refuses): meta.warnings is an array of
// strings made by the caller's formatter, as in the Python payload.
//
// # Acceptable differences from Python
//
//   - Unicode tables: the Nd digits (float() and range weights), unicode.IsPrint (pyRepr) and the white-space
//     set come from Go's Unicode tables, Python 3.12 uses Unicode 15.0. Characters added or reclassified since
//     can behave differently.
//   - The regular expressions use ASCII \d (Python's \d also matches other Unicode decimal digits).
//   - Seat and button numbers must fit an int; Python accepts any integer. Larger numbers give the new message
//     engine.parse.bad_seat.
//   - Amounts are exact (math/big.Rat). Python's Decimal rounds to 28 significant digits, so two amounts that
//     only differ beyond the 28th digit compare equal there and different here.
//   - The reasons for unreadable files are the Go error text (and engine.range.invalid_utf8 for bad UTF-8), not
//     the Python OSError / UnicodeDecodeError text. Hand files with bad bytes are decoded like
//     errors="replace" (one U+FFFD per maximal ill-formed subpart), which is identical.
//   - Folders are walked with io/fs: the .txt suffix is matched case-sensitively and ordering is by path
//     component (code point); on Windows, case-insensitive file systems can order and match differently.
//   - Data.Meta.Warnings holds Message values; only the formatted strings are compared with the Python text.
package engine
