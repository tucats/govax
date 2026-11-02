# Phase 83 — FORTRAN and VMS: definitions, services, handlers, ASTs

**Status:** in progress. Subtask 1, the survey (Part A below), done 2026-11-01, and reviewed
by the author the same day ("The author's answers"); subtask 2, the probe, the same day ("What the probe
settled"); subtask 3, text libraries and INCLUDE, the same day; subtask 4, govax's definition modules, the same
day; subtask 5, calling services and RTL routines, the same day; subtask 6, condition handlers, the same day; subtask 7, ASTs and exit handlers, the same day; subtask 8, the system subroutines, the same day. Needs Phase 81 (files) and Phase 82 (structures). Part
of the FORTRAN program laid out in `PHASE-74 - FORTRAN plan.md` (Decisions 11, 17, and 21).

## Goal

Everything a FORTRAN program needs to use VMS as VMS programmers did:

- **INCLUDE '(module)'** from text libraries: `/LIBRARY` on the command
  (`FORTRAN PROG+MYLIB/LIBRARY` and the qualifier), FORT$LIBRARY,
  `SYS$LIBRARY:FORSYSDEF.TLB` when present, then govax's own modules
  generated from `vmsdef` (Decision 17): `$SSDEF`, `$IODEF`, `$JPIDEF`,
  `$SYIDEF`, `$DVIDEF`, `$RMSDEF`, `$LIBDEF`, `$FORDEF`, `$MTHDEF`,
  `$DSCDEF`, `$STSDEF`, `$LNMDEF`, `$SECDEF`, `$PSLDEF`, `$PRVDEF`,
  `$CHFDEF`, and `$SYSSRVNAM` (each service declared as an INTEGER*4
  function), as many others as programs need;
- **LIBRARY/TEXT** (create and insert text modules), if govax's LIBRARY
  lacks it, so users can build their own;
- **calling system services and RTL routines:** status functions
  (`ISTAT = SYS$GETJPIW(...)`), `IF (.NOT. ISTAT) CALL LIB$SIGNAL(%VAL(ISTAT))`,
  item lists from structures, descriptors for character arguments,
  quadword times, `%VAL` for flags, and null arguments;
- **condition handlers** written in FORTRAN: LIB$ESTABLISH and
  LIB$REVERT, a handler function's signal and mechanism arrays,
  SS$_RESIGNAL, SS$_CONTINUE, LIB$SIGNAL and LIB$STOP from FORTRAN,
  `$UNWIND`, LIB$MATCH_COND; and the condition values of FTNRTL's
  errors reaching them;
- **AST routines and exit handlers** in FORTRAN (`$SETIMR` with an AST,
  `$QIO` completion ASTs, `$DCLEXH`);
- **the system subroutines** of appendix D.4: DATE, IDATE, ERRSNS
  (Phase 80), EXIT, SECNDS, TIME, RAN, and MVBITS (D.5.3);
- **the LIB$ routines this needs** that govax lacks, Phase 61's among
  them (LIB$SIG_TO_RET, LIB$FIXUP_FLT, LIB$FLT_UNDER, LIB$INT_OVER), and
  any STR$ routine the example programs call.

## What earlier phases leave in place

- **Phase 79:** the calling standard, %VAL/%REF/%DESCR/%LOC, null
  arguments, procedure arguments.
- **Phase 82:** structures for item lists.
- **`vmsdef`**: every symbolic constant, and the system services'
  entries for LINK.
- **`internal/lbr`**: text libraries.
- **LIBRTL's condition routines** and Phase 70's condition search.

## Design notes

- **govax's FORSYSDEF modules** are generated in Go at compile time from
  `vmsdef.Symbols` by prefix, formatted as FORTRAN source (PARAMETER
  statements with INTEGER*4 declarations). No licensed text is copied:
  the values are those `vmsdef` already holds. A command (or test)
  writes them out as a `.TLB`, so users can see them.
- **LIB$ESTABLISH** works for FORTRAN as for MACRO: it stores the
  handler's address at 0(FP) of the caller's frame, which a FORTRAN
  routine's CALLS frame has.
- **AST reentrancy:** an AST routine in FORTRAN uses static storage; if
  it calls a routine the interrupted code was in, they share variables.
  That is VAX FORTRAN's behavior too; AUTOMATIC (Phase 82) is the
  remedy, and the manual's chapter says so.

## Open questions

- **Which definition modules** to generate first: the list above, then
  whatever the corpus programs INCLUDE.
- **STRUCTURE definitions** in VMS's FORSYSDEF (some modules declare
  structures as well as constants): generated where `vmsdef` has the
  layout, otherwise left to the program.

# Part A — the survey (2026-11-01)

Sources: the 1988 *VAX FORTRAN Language Reference Manual* ("the LRM"; 1.4.2, 5.1, 6, D.4, D.5),
the VMS 5.0 *LIB$* manual (OCR), *STR$* manual, and *System Services* manual, VSI's LIB$ and
system services texts, and the HP Fortran manual for what the LRM leaves out. The *VAX FORTRAN
User Manual*, which documents `/LIBRARY`, `FORT$LIBRARY`, and FORSYSDEF, isn't in the folder, so
what rests on it is marked unconfirmed. Everything under "What works" and "What is wrong" was
compiled and run on govax on 2026-11-01 with throwaway programs, not committed; subtask 4 and on
make such programs into `testdata/fortran`'s.

## What works today

Calling VMS from FORTRAN needs little from the compiler that Phases 79 and 82 didn't give it.
Each of these ran and gave the right answer:

- **Services and RTL routines, with every argument spelled out:** `$ASCTIM`, `$GETSYIW` and
  `$GETJPIW` with item lists from structures (the first real use of `itemlist.for`'s layout, which
  Phase 82 left to this phase), `$TRNLNM`, `$FAO` with `%VAL` and character arguments, `$QIO`,
  `$ASSIGN`, `$BINTIM`, LIB$DATE_TIME, LIB$GETJPI, LIB$SPAWN. `IF (STATUS)` and `.NOT. STATUS` on
  an INTEGER test bit 0, which is the status idiom's whole need.
- **`EXTERNAL` for a constant:** `EXTERNAL SYI$_VERSION`, then `%LOC(SYI$_VERSION)`, the 5.0
  `$GETSYI` example's way, for names in STARLET's tables (`SS$`, `FOR$`, `LIB$`, `JPI$`, `STS$`,
  `MTH$`, ...). `PSL$C_USER` isn't there: LINK leaves it undefined. Whether VMS's STARLET defines it
  globally is unconfirmed.
- **Condition handlers:** a FORTRAN function as handler through LIB$ESTABLISH and LIB$REVERT (both
  return the old handler), its signal and mechanism arrays, SS$_CONTINUE, SS$_RESIGNAL, `$UNWIND`
  with `MECH(3)`, LIB$MATCH_COND, LIB$SIGNAL, LIB$STOP, LIB$SIG_TO_RET as a handler; FTNRTL's errors reaching the handler with FOR's
  condition values; an access violation and integer divide by zero; the 5.0 manual's LIB$MATCH_COND
  example, with its output. `$PUTMSG` calling a FORTRAN action routine.
- **Unwinding out of an I/O statement in progress:** 2,000 times from a function in a PRINT list,
  and the next statements are right (FTNRTL's statement contexts record their frames; Phase 80).
- **ASTs and exit handlers:** `$SETIMR` and `$QIO` ASTs with their parameters, `$HIBER` and `$WAKE`,
  `$DCLEXH` (with the exit status it reports, including a FORTRAN error's, and with
  `CALL SYS$EXIT`). A stress run: 850 timer ASTs, each doing an internal WRITE and a PRINT and setting
  the next timer, during 40,000 formatted WRITEs and 40,000 READs on a file: no record wrong, no
  AST line malformed.
- **Volume of definitions:** a file of 3,000 typed PARAMETERs compiles in 0.02 seconds. Names with
  `$` and of 31 characters work.

## What is wrong or missing

| # | Finding | Where |
|---|---|---|
| 1 | **A null argument in a function reference is SYNTAX.** `STATUS = SYS$GETSYIW(,,,L,IOSB,,)`, the 5.0 manual's own form and how every status-function call of a service is written, doesn't compile; `CALL` allows it. This is the biggest gap: today `%VAL(0)` has to stand in. | `parse/expr.go`'s `subscripts`, with the rule in its comment |
| 2 | **`INCLUDE '(module)'` and `INCLUDE 'lib(module)'` are NOTIMPL.** | `source/include.go` |
| 3 | **`file+file` and `+LIB/LIBRARY` don't exist**: `T1+T1` is taken as one name. Phase 74 left them to this phase. LINK's positional qualifiers (`items[i].Present("LIBRARY")`, `console/dispatch.go`) are the model. | `console.dcl`, `dclcmd/fortran.go`, `govax fortran` |
| 4 | **LIBRARY has no `/TEXT`.** `internal/lbr` has the type and its defaults, unchecked against a real text library, and no function that makes a module from a file. `/EXTRACT` already knows the type's records. | `dclcmd/library.go`, `lbr/input.go` |
| 5 | **The system subroutines but ERRSNS are NOTIMPL.** | `sema/intrinsics.go`'s `builtin` |
| 6 | **`STR$UPCASE` is wrong**: it takes one argument and changes it in place, where the manual's form is `destination-string, source-string`. `STR$UPCASE(A,'abc')` leaves A as it was, in capitals. | `librtl/strings.go` |
| 7 | **A service govax lacks is a reserved operand fault** at the vector's address, the service's entry falling through to run the vector (`Environment.SystemService` returns "not mine"). 191 of the 312 vector entries are unregistered, among them `$GETCHN`, `$GETDEV`, `$SETDDIR`, `$GETQUI`, `$SNDJBC`, `$GETUTC`, and RMS's `$EXTEND`, `$SPACE`, `$ENTER`, `$REMOVE`. A missing LIB$ routine has said what it is since Phase 53. | `corevms/environment.go` |
| 8 | **An EXTERNAL name is always a `.EXTRN`**, used or not. That must stay for a program's own statements (`EXTERNAL BLKDAT` is how BLOCK DATA gets linked), but `$SYSSRVNAM` would put 200-odd in every module that includes it. | `codegen` |
| 9 | **Every INCLUDEd line is copied into the `.MAR`**: `$SSDEF` would add about 1,460 comment lines per unit that includes it, and `$IODEF` over 1,000. The LRM says INCLUDE's default is `/NOLIST`, and `trimListQualifier` throws the qualifier away. See Decision 4 and question 1. | `source/include.go`, `codegen/source.go` |
| 10 | **Status texts for STR$ are missing.** `STR$_` codes are in `vmsdef.LibrarySymbols`, but `vmsdef.Messages` has no STR facility, so a signaled one is NOMSG. | `vmsdef` |

## Definition modules

`vmsdef` already holds the values: 8,916 names, in `Symbols` and `LibrarySymbols` together. Five
names differ between the two, all `LAST` end markers (`JPI$_LASTPHD`, ...), and `Symbols`, the 7.3
definitions, wins. As a prototype, 38 modules were generated by prefix as `INTEGER*4` and
`PARAMETER (NAME = 'hex'X)` and compiled: all but `$TTDEF`, where `TT$_Pro_Series` and
`TT$_PRO_SERIES` collide, so the generator must drop duplicates by case. One name passes 31
characters (`ATR$S_ATR_FILESYSTEM_INFO_FIELDS`) and is left out.

| Modules | State |
|---|---|
| The plan's list: `$SSDEF` 731 names, `$IODEF` 547, `$JPIDEF` 222, `$SYIDEF` 445, `$DVIDEF` 187, `$RMSDEF` 268, `$LIBDEF` 117, `$FORDEF` 172, `$MTHDEF` 32, `$DSCDEF` 138, `$STSDEF` 32, `$LNMDEF` 40, `$SECDEF` 67, `$PSLDEF` 39, `$PRVDEF` 43, `$CHFDEF` 21 | ready by prefix |
| Others ready by prefix: `$LIBDCFDEF` (`LIB$K_DCF`), `$OTSDEF`, `$STRDEF`, `$CLIDEF`, `$FABDEF`, `$RABDEF`, `$NAMDEF`, `$XABDEF`, `$TTDEF`, `$TT2DEF`, `$DCDEF`, `$DEVDEF`, `$PRCDEF`, `$PQLDEF`, `$LCKDEF`, `$LKIDEF`, `$ACCDEF`, `$PRTDEF`, `$MSGDEF`, `$ATRDEF`, `$FIBDEF`, `$SMGDEF` | ready by prefix |
| `$SYSSRVNAM` | from `vmsdef.P1VectorTable`, without data cells, spares, and internal `_2`-style entries |
| `$FORIOSDEF` (`FOR$IOS_name`, the IOSTAT numbers) | not in `vmsdef`; can be made from the FOR message table (number and identifier). Its names are unconfirmed, there being no FORSYSDEF to read |
| `$OPCDEF`, `$QUIDEF`, `$SJCDEF`, which the services manual's examples include | not in `vmsdef`. `$QUIDEF` and `$SJCDEF` go with services govax has no queue manager for |

**What programs include:** the manuals' FORTRAN examples use `$SSDEF` (nearly all), `$FORDEF`,
`$LIBDEF`, `$LIBDCFDEF`, `$PSLDEF`, `$DVIDEF`, `$OPCDEF`, `$QUIDEF`, `$SJCDEF`, and in VSI's text
`$CHFDEF`. DUNGEON, the program's milestone, includes none and calls no `SYS$`, `LIB$`, or
`STR$` routine by name; of this phase, it needs only the system subroutines, on one line.

**Structures.** VSI's LIB$MATCH_COND example has `INCLUDE '($CHFDEF)'` and `RECORD /CHFDEF2/
MECHARGS`, then `MECHARGS.CHF$L_MCH_DEPTH`. The 5.0 examples use plain arrays (`MECHARGS(3)`), and
those work whatever the module holds. `vmsdef` has CHF's offsets and sizes, which settle `CHFDEF`,
`CHFDEF1`, and `CHFDEF2`. For the RMS blocks the offsets are there, but alternate names at one
offset would need unions worked out (question 3).

**No FORSYSDEF.TLB is on the VMS volumes here** (the simh system disk's text holds no such name; it
comes with the compiler). So nothing can check our modules against VMS's, and the search for
`SYS$LIBRARY:FORSYSDEF.TLB` only matters to someone who has one. Other `.TLB` files are on that disk,
which VMS's LIBRARY and `internal/lbr`'s reader can be tried on.

## The system subroutines (D.4, D.5.3)

The arguments' type picks the routine, as for ERRSNS (`sema/errsns.go`). FORRTL's symbol table shows
the same split (`FOR$DATE`, `FOR$DATE_T_DS`, `FOR$IDATE`, `FOR$JDATE`, `FOR$TIME`, `FOR$TIME_T_DS`,
`FOR$SECNDS`, `FOR$EXIT`, `FOR$EXIT_W`, `FOR$IMVBITS`, `FOR$JMVBITS`), and those routines can be
called on VMS to set the requirements (the probe's part A, below).

| Name | Rule (LRM) | FTNRTL |
|---|---|---|
| `DATE(buf)` | 9 bytes, `dd-mmm-yy`; a variable, array, element, or substring | `FTN$DATE` (by reference), `FTN$DATE_T_DS` (descriptor), on `$ASCTIM` |
| `TIME(buf)` | 8 bytes, `hh:mm:ss`, 24-hour | `FTN$TIME`, `FTN$TIME_T_DS` |
| `IDATE(i,j,k)` | month, day, year's last two digits, all INTEGER\*2 or all INTEGER\*4 | `FTN$IDATE` (words), `FTN$JDATE` (longs), on `$NUMTIM` |
| `EXIT[(status)]` | ends the program, closes files, status optional | `FTN$EXIT`, `FTN$EXIT_W`, `$EXIT` |
| `y = SECNDS(x)` | REAL\*4, seconds since midnight minus x, right across midnight | `FTN$SECNDS`, typed REAL whatever IMPLICIT says |
| `y = RAN(i)` | INTEGER\*4 seed, the same generator as MTH$RANDOM | a call of `MTH$RANDOM`, typed REAL |
| `MVBITS(m,i,len,n,j)` | moves a bit field; other bits unchanged | `FTN$IMVBITS`, `FTN$JMVBITS`, or EXTZV and INSV in line |

As with ERRSNS, a unit's own routine of the name wins, and so does `EXTERNAL DATE`, a program's way
of saying it has its own in another file.

## LIB$ and STR$: what's missing

**Phase 60 and Phase 61 are still only plans.** Phase 80 took TPARSE from Phase 60, leaving the trees.
Of Phase 61's routines the LIB$ program has: SIGNAL, STOP, ESTABLISH, REVERT, SIG_TO_RET, and
MATCH_COND, all working above, none yet reviewed against its manual entry (DEVIATIONS.md's note
stands: a continued LIB$SIGNAL returns 0 where VMS gives the caller's R0). Missing, each signaling
NOTIMPL: SIG_TO_STOP, FIXUP_FLT, DECODE_FAULT, SIM_TRAP, INT_OVER, DEC_OVER, FLT_UNDER,
FIND_IMAGE_SYMBOL, GET_OPCODE.

**The manuals' FORTRAN examples:** the LIB$ manual has about twenty, for ADDX, SUBX, EMUL, EDIV,
BBCCI, BBSSI, POLYF, ESTABLISH, MATCH_COND, SIGNAL, SIG_TO_RET, FLT_UNDER, DECODE_FAULT, LP_LINES,
SKPC, SPANC, and others. Of those named, only FLT_UNDER's and DECODE_FAULT's need a routine govax lacks. The services
manual's are `$FAO`/`$FAOL`, `$GETSYI`, `$PUTMSG`, `$GETQUI` (two), `$SNDJBC`, and one that enables
OPER1 through `$SNDOPR`.

**STR$:** of 32 entries only UPCASE exists, and it's wrong. Missing: ANALYZE_SDESC, APPEND, COMPARE,
CASE_BLIND_COMPARE, COMPARE_EQL, COMPARE_MULTI, CONCAT, COPY_DX, COPY_R, DUPL_CHAR, ELEMENT,
FIND_FIRST_IN_SET, FIND_FIRST_NOT_IN_SET, FIND_FIRST_SUBSTRING, FREE1_DX, GET1_DX, LEFT, LEN_EXTR,
MATCH_WILD, POSITION, POS_EXTR, PREFIX, REPLACE, RIGHT, TRANSLATE, TRIM; and the arithmetic ADD,
DIVIDE, MUL, RECIP, ROUND. No phase owns them. They are what a FORTRAN program reaches for, largely
because they write to any class of descriptor; `librtl/descriptor.go` has what they need.

## The probe

There is no VAX FORTRAN (Phase 74's answer 5), but VMS 7.1 has FORRTL, STR$, and LIBRARY.
`testdata/for83` (the author's simh round, in the usual form: `gen.go`, `.com`, `exchange.cmd`,
`copyout.cmd`), in three parts, with a fourth directory if question 2 asks for it:

- **A, the system subroutines on FORRTL:** each `FOR$` routine above, its arguments' forms and
  short buffers, IDATE's year, EXIT's status, RAN against MTH$RANDOM, SECNDS's midnight.
- **B, conditions on FORRTL:** a MACRO handler's choices for FORRTL's errors: which are signaled
  and which stopped (does SS$_CONTINUE go on after INPCONERR? after FILNOTFOU? FTNRTL gives ATTCONSTO),
  the unit's state after an unwind, the file's state after an exit handler runs, and I/O from an AST or
  a handler during a statement on the same unit and on another.
- **C, LIBRARY/TEXT:** VMS's own `.TLB` files (`/CREATE`, `/INSERT`, `/REPLACE`, `/MODULE=`,
  `/EXTRACT`, `/LIST`, the default module name) to be matched byte for byte, as `.MLB`s were; `$OPCDEF`
  through MACRO, as `testdata/mp/defs` did.
- **STR$, if question 2 takes them:** `testdata/str83`, as `testdata/lib54`.

## Decisions

1. **Null arguments in function references** go in the parser, and sema refuses one where the name
   turns out to be an array. `F(,)` has two null arguments, as `CALL`'s rule gives; `F()` none.
2. **`FORTRAN a+b+LIB/LIBRARY,c`:** `+` joins sources into one compilation, in a list item, with
   `/LIBRARY` positional, as LINK's is. The search for `(module)` is, unconfirmed: the libraries named
   in the item, in order; the `FORT$LIBRARY` logical name; `SYS$LIBRARY:FORSYSDEF.TLB` if present;
   then govax's modules. `'lib(module)'` reads only `lib`, default type TLB. A `(module)` not found is
   INCLOPEN naming the module. `source.Opener` gains a method for modules, keeping the compiler
   ignorant of files.
3. **govax's modules** are a new package, `internal/fortran/sysdef`: a table of module name and
   prefixes, each module's text made on demand from `vmsdef`, cached, in the same `INTEGER*4`/`PARAMETER`
   form; names sorted, duplicates by case dropped, over 31 characters left out. `$SYSSRVNAM` declares
   each service `INTEGER*4` and `EXTERNAL`. A `go generate`-style command (`mkforsysdef`, or
   `LIBRARY/TEXT` in a test) writes them as a `.TLB`, as the plan says, so users can read them.
4. **INCLUDE's `/LIST` and `/NOLIST` govern the `.MAR` as they will the listing** (the author's
   suggestion, 2026-11-01; question 1 settles the default). `source.Line` keeps a `Listed` flag, inherited
   by nested INCLUDEs (a `/NOLIST` file's own `/LIST` doesn't bring lines back), and `codegen/source.go`
   skips unlisted lines but keeps the comment naming the file. Phase 85's listing uses the same flag.
   Line numbers in messages are unaffected: a message always quotes the line.
5. **An unused EXTERNAL from a text library module gets no `.EXTRN`**; a program's own statement
   always does, for BLOCK DATA's sake. The storage map likewise leaves out a module's unused names.
6. **A missing service signals**, as a missing LIB$ routine does: SS$_UNSUPPORTED with
   GOVAX-E-NOTIMPL naming it, instead of a reserved operand fault. A fix under Decision 21, in
   subtask 4. Services themselves are added only when a program needs them; none does so far.
7. **LIBRARY/TEXT** gets `lbr.TextModules`: a file a module, named by `/MODULE=` or the file's name, its
   records as the probe shows. `/TEXT` joins `/MACRO`, `/OBJECT`, and `/SHARE`, default type TLB.
8. **The system subroutines are FTNRTL routines in MACRO**, with FORRTL's split by argument type,
   checked against the probe's part A. RAN is a call of MTH$RANDOM.
9. **STR$UPCASE is fixed** with whichever group takes STR$, or by itself if none does.
10. **`$QUIDEF`, `$SJCDEF`, `$GETQUI`, and `$SNDJBC` stay out**: govax has no queue manager, and a
    module for services that can't be called helps nobody. `$OPCDEF` goes in from the probe.

## Questions for the author

1. **INCLUDE's default in the `.MAR`.** Your suggestion has `/NOLIST` keep lines out of the `.MAR`.
   The LRM says `/NOLIST` is the default, so should a plain `INCLUDE` also leave its lines out, for both
   files and modules? Recommended: yes, the manual's default everywhere, with `/LIST` bringing them
   back and a comment always naming the file. The catch is that code from an included file with
   executable statements then has no source beside it unless the INCLUDE says `/LIST`, and
   FORTRAN-INTERNALS.md's and the manual's examples may change. The alternative is that only an
   explicit `/NOLIST`, and every text library module, leaves lines out.
2. **STR$.** The plan says "any STR$ routine the example programs call", and no example calls one.
   Recommended: do the 26 string routines here, as their own subtask with a probe, since FORTRAN
   programs on VMS use them heavily and no later phase owns them; leave the 5 arithmetic ones
   (ADD, DIVIDE, MUL, RECIP, ROUND) as NOTIMPL. Alternatives: only fix UPCASE, or do all 31.
3. **Structures in the modules.** Recommended: `$CHFDEF`'s three, which VSI's example needs and the
   offsets settle. Should `$FABDEF`, `$RABDEF`, `$NAMDEF`, and `$XABDEF` also declare structures
   (`RECORD /FABDEF/ FAB`, useful with USEROPEN)? Recommended: not now, as constants only; a program can
   already use `FAB$L_FOP` as an offset, and a structure's unions would be our guesses at VMS's.
4. **Phase 61's routines.** The plan names SIG_TO_RET (it exists), FIXUP_FLT, FLT_UNDER, and INT_OVER.
   Recommended: move SIG_TO_STOP, FIXUP_FLT, INT_OVER, DEC_OVER, FLT_UNDER, and the review of the six
   existing routines here, so that a FORTRAN handler's tools are complete and the DEVIATIONS.md note on
   LIB$SIGNAL's R0 is settled, with Phase 61 keeping DECODE_FAULT, SIM_TRAP, FIND_IMAGE_SYMBOL, GET_OPCODE,
   and LIB$INITIALIZE. The price: DECODE_FAULT's manual example stays unrun.
5. **The probe.** It needs a round on your simh VAX, as Phases 80 and 81 did. All right to plan on
   that, once subtask 2 has `gen.go` ready?

## The author's answers (2026-11-01)

All five as recommended:

1. **A plain INCLUDE leaves its lines out of the `.MAR`,** for files and modules alike, the LRM's
   `/NOLIST` default; `/LIST` brings them back, and a comment always names the file.
2. **STR$:** the 26 string routines, a subtask with a probe (`testdata/str83`); the 5 arithmetic ones
   stay NOTIMPL.
3. **Structures:** `$CHFDEF`'s three only; `$FABDEF`, `$RABDEF`, `$NAMDEF`, and `$XABDEF` are constants.
4. **Phase 61:** SIG_TO_STOP, FIXUP_FLT, INT_OVER, DEC_OVER, FLT_UNDER, and the review of SIGNAL, STOP,
   ESTABLISH, REVERT, SIG_TO_RET, and MATCH_COND move here; Phase 61 keeps DECODE_FAULT, SIM_TRAP,
   FIND_IMAGE_SYMBOL, GET_OPCODE, and LIB$INITIALIZE.
5. **The probe** gets a round on the author's simh VAX.

# What the probe settled (2026-11-01)

VMS 7.1 ran `FOR83.COM` whole on the author's simh VAX: `testdata/for83/vax/` and
`testdata/str83/vax/`. Parts A and B also ran over govax with VMS's FORRTL.EXE, so a difference
between the two runs is govax's services, RMS, LIBRTL, or DCL, and not FORRTL's.

**Parts A and B over govax:** part A matches VMS but for what depends on where things are. Part B
differs in three ways only, each listed below as 1, 2, and 8 of what govax gets wrong.

## What VMS does

- **DATE and TIME:** `FOR$DATE` and `FOR$TIME` store 9 and 8 bytes at an address, no more, and take
  no account of a second argument; with nothing there or a null address `FOR$DATE` signals an access
  violation and `FOR$TIME` returns SS$_ACCVIO. The
  `_T_DS` forms copy to the descriptor's pointer for its length, blank-padded or cut, whatever its
  class: a dynamic string of no length gets nothing, a varying string's text lands over its CURLEN,
  and a class A descriptor gets one element. `FOR$DATE_T_DS` and `FOR$TIME_T_DS` with no argument just
  return. The day has a leading blank, the month is in capitals: ` 1-NOV-26`. [Subtask 8.]
- **IDATE:** the month, day, and year's last two digits, as words in `FOR$IDATE`, longwords in
  `FOR$JDATE`, stored before any later fault shows: a null second argument leaves the first and
  third set. Too few arguments are an access violation.
- **SECNDS:** the seconds since midnight less x, plus 86400 once if that is negative, whatever x
  is (x = 1E7 still adds only one day). No argument signals nothing (the probe didn't show the value),
  and a null address is an access violation.
- **MVBITS:** `EXTZV` then `INSV`, with no checks of its own, every argument by reference, words in
  `FOR$IMVBITS` and longwords in `FOR$JMVBITS`. A length past the width is SS$_ROPRAND past 32, and short
  of that runs into the next bytes (16 bits and one more changes the next word); a negative position
  counts back. So D.5.3's limits are the program's to keep, and the compiler can write it in line.
- **RAN:** `MTH$RANDOM` as it stands; govax's gives VMS's numbers.
- **Which FORRTL errors a handler can continue from:** FILNOTFOU, ENDDURREA, INVLOGUNI, SYNERRFOR,
  INFFORLOO, NO_CURREC, and RECIO_OPE can't: FORRTL stops. INPCONERR (the item's value 0) and
  OUTCONERR can, and the statement goes on. FTNRTL's `FTN$IO_END` treats all alike and must not.
  Those two are all the probe asked of the continuing kind: `FORVARMIS`'s case gave OUTCONERR, its text
  being edited by I4. Subtask 6 either runs FTNRTL's whole list of statement-continuing errors on FORRTL
  in a second, smaller round, or marks the others unconfirmed.
- **Continuing from a stop:** no LIB$_ATTCONSTO. The condition's own message appears and the image
  ends with its status and the inhibit bit, `%X18018004`, for LIB$STOP and for FORRTL's alike. Neither
  run of VMS's shows ATTCONSTO anywhere. **Fix in govax's LIB$STOP.**
- **Unwinding out of a statement:** the statement is gone: a record begun isn't written, the next
  statement on the unit is fine, and a READ begun has used up its record.
- **I/O during a statement,** from a handler or an AST alike: the same unit (or a CLOSE of it) is
  RECIO_OPE, stopping; another unit, an internal file, and unit 6 work, and the statement goes on.
- **Ending the image:** `FOR$EXIT` takes a longword's address, `FOR$EXIT_W` a word's (`-1` is
  `%X0000FFFF`, not sign-extended); no argument is SS$_NORMAL, and a null address is an access violation.
  Open units are closed by all of them, by STOP, by `$EXIT`, and by falling off the end. FORRTL's exit handler
  closes units, so an exit handler declared before the first OPEN finds the unit closed (a write makes
  `FOR012.DAT`) and one declared after finds it open.
- **LIB$SIGNAL and LIB$STOP:** the mechanism array's R0 and R1 are the caller's, and a continued
  LIB$SIGNAL returns them, as a handler set them or not. That settles DEVIATIONS.md's note. **No
  argument** signals LIB$_SIGNO_ARG (`0015825C`) as any condition, from LIB$STOP too. LIB$STOP forces severe: `08018001` signals as `08018004`.
- **LIB$ESTABLISH** with no argument returns the old handler and leaves it. **LIB$REVERT** returns it
  and clears 0(FP).
- **LIB$MATCH_COND** matches by the manual: a value with bit 15 clear compares only message numbers,
  so `0C` matches `0802000C`, and a value's severity and control bits don't count.
- **LIB$SIG_TO_RET** doesn't check its arguments: too few are an access violation.
- **LIB$SIG_TO_STOP** forces the array's severity to severe and returns SS$_NORMAL, which as a
  handler's value means continue: so as the handler itself it ends the image with the message, and no outer
  handler hears of it. Called by a handler that then resignals, the outer handler sees it severe.
  On SS$_UNWIND it returns LIB$_INVARG and changes nothing. Too few arguments: an access violation.
- **LIB$FIXUP_FLT:** a reserved operand in a register, memory, deferred, autoincrement, autodecrement,
  or indexed replaced in place and the instruction restarted (autoincrement's register ending with its
  final value right), for F, D, G, H, and either operand of an instruction, each in turn. The new operand is
  F, converted to the operand's format. SS$_RESIGNAL for another condition, a POLY table, or a decimal
  reserved operand; SS$_ROPRAND for a reserved new operand; SS$_ACCVIO for an operand in read-only code,
  an immediate. No arguments, or too few: SS$_RESIGNAL.
- **LIB$INT_OVER, DEC_OVER, FLT_UNDER:** each returns the caller's old bit, 0 or 1, by bit 0 of a
  longword (`LTWO` is off, `LTHREE` on), changes the caller's frame only, and takes effect at once:
  SS$_INTOVF `47C` and SS$_DECOVF `4A4` trap there and go on; SS$_FLTUND_F `4C4` is a fault, so a
  handler that continues gets it again, for ever. No argument returns 0 with no fault; a null address
  is an access violation. [All three with subtask 6.]
- **LIBRARY/TEXT:** a new library's key is 39 characters and index 11 blocks; a module defaults to the
  file's name, in capitals, is named otherwise by `/MODULE=` on the file, one file's; a name past 39 is
  MODNAMLNG; `/INSERT` of one there already is DUPMOD, INSERTERR, DUPKEY, `%X108680E2`; `/REPLACE` needs no
  module there before; `/EXTRACT` of none is LOOKUPERR, KEYNOTFND, `%X108680F2`; several names go in one file,
  in the order named; `/LIST/NAMES` on a text library is a CONFLICT; an empty file is a module of one empty
  record; `/COMPRESS/DATA=REDUCE` gives DCX, which `internal/lbr` reads already, as it does all of VMS's `.TLB`s
  here. `/LOG` says `%LIBRAR-S-INSERTED`. `$STATUS` on success is `%X10000001`.
- **Definitions:** LINK's 11 undefined names are govax's 11 exactly (`PSL$`, `PRV$`, `LNM$_STRING`, ...), so
  `EXTERNAL PSL$C_USER` is undefined on VMS too and nothing is to change. `$OPCDEF` gives 228 names in
  `vax/def_opc.obj`, for `vmsdef` by `testdata/mp/defs`'s `decode.go` way.
- **STR$:** `testdata/str83/vax/str83-vax.txt` is the requirement, and has the facility's 17 message
  texts; subtask 9 reads it case by case. APPEND and PREFIX on a fixed string are ILLSTRCLA, and a bad class
  signals, as does a string past 65,535.

## What govax gets wrong

| # | Finding | Subtask |
|---|---|---|
| 1 | Continuing from LIB$STOP or FORRTL's stop says ATTCONSTO; VMS gives the condition's message and ends with its status | 6 |
| 2 | LIB$SIGNAL's R0 is 0 in the mechanism array, and after a continue or an unwind (DEVIATIONS.md) | 6 |
| 3 | LIB$SIGNAL and LIB$STOP with no argument print `%rtl:` and end the image | 6 |
| 4 | LIB$ESTABLISH with no argument clears 0(FP) | 6 |
| 5 | LIB$MATCH_COND ignores the system-code rule | 6 |
| 6 | LIB$SIG_TO_RET with too few arguments returns SS$_NOSIGNAL, where VMS has an access violation | 6 |
| 7 | FTNRTL's `FTN$IO_END` stops for every error, not only those FORRTL stops for | 6 |
| 8 | TYPE ignores FORTRAN carriage control: VMS's drops a record's first byte from a file with RAT=FTN (`unit 13` types as `nit 13`, `*` as an empty line) | 5 |
| 9 | `DEFINE/USER_MODE` outlives govax's MACRO and LINK, which aren't images | 5 |

## Subtasks (provisional, revised by the survey)

1. **Survey** (done 2026-11-01): Part A.
2. **The probe** (done 2026-11-01): `testdata/for83`, `testdata/str83`, VMS's run, and "What the probe
   settled".
3. **Text libraries and INCLUDE** (done 2026-11-01): LIBRARY/TEXT held to the probe's part C, `INCLUDE '(module)'` and
   `'lib(module)'`, `+` and `/LIBRARY`, `FORT$LIBRARY`, and `/[NO]LIST` on INCLUDE governing the `.MAR`.
4. **govax's definition modules** (done 2026-11-01): from `vmsdef`, `$SYSSRVNAM`, `$CHFDEF`'s structures, no `.EXTRN` for an
   unused name from one; the `.TLB` command.
5. **Calling services and RTL routines** (done 2026-11-01): null arguments in function references; a missing service's
   signal; programs calling `$ASCTIM`, `$GETJPIW` (`itemlist.for`'s list, handed over at last), `$GETSYIW`,
   `$TRNLNM`, `$FAO`, `$FAOL`, LIB$GET_INPUT, LIB$SPAWN, LIB$DATE_TIME; the manuals' examples.
6. **Condition handlers in FORTRAN** (done 2026-11-01), by the probe's part B; what question 4 moves here.
7. **ASTs and exit handlers in FORTRAN** (done 2026-11-01), by the probe's part B; programs for `testdata/fortran`, whose
   timing doesn't show in their output.
8. **The appendix D system subroutines** (done 2026-11-01), held to the probe's part A.
9. **STR$** (question 2) and `testdata/str83`, with the STR facility's messages.
10. **End to end**, then **close-out** (MANUAL.md's "Calling VMS from FORTRAN", with AST reentrancy;
    FORTRAN-INTERNALS.md; HELP FORTRAN and HELP LIBRARY; CLAUDE.md).

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-11-01:** Subtask 1, the survey: Part A. Throwaway programs ran services, handlers, unwinds,
  ASTs, and exit handlers on govax, most of which work already. It found the null arguments in function
  references, STR$UPCASE's wrong form, the reserved operand fault for a missing service, and the lines an
  INCLUDE adds to the `.MAR` (the author's suggestion, Decision 4). 38 prototype modules compile.
  Nothing else changed; five questions wait for the author.
- **2026-11-01:** The author's answers: all five as recommended. Phase 61's doc notes what moved here.
- **2026-11-01:** Subtask 2, the probe, written: `testdata/for83` (part A, 79 cases of FORRTL's system
  subroutines; part B, 64 runs of conditions on FORRTL; part C, 109 runs of LIBRTL's condition routines;
  part D, LIBRARY/TEXT; `lnk83.mar`, `def_opc.mar`) and `testdata/str83` (255 cases), one volume and one
  procedure. `FOR83.COM` runs whole on govax with VMS's FORRTL.EXE from `xfer.dsk` (LIBRARY/TEXT and
  `$OPCDEF` failing there, which is what they're for), every case reaching its end, and the handler stopping
  FLT_UNDER's repeating trap. That run only proves the programs; nothing is settled until VMS's run. Two things
  seen on the way, to compare with it: FORRTL ends the image with LIB$_ATTCONSTO if a handler continues from
  FILNOTFOU or ENDDURREA but not from INPCONERR, so FTNRTL's choice can't be one rule; and govax's LIB$SIGNAL
  with no arguments prints `%rtl:` and ends the image.
  A third: `DEFINE/USER_MODE` outlives govax's MACRO and LINK, which aren't images, so the first RUN after
  them writes to the file named (case 1 of parts B and C goes to its `.MLG` on govax); on VMS MACRO's exit
  ends it. To fix with the oracle tests, which need that case.
- **2026-11-01:** VMS's run of the probe, whole, with every case reaching its end: "What the probe settled".
  `copyout.cmd` first took the libraries as text, adding line ends; it now takes them and the object with
  `/BINARY`. The log shows no commands after part B, FOR83R's `SET NOVERIFY` lasting, which costs nothing.
  This subtask's oracle tests are the later subtasks': each holds what it builds to these files.
- **2026-11-01:** Subtask 3, text libraries and INCLUDE. **LIBRARY/TEXT**: `lbr.TextModule`, `lbr.CreateWith`
  (`/CREATE=(BLOCKS:n,GLOBALS:n,HISTORY:n,KEYSIZE:n,MODULES:n)`), an edited library keeping its preallocated
  blocks, the positional `/MODULE=`, default types TLB and TXT. A text library's failures are LIBRARIAN's messages
  with its statuses (DUPMOD, INSERTERR, DUPKEY; LOOKUPERR, KEYNOTFND; MODNAMLNG), through `library.stop`, and
  `/LIST/NAMES` is CONFLICT; the other types keep govax's words, unprobed. `/LOG`'s messages are `%LIBRAR-`, for
  every type (a new `LIBRFacility`). `TestFor83NewLibraries` matches `S1.TLB`, `T2.TLB`, and `T3.TLB` byte for byte, given VMS's
  times and librarian's name, and `TestFor83LibraryOracle` runs the part D commands on a volume and matches VMS's log
  line for line (times, file IDs, the creator, and the count of history records masked).
  **What part D's `S2.TLB` to `S7.TLB` show and govax doesn't do**, so the survey's "byte for byte" holds only for a
  library made by one command, which is also all `.MLB`s were held to. LIBRARIAN changes a library in place: each session starts a new data block, a
  replaced module's header gets a reference count of 0, a later module may reuse a deleted one's block, a failed
  `/INSERT` leaves its data behind, and history records (`ABBA`, length, time, a word of 1, 2 for insert or 3 for
  replace, the user, the module) go in a block of their own, which the header locates, with the bytes they use. LIBRARY
  builds a whole new layout, as it has since Phase 28, with the same modules and index and no history records, which it also
  drops from a VMS library it edits; `/LIST` shows 0 there. `/COMPRESS` and `/DATA=REDUCE` are left out. A failed `/INSERT`
  of several files leaves the library unchanged, where LIBRARIAN keeps the files before the failure.
  **INCLUDE**: `'(module)'` and `'lib(module)'` through `source.Opener`'s `OpenModule`; `/[NO]LIST` as `Line.Listed`, by the
  author's answer 1, which also changed the manual's 14.5 example to `INCLUDE 'limits/LIST'`. **The command**: a `/joinable`
  list in the grammar (`dcl.Result.Joined`), the positional `/LIBRARY`, `dclcmd.FortranSource`, `fortran.CompileAll` and
  `source.ReadAll`; `govax fortran` splits an argument at `+`, so a host path holding one can't be named there, and
  `govax library --text` is the subcommand's. The search is Decision 2's, without govax's modules, which are subtask 4's.
  Unconfirmed, there being no VAX FORTRAN or its User Manual: the search's order, a library named last still searched
  from the first INCLUDE, and `A+B`'s MACRO file named for A. Seen on the way and left: govax's COPY reports
  `%COPY-S-COPIED` without `/LOG`, where VMS's says nothing (the oracle filters the line).
- **2026-11-01:** Subtask 4, govax's definition modules. **`internal/fortran/sysdef`**: 47 modules, made once as
  FORTRAN text from `vmsdef` (`Symbols` over `LibrarySymbols`, as Part A found). A module is a list of prefixes and a name
  goes to the longest that matches, so no name is in two (`TestAllModulesTogether` compiles them in one unit under IMPLICIT
  NONE); names are in capitals, those that differ only by case one (`TT$_PRO_SERIES` stays), and past 31 characters none. The
  plan's list has the survey's counts (`TestPlannedModules`), with its "others ready by prefix". Added to them, by the same
  rule: `$LIBCLIDEF`, `$LIBDTDEF`, and `$LIBVMDEF`, which take `LIB$` names `$LIBDEF` and `$LIBDCFDEF` don't; and `$BRKDEF`,
  `$SHRDEF`, and `$STATEDEF`, which `vmsdef` already held. `$OPCDEF`'s 228 names are from VMS's `vax/def_opc.obj`
  (`testdata/for83/decode.go`, `defined.txt`, `gen -values`), so `vmsdef` now has them as Decision 10 said.
  `$SYSSRVNAM` is `vmsdef.P1VectorTable` without data cells, spares, `_2` halves, and `SYS$SRCHANDLER`. `$CHFDEF` has `CHFDEF`, `CHFDEF1`, and
  `CHFDEF2` as well as its constants (`TestHandlerLayouts` holds them to `vmsdef`), `L_` names only, so VSI's example's Alpha
  `CHF$IS_MCH_DEPTH` isn't a field, as its comment there says VAX's isn't. `$FORIOSDEF` is each `FOR$_` name's message number, which
  equals `FOR$K_`'s wherever both exist (`TestIOSTATNames`); its names are unconfirmed, and an end of file's IOSTAT is -1, not its number.
  The search of Decision 2 now ends in these (`FORSYSDEF.TLB (built into govax)` in messages and the MACRO file).
  **Decision 5**: `source.Line.Library` and `Statement.Library` (inherited through a module's own INCLUDEs), `sema`'s
  `Symbol.Linked` for the `.EXTRN`: a program's own EXTERNAL always, a module's if called or passed, a `%LOC` being passed. The
  storage map leaves out only a scalar a module declares and the unit never uses (`UnusedFromLibrary`, what layout
  already calls `onlyDeclared`). A module's unused array, COMMON, or EQUIVALENCE name still shows, since nothing tracks
  those; govax's modules declare none.
  **The `.TLB`**: `sysdef.Library` and `go run ./internal/fortran/sysdef/mkforsysdef [file]`, read by LIBRARY and `/LIBRARY`
  (`TestFortranBuiltinLibrary`). It isn't a govax command, so a user with only the binary can't make one. `testdata/fortran/sysdef.for` runs
  the modules end to end in both paging modes. A program's own `INTEGER*4` or `EXTERNAL` of a name a module already declares is DUPDECL or DUPATTR,
  as any second declaration is; whether VAX FORTRAN and its FORSYSDEF allow it is unconfirmed, and it matters to a program that
  declares a service it also gets from `$SYSSRVNAM`.
- **2026-11-01:** Subtask 5, calling services and RTL routines. **Null arguments in function references**, Decision 1: the
  parser's `subscripts` keeps a nil as `parseCall` does (`F(A,,B)` three arguments, `F(,)` two, `F()` none), and sema refuses
  one where the name isn't a routine's: NULLSUBSCRIPT for an array element, a field's subscripts, DATA, and EQUIVALENCE,
  NULLSTMTARG for a statement function, and NULLINTRINARG for an intrinsic (in place of an INTRINARGS that said `not 2` of a
  two-argument list). `TestNullArguments` compiles each form through to the MACRO file. **A missing service signals**, Decision 6
  (`corevms/missingservice.go`; `Environment.StackString` moved from `librtl`, which still calls it), found in `TestMissingService`.
  **The probe's findings 8 and 9:** TYPE takes a record's first byte as its control in a file of RAT=FTN (`rms.writeFortranRecords`,
  over `ftnTerminalText`; COPY is unchanged); a line a `$` record leaves open at the end of the file is ended, unconfirmed. A
  command VMS runs as an image deletes the process's user-mode names when it ends (`Interpreter.endUtility`, by `dclOwnCommands`),
  except RUN and DEBUG, whose images have a rundown of their own or may still be running under the debugger.
  **Programs:** `testdata/fortran/services.for` (with `.in`) and `rtlcalls.for`, in both paging modes. They are written fresh, with
  values of their own: DIGITAL's examples aren't copied into the repository.
  **The manuals' examples** ran as throwaway programs: ADDX, BBCCI, EDIV, LP_LINES, MATCH_COND (with `CALL EXIT` taken out, which is subtask 8's), POLYF, SIG_TO_RET,
  SKPC, and ESTABLISH's PUTMSG action routine give the manual's output, allowing for its OCR. Two don't, and are left:
  **EMUL's** has `TYPE *,14H'10000000000'x`, a Hollerith constant as a list item printing as text in the manual's output, where the
  LRM (2.2.1.7) says any other context makes it INTEGER\*4, so govax says HOLLCUT, as `TestErrors` has held since Phase 82;
  there is no compiler to settle which is right, and the LRM is what this program follows. **SIGNAL's**, LIB$SIGNAL of a bare
  `SS$_ACCVIO`, shows `reason mask=!XB, virtual address=!XL, ...` here, where the manual shows the signal's PC and PSL taken as the
  message's parameters (`reason mask=10, virtual address=03C00020, ...`); that is for subtask 6's review of LIB$SIGNAL.
  **Seen on the way:** `SYS$UNWIND(MECHARGS(3),)` returns to the establisher, past the statement that failed, and depth + 1 to
  its caller, which is what the manual's LIB$MATCH_COND example needs (it goes on to LIB$REVERT and CLOSE), and the unwind calls the
  handler of a frame it removes with SS$_UNWIND. A name used both as a function and in a CALL in one unit is BADUSE (ANSI's rule);
  whether VAX FORTRAN allowed it is unconfirmed, and it matters to a program calling a routine both ways.
- **2026-11-01:** Subtask 6, condition handlers. **LIBRTL, held to part C** (`TestFor83ConditionOracle`, all 109 cases;
  six compare only the condition, listed below). Findings 1 to 6 of "What govax gets wrong": continuing a stop ends the
  image with the condition's message and status (`attemptToContinueFromStop`; `libAttConSto` is gone); LIB$SIGNAL's and LIB$STOP's
  mechanism R0 is the caller's, by a stub that pushes R0 first (`corevms.CallerR0Stub`, `ShimLink.KeepR0`, read by
  `Environment.Signal`), which settles DEVIATIONS.md's note; no arguments is LIB$_SIGNO_ARG; LIB$ESTABLISH with none leaves the
  handler; LIB$MATCH_COND's systemwide rule, the facility being bits 17 to 27 by the manual, which the probe's values can't tell from
  16 to 27; LIB$SIG_TO_RET with too few arguments is an access violation. **New:** LIB$SIG_TO_STOP (`conditionDispatch.stopped`,
  `stops()`), LIB$INT_OVER, DEC_OVER, FLT_UNDER (`setTrapEnable`: the stub frame's saved PSW, which its RET restores), and LIB$FIXUP_FLT
  (`librtl/fixupflt.go`, over `disasm`; `corevms/condregs.go` finds the exception's registers in the mechanism array, the frames
  up to the handler's, or the registers themselves). Unconfirmed: no argument to the three with the bit set (it returns
  the old bit; VMS's 0 was with it clear); FIXUP_FLT of an immediate is always SS$_ACCVIO, as an image's code is read-only, even in a
  writable section; FIXUP_FLT of a SS$_ROPRAND from LIB$SIGNAL is SS$_RESIGNAL.
  **The cases that compare only the condition** (32, 33, 39, 97, 103, 109): VMS takes the access violation in the routine's own code, so
  a handler's continue runs the instruction again until the program stops itself. A shim's memory fault is a condition in
  its caller's frame that returns SS$_ACCVIO when continued (Phase 53, Decision 5), and that stands.
  **A CPU bug found by it:** a fault left the registers its operands' autoincrement and autodecrement had changed, so an instruction a
  handler continued ran from the wrong place. Decode now keeps a log (`registers/operandlog.go`), and `Engine.undoOperands`
  restores them for a fault, not a trap or a change mode instruction's exception. All faults get this, access violations too.
  **Messages:** a system message with too few arguments shows zeros past the signal's PC and PSL, as in the LIB$ manual's SIGNAL
  example, so its FORTRAN example now runs but for the last longword, stack contents there (`08000000`); RMS$_FNF doesn't show a status value of SS$_NOSUCHFILE, where
  RMS$_DNF does (part B, `testdata/dcl50`; lib58 shows the value is there); another value is shown, unconfirmed.
  **FORRTL's rule, by a new round:** `testdata/for83`'s part E (`for83e.mar`, 18 cases, `vax/for83e-forrtl.txt`) runs on FORRTL over govax, needing no VAX, since whether
  FORRTL goes on is FORRTL's own code (`TestFor83ContinueOnFORRTL`). A handler can go on from INPCONERR, at its item, the statement
  reading its other items; from OUTCONERR and FORVARMIS, at the statement's end; and from nothing else tried: OUTSTAOVE and INPRECTOO stop, with part
  B's FILNOTFOU, ENDDURREA, INVLOGUNI, SYNERRFOR, INFFORLOO, NO_CURREC, and RECIO_OPE. The other severe conditions, the namelist ones among them, are
  taken to stop, unconfirmed. After INPCONERR and a continue, I, L, and F have left the item 0, and O and Z as it was (where, unwound or with
  a specifier, Phase 80's oracle has 0: at the signal it's 0, and the continue puts it back); list-directed leaves it. A wrong type under I
  is INPCONERR and then FORVARMIS at the end, and under F only FORVARMIS, with the item 0.
  **FTNRTL does so** (finding 7): `FTN$IO_END`'s signal is `SIGNAL_FAILURE`, by LIB$SIGNAL for those three and LIB$STOP for the rest, and
  `FTN$$FMT_FAIL` signals INPCONERR at once when the statement has no ERR= or IOSTAT= (`FTN$$IO_SIGNAL`, the user PC the item call's,
  `CTX_ITEMPC`). A namelist's INPCONERR takes the same path, its going on unconfirmed. FORRTL's FOR$IO_END also returns the FOR number after a list-directed
  or wrong-type condition, which compiled FORTRAN with no specifier ignores, so FTN$IO_END doesn't.
  **Programs:** `testdata/fortran/handlers.for`, in both paging modes, and `TestFor83PartBOnFORRTL`, part B whole against VMS's log on VMS's FORRTL over
  govax. **Seen and left:** VMS's TYPE ends no line after a record with no carriage control (case 33's unformatted file), and govax's does. The five new routines
  moved the stubs after LIB$MATCH_COND in the table, so `testdata/nml80/vax/nml80e-forrtl.txt`'s one stub address changed.
- **2026-11-01:** Subtask 7, ASTs and exit handlers. **Part B held to FTNRTL:** `testdata/for83/for83x.for` is cases 36 to 46 and 55 to 64 as
  FORTRAN, numbered as `for83b.mar`'s, and `TestFor83PartBOnFTNRTL` holds them to VMS's log, less what only MACRO can see (R0, frame offsets): I/O from
  a handler and an AST during a statement, a unit open at the end, exit handlers before and after the OPEN. They passed unchanged. Cases 47 to 54, EXIT's forms, are
  subtask 8's. The two faults below are what reading FTNRTL for ASTs found; part B doesn't reach either.
  **An AST's statement could change an interrupted one's.** `FTN$IO_END` put its context on the free list and then read its status, so an AST between took the
  context and gave it the AST statement's: a READ that failed reported IOSTAT 0, 57 times in 300,000 iterations of a timer program. The stack and free list also
  took several instructions to change. Now ASTs are held off with `$SETAST` while they change (`HOLD_ASTS`, `RELEASE_ASTS`, on `PUSH_CONTEXT`, `DROP_TOP`, `POP_TO`,
  `POP_GONE`, `FREE_CONTEXT`), and a context goes on the free list only once it's been read, before the signal if there is one. That's six more service calls per statement.
  The first OPEN's `EXIT_DECLARED` is a `BBSS`. **`TestFor83ASTEverywhere`** (`for83t.for`, `for83u.mar`) is the regression test, as timers aren't: a change of one line moved
  `astrace.for`'s first failure past its 2,000 iterations, so that program, with no failures on the old library either, shows only that timers work among statements. `for83t.for` traces itself, and its handler
  declares an AST at every instruction that isn't held off, about 8,000 of them in under a second; the old library gets its IOSTAT wrong every run. RET ends tracing, by the
  manual and by Phase 73's probe, so `KEEPT` sets T in the saved PSW of each frame out to the establisher. On govax an AST declared by the handler runs in the handler, which sees the statement's state as it was at that instruction.
  **A statement a handler unwound from wrongly held its unit.** Its context stayed until a begin call from a routine at or above its frame, so after an unwind a statement on that unit from any deeper
  routine was RECIO_OPE, and an item or end of an outer statement used the gone statement's context. A third round on FORRTL (`for83g.mar`, 9 cases, `vax/for83g-forrtl.txt`, over govax, `TestFor83GoneOnFORRTL`) settled
  it, and `for83y.for` holds FTNRTL to it (`TestFor83GoneOnFTNRTL`). A context now records its frame's saved PC, and a begin call pops one the call chain doesn't reach with that FP and PC; `FTN$IO_ITEM` and
  `FTN$IO_END` pop those above their own statement (an AST's among them). **One difference stands** (cases 2 and 7, `for83GoneSameFrame`): a routine called again from the same place has the very frame of its gone statement, and if something deeper uses the unit with
  no statement between, FTNRTL says RECIO_OPE and FORRTL doesn't. Nothing in the frame tells them apart, and how FORRTL does isn't to be looked into; DEVIATIONS.md has it. A statement in that routine itself, the likelier form, is fine.
  **FORRTL declares its exit handler at a unit's first use, unit 6's too** (case 9), as FTNRTL does: one declared after a PRINT runs before the units are closed. **Programs:** `testdata/fortran/asts.for` (`$DCLAST`, `$SETIMR`, `$CANTIM`, `$QIO`'s AST, `$HIBER`, `$WAKE`,
  `$SETAST`, I/O from an AST during a statement, AUTOMATIC and static variables in a routine an AST reenters), `astrace.for`, and `exits.for`, in both paging modes. Unconfirmed: `exits.for`'s handler calling `$EXIT`, after which the rest run with the new status, as govax does and the services manual doesn't settle.
- **2026-11-01:** Subtask 8, the system subroutines. **FTNRTL** (`ftn_system.mar`, GSMATCH 1,16): `FTN$DATE`, `FTN$DATE_T_DS`, `FTN$TIME`, `FTN$TIME_T_DS`, `FTN$IDATE`, `FTN$JDATE`, `FTN$SECNDS`, `FTN$EXIT`, `FTN$EXIT_W`,
  `FTN$IMVBITS`, `FTN$JMVBITS`, with FORRTL's split by argument type, as Decision 8 has it; RAN is `MTH$RANDOM`. **The compiler** (`sema/system.go`): a table of the subroutines, each analyzing its own CALL with ERRSNS beside them, and of the two functions, REAL\*4 whatever IMPLICIT
  says. As ERRSNS's, a symbol in no unit's table stands for the routine, and a unit's own name or an `EXTERNAL` wins. NOTIMPL's `builtin` and `notImpl` are gone, no longer used. **Part A** (`TestFor83PartAOnFTNRTL`): `for83.mar` with each `FOR$` name's `FTN$`, every case, R0 too,
  against VMS's run, with the date, time, and year masked and then checked against `$ASCTIM`'s line of the same run. It differs in two lines, both past a bad argument: the address `IMVBITS` with a position of 65535 faults at (DEVIATIONS.md), and the reason mask's write bit where `JMVBITS` is given 4 arguments.
  **Part B's** cases 47 to 54 went into `for83x.for`, so `TestFor83PartBOnFTNRTL` now compares every case from 36 on. `testdata/fortran/sysroutines.for` and `sysown.for` are the end-to-end programs, in both paging modes, and DUNGEON's source files that use these routines (local only) compile with no message.
  **Choices the probe can't settle**, there being no VAX FORTRAN, unconfirmed: the compiler refuses what a routine would fault on, a null, a missing argument, or a constant for a place, though it could leave those to run time; types are the LRM's, so `CALL EXIT(L)` of a LOGICAL and `RAN` of an INTEGER\*2 are errors (SYSARG); an `EXIT` status or a `SECNDS` argument may be any
  expression; MVBITS's `m` and `n` must be of one length, which chooses the routine, and `i`, `len`, and `j` are converted to it, so `CALL MVBITS(W, I, 8, X, 0)` works with INTEGER\*2 W and X and INTEGER\*4 I, where the LRM's "all" allows that only for values in range. `RAN` of two arguments, the older form the 1988 LRM doesn't have, is SYSCOUNT. `INTEGER RAN` doesn't change what `RAN` returns, a REAL\*4 converted where it's used.
