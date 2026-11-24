# Phase 113's probe round

`docs/planning/PHASE-113 - Fortran errata.md`'s subtask 12 (Part A's A.12): one simh
session that asks VMS 7.1 what the phase's earlier subtasks left unconfirmed, after their
work, so the round also checks it. This directory is the round's own volume; the session
also runs two older procedures again on their own volumes (below).

The files here are written by hand (no `gen.go`); `str113.mar` and `svc113.mar` come from
`testdata/str113` and `testdata/svc113`, whose `gen.go`s write them.

## What ERR113.COM asks

Each part is a section of the log, `==== NAME`:

| Section | The question | Where it came from |
| --- | --- | --- |
| STR113 | STR$ADD, MUL, DIVIDE, RECIP, ROUND, and ELEMENT_R8 (`testdata/str113`'s README), and STR$MATCH_WILD with `%`, which `testdata/str83`'s probe never asked (its label prefix replaced every `%`). The output is `STR113.TXT` | subtask 7; A.11.2 |
| SVC113 | `$GETUTC`, `$ASCUTC`, `$BINUTC`, `$NUMUTC`, `$TIMCON`, `$GETCHN`, `$GETDEV` (`testdata/svc113`'s README); `DEF_DIB.OBJ` is `$DIBDEF`'s values. The output is `SVC113.TXT` | subtask 8 |
| INI113, INI113NT | LIB$INITIALIZE (`ini113.mar`, `ini113s.mar`): does VMS run a shareable image's routines, and before the main image's? What arguments does each routine get (govax: the init-coroutine, then 0 for the CLI coroutine and image information), and the main program? Does the init-coroutine run the main program and return its status? The same linked `/NOTRACEBACK` | subtasks 6 and 10 |
| MAC113A | A register name in a data expression (`.LONG R1`, `.WORD R5`, `.ADDRESS R3`) | A.6.3; DEVIATIONS.md |
| MAC113B | `;` inside `<...>` in a macro call; what `.WARN`, `.ERROR`, and `.PRINT` show | A.6.6 |
| MAC113C | `MACRO/DEBUG A+B`: the listing's numbering of the second file's lines, the source files the object's debugger records name | A.6.7 |
| MAC113D | `MACRO A+B` with an `.END` before the last file | A.6.7 |
| DCL | DELETE without a version (DELVER and `$STATUS`), a bad specification to PURGE and as COPY's output, PURGE/KEEP=0, INITIALIZE and START alone | subtask 11 |
| NOCC | TYPE of a file whose records have no carriage control, into the log | A.11.2 |
| FULL | A full volume: COPY's message and `$STATUS`, whether the copy that didn't fit stays (and how big), DCL's OPEN/WRITE and WRITE, CREATE, and MACRO's object | subtask 11; A.11.1 |
| INDEX | The index file past its header: 3,000 files of a block each on a volume made `/HEADERS=16/EXTENSION=1`, so the index file grows in pieces between the files' blocks. Does VMS give INDEXF.SYS an extension header, or refuse (HEADERFULL)? DUMP/HEADER and ANALYZE/DISK_STRUCTURE show what VMS made | A.9.1 |

## Running the session

1. Build the volumes, from the repository root. The scratch disk is RD51-sized; VMS
   initializes it itself:

       go build -o govax ./cmd/govax
       ./govax --profile tbprobe --system-file none console < testdata/err113/exchange.cmd
       ./govax console < testdata/ftn86/exchange.cmd
       ./govax console < testdata/ftn86/exchanged.cmd
       ./govax --profile tbprobe --system-file none console < testdata/f90r/exchange.cmd

   Rename the old `ftn86-exchange.dsk` and `f90r-exchange.dsk` in `testdata/disks` first
   (their READMEs).

2. In simh, attach the round's volume as DUA1 and the scratch disk as DUA3 (`set rq3 rd51`,
   `attach rq3 testdata/disks/err113-scratch.dsk`). As SYSTEM (INITIALIZE needs it):

       $ MOUNT DUA1: ERR113
       $ SET DEFAULT DUA1:[000000]
       $ @ERR113/OUTPUT=ERR113.LOG
       $ TYPE DUA1:[ERR113]NOCC.DAT
       $ WRITE SYS$OUTPUT "AFTER"
       $ DISMOUNT DUA1:

   If DUA3 is taken, give another unit as P1 (`@ERR113/OUTPUT=ERR113.LOG DUA2:`). The
   INDEX part makes up to 3,000 files, some minutes on simh. The TYPE after it is the NOCC
   question at the terminal: note whether `AFTER` starts a line of its own, and whether
   `ONE` and `TWO` do.

   Then the older procedures, each volume as DUA1 in turn (their READMEs):

       $ MOUNT DUA1: FTN86
       $ SET DEFAULT DUA1:[000000]
       $ @FTN86/OUTPUT=FTN86.LOG
       $ @FTN86D/OUTPUT=FTN86D.LOG
       $ DISMOUNT DUA1:
       $ MOUNT DUA1: F90R
       $ SET DEFAULT DUA1:[000000]
       $ @F90R/OUTPUT=F90R.LOG
       $ DISMOUNT DUA1:

   FTN86 runs RECURAST with a 200 ms timer now (A.9.3), and the programs in
   `ftn86AwaitingVMS` and FTNRTL's FTN_ALLOC (A.9.5); F90R runs `f90rAwaitingVMS`, Phase
   112's programs (A.15.3).

3. Copy the results back:

       mkdir -p testdata/err113/vax testdata/str113/vax testdata/svc113/vax
       ./govax --profile tbprobe --system-file none console < testdata/err113/copyout.cmd

   and the older rounds' by their READMEs. The scratch disk stays in `testdata/disks` as the
   VAX left it, for subtask 13 to read its index file.

`TestErr113Procedure` (`internal/console/err113_test.go`) runs ERR113.COM on govax (the
scratch volume mounted first, so it isn't initialized there), so a mistake in it shows here
first. govax's own run differs where the round asks: it has no `$DIBDEF` (DEF_DIB), no
terminal or disk in the test's SVC113 cases, no DUMP/HEADER or ANALYZE/DISK_STRUCTURE, and
its index file takes all 3,000 files.

## What came back

Nothing yet.
