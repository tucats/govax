#!/usr/bin/env python3
"""Run testdata/fortran's portable recursive programs, and testdata/fortran90's programs, on
gfortran, and compare what they print with their .out files (docs/planning/PHASE-89 - reentrant
FORTRAN.md, subtask 11; docs/planning/PHASE-99 - Fortran 90.md, subtask 6 and Decision 16).

The .out files are worked by hand from the language's rules, and govax is held to them by
TestFortranPrograms and TestFortranModules. gfortran is a second opinion on the hand-worked
outputs, never the reference: a line it prints differently is a question to look into, and the
answers so far are in KNOWN below.

    compare.py [--keep DIR] [program ...]      every program below if none is named

What's changed in gfortran's copy only says in its dialect what VAX FORTRAN says in its own:

  - A recursive function or ENTRY without RESULT gets one, NAME_R: gfortran refuses a function's
    own name as a call (it reads it as the result, Fortran 95's rule), where govax reads it with an
    argument list as a call (Fortran 90's rule for a scalar function, the phase's answer 2). In
    the unit, the name followed by ( is then a call and any other use the result variable.
  - OPTIONS /RECURSIVE is taken out, and every file compiled -frecursive.

The Fortran 90 corpus (PROGRAMS90) is given to gfortran as it is, each file in the form its type
says, with -std=f95: gfortran has no -std=f90, and the corpus keeps Fortran 95's additions out by
review. Its programs read name.in, if there is one, as their input.

What's normalized is only FORTRAN's carriage control: gfortran prints column 1 as text, and VMS
acts on it; and the .out files' last line, SHOW SYMBOL's $STATUS.
"""
import argparse
import difflib
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

HERE = pathlib.Path(__file__).resolve().parent
CORPUS = HERE.parent
CORPUS90 = CORPUS.parent / "fortran90"
FLAGS = "-std=legacy -fdec -frecursive -fno-range-check -w -O0".split()
FLAGS90 = "-std=f95 -O0".split()

# Each program: its source files, relative to testdata/fortran, the main one first.
PROGRAMS = {
    "recursion": ["recursion.for"],
    "optrecur": ["optrecur.for"],
    "recurtree": ["recurtree.for"],
    "recurcalc": ["recurcalc.for"],
    "mutual": ["modules/mutual/mutual.for", "modules/mutual/fem.for", "modules/mutual/male.for"],
}

# The Fortran 90 corpus's programs: their files, relative to testdata/fortran90, the main one
# first, its directory searched for INCLUDE files.
PROGRAMS90 = {
    "bytes": ["bytes.f90"],
    "cplx16": ["cplx16.f90"],
    "declare": ["declare.f90"],
    "inquire": ["inquire.f90"],
    "kinds": ["kinds.f90"],
    "precise": ["precise.f90"],
    "spellings": ["spellings.f90"],
    "squares": ["squares.f90"],
    "strings": ["strings.f90"],
    "layout": ["layout.f90"],
    "average": ["average.f90"],
    "selects": ["selects.f90"],
    "tokens": ["tokens.f90"],
    "loops": ["loops.f90"],
    "cases": ["cases.f90"],
    "examples": ["examples.f90"],
    "arrays": ["arrays.f90"],
    "overlap": ["overlap.f90"],
    "vectors": ["vectors.f90"],
    "constants": ["constants.f90"],
    "elements": ["elements.f90"],
    "reduce": ["reduce.f90"],
    "mixed": ["modules/mixed/mixed.f90", "modules/mixed/fill.for", "modules/mixed/report.f90"],
}

# Lines gfortran prints differently for a reason, each program's: the .out's line and gfortran's.
KNOWN = {
    "inquire": {
        # The VAX's F, D, and H formats' models, where gfortran's are IEEE's: F's exponent
        # field biased by 128 with no denormals, and REAL(8) D_floating, whose range is F's,
        # so 15 digits and a range of 307 need REAL(16), H (Phase 100's finding 9).
        "Real range:  38": "Real range:  37",
        "Exponents: -127  127": "Exponents: -125  128",
        "HUGE:  0.1701412E+39": "HUGE:  0.3402823E+39",
        "TINY:  0.2938736E-38": "TINY:  0.1175494E-37",
        "Double:  56 16  38": "Double:  53 15 307",
        "Its EPSILON:  0.2775558E-16": "Its EPSILON:  0.2220446E-15",
        "For 15 digits and 307: 16": "For 15 digits and 307:  8",
    },
    "recursion": {
        # IEEE's negative zero, which the VAX's floating formats don't have.
        "I**7: ( 0.0, -1.0)": "I**7: (-0.0, -1.0)",
    },
}

HEADER = re.compile(r"^(\s+)((?:RECURSIVE\s+)?(?:(?:INTEGER|REAL|LOGICAL|DOUBLE\s*PRECISION|COMPLEX|"
                    r"CHARACTER)(?:\*\s*(?:\d+|\(\*\)))?\s+)?(?:RECURSIVE\s+)?)FUNCTION\s+(\w+)\s*(\([^)]*\))\s*$",
                    re.IGNORECASE)
ENTRY = re.compile(r"^(\s+ENTRY\s+)(\w+)\s*(\([^)]*\))?\s*$", re.IGNORECASE)


def units(lines):
    """Splits a source's lines into units, each ending at a line that is END alone."""
    unit = []
    for line in lines:
        unit.append(line)
        if not line[:1].upper() in ("C", "*", "!") and line.strip().upper() == "END":
            yield unit
            unit = []
    if unit:
        yield unit


def with_results(text, every):
    """The source, each recursive function given RESULT (the docstring says why); with every,
    each function is recursive, as OPTIONS /RECURSIVE said."""
    out = []
    for unit in units(text.splitlines()):
        head = next((i for i, line in enumerate(unit) if HEADER.match(line)), None)
        if head is None or not (every or "RECURSIVE" in unit[head].upper()) or "RESULT" in unit[head].upper():
            out.extend(unit)
            continue
        names = {}
        m = HEADER.match(unit[head])
        names[m.group(3).upper()] = m.group(3).upper() + "_R"
        unit[head] = f"{m.group(1)}{m.group(2)}FUNCTION {m.group(3)}{m.group(4)} RESULT({m.group(3)}_R)"
        for i, line in enumerate(unit):
            e = ENTRY.match(line)
            if e:
                names[e.group(2).upper()] = e.group(2).upper() + "_R"
                unit[i] = f"{e.group(1)}{e.group(2)}{e.group(3) or '()'} RESULT({e.group(2)}_R)"
        for i, line in enumerate(unit):
            if i == head or ENTRY.match(line) or line[:1].upper() in ("C", "*", "!"):
                continue
            for name, result in names.items():
                # The name not followed by ( is the result; inside quotes it's text.
                parts = re.split(r"('[^']*')", line)
                for k in range(0, len(parts), 2):
                    parts[k] = re.sub(rf"\b{name}\b(?!\s*\()", result, parts[k], flags=re.IGNORECASE)
                line = "".join(parts)
            unit[i] = line
        out.extend(unit)
    return "\n".join(out) + "\n"


def gfortran_copy(source):
    """The text gfortran gets for one source file."""
    lines = source.read_text().splitlines()
    kept = [line for line in lines if line.strip().upper() != "OPTIONS /RECURSIVE"]
    return with_results("\n".join(kept), len(kept) != len(lines))


def expected(out):
    """What the program prints, from its .out file, less the $STATUS line."""
    return [line for line in out.read_text().splitlines() if not line.startswith("  $STATUS ==")]


def printed(stdout):
    """gfortran's output with column 1 taken as VMS takes a blank there."""
    return [line[1:] if line.startswith(" ") else line for line in stdout.splitlines()]


def compare(name, work):
    """Builds one program on gfortran in work, runs it, and compares its output with the .out's."""
    work.mkdir()
    if name in PROGRAMS90:
        files = PROGRAMS90[name]
        main = CORPUS90 / files[0]
        sources = [str(CORPUS90 / file) for file in files]
        flags = [*FLAGS90, f"-I{main.parent}"]
    else:
        files = PROGRAMS[name]
        main = CORPUS / files[0]
        sources = []
        for i, file in enumerate(files):
            target = work / f"{i}_{pathlib.Path(file).stem}.f"
            target.write_text(gfortran_copy(CORPUS / file))
            sources.append(target.name)
        flags = FLAGS
    env = dict(os.environ)
    if sys.platform == "darwin" and "SDKROOT" not in env:
        env["SDKROOT"] = subprocess.run(["xcrun", "--show-sdk-path"], capture_output=True, text=True,
                                        check=True).stdout.strip()
    build = subprocess.run(["gfortran", *flags, "-o", name, *sources], cwd=work, env=env,
                           capture_output=True, text=True)
    if build.returncode != 0:
        print(f"{name}: gfortran failed:\n{build.stderr}")
        return False
    typed = main.with_suffix(".in")
    stdin = typed.read_text() if typed.exists() else ""
    run = subprocess.run([str(work / name)], cwd=work, input=stdin, capture_output=True, text=True,
                         timeout=60)
    got = printed(run.stdout)
    want = expected(main.with_suffix(".out"))
    known = KNOWN.get(name, {})
    want = [known.get(line, line) for line in want]
    if got == want:
        print(f"{name}: the same ({len(want)} lines" + (f", {len(known)} known difference)" if known else ")"))
        return True
    print(f"{name}: differs")
    sys.stdout.writelines(line + "\n" for line in difflib.unified_diff(want, got, "expected", "gfortran", lineterm=""))
    return False


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--keep", help="a directory to leave gfortran's copies and programs in")
    parser.add_argument("programs", nargs="*", default=[*PROGRAMS, *PROGRAMS90])
    args = parser.parse_args()
    if shutil.which("gfortran") is None:
        sys.exit("no gfortran on the PATH")
    root = pathlib.Path(args.keep) if args.keep else pathlib.Path(tempfile.mkdtemp())
    root.mkdir(parents=True, exist_ok=True)
    ok = True
    try:
        for name in args.programs:
            ok &= compare(name, root / name)
    finally:
        if not args.keep:
            shutil.rmtree(root)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
