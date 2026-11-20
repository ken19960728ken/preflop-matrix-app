#!/usr/bin/env python3
"""Copy a corpus directory and apply its specials/specials.json (rename files, create symlinks); stdlib only.

    materialize_extras.py <corpus dir> <destination dir>

The Go side does the same in internal/paritydiff (Materialize); both read the same specials.json.
"""
import json
import os
import shutil
import sys
from pathlib import Path


def materialize(src: Path, dst: Path) -> None:
    shutil.copytree(src, dst, symlinks=True)
    specials = json.loads((src / "specials" / "specials.json").read_text(encoding="utf-8"))
    for item in specials.get("rename", []):
        os.rename(dst / item["from"], dst / item["to"])
    for item in specials.get("symlink", []):
        link = dst / item["path"]
        link.parent.mkdir(parents=True, exist_ok=True)
        os.symlink(item["target"], link)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    materialize(Path(sys.argv[1]), Path(sys.argv[2]))
