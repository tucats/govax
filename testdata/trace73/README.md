# The Phase 73 probe: the trace trap

These fixtures are `docs/PHASE-73 - trace trap.md`'s subtask 4. They ask
VMS how a program that traces itself is traced: it sets PSL<T>, and its
condition handler takes the `SS$_TBIT` that follows each instruction.
They answer the phase's open questions (Part B):

- where each trace comes, and with what PC and PSL, through ordinary
  code, arithmetic traps, a fault, CALLS and RET (and a routine that
  sets T in its own frame), JSB and RSB, and REI with each combination
  of T and TP;
- the traces around a system service call and a LIBRTL call (Question
  6, Decision 6: govax's services are shims, so its traces there
  differ, and the difference is documented rather than imitated);
- whether an AST routine runs traced (Question 3, Decision 7);
- whether the handlers LIB$SIGNAL calls run traced, and where their
  traces go (Question 4, Decision 8);
- what a handler can change in the signal array's PC and PSL before it
  continues (the PC; TP; a condition code; T);
- the signal array's PSL and what an unhandled `SS$_TBIT` does
  (Question 5): its message, the traceback, the "improperly handled"
  dump without traceback, `$STATUS`;
- the debugger beside a traced program (Question 2, Decision 5): whose
  traces the debugger's STEP and GO take, what SET BREAK/EXCEPTION shows
  for `SS$_TBIT`, a breakpoint in the trace handler, DEPOSIT to the PSL.

| File | What it holds |
| ---- | ------------- |
| `trace73.mar` | The program: 12 parts, picked by the logical name `TRACE73_PART` (the source's opening comment lists them) |
| `trace73.com` | Builds the program three ways (TRACE73 with traceback, TRACE73NT `/NOTRACEBACK`, TRACE73D `/DEBUG`), runs every part, part J again without traceback and with `/DEBUG`'s lines, then the ten debugger sessions |
| `tr*.dbg` | The debugger sessions' commands, each run on one part (`trace73.com`'s `CALL DEBUG` lines name it); each file's opening comment says what it asks |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results into `vax/` |
| `trace73b.mar`, `trace73b.com` | Round 2: what round 1's part G left open (the source's opening comment lists its parts, picked by `TRACE73B_PART`) |
| `exchange2.cmd`, `copyout2.cmd` | Add round 2 to the exchange volume, and copy its results into `vax/` |

## How the program works

Each part writes a line saying what it is, establishes `TRACER` as its
handler, and sets T. `TRACER` keeps a record of every condition it's
called for, and so does `OUTER`, the main routine's handler, which gets
what `TRACER`'s frame can't (a trace taken while `TRACER` itself runs,
whose search passes over the frames already searched). Each record is
who was called (`T` or `O`), the condition, the signal array's PC and
PSL, and the PSL the handler ran with. After the part the main routine
prints

    A ordinary code
      traces: tracer 10, outer 0; records 10
      AST calls 0, AST PSL 00000000; BEQL fell through 0
      T 00000464 CODE+03E0 PSL=03C00010 handler=03C00000
      ...

A PC in the program is `CODE+offset` from the CODE psect's start (a
listing of the source from govax's MACRO, which lays the code out as
VMS's does, gives each label's offset); one within 1F bytes past a system
service or LIBRTL entry point the program calls is its name plus an
offset (`SYS$GETTIM+0002`); any other is its region and address
(`P1 7FFEE118`), which differs between systems. `TRACER` continues
`SS$_TBIT` and the other conditions the parts raise; each handler stops
the tracing (clears T in the signal array's PSL) after 300 traces, so a
part that traces into a long stretch of VMS's code still ends, and at
most 600 records are kept. Part J has no handler at all, so its
`SS$_TBIT` reaches the catch-all.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/trace73/exchange.cmd

   This makes `testdata/disks/trace73-exchange.dsk` (RD51 size, label
   TRACE73, gitignored). The scripts mount it as DUA2, since govax's
   own configuration may have DUA1.
2. Attach it to the simh VAX, mount it, set its `[000000]` as the
   default directory, and run:

       @TRACE73/OUTPUT=TRACE73.LOG

   Everything goes to `TRACE73.LOG`, and each debugger session also to
   its own `.DLG`. Part J's message and dump also reach the terminal
   (SYS$ERROR). Nothing should wait for input. If a run hangs or loops,
   a Ctrl/Y and `STOP` ends it, and the log up to there is still useful;
   `@TRACE73/OUTPUT=TRACE73B.LOG "B,C"` reruns some parts (the debugger
   sessions run again too).
3. With simh paused (or the disk detached), copy the results back, from
   the repository root:

       mkdir -p testdata/trace73/vax
       govax console < testdata/trace73/copyout.cmd

   That copies the log, maps, and sessions as text, the images byte for
   byte (so govax can run VMS's own images and the PCs match), and the
   objects in the host variable-length record layout.

The program calls no system macro but `$LNMDEF`, `$TRNLNM_S`, `$FAO_S`,
and `$DCLAST_S`, and nothing copied back is a DIGITAL file, so the
volume needs no erasing afterward.

## Round 2

Round 1's part G (LIB$SIGNAL from traced code) ended the image before
it could report. Round 2 reports from an exit handler, so what was
recorded before the image ended is seen. On the same volume:

1. From the repository root: `govax console < testdata/trace73/exchange2.cmd`
2. On the VAX, in the volume's `[000000]`: `@TRACE73B/OUTPUT=TRACE73B.LOG`
3. From the repository root: `govax console < testdata/trace73/copyout2.cmd`

## The runs

Round 1: 2026-10-25, OpenVMS V7.1 on simh's VAX 8600, the volume on
DUA1. MACRO and LINK had no messages; every part and session ran; part
G ended its image early. Round 2: the same day, on the same system and
volume; every part ran (G and M end their images, as VMS does).
`docs/PHASE-73 - trace trap.md`'s progress log has the answers.

`TestTrace73Oracle` (`internal/console/trace73oracle_test.go`) runs
VMS's images part by part on govax and holds the reports and `$STATUS`
to VMS's logs; `TestDebuggerSessionOracle` (`internal/debugger`)
replays the sessions. Once govax reads `DBG$INPUT` (the phase's
subtask 5), `trace73.com` can run on govax whole.

## govax's dry run

govax ran every part before the VAX run (2026-10-25); the progress log
records its answers then, beside VMS's.
