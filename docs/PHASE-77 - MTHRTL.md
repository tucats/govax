# Phase 77 — MTHRTL: VMS's mathematics library, emulated

**Status:** in progress (subtask 1, the survey and capture, 2026-10-30,
its Decision 3 accepted by the author the same day; subtasks 2, the package
and its registration, and 3, the engine, square root, exponentials, and
logarithms, the same day). Needs Phase 74 (the plan)
only: this phase is independent of the compiler and could be done at any
time before Phase 78. Part of the FORTRAN program laid out in `PHASE-74 -
FORTRAN plan.md` (Decision 14).

## Goal

Add VMS's mathematics run-time library, MTHRTL.EXE, to govax's RTL
support, as LIBRTL is: Go routines a program reaches through stubs, for
**any** program (MACRO, govax's FORTRAN, or an image VMS's compilers
made), not part of FTNRTL. This was the author's direction
(2026-10-29). It covers:

- the **MTH$ routines** in the VMS 5.0 *MTH$* manual, each in the
  floating formats it has (F, D, G, H): square root, exponential and
  logarithms, the trigonometric functions in radians and degrees and
  their inverses, the hyperbolic functions, ATAN2, the complex
  functions (CABS, CEXP, CLOG, CSQRT, CSIN, CCOS, and the D and G
  forms), the random number generator (MTH$RANDOM), and the
  integer and bit functions FORTRAN names as MTH$ routines;
- the **OTS$ routines** the compiler needs that VMS keeps with them:
  the powers (OTS$POWxx for every base and exponent type pair) and
  complex division (OTS$DIVC and its D and G forms);
- **their conditions** as the manual gives them (MTH$_LOGZERNEG,
  MTH$_SQUROONEG, MTH$_UNDEXP, MTH$_FLOOVEMAT, MTH$_INVARGMAT, ...),
  signaled with the manual's arguments;
- **JSB entry points** (`MTH$SIN_R4` and the like) where the manual
  documents them, through Phase 54's JSB shim mechanism.

## What earlier phases leave in place

- **`internal/librtl`** (Phases 34, 53–59) is the model: a routine
  table, `XFC$SHIM` codes, stubs placed by `EnsureShims`, JSB entries
  (`JSBRoutines`), missing-routine stubs, and descriptor helpers.
- **`machine.runtimeLibraries`** already lists MTHRTL, so programs
  linked against it activate, with every routine resolving to a
  NOTIMPL stub today.
- **`vmsdef`** has the MTH$ condition values and 15 MTH message texts,
  and `vmsdef/gen -image` captures a shareable image's global symbol
  table.
- **`internal/vaxfloat`** unpacks and packs every format, and rounds
  exactly.

## Design notes

- **A package of its own**, `internal/mthrtl`, registered beside
  `internal/librtl`. Its routines take their arguments by the manual's
  mechanisms and return F and integer results in R0 and D and G results
  in R0:R1. Per the calling standard, H and D/G complex results come
  back through a first argument.
- **Accuracy:** every form, F, D, G, and H, is computed with `math/big` at
  the format's precision plus guard bits, then rounded once with
  `vaxfloat` (the survey's Decision 3, which replaced the plan's `float64`
  for F and G). Results should be within one ulp of the true value. The
  tests compare each form against a high-precision reference.
- **The vector** (the author's answer to Phase 74's review question 2):
  MTHRTL's global symbol table is captured from VMS 7.1's MTHRTL.EXE on
  the local system disk (`testdata/disks/rq0-ra92.dsk`) with
  `vmsdef/gen -image`, so each routine's offset is VMS's and images
  VMS's compilers made call the right routine. LINK then gets MTHRTL's
  symbols from govax's tables, as it does LIBRTL's.
- **Which library holds the OTS$ powers** on VMS 7.1 (MTHRTL or
  LIBRTL) is settled by the captured tables; the routine goes where VMS
  put it.

## The survey (subtask 1)

### Sources

- VMS 5.0 *MTH$* manual (`AA-LA72A`) and *OTS$* manual (`AA-LA73A`), both
  OCR, read with their errors in mind; the 7.3 *MTH$* manual
  (`OVMS_73_RTL_MTH.PDF`), whose text is clean.
- MTHRTL.EXE from VMS 7.1's system disk (`testdata/disks/rq0-ra92.dsk`,
  `[VMS$COMMON.SYSLIB]`): its global symbol table, captured into `vmsdef`,
  and the first eight bytes of each transfer vector entry, read once to
  tell CALL entries from JSB entries. The code behind the vector was not
  read. The image stays local.

### What MTHRTL.EXE holds

Its header says 337 pages, ident major 129 minor `0x800D`, match LEQUAL,
and 404 universal symbols: 376 `MTH$` and 28 `OTS$`. It has no `MTH$_`
condition values; those are STARLET's, and `vmsdef.LibrarySymbols` already
has them.

An entry is 8 bytes. A CALL entry is a two-byte mask and a `JMP`; a JSB
entry is a `JMP` alone. That settles the mechanism of every name:

- **262 CALL entries, 142 JSB entries.** Every JSB entry's name ends
  `_Rn`, and every `_Rn` name that isn't a JSB entry is an `OTS$` one.
- **The `OTS$..._R3` routines are CALL entries** (11 of them), which the
  manual doesn't say outright. Their arguments are by value in the argument
  list and their results are in R0:R3. Their masks leave R2 and R3 unsaved,
  which is how a result gets back through `RET`.
- **Two pairs of names share an entry:** `MTH$IIFIX`/`MTH$IINT` and
  `MTH$JIFIX`/`MTH$JINT`. So 402 entries, and the table must allow a stub
  with two names.
- **Ten JSB names are another name's second entry:** both `JMP` to the same
  address. `ACOS_R4`/`_R5`, `ASIN_R4`/`_R5`, `TAN_R4`/`_R5`,
  `TAND_R4`/`_R5`, `DACOS_R7`/`_R9`, `DASIN_R7`/`_R9`, `GACOS_R7`/`_R9`,
  `GASIN_R7`/`_R9`, `DEXP_R6`/`_R7`, `GEXP_R6`/`_R7`.
- **The masks' IV bit** (integer overflow enable) is set on 97 CALL entries.
  It is set on the integer-result routines Table 1-1 footnotes (`JIFIX`,
  `JNINT`, `IMOD`, `JIABS`, ...) and on `OTS$POWII` and `OTS$POWJJ`. It is
  clear on many others, such as `MTH$EXP`. So section 1.2's "All CALL entry
  points ... enable integer overflow" doesn't hold literally, and it doesn't
  matter to a Go routine.

### The routines

Counts of names in MTHRTL.EXE. "JSB, manual" is a JSB entry either manual
names. "JSB, not in the manuals" is one neither names.

| Family | CALL | JSB, manual | JSB, not in the manuals | Subtask |
|---|---:|---:|---:|---:|
| Square root (`SQRT`, D, G, H) | 4 | 4 | 1 | 3 |
| Exponential (`EXP`, D, G, H) | 4 | 4 | 2 | 3 |
| Logarithms (`ALOG`, `ALOG2`, `ALOG10`; D, G, H) | 12 | 8 | 0 | 3 |
| `SIN`, `COS`, `TAN`, `SINCOS`, radians | 16 | 16 | 2 | 4 |
| `SIND`, `COSD`, `TAND`, `SINCOSD`, degrees | 16 | 16 | 2 | 4 |
| `ASIN`, `ACOS`, `ATAN`, radians and degrees | 24 | 24 | 6 | 4 |
| `ATAN2`, `ATAND2` | 8 | 0 | 6 | 5 |
| `SINH`, `COSH`, `TANH`, `ATANH` | 16 | 0 | 9 | 5 |
| Absolute value (`ABS`, D, G, H, `IIABS`, `JIABS`) | 6 | 0 | 0 | 6 |
| Positive difference (`DIM`, ...) | 6 | 0 | 0 | 6 |
| Sign (`SIGN`, ..., `SGN`) | 7 | 0 | 4 | 6 |
| Greatest integer (`FLOOR`, ...) | 4 | 4 | 0 | 6 |
| Truncation (`AINT`, ..., `JIFIX`, ...) | 14 | 4 | 0 | 6 |
| Nearest integer (`ANINT`, ..., `JNINT`) | 12 | 0 | 0 | 6 |
| Remainder (`AMOD`, ..., `IMOD`, `JMOD`) | 6 | 0 | 3 | 6 |
| Maximum, minimum (`AMAX1`, ..., `UMAX`, `UMIN`) | 22 | 0 | 0 | 6 |
| Conversions (`DBLE`, `SNGL`, `FLOATJ`, `DPROD`, `CVT_D_G`, ...) | 16 | 0 | 0 | 6 |
| Bit functions (`IIAND`, ..., `JISHFT`, `JNOT`) | 10 | 0 | 0 | 6 |
| Complex functions (`CABS`, `CEXP`, `CLOG`, `CSIN`, `CCOS`, `CSQRT`; C, CD, CG) | 18 | 0 | 18 | 7 |
| Complex parts (`CMPLX`, `CONJG`, `AIMAG`, `REAL`; F, D, G) | 12 | 0 | 0 | 7 |
| `RANDOM` | 1 | 0 | 0 | 9 |
| Internal reduction (`MTH$$REDUCE_...`) | 0 | 0 | 9 | stays a stub |
| **All `MTH$`** | **234** | **80** | **62** | |
| `OTS$POW`: floating and integer | 17 | 0 | 0 | 8 |
| `OTS$POW`: complex (`CC`, `CJ`, `CDCD_R3`, `CDJ_R3`, `CGCG_R3`, `CGJ_R3`) | 6 | 0 | 0 | 8 |
| `OTS$DIVC`, `DIVCD_R3`, `DIVCG_R3`, `MULCD_R3`, `MULCG_R3` | 5 | 0 | 0 | 8 |

The `OTS$` names are `POWDD`, `POWDJ`, `POWDLU`, `POWDR`, `POWGG`, `POWGJ`,
`POWGLU`, `POWHH_R3`, `POWHJ_R3`, `POWHLU_R3`, `POWII`, `POWJJ`, `POWLULU`,
`POWRD`, `POWRJ`, `POWRLU`, `POWRR`, and the complex and multiply/divide
names above.

The 62 JSB names no manual gives: the 10 second entries; 18 complex
(section 1.5 says complex functions have no JSB entries); 9 hyperbolic; 9
`MTH$$REDUCE_`; 9 two-argument (`ATAN2`, `ATAND2`, `MOD`); 4 sign; and
`SQRT_R2`, `HTAN_R7`, `HTAND_R7`, none of which shares a target with its
documented neighbor.

### Calling conventions

| Kind | Arguments | Result |
|---|---|---|
| `MTH$` CALL, F or integer result | by reference | R0 |
| `MTH$` CALL, D or G result, F complex | by reference | R0:R1 |
| `MTH$` CALL, H, D complex, or G complex result | by reference, after the result | written through the first argument |
| `MTH$` JSB, F | R0 | R0 |
| `MTH$` JSB, D or G | R0:R1 | R0:R1 |
| `MTH$` JSB, H | R0:R3 | R0:R3 |
| `MTH$` JSB, two results (`SINCOS`) | as above | R0:Rm, then R(m+1):R(2m+1): `HSINCOS_R7` ends in R7 |
| `OTS$` CALL | **by value** in the argument list: a D is two longwords, an H or D complex four | R0, R0:R1, or R0:R3 |

Exceptions to "by reference": `MTH$UMAX` and `MTH$UMIN` are by value, and
`MAX`/`MIN` take any number of arguments.

### Conditions

Every value is in `vmsdef`, and all have severity SEVERE. The message texts
are there too, each with one FAO argument, `user PC !XL`.

| Condition | Value | When (the manual) |
|---|---|---|
| `MTH$_INVARGMAT` | `0x16828C` | `ASIN`, `ACOS` with \|x\| > 1; `ATAN2` of (0, 0); `ATANH` with \|x\| >= 1 |
| `MTH$_LOGZERNEG` | `0x16829C` | a logarithm of x <= 0 |
| `MTH$_SQUROONEG` | `0x1682A4` | a square root of x < 0 |
| `MTH$_FLOOVEMAT` | `0x1682C4` | a result too large: `EXP` above 88.029 (F, D), 709.089 (G), 11355.830 (H); `COSH`, `SINH` above 88.722, 709.782, 11356.523; `TAN`, complex forms, conversions |
| `MTH$_FLOUNDMAT` | `0x1682CC` | `EXP` at or below -88.722 (F; H -11356.523) *if the caller has FU set*, else 0.0 quietly; `SIND`-family angles below 180/pi * 2^-m (no such gate stated) |
| `MTH$_UNDEXP` | `0x168294` | an `OTS$` power of 0 to 0 or below, or a negative base to a floating exponent |
| `MTH$_WRONUMARG` | `0x168284` | in neither manual's routine pages |
| `MTH$_SIGLOSMAT` (`SINCOSSIG`) | `0x1682BC` | in neither manual's routine pages |
| `SS$_ROPRAND` | | any floating input with sign 1 and exponent 0 |
| `SS$_FLTOVF`, `SS$_FLTDIV`, `SS$_INTOVF`, `SS$_INTDIV` | | hardware traps Table 1-1's footnotes and the `OTS$` pages name |

The limits follow from the formats, so no table of them is needed. Rounding
reports `ErrOverflow` and `ErrUnderflow`, and the manual's limits become
test cases.

A math routine's condition is signaled through LIB$SIGNAL with the reserved
operand (-0.0: R0 `0x8000`, R1 0) as the mechanism array's R0 and R1. A
handler that returns SS$_CONTINUE returns those registers, corrected if it
changed them, to the routine's caller. A result in memory or in R2:R3 can't
be corrected.

### What the code has, and what it lacks

- **The signal and continue path is enough.** `signal` takes R1 from the CPU
  and R0 from its argument, and `continueCondition` restores both from the
  mechanism array, for CALL and JSB alike. So a routine sets R1 to 0 and calls
  `SignalFailure(argv, 0x8000)`. No `corevms` change is needed.
- **Writing R2 and R3 from a CALL shim works:** `ShimStub`'s mask is 0, so
  `RET` restores nothing. `Environment.CPU()` gives the registers.
- **`JSBRegisters` is R0 to R6, and `HSINCOS_R7` and `HSINCOSD_R7` return in
  R4:R7.** It must grow by one register. Nothing else needs R7 or above: the
  `n` of `_Rn` is what a routine may change, not what it must.
- **The caller's FU bit isn't exported.** `signal` reads the frame's saved
  PSW (the PSL for a JSB); `EXP` needs the same answer before it signals.
- **`librtl`'s `memoryFault` is private.** An argument a routine can't reach
  should signal SS$_ACCVIO here too.
- **`vaxfloat.Value` hides its `big.Float`.** The routines need a way in and
  out. `math/big` has `Sqrt` but no exp, log, or arc tangent, and pi must be
  computed: to about 16,500 bits for H's largest angle, once, on first need.
- **A JSB to today's NOTIMPL stub doesn't reach its message.** `unresolvedStub`
  makes a CALL stub, whose first bytes are a zero mask, and `00` is HALT. This
  comes from reading the stub's bytes, not from a run. The JSB names govax
  lacks need `JSBShimStub` stubs, as `librtl.MissingJSB` makes.
- **Shim codes:** `librtl` is below 1000, its missing routines start at 1000
  and 2000, the traceback handler is 3000, and unresolved stubs start at
  10000. 4000 up is free. (The survey said 3000 up, missing the traceback
  handler; subtask 2's first test run found it.)
- **404 stubs at 12 bytes is about 10 pages of S0.** That moves whatever
  `EnsureShims` places after them, so the oracles must be rerun in subtask 2.

### Measurements

- **Go's `math` isn't the same from host to host.** Two million arguments each
  through 15 functions, hashing the result bits, on arm64 and amd64 (under
  Rosetta): only `Sqrt` agreed. The other 14 differ somewhere. The Go spec lets
  arm64 fuse multiply-add, and some amd64 routines are assembly. A G result's
  last bit would depend on the host, which breaks a repeatable test or
  transcript.
- **`float64` doesn't hold every G value.** G's range starts at 2^-1024, and
  float64's normal range at 2^-1022, so the lowest band is subnormal and loses
  bits.
- **`math/big` is cheap beside emulation.** A sine's Taylor series on this
  machine took 0.8 us at 64 bits (F plus guard bits), 1.3 us at 93 (G), 1.4 us
  at 96 (D), and 2.0 us at 153 (H), against 3 ns for `math.Sin`. Reduction,
  unpacking, and packing are extra. At 10 to 20 million VAX instructions a
  second, 1 us is 10 to 20 instructions.

### Findings for later phases

- **Integer powers:** the `OTS$POWRJ` and `POWDJ` tables give a negative
  exponent as 1.0 over the product. Phase 76's folding
  (`sema/constant.go`'s `power`) takes the product from the reciprocal, which
  can differ in the last bit. Subtask 8 changes the folding to the manual's
  order unless the probe says otherwise.
- **`OTS$POWII` and `POWJJ`** say a negative base is `MTH$_UNDEXP`, and name
  floating traps, which looks copied from a floating page: FORTRAN's `(-2)**3`
  is -8. Until the probe answers, a negative base computes normally, and only
  zero to zero or below signals. Unconfirmed.
- **COPY to the host isn't a byte copy of an image.** Copying MTHRTL.EXE with
  govax's COPY wrote each fixed-length record with a newline, then failed with
  "corrupt record" on the last record, which LINK ends mid-block. `ods2 copy
  /BINARY` copied it whole. This is not fixed here; it needs its own decision
  on what a host copy of a binary file should be.

## Decisions

1. **The set is VMS 7.1's image, not a manual's** (answers "the exact set"):
   every one of the 404 names resolves. The 7.3 manual's additions are the
   vector routines (`MTH$V...`), which are VMTHRTL's and out of scope. The
   manuals are the behavior, and the image is the list.
2. **The `OTS$` routines go in MTHRTL** (answers "which library"): all 28 are
   in its table, and none is in LIBRTL's.
3. **One `math/big` engine for all four formats. Accepted by the author
   (2026-10-30), who chose results that agree across hosts over speed; it
   replaces the plan's Decision 14 on this point** ("F and G are computed in
   float64").
   Every function is written once, taking a precision: the format's fraction
   plus 40 guard bits, rounded once by `vaxfloat`. It gives the same bits on
   every host, covers G's whole range, needs one implementation and one set of
   tests, and is correctly rounded except about once in 2^40, so within the
   plan's one ulp. It costs several hundred times `math.Sin`, or tens of
   VAX instructions a call. Go's `math` is not used for any result.
4. **Exact last bits aren't a goal, and exact values are.** `SIND(180)`,
   `COSD(90)`, `TAND(45)`, `SQRT(4)`, and `EXP(0)` come out exact: degrees are
   reduced exactly, in rational arithmetic, before any conversion to radians.
5. **`MTH$RANDOM` is the manual's generator:** SEED = (69069 * SEED + 1) mod
   2^32, with the high 24 bits as a fraction in [0, 1). How those bits become
   an F value is for the probe.
6. **Entries are a table, as LIBRTL's are:** name, code, and function, with
   offsets from `vmsdef.ImageSymbols` and a stub for each entry. Codes start at
   4000, and each is 4000 plus the entry's offset over 8, computed rather than
   written in the table (subtask 2), so the two names of an entry are one
   routine and a code can't disagree with the vector. A name with no routine gets a missing-routine stub of its kind: JSB if
   it is `MTH$..._Rn`, CALL otherwise, a rule all 404 names satisfy.
7. **Second entries share the first's routine.** The 10 pairs are the same
   code by the vector. Undocumented JSB entries with one argument follow
   section 1.2's convention and are added with their family, if the probe
   confirms it. The others stay stubs until a probe or a program asks.
8. **Hardware conditions are signaled from Go** (`SS$_ROPRAND`, `SS$_INTOVF`,
   ...), with PC the routine's return address. VMS's PC would be inside MTHRTL,
   so a traceback line differs. No program can depend on that address.

## Open questions

- **For the VMS probe** (subtask 10, `testdata/mth77`, run by the author):
  - the signal array's arguments for each `MTH$_` condition, and what user PC
    holds;
  - whether `SIND`'s `FLOUNDMAT` depends on FU;
  - what continuing from `SS$_ROPRAND` returns;
  - `OTS$POWII` and `POWJJ` for negative bases and exponents, and their
    overflow;
  - `MTH$RANDOM`'s values from known seeds;
  - what an H routine's result in memory holds after a handler continues from
    its condition (govax writes the reserved operand, or 0.0 for an underflow,
    before signaling), and R2:R3 after an H JSB entry's (govax clears them);
  - what a handler that continues from `FLOUNDMAT` gets in R0 and R1 (the
    manual says "the result is set to 0.0", which govax follows);
  - `MTH$GEXP`'s underflow limit: the manual says -709.774, where the format's
    limit, 1024 ln 2, is -709.783. govax follows the format, as the other three
    forms' figures do;
  - `AMAX1` with no arguments (`WRONUMARG`?);
  - whether `SIN` of a huge argument signals `SIGLOSMAT`;
  - the undocumented JSB entries' registers, starting with `COSH_R4` and
    `SQRT_R2`;
  - last-bit comparison, reported as a count rather than held as a test.

## Subtasks (revised by the survey)

1. **Survey and capture.** Done.
2. **The package and its registration:** `internal/mthrtl`, the table, stubs of
   both kinds, `Emulated`'s entry, `JSBRegisters` to R7, the helpers (signal
   with the reserved operand, caller's FU, argument access, H and R0:R3
   results), `vaxfloat`'s way in and out; the oracles rerun. Done.
3. **The engine, square root, exponentials, logarithms** in all four formats,
   JSB entries with them. Done.
4. **Trigonometric and inverse functions**, radians and degrees, exact
   reduction.
5. **Hyperbolic functions and ATAN2.**
6. **Table 1-1's routines:** absolute value, positive difference, sign, floor,
   truncation, nearest integer, remainder, maximum and minimum, conversions,
   bit functions. (New: the plan's subtask 5 was too large.)
7. **Complex functions and parts.**
8. **OTS$ powers, multiplication, and division**, and Phase 76's folding made
   to agree (`sema/constant.go`'s `power`, `complexArith`).
9. **MTH$RANDOM and the remaining conditions.**
10. **Tests from MACRO** programs, and the VMS probe.
11. **Close-out:** CLAUDE.md, MANUAL.md's RTL section, HELP.

## Progress log

- **2026-10-29:** Planned with Phase 74. The author directed that the
  MTH$ functions be general RTL support, usable outside FORTRAN, and
  approved capturing VMS 7.1's MTHRTL vector.
- **2026-10-30:** Subtask 1. Read the 5.0 MTH$ and OTS$ manuals and the 7.3
  MTH$ manual; merged VMS 7.1's MTHRTL.EXE into `vmsdef`
  (`gen -image`, 404 symbols, `ImageSources` now names it); read each
  transfer vector entry's first bytes for its mechanism. LINK now resolves
  MTH$ and OTS$ names from the table with no MTHRTL.EXE present, and a call
  signals NOTIMPL (`TestMTHRTLLinks`, in both paging modes); `TestMTHRTLVector`
  holds the table's shape. The author accepted the survey's Decision 3 the same
  day: one `math/big` engine for all four formats, in place of the plan's
  `float64` for F and G.
- **2026-10-30:** Subtask 2. `internal/mthrtl` is the tables (empty so far),
  the 402 stubs (`Links`, in `machine.ShimTable` after the C library's), and
  `Register`, which gives each entry a NOTIMPL routine by its name's kind
  until a table has it: a call now names the routine (`routine MTH$SIN is not
  implemented`, no longer `at MTHRTL+^X...`), and a JSB entry reaches its
  message, where it used to run the unresolved stub's mask as a HALT. An
  entry's two names share a stub, which goes by the name that sorts first
  (`MTH$IIFIX`, `MTH$JIFIX`), the rule `machine`'s `sharedName` already
  follows. The helpers are `float.go`'s (`loadArg`, `storeResult`,
  `returnFloat`, `loadRegisters`, `storeRegisters`) and `condition.go`'s
  (`signalCondition`, whose array is the condition, 1, and the user PC, and
  `fail`, for SS$_ROPRAND and SS$_ACCVIO), over `corevms`'s new `CallerPC` and
  `CallerFU` (`callerState`, now `signal`'s too) and `librtl`'s exported
  `MemoryFault`, `Writing`, `LoadFloat`, `StoreFloat`, and `NotImplementedJSB`.
  `JSBRegisters` is R0 to R7. `vaxfloat` gains `FromBig` and `Value.Big`.
  `TestHelpersFromAProgram` (`run_test.go`, an external test package, since
  the console imports this one) runs a MACRO program against routines built on
  the helpers: conditions continued from a CALL and a JSB entry, a reserved
  operand, an argument it can't read, results in R0:R1, R0:R3, R0:R7, and
  through the first argument, the caller's FU both ways.
  `TestUnhandledCondition` is the message and the image's end. The 10 pages of
  stubs moved nothing an oracle looks at: every oracle passes unchanged, and
  the only test that changed for the move is `shim_test.go`'s, whose bound
  assumed the table fit VMINIT's page. The console, debugger, and machine
  packages skip five tests, none about this: `TestLink68Oracle`'s `files.lst`
  and `myshr.olb` comparisons in each loader, and `TestMacro_realStarlet`.
- **2026-10-30:** Subtask 3. `engine.go` is the engine: each function takes a
  precision and works above it, and a routine asks for its format's precision
  plus 40 bits and rounds once (`rounded`). `exponential` is 2^k times a Taylor
  series after 8 halvings, `logarithm` is n ln 2 plus the atanh series, which
  stays accurate however close x is to 1, `logBase2` makes a power of 2 exact
  by keeping n apart, and ln 2 and ln 10 are `constant`s made once from atanh
  series (a `constant` recomputes only for more bits, so pi can follow it to
  H's 16,500 bits in subtask 4). Square root is `vaxfloat.Sqrt`. `function.go`
  makes a routine from a function and a format: `callEntry` (R0 or R0:R1, or
  through the first argument for H) and `jsbEntry`, with `evaluate`, which gives
  0.0 quietly for an underflow unless the caller set FU, and `signalFailure`,
  which turns the function's error into the condition. The tables hold 20 CALL
  and 18 JSB entries, the second entries `DEXP_R7` and `GEXP_R7` among them
  (Decision 7). `MTH$SQRT_R2` stays a NOTIMPL stub until the probe gives its
  registers.
  `engine_test.go` checks each function against a reference by other means at
  512 bits (a Taylor series with no ln 2 in it, Halley's method on it for the
  logarithm, and exact squares for the root), in every format, over the whole
  range, near 1, and past the limits; a run with the guard and scratch bits set
  to 0 fails 713 of its checks, so the comparisons can tell. It also checks
  published digits, exact values, the manual's limits, and the domains.
  `TestEntriesFromAProgram` runs every entry from a MACRO program it generates:
  answers, each condition continued from a handler, FU clear and set, a reserved
  operand. `TestRoutineUnhandled` is the message and the image's end. A call
  costs 1.5 to 2.2 us for an exponential, 2 to 3.9 us for a natural logarithm,
  2.5 to 5.2 us for a common logarithm, and under 0.3 us for a square root,
  F to H, so tens of VAX instructions' time, as the survey estimated. The
  probe's questions gained three: H results after a condition, FLOUNDMAT's
  continued result, and G's underflow limit, where the manual's -709.774 isn't
  the format's -709.783.
