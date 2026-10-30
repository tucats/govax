# Phase 77 — MTHRTL: VMS's mathematics library, emulated

**Status:** in progress (subtask 1, the survey and capture, 2026-10-30,
its Decision 3 accepted by the author the same day; subtasks 2, the package
and its registration, 3, the engine, square root, exponentials, and
logarithms, 4, the trigonometric functions and their inverses, 5, the
hyperbolic functions and ATAN2, 6, Table 1-1's routines, 7, the complex
functions and parts, 8, the OTS$ powers, multiplication, and division, and 9,
MTH$RANDOM and the conditions' audit, the same day).
Needs Phase 74 (the plan)
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

Exceptions to "by reference": none. (The survey said `MTH$UMAX` and `MTH$UMIN`
are by value; their pages in the 7.3 manual say by reference, and subtask 6
follows them.) `MAX`/`MIN`, `UMAX`, and `UMIN` take any number of arguments.

### Conditions

Every value is in `vmsdef`, and all have severity SEVERE. The message texts
are there too, each with one FAO argument, `user PC !XL`.

| Condition | Value | When (the manual) |
|---|---|---|
| `MTH$_INVARGMAT` | `0x16828C` | `ASIN`, `ACOS` with \|x\| > 1; `ATAN2` of (0, 0); `ATANH` with \|x\| >= 1; `CDLOG`, `CGLOG` of (0, 0) |
| `MTH$_LOGZERNEG` | `0x16829C` | a logarithm of x <= 0 |
| `MTH$_SQUROONEG` | `0x1682A4` | a square root of x < 0 |
| `MTH$_FLOOVEMAT` | `0x1682C4` | a result too large: `EXP` above 88.029 (F, D), 709.089 (G), 11355.830 (H); `COSH`, `SINH` above 88.722, 709.782, 11356.523; `TAN`, complex forms, conversions |
| `MTH$_FLOUNDMAT` | `0x1682CC` | `EXP` at or below -88.722 (F; H -11356.523) *if the caller has FU set*, else 0.0 quietly; `SIND`-family angles below 180/pi * 2^-m (the routine's conditions state no gate, but `SINCOSD`'s and `TAND`'s descriptions do: "and underflow signaling is enabled") |
| `MTH$_UNDEXP` | `0x168294` | an `OTS$` power of 0 to 0 or below, or a negative base to a floating exponent |
| `MTH$_WRONUMARG` | `0x168284` | in neither manual's routine pages |
| `MTH$_SIGLOSMAT` (`SINCOSSIG`) | `0x1682BC` | in neither manual's routine pages |
| `SS$_ROPRAND` | | any floating input with sign 1 and exponent 0 |
| `SS$_FLTOVF_F`, `SS$_FLTDIV_F`, `SS$_FLTUND_F`, `SS$_INTOVF`, `SS$_INTDIV` | | hardware exceptions Table 1-1's footnotes and the `OTS$` pages name; floating ones in the fault forms govax's CPU takes (the survey had the trap names) |

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
   With `vax.runtime.random` true (the author's request, 2026-10-30; off by
   default), each new seed is instead 32 bits from the host's `crypto/rand`: see
   subtask 9's log.
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
  - `MTH$RANDOM`'s values from known seeds, how its bits become an F value, and
    whether it leaves R1 alone;
  - what an H routine's result in memory holds after a handler continues from
    its condition (govax writes the reserved operand, or 0.0 for an underflow,
    before signaling), and R2:R3 after an H JSB entry's (govax clears them);
  - what a handler that continues from `FLOUNDMAT` gets in R0 and R1 (the
    manual says "the result is set to 0.0", which govax follows);
  - `MTH$GEXP`'s underflow limit: the manual says -709.774, where the format's
    limit, 1024 ln 2, is -709.783. govax follows the format, as the other three
    forms' figures do;
  - `AMAX1` with no arguments (`WRONUMARG`?);
  - whether `SIN` of a huge argument signals `SIGLOSMAT` (govax computes the
    true value, whatever the size);
  - what a handler that continues from `SINCOSD`'s `FLOUNDMAT` finds in the two
    results (govax has 0.0 and the good cosine, in memory or registers, an F JSB
    entry's cosine going through the mechanism array's R1);
  - what `TAND` gives where the cosine is exactly 0 beyond the manual's "signal
    overflow": R0 and R1 after continuing (govax's reserved operand), and whether
    `TAND(90)` is exact on VMS at all, or a large finite value, as `TAN` is;
  - what `MTH$TAN`'s overflow is for: no argument in any format reaches it in
    govax;
  - the undocumented JSB entries' registers, starting with `COSH_R4` and
    `SQRT_R2`, and how the two-argument ones (`ATAN2_R4`, `DATAN2_R7`, ...)
    take their arguments;
  - `ATAN2` of a sine too small beside its cosine for the quotient to fit the
    format: govax returns 0.0 quietly, whatever FU, because the routine's
    conditions list no `FLOUNDMAT` and, unlike `SIND`'s and `TAND`'s, its
    description doesn't mention underflow;
  - `ATAN2` of 0 and a negative cosine: govax gives pi, the manual's
    `pi * sign(Y)` with 0 counted as positive;
  - Table 1-1's conditions: which floating exceptions these routines really
    signal, under which names (`SS$_FLTOVF_F` or a trap's, `MTH$_FLOOVEMAT`), and
    what a continuing handler finds, since VMS's would resume in a faulting
    instruction;
  - `MTH$AMOD`'s kind: whether a quotient beyond the format overflows (govax says
    yes, from the footnote), what a quotient too small does with FU set (govax
    goes on, the remainder being the dividend), what a zero divisor signals (no
    footnote; govax signals `SS$_FLTDIV_F`), and how exact a remainder is when the
    quotient has more bits than the format;
  - `MTH$SNGLG`'s and `MTH$AMOD`'s underflow: which condition, and whether it
    follows the caller's FU, as govax has it, since a CALL entry clears FU for its
    own instructions (the 5.0 manual also gives `MTH$DIM` underflow, and 7.3 not);
  - `MTH$ANINT` just below 0.5: govax's 0 is the true nearest, where the FORTRAN
    manual's `INT(a + .5)` in F_floating would give 1;
  - `MTH$JMOD(-2**31, -1)` and `MTH$IMOD(-32768, -1)`: govax's 0, from the footnote's
    silence, though VMS's mask sets IV; and R0 after continuing from a zero divisor
    (govax's dividend);
  - a word result's high word in R0 (govax's is the sign);
  - `MTH$AMAX1`'s kind with no arguments: govax signals `MTH$_WRONUMARG`;
  - `MTH$UMAX` by reference, and `MTH$SGN` of a G or H value;
  - `MTH$CVT_DA_GA`'s kind: R0, and an element out of range (govax stores what a
    single conversion leaves, goes on, and signals once at the end, the handlers
    running only after the Go routine ends);
  - the undocumented JSB entries of this family: `MTH$AMOD_R5`, `DMOD_R9`,
    `GMOD_R9`, `SIGN_R1`, `DSIGN_R3`, `GSIGN_R3`, `SGN_R1`;
  - `MTH$CEXP` of a large negative r: the page's overflow says "the absolute value
    of r", where govax takes only a large positive r, and gives a part too small for
    the format as 0.0;
  - `MTH$CSIN`'s and `MTH$CCOS`' overflow: whether it's at the page's 88.029 (EXP's
    limit, which govax follows, whatever r) or at `COSH`'s 88.722, and if r can save
    an i past it;
  - `MTH$CLOG` of (0, 0), whose F page has no condition (govax signals the D and G
    pages' `MTH$_INVARGMAT`), and of a number whose modulus overflows, which only
    the D and G pages give (`SS$_FLTOVF_F`, there "signaled from `MTH$CxABS`", whose
    own page says `MTH$_FLOOVEMAT`);
  - what arguments give `MTH$CSQRT`'s `SS$_FLTOVF_F`, which no result needs: govax
    takes it at the page's `ABS(r) + CABS(r, i)`, so (1.0E38, 0.0) signals;
  - what a handler that continues from a complex routine's condition finds: an F
    result's R1 (govax's 0), and a D or G result's two parts in memory (govax's
    reserved operands);
  - whether a complex routine's part too small for the format follows FU (govax's
    0.0 doesn't, no page listing `MTH$_FLOUNDMAT`);
  - whether `MTH$REAL` and `MTH$AIMAG` fault on a reserved operand in the part they
    don't return (govax doesn't read it);
  - an integer power's rounding and order: whether VMS's `OTS$POWRJ` rounds at each
    product, as its page says, and takes 1.0 over the product, so `2.0 ** (-127)` in F
    overflows though its value fits, and `3.0 ** (-5)` has govax's last bit;
  - which overflow each `OTS$` power signals where its page lists both `SS$_FLTOVF` and
    `MTH$_FLOOVEMAT` (govax's is `MTH$_FLOOVEMAT` for real bases, and for a complex base
    `POWCC`'s and `SS$_FLTOVF_F` for `POWCJ`, by their lists), and whether a product of
    logarithm and exponent too large for the format is one of them;
  - `OTS$POWRJ(0.5, -100000)`, whose products reach 0.0 with FU clear: govax signals
    `SS$_FLTDIV_F`, which only the complex page lists; and with FU set `MTH$_FLOUNDMAT`;
  - what a handler that continues finds in R0 to R3: govax has the reserved operand in R0
    and R1 to R3 clear, for an H or D or G complex, and an integer routine's R0 after
    `MTH$_UNDEXP` is that `^X8000` too;
  - `OTS$POWCC`'s result where a part should be exactly 0, `(-1, 0) ** (2, 0)`: govax's is
    a tiny number, far below the other part's last bit;
  - `OTS$POWCC`'s overflow and small parts, which govax takes from `MTH$CEXP`'s;
  - `OTS$POWCJ` with a negative exponent, whose row says only "the product";
  - `OTS$MULCx` and `DIVCx`: how each part rounds (govax's is exact, then rounded once),
    and whether a part too small follows FU (govax's is 0.0, whatever FU);
  - `OTS$POWII`'s and `POWJJ`'s negative bases and exponents (the survey's finding):
    `(-2) ** 3`, `2 ** (-1)`, `(-1) ** (-3)`, and overflow, `SS$_INTOVF` with the low-order
    bits in R0;
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
   reduction. Done.
5. **Hyperbolic functions and ATAN2.** Done.
6. **Table 1-1's routines:** absolute value, positive difference, sign, floor,
   truncation, nearest integer, remainder, maximum and minimum, conversions,
   bit functions. (New: the plan's subtask 5 was too large.) Done.
7. **Complex functions and parts.** Done.
8. **OTS$ powers, multiplication, and division**, and Phase 76's folding made
   to agree (`sema/constant.go`'s `power`, `complexArith`). Done.
9. **MTH$RANDOM and the remaining conditions.** Done.
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
- **2026-10-30:** Subtask 4. `enginetrig.go` is the engine's part. `pi` is a
  `constant` from Machin's formula in fixed point, which is much cheaper at
  thousands of bits than a `big.Float` series; asked for more bits than it has,
  a constant now at least doubles them. `reduceRadians` works at the bits of the
  angle's integer part plus `reductionSlack`, looks at how far the difference
  cancelled, and tries again with twice the slack if it went further (the
  nearest double to a multiple of pi/2 cancels about 61 bits, and each format's
  own nearest value to pi/2 all of its own). `reduceDegrees` is exact, in
  integers (Decision 4), so a multiple of 90 reduces to exactly 0 however large.
  `sinCosSeries` makes both functions in one loop. `arcTan` inverts above 1 and
  halves with the half-angle formula, down below 1/8, then sums `atanSeries`,
  which with `atanhSeries` is now one `oddPowerSeries`. `arcSin` and `arcCos` are
  the manual's forms through `complementRoot`, which is `(1 - x) * (1 + x)` so
  nothing cancels near 1. `trig.go` makes the functions from these, radians and
  degrees alike. `pairFunction`, `callPairEntry`, and `jsbPairEntry` are the
  two-result routines, `SINCOS` and `SINCOSD`, with their results in R0:R7 for H.
  The tables gain 56 CALL and 64 JSB entries, the second entries `TAN_R5`,
  `TAND_R5`, `ACOS_R5`, `ASIN_R5`, `DACOS_R9`, `DASIN_R9`, `GACOS_R9`, and
  `GASIN_R9` among them (Decision 7). `MTH$HTAN_R7` and `MTH$HTAND_R7`, which no
  manual gives and whose targets differ from their neighbors', stay stubs, as
  `MTH$SQRT_R2` does; `TestMTHRTLLinks` moved to `MTH$SINH` and `MTH$SQRT_R2`, now
  that `MTH$SIN` is written.
  `TAND` at an odd multiple of 90 is `FLOOVEMAT`; `TAN` has no such argument, so
  its overflow, which the manual lists, is never reached (the probe's question).
  `SIND`, `TAND`, and `SINCOSD`'s underflow follows FU as `EXP`'s does; the 7.3
  manual's descriptions say so, though its condition tables don't.
  `enginetrig_test.go` checks each function against a reference by other means at
  1024 bits (pi from the Gauss-Legendre iteration, checked against published
  digits; an angle reduced by 2 pi with no quadrants, or in rationals for
  degrees; the Taylor series directly; `tan y = x` solved by iteration), in every
  format, over the whole range, at the tiny end, near multiples of pi/2 and 45
  degrees, and near the arcs' ends. It also checks published digits, the largest
  angle in each format, the published hardest double (`6381956970095103 * 2^797`,
  whose cosine is `-4.68716592425462761112E-19`), exact values, the manual's
  limits, and the domains. Breaking the code showed the checks can tell: guard and
  scratch bits at 0 fail 8,161, no retry in the reduction 246, no bits for the
  angle's size 146, `1 - x * x` 14, pi 40 working bits short 3, a wrong sign in the
  quadrants 2,364. Taking out the degree reduction's rounding to nearest fails
  none, rightly, since that only makes the series a little longer.
  `TestEntriesFromAProgram` runs every new single-result entry from its generated
  MACRO program (answers, each condition continued, FU clear and set, a reserved
  operand), and `TestPairsFromAProgram` the pairs, CALL and JSB. A call costs 2.3
  to 4.1 us for a sine or cosine, 1.3 to 2.6 us in degrees at an angle needing no
  reduction, 3.3 to 6.4 us for an arc tangent, and 3.5 to 7.8 us for an arc sine
  or cosine, F to H: still tens of VAX instructions' time. Pi to the 16,700 bits
  H's largest angles need takes 5.2 ms, once.
- **2026-10-30:** Subtask 5. `enginehyper.go` is the engine's part. `sinhCosh`
  uses the exponentials for |x| of at least 1 and below that the Taylor series,
  where a difference of exponentials would cancel; the series is
  `factorialSeries`, the loop `sinCosSeries` already had, now with an
  `alternating` flag, as `oddPowerSeries` has. `arcTanh` is `atanhSeries` below 1/8
  and the manual's logarithm above it. `arcTan2` (`enginetrig.go`) is the manual's
  three cases, with 0 counted as positive. `hyperbolic.go` makes the routines:
  to `sineFunction`, `cosineFunction`, and `tangentFunction` a hyperbolic pair is
  one more pair, so `angleFunction` became `sinCosFunction`, and `MTH$TANH` is
  `tangentFunction(sinhCosh)`. `function.go`'s `binaryFunction` and
  `callBinaryEntry` are the two-argument routines; `callEntry` and they now share
  `callWithArguments`, in place of `throughFirstArgument`, and `evaluate` became
  `gateUnderflow`, on a result already computed. The tables gain 24 CALL entries.
  The 15 JSB entries of these families, which no manual gives, stay stubs
  (Decision 7), and `TestMTHRTLLinks` moved from `MTH$SINH` to `MTH$RANDOM`.
  The manual's piecewise limits aren't written down, since rounding reaches
  them: `a(z)`'s "returns X" or "returns 1", `b(z)`'s single exponential, `h`'s
  sign, `ATAN2`'s `Y/X > 2^(f+1)`. `SINH` and `COSH` overflow past the manual's
  88.722, 709.782, and 11356.523, and `TANH` is 1 at each format's `h`, both held
  by tests. The manual's `g` is not: `TANH` at `G`'s `2^-26` and `H`'s `2^-56`, both
  powers of 2, rounds to the value just below the `X` the manual returns, so govax
  is the more accurate there. The H page's
  `zLOG((X + 1)/(X - 1))/2` for `ATANH` is a misprint; the F page's
  `(1 + X)/(1 - X)` is right, and `(X + 1)/(X - 1)` is negative inside the domain.
  `enginehyper_test.go` checks each function against a reference by other means at
  512 or 1024 bits (the definitions on `referenceExp`, `tanh` as
  `(e^2x - 1)/(e^2x + 1)`, `atanh` by `referenceLog` where the code uses its series,
  and the first terms of the power series for a tiny argument), in every format,
  across the range and out of it, either side of the points where the code changes
  method, and at each end. It also checks published digits, symmetry, exact values
  (`ATAND2` of the axes and diagonals), the limits, the domains, and the
  extremes, where a quotient is beyond any format. `TestEntriesFromAProgram` runs
  every new entry from its MACRO program, its generator now taking a second
  argument. Breaking the code showed the checks can tell: guard and scratch bits
  at 0 fail 11,234, sinh by exponentials everywhere 549, an alternating series
  1,326, atanh by logarithm everywhere 370, `1 - x` at 64 bits 94, no pi for a
  negative cosine 894, pi's sign ignoring the sine's 554, a quotient at 64 bits 181,
  `ATAN2`'s underflow reported 53, its arguments swapped 40, and H's arguments off
  by one 179; `ATANH`'s domain admitting 1 panics. A call costs 1.7 to 2.9 us for a
  hyperbolic sine or cosine by exponentials, 2.0 to 3.7 us by the series, 2.1 to
  3.3 us for a tangent, 2.6 to 4.8 us for `ATANH`, and 2.7 to 5.2 us for `ATAN2`,
  F to H: still tens of VAX instructions' time.
- **2026-10-30:** Subtask 6. `floating.go` is the routines on floating values:
  absolute value, positive difference, transfer of sign, `wholeNumber` (`floorOf`,
  `truncated`, `nearest`, all exact), `remainder`, the conversions, `productEntry`,
  and `convertArrayEntry`. `integer.go` is the word and longword ones: `width`
  (`load`, `lowBits`, `holds`), `returnInteger` (R0 the low-order bits, and
  `SS$_INTOVF` with them in R0 if it doesn't fit), `integerEntry`, `wholeEntry`
  (floating argument, integer result, `MTH$SGN` among them), `integerRemainderEntry`,
  and the bit functions. `extreme.go` is the maximum and minimum, any number of
  arguments through a `loader`, which `callWithArguments` now takes too, so a
  routine's argument type and result format can differ (`conversionEntry`).
  `condition.go`'s `trapError` is a hardware exception as a function's error, with
  its vaxfloat cause under it so the FU gate still sees an underflow; `trapping` and
  `noUnderflow` say what a routine's footnotes give it. The tables gain 101 CALL
  lines (103 names, `MTH$IIFIX` and `MTH$JIFIX` standing for their second names)
  and the 8 JSB entries the manual gives. The 7 undocumented JSB entries of these
  families stay stubs (Decision 7), and `TestMTHRTLLinks` no longer calls the
  shared `MTH$IIFIX` entry, now written; `TestVector` holds its stub's name.

  Table 1-1 gives no algorithms and only footnotes for conditions, and the 5.0 and
  7.3 manuals' footnotes differ, so these follow 7.3's, the nearer to VMS 7.1, with
  what the routines compute from their names and the FORTRAN manual's Table D-3. A
  second reading of VMS's vector showed each entry's mask: IV is set on every
  routine with the integer overflow footnote (and on `MTH$SGN`, `IMOD`, and `JMOD`,
  without it), clear on `IISIGN` and `JISIGN`, so those two wrap quietly and the rest
  trap whatever the caller's IV. Three choices the manuals don't settle, each a
  question for the probe: a remainder is exact however large the quotient, a quotient
  beyond the format being `SS$_FLTOVF_F`; a zero floating divisor is `SS$_FLTDIV_F`,
  in no footnote; and no arguments to `MTH$AMAX1`'s kind is `MTH$_WRONUMARG`. The
  survey's "`UMAX` and `UMIN` are by value" was wrong: their pages say by reference.

  `floating_test.go` holds the whole numbers and the remainder to a reference in
  rationals, in every format, and the rest at their limits. `TestTable1FromAProgram`
  runs every entry from a generated MACRO program (328 calls: answers, each condition
  continued, the signal array's size, FU clear and set, reserved operands, arguments
  it can't read, H's result through its first argument), `TestTable1Covers` holds it
  to the list of names, `TestConvertArray` runs the array routines, and
  `TestHardwareConditionsUnhandled` is the messages and the image's end. Breaking the
  code showed the checks can tell, from 1 to 65 failures for each of 31 changes:
  nearest truncating 63, a floor the wrong way 49, a floored quotient in the remainder
  21, no quotient overflow 16, no integer overflow 50, the range one bit wide 52, word
  arguments not sign-extended 24, word results 37, maximum and minimum swapped 34, H's
  arguments off by one 65, an array stopping at its first failure 2, and a wrong line
  in the table (`IIFIX`, `IIGNNT`, `AJMIN0`, `HFLOOR_R7`) 1 to 3. Three changes first
  got through: `DPROD`'s underflow and `AJMIN0`'s width were gaps, now cases; an `ISHFT`
  case for counts past the width was redundant, Go's shifts being defined there, and
  it's gone. Only `DIM` at equal values passes, rightly, the result being 0 either way.
- **2026-10-30:** Subtask 7. `complex.go` is the 30 CALL entries: `MTH$CABS`, `CEXP`,
  `CLOG`, `CSIN`, `CCOS`, `CSQRT` and their D and G forms, and `CMPLX`, `CONJG`, `REAL`,
  and `AIMAG` and theirs. `complexEntry` makes a routine with a complex result, in R0
  and R1 for F and through the first argument for D and G, from a `complexFunction` and
  a `complexLoader` (`complexAt`, or `complexFromParts` for `CMPLX`'s two addresses);
  `modulusEntry` is `CABS`; `REAL` is `callEntry` with `convert`, and `AIMAG`
  `conversionEntry` with the `imaginaryArgs` loader. The pages' formulas are each
  part's exact value, rounded once. `sumOfSquares` is exact (`vaxfloat.MulExact`,
  `AddExact`), however far apart the parts are, so the logarithm's real part is right
  when the modulus is within far less than a last bit of 1: `CGLOG(1, 2^-500)` is
  `2^-1001`. The 18 JSB entries of these families, of which section 1.5 says there are
  none, stay stubs (Decision 7).

  The pages settle when a routine gives up, and not all of it. `CEXP`'s overflow
  is the factor `EXP(r)`'s, the page's 88.029 and 709.089, whatever the sine and cosine;
  a large negative r is 0.0, though the page says "the absolute value of r". `CSIN`'s and
  `CCOS`' is at the same figures on |i|, which are EXP's and not `COSH`'s 88.722, so
  `MTH$COSH(88.5)` has an answer and `MTH$CCOS(1, 88.5)` overflows. `CDLOG` and `CGLOG`
  signal `SS$_FLTOVF_F` when the modulus overflows, as their page says, and `CLOG`, whose
  page doesn't, has its answer. `CSQRT`'s `SS$_FLTOVF_F` has to be an intermediate's,
  no root being that large, so it's the sum in the page's `ROOT`, which overflows at
  (1.0E38, 0.0). Zero's root is 0, where the page's Q has no value. No page lists
  `MTH$_FLOUNDMAT`, so a part too small is 0.0, whatever FU, as for `ATAN2`. A D or G
  result that fails holds two reserved operands; an F result is R0 reserved and R1
  clear, as section 1.4 has it, the registers corrected "as double-precision" for
  every routine. `REAL` and `AIMAG` read only their part. Each is a new question for
  the probe.

  `complex_test.go` holds each function to a reference by other means at 512 bits (the
  manual's `MAX * SQRT((MIN/MAX)^2 + 1)` for the modulus; `ln(1 + d)`'s series on a
  difference from 1 in rationals near the unit circle; the root as two square roots,
  long enough for their cancelling; the existing exponential, trigonometric, hyperbolic,
  and `ATAN2` references), in all three formats: parts of a size, far apart, or zero,
  moderate and tiny. It also checks exact values, the page's printed examples, the near
  circle, every limit either side, and parts too small. `TestComplexFromAProgram` runs
  every entry from a generated MACRO program (answers, each condition continued, the
  signal array's size, FU, reserved operands in each part, results over arguments,
  arguments and results it can't reach); `TestManualsComplexExample` is section 1.7's
  `COMPLEX_EX1` and `EX2`, whose R0 `0210404E` and R1 `63383FED` match; `TestComplexCovers`
  holds it to the list of names; `TestComplexUnhandled` is the messages and the image's
  end. `table1Program`'s frame is now `showProgram`, shared.

  Breaking the code showed the checks can tell: 41 changes, 39 of them failing from 2
  to 7,739 checks: a modulus rounded before the logarithm 59, `COSH`'s limit 88, a sum of squares
  at 128 bits 43, no factor precision 245, a `CSQRT` overflow at the modulus 18, or under
  `FLOOVEMAT`'s name 19, an underflow reported 67 and 364, the failed result's second part
  36, no store before signaling 74, a wrong line in the table 2 to 37. The other two panic and hang, rightly: `CSQRT` and `CLOG`
  with no zero case, the logarithm of 0 having no end. Two first got through. `DCMPLX` reading one complex number passed because the
  test's parts were side by side, so now a word is between them; and `DREAL` as `GREAL`,
  since each passes the bits on, so now a part of `^X8070`, reserved in D and a value in G,
  tells them apart. A call costs 0.4 us for a modulus, 1.1 us for a square root, 4.4 to
  5.3 us for an exponential, 5.8 to 7.7 us for a logarithm, and 5.9 to 7.3 us for a sine
  or cosine, F to G: still tens of VAX instructions' time.
- **2026-10-30:** Subtask 8. `ots.go` is all 28 `OTS$` entries, by value in the argument
  list, their results in R0 up. `power` is the floating exponents' table and
  `exp(y * ln x)`, the exact value rounded once; `powerEntry` takes the base's, the
  exponent's, and the result's formats, for `POWRD` and `POWDR`. `integerPower` is the
  integer exponents', `integerPowerEntry` taking a `width` for signed and unsigned.
  `complexPower` is the page's `exp(y * log(x))`, on `exponentialParts`, which is
  `MTH$CEXP`'s work taken out of `complexExponential`; `complexIntegerPower`,
  `complexQuotient`, and `complexProduct` are the rest, and `integerPowerOfEntry` is
  `POWII`, `POWJJ`, and `POWLULU`.

  The agreement with FORTRAN's folding is by sharing the code: `vaxfloat/power.go` has
  `PowerBySquaring`, generic in the kind of number, and `ComplexMul` and `ComplexDiv`,
  moved from `sema/constant.go`, whose `power` and `complexArith` now call them. As the
  survey said it would, the folding took the manual's order in place of its reciprocal
  first: the product of the base's squares, each rounded, then 1.0 over it. So
  `PARAMETER (X = 2.0 ** (-127))` is now `REALRANGE`, the product being too large though
  its reciprocal fits, and `0.5 ** (-100000)` is `DIVZERO`, the product having reached
  0.0, and `3.0 ** (-5)` changes in its last bit. The integer powers round at each
  product because their tables spell a product out and their pages say "the same
  precision as the base"; a floating exponent follows Decision 3 instead, the same page's
  words notwithstanding. An exact tie can round either way there, as `4097.0 ** 2.0`
  is exactly halfway in F, which stays within the plan's one ulp and isn't handled.

  Where a page lists both overflows, it's `MTH$_FLOOVEMAT`, the one `POWDD` and
  `POWxLU`, of the same algorithms, list alone. `POWCJ` and `DIVC` have the hardware's,
  `MULC` `MTH$_FLOOVEMAT`, and `POWCC`'s base of (0, 0) is `MTH$_INVARGMAT` even for a
  positive exponent, as its page says. The survey's reading of `POWII` and `POWJJ` stands:
  a negative base computes, and overflow is `SS$_INTOVF` with the low-order bits in R0.
  `POWLULU` is only low-order bits. An H or D or G complex result that fails has R2 and R3
  clear. The new questions are in the list above. `POWCxJ`'s page lists `SS$_FLTDIV`, which
  is what 1.0 over a product of (0.0, 0.0) takes; `POWRJ`'s doesn't, and govax signals it
  there too, the hardware's answer to the same division.

  `ots_test.go` holds `power` to `referenceExp(y * referenceLog(x))` across each format's
  range and past it, near 1, and with tiny exponents, and to an exact product for whole
  exponents; the integer powers to their products spelled out by hand; `complexPower` to
  a reference with its angle from `referenceAtan`, including an angle of many turns; and
  products and quotients to exact rationals. It also checks the pages' printed examples
  (`POWRD`, `POWGG`, `POWHH_R3`, `POWCC`, whose `POWCGCG_R3` twin misprints its imaginary
  part), and every table. `TestOTSFromAProgram` runs every entry from a generated MACRO
  program (answers from mpmath, each condition continued, the signal array's size, FU,
  reserved operands in each place, R2 and R3 left alone by a short result, and the high
  word of a word argument); `TestOTSCovers` holds it to all 28 names; `TestOTSUnhandled`
  is the messages and the image's end. `vaxfloat/power_test.go` counts `PowerBySquaring`'s
  steps and tells the orders apart; `TestParameterValues` has the folded constants that the
  program test has at run time, the two sharing the code but each checked.

  Breaking the code showed the checks can tell: of 42 changes, 38 failed from 1 to 1,744
  checks, among them the reciprocal first 50, a wrong sign in a product 805 or in a quotient
  915, R2 and R3 left set 66, a complex power's overflow not a trap 9, a table line naming
  the wrong format or width 4 to 15, the exponent's bits left out of `complexPower` 3, and
  the product at only the format's bits 1,744, or 20 bits short of the working precision 1.
  Two make the run time out, which the tests' limit catches: a negative base reaching the
  logarithm, and `integerPowerBits` with no shortcut for a large exponent, whose exact value
  has 3.4 billion bits. Two pass, rightly. `POWDR` reading its F exponent as a D is the same
  value, an F in the list reading as that D with nothing after it. And the product at the
  working precision, without its extra bits, still has guard bits enough for these seeded
  runs; the extra bits are margin no run shows.

  A function's cost, from the benchmarks, leaving out the stub: 0.2 to 0.3 us for an
  integer power, 4 to 6.9 us for a floating exponent, and 1.2 to 1.9 us for a complex
  quotient, F to H, so tens of VAX instructions' time as before. A complex power is the
  first that isn't: 12 to 15 us, 120 to 300 instructions' time at 10 to 20 million a second.
- **2026-10-30:** Subtask 9. `random.go` is `MTH$RANDOM`, the manual's generator: the seed
  read, `(69069 * SEED + 1)` modulo 2^32 written back, and the new seed's high 24 bits over
  2^24 in R0 (Decision 5). F_floating's 24 bits hold each exactly, so none rounds up to 1.0,
  and the library keeps no state: two seeds are two sequences. A seed it can't read or write
  is `SS$_ACCVIO`, the seed having modify access. The conversion is the one thing the manual
  leaves open ("converted"); the probe has it.

  At the author's request `vax.runtime.random` (`randomSetting`, read at each call; off when
  unset) makes the generator unpredictable: the new seed is 32 bits from `crypto/rand`
  (`hostSeed`), and the old seed plays no part. The seed is still read and rewritten and the
  result is still its high bits, so a program that looks at its seed or scales the result
  behaves as before, and only one that sets a seed again to replay a sequence sees the
  difference, which is what the setting asks for. Keeping the seed cell's role, rather than
  ignoring it, was the choice here: it also keeps the access violation for a seed the caller
  can't write. Off is what the probe and every oracle compare, as `vax.hardware.clock`'s
  default is for time.

  "The remaining conditions" turned out to be none. `conditions_test.go`'s
  `TestPagesConditions` is an audit: each condition each of the 7.3 manual's routine pages
  lists, one call per name (249 calls), and every one was already signaled, with the right
  signal array. `MTH$HTAN`'s and `TAN`'s `FLOOVEMAT` stay out, no argument reaching them
  (subtask 4). `MTH$_SIGLOSMAT` and `MTH$_NOTIMP` are in no routine's list, and nothing signals
  them. `TestConditionMessages` is each condition govax does signal, with no handler: the
  message file's text, the user PC, the image's end. `TestLeftAsStubs` holds what's left: every
  CALL entry now has its routine, and the 52 stubs are JSB entries no manual gives, 9 of them
  the internal `MTH$$REDUCE_`, the survey's 62 less the 10 second entries.

  `random_test.go` holds the formula to wider integers and to a sequence worked out apart, the
  value to rationals at 100,000 seeds and the range's edges, and the spread over 16 bins;
  `TestRandomFromAProgram` runs the routine with bit patterns worked out apart, and arguments
  it can't reach; `TestRandomSetting` and `TestRandomSettingFromAProgram` run it both ways. The
  three "went on after a severe condition" tests used a call of the `MTH$RANDOM` stub as their
  sign, and now print a line instead, and `TestMTHRTLLinks` has `MTH$RANDOM` answering beside
  the JSB stub's NOTIMPL. Breaking the code showed the checks can tell, 1 to 24 failures for
  each of 10 changes: a wrong multiplier 24 or increment 8, 23 bits 6, the wrong scale 8, no
  store 8, a failed store not signaled 2, the setting reversed 11 or ignored 2, a generator
  with 16 values 1, a fixed seed 2.
