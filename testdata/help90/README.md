# The Phase 90 probe: VMS help

These fixtures are `docs/PHASE-90 - VMS help.md`'s probe (subtask 2). They
ask VMS 7.1 how LIBRARY/HELP builds help libraries, how HELP finds and shows
topics and prompts for more, how the debugger's HELP finds its library, and
what the LBR$ routines do. Every question is asked of **help modules written
for the probe**, whose text names its own topic ("Text of ALPHA
Parameters."), so a log shows which text HELP chose. None of it is VMS's.

`gen.go` writes everything here but this README (round 3's and 3b's files
are in their own sections below):

    go run testdata/help90/gen.go

| File | What it holds |
| ---- | ------------- |
| `h90a1.hlp`, `h90a2.hlp` | Library H90A's modules: ALPHA (parameters, a Qualifiers topic of `/` lines, a numbered `/DIRECT` key, a `/` line outside Qualifiers), ALPHABET and ALPS (abbreviations), HELP; DEEP (to level 9), WIDE (keys of 1 to 32 characters), MANY (40 keys), ORDER (out of order, mixed case), ODD (punctuation and blanks), SAME (repeated keys), TEXTY (empty, tabbed, long, and digit-led lines), NOSUB, EMPTY, MixedCase, BETA_GAMMA, QUALS (`/` lines under other topics, an indented `/`, `/[NO]FLAG`, a numbered key after `/` lines) |
| `alpha2.hlp` | A replacement ALPHA, for LIBRARY/REPLACE |
| `h90b.hlp`, `h90c.hlp`, `h90g.hlp`, `h90s.hlp` | The user libraries' modules (process, group, and system tables) |
| `h90d.hlp` | A debugger library of the probe's own: one DEBUG module, as VMS's DBG$HELP.HLB is, and an EXTRA module |
| `badtext.hlp`, `skip.hlp`, `noblank.hlp`, `toolong.hlp`, `lower.hlp`, `digits.hlp`, `zero.hlp` | Malformed modules: text before the first key, a skipped level, key lines with no blank, a bare digit or a tab, a level-1 key of 40 characters, a lowercase level-1 key, and (since round 1b) lines starting with `10` and `0` |
| `t1.txt`, `t2.txt`, `h90m.mar` | A text library's and a macro library's modules, for the LBR$ routines |
| `resp.txt` | HELP's answers from a file (part D) |
| `help90.com` | Round 1, parts A (LIBRARY), B (HELP's matching and layout, 126 requests), B2 (output), C (user libraries), E (`LBR90`), then `HELP90D.COM` (part D), and a save set of the libraries and listings |
| `help90d.com` | Part D: HELP's prompts answered by a procedure's data lines, and by a file |
| `help90f.com` | Part F (since round 1b): the debugger's HELP, with DBG$HELP naming the probe's library, then without it. Its log holds VMS's text; `DBGH.DLG`, the first session's, doesn't |
| `deeper.hlp`, `help90c.com` | Round 1c: DEEPER, a chain of levels to 9 (round 1b's DEEP was nested wrongly), its cases, and the libraries and image round 2's terminal session uses |
| `help90v.com` | Part G: HELP on VMS's own EXCHANGE, SUBMIT, and ANALYZE modules (round 1 only). Its output holds VMS's text |
| `vax/round1/` | Round 1's VAX run, checked: `help90.log`, `h90.bck`, `dbgh.dlg`, and `help90-term.log` |
| `vax/round2/` | Round 1c's `help90c.log` and round 2's `help90-term.log` (simh's console log, escape sequences kept, cut before simh's own help text), checked |
| `vax/round3/` | Round 3's second run: `help90w.log` and `h90w.bck`, every library it made |
| `vax/round3b/` | Round 3b's run, once made and checked: `help90x.log` and `h90x.bck` |
| `vax/round1b/` | Round 1b's, checked: `help90.log`, `h90.bck`, `dbgh.dlg` (`help90f.log` and `dbgv.dlg` are local) |
| `help90d.dbg`, `help90v.dbg`, `dbgt.mar` | The debugger sessions' commands, and the image they run |
| `lbr90.mar` | Part E: 79 cases calling the LBR$ routines on the probe's libraries: the definitions' values, INI_CONTROL, OPEN, GET_HEADER, LOOKUP_KEY, GET_RECORD (move and locate), FIND, GET_INDEX with matches, SET_INDEX, GET_HELP with its action routine's every call (flags, level, data), OUTPUT_HELP with output and input routines of its own (scripted answers to its prompts), the errors |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts for the exchange volume (round 3b's) |

`lbr90.mar` was rehearsed on govax against stand-in LBR$ routines (not
kept): it assembles, links, runs all 79 cases, and its handler reports a
fault in a routine and goes on to the next case.

## Keeping VMS's text out of the repository

The phase doc's Decision 9 is a deliberately conservative rule: nothing
that may hold VMS's own help text goes into the repository. Logs of HELP
showing our modules hold only our text and HELP's framing (its headings and
messages), and those are committed. The rest stays local:

- `copyout.cmd` copies **everything** to `testdata/disks/help90-vms/`,
  which git ignores. Nothing goes straight into `vax/`.
- `help90.log` and `h90.bck` move to `vax/` only after a check that every
  line of HELP's output in them is one of our modules' lines, a command, or
  HELP's framing. If HELP took any text from VMS's own libraries (its
  instructions for `HELP` with no topic could come from SYS$HELP's
  HELPLIB.HLB rather than the probe's library), that case is cut from the
  committed log and noted here.
- `help90v.log` (part G is VMS's own modules), `help90f.log`, and
  `dbgv.dlg` (the debugger's own library) never leave `testdata/disks/help90-vms/`. Their
  tests skip without them, as DUNGEON's do. `dbgh.dlg` moves to `vax/` only
  if the check shows the debugger read the probe's library through
  DBG$HELP.

## Round 1 (2026-11-07)

Run on VMS V7.1 (a VAX 8600 under simh). `help90.log`, `h90.bck`, and
`dbgh.dlg` passed the check and are in `vax/round1/`; `help90v.log` and
`dbgv.dlg` are local. `help90-term.log` is the start of round 2's terminal
session, transcribed from the console as pasted (the runs of blank lines as
they came, an OPCOM message left out).

Round 1 settled much of HELP's layout and matching (the phase doc's "What
the probe settled"), but four mistakes of the probe's own spoiled other
parts:

- **LIBRARY took every library for an object library.** `/LIST`,
  `/EXTRACT`, `/REPLACE`, and `/DELETE` without `/HELP` look for `.OLB`, so
  none of them ran.
- **`/DATA=REDUCE` is a command of its own:** with `/CREATE` it drew
  MAXPARM, so H90R was never made.
- **A line starting with a digit is a key line.** TEXTY's `10 apples`
  stopped LIBRARY/CREATE at ILLKEYLVL, so H90A has only the ten modules
  before TEXTY. Every case on TEXTY, NOSUB, EMPTY, MixedCase, BETA_GAMMA,
  and QUALS, in HELP and in LBR90, found nothing.
- **The debugger's HELP prompts**, reading the debugger's own input, so
  the first HELP took every later command as an answer to `DEBUG
  Subtopic?`.

Part D's `DEFINE/USER_MODE SYS$INPUT RESP.TXT` also failed (IVDEVNAM), and
`LBR$_NOHLPLIS` doesn't exist (round 1b found `LBR$_NOHLPLIBS` doesn't either).

## Round 1b

The same procedure with those fixed. `gen.go` now gives every LIBRARY
command its type, reduces H90R with `LIBRARY/DATA=REDUCE/HELP`, moves the
digit-led lines into DIGITS.HLP and ZERO.HLP, makes part F its own
procedure (`help90f.com`) whose sessions follow each HELP with empty lines
to leave its prompts, names RESP.TXT in full, and asks for
`LBR$_NOHLPLIBS`. Part G isn't run again. The exchange volume is a new
one, `help90b-exchange.dsk`, so round 1's stays as it was.

Run on VMS V7.1 2026-11-07, on DUA1 (the VAX had no RQ5). `help90.log`,
`h90.bck`, and `dbgh.dlg` passed the check and are in `vax/round1b/`;
`help90f.log` and `dbgv.dlg` are local. Every part ran. One mistake was
left: DEEP puts each level under the sibling before it (`2 L2`, `2 S2`,
`3 L3` makes L3 a child of S2), so every `DEEP L2 L3 ...` case, at the
terminal too, found nothing past L2. Round 1c has the chain DEEP was meant
to be.

The terminal session typed after round 1b went as far as row 11, where
the table was wrong: rows 9 and 10 ended HELP, and row 11 (`@H90DIR:H90B`)
belongs at a `Topic?` prompt, not at `$`. The table below says where each
row starts.

## Round 1c

`help90c.com` makes H90Y from `deeper.hlp` and runs DEEPER's cases,
and makes again what round 2 uses (H90A, H90B, H90D, and DBGT.EXE),
on a new exchange volume, `help90c-exchange.dsk`.

1. Build the exchange volume, from the repository root:

       govax console < testdata/help90/exchange.cmd

2. On the VAX, with the volume on any free unit (DUA1 below):

       $ MOUNT DUA1: HELP90
       $ SET DEFAULT DUA1:[000000]
       $ @HELP90C/OUTPUT=HELP90C.LOG

   then round 2 below, with the volume still mounted, then
   `DISMOUNT DUA1:`.

3. Back on the host, with the terminal log saved as
   `testdata/disks/help90-vms/round2/help90-term.log`:

       govax console < testdata/help90/copyout.cmd

## Round 2, at the terminal

HELP's prompts typed by hand, which a procedure can't show (the prompts
and the terminal's echo). Log the session on the host (simh's
`SET CONSOLE LOG=help90-term.log`, or the terminal program's log). The log
is checked like `help90.log` before it goes to `vax/`.

Before each display at the terminal, HELP wrote a run of empty lines (27
to 46 in round 1b's session). That looks like HELP clearing the screen
for a terminal it takes for a video terminal, so the session starts by
showing what VMS thinks the terminal is, and row 12 tries `/NOPAGE`.

    $ SHOW TERMINAL
    $ DEFINE H90DIR DUA1:[000000]
    $ HP := HELP/NOUSERLIBRARY/LIBRARY=H90DIR:H90A
    $ HY := HELP/NOUSERLIBRARY/LIBRARY=H90DIR:H90Y
    $ SET TERMINAL/WIDTH=80

Then type each row's lines in order. "Return" means a Return on an empty
line. **Each row starts at `$`**; if a row ends at a HELP prompt instead,
press Return until `$` comes back before the next row. Already known:
Return at `X Subtopic?` goes up one level, Return at `Topic?` ends HELP,
and after a topic with no subtopics the prompt stays at its parent.

| # | Type | What it shows |
| - | ---- | ------------- |
| 1 | `HP`, then `@H90DIR:H90B` | Another library's topics, and the prompt it gives |
| 2 | `BONLY` | A topic there, and its prompt |
| 3 | Return, Return, Return | Which library each Return goes back to, and when HELP ends |
| 4 | `HY DEEPER`, then `L2`, `L3`, `L4` | The prompt at each level (does it name the whole path?) |
| 5 | `S5` | A leaf: which prompt after it? |
| 6 | `?` | What `?` shows there |
| 7 | Return until `$` | One prompt per level on the way up |
| 8 | `HY DEEPER L2 L3` | Starting partway down: which prompt? Then Return until `$` |
| 9 | `HP ALPHA`, then Ctrl/Z | Ctrl/Z at a subtopic prompt: straight to `$`? |
| 10 | `HP NOSUCH` | "Sorry", and the prompt after it; Return until `$` |
| 11 | `HP ALPHA`, then `NOSUCH` | A subtopic that isn't there, and the prompt; Return until `$` |
| 12 | `HP/NOPAGE ALPHA`, then Return, Return | The same without paging: are the empty lines gone? |
| 13 | `HP/PAGE TEXTY` | /PAGE's empty lines with a short topic; Return until `$` |
| 14 | `SET TERMINAL/WIDTH=132`, `HP/NOPROMPT WIDE`, `SET TERMINAL/WIDTH=80` | The subtopic columns at 132 |
| 15 | `DEFINE DBG$HELP H90DIR:H90D`, `RUN/DEBUG DBGT`, then at `DBG>` `HELP SET`, `BREAK`, Return, Return, Return, `EXIT` | The debugger's prompts on the probe's library |
| 16 | `DEASSIGN DBG$HELP` | |

Round 1c and round 2 ran 2026-11-07. Both logs passed the check and are in
`vax/round2/`. `SHOW TERMINAL` explained round 1's empty lines: OPA0: is a
VT100 with a 52-line page, and HELP's `/PAGE` clears the screen before
each display. Round 3, the LBR$ writing routines, is subtask 11's.

## Round 3, the LBR$ writing routines

Prepared 2026-11-08 (subtask 11a). `exchange.cmd` and `copyout.cmd` are
now round 3's (rounds 1 to 1c's are in the repository's history), on a new
volume, `help90w-exchange.dsk`. Its files:

| File | What it holds |
| ---- | ------------- |
| `lbr90w.mar` | 36 cases calling the LBR$ routines to write libraries of every kind, each case on a library of its own (W1 to W16), most read back after: the definitions round 1 didn't print and every LBR$_ status's message; an empty text library with the manual's create options; modules written, keyed, replaced, and deleted (PUT_RECORD, PUT_END, INSERT_KEY, REPLACE_KEY, DELETE_KEY, DELETE_DATA, and what a new module takes of the space freed); keys' errors and edges (duplicates, case, bad and zero RFAs, two keys on one module, keys of 31 and 32 characters, an empty key, empty and over-long records); INSERT_KEY and DELETE_KEY inside GET_INDEX's and SEARCH's routines; writing a library opened to read, or not open; history records past LUHMAX and with LUHMAX 0 (PUT_HISTORY, GET_HISTORY); module headers with UHDMAX (SET_MODULE, read and updated, dumped in hex); FLUSH; 126 modules in a scrambled order and a 300-record module; a help library (with help casing, W10, and with text casing, W10T, beside LIBRARY's W10L), a macro library, an object library with its symbol index (H90O.OBJ's records), binary keys in a library of a user's type, the VMS V2 format, options all zero, create errors; a library opened twice to update; records with no PUT_END or key; LIBRARY's H90T changed by the routines; LIBRARY's shareable image library H90S read; the NAM block (open by NAM, a related name, the resultant name); and last, 17 control indexes |
| `help90w.com` | Part A makes what LBR90W reads (H90T, H90O.OBJ, H90OS.EXE and H90S.OLB); part B runs LBR90W; part C lists every library LIBRARY's way, with LIBRARY's own W2L, W10L, and W12L to compare with W2, W10, and W12, HELP on W10, and a LINK against W12; part D is what subtask 3 left of LIBRARY/HELP, each command followed by its `$STATUS` (DUPMODULE, ILLKEYLVL, KEYNAMLNG, NOMTCHFOU, RECTYP, a level-1 key of several words, a file whose first key is level 2, a bad key after good modules, `/EXTRACT` misses in a text library, `/DATA=REDUCE` and `/DATA=EXPAND`); then a save set of the libraries |
| `one.txt`, `two.txt`, `w10.hlp`, `h90o.mar`, `h90om.mar`, `multi.hlp`, `notfirst.hlp`, `goodbad.hlp` | The input files: W2L's and W10L's modules (the records LBR90W writes for W2 and W10), the object module, the program that links against W12, and part D's help files |

Everything in the run is our own text, so its log and save set are
committed once checked as before (`vax/round3/`).

`lbr90w.mar` was rehearsed on govax (not kept) with stand-in values for the
names govax's STARLET lacks (`$MHDDEF`, the `CRE$C_` values,
`LBR$C_MAXRECSIZ`, `LBR$C_FLUSHDATA`, `LBR$C_FLUSHALL`, `MHD$B_USRDAT`): the
procedure reaches its end, the program assembles, links, and runs all 36
cases (the writing routines NOTIMPL), and the handler goes on past each
failure. On VMS the cases that may wait: the second update OPEN of W2
retries for 30 seconds before RMS$_FLK.

**The first run (2026-11-08) hung.** Cases 1 to 21 ran; case 22, W10 (a
help library created with `CRE$C_HLPCASING`), wrote its first module and
then used CPU for minutes with no I/O in its first `LBR$INSERT_KEY`, and
was stopped with Ctrl/Y. Its log is local, superseded by the second run.
So since then `LBR90W n` runs case n alone (the procedure runs each in
turn, `LBR90W` with no number runs them all), under a timer of two
minutes whose AST writes `=t timed out after two minutes` and ends the
image; a hang costs one case, and the procedure goes on. W10T is W10 with
text casing, to tell whether the casing is the cause, and the program reads
LIBRARY's own W10L. The second run starts on a fresh exchange volume.

The second run (2026-11-08) ran all 36 cases; W13 (binary keys) ended in an
access violation and the double update OPEN of W2 at the timer. Its
`help90w.log` and `h90w.bck` passed the check (our modules' text, VMS's
messages and listings) and are in `vax/round3/`.

1. Build the exchange volume, from the repository root:

       govax console < testdata/help90/exchange.cmd

2. On the VAX, with the volume on any free unit (DUA1 below):

       $ MOUNT DUA1: HELP90
       $ SET DEFAULT DUA1:[000000]
       $ @HELP90W/OUTPUT=HELP90W.LOG
       $ SET DEFAULT SYS$LOGIN
       $ DISMOUNT DUA1:

3. Back on the host:

       govax console < testdata/help90/copyout.cmd

   which puts `help90w.log` and `h90w.bck` in
   `testdata/disks/help90-vms/round3/`. (Since round 3b the scripts are
   its own; round 3's are in the repository's history.)

## Round 3b, binary keys, preallocation, and opens

Prepared 2026-11-08 (subtask 11d), for what round 3 left to VMS.
`exchange.cmd` and `copyout.cmd` are now round 3b's, on a new volume,
`help90x-exchange.dsk`, which holds only its two files:

| File | What it holds |
| ---- | ------------- |
| `lbr90x.mar` | 17 cases on LBR90W's routines and data (gen.go's `assemble`), under a timer of three minutes. X1 to X5: binary keys (CRE$L_KEYLEN 0) in a text library and a user's with ENTALL 0, with W13's options, with W13's options and keys passed as descriptors, and with W15's (no index); each library is flushed after OPEN and after each key, so its header and blocks are on the disk if a call fails; keys 5, 1, 3, 256 and a duplicate 3, LOOKUP_KEY of 3 and of a missing 7, GET_INDEX, SEARCH, DELETE_KEY, REPLACE_KEY, then read back. X6 to X10: one module each, for the index blocks preallocated (keys of 31 with ENTALL 0 and 1; keys of 15 with 96, in V3 and V2; V2 keys of 31 with 8). X11 makes WX; X12 holds it open to update for 90 seconds; X13 and X14 open it to update and to read, timed (`elapsed`) with the time of day before and after (`now`), as X12 shows its own; X15 to X17 open it twice in one image (update then read, read then update, read twice), timed |
| `help90x.com` | MACRO and LINK of LBR90X, then cases 1 to 11; X12 spawned with `/NOWAIT/OUTPUT=HOLD1.LOG`, X13 15 seconds later, a wait for the holder to end, its log; the same for X14 (`HOLD2.LOG`); cases 15 to 17; `LIBRARY/LIST/FULL` of each library; a save set, `H90X.BCK` |

The manual says a second open of a library open for writing retries every
second for 30 seconds, then RMS$_FLK; X13's `elapsed` says whether VMS
7.1's does, from another process.

Rehearsed on govax (2026-11-08): the procedure ran to its end, all 17
cases. It showed a govax bug the run doesn't depend on: in quantum-clock
mode a procedure's WAIT, while a spawned subprocess waits, returns long
after its time (5 seconds took 33 emulated minutes), so X13 ran after X12
had closed WX.

1. Build the exchange volume, from the repository root:

       govax console < testdata/help90/exchange.cmd

2. On the VAX, with the volume on any free unit (DUA1 below):

       $ MOUNT DUA1: HELP90
       $ SET DEFAULT DUA1:[000000]
       $ @HELP90X/OUTPUT=HELP90X.LOG
       $ SET DEFAULT SYS$LOGIN
       $ DISMOUNT DUA1:

   The run takes about eight minutes, most of it the waits.

3. Back on the host:

       govax console < testdata/help90/copyout.cmd

   which puts `help90x.log` and `h90x.bck` in
   `testdata/disks/help90-vms/round3b/`. They're our own text and VMS's
   messages and listings; once checked they go in `vax/round3b/`.

## The tests

`internal/lbr`'s `TestHelp90NewLibraries` and the console's
`TestHelp90LibraryOracle` hold LIBRARY/HELP to part A (subtask 3).
`internal/help`'s `TestHelp90Oracle` holds HELP's lookup and layout to
part B of `vax/round1b/help90.log` and to `vax/round2/help90c.log`
(subtask 4), and `TestHelp90VMSModules` to part G's local log, with
VMS's HELPLIB.HLB copied beside it (the test's comment says how); it
skips without them. `TestHelp90Prompting` holds HELP's prompting to
part D and round 1c's chain, and `TestHelp90Terminal` to round 2's
terminal session (subtask 5). The console's `TestLBR90WOracle` runs round 3's
cases on govax, each as `LBR90W n`, and holds their output to
`vax/round3/help90w.log` and the libraries they made to `h90w.bck`'s, byte
for byte but for times (subtask 11b; the cases later parts of subtask 11
finish are listed in the test, with why); `internal/lbr`'s `TestUpdaterW9`
makes W9 again without the console.

## What the run settles

The phase doc's A.4 lists the questions. Its "What the probe settled"
section, written when the logs come back, records the answers.
