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
| `kinds.f90` | Fortran 90's kinds (Phase 100, subtask 4): kind selectors on INTEGER, REAL, COMPLEX, LOGICAL, and CHARACTER, with `KIND=` and without, kinds named by PARAMETERs, `LEN=` and `KIND=` in both orders, kinds on constants of every type, a REAL(8) given a REAL's value and a REAL(8) constant's, a REAL(16) product, a function with a kind selector |
| `bytes.f90` | Fortran 90's INTEGER(1) (Phase 100, subtask 5): byte arithmetic, a byte beside INTEGER(2), INTEGER, and REAL, constants of kind 1, the generic functions of bytes (ABS, MAX, MIN, IAND, IOR, IEOR, ISHFT, NOT, SIGN, MOD, DIM) and INT, a power, a DO loop and subscripts of kind 1, an array in DATA, a byte argument and a byte function, and I, O, Z, list-directed, and internal I/O both ways |
| `cplx16.f90` | Fortran 90's COMPLEX(16) (Phase 100, subtask 7): an initial value and constants of kind 16, sums, differences, products, quotients (FTNRTL's FTN$DIVCH), negation, `==` and `/=`, a folded quotient equal to the computed one, one third to 33 digits, conversions from COMPLEX(8), REAL(16), and INTEGER and back, CMPLX with KIND=16, REAL, AIMAG, CONJG, INT, KIND, PRECISION, an array with subscripts, a dummy array, and a COMPLEX(16) function; printed by its parts, its I/O not there yet |
| `inquire.f90` | Fortran 90's kind and inquiry functions (Phase 100, subtask 6): SELECTED_INT_KIND, SELECTED_REAL_KIND, and KIND in PARAMETERs and executable statements; HUGE, TINY, EPSILON, DIGITS, PRECISION, RANGE, RADIX, MINEXPONENT, MAXEXPONENT, and BIT_SIZE of every integer kind, REAL, and a REAL(8) dummy; KIND= on INT, NINT, REAL, CMPLX, and LOGICAL; keyword arguments. The real model's lines are the VAX's, `compare.py`'s KNOWN gfortran's |
| `precise.f90` | A program portable to any Fortran 90 processor (Phase 100, subtask 8): its precision asked for by SELECTED_REAL_KIND and SELECTED_INT_KIND in each unit's PARAMETERs, constants with kinds, KIND= on NINT, INT, REAL, and CMPLX, IMPLICIT with a kind given by SELECTED_REAL_KIND, a kind named by KIND of a dummy, a RESULT variable, INTENT, a function passed as an argument; Newton's square root, Simpson's rule, a loan's payments, a quadratic's complex roots. The same on gfortran with no known difference |
| `forms.f90` | Declarations of every form Phase 100 compiles, in one program (subtask 8): `::` and the old statements side by side, attributes as attributes and as statements (VOLATILE, INTENT, AUTOMATIC, STATIC, OPTIONAL, TARGET, EXTERNAL, INTRINSIC, DIMENSION), DEC's `*n` beside kind selectors and after a name (`REAL(8) :: X*4`), IMPLICIT with kinds, `/clist/`, DATA, and `=`, PARAMETER both ways, COMMON and EQUIVALENCE of names declared with `::`, INTEGER\*1, BYTE, DOUBLE COMPLEX, and COMPLEX(16). DEC's forms keep it from gfortran's `-std=f95`, so it isn't in `compare.py` |
| `modules/mixed` | Both source forms in one program: `mixed.f90` and `report.f90` free, `fill.for` fixed, and `limits.inc`, INCLUDEd by both forms, written to read the same way in either |
