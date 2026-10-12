# The Phase 53 probe: LIBRTL's instruction routines

These fixtures are `docs/PHASE-53 - RTL LIB support.md`'s subtasks 12
and 13. One MACRO program calls every LIBRTL routine the phase adds, case
by case, and prints what each returned, signaled, and stored. govax's
output is checked by `TestLib53Program` (`internal/console/lib53_test.go`).
The same program, run on VMS, is the probe that settles the choices the
phase doc marks unconfirmed.

`gen.go` writes everything here but `lib53-govax.txt` and `vax/`, from
one table of cases:

    go run testdata/lib53/gen.go

| File | What it holds |
| ---- | ------------- |
| `lib53.mar` | The program: 132 cases of the bit field, integer, floating, queue and interlock, character string, CALLG, and CRC routines, with the manual's examples, the edges, and every unconfirmed choice |
| `lib53.com` | Assembles, links, and runs it on VMS, and copies the shareable images LIBRTL2.EXE and CRFSHR.EXE |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `lib53-govax.txt` | govax's output (`go test ./internal/console -run TestLib53Program -update` rewrites it) |

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

Then, in govax: `LIB53.OUT` (as `vax/lib53-vax.txt`: `*.out` files are ignored), `LIB53.LOG`, and `LIB53.MLG` go to `vax/`
(after the author has looked at them); the two images' symbols are merged
into `vmsdef` with `go run ./internal/vmsdef/gen -image FILE`; and the
images, which are licensed, are deleted from the container and its free
blocks zeroed before the container is kept anywhere.
