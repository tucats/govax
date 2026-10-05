# Phase 42's debugger probe

How the VMS debugger (VMS 7.3, on simh) behaves at its run-control
commands, and how it words their output: breakpoints, tracepoints,
watchpoints, STEP, EXAMINE and DEPOSIT, CALL, exceptions, and its error
messages. govax's debugger (docs/PHASE-42.md) is checked against it.
Phase 41's probe (`../dbg/`) already shows instructions, symbols,
`SHOW CALLS`, simple breaks, and steps; this one covers what that one
doesn't. Clean room: what govax does comes from these logs and DIGITAL's
manuals (the *VMS 5.5 Debugger Manual*), never VMS's source.

## The program

`dbgcmd.mar` is one module, built `MACRO/DEBUG` and `LINK/DEBUG`:

| Part | For |
|---|---|
| `FACT`, recursive, called as FACT(5); `BACK` is where each call returns | STEP/OVER and STEP/RETURN across recursion, `SET BREAK/AFTER`, `/RETURN`, tracepoints at each depth |
| `LOOP`/`ODD`, a three-pass loop of branches | `SET BREAK/BRANCH`, `/LINE`, a list of breaks |
| `WATCHL`, `WATCHB`, `BUFFER`, `RESULT`, written by MOVL, ADDL2, INCL, MOVB, and MOVC3 | watchpoints on a longword, a byte, and 16 bytes |
| `BUMP`, a JSB subroutine | STEP into and out of a JSB subroutine |
| `CATCH` signals `SS$_ENDOFFILE` (a warning) with `HANDLR` established; then START signals it with no handler | `SET BREAK/EXCEPTION`, STEP into a handler, and what the debugger does at an unhandled warning |

It runs to the end (`DBGCMD: done`), and govax runs it too: the handled
signal is silent, and the unhandled one prints `%SYSTEM-W-ENDOFFILE,
end of file` and continues. The DATA psect is at 200 and CODE at 400.

## The debugger command files

`DBGCMD.COM` builds the image, then runs it under the debugger once per
file, logging each session (commands and output) to a `.dlg` of the
same name:

| File | What it asks |
|---|---|
| `break.dbg` | `SET BREAK` on a routine, a label, an address with no label, a list; `/AFTER:3`, `/TEMPORARY`, `WHEN` (true and false), `DO`; `SHOW BREAK` after each; `CANCEL BREAK` one and `/ALL` |
| `brkcls.dbg` | `SET BREAK/CALL`, `/RETURN FACT`, `/BRANCH`, `/LINE`, `/INSTRUCTION=(MOVB,MOVC3)`, and `/INSTRUCTION`, each with `SHOW BREAK`, a few `GO`s, and `CANCEL` |
| `step.dbg` | `STEP` by line (the default) and `/INSTRUCTION`; `/INTO`, `/OVER` of FACT's recursive CALLS (which frame it stops in), a count, `/RETURN` from a routine and a JSB subroutine, `/BRANCH`, `/CALL`, `/SILENT`, `/NOSOURCE`; `SET STEP` keywords and `SHOW STEP`; stepping over the signals |
| `watch.dbg` | `SET WATCH` on each data item, `/TEMPORARY`, `SHOW WATCH`, the report for each kind of write, a DEPOSIT to a watched location, `CANCEL WATCH` one and `/ALL` |
| `trace.dbg` | `SET TRACE/INSTRUCTION`, `SET TRACE` on a routine and a JSB subroutine, `/LINE`, `/BRANCH`; `SHOW TRACE`, `CANCEL TRACE` |
| `exam.dbg` | four calls deep: `SHOW STACK`, registers (`R2`, `AP`, `SP`, `PSL`, `.SP`, `@SP`, `.AP+4`), `EXAMINE` with no address, `.`, and `^`; `/ASCII:n`, `/BYTE`…`/QUADWORD`, radix qualifiers, ranges and lists, labels as arrays (`BUFFER[2]`); numeric addresses; `EVALUATE` with MACRO's operators (`MOD`, `EQL`, `NEQ`, `@`, `NOT`, `AND`); `DEPOSIT` to memory and registers; `SET MODE` (`NOSYMBOLIC`, `NOLINE`, `OPERANDS[=FULL]`, `G_FLOAT`), `CANCEL MODE`, `SET RADIX` (`/INPUT`, `/OUTPUT`), `CANCEL RADIX`, `SHOW MODE`, `SHOW RADIX`, `SHOW SCOPE` |
| `call.dbg` | `CALL FACT (%VAL n)` (the value returned; registers kept), `CALL` with no arguments, a breakpoint inside a called routine, `CALL LIB$PUT_OUTPUT (MSG)` |
| `except.dbg` | `SET BREAK/EXCEPTION`: the break for a handled condition, STEP from it, the break for an unhandled one, and `GO` from each |
| `errors.dbg` | unknown symbols, verbs, keywords, and qualifiers; an address with `/CALL`; cancelling what isn't set; DCL commands (`DIRECTORY`, `SET DEFAULT`); conflicting qualifiers; inaccessible addresses; the unhandled warning without `/EXCEPTION`; `GO`, `STEP`, `SHOW CALLS`, and `EXAMINE` after the program has exited |

Ctrl/C can't be scripted, so it isn't probed; the manual's description
stands for it (docs/PHASE-42.md).

Some commands are there to find out whether VMS accepts them (`.AP+4`,
`EVALUATE .R2`, `CALL LIB$PUT_OUTPUT` without `SET IMAGE LIBRTL`, the
`WHEN` clause's `.WATCHL`). An error in the log is a result too. A
command that fails can shift what later `GO`s reach; the logs say where.

## Running it

1. Build the volume, from the repository root (it makes
   `testdata/disks/dbgcmd-exchange.dsk`, label DBGCMDX, gitignored):

       go build -o govax ./cmd/govax
       ./govax console < testdata/dbgcmd/exchange.cmd

2. On VMS, with the disk attached to simh:

       $ MOUNT DUA1: DBGCMDX            ! or the unit you attached it as
       $ SET DEFAULT DUA1:[000000]
       $ @DBGCMD/OUTPUT=DBGCMD.LOG

   It takes a minute or two, and prints nothing to the terminal but the
   program's own output.

3. Pause simh (or detach the disk), then copy the results back:

       mkdir -p testdata/dbgcmd/vax
       ./govax console < testdata/dbgcmd/copyout.cmd

   `vax/` gets `dbgcmd.log`; the image's `.exe`, `.map`, and `.ani`; the
   listing and object; and the nine session logs (`.dlg`).

If a session's `.dlg` is missing or empty, the debugger didn't take its
commands from `DBG$INPUT`. Then run that one by hand: `RUN/DEBUG DBGCMD`,
and at `DBG>` type `@RUN.DBG` after making `RUN.DBG` as `DBGCMD.COM`'s
`DEBUG` subroutine does (`SET LOG`, `SET OUTPUT LOG,VERIFY`,
`@probe.DBG`, `EXIT`).

## What came back (`vax/`)

Not yet run.
