# Theory ranges: requirements and format

[繁體中文](RANGES.md)｜[简体中文](RANGES.zh-Hans.md)

Everything in PreflopMatrix's "theory" column comes from one ranges folder. This document explains where the program reads ranges from, which files it needs, the format of each file, and how to check them.

Ranges are not distributed with the program and are not built into the app: prepare them yourself (compute them, or use a source you are allowed to use). `.gitignore` excludes `*.txt` (except test data under `testdata/parity/` and `internal/**/testdata/`) as well as `ranges/` and `Ranges/` at the repository root; the pre-commit hook rejects any staged file that matches `.gitignore` and any `.txt` regardless of case (except in those two test-data locations), so range files cannot be committed by accident.

## 1. Where ranges are read from

### The app (GUI)

Only the "theory ranges folder" you choose in the window is used (the app remembers it). There is no environment variable and no default folder.

- Not chosen: the report is still generated, the theory column shows `–` everywhere, and the top of the report says that no theory ranges were chosen.
- Chosen, but the folder no longer exists: an error is shown and no report is generated; you can choose again or clear it.
- Chosen, but the folder contains no `.txt` at all: an error is shown and no report is generated (the same as exit code 1 in the CLI).

### The command line (CLI)

| Order | Source | Example |
|---|---|---|
| 1 | the `--ranges DIR` option | `preflop-matrix --hands hh --ranges ~/my-ranges` |
| 2 | the `PREFLOP_MATRIX_RANGES` environment variable | `export PREFLOP_MATRIX_RANGES=~/my-ranges` |
| 3 | the default folder | macOS / Linux: `$XDG_CONFIG_HOME/preflop-matrix/ranges`, or `~/.config/preflop-matrix/ranges` when it is unset or not an absolute path; Windows: `%AppData%\preflop-matrix\ranges` (`XDG_CONFIG_HOME` is ignored; the Windows path logic is kept, but Windows is not officially supported or tested yet) |

- The first line of every run prints which one was used, for example `Theory ranges source: default folder -> /Users/…/.config/preflop-matrix/ranges`.
- The **first source that is set** wins; if its folder does not exist the program stops with exit code `2` and does not look further (no fall-through). `--ranges ""` counts as set and means the current folder `.`; an empty environment variable counts as not set.
- Error messages give hints based on the source: for a missing default folder they list the three ways to set one; for a wrong environment variable they suggest fixing it or running `unset PREFLOP_MATRIX_RANGES`.
- Only a leading `~` (`~/…`) is expanded; `~user` is not.

### Setting up the default folder (CLI)

```bash
mkdir -p ~/.config/preflop-matrix/ranges
cp -R "<your ranges folder>"/. ~/.config/preflop-matrix/ranges/
```

### How the folder is scanned

All `.txt` files are scanned recursively (the extension must be lower case; `.TXT` is not read). Sub-folders can be used for organizing, but symbolic links to folders are not followed (symbolic links to files are read). A folder named `X.txt` whose name matches a node is listed as "unreadable".

## 2. Which files are needed

Only heads-up nodes are used. Position order: UTG → HJ → CO → BTN → SB → BB; the opener always acts before the opponent.

| Node | Decider | Files needed (per cell) | Cells | Required action slots |
|---|---|---|---|---|
| `open` | opener | opening range | 5 (UTG, HJ, CO, BTN, SB) | 5 |
| `vs_open` | defender | call, 3bet | 15 (each opener × each later position) | 30 |
| `vs_3bet` | opener | call, 4bet (either `4bet` or `4bet all in`) | 15 | 30 |
| `vs_4bet` | 3bettor | call, 5bet (either `5bet` or `5bet all in`) | 15 | 30 |

### 95 required action slots vs the 108-file standard set

- **95 required action slots** = 5 + 30 + 30 + 30. A range set is "complete" when all 95 slots have a file and the base range is present too (`vs_3bet` needs the opening file, `vs_4bet` needs that 3bettor's 3bet file). The 4bet of `vs_3bet` and the 5bet of `vs_4bet` can each be written two ways (see below); either counts as filled.
- **The standard set of 108** is the number of file names created by `--init-ranges` / "Create range filename template", and is only a reference: each of the 15 `vs_4bet` cells has `call` and `5bet all in`, and 13 of them also have `5bet`, so 5 + 30 + 30 + 43 = 108.

### 5bet and 5bet all in replace each other

Filling one of them is enough and the other blank file can be deleted; if both are filled they are added together (capped at 1). In the cells `BB vs SB 4bet` and `BTN vs CO 4bet` the template only creates `5bet all in`: in the Rush & Cash 6-max setup the standard set is based on, the 5bet in these two spots is always all in (there is no smaller size), so no plain 5bet file is offered. The check still accepts either way of writing it. The `4bet` and `4bet all in` files of `vs_3bet` follow the same adding rule.

### Behavior when files are missing

| Situation | Result |
|---|---|
| The base range is missing (`vs_3bet` without the opening file, `vs_4bet` without the 3bet file) | the cell shows `–` (no theory) |
| The base range exists but one action file is missing | the missing action counts as 0, and the summary prints "theory ranges incomplete: … is missing …, counted as 0" |
| The whole cell is missing | the cell shows `–`, no error |
| `vs_5bet` | never has theory (such files are ignored) |

## 3. File name rules

The file name (without `.txt`) decides which node a file belongs to. Matching is case-insensitive; the size token (`2bb`, `9.5bb`) may be left out (`BTN open` works too); several spaces count as one.

| Node | File name format | Examples |
|---|---|---|
| `open` | `<opener> open [size]` | `BTN Open 2bb`, `SB open` |
| `vs_open` | `<defender> vs <opener> open [size] call` | `BB vs BTN open 2bb call`, `BB vs UTG open call` |
| | `<defender> vs <opener> open [size] 3bet [size]` (`3b` also works) | `BB vs BTN open 2bb 3bet 9.5bb`, `BB vs HJ open 2bb 3b 10bb` |
| `vs_3bet` | `<opener> vs <3bettor> 3bet [size] call` | `BTN vs BB 3bet 9.5bb call` |
| | `<opener> vs <3bettor> 3bet [size] 4bet [size]` | `BTN vs BB 3bet 9.5bb 4bet 22bb` |
| | `… 4bet all in` (added to the 4bet file, capped at 1) | `SB vs BB 3bet 9bb 4bet all in` |
| `vs_4bet` | `<3bettor> vs <opener> 4bet [size] call` | `BB vs BTN 4bet 22bb call` |
| | `<3bettor> vs <opener> 4bet [size] 5bet [size]` | `BB vs CO 4bet 23bb 5bet 48.5bb` |
| | `… 5bet all in` (added to the 5bet file, capped at 1) | `BB vs BTN 4bet 22bb 5bet all in` |

Note that the second position means different things: in `vs_open` and `vs_4bet` it is the "opponent", in `vs_3bet` it is the "3bettor"; the first position is always the player who makes the decision.

**Files that are ignored** (listed as "ignored" in the summary):
- Names with a third position or multiway action: `BB vs BTN open 2bb SB call call`, `… squeeze …`, `CO vs HJ 3bet UTG …`
- Nodes after a 5bet/6bet: `BTN vs BB 5bet 46bb all in`, `… 6bet all in call`
- Any other file that does not fit the formats

When several files map to the same node, they are sorted by full path **comparing one path component at a time** (case-sensitive, by Unicode code point; for example `a/` sorts before `a b/`), and the **first one that reads successfully and is not blank** wins. Other files with content are listed as "duplicates" (this includes unreadable ones); blank files are always listed as "unfilled" (see section 4).

### File name template

If you would rather not type 108 file names, use the app's "Create range filename template…" or the CLI's `--init-ranges DIR` to create blank files in a folder:

```
UTG open.txt
BB vs UTG open call.txt
BB vs UTG open 3bet.txt
UTG vs BB 3bet call.txt
UTG vs BB 3bet 4bet.txt
BB vs UTG 4bet call.txt
BB vs UTG 4bet 5bet.txt
BB vs UTG 4bet 5bet all in.txt
```

- Template names leave out the size; `BB vs SB 4bet` and `BTN vs CO 4bet` only have `5bet all in` (no `5bet`).
- Existing files are never overwritten, and the number of created and skipped files is reported; if the folder already has a file for the same node (for example one with a size in its name), no blank duplicate is created.
- A `HOW-TO-FILL.md` (instructions in three languages) is written as well, unless it already exists.
- Blank file = not filled in: after creating the template, fill in the files you want to use; an action you never play must be written as weight 0 (next section).

## 4. File content format

The combo-weight text format exported by common solvers, separated by commas or newlines:

```
AsAh: 1,AsAd: 1,KsKh: 0.5,AhKh,7c6c: 0.25
```

- Each token is `<two cards>[: weight]`. A card is a rank from `AKQJT98765432` plus a suit from `shdc`, for example `Ts9s`.
- A missing weight means 1. A weight must be a finite number that is 0 or more.
- The same combo (in either order) may not appear twice; the two cards of a combo must differ (different combos sharing a card is fine).
- The file must be UTF-8 and may start with one BOM.
- Breaking any of these makes the file "unreadable"; it does not stop the whole run.

### Blank file = not filled in, and how to write weight 0

- "Unfilled", "blank" and "empty" mean the same thing throughout (the app says "empty", the CLI says "blank"). A file that **maps to a node** and contains only whitespace after removing one BOM counts as **unfilled**: it does not occupy the node (a later file for the same node that has content wins), the summary and the range check list it as "unfilled (blank files)", and it is neither unreadable nor a duplicate.
- "Whitespace" is Python's `str.isspace` set: space, tab, newline, U+001C–U+001F, U+0085, U+00A0, U+3000 and so on. U+200B (zero-width space) does not count, and a file containing it is "unreadable"; two BOMs are also "unreadable".
- A blank file that does not map to a node (for example a multiway name) is still listed as "ignored".
- **If you never play an action, write weight 0 explicitly** instead of leaving the file blank: `AsAh: 0`, or just a single comma `,` (a file with only commas is not blank; it is a valid range with all weights 0).

The program merges combos into the 169 starting hands: the value of a hand is the sum of its weights ÷ its number of combos (pairs 6, suited 4, offsuit 12), capped at 1.

### Joint weights

| Node | What the weight means |
|---|---|
| `open`, `vs_open` | the share of **all deals** of this hand that take this action |
| `vs_3bet`, `vs_4bet` | the same, as a **joint weight**: it already includes the filtering of the previous step |

For example `AhKh: 0.3` in `BTN vs BB 3bet 9.5bb call` means that 30% of all AhKh go down the line "BTN opens → BB 3bets → BTN calls", not that 30% of the AhKh in the opening range call. This is usually the weight solvers export for a node.

Formula: each hand (at the level of the 169 hands, not per combo) is first capped by the base range, `min(file weight, base range weight)`; the conditional frequency of the whole cell is the average weighted by the base range:

```
conditional frequency = Σ min(w, base) × combos ÷ Σ base × combos
```

Hands whose base-range weight is 0 are not counted. The base range is the opening file for `vs_3bet` and, for `vs_4bet`, that 3bettor's 3bet file against that opener. `open` and `vs_open` have no base range and use `Σ w × combos ÷ 1326` directly.

If your files hold conditional weights (for example "30% of the opening hands call"), the theory values of `vs_3bet` / `vs_4bet` are overestimated. Multiply by the base range before saving.

## 5. Checking your ranges folder

### The app

The check runs automatically once a ranges folder is chosen (or press "Check again"). The "Range check" panel shows:

```
Required actions X/95; files N (standard set 108)
```

and below it, in groups: missing nodes (a whole cell missing, or the base range missing, listed with standard file names), missing action files (counted as 0), empty files (still missing / can be deleted), ignored, duplicates, and unreadable (with the reason).

### The command line

```bash
preflop-matrix --check-ranges --ranges <ranges folder>
```

It prints the same information. Exit codes: complete `0`, incomplete `1` (including a folder with no `.txt`, only blank files, or a gap caused by an unreadable file), folder missing or bad option `2`.

When generating a report, the stdout summary has this line as well:

```
Theory ranges: 108 file(s), mapped to 108 node(s); ignored (multiway or unrecognized) 0, duplicates 0, unreadable 0, unfilled (blank files) 0
```

- "mapped to" is below 108: check the file names against section 3.
- "unreadable" is not 0: the file names and reasons are listed (for example a bad card, a duplicate combo or an invalid weight).
- Only a folder with no `.txt` at all gives exit 1; all-blank or all-ignored folders still produce a report and exit 0.
- "unfilled (blank files)" is not 0: those files have no content yet; they do not count as a range and do not occupy the node.
- "Theory ranges incomplete": a node is missing an action file.
- "theory call + raise = … exceeds 1, scaled down proportionally to 1" (stderr): the call + raise of a cell is above 1. For `vs_3bet` / `vs_4bet` this usually means conditional and joint weights were mixed; `vs_open` has no base range, so it means the call file and the 3bet file add up to more than 1 of all deals (the two files overlap).
