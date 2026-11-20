#!/bin/sh
# Regenerate testdata/parity/<corpus>/golden{.json,.meta.json,.stdout.txt,.stderr.txt} with the Python original
# (the only allowed source of golden data).
#   tools/parity/make_golden.sh [PY_REPO]      (default ~/Developer/PythonCode/preflop_matrix)
# Runs from the repository root so that the recorded command uses relative paths and stays reproducible.
# Everything is written to a temp dir first and moved into place only after all corpora succeeded.
set -eu

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
PY_REPO=${1:-$HOME/Developer/PythonCode/preflop_matrix}
cd "$ROOT"
CORPORA="handcrafted random_s42 posix_extras"

if [ -n "$(git -C "$PY_REPO" status --porcelain --untracked-files=no)" ]; then
	echo "make_golden: $PY_REPO has uncommitted changes; golden must come from a clean commit" >&2
	exit 1
fi
COMMIT=$(git -C "$PY_REPO" rev-parse HEAD)
PYRUN="uv run --frozen --project $PY_REPO"
PYVER=$($PYRUN python -c 'import sys; print(sys.version)')
PLATFORM=$($PYRUN python -c 'import platform; print(platform.platform())')
UVVER=$(uv --version)
MANIFEST=$($PYRUN python tools/parity/make_corpus.py --py-repo "$PY_REPO" --manifest-sha256)

$PYRUN python tools/parity/make_corpus.py --py-repo "$PY_REPO" --check

TMP=$(mktemp -d "${TMPDIR:-/tmp}/make_golden.XXXXXX")
trap 'rm -rf "$TMP"' EXIT

# posix_extras contains placeholders (tab in a file name, symlinks); run it from a materialized copy at the same relative path.
mkdir -p "$TMP/work/testdata/parity"
$PYRUN python tools/parity/materialize_extras.py testdata/parity/posix_extras "$TMP/work/testdata/parity/posix_extras"

for corpus in $CORPORA; do
	dir=testdata/parity/$corpus
	mkdir -p "$TMP/$corpus"
	workdir=$ROOT
	[ "$corpus" = posix_extras ] && workdir=$TMP/work
	set +e
	(cd "$workdir" && $PYRUN preflop-matrix --hands "$dir/hands" --ranges "$dir/ranges" -o /dev/null --json "$TMP/$corpus/golden.json" \
		>"$TMP/$corpus/golden.stdout.txt" 2>"$TMP/$corpus/golden.stderr.txt")
	code=$?
	set -e
	if [ "$code" -ne 0 ]; then
		echo "make_golden: Python exited $code for $corpus" >&2
		cat "$TMP/$corpus/golden.stderr.txt" >&2
		exit 1
	fi
	# the CLI echoes the --json path it was given; record the repository-relative one instead of the temp path
	sed "s#$TMP/$corpus/golden.json#$dir/golden.json#" "$TMP/$corpus/golden.stdout.txt" >"$TMP/$corpus/golden.stdout.fixed"
	mv "$TMP/$corpus/golden.stdout.fixed" "$TMP/$corpus/golden.stdout.txt"
	cmd="uv run --frozen --project <python repo> preflop-matrix --hands $dir/hands --ranges $dir/ranges -o /dev/null --json $dir/golden.json"
	$PYRUN python -c '
import json, sys
corpus, version, commit, platform, uv, manifest, code, cmd = sys.argv[1:]
print(json.dumps({"corpus": corpus, "python_version": version, "python_repo_commit": commit, "platform": platform,
                  "uv_version": uv, "exit_code": int(code), "corpus_manifest_sha256": manifest, "command": cmd},
                 indent=1, ensure_ascii=False))
' "$corpus" "$PYVER" "$COMMIT" "$PLATFORM" "$UVVER" "$MANIFEST" "$code" "$cmd" >"$TMP/$corpus/golden.meta.json"
done

for corpus in $CORPORA; do
	for f in golden.json golden.meta.json golden.stdout.txt golden.stderr.txt; do
		mv "$TMP/$corpus/$f" "testdata/parity/$corpus/$f"
	done
	echo "$corpus: $(wc -c <"testdata/parity/$corpus/golden.json" | tr -d ' ') bytes"
done
