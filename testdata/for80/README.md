# The Phase 80 probe: FORRTL's formatted I/O

These fixtures are `docs/PHASE-80 - FORTRAN formatted IO.md`'s subtask 2. VAX
FORTRAN's User Manual isn't to hand, and the simh VAX has no compiler, so VMS's
own FORRTL.EXE is called from MACRO and its behavior becomes FTNRTL's requirement
(the author's ruling; only what it does is used, never its code). A later subtask
holds FTNRTL to `vax/for80-vax.txt`.

`gen.go` writes everything here but this README and the output files:

    go run testdata/for80/gen.go

| File | What it holds |
| ---- | ------------- |
| `for80.mar` | The probe: 859 cases by subject, then FOR's message texts by `$GETMSG` |
| `for80d.mar` | Prints `FOR80.DAT`'s records between bars, control bytes and trailing blanks included |
| `for80e.mar` | Ten conditions with no handler, to show VMS's own messages and dumps |
| `for80t.mar` | Carriage control and a prompt at a real terminal, run by hand |
| `for80r.com` | Runs `FOR80`, with the records its unit 5 cases read |
| `for80.com` | Builds and runs everything |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts for the exchange volume |
| `for80.for` | The output cases to unit 6 as FORTRAN statements, for govax (Decision 2), each ending with ERRSNS's five values, compared with VMS's `e=` (subtask 12) |
| `for80i.for`, `for80i.dat` | The formatted and list-directed input cases as FORTRAN READs from SYS$INPUT, and their records, each case's under its number, for govax; ERRSNS's values after each, as in `for80.for` |
| `for80e.for` | `for80e.mar`'s ten conditions as FORTRAN statements, for govax: `TestFor80FTNRTLMessages` holds FTNRTL's messages to run 3's (subtask 12) |
| `vax/for80-vax.txt` | Run 1 on VMS, as `FOR80D` printed it |
| `vax/for80p-vax.txt` | Run 2 on VMS: the same cases with unit 6 a process-permanent file |
| `vax/for80.log` | `FOR80.COM`'s log: run 3's messages, and the file's attributes |
| `vax/for80t.log` | The terminal session, as the author pasted it (see below) |
| `for80b.mar`, `for80br.com`, `for80b.com` | Round 2: 89 cases round 1 left open, the message table first |
| `exchangeb.cmd`, `copyoutb.cmd` | Round 2's exchange volume |
| `vax/for80b-vax.txt`, `vax/for80b.log` | Round 2 on VMS |

## How the program works

Everything goes through unit 6, so a case's records and the driver's lines are
one stream in order. In run 1 `FOR006` names a file, so each record keeps the
first byte a terminal or a log would take as carriage control. A case is

    #n  what it does                   the driver's title
    ... the records the case wrote
    =n r=x e=n x x u x                 FOR$IO_END's R0, and FOR$ERRSNS's five values
    =c cond n=x a=x x                  each condition, with the signal array's size and
                                       first two arguments (an address or PC shows as 0)
    =u                                 the handler unwound
     v0=x x   or   v0=|text|           what an input case stored; slots start as 55s

`r=DEADBEEF` means the statement never got to `FOR$IO_END`. The handler records a
condition, continues from one below severe (the statement goes on, as it does
after OUTCONERR), and unwinds from a severe one, so no case stops the program.

A format is passed as text: `FOR$WRITE_SO(unit, address)`, a NUL ending the text,
which is how a run-time format reaches FORRTL. So the syntax cases show the parser
a run-time format gets, and any format can be tried without knowing how a compiler
encodes one.

Input cases read an internal record, or SYS$INPUT through `FOR80R.COM`'s data:
each case's lines sit under a line holding its number, and the driver reads up to
that line first (`SYNC`). A `/` pad after each case's lines stops a case that reads
too far, so one case can't take the next one's data.

The namelist block was found by calling `FOR$WRITE_SN` with a guard block of bad
addresses, and seeing which one its access violation named; the phase doc has the
layout.

## The VAX run

Allow a minute or two; if a case hangs, Ctrl/Y and tell me which `#n` the log or
`FOR80.DAT` reached.

1. Build the exchange volume, from the repository root:

       govax console < testdata/for80/exchange.cmd

   This makes `testdata/disks/for80-exchange.dsk` (RD51 size, label FOR80,
   gitignored), on DUA2, since a startup procedure may mount DUA1.
2. Attach it to the simh VAX as a spare drive, DUA2 for example, log in as SYSTEM,
   and:

       $ MOUNT DUA2: FOR80
       $ SET DEFAULT DUA2:[000000]
       $ @FOR80/OUTPUT=FOR80.LOG

3. Then the terminal session, with simh's console log on (`SET CONSOLE
   LOG=for80t.log` at the `sim>` prompt) so that the lines fall as a terminal shows
   them. It prompts once: type `ABC` and Return.

       $ RUN FOR80T
       $ DISMOUNT DUA2:

   Save the log as `testdata/for80/vax/for80t.log`.
4. Copy the results back, from the repository root:

       govax console < testdata/for80/copyout.cmd

No licensed file goes on the volume, so it needs no erasing.

## The VAX run, round 2

Round 1 ran on 2026-10-31 (its findings are in the phase doc). It stopped at
case #853, where `FOR$IO_END` called twice ends the image, before the message
table. Round 2 asks what it left open, on a volume of its own:

    govax console < testdata/for80/exchangeb.cmd

makes `testdata/disks/for80b-exchange.dsk` (label FOR80B). On the VAX:

    $ MOUNT DUA2: FOR80B
    $ SET DEFAULT DUA2:[000000]
    $ @FOR80B/OUTPUT=FOR80B.LOG
    $ DISMOUNT DUA2:

then `govax console < testdata/for80/copyoutb.cmd`. No terminal part this time.
It ran on 2026-10-31, whole; the phase doc has its findings.

## Round 3

`for80c.mar` (133 cases, Phase 80's subtask 9) is what the first two rounds left
open of internal files, ENCODE, and DECODE: positions and overflow, how a record
ends, long records, list-directed output and input, and the count's limits. A case
on a file longer than 100 bytes shows its last 100, as unit 6 holds 133. It has run
on VMS's FORRTL.EXE over govax, not on the VAX: `TestFor80Round3` runs it and holds
it to `vax/for80c-forrtl.txt` (`FOR80C_WRITE=1` rewrites that). To run it on VMS,
`govax console < testdata/for80/exchangec.cmd`, then `@FOR80C/OUTPUT=FOR80C.LOG`.

`TestFor80InternalOracle` compiles `for80x.for`: the internal file, ENCODE, and
DECODE cases of rounds 1 and 3 as FORTRAN, round 3's numbered from 2000, each on a
place of the case's size, and holds its IOSTAT, its stored values, and the file's
bytes to those runs.

## On govax

`TestFor80Assembles` (`internal/console/for80_test.go`) checks that the programs,
both rounds', assemble. `TestFor80RunsOnFORRTL` links them against VMS's FORRTL.EXE from a
local-only volume, if there is one, and checks that the program reaches its end.
That is FORRTL's logic over govax's LIBRTL, so it isn't the requirement: VMS's run
is. Its namelist section is compared with VMS's, as a check on govax's `LIB$TPARSE`
(Phase 80's subtask 10; `testdata/tpa80`). `TestFor80Messages` runs FOR80E's parts the same way and
holds their messages and statuses to VMS's `vax/for80.log`.

`TestFor80Oracle` (`internal/console/for80oracle_test.go`, Phase 80's subtask 4)
holds FTNRTL to VMS's run: `for80.for` is round 1's output cases to unit 6, each
a run-time format in a character constant (so a format the compiler would refuse
reaches FTNRTL's parser, as it reached FORRTL's), between a `#n` line and a `=n`
line with its IOSTAT and, since subtask 12, ERRSNS's five values. The test runs it
with SYS$OUTPUT a file, so its records keep their first bytes as VMS's FOR006 file
did, and matches each case's records, IOSTAT, and ERRSNS with `vax/for80-vax.txt`'s
records and e= line. A case whose IOSTAT is 2,
NOTIMP, uses an edit FTNRTL doesn't have yet, and waits for its subtask.

`TestFor80InputOracle` (the same file, Phase 80's subtask 5) does the same for
input: `for80i.for` is round 1's formatted and list-directed input cases (`rd`, from an internal
record, and `rd5` and `rl5`, from SYS$INPUT; those with G-floating items come last, in
subroutines with `OPTIONS /G_FLOATING`, as `for80.for`'s do), each a READ from unit 5 with a run-time format or `*`, its
items in 16-byte slots of a COMMON block whose bytes are first set to 55s as
VMS's were. `for80i.dat` holds each case's records under a `#n` line, then three
`/` lines as `FOR80R.COM` had, and the test runs the program from a command
procedure with those records as its data lines, as `FOR80R.COM` did. Each case
prints its IOSTAT, ERRSNS's values, and what its slots hold, which must be VMS's
e= and `v` lines (an `rd` case's unit is 5 on govax, -5 on VMS).
An internal record read as a record of SYS$INPUT is the same to the format but
for what follows it, so the cases whose internal record ran out (e=24) aren't
compared.
