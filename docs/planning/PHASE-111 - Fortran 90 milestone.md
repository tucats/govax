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
7. **TABLE and PLOT**; `plot`. (done 2026-11-21)
8. **State and files**: SAVE, LOAD, `@`, LOG, the foreign command; `state`,
   `oneshot`, `errors`. (done 2026-11-22)
   - 8a. A substring of a literal character constant under `/F90`
     (`'ABCDEF'(K:K)`, R609); `testdata/fortran90/constsub.f90`.
     (done 2026-11-21)
9. **Every mode**: `TestFcalcMilestone`, and the compiler's tools on fcalc.
   (done 2026-11-22)
10. **gfortran's witness**. (done 2026-11-22)
11. **The simh round**: `testdata/f90r`, fcalc and the whole Fortran 90
    corpus, run by the author on the simh VAX, and its oracle. (done
    2026-11-22: round 2 matches VMS in every section; `testdata/tir111` the
    MACRO probe it led to)
12. **fcalc as a tool**: the system volume, the built-in image, SYLOGIN's
    symbol, HELP FCALC, MANUAL.md's section. (done 2026-11-22)
13. **The manual's 14.13** finished. (done 2026-11-22; with Appendix E, by the author's
    request)
14. **FORTRAN-INTERNALS.md** finished, with fcalc as its worked example of a
    whole program. (done 2026-11-22)
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
- **2026-11-21:** Subtask 7, TABLE and PLOT, in a new module, `fcplot.f90`
  (after FCNUMERIC, whose command readers, AT_WORD, READ_WORD, READ_NAME,
  READ_BOUND, and AT_END_OF_LINE, are now public and taken by USE ... ONLY;
  so is VALUE, which govax's ACCESSPROC asked for, READ_BOUND's argument
  being one). TABLE and PLOT are reserved words, as SOLVE is; four messages
  (BADSTEP, TOOMANY, BADLIMITS, NOPOINTS), four more lines of the table holding
  two each to stay within 39 continuations. The `plot` script is printed alike
  by gfortran and by govax in every mode (`/[NO]OPTIMIZE`, `/RECURSIVE`,
  `/CHECK=ALL`, `/NOOPTIMIZE/DEBUG=ALL`, paging on), as are the nine earlier
  scripts. No compiler bug found: an automatic CHARACTER array of a run-time
  length and number (the graph), an array-valued function of a run-time size,
  WHERE assigning its own mask, MINVAL and MAXVAL with MASK=, an array of a type
  with a pointer component (the expressions' trees), LEN_TRIM of an array
  section, an implied-DO constructor, and an internal procedure's function of a
  run-time CHARACTER length all ran as gfortran's.
  Decisions made here, beyond A.1 and A.8:
  - **The forms.** `TABLE e, ... FOR x FROM a TO b [STEP s]` (ten steps if no
    STEP, either way; one row if a = b); `TABLE v, ...` with no FOR, vectors of
    one length side by side (A.8's TABLE of vectors); `PLOT e, ... FOR x FROM a
    TO b`, `PLOT v, ...` against the index (lengths may differ), and `PLOT v,
    ... VERSUS u` (A.8's `PLOT y VERSUS x`); SIZE w, h (10 to 100 by 3 to 60,
    61 by 19 unless given) and Y lo, hi in any order. At most 8 expressions, a
    mark each (`*o#x@%&$`), and 1,000 rows.
  - **A table** has a heading over each column (the expression as typed) and a
    rule of `-` under it, the columns right-aligned two blanks apart as a
    matrix's are; values are shown as an expression's are, by the settings,
    with no "beside" rule (a column of exp(-x) keeps its small values). A
    point A + K*S within 64 epsilons of the larger end of 0 is 0.
  - **A point's errors.** DIVZERO, OVERFLOW, DOMAIN, NOTREAL, NOTWHOLE,
    SINGULAR, and NOCONVERGE at a point are its own: a table shows the
    identifier in the cell, a graph no mark. Any other error (UNDEFINED,
    NOTSCALAR, ...) stops the command. A value that isn't real has no mark.
  - **A graph** marks each value in the nearest row and column by NINT, the
    range's top and bottom labeled (six digits at most, by the format and
    base), `0` beside the x axis and under the y axis where there's room; the
    frame is the y axis when x = 0 is the left edge. A later curve's mark
    covers an earlier one's; a range of one value is widened by max(1, |v|)
    each way; Y's range drops the values outside it; NOPOINTS if no value is
    real. No line joins the marks: a steep curve has gaps, as the default
    `exp(-x^2)` shows.
  - **Ties.** A value halfway between two rows rounds away from zero, but only
    a value computed exactly is sure to be there on both machines; the script's
    sizes and ranges were chosen, with a Python model of the rule, so no other
    sample lies within a millionth of a row of a tie or on a Y range's edge.
- **2026-11-22:** Subtask 8, state and files, in a new module, `fcstate.f90`
  (after FCPLOT): SAVE, LOAD, `@` and RUN, LOG, and the foreign command.
  The author committed the work in progress on 2026-11-21 before a system
  upgrade; this entry closes it. Seven messages: FILEREQ, NOFILE, FILEERR,
  NESTING, FILELOOP, the warning NOLOG, and INFILE, the `-` line that says
  which line of which file a message came from. The `state` and `errors`
  scripts and the files they write (`expected/saved.fcalc`,
  `session.fcalc`) are printed alike by gfortran and by govax in every mode
  (`/[NO]OPTIMIZE`, `/RECURSIVE`, `/CHECK=ALL`, `/NOOPTIMIZE/DEBUG=ALL`,
  paging on), as are the ten earlier scripts; `oneshot.com` runs on govax
  only. One compiler bug found, 8a: under `/F90` a substring of a literal
  character constant, `'0123456789'(K:K)` (R609's parent string may be any
  scalar constant), was a syntax error; the parser now takes one after a
  character literal (`parse`'s `isCharLiteral`), and sema folds it or reads
  the constant's datum as it does a named constant's. Held by
  `testdata/fortran90/constsub.f90`, the same on gfortran. (gfortran's
  witness now needs `SDKROOT=$(xcrun --show-sdk-path)` after the host's
  upgrade, or its link can't find library `System`.)
  Decisions made here, beyond A.1:
  - **SAVE's file is fcalc's own commands**: a comment heading with the date
    and time, each setting as SHOW SETTINGS writes it, then each variable and
    function, every line ending in `;` so running the file shows nothing. A
    number has as many digits as it takes to read back the very same value
    (A.3), so LOAD restores it exactly; an array past `save_width` (100
    characters) is made a piece at a time. A seed never given is a comment.
  - **LOAD clears every variable and function first**, then runs the file
    quietly; a LOAD whose file can't be opened leaves everything as it was.
    `@file` and RUN run a file's lines as if typed, with nothing cleared.
  - **File names**: a name with no type gets `.fcalc` (VAX FORTRAN would give
    `.DAT`); one with a type keeps it. RUN, LOAD, SAVE, and LOG are commands
    only when a file name follows: with `=` or `(` after them they are a
    variable or a function (`run = 3`, `log(8, 2)`).
  - **Nesting**: files may run each other 8 deep (`max_depth`), each on a unit
    of its own (20 + depth), NESTING past that; a file that is already
    running (asked by INQUIRE's OPENED=, so however its name is spelled) is
    FILELOOP. EXECUTE, RUN, and RUN_FILE are RECURSIVE.
  - **An error ends the file and every file that ran it**, each saying where
    with an INFILE line; a warning doesn't; EXIT ends the file, not fcalc.
  - **LOG** appends a transcript (what SAY writes, without carriage control)
    to its file; LOG OFF closes it, NOLOG if none is open.
  - **The foreign command**: `$ FCALC line` runs the one line and ends;
    with no text fcalc reads lines as usual. Its text comes from
    `fcplatform.f90`'s LIB$GET_FOREIGN (gfortran's twin has none). An error
    in it is shown with the line it came from, since the terminal never
    echoed it.
- **2026-11-22:** Subtask 9, every mode and the compiler's tools on fcalc.
  `TestFcalcMilestone` (`internal/console/fcalc_test.go`) replaces
  `TestFcalc` and the scratch `TestFcalcTry`: fcalc built `/NOOPTIMIZE`,
  `/OPTIMIZE`, `/RECURSIVE/NOOPTIMIZE`, and `/RECURSIVE/OPTIMIZE` (the
  corpus's `optimizerModes` and `recursiveModes`) with paging off and on, then
  `/CHECK=ALL` and `/NOOPTIMIZE/DEBUG=ALL` with paging off, ten builds, each
  running all eleven scripts, checking SAVE's and LOG's files, and running
  `oneshot.com`; all print the same lines (about 66 seconds in all).
  The compiler's tools, by `internal/dclcmd`'s `TestFcalcTools`:
  `/LIST/CROSS_REFERENCE` of every module says nothing and writes its
  listing (77 to 3,796 lines); `/STANDARD=F90` and `/WARNINGS=ALL` say only
  `fcalcToolMessages`, each explained there and in the README. Fixed in fcalc:
  ALIGNCOMMON three times in FCMESSAGES (INTEGERs declared after a 199-byte
  CHARACTER, off their longwords; the CHARACTERs now come last, with a
  comment on why), and UNUSEDNAME in FCMATRIX (MANY in a USE ... ONLY no
  longer used). Explained and kept: FCPLATFORM's two EXTENSIONs
  (LIB$GET_FOREIGN's `$` and null argument), 13 HOSTHIDDENs (internal
  procedures declaring their own locals, as they should), and FCSTATE's
  UNREAD (`REST`, read only to drop an overlong line's tail, now commented).
  gfortran prints every script and file as before after these changes.
  The debugger session: `testdata/fcalc/fcalcd.com` builds fcalc `/DEBUG`
  (MACRO/DEBUG, LINK/DEBUG) and runs `fact(3)` with DBG$INPUT naming
  `fcalcd.dbg`, which breaks at FCEVAL.CALL_USER at each level of the
  recursion, walks the expression trees through their pointers (`FN%BODY%KID%OP`,
  `P%KID%KID%NEXT%NUMBER%Z`, NOTASSOC for `P%KID%NEXT%KIND`), reads the
  caller's frame (`F%NAMES(1)`) and a module variable (`CALLS`), and shows 23
  calls three levels deep; `TestFcalcDebugger` (`internal/console`, beside
  `runProbeProcedure`) holds the session's log to `fcalcd.dlg` and the
  procedure's to `fcalcd.out`, masking what moves with any change (SHOW CALLS's
  numbers, the MACRO line under a break, the FORTRAN line's number). No
  compiler or debugger bug found. Observed, and left as it is: SHOW CALLS's line
  column is the MACRO file's line, not the FORTRAN line a break shows, since
  under `/DEBUG` the module's line table is the MACRO file's (Phase 110's 7c),
  which `%LINE n`, TYPE, and EXAMINE/SOURCE also count; numbering SHOW CALLS
  alone by FORTRAN lines would make it disagree with them. The session says SET
  RADIX DECIMAL, as the manual tells a FORTRAN user to (Phase 85: the language
  stays MACRO).
- **2026-11-22:** Subtask 10, gfortran's witness. `compare.py` (in
  `testdata/fortran/gfortran`) gains fcalc, run with the corpus when no
  program is named: built from `compiles.txt` with `gfortran/fcplatform.f90`,
  every script run in one directory holding `files/` in their names' order, as
  `TestFcalcMilestone` runs them, and each script's output and each file in
  `expected/` compared line for line, times masked. With nothing to normalize
  (gfortran's twin writes no carriage control), it needs no `KNOWN` entries.
  fcalc is built three ways (`FCALC_BUILDS`), and all 13 comparisons in each
  are the same: `-std=f95 -O0`, as the corpus is; `-O2`, where on the arm64
  host gfortran's fused multiply-adds change last bits, which A.3's rounding
  rules must hide, and do; and checked, `-fcheck=all,no-array-temps`,
  `-ffpe-trap=invalid,zero,overflow,underflow` (VAX traps, underflow as
  `/CHECK=UNDERFLOW` traps it), `-finit-real=snan`, `-finit-integer=-77777`,
  so a value used before it is set, an index or pointer out of place, or an
  operation the guards should have stopped would end the run, and none does.
  `-Wall -Wextra`'s 95 warnings are reviewed: 92 exact comparisons of reals,
  each deliberate (against zero, whole-number tests, SAVE's check that a value
  reads back the same), and three on the Gauss-Kronrod loop's `I / 2`, which
  `MOD(I, 2) == 0` keeps from indexing at 0. No difference found, so no
  difference to explain against the standard, and no fcalc or compiler change.
  The whole witness (the corpus and fcalc) passes, 147 comparisons. The fcalc
  README's new section "gfortran's witness" describes it.
- **2026-11-22:** Subtask 11, the simh round, part 1: the probe, ready for the
  author's run. `testdata/f90r` (README.md), `testdata/ftn86`'s pattern:
  `gen.go` writes `exchange.cmd` (a volume with FTNRTL's sources, fcalc
  compiled by govax `/G_FLOATING` four ways, `[FCALC]`, `[FCALCN]`
  `/NOOPTIMIZE`, `[FCALCR]` `/RECURSIVE`, `[FCALCC]` `/CHECK=ALL`, and every
  Fortran 90 corpus program three ways, a program of modules compiled in its
  `compiles.txt`'s first order in a directory of its own with its module
  files), `f90r.com` (VMS's MACRO and LINK build FTNRTL, then each fcalc build
  runs every script in its name's order, shows the files the scripts wrote a
  record at a time by a DCL READ loop, since TYPE would take their first
  character for FORTRAN carriage control, and runs `ONESHOT.COM`; then each
  corpus program), and `copyout.cmd` (the log, FTNRTL's objects, image, map,
  and sources, and fcalc's plain build's `.MAR` files, objects, and image, for a
  later comparison of govax's MACRO and LINK with VMS's on the compiler's own
  output). The volume: 1,671 files, about 57,000 of RD54's blocks.
  `TestF90RProcedure` (`internal/console/f90r_test.go`) rehearses it on govax
  (`runFTN86`'s volume and run are now `runVMSRound`, shared): FTNRTL builds
  with no message, every corpus program prints its `name.out` but ADVANCE (its
  nonadvancing prompt stays in its record before a read of a file,
  `f90rFileDifferences`), and every fcalc build prints every script's
  `name.out`, writes `expected/`'s files, and runs `oneshot.com` as its `.out`
  says (about 45 seconds).
  The rehearsal found an fcalc bug: SAY wrote its carriage-control blank even
  when continuing the record a prompt left open, which FTNRTL leaves open when
  SYS$INPUT isn't a terminal (Fortran 90's rule; FTN$$UNIT_PROMPT ends it only
  at a terminal READ), so every echoed line read from a file showed as `fcalc>
  set echo` with two blanks (and `oneshot.out` had recorded it, its data lines
  being no terminal), and a typed line longer than 125 characters overflowed the
  133-byte record (OUTSTAOVE), ending fcalc in the middle of `basics`. Fixed in
  fcalc: `fcplatform.f90` gains `prompt_ends` (LIB$GETDVI's DVI$_TRM of
  SYS$INPUT, asked once) and `record_room` (132; gfortran's twin `.false.` and
  `HUGE(1)`, so gfortran's output is unchanged), and SAY continues an open
  prompt's record without the blank, its first piece the room the prompt left;
  the log's records are pieces of 132 as before. The one difference left from a
  terminal is where such a long line is split, which the test allows for
  (`f90rFromFile`). `oneshot.out` and `fcalcd.out` corrected; `/STANDARD=F90`'s
  two new EXTENSIONs (LIB$GETDVI's `$` and null argument) in
  `fcalcToolMessages`; `TestFcalcMilestone`, `TestFcalcDebugger`,
  `TestFcalcTools`, and gfortran's witness (all three builds) pass unchanged
  otherwise. Next: the author's run, then `TestF90ROracle`, govax's run held to
  VMS's log, and part 2's fixes.
- **2026-11-22:** Subtask 11, the first attempt: VMS 7.1 mounted the volume, and
  `@F90R/OUTPUT=F90R.LOG` bug-checked the VAX at once (UNXSIGNAL, "Unexpected
  signal name in ACP", in F11BXQP). The signal array the register dump points
  at (R0) is SS$_ACCVIO, reason 1, virtual address `0000000C`, PC 818B5417:
  the XQP read through a null pointer to one of its own blocks, offset 12, so
  something on the volume wasn't where it looked. `tools/odscheck` (its own
  module, outside govax's build; by the author's wish the start of an
  ANALYZE/DISK_STRUCTURE/REPAIR for govax) checks the container from its raw
  blocks: headers, checksums, maps, extension chains (INDEXF.SYS's too), back
  links, directories' contiguity, records, order, and FIDs, free header slots,
  and BITMAP.SYS by ods2's `AnalyzeDisk`. It finds no problem. What differs
  from Phase 89's volume, which VMS extended without trouble: INDEXF.SYS has 10
  free header slots left (1,681 in use) with an extension header (file 1279, no
  ident area); and the MFD's 21 blocks are all in use (EOF 22/0), with
  F90R.COM in a full block (508 bytes), so inserting F90R.LOG makes the XQP
  split that block and extend the MFD; Phase 89's, as VMS left it, has a spare
  block. Neither is wrong by ODS-2's rules; the cause is open. Next: VMS's own
  view (SDA on the crash dump, ANALYZE/DISK_STRUCTURE on the volume mounted
  /NOWRITE), from the author.
- **2026-11-22:** Subtask 11, the second VAX crash and a way round it. No
  crash dump (no SYSDUMP.DMP on the VAX). `MOUNT/NOWRITE` then
  `ANALYZE/DISK_STRUCTURE DUA1:` bug-checked too (XQPERR, "Error detected by
  file system XQP", in VERIFY.EXE), so reading the volume is enough: no create,
  no MFD split. The stack held `00000516`, 1,302, the blocks INDEXF.SYS's
  primary header maps; past them only govax's extension header (file 1279)
  maps it. So the XQP fails on an index file extension header ods2 made. A
  correction to Phase 89's log: the copy of its volume VMS left has INDEXF.SYS
  in one header (72 extents), which the XQP can't have made from one with an
  extension header, so that round's volume most likely never had one, and this
  is the first govax index file extension header VMS has read. Every extension
  header on the local volumes (all ods2-made) has file 1279's form: ident and
  map areas both at word 40, no record attributes, the back link the primary's
  FID; what VMS writes isn't known here (no VMS-made one is at hand), so which
  field VMS objects to is open (DEVIATIONS.md). The round goes round it: govax's
  INITIALIZE/CONTAINER gains VMS's `/HEADERS=n` and `/DIRECTORIES=n` (ods2's
  `InitializeOptions` had both; `rms.ContainerSpace`, `InitializeContainerWith`;
  HELP INITIALIZE /CONTAINER; `TestInitializeContainerWith_space`), and
  `exchange.cmd` makes the volume `/HEADERS=6000 /DIRECTORIES=800`: INDEXF.SYS
  6,024 blocks in one header, the MFD 51 blocks with 21 in use. `runVMSRound`
  takes the space too, so the rehearsal's volume is the same. `odscheck` gains
  `-extensions` (each extension header, raw). The crashed volume is kept as
  `testdata/disks/f90r-exchange-attempt1.dsk`.
- **2026-11-22:** Subtask 11, round 1 on VMS 7.1, and its fix. VMS's
  ANALYZE/DISK_STRUCTURE of the volume made `/HEADERS=6000 /DIRECTORIES=800`
  found nothing but the missing QUOTA.SYS, and `@F90R` ran to its end: VMS's
  MACRO and LINK built FTNRTL with no message, and of the corpus's 366 runs
  351 printed what govax's run of the procedure prints (`TestF90ROracle`,
  holding the rehearsal to `testdata/f90r/vax/f90r.log`). The rest, AUTOLEN,
  AUTOS, CALENDAR, CHARFNS, and TEXTFMT in all three variants, and fcalc in
  all four, stopped at `%MACRO-E-DIRSYNX` on a bare `.ASCII`: the compiler
  wrote one for a zero-length text (`''`, a CHARACTER*0 initial value), its
  `asciiPieces` giving an empty text one empty piece, and govax's MACRO took
  it. Fixed both ways: `asciiPieces` gives an empty text no pieces, so a text
  datum is its label alone and an initial value a comment
  (`codegen`'s `TestTexts`); and govax's MACRO refuses a string directive
  with nothing after it, DIRSYNX, before storing anything, as VMS's does
  (`asm`'s `TestStringDirectiveWithNothing`; the author confirmed `.ASCII //`
  is accepted on VMS and stores nothing; `.ASCIZ`, `.ASCIC`, and `.ASCID` with
  nothing are taken to be DIRSYNX too, unconfirmed). Those programs are in
  `f90rAwaitingVMS` until round 2. The corpora (every mode), fcalc's
  milestone, `TestNoF90Unchanged`, and both rounds' rehearsal and oracle pass.
  The volume is rebuilt from the fixed compiler (round 1's kept as
  `testdata/disks/f90r-exchange-round1.dsk`), and `copyout.cmd` writes round 2
  to `vax/round2/`.
- **2026-11-22:** Subtask 11, round 2 on VMS 7.1: every section of VMS's log
  is what govax's run of the procedure prints. On the volume rebuilt from the
  fixed compiler every program built with no message; all 366 corpus runs,
  and fcalc in all four builds (its 11 scripts each, the files SAVE and LOG
  wrote, ONESHOT), match govax's output (`TestF90ROracle`, fcalc's file times
  masked; `f90rAwaitingVMS` empty). Round 2 is now `testdata/f90r/vax`, round
  1 `vax/round1`. fcalc is the first program to run all of Fortran 90's
  run-time library on a real VAX, under VMS's own LIBRTL, MTHRTL (G_floating),
  and RMS, and it printed every line govax and gfortran print.
  Round 2 also brought back VMS MACRO's objects of fcalc's 16 compiled `.MAR`
  files, and `TestF90RObjects` holds govax's MACRO to them (GSD, TIR, and end
  records; the header records differ by nature, the traceback records by
  design, Phase 110's FORTRAN lines). It found govax storing a repetition
  factor (`.LONG 0[2]`, which the compiler writes for zeroed locals) as
  immediate bytes, where VMS's MACRO writes `STA_UB 0`, `STA_UB 2`, `STO_RL`:
  fixed (`asm`'s `evRepeat`, `emitRepeated`; a known value with a count over
  1; `TestObjectRepetition`), and govax's LINK, which had no `STO_RB`,
  `STO_RW`, or `STO_RL`, now runs them (`link`'s `storeRepeated`). 14 objects
  are now VMS's. FCFUNCS and FCMATRIX still differ (`f90rObjectDifferences`):
  in three places VMS's MACRO stored one byte of a long run of immediate data
  through the stack (`STA_UB 0`, `STO_B`), after 534, 498, and 321 bytes of
  the run, at no point the source marks (the same table elsewhere in FCMATRIX
  has none), so the rule is in VMS's record packing. `testdata/tir111` is a
  probe for it, a module per question (the modules truncated, the tables
  alone, shifted, re-split, and with other values; and repetition factors and
  empty string directives of every kind); the author offered the VAX for it.
  Also noted: govax's MACRO writes no object for a module with errors, where
  VMS's writes one (round 1's LINK named "compilation errors" in it); TIRX
  will show it. `go test ./...` passes.
- **2026-11-22:** Subtask 11, `testdata/tir111`'s run on VMS 7.1. The stacked
  byte: TIRA, TIRE, and TIRF reproduce it; TIRB (the table alone, the same
  offsets and record packing), TIRC1 to TIRC8 (shifted), TIRD (re-split), and
  TIRG don't, and their objects are VMS's record for record. It needs the
  earlier psects, the exact lines, and the exact offsets, but not the values;
  no rule found (neither the record boundaries nor the source's block layout
  explain it), and the bytes are the same, so it stays a known difference
  (`tir111Differences`, `f90rObjectDifferences`; DEVIATIONS.md). Repetition
  factors (TIRR): the value stacked as written, `-1` as `STA_UB 1`,
  `OPR_NEG`, the count, any count, 1 included, a value defined later stacked
  with its value in place, unfolded; govax's MACRO now does all of it
  (`emitRepeated` takes the value's tree; a forward value's copies filled in by
  fixups marked `repeated`, never relocations; `TestObjectRepetition`), and
  `.LONG R1[2]` is DIRSYNX at its line as on VMS. TIRX: `.ASCIZ`, `.ASCIC`, and
  `.ASCID` with nothing are DIRSYNX, as govax says. VMS's MACRO writes an object
  for a module with errors where govax's writes none, and whether a register
  name may stand in a data expression is unsettled: both in DEVIATIONS.md.
  `TestTIR111Objects` and `TestTIR111Errors` hold govax to the run.
  `go test ./...` passes.
- **2026-11-22:** Subtask 12, fcalc as a tool. fcalc's sources moved from
  `testdata/fcalc` to `internal/fcalc/src` (gfortran's twin to `src/gfortran`),
  so a package can embed them; `testdata/fcalc` keeps the scripts, and every
  user follows (`TestFcalcCompiles`, `TestFcalcMilestone`, `TestFcalcDebugger`,
  which gathers its files in a directory of its own, `testdata/f90r`'s `gen.go`
  and `exchange.cmd`, only paths changed, and `compare.py`'s `FCALC_SOURCES`).
  `internal/fcalc` embeds the sources, `buildfcalc.com`, and `fcalc.exe`, the
  committed image; `internal/fcalc/build` compiles each module (`/G_FLOATING
  /OPTIMIZE`, its module text kept in memory for the next USE, a
  `sema.ModuleFinder` of its own), assembles, and links, times and tool names
  fixed, its symbols `dclcmd.BuiltinSources` (new: FTNRTL from the built-in
  image, then govax's tables), in about a second; `go generate
  ./internal/fcalc` (`mkfcalc`) writes the image, `TestCommittedImage` holds it
  to the sources, `TestImageShape` checks its name and libraries. The system
  volume gains `[SYSTEM.SYSHLP.EXAMPLES]` and `[.FCALC]` (the 16 sources and
  BUILDFCALC.COM, in compiles.txt's order) and `SYS$SYSTEM:FCALC.EXE`
  (`sysvol`'s `Examples`, `FcalcExamples`). BUILDFCALC.COM builds in its own
  directory (`F$ENVIRONMENT("PROCEDURE")`), deletes the `.MAR`, `.OBJ`, and
  `.F90$MOD` files, and with INSTALL copies the image to SYS$SYSTEM (COPY/QUIET,
  govax's COPY confirming by default). The supplied SYLOGIN.COM defines
  `FCALC :== $SYS$SYSTEM:FCALC` (seeded, so a volume made before keeps its
  copy). HELP FCALC is `bootdata/files/help/fcalc.hlp`: a topic per subject,
  function group, and command, each under 40 lines by the author's direction
  (`TestFcalcHelpTopicsShort`), but Examples' sessions, which may be longer;
  every example in it is run on the shipped image and held to the text
  (`TestFcalcHelpExamples`, whose `FCALC_HELP_WRITE=1` writes the Examples
  topics from fcalc's output; it found five of the draft's claims wrong).
  fcalc gains HELP [topic ...]: FCPLATFORM's `show_help` calls
  LBR$OUTPUT_HELP with FCMESSAGES' `help_line` as its output routine (a
  module procedure passed on as a dummy procedure), no flags, the library
  defaulted, so its help goes through SAY and the log and is DCL's HELP FCALC
  line for line (`TestFcalcOnSystemVolume`, against HELP/NOPROMPT); gfortran's
  twin has none, and fcalc says `%FCALC-E-NOHELP` (message 52, two of the
  table's lines joined to stay within 39 continuations; not in the `errors`
  script, since no script runs HELP and VMS's round is unchanged). HELP is read
  with the file commands, since its topics needn't be fcalc's tokens.
  LBR$OUTPUT_HELP with its library defaulted and no SYS$HELP:HELPLIB.HLB now
  reads govax's built-in one, as DCL's HELP does (DEVIATIONS.md).
  `fcalcToolMessages` has LBR$OUTPUT_HELP's two EXTENSIONs. MANUAL.md's 6.6
  "FCALC, a calculator", its system volume table, and HELP SYSTEM_VOLUME name
  the new files. `TestFcalcOnSystemVolume` runs SYLOGIN.COM, then `FCALC 2^10
  + 1`, fcalc's HELP beside DCL's for four requests, and BUILDFCALC INSTALL,
  whose image runs `basics` as the milestone's build does. The fcalc tests,
  gfortran's witness (all three builds), and `TestF90R*` pass.
- **2026-11-22:** Subtask 13, the manual's 14.13 finished. Its opening is no longer
  "being added, a part at a time": it says what Fortran 90 compiles, how it was checked (the
  corpus by hand, gfortran, VMS 7.1), and has a table of the section's parts. The section's
  48 bold-led paragraphs, in phase order, are now under subheadings (Source form and
  spellings, Declarations and kinds, The intrinsic procedures, Writing for any processor,
  Control constructs, Arrays as values, Procedures and interfaces, Derived types, Input and
  output, Dynamic arrays, Modules, Pointers). Derived types had never been introduced (no
  TYPE ... END TYPE, `%`, or constructor before their arguments and arrays): a new opening
  paragraph and example (`shapes`, run on govax for its output) do it. Three new parts end
  the section: "fcalc, a whole program" (a table of its 16 files, what each holds and shows
  of Fortran 90, from each file's own header comment, A.2's planned table having changed;
  the build; why `/G_FLOATING` and nothing traps; the modes and tools run on it), "On
  gfortran and on VMS" (subtasks 10 and 11), and "What Fortran 90 isn't here", a table
  replacing "Not there yet", each row compiled to check it: off it, sections of arrays of
  records (they compile since Phase 105); on it, newly listed, CSHIFT, EOSHIFT, and SPREAD
  with a DIM= not constant and RESHAPE with an ORDER= not constant, a statement function of
  a derived type, a name an internal procedure's declarations use that its host uses only in
  executable statements, an assumed-shape array as `%LOC`'s or a system subroutine's
  argument, a CHARACTER array expression of run-time length not made of names as an actual
  argument; and FORALL, PURE, and ELEMENTAL, Fortran 95's, which are `UNRECSTMT`, not
  NOTIMPL. 14.7's "What isn't there" row now points there, and its "Choices" gain a Fortran
  90 bullet gathering Phases 99 to 110's unconfirmed choices a user can see. Chapter 14's
  opening, "How far it can be trusted", and its table name the finished section and fcalc.
  The author asked, while this was written, for a section on rebuilding the system's parts
  from source, as an appendix: **Appendix E, "Customizing the system from source"**, on
  FTNRTL, the help libraries, fcalc, and STARLET's macro library, each on the system volume
  (its procedure) and in govax's tree (`go generate`, the tests that hold each); section 5's
  "Rebuilding FTNRTL and the macro library", chapter 10's help paragraph, and 6.6's FCALC
  point to it. Each volume procedure it shows was run on a scratch system volume (BUILDFTN,
  BUILDHELP, a single module's LIBRARY/HELP/REPLACE, BUILDFCALC from a copy of the examples
  directory, the image run as a foreign command). Found: chapter 10 named the help sources
  `SYS$HELP:[.SOURCES]`, which RMS rightly refuses (SYS$HELP isn't rooted); now
  `SYS$SYSROOT:[SYSHLP.SOURCES]`. Every `(#anchor)` link in MANUAL.md resolves.
- **2026-11-22:** Subtask 14, FORTRAN-INTERNALS.md finished. Its opening no longer says
  Fortran 90 "is being added, a part at a time": it says Fortran 90 is in the same compiler
  beside VAX FORTRAN, points to the manual's "What Fortran 90 isn't here", and names the two
  whole programs, DUNGEON and fcalc. By the author's request while it was written, a
  **contents** list follows the opening, every section under one of seven parts (generated from
  the headings, every anchor checked), and the opening's long section-by-section roadmap is now
  a short paragraph. New, **section 36, "A whole program: fcalc"**: the sixteen files and their
  order, module files, separate compilation, and the built-in image's build; `v = a + (-b)` in
  FCVALUES' V_SUB, compiled for the section (a non-SEQUENCE VALUE of 32 bytes, naturally aligned;
  operators resolved while compiling; values past 8 bytes through a hidden argument and the
  statement's temporaries; the final 32-byte move a safe choice left); FCEVAL's CHOOSE, a pointer
  path through NODEs as one MOVL per component, in a RECURSIVE unit's frame, with an OPTIONAL
  dummy's cell; the numerical design that lets three machines agree, and the compiler's part in
  it; a table of the ten compiler, MACRO, and LINK bugs fcalc found (2a to 8a and the VMS round's)
  with what they had in common (two features meeting, one build mode, the real machine less
  forgiving); and how a whole program is tested. Section 21's map gains a row for each Fortran 90
  topic and fcalc (every file named checked), its "Why" row reaching Phase 111, and two pieces of
  advice; section 17 points to section 36; section 24's opening and section 25's passages that
  looked ahead (the attribute table's POINTER row, "recorded for a later phase", keywords waiting
  for Phase 108) now say how things stand, and `sema/attrtable.go`'s comment on `attrRecorded` no
  longer says "meaning nothing yet". Found while checking the section's claims: fcalc's FCKINDS
  said that under D_floating `SELECTED_REAL_KIND(15, 300)` is -2 and the compile stops, and A.3
  and MANUAL.md's 14.13 said the same in other words; in fact REAL\*16, H_floating, has the range,
  so the request is answered with it and fcalc compiles, larger and slower. FCKINDS' comment
  (the same number of lines, so the committed image is unchanged, `TestCommittedImage`) and the
  manual are corrected; A.3 above stays as the survey wrote it.
