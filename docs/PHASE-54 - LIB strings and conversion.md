# Phase 54 — LIB$ strings, descriptors, and text conversion

**Status:** in progress (2026-10-12; subtasks 1 to 7 done). Needs Phase 53. Part of the LIB$
program laid out in `PHASE-53 - RTL LIB support.md` (Part A), which also
has the sources and the conventions (Part B) this phase follows.

## Goal

The LIB$ routines that copy, allocate, and examine strings by
descriptor; character and numeric text conversion; and the locale
routines that read a logical name. With them, the dynamic string becomes
a real RTL object. Phase 53's output helper allocates one with
LIB$SGET1_DD's own code, so every routine that returns a string, in this
phase and later ones, shares one allocator.

This phase also brings two new kinds of transfer-vector entry:

- **JSB entry points.** `LIB$SCOPY_DXDX6`, `LIB$SCOPY_R_DX6`,
  `LIB$SGET1_DD_R6`, `LIB$SFREE1_DD6`, `LIB$SFREEN_DD6`, and
  `LIB$ANALYZE_SDESC_R2` take their arguments in registers (the manual:
  "with R0 containing the first argument ...") and return with RSB. A
  CALLS stub's entry mask, reached by JSB, would run as a HALT
  (`machine/shim.go`'s note on `kernelLinks`). They need a stub without
  an entry mask that keeps the registers the routine doesn't return in,
  ending in RSB, and a way for the Go function to read its arguments
  from registers rather than an argument list.
- **Data.** `LIB$AB_ASC_EBC`, `LIB$AB_EBC_ASC`, and `LIB$AB_UPCASE`
  (offsets 0x000, 0x100, and 0x200) are 256-byte translation tables, not
  routines. Their `SHIM$LIBRTL_<offset>` symbols must name the tables
  themselves, laid down in system space with the stubs.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Dynamic strings | SGET1_DD (350), SFREE1_DD (347), SFREEN_DD (348), and their JSB entries SGET1_DD_R6, SFREE1_DD6, SFREEN_DD6 |
| Copying | SCOPY_DXDX (336), SCOPY_R_DX (338), and JSB entries SCOPY_DXDX6, SCOPY_R_DX6 |
| Descriptors | ANALYZE_SDESC (10), ANALYZE_SDESC_R2 (JSB) |
| Characters and search | LEN (257), CHAR (25), ICHAR (227), INDEX (229) |
| Translation | TRA_ASC_EBC (453), TRA_EBC_ASC (457); tables AB_ASC_EBC, AB_EBC_ASC, AB_UPCASE |
| Numeric text | CVT_DTB, CVT_HTB, CVT_OTB (76, "LIB$CVT_xTB") |
| Keywords | LOOKUP_KEY (261) |
| Locale | DIGIT_SEP (120), RADIX_POINT (313), CURRENCY (59), LP_LINES (267): SYS$DIGIT_SEP, SYS$RADIX_POINT, SYS$CURRENCY, SYS$LP_LINES, with the manual's defaults |
| LIBRTL2 | CVT_DX_DX (61): general data-type conversion between descriptors; needs LIBRTL2.EXE's offset (Phase 53, subtask 13) |

## What earlier phases leave in place

- Phase 53's `descriptor.go`: input of every class, S/VS/D output,
  LIB$_STRTRU, LIB$_INVSTRDES.
- The heap: `corevms.Environment.AllocateVM`/`FreeVM`, shared with
  DECC$MALLOC and LIB$GET_VM. Phase 59 gives it zones; this phase
  allocates dynamic strings as one zone's blocks if 59's design is
  already known, and from the default heap otherwise.
- The STR$ facility: only STR$UPCASE is emulated. The STR$ manual is a
  separate book (`OVMS_73_RTL_STR.PDF`, 7.3; `AA-LA75A-TE`, 5.0), and its
  routines are out of this program's scope. They'd share this phase's
  dynamic-string code when they come.

## Open questions

- ~~**JSB stubs**: how the shim learns its arguments and which registers
  the routine must preserve.~~ Settled by the survey ("The JSB entry
  points"): arguments in R0, R1, R2 by the STR$ manual's rule, a
  `PUSHL #code` / `XFC #XFC$JSBSHIM` / `RSB` stub, and only the
  registers the routine returns changed. Recorded in Phase 53's Part B
  with subtask 2.
- ~~**The EBCDIC tables**: which mapping they are.~~ Transcribed from the
  PDF's page images (the survey's "Data entries"); DIGITAL's own mapping,
  to be compared with a VMS dump by the probe.
- ~~**CVT_DX_DX (author)**: the 5.0 manual's table of accepted
  conversions is misprinted (the survey).~~ The author accepted the
  proposal (2026-10-12): in this phase, the integer types and F/D/G/H to
  and from text, with a LIBRTL2 stub area (subtask 10).

## The survey (subtask 1)

Each routine's interface, from the 5.0 LIB$ manual, in the notation of
Phase 53's survey (`name.access-type.mechanism`; `[ ]` optional). Page
numbers are the manual's LIB-nnn where the entry starts. The OCR text
was checked against the PDF wherever it mattered: the three translation
tables and CVT_DX_DX's Table LIB-2 were read from the page images
(PDF pages 446, 609, 613, 219). Two other DIGITAL books fill gaps the
LIB$ manual leaves: the 5.0 OTS$ manual (`AA-LA73A-TE`), whose
OTS$SCOPY_*, OTS$SGET1_DD, and OTS$SFREEx_DD entries describe the same
JSB entry points with their registers, and the 7.3 STR$ manual
(`OVMS_73_RTL_STR.PDF`), whose section 2.3 gives the JSB argument rule.
Choices the manuals leave open are marked **unconfirmed**, each a case
for the probe (subtask 11). Every condition value named here is in
`vmsdef.LibrarySymbols`.

### The JSB entry points

The STR$ manual (7.3, section 2.3.2): "Routines in the LIB$ and STR$
facilities pass scalar input arguments by reference to CALL entry points
and by immediate value to JSB entry points." A JSB entry "does not
establish a stack frame and executes entirely in the environment of the
calling program", so it can't have a condition handler, and its caller
saves the registers it uses. The arguments go in R0, R1, R2, ... in the
CALL format's order. The LIB$ manual says so for the two copy routines
(R0 the first argument, R1 the second, R2 the third; SCOPY_R_DX6's length
"in bits 15:0 of R0"); the OTS$ manual says the same of its SGET1 and
SFREE entries, whose arguments are already by value. The suffix names
the registers the routine may change: `_R2` is R0 to R2, `_R6` and the
bare `6` (`DXDX6`, `DD6`) R0 to R6.

| JSB entry | Registers in | Registers out |
| --- | --- | --- |
| SCOPY_DXDX6 | R0 source descriptor, R1 destination descriptor | R0 status |
| SCOPY_R_DX6 | R0<15:0> length, R1 source address, R2 destination descriptor | R0 status |
| SGET1_DD_R6 | R0<15:0> length, R1 descriptor | R0 status |
| SFREE1_DD6 | R0 descriptor | R0 status |
| SFREEN_DD6 | R0 count, R1 first descriptor | R0 status |
| ANALYZE_SDESC_R2 | R0 descriptor | R0 status, R1 length, R2 data address |

- **ANALYZE_SDESC_R2**'s outputs are govax's reading (**unconfirmed**):
  the STR$ manual's STR$ANALYZE_SDESC_R1 returns its two outputs "by
  value" at a JSB entry, which with no status means R0 and R1; the LIB$
  routine adds a status, and `_R2` leaves room for exactly three.
- **The other registers.** The OTS$ copy routines' JSB entries leave R0
  to R5 "as they would be after a MOVC5 instruction"; the LIB$ manual
  says nothing of R1 to R6. govax changes only the registers in the
  table and leaves the rest as they were (**unconfirmed**; within the
  contract either way, since the caller has saved them). The probe can
  print them.
- **The stub (decided here).** A CALL stub's `MOVL #code,R0` would
  destroy the first argument, and its entry mask would run as a HALT. A
  JSB stub is `PUSHL #code` (`DD 8F` and the code), `XFC #XFC$JSBSHIM`
  (a new selector), `RSB`: 9 bytes, padded to the 12 of a CALL stub so
  the stub area stays one array. XFC$JSBSHIM pops the code, finds the
  routine, and hands it the registers instead of an argument list; the
  routine returns the registers it changes. In `librtl.Routines` such a
  row is flagged JSB, with its own Go function, which usually reads the
  registers into the CALL form's arguments and calls the same code. A
  signal from inside one (an access violation) is dispatched from the
  caller's frame, which is what VMS does for a routine without one.
  Recorded in Phase 53's Part B as Decision 8 when subtask 2 lands.
- **The OTS$ twins.** LIBRTL.EXE's transfer vector also has
  OTS$SCOPY_DXDX(6), OTS$SCOPY_R_DX(6), OTS$SGET1_DD(_R6), and
  OTS$SFREE1_DD(6)/OTS$SFREEN_DD(6), at their own offsets. They aren't
  LIB$'s aliases: the OTS$ manual has them signal every error but
  truncation, return MOVC5's R0 (the bytes not copied), and take
  scalars by value at the CALL entry too. They're the OTS$ facility's,
  outside this program; their CALL entries stay missing-routine stubs.
  They'd be small once the LIB$ code is in place.

### Data entries

`LIB$AB_ASC_EBC`, `LIB$AB_EBC_ASC`, and `LIB$AB_UPCASE` are LIBRTL's
offsets 0x000, 0x100, and 0x200: the transfer vector's first 768 bytes
are the tables themselves, which a program addresses (`MOVTC
...,G^LIB$AB_UPCASE,...`; the manual's BASIC example puts
`LOC(LIB$AB_ASC_EBC)` in a descriptor). So their `SHIM$LIBRTL_<offset>`
symbols name 256-byte tables in system space, read-only to every mode,
laid down with the stubs. The other eleven `LIB$AB_` tables in the
manual's MOVTC entry (Tables LIB-8 to LIB-20: the `_REV` tables, the
packed and zoned conversion tables, `LIB$AB_LOWERCASE`) are LIBRTL2's
and out of scope; they'd need a LIBRTL2 stub area.

The tables, as the manual prints them (rows by the high hex digit here,
where the manual's rows are the low digit):

LIB$AB_ASC_EBC (Table LIB-23, LIB-454): ASCII to EBCDIC.

    00: 00 01 02 03 37 2D 2E 2F 16 05 25 0B 0C 0D 0E 0F
    10: 10 11 12 13 3C 3D 32 26 18 19 3F 27 1C 1D 1E 1F
    20: 40 4F 7F 7B 5B 6C 50 7D 4D 5D 5C 4E 6B 60 4B 61
    30: F0 F1 F2 F3 F4 F5 F6 F7 F8 F9 7A 5E 4C 7E 6E 6F
    40: 7C C1 C2 C3 C4 C5 C6 C7 C8 C9 D1 D2 D3 D4 D5 D6
    50: D7 D8 D9 E2 E3 E4 E5 E6 E7 E8 E9 4A E0 5A 5F 6D
    60: 79 81 82 83 84 85 86 87 88 89 91 92 93 94 95 96
    70: 97 98 99 A2 A3 A4 A5 A6 A7 A8 A9 C0 6A D0 A1 07
    80-FE: 3F            FF: FF

LIB$AB_EBC_ASC (Table LIB-24, LIB-458): EBCDIC to ASCII.

    00: 00 01 02 03 5C 09 5C 7F 5C 5C 5C 0B 0C 0D 0E 0F
    10: 10 11 12 13 5C 5C 08 5C 18 19 5C 5C 1C 1D 1E 1F
    20: 5C 5C 5C 5C 5C 0A 17 1B 5C 5C 5C 5C 5C 05 06 07
    30: 5C 5C 16 5C 5C 5C 5C 04 5C 5C 5C 5C 14 15 5C 1A
    40: 20 5C 5C 5C 5C 5C 5C 5C 5C 5C 5B 2E 3C 28 2B 21
    50: 26 5C 5C 5C 5C 5C 5C 5C 5C 5C 5D 24 2A 29 3B 5E
    60: 2D 2F 5C 5C 5C 5C 5C 5C 5C 5C 7C 2C 25 5F 3E 3F
    70: 5C 5C 5C 5C 5C 5C 5C 5C 5C 60 3A 23 40 27 3D 22
    80: 5C 61 62 63 64 65 66 67 68 69 5C 5C 5C 5C 5C 5C
    90: 5C 6A 6B 6C 6D 6E 6F 70 71 72 5C 5C 5C 5C 5C 5C
    A0: 5C 7E 73 74 75 76 77 78 79 7A 5C 5C 5C 5C 5C 5C
    B0: 5C 5C 5C 5C 5C 5C 5C 5C 5C 5C 5C 5C 5C 5C 5C 5C
    C0: 7B 41 42 43 44 45 46 47 48 49 5C 5C 5C 5C 5C 5C
    D0: 7D 4A 4B 4C 4D 4E 4F 50 51 52 5C 5C 5C 5C 5C 5C
    E0: 5C 5C 53 54 55 56 57 58 59 5A 5C 5C 5C 5C 5C 5C
    F0: 30 31 32 33 34 35 36 37 38 39 5C 5C 5C 5C 5C FF

LIB$AB_UPCASE (Table LIB-19, LIB-291): the identity, except 61-7A to
41-5A, E0-EF to C0-CF, and FA-FD to DA-DD. (So F0-F9, F7 among them,
and FE and FF are unchanged, as is 60. That's what the table shows,
though it isn't DEC Multinational's full case mapping: **unconfirmed**
against VMS's own table, which the probe dumps.)

- **The mapping.** ASC_EBC is one-to-one on 00-7F and its inverse is
  exactly EBC_ASC's 128 translatable entries (checked by computing both
  directions from the transcription: every ASCII character comes back,
  and every EBCDIC code EBC_ASC translates comes back), plus FF to FF
  both ways. The untranslatable entries are 3F (EBCDIC SUB) one way and
  5C (ASCII backslash) the other. It's DIGITAL's own choice of mapping
  (ASCII `[` is 4A, `]` 5A, `!` 4F, `|` 6A), taken from the manual as
  printed; the probe dumps VMS's tables to **compare**.

### Dynamic strings

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| SGET1_DD (349) | `word-integer-length.rwu.r, descriptor-part.wq.r` | cond | SS$_NORMAL; LIB$_FATERRLIB; LIB$_INSVIRMEM |
| SFREE1_DD (346) | `descriptor-address.mq.r` | cond | SS$_NORMAL; LIB$_FATERRLIB |
| SFREEN_DD (348) | `number-of-descriptors.rlu.r, first-descriptor-array.mq.ra` | cond | SS$_NORMAL; LIB$_FATERRLIB |

- **SGET1_DD** sets DSC$B_CLASS to D (2), DSC$W_LENGTH to the length, and
  DSC$A_POINTER to the area; the class isn't checked first, nor DSC$B_DTYPE
  set (govax leaves the type byte, **unconfirmed**). A length of 0
  allocates "a small amount of space"; the descriptor's length is 0 and
  its pointer nonzero. The area may be rounded up, and existing storage
  is kept when it's at least the length (the LIB$ entry: freed only when
  "less than"; the OTS$ entry frees it when "greater than or less than",
  govax follows LIB$'s). So the allocator has to know a block's real
  size, not the descriptor's length: the heap does (`memAllocated`), and
  subtask 3 asks it, which also lets `writeString`'s D class keep storage
  a shorter string left behind (today it compares with DSC$W_LENGTH).
- **SFREE1_DD and SFREEN_DD** free each area (pointer nonzero) and set
  the pointer and length to 0. The class isn't checked. A pointer that
  isn't a heap block's (a static area) is LIB$_FATERRLIB, "an internal
  consistency check has failed", **unconfirmed**; SFREEN_DD stops at the
  first such descriptor (**unconfirmed**). A count of 0 does nothing.
- **Where the storage comes from.** Phase 59 hasn't designed zones yet,
  so dynamic strings come from the default heap
  (`Environment.AllocateVM`, zone 0), and move to the default zone with
  Phase 59 (its subtask 4 already says so).

### Copying and descriptors

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| SCOPY_DXDX (335) | `source-string.rt.dx, destination-string.wt.dx` | cond | SS$_NORMAL; LIB$_STRTRU; LIB$_FATERRLIB; LIB$_INSVIRMEM; LIB$_INVSTRDES |
| SCOPY_R_DX (337) | `word-integer-source-length.rwu.r, source-string-address.rt.r, destination-string.wt.dx` | cond | the same |
| ANALYZE_SDESC (9) | `input-descriptor.rt.dx, data-length.wwu.r, data-address.wa.r` | cond | SS$_NORMAL; LIB$_INVSTRDES |

- **SCOPY_DXDX and SCOPY_R_DX** write by the destination's class with
  exactly Phase 53's `writeString` table (which came from this entry):
  S, Z, SD, A, NCA filled or truncated; D reallocated when too small; VS
  up to MAXSTRLEN, CURLEN set. Truncation is LIB$_STRTRU, a success.
  The source of SCOPY_DXDX is any input class (Z, S, D, SD, A, NCA, VS).
- **ANALYZE_SDESC** is `readDescriptor` as a routine (the class table
  Phase 53 took from this entry: A and NCA by ARSIZE, at most 65,535;
  VS's CURLEN; D, SD, Z as S). The length is a word, the address a
  longword. An invalid class writes nothing (**unconfirmed**).

### Characters and search

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| LEN (256) | `source-string.rt.dx` | the length, zero-extended | none |
| CHAR (24) | `one-character-string.wt.dx, ascii-code.rbu.r` | cond | SS$_NORMAL; LIB$_STRTRU; LIB$_FATERRLIB; LIB$_INSVIRMEM; LIB$_INVSTRDES |
| ICHAR (226) | `source-string.rt.dx` | the first character, zero-extended; 0 for an empty string | none |
| INDEX (228) | `source-string.rt.dx, sub-string.rt.dx` | position (1-based) of the substring, or 0 | none |

- **CHAR** writes one character, blank-filled to a fixed destination's
  length; a dynamic one gets length 1. A zero-length fixed destination
  is LIB$_STRTRU (**unconfirmed**).
- **INDEX**: an empty substring gives 1 "no matter how long the source
  string is" (so 1 for an empty source too); an empty source with a
  nonempty substring gives 0. That's `cpu.MatchCharacters`, the rule
  LIB$MATCHC already follows (with its arguments the other way round).
- **ICHAR's example** says "this routine changes any uppercase characters
  to lowercase"; its own output (80 for "Pencil", 112 for "pencil") shows
  it doesn't. The text is wrong; no case change.
- **No status to return.** LEN, ICHAR, and INDEX return values, so an
  invalid descriptor can't be a status. VMS's run of Phase 53's probe
  showed LIB$LOCC of a class 3 source returning 0 with no signal; govax
  does the same here (0, **unconfirmed** for these routines).

### Translation

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| TRA_ASC_EBC (452) | `source-string.rt.dx, byte-integer-dest-string.wt.dx` | cond | SS$_NORMAL; LIB$_INVCHA; LIB$_INVARG |
| TRA_EBC_ASC (456) | `byte-integer-source-string.rt.dx, destination-string.wt.dx` | cond | SS$_NORMAL; LIB$_INVCHA; LIB$_INVARG |

- **A fixed destination must be the source's length**: otherwise
  LIB$_INVARG, nothing translated. "No filling is done." A dynamic
  destination takes the source's length, and a varying one the
  source's length if MAXSTRLEN allows (LIB$_INVARG otherwise:
  **unconfirmed**).
- **LIB$_INVCHA**: "one or more occurrences of an untranslatable
  character": the whole string is still translated (each such character
  becomes the table's 3F or 5C), and the status says so. Which
  characters count is govax's reading of the tables (**unconfirmed**):
  one whose translation doesn't translate back (ASCII 80-FE; the 127
  EBCDIC codes outside the table's image). So ASCII 1A (SUB, a real
  mapping to EBCDIC 3F) and EBCDIC E0 (to backslash) are translatable,
  and ASCII 5C is too.

### Numeric text

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| CVT_DTB, CVT_HTB, CVT_OTB (75) | `byte-count.rl.v, numeric-string.rt.r, result.wl.r` | 1 or 0 | 1 success; 0 failure |

- **By value and by reference**, unusually: the count is a value, the
  string an address ("intended to be called primarily from BLISS and
  MACRO").
- **The statuses are 1 and 0**, not condition values: 0 for a character
  that isn't a digit of the radix, a sign anywhere but first, a blank or
  tab, or "an overflow from 32 bits (unsigned)".
- **Signs**: a leading `+` or `-`; `-` negates. The overflow test is on
  the unsigned magnitude (so `4294967295` is accepted, as -1;
  **unconfirmed**), and the negation is two's complement. Hex digits
  either case (**unconfirmed** for lowercase). A count of 0 (or a sign
  alone) gives 0 and success (**unconfirmed**). A negative count:
  failure (**unconfirmed**). Whether result is written on failure:
  govax doesn't (**unconfirmed**).

### Keywords

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| LOOKUP_KEY (260) | `search-string.rt.dx, key-table-array.rlu.ra [,key-value.wlu.r] [,keyword-string.wt.dx] [,resultant-length.wwu.r]` | cond | SS$_NORMAL; LIB$_AMBKEY; LIB$_UNRKEY; LIB$_INVARG; LIB$_INSVIRMEM; LIB$_STRTRU |

- **The table** (Figure LIB-5): a longword vector-count, the number of
  longwords that follow (twice the number of keywords), then for each
  keyword the address of a counted ASCII string and its longword value.
  An odd vector-count is "bad keyword table", LIB$_INVARG
  (**unconfirmed**).
- **Matching**: the search string matches a keyword whose first n
  characters are it (n its length). An exact match (equal lengths) ends
  the search with SS$_NORMAL; otherwise the rest of the table is
  scanned, and a second match is LIB$_AMBKEY, one alone SS$_NORMAL, none
  LIB$_UNRKEY. Case: the manual says nothing; govax compares exactly as
  given (**unconfirmed**; a program upcases its input, the keywords
  being uppercase). An empty search string matches every keyword
  (AMBKEY with two or more: **unconfirmed**).
- **Outputs**: key-value gets the keyword's value, keyword-string the
  full keyword (any class, so `writeString`, LIB$_STRTRU when it's cut),
  resultant-length its length "independent of padding". The manual
  writes them on a match; on AMBKEY, govax writes the first match's
  (**unconfirmed**). LIB$_INVARG for "not enough arguments": fewer than
  two (`tooFewArgs`).
- **`$LIB_KEY_TABLE`** is a STARLET macro (the manual's MACRO example:
  `$LIB_KEY_TABLE <<ADD, 1>, <DELETE, 2>, <EXIT, 3>>`). govax's
  `starlet.mar` lacks it; subtask 8 writes it from the figure (clean
  room), and the probe's object checks its layout.

### Locale

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| DIGIT_SEP (119) | `digit-separator-string.wt.dx [,resultant-length.wwu.r]` | cond | SS$_NORMAL; LIB$_STRTRU; LIB$_FATERRLIB; LIB$_INSVIRMEM; LIB$_INVSTRDES |
| RADIX_POINT (312) | `radix-point-string.wt.dx [,resultant-length.wwu.r]` | cond | the same |
| CURRENCY (58) | `currency-string.wt.dx [,resultant-length.wwu.r]` | cond | the same |
| LP_LINES (266) | none | the page length | none |

- **Each translates its logical name** "as a process, group, or system
  logical name, in that order" (the process, group, and system tables;
  govax's `lnm` lookup through LNM$FILE_DEV searches them in that order
  unless a job table comes between), defaulting to `,`, `.`, and `$`. The translation's text is
  returned whole (it may be more than one character). resultant-length
  is what was written, the truncated length when cut.
- **LP_LINES**: SYS$LP_LINES converted to binary (as CVT_DTB would,
  **unconfirmed**), in [30, 255], else 66.

### LIBRTL2: CVT_DX_DX

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| CVT_DX_DX (60) | `source-item.rz.dx, destination-item.wz.dx [,word-integer-dest-length.wwu.r]` | cond | SS$_NORMAL; LIB$_DECOVF, FLTOVF, FLTUND, INTOVF, ROPRAND; LIB$_INVCLADSC, INVCLADTY, INVDTYDSC; LIB$_INVCVT; LIB$_INVNBDS; LIB$_OUTSTRTRU |

- **Offset**: LIBRTL2.EXE's 0x000 (`vmsdef.ImageSymbols`, from Phase
  53's subtask 13).
- **What it converts** is "a subset of the VAX standard data types",
  between atomic types and numeric byte data strings (NBDS: text of
  class A, D, NCA, S, SD, VS; BU arrays; VT). Table LIB-3 gives an NBDS
  destination's format for every atomic source: B, BU, W, WU, L, LU, Q
  as digits, D/F/G/H in E format, an NBDS by the architecture's rules.
  An NBDS source is read as OTS$CVT_T_z reads text (blanks and tabs
  ignored). Scale applies to class SD.
- **Table LIB-2 is misprinted.** The PDF's "Data Types Accepted by
  LIB$CVT_DX_DX" is a two-column list (text in decimal, hex, octal,
  binary, signed integer, and logical to longword; text to D, F, G, H;
  longword to text in five radixes) that matches neither the
  description (no B, W, Q, or unsigned types) nor Table LIB-3. It reads
  like another routine's table. So which conversions VMS accepts isn't
  settled by the manual.
- **Scope (subtask 10; accepted by the author, 2026-10-12)**: the integer types (B, BU, W, WU,
  L, LU, Q) and F, D, G, H to and from an NBDS of text (T, VT; classes S,
  D, VS, A, NCA, SD), with LIB$_INVCLADTY for any other pair, and the
  packed-decimal and BU-array forms left out. An NBDS to NBDS copy is
  `writeString` (LIB$_OUTSTRTRU on truncation). The text forms come from
  `internal/vaxfloat` (decimal parsing and printing) and Go's integer
  conversion. This needs a LIBRTL2 stub area: a second library in
  `librtl.Routines` (the `SHIM$LIBRTL2_00000000` symbol), whose
  missing-routine stubs (DECODE_FAULT, SIM_TRAP, the zone routines) come
  with it.

## Subtasks (provisional)

1. **Survey**: each routine's format, arguments, and condition values;
   the JSB entries' register conventions. **Done** 2026-10-12.
2. **JSB stubs and data entries** in the stub area and the table.
   **Done** 2026-10-12.
3. **Dynamic strings**: SGET1_DD, SFREE1_DD, SFREEN_DD (and JSB forms);
   Phase 53's D-class output moved onto them. **Done** 2026-10-12.
4. **Copying and descriptors**: SCOPY_DXDX, SCOPY_R_DX, ANALYZE_SDESC (and
   JSB forms). **Done** 2026-10-12.
5. **Characters**: LEN, CHAR, ICHAR, INDEX. **Done** 2026-10-12.
6. **Translation**: the three tables, TRA_ASC_EBC, TRA_EBC_ASC.
   **Done** 2026-10-12 (the tables in subtask 2).
7. **Numeric text**: CVT_DTB, CVT_HTB, CVT_OTB. **Done** 2026-10-12.
8. **LOOKUP_KEY**, and the `$LIB_KEY_TABLE` macro in `starlet.mar`.
9. **Locale**: DIGIT_SEP, RADIX_POINT, CURRENCY, LP_LINES.
10. **CVT_DX_DX** (LIBRTL2's 0x000): integers and F/D/G/H to and from
    text, with a LIBRTL2 stub area (the scope the author accepted).
11. **The program** (`testdata/lib54/`) and its console test; VMS probe
    with the author if wanted.
12. **Close-out**.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
- **2026-10-12** — Subtask 1, the survey ("The survey" above). Read
  every entry of the phase in the 5.0 LIB$ manual's OCR text, and the
  page images where the OCR failed: the three translation tables
  (transcribed, and checked by computing both directions: ASC_EBC is
  one-to-one on 00-7F and EBC_ASC its inverse) and CVT_DX_DX's Table
  LIB-2. The LIB$ manual gives JSB registers only for the two copy
  routines; the 5.0 OTS$ manual's twin entries and the 7.3 STR$
  manual's section 2.3 settle the rest (arguments in R0, R1, R2,
  scalars by value). Findings that shape the plan:
  - A JSB stub can't load its shim code into R0 (the first argument),
    so it pushes it: `PUSHL #code`, a new `XFC #XFC$JSBSHIM`, `RSB`.
  - SGET1_DD keeps storage that's big enough, judged by the block's
    real size, so the heap has to say how big a block is; Phase 53's
    `writeString` D class then keeps storage the same way.
  - The OTS$ twins of the copy and dynamic-string routines aren't
    aliases (they signal, and return MOVC5's R0); they stay missing.
  - LIB$ICHAR's example text contradicts its own output (no case change).
  - LOOKUP_KEY needs the `$LIB_KEY_TABLE` macro, which govax's STARLET
    lacks.
  - CVT_DX_DX's accepted-conversions table is misprinted in the 5.0
    manual; its scope is the author's call (Open questions), and it
    needs a LIBRTL2 stub area.

  No code changed.
- **2026-10-12** — The author accepted the proposed scope for
  CVT_DX_DX: this phase, integers and F/D/G/H to and from text, with a
  LIBRTL2 stub area (subtask 10).
- **2026-10-12** — The author asked for the remaining subtasks to run
  back to back, each committed when complete and tested, unpushed.
- **2026-10-12** — Subtask 2, JSB stubs and data entries (Phase 53's
  Part B, Decision 8):
  - **The CPU.** `XFC #XFC$JSBSHIM` (selector 5) pops the shim code off
    the stack and calls `SystemServices.JSBShim(code)`; a waiting shim
    runs again with the code pushed back. `TestEmulXfcJSBShim` and
    `TestEmulXfcJSBShimWait` run a stub reached by JSB.
  - **corevms.** `JSBShimFunc` works on `JSBRegisters` (R0 to R6), which
    `Environment.JSBShim` reads and puts back, unless the routine started
    a condition dispatch. `ShimTable.RegisterJSB`/`LookupJSB`/`Has` (CALL
    and JSB routines share one space of codes). While a JSB shim runs
    (`jsbCall`, `jsbSP`), `signal` raises a condition in the caller's
    frame, at the return address on the stack, and resumes with SP as
    the JSB left it, so SS$_CONTINUE returns through the stub's RSB.
    `ShimLink` gained `JSB` and `Data`; `JSBShimStub` is the stub.
    `CRTLLinks`' literals are now keyed.
  - **The stub area.** `machine.EnsureShims` writes a JSB stub for a JSB
    row (its name a plain symbol, not an entry: it has no mask) and
    copies each data row into the S0 overflow pages after the stubs,
    quadword aligned, defining its SHIM$ symbol and its name there.
  - **librtl.** `JSBRoutines` (empty until subtask 3), `Tables`
    (`tables.go`: the survey's three tables), and `MissingJSB`: every
    JSB entry govax lacks (all 43 for now, LIB$SCOPY_DXDX6 and OTS$MOVE3_R5 among them) gets a
    stub that signals SS$_UNSUPPORTED and GOVAX-E-NOTIMPL as a missing
    CALL entry's does. So every entry of LIBRTL.EXE's transfer vector now
    resolves. `Links` puts Routines, JSBRoutines, and Tables first, so
    the stubs in VMINIT's page keep their addresses.
  - **Tests.** `TestMissing` now checks that every LIBRTL entry is
    covered once, by a CALL stub, a JSB stub, or a table, of the right
    kind; `TestTranslationTables` and `TestUpcaseTable` (the tables'
    inverse property and the manual's spot values);
    `TestEnsureShims_overflowPages` checks JSB stubs and the tables'
    contents; the console's `TestLibrtlMissingJSBEntry` (a JSB to
    OTS$MOVE3_R5: the catch-all's message, the JSB returning
    SS$_UNSUPPORTED with SP and R6 as they were),
    `TestLibrtlMissingJSBEntryContinued` (the caller's handler sees it),
    and `TestLibrtlDataTables` (MOVTC through `G^LIB$AB_UPCASE`, and to
    EBCDIC and back).
- **2026-10-12** — Subtask 3, dynamic strings (`internal/librtl/
  dynstr.go`): LIB$SGET1_DD, LIB$SFREE1_DD, LIB$SFREEN_DD (codes 79 to
  81) and the JSB entries LIB$SGET1_DD_R6, LIB$SFREE1_DD6,
  LIB$SFREEN_DD6 (82 to 84), the first `JSBRoutines`.
  - **The storage rule** is one function, `dynamicText`: the block at
    the descriptor's pointer is kept when it holds the new length, judged
    by the heap block's real size (`corevms.Environment.BlockSize`, new),
    or by the descriptor's length when the pointer isn't a heap block's
    (storage the program set up); otherwise the block is freed and a new
    one allocated. `writeString`'s class D output now uses it, so a
    dynamic string keeps its storage through a shorter value and a
    longer one that still fits (it used to compare with DSC$W_LENGTH,
    so it reallocated more often).
  - **LIB$SGET1_DD** sets the length, class D (the type byte left), and
    pointer; a length of 0 still allocates (the heap's smallest block).
    **LIB$SFREE1_DD** frees and zeroes the pointer and length; a zero
    pointer is SS$_NORMAL, and a pointer that isn't a heap block's is
    LIB$_FATERRLIB with the descriptor untouched (unconfirmed).
    **LIB$SFREEN_DD** stops at the first failure (unconfirmed). An
    unreachable descriptor is `memoryFault`'s signaled SS$_ACCVIO.
  - **The JSB entries** read R0's low word as the length (SGET1_DD_R6),
    the count by value (SFREEN_DD6), and change only R0.
  - **Tests.** `dynstr_test.go`: allocation, storage kept within the
    block's size, a longer length's new block, length 0, freeing one and
    several, LIB$_FATERRLIB, the JSB entries' registers, and
    `writeString`'s kept block; `callJSB` (librtl_test.go) runs a JSB
    routine as the XFC would. The console's `TestLibrtlDynamicStrings`
    gets one string by CALL and one by JSB, writes and shows them, and
    frees both with LIB$SFREEN_DD6.
- **2026-10-12** — Subtask 4, copying and descriptors (`internal/librtl/
  copy.go`): LIB$SCOPY_DXDX, LIB$SCOPY_R_DX, LIB$ANALYZE_SDESC (codes 85
  to 87) and the JSB entries LIB$SCOPY_DXDX6, LIB$SCOPY_R_DX6,
  LIB$ANALYZE_SDESC_R2 (88 to 90).
  - **The copies** read the source (any input class, or length and
    address: `readText`, now shared with `readString`) and write it with
    `writeString`, returning SS$_NORMAL, LIB$_STRTRU, LIB$_INVSTRDES, or
    LIB$_INSVIRMEM. SCOPY_R_DX's length is a word by reference;
    SCOPY_R_DX6's R0's low word.
  - **ANALYZE_SDESC** writes the length (a word) and the text's address
    by `readDescriptor`'s rules; an invalid class is LIB$_INVSTRDES with
    nothing written (unconfirmed). ANALYZE_SDESC_R2 returns them in R1
    and R2, left alone on an error (unconfirmed, the survey's reading).
  - **Tests.** `copy_test.go`: every destination class, truncation, a
    varying source, a bad class on either side, the length word's high
    half ignored, the JSB entries' registers, and ANALYZE_SDESC by class
    (S, A's ARSIZE, VS's CURLEN) through both entries. The console's
    `TestLibrtlCopyAndAnalyze` copies into a dynamic string by CALL and
    by JSB and analyzes a `.ASCID` string.
- **2026-10-12** — Subtask 5, the string functions (`internal/librtl/
  strfuncs.go`): LIB$LEN, LIB$CHAR, LIB$ICHAR, LIB$INDEX (codes 91 to
  94). LEN is the descriptor's length by class; ICHAR the first
  character, unsigned, or 0 (no case change, whatever the manual's
  example text says); INDEX is LIB$MATCHC's search with the arguments the
  other way round (an empty substring at 1, even in an empty source). An
  invalid descriptor reads as an empty string, as LIB$LOCC's does on VMS
  (so 0 from LEN, ICHAR, and INDEX; unconfirmed for these). CHAR writes
  its one character through the copy routines' `copyString`: blank-filled
  to a fixed destination, LIB$_STRTRU into one of length 0
  (unconfirmed). Tests: `strfuncs_test.go` (classes, the manual's ICHAR
  and INDEX examples, empty strings) and the console's
  `TestLibrtlStringFunctions`. `TestLibrtlMissingRoutine` used LIB$INDEX
  as its missing routine; it now uses LIB$GETQUI, which isn't planned.
- **2026-10-12** — Subtask 6, translation (`internal/librtl/
  translate.go`): LIB$TRA_ASC_EBC and LIB$TRA_EBC_ASC (codes 95, 96),
  through subtask 2's tables. A character is untranslatable when the
  other table doesn't turn its translation back into it (ASCII 80-FE;
  the 127 EBCDIC codes outside the image): the string is still
  translated whole, and the status is LIB$_INVCHA (unconfirmed which
  characters count). A fixed destination of another length, or a
  varying one too small (unconfirmed), is LIB$_INVARG with nothing
  written; a dynamic one gets the source's length. Tests:
  `TestTraAscEbc` (both directions, LIB$_INVCHA with ASCII 1A and EBCDIC
  E0 counted translatable, LIB$_INVARG, VS and D destinations) and the
  console's `TestLibrtlTranslate` ("GOVAX" to EBCDIC and back, and
  LIB$_INVCHA from a linked program).
- **2026-10-12** — Subtask 7, numeric text (`internal/librtl/cvttb.go`):
  LIB$CVT_DTB, LIB$CVT_HTB, LIB$CVT_OTB (codes 97 to 99), the count by
  value and the text by reference, returning 1 or 0. One optional
  leading sign; every other character a digit of the radix (hex in
  either case); a magnitude over 32 bits unsigned fails; `-` negates in
  two's complement. The survey's open choices are as it proposed: no
  digits is 0 and success, a negative count fails, the result is written
  only on success (all unconfirmed). Tests: `cvttb_test.go` (each radix,
  signs, overflow at 2^32, blanks, tabs, a misplaced sign, the count
  bounding the text) and the console's `TestLibrtlCvtTextToBinary`.
