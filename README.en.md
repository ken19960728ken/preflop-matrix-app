# PreflopMatrix

[繁體中文](README.md)｜[简体中文](README.zh-Hans.md)｜[English](README.en.md)

A desktop app for 6-max preflop position-vs-position frequency heatmaps (macOS, Go + [Wails v3.0.0](https://v3.wails.io)), ported from the Python tool [`preflop_matrix`](https://github.com/ken19960728ken/preflop_matrix). The interface and the reports come in three languages: Traditional Chinese, Simplified Chinese and English.

## 1. Status

Under development and not released yet. Only macOS (13 or later) is supported for now, and you build it yourself (see [Development](#8-development)). The command-line mode (CLI) keeps its Linux / Windows path logic, but those platforms are not officially supported or tested yet.

## 2. Overview

It reads GG Rush & Cash 6-max hand histories plus a folder of theory ranges and produces one preflop position-matrix report:

- For every position matchup (for example BB facing a CO open) it shows three frequencies: **theory**, **Hero (me)** and the **player pool** (everyone except Hero).
- Two views: conditional frequencies, and a heatmap in raw space, where each frequency is multiplied back through the earlier steps so it is a share of all 1326 starting hands.
- **Deviation summary**: lists the cells where the observed frequency is furthest from theory (see [section 4](#4-how-the-deviation-summary-is-computed)).
- **Range check**: tells you whether the ranges folder is complete, which files are missing and which cannot be read.
- **Range file name template**: creates blank files with the standard names in one click, ready for you to fill in.
- Three-language interface and reports (the report page can switch language too).

Nodes (each node is a decision between two players only; once a third player has taken a non-fold action, the decision is excluded):

| Node | Decider | Options |
|---|---|---|
| `open` | a position with only folds in front (an SB limp is not an open) | open / other |
| `vs_open` | a defender after an open, with no call or raise in between | fold / call / 3bet |
| `vs_3bet` | the opener facing a single 3bet | fold / call / 4bet |
| `vs_4bet` | the 3bettor facing the opener's 4bet | fold / call / 5bet |
| `vs_5bet` | the opener facing a 5bet | fold / call / 6bet (no theory) |

Limits: only the GG / Natural8 Rush & Cash format, only preflop, and only 6-max tables (hands with other table sizes are excluded and counted in the summary). Reports use Google Fonts and fall back to system fonts when offline.

## 3. Using the app

1. **Hand history folder**: press "Choose…" and pick the folder of GG exports (`.txt` files are scanned recursively; each hand id counts once).
2. **Theory ranges folder** (optional): pick the ranges folder you prepared. Without one, a report is still generated, the theory column shows `–` everywhere, and the top of the report says that no theory ranges were chosen. If you chose a folder that no longer exists, an error is shown and you can choose again or clear it. Requirements and format of the ranges: [docs/RANGES.en.md](docs/RANGES.en.md).
3. **Hero name**: your player name in the hand files (default `Hero`); it is saved when you press Enter or leave the field.
4. Press "**Generate report**". The buttons are disabled while it runs; when it finishes, the summary and warnings are shown and the report fills the main area.
5. **Range check**: open the panel to see "Required actions X/95; files N (standard set 108)" together with missing nodes, missing action files, empty files, ignored files, duplicates and unreadable files. It runs automatically once a ranges folder is chosen.
6. **Create range filename template…**: choose a folder and the app creates blank files with the standard names in it (existing files are never overwritten, and a three-language `HOW-TO-FILL.md` is added). If you chose the current ranges folder, it is checked again automatically.
7. **Deviation summary**: two tables, "Player pool" and "Hero"; the number of rows can be set from 0 to 20 (default 5, 0 hides the tables); click a row and the report jumps to the matching cell.
8. **Save HTML** saves a single HTML file (in the current language, with the three-language switcher). **Print** opens a new window and brings up the print dialog (the language switcher is hidden when printing and the deviation summary is kept). **Open in browser** writes the report to `~/Library/Caches/preflop-matrix/report.html` (it is not cleaned up automatically) and opens it in your default browser.
9. **Language**: switch between Traditional Chinese, Simplified Chinese and English at the top. The first launch follows the system language; once you choose one, it is remembered.

Settings are stored in `~/Library/Application Support/preflop-matrix/settings.json` (language, hand histories folder, ranges folder, Hero, number of rows, last save location). A corrupt file is renamed and kept, and the defaults are used.

In the macOS folder picker, press `⌘⇧.` (Command + Shift + period) to show hidden folders such as `~/.config`.

## 4. How the deviation summary is computed

- Nodes considered: `open`, `vs_open`, `vs_3bet`, `vs_4bet` (`vs_5bet` has no theory and is never listed). A cell needs both a theory value and an observation, and a sample size of **n ≥ 30**.
- `open` looks at the open rate only; the other nodes look at fold, call and raise (theory fold = 1 − call − raise, limited to 0–1).
- Significance: z = (observed − θ) ÷ √(θc(1 − θc) ÷ n), and only actions with **|z| ≥ 3** are listed. θc is the theory value θ limited to 0.5%–99.5%, and it is used in the denominator only; the numerator still uses the original theory value. Rows where this adjustment applied are marked "theory adjusted".
- Ranking: by **Cohen's h** (|2·asin√observed − 2·asin√θ|, with the original θ), largest first; ties are broken by |z|, then |difference|, then n (larger first), and finally by node, cell and action (fold, call, raise) order.
- **Each cell is listed once, with the action that has the largest h.**
- The player pool and Hero each get the top N rows (default 5, range 0–20).
- Each row shows: the matchup (decider first, for example `BB vs CO`), node, action, observed %, theory %, difference (percentage points), n and z.

This only presents numbers; it is **not strategy advice**.

## 5. Theory range files

Theory ranges are **not distributed with the program** and are not built into the app; prepare them yourself (compute them, or use a source you are allowed to use). Which files are needed, the file name rules, the content format, what joint weights mean and how to check them are described in:

- [docs/RANGES.md](docs/RANGES.md) (Traditional Chinese)
- [docs/RANGES.zh-Hans.md](docs/RANGES.zh-Hans.md) (Simplified Chinese)
- [docs/RANGES.en.md](docs/RANGES.en.md) (English)

File name template: "Create range filename template…" in the app, or `--init-ranges DIR`, creates 108 blank files with the standard names, for example `UTG open`, `BB vs UTG open call`, `BB vs UTG open 3bet`, `UTG vs BB 3bet call`, `UTG vs BB 3bet 4bet`, `BB vs UTG 4bet call`, `BB vs UTG 4bet 5bet` and `BB vs UTG 4bet 5bet all in`.

## 6. Command line (CLI)

Two ways to run it:

```sh
go run ./cmd/preflop-matrix --hands <GG hand folder> --ranges <ranges folder> -o out.html
# or build it first
go build -o preflop-matrix ./cmd/preflop-matrix
./preflop-matrix --help
```

The app itself (`PreflopMatrix.app/Contents/MacOS/PreflopMatrix`) also runs in command-line mode when its first argument is one of the options below; otherwise it opens the window.

### The three modes

```text
preflop-matrix --hands DIR [--ranges DIR] [-o FILE] [--hero NAME] [--title TEXT] [--json PATH] [--lang LANG] [--top N]
preflop-matrix --check-ranges [--ranges DIR] [--lang LANG]
preflop-matrix --init-ranges DIR [--lang LANG]
```

| Option | Description |
|---|---|
| `--hands DIR` | Required when generating a report. Scans `.txt` recursively; each hand id counts once |
| `--ranges DIR` | Theory ranges folder (`.txt`) |
| `-o, --out FILE` | Output HTML, default `preflop_matrix.html` |
| `--hero NAME` | Hero's player name, default `Hero` |
| `--title TEXT` | Page title; the default follows the output language (`翻前位置矩陣`, `翻前位置矩阵` or `Preflop Position Matrix`) |
| `--json PATH` | Also save the matrix data as JSON (the same data that is embedded in the HTML; it may not point to the same file as `-o`) |
| `--lang LANG` | Output language: `zh-Hant`, `zh-Hans` or `en` (`zh-TW` and `zh-CN` are also accepted); defaults to the system language |
| `--top N` | Rows per group (player pool, Hero) in the deviation summary, 0 to 20, default 5, 0 hides it; a value outside the range exits with 2 |
| `--check-ranges` | Only check how complete the ranges folder is; no report |
| `--init-ranges DIR` | Create blank range file name templates in DIR without overwriting existing files; no report |
| `-h, --help` | Show the help |

`--check-ranges` and `--init-ranges` cannot be combined with each other or with the report options (exit code 2); `--init-ranges` cannot be combined with `--ranges` either.

### Exit codes

| Mode | `0` | `1` | `2` |
|---|---|---|---|
| Generate a report | success (a report is written as long as at least one hand is usable; hands that cannot be parsed are counted, skipped and listed in the summary) | no usable hands, no `.txt` in the ranges folder, or the output cannot be written | a folder does not exist, or bad options |
| `--check-ranges` | ranges are complete | incomplete (including no `.txt`, only blank files, or a gap caused by an unreadable file) | folder does not exist, or bad options |
| `--init-ranges` | success (also when everything was skipped) | writing failed | bad options, or the path is not a folder |

### Order of the ranges source

`--ranges` > the `PREFLOP_MATRIX_RANGES` environment variable > the default folder (macOS / Linux: `$XDG_CONFIG_HOME/preflop-matrix/ranges`, or `~/.config/preflop-matrix/ranges` when it is unset or not an absolute path). The first source that is set wins; if its folder does not exist the program stops and does not look further (no fall-through). `--ranges ""` means `.`. See [docs/RANGES.en.md](docs/RANGES.en.md).

When generating a report, stdout prints the hand and range summaries and then the deviation summary (not printed with `--top 0`).

### Relationship to the Python version

This is a port of the Python tool `preflop_matrix`, checked by tests:

- On the synthetic corpora in `testdata/parity/` (synthetic hands and synthetic ranges), the matrix data in `--json` equals the Python version's value by value (zero tolerance).
- With `--lang zh-Hant --top 0`, the classification summary on stdout is the same as the Python version's (except the path in the "output" line and the reasons for unreadable files), and stderr is identical character for character.
- The differences listed below can change the result for extreme inputs; ordinary data is not affected.

The Go version adds things the Python version does not have: the three-language interface, the deviation summary, the range check, the file name template and the desktop app.

Acceptable differences (the full list is in [`internal/cli/doc.go`](internal/cli/doc.go) and [`internal/engine/doc.go`](internal/engine/doc.go)):

- No argparse-style option abbreviations (`--ran`); an argument error always wins over `-h`; the error messages are the Go version's own text.
- `~user` is not expanded (only a leading `~` is).
- Format details of `--json` (indentation, no HTML escaping, U+2028 / U+2029 written as escape sequences); the values are the same.
- Unicode version differences: `\d` matches ASCII digits only; the tables for whitespace and printable characters can differ as well.
- The `-o` and `--json` paths in the "output" line are not made absolute.
- Amounts are computed as exact rationals, not as Decimal with 28 significant digits.
- The reason text for an unreadable file is Go's error text.
- On Windows, file ordering and case matching can differ.

## 7. Opening an unsigned app on macOS for the first time

If you build the app yourself or get it from someone else, it is not notarized by Apple, so macOS may block it the first time you open it (an app you built on your own machine usually has no quarantine flag). To allow it:

- **macOS 13–14**: in Finder, right-click the app (or Control-click) → "Open" → "Open" again.
- **macOS 15 or later**: try to open it once, then go to "System Settings → Privacy & Security", and in the "Security" section click "Open Anyway" (the button is available for about an hour after you try to open the app), then enter your login password.
- Or remove the quarantine flag in Terminal:

```sh
xattr -dr com.apple.quarantine /path/to/PreflopMatrix.app
```

See Apple's instructions: [Open a Mac app from an unidentified developer](https://support.apple.com/guide/mac-help/mh40616/mac).

## 8. Development

Requirements: macOS 13+, Go ≥ 1.26.10, Node (installed with nvm; the version must satisfy `^20.19.0 || >=22.12.0`, the project's `.nvmrc` says 22 so `nvm use` is enough; `npm ci` and `npm run build` were verified with v24), and `wails3` exactly v3.0.0.

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
export PATH="$PATH:$(go env GOPATH)/bin"   # wails3 is installed in ~/go/bin, which must be on PATH
git config core.hooksPath .githooks         # enable the pre-commit hook
cd frontend && npm ci                       # @wailsio/runtime, typescript and vite are pinned to exact versions
```

`.gitignore` excludes `*.txt` (except test data under `testdata/parity/` and `internal/**/testdata/`), `ranges/` and `Ranges/` at the repository root, `*.html`, `*.json` (except the few the project needs), `.env` and so on. The pre-commit hook rejects any staged file that matches `.gitignore` (including files forced in with `git add -f`) and any `.txt` regardless of case (except in those two test-data locations), so range files, hand histories and reports cannot be committed. `wails3 build` first checks that `wails3 version` is v3.0.0.

The contents of `frontend/dist/` are not under version control (only `.gitkeep` is), so a fresh clone can run `go vet ./...` and `go test ./...` without building the frontend first.

| Command | Description |
|---|---|
| `go test -short ./...` | Everyday tests (skips the tests that need Chrome; includes the value-by-value comparison with the golden data) |
| `task test:report` | Report tests including headless Chrome (Chrome required); run before committing |
| `npm run typecheck` (in `frontend/`) | TypeScript type check |
| `wails3 build` | Build, output `bin/PreflopMatrix` |
| `wails3 task darwin:package:universal` | Package a universal (arm64 + amd64) `bin/PreflopMatrix.app`, ad-hoc signed |
| `task dev` (that is, `wails3 dev -config ./build/config.yml -port 9245`) | Development mode |

`task` can be installed with `brew install go-task`, or use `wails3 task <name>` instead. After packaging the universal app, delete the universal `bin/PreflopMatrix` before running `wails3 build` again; otherwise `go build` refuses to overwrite that universal file and fails.

### Parity (comparison with the Python version)

```sh
tools/parity/make_golden.sh [PY_REPO]             # regenerate the golden data with the Python version (pass the Python repo as the argument; it must be a clean checkout)
scripts/parity_real.sh <hands folder> [ranges folder]   # compare Python and Go on real data, output is not kept; ranges default to ~/.config/preflop-matrix/ranges (read-only); set the Python repo with the PY_REPO environment variable
go run ./tools/paritydiff-cli [--summary] a b     # compare two JSON files (with --summary, two stdout summaries)
```

The corpora are generated by `tools/parity/make_corpus.py` and the golden data by `tools/parity/make_golden.sh`, which runs the Python version; everything is synthetic and contains no real hands or ranges.

### Development environment variable and mode

- `PMA_SETTINGS_PATH`: points the settings file somewhere else so your real settings are untouched. **It only works in builds without the `production` build tag** (for example `wails3 dev`); apps produced by `wails3 build` / `package` ignore it.
- Frontend-only development (no Go backend, with a mock report):

```sh
cd frontend && npm run dev:mock    # same as VITE_MOCK=1 vite, at http://127.0.0.1:9245
```

## 9. License

GPL-3.0-only. You may use, modify and distribute it freely; when you distribute this program (including modified versions), you must license it under GPL-3.0 and provide the corresponding source code. The full text is in [LICENSE](LICENSE).
