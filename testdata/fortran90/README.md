# The Fortran 90 end-to-end programs

`docs/planning/PHASE-99 - Fortran 90.md`'s Decision 16 and subtask 6: each
`name.f90` is a program in free source form and `name.out` what running it
prints at the terminal, followed by SHOW SYMBOL's line for `$STATUS`. A
program that reads the terminal has `name.in`, the lines typed (not echoed).
`TestFortran90Programs` (`internal/console/fortranprograms_test.go`) runs
them as `TestFortranPrograms` runs `testdata/fortran`'s: each source copied
to a volume as `NAME.F90`, which FORTRAN reads in free form with `/F90` by its
type; FORTRAN, MACRO, and LINK must say nothing; compiled `/NOOPTIMIZE`,
`/OPTIMIZE`, and both again `/RECURSIVE`; with paging off, and on for those
in `fortran90PagingPrograms` (all of them so far).

`modules/` holds programs of several files, a directory each, run by
`TestFortran90Modules` once per line of its `links.txt`, as
`testdata/fortran/modules`'s are. Its sources may be of either form: `.f90`
free, `.for` fixed.

The expected output is worked by hand from the standard (ISO/IEC 1539:1991)
and the *VAX FORTRAN Language Reference Manual* for what FTNRTL prints, never
copied from a run. `testdata/fortran/gfortran/compare.py` is the witness: it
builds each program here with `gfortran -std=f95` and compares its output
with the `.out` file. A difference is read against the standard, and neither
compiler is assumed right. To add a program, add its files here and its row
to `fortran90PagingPrograms` and to `compare.py`'s `PROGRAMS90` if it is
portable; this directory's `.gitignore` keeps `*.out` from the repository's
own rule.

The first programs (Phase 99) are FORTRAN-77, with DEC's END DO and DO WHILE,
in free form with Fortran 90's spellings; each later phase of the program adds
its own.

| Program | What it exercises |
| ------- | ----------------- |
| `spellings.f90` | `==`, `/=`, `<`, `<=`, `>`, `>=` beside the old spellings; character constants between quotation marks or apostrophes, each doubled inside its own kind, in expressions and in FORMAT statements; `!` comments, and a `!` or `&` inside a constant; `;`; a constant, a number, and a statement continued with `&`, over a comment line; B, O, and Z constants in DATA; `END PROGRAM`, `END FUNCTION`, `END SUBROUTINE` with names |
| `squares.f90` | DO ... END DO, DO WHILE, a labeled DO ending at a CONTINUE, a block IF with ELSE IF written with `<`, `==`, and `>=`, labels first on their lines, DOUBLE PRECISION, quoted text in FORMAT statements |
| `strings.f90` | Concatenation, substrings, LEN, INDEX, ICHAR and CHAR, LLT, comparisons written with `<`, `==`, and `/=` of different lengths, passed-length dummies, a logical IF with a substring assignment |
| `layout.f90` | Free form's layout (the standard's 3.3.1): continuation over comment and blank lines, an `&` starting the next line or not, a character constant continued with its blanks, a label after a `;`, a DO loop on one line, GO TO, END DO, ELSE IF, END IF, and DOUBLE PRECISION with their blanks and without, keywords in either case, a line of 132 characters |
| `average.f90` | Terminal input in free form: an A edit and list-directed numbers until a negative one, by DO WHILE (`average.in`) |
| `declare.f90` | Fortran 90's declarations (Phase 100, subtask 3): `::` with PARAMETER, DIMENSION, SAVE, EXTERNAL, and INTENT; initial values given with `=`, one value for a whole array; a subroutine counting its calls in a variable an initialization makes SAVE; DIMENSION `::` and SAVE `::` as statements; CHARACTER\*(\*) PARAMETER |
| `modules/mixed` | Both source forms in one program: `mixed.f90` and `report.f90` free, `fill.for` fixed, and `limits.inc`, INCLUDEd by both forms, written to read the same way in either |
