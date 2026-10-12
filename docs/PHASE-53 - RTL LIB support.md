# Phase 53 — LIB$ routines: the plan, the groundwork, and the instruction routines

**Status:** in progress (2026-10-12). Needs Phases 34 (`internal/librtl`)
and 52. The plan (Part A) and the decisions (Part B) were reviewed and
accepted by the author on 2026-10-12. Subtasks 2 to 5 are done.

govax provides 15 of LIBRTL.EXE's LIB$ routines (and STR$UPCASE): the ones
the sample programs and tests needed (Phase 34's table,
`internal/librtl/routines.go`). Real programs, and the compilers' own
code, call many more. A program that names a routine govax lacks doesn't
start: its G^ fixup finds no `SHIM$LIBRTL_<offset>` symbol, and RUN
reports LIB_UNRESOLVED (`machine/image.go`'s `resolveFixupTarget`).

This phase does three things:

- **Part A** maps every LIB$ routine onto a sequence of phases (53 to
  61), grouped by subject and ranked by how much they unlock. The later
  phases each have a planning doc of their own.
- **Part B** is the groundwork every later phase uses: room for many
  more stubs, string descriptors of every class, the routines' argument
  and status conventions, and how a routine govax lacks behaves.
- **Part C** is the first group: the routines that make a VAX
  instruction callable (bit fields, extended integer arithmetic,
  floating EMOD and POLY, the interlocked queues, the character-string
  instructions, CALLG, and CRC).

## Sources

- **The VMS RTL Library (LIB$) Manual**, VMS 5.0 (AA-LA76A-TE, April
  1988): `~/Documents/Technical Doc/VMS/AA-LA76A-TE_VMS_5.0_RTL_Library_LIB$_Manual_198804.pdf`,
  with an OCR text copy, `rtl-library-lib-manual.txt`, beside it. The
  scan is low resolution and the OCR has errors. The most common is a
  space inside a name with an underscore (`LIB$ADD_ TIMES`). Others are
  `O`/`Q`, `I`/`1`, `Y`/`V` swaps (`LIB$DQ_CQMMAND`, `LIB$1NSV`,
  `LIB$SVS_FAO`) and `DX` read as `ox`. A routine's page in the manual
  (LIB-nnn) is given in each phase's tables, so the PDF page can be
  checked when the text is doubtful.
- **No VMS 7.x LIB$ manual is at hand** (the author, 2026-10-12). The
  5.0 manual is the reference. A routine that VMS 7.3's LIBRTL.EXE has
  but the 5.0 manual doesn't describe is marked **7.x only**. It is
  implemented only if its interface can be settled from other DIGITAL
  documentation or a VMS probe, and otherwise left out and listed.
- **VMS 7.3's LIBRTL.EXE transfer vector**: `vmsdef.ImageSymbols`
  (generated from the image's global symbol table, Phase 31) has every
  universal symbol and its offset. That is 188 LIB$ names: 184 routines,
  three data tables (`LIB$AB_*`), and `LIB$$GET_VM_POINTERS`. So no probe
  is needed for LIBRTL's offsets.
- **What isn't in LIBRTL.EXE.** Seven routines in the 5.0 manual aren't
  in 7.3's LIBRTL.EXE: `LIB$CVT_DX_DX`, `LIB$DECODE_FAULT`,
  `LIB$SIM_TRAP`, `LIB$FIND_VM_ZONE`, `LIB$SHOW_VM_ZONE`, and
  `LIB$VERIFY_VM_ZONE`, which by their names are in LIBRTL2.EXE; and
  `LIB$CRF_INS_KEY`, `LIB$CRF_INS_REF`, and `LIB$CRF_OUTPUT`, in
  CRFSHR.EXE. Their offsets need the images' symbol tables. The author
  offered (2026-10-12) to run ANALYZE/IMAGE on the simh VAX; see
  subtask 13.
- **Clean room** (CLAUDE.md): routines are written from the manual's
  descriptions and checked against VMS's behavior where a probe can.
  The VAX Architecture Reference Manual
  (`~/Documents/Technical Doc/VMS/vax_instr_set.pdf`) is the reference
  for the instruction routines' semantics. govax's own CPU
  (`internal/cpu`, checked by `TestInsn35Oracle`) and `internal/vaxfloat`
  already implement them.

# Part A — the plan

## The inventory

Every LIB$ name in VMS 7.3's LIBRTL.EXE, plus the manual's routines that
live elsewhere, sorted into phases. "Done" means implemented before this
phase. Page numbers are the 5.0 manual's (LIB-nnn); "7.x" marks a
routine the 5.0 manual doesn't describe.

| Phase | Subject | Routines |
| --- | --- | --- |
| done | (Phase 34 and earlier) | ADAWI, CREATE_DIR, DELETE_VM_ZONE, ESTABLISH, FREE_VM, GET_FOREIGN, GET_INPUT, GET_VM, MATCH_COND, PUT_OUTPUT, REVERT, SIGNAL, SIG_TO_RET, SPAWN, STOP |
| 53 | VAX instruction routines | EXTV, EXTZV, INSV, FFC, FFS; ADDX, SUBX, EMUL, EDIV; EMODF, EMODD, EMODG, EMODH, POLYF, POLYD, POLYG, POLYH; CVTDF (7.x); INSQHI, INSQTI, REMQHI, REMQTI, BBSSI, BBCCI; LOCC, MATCHC, SCANC, SKPC, SPANC, MOVC3, MOVC5, MOVTC, MOVTUC; CALLG, CRC, CRC_TABLE; ADAWI reviewed |
| 54 | Strings, descriptors, and text conversion | SCOPY_DXDX, SCOPY_DXDX6, SCOPY_R_DX, SCOPY_R_DX6, SGET1_DD, SGET1_DD_R6, SFREE1_DD, SFREE1_DD6, SFREEN_DD, SFREEN_DD6, ANALYZE_SDESC, ANALYZE_SDESC_R2; LEN, CHAR, ICHAR, INDEX; TRA_ASC_EBC, TRA_EBC_ASC, and the tables AB_ASC_EBC, AB_EBC_ASC, AB_UPCASE; CVT_DTB, CVT_HTB, CVT_OTB; LOOKUP_KEY; DIGIT_SEP, RADIX_POINT, CURRENCY, LP_LINES; CVT_DX_DX (LIBRTL2) |
| 55 | System service jackets, process resources, and measurement | SYS_ASCTIM, SYS_FAO, SYS_FAOL, SYS_GETMSG, SYS_TRNLOG; GETDVI, GETJPI, GETSYI; GET_EF, FREE_EF, RESERVE_EF, GET_LUN, FREE_LUN; INIT_TIMER, SHOW_TIMER, STAT_TIMER, FREE_TIMER; AST_IN_PROG, ASN_WTH_MBX, WAIT, GET_USERS_LANGUAGE; FORMAT_SOGW_PROT, PARSE_SOGW_PROT, PARSE_ACCESS_CODE (7.x) |
| 56 | Date and time | ADD_TIMES, SUB_TIMES, MULT_DELTA_TIME, MULTF_DELTA_TIME; CVT_TO_INTERNAL_TIME, CVTF_TO_INTERNAL_TIME, CVT_FROM_INTERNAL_TIME, CVTF_FROM_INTERNAL_TIME, CVT_VECTIM; DATE_TIME, DAY, DAY_OF_WEEK; CONVERT_DATE_STRING, FORMAT_DATE_TIME, INIT_DATE_TIME_CONTEXT, FREE_DATE_TIME_CONTEXT, GET_DATE_FORMAT, GET_MAXIMUM_DATE_LENGTH |
| 57 | The command language interpreter | GET_SYMBOL, SET_SYMBOL, DELETE_SYMBOL; SET_LOGICAL, DELETE_LOGICAL; GET_COMMAND; DO_COMMAND, RUN_PROGRAM; GET_COMMON, PUT_COMMON; DISABLE_CTRL, ENABLE_CTRL; ATTACH, PAUSE |
| 58 | Calling back into the program; files | the callback mechanism; FIND_FILE, FIND_FILE_END, FILE_SCAN, FILE_SCAN_END, DELETE_FILE, RENAME_FILE, FID_TO_NAME, TRIM_FILESPEC |
| 59 | Virtual memory zones | CREATE_VM_ZONE, CREATE_USER_VM_ZONE, RESET_VM_ZONE, GET_VM_PAGE, FREE_VM_PAGE, SHOW_VM, STAT_VM; GET_VM, FREE_VM, DELETE_VM_ZONE redone with zones; VM_MALLOC, VM_FREE, VM_CALLOC, VM_REALLOC, VM_MALLOC_P, VM_FREE_P, VM_CALLOC_P, VM_REALLOC_P, VM_MALLOC_INIT_P (7.x); FIND_VM_ZONE, SHOW_VM_ZONE, VERIFY_VM_ZONE (LIBRTL2) |
| 60 | Trees and table-driven parsing | INSERT_TREE, LOOKUP_TREE, TRAVERSE_TREE; TPARSE, TABLE_PARSE (7.x), and the `$INIT_STATE`/`$STATE`/`$TRAN`/`$END_STATE` macros |
| 61 | Conditions, faults, and images | SIG_TO_STOP; FIXUP_FLT, FLT_UNDER, INT_OVER, DEC_OVER; DECODE_FAULT, SIM_TRAP (LIBRTL2); GET_OPCODE (7.x); FIND_IMAGE_SYMBOL; LIB$INITIALIZE (the manual's chapter 7); SIGNAL, STOP, ESTABLISH, REVERT, SIG_TO_RET, MATCH_COND reviewed against the manual |
| not planned | clusters, networks, other subsystems | BUILD_NODESPEC, COMPARE_NODENAME, COMPRESS_NODENAME, EXPAND_NODENAME, FIT_NODENAME, PARSE_NCB, GET_HOSTNAME, GET_FULLNAME_OFFSET, TRIM_FULLNAME (DECnet node and full names); ASCII_TO_UID, UID_TO_ASCII, COMPARE_UID (DCE UIDs); GETQUI (no queue manager); GET_ACCNAM, GET_ACCNAM_BY_CONTEXT (security object classes); CRF_INS_KEY, CRF_INS_REF, CRF_OUTPUT (CRFSHR.EXE's cross-reference routines); `LIB$$GET_VM_POINTERS` (internal) |

The routines the 5.0 manual groups as "access to VAX machine
instructions" (its section 2.3) are Phase 53. Its "system service
access" (2.1), resource allocation (2.4), and performance measurement
(2.5) are Phase 55, and its CLI access (2.2) is Phase 57. Its chapters 3
(date and time), 5 and 6 (memory), and 7 (image initialization) are
Phases 56, 59, and 61.

## The ranking, and why

1. **Phase 53, the instruction routines**, comes first. These routines
   are self-contained: arguments in, results out, no new process state.
   Their semantics are the instructions', which govax's CPU and
   `vaxfloat` already implement and VMS's run has checked. So this group
   is the cheapest place to build and prove the groundwork (Part B)
   every later phase leans on. They're also what compilers' generated
   code calls (FORTRAN's and BASIC's bit, queue, and string intrinsics).
2. **Phase 54, strings and conversion.** Dynamic strings (SGET1_DD,
   SFREE1_DD, SCOPY_*) are what every string-returning routine and
   every higher-level language's string handling sit on. CVT_DTB and its
   kind, INDEX, and LOOKUP_KEY are common in MACRO programs. This phase
   also brings JSB entry points and data symbols (`LIB$AB_UPCASE`) into
   the transfer vector.
3. **Phase 55, service jackets and resources.** LIB$SYS_FAO and
   LIB$GETJPI are among the most called LIB$ routines. They're thin
   layers over services govax already has ($FAO, $GETJPI, $GETDVI,
   $GETSYI, $GETMSG, $ASCTIM, $TRNLOG). The event flag and LUN allocators
   and the timer routines are small and widely used.
4. **Phase 56, date and time.** LIB$DATE_TIME and the internal-time
   arithmetic are common and need nothing new. The formatting routines
   (the manual's chapter 3) are a self-contained body of work with its
   own logical names and format mnemonics.
5. **Phase 57, the CLI.** Symbols and logical names from a program, and
   DO_COMMAND/RUN_PROGRAM (chaining: the image exits and DCL runs the
   command). These touch `internal/dclshell`, process 1's DCL and a
   subprocess's, so they come once the simpler groups have settled the
   conventions.
6. **Phase 58, callbacks and files.** FILE_SCAN, DELETE_FILE, and
   RENAME_FILE call the program's own action routines, so this phase
   builds the general mechanism by which a Go-implemented routine calls
   VAX code and continues when it returns. FIND_FILE is RMS's $SEARCH
   with a context, and RMS already has it.
7. **Phase 59, memory zones.** govax's heap is a port of eVAX's: GET_VM
   ignores its zone, and FREE_VM returns a status VMS doesn't
   document (4042). Zones, their attributes and algorithms, and the
   statistics routines are a coherent redesign. CREATE_USER_VM_ZONE needs
   Phase 58's callbacks.
8. **Phase 60, trees and TPARSE.** Both are callback-heavy. TPARSE is a
   state-table interpreter with its own STARLET macros, which govax's
   clean-room `starlet.mar` will need, and it has the manual's longest
   entry (LIB-411 to LIB-452).
9. **Phase 61, conditions and images.** The rest of condition handling
   (SIG_TO_STOP, the enable/disable routines, FIXUP_FLT, DECODE_FAULT,
   SIM_TRAP), dynamic image activation (FIND_IMAGE_SYMBOL, which needs
   the image loader to activate a shareable image at run time), and
   LIB$INITIALIZE per the manual. Last because it touches the most
   machinery (the condition dispatcher, the image loader) for routines
   that ordinary programs call least.

**Not planned.** Routines for clusters and networks (node names, NCBs,
host name, full names), DCE UIDs, the queue manager (GETQUI), security
object classes (GET_ACCNAM), and the cross-reference shareable image
have nothing in govax to stand on. govax could still give them stubs that
fail politely (Part B, "A routine govax lacks"), so a program that names
one but doesn't reach it can run. GET_HOSTNAME is the one cheap
exception: it could return the node name govax already reports (SYI$_NODENAME).
It's listed here because the 5.0 manual doesn't describe it, and it can
move into Phase 55 if its interface is settled.

## Working rule for every phase of the program

Unless the author says otherwise, each subtask is finished and testable
before it's committed: its code, its tests passing (`go build`,
`go vet`, `go test ./...`), and its progress-log entry. After each
subtask's commit, work stops so the author can review and push. The
next subtask starts only when the author asks for it (the author,
2026-10-12).

## Phase sizing

Each phase is sized like the RMS and multiprocessing phases (24, 33, 47,
49): one subject, about ten to fifteen subtasks, each a commit. A phase
should be small enough that its survey, decisions, and progress log can
be held in mind while any one subtask is worked on. The groups run from
about 15 routines (57, 58, 60) to about 35 (53, 54), since the
instruction and string routines are each small and alike. Each later
phase's doc has its routines, what it builds on, its open questions, and
provisional subtasks. Each phase starts with a survey subtask that
transcribes its routines' interfaces from the manual, so the planning
docs don't need to carry that detail now.

# Part B — the groundwork

## What earlier phases leave in place

- `internal/librtl` (Phase 34): `Routines`, a table of name, transfer-vector
  offset, XFC$SHIM code, and Go function. `Register` installs them into a
  process's `corevms.ShimTable`, and `Links` gives the machine their stubs.
  `TestShimOffsetsMatchLIBRTL` checks the offsets against
  `vmsdef.ImageSymbols`. Helpers so far: `arg` (an omitted argument is 0),
  `storeString` (`foreign.go`: fixed or dynamic output), and
  `corevms.Environment.StringDescriptor` (input).
- `internal/machine/shim.go`: `EnsureShims` writes a 12-byte stub (entry
  mask; `MOVL #code,R0`; `XFC #XFC$SHIM`; `RET`) per routine into
  VMINIT's 512-byte shim page, with one overflow page from the S0 pool. That
  is room for 84 stubs, and about 43 are in use (LIBRTL's and CRTL's). Rows
  with code 0 (`kernelLinks`) are routines in native VAX code in
  `kernel.asm`, which need no stub.
- `corevms`: `CallRequest` (`exit.go`): a service can return one to have
  the engine call a VAX routine with the service's own XFC as the return
  address. `$CMKRNL` (`cmode.go`), exit handlers, and the condition
  dispatcher use it. Shims don't yet. Phase 58 generalizes it.
- `vmsdef.Messages` has 117 `LIB$_` condition values and their texts.
- `internal/vaxfloat`: Unpack/Pack and exact arithmetic for F, D, G, and H,
  with EMOD's and POLY's arithmetic, so the floating routines don't need
  the CPU.

## Decisions

The author accepted all of these on 2026-10-12, including the proposed
answers to the two marked **(author)**.

1. **Go shims by default.** Each routine is a Go function behind an XFC
   stub, as now. The manual's instruction routines return condition
   values (LIB$EDIV returns SS$_INTOVF and SS$_INTDIV; it doesn't trap).
   So a bare instruction in VAX code would behave differently from the
   manual, and the Go version is easier to test. The exception is a
   routine whose whole job is to transfer control: **LIB$CALLG** is VAX
   code (`.ENTRY`, `CALLG @4(AP),@8(AP)`, `RET`), so the called routine's
   frame, handler, and R0/R1 are exactly a CALLG's. Native rows are
   code-0 rows, as `kernelLinks` are. They're in a section of
   `kernel.asm` of their own, "LIBRTL routines written in VAX code"
   (subtask 2), each with a `.SHIM` line in its native table and a row
   in `librtl.Routines` with code 0 and no Go function.
2. **Room for every stub.** The stub area grows by S0 pages as the table
   needs them, rather than one overflow page. VMINIT's page stays first,
   so no existing stub moves. A routine's offset comes from
   `vmsdef.ImageSymbols` by name instead of being written in the table
   (one fewer thing to get wrong; the test then checks only that each
   name exists). XFC$SHIM codes are allocated in order, as now.
3. **Descriptors, one helper for each direction** (`descriptor.go`). An
   input string is read from a descriptor of any class the manual says
   input accepts: S, D, VS, NCA, Z, and SD/A where it applies. An output
   string is written by its descriptor's class: S truncates or pads with
   blanks; VS sets the current length within its maximum; D reallocates
   from the heap. The routine returns LIB$_STRTRU when the text didn't
   fit (the manual's usual "string truncated" success status). An
   unsupported class gives LIB$_INVSTRDES. `storeString` and
   `StringDescriptor`'s callers move onto these helpers. Phase 54's
   SGET1_DD and SFREE1_DD later become the dynamic-string allocator these
   helpers call.
4. **Arguments.** The `arg` rule stays: a missing optional argument and a
   0 address are the same. A required argument that's missing gives
   LIB$_WRONUMARG where the manual lists it (it does for many routines),
   and the shim reads the argument count to tell.
5. **An access violation inside a routine (author).** Today a shim that
   can't read or write an argument returns SS$_ACCVIO in R0. On VMS the
   routine's own instruction faults, and SS$_ACCVIO is **signaled** (a
   condition the program's handlers see, and that the catch-all reports
   and exits on), not returned. Proposed: a helper the routines share
   that signals SS$_ACCVIO with the faulting address, as the hardware
   exception would, through the dispatcher LIB$SIGNAL uses. The existing
   routines change too, and the change is logged in DEVIATIONS.md.
   Alternatively, keep returning the status (simpler, but programs that
   rely on a handler see a difference). **Accepted: signal.**
6. **A routine govax lacks (author).** Today an image naming any
   unimplemented LIBRTL routine can't be activated (LIB_UNRESOLVED), even
   if it never calls it. Proposed: every LIBRTL transfer-vector entry
   gets a stub. One govax lacks signals a condition when called (proposed:
   SS$_UNSUPPORTED, with the routine's name in govax's message), so a
   program runs up to the call and its handlers can see the failure. This
   needs no stub per missing routine: they can share one code, and the
   XFC finds the name from the stub's address. A setting could restore
   the strict behavior for tests that want it. **Accepted: a stub for
   every entry, signaling when called.**
7. **Testing.** Each routine has Go unit tests on `librtl`'s fixture,
   cases taken from the manual's argument and condition-value
   descriptions. Each group also has a MACRO-32 program
   (`testdata/lib53/`) that calls every routine and prints the results
   with LIB$PUT_OUTPUT, run by a console test. The same program is the
   probe for VMS: run on the simh VAX, its log becomes an oracle, as
   `testdata/credir` and `testdata/insn35` did.

# Part C — the instruction routines

| Group | Routines (manual page) |
| --- | --- |
| Bit fields | EXTV (142), EXTZV (145), INSV (253), FFC and FFS (147, "LIB$FFx") |
| Integer | ADDX (7), SUBX (399), EMUL (136), EDIV (126); ADAWI (3) reviewed |
| Floating | EMODF (130), EMODD (128), EMODG (132), EMODH (134), POLYF (302), POLYD (300), POLYG (305), POLYH (307); CVTDF (7.x) |
| Queues and interlocks | INSQHI (248), INSQTI (251), REMQHI (315), REMQTI (317), BBSSI (21), BBCCI (19) |
| Character strings | LOCC (258), MATCHC (270), SCANC (334), SKPC (376), SPANC (378), MOVC3 (275), MOVC5 (276), MOVTC (278), MOVTUC (295) |
| Calls and CRC | CALLG (23), CRC (31), CRC_TABLE (33) |

Notes for the survey:

- **ADDX and SUBX** add and subtract multiprecision integers of a given
  length in longwords (default 2), reporting overflow as SS$_INTOVF.
- **EDIV** returns SS$_INTOVF or SS$_INTDIV, and in both cases stores the
  dividend's low longword as the quotient and 0 as the remainder (LIB-126).
- **The queue routines** take a retry count (the hardware's secondary
  interlock) and return LIB$_ONEENTQUE (the queue had one entry), LIB$_QUEWASEMP
  (it was empty), or LIB$_SECINTFAI (the interlock failed). With one CPU,
  govax's interlock never fails, but the statuses are still the manual's.
  REMQHI and REMQTI return the removed entry's address through an
  argument, which is why the RTL has them at all.
- **The string routines** take descriptors where the instructions take
  lengths and addresses, and return what the instruction leaves in R0
  (LOCC: the position, 1-based, or 0), as the manual describes each.
  MOVTC and MOVTUC take a translation table by descriptor.
- **ADAWI** (Phase 34's eVAX port) doesn't follow the manual's argument
  order or status (LIB$_INTOVF on overflow; the survey below corrects
  this note's first guess, SS$_INTOVF). The survey checks it, and it's
  fixed here.
- **CVTDF** is 7.x only. Without a description it's left out (Part A's
  rule) and listed.
- **FFC and FFS** are one manual entry, "LIB$FFx". The OCR's
  "LIB$CVT_)(TB" is CVT_xTB (Phase 54), and "LIB$CVT_ox_ox" is CVT_DX_DX.

## The survey (subtask 5)

Each routine's interface, from the 5.0 manual. Arguments are in the
VMS argument notation, `name.access-type.mechanism`: access `r`, `w`,
`m` (read, write, modify); types `l`, `w`, `b`, `q` (longword, word,
byte, quadword; `u` unsigned), `a` an address, `t` text, `x` the
routine's floating format, `z` unspecified; mechanism `r` by reference,
`v` by value, `dx` by descriptor, `ra` an array by reference. `[ ]`
marks an optional argument.
"Returns" is the routine's R0: a condition value (cond), or a value
(the manual's "RETURNS" names a result, not a status). Statuses listed
under the manual's "CONDITION VALUE SIGNALED" are signaled; the rest are
returned. Every condition value named here is in `vmsdef.Symbols` and
`vmsdef.Messages`. **None of Part C's entries lists LIB$_WRONUMARG**, so
a missing required argument reads as address 0 (`arg`) and is
`memoryFault`'s signaled SS$_ACCVIO; `tooFewArgs` waits for Phase 54.
Where the text is unclear, the choice is govax's, marked
**unconfirmed**; each such choice is a case in the probe (subtask 13).

### Bit fields

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| EXTV (142) | `position.rl.r, size.rbu.r, base-address.rlu.v` | the field, sign-extended | SS$_ROPRAND signaled (size > 32) |
| EXTZV (145) | `position.rl.r, size.rbu.r, base-address.rlu.v` | the field, zero-extended | SS$_ROPRAND signaled (size > 32) |
| INSV (253) | `longword-integer-source.rl.r, position.rl.r, size.rbu.r, base-address.rlu.v` | nothing | SS$_ROPRAND signaled (size > 32) |
| FFC, FFS (147) | `position.rl.r, size.rbu.r, base.ra.r, find-position.wl.r` | cond | SS$_NORMAL (found); LIB$_NOTFOU (none, or size 0); SS$_ROPRAND signaled (size > 32) |

- **The base.** EXTV, EXTZV, and INSV pass base-address "by value" and
  FFx passes base "by reference", described as "the address of an
  unsigned longword containing the base address". The examples settle
  EXTV and INSV: `LIB$EXTV(0, 4, VALUE)` (PL/I) and
  `CALL LIB$INSV(4, 0, 3, COND_VALUE)` (FORTRAN) pass the variable
  holding the field by reference, so the argument *is* the field's
  address. govax reads FFx's argument the same way (the field starts at
  the address in the argument list), not as a pointer to a pointer:
  **unconfirmed**.
- **Position** is any signed longword; a negative one reaches lower
  bytes (the instructions' memory-field rule, `internal/cpu/bitfield.go`'s
  `getMemoryField`/`setMemoryField`, plain functions of the CPU and
  memory the routines can share). Size 0: EXTV and EXTZV return 0, INSV
  stores nothing, FFx returns LIB$_NOTFOU.
- **FFx** writes find-position in both cases: the found bit's position
  (relative to the base, not to position), or position + size when none
  is found.
- **INSV** inserts bits 0 to size-1 of the source.

### Integers

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| ADDX (7) | `addend-array.rl.ra, augend-array.rl.ra, resultant-array.wl.ra [,array-length.rl.r]` | cond | SS$_NORMAL; SS$_INTOVF (result correct but for the sign bit) |
| SUBX (399) | `minuend-array.rl.ra, subtrahend-array.rl.ra, difference-array.wl.ra [,array-length.rl.r]` | cond | SS$_NORMAL; SS$_INTOVF; LIB$_INVARG (length negative; output unchanged) |
| EMUL (136) | `longword-integer-multiplier.rl.r, longword-integer-multiplicand.rl.r, addend.rl.r, product.wq.r` | cond | SS$_NORMAL |
| EDIV (126) | `longword-integer-divisor.rl.r, quadword-integer-dividend.rq.r, longword-integer-quotient.wl.r, remainder.wl.r` | cond | SS$_NORMAL; SS$_INTOVF, SS$_INTDIV (quotient = dividend<31:0>, remainder 0) |
| ADAWI (3) | `add.rw.r, sum.mw.r, result.mw.r` | cond | SS$_NORMAL; LIB$_INTOVF |

- **ADDX and SUBX**: little-endian arrays of longwords, the
  highest-addressed holding the sign; array-length defaults to 2 (a
  quadword). Any of the three arrays may be the same. The ADDX entry
  lists no LIB$_INVARG; govax gives a negative length LIB$_INVARG in both
  (**unconfirmed** for ADDX). A length of 0 does nothing and returns
  SS$_NORMAL (**unconfirmed**). The manual's examples are test cases:
  `7FFF407F 00000001` + `8000BF80 FFFFFFFF` = 0;
  `10000000 00000001` + `00000000 FFFFFFFF` = `10000001 00000000`;
  `10000000 00000001` − `00000000 FFFFFFFF` = `0FFFFFFF 00000002`.
- **EMUL**: product = multiplier × multiplicand + sign-extended addend;
  2²⁸ × 2¹² gives `00000100 00000000` (the manual's example).
- **EDIV**: 4,600,387,192 (`00000001 12345678`) ÷ 4096 is 1,123,141
  remainder 1656 (the manual's example).
- **ADAWI is wrong today.** The manual's operands are *words*: sum ←
  sum + add (the instruction, with its word-alignment rule: an odd sum
  address is the instruction's reserved operand fault, signaled as
  SS$_ROPRAND, **unconfirmed**), returning LIB$_INTOVF (not SS$_INTOVF,
  as Part C's note above guessed) on overflow, the truncated sum stored.
  govax's port reads two *longwords* and never stores the sum, only a
  sign. The third argument, "result" (word, access modify), is "the
  result of adding add and sum"; its *value* isn't defined further.
  govax will store the sum's sign there as a word (−1, 0, or 1), as the
  port does (a longword there), and as DIGITAL's later documentation is
  remembered to describe it, naming the argument `sign` (from memory:
  no copy is at hand): **unconfirmed**, and the probe prints it.

### Floating

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| EMODF (130), EMODD (128) | `floating-point-multiplier.rx.r, multiplier-extension.rbu.r, floating-point-multiplicand.rx.r, integer-portion.wl.r, fractional-portion.wx.r` | cond | SS$_NORMAL; SS$_INTOVF; SS$_FLTUND; SS$_ROPRAND |
| EMODG (132), EMODH (134) | the same, with `multiplier-extension.rwu.r` | cond | the same |
| POLYF (302), POLYD (300), POLYG (305), POLYH (307) | `polynomial-argument.rx.r, degree.rw.r, coefficient.rx.ra, floating-point-result.wx.r` | cond | SS$_NORMAL; SS$_FLTOVF; SS$_ROPRAND |

- **EMOD**: the extension adds x bits to the multiplier (F and D 8,
  bits 7:0 of a byte; G 11, bits 15:5 of a word; H 15, bits 15:1); the
  arithmetic is `vaxfloat.EMOD`, which the CPU's EMODx already uses.
  SS$_INTOVF: the integer gets the true integer part's low-order 32
  bits; a floating overflow is SS$_INTOVF too. SS$_FLTUND: integer and
  fraction both 0. SS$_ROPRAND: neither stored. All three are returned,
  not signaled.
- **POLY**: Horner's rule over degree + 1 coefficients, the
  highest-order term's first (lowest address); degree 0 to 31 (above,
  the instruction's reserved operand: SS$_ROPRAND, returned). The
  manual's formula `C[0]+X*(C[1]+...)` numbers them the other way; its
  example settles the order: X = 2, degree 4, coefficients 1, 2, −1, 1,
  −3 gives 27, and X = 1, degree 2, coefficients 1, 2, 2 gives 5. Each
  step is `vaxfloat.POLYStep`. Underflow isn't listed: govax stores 0
  and returns SS$_NORMAL, as the instruction does with `PSL<FU>` clear
  (**unconfirmed**).
- **CVTDF** (7.x only) has no description, so it stays a missing-routine
  stub (Part A's rule).

### Queues and interlocks

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| INSQHI (248), INSQTI (251) | `entry.mq.ra, header.mq.r [,retry-count.rlu.r]` | cond | SS$_NORMAL (more than one entry now); LIB$_ONEENTQUE (one entry now); LIB$_SECINTFAI (severe; queue unchanged) |
| REMQHI (315), REMQTI (317) | `header.mq.r, remque-address.wa.r [,retry-count.rlu.r]` | cond | SS$_NORMAL (entries remain); LIB$_ONEENTQUE (now empty); LIB$_QUEWASEMP (was empty; remque-address = the header's address); LIB$_SECINTFAI; SS$_ROPRAND |
| BBSSI (21), BBCCI (19) | `position.rl.r, bit-zero-address.mz.r` | the bit's old state, 0 or 1 | none |

- **Self-relative queues**: header and entry quadword aligned, each a
  forward and a backward displacement; a zeroed header is empty. The
  retry count defaults to 10 (1 means no retries).
- **The secondary interlock.** Bit 0 of the header is the interlock: if
  it's set, the instruction fails with C set and the queue unchanged,
  and the routine tries again, up to retry-count times, before returning
  LIB$_SECINTFAI. With one CPU and no other user of the bit nothing sets
  it, but a program can (a header built by hand, or a test), so govax
  models it rather than never failing.
- **SS$_ROPRAND** (an entry or header not quadword aligned, or the
  header the entry) is in REMQHI's and REMQTI's "returned" list but
  described as "a reserved operand fault"; govax signals it, as the
  instruction's fault would be (**unconfirmed**), and does the same for
  INSQHI and INSQTI, whose entries don't mention it.
- **internal/cpu's queue instructions** check neither the alignment nor
  the interlock bit (`queue.go`'s comments: no interlock logic). That's
  a fidelity gap in the CPU, the ARM's rules; subtask 9 fixes the CPU
  too and logs it, and the routines share the fixed code.
- **BBSSI/BBCCI**: position as for the bit fields (any signed longword
  from the byte at bit-zero-address); the manual's examples test bit 42
  and bit 104 of a 128-bit array.

### Character strings

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| LOCC (258) | `character-string.rt.dx, source-string.rt.dx` | position of the first equal character (1-based), or 0 | none |
| SKPC (376) | `character-string.rt.dx, source-string.rt.dx` | position of the first unequal character, or 0 | none |
| MATCHC (270) | `sub-string.rt.dx, source-string.rt.dx` | position of the substring's first character, or 0 | none |
| SCANC (334) | `source-string.rt.dx, table-array.rbu.ra, byte-integer-mask.rbu.r` | position of the first character whose table byte AND mask is nonzero, or 0 | none |
| SPANC (378) | `source-string.rt.dx, table-array.rbu.ra, byte-integer-mask.rbu.r` | position of the first character whose table byte AND mask is zero, or 0 | none |
| MOVC3 (275) | `word-integer-length.rwu.r, source.rz.r, destination.wz.r` | nothing | none |
| MOVC5 (276) | `word-integer-source-length.rwu.r, source.rz.r, fill.rb.r, word-integer-destination-length.rwu.r, destination.wz.r` | nothing | none |
| MOVTC (278) | `source-string.rt.dx, fill-character.rt.dx, translation-table.rt.dx, destination-string.wt.dx` | cond | SS$_NORMAL; LIB$_STRTRU; LIB$_FATERRLIB; LIB$_INSVIRMEM; LIB$_INVSTRDES |
| MOVTUC (295) | `source-string.rt.dx, stop-character.rt.dx, translation-table.rt.dx, destination-string.wt.dx [,fill-character.rt.dx]` | position of the source character translated to the stop character, 0 if none, −1 if the destination can't be allocated | none |

- **Positions** are 1-based indexes into the source; 0 means not found,
  and a zero-length source gives 0. MATCHC with an empty substring
  returns 1. LOCC's, SKPC's, and the fill and stop characters' strings
  give only their first byte, their length "not checked" (so a
  zero-length one still gives the byte at its address).
- **SCANC's description** says the scan ends when the AND "is equal to
  1"; the instruction (and the RETURNS text) end it on nonzero, which
  govax follows.
- **The manual's examples**: LOCC of "DAY" in "ONE DAY AT A TIME" is 5;
  SKPC of " " in " ABC" is 2, of "A" in "AAA" is 0; LOCC of "X" in the
  alphabet is 24.
- **MOVC3 and MOVC5** take lengths and addresses, not descriptors, and
  overlap doesn't matter. "RETURNS None": govax returns what the
  instruction leaves in R0 (MOVC3 0, MOVC5 the source bytes not moved),
  **unconfirmed**.
- **MOVTC's and MOVTUC's destination.** Their statuses (LIB$_STRTRU,
  LIB$_INSVIRMEM; MOVTUC's −1) say the destination is written as any
  output string is, so govax writes the translated text through
  `writeString`, padded with the fill character rather than blanks.
  The destination's length is its own for fixed classes; a D
  destination is given the source's length and VS takes up to MAXSTRLEN
  (**unconfirmed**). MOVTC's LIB$_STRTRU means the source was longer
  than the destination. MOVTUC without a fill character leaves the rest
  of the destination as it was. The table is 256 bytes from its
  descriptor's address, its length not checked. The DIGITAL tables
  (`LIB$AB_ASC_EBC` and the rest) are Phase 54's data symbols.

### Calls and CRC

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| CALLG (23) | `argument-list.rlu.ra, user-procedure.ra.v` | the called routine's R0 | none (done in subtask 2) |
| CRC (31) | `crc-table.rl.ra, initial-crc.rl.r, stream.rt.dx` | the CRC | none |
| CRC_TABLE (33) | `polynomial-coefficient.rlu.r, crc-table.wl.ra` | nothing | none |

- **CRC_TABLE** builds the ARM's 16-longword table: entry i is i shifted
  right four times, the polynomial XORed in after each shift that drops
  a 1. **CRC** runs the instruction's nibble-at-a-time algorithm (the
  CPU's `emulCrc` loop) over the stream's bytes.
- **The manual's example** is a test case: with CRC-16's polynomial
  (octal 120001, `A001`) and an initial CRC of 0, "DOVE" and "HOSE" both
  give 29,915 (`74DB`), checked with a reference implementation;
  "123456789" gives `BB3D`, CRC-16/ARC's check value.

### Sharing code with the CPU

The CPU's instruction handlers work on a decoded instruction's operands,
so the routines can't call them. Where the semantics live in plain
functions they're shared: `vaxfloat.EMOD` and `POLYStep`, the bit field
memory functions, and the queue code once subtask 9 makes it a function
of addresses. Each subtask moves the instruction's core into such a
function where that's small (exported from `internal/cpu` for `librtl`),
so there's one implementation of each rule.

## Subtasks

1. **Plan** (this document, Part A, and Phases 54–61's docs). Done
   2026-10-12, for review.
2. **The stub area and the table**: stubs on as many S0 pages as needed;
   offsets from `vmsdef.ImageSymbols`; the native-code home for code-0
   LIB$ routines (Decision 1), proved with LIB$CALLG; tests for addresses,
   offsets, and codes. **Done** 2026-10-12.
3. **Descriptors and arguments** (`descriptor.go`): reading every input
   class, writing S, VS, and D output, LIB$_STRTRU and LIB$_INVSTRDES,
   LIB$_WRONUMARG; the existing routines moved onto them. **Done**
   2026-10-12.
4. **Access violations and missing routines** (Decisions 5 and 6).
   **Done** 2026-10-12.
5. **Survey** of Part C's routines: each one's format, arguments,
   condition values, and examples, from the manual (OCR checked against
   the PDF where doubtful), recorded in this doc. **Done** 2026-10-12.
6. **Bit fields**: EXTV, EXTZV, INSV, FFC, FFS.
7. **Integers**: ADDX, SUBX, EMUL, EDIV; ADAWI fixed.
8. **Floating**: EMOD and POLY in F, D, G, and H, on `vaxfloat`.
9. **Queues and interlocks**: INSQHI, INSQTI, REMQHI, REMQTI, BBSSI, BBCCI.
10. **Character strings**: LOCC, MATCHC, SCANC, SKPC, SPANC, MOVC3, MOVC5,
    MOVTC, MOVTUC.
11. **CRC, CRC_TABLE** (CALLG was done in subtask 2).
12. **The program**: `testdata/lib53/lib53.mar`, calling every routine of
    the phase, with a console test of its output.
13. **VMS probe** (with the author on the simh VAX): `lib53.mar`'s run,
    becoming an oracle test. In the same session, ANALYZE/IMAGE of
    SYS$SHARE:LIBRTL2.EXE (and CRFSHR.EXE, for completeness), so Phases 54,
    59, and 61 have those offsets. The images' symbols go into
    `vmsdef` through `internal/vmsdef/gen -image`, and the licensed files
    are erased from the exchange container afterward.
14. **Close-out**: DEVIATIONS.md (choices unconfirmed), CLAUDE.md,
    PLAN.md, `internal/librtl/doc.go`, and HELP if a routine list is
    added there.

## Open questions

- ~~Decisions 5 and 6 (above) are the author's.~~ Accepted as proposed
  (2026-10-12).
- Is a list of the supported LIB$ routines wanted in HELP (a `HELP RTL`
  topic, say), or only in the docs?
- Which 7.x-only routines matter enough to probe for their interfaces
  (CVTDF, TABLE_PARSE, GET_OPCODE, PARSE_SOGW_PROT, FORMAT_SOGW_PROT,
  PARSE_ACCESS_CODE, the VM_MALLOC family)? A probe can show what a
  routine returns, but not its documented argument list. The VMS 7.3
  STARLET or LIBRTL documentation, if one turns up, would settle them.

## Progress log

- **2026-10-12** — Planned. Read the 5.0 manual's contents and chapter
  2, compared its routine list with VMS 7.3's LIBRTL.EXE transfer vector
  (`vmsdef.ImageSymbols`: 188 LIB$ names), and sorted every routine into
  Phases 53–61 or "not planned" (Part A). Looked at what the groundwork
  must change: the stub area's room (84, about 43 used), output strings
  (`storeString` handles only S and D), JSB entries and data symbols
  (Phase 54), and calling back into VAX code (Phase 58, from
  `CallRequest`). Wrote planning docs for Phases 54–61. Implementation
  waits for the author's review.
- **2026-10-12** — Plan reviewed; the author accepted Part B's decisions
  as proposed, and set the working rule (each subtask complete and
  tested before its commit, then a stop for review). Subtask 2:
  - **The table.** `librtl.Routine` no longer carries an offset:
    `Routine.Offset` looks the name up in `vmsdef.ImageSymbols` (and
    panics for a name LIBRTL.EXE doesn't have, which
    `TestRoutinesMatchLIBRTL` reports first). A row with code 0 has no Go
    function and isn't registered as a shim; the test checks that a row
    has Go code exactly when its code isn't 0.
  - **The stub area.** `machine.EnsureShims` fills VMINIT's page (42
    stubs) as before, then puts the rest in one block of contiguous S0
    pages, as many as they need (`shimOverflow(pages)`), in place of a
    single overflow page and an error past it. The stubs in VMINIT's page
    keep their addresses. `TestEnsureShims_overflowPages` (in place of
    `_overflowPage`) adds 400 stubs and checks that the SCB is untouched,
    every stub is whole at its own address, and LIB$ADAWI's is still
    VMINIT's first.
  - **The native-code home** is a new section of `kernel.asm`, "LIBRTL
    routines written in VAX code", before the XFC shim table, with a
    `.SHIM` line in the native table. LIB$CALLG is its first routine:
    `.ENTRY LIB$CALLG ^M<>`, `CALLG @B^4(AP),@B^8(AP)`, `RET`. The
    manual (LIB-23) passes user-procedure by value, so the operand is
    deferred once. The empty entry mask leaves the called routine's R0
    and R1 as they were. `TestLibCallg` calls it at
    `SHIM$LIBRTL_00000810` with a two-argument list and a routine that
    returns their sum in R0 and 7 in R1. The 8 bytes moved every later
    kernel.asm address, so `internal/asm`'s golden snapshots were
    rewritten (`-update`): only addresses changed.
- **2026-10-12** — Subtask 3, descriptors and arguments
  (`internal/librtl/descriptor.go`):
  - **Reading.** `readDescriptor` decodes a string descriptor into a
    `stringDesc` (class, length, room, text address); `readString` reads
    the text. Z, S, D, and SD are LENGTH bytes at POINTER; A and NCA are
    DSC$L_ARSIZE bytes (65,536 or more is LIB$_INVSTRDES); VS is CURLEN
    bytes after the length word, at most MAXSTRLEN. The rules are the
    manual's LIB$ANALYZE_SDESC (LIB-10). Any other class is
    `errInvStrDes`.
  - **Writing.** `writeString` follows LIB$SCOPY_DXDX's table (LIB-337):
    Z, S, SD, A, and NCA are filled, padded or truncated; D keeps its
    storage when the string fits and otherwise frees it and allocates
    anew, setting the length (and address); VS copies up to MAXSTRLEN,
    unpadded, and sets CURLEN. It reports what it stored and whether it
    truncated, and each routine picks its status (LIB$_STRTRU, or
    LIB$_INPSTRTRU for the input routines). `stringStatus` turns a
    helper's error into LIB$_INVSTRDES, LIB$_INSVIRMEM, or SS$_ACCVIO
    (subtask 4 makes the last a signal).
  - **Arguments.** `tooFewArgs(argv, n)` is the LIB$_WRONUMARG test for
    routines whose manual entry lists it. None of the routines so far
    does (LIB$SPAWN's is for too many), so its first users are subtask
    6's.
  - **The routines moved.** LIB$PUT_OUTPUT, LIB$GET_FOREIGN,
    LIB$GET_INPUT, LIB$CREATE_DIR, and LIB$SPAWN read through
    `readString` and write through `writeString`; `storeString` and
    `corevms.Environment.StringDescriptor` are gone. Each returns the
    manual's status for a bad class: LIB$_INVSTRDES, except LIB$GET_INPUT,
    whose entry (LIB-213) says LIB$_INVARG for a class "not a recognized
    code or ... zero", so it refuses class Z output too, before reading a
    line (unconfirmed against VMS). LIB$SPAWN's SS$_IVLOGNAM for a string
    over 65,535 bytes went: no descriptor can describe one. A prompt
    longer than 255 is now cut to 255 rather than dropped. STR$UPCASE
    (eVAX's in-place form, not the manual's two-argument one) is left for
    the STR$ work.
  - **Tests** (`descriptor_test.go`): every class read, S/Z/SD/A padded
    and truncated, VS's CURLEN, D's storage kept and replaced, the error
    statuses, and the routines taking a VS string and refusing class V.
- **2026-10-12** — Subtask 4, access violations and missing routines
  (Decisions 5 and 6):
  - **Signaling a routine's failure.** `corevms.Environment.SignalFailure`
    is LIB$SIGNAL's dispatch with the mechanism array's R0 set to a
    status, so a handler that continues the condition makes the routine
    return it (LIB$SIGNAL's own R0 there stays 0).
  - **Access violations** (`internal/librtl/fault.go`). `memoryFault`
    takes the error from a routine's own load or store: a translation
    fault (or a physical address error) is signaled as SS$_ACCVIO with
    the hardware's signal array, the reason mask and the address; any
    other error goes back to the XFC as before. The memory errors don't
    say whether a read or a write failed, so a store's error is marked
    with `writing`, which sets the reason mask's write bit (2); a
    length violation sets bit 0, a protection violation neither. (The
    CPU's own ACCVIO uses 1 and 2 for those, which isn't the
    architecture's mask; left alone.) `stringFailure` replaces
    `stringStatus`: LIB$_INVSTRDES and LIB$_INSVIRMEM stay statuses, a
    memory error is `memoryFault`'s, and `writeString` marks its stores.
    Every routine moved: LIB$ADAWI, STR$UPCASE, LIB$GET_VM, LIB$FREE_VM,
    LIB$DELETE_VM_ZONE, LIB$PUT_OUTPUT, LIB$GET_FOREIGN, LIB$GET_INPUT,
    LIB$SPAWN. LIB$CREATE_DIR's own `accessViolation` (reason 0, at the
    addresses its probe showed) became the shared one, unchanged in
    effect but for continuing (it now returns SS$_ACCVIO). If a handler
    continues, the routine returns SS$_ACCVIO; on VMS the faulting
    instruction would run again (DEVIATIONS.md).
  - **Missing routines** (`missing.go`). `Missing` lists every entry of
    LIBRTL.EXE's transfer vector (`vmsdef.ImageSymbols`) that `Routines`
    doesn't provide: 229 of them, in offset order, coded from 1000. Left
    out are the JSB entries (names ending in a register, `_R2` or
    `CHARR8`, or the string routines' `DX6`/`DD6`), whose callers JSB
    to them, so a CALL stub's entry mask would run as a HALT, and the
    `LIB$AB_` data tables. Both are Phase 54's. Aliases (FOR$CNV_IN_I is
    OTS$CVT_TI_L) share one stub, named LIB$ before STR$, OTS$, CVT$,
    FOR$, COB$, then alphabetically. `Register` and `Links` include
    them, so `machine.EnsureShims` writes their stubs (the S0 overflow
    from subtask 2 takes them). Each stub's code has its own shim, a
    closure over the name, rather than one shared code that finds the
    name from the stub's address as Decision 6 suggested: simpler, and
    the codes cost nothing. The shim puts the name, with a class S
    descriptor, on the stack in the stub's frame (`stackString`) and
    signals SS$_UNSUPPORTED followed by GOVAX-E-NOTIMPL, 1, the
    descriptor. GOVAX-E-NOTIMPL ("routine !AS is not implemented in
    govax") is govax's own message, in facility 0xFFF, the last
    customer facility (`vmsdef.GovaxNotImplemented`, found by
    `LookupMessage` after the system's). With no handler the catch-all
    shows

        %SYSTEM-E-UNSUPPORTED, unsupported operation or function
        -GOVAX-E-NOTIMPL, routine LIB$INDEX is not implemented in govax

    and, the condition being an error, the program goes on, the call
    returning SS$_UNSUPPORTED. No setting restores the strict
    LIB_UNRESOLVED behavior: no test wanted it.
  - **Tests.** `TestMissing` (every non-JSB, non-data entry has exactly
    one stub; aliases, codes distinct from every shim's),
    `TestMissingRegistered`; the console's `TestLibrtlMissingRoutine`
    (MACRO, LINK, and RUN of a call to LIB$INDEX), and
    `TestLibrtlAccessViolation` (LIB$ADAWI reading beyond P0, mask 01,
    and writing the read-only code psect, mask 04, each ending the
    image with SS$_ACCVIO) and `TestLibrtlAccessViolationContinued` (a
    handler continues; the call returns SS$_ACCVIO). `TestSharedName`'s
    offset 0x940 is now a stub's (OTS$CNVOUT); a JSB entry took its
    place as the case with none. The fixture's LIB$PUT_OUTPUT test now
    expects a signal.
- **2026-10-12** — Subtask 5, the survey ("The survey" above): read
  every Part C entry in the 5.0 manual's OCR text (EXTV through CRC,
  LIB-3 to LIB-399) and recorded each routine's format, result, and
  condition values, with the manual's examples as test cases (the
  arithmetic ones, CRC's "DOVE"/"HOSE" pair, checked by a reference
  computation). Findings that change the plan: LIB$ADAWI works on words
  and returns LIB$_INTOVF, and govax's port neither adds into the sum
  nor uses words (fixed in subtask 7); no Part C routine lists
  LIB$_WRONUMARG; the CPU's queue instructions lack the alignment check
  and the secondary interlock (fixed with subtask 9); MOVTC and MOVTUC
  write their destinations as output strings, filled with their fill
  character. The choices the manual leaves open are marked
  unconfirmed, each a case for the probe. No code changed.
