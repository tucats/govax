# Phase 70 — traceback

**Status:** in progress. Planned 2026-10-21; Part B's decisions
accepted by the author the same day. Subtasks 1 (the survey and the
VMS probe, `testdata/tb70`), 2 (the probe's two VAX rounds; Part A,
"The probe's answers"), 3 (the nested-signal search and the stack
after a blown run), 4 (the arithmetic traps), 5 (the image startup
frame and the search's order), 6 (the traceback dump), and 7
(EXE$EXCMSG's dump) are done.

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
   lower: image rundown doesn't recover the stack. (Subtask 3 found
   three causes: the run ended in a HALT, not an exit, so nothing
   reset the stack; see the progress log.)
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

### The probe's answers (round 1, 2026-10-21)

OpenVMS V7.1 on simh's VAX 8600 ran `@TB70/OUTPUT=TB70.LOG`
(`testdata/tb70/vax/tb70.log`, with the images, maps, and objects). In
summary, with the cases that show each rule:

**When there's a dump.**

- In an image linked with traceback, a W, E, or F condition no handler
  takes gets the traceback dump; S and I conditions get only their
  message (S1, S2). The `%TRACE` line's letter is the condition's
  severity as the catch-all sees it: a handler's change counts (S10: W),
  and only the first condition's severity matters (S19: W, with an F
  after it). A W or E then goes on; an F ends the image.
- With STS$M_INHIB_MSG set, the message isn't shown, but the dump is
  (S5). Without traceback, nothing at all is shown (TB70NT's S5).
- LIB$STOP makes any condition severe: `%SYSTEM-F-ENDOFFILE`,
  `%RMS-F-EOF`, and `%TRACE-F` (parts W, E); `$STATUS` is the F form
  with STS$M_INHIB_MSG (`%X10000874`).
- Without traceback, the "improperly handled" dump follows the message
  of every hardware exception (A, O, I, Z, R, J, P, L, H, T, D, G, V, M,
  N) and of SS$_ACCVIO signaled by LIB$SIGNAL (C), but not of a severe
  LIB$SIGNAL of SS$_BADPARAM (S20) or any LIB$STOP (K, W, E). Round 2:
  it's the condition's value. SS$_OPCDEC by LIB$SIGNAL (a signal array
  of 3) and LIB$STOP of SS$_ACCVIO get it; SS$_BADPARAM with an array
  of 5 doesn't. govax gives it for the conditions its exception
  dispatcher raises (`exceptionCondition`'s); only ACCVIO, OPCDEC,
  INTOVF, INTDIV, and ROPRAND are confirmed.
- An exit handler's access violation (part X) gets the "improperly
  handled" dump even in an image linked with traceback: by then image
  rundown has removed the image startup's frame, and the catch-all
  takes it.
- A subprocess's run (SPAWN) prints the same dump.

**The frames.** The lines are, innermost first: the condition's PC
(the exception's, or for LIB$SIGNAL and LIB$STOP their return PC),
then, for each call frame from the one the condition happened in
(for LIB$SIGNAL, its caller's), the frame's saved PC; the frame whose
caller is the image startup's isn't shown, so the main routine's line
is the last (`TB70+67`, the return from its CALLS). Consequences:

- A JSB subroutine makes no frame, so the routine that called it is
  missing (S7, J: RECSIG's JSB code, then TB70; no PART_S).
- An AST routine's frame's saved PC is in S0 (the AST delivery code,
  8192B447), and the next line is the PC the AST interrupted (S12, T).
- LIB$CALLG's frame shows as a PC in LIBRTL (S13, K), a callback
  through TB70SHR as a PC in TB70SHR (S16, N).
- No depth limit: 60 and 100 frames are all shown (S17, D).
- A condition raised in a condition handler (S11, H): the handler's
  frame's line, then

      ----- above condition handler called with exception 00000870:
      %SYSTEM-W-ENDOFFILE, end of file
      ----- end of exception message

  (the earlier condition's value and its message), then the earlier
  condition's PC and the frames above it: the dispatcher's frames
  between the handler and the earlier condition aren't shown.
- A handler is never called again for a condition raised while it runs
  (S11, H: one "HAV called"); side finding 1 is confirmed.

**Naming a PC.** One rule, with traceback alone or with `/DEBUG`:

1. The routine, of any module, with the greatest address at or below
   the PC, if the PC less its address, *compared signed*, is below 400
   (hexadecimal): that routine, rel PC from it. The DST's routine
   sizes don't matter: PART_S+3CE is PART_S (S16), PART_S+40B isn't
   (S17's next frame: `CODE`); code in psect CODE2 is TB70LONG's last
   routine +2D (S8), and a PC in S0 is that routine too, with a
   "negative" rel PC (8192A52E, S12, T). Round 2 pinned the limit:
   +3FF is the routine, +400 isn't (`R3FF +3FF`, then `CODE +9C1`).
2. Otherwise the psect contribution holding the PC: its name in the
   routine column and its module, rel PC from the contribution's start
   (S17 to S20: `CODE` +498, from 400; O: `DATA` +A5).
3. Otherwise nothing: blank columns, rel PC the same as abs PC (P's
   3FFF0000; PCs in LIBRTL and TB70SHR: shareable images' own debug
   tables aren't used, though TB70SHR was linked with traceback).

`/DEBUG`'s DST gives code that's outside any `.ENTRY` routine a routine
with no name (CODE2's, at F38, in TB70D), which rule 1 finds: S8 and
the S0 PCs are `TB70` with a blank routine (rel from F38).

**Line numbers** (TB70D): the module's line-number table's line for the
condition's PC if a hardware fault raised it (AV2's MOVL, line 329; the
reserved operand, 357), and for the address before the PC otherwise: a
trap (INTOVF's ADDL2, 343; INTDIV's DIVL2, 351), LIB$SIGNAL and LIB$STOP
(the line of the call), and every saved PC (the CALLS's line). The line
is found whether or not rule 1 found a routine (`CODE` lines have one,
207). A PC with no line (DATA, S0, other images) has a blank line.

**The layout.** `%TRACE-x-TRACEBACK, symbolic stack dump follows`, the
heading `module name     routine name                     line       rel PC    abs PC`,
a blank line, then each line as `%-15.15s %-31s%6s      %08X  %08X`
(module name cut to 15 characters, `TB70_LONG_MODUL`; the line number
right-aligned in 6, blank without one; 77 characters).

**The "improperly handled" dump** (EXE$EXCMSG): the message, a blank
line, `  Improperly handled condition, image exit forced.`, a blank
line, then (`\t` is a tab):

    \tSignal arguments\t      Stack contents
    (blank)
    \tNumber = 00000005\t\t 00000000
    \tName   = 0000000C\t\t 2FFC0000
    \t\t 00000000\t\t 7FED5398
    ...
    \t\t\t\t\t 7FED5384
    (blank)
    \tRegister dump
    (blank)
    \tR0 = 10101010  R1 = 11111111  R2 = 22222222  R3 = 33333333
    \tR4 = ...       R5 = ...  R6 = ...  R7 = ...
    \tR8 = ...  R9 = ...  R10= ...  R11= ...
    \tAP = ...  FP = ...  SP = ...  PC = ...
    \tPSL= 03C00008
    (blank)

The signal array (its count and every longword) beside ten longwords
of the stack. R0 to R11, PC, and PSL are the program's at the
exception (part A's patterns; the signal array's PC and PSL). Round 2's
part S (three longwords pushed) showed the rest:

- **SP** is the program's SP at the exception, and the stack shown is
  the ten longwords from it (53535353 first). For a condition from
  LIB$SIGNAL or LIB$STOP, it's the SP its caller will have after the
  call returns (round 2's O and Q: the caller's frame first).
- **FP and AP** aren't the program's: in all seven dumps FP is
  SP − (68 + 4n), n the signal array's count (SP − 7C for 5, SP − 74
  for 3), and AP is FP+40: VMS's own frames below the condition's
  arrays. govax shows the same values.

**SYS$OUTPUT and SYS$ERROR apart.** Part A's message and dump went to
both files, with and without traceback. Part S, whose program had
already opened SYS$OUTPUT (LIB$PUT_OUTPUT, an RMS `$CREATE` of the
file the logical name names), had its messages and dumps written to
SYS$ERROR only, and its success message (S1) to neither: each file
holds one version. The reading: the catch-all writes a success message
to SYS$OUTPUT only, the others to SYS$ERROR and to SYS$OUTPUT if
different, and an open of SYS$OUTPUT that fails (the program holds the
file) is passed over. Round 2 confirmed it: with the program writing
nothing (N), SYS$OUTPUT got the success message, and both files the
informational and warning messages and the dump; after a line (M),
SYS$OUTPUT held only that line and SYS$ERROR the rest.

**Side findings settled.** 1 is confirmed. 4 is confirmed: the
program's last-chance handler was never called (S18, V); the catch-all
is a frame's. 6: the catch-all honors STS$M_INHIB_MSG (S5). 3: VMS
reports `%SYSTEM-F-INTOVF, arithmetic trap, integer overflow at
PC=00000C73, PSL=03C0002A` and `%SYSTEM-F-INTDIV, arithmetic trap,
integer divide by zero at PC=00000C7F, PSL=03C00002`. Also: OPCDEC's
and ROPRAND's messages say `PSL=`, where govax's say `PS=`.

## Part B — decisions and questions

### Decisions (accepted by the author, 2026-10-21)

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

### Questions for the author (answered 2026-10-21)

1. Decision 1's driver frame, or traceback in Go at the catch-all? The
   driver frame (accepted).
2. Should the oracle run VMS's own images (`vax/*.exe`), as Phases 66
   and 67's do, with LIBRTL's addresses masked? Yes.

### Revised by the probe (2026-10-21)

- Decision 1's handler takes W, E, and F; it resignals S and I; and the
  catch-all, the next frame's (DCL's), is found before the last-chance
  vector, which only a stack that can't be searched reaches.
- Decision 2: only the main image's debug symbol table is used.
- Decision 4: the "improperly handled" dump is the catch-all's, for a
  severe condition round 2's rule picks, in either kind of image (an
  exit handler's exception in a traceback image gets it).
- Decision 3: a success message to SYS$OUTPUT only; the rest to
  SYS$ERROR, and to SYS$OUTPUT if it's a different file the program
  doesn't hold open.

### The probe's cases (subtask 1; `testdata/tb70`, round 1)

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

Round 2 (`tb70b.mar`, `tb70b.com`; parts by `TB70B_PART`), linked with
traceback and `/NOTRACEBACK`:

| Part | Case | What it settles |
| --- | --- | --- |
| R | warnings returning to routine+3FF and routine+400 | rule 1's limit |
| O | LIB$SIGNAL of SS$_OPCDEC, no arguments (a signal array of 3) | the "improperly handled" dump's rule: the condition |
| P | LIB$SIGNAL of SS$_BADPARAM with two longwords after it (5) | ... or the signal array's size |
| Q | LIB$STOP of SS$_ACCVIO | ... and LIB$STOP |
| S | access violation with three longwords pushed | where the dump's stack starts, and its SP |
| N, M | S, I, and W conditions with SYS$OUTPUT and SYS$ERROR apart, without and with a line written to SYS$OUTPUT first | where the catch-all writes |

## Part C — subtasks

1. **The survey and the probe** (this doc's Part A, `testdata/tb70`).
2. **The probe's VMS run**: Part A's answers, and Part B revised by
   them; round 2 for what round 1 left open.
3. **The nested-signal search** (finding 1) and stack recovery at
   rundown (finding 2).
4. **Arithmetic trap conditions** (finding 3).
5. **The image startup frame and the search's order** (Decision 1,
   finding 4).
6. **The traceback dump**: frames, names, rel PC, lines, the layout.
7. **EXE$EXCMSG's dump** for `/NOTRACEBACK`.
8. **Output to SYS$OUTPUT and SYS$ERROR** (Decision 3), and RUN's
   `DEFINE/USER_MODE SYS$OUTPUT file` (finding 5).
9. **The AST exit in S0** (added by subtask 6): an AST routine returns
   to EXE$ASTRET, system code in S0 (Internals book, 7.3.1 and 7.3.3),
   where govax's returns to the SYS$CLRAST P1-vector stub; VMS's dump
   names that PC by rule 1 (`8192B447`, S12 and T), govax's has blank
   columns.
10. **The oracle**: `TestTB70Oracle` runs `tb70.com`'s parts on govax
    and matches VMS's log.
11. **The close-out**: the manual (`docs/MANUAL.md`), CLAUDE.md,
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

### 2026-10-21 — Plan accepted; subtask 2, the probe's first round

The author accepted Part B's decisions and returned the probe's VAX
run (`testdata/tb70/vax`: the log, the four images, their maps, and the
objects). Its answers are Part A's "The probe's answers": which
conditions get which dump, the frames shown (no depth limit; a
handler's condition shown with the one it was handling), the rule that
names a PC (a 400-byte routine limit, compared signed, then the psect,
then nothing; shareable images unnamed), line numbers (the PC for a
fault, the address before it otherwise), both dumps' layouts, and where
the output goes when SYS$OUTPUT and SYS$ERROR differ. It confirmed side
findings 1 and 4, settled 6 (the inhibit bit is honored), and gave
VMS's messages for 3; it also showed that OPCDEC's and ROPRAND's
messages say `PSL=` where govax's say `PS=`. Part B is revised to
match.

Round 2 (`tb70b.mar`, `tb70b.com`, `exchange2.cmd`, `copyout2.cmd`)
asks what round 1 left open: the routine limit's exact value, which
conditions get the "improperly handled" dump, where its stack starts
and what its SP is, and where the catch-all writes with SYS$OUTPUT and
SYS$ERROR apart. It assembles, links, and runs on govax.

### 2026-10-21 — Subtask 2: the probe's second round

The author returned round 2 (`vax/tb70b.log`, its images, maps, and
object). It settled every open question: the routine limit is 400
(+3FF in the routine, +400 not); the "improperly handled" dump follows
exception conditions whichever way they're raised; its stack starts at
the program's SP, which it shows, and its FP and AP are VMS's own
(SP − (68 + 4n) for a signal array of n, and FP+40); a success message
goes to SYS$OUTPUT only, the rest to SYS$ERROR and to a different
SYS$OUTPUT the program doesn't hold. Part A and Part B record the
answers. Subtask 2 is done; the probe has no more questions.

### 2026-10-21 — Subtask 3: the nested-signal search, and the stack after a blown run

**Finding 1.** The frame walk (`corevms/condition.go`'s `nextHandler`)
now does what the Internals book's 4.3.3, "Multiply Active Signals",
describes: a frame whose saved PC is SYS$SRCHANDLER's XFC is a
condition handler's (the dispatcher called it), so after it is searched
the walk goes on not to its saved FP (the frame the earlier condition
happened in) but to the caller of the earlier condition's establisher
(`pastSearchedFrames`). As in the book, the earlier dispatch's arrays
are found from the handler's frame alone: the SP before its call, from
the frame's register save mask and alignment bits, is the handler's
argument list, whose mechanism array names the establisher. Depths go
on counting one a frame. VMS's own TB70 on govax now calls HAV once
(part H, no loop) and INHANDH once (S11), with VMS's messages; the
dumps are subtasks 6 and 7. `TestSrchandler_multiplyActive`.

**Finding 2** had three causes, all fixed:

1. With paging off, an exception whose arrays the user stack can't take
   fell back to the console's report and halted the machine. The book's
   16.2.1.3 rule (the last-chance exit: the image ends with the
   condition, STS$M_INHIB_MSG, and no message, as `testdata/pg67`'s VMS
   run showed) applied only in created space; `noStackToReport` now
   applies it in both modes. `TestDispatchException_noUserStack`.
2. Image exit (`cpu/exit.go`'s `exitImage`) walks the frame chain to
   the console's frame, and gave up after 100,000 frames: a runaway
   recursion leaves about 290,000 in the default 10 MB stack. Only steps
   that don't go up the stack count toward the limit now (a climbing
   chain can't loop). With paging on, the walk paged in ~20,000 stack
   pages in one step, emptying the free list before the reclaimer could
   run (`%GOVAX-F-MEMEXHAUST`); each frame is now an instruction
   boundary for the pager (`vm.Memory.BeginInstruction`), as the walk
   keeps nothing it translated. `TestImageExitLongChain`,
   `TestImageExitLoopingChain`.
3. An image that never exits (a HALT, Ctrl/C, a limit) left SP where it
   stopped, so the next RUN started there. RUN records process 1's SP
   and mode when it calls the image (`Machine.imageSP`, `imageMode`),
   and `EndAbandonedImage` puts it back before the rundown, as SP or the
   mode's stack pointer register (`restoreImageStack`); RUN now brings
   the CPU back to process 1 before activating. Checked with a program
   that pushes 1,000 longwords and halts in kernel mode: the next image
   starts at the same SP (it started 4,080 bytes lower before).
   `TestEndAbandonedImage_restoresStack`.

A runaway recursion now ends with `$STATUS` `%X1000000C` and no
message, in both paging modes, and the next image runs normally.

### 2026-10-21 — Subtask 4: the arithmetic traps

**Finding 3** wasn't the dispatcher's: its table of arithmetic type
codes was right, but no integer instruction ever trapped. Parts I and Z
ran on, and the images ended with R0 as their status (80000000 after the
ADDL2, NOMSG; 0A, the dividend DIVL2 left, a bare ACCVIO). The
architecture manual (8.4.6, 8.5.1, appendix E.1) has the rules, now
govax's (`internal/cpu/inttrap.go`):

- An instruction whose V reports an integer overflow (ADD, SUB, MUL,
  DIV, ADWC, SBWC, INC, DEC, MNEG, ADAWI, ASHL, ASHQ, CVTWB, CVTLB,
  CVTLW, ACB, AOB, SOB, EDIV) is marked in the instruction table
  (`Instruction.TrapsOnOverflow`), and `Step` takes the integer overflow
  trap (type 1) after it when V and PSL<IV> are both set: after its
  stores, and after its branch, so the PC saved is the next
  instruction's. Other instructions' V (REMQUE's empty queue, say)
  doesn't trap.
- An integer divide by zero (DIV, EDIV) always traps, type 2, IV or not,
  after storing the dividend and setting V: VMS's PSL for part Z,
  03C00002, has IV clear.
- INDEX's subscript range was a fault that stored nothing; it's a trap
  (E.1.7): the index stored and the condition codes set, then type 7
  with the next instruction's PC.
- The exception conditions' messages take the VAX's form,
  `PC=!XL, PSL=!XL` (`vmsdef`'s `vaxMessages`), as ACCVIO's already did:
  OPCDEC, ROPRAND, INTOVF, and INTDIV are confirmed by the probe; the
  other exception conditions follow them (unconfirmed).

VMS's TB70 and TB70NT on govax now print part I's
`%SYSTEM-F-INTOVF, arithmetic trap, integer overflow at PC=00000C73,
PSL=03C0002A` and part Z's INTDIV at C7F, PSL 03C00002, with `$STATUS`
`%X1000047C` and `%X10000484`, and parts O and R VMS's OPCDEC and
ROPRAND lines: all as in `vax/tb70.log`. `inttrap_test.go`'s tests
replace the three that held the old behavior. docs/DEVIATIONS.md has
the entry ("[Phase 70] Integer overflow and divide-by-zero never
trapped").

### 2026-10-21 — Subtask 5: the image startup frame, and the search's order

**Decision 1.** The traceback handler is `corevms/traceback.go`'s
EXE$TRACEBACK, a shim (code 3000, outside LIBRTL's ranges) whose stub
`EnsureShims` writes with the others (`corevms.CRTLLinks`, library
EVAX). RUN's IMAGE$INIT driver (`machine/run.go`'s `buildDriver`, so
also a `$CREPRC` process's and a subprocess's image) stores the stub's
address in its own frame, `MOVL I^#handler, (FP)` after its entry
mask, when the image's first transfer address is SYS$IMGSTA's: the
image was linked with traceback. The search then finds it after every
frame of the program's own, as VMS's does: in VMS's TB70 on govax,
part V's access violation reaches it at depth 4 (AV2, AV1, PART_V,
TB70, the driver), and TB70NT's goes to the catch-all.

The handler, by the Internals book's 4.5.1 and 21.1.3.3: a warning,
error, or severe condition (as the signal array has it now, so a
handler's change counts) gets its message, and is continued (W, E) or
ends the image by a tail call of `$EXIT` with the condition marked
STS$M_INHIB_MSG (F), the dispatch removed; success, informational, and
SS$_DEBUG are resignaled to the catch-all. The dump goes where the
message ends (subtask 6). It offers the condition to the debugger as
the catch-all does (`reportCondition`, now shared: the message once,
then `OnUnhandled`, with the same ErrWait rerun). A condition raised
while the image's exit handlers run is resignaled: on VMS image rundown
has removed the startup frame by then (part X); `Process.exiting` is
set by `$EXIT` and cleared at rundown (`cancelUserExitHandlers`).

**Finding 4.** The frame walk's end (frame 0 or the console's sentinel)
is now the catch-all's (`stageCatchAll`); the last-chance vector is
searched only when a frame can't be read, as 4.5.2 says. VMS's TB70's
S18 and V no longer call the program's last-chance handler.
`testdata/asm/exception_vectors.asm`, which relied on the old order,
now points FP at an address no page backs before its access violation,
so its last-chance vector is reached the VMS way (its golden listing
regenerated).

**Finding 6.** The catch-all, and the traceback handler, show no
message for a condition marked STS$M_INHIB_MSG (S5; TB70NT's S5 now
prints nothing, as VMS's did).

The debugger's step.dlg session differs from VMS's at two more
commands (#40, #41) under the listed reason: its break on the
unhandled warning now happens inside the traceback handler, so
stepping out runs one more RET (the stub's) through the paused
dispatcher. Tests: `traceback_test.go` (each severity, SS$_DEBUG, the
exiting image, the inhibit bit), `TestSrchandler_catchAllBeforeLastChance`,
`TestSrchandler_catchAllInhibited`, `TestSrchandler_vectors` (the last
chance reached past an unreadable frame).

### 2026-10-21 — Subtask 6: the traceback dump

**The dump** (Decision 2) is `corevms/traceback.go`'s `tracebackDump`,
which the traceback handler writes after the condition's message: the
`%TRACE` line with the condition's severity as the signal array has it
now, the heading, a blank line, then a line per PC in VMS's layout. The
PCs are the condition's, then each call frame's saved PC from the frame
the condition happened in (`startFP`: for LIB$SIGNAL and LIB$STOP their
caller's), stopping at the frame whose caller is the image startup's
(the frame the handler was called for, the mechanism array's), so the
main routine's line is the last. A frame whose saved PC is
SYS$SRCHANDLER's is a handler's: the dispatch that called it is found
by the handler's argument list (`dispatchForHandler`, through
`handlerArgList`, now shared with `pastSearchedFrames`), and its
condition is shown between the "-----" lines, then its PC and its
frames. There's no depth limit, as on VMS; a chain that doesn't climb
the stack ends the dump.

**Naming a PC** is `internal/dbgsym`'s `Program.TraceName`, Part A's
three rules from the main image's debug symbol table (`ICB.Debug`),
reached through `corevms.System.TraceName`, which the machine installs
(`ImageState(env).ICBList[0]`). The line is the PC's own for a fault
and the byte before it otherwise, and VMS decides which **by the
condition, not by how it was raised**: TB70D's part C, SS$_ACCVIO from
LIB$SIGNAL, has the line of the instruction after the call (472), so a
condition of an exception that's a fault (`faultConditions`: ACCVIO,
OPCDEC, OPCCUS, ROPRAND, RADRMOD, the floating faults) uses the PC's
line, wherever it came from (ACCVIO, OPCDEC, ROPRAND confirmed).

**The AST frame** needed nothing special: VMS's line after an AST
routine's is the saved PC of the frame the AST interrupted (S12: 743,
PART_S's return from the service it was waiting in), as the frame chain
gives it. The Internals book's 7.3 describes AST delivery as govax does
it (a CALLG with the six-longword argument list, the frame's saved FP
the interrupted one), but for the return address: VMS's EXE$ASTRET is
in S0, govax's the SYS$CLRAST stub in P1, so VMS's dump names that PC
by rule 1 with a "negative" rel PC, and govax's has blank columns. That
is subtask 9 (Part C, added).

**The debugger**: under the VMS debugger an unhandled condition gets
its message and the debugger's break, and no dump, even when the
program goes on (`testdata/dbgcmd`'s except.dlg: GO after the break
exits normally). `reportCondition` offers the condition to the debugger
before the dump, and writes the dump only when the debugger doesn't
take it; every debugger oracle session matches as before.

`TestTB70Traceback` (`internal/console/tb70_test.go`) runs every part of
VMS's TB70 and TB70D on govax and matches VMS's log, dumps, line
numbers, and `$STATUS`, but for the PCs outside the image, which govax
places elsewhere (LIBRTL's shims in S0, TB70SHR after the image, the AST
exit; `tb70Masked`), and three parts (`tb70DumpDifferences`): L
(LIB$MOVC3 is a shim, so its fault is reported at its caller), M
(TB70SHR's PC in the message), and X (subtask 7's dump). `TestTraceName`
checks the rules on VMS's images, round 2's +3FF and +400 among them;
`traceback_test.go` the walk, the layout, the inhibit bit, and the
debugger's case. Two LIBRTL tests (`TestLibrtlMissingRoutine`,
`TestLibrtlMissingJSBEntry`), whose images are linked with traceback,
now expect the dump. The manual's example 5 shows the dump (run on
govax), and section 9 describes it.

### 2026-10-21 — Subtask 7: EXE$EXCMSG's dump

**The dump** (Decision 4, as revised) is `corevms/excmsg.go`'s
`improperlyHandledDump`, which the catch-all writes after the message
when it ends the image (a severe condition, or any LIB$STOP) for a
condition its exception dispatcher raises (`exceptionConditions`:
`exceptionCondition`'s, by condition ID), however it was raised: so
TB70NT's hardware exceptions, part C's SS$_ACCVIO by LIB$SIGNAL, and
round 2's SS$_OPCDEC by LIB$SIGNAL and SS$_ACCVIO by LIB$STOP get it;
S20's SS$_BADPARAM and every other LIB$STOP don't. It goes through
`reportCondition`'s `after`, so under the debugger there's no dump, as
for traceback's. Part X, an exit handler's access violation, gets it in
TB70 and TB70D too, the traceback handler having resignaled (subtask 5).

The layout is Part A's, tabs and all: the signal array (count, "Name",
the rest) beside ten longwords of the stack, as many rows as the longer
column (a signal array longer than ten longwords goes on alone, and an
unreadable stack longword ends its column: neither confirmed); then R0
and R1 from the mechanism array, R2 to R11 the CPU's (the program's:
every handler has returned), AP, FP, SP, PC, and PSL. SP is the
program's as the condition leaves it, a new `conditionDispatch` field
(`conditionSP`): the SP at the exception, or for LIB$SIGNAL and
LIB$STOP their caller's once the call returns (`spAfterReturn`: the
stub frame's argument list, a CALLS's arguments popped; a JSB entry's
return address popped, unconfirmed). FP is SP − (68 + 4n) and AP FP+40,
VMS's own frames, as round 2 found. PC and PSL are the signal array's.

**A CPU bug** the dump's stack showed (part I: the frame's saved PSW
20000020 where VMS's is 20000000): CALLS and CALLG saved the PSW after
setting the entry mask's trap enables, so a frame held the callee's IV
and DV, and RET gave them to the caller. The manual's order is clear
(condition codes cleared, PSW pushed, then the enables set);
`buildCallFrame` now saves first. docs/DEVIATIONS.md has the entry
("[Phase 70, resolved] CALLS and CALLG saved the callee's trap
enables"); `TestEmulRetRestoresFullPSWFromFrame`, which held the old
behavior, now checks the caller's IV coming back.

`TestTB70ImproperlyHandled` (`internal/console/tb70_test.go`) runs every
part of VMS's TB70NT, round 2's TB70BNT O, P, Q, and S, and part X of
TB70, TB70D, and TB70NT, and matches VMS's output and dumps line for
line (`excmsgMatch`): the layout, the signal arrays, PCs, and PSLs
exactly; SP, FP, AP, and the stack's addresses once govax's stack is
moved to VMS's (D53E8 bytes higher, D5400 above the main routine's
argument list: VMS's image startup passes the main routine six
arguments, govax's driver none, a difference left as it is); registers
and stack longwords the program set. Not compared: values the program
never set (VMS's are its activator's and DCL's, in its own P1 and S0
space), the stack from the main routine's argument list up, R0 at a
LIB$SIGNAL or LIB$STOP (govax's stub has replaced the caller's R0 by
the time the shim runs: VMS shows the caller's), TB70B's never-set R2,
and the registers and stack of four parts whose frames come from code
that isn't VMS's (`excmsgLoose`: H, the dispatcher's; T, the AST
delivery's; N, TB70SHR's place; X, image rundown's). `excmsg_test.go`
checks the layout, which conditions get the dump by LIB$SIGNAL and
LIB$STOP, the SP after a CALLS, and a long signal array. The manual's
example 5 shows the dump (run on govax).
