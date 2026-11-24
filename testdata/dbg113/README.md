# Phase 113's debugger round

What the VMS debugger (VMS 7.1, on simh) does where Phases 42, 85, and 89 left govax's
debugger to choose (`docs/planning/PHASE-113 - Fortran errata.md`, subtask 12c, Part A's
A.8.6): one program and eleven sessions. It goes to the VAX on `testdata/err113`'s volume,
in `[DBG113]`, in the same simh session as the rest of the round (that README's "Running
the session").

VMS's session logs leave out a command that fails, so each `*.dbg` names a case in a
comment (`!>`) just before it. Numbers are written `%HEX n` or with a leading 0, the debugger's
own forms: VMS's doesn't read `^X` (testdata/dbg89's DEPOSITs were lost that way).

## The program

`dbg113.mar`, one module, built `MACRO/DEBUG` and `LINK/DEBUG`. Each routine is a stage
(its header comment has the table): REC(3) down to LEAF as testdata/dbg89's, with IV set by
REC's entry mask, DV by LEAF's, and N live at INLEAF; FLOATS, short literals in F, D, G,
and H instructions; REGINS, registers holding an instruction's bytes; LOOP, run twice,
three passes each over LOOPTOP, writing WATCHL and COUNT; SUBA, SUBD, and SUBE, each
calling its own SUBBn; TWICE, a macro whose one line is two calls of SUBC; SIGNAL, two
unhandled warnings. START declares an exit handler, which prints `DBG113: the exit handler
ran`.

## The sessions

| File | What it asks | Where it came from |
| --- | --- | --- |
| `dbg113s.dbg` | TYPE with no line before any is shown; an outer scope's PSL (its condition codes, IV, and DV: the live PSL's, the frame's saved PSW's, or neither); `%NEXT_SCOPE_ENTRY` at the last levels; EXAMINE with no location after PC, PSL, SP, and R11; DEPOSIT to an outer scope's registers (one a frame saved, one no frame saved, R0, SP, PSL, PC), and where each value is after the RETs | Phase 89's unconfirmed scope details |
| `dbg113a.dbg` | A register's name in an address expression: testdata/dbg85's `EXAMINE @(AP+SUB85.N)` read memory at FFFF0038, as if AP's address were FFFF0030. EVALUATE and EVALUATE/ADDRESS of registers, `AP+4`, `@AP`, `@(AP+4)`, `R2+1`, `R2:R4`, and EXAMINE and DEPOSIT of FFFF0008 to FFFF0044 | testdata/dbg85's `@(AP+N)` |
| `dbg113i.dbg` | G and H short literals' digits (govax writes F's seven); EXAMINE/INSTRUCTION of a register whose instruction runs past its four bytes (R5 into R6, R11 into AP) and EXAMINE after it; an entry mask and the location after it; EXAMINE/OPERANDS brief and FULL of every operand kind, and SET MODE OPERANDS=FULL | Phases 42 and 89 |
| `dbg113t.dbg` | CANCEL TRACE and WATCH of nothing; SHOW BREAK's and SHOW TRACE's order when set out of address order and of several kinds; tracepoints by address, line, and branch meeting at LOOPTOP (their report order); `/AFTER` and `WHEN` on tracepoints and watchpoints | Phase 42 |
| `dbg113r.dbg` | STEP/RETURN interrupted by a break, then GO: does the return still stop? | Phase 42 |
| `dbg113n.dbg` | STEP/RETURN in SUBD interrupted by a break in SUBB2, STEP/RETURN there, then GO: does SUBD's still stop? | Phase 42 |
| `dbg113o.dbg` | STEP/OVER interrupted by a break, then GO: does the step's end still stop? | Phase 42 |
| `dbg113l.dbg` | A line step from a RET whose return is in the middle of the caller's line (TWICE) | Phase 42 |
| `dbg113x.dbg` | Exception breaks: the location's text with SET STEP LINE and INSTRUCTION; SHOW LANGUAGE with the PC in no module; then GO to the program's end, whose log shows the exit handler's line | Phase 42 |
| `dbg113m.dbg` | SHOW SYMBOL with `/ADDRESS` and `/TYPE`; SHOW LANGUAGE, and SET LANGUAGE UNKNOWN; SET, SHOW, and CANCEL SOURCE (with `/MODULE`) and their wording; NOSUCHMODULE; a source record of 184 columns; EXIT at a break: does the exit handler run? | Phase 42 |
| `dbg113q.dbg` | QUIT at a break: does the exit handler run, and is QUIT logged? | Phase 42 |
| `dbg113.com` | Builds the image, runs each session (`CALL DEBUG image commands log`), and DBG113S again on GVDBG113, govax's image of the same source, with DUA5 a name for the round's device | |

Phase 42's multi-dimension array bounds in SHOW SYMBOL aren't asked: MACRO can't
describe an array. testdata/dbg85's other differences (`SHOW SYMBOL *` listing VMS's
debugger's own images, `dbg85c.dlg` not replayed) are VMS's own images, not questions.

## Running it

In the round's session (`testdata/err113/README.md`), with the round's volume on DUA1:

    $ MOUNT DUA1: ERR113
    $ SET DEFAULT DUA1:[DBG113]
    $ @DBG113/OUTPUT=DBG113.LOG
    $ DISMOUNT DUA1:

No privileges are needed. `testdata/err113/copyout.cmd` copies the log, the sessions'
logs, and the listing, map, object, and image back into `vax/`.

`TestDbg113Procedure` (`internal/console/dbg113_test.go`) runs DBG113.COM on govax, so a
mistake in a session shows first, and once `vax/` is here logs each session that differs
from VMS's.

## What came back

Nothing yet.
