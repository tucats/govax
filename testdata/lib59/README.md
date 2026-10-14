# The Phase 59 probe: LIBRTL's memory routines

These fixtures are `docs/PHASE-59 - LIB VM zones.md`'s subtask 10. One
MACRO program calls every routine the phase adds, case by case, and
prints what each returned, signaled, and stored, and what it passed the
routines the program gave it. Run on VMS, it settles the choices the
phase doc's survey and progress log marked unconfirmed.
`TestLib59Oracle` (`internal/console/lib59_test.go`) checks that govax's
output matches VMS's (`vax/lib59-vax.txt`) line for line, except for the
lines it lists as expected differences.

`gen.go` writes everything here but this README and the output files,
from one table of cases:

    go run testdata/lib59/gen.go

| File | What it holds |
| ---- | ------------- |
| `lib59.mar` | The program, 526 cases: the zones and statistics at the start; the page routines (adjacency, reuse, partial and combined frees, every bad argument, `$EXPREG` between two calls); CREATE_VM_ZONE's attributes at and past each bound, each zone shown at detail level 1; GET_VM and FREE_VM in each algorithm (block distances, rounding, alignment, tags, fills, the default and string zones, the page limit, NO_EXTEND, EXTEND_AREA, TAIL_LARGE); RESET and DELETE; user zones (the manual's Example 5-1, routines omitted, a failing delete routine, LIB$DELETE_VM_ZONE as the delete routine); a zone's get-page and free-page routines at arguments 12 and 13, and 13 and 14; STAT_VM and SHOW_VM; FIND, SHOW_VM_ZONE, VERIFY; the VM_MALLOC family by value; and, last, the cases that may damage a zone (double frees, interior and wrong-size frees, a changed tag, an overwritten free block, a freed area page, deleted and junk zone IDs, the system zones, argument counts, a changed control block) |
| `lib59.com` | Assembles, links, and runs the program |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `vax/lib59-vax.txt` | The program's output on VMS (`LIB59.OUT`) |
| `vax/lib59.log` | `LIB59.COM`'s log on VMS |

## How the program works

As Phase 58's: each case runs in a routine of its own, whose condition
handler (`HANDLER`) records the condition it's given, then unwinds to
the case's caller with the condition in R0. A case prints

    name R0=xxxxxxxx [signal condition mask address]

then its lines. While a case runs, the lines it makes (the program's
routines', the zone reports') go to a buffer, printed after its R0 line.

Addresses differ from system to system, so they're shown only in forms
that don't:

- `  B_X-B_Y: n`: one block's address less another's (`untouched` when
  the routine didn't write the cell, preset to ^X55555555);
- `  B_X in its page: n`: an address's offset in its page;
- `  Z_X: set`: whether a routine wrote the cell;
- `  B_X 00: FF..F.` or `FF`: a fill map, a character a longword, `F`
  where it holds the fill, `.` where it doesn't.

Values that may vary (the statistics at the start and end, a block's
tags) are on `  ~` lines, whose names alone are compared.

What else is new:

- **ACTION**, the action routine given to LIB$SHOW_VM and
  LIB$SHOW_VM_ZONE, logs each line as `  | text`, then records its call
  (`  action: calls argc arg dsc`: the calls, the last one's argument
  count, user argument, and descriptor type and class). A routine's
  lines to SYS$OUTPUT come between `{{` and `}}`. The oracle masks every
  8-digit hex number in both kinds of line (they hold zone IDs and
  areas' addresses).
- **FINDALL** lists every zone LIB$FIND_VM_ZONE finds from context 0,
  each by its report at detail level 0, then `  = end status after n`.
- **The user zone routines** (UALLOC, UFREE, URESET, UDELETE) log
  `  > name argc`, then each argument by what it is: the case's cells
  GVN (with its value) and GVB, an offset in the user zone's control
  block or a place on the stack (with the longword there: the real zone
  or a value), or elsewhere. Then they do as the manual's Example 5-1
  does on the zone their user argument names, logging its status. What
  they return comes a bit a call from UBITS and DBITS (1 is success, 0
  ^X1230).
- **The page routines** (GPAGE, FPAGE) log `  > get-page argc n=count`
  and pass the call to LIB$GET_VM_PAGE or LIB$FREE_VM_PAGE (PBITS as
  above).
- **Statistics**: a case marked counted shows `  SD:`, what it changed
  in LIB$STAT_VM's codes 1, 2, 3, 5, 6, 7.
- DECC$MALLOC isn't called: linking against DECC$SHR would activate the
  C RTL with the program, which may make zones of its own before the
  first case.

## The VAX run

1. Build the exchange volume, from the repository root:

       go run testdata/lib59/gen.go
       govax console < testdata/lib59/exchange.cmd

   This makes `testdata/disks/lib59-exchange.dsk` (RD51 size, label
   LIB59, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @LIB59/OUTPUT=LIB59.LOG

   `LIB59.OUT` gets the program's output, `LIB59.MLG` MACRO's messages
   (none are expected), and `LIB59.LOG` the
   procedure's own: the version, the process's quotas, LINK's messages,
   and `$STATUS`.
3. Copy `LIB59.OUT` and `LIB59.LOG` back as `vax/lib59-vax.txt` and
   `vax/lib59.log`.
