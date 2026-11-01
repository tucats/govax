# Phase 83 — FORTRAN and VMS: definitions, services, handlers, ASTs

**Status:** in progress. Subtask 1, the survey (Part A below), done 2026-11-01, waiting for
the author's answers to its questions. Needs Phase 81 (files) and Phase 82 (structures). Part
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

## Subtasks (provisional, revised by the survey)

1. **Survey** (done 2026-11-01): Part A.
2. **The probe:** `testdata/for83`, its VMS round, and what it settles.
3. **Text libraries and INCLUDE:** LIBRARY/TEXT held to the probe's part C, `INCLUDE '(module)'` and
   `'lib(module)'`, `+` and `/LIBRARY`, `FORT$LIBRARY`, and `/[NO]LIST` on INCLUDE governing the `.MAR`.
4. **govax's definition modules** from `vmsdef`, `$SYSSRVNAM`, `$CHFDEF`'s structures, no `.EXTRN` for an
   unused name from one; the `.TLB` command.
5. **Calling services and RTL routines:** null arguments in function references; a missing service's
   signal; programs calling `$ASCTIM`, `$GETJPIW` (`itemlist.for`'s list, handed over at last), `$GETSYIW`,
   `$TRNLNM`, `$FAO`, `$FAOL`, LIB$GET_INPUT, LIB$SPAWN, LIB$DATE_TIME; the manuals' examples.
6. **Condition handlers in FORTRAN**, by the probe's part B; what question 4 moves here.
7. **ASTs and exit handlers in FORTRAN**, by the probe's part B; programs for `testdata/fortran`, whose
   timing doesn't show in their output.
8. **The appendix D system subroutines**, held to the probe's part A.
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
