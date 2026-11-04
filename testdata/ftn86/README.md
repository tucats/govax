# The FORTRAN program's VMS round

Does a FORTRAN program govax's compiler made run on a real VMS, if FTNRTL goes with it?
(`docs/PHASE-86 - FORTRAN milestone.md`, subtask 4.) There's no VAX FORTRAN on the VAX, and none
is needed: the compiler's output is MACRO-32, and FTNRTL is MACRO-32 too, so VMS's MACRO and LINK
build everything here from source. Nothing of govax's runs on the VAX but its compiler's output.

## The files

| File | What it is |
| --- | --- |
| `gen.go` | Writes the rest: `go run testdata/ftn86/gen.go`, after the corpus or FTNRTL's modules change |
| `exchange.cmd` | Makes the volume: FTNRTL's sources, and each program compiled by govax both ways, each in a directory of its own (`[NAME]` by `/OPTIMIZE`, `[NAMEN]` by `/NOOPTIMIZE`, `[M_NAME]` for a program of several files) |
| `ftn86.com` | On VMS: assembles and links FTNRTL, defines it, then assembles, links, and runs each program, reading `NAME.IN` if it has one |
| `ftn86.opt` | The options that link a program against FTNRTL. govax's LINK finds FTNRTL by itself; VMS's has to be told |
| `copyout.cmd` | Copies the log, and FTNRTL's objects, image, and map, to `vax/` |
| `exchanged.cmd`, `ftn86d.com`, `copyoutd.cmd` | The same for DUNGEON, playing `testdata/dungeon86`'s game, then saving a game and restoring it |

DUNGEON's part reads the kit through a symbolic link, and its log goes back to `testdata/disks`, since
it and the volume's `.MAR` files hold the game's text. Everything in `testdata/disks` is gitignored:

    ln -s "$HOME/Documents/Technical Doc/VMS/dungeon-3.2A" testdata/disks/dungeon-3.2A

## Running it

1. Build the volume, from the repository root (it makes `testdata/disks/ftn86-exchange.dsk`, label
   FTN86, RD54 size). Skip the second line if you don't want DUNGEON:

       go build -o govax ./cmd/govax
       ./govax console < testdata/ftn86/exchange.cmd
       ./govax console < testdata/ftn86/exchanged.cmd

2. On VMS, with the disk attached to simh (no privileges needed), as DUA1:

       $ MOUNT DUA1: FTN86
       $ SET DEFAULT DUA1:[000000]
       $ @FTN86/OUTPUT=FTN86.LOG
       $ @FTN86D/OUTPUT=FTN86D.LOG

   Nothing waits for input. A program's condition messages also show at the terminal, many of them on
   purpose: the corpus tests its error handling.

3. Dismount it or pause simh, then copy the results back:

       mkdir -p testdata/ftn86/vax/round4
       ./govax console < testdata/ftn86/copyout.cmd
       ./govax console < testdata/ftn86/copyoutd.cmd

`TestFTN86Procedure` runs the same procedure on govax, so a mistake in it shows here first.

## What came back

`vax/` is the author's third run on VMS 7.1, 3-NOV-2026: the log, FTNRTL's objects, image, and map. `vax/mar/` is the
sources it built, so `TestFTN86Objects` stays true as FTNRTL changes. `TestFTN86Oracle` holds the log to govax's run but for
`ftn86VMSDifferences`, and `TestFTN86Dungeon` DUNGEON's to VMS's digests: its log is local, `testdata/disks/ftn86d.log`.
`vax/round1/` and `vax/round2/` are the runs before the fixes each led to. `exitmore.for` and `powbits.for` are questions those
rounds raised. The phase doc's log has what each round found.

Before a later round, rename the old volume in `testdata/disks`: `exchange.cmd` makes a new one. The scripts use DUA5, since
the author's startup mounts XFER on DUA2 (a failed MOUNT there once put a round's files on XFER). `copyout.cmd` writes the next
run to `vax/round4/`, and `copyoutd.cmd` DUNGEON's log to `testdata/disks/ftn86d-round4.log`, beside the run the tests read,
until it's checked; gen.go's `round` names them. `vax/round4/mar/` is the sources that round builds, for `TestFTN86Objects`.
