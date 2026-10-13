# The Phase 56 probe: LIBRTL's date and time routines

These fixtures are `docs/PHASE-56 - LIB date and time.md`'s subtask 8.
One MACRO program calls every routine the phase adds, case by case, and
prints what each returned, signaled, and stored. Run on VMS, it settles
the choices the phase doc's survey and progress log marked unconfirmed.
`TestLib56Oracle` (`internal/console/lib56_test.go`) checks that govax's
output matches VMS's (`vax/lib56-vax.txt`) line for line, except for the
lines it lists as expected differences.

`gen.go` writes everything here but the output files, from one table of
cases:

    go run testdata/lib56/gen.go

| File | What it holds |
| ---- | ------------- |
| `lib56.mar` | The program, 395 cases: the arithmetic (ADD_TIMES, SUB_TIMES, MULT[F]_DELTA_TIME); the conversions (every CVT_FROM_INTERNAL_TIME operation on three times, CVTF_FROM, CVT[F]_TO, CVT_VECTIM); DATE_TIME, DAY, DAY_OF_WEEK; FORMAT_DATE_TIME with each of the 60 predefined formats and the names, flags, and contexts; GET_MAXIMUM_DATE_LENGTH; GET_DATE_FORMAT; INIT and FREE_DATE_TIME_CONTEXT; and CONVERT_DATE_STRING |
| `lib56.com` | Shows the logical names involved (SYS$LANGUAGE, LIB$DT_FORMAT, LIB$DT_INPUT_FORMAT, LNM$DT_FORMAT_TABLE's predefined formats, the language tables), then assembles, links, and runs the program |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `vax/lib56-vax.txt` | The program's output on VMS (`LIB56.OUT`) |
| `vax/lib56.log` | `LIB56.COM`'s log on VMS |

## How the program works

As Phase 55's: each case runs in a routine of its own, whose condition
handler (`HANDLER`) records the condition it's given, then unwinds to
the case's caller with the condition in R0. A case prints

    name R0=xxxxxxxx [signal condition mask address]

then a line for each value it shows: longwords in hex, a string's length
and text, or a quadword time as `$ASCTIM` writes it (`ASCTIM xxxxxxxx`
when `$ASCTIM` fails, as it does for a result left at its preset
`EEEEEEEE EEEEEEEE`).

What's new:

- **Values that vary** from run to run (anything from the current time)
  are on lines that start `  ~`; the oracle compares only the name before
  the colon. The relative days (YESTERDAY, TOMORROW, ...) are shown as
  their difference from TODAY's result, which doesn't vary.
- **The program defines the formatting names itself**, with `$CRELNM` in
  the process table, a case at a time (LIB$DT_FORMAT, LIB$DT_INPUT_FORMAT,
  SYS$LANGUAGE), and deletes them after with `SYS$DELLNM`. Each such case
  uses a context cell of its own, so a context made earlier can't answer
  for it. It expects LIB$DT_FORMAT, LIB$DT_INPUT_FORMAT, and a French
  language table to be undefined.

## The VAX run

1. Build the exchange volume, from the repository root:

       go run testdata/lib56/gen.go
       govax console < testdata/lib56/exchange.cmd

   This makes `testdata/disks/lib56-exchange.dsk` (RD51 size, label
   LIB56, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @LIB56/OUTPUT=LIB56.LOG

   `LIB56.OUT` gets the program's output, `LIB56.MLG` MACRO's messages
   (none are expected), and `LIB56.LOG` the procedure's own: the names,
   then LINK's messages.
3. Copy `LIB56.OUT` and `LIB56.LOG` back as `vax/lib56-vax.txt` and
   `vax/lib56.log`.

## The runs

Both on 2026-10-13, OpenVMS V7.1 on simh's VAX 8600, MACRO and LINK with
no messages.

- **Run 1** (`vax/lib56-run1.txt`, `vax/lib56-run1.log`) ran 301 cases
  and hung on the next, an INIT of a cell holding ^X200 (the image's own
  first page, taken for a context), since dropped. The VAX's
  LNM$DT_FORMAT_TABLE held only LIB$DATE_FORMAT_001 and
  LIB$TIME_FORMAT_001, and LIB$DT_FORMAT and LIB$DT_INPUT_FORMAT were
  defined in the system table, as the standard format.
- **Run 2** (`vax/lib56-vax.txt`, `vax/lib56.log`) ran all 426. Round 2 of
  the program added cases for what run 1 left open, and `lib56.com` ran
  SYS$MANAGER:LIB$DT_STARTUP.COM, which defined the predefined formats:
  DIGITAL's 001–040 and 001–020 are govax's, string for string, and V7.1
  adds date formats 041–047 and time formats 021–023 (all but 041
  double-byte). Its log has the procedure's echoed lines removed (they
  are DIGITAL's file); `lib56.com` now turns verification off around it.

govax now matches run 2 line for line but for the cases
`TestLib56Oracle` lists in `expectedLib56Cases`: on that VAX
LIB$CONVERT_DATE_STRING returned SS$_ROPRAND for nearly every string
with a letter in it or beginning with punctuation (the manual's own
examples among them), which looks like a fault in VMS or the emulator,
not a rule (docs/PHASE-56's progress log).
