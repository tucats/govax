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
| `INTEGRATE expr FOR x FROM a TO b` | adaptive Gauss-Kronrod quadrature (adaptive Simpson's rule until subtask 5), with an error estimate |
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
   (done 2026-11-20)
   - 3a. A section of a scalar record's array component under
     `/CHECK=BOUNDS` (`T%V(1:N)`) crashed the compiler; `TestFortranCheck`'s
     COMPSECTION and COMPSECTIONOK. (done 2026-11-20)
   - 3b. `@(Rn)` operands' bound one byte short, so the optimizer made a byte
     branch MACRO couldn't reach; `codegen`'s `TestOperandBounds`, and
     `TestBoundsHold` over the Fortran 90 corpus and `/CHECK=ALL`.
     (done 2026-11-20)
   - 3c. An internal procedure of a module procedure whose name passes 31
     characters called by another name than its own;
     `testdata/fortran90/longhost.f90`. (done 2026-11-20)
4. **Vectors and matrices** (A.8): the values, literals, indexing, operators,
   `fcmatrix`'s functions; `vectors`, `matrices`. (done 2026-11-20)
5. **Numerics**: SOLVE, INTEGRATE, DIFF, ROOTS; `numeric`. (done 2026-11-20)
   - 5a. SIZE, LBOUND, UBOUND, SHAPE, LEN, KIND, and the other inquiries of
     an INTENT(OUT) dummy were INTENTOUTREF; `TestIntent`'s cases and
     `testdata/fortran90/inqout.f90`. (done 2026-11-20)
   - 5b. Under `/F90` an adjustable array of no elements (ARGS(N), N 0) was
     ADJARRDIM under `/CHECK=BOUNDS`; `TestFortranCheck`'s ADJZERO.
     (done 2026-11-20)
6. **Display and settings**: SET and SHOW, the formats and bases; `display`.
   (done 2026-11-21)
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
- **2026-11-20:** Subtask 3, functions. `fcfuncs.f90`, a new module between
  FCSYMBOLS and FCEVAL: the built-in functions as a table (`BUILTINS` and
  `MORE_BUILTINS`, two since a statement may have only 39 continuation
  lines), each a CASE of `APPLY_BUILTIN`, all of A.1's list plus `hypot` and
  `rand(n)`. A real argument in a function's real domain takes the
  intrinsic (MTHRTL's routine on a VAX); anything else the principal value
  of the complex function, from FCVALUES' guarded operators, with these
  conventions: `asin z = -i ln(iz + sqrt(1 - z**2))`, `acos z = pi/2 -
  asin z` (so `asin(2)` is `pi/2 - 1.317i`, `acos(2)` `1.317i`), `cbrt`
  of a real number the real root, `arg(0)` 0, `atan2(0, 0)` and the poles
  (`ln(0)`, `atanh(1)`, `gamma(0)`) DOMAIN. SET ANGLE (A.1's, needed now
  for the trigonometric functions; SHOW SETTINGS stays subtask 6's):
  degrees and grads reduced exactly to a turn, and exact at quarter turns.
  The special functions take real arguments (NOTREAL otherwise; `beta`
  positive ones): gamma by factorials at whole numbers and Lanczos (g = 7,
  9 terms) elsewhere, through the reflection below 1/2; lgamma exactly 0 at
  1 and 2; erf by its series of positive terms below 1.5 and erfc by its
  continued fraction, 200 deep, above (the crossover chosen by a trial in
  IEEE doubles against Python's: 1 - erf from the series loses digits past
  2, 1e-12 of erfc at 2.5, and the fraction needs about 80 terms at 1.5). `!` off the whole numbers is now gamma(x + 1),
  as A.1 says, so `basics`' `2.5!` became `(-2)!`. The list functions
  exercise SUM, MINVAL, MAXVAL, WHERE, automatic arrays, and a quickselect
  on PACK's results for the median. Users' functions: `f(x, y) = expr`, up to
  8 parameters, kept in FCSYMBOLS as a second binary search tree (a name may
  be a variable and a function; CLEAR forgets both; LIST with no word lists
  both, functions after variables); each call has a FRAME of its parameters,
  passed to EVALUATE as an OPTIONAL argument; an error inside a function is
  reported at the column of the call typed (`FRAME%COLUMN`); calls nest at
  most 200 deep (RECURSION). The `functions` values were checked against
  Python's `math` and `cmath`; gfortran printed every line of both new
  scripts as govax does, in every mode subtask 2 ran (`/[NO]OPTIMIZE`,
  `/RECURSIVE`, `/CHECK=ALL`, `/NOOPTIMIZE/DEBUG=ALL`, paging on).
  `TestFcalcCompiles` now also compiles `/CHECK=ALL`. /CHECK=ALL found that
  fcalc must not underflow either (govax faults on it there): FCVALUES'
  `FORMAT_VALUE` (subtask 2's) multiplied a value near 1E-300 by 1E-12, now
  `NEGLIGIBLE`; ATAN2 is `ANGLE_OF`, which never asks it about a point
  near an axis; `V_DIV` gives no IEEE -0, which ATAN2 would put on the
  other side of the branch cut. Three compiler bugs found, 3a to 3c. And
  subtask 2's `basics.out` and `complex.out` had never been committed (the
  repository's `.gitignore` drops `*.out`): `testdata/fcalc/.gitignore`
  keeps them, as the corpus's does. By the author's request (2026-11-20),
  fcalc's sources are commented for a reader learning Fortran 90 and
  numerical programming, every module's included.
- **2026-11-20:** Subtask 3a: `ir`'s `checkSubscripts` asked the record's
  symbol for the bounds of a section of its array component before seeing
  that the dimensions were the field's (`T%V(1:N)` under `/CHECK=BOUNDS`,
  FCSYMBOLS' `T%PARAMS(1:COUNT)`); a scalar record has none, and the
  compiler panicked. Now the field's bounds are taken first. Held by
  `TestFortranCheck`'s COMPSECTION (past the end, SUBRNG) and COMPSECTIONOK
  (through a pointer, as fcalc's).
- **2026-11-20:** Subtask 3b: `codegen`'s `operandBound` counted `@(Rn)` as
  one byte; the VAX has no register deferred mode deferred again, and MACRO
  assembles it as `@0(Rn)`, two bytes. The branch passes, adding bounds,
  then made FCEVAL's `BGTR` a byte branch to a label 128 bytes on, and
  MACRO refused it (`/OPTIMIZE/CHECK=ALL`). Held by `TestOperandBounds`;
  `TestBoundsHold` now also reads the Fortran 90 corpus, free form, and
  compiles each program `/CHECK=ALL` too (822 compilations), though no
  corpus program makes `@(Rn)`.
- **2026-11-20:** Subtask 3c: a call of an internal procedure divided its
  name `M.HOST.INNER` at the first period, a routine's at the last; the two
  halves rejoined to the same text while it fit 31 characters, but a longer
  one was numbered from different prefixes, so the call named a symbol
  nothing defined (FCVALUES' `FORMAT_VALUE.NEGLIGIBLE`, LINK's NUDFSYMS).
  The call now divides at the last period, an inner name having none. Held
  by `testdata/fortran90/longhost.f90` (also in `compare.py`).
- **2026-11-20:** Subtask 4, vectors and matrices (A.8), in two new modules,
  `fcarrays.f90` (after FCVALUES) and `fcmatrix.f90` (after FCFUNCS), with the
  lexer, parser, symbols, evaluator, and main program extended; fcalc is now
  about 7,400 lines with its comments. Two scripts, `vectors` and `matrices`,
  worked by hand and in Python (characteristic polynomials' roots for the
  eigenvalues, exact fractions for the inverses), are printed alike by
  gfortran and by govax in every mode subtask 3 ran (`/[NO]OPTIMIZE`,
  `/RECURSIVE`, `/CHECK=ALL`, `/NOOPTIMIZE/DEBUG=ALL`, paging on), and the
  four earlier scripts are unchanged. This subtask found no compiler bug:
  pointer components to rank-2 arrays, vector subscripts on them, automatic
  arrays of a type with pointer components, array-valued functions of a
  run-time shape, WHERE, CSHIFT, MAXLOC, MATMUL, DOT_PRODUCT, and strided
  sections all compiled and ran as gfortran's.
  Decisions made here, against A.8 or beyond it:
  - **A QUANTITY beside VALUE.** A.8 put the array pointer in `TYPE VALUE`
    with a deep-copying `ASSIGNMENT(=)`. In Fortran 90 that leaks every
    function result (a result is itself a value with storage, which a copying
    assignment copies and drops, and there are no FINAL procedures), and it
    would have changed every scalar routine already tested. So `VALUE` stays
    the scalar, and `FCARRAYS`' `TYPE QUANTITY` is a `VALUE` or ROWS by COLS
    elements through a `COMPLEX(RK), POINTER :: E(:, :)`, with an explicit
    ownership rule: a quantity made is its maker's caller's, assignment moves
    it, `DUPLICATE` copies, `RELEASE` frees; variables take what they're given
    and hand out copies. A count of arrays (and FCTREE's of nodes) in use is
    `SHOW MEMORY`, which each script checks is 0 after `CLEAR ALL`.
  - **No trap, three ways** (FCMATRIX's header): LU, solve, inverse,
    determinant, and rank by guarded complex operations, `C_ADD`, `C_SUB`,
    `C_MUL`, `C_DIV`, now FCVALUES' public helpers, which `V_ADD`, `V_MUL`,
    and `V_DIV` are built on; MATMUL and DOT_PRODUCT only when every part is 0
    or in 1E-100 to 1E100, a guarded loop otherwise; `eig` on the matrix
    divided by its largest element, parts below 1E-50 set to zero as they
    appear, plain complex arithmetic between. `C_ABS` and `NEAR` no longer
    form a product that could underflow for values near SMALL.
  - **Shapes.** A vector is 1 by n or n by 1; an array of one element is a
    scalar (`[5]`, a row times a column); at most 10,000 elements (TOOBIG).
    Elements may be arrays, joined (`[v, 4]`, `[m; 0, 0]`).
  - **Indexes** are Fortran's: `lo:hi:stride`, any part left out, so
    `v(5:1:-1)` reverses and `v(1:2:9)` is one element; a vector of numbers is
    a vector subscript; one index counts in array element order. `NAME(...)`
    is a built-in function, a parameter's index, a user's function, or a
    variable's index, in that order; `NAME(...) =` assigns elements if NAME
    holds an array and isn't a function, and defines a function otherwise.
  - **Operators.** `*` is the matrix product between arrays, `/` and `\`
    right and left division (A / B by B.' \ A.'), `^` a square matrix's whole
    power; everything else element by element, a scalar with each element.
    A pivot no larger than n EPS times the largest element is zero: `det` is
    then 0, and `inv` and `\` SINGULAR. The list functions take every element
    of every argument; FCFUNCS' other functions apply to each element.
    `norm(m)` of a matrix is Frobenius's, `norm(m, 1)` and `norm(m, huge)` the
    column and row sums; `norm(m, 2)` is DOMAIN (it needs an SVD).
  - **Display.** Columns right-aligned, two blanks apart; a row longer than
    132 is shown in groups under "columns j to k:"; each element beside the
    largest (FORMAT_VALUE's new BESIDE), so rounding's noise shows as 0 on
    both machines. LIST shows an array as the literal that makes it, written
    a piece at a time (FCMESSAGES' `SAY_PART`, `SAY_END`). `sort` and `eig`
    count real parts the same when they differ by less than the digits show,
    then order by imaginary part, so conjugate pairs come out alike.
  - **Borrowed from subtask 6:** `SET DIGITS n` (A.8's tests show 10 digits)
    and `SHOW MEMORY`; SET and SHOW's other settings stay subtask 6's.
  - **Left for later:** A.8's "Elsewhere": ROOTS of a vector and `poly(r)`
    (subtask 5), `PLOT v`, `PLOT y VERSUS x`, and TABLE of vectors (subtask
    7), SAVE writing arrays as literals (subtask 8; LIST already does).
- **2026-11-20:** Subtask 5, numerics, in two new modules: `fcpoly.f90`
  (after FCFUNCS: roots by Durand-Kerner, each root polished by two of
  Newton's steps kept only if they shrink |p|, zero roots taken out first;
  poly; polyval) and `fcnumeric.f90` (after FCEVAL: SOLVE, INTEGRATE, DIFF,
  and the ROOTS command). FCEVAL's `EVALUATE_WITH` evaluates a tree with one
  name bound in a frame of its own, whose column 0 reports an error at the
  node it arose in; `roots`, `poly`, and `polyval` are FCMATRIX's functions,
  so `roots(p)` works in any expression and is sorted as `eig` is; four
  messages (KEYWORD, NOROOT, SAMESIGN, and the warning INACCURATE); SOLVE,
  INTEGRATE, and DIFF reserved words, so `DIFF (X + 1)^2 FOR X AT 3` reads as
  the command. The `numeric` script, worked in Python (Newton's method, `erf`,
  a model of Ridders' table), is printed alike by gfortran and by govax in
  every mode subtask 4 ran (`/[NO]OPTIMIZE`, `/RECURSIVE`, `/CHECK=ALL`,
  `/NOOPTIMIZE/DEBUG=ALL`, paging on), as are the six earlier scripts.
  Decisions made here, against A.1 or beyond it:
  - **INTEGRATE by Gauss-Kronrod**, not A.1's adaptive Simpson's rule.
    Simpson's rule with Richardson's correction took 2,000 to 12,000 values
    for ordinary integrals to 13 digits (5,385 for `exp(-x^2)` from -3 to 3,
    11,961 for `sqrt(1 - x^2)` from -1 to 1), some 10 seconds of the emulated
    VAX for three integrals. The 7-15 point rule, bisecting the piece with the
    largest error (QUADPACK's QAG), takes 15 to 225 values for the same
    smooth integrals and 1,485 for the circle; it never evaluates the ends,
    so `1/sqrt(x)` and `ln(x)` from 0 are integrated (Simpson's rule
    evaluates them, DIVZERO); and an interior singularity Simpson's model met
    exactly at 1/3 is no longer a test case, since whether a point lands on
    it is the machine's rounding. The tolerance is 10^-(DIGITS+1) of the
    integral of |f|, no piece's error below 50 EPS of its own; at most 200
    pieces, then the warning.
  - **SOLVE's start** is 0 when FROM isn't given; the secant method's second
    point is 0.001 times max(|a|, 1) on. A complex start finds a complex root.
    The root is shown `x = root` and kept as ANS; the variable named after FOR
    is unchanged (it is only the frame's parameter). Brent's method sets a root
    within a rounding error of the bracket's ends' size to 0, as INTEGRATE's
    and DIFF's answers set parts below their own error estimate to 0.
  - **The warning** INACCURATE gives the error estimate to two digits, after
    the answer. DIFF warns when its estimate is past SQRT(EPS) of the
    derivative (or of 1).
  - **Polynomials' coefficients** are the highest power's first, as MATLAB's
    and Octave's, so `poly(roots(p))` is p. A double root has about half the
    digits (the script shows it with SET DIGITS 6); the iteration stops when
    its steps stop shrinking, if they are below 1E-3.
  - **ROOTS's arguments** are joined, each a scalar or a vector (NOTVECTOR for
    a matrix); `roots(p)` takes one vector.
  Two compiler bugs found, 5a and 5b. And subtask 3's and 4's "every mode" had
  missed one: under `/CHECK=ALL` the `functions` script stopped at `rand()` with
  ADJARRDIM (5b), and the rest of its input ran on in the next script. Each
  mode's run is now held to every script's whole output (in the tree from
  subtask 9's milestone test).
- **2026-11-20:** Subtask 5a: an inquiry function's argument (SIZE, LBOUND,
  UBOUND, SHAPE, LEN, and the type inquiries KIND, DIGITS, HUGE, ...) asks
  about a dummy's shape, length, or type, not its value, so an INTENT(OUT)
  dummy may be one before it is given a value (FCPOLY's `N = SIZE(Z)`). Such a
  mention is noted `inquired` (`sema`'s `noteInquired`, from `arrayFunction`,
  the `inquiryArgType` of the INQ functions, LEN, and the constant evaluator's
  `arrayBounds`) and is neither a use nor a definition to
  `reportIntentOut`, so a use after it is still INTENTOUTREF. Held by
  `TestIntent`'s five new cases and `testdata/fortran90/inqout.f90` (also in
  `compare.py`). The corpus README now has rows for `cparts.f90` and
  `intentread.f90`, which subtasks 2a and 2b left out.
- **2026-11-20:** Subtask 5b: under `/F90` an upper bound below the lower makes
  an array of no elements (the standard's 5.1.2.4.1), so `/CHECK=BOUNDS` no
  longer calls FTN$ADJARRDIM for an adjustable array of a `/F90` unit
  (`ir`'s `checkArrayBounds`); FCEVAL's `ARGS(N)` for `rand()`, N 0, stopped
  fcalc. `/NOF90` keeps VAX FORTRAN's check (ADJDIM, ADJENTRY). Held by
  `TestFortranCheck`'s ADJZERO; the golden `f90shaped` loses its check. Left
  for later: an extent below zero (X(N), N -3) isn't made zero (the
  descriptor's extent is still U - L + 1); nothing in fcalc or the corpus makes
  one.
- **2026-11-21:** Subtask 6, display and settings. FCVALUES' showing of
  values is now by three settings: SET DIGITS, SET FORMAT (AUTO, as before;
  FIXED, DIGITS places after the point by F editing; SCIENTIFIC, every
  significant digit shown; ENGINEERING, the exponent a multiple of 3, by
  MODULO), and SET BASE (DECIMAL, HEX, OCTAL, BINARY: a value whose parts
  are whole numbers below FCKINDS' new `WHOLE_LIMIT`, 4E15, is shown exactly
  with a prefix, `0xFF`, `0o377`, `0b101`, its digits by division in
  REAL(RK), exact for the powers of two, so no integer wider than 32 bits
  is needed). FCLEXER reads the same literals (`BASED_NUMBER`, either case,
  an `i` after for an imaginary one; BADNUMBER for a digit or letter run on,
  OVERFLOW past `WHOLE_LIMIT`). SET SEED n (0 to 999,999,999) puts a seed
  made of n through RANDOM_SEED, whose size is the compiler's (one on
  govax, eight under gfortran), so an ALLOCATABLE seed of SIZE='s length.
  SHOW SETTINGS, and SHOW DIGITS, FORMAT, ANGLE, BASE, SEED, or ECHO, show
  each setting as the SET command that makes it, so the lines may be typed
  back or run from a file (subtask 8's SAVE will write them); a seed never
  given shows as a comment. The `display` script, worked by hand from these
  rules, is printed alike by gfortran and by govax in every mode (`/[NO]OPTIMIZE`,
  `/RECURSIVE`, `/CHECK=ALL`, `/NOOPTIMIZE/DEBUG=ALL`, paging on), as are
  the eight earlier scripts. No compiler bug found: ES with no digits after
  the point (`ES11.0E3`, SET DIGITS 1), F editing, MODULO of a negative
  number, RANDOM_SEED's SIZE= and PUT= with an allocatable array, and an
  implied DO in an array constructor all ran as gfortran's.
  Decisions made here, beyond A.1:
  - **FIXED's DIGITS are places**, a calculator's FIX n, where the other
    formats' are significant digits: one setting, read by the format. A
    value whose digits before the point and the places would pass 15 is
    shown in scientific notation with as many places, so no digit shown is
    one the machines' last bits could change; a part too small for a
    place is zero (`1e-9 + 1e-6i` is `0.0000`), with no minus sign.
  - **SCIENTIFIC and ENGINEERING show every digit**, trailing zeros too,
    and zero as `0.000e+0`; AUTO alone drops them.
  - **Bases show whole values only**: a value with a part that isn't whole
    is shown in decimal by the format, and whole ones are never rounded or
    zeroed beside a matrix's largest, having nothing to hide. Zero is `0`
    in every base.
  - **A quiet line**: one ending in `;` is run without showing its value,
    assignment, or definition, as in MATLAB and Octave (fcalc's other
    borrowing from them), so the script keeps a random value without
    printing it.
  - **Ties**: a decimal tie (12345 to four digits) rounds half to even
    under gfortran, and may round half away on a VAX, so the script has
    none; the README says so.
  - **A minus sign** before SET DIGITS's or SET SEED's number is DOMAIN at
    the sign: the sign is a token of its own.
