# The Phase 54 probe: LIBRTL's strings, descriptors, and conversion

These fixtures are `docs/PHASE-54 - LIB strings and conversion.md`'s
subtask 11. One MACRO program calls every routine the phase adds, case
by case, and prints what each returned, signaled, and stored. Run on
VMS, it settled the choices the phase doc's survey had marked
unconfirmed, and `TestLib54Oracle` (`internal/console/lib54_test.go`)
checks that govax's output matches VMS's (`vax/lib54-vax.txt`) line for
line, but for the few lines it lists as expected differences.

`gen.go` writes everything here but the output files, from one table of
cases:

    go run testdata/lib54/gen.go

| File | What it holds |
| ---- | ------------- |
| `lib54.mar` | The program: 143 cases of the dynamic-string, copy, descriptor, string-function, translation, numeric-text, keyword, locale, and LIB$CVT_DX_DX routines, with the manual's examples, the edges, and every unconfirmed choice |
| `lib54.com` | Shows the locale logical names, then assembles, links, and runs the program on VMS |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `vax/lib54-vax.txt` | The program's output on VMS (`LIB54.OUT`) |
| `vax/lib54.log` | `LIB54.COM`'s log on VMS |

## How the program works

As Phase 53's: each case runs in a routine of its own, whose condition
handler (`HANDLER`) records the condition it's given (and an access
violation's reason mask and address), then unwinds to the case's caller
with the condition in R0. So the output tells a status a routine
*returned* from a condition it *signaled*. A case prints

    name R0=xxxxxxxx [signal condition mask address]

then a line for each value it shows: longwords in hex, or a string's
length and text.

What's new:

- **JSB cases** ("(JSB)" in the name) load their arguments into R0, R1,
  ..., with R1 to R6 set to `5A5A5A51` to `5A5A5A56` first, and show
  R1 to R6 after the JSB (`REGS`), so the probe sees which registers a
  routine changes. A register that held an address argument is shown
  relative to it: 0 if the routine left it alone.
- **Nothing depends on load addresses.** A dynamic string's pointer is
  shown only as `Z`, 1 when it isn't 0 (or, for "kept", when it's the
  same as before the call); a routine's returned address as an offset
  from the string's text; `$LIB_KEY_TABLE`'s string addresses as offsets
  from the table.
- **The translation tables** are copied (`MOVC3` from
  `G^LIB$AB_ASC_EBC` and the others) and shown whole, so the run compares
  govax's transcription of the manual's tables with VMS's own.
- **`$LIB_KEY_TABLE`'s layout** (`KEYTAB2`, two keywords) is shown, so
  VMS's macro can be compared with govax's clean-room one.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/lib54/exchange.cmd

   This makes `testdata/disks/lib54-exchange.dsk` (RD51 size, label
   LIB54, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @LIB54/OUTPUT=LIB54.LOG

   `LIB54.OUT` gets the program's output, `LIB54.MLG` MACRO's messages
   (none are expected), and `LIB54.LOG` the procedure's own: the locale
   logical names first (the program expects none to be defined), then
   LINK's messages.
3. Copy `LIB54.OUT` and `LIB54.LOG` back as `vax/lib54-vax.txt` and
   `vax/lib54.log`.

## The run

2026-10-13, OpenVMS V7.1 on simh's VAX 8600. No locale logical name was
defined, MACRO wrote no messages, and the program exited with
SS$_NORMAL. The output file's leading and trailing blank lines came from
the copy to the host and were removed. VMS differed from govax's
choices in about twenty lines; govax now does what VMS did (the phase
doc's progress log and DEVIATIONS.md list them), but for two things
`TestLib54Oracle` expects to differ: the registers the dynamic-string
routines' JSB entries leave (VMS's heap internals) and LIB$CVT_DX_DX of
packed decimal (outside the phase's scope). If `lib54.mar` changes, run
it on VMS again and replace `vax/`.
