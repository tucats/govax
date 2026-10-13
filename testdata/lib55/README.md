# The Phase 55 probe: LIBRTL's jackets, items, resources, and timers

These fixtures are `docs/PHASE-55 - LIB service jackets and resources.md`'s
subtask 8. One MACRO program calls every routine the phase adds, case by
case, and prints what each returned, signaled, and stored. Run on VMS,
it settles the choices the phase doc's survey marked unconfirmed, and
`TestLib55Oracle` (`internal/console/lib55_test.go`) checks that govax's
output matches VMS's (`vax/lib55-vax.txt`) line for line, but for the
lines it lists as expected differences.

`gen.go` writes everything here but the output files, from one table of
cases:

    go run testdata/lib55/gen.go

| File | What it holds |
| ---- | ------------- |
| `lib55.mar` | The program: the jackets (SYS_FAO, SYS_FAOL, SYS_ASCTIM, SYS_GETMSG, SYS_TRNLOG) into fixed, dynamic, and varying strings; GETJPI, GETDVI, and GETSYI's numeric and text forms; the event flag and LUN pools; the timers; AST_IN_PROG, ASN_WTH_MBX, WAIT, GET_USERS_LANGUAGE; and the protection-text routines |
| `lib55.com` | Defines `LIB55_DISK`, shows the names the program expects undefined and a few lexical functions' items, then assembles, links, and runs the program |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `vax/lib55-vax.txt` | The program's output on VMS (`LIB55.OUT`) |
| `vax/lib55.log` | `LIB55.COM`'s log on VMS |

## How the program works

As Phase 54's: each case runs in a routine of its own, whose condition
handler (`HANDLER`) records the condition it's given, then unwinds to
the case's caller with the condition in R0. A case prints

    name R0=xxxxxxxx [signal condition mask address]

then a line for each value it shows: longwords in hex, or a string's
length and text.

What's new:

- **Values that vary** from run to run (times, PIDs, counts, this
  system's names and privileges) are shown on lines that start `  ~`;
  the oracle compares only the name before the colon.
- **SHOW_TIMER's line** goes to an action routine (`TIMACT`), which
  copies it; it's shown with every digit made a 9 (`masked`), so its
  layout is compared, then as it was (`~`).
- **Elapsed times** (LIB$WAIT) are a bucket, `Q`: 1 if at least the
  case's number of hundredths passed; the hundredths themselves are a
  `~` line.
- **The program defines names of its own** with `$CRELNM` in the
  process table (`LIB55_NAME`, `SYS$LANGUAGE`), and expects
  `LIB55_DISK` to name the exchange volume's device.

## The VAX run

1. Build the exchange volume, from the repository root:

       go run testdata/lib55/gen.go
       govax console < testdata/lib55/exchange.cmd

   This makes `testdata/disks/lib55-exchange.dsk` (RD51 size, label
   LIB55, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @LIB55/OUTPUT=LIB55.LOG

   `LIB55.OUT` gets the program's output, `LIB55.MLG` MACRO's messages
   (none are expected), and `LIB55.LOG` the procedure's own: the logical
   names the program expects undefined, F$GETDVI's VPROT and ACPTYPE and
   other lexical items, then LINK's messages (VMS 7.1's LIBRTL may lack
   the protection-text routines or LIB$GET_ACCNAM; their cases then show
   the access violation of a call to an undefined symbol).
3. Copy `LIB55.OUT` and `LIB55.LOG` back as `vax/lib55-vax.txt` and
   `vax/lib55.log`.

## The run

2026-10-13, OpenVMS V7.1 on simh's VAX 8600 (the exchange volume needed
ods2's contiguous-extent fix first: LIB55.MAR had been spread over
three file headers, which VMS couldn't open). SYS$LANGUAGE was defined
as ENGLISH in the system table, so the oracle's console defines it too;
MACRO and LINK wrote no messages, and the program exited with
SS$_NORMAL. VMS settled every choice the survey left open (the phase
doc's progress log lists them), and govax now does what VMS did, but for
the lines `TestLib55Oracle` lists as expected differences. If
`lib55.mar` changes, run it on VMS again and replace `vax/`.
