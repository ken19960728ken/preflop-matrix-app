#!/usr/bin/env python3
"""Generate the synthetic parity corpora (stdlib only).

    uv run --project <python repo> python tools/parity/make_corpus.py --py-repo <python repo>
    ... --check      # regenerate in memory twice and compare with what is on disk

Everything is synthetic: hands come from the Python repo's tests/helpers.py, ranges from a seeded RNG.
Output is deterministic (fixed seeds, sorted writes) so that regenerating gives bit-identical files.
"""
import argparse
import datetime
import hashlib
import json
import os
import random
import re
import shutil
import sys
from pathlib import Path

BOM = "\ufeff"
POS = ("UTG", "HJ", "CO", "BTN", "SB", "BB")
PAIRS = [(a, b) for i, a in enumerate(POS[:-1]) for b in POS[i + 1:]]
RANKS = "AKQJT98765432"
CLASSES = [RANKS[i] * 2 if i == j else RANKS[min(i, j)] + RANKS[max(i, j)] + ("s" if i < j else "o")
           for i in range(13) for j in range(13)]
CORPUS_DIRS = {"handcrafted": ("hands", "ranges"), "random_s42": ("hands", "ranges"),
               "posix_extras": ("hands", "ranges", "ranges_decoy", "specials")}
Manifest = dict  # relative path -> bytes
H = None  # tests/helpers.py of the Python repo


def load_helpers(py_repo: Path) -> None:
    global H
    sys.path.insert(0, str(py_repo / "tests"))
    import helpers
    H = helpers


# ---------------------------------------------------------------- range text

def fmt(w) -> str:
    return str(int(w)) if float(w).is_integer() else repr(w)


def rand_weight(rng, p0, p1):
    r = rng.random()
    if r < p0:
        return 0
    if r < p0 + p1:
        return 1
    return round(rng.random(), rng.randint(1, 6))


KEEP_ZERO = 0.03  # sparse ranges keep this fraction of their weight-0 tokens


def body(weight_of, sep=",", keep_zero=None) -> str:
    """keep_zero: None writes every combo; a Random drops most weight-0 tokens (a missing combo weighs 0 anyway)."""
    tokens = []
    for h in CLASSES:
        for c in H._combos(h):
            w = weight_of(h, c)
            if keep_zero is None or w != 0 or keep_zero.random() < KEEP_ZERO:
                tokens.append(f"{c}: {fmt(w)}")
    return sep.join(tokens)


def R(seed, p0=0.4, p1=0.3, per_combo=False, sep=",", sparse=False) -> str:
    rng = random.Random(seed)
    keep = random.Random(f"{seed}:keep") if sparse else None
    if per_combo:
        return body(lambda h, c: rand_weight(rng, p0, p1), sep, keep)
    by_class = {h: rand_weight(rng, p0, p1) for h in CLASSES}
    return body(lambda h, c: by_class[h], sep, keep)


def ALL(w) -> str:
    return body(lambda h, c: w)


def exotic(style: int) -> str:
    """Weight spellings Python float() accepts (and one empty / missing value)."""
    spell = [" 0.5 ", "\t.5", "5.", "+1", "1E-3", "1e-400", "0.333333", "1.0", "0e0", "1_0", "", "0.666667", "-0", "2"]
    out = []
    k = style
    for h in CLASSES:
        for c in H._combos(h):
            s = spell[k % len(spell)]
            k += 1
            out.append(c if s == "" and k % 2 else f"{c}:{s}")
    return ",".join(out)


# ---------------------------------------------------------------- handcrafted ranges

class Files:
    def __init__(self):
        self.files: Manifest = {}
        self.used: set = set()

    def add(self, path, content, *keys):
        self.files[path] = content.encode("utf-8") if isinstance(content, str) else content
        self.used.update(keys)


def std_name(node, a, b, action, variant):
    if node == "vs_open":
        base = f"{b} vs {a} open {action}"
    elif node == "vs_3bet":
        base = f"{a} vs {b} 3bet {action}"
    else:
        base = f"{b} vs {a} 4bet {action}"
    return [base, base.lower(), base.upper(), base.replace(" ", "  ")][variant % 4]


def handcrafted_ranges() -> Manifest:
    f = Files()
    seed = iter(range(100, 1000))
    r = lambda *a, **k: R(next(seed), *a, **k)  # noqa: E731

    # open
    f.add("UTG open.txt", r(0.5, 0.2, per_combo=True), ("open", "UTG"))
    f.add("hj  open  2.5bb.txt", r(0.5, 0.2), ("open", "HJ"))
    f.add("CO open.TXT", r(), ("open", "CO"))
    f.add("BTN OPEN 2bb.txt", r(0.4, 0.3, per_combo=True), ("open", "BTN"))
    f.add("SB open.txt", ",,,", ("open", "SB"))
    f.add("BB open.txt", r())

    # vs_open: every pair is special
    f.add("BB vs UTG open call.txt", r(0.6, 0.1), ("vs_open", "UTG", "BB"))
    f.add("BB vs UTG open 3bet.txt", r(0.7, 0.1))
    f.add("bb vs hj open 2.5bb 3b.txt", ALL(1), ("vs_open", "HJ", "BB"))
    f.add("BB vs HJ open call 7.5bb.txt", ALL(1))
    f.add("BB vs CO open call.txt", r(0.5, 0.2), ("vs_open", "CO", "BB"))
    f.add("BB vs BTN open 3bet.txt", r(0.6, 0.2), ("vs_open", "BTN", "BB"))
    f.add("BTN vs CO open call.txt", r(0.5, 0.2), ("vs_open", "CO", "BTN"))
    f.add("BTN vs CO open 3b 9bb.txt", r(0.6, 0.2))
    f.add("SB vs BTN open call.txt", r(0.7, 0.1), ("vs_open", "BTN", "SB"))
    f.add("SB vs BTN open 3bet.txt", r(0.7, 0.1))
    f.used.add(("vs_open", "CO", "SB"))
    f.add("BB vs SB open call.txt", ALL(1))
    f.add("BB vs SB open 3bet.txt", ALL(5e-7), ("vs_open", "SB", "BB"))
    # unreadable files, then a good one that takes the node
    f.add("a_bad_card/HJ vs UTG open call.txt", "AsXx: 1,AsAh: 1")
    f.add("a_dup_combo/HJ vs UTG open call.txt", "AsAh: 1,AhAs: 1")
    f.add("a_neg_weight/HJ vs UTG open call.txt", "AsAh: -1")
    f.add("a_nan/HJ vs UTG open call.txt", "AsAh: nan")
    f.add("a_same_card/HJ vs UTG open call.txt", "AsAs: 1")
    f.add("a_short_token/HJ vs UTG open call.txt", "AsA: 1")
    f.add("z_good/HJ vs UTG open call.txt", ALL(1), ("vs_open", "UTG", "HJ"))
    f.add("HJ vs UTG open 3bet.txt", ALL(2e-6))
    f.add("a_invalid_utf8/CO vs UTG open call.txt", b"AsAh: 1,\xff\xfe: 1")
    f.add("a_double_bom/CO vs UTG open call.txt", b"\xef\xbb\xbf\xef\xbb\xbfAsAh: 1")
    f.add("a_zwsp/CO vs UTG open call.txt", "\u200bAsAh: 1")
    f.add("a_hex/CO vs UTG open call.txt", "AsAh: 0x10")
    f.add("a_inf/CO vs UTG open call.txt", "AsAh: inf")
    f.add("a_overflow/CO vs UTG open call.txt", "AsAh: 1e400")
    f.add("a_upper_suit/CO vs UTG open call.txt", "AsAH: 1")
    f.add("a_lower_rank/CO vs UTG open call.txt", "asAh: 1")
    f.add("a_double_colon/CO vs UTG open call.txt", "AsAh: 1: 2")
    f.add("z_good/CO vs UTG open call.txt", r(0.5, 0.2), ("vs_open", "UTG", "CO"))
    f.add("CO vs UTG open 3bet.txt", r(0.6, 0.2))
    # blank files before / after the real one
    for name, text in [("empty", b""), ("bom", BOM), ("ws", "  \n\t "), ("1c", "\x1c"), ("nbsp", "\u00a0\u2003"),
                       ("crlf", "\r\n\r\n"), ("bom_ws", BOM + " \n")]:
        f.add(f"0blank_{name}/BTN vs UTG open call.txt", text)
    f.add("5real/BTN vs UTG open call.txt", r(0.5, 0.2), ("vs_open", "UTG", "BTN"))
    f.add("1real/BTN vs UTG open 3bet.txt", r(0.6, 0.2))
    f.add("7dup/BTN vs UTG open 3bet.txt", ALL(1))
    f.add("9blank_empty/BTN vs UTG open 3bet.txt", b"")
    f.add("9blank_ws/BTN vs UTG open 3bet.txt", " \r\n ")
    # Path sort order: a < a b < a-x (a whole-string sort would put "a b" first)
    for folder in ("a", "a b", "a-x"):
        f.add(f"{folder}/SB vs UTG open call.txt", r(0.4, 0.3))
    f.add("SB vs UTG open 3bet.txt", r(0.6, 0.2), ("vs_open", "UTG", "SB"))
    # exotic but valid weights / separators / line endings
    f.add("CO vs HJ open call.txt", exotic(0), ("vs_open", "HJ", "CO"))
    f.add("CO vs HJ open 3bet.txt", ",\n" + R(next(seed), 0.4, 0.3, sep=" ,\n ") + ",\n\n,", )
    f.add("BTN vs HJ open call.txt", BOM + r(0.5, 0.2), ("vs_open", "HJ", "BTN"))
    f.add("BTN vs HJ open 3bet.txt", R(next(seed), 0.5, 0.2, sep="\r\n"))
    f.add("SB vs HJ open call.txt", R(next(seed), 0.5, 0.2, sep="\r") + "\r", ("vs_open", "HJ", "SB"))
    f.add("SB vs HJ open 3bet.txt", r(0.5, 0.2) + "\r\n")

    # vs_3bet
    f.add("UTG vs BB 3bet call.txt", r(0.5, 0.2, per_combo=True), ("vs_3bet", "UTG", "BB"))
    f.add("UTG vs BB 3bet 4bet.txt", r(0.7, 0.1))
    f.add("hj vs sb 3bet call.txt", r(0.5, 0.3), ("vs_3bet", "HJ", "SB"))
    f.add("HJ vs SB 3bet 4bet 22bb allin.txt", r(0.6, 0.2))
    f.add("HJ vs SB 3bet 4bet all in.txt", r(0.6, 0.2))
    f.add("HJ vs SB 3bet 4bet.txt", r(0.6, 0.2))
    f.add("BTN vs SB 3bet 4bet 22bb all in.txt", r(0.5, 0.3), ("vs_3bet", "BTN", "SB"))
    f.add("CO vs BB 3bet call.txt", r(0.5, 0.2), ("vs_3bet", "CO", "BB"))
    f.add("CO vs BB 3bet 4bet 4bet.txt", r(0.6, 0.2))
    f.add("UTG vs BTN 3bet call.txt", ALL(1), ("vs_3bet", "UTG", "BTN"))
    f.add("UTG vs BTN 3bet 4bet.txt", ALL(1))
    f.add("SB vs BB 3bet call.txt", r(0.5, 0.2), ("vs_3bet", "SB", "BB"))
    f.add("SB vs BB 3bet 4bet.txt", r(0.5, 0.2))
    f.add("BTN vs BB 3bet call.txt/inner.txt", r(), ("vs_3bet", "BTN", "BB"))
    f.add("z_ok/BTN vs BB 3bet call.txt", r(0.5, 0.2))
    f.add("BTN vs BB 3bet 4bet.txt", r(0.6, 0.2))
    f.add("HJ\u00a0vs BB 3bet call.txt", r(0.5, 0.2), ("vs_3bet", "HJ", "BB"))
    f.add("HJ vs BB 3bet 4bet.txt", r(0.6, 0.2))
    f.add("utg VS hj 3BET CALL 9bb.txt", r(0.5, 0.2), ("vs_3bet", "UTG", "HJ"))
    f.add("UTG vs HJ 3bet 4BET 4bet 22BB ALLIN.txt", r(0.6, 0.2))
    f.add(".hidden/UTG vs HJ 3bet 4bet.txt", r(0.6, 0.2))
    f.add("UTG  vs  SB 3bet 9bb call.txt", r(0.5, 0.2), ("vs_3bet", "UTG", "SB"))
    f.add("UTG vs SB 3bet 4bet.txt", r(0.6, 0.2))
    f.add("HJ vs BTN 3bet call.txt", r(0.5, 0.2), ("vs_3bet", "HJ", "BTN"))
    f.add("HJ vs BTN 3bet 4bet allin.txt", r(0.6, 0.2))

    # vs_4bet
    f.add("BB vs UTG 4bet call.txt", ALL(1), ("vs_4bet", "UTG", "BB"))
    f.add("BB vs UTG 4bet 5bet.txt", r(0.5, 0.2))
    f.add("BB vs UTG 4bet 5bet all in.txt", r(0.5, 0.2))
    f.add("BB vs HJ 4bet 5bet all in.txt", r(0.5, 0.2), ("vs_4bet", "HJ", "BB"))
    f.add("BTN vs CO 4bet call.txt", r(0.5, 0.2), ("vs_4bet", "CO", "BTN"))
    f.add("BTN vs CO 4bet 5bet all in.txt", r(0.6, 0.2))
    f.add("SB vs CO 4bet call.txt", r(0.5, 0.2), ("vs_4bet", "CO", "SB"))
    f.add("SB vs CO 4bet 5bet.txt", r(0.6, 0.2))
    f.add("BB vs CO 4bet call.txt", r(0.5, 0.2), ("vs_4bet", "CO", "BB"))

    # ignored files
    f.add("notes.txt", "not a range\n")
    f.add("UTG vs HJ vs BB 3bet call.txt", r())
    f.add("UTG vs BB open call.txt", r())
    f.add("UTG open 2.5.txt", r())
    f.add("HJ open (copy).txt", r())
    f.add("UTG open.TXT.txt", r())
    f.add(".UTG open.txt", r())
    f.add("README.md", "scanned? no: only .txt\n")

    # std fill for the pairs nobody special-cased
    for node, actions in [("vs_3bet", ("call", "4bet")), ("vs_4bet", ("call", "5bet", "5bet all in"))]:
        for n, (a, b) in enumerate(PAIRS):
            if (node, a, b) in f.used:
                continue
            for k, action in enumerate(actions):
                f.add(f"std/{std_name(node, a, b, action, n + k)}.txt", r(0.45, 0.25, sparse=True))
    return f.files


def posix_extras() -> dict[str, Manifest]:
    """Files that cannot live in the repository as-is (tab in a name, symlinks); see specials/specials.json."""
    tab_placeholder = "ranges/HJ__TAB__vs__TAB__BTN__TAB__3bet__TAB__call.txt"
    specials = {
        "rename": [{"from": tab_placeholder, "to": "ranges/HJ\tvs\tBTN\t3bet\tcall.txt"}],
        "symlink": [{"path": "ranges/link/BB vs SB open call.txt", "target": "../shared/source_for_link.txt"},
                    {"path": "ranges/0link", "target": "../ranges_decoy"}],
    }
    ranges = {"HJ open.txt": R(7001, 0.5, 0.2), "BTN OPEN 2bb.txt": R(7002, 0.4, 0.3),
              tab_placeholder.removeprefix("ranges/"): R(7003, 0.5, 0.2), "HJ vs BTN 3bet 4bet.txt": R(7004, 0.6, 0.2),
              "shared/source_for_link.txt": ALL(1), "BB vs SB open 3bet.txt": R(7005, 0.5, 0.2)}
    hands = []
    for n, (a, b, depth) in enumerate((a, b, d) for a, b in (("HJ", "BTN"), ("SB", "BB")) for d in range(1, 4)):
        for last in ("fold", "call", "raise"):
            acts, dec, opp = chain(a, b, depth, last, n)
            for hero in (dec, opp, "NONE"):
                hands.append(mk(acts, f"PX{len(hands):04d}", hero, "2026/10/0%d" % (len(hands) % 9 + 1), len(hands) % 6 + 1))
    return {"hands": {"posix.txt": "".join(hands).encode()},
            "ranges": {k: v.encode() for k, v in ranges.items()},
            "ranges_decoy": {"BTN OPEN 2bb.txt": R(9999, 0.1, 0.1).encode()},
            "specials": {"specials.json": (json.dumps(specials, indent=1, sort_keys=True) + "\n").encode()}}


# ---------------------------------------------------------------- hands

def seats_for(button):
    return tuple(H.SEAT_POSITIONS[(s - button) % 6] for s in range(1, 7))


def mk(actions, hid, hero="BB", day="2026/10/06", button=1, extra=()):
    return H.make_hand(actions, hid, hero, day, seats_for(button), button, extra)


def commas(text):
    return re.sub(r"\$(\d{4,})(\.\d+)?", lambda m: f"${int(m[1]):,}{m[2] or ''}", text)


def mk_scaled(actions, hid, bb, hero="BB", day="2026/10/06", button=1):
    old, H.BB = H.BB, bb
    try:
        text = mk(actions, hid, hero, day, button)
    finally:
        H.BB = old
    text = text.replace("($0.05/$0.1)", f"(${bb / 2:,.2f}/${bb:,.0f})").replace("($10 in chips)", "($10,000.50 in chips)")
    return commas(text)


def between(a, b):
    return POS[POS.index(a) + 1:POS.index(b)]


def chain(a, b, depth, last, v):
    """Actions of a hand that reaches decision `depth` of the pair (1 vs_open by b ... 4 vs_5bet by a) and takes `last`."""
    sizes = [(2, 2.5, 3)[v % 3], (7.5, 9, 10.5, 12)[v % 4], (18, 22, 25)[v % 3], 55.0]
    acts = [H.folds(p) for p in POS[:POS.index(a)]] + [H.raises(a, sizes[0])] + [H.folds(p) for p in between(a, b)]
    players = (b, a, b, a)
    for k in range(1, depth):
        acts.append(H.raises(players[k - 1], sizes[k]))
        if k == 1:
            acts += [H.folds(p) for p in POS[POS.index(b) + 1:]]
    decider = players[depth - 1]
    if depth == 1:
        tail = [H.folds(p) for p in POS[POS.index(b) + 1:]]
    else:
        tail = []
    if last == "fold":
        acts.append(H.folds(decider))
    elif last == "call":
        acts.append(H.calls(decider))
    else:
        allin = depth >= 3 or v % 4 == 3
        acts.append(H.raises(decider, 100 if allin else sizes[depth], allin))
    return acts + tail, decider, (a if decider == b else b)


def node_hands():
    out, n = [], 0
    days = ["2026/10/06", "2026/10/07", "2026/10/08", "2026/09/30", "2026/12/31", "2025/01/01"]
    for v in range(2):
        for a, b in PAIRS:
            for depth in range(1, 5):
                for last in ("fold", "call", "raise"):
                    acts, dec, opp = chain(a, b, depth, last, v + n % 3)
                    for hero in (dec, opp, "NONE"):
                        n += 1
                        out.append(mk(acts, f"HC{n:05d}", hero, days[n % len(days)], n % 6 + 1))
    for v in range(3):
        for pos in POS[:-1]:
            before = [H.folds(p) for p in POS[:POS.index(pos)]]
            for last in (H.raises(pos, (2, 2.5, 3)[v]), H.folds(pos), H.calls(pos)):
                for hero in (pos, "NONE"):
                    n += 1
                    out.append(mk(before + [last] + [H.folds(p) for p in POS[POS.index(pos) + 1:]], f"HO{n:05d}", hero,
                                  days[n % len(days)], n % 6 + 1))
    return out


def multi_scenarios():
    f, c, r = H.folds, H.calls, H.raises
    return [
        [c("UTG"), r("HJ", 4), f("CO"), f("BTN"), f("SB"), f("BB"), c("UTG")],
        [r("UTG", 2.5), f("HJ"), c("CO"), r("BTN", 10), f("SB"), f("BB"), f("UTG"), f("CO")],
        [r("UTG", 2.5), r("HJ", 9), r("CO", 22), f("BTN"), f("SB"), f("BB"), f("UTG"), f("HJ")],
        [f("UTG"), f("HJ"), c("CO"), r("BTN", 4), r("CO", 12), c("BTN"), f("SB"), f("BB")],
        [r("UTG", 2.5), c("HJ"), c("CO"), r("BTN", 12), f("SB"), f("BB"), f("UTG"), f("HJ"), f("CO")],
        [f("UTG"), f("HJ"), f("CO"), f("BTN"), c("SB"), r("BB", 4), c("SB")],
        [f("UTG"), f("HJ"), f("CO"), f("BTN"), f("SB")],
        [f("UTG"), f("HJ"), f("CO"), r("BTN", 2.5), r("SB", 10), c("BB"), c("BTN")],
        [r("UTG", 2.5), f("HJ"), f("CO"), f("BTN"), f("SB"), r("BB", 9), r("UTG", 22), c("BB")],
        [r("UTG", 2.5), c("HJ"), f("CO"), f("BTN"), f("SB"), r("BB", 9), r("UTG", 22), f("HJ"), c("BB")],
        [r("UTG", 2.5), f("HJ"), f("CO"), f("BTN"), f("SB"), r("BB", 9), c("UTG"), ("BB", "checks")],
    ]


def under_raise_scenarios():
    f, c, r = H.folds, H.calls, H.raises
    return [
        [r("UTG", 1.5, True), r("HJ", 4), f("CO"), f("BTN"), f("SB"), f("BB"), c("UTG")],
        [r("UTG", 2.5), f("HJ"), f("CO"), f("BTN"), f("SB"), r("BB", 3.0, True), f("UTG")],
        [r("UTG", 2.5), r("HJ", 9), f("CO"), f("BTN"), f("SB"), f("BB"), r("UTG", 10, True), c("HJ")],
        [r("UTG", 2.5), f("HJ"), f("CO"), f("BTN"), f("SB"), r("BB", 9), r("UTG", 22), r("BB", 24, True), f("UTG")],
        [r("UTG", 2.5), f("HJ"), f("CO"), f("BTN"), f("SB"), r("BB", 4.0, True), f("UTG")],
        [r("UTG", 2.5), f("HJ"), f("CO"), f("BTN"), f("SB"), r("BB", 9), r("UTG", 22), r("BB", 55), r("UTG", 60, True), c("BB")],
        [r("UTG", 2.5), f("HJ"), f("CO"), f("BTN"), r("SB", 9), r("BB", 12, True), r("UTG", 40), f("SB"), c("BB")],
        [r("UTG", 2.5), f("HJ"), f("CO"), r("BTN", 9), f("SB"), f("BB"), r("UTG", 22), r("BTN", 100, True), f("UTG")],
    ]


def handcrafted_hand_files() -> Manifest:
    fl: dict[str, str] = {}
    nodes = node_hands()
    third = len(nodes) // 3
    fl["nodes/part1.txt"], fl["nodes/part2.txt"] = "".join(nodes[:third]), "".join(nodes[third:2 * third])
    fl["nodes/2026-10/part3.txt"] = "".join(nodes[2 * third:])
    special = []
    n = 0
    for acts in multi_scenarios() + under_raise_scenarios():
        for hero in (*POS, "NONE"):
            n += 1
            special.append(mk(acts, f"HM{n:04d}", hero, "2026/10/0%d" % (n % 9 + 1), n % 6 + 1))
    fl["special/multiway_and_underraise.txt"] = "".join(special)
    # all-in exactly the minimum raise is a real raise
    fl["special/min_raise_allin.txt"] = "".join(
        mk([H.raises("UTG", 2.5), H.folds("HJ"), H.folds("CO"), H.folds("BTN"), H.folds("SB"), H.raises("BB", 4.0, True),
            H.calls("UTG")], f"HQ{i}", hero) for i, hero in enumerate(("BB", "UTG", "NONE")))

    # shapes of table / ids / garbage
    base = mk([H.raises("UTG", 2.5), H.folds("HJ"), H.folds("CO"), H.folds("BTN"), H.folds("SB"), H.calls("BB")], "HB1")
    five = H.make_hand([H.raises("UTG", 2.5), H.folds("HJ"), H.folds("BTN"), H.folds("SB"), H.calls("BB")], "HT5", "BB",
                       "2026/10/06", ("BTN", "SB", "BB", "UTG", "HJ"), 1)
    seven = H.make_hand([H.raises("UTG", 2.5), H.folds("HJ")], "HT7", "BB", "2026/10/06",
                        ("BTN", "SB", "BB", "UTG", "HJ", "CO", "X"), 1)
    two = H.make_hand([H.folds("SB")], "HT2", "BB", "2026/10/06", ("SB", "BB"), 1)
    nine = H.make_hand([H.raises("UTG", 2.5)], "HT9", "BB", "2026/10/06",
                       ("BTN", "SB", "BB", "UTG", "HJ", "CO", "A", "B", "C"), 1).replace("6-max", "9-max")
    fl["shapes/table_sizes.txt"] = five + seven + two + nine + base
    dup_a = mk([H.raises("UTG", 2.5), H.folds("HJ"), H.folds("CO"), H.folds("BTN"), H.folds("SB"), H.raises("BB", 9)], "HD1")
    dup_b = mk([H.raises("HJ", 2.5), H.folds("CO"), H.folds("BTN"), H.folds("SB"), H.folds("BB")], "HD1")
    bad = [
        ("omaha", base.replace("Hold'em No Limit", "Omaha Pot Limit").replace("HB1", "HU1")),
        ("no_summary", base.replace("*** SUMMARY ***", "*** SHOW DOWN ***").replace("HB1", "HU2")),
        ("unknown_action", base.replace("p_HJ: folds", "p_HJ: dances").replace("HB1", "HU3")),
        ("unseated", base.replace("p_HJ: folds", "p_GHOST: folds").replace("HB1", "HU4")),
        ("empty_button", base.replace("Seat #1 is the button", "Seat #9 is the button").replace("HB1", "HU5")),
        ("blind_mismatch", H.make_hand([H.raises("UTG", 2.5), H.folds("HJ")], "HU6", "BB", "2026/10/06", H.SEAT_POSITIONS, 2)),
        ("dup_names", base.replace("p_HJ ($10", "p_UTG ($10").replace("HB1", "HU7")),
        ("pre_deal_line", base.replace("*** HOLE CARDS ***", "Weird table message\n*** HOLE CARDS ***").replace("HB1", "HU8")),
        ("truncated", base[:base.index("*** SUMMARY ***")].replace("HB1", "HU9")),
        ("header_only", "Poker Hand #HU10: nothing else\n"),
        ("bad_amount", base.replace("p_UTG: raises $0.25 to $0.25", "p_UTG: raises $, to $,").replace("HB1", "HU11")),
        ("no_table_line", base.replace("Table 'RushAndCash1' 6-max Seat #1 is the button\n", "").replace("HB1", "HU12")),
        ("flop_junk", base.replace("*** SUMMARY ***", "*** FLOP *** [Ah Kd 2c]\nweird: ???\n*** SUMMARY ***").replace("HB1", "HU13")),
    ]
    fl["shapes/unparseable.txt"] = "stray line before first hand\n" + dup_a + "".join(t for _, t in bad) + dup_b
    fl["shapes/dups_1.txt"] = dup_a + dup_a + base
    fl["shapes/dups_2.txt"] = dup_b + base.replace("p_UTG: raises $0.25 to $0.25", "p_UTG: folds")
    # extra lines and trailing whitespace are fine
    noisy = mk([H.raises("UTG", 2.5), H.folds("HJ"), H.folds("CO"), H.folds("BTN"), H.folds("SB"), H.raises("BB", 9),
                H.calls("UTG")], "HN1", "BB", extra=(
                    "Uncalled bet ($0.5) returned to p_BB", "p_UTG: shows [Ah Kd] (a pair)", "Cash Drop to Pot : $0.01",
                    "p_BB: Bet & Muck folds", "*** FLOP *** [Ah Kd 2c]", "p_BB: bets $0.5", "p_UTG: calls $0.5",
                    "All-in Insurance paid $0.2"))
    fl["shapes/noisy.txt"] = noisy.replace("p_HJ: folds", "p_HJ: folds   ") + noisy.replace("HN1", "HN2").replace(
        "*** HOLE CARDS ***", "Cash Drop to Pot : $0.01\n*** HOLE CARDS ***")
    commas_hands = "".join(
        mk_scaled([H.raises("UTG", 2.5), H.folds("HJ"), H.folds("CO"), H.folds("BTN"), H.folds("SB"), H.raises("BB", 9),
                   H.raises("UTG", 22), H.raises("BB", 12000, True) if i else H.calls("BB")], f"HK{i}", 1000.0, hero) for i, hero in
        enumerate(("BB", "UTG", "NONE")))
    fl["shapes/comma_amounts.txt"] = commas_hands
    fl["shapes/no_hero.txt"] = "".join(mk(a, f"HV{i}", "NONE", "2026/11/0%d" % (i % 9 + 1)) for i, a in
                                       enumerate(multi_scenarios()))
    renum = mk([H.raises("UTG", 2.5), H.folds("HJ"), H.folds("CO"), H.folds("BTN"), H.folds("SB"), H.calls("BB")], "HS1", button=3)
    renum = "\n".join(re.sub(r"^Seat (\d+):", lambda m: f"Seat {int(m[1]) * 2 + 1}:", ln) for ln in renum.split("\n"))
    fl["shapes/seat_numbers.txt"] = renum.replace("Seat #3 is the button", "Seat #7 is the button")
    fl["shapes/no_trailing_newline.txt"] = mk([H.raises("CO", 2.5), H.folds("BTN"), H.folds("SB"), H.calls("BB")], "HL1").rstrip("\n")

    out: Manifest = {k: v.encode("utf-8") for k, v in fl.items()}
    sample = mk([H.raises("UTG", 2.5), H.folds("HJ"), H.folds("CO"), H.folds("BTN"), H.folds("SB"), H.raises("BB", 9),
                 H.calls("UTG")], "EA1") + mk([H.raises("HJ", 2.5), H.folds("CO"), H.folds("BTN"), H.folds("SB"),
                                                H.calls("BB")], "EA2", "HJ", "2026/10/07")
    out["encoding/bom_lf.txt"] = (BOM + sample).encode()
    out["encoding/double_bom.txt"] = (BOM + BOM + sample.replace("EA", "EB")).encode()
    out["encoding/crlf.txt"] = sample.replace("EA", "EC").replace("\n", "\r\n").encode()
    out["encoding/bom_crlf.txt"] = (BOM + sample.replace("EA", "ED").replace("\n", "\r\n")).encode()
    out["encoding/cr_only.txt"] = sample.replace("EA", "EE").replace("\n", "\r").encode()
    out["encoding/mixed_eol.txt"] = (sample.replace("EA", "EF").replace("\n", "\r\n", 12)).encode()
    out["encoding/invalid_utf8_in_dealt_line.txt"] = (
        sample.replace("EA", "EG").replace("Dealt to p_HJ ", "Dealt to p_HJ \x00FF", 1).encode().replace(b"\x00FF", b"\xff\xfe"))
    out["encoding/empty.txt"] = b""
    out["encoding/only_bom.txt"] = BOM.encode()
    out["encoding/whitespace_only.txt"] = b"\n\n  \n"
    out["ignored/readme.md"] = sample.replace("EA", "EH").encode()
    out["ignored/upper_ext.TXT"] = sample.replace("EA", "EI").encode()
    out["ignored/day.txt.bak"] = sample.replace("EA", "EJ").encode()
    out["2026-10/nested/deep/session.txt"] = (sample.replace("EA", "EK")).encode()
    return out


# ---------------------------------------------------------------- random_s42

def std_names():
    names = [f"{p} open" for p in POS[:-1]]
    for a, b in PAIRS:
        names += [f"{b} vs {a} open call", f"{b} vs {a} open 3bet", f"{a} vs {b} 3bet call", f"{a} vs {b} 3bet 4bet",
                  f"{b} vs {a} 4bet call", f"{b} vs {a} 4bet 5bet", f"{b} vs {a} 4bet 5bet all in"]
    skip = {"BB vs SB 4bet 5bet", "BTN vs CO 4bet 5bet"}
    return [n for n in names if n not in skip]


def random_ranges() -> Manifest:
    names = std_names()
    assert len(names) == 108, len(names)
    out: Manifest = {}
    for i, name in enumerate(names):
        rng = random.Random(42_000 + i)
        by_class = rng.random() < 0.5
        p0, p1 = rng.uniform(0.4, 0.7), rng.uniform(0.1, 0.25)
        text = R(rng.randrange(10 ** 9), p0, p1, per_combo=not by_class, sparse=True)
        values = {tok.split(": ")[1] for tok in text.split(",")}
        assert "0" in values and "1" in values and len(values) > 5, name
        out[f"{name}.txt"] = text.encode()
    return out


def simulate(rng):
    stack = {p: rng.choice((20, 40, 60, 100, 100, 100, 150, 250)) for p in POS}
    committed = {p: 0.0 for p in POS}
    committed["SB"], committed["BB"] = 0.5, 1.0
    folded, allin, acted = set(), set(), set()
    current, last_size, raises_n, acts, i = 1.0, 1.0, 0, [], 0
    while True:
        live = [p for p in POS if p not in folded]
        pending = [p for p in live if p not in allin and (p not in acted or committed[p] < current)]
        if len(live) == 1 or not pending:
            return acts
        while POS[i % 6] not in pending:
            i += 1
        p = POS[i % 6]
        i += 1
        to_call = current - committed[p]
        x = rng.random()
        fold_p, call_p = (0.62, 0.08) if raises_n == 0 else (0.6, 0.25) if raises_n == 1 else (0.5, 0.3)
        can_raise = stack[p] > current and raises_n < 6
        if to_call == 0:
            action = "raise" if can_raise and x < 0.15 else "check"
        elif x < fold_p:
            action = "fold"
        elif x < fold_p + call_p or not can_raise:
            action = "call"
        else:
            action = "raise"
        acted.add(p)
        if action == "fold":
            folded.add(p)
            acts.append(H.folds(p))
        elif action == "check":
            acts.append((p, "checks"))
        elif action == "call":
            committed[p] = min(current, stack[p])
            if committed[p] == stack[p]:
                allin.add(p)
            acts.append(H.calls(p))
        else:
            min_to = current + last_size
            target = rng.choice((2, 2.2, 2.5, 3)) if raises_n == 0 else round(current * rng.uniform(2.2, 3.5) * 2) / 2
            target = max(target, min_to)
            go_all = target >= stack[p]
            to = float(stack[p]) if go_all else target
            if to - current >= last_size:
                last_size = to - current
            current, committed[p] = to, to
            raises_n += 1
            if go_all:
                allin.add(p)
            acts.append(H.raises(p, to, go_all))
            acted = {p}


STRAT_PER_CELL = 64  # hands per (position pair, depth); half have the Hero as the player deciding at that depth


def strat_actions(rng, a, b, depth):
    """Random legal raise sizes for a chain open / 3bet / 4bet / 5bet / 6bet; returns (actions, decider, opponent)."""
    open_to = rng.choice((2, 2.5, 3))
    three = round(open_to * rng.uniform(3, 4.5) * 2) / 2
    four = round(three * rng.uniform(2, 2.7) * 2) / 2
    five_allin = depth == 3 and rng.random() < 0.5
    five = 100.0 if five_allin else round(four * rng.uniform(2, 2.5) * 2) / 2
    sizes = [open_to, three, four, five]
    last = rng.choice(("fold", "call", "raise"))
    acts = [H.folds(p) for p in POS[:POS.index(a)]] + [H.raises(a, open_to)] + [H.folds(p) for p in between(a, b)]
    players = (b, a, b, a)
    for k in range(1, depth):
        acts.append(H.raises(players[k - 1], sizes[k], sizes[k] >= 100))
        if k == 1:
            acts += [H.folds(p) for p in POS[POS.index(b) + 1:]]
    decider = players[depth - 1]
    tail = [H.folds(p) for p in POS[POS.index(b) + 1:]] if depth == 1 else []
    if last == "raise":
        to = 100.0 if depth == 4 or (depth == 3 and rng.random() < 0.5) else sizes[depth]
        acts.append(H.raises(decider, to, to >= 100))
    else:
        acts.append(H.folds(decider) if last == "fold" else H.calls(decider))
    return acts + tail, decider, (a if decider == b else b)


def strat_hands(rng, start_id: int) -> Manifest:
    """Every (pair, depth) cell gets STRAT_PER_CELL hands so that each vs_* cell has enough me and pool observations."""
    out: Manifest = {}
    n = start_id
    start = datetime.date(2026, 9, 1)
    for a, b in PAIRS:
        for depth in range(1, 5):
            texts = []
            for i in range(STRAT_PER_CELL):
                acts, decider, opp = strat_actions(rng, a, b, depth)
                if i % 2 == 0:
                    hero = decider
                else:
                    hero = rng.choice((opp, opp, "NONE", rng.choice(POS)))
                day = start + datetime.timedelta(days=rng.randrange(30))
                texts.append(mk(acts, f"RS{1000000000 + n}", hero, day.strftime("%Y/%m/%d"), rng.randrange(1, 7)))
                n += 1
            out[f"strat/{a}-{b}_depth{depth}.txt"] = "".join(texts).encode()
    return out


def random_hands(count=3000, per_file=100) -> Manifest:
    rng = random.Random(42)
    out: dict[str, list[str]] = {}
    start = datetime.date(2026, 9, 1)
    for n in range(count):
        day = start + datetime.timedelta(days=n // per_file)
        hero = rng.choice(POS) if rng.random() > 0.08 else "NONE"
        text = mk(simulate(rng), f"RC{1000000000 + n}", hero, day.strftime("%Y/%m/%d"), rng.randrange(1, 7))
        out.setdefault(f"{day:%Y-%m}/{day:%Y-%m-%d}.txt", []).append(text)
    return {k: "".join(v).encode() for k, v in out.items()} | strat_hands(rng, count)


# ---------------------------------------------------------------- output

def build() -> dict[str, Manifest]:
    return {
        "handcrafted/hands": handcrafted_hand_files(),
        "handcrafted/ranges": handcrafted_ranges(),
        "random_s42/hands": random_hands(),
        "random_s42/ranges": random_ranges(),
    } | {f"posix_extras/{sub}": files for sub, files in posix_extras().items()}


def disk_manifest(root: Path) -> Manifest:
    out: Manifest = {}
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        for name in dirnames + filenames:
            p = Path(dirpath) / name
            rel = p.relative_to(root).as_posix()
            if p.is_symlink():
                out[rel] = ("symlink", os.readlink(p))
            elif p.is_file():
                out[rel] = p.read_bytes()
    return out


def flatten(tree: dict[str, Manifest]) -> Manifest:
    return {f"{d}/{p}": v for d, files in tree.items() for p, v in files.items()}


def manifest_sha256(tree: dict[str, Manifest]) -> str:
    """sha256 over the sorted lines '<path>\\t<sha256 of content>'."""
    lines = []
    for rel, value in sorted(flatten(tree).items()):
        lines.append(f"{rel}\t{hashlib.sha256(value).hexdigest()}\n")
    return hashlib.sha256("".join(lines).encode("utf-8")).hexdigest()


def write(out_dir: Path, tree: dict[str, Manifest]) -> None:
    for corpus, subs in CORPUS_DIRS.items():
        for sub in subs:
            shutil.rmtree(out_dir / corpus / sub, ignore_errors=True)
    for rel, value in sorted(flatten(tree).items()):
        path = out_dir / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(value)


def check(out_dir: Path, tree: dict[str, Manifest]) -> int:
    expected = flatten(tree)
    if flatten(build()) != expected:
        print("NONDETERMINISTIC: two in-memory builds differ", file=sys.stderr)
        return 1
    actual = {}
    for corpus, subs in CORPUS_DIRS.items():
        for sub in subs:
            base = out_dir / corpus / sub
            actual |= {f"{corpus}/{sub}/{k}": v for k, v in disk_manifest(base).items()} if base.is_dir() else {}
    bad = sorted(k for k in expected.keys() | actual.keys() if expected.get(k) != actual.get(k))
    for k in bad:
        print("DIFF:", k, file=sys.stderr)
    print(f"checked {len(expected)} entries, {len(bad)} differ")
    return 1 if bad else 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--py-repo", required=True, type=Path, help="Python preflop_matrix repo (for tests/helpers.py)")
    ap.add_argument("--out", type=Path, default=Path(__file__).resolve().parents[2] / "testdata" / "parity")
    ap.add_argument("--manifest-sha256", action="store_true", help="print the sha256 of the sorted corpus manifest; write nothing")
    ap.add_argument("--check", action="store_true", help="compare a fresh in-memory build with --out; write nothing")
    args = ap.parse_args()
    load_helpers(args.py_repo.expanduser().resolve())
    tree = build()
    if args.manifest_sha256:
        print(manifest_sha256(tree))
        return 0
    if args.check:
        return check(args.out, tree)
    write(args.out, tree)
    print(f"wrote {sum(len(v) for v in tree.values())} entries under {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
