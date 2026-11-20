"""Regenerates cli_zh_hant_golden.json from the Python original's real output.

Run: uv run --project ~/Developer/PythonCode/preflop_matrix python internal/i18n/testdata/gen_cli_golden.py
Temp directories are written as <TMP> in both args and want.
Each entry is {key, args, want}: T("zh-Hant", key, args) in Go must equal want, character for character.
"""
import contextlib
import io
import json
import os
import sys
import tempfile
from pathlib import Path

from preflop_matrix import cli
from preflop_matrix.config import ENV_VAR, resolve_ranges_dir
from preflop_matrix.parser import LoadResult
from preflop_matrix.ranges import RangeBook
from preflop_matrix.render import DEFAULT_TITLE

OUT = Path(__file__).with_name("cli_zh_hant_golden.json")
entries = []
TMP_PATHS = []


def scrub(text):
    for tmp in TMP_PATHS:
        text = text.replace(tmp, "<TMP>")
    return text


def add(key, args, want):
    entries.append({"key": key, "args": {k: scrub(v) for k, v in args.items()}, "want": scrub(want)})


def run(argv):
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        code = cli.main(argv)
    return code, out.getvalue().splitlines(), err.getvalue().splitlines()


def strip_prefix(line, prefix):
    assert line.startswith(prefix), (line, prefix)
    return line[len(prefix):]


os.environ["HOME"] = "/home/x"
os.environ.pop("XDG_CONFIG_HOME", None)
os.environ.pop(ENV_VAR, None)
default_dir = "/home/x/.config/preflop-matrix/ranges"

# argparse text
p = cli._parser()
add("cli.description", {}, p.description)
helps = {a.dest: a.help for a in p._actions}
add("cli.help.hands", {}, helps["hands"])
add("cli.help.ranges", {"env": ENV_VAR, "default": default_dir}, helps["ranges"])
add("cli.help.out", {}, helps["out"])
add("cli.help.hero", {}, helps["hero"])
add("cli.help.title", {"default": DEFAULT_TITLE}, helps["title"])
add("cli.help.json", {}, helps["json_path"])

# origins
add("cli.origin.flag", {}, resolve_ranges_dir("/x").origin)
os.environ[ENV_VAR] = "/nonexistent/envr"
add("cli.origin.env", {"env": ENV_VAR}, resolve_ranges_dir(None).origin)
os.environ.pop(ENV_VAR)
add("cli.origin.default", {}, resolve_ranges_dir(None).origin)

# summaries
r = LoadResult(hands=[object()] * 7, files=12, duplicates=3, not_six_max=4, unparseable=2, unreadable_files=1,
               error_samples=["a.txt: bad line"])
lines = cli._hands_summary(r)
add("cli.hands_summary", {"files": "12", "hands": "7", "duplicates": "3", "not_six_max": "4", "unparseable": "2", "unreadable": "1"}, lines[0])
add("cli.hands_error_sample", {"sample": "a.txt: bad line"}, lines[1])

book = RangeBook(ranges={i: None for i in range(6)}, files=10, ignored=["i.txt"], duplicates=["d.txt"],
                 unreadable=["u.txt: err"], unfilled=["f.txt"])
cli.missing_actions = lambda b: ["MISSING-X"]
lines = cli._ranges_summary(book)
add("cli.ranges_summary", {"files": "10", "nodes": "6", "ignored": "1", "duplicates": "1", "unreadable": "1", "unfilled": "1"}, lines[0])
add("cli.ranges_unreadable", {"item": "u.txt: err"}, lines[1])
add("cli.ranges_unfilled", {"item": "f.txt"}, lines[2])
add("cli.ranges_incomplete", {"item": "MISSING-X"}, lines[3])
add("cli.ranges_duplicate", {"item": "d.txt"}, lines[4])

# error / warning / progress lines from the real main()
with tempfile.TemporaryDirectory() as tmp:
    TMP_PATHS.extend(sorted({os.path.realpath(tmp), tmp}, key=len, reverse=True))
    hands, ranges = Path(tmp, "hands"), Path(tmp, "ranges")
    hands.mkdir()
    ranges.mkdir()
    hands_s, ranges_s = str(hands), str(ranges)

    def stderr_message(argv, index=0):
        _, _, err = run(argv)
        line = err[index]
        add("cli.error_prefix", {"message": strip_prefix(line, "錯誤：")}, line)
        return strip_prefix(line, "錯誤：")

    msg = stderr_message(["--hands", "/nonexistent/hands", "--ranges", ranges_s])
    add("cli.error.hands_dir", {"path": "/nonexistent/hands"}, msg)

    for label, origin_key, origin_args, argv, env in (
        ("flag", "cli.origin.flag", {}, ["--ranges", "/nonexistent/r"], None),
        ("env", "cli.origin.env", {"env": ENV_VAR}, [], "/nonexistent/envr"),
        ("default", "cli.origin.default", {}, [], None),
    ):
        if env:
            os.environ[ENV_VAR] = env
        _, _, err = run(["--hands", hands_s] + argv)
        os.environ.pop(ENV_VAR, None)
        msg = strip_prefix(err[0], "錯誤：")
        path = {"flag": "/nonexistent/r", "env": "/nonexistent/envr", "default": default_dir}[label]
        origin = next(e["want"] for e in entries if e["key"] == origin_key)
        base = f"理論範圍資料夾不存在或不是資料夾（來源：{origin}）：{path}"
        assert msg.startswith(base), msg
        hint = msg[len(base):]
        add("cli.error_prefix", {"message": msg}, err[0])
        add("cli.error.ranges_dir", {"origin": origin, "path": path, "hint": hint}, msg)
        if label == "env":
            add("cli.hint.env", {"env": ENV_VAR}, hint)
        if label == "default":
            add("cli.hint.default", {"env": ENV_VAR, "default": default_dir}, hint)

    _, out, err = run(["--hands", hands_s, "--ranges", ranges_s, "--json", "same", "--out", "same"])
    add("cli.ranges_source", {"origin": "參數 --ranges", "path": ranges_s}, out[0])
    add("cli.error.same_output", {}, strip_prefix(err[0], "錯誤："))

    _, _, err = run(["--hands", hands_s, "--ranges", ranges_s])
    add("cli.error.no_hands", {"dir": hands_s}, strip_prefix(err[0], "錯誤："))

    # paths that need real hands are driven through stubs
    cli.load_hands = lambda d: LoadResult(hands=[object()], files=1)
    cli.load_ranges = lambda d: RangeBook(files=0)
    _, _, err = run(["--hands", hands_s, "--ranges", ranges_s])
    add("cli.error.no_ranges_files", {"dir": ranges_s}, strip_prefix(err[0], "錯誤："))

    cli.load_ranges = lambda d: RangeBook(files=1)
    cli.build_matrix = lambda h, b, hero: {"meta": {"warnings": ["w1"], "hero_hands": 0}}
    cli.render_html = lambda data, title: "<html>"
    blocker = Path(tmp, "file.txt")
    blocker.write_text("x")
    bad_out = str(blocker / "sub" / "x.html")
    code, _, err = run(["--hands", hands_s, "--ranges", ranges_s, "--hero", "Hero", "--out", bad_out])
    assert code == 1
    add("cli.warning_prefix", {"message": strip_prefix(err[0], "警告：")}, err[0])
    add("cli.warning.no_hero", {"hero": "'Hero'"}, strip_prefix(err[1], "警告："))
    add("cli.error.write_output", {"err": strip_prefix(err[2], "錯誤：無法寫入輸出：")}, strip_prefix(err[2], "錯誤："))

    good_out = str(Path(tmp, "o.html"))
    good_json = str(Path(tmp, "o.json"))
    _, out, _ = run(["--hands", hands_s, "--ranges", ranges_s, "--out", good_out])
    add("cli.done", {"out": good_out}, out[-1])
    _, out, _ = run(["--hands", hands_s, "--ranges", ranges_s, "--out", good_out, "--json", good_json])
    add("cli.done_with_json", {"out": good_out, "json": good_json}, out[-1])

text = json.dumps(entries, ensure_ascii=False, indent=1) + "\n"
for local in ("/var/folders", "/private/", "/Users/", "/tmp/"):
    assert local not in text, f"local path {local} leaked into the golden file"
OUT.write_text(text, encoding="utf-8")
print(f"wrote {len(entries)} entries", file=sys.stderr)
