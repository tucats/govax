# Phase 70 — traceback

**Status:** planned (2026-10-21). Subtask 1, the survey and the VMS
probe (`testdata/tb70`), is written and dry-run on govax; the probe
awaits its VAX run. Part B's decisions await the author's review.

## Goal

When a program's condition goes unhandled, govax prints what VMS
prints:

1. **The traceback dump.** In an image linked with traceback (LINK's
   default), an unhandled warning, error, or severe condition gets the
   traceback facility's symbolic stack dump after its message:

       %SYSTEM-F-ACCVIO, access violation, reason mask=00, virtual address=00000000, PC=0000021C, PSL=03C00000
       %TRACE-F-TRACEBACK, symbolic stack dump follows
       module name     routine name                     line       rel PC    abs PC

       FAILSUB         SUB2                                       00000005  0000021C
       FAILSUB         SUB1                                       0000000A  00000216
       FAILMAIN        FAILMAIN                                   00000009  00000209

   A warning or an error then goes on, as now; a severe condition ends
   the image.
2. **The "improperly handled condition" dump.** In an image linked
   `/NOTRACEBACK`, a severe hardware exception gets EXE$EXCMSG's dump
   (the signal arguments, the stack, the registers) after its message.
3. **The search's last stages in VMS's order**, and the bugs the
   probe's dry run found in the condition search (Part A's side
   findings).

Today govax prints only the condition's message, in every case.

Out of scope: SS$_DEBUG (DCL's DEBUG after a Ctrl/Y, which maps the
debugger), traceback of images in other access modes, and languages
other than MACRO (govax makes no other kind of object).

## Part A — the survey (2026-10-21)

### Sources

- *VAX/VMS Internals and Data Structures* (usable in full):
  - 4.5.1, "Traceback Handler Established by Image Startup": the image
    startup puts a frame on the user stack before the image is called,
    with a handler in it. SS$_DEBUG maps the debugger; otherwise "if
    the severity level is warning, error, or severe error, the handler
    maps the traceback facility ... and passes control to it"; other
    severities are resignaled, "passed on to the catch-all condition
    handler".
  - 4.5.2 and 20.3.2, the catch-all: established in the outermost call
    frame (by PROCSTRT or the CLI) *and* in the user-mode last-chance
    vector; it calls `$PUTMSG`, then, called through the last-chance
    vector (depth -3) or for a severe condition, EXE$EXCMSG's summary
    and image exit; otherwise it continues. For a condition from
    LIB$SIGNAL the PC and PSL are left off the message's arguments.
  - 21.1.3.2 and 21.1.3.3, the image startup system service
    (SYS$IMGSTA, "the debugger bootstrap"): the image's first transfer
    vector entry, unless linked `/NOTRACEBACK`; it establishes the
    traceback handler in its own frame and calls the next transfer
    address.
  - 30.2.3, EXE$EXCMSG: "a formatted dump of the general registers, the
    signal array, and the stack, as well as the caller's message text",
    "for all fatal errors that occur in images that were linked without
    the traceback handler".
- **The author** (2026-10-21): on VMS, traceback goes to both
  SYS$OUTPUT and SYS$ERROR when they're different files or devices.
- **Real VMS output already on hand** (VMS 7.3, Phase 29's probe,
  `testdata/mar/list/vax/list.log`, and its replay in
  `testdata/mar/round/vax/round.log`; VMS 7.1 in
  `testdata/mp/probe5/vax/probe5.log`). What they show:
  - The layout above: the `%TRACE` line, the column heading, a blank
    line, then a line per frame, innermost first: module in 16 columns,
    routine in 33, the line (blank for a traceback link, which has no
    line numbers), rel PC and abs PC in hex.
  - `%TRACE`'s severity letter is the condition's: `%TRACE-E-` after
    `%RMS-E-EOF` (which then goes on), `%TRACE-F-` after
    `%SYSTEM-F-ABORT` from LIB$STOP.
  - **rel PC is relative to the routine**: SUB2 starts at 217, so 21C
    is SUB2+5. The innermost frame's PC is the exception's; outer
    frames' are their return PCs (FAILMAIN+9 is after its CALLS). For a
    LIB$SIGNAL, the innermost frame is the caller's, at the return PC
    from LIB$SIGNAL (WARNIT+0F).
  - The dump stops at the program's main routine: neither the image
    startup's frame nor DCL's shows.
  - **No routine**: probe5's PROBE5\START frame shows a PC in S0
    (818D49B3) as `START` with rel PC 818D3DB3 (the PC less START's
    address, C00), and the next frame, at 101F, as `CODE`, a psect
    name, rel 41F. One reading: a PC past a routine's end falls to its
    psect, and the comparison is signed, so a PC in S0 counts as
    "before the end". The probe's S8, J, O, and P cases test it.
  - `$STATUS` after a fatal one is the condition with STS$M_INHIB_MSG
    (`%X1000000C`), with traceback or without.
  - **`/NOTRACEBACK`**: an access violation prints the message, a blank
    line, `  Improperly handled condition, image exit forced.`, then
    "Signal arguments" and "Stack contents" side by side, then
    "Register dump" (R0 to PSL, four to a line). A LIB$STOP of
    SS$_ABORT prints only its message: no dump. So the dump isn't
    simply "severe" as the book's catch-all says; the probe's parts C
    (SS$_ACCVIO by LIB$SIGNAL) and S20 (a severe LIB$SIGNAL) narrow it.
    The registers shown aren't all the program's (R0 = 7FFE5E40, where
    the program had none in R0), which the probe's part A (every
    register set to a pattern) should explain.

### What govax has

- The condition dispatcher (`internal/corevms/condition.go`): the
  vectors, the frame walk, the last-chance vector, then the catch-all
  in Go (`catchAll`), which writes the message (`formatMessageVector`)
  and continues or calls `$EXIT`. There is no traceback handler and no
  image startup frame: RUN's IMAGE$INIT driver (`internal/machine/run.go`,
  `buildDriver`) calls the main routine from a frame with no handler,
  below the console's sentinel frame, where the walk stops
  (`consoleCallSentinel`).
- The image's first transfer address is SYS$IMGSTA's (7FFEDF68) with
  traceback and is skipped (`mainTransferAddress`); whether the image
  has it is how govax can tell traceback from `/NOTRACEBACK`, as VMS
  does. The debug symbol table is read at activation (`ICB.Debug`, nil
  for `/NOTRACEBACK`), and `internal/dbgsym` already answers
  `ModuleAt`, `RoutineAt`, and line lookup for an address.
- LINK writes the DST from TBT records, and VMS's traceback already
  reads govax's images correctly (Phase 29: `round.log`).

### Side findings: the dry run (2026-10-21)

`tb70.com` runs on govax (`@TB70/OUTPUT=TB70.LOG`, a volume made by
`exchange.cmd`): every source assembles and every link succeeds. What
it showed of govax, beside the missing dumps:

1. **A handler that signals is called again for its own condition.**
   S11's INHANDH, which signals an error while handling a warning, is
   called for the error too; part H's HAV, which takes an access
   violation while handling an error, is called for it again and
   again, until the stack is gone. VMS's search, for a condition
   signaled while a handler runs, passes over the frames the first
   search has already looked at (from the handler's frame back to its
   establisher), so neither handler should be called a second time.
   The probe's S11 and H show what VMS does.
2. **A run that overflows the stack breaks every later run.** After
   part H, every later image (also under the other images) failed at
   once with `%SYSTEM-F-ACCVIO, reason mask=05, address 0x7fbffff8`
   (written by the engine, not the catch-all), each run's address 4
   lower: image rundown doesn't recover the stack.
3. **Arithmetic traps have the wrong condition.** Part I (integer
   overflow, IV set) reported `%NONAME-W-NOMSG, Message number
   80000000`, and part Z (integer divide by zero) `%SYSTEM-E-ACCVIO`
   with its `!XB` directives unformatted (condition 0000000A), where
   SS$_INTOVF and SS$_INTDIV belong.
4. **The last-chance vector runs before the catch-all.** S18 and part
   V call the program's last-chance handler. On VMS the catch-all is a
   frame handler (the CLI's outermost frame), found first, so the
   vector should be reached only when the stack can't be searched. The
   probe settles it.
5. **`DEFINE/USER_MODE SYS$OUTPUT file` before RUN is ignored** in
   process 1: no file is made, and part S's own lines went nowhere
   (Phase 48's file SYS$OUTPUT is a subprocess's).
6. **STS$M_INHIB_MSG is ignored by the catch-all's message**: S5's
   message is shown. VMS's `$PUTMSG` doesn't look at the bit, but the
   catch-all may; the probe shows which.

## Part B — decisions and questions (proposed, for the author)

### Proposed decisions

1. **A real image startup frame.** RUN's driver gets a handler of its
   own in its frame, when the image's first transfer address is
   SYS$IMGSTA's: govax's traceback handler, a shim reached through a
   stub, as SYS$SRCHANDLER is. So the search finds it after the
   program's frames, as VMS's does, and a program's own handlers and
   `$UNWIND` see the frames VMS's would. For W, E, and F it writes the
   dump and (W, E) continues or (F) goes on to the catch-all's action;
   for S and I it resignals. The catch-all stays in Go, but moves ahead
   of the last-chance vector if the probe confirms finding 4.
2. **The dump in Go**, from the debug symbol tables (`ICB.Debug`) of
   the main image, and of shareable images if VMS names their routines
   (S15, part M). Frames are walked as the condition search walks them,
   from the condition's frame (the signal PC first) to the image
   startup's frame, which isn't shown.
3. **Output as `$PUTMSG`'s**: to SYS$OUTPUT, and to SYS$ERROR too when
   it's a different file or device (the author's note), in the order
   the probe's split files show.
4. **EXE$EXCMSG's dump** for `/NOTRACEBACK`, with the probe's rules for
   which conditions get it and which registers it shows.
5. **The side findings' fixes are in this phase**, each its own
   subtask (findings 1 to 3 and 5 are bugs whatever VMS says; 4 and 6
   wait for the probe).

### Questions for the author

1. Is Decision 1's driver frame acceptable, or should traceback stay in
   Go at the catch-all (simpler, but a program's handlers would never
   see VMS's frame order)?
2. Should the oracle run VMS's own images (`vax/*.exe`), as Phases 66
   and 67's do, so that every PC matches? LIBRTL's addresses still
   differ and would be masked.

### The probe's cases (subtask 1; `testdata/tb70`)

`tb70.mar`'s parts (by `TB70_PART`), each run under TB70 (traceback),
TB70NT (`/NOTRACEBACK`), and TB70D (`LINK/DEBUG`, run `/NODEBUG`, for
line numbers):

| Part | Case | What it settles |
| --- | --- | --- |
| S | S1 to S5: S, I, W, E, and W with STS$M_INHIB_MSG by LIB$SIGNAL | which severities get a dump; the inhibit bit |
| S | S6, S17: an error 5 and 60 calls deep | a depth limit |
| S | S7, S8: from a JSB subroutine; from a psect with no routine | naming a PC outside a routine |
| S | S9, S10: a handler resignals; a handler changes the condition to a warning | the frames shown; the letter from the changed condition |
| S | S11: a warning whose handler signals an error | finding 1; the dispatcher's frames in a dump |
| S | S12, S13: in an AST routine; through LIB$CALLG | system and LIBRTL frames |
| S | S14: in a module and routine with 31-character names | the columns |
| S | S15, S16: in TB70SHR; called back by TB70SHR | shareable images' names |
| S | S18: with a last-chance handler | finding 4 |
| S | S19: a warning with a severe condition after it | the letter from the first condition |
| S | S20, S21: a severe LIB$SIGNAL | its dump under `/NOTRACEBACK` |
| A | access violation two deep, registers set | EXE$EXCMSG's registers and stack |
| O, I, Z, R | HALT in the DATA psect; integer overflow; divide by zero; reserved operand | faults and traps (the trap's PC is the next instruction's); a PC in data |
| J, P | access violation in a JSB subroutine; a jump to 3FFF0000 | a PC in no routine, in no image |
| L | LIB$MOVC3 to address 0 | a fault inside LIBRTL |
| K, W, E | LIB$STOP through LIB$CALLG; LIB$STOP of a warning, of an error | LIB$STOP's severity |
| H | access violation in a handler | finding 1 |
| T, X | in an AST routine; in an exit handler | those frames |
| D | access violation 100 calls deep | the depth limit |
| G | in TB70LONG | the columns |
| V | with a last-chance handler | finding 4 |
| C | SS$_ACCVIO by LIB$SIGNAL | a software ACCVIO's message and dump |
| M, N | in TB70SHR; in a routine TB70SHR called back | shareable images |

Then part A and part S with SYS$OUTPUT and SYS$ERROR in separate files
(with and without traceback), part A in a subprocess, and no part.

## Part C — subtasks

1. **The survey and the probe** (this doc's Part A, `testdata/tb70`).
2. **The probe's VMS run**: Part A's answers, and Part B revised by
   them.
3. **The nested-signal search** (finding 1) and stack recovery at
   rundown (finding 2).
4. **Arithmetic trap conditions** (finding 3).
5. **The image startup frame and the search's order** (Decision 1,
   finding 4).
6. **The traceback dump**: frames, names, rel PC, lines, the layout.
7. **EXE$EXCMSG's dump** for `/NOTRACEBACK`.
8. **Output to SYS$OUTPUT and SYS$ERROR** (Decision 3), and RUN's
   `DEFINE/USER_MODE SYS$OUTPUT file` (finding 5).
9. **The oracle**: `TestTB70Oracle` runs `tb70.com`'s parts on govax
   and matches VMS's log.
10. **The close-out**: the manual (`docs/MANUAL.md`), CLAUDE.md,
    PLAN.md.

## Progress log

### 2026-10-21 — Planned; subtask 1, the survey and the probe

Read the Internals book's sections on the traceback handler, the
catch-all, the image startup, and EXE$EXCMSG, and the VMS dumps
already on hand from Phases 29 and 45 (Part A). Wrote the probe,
`testdata/tb70`: `tb70.mar` (21 parts), `tb70long.mar`, `tb70shr.mar`,
`tb70.com`, `exchange.cmd`, `copyout.cmd`. A dry run on govax
(`@TB70 "..."` without part H) assembles and links everything and runs
every part; it found Part A's six side findings. The probe's first
draft signaled SS$_WASSET for its success case, whose message number
is ACCVIO's (govax showed `%SYSTEM-S-ACCVIO`; what VMS shows for 9 is
unconfirmed), and gave LIB$SIGNAL FAO counts for system conditions,
which take none; both fixed (SS$_NORMAL; no counts). Waiting for the
VAX run.
