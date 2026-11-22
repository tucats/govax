# The Fortran 90 program's VMS round

Does a Fortran 90 program govax's compiler made run on a real VMS, if FTNRTL goes with it?
(`docs/planning/PHASE-111 - Fortran 90 milestone.md`, subtask 11.) This is `testdata/ftn86`'s
pattern, for fcalc (`testdata/fcalc`, the milestone program) and the whole Fortran 90 corpus
(`testdata/fortran90`). There's no Fortran 90 compiler on the VAX, and none is needed: the
compiler's output is MACRO-32, and FTNRTL is MACRO-32 too, so VMS's MACRO and LINK build
everything here from source. Nothing of govax's runs on the VAX but its compiler's output.

It is the first time all of Fortran 90's run-time library (ALLOCATE, pointers, the
nonadvancing and namelist forms, the new intrinsics) runs on a real VAX, and the largest
exercise yet of VMS's MTHRTL under govax's code: fcalc is built `/G_FLOATING`.

## The files

| File | What it is |
| --- | --- |
| `gen.go` | Writes the rest: `go run testdata/f90r/gen.go`, after fcalc, the corpus, or FTNRTL's modules change |
| `exchange.cmd` | Makes the volume: FTNRTL's sources in `[FTNRTL]`; fcalc compiled by govax four ways, `[FCALC]` by `/OPTIMIZE`, `[FCALCN]` by `/NOOPTIMIZE`, `[FCALCR]` by `/RECURSIVE`, `[FCALCC]` by `/CHECK=ALL`, every one `/G_FLOATING`, each with fcalc's scripts and the files they run; and each corpus program three ways, `[NAME]`, `[NAMEN]`, `[NAMER]` (`[M_NAME]` for a program of several files, compiled in its `compiles.txt`'s first order and linked in its `links.txt`'s first) |
| `f90r.com` | On VMS: assembles and links FTNRTL and defines it; then builds fcalc in each directory, runs every script in its name's order (STATE's files are read later), shows the files the scripts wrote record by record, and runs `ONESHOT.COM`; then assembles, links, and runs each corpus program, reading `NAME.IN` if it has one |
| `f90r.opt` | The options that link a program against FTNRTL. govax's LINK finds FTNRTL by itself; VMS's has to be told |
| `copyout.cmd` | Copies the log, FTNRTL's objects, image, map, and the sources VMS built, and fcalc's `.MAR` files, objects, and image (its plain build) to `vax/` |

## Running it

1. Build the volume, from the repository root (it makes `testdata/disks/f90r-exchange.dsk`,
   label F90R, RD54 size, 1,680 files; INITIALIZE's `/HEADERS=6000` and
   `/DIRECTORIES=800`, below):

       go build -o govax ./cmd/govax
       ./govax --profile tbprobe --system-file none console < testdata/f90r/exchange.cmd

2. On VMS, with the disk attached to simh as DUA1 (no privileges needed):

       $ MOUNT DUA1: F90R
       $ SET DEFAULT DUA1:[000000]
       $ @F90R/OUTPUT=F90R.LOG
       $ DISMOUNT DUA1:

   Nothing waits for input. A program's condition messages also show at the terminal, many of
   them on purpose: the corpus and fcalc's `errors` script test error handling.

3. Copy the results back:

       mkdir -p testdata/f90r/vax/round3/mar testdata/f90r/vax/round3/fcalc
       ./govax --profile tbprobe --system-file none console < testdata/f90r/copyout.cmd

`TestF90RProcedure` (`internal/console/f90r_test.go`) runs the same procedure on govax, so a
mistake in it shows here first: FTNRTL builds with no message, every corpus program prints its
`name.out` but for `f90rFileDifferences`, and each fcalc build prints every script's `name.out`,
writes the files `testdata/fcalc/expected` holds, and runs `ONESHOT.COM` as `oneshot.out` says.

## What reading a file changes

The procedure's runs read their input from files, not a terminal, and two things follow:

- **fcalc's prompt.** fcalc writes `fcalc> ` with nonadvancing output. At a terminal, FTNRTL
  writes that open record as the READ's prompt, and the next line is a record of its own;
  reading a file, the record stays open and the next line goes on in it, as Fortran 90 says it
  should. fcalc asks whether SYS$INPUT is a terminal (`fcplatform.f90`'s `prompt_ends`, through
  LIB$GETDVI) and so prints the same lines either way, but for one: a line too long for the room
  the prompt left (125 characters, not 132) is split there, which the test allows for
  (`f90rFromFile`). Building this round found fcalc writing its carriage-control blank into the
  middle of such a record (`fcalc>  set echo`), and overflowing it on a long line; the phase
  doc's log has the fix.
- **ADVANCE's prompt**, likewise, stays in its record (`f90rFileDifferences`).

## The first attempt, and /HEADERS

The first volume, made with INITIALIZE's defaults, bug-checked VMS 7.1 at once: `@F90R`
crashed in the file system (UNXSIGNAL, an access violation in F11BXQP), and so did a
read-only `ANALYZE/DISK_STRUCTURE` (XQPERR). Its INDEXF.SYS had grown 18 blocks at a time
to 98 pieces, more than its header holds, so the last 411 blocks were mapped by an
extension header; the stack in the second crash held 1302, the blocks the primary header
mapped. `tools/odscheck` found nothing wrong by ODS-2's rules, so the form VMS objects to
is still open (the phase doc's log), but the volume is now made with an index file big
enough for every file the round makes, in one piece, and an MFD with room to spare. The
old volume is `testdata/disks/f90r-exchange-attempt1.dsk`.

## What came back

`vax/` is round 2, the author's run on VMS 7.1, 22-NOV-2026, on a volume rebuilt after round
1's fix: the log, FTNRTL's objects, image, map, and the sources VMS built (`vax/mar/`), and
fcalc's plain build's `.MAR` files, objects, and image (`vax/fcalc/`). Every program built
with no message, and **every section of the log is what govax's run of the procedure prints**:
all 366 corpus runs, and fcalc in all four builds, its 11 scripts each, the files they saved,
and ONESHOT (`TestF90ROracle`, fcalc's file times masked).

`TestF90RObjects` holds govax's MACRO to VMS's on fcalc's 16 `.MAR` files, the compiler's own
output: their GSD, TIR, and end records. 14 are VMS's; it found govax storing a repetition
factor (`.LONG 0[2]`) byte by byte where VMS's MACRO writes `STA_UB 0`, `STA_UB 2`, `STO_RL`
(fixed, with govax's LINK taught `STO_RB`, `STO_RW`, and `STO_RL`). FCFUNCS and FCMATRIX still
differ where VMS stores one byte of a long run of immediate data through the stack;
`testdata/tir111` is the probe to settle it.

`vax/round1/` is round 1, 22-NOV-2026, whose volume (`testdata/disks/f90r-exchange-round1.dsk`)
had the compiler's bare `.ASCII` for an empty text, which VMS's MACRO refused (DIRSYNX), so
five programs and fcalc never built; the 351 runs that did printed what govax's do.

The scripts use DUA5 on govax, since the author's startup mounts XFER on DUA2. Before a
later round, rename the old volume in `testdata/disks` (`exchange.cmd` makes a new one),
and point `gen.go`'s `results` at `vax/round3`.
