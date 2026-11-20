# Phase 108 — Fortran 90: the intrinsic procedures

**Status:** done 2026-11-19. Planned 2026-11-17 with Phase 99 (`PHASE-99 -
Fortran 90.md`, Part C). Subtask 1, the survey (Part A), done 2026-11-19 and
accepted by the author the same day ("The author's answers"); subtasks 2 to 9
done 2026-11-19. "Where things stand", at the end, is what the phase left.

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
   -HUGE (0). **Settled on VMS 7.1** (the author's check on the simh VAX,
   2026-11-19): `SHOW LOGICAL SYS$TIMEZONE_DIFFERENTIAL` gives
   `"SYS$TIMEZONE_DIFFERENTIAL" = "-18000" (LNM$SYSTEM_TABLE)`, a signed decimal
   count of seconds (US Eastern standard time, UTC - 5 hours) in the system
   table. govax writes the same form; a positive offset is written without a `+`
   (unconfirmed: the VAX showed a negative one), and FTNRTL's reader takes an
   optional sign either way.
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

1. **Survey** (this Part A) (done 2026-11-19; accepted by the author the same
   day).
2. **The table and keywords**: every row's class and dummies, the new names in
   `fortran90Table`, ISHFTC's optional SIZE, question 3's kinds, the intrinsic
   subroutines' table and MVBITS as the standard's (elemental too), GENERICINTRINSIC's
   transformational ranks, SELECTED_*_KIND of values (findings 1, 2, 5's last part,
   8's last part) (done 2026-11-19).
3. **Character functions** and zero lengths (findings 3, 4), FTNRTL's entries,
   GSMATCH 1,27 (subtask 2's FTN$BMVBITS took 1,26), condition 305 (done
   2026-11-19).
4. **Numeric functions and COMPLEX(16)** (findings 5, 6), GSMATCH 1,28 (done
   2026-11-19).
5. **The model functions** in F, D, G, and H (finding 7), GSMATCH 1,29 (done
   2026-11-19).
6. **TRANSFER** (finding 8) (done 2026-11-19).
7. **The subroutines**: DATE_AND_TIME with `SYS$TIMEZONE_DIFFERENTIAL`,
   SYSTEM_CLOCK, RANDOM_NUMBER, RANDOM_SEED (findings 9 to 11), GSMATCH 1,30
   (done 2026-11-19).
8. **Programs end to end**, every mode, gfortran's witness but for random numbers,
   times, and D's model (finding 12) (done 2026-11-19).
9. **Close-out**: FORTRAN-INTERNALS.md (a section on the intrinsic table, how each
   form is chosen, TRIM as a substring), MANUAL.md's 14.13, its FTNRTL table and
   Appendix B, HELP FORTRAN, HELP's `SYS$TIMEZONE_DIFFERENTIAL` (done
   2026-11-19).

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

## The author's answers (2026-11-19)

The author read the survey and accepted every recommendation:

1. New numeric routines (COMPLEX(16)'s functions and powers, the model functions)
   are FTNRTL's, in MACRO over MTHRTL's H routines; MTHRTL's vector stays VMS's
   (the plan's open question 1).
2. RANDOM_NUMBER draws with MTH$RANDOM on a one-longword seed FTNRTL keeps (N = 1,
   2 under `/NOI4`), wider formats from several draws, a fixed starting seed that
   RANDOM_SEED () restores (the plan's open question 2).
3. IBITS, IBCLR, and ISHFTC of an INTEGER\*2 give I's kind under `/F90`, the 1988
   manual's INTEGER\*4 under `/NOF90`.
4. The zone comes from `SYS$TIMEZONE_DIFFERENTIAL`, defined at startup from the
   host's offset and read by FTNRTL. Its form on VMS 7.1 is settled by the author's
   check (finding 9): a signed count of seconds in LNM$SYSTEM_TABLE, `"-18000"`. No
   further probe is needed for the phase.
5. TRANSFER's undefined tail is zero; SYSTEM_CLOCK's rate is 100 and its maximum
   HUGE of the default integer, the count wrapping.
6. REPEAT's error is FOR number 305, `%FOR-F-BADREPEAT`, "REPEAT count negative or
   result too long".
7. A declared zero length is a run-time length whose expression is 0, through
   Phase 104's machinery.
8. MVBITS follows the standard's rule under both settings.

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
- **2026-11-19:** The author accepted every recommendation ("The author's
  answers"), and settled question 4's open point on the simh VAX: VMS 7.1's
  `SYS$TIMEZONE_DIFFERENTIAL` is `"-18000"` in LNM$SYSTEM_TABLE, a signed count of
  seconds (finding 9). No other probe is needed before the implementation.
- **2026-11-19:** Subtask 2, the table and keywords. Every row of `sema`'s
  `intrinsicTable` now has its class and dummies, Table D-3's by the standard's names
  (DEC's by the HP manual's), read by `readDummies`: `...` after MAX's and MIN's A1 and A2
  for more by position (`form.variadic`), `-` for NWORKERS's none, and a rank on a
  transformational or inquiry function's array dummies (`ARRAY[]`, `VECTOR[1]`,
  `MATRIX[2]`, `X[*]`). A specific name takes its generic's dummies but KIND
  (`dummiesOf`); `keywordSlots` (`intrinsickeys.go`) is the one ordering of keyword
  arguments for functions and subroutines, too few by position still INTRINARGS, as
  before, and MISSINGARG only after a keyword. The new names are `fortran90Table`'s:
  ACHAR and IACHAR as CHAR's and ICHAR's rows, done; the rest `LATER` rows, NOTIMPL by
  name ("the intrinsic function TRIM") until their subtasks, so under `/F90` they are no
  longer silently external functions. INDEX takes BACK (`.FALSE.` today's search, any
  other NOTIMPL until subtask 3, `indexBack`); ISHFTC's SIZE is optional, BIT_SIZE (I)
  of I's own type when left out (8 for an INTEGER(1), shifted as a word); CHAR takes
  KIND. Under `/F90` IBITS, IBCLR, IBSET, ISHFT, and ISHFTC are of I's kind, the other
  arguments converted to it before the row is chosen (`iKindFunctions`; question 3);
  under `/NOF90` Table D-3's rules stand. A fix: ISHFT of an INTEGER(1) zero-extends it
  to the word it is shifted in (`intrinsicArgs`), so `ISHFT(-1_1, -4)` is 15, not -1.
  SELECTED_INT_KIND and SELECTED_REAL_KIND of values not constant are a chain of MERGEs
  of constants on comparisons (`inquiry.go`'s `selectedValue`), each argument read once
  per comparison (unconfirmed: a function reference as the argument is called that many
  times; the standard leaves it open). GENERICINTRINSIC reads the table's ranks
  (`takenByIntrinsic`, `dummyShape`'s rank -2 for an array of any rank): a specific of SUM
  taking a scalar is told from SUM, one taking an array isn't. The intrinsic subroutines
  are `intrinsicsub.go`'s table (MVBITS elemental; DATE_AND_TIME, SYSTEM_CLOCK,
  RANDOM_NUMBER, and RANDOM_SEED NOTIMPL until subtask 7), intrinsic under `/F90` or by
  an INTRINSIC statement, MVBITS always, a unit of the file of the name winning; new
  messages INTRINSUBARGS and NULLINTRINSUB. MVBITS is the standard's under both
  settings (question 8): FROM any integer expression of TO's kind (a constant of another
  converted as assigned, INTRANGE past it; a value of another SYSLEN), the positions any
  integer kind converted to TO's, TO a variable, element, array, or section; on arrays a
  `Call` with a `Shape`, lowered by `ir`'s `elementalCall` as a loop nest calling the
  routine per element in array element order (TO may be FROM). FTNRTL's `FTN$BMVBITS`
  for an INTEGER(1) TO, at vector offset 736, GSMATCH 1,26. `testdata/fortran90/
  keywords.f90` runs it all in every mode, and gfortran prints the same
  (`compare.py`); `sema`'s `intrinsics108_test.go` holds the dumps and messages.
- **2026-11-19:** Subtask 3, the character functions and zero lengths. `sema/charfuncs.go`'s
  `characterFunction` takes LEN_TRIM, TRIM, ADJUSTL, ADJUSTR, REPEAT, SCAN, VERIFY, and INDEX with
  a BACK (the table's rows `LENTRIM` to `VERIFY`, no longer LATER; subtask 2's `indexBack` is gone)
  and picks each reference's form: of constants folded (`lenTrim`, `adjustLeft`, `scanText`,
  ...; the same code is the evaluator's, `constfuncs.go`'s new rows, so a PARAMETER or an initial
  value may use them, with ACHAR and IACHAR; INDEX there only under `/F90`, section 4.11 not having
  it); LEN_TRIM, TRIM, and SCAN and VERIFY of one constant character with a constant BACK in line
  (new ops `LenTrimOp`, `TrimOp`, `ScanOp`, `VerifyOp`; SCAN forward of one character is INDEX's
  MATCHC); the rest FTNRTL's (`AdjustLOp`, `AdjustROp`, `RepeatOp` with their routine, and a
  `CallOp` of `FTN$SCAN`, `FTN$VERIFY`, or `FTN$INDEX_BACK`). BACK goes **by reference**, a
  default LOGICAL, `.FALSE.` when left out, where finding 3 said by value: `callIntrinsic`'s
  arguments are all by reference, and FTNRTL's `BLBS @12(AP)` costs nothing; and
  `FTN$INDEX_BACK` takes BACK too, searching forward (by `FTN$INDEX`) when it is false, so a
  BACK not constant needs no branch in line. An array BACK is NOTIMPL, named. `ir/lowercharfuncs.go`:
  `search` is one loop for LEN_TRIM, SCAN, and VERIFY in either direction, the string's address
  and length in temporaries; TRIM is its argument's text with LEN_TRIM's length, no storage;
  `madeByRoutine` gives ADJUSTL and ADJUSTR a `$Tn` of the string's length (a dynamic string when
  that isn't known) and REPEAT a `$Tn` when its length is known, otherwise an empty class D
  descriptor that FTN$REPEAT gives storage (LIB$SGET1_DD) once it has checked the count, freed
  at the statement's end. FTNRTL's six routines are in `ftn_intrinsics.mar` (ADJUSTL by SKPC and
  MOVC5, ADJUSTR by a backward count, SCAN and VERIFY by a 256-byte table on the stack and SCANC or
  SPANC forward, a loop backward, INDEX_BACK by CMPC3 from the last place), at vector offsets 744
  to 784, GSMATCH 1,27; `%FOR-F-BADREPEAT`, FOR number 305, is in `vmsdef/formessages.go`.
  Zero lengths (finding 4): under `/F90` a constant length below 1 (`LEN=0`, `*0`, which the parser
  now keeps as `*(0)` since `Len.Length` 0 means none, `*(-4)`) is a `LengthExpr` with the constant
  0 as its `LengthValue`, `Symbol.ZeroLength`, no storage and nothing computed at entry
  (`runtimeLength`, `prepareLength`, `charLength`, `lengthWord`); a PARAMETER of length zero
  takes none of its value. `/NOF90` keeps BADCHARLEN (and `*0`, which used to be read as no
  length, is now BADCHARLEN there too). A component, or a function's value, of length zero
  is NOTIMPL, named ("a CHARACTER length of zero here"): a field's size of 0 means something
  else to the structure code. A substring with constant bounds, the lower past the upper, is
  empty under `/F90` (`emptySubstring`, its upper made the lower less 1; SUBRANGE and SUBORDER
  under `/NOF90`), and under `/F90` a run-time substring's length below 0 is made 0, and
  `/CHECK=BOUNDS` checks its bounds only when it isn't empty (`checkedF90Substring`, a block the
  empty substring jumps past). `testdata/fortran90/charfns.f90` runs it all in every mode, and
  gfortran prints the same (`compare.py`); `sema`'s `TestPhase108Characters` and
  `TestPhase108ZeroLengths` hold the forms, the folding, and the messages. MANUAL.md's 14.13
  (and two sentences subtask 2 left stale: SELECTED\_ of values, keywords to every function) and
  its FTNRTL table, and HELP FORTRAN, have them. The MACRO test of FTNRTL's entries (finding 12's
  `TestFTNRTLIntrinsics`) waits for subtask 8, the corpus calling each routine meanwhile.
- **2026-11-19:** Subtask 4, the numeric functions and COMPLEX(16). CEILING, FLOOR, and
  MODULO are table rows (`CEIL`, `FLOOR`, `MODULO`; CEILING and FLOOR of each REAL kind, a row
  per default integer as INT's, so `/NOI4` gives INTEGER\*2; MODULO of INTEGER\*2 and \*4,
  INTEGER(1) through `byteResult`, and each REAL), new ops `CeilingOp`, `FloorOp`, and
  `ModuloOp` (`sema`'s `wholeOrModulo`; a REAL MODULO's `Routine` is MOD's row's,
  MTH$AMOD, DMOD, GMOD, or HMOD). `ir/lowerintrinsic.go`'s `wholeNumber` is CVTxL into a
  longword temporary, then a comparison of A with it made REAL again and an add or
  subtract of 1 (exact: a value too large for a fraction was whole); `modulo` is MOD's code,
  or the MTHRTL call with both arguments from temporaries so P is computed once, then
  `giveSign`, P added when the remainder isn't 0 and its sign isn't P's (one comparison for a
  constant P). The evaluator folds all three (`constfuncs.go`'s `wholeFunction`,
  `constModulo`, exactly and rounded once), so a reference of constants, in an executable
  statement too, is a constant, and `MODULO(10, 0)` is DIVZERO as `MOD(10, 0)` is. **A
  PARAMETER may use them with REAL arguments**, as VAX FORTRAN's section 4.11 has NINT and ABS,
  though the standard's 7.1.6.1 takes only INTEGER and CHARACTER arguments to elemental
  functions there (gfortran `-std=f95` refuses them), so the corpus program keeps to INTEGER
  ones. COMPLEX(16) (finding 6): ABS, SQRT, EXP, LOG, SIN, and COS are `FTNC` rows, calls of
  FTNRTL's `FTN$CHABS` to `FTN$CHCOS` (`ftn_intrinsics.mar`, over MTH$HSQRT, HEXP, HLOG,
  HSINCOS, HSINH, HCOSH, and HATAN2: ABS as m·SQRT(1 + (n/m)²), SQRT by the half-angle formula
  with each part halved first, LOG's imaginary part ATAN2's), the value through the hidden
  first argument (`callIntrinsic` now passes one for a `Hexa`, and `assignHexa` moves such a
  value, and MERGE's, as two octawords). Powers: **only a constant exponent from 1 to 8 is in
  line** (`ir`'s `hexaPower`, `hexaProduct`), where finding 6 said any INTEGER exponent;
  another INTEGER exponent calls `FTN$CHPOWJ (result, x, n)`, PowerBySquaring's order with
  `CMUL`, the same five instructions as the in-line product, so the two agree to the bit (the
  corpus compares them), a negative exponent each square's reciprocal by FTN$DIVCH, and (0, 0)
  to a power of 0 or less MTH$_UNDEXP as OTS$POWCJ's; a COMPLEX (or REAL) exponent calls
  `FTN$CHPOW`, EXP(Y × LOG(X)), (0, 0) MTH$HLOG's LOGZERNEG whatever Y is, as OTS$POWCC's.
  FTNRTL's eight entries are at vector offsets 792 to 848, GSMATCH 1,28. MERGE of COMPLEX(16)
  needed only sema's refusal gone. A COMPLEX(16) argument to a function with no COMPLEX rows at
  all (TAN, CEILING) is now BADARG, not NOTIMPL (`takesComplex`); SUM and the other functions
  with COMPLEX rows but not COMPLEX(16)'s stay NOTIMPL. A fix the corpus found: the constant
  pool's key (`ir/lower.go`'s `constant`) left out a COMPLEX\*32's imaginary half, so `(0.0_16,
  0.5_16)` and `(0.0_16, 1.0_16)` in one unit shared one datum. `testdata/fortran90/numfns.f90`
  runs it all in every mode, and gfortran prints the same (`compare.py`); `sema`'s
  `TestPhase108Numeric` holds the forms, the folding, and the messages. MANUAL.md's 14.13 and its
  FTNRTL table, and HELP FORTRAN, have them.
- **2026-11-19:** Subtask 5, the model functions. EXPONENT, FRACTION, NEAREST, RRSPACING, SCALE,
  SET_EXPONENT, and SPACING are `MODEL` rows, no longer LATER, read by `sema/model.go`'s
  `modelFunction` before a row is chosen (their arguments' types differ: X a REAL, I an INTEGER
  made INTEGER\*4, S a REAL of any kind left as it is, since only its sign bit, bit 15 of the
  first word in every format, is read). Of constants all seven are folded (`constfuncs.go`'s
  `constModel`) by vaxfloat's model functions (`model.go`: `Exponent`, `Fraction`, and
  `Format`'s `Scale`, `SetExponent`, `Spacing`, `RRSpacing`, `Nearest`, exact on big.Float's
  MantExp, held to the standard's examples and each format's edges by `TestModelFunctions`), a
  result past the range REALRANGE. EXPONENT and FRACTION are written in line (new ops
  `ExponentOp`, `FractionOp`; `ir/lowermodel.go`): X stored into a text temporary of its own,
  whose first longword and word are then read and changed as integers, an EXTZV of the
  exponent field less the bias (0 kept for 0), and for FRACTION a BICW and BISW of the field
  where it isn't 0 (the bias's bits are ^X4000 in every format). **SET_EXPONENT is FTNRTL's**,
  where finding 7 put it in line: it needs SCALE's range checks, and shares SCALE's code there
  but for its base (the bias rather than X's field). The other five are `FTN$name_F`, `_D`, `_G`,
  `_H` (`ftn_intrinsics.mar`, 20 entries at vector offsets 856 to 1008, GSMATCH 1,29): each entry
  points R11 at its format's parameters (`FORMAT_F` ...: the field's position and size, the bias,
  the largest field, the digits, the longwords) and branches to the function's code, which runs
  in the entry's frame on X's longwords copied as integers (never a floating instruction on X, so
  a reserved operand doesn't fault there; it is copied as it is or, by NEAREST and RRSPACING, as
  any value with a field of 0). SCALE's I is kept within ±65,536 so the sum can't overflow. NEAREST
  adds or takes 1 from the magnitude as a whole number of words, the most significant first, by
  INCW and DECW with the carry; from 0 it is ±TINY, toward 0 from ±TINY 0. A result past the range
  is a real overflow (`TOO_LARGE`: the largest exponent, X's sign, doubled by MULx2), so a handler
  sees `SS$_FLTOVF_F` as from any instruction; below it, 0. `testdata/fortran90/model.f90` runs
  the standard's examples folded, at run time, on arrays, and by keyword, of F, D, and H, in every
  mode, and gfortran prints the same but D's two lines (`compare.py`'s KNOWN: 56 digits, not 53);
  `modelvms.f90` is the VAX's edges, TINY and HUGE in F, D, G (a unit under `OPTIONS
  /G_FLOATING`), and H, and every overflow, each caught by LIB$SIG_TO_RET as `%X000004B4`.
  `sema`'s `TestPhase108Model` holds the forms, the folding, and the messages. MANUAL.md's 14.13
  and its FTNRTL table, and HELP FORTRAN, have them.
- **2026-11-19:** Subtask 6, TRANSFER. The table's row is `TRANSFER`, no longer LATER (the last
  LATER row, so the mechanism is gone: `intrinsicRef`'s check and the table's paragraph).
  `sema/transfer.go`'s `transfer` takes SOURCE and MOLD as `operand`s (any type, a record's too,
  scalar or array; MOLD only for its type and rank) and SIZE as an INTEGER scalar; the result is a
  scalar `IntrinsicRef` of the new `TransferOp` when MOLD is a scalar and SIZE is left out, else an
  `ArrayFunc` named TRANSFER (`Array` SOURCE, `Other` SIZE), now among `transformational`'s, so
  `Transformed`: its extent SIZE's if constant, else, with no SIZE, the bytes over MOLD's size if
  SOURCE's size is known, else the function's own. Of a constant SOURCE (a `Const` or an
  `ArrayConst`) and a constant or absent SIZE it is folded, whatever MOLD is, by `transferValue`
  (the bytes by `valueBytes`, `constBytes` or a constant's `Raw`; each element by
  `constFromBytes`, a REAL or COMPLEX whose bytes don't pack back from its value keeping them as
  `Raw` with the value the VAX's arithmetic takes them as, 0 for a reserved operand or a zero with
  fraction bits); the constant evaluator does the same (`transferIsArray`, `transferConst`;
  TRANSFER among `arrayFunctions`), so a PARAMETER may use it. Folded only to a number, a LOGICAL,
  or a CHARACTER value: a derived type's constant needs its components (`Const.Parts`), so TRANSFER
  to a derived type is computed when running even of constants, and in a PARAMETER isn't constant
  (unconfirmed: the standard allows it). A MOLD of a CHARACTER length known only when running, or
  of zero, is NOTIMPL, named; so is a CHARACTER array SOURCE of a run-time length that isn't a
  name's or a section's. `ir/lowertransfer.go`: the value is a temporary of the result's size,
  SOURCE's bytes moved in, the rest cleared, then read as MOLD's type (`transferPlace` for a
  scalar, through `intrinsic`, `char`, `mem`, `complexParts`, `complexValue`; `transferArray` for
  an array, `transformed`'s TRANSFER case, `keepTransformed`). **Where finding 8 had the temporary
  of the larger length**, only the result's is made: SOURCE's leading bytes are moved and the rest
  of the result cleared (`moveFilled`: MOVC3 and `clearBytes`' CLRO to CLRB when both lengths are
  constants of a word's length, else LIBRTL's `OTS$MOVE5` with a fill of 0, whose lengths are
  longwords), which is the same value without a second copy. A scalar number or LOGICAL SOURCE is
  stored in the temporary as itself (`storeSource`), a REAL or COMPLEX variable, element, or field as
  an integer of its size; an array SOURCE is moved from where its elements are (a whole array that
  isn't strided) or through `lowersequence.go`'s `sequence` (a strided section, or an array
  expression, copied first); a record by its bytes; a CHARACTER value by `chars`. A scalar TRANSFER
  assigned to a REAL variable of its type is moved as an integer (`store`'s `rawMove`), and one
  assigned to a COMPLEX moved whole (`assignComplex`, `assignHexa`), so a reserved operand passes
  through TRANSFER and the assignment without a fault; an array's elements are read as their type
  by the statement's loops, where such bits fault as any REAL's would. Folding a reserved
  operand's bits into an expression computed when compiling takes them as 0 (unconfirmed: the VAX
  would fault). `testdata/fortran90`'s `transfer.f90` (gfortran prints the same, `compare.py`) and
  `transfervms.f90` (the VAX's bits, the zero tail, a reserved operand through and back) run in
  every mode; `sema`'s `TestPhase108Transfer` holds the forms, the folding, PARAMETERs, the
  messages, and `/NOF90`. The corpus README gained the rows subtasks 4 and 5 left out (`numfns`,
  `model`, `modelvms`). MANUAL.md's 14.13 and HELP FORTRAN have TRANSFER.
- **2026-11-19:** Subtask 7, the subroutines. `sema/clocksubs.go` gives the intrinsic
  subroutines' table its last four analyzers (`intrinsicsub.go`'s NOTIMPL is gone): each CALL is a
  call of FTNRTL's routine for its arguments, chosen as the system subroutines' are, an argument
  left out a null one (the longword 0). `DATE_AND_TIME` takes CHARACTER variables, elements, or
  substrings by descriptor and VALUES, an integer array of rank 1 and at least eight elements, by
  address; `SYSTEM_CLOCK` three integer variables or elements; `RANDOM_NUMBER` a REAL of any kind,
  scalar or array (`Call.Shape`, ir's `elementalCall`, so an array is filled in array element
  order), its routine by format, D or G by the unit; `RANDOM_SEED` at most one argument (a new
  message, `SEEDARGS`), PUT any integer array value and GET a definable one. **Every integer
  argument may be INTEGER\*2 or INTEGER\*4 under either `/I4` setting**, all of one length (SYSLEN),
  where the standard says the default kind: the routine's `_W` form is for words. An array argument
  goes as its elements one after another (`sequence.go`: a section that isn't is copied, and for
  VALUES and GET copied back); a whole array named as an argument has no shape of its own, so
  `argumentShape` takes its symbol's. FTNRTL's ten entries are in `ftn_system.mar` at vector
  offsets 1016 to 1088, GSMATCH 1,30: `FTN$DATE_AND_TIME` and `_W` (`$NUMTIM`, the texts by a
  digit writer, `DIGITS`, each moved as an assignment moves it; the zone by `$TRNLNM` of
  `SYS$TIMEZONE_DIFFERENTIAL` **through LNM$FILE_DEV**, where finding 9 said the system table, so a
  process may define its own and the corpus can fix it; an optional sign and digits, else blanks and
  -HUGE of the kind), `FTN$SYSTEM_CLOCK` and `_W` (`$GETTIM`'s quadword over 100,000 in two EDIVs,
  the high half's remainder less 50,000 when it is that or more, so the quotient fits a longword and
  only its bit 31 changes; words modulo 2\*\*15), `FTN$RANDOM_F` to `_H` (MTH$RANDOM on `SEED`, a
  longword in `$FTN_DATA`; D, G, and H join 3, 3, and 5 draws by Horner's rule from the last, the
  last cut to 8, 5, or 17 bits, so every step is exact and none rounds to 1), and
  `FTN$RANDOM_SEED` and `_W` (the seed's size 1 or 2; an INTEGER\*2 array's two words, low first,
  are the longword's own bytes). **The first seed is 1234567**, govax's choice: the manual's
  MTH$RANDOM suggests none, and the Language Reference Manual's RAN (D.4.7) asks only for "a large,
  odd integer value". `lnm/timezone.go` defines `SYS$TIMEZONE_DIFFERENTIAL` in the system table,
  executive mode, as the database is made, from the host's offset then (`TimeZoneDifferential`, no
  `+` for a positive one, unconfirmed). `testdata/fortran90/clockrand.f90` prints only what any
  processor must, and gfortran prints the same (`compare.py`); `clockvms.f90` is govax's own
  answers, worked from the generator's formula, with the zone set by LIB$SET_LOGICAL; both run in
  every mode. `sema`'s `TestPhase108Clock` holds the routines, the arguments present, and the
  messages. MANUAL.md's 14.13 and its FTNRTL table, and HELP FORTRAN, have them; HELP's own topic
  for `SYS$TIMEZONE_DIFFERENTIAL` is subtask 9's.
- **2026-11-19:** Subtask 8, programs end to end. Finding 12's MACRO test of FTNRTL's entries is
  `internal/console`'s `TestFTNRTLIntrinsics` (`ftnrtlintrinsics_test.go`), in both paging modes:
  a program made from a table of cases (`ftnrtlCases`: the code, how the value is shown, what it
  must show), calling every routine the phase added, FTN$BMVBITS and the 44 at vector offsets 744
  to 1088, each at its edges as the compiler never reaches them: ADJUSTL and ADJUSTR of blanks and
  of an empty string, REPEAT into a static and a dynamic result, of an empty string, 0 times, and
  BADREPEAT for a negative count and a value past 65,535; SCAN, VERIFY, and INDEX_BACK of empty
  strings and sets; COMPLEX(16)'s functions at their exact points, (0, 1) to the powers 2 and -1,
  (0, 0)\*\*0's UNDEXP and (0, 0)\*\*(1, 0)'s LOGZERNEG; the model's 20 entries at 1.0, 0, ±TINY,
  and ±HUGE, every overflow a `SS$_FLTOVF_F` and every underflow 0, and NEAREST toward 0 from -TINY
  0, not a reserved operand; SYSTEM_CLOCK's rate and maximum of each kind; the seed's size of each
  kind, its first value, a draw MTH$RANDOM's own, the seed after one and after three draws (worked
  from the formula), D's sequence repeated after PUT, G's and H's numbers within [0, 1), a seed put
  as two words, and RANDOM_SEED with null arguments and with none restoring the first seed;
  DATE_AND_TIME's zone defined, changed, nonsense, cut short and padded, and, once the process's
  name is deleted, the system table's from the host's offset (the test works it from Go's), with
  VALUES(4) of each kind, and an empty DATE. Each value is shown as its longwords by `$FAOL`, every
  one worked from the formats by hand; two of the hand values were wrong (H's SPACING and RRSPACING
  of 1.0 are fields ^X3F91 and ^X4071: the bias is 16,384, 1.0's field 16,385), and the routines
  right. Two programs end to end in `testdata/fortran90`, both worked by hand and both printing the
  same on gfortran (`compare.py`), in every mode and with paging on: `textfmt.f90`, a text
  formatter over `textfmt.in` (blanks closed up by INDEX and an overlapping substring assignment,
  words by VERIFY and SCAN, lines filled and printed flush left, by ADJUSTR, and centered by REPEAT,
  titles capitalized by IACHAR and ACHAR, the last word by INDEX with BACK, a table of counts), and
  `calendar.f90` (day numbers by the civil calendar's eras with MODULO, weekdays before 1970, weeks
  by CEILING, whole months by FLOOR both ways, dates packed by MVBITS, turned by ISHFTC, unpacked by
  IBITS, a name folded by TRANSFER and IEOR, two months laid out, the model's functions at powers of
  two). They found two things. **A fix**: under `/F90` a substring of a constant, a named one's or
  an array constant's element's (`MONTHS(11)(1:8)`, `INITIALS(I:I)`), is the standard's constant
  subobject (R701, R702, R610's scalar-constant), where sema refused it with SUBSTR or BADUSE:
  `substring` takes a CHARACTER `Const` as the parent, folds it when both bounds are constants, and
  otherwise leaves a `Substring` of the constant, which ir's `chars` already reads from the
  constant's datum; `target` refuses one as an assignment's target (BADUSE, `substringBase`), and
  `definable` an element or section of an array constant for an INTENT(OUT) or INTENT(INOUT) dummy;
  under `/NOF90` nothing changes (`sema`'s `TestConstantSubstrings`; MANUAL.md's 14.13). **Left for
  Phase 109**: FTNRTL's format interpreter has no ES (or EN) editing, which ends a statement with
  SYNERRFOR at run time (a character constant's format isn't checked when compiling, as VAX
  FORTRAN's isn't); `calendar.f90` uses E instead. gfortran's witness over the whole Fortran 90
  corpus: all 93 programs the same.
- **2026-11-19:** Subtask 9, the close-out. FORTRAN-INTERNALS.md's section 33, "Fortran 90: the
  intrinsic procedures", follows a short program through the phase: the table's eight columns and
  the three ways it is read (typing, `keywordSlots`, generic resolution), the four forms a reference
  may take and how one is chosen, LEN_TRIM's loop, CEILING and EXPONENT side by side (the first
  ordinary IR, which forwarding folds, the second through memory, which it doesn't), TRIM as a
  substring needing no storage, ADJUSTR into a temporary and REPEAT's class D, a declared zero
  length as a run-time length of 0, the model functions on the VAX formats' fields with FTNRTL's
  per-format entries, TRANSFER as bytes, the four subroutines, and what the programs found; its
  MACRO is govax's own `/OPTIMIZE` output. MANUAL.md's 14.13 names the intrinsic procedures in its
  introduction, and its list of what isn't there loses "Fortran 90's other new intrinsic functions"
  for what of them is still missing (an array BACK, TRANSFER to a CHARACTER MOLD of zero or run-time
  length, a zero-length component or function value, COMPLEX(16) in SUM and the like and in I/O,
  ES and EN editing); Appendix B has a paragraph on FTNRTL, pointing at 14.6's table. HELP STARTUP
  has a TIME_ZONE topic for `SYS$TIMEZONE_DIFFERENTIAL` (its example checked on govax), which HELP
  FORTRAN points to, and HELP FORTRAN says ES and EN aren't there; the help library rebuilt (`go
  generate ./internal/bootdata`). PLAN.md and CLAUDE.md summarize the phase. Noticed, not changed:
  EXPONENT and FRACTION store X in their temporary with MOVF, so a reserved operand faults there
  where the FTNRTL model routines never do (any other use of such an X faults too).

## Where things stand

Phase 108 is done. What it left, and where each goes:

- **Phase 109:** ES and EN editing in FTNRTL's format interpreter (today SYNERRFOR at run time;
  `calendar.f90` uses E); COMPLEX(16) in I/O lists.
- **Phase 110:** `/STANDARD=F90`'s judgment of the new names and forms (a REAL argument to
  CEILING, FLOOR, or MODULO in a PARAMETER, which VAX FORTRAN's rules allow and the standard's
  7.1.6.1 doesn't); the debugger's EXAMINE of their values.
- **Phase 112:** Fortran 95's additions: CEILING's and FLOOR's KIND, MAXLOC's and MINLOC's DIM,
  NULL, CPU_TIME.
- **Not planned, ours (NOTIMPL, named):** a BACK= that is an array; TRANSFER to a CHARACTER MOLD of
  zero or run-time length, or of a CHARACTER array SOURCE of a run-time length that isn't a name's
  or a section's; a derived type's component, or a function's value, of length zero; SUM,
  PRODUCT, and the other functions with COMPLEX rows but not COMPLEX(16)'s, of a COMPLEX(16).
- **Noticed, not changed:** EXPONENT and FRACTION move X with MOVF into their temporary, so a
  reserved operand faults there; TRANSFER to a derived type is computed when running even of
  constants, so it can't give a PARAMETER its value.
- **Unconfirmed choices of ours:** the first seed, 1234567; a positive
  `SYS$TIMEZONE_DIFFERENTIAL` without a `+`; DATE_AND_TIME reading the name through LNM$FILE_DEV
  rather than the system table alone; SELECTED_INT_KIND and SELECTED_REAL_KIND of values reading
  an argument once per comparison; a reserved operand's bits folded into a constant expression as
  0; INTEGER\*2 arguments to the four subroutines under either `/I4` setting.
