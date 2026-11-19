# Phase 108 — Fortran 90: the intrinsic procedures

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Subtask 1, the survey (Part A), done 2026-11-19, for the author's
review.

## Goal

The rest of Fortran 90's intrinsic procedures (the standard's chapter 13, the
Handbook's Appendix A), each a row in sema's table (Decision 14), each usable
on arrays where elemental:

- **character**: ACHAR, IACHAR, ADJUSTL, ADJUSTR, TRIM, LEN_TRIM, REPEAT, SCAN,
  VERIFY, INDEX with BACK=, LGE/LGT/LLE/LLT (already), and character results
  whose length is known only at run time (TRIM, REPEAT, ADJUSTL of a passed
  length), through class D strings as concatenation's are (Phase 78);
- **numeric**: CEILING, FLOOR, MODULO; the kind= arguments of AINT, ANINT, INT,
  NINT, REAL, CMPLX, LOGICAL, CHAR;
- **the model**: EXPONENT, FRACTION, NEAREST, RRSPACING, SPACING, SCALE,
  SET_EXPONENT, on each VAX format's bits (in line where short, MTHRTL or FTNRTL
  where not);
- **bits**: MVBITS (a subroutine), BTEST, IAND, IBITS, ISHFT, ISHFTC, NOT, and
  the rest already from MIL-STD-1753, now with Fortran 90's argument names;
- **TRANSFER**, with its three forms;
- **subroutines**: DATE_AND_TIME, SYSTEM_CLOCK, RANDOM_NUMBER, RANDOM_SEED
  (`MTH$RANDOM`'s generator or a new one: the survey decides), MVBITS;
- **keyword arguments** for every intrinsic (the table's dummies' names).

## What earlier phases leave in place

- The intrinsic table and its in-line, MTHRTL, and FTNRTL forms
  (`sema/intrinsics.go`, `ir/lowerintrinsic.go`, `ftn_intrinsics.mar`), the
  system subroutines (`ftn_system.mar`, Phase 83: DATE, TIME, RAN, SECNDS).
- `vaxfloat` for the model functions' bits; MTHRTL's engine.

## Design notes

- **Which form for each** is a column in the table, chosen by cost: a few
  instructions in line; a loop or a heap-using result in FTNRTL; a numeric
  function MTHRTL would naturally hold in MTHRTL (but MTHRTL is VMS's vector, so
  new routines there would be govax's own names: the survey decides whether that
  is acceptable or whether FTNRTL takes them).
- **DATE_AND_TIME's zone** is the host's offset, as LIB$'s time routines see it.
- **RANDOM_NUMBER** must give the same sequence for the same seed on every host
  (as `MTH$RANDOM` does).
- **From Phase 100's survey** (2026-11-17): KIND= on INT, REAL, CMPLX, NINT,
  AINT, ANINT, and LOGICAL moved to Phase 100, with the intrinsic table's
  keyword and optional columns (Decision 14), filled there for its own
  functions. Left here: zero-length CHARACTER entities (`LEN=0`, NOTIMPL
  until this phase), and the complex intrinsics (ABS, SQRT, EXP, ...) of
  COMPLEX(16), which Phase 100 declares and computes with in line.

## Open questions

1. New numeric routines in MTHRTL under govax's own names, or in FTNRTL? Proposed:
   FTNRTL (MTHRTL's vector stays VMS's).
2. RANDOM_SEED's seed size: one longword (MTH$RANDOM's) or more?

## Subtasks (provisional)

1. **Survey** (each intrinsic's form, cost, and test), for review (done
   2026-11-19; Part A).
2. **Character intrinsics** and run-time-length results.
3. **Numeric and kind= arguments.**
4. **The model functions** in every format.
5. **Bits and TRANSFER.**
6. **The subroutines**: DATE_AND_TIME, SYSTEM_CLOCK, RANDOM_NUMBER,
   RANDOM_SEED, MVBITS.
7. **Programs end to end**, every mode, gfortran's witness (but random numbers
   and times).
8. **Close-out**: FORTRAN-INTERNALS.md (section 27's table extended), MANUAL.md's
   Appendix B.

# Part A — the survey (subtask 1)

Read for it: the standard's 13 (13.3's keyword rule, 13.5.7's bit model, 13.7.1's
real model, 13.10's lists, 13.11's subroutines, 13.12's specific names, and every
13.13 entry this phase adds), 7.1.6.1 (what a constant and an initialization
expression may call), the *Fortran 90 Handbook*'s Appendix A beside them, the
HP Fortran manual's table of DEC's intrinsics (`VMS-FORTRAN-LANGUAGE.txt`, around
its line 11,700) for the keyword names of the functions the standard lacks, and
the 1988 manual's Table D-3 and D.4–D.5; and what govax has: `sema/intrinsics.go`,
`intrinsicref.go`, `intrinsickeys.go`, `constfuncs.go`, `inquiry.go`, `system.go`,
`charlength.go`, `kinds.go`, `generic.go`, the IR's `IntrinsicOp`s
(`sema/program.go`), `ir/lowerintrinsic.go`, FTNRTL's `ftn_intrinsics.mar` and
`ftn_system.mar`, and MTHRTL's tables and `random.go`. A program using every new
name was given to govax on 2026-11-19 (below), and its standard parts to
gfortran 16.2, `-std=f95`, the witness. There is no VMS probe: no VAX Fortran 90.

## A.1 What the standard asks

- **Character functions** (13.10.4). Elemental: ACHAR (I), IACHAR (C), ADJUSTL
  and ADJUSTR (STRING; the result is STRING's length, the blanks moved),
  LEN_TRIM (STRING), SCAN and VERIFY (STRING, SET [, BACK]), and INDEX's new
  BACK. Transformational: TRIM (STRING), a scalar whose length is LEN_TRIM's, and
  REPEAT (STRING, NCOPIES), a scalar of LEN (STRING) × NCOPIES characters,
  NCOPIES ≥ 0. Both may give a length of zero, and both, with REPEAT, RESHAPE,
  TRANSFER, and the SELECTED functions, are allowed in an **initialization
  expression** (7.1.6.1 (5)), as is every elemental intrinsic of type integer or
  character with such arguments (7.1.6.1 (4)): `PARAMETER (N = LEN_TRIM (NAME))`,
  `CHARACTER (LEN = 3) :: S = REPEAT ('-', 3)`. CHAR takes an optional KIND (only
  the default exists here). ACHAR of a code past 127 and IACHAR of a character
  not in ASCII are processor dependent.
- **Numeric functions** (13.10.2). CEILING (A) and FLOOR (A), of a REAL, default
  integer results (no KIND in Fortran 90; Fortran 95 adds it). MODULO (A, P), of
  an INTEGER or REAL, with P's sign (`MODULO (-8, 5)` is 2, `MODULO (8, -5)` -2);
  P = 0 processor dependent.
- **Floating-point manipulation** (13.10.12), elemental, of a REAL, on 13.7.1's
  model: EXPONENT (X) (default integer, 0 for 0), FRACTION (X) (X × b**-e),
  NEAREST (X, S) (the neighbor toward S's sign), RRSPACING (X) (|X × b**-e| ×
  b**p), SCALE (X, I) (X × b**I), SET_EXPONENT (X, I) (X × b**(I-e)), SPACING (X)
  (b**(e-p), or TINY (X) if that is out of range, and for 0). Out-of-range results
  of SCALE and SET_EXPONENT are processor dependent.
- **Bit functions** (13.10.10, 13.5.7): those Table D-3 already has, with the
  standard's names (I, J, POS, LEN, SHIFT, SIZE) and **ISHFTC's SIZE optional**
  (BIT_SIZE (I) when absent; Table D-3 requires three). Every result is of I's
  kind (13.13.43, .46, .48, .52).
- **TRANSFER (SOURCE, MOLD [, SIZE])** (13.13.108), transformational: SOURCE's
  bits as MOLD's type; a scalar for a scalar MOLD, else a rank-one array as
  short as holds SOURCE, or of SIZE elements. A result longer than SOURCE has an
  undefined tail; a shorter one is SOURCE's leading part.
- **Subroutines** (13.11). **MVBITS (FROM, FROMPOS, LEN, TO, TOPOS)** is
  *elemental*: FROM any integer expression (INTENT (IN)), TO a variable of its
  kind, on arrays as on scalars. **DATE_AND_TIME ([DATE] [, TIME] [, ZONE] [,
  VALUES])**: CCYYMMDD, hhmmss.sss, ±hhmm, and eight default integers (year,
  month, day, the zone in minutes, hour, minute, second, millisecond), blanks or
  -HUGE (0) for what isn't known. **SYSTEM_CLOCK ([COUNT] [, COUNT_RATE] [,
  COUNT_MAX])**: a count from 0 to COUNT_MAX that wraps, its rate per second;
  -HUGE (0) and zeros without a clock. **RANDOM_NUMBER (HARVEST)**: a REAL scalar
  or array, each value in [0, 1). **RANDOM_SEED ([SIZE] [, PUT] [, GET])**: at
  most one argument; SIZE the number N of default integers the seed takes, PUT
  and GET rank-one arrays of N or more; no argument a processor-dependent seed.
  All five take keyword arguments, so they need what an explicit interface gives.
- **Keyword arguments** (13.3) to every intrinsic, by the dummy names 13.13
  gives; specific names (13.12) take the same names. Only 13.12's specific names
  may be passed as actual arguments; the new functions have none, so nothing here
  becomes passable.

## A.2 What govax has

- **The table** (Decision 14). `intrinsicTable` is Table D-3's rows, and
  `fortran90Table` Fortran 90's so far (LOGICAL, the kind and numeric inquiries,
  COMPLEX(16)'s conversions and parts, MERGE, Phase 102's and 104's array
  functions), with columns for class and dummies. **Only Phase 100's and later rows
  name their dummies**: a keyword to any other is NOTIMPL. Tried 2026-11-19 (a
  program using every new name, `/F90`): `INDEX (S, 'B', BACK = .TRUE.)` and
  `IAND (I = 5, J = 3)` are NOTIMPL by keyword; `CALL DATE_AND_TIME (DATE = D)` is
  KEYWORDNOINTERFACE (an external subroutine); `CALL MVBITS (7, 2, 2, I, 0)` is
  SYSARG (VAX FORTRAN's MVBITS takes only variables for FROM and TO); and **TRIM,
  CEILING, SCAN, NEAREST, TRANSFER, and the rest are compiled as external
  functions** typed by IMPLICIT, without a word: under `/F90` the new names aren't
  intrinsic yet (`f90Intrinsics` lacks them).
- **Table D-3's oddities**: IBITS, IBCLR, and ISHFTC of an INTEGER\*2 give
  INTEGER\*4, as the 1988 manual's table says (its lines for IIBITS, IIBCLR, and
  IISHFTC), where IBSET of one gives INTEGER\*2; the standard gives I's kind, and
  so does gfortran. INTEGER\*1 arguments use the INTEGER\*2 rows and are converted
  back where the result is the argument's type (`byteResult`).
- **Constants**: `constfuncs.go`'s evaluator folds section 4.11's functions (MIN to
  CHAR) when every argument is constant, and Phase 102's `constarray.go` RESHAPE of
  constants; the SELECTED functions only of constants (a value that isn't is
  NOTIMPL, Phase 100's deferral to here).
- **In-line operations**: the IR's `IntrinsicOp`s (abs to merge), each lowered in
  `ir/lowerintrinsic.go`; INDEX is `CharIndex` (MATCHC); ICHAR, CHAR, LEN in line;
  MTHRTL's functions by `CallOp`; FTNRTL's FTN$IISHFTC and FTN$JISHFTC.
- **Character values of run-time length** (Phases 78, 104): a sema type
  `CHARACTER` of size 0 is any length known only when running (a passed-length
  dummy, a substring with bounds that aren't constant, Phase 104's automatic and
  `LengthExpr` lengths); concatenation of such parts makes a class D string
  (LIB$SGET1_DD, freed at the statement's end); a function's run-time-length
  result is `FuncRef.Made`. So **a size of 0 already means "not known yet", and a
  declared `LEN = 0` can't use it**: Phase 100 made that NOTIMPL (`decl.go`'s
  `characterType`), for here.
- **The system subroutines** (`sema/system.go`, `ftn_system.mar`, Phase 83): DATE,
  TIME, IDATE, ERRSNS, EXIT, MVBITS (FTN$IMVBITS/FTN$JMVBITS, every argument at the
  places' length), RAN (MTH$RANDOM itself) and SECNDS; time by `$NUMTIM` and
  `$ASCTIM`, hundredths the finest. No time zone is known anywhere in govax: the
  VMS time is the host's local time (`vmsdef.Time`), and no
  `SYS$TIMEZONE_DIFFERENTIAL` is defined.
- **MTH$RANDOM** (Phase 77): the manual's LCG on a caller's longword seed, the
  result its high 24 bits; deterministic unless `vax.runtime.random`.
- **MTHRTL** has every real function in H (`MTH$HSQRT`, `MTH$HEXP`, `MTH$HLOG`,
  `MTH$HSINCOS`, `MTH$HATAN2`, ...), and no H complex routine; COMPLEX(16)'s
  ABS, SQRT, EXP, LOG, SIN, COS, its powers, and MERGE of it are NOTIMPL
  (`complex32Intrinsic`, `complex32Power`; Phase 100's answer 1 and Phase 102's
  list).
- **The model** (`vaxfloat/model.go`): each VAX format is 13.7.1's model exactly,
  b = 2, the hidden bit f1, e the exponent field less 128 (F, D), 1024 (G), or
  16384 (H); p is 24, 56, 53, 113. So EXPONENT, FRACTION, and SET_EXPONENT are the
  exponent field read or replaced, and NEAREST is the fraction bits plus or minus
  1, with the VAX's word order (a longword's halves swapped) to mind. No
  denormals, no infinities: below TINY is 0, above HUGE an overflow.
- **Generic resolution** (Phase 103): GENERICINTRINSIC takes a transformational
  intrinsic's rows as scalars, left for here.
- **FTNRTL** is at GSMATCH 1,25; govax's own FOR numbers run 300 to 304.

## A.3 Findings, and what they change

1. **The table** (Decision 14, finished). Every row gets its class and dummies:
   Table D-3's by the standard's names (ABS (A), ATAN2 (Y, X), MAX (A1, A2, ...),
   MOD (A, P), SIGN (A, B), DIM (X, Y), INDEX (STRING, SUBSTRING, BACK?), LGE
   (STRING_A, STRING_B), ICHAR (C), CHAR (I, KIND?), the bit functions' I, J, POS,
   LEN, SHIFT, SIZE?), DEC's own by the HP manual's (SIND (X), ZEXT (X), QEXT (A),
   DFLOAT (A), SIZEOF (X); its ZEXT KIND is HP's, left out). Specific names take
   their generic's names. MAX and MIN's A3, ... are positional only. ISHFTC's
   SIZE becomes optional, BIT_SIZE (I) when absent (a FTNRTL call with the size
   given). Every new name joins `fortran90Table`, so under `/F90` it is intrinsic
   and under `/NOF90` only by an INTRINSIC statement (Decision 2), and nothing old
   changes (`TestNoF90Unchanged`). The kinds of IBITS, IBCLR, and ISHFTC of an
   INTEGER\*2 are question 3.
2. **The intrinsic subroutines** get a table of their own beside the functions
   (`intrinsicSubroutines`: name, dummies, optional, elemental), read by the same
   keyword code (`intrinsicSlots`), so `CALL DATE_AND_TIME (VALUES = V)` reorders
   into the dummies' order with absent arguments null (Phase 103's rule), and no
   explicit interface is asked for. Their names are intrinsic under `/F90` (and
   MVBITS, VAX FORTRAN's already, always); a unit's own of the name wins, as for
   the system subroutines. **MVBITS** is the standard's: FROM any integer
   expression (a temporary if not a place), FROMPOS, LEN, and TOPOS any integer
   kind converted to TO's, and **elemental**: with an array TO, a CALL in the
   scalarizer's loop nest, each element's call FTN$IMVBITS or FTN$JMVBITS
   (an INTEGER\*1 TO through a byte form, `FTN$BMVBITS`). This loosens VAX
   FORTRAN's rule (FROM was a variable) for programs that were errors; no valid
   program changes (question 8).
3. **Character functions**, each by cost (the plan's design note):
   - ACHAR and IACHAR are CHAR and ICHAR (the VAX's characters are ASCII; codes
     past 127 are the byte, as CHAR's): new rows, the same operations.
   - **LEN_TRIM** in line: a backward loop of a few instructions (the VAX has no
     reverse SKPC), a new `LenTrimOp`, 0 for an all-blank or empty string.
   - **TRIM (S)** needs no storage: it is `S (1 : LEN_TRIM (S))`, a substring of
     run-time length, which every consumer already takes (assignment,
     concatenation, I/O, comparison, an actual argument by descriptor). An S that
     isn't a variable or substring is first held in a temporary (as Phase 104's
     run-time-length values are).
   - **ADJUSTL, ADJUSTR**: the result is S's length; FTNRTL routines
     `FTN$ADJUSTL (dst, src)` and `FTN$ADJUSTR`, both by descriptor (SKPC and
     MOVC5 for the left, the trailing blanks counted then MOVC3 into place for the
     right), into a static `TextCell` when S's length is constant, else a class D
     string as concatenation's. On arrays, per element (Phase 104's rule for
     run-time-length elements).
   - **REPEAT (S, N)** into a destination the same way, `FTN$REPEAT (dst, src,
     n)` (MOVC3 in a loop); N < 0, or a length past 65,535, is a new condition,
     govax's own, FOR number 305, `%FOR-F-BADREPEAT` (question 6 has its wording).
   - **SCAN and VERIFY**: FTNRTL's `FTN$SCAN (string, set, back)` and
     `FTN$VERIFY`, descriptors and BACK by value (a 256-bit table of SET, then
     SPANC/SCANC forward, a loop backward); in line as LOCC or SKPC when SET is a
     constant of one character and BACK is absent or false, `VERIFY (S, ' ')`
     being the common case.
   - **INDEX (S, T, BACK)** with BACK true (or not constant): `FTN$INDEX_BACK`, a
     backward search by CMPC3; false or absent, today's MATCHC.
   - **Constants**: the evaluator folds ACHAR, IACHAR, LEN_TRIM, ADJUSTL,
     ADJUSTR, INDEX, SCAN, VERIFY, TRIM, and REPEAT of constant arguments, so
     7.1.6.1's initialization expressions work (`PARAMETER`, initial values, DATA,
     a length).
4. **Zero-length CHARACTER.** TRIM and REPEAT make zero lengths when running
   (MOVC3, CMPC, descriptors, LIB$SGET1_DD, and FTNRTL's I/O all take a length of
   0; each consumer gets a test with one). A *declared* `LEN = 0`, or a constant
   `''`, or a substring `S (5 : 4)`, is made a length known only when running
   whose expression is the constant 0 (`LengthExpr`, Phase 104's), with no
   storage, rather than a new representation beside size 0's "not known yet"
   (question 7).
5. **Numeric functions**: CEILING and FLOOR in line (`CVTxL` truncates; one
   compare and an INCL or DECL when the value wasn't whole: two new ops, of the
   default integer). MODULO in line: integer, MOD's code and `R + P` when R ≠ 0
   and its sign differs from P's; real, MTH$AMOD (DMOD, ...) and the same
   correction. P = 0: MOD's divide-by-zero trap for integers, MTHRTL's for reals.
   SELECTED_INT_KIND and SELECTED_REAL_KIND of a value that isn't constant: a
   chain of compares in line (SELECTED_REAL_KIND's by the unit's `/G_FLOATING`).
6. **COMPLEX(16)**, the rest of it (Phase 100's answer 1, Phase 102's list): ABS,
   SQRT, EXP, LOG, SIN, and COS as FTNRTL routines in MACRO, `FTN$CHABS` ...
   `FTN$CHCOS`, the value through a hidden first argument as MTH$H routines
   return theirs, each composed from MTHRTL's H routines (ABS by scaling to avoid
   overflow, then HSQRT; EXP by HEXP and HSINCOS; LOG by HLOG and HATAN2; SIN and
   COS by HSINCOS, HSINH, and HCOSH; SQRT by the usual half-angle formula); `**`
   by an integer in line by repeated squaring (`vaxfloat.PowerBySquaring`'s order,
   as COMPLEX\*8's), by anything else `FTN$CHPOW` (EXP (Y × LOG (X))); MERGE of
   COMPLEX(16) as of any 32-byte value. This answers the plan's open question 1:
   **new numeric routines are FTNRTL's**; MTHRTL's vector stays VMS's (question 1).
7. **The model functions** on each format's fields: EXPONENT (`EXTZV` of the
   exponent field, less the bias, 0 for 0), FRACTION and SET_EXPONENT (`INSV` of
   the bias, or bias + I) in line; SCALE, NEAREST, SPACING, and RRSPACING by
   FTNRTL, a routine per format (`FTN$SCALE_F`, `_D`, `_G`, `_H`, and so on; F, D,
   and G results in R0 and R1, H through a hidden argument). SCALE and
   SET_EXPONENT out of range: an overflow is made a real one (a MULx of HUGE by 2,
   so the program's handlers see `SS$_FLTOVF` as from any instruction), an
   underflow is 0 as the VAX's is with FU clear. NEAREST of 0 is ±TINY, of ±HUGE
   outward an overflow; SPACING of 0 or past the bottom of the range TINY.
   NEAREST with S = 0 is the standard's error, unchecked. A REAL\*8 is D or G by
   the unit (`analyzer.format`); D's 56 digits make its answers differ from
   IEEE's, so gfortran is the witness for REAL\*4 and G only (`compare.py`'s
   differences).
8. **TRANSFER**: SOURCE is computed whole into a contiguous temporary (Phase
   104's transformational temporaries, `ir/lowertransform.go`) of the larger of
   the two lengths, **zero-filled** past SOURCE (question 5), and the result is
   that storage taken with MOLD's type and shape: no second copy. A scalar to a
   scalar of a length known when compiling moves the bytes **as integers**
   (`RawConst`'s rule), so bits that are a reserved operand in MOLD's format don't
   fault in TRANSFER (a later arithmetic use faults, as on any VAX). Constants are
   folded (vaxfloat's bits), for initialization expressions. GENERICINTRINSIC
   learns TRANSFER's and the other transformational rows' ranks (Phase 103's
   deferral).
9. **DATE_AND_TIME** is `FTN$DATE_AND_TIME (date, time, zone, values)`, the
   strings by descriptor, VALUES by address, each 0 when absent, and a `_W` entry
   for an INTEGER\*2 VALUES (`/NOI4`). The date and time from `$NUMTIM`
   (milliseconds are hundredths × 10). The zone (question 4): govax defines
   **`SYS$TIMEZONE_DIFFERENTIAL`** in the system table at startup, as VMS V6 and
   later do (seconds east of UTC, as text), from the host's offset then, and
   FTNRTL translates it with `$TRNLNM`; without it ZONE is blanks and VALUES (4)
   -HUGE (0).
10. **SYSTEM_CLOCK** is `FTN$SYSTEM_CLOCK (count, rate, max)` (and `_W`): the
    rate 100 (the VAX clock's hundredths), the maximum HUGE of the default integer,
    the count the system time (`$GETTIM`, 100 ns units) in hundredths modulo the
    maximum + 1 (question 5). In the quantum clock's mode it follows the
    emulated clock, so a run is repeatable.
11. **RANDOM_NUMBER and RANDOM_SEED** (question 2, the plan's open question 2).
    FTNRTL keeps the seed (a longword in its own writable psect, per image) and
    draws with **MTH$RANDOM**, so the sequence is the same on every host, the VAX's
    own generator is used, and `vax.runtime.random` reaches it as it reaches RAN.
    A REAL\*4 is one draw; a REAL\*8 or REAL\*16 joins as many draws' 24 bits as the
    format's digits need (3 for D and G, 5 for H), the first the most significant.
    `FTN$RANDOM_F`, `_D`, `_G`, `_H` take one element's address; an array HARVEST
    is the scalarizer's loop over them, in array element order. RANDOM_SEED: N is 1
    (2 under `/NOI4`, the longword in two words), PUT sets it, GET returns it;
    `FTN$RANDOM_SEED (size, put, get)` and `_W`. The image starts with a fixed seed
    (the manual's suggested value), and CALL RANDOM_SEED () with no argument
    restores it.
12. **The corpus**: a program per subtask (`charfns.f90`, `numfns.f90`,
    `model.f90`, `bitsxfer.f90`, `clockrand.f90`), worked by hand; gfortran
    witnesses them but for the model's D answers, random values, and times
    (`clockrand.f90` prints only what is fixed: the seed's size, a sequence's
    repetition after PUT, a count that doesn't go backward). FTNRTL's routines are
    held by a MACRO test calling each entry (`TestFTNRTLIntrinsics`), its edge
    cases (zero lengths, ±TINY, ±HUGE, a reserved operand through TRANSFER).
13. **What waits**: `/STANDARD=F90`'s judgment of these names and the
    debugger's EXAMINE of their values, Phase 110; Fortran 95's additions
    (CEILING's and FLOOR's KIND, MAXLOC's DIM, NULL, CPU_TIME), Phase 112.

## A.4 The subtasks, revised

1. **Survey** (this Part A) (done 2026-11-19, for review).
2. **The table and keywords**: every row's class and dummies, the new names in
   `fortran90Table`, ISHFTC's optional SIZE, question 3's kinds, the intrinsic
   subroutines' table and MVBITS as the standard's (elemental too), GENERICINTRINSIC's
   transformational ranks, SELECTED_*_KIND of values (findings 1, 2, 5's last part,
   8's last part).
3. **Character functions** and zero lengths (findings 3, 4), FTNRTL's entries,
   GSMATCH 1,26, condition 305.
4. **Numeric functions and COMPLEX(16)** (findings 5, 6).
5. **The model functions** in F, D, G, and H (finding 7).
6. **TRANSFER** (finding 8).
7. **The subroutines**: DATE_AND_TIME with `SYS$TIMEZONE_DIFFERENTIAL`,
   SYSTEM_CLOCK, RANDOM_NUMBER, RANDOM_SEED (findings 9 to 11).
8. **Programs end to end**, every mode, gfortran's witness but for random numbers,
   times, and D's model (finding 12).
9. **Close-out**: FORTRAN-INTERNALS.md (a section on the intrinsic table, how each
   form is chosen, TRIM as a substring), MANUAL.md's 14.13, its FTNRTL table and
   Appendix B, HELP FORTRAN, HELP's `SYS$TIMEZONE_DIFFERENTIAL`.

## A.5 Questions for the author

1. **New numeric routines** (the plan's open question 1; findings 6, 7). (a)
   **FTNRTL, in MACRO, over MTHRTL's H routines; MTHRTL's vector stays VMS's**; (b)
   MTHRTL under govax's own names, in Go.
2. **RANDOM_NUMBER's generator** (the plan's open question 2; finding 11). (a)
   **MTH$RANDOM's, a one-longword seed held by FTNRTL (N = 1, or 2 under
   `/NOI4`), wider formats from several draws, a fixed starting seed that
   RANDOM_SEED () restores**; (b) a new generator of longer period and a wider seed
   in FTNRTL; (c) as (a), but RANDOM_SEED () seeds from the clock.
3. **IBITS, IBCLR, and ISHFTC of an INTEGER\*2** (finding 1). (a) **Under `/F90`
   the standard's (I's kind), under `/NOF90` the 1988 manual's (INTEGER\*4)**; (b)
   the manual's always; (c) the standard's always (an old program's result type
   changes).
4. **The time zone** (finding 9). (a) **`SYS$TIMEZONE_DIFFERENTIAL` defined at
   startup from the host's offset, read by FTNRTL** (its form on VMS 7.1 is
   unconfirmed; a one-line DCL check on the simh VAX would settle it); (b) no zone
   (blanks and -HUGE (0)).
5. **Choices the standard leaves open** (findings 8, 10). (a) **TRANSFER's
   undefined tail zero; SYSTEM_CLOCK's rate 100 and maximum HUGE of the default
   integer, the count wrapping**; (b) other values the author names.
6. **REPEAT's error** (finding 3): **a new condition, FOR number 305,
   `%FOR-F-BADREPEAT`, "REPEAT count negative or result too long"**, or another
   wording.
7. **A declared zero length** (finding 4). (a) **A run-time length whose
   expression is 0, through Phase 104's machinery**; (b) a new representation of
   length 0 beside size 0's "not known yet".
8. **MVBITS** (finding 2). (a) **The standard's rule under both settings** (FROM
   any expression, elemental; only programs that were errors change); (b) under
   `/F90` only.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-19:** Subtask 1, the survey (Part A), for review: what chapter 13 asks
  of the remaining intrinsics (character, numeric, the model, bits, TRANSFER, the
  five subroutines, keywords), what govax has (the table's two parts, the
  constant evaluator, the in-line operations, run-time-length text, the system
  subroutines, MTH$RANDOM, MTHRTL's H routines, the model's exactness), and
  thirteen findings: one table finished with every row's dummies and a table of
  subroutines; TRIM a substring needing no storage, LEN_TRIM in line, the other
  character functions FTNRTL's into a static or class D destination; a declared
  zero length through Phase 104's run-time lengths; COMPLEX(16)'s functions and
  powers in FTNRTL over MTHRTL's H routines; the model's three cheap functions in
  line and four in FTNRTL by format; TRANSFER as a temporary seen with MOLD's
  type; DATE_AND_TIME's zone from `SYS$TIMEZONE_DIFFERENTIAL`; RANDOM_NUMBER on
  MTH$RANDOM with FTNRTL's seed. The subtasks are revised (A.4, nine), with eight
  questions (A.5). Tried on govax: the new names are external functions today, a
  keyword to a Table D-3 function NOTIMPL, DATE_AND_TIME by keyword
  KEYWORDNOINTERFACE, MVBITS of a constant SYSARG. Tried on gfortran: its seed is
  8 integers, its clock's rate 1000, ADJUSTR, TRIM of blanks, SCAN with BACK,
  MODULO, and the model functions of REAL\*4 as the standard's examples give.
