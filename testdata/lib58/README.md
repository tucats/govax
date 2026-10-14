# The Phase 58 probe: LIBRTL's file routines and callbacks

These fixtures are `docs/PHASE-58 - LIB callbacks and files.md`'s
subtask 8. One MACRO program calls every routine the phase adds, case by
case, and prints what each returned, signaled, and stored, and what it
passed the routines the program gave it. Run on VMS, it settles the
choices the phase doc's survey and progress log marked unconfirmed.
`TestLib58Oracle` (`internal/console/lib58_test.go`) checks that govax's
output matches VMS's (`vax/lib58-vax.txt`) line for line, except for the
lines it lists as expected differences.

`gen.go` writes everything here but this README and the output files,
from one table of cases:

    go run testdata/lib58/gen.go

| File | What it holds |
| ---- | ------------- |
| `lib58.mar` | The program, 332 cases: the tree it works on; FIND_FILE and FIND_FILE_END (wildcards, NOWILD, a wildcard directory, a search list, a new filespec on a context, missing files, default and related names, MULTIPLE, the resultant's class, long names, flag bits); FILE_SCAN and FILE_SCAN_END (the routines' arguments and answers, defaulting through a context, a success routine that signals, FILE_SCAN_END's arguments); DELETE_FILE (versions, a refusing confirm routine, the error routine and its answers, a confirm routine that opens the FAB or leaves it open, resultant names, a nested call, the flags, a context, a signal from a confirm or success routine, a directory with a file in it); RENAME_FILE (`*` fields, a new directory, versions and CUR_VER, the flags, error sources 0 to 2, resultant names, a context, a signal); FID_TO_NAME (each form of the device name, short outputs, the reserved files, bad file IDs, a file with no directory entry, one whose directory was erased, other devices); TRIM_FILESPEC (the manual's example, each width's edge, the output's class, long names); the shared messages' texts; and, last, the cases that give a routine something that isn't a context or FAB, or too few arguments |
| `lib58.com` | Shows the device's names, then assembles, links, and runs the program, and lists the tree it leaves |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `vax/lib58-vax.txt` | The program's output on VMS (`LIB58.OUT`) |
| `vax/lib58.log` | `LIB58.COM`'s log on VMS |

## How the program works

As Phase 55's: each case runs in a routine of its own, whose condition
handler (`HANDLER`) records the condition it's given, then unwinds to
the case's caller with the condition in R0. A case prints

    name R0=xxxxxxxx [signal condition mask address]

then a line for each value it shows: longwords in hex (a word, such as
a resultant length, in a longword whose high half is `5555`, the preset
value), a string's length and text (a fixed string's whole buffer,
preset to dashes), or a FAB and its NAM (`fab`, `fna`, `dna`, `nam`,
`rlf` lines: the status and status value, FAB$L_CTX, whether it's
open, the file and default names, the resultant and expanded strings,
whether NAM$L_WCC holds a context, and a related NAM's resultant).

What's new:

- **The program's own routines** (FSOK and FSERR for FILE_SCAN; DFOK,
  DFERR, DFCONF; RFOK, RFERR, RFCONF) log each call, `  > name argc`,
  then a line per argument: `dsc type,class length "text"`, `ref`
  (the longword it points at), `val`, `fab` (with the FAB's lines), or
  `0`. What a routine returns comes a bit a call from a cell
  (CONFBITS, ERRBITS, SUCCBITS; 1 is success, 0 failure, ^X1230), so a
  case can have a confirm routine refuse every other file. A case can
  also have the next confirm or success routine do one thing more:
  `$OPEN` and `$CLOSE` its FAB (`  >   open ... close ...`), signal
  ^X0888888A (which the case's handler unwinds past the LIBRTL
  routine), call LIB$DELETE_FILE on another file (`  >   nested ...`),
  or `$OPEN` the FAB and leave it open.
- **The log.** While a case runs, the lines it makes go to a buffer,
  printed after the case's R0 line, so they read in order under it.
- **LISTF** lists what a wildcard names with LIB$FIND_FILE (`  = name`,
  then `  = end status`), to show what a delete or rename left.
- **The tree.** The program makes what it works on under `[LIB58]` on
  the default device: directories with LIB$CREATE_DIR, every access
  allowed so they can be deleted, and files of one record each
  (a name made twice is two versions). So it needs a **fresh** exchange
  volume each run: the versions it prints depend on it.
- **A file with no directory entry** (`ORPHAN.TXT`) is made by removing
  its entry with the disk ACP's `IO$_DELETE` (no IO$M_DELETE), the
  directory's ID in the FIB; one whose directory has gone is `G.TXT`,
  after a `$ERASE` of `[LIB58]GONE.DIR;1` with the file still in it
  (its status shows whether VMS allows that).
- File IDs vary from system to system: they're on `  ~` lines, whose
  names alone are compared.

## The VAX run

1. Build the exchange volume, from the repository root:

       go run testdata/lib58/gen.go
       govax console < testdata/lib58/exchange.cmd

   This makes `testdata/disks/lib58-exchange.dsk` (RD51 size, label
   LIB58, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @LIB58/OUTPUT=LIB58.LOG

   `LIB58.OUT` gets the program's output, `LIB58.MLG` MACRO's messages
   (none are expected), and `LIB58.LOG` the procedure's own: the
   device's and node's names, LINK's messages, `$STATUS`, and the tree
   the program left.
3. Copy `LIB58.OUT` and `LIB58.LOG` back as `vax/lib58-vax.txt` and
   `vax/lib58.log`.

## The run

2026-10-14, OpenVMS V7.1 on simh's VAX 8600 (node SIMVAX, the volume on
DUA1, its logical volume name DISK$LIB58), MACRO and LINK with no
messages; the program ran all 332 cases and exited with SS$_NORMAL.
VMS settled the choices the phase doc left open (its progress log lists
them), and govax now does what VMS did, but for the lines
`TestLib58Oracle` lists in `expectedLib58Differences`: an access
violation's address read from the stack, and a header slot that was
inside VMS's index file (extended to 829 blocks) but past govax's. The
VAX's LIB58.OBJ, compared with govax's (they now match byte for byte
but for the headers that name the assembler), showed an assembler
difference too: real MACRO assembles a known displacement of 0 as
register deferred mode. If `lib58.mar` changes, run it on a fresh volume
again and replace `vax/`.
