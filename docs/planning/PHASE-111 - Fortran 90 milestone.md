# Phase 111 — Fortran 90: the milestone and close-out

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Subtask 1, the survey and the milestone program's design (Part A),
done 2026-11-20 and accepted by the author the same day ("The author's
answers"; A.8 adds vectors and matrices, A.6 the subtasks). Needs every phase from 99 to 110.

## Goal

Fortran 90 shown working on substantial programs, and the program documented:

- **a milestone program**, by Phase 99's question 7: one the author supplies, or
  a set of our own that uses every phase together (a module-based linked list and
  tree with pointers and derived types, a matrix library with assumed-shape
  arrays and generic interfaces, a text processor with the character intrinsics
  and nonadvancing I/O, a simulation with allocatable arrays and WHERE);
- compiled and run in **every mode** (`/OPTIMIZE` both ways, `/RECURSIVE` both
  ways, both paging modes, `/CHECK=ALL`, `/DEBUG`), printing the same output;
- **gfortran's witness**: the same sources built by gfortran, every line of output
  compared (`compare.py`, extended), differences explained against the standard;
- **the old program unchanged**: DUNGEON and the FORTRAN-77 corpus give the same
  MACRO files under `/NOF90` as before Phase 99 (`TestNoF90Unchanged`), and play
  the same games;
- **the VMS round** (no longer optional: the author offered the simh VAX
  on 2026-11-20): VMS 7.1's MACRO and LINK build FTNRTL and the
  Fortran 90 corpus from govax's `.MAR` files and run them (`testdata/ftn86`'s
  pattern, a new `testdata/f90r`);
- **documentation finished**: MANUAL.md's 14.13 as a finished section (with
  "What Fortran 90 isn't here" and the differences in 14.7), FORTRAN-INTERNALS.md's
  Fortran 90 sections finished and a revised map, HELP FORTRAN, PLAN.md's summary,
  CLAUDE.md.

## Subtasks (provisional; Part A's A.6 revises them)

1. **The milestone program compiled**, whatever it finds fixed in subtasks of its
   own.
2. **Run in every mode**, the milestone test.
3. **gfortran's witness.**
4. **The VMS round** (optional).
5. **The manual's section.**
6. **FORTRAN-INTERNALS.md finished.**
7. **Close-out**: PLAN.md, CLAUDE.md, the follow-up list ("Where things stand"),
   and Phase 112's go or no-go (Phase 99's question 2).

# Part A — the survey (subtask 1)

Phase 99's question 7 asked for a milestone program; the author found none to
use and asked (2026-11-20) for one of our own: **fcalc**, a scientific
calculator at a prompt, which is to be both the Fortran 90 program's test and
a tool a govax user would keep. This part designs it: what the user sees
(A.1), how the program is built and which phase each piece exercises (A.2),
the numerical rules that let govax, gfortran, and a real VAX print the same
lines (A.3), how it is tested (A.4), how it ships as a tool (A.5), the
subtasks revised (A.6), and the questions for the author (A.7).

## A.1 fcalc as the user sees it

fcalc reads lines at a `fcalc> ` prompt (nonadvancing output, so the cursor
stays on the prompt's line) until `QUIT`, `EXIT`, or end of file. Keywords
and names are case-insensitive, as DCL's are. A line is a command if it
starts with one of the command words below; otherwise it is an **expression**,
whose value is printed and kept as `ans`.

**Values.** Every value is complex (COMPLEX(8)); one with no imaginary part
prints as a real number, and one that is also a whole number of moderate size
prints with no decimal point. Literals: `42`, `1.5e-3`, `2i`, `.5`. Built-in
constants: `pi`, `e`, `i`, `ans`, `eps` (the machine epsilon), `huge`.

**Operators**, loosest first: `or`, `and`, `not`; `==` `<>` `<` `<=` `>`
`>=` (real parts compared; true is 1, false 0); `+` `-`; `*` `/` `mod`;
unary `-` and `+`; `^` (right-associative); postfix `!` (factorial; the gamma
function off the integers). Parentheses group.

**Built-in functions** (a table, so adding one is one row):

- elementary: `sqrt cbrt exp ln log10 log2 log(x,b)` (principal branches
  for negative and complex arguments);
- trigonometric and inverse, in the angle mode: `sin cos tan sec csc cot
  asin acos atan atan2(y,x)`; hyperbolic and inverse: `sinh cosh tanh asinh
  acosh atanh`;
- complex: `re im abs arg conj polar(r,t) cis(t)`;
- rounding and integers: `floor ceil round trunc frac sign gcd lcm isprime`,
  `ncr(n,k) npr(n,k)`;
- special: `gamma lgamma erf erfc beta(a,b)` (written in fcalc: Fortran 90
  has none);
- lists, of any number of arguments: `min max sum prod mean median var stdev`;
- conditional, its arms evaluated only when chosen: `if(c, a, b)`;
- `rand()` and `rand(a,b)`, from RANDOM_NUMBER, seeded by `SET SEED`.

**Variables and functions.**

```
fcalc> r = 2.5                      assignment; prints r = 2.5
fcalc> area = pi*r^2
fcalc> f(x) = x^3 - 2*x - 5         a function of up to 8 parameters
fcalc> g(x, y) = sqrt(x^2 + y^2)
fcalc> f(2.1)
```

A function is kept as its parsed tree and its text; it may call other
functions and itself (`fact(n) = if(n<2, 1, n*fact(n-1))`), to a depth limit
fcalc reports.

**Commands:**

| Command | What it does |
|---|---|
| `SOLVE expr [= expr] FOR x [FROM a [TO b]]` | a root: the secant method from a, or bisection then Brent's method on a bracket |
| `INTEGRATE expr FOR x FROM a TO b` | adaptive Simpson's rule, with an error estimate |
| `DIFF expr FOR x AT a` | the derivative by Richardson extrapolation |
| `ROOTS c0, c1, ..., cn` | every (complex) root of a polynomial, by Durand–Kerner, polished |
| `TABLE expr[, expr...] FOR x FROM a TO b [STEP s]` | a table of values in columns |
| `PLOT expr[, expr...] FOR x FROM a TO b [SIZE w,h] [Y lo,hi]` | an ASCII graph: axes, tick labels, a mark per curve, a legend |
| `LIST [VARIABLES / FUNCTIONS / ALL]` | what is defined, in name order |
| `CLEAR name... / CLEAR ALL` | forget names |
| `SET DIGITS n`, `SET FORMAT AUTO/FIXED/SCIENTIFIC/ENGINEERING`, `SET ANGLE RADIANS/DEGREES/GRADS`, `SET BASE DECIMAL/HEX/OCTAL/BINARY`, `SET SEED n`, `SET [NO]ECHO` | the display and the settings; `SHOW SETTINGS` |
| `SAVE file`, `LOAD file` | the state (settings, variables, functions) as a file of fcalc commands, which LOAD runs; a user can edit it |
| `@file` (or `RUN file`) | run a file of commands, nesting to a limit |
| `LOG file / LOG OFF` | a transcript of the session, appended |
| `HELP [topic]` | help text built into the program |

**Errors** name the problem and point at it: the line again with a `^`
under the column (`%FCALC-E-DIVZERO, division by zero`), in VMS's message
form. No input stops fcalc; nothing it computes may trap (A.3).

**One-shot use.** `$ FCALC == "$SYS$SYSTEM:FCALC"` and `$ FCALC 2^10+1`
prints the value and exits, through LIB$GET_FOREIGN (A.5); with no text, the
prompt. `DEFINE/USER SYS$INPUT script` runs a script without prompts' noise.

## A.2 The program and what each part exercises

About 4,000 to 5,000 lines in nine modules and a main program, each file a
module, compiled in the order of their USEs (and in the others
`compiles.txt` allows):

| File | Contents | Exercises (phase) |
|---|---|---|
| `fckinds.f90` | kinds by SELECTED_REAL_KIND, constants, limits from HUGE, EPSILON, TINY, RANGE | kinds, inquiry and model functions (100, 108) |
| `fcvalues.f90` | `TYPE VALUE` (a COMPLEX(8) and an error code); `OPERATOR(+)` and the rest, generic, each guarding against overflow and division by zero; safe complex division (Smith's), powers, the formatting of a value | derived types, functions of a type past 8 bytes, generic names and defined operators, PRIVATE (103, 105, 106) |
| `fclexer.f90` | the token stream: `TYPE TOKEN`, numbers by internal READ, names | character intrinsics (SCAN, VERIFY, INDEX, ADJUSTL, LEN_TRIM, IACHAR), internal files, SELECT CASE (101, 108, 109) |
| `fctree.f90` | `TYPE NODE` with POINTER children; the recursive descent parser; copying and freeing trees | recursive types, ALLOCATE and DEALLOCATE of pointers, pointer results, RECURSIVE (89, 107) |
| `fcsymbols.f90` | variables and functions in binary search trees of pointers; LIST in order | pointer components, module data, USE ONLY and renames (106, 107) |
| `fcfuncs.f90` | the built-in table and the special functions (gamma by Lanczos, erf by series and continued fraction) | MTHRTL through the intrinsics, array constructors and PARAMETERs, assumed-shape list functions, SUM, PRODUCT, MAXVAL, PACK (77, 102, 104) |
| `fceval.f90` | the evaluator, a frame of parameter values per call, the depth limit | RECURSIVE, automatic arrays, OPTIONAL and keyword arguments, INTENT (89, 103, 104) |
| `fcnumeric.f90` | SOLVE, INTEGRATE, DIFF, ROOTS | allocatable arrays, array expressions and sections, WHERE, internal procedures, construct names with EXIT and CYCLE (101, 102, 103, 104) |
| `fcplot.f90` | TABLE and PLOT: samples, scaling, the grid as an automatic CHARACTER array, axes and labels | array-valued functions, WHERE, MINVAL/MAXVAL with MASK=, run-time CHARACTER lengths, REPEAT, nonadvancing output (102, 104, 108, 109) |
| `fcstate.f90` | SAVE, LOAD, `@`, LOG: OPEN with STATUS='REPLACE', ACTION=, POSITION='APPEND', INQUIRE (EXIST=), IOSTAT=, DATE_AND_TIME in the saved file's heading | Fortran 90's I/O and the intrinsic subroutines (108, 109) |
| `fcalc.f90` | the main program: the prompt, the command words, dispatch, help | ADVANCE='NO', SIZE=, EOR= (109) |
| `fcplatform.f90` | the one thing that differs by system: the foreign command's text (LIB$GET_FOREIGN on VMS) | calling an RTL routine from Fortran 90 (83) |

`fcplatform.f90` has a twin for gfortran, `gfortran/fcplatform.f90`, which
returns no text (A.4), so every other file is the same source on both.

## A.3 Numbers: one output from three machines

fcalc must print the same lines on govax, under gfortran on the host
(IEEE arithmetic), and on the simh VAX. The rules that make that possible:

- **G_floating.** fcalc is compiled `/G_FLOATING`, so REAL(8) has 53 bits of
  significand and a range near 1e308, as IEEE double has. Under D_floating
  (the default) the range is 1e38, and values IEEE prints would trap.
- **Digits shown.** The default is 12 significant digits (`SET DIGITS` up
  to 15). Round-half-away versus round-half-even, and a last bit from MTHRTL
  versus the host's libm, rarely reach the twelfth digit; a test that finds
  such a case changes the case, and A.4's witness lists any left.
- **Nothing traps.** A VAX faults on floating overflow, divide by zero, and a
  reserved operand where IEEE gives an infinity or NaN. Every operation and
  function in `fcvalues` and `fcfuncs` checks its operands first (a product
  by comparing exponents, EXP's argument against LOG(HUGE), a divisor against
  zero, a function's domain), and reports `%FCALC-E-OVERFLOW` or the like
  instead. Underflow gives zero on both (the VAX has no denormals; fcalc
  flushes values below TINY to zero itself, so IEEE agrees).
- **Random numbers** differ by generator: a test shows only properties
  (`rand()` in [0,1), a seeded sequence repeated), never the values.
- **The time** in a saved file's heading is a comment; a test compares files
  without it.

## A.4 Testing

`testdata/fcalc/` holds the sources, `README.md`, and test scripts, each
`name.in` with `name.out`: `basics`, `complex`, `functions`, `userfuncs`,
`numeric`, `display`, `plot`, `state` (SAVE, CLEAR ALL, LOAD, `@`, LOG; the
files on the volume), `errors` (every message), and `oneshot` (the foreign
command, govax and VMS only).

- **Expected output** is worked from the mathematics, the same rule as the
  corpus: each value a known one (tables, identities, a second method), and
  each plot drawn from its samples by hand. gfortran then witnesses it.
- **The milestone test**, `TestFcalcMilestone` (`internal/console`), builds
  fcalc in each of Phase 86's modes and Phase 111's goal (`/OPTIMIZE` both
  ways, `/RECURSIVE` both ways, both paging modes, `/CHECK=ALL`, `/DEBUG`)
  and runs every script in each, comparing with the `.out` files.
- **The compiler's tools** on a real program: a `/LIST/CROSS_REFERENCE`
  compile without failure, `/STANDARD=F90` and `/WARNINGS=ALL` with each
  message explained or fixed, and a debugger session (`fcalc.dbg`: a break
  in the evaluator, EXAMINE through a tree's pointers, SHOW CALLS of the
  recursion) held to a log.
- **gfortran's witness**: `compare.py` gains fcalc (the gfortran platform
  file, `-std=f95` but for that file), running every script and comparing
  every line, differences explained against the standard.
- **The simh round** (the author's offer, 2026-11-20): `testdata/f90r/`, as
  `testdata/ftn86` did: a volume with FTNRTL's sources and fcalc's (and the
  Fortran 90 corpus's) MACRO files from govax, and a procedure where VMS
  7.1's MACRO and LINK build FTNRTL and each program and run every script.
  VMS's log becomes an oracle, `TestF90ROracle`. fcalc is the first program
  to run all of Fortran 90's run-time library on a real VAX.
- **Fast tests**: what can be tested below the console (the compile of every
  file without messages, in `dclcmd` or `internal/fortran`), as the memory on
  compiler tests asks.

## A.5 fcalc as a tool

fcalc ships on the system volume (Phase 87's table, `internal/sysvol`):

- its sources in `SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC]` (or wherever the author
  prefers) with `BUILDFCALC.COM`, which compiles, links, and copies the image
  to SYS$SYSTEM, as `BUILDFTN.COM` builds FTNRTL;
- the image itself, built by govax's own build as FTNRTL's is (`mkftnrtl`'s
  pattern: committed, embedded, checked by a test), so it is there on a first
  start without a compile;
- `FCALC` defined as a foreign command by the supplied SYLOGIN.COM;
- `HELP FCALC` in HELPLIB.HLB, and fcalc's own HELP from the same text;
- MANUAL.md: a section on fcalc as a tool, and in 14.13 as the example of a
  Fortran 90 program.

## A.6 The subtasks, revised

As revised by the author's answers (below), with vectors and matrices added:

1. **The survey and design** (this part). (done 2026-11-20)
2. **The core**: `fckinds`, `fcvalues`, `fclexer`, `fctree`, `fcsymbols`,
   `fceval`, and the main loop: expressions, operators, variables, the
   prompt, errors; `basics` and `complex` scripts. Compiler bugs found are
   fixed in subtasks of their own (2a, 2b, ...), each with a corpus program.
   (done 2026-11-20)
   - 2a. `REAL` and `AIMAG` of a COMPLEX(8) under `/F90`, and `CMPLX` of
     constants with `KIND=`; `testdata/fortran90/cparts.f90`. (done 2026-11-20)
   - 2b. An INTENT(IN) dummy as a READ's internal file or DECODE's buffer;
     `testdata/fortran90/intentread.f90`. (done 2026-11-20)
3. **Functions**: the built-in table, special functions, list functions,
   user-defined functions and recursion; `functions`, `userfuncs`.
4. **Vectors and matrices** (A.8): the values, literals, indexing, operators,
   `fcmatrix`'s functions; `vectors`, `matrices`.
5. **Numerics**: SOLVE, INTEGRATE, DIFF, ROOTS; `numeric`.
6. **Display and settings**: SET and SHOW, the formats and bases; `display`.
7. **TABLE and PLOT**; `plot`.
8. **State and files**: SAVE, LOAD, `@`, LOG, the foreign command; `state`,
   `oneshot`, `errors`.
9. **Every mode**: `TestFcalcMilestone`, and the compiler's tools on fcalc.
10. **gfortran's witness**.
11. **The simh round**: `testdata/f90r`, fcalc and the whole Fortran 90
    corpus, run by the author on the simh VAX, and its oracle.
12. **fcalc as a tool**: the system volume, the built-in image, SYLOGIN's
    symbol, HELP FCALC, MANUAL.md's section.
13. **The manual's 14.13** finished.
14. **FORTRAN-INTERNALS.md** finished, with fcalc as its worked example of a
    whole program.
15. **Close-out**: PLAN.md, CLAUDE.md, "Where things stand", and Phase 112's
    go or no-go.

## A.7 Questions for the author

1. **The command language** in A.1: is this the calculator you meant? In
   particular, is a keyword-first command form (`SOLVE ... FOR x FROM a`) right,
   or would you rather have everything as functions (`solve(f(x), x, 1)`)?
2. **Vectors and matrices** as values (`[1,2,3]`, element-wise operations,
   `det`, `inv`, a linear solve) would exercise arrays further, at perhaps
   1,500 more lines. In fcalc now, later, or not at all?
3. **G_floating** (A.3): agreed that fcalc is built `/G_FLOATING`?
4. **Expected output** (A.4): worked by hand from the mathematics, as the
   corpus rule says, with gfortran as the witness. For plots and long tables,
   may the hand-worked file be checked against gfortran's output, and the
   differences resolved by hand, instead of every character drawn by hand
   first?
5. **Where it ships** (A.5): sources in `[SYSHLP.EXAMPLES.FCALC]`, the image
   in SYS$SYSTEM built in and committed, and the `FCALC` symbol in the supplied
   SYLOGIN.COM. Good, or another place?
6. **The simh round** (A.4): run fcalc and the whole Fortran 90 corpus in it,
   or fcalc alone?

## The author's answers (2026-11-20)

1. **Keyword commands**, as A.1 has them (`SOLVE expr FOR x FROM a`).
2. **Vectors and matrices** are in fcalc: A.8, subtask 4.
3. **G_floating**: yes, fcalc is built `/G_FLOATING`.
4. **Expected output**: worked by hand, and for plots and long tables checked
   against gfortran's output, each difference resolved by hand.
5. **Shipping** as A.5 proposes: an example on the system volume, sources in
   `[SYSHLP.EXAMPLES.FCALC]` with `BUILDFCALC.COM`, the image built in and
   committed, `FCALC` in the supplied SYLOGIN.COM.
6. **The simh round** runs the whole Fortran 90 corpus beside fcalc: it will be
   the largest exercise yet of the VAX CPU and MTHRTL, and may find what the
   corpus alone hasn't.

## A.8 Vectors and matrices

A value has a rank, 0 (a scalar, as before), 1, or 2. `TYPE VALUE` keeps a
scalar in place and an array's elements through a POINTER component to a
rank-2 COMPLEX(8) array (a vector is n by 1 or 1 by n; Fortran 90 has no
allocatable components). Ownership is explicit: each operation's result has
storage of its own, a defined ASSIGNMENT(=) copies deeply, and the evaluator
frees each operand it has finished with. A test counts the blocks in use after
each script (a `SHOW MEMORY` command of fcalc's own) so a leak shows.

- **Literals**: `[1, 2, 3]` a row, `[1; 2; 3]` a column, `[1, 2; 3, 4]` a
  matrix; elements are expressions.
- **Indexing**: `v(2)`, `m(1, 2)`, sections `m(:, 1)`, `v(2:4)`, assignment
  to an element or section (`m(2, :) = [0, 1]`).
- **Operators**: `+` `-` element by element, with a scalar spread over an
  array; `*` the matrix product (a scalar times an array element by element);
  `.*` `./` `.^` element by element; `'` the conjugate transpose, `.'` the
  transpose; `A \ b` the linear solve; `^` of a square matrix and an integer
  by squaring (a negative power through the inverse); comparisons element by
  element.
- **Functions**: every elementary function element by element; `size rows
  cols numel transpose trace det inv rank norm(v [,p]) dot cross kron reshape
  diag eye(n) zeros(m,n) ones(m,n) linspace(a,b,n) range(a,b[,s]) cumsum
  sort`; `eig(m)`, the eigenvalues by the shifted QR method on the Hessenberg
  form (complex values allowed); the list functions (`sum mean max ...`) of an
  array argument as well as of a list.
- **Elsewhere**: ROOTS of a vector of coefficients, and `poly(r)` from roots;
  `PLOT v` draws a vector against its index, `PLOT y VERSUS x` one vector
  against another; TABLE of vectors as columns; SAVE writes arrays as
  literals; a matrix prints in aligned columns, its digits by `SET DIGITS`.
- **Module**: `fcmatrix.f90` (LU with partial pivoting, the determinant,
  inverse, and solve from it; Householder reduction to Hessenberg and the QR
  iteration; norms), exercising MATMUL, TRANSPOSE, SPREAD, MAXLOC, DOT_PRODUCT,
  CSHIFT, RESHAPE of values, array sections with strides, assumed-shape
  dummies, array-valued functions of a run-time shape, and pointer arrays (102,
  104, 107, 108). About 1,500 more lines; the program's estimate is now 5,500
  to 6,500.
- **Numbers**: eigenvalues and inverses are where machines differ most; tests
  choose well-conditioned matrices and show 10 digits, and `eig`'s values are
  printed sorted so the order of convergence doesn't show.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-20:** Subtask 1 written: Part A, the design of fcalc, the
  milestone program the author asked for in place of one found (a scientific
  calculator with complex values, functions, numerics, ASCII plots, and saved
  state), with the subtasks revised (A.6) and six questions (A.7). The author
  offered a run on the simh VAX, now subtask 10.
- **2026-11-20:** The author's answers: keyword commands, vectors and matrices
  added (A.8, subtask 4, the later subtasks renumbered), `/G_FLOATING`, hand-
  worked output checked against gfortran's, fcalc shipped as an example on the
  system volume, and the simh round running the whole Fortran 90 corpus too.
  Subtask 1 done.
- **2026-11-20:** Subtask 2, the core, in `testdata/fcalc` (README.md):
  `fcplatform`, `fckinds`, `fcmessages` (a module A.2 didn't have: output
  and the messages), `fcvalues`, `fclexer`, `fctree`, `fcsymbols`,
  `fceval`, and `fcalc`, about 2,100 lines; `compiles.txt` their order. The
  `basics` and `complex` scripts, worked by hand, are printed alike by govax
  (in every mode: `/[NO]OPTIMIZE`, `/RECURSIVE`, `/CHECK=ALL`,
  `/DEBUG=ALL`) and by gfortran. `internal/console`'s `TestFcalc` builds
  fcalc and runs every script; `internal/dclcmd`'s `TestFcalcCompiles`
  compiles it without a console. What VMS made fcalc do, beside A.3's
  arithmetic: carriage control (`fcplatform`'s `CC`, a blank on VMS and
  nothing in gfortran's twin, so both print the same lines and `compare.py`
  needn't normalize fcalc's); terminal records of at most 133 bytes
  (OUTSTAOVE), so `SAY` writes a longer line in records of 132; input lines
  of at most 199 characters (INPRECTOO), so fcalc's limit is 199, the error
  caught by IOSTAT= as LINELONG, and a too-long line not echoed; Fortran 90
  has no `I0` (gfortran's `-std=f95` took it; FTNRTL gave OUTCONERR); a
  complex constant's parts must be constants, so values are made by `CMPLX`.
  The design held, with three choices made here: `SET ECHO` (A.1's, needed
  now for the scripts) echoes each line after its prompt, and the line that
  set it; equality is to within 64 epsilons and the order is the real parts';
  a part of a complex value smaller than the other by more than the digits
  shown is shown as zero, which also keeps the machines' last bits out of
  the output. Findings: subtasks 2a and 2b, below; and govax refused a
  public type with a component of a type that is public where it is defined
  but private in the using module by its default PRIVATE (ACCESSCOMPONENT,
  ACCESSPROC), which gfortran accepts. That is Phase 106's reading (access
  belongs to a name in a module), defensible under F90's 4.4.1 and 5.2.3, so
  fcalc's modules name such types PUBLIC and the compiler is unchanged; a
  question for the author. govax's terminal hands back a line past 256
  bytes in pieces, as a VMS terminal read whose buffer fills does, and keeps
  type-ahead between images, so the scripts have no line past 256 and
  nothing after QUIT or EXIT.
- **2026-11-20:** Subtask 2a: under `/F90`, `REAL(Z)` and `AIMAG(Z)` of a
  COMPLEX*16 are REAL*8 (`sema`'s `fortran90Readings`, rows that stand for
  Table D-3's under `/F90` only, chosen by `analyzer.formsFor`; under
  `/NOF90` REAL of one is still a REAL*4 and AIMAG of one BADARG), a new
  row of Phase 99's table of texts the two read differently and MANUAL.md's
  14.13; and `CMPLX(X, Y, KIND)` of constants is computed in that kind
  (`evaluator.complexType`, `kindOf`), where it was computed in COMPLEX*8
  and converted, so `CMPLX(1D0/3D0, 0D0, 8)` lost all but 24 bits. Held by
  `sema`'s `TestComplexParts` and `testdata/fortran90/cparts.f90` (also in
  `compare.py`, the same on gfortran).
- **2026-11-20:** Subtask 2b: a READ's internal file and DECODE's buffer are
  read, not given a value, so an INTENT(IN) dummy's text may be either
  (`sema`'s `sourcePlace`, `analyzer.readFrom`); a WRITE's or ENCODE's is
  still INTENTIN. Held by `TestIntent`'s new cases and
  `testdata/fortran90/intentread.f90` (DECODE's, so not in `compare.py`).
