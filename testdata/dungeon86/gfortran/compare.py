#!/usr/bin/env python3
"""Play DUNGEON games on govax and on gfortran, and compare what they print.

There is no VAX FORTRAN to hold govax's compiler to, but DUNGEON's own source is a
FORTRAN program another compiler can build. This builds it both ways from the kit (which
stays out of the repository: nothing here holds its text), plays each script given, and
reports any line that differs. docs/PHASE-86 - FORTRAN milestone.md has what it found.

    compare.py [--kit DIR] [--govax BINARY] [--keep DIR] [--noopt] script.in ...
    compare.py --sha script.in      the SHA-256 TestDungeonMilestone holds govax's run to

gfortran gets a patched scratch copy. Each change only says in its dialect what VAX FORTRAN
says in its own: FILE= and ACTION= for NAME= and READONLY, RECL in bytes, a LOGICAL
declaration for a function of a logical value, IDATE, ITIME, and RAN as gfsubr.f's.

What's normalized is only what gfortran does with FORTRAN's carriage control: it prints
column 1 as text, a blank or a +, and VMS acts on it. A game may never end, the echo room
repeating even QUIT; at the end of its input, DUNGEON's READ tries again on VMS and stops
on gfortran, so govax's run is cut off, and compared as far as gfortran's went.
"""
import argparse
import difflib
import hashlib
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent.parent.parent
SOURCES = "dungeon game gdt parser subr rooms verbs objects timefnc".split()
FLAGS = ("-std=legacy -fdec -fallow-invalid-boz -fno-automatic -finit-local-zero -fwrapv "
         "-fno-range-check -ffixed-line-length-none -w -O0").split()
LIMIT = 45  # seconds a game of govax's may run


def build_gfortran(kit, work):
    """Makes the gfortran program in work, from a patched copy of the kit."""
    work.mkdir()
    for name in SOURCES:
        shutil.copy(kit / (name + ".f"), work)
    for name in ("dparam.for", "dindx", "dtext"):
        shutil.copy(kit / name, work)
    shutil.copy(HERE / "gfsubr.f", work)

    def patch(name, *pairs):
        path = work / name
        text = path.read_text(errors="surrogateescape")
        for old, new in pairs:
            if old not in text:
                sys.exit(f"{name} has no {old!r}: the kit isn't the one this was written for")
            text = text.replace(old, new)
        path.write_text(text, errors="surrogateescape")

    patch("game.f", ("CALL IDATE(", "CALL XIDATE("), ("CALL ITIME(", "CALL XITIME("),
          ("RECL=RECLNT", "RECL=RECLNT*4"), ("NAME=", "FILE="), ("READONLY", "ACTION='READ'"))
    patch("subr.f", ("CALL ITIME(", "CALL XITIME("), ("REAL RAN", "REAL XRAN"), ("RAN(RNSEED)", "XRAN(RNSEED)"))
    patch("verbs.f", ("NAME=", "FILE="), ("NOTHIS,QHERE,QBUNCH", "NOTHIS,QHERE,QBUNCH,NOHERE"))

    env = dict(os.environ)
    if sys.platform == "darwin" and "SDKROOT" not in env:
        env["SDKROOT"] = subprocess.run(["xcrun", "--show-sdk-path"], capture_output=True, text=True, check=True).stdout.strip()

    files = [name + ".f" for name in SOURCES] + ["gfsubr.f"]
    subprocess.run(["gfortran", *FLAGS, "-o", "dungeon", *files], cwd=work, env=env, check=True)


def build_govax(kit, work, binary, qualifier):
    """Makes the volume, with its image, that each of govax's games starts from."""
    work.mkdir()
    for name in SOURCES:
        shutil.copy(kit / (name + ".f"), work)
    for name in ("dparam.for", "dindx", "dtext"):
        shutil.copy(kit / name, work)
    shutil.copy(HERE.parent / "fixtime.for", work)

    copies = "".join(f'COPY/QUIET "{name}.f"/HOST {name.upper()}.F\n' for name in SOURCES)
    others = [name for name in SOURCES if name != "game"]
    lists = ",".join(name.upper() + ".F" for name in others) + ",GAME.F+FIXTIME.FOR"
    modules = ",".join(name.upper() for name in SOURCES)
    commands = (
        'INITIALIZE/CONTAINER "dun.dsk" /DEVICE=RD54 DUN\nMOUNT DUA1: "dun.dsk"\nSET DEFAULT DUA1:[000000]\n'
        'COPY/QUIET "dindx"/HOST DINDX.DAT\n'
        'COPY/QUIET "dtext"/HOST DTEXT.DAT/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=80\n'
        'COPY/QUIET "dparam.for"/HOST DPARAM.FOR\nCOPY/QUIET "fixtime.for"/HOST FIXTIME.FOR\n'
        + copies
        + f"FORT/NOCHECK/CONT=40{qualifier} {lists}\nMACRO {modules}\nLINK {modules}\nQUIT\n")
    out = run_govax(binary, work, commands, 120)
    if "%" in out.split("mounted on _DUA1:")[-1]:
        sys.exit("govax's build said:\n" + out)
    shutil.copy(work / "dun.dsk", work / "dun.clean")


def run_govax(binary, work, commands, limit):
    """Runs a console session under a HOME of its own: never the user's volumes."""
    home = work / "home"
    home.mkdir(exist_ok=True)
    env = dict(os.environ, HOME=str(home))
    run = subprocess.run([binary, "console"], cwd=work, input=commands.encode(), capture_output=True, env=env, timeout=limit)
    return run.stdout.decode(errors="replace")


def play_govax(binary, work, script):
    """The game's output, its moves read from a file by DEFINE/USER SYS$INPUT, so what's
    left when a game ends can't reach DCL. Returns the output, and if it was cut off."""
    shutil.copy(work / "dun.clean", work / "dun.dsk")
    (work / "game.in").write_text(script)
    commands = ('MOUNT DUA1: "dun.dsk"\nSET DEFAULT DUA1:[000000]\nCOPY/QUIET "game.in"/HOST GAME.IN\n'
                "DEFINE/USER_MODE SYS$INPUT GAME.IN\nRUN DUNGEON\nQUIT\n")
    try:
        out, cut = run_govax(binary, work, commands, LIMIT), False
    except subprocess.TimeoutExpired as e:
        out, cut = (e.stdout or b"").decode(errors="replace"), True
    at = out.find("Welcome to Dungeon.")
    return (out[at:] if at >= 0 else out), cut


def play_gfortran(work, script):
    for name in ("DSAVE.DAT", "dsave.dat"):
        (work / name).unlink(missing_ok=True)
    run = subprocess.run(["./dungeon"], cwd=work, input=script.encode(), capture_output=True, timeout=120)
    return run.stdout.decode(errors="replace")


def prompts(text):
    """The blanks and line ends after a $ prompt are carriage control, on either side."""
    return re.sub(r"(?m)(^>|GDT>|Limits:|Entry:|New=)[ \n]*", r"\1 ", text)


def from_gfortran(text):
    text = re.sub(r"\+(?=[ 0-9]|$)", "", "\n" + text, flags=re.M)
    return prompts(text.replace("\n ", "\n").lstrip("\n")).splitlines()


def compare(path, gf, gv, binary):
    script = pathlib.Path(path).read_text()
    a = from_gfortran(play_gfortran(gf, script))
    out, cut = play_govax(binary, gv, script)
    b = prompts(out).splitlines()
    note = ""
    if cut:
        a = a[:-1]
        note = " (never ended)"
        if len(b) < len(a):
            note = f" (cut off at {len(b)} of {len(a)} lines: INCONCLUSIVE)"
            a = a[:len(b)]
        b = b[:len(a)]
    diff = list(difflib.unified_diff(a, b, "gfortran", "govax", lineterm="", n=2))
    if not diff:
        print(f"{path}: same, {len(a)} lines printed{note}")
        return True
    print(f"{path}: DIFFERENT, {len(a)} and {len(b)} lines{note}")
    for line in diff[:60]:
        print("  " + line[:110])
    return False


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--kit", default=os.path.expanduser("~/Documents/Technical Doc/VMS/dungeon-3.2A"))
    parser.add_argument("--govax", help="govax's binary (default: go build)")
    parser.add_argument("--noopt", action="store_true", help="compile with /NOOPTIMIZE")
    parser.add_argument("--sha", action="store_true", help="print each script's SHA-256 on govax only")
    parser.add_argument("scripts", nargs="+")
    args = parser.parse_args()

    kit = pathlib.Path(args.kit)
    with tempfile.TemporaryDirectory() as scratch:
        scratch = pathlib.Path(scratch)
        binary = args.govax
        if not binary:
            binary = str(scratch / "govax")
            subprocess.run(["go", "build", "-o", binary, "./cmd/govax"], cwd=ROOT, check=True)
        gv = scratch / "govax-build"
        build_govax(kit, gv, binary, "/NOOPTIMIZE" if args.noopt else "")
        if args.sha:
            for path in args.scripts:
                out, cut = play_govax(binary, gv, pathlib.Path(path).read_text())
                print(path, "never ended" if cut else hashlib.sha256(out.encode()).hexdigest())
            return 0
        gf = scratch / "gfortran-build"
        build_gfortran(kit, gf)
        ok = True
        for path in args.scripts:
            ok = compare(path, gf, gv, binary) and ok
        return 0 if ok else 1


sys.exit(main())
