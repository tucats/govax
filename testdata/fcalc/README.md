# fcalc, a scientific calculator in Fortran 90

The Fortran 90 program's milestone (`docs/planning/PHASE-111 - Fortran 90
milestone.md`; Part A is its design). fcalc reads lines at a `fcalc> ` prompt:
an expression's value is shown and kept as `ans`, `name = expr` assigns a
variable, and a line starting with a command word is a command. Every value is
complex, `COMPLEX(8)`, or a vector or matrix of complex values: `[1, 2; 3, 4]`.
A line ending in `;` is quiet, as in MATLAB and Octave: the value is kept, not
shown. SET FORMAT, SET DIGITS, and SET BASE choose how values are shown
(AUTO, FIXED, SCIENTIFIC, or ENGINEERING; whole numbers in hexadecimal, octal,
or binary as `0xFF`, `0o377`, `0b11111111`, which fcalc also reads), and SHOW
SETTINGS lists every setting as the SET command that makes it. TABLE shows
expressions' values in columns, and PLOT draws them as a graph of characters.
SAVE writes the settings, variables, and functions to a file of fcalc's own
commands, which LOAD runs again; `@file` (or RUN) runs any such file, and LOG
keeps a transcript. Run as a foreign command, `$ FCALC 2^10 + 1`, fcalc runs
that one line and ends. HELP shows the FCALC topic of the system's help
library (`internal/bootdata/files/help/fcalc.hlp`), the text DCL's HELP FCALC
shows.

Since Phase 111's subtask 12 fcalc is also a tool govax ships: its sources
are in `internal/fcalc/src` (the package embeds them, with BUILDFCALC.COM and
the committed image, `internal/fcalc/fcalc.exe`, for the system volume), and
this directory holds the scripts and what they print.

## The sources

Each file is a module but the last, the main program, compiled in the order
`internal/fcalc/src/compiles.txt` gives, a FORTRAN command each:

| File | What it holds |
|---|---|
| `fcplatform.f90` | what differs by system: VMS's carriage control and record length, whether a READ ends a prompt's record (SYS$INPUT a terminal, from LIB$GETDVI), the foreign command's text, from LIB$GET_FOREIGN, and HELP, by LBR$OUTPUT_HELP (`gfortran/fcplatform.f90` is gfortran's twin, which has neither) |
| `fckinds.f90` | the kind `RK`, fcalc's limits (`LIMIT`, `SMALL`), the constants |
| `fcmessages.f90` | output (`SAY`, which keeps records to 132 characters, and `HELP_LINE`, which the librarian calls with each line of help) and the messages, `%FCALC-E-...` |
| `fcvalues.f90` | `TYPE VALUE`, its operators, guarded so nothing traps, and how a value is shown (the formats and bases) |
| `fcarrays.f90` | `TYPE QUANTITY`, a scalar or an array through a pointer, its ownership rule, and how an array is shown |
| `fclexer.f90` | the tokens, numbers in other bases among them |
| `fctree.f90` | `TYPE NODE`, a tree of pointers, and the recursive descent parser |
| `fcsymbols.f90` | the variables and the user's functions, each a binary search tree, and the constants |
| `fcfuncs.f90` | the built-in functions: their table, and each function, real and complex, guarded so nothing traps |
| `fcpoly.f90` | polynomials as vectors of coefficients: roots (Durand-Kerner, polished by Newton's method), poly, polyval |
| `fcmatrix.f90` | the matrix product, LU (solve, inverse, determinant), powers, rank, norms, eigenvalues (Hessenberg and shifted QR), and the functions that make and rearrange arrays |
| `fceval.f90` | the evaluator: element-by-element operations, matrix literals, indexes, and a frame of parameters for each call of a user's function |
| `fcnumeric.f90` | the numerical commands: SOLVE (the secant method, or Brent's on a bracket), INTEGRATE (adaptive Gauss-Kronrod quadrature), DIFF (Ridders' extrapolation), and ROOTS |
| `fcplot.f90` | TABLE and PLOT: values at points or vectors side by side, and a graph's grid, axes, labels, and legend |
| `fcstate.f90` | the session's state and files: the settings' SET lines, SAVE (each number with the fewest digits that read back the same), reading files of commands a line at a time, and LOG |
| `fcalc.f90` | the main program: the foreign command's line, the prompt, the commands (SET, SHOW, and HELP among them), @ and LOAD running a file's lines, assignment, function definitions |

The sources are commented for a reader learning Fortran 90 and numerical
programming: each Fortran 90 feature is explained where it is first used,
and each numerical method where it is applied (guarding against overflow
and underflow, principal branches, cancellation, series and continued
fractions, Lanczos's gamma function, LU with partial pivoting, Householder
reflections and the shifted QR iteration, the Durand-Kerner method, the
secant and Brent's methods, Gauss-Kronrod quadrature, and Richardson's
extrapolation).

fcalc must be compiled `/G_FLOATING`: no kind of D_floating has the range of
300 `SELECTED_REAL_KIND(15, 300)` asks for. On govax, on a volume holding the
sources as `NAME.F90` (as `BUILDFCALC.COM` does, on the system volume):

```
$ FORTRAN/G_FLOATING FCPLATFORM
$ FORTRAN/G_FLOATING FCKINDS
  ... each file in compiles.txt's order ...
$ MACRO FCPLATFORM
  ... each file ...
$ LINK FCALC,FCPLATFORM,FCKINDS,FCMESSAGES,FCVALUES,FCARRAYS,FCLEXER,FCTREE,-
       FCSYMBOLS,FCFUNCS,FCPOLY,FCMATRIX,FCEVAL,FCNUMERIC,FCPLOT,FCSTATE
$ RUN FCALC
```

On gfortran, in `internal/fcalc/src`, with `gfortran/fcplatform.f90` in place of `fcplatform.f90`, in
the same order: `gfortran -std=f95 gfortran/fcplatform.f90 fckinds.f90 ...
fcalc.f90` (on macOS, if the link can't find library `System`, set
`SDKROOT=$(xcrun --show-sdk-path)` first). The sources keep to Fortran 90: nothing Fortran 95 added (`I0`,
`NULL()`, default component values, `ELEMENTAL`, `PURE`, `FORALL`).

## The scripts

Each `name.in` is what a user types, and `name.out` what fcalc prints, worked by
hand from the mathematics and fcalc's rules (Part A's A.1 and A.3), then
compared with govax's and gfortran's runs, which print the same lines. Each
script starts with `set echo`, so the transcript shows each line after its
prompt, as a terminal does.

| Script | What it covers |
|---|---|
| `basics` | numbers and how they are shown, every operator and its precedence, the constants, variables and `ans`, LIST, CLEAR, SET [NO]ECHO, comments, and the messages |
| `complex` | complex values: their display, arithmetic, comparisons, and powers |
| `functions` | every built-in function: roots, exponentials, logarithms, trigonometry in each angle mode (SET ANGLE), hyperbolic functions, complex parts, rounding, whole numbers, the special functions, lists, IF, and random numbers (only their properties), with their errors |
| `userfuncs` | functions the user defines: parameters and variables, calls between functions, recursion and its limit, redefinition, LIST and CLEAR, and the errors of definitions and calls |
| `vectors` | vectors: literals and joining, the functions that make them, indexes (ranges, vector subscripts), assigning elements, element-by-element operations and functions, list functions of arrays, sizes, reshape, cumsum, sort, dot, cross, norms, transposes, users' functions of vectors, long vectors' display and LIST, and the errors |
| `matrices` | matrices: literals and blocks, sections, element-by-element and matrix products, transposes, powers, determinants, inverses, solves, trace, rank, norms, kron, eigenvalues (SET DIGITS 10 for some), the display beside the largest element, guarded products out of the safe band, users' functions of matrices, and the errors |
| `numeric` | the numerical commands: SOLVE from a point (real and complex roots) and on a bracket, with = and users' functions, the variable after FOR left alone; INTEGRATE of smooth functions, a jump, singular ends, and one whose pieces run out (the warning INACCURATE); DIFF at real and complex points, and a jump's warning; ROOTS of real and complex coefficients, roots at zero, a double root (SET DIGITS 6), vectors of coefficients, roots(), poly, polyval; and each command's errors |
| `display` | how values are shown: SHOW SETTINGS and each SHOW, SET DIGITS, each SET FORMAT (rounding, padding, the exponent a multiple of 3, FIXED's places and its turn to scientific notation past 15 digits), SET BASE (each base, negative and complex whole numbers, matrices, LIST, the literals typed in, and their errors), SHOW ANGLE, SET SEED (the same seed repeating a sequence, a quiet `;` line keeping a random value unshown), and SET's and SHOW's errors |
| `state` | files of commands: @ and RUN of a file written by hand (`files/setup.fcalc`), RUN, LOAD, SAVE, and LOG as names when = or ( follows; SAVE of every kind of value and setting, the file shown by running it with echo on, LOAD clearing first and restoring each value exactly and the seed given, a name with a type of its own; LOG twice to one file; an error two files deep, EXIT and a warning in a file, a LOAD that fails |
| `errors` | every message, in the order of fcalc's table, but FILEERR (the oneshot procedure's), NOCONVERGE (which no matrix tried has given), and NOHELP (no help library, which only gfortran's build has, and no script runs HELP, its text being the help library's, which `internal/console`'s `TestFcalcHelpExamples` checks): eight files running each other (`files/deep1.fcalc` to `deep8.fcalc`) and a file running itself (`files/self.fcalc`) among them |
| `plot` | TABLE: values at points with and without STEP, either way, a single point, a point that is 0, the variable after FOR left alone, a point's error shown by name, users' functions, the display's settings, vectors side by side, and the errors; PLOT: graphs of one and two curves, the axes, the default size, Y's range and the values outside it, points that fail or aren't real, constants, vectors against their index and VERSUS another, labels by the display's settings, and the errors |

The files the scripts run are in `files/`, copied beside fcalc; the files
`state` writes, SAVE's and LOG's, are held to `expected/`, the date and time
in each heading written `YYYY-MM-DD hh:mm:ss` (and in the scripts' output too,
where the state script shows the saved file). `oneshot.com` is a DCL procedure,
for govax and VMS only (gfortran's fcalc never has a command line): FCALC as
a foreign command with a line, with a line that fails, with `@setup`, with a
SAVE that can't be made (FILEERR), and with no line, reading the procedure's
data lines; `oneshot.out` is what it prints, and `$STATUS` after it.

The `functions` values were checked against Python's `math` and `cmath` as a
second method; the `vectors` and `matrices` values by hand and in Python
(characteristic polynomials' roots for the eigenvalues, exact fractions for
the inverses); and the `numeric` values in Python (Newton's method for the
roots, `erf` for the Gaussian integral, a model of Ridders' table for the
warning's derivative). The `display` values follow from the rules above,
by hand. The `plot` tables follow from the values, and each graph from its
samples by the rule in `fcplot.f90`'s header (a mark in the nearest row and
column, by NINT); the graphs were checked against a Python model of that rule,
which also showed that no sample lies within a millionth of a row of halfway
between two rows, or on a Y range's edge, unless its value is computed exactly,
as 0 is: elsewhere the last bits of the two machines could
choose different rows. `vectors`, `matrices`, `numeric`, `display`, and `plot`
each end with
`SHOW MEMORY` after `CLEAR ALL`, which must count no arrays and no tree nodes
in use: a leak, of an operation's operand or after an error, shows there.

`internal/console`'s `TestFcalcMilestone` builds fcalc in every mode
(`/NOOPTIMIZE` and `/OPTIMIZE`, each also `/RECURSIVE`, with paging off and on;
then `/CHECK=ALL` and `/NOOPTIMIZE/DEBUG=ALL`), and in each runs every script,
checks the files written, and runs `oneshot.com`;
`go test -run TestFcalcMilestone/nopaging/opt ./internal/console` is the usual
build alone. `internal/dclcmd`'s `TestFcalcCompiles` compiles it, quickly,
without a console.

## The compiler's tools on fcalc

`internal/dclcmd`'s `TestFcalcTools` compiles every module
`/LIST/CROSS_REFERENCE`, which must say nothing, then `/STANDARD=F90` and
`/WARNINGS=ALL`, which must say only these, each left for its reason:

- **`/STANDARD=F90`**: six `EXTENSION`s in `fcplatform.f90`, LIB$GETDVI's,
  LIB$GET_FOREIGN's, and LBR$OUTPUT_HELP's `$`s and their null arguments. That module is where fcalc
  keeps what only VMS has.
- **`/WARNINGS=ALL`**: 13 `HOSTHIDDEN`s, each an internal procedure
  declaring a local (`K`, `E`, `H`, `V`, `WIDTH`, `STATUS`) whose name its host
  also uses, so as not to change the host's; and one `UNREAD`, `READ_LINE`'s
  `REST`, which takes the rest of an overlong line only to drop it.

`fcalcd.com` builds fcalc `/DEBUG` (FORTRAN `/NOOPTIMIZE/DEBUG=ALL`,
MACRO/DEBUG, LINK/DEBUG) and runs it on `fact(3)` with DBG$INPUT naming
`fcalcd.dbg`. That session breaks at each level of a user's function's
recursion (`FCEVAL.CALL_USER`), walks the expression trees through their
pointers (`FN%BODY%KID%KID%NAME`, `P%KID%NEXT` not associated), reads the
caller's frame and a module's variable, and shows the calls, and it logs itself.
`internal/console`'s `TestFcalcDebugger` holds the session's log to `fcalcd.dlg`
and the procedure's to `fcalcd.out`, with SHOW CALLS's numbers, the MACRO line
under a break, and its FORTRAN line number masked, since any change to fcalc or
the compiler moves them (`FCALCD_WRITE=1` writes the files again). In the
debugger a compiled program's lines are its MACRO file's, so SHOW CALLS's line
column is the MACRO line, as `%LINE n` and TYPE take it. A break shows the
FORTRAN line as well.

## gfortran's witness

`testdata/fortran/gfortran/compare.py fcalc` builds fcalc on gfortran from
`internal/fcalc/src/compiles.txt`'s files, `gfortran/fcplatform.f90` in VMS's module's place, and
runs every script in one directory holding `files/`, in the order of their
names, as `TestFcalcMilestone` does; each script's output and each file in
`expected/` must be the same line for line, the times in a saved file's heading
masked. It builds fcalc three ways, and each prints the same lines:

- **as the corpus is**, `-std=f95 -O0`;
- **optimized**, `-O2`, where on an arm64 host gfortran fuses multiplies and
  adds and so changes the last bits of many results, which fcalc's output must
  not show (the rounding rules below);
- **checked**: `-fcheck=all` (bounds, pointers, recursion, and the rest),
  traps on invalid operations, division by zero, overflow, and underflow (what
  a VAX traps, underflow as govax's `/CHECK=UNDERFLOW` does), and every real
  starting as a signaling NaN and every integer as -77777, so that a value
  used before it is set, or an operation fcalc's guards should have stopped,
  ends the run instead of passing.

With no program named, `compare.py` runs the corpus's programs and fcalc.
gfortran's `-Wall -Wextra` warns only of fcalc's exact comparisons of reals,
each deliberate (against zero, a whole number's test, SAVE's check that a
number reads back the same), of the Gauss-Kronrod loop's `I / 2`, which
`MOD(I, 2) == 0` keeps from indexing at 0, and of the twin's `SHOW_HELP`'s two
unused arguments (it has no help to show).

## What the two systems make fcalc do

- **Carriage control.** VMS treats a terminal record's first character as
  carriage control; gfortran prints it. `fcplatform.f90`'s `CC` is a blank on
  VMS and nothing on gfortran, and every line fcalc writes starts with it.
- **Record lengths.** VMS's FORTRAN writes terminal records of up to 133 bytes
  and reads lines of up to 199 characters. fcalc writes a longer line in records
  of 132 characters, and takes lines of up to 199 (`LINELONG` past that); a
  too-long line isn't echoed, since VMS gives none of it.
- **Arithmetic.** A VAX traps where IEEE makes an infinity, and its range is
  smaller: no value fcalc keeps is larger than 1e300 or, but for zero, smaller
  than 1e-300, and each operation checks its operands first (`fcvalues.f90`).
  Built under `/CHECK=UNDERFLOW` (in `/CHECK=ALL`), govax faults on floating
  underflow too, so no operation may compute a value below the VAX's smallest
  either; fcalc arranges its comparisons so none does.
- **The command line.** Fortran 90 has no way to read the text after a
  program's name. On VMS, LIB$GET_FOREIGN gives it (`fcplatform.f90`, with a
  null argument for the prompt it isn't to give); gfortran's twin gives none.
- **Files.** fcalc gives a file with no type of its own `.fcalc` itself
  (`state` is `state.fcalc` on both systems; VAX FORTRAN would otherwise give
  it `.DAT`), and asks INQUIRE's OPENED= whether a file is already running,
  which each system answers however the name is spelled.
- **Prompts.** FTNRTL ends a prompt's open record at a READ only from a
  terminal; with its input from a file or a procedure's data lines the record
  stays open, and the next line written goes on in it, as Fortran 90 says it
  should (gfortran does the same, reading anything). So `SAY` writes such a
  line without its carriage control's blank, the record having one already,
  and fits its first piece in the 125 characters the prompt left; it asks
  `fcplatform.f90`'s `prompt_ends`, which asks LIB$GETDVI whether SYS$INPUT
  is a terminal. Until Phase 111's subtask 11 it didn't, and printed
  `fcalc>  set echo` in `oneshot.out`, and a long typed line read from a file
  overflowed its record (`testdata/f90r`'s README).
- **Rounding.** Values are shown to 12 significant digits, where the two
  machines' last bits don't reach; a part of a complex value smaller than the
  other by more than the digits shown is shown as zero, and equality is to
  within 64 epsilons, so `0.1 + 0.2 == 0.3`. An array's elements are shown
  beside its largest, so the noise a computation leaves (`1e-17` beside `1`
  in `a * inv(a)`) is shown as zero on both; eigenvalues are sorted with
  real parts that differ only that much counted the same, so their order
  doesn't depend on either machine's last bits either.
  Under SET FORMAT and fewer digits the rounding itself may differ: a
  decimal tie (12345 to four digits) rounds half to even under gfortran and
  may round half away from zero on a VAX, so the `display` script has no
  exact tie.
