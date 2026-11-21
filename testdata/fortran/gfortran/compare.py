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
review. Its programs read name.in, if there is one, as their input. A program of several files
names its main program's file first and the rest in the order their modules need; gfortran is
given the rest first, then the main program's.

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
    "moddata": ["moddata.f90"],
    "modprocs": ["modprocs.f90"],
    "modaccess": ["modaccess.f90"],
    "bytes": ["bytes.f90"],
    "keywords": ["keywords.f90"],
    "charfns": ["charfns.f90"],
    "numfns": ["numfns.f90"],
    "model": ["model.f90"],
    "transfer": ["transfer.f90"],
    "clockrand": ["clockrand.f90"],
    "textfmt": ["textfmt.f90"],
    "calendar": ["calendar.f90"],
    "edits": ["edits.f90"],
    "cplx16": ["cplx16.f90"],
    "cplx16io": ["cplx16io.f90"],
    "cparts": ["cparts.f90"],
    "openings": ["openings.f90"],
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
    "where": ["where.f90"],
    "arrayio": ["arrayio.f90"],
    "matrix": ["matrix.f90"],
    "internal": ["internal.f90"],
    "hosts": ["hosts.f90"],
    "recurhost": ["recurhost.f90"],
    "interfaces": ["interfaces.f90"],
    "optional": ["optional.f90"],
    "generics": ["generics.f90"],
    "operators": ["operators.f90"],
    "hostopt": ["hostopt.f90"],
    "numbers": ["numbers.f90"],
    "stats": ["stats.f90"],
    "smooth": ["smooth.f90"],
    "allocs": ["allocs.f90"],
    "pointers": ["pointers.f90"],
    "ptrarrays": ["ptrarrays.f90"],
    "ptralloc": ["ptralloc.f90"],
    "ptrcomps": ["ptrcomps.f90"],
    "ptrcarr": ["ptrcarr.f90"],
    "ptrdum": ["ptrdum.f90"],
    "ptrres": ["ptrres.f90"],
    "ptralias": ["ptralias.f90"],
    "ptrstd": ["ptrstd.f90"],
    "sparse": ["sparse.f90"],
    "allocstat": ["allocstat.f90"],
    "autos": ["autos.f90"],
    "autolen": ["autolen.f90"],
    "rtemps": ["rtemps.f90"],
    "shaped": ["shaped.f90"],
    "shapes": ["shapes.f90"],
    "shapeargs": ["shapeargs.f90"],
    "seqargs": ["seqargs.f90"],
    "values": ["values.f90"],
    "ramps": ["ramps.f90"],
    "madetext": ["madetext.f90"],
    "transform": ["transform.f90"],
    "solver": ["solver.f90"],
    "sieve": ["sieve.f90"],
    "sorter": ["sorter.f90"],
    "structs": ["structs.f90"],
    "results": ["results.f90"],
    "typearrays": ["typearrays.f90"],
    "typevalues": ["typevalues.f90"],
    "typeargs": ["typeargs.f90"],
    "typedyn": ["typedyn.f90"],
    "seqtypes": ["seqtypes.f90"],
    "typeio": ["typeio.f90"],
    "geometry": ["geometry.f90"],
    "stock": ["stock.f90"],
    "accounts": ["accounts.f90"],
    "advance": ["advance.f90"],
    "listnml": ["listnml.f90"],
    "ioitems": ["ioitems.f90"],
    "journal": ["journal.f90"],
    "wordcount": ["wordcount.f90"],
    "gradebook": ["gradebook.f90"],
    "optfuse": ["optfuse.f90"],
    "optstride": ["optstride.f90"],
    "mixed": ["modules/mixed/mixed.f90", "modules/mixed/fill.for", "modules/mixed/report.f90"],
    "standard": ["modules/standard/standard.f90", "modules/standard/globals.f90", "modules/standard/strings.f90",
                 "modules/standard/library.f90", "modules/standard/renames.f90", "modules/standard/users.f90",
                 "modules/standard/work.f90"],
    "polys": ["modules/polys/polys.f90", "modules/polys/kinds.f90", "modules/polys/polynomials.f90",
              "modules/polys/roots.f90", "modules/polys/report.f90", "modules/polys/table.for"],
    "concord": ["modules/concord/concord.f90", "modules/concord/words.f90", "modules/concord/lists.f90",
                "modules/concord/trees.f90"],
}

# Lines gfortran prints differently for a reason, each program's: the .out's line and gfortran's.
KNOWN = {
    "model": {
        # REAL(8) is D_floating, 56 digits, where gfortran's is IEEE's double, 53: a unit in the
        # last place of 3 is 2**-54, not 2**-51 (PHASE-108's finding 7).
        "D spacings:  5.5511151231257827E-17   54043195528445952.0":
            "D spacings:  4.4408920985006262E-16    6755399441055744.0",
        "D NEAREST:  5.5511151231257827E-17": "D NEAREST:  4.4408920985006262E-16",
    },
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
    "ptrres": {
        # A pointer function's value is cleared at entry (PHASE-107's finding 2), where gfortran
        # leaves it undefined; and an assignment whose value is reached through a pointer function
        # goes through a temporary (7.5.1.5), which gfortran leaves out.
        "outside F": "outside T",
        "overlap  1.0  2.0  3.0  2.0  4.0  6.0": "overlap  1.0  2.0  3.0  2.0  2.0  6.0",
    },
    "openings": {
        # POSITION= after the file has moved since its OPEN is the processor's (the standard's
        # 9.6.1.16): govax's is the OPEN's, gfortran's where the file is.
        "Unit 10 WRITE ASIS QUOTE NO NO YES NO": "Unit 10 WRITE APPEND QUOTE NO NO YES NO",
        # PAD= of a unit not connected is YES by Fortran 90's 9.6.1.21 and the HP Fortran
        # manual's 12.5.20; gfortran answers Fortran 2003's UNDEFINED.
        "Unit 12 UNDEFINED UNDEFINED UNDEFINED YES UNKNOWN UNKNOWN UNKNOWN":
            "Unit 12 UNDEFINED UNDEFINED UNDEFINED UNDEFINED UNKNOWN UNKNOWN UNKNOWN",
        # IOLENGTH= is in RECL='s units, longwords on VMS (PHASE-109's finding 13), bytes in
        # gfortran.
        "Scalar and arrays:   61": "Scalar and arrays:  244",
        "Section:   10": "Section:   40",
        "Implied DO:    7": "Implied DO:   28",
        "Allocatable:    8": "Allocatable:   31",
        "Derived type:    6": "Derived type:   24",
        "Passed length:    3": "Passed length:    9",
    },
    "cplx16io": {
        # List-directed and namelist output's layouts are the processor's (the standard's
        # 10.8.2, 10.9.2): govax's are VMS's FORRTL's, G fields with d significant digits, a
        # value on the record after its name when it won't fit there; gfortran's are 36 digits.
        "(1.50000000000000000000000000000000,-2.00000000000000000000000000000000)":
            "               (1.50000000000000000000000000000000000,-2.00000000000000000000000000000000000)",
        "(1.000000000000000000000000000000000E-0100,-1.000000000000000000000000000000000E+0100)":
            " (1.000000000000000000000000000000000035E-0100,-9.999999999999999999999999999999999627E+0099)",
        " (1.000000000000000000000000000000000E-0100,-1.000000000000000000000000000000000E+0100)":
            "  (1.000000000000000000000000000000000035E-0100,-9.999999999999999999999999999999999627E+0099)",
        # Its group's name starts in column 2, after a record's carriage control, where
        # gfortran's starts in column 1 (10.9.2: both read back).
        " &PAIR": "&PAIR",
        " Z       =": [],
        " (1.50000000000000000000000000000000,-2.00000000000000000000000000000000),":
            " Z=(1.50000000000000000000000000000000000,-2.00000000000000000000000000000000000),",
        " W       =": [],
        " (0.250000000000000000000000000000000,1000.00000000000000000000000000000)":
            " W=(0.250000000000000000000000000000000000,1000.00000000000000000000000000000000),",
    },
    "listnml": {
        # A list-directed record's length is the processor's (10.8.2): govax's is VMS's 81
        # bytes, so a delimited constant of 123 goes on in a second record; gfortran's are longer.
        "Records of the long constant: 2": "Records of the long constant: 1",
        # Namelist output's layout is the processor's (10.9.2): govax's is VMS's FORRTL's, a name
        # padded to a column and list-directed output's fields; and its text undelimited without
        # DELIM= (the standard's 10.9.2.1, the author's answer 2), where gfortran quotes it.
        "I       =          42,": "I=42         ,",
        "X       =   1.500000    ,": "X=  1.50000000    ,",
        "NM      = it's": "NM=\"it's\",",
        " NM      = 'it''s'": " NM='it''s',",
    },
    "advance": {
        # IOSTAT='s value for an error is the processor's: govax's is the FOR number, 308 for a
        # bad ADVANCE= (Phase 109's finding 8).
        "A bad ADVANCE=: 308": "A bad ADVANCE=:5002",
        # A list-directed statement on a unit a nonadvancing one left a record open in is the
        # standard's undefined case: govax ends the record first (Phase 109's finding 4),
        # gfortran goes on with it.
        "part": "part list",
        "list": [],
        # An advancing READ that needs more than a PAD='NO' record holds breaks a rule of the
        # standard's (9.4.4.4.2): govax's error is RECTOOSHO, FOR number 310; gfortran's is an
        # end of record, Fortran 2003's rule. Then the nonadvancing READ's SIZE=: govax counts
        # the characters A5 took from the record, gfortran none when it can't pad.
        "A short record, PAD='NO': 310": "A short record, PAD='NO':  -2",
        "  2  -2": "  0  -2",
        # A record left open at the terminal is written as a prompt before the terminal is read
        # (Phase 109's answer 4); gfortran keeps it until the next record, the PRINT's, ends it.
        "Your name? Hello, Pat": "Your name?  Hello, Pat",
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
        # The main program's file is named first, for its .in and .out; the
        # others follow in the order their modules need, and gfortran is given
        # them first, so each module is compiled before a USE reads it.
        sources = [str(CORPUS90 / file) for file in [*files[1:], files[0]]]
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
    # A known difference is a line, or a list of lines (none, for a line gfortran doesn't print).
    want = [new for line in want for new in (lambda k: k if isinstance(k, list) else [k])(known.get(line, line))]
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
