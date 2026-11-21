// Package cli is the command line mode of preflop-matrix: report generation (behaviour, stdout/stderr text and
// exit codes 0/1/2 follow the Python cli.py when --lang zh-Hant), --check-ranges and --init-ranges.
//
// Everything printed goes through internal/i18n; the zh-Hant text is checked against the Python original.
// Reading and writing use the path strings as given (only a leading ~ is expanded), so ".." is resolved by the
// operating system after symbolic links, like Python's Path. The "output" line prints the Python Path form.
//
// # Acceptable differences from Python argparse
//
//   - No abbreviations: argparse accepts --ran for --ranges, this parser only accepts full names.
//   - All arguments are scanned before anything happens, so an argument error always wins over -h/--help:
//     "-h --bogus" prints help and exits 0 in argparse (it stops at -h), here it reports the error and exits 2.
//   - Values after a space that start with "-" (and are not negative numbers) are rejected as a missing value,
//     as in argparse; use --x=value for such values. Combined short flags (-ho) are not supported.
//   - --top accepts what strconv.Atoi accepts after trimming spaces (Python int() also accepts "1_0" and
//     non-ASCII digits).
//   - Messages for argument errors are the Go ones (cli.error.*), not argparse's usage text.
//   - ~user is not expanded (see internal/config).
//   - A dangling symbolic link as -o or --json is not followed by the same-file check.
//   - --json is written with Python's indent (one space) and without HTML escaping of < > &; U+2028 and
//     U+2029 are written as \u2028 and \u2029 (Python writes the raw characters). Values are equal.
//   - The -o and --json paths in the "output" line are not made absolute.
//
// # Known limits
//
//   - A file name with ANSI control characters (ESC sequences) is printed as is in the range and hand
//     summaries; nothing strips or escapes it, so a terminal may interpret it. Python prints them the same way.
package cli
