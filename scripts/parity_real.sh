#!/bin/sh
# 用真實範圍（唯讀）+ 手牌比對 Python 與 Go：--json 輸出零容差，另外逐行比 stdout 分類摘要。
#   scripts/parity_real.sh <hands_dir> [ranges_dir]
# ranges_dir 預設 ~/.config/preflop-matrix/ranges。輸出放在 ${TMPDIR}，結束時刪除；KEEP=1 則保留並印出路徑。
# 不要把輸出 commit。
set -eu

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
	echo "usage: $0 <hands_dir> [ranges_dir]" >&2
	exit 2
fi
abs() { (cd "$1" 2>/dev/null && pwd) || { echo "not a directory: $1" >&2; exit 2; }; }
HANDS=$(abs "$1")
RANGES=$(abs "${2:-$HOME/.config/preflop-matrix/ranges}")
PY_REPO=${PY_REPO:-$HOME/Developer/PythonCode/preflop_matrix}
ROOT=$(cd "$(dirname "$0")/.." && pwd)

OUT=$(mktemp -d "${TMPDIR:-/tmp}/preflop-matrix-parity.XXXXXX")
if [ "${KEEP:-}" = 1 ]; then
	echo "output dir (kept): $OUT"
else
	trap 'rm -rf "$OUT"' EXIT
fi

echo "== Python =="
uv run --frozen --project "$PY_REPO" preflop-matrix --hands "$HANDS" --ranges "$RANGES" \
	-o "$OUT/py.html" --json "$OUT/py.json" >"$OUT/py.stdout.txt" 2>"$OUT/py.stderr.txt" ||
	{ echo "Python failed:" >&2; cat "$OUT/py.stderr.txt" >&2; exit 1; }

echo "== Go =="
go -C "$ROOT" run ./cmd/preflop-matrix --hands "$HANDS" --ranges "$RANGES" \
	-o "$OUT/go.html" --json "$OUT/go.json" --lang zh-Hant >"$OUT/go.stdout.txt" 2>"$OUT/go.stderr.txt" ||
	{ echo "Go failed:" >&2; cat "$OUT/go.stderr.txt" >&2; exit 1; }

echo "== compare json (float64 exact) =="
status=0
go -C "$ROOT" run ./tools/paritydiff-cli "$OUT/py.json" "$OUT/go.json" || status=1
echo "== compare stdout classification (zh-Hant, reasons stripped) =="
go -C "$ROOT" run ./tools/paritydiff-cli --summary "$OUT/py.stdout.txt" "$OUT/go.stdout.txt" || status=1
exit $status
