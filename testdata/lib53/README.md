# The Phase 53 probe: LIBRTL's instruction routines

These fixtures are `docs/PHASE-53 - RTL LIB support.md`'s subtasks 12
and 13. One MACRO program calls every LIBRTL routine the phase adds, case
by case, and prints what each returned, signaled, and stored. Run on VMS,
it settled the choices the phase doc had marked unconfirmed, and
`TestLib53Oracle` (`internal/console/lib53_test.go`) checks that govax's
output matches VMS's (`vax/lib53-vax.txt`) line for line.

`gen.go` writes everything here but `vax/`, from one table of cases:

    go run testdata/lib53/gen.go

| File | What it holds |
| ---- | ------------- |
| `lib53.mar` | The program: 132 cases of the bit field, integer, floating, queue and interlock, character string, CALLG, and CRC routines, with the manual's examples, the edges, and every unconfirmed choice |
| `lib53.com` | Assembles, links, and runs it on VMS, and copies the shareable images LIBRTL2.EXE and CRFSHR.EXE |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `vax/lib53-vax.txt` | The program's output on VMS (`LIB53.OUT`; `*.out` files are ignored by git) |
| `vax/lib53.log` | `LIB53.COM`'s log on VMS |

## How the program works

Each case runs in a routine of its own, whose condition handler
(`HANDLER`) records the condition it's given (and an access violation's
reason mask and address), then unwinds to the case's caller with the
condition in R0. So the output tells a status a routine *returned* from
a condition it *signaled*, which is one of the questions the probe
answers. A case prints

    name R0=xxxxxxxx [signal condition mask address]

then a line for each value it shows: longwords in hex, or a string's
length and text. Nothing printed depends on where the program was
loaded: a queue routine's remque-address is printed as an offset from the
queue's header (the header and entries are self-relative already).

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/lib53/exchange.cmd

   This makes `testdata/disks/lib53-exchange.dsk` (RD51 size, label
   LIB53, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @LIB53/OUTPUT=LIB53.LOG

   `LIB53.OUT` gets the program's output, `LIB53.MLG` MACRO's messages
   (none are expected), and `LIB53.LOG` the procedure's own, LINK's
   included.
   The procedure also copies `SYS$SHARE:LIBRTL2.EXE` and `CRFSHR.EXE`
   (from SYS$SHARE or SYS$LIBRARY) to the volume, for their symbol
   tables.
3. Dismount the volume and copy the container back as
   `testdata/disks/lib53-vax.dsk`.

## The run

2026-10-12, OpenVMS V7.1 on simh's VAX 8600. MACRO wrote no messages, so
`LIB53.MLG` was never created. Of the 132 cases, 125 matched govax's
output as it stood; the seven that didn't were govax's own choices, and
govax now does what VMS did (the phase doc's progress log lists them).
If `lib53.mar` changes, run it on VMS again and replace `vax/`.

The images' symbols are in `vmsdef` (`go run ./internal/vmsdef/gen -image
FILE`, from raw-block copies, `COPY/BINARY`). The images were then
removed: the run's container was replaced by a fresh one,
`testdata/disks/lib53-vax.dsk` (local only), holding just `LIB53.MAR`,
`LIB53.COM`, `LIB53.LOG`, and `LIB53.OUT`, and checked to contain none of
the images' symbol names.
