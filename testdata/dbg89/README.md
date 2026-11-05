# Phase 89's debugger probe

How the VMS debugger (VMS 7.1, on simh) picks the scope it looks names up in on a
recursion, for govax's SET SCOPE, CANCEL SCOPE, and the rest to be checked against
(`docs/PHASE-89 - reentrant FORTRAN.md`, subtask 10). The *VMS 5.5 Debugger Manual*
describes the commands (SET SCOPE's dictionary entry, CD-165, and 5.3.2 and 5.3.3);
the probe settles what it leaves open: the layouts, the messages, which registers a
scope below 0 shows, where a DEPOSIT to one goes, and what resets a scope.

There's no VAX FORTRAN on the VAX, and VMS's MACRO can't describe a frame variable
to the debugger, so a FORTRAN frame variable is reached by hand here
(`.FP+FACT.K`); govax's debugger names them from the compiler's `/DEBUG` table, by
the same rules as the registers.

VMS's session logs leave out a command that fails, so `*.dbg` names each case in a
comment (`!>`) just before it.

## The program

`dbg89.mar` and `dbg89b.mar`, linked together `/DEBUG`. START calls DBG89B (a routine
of the second module, which returns at once), then REC(3), which calls itself down to
REC(1), which calls LEAF. Each register says who wrote it:

| Routine | Writes | Saves (entry mask) |
|---|---|---|
| START | R2 to R5 = ^X1002 to ^X1005 | R2 to R5 |
| REC(n) | R1 to R5 = n*^X100 + 1 to 5 | R2 to R4 (not R5) |
| LEAF | R6 = ^X6006, R7 = ^X7007, R0 = ^XF00, R1 = ^XF01 | R6 (not R7) |

So at LEAF's INLEAF, a scope's R2 is the one a frame above it saved, R5 and R7 the
live ones (no frame saved them), and R0 and R1 are what the debugger makes of
registers no call saves. Y is a datum in both modules, Z only in DBG89B, and DEPTH
counts REC's activations.

`dbg89f.for` is a recursive FORTRAN function, FACT(4), with no I/O statement; govax's
compiler writes `DBG89F.MAR` (`/NOOPTIMIZE`) for VMS's MACRO and LINK. FACT's N is
reached through `4(AP)` and its K at `FACT.K(FP)`.

## The debugger command files

| File | What it asks |
|---|---|
| `dbg89.dbg` | At INLEAF, five levels deep: SHOW SCOPE; each register in scopes 0, 1, 2, 4, 5, and past the stack; numeric path names (`1\%R2`, `9\%R2`) and invocation numbers (`REC 1\%R2`, `DBG89\REC 2\%R2`, one past the last); `%CURRENT_SCOPE_ENTRY`, `%NEXT_SCOPE_ENTRY`, `%PREVIOUS_SCOPE_ENTRY`; `SET SCOPE n`, a list, `\`, a routine with several activations, `REC 1`, a routine of a module not set (does SET SCOPE set it?), `/MODULE`, a name that doesn't exist; `SET SCOPE/CURRENT` (over a list, over itself, 0, past the stack, not a number, a list, none, `%NEXT_SCOPE_ENTRY`); EXAMINE/SOURCE and TYPE under `/CURRENT`; which of `/CURRENT` and a plain `SET SCOPE 1` STEP and GO reset; a DEPOSIT to R2 and R5 in scope 2, and whether R2 comes back from the frame when REC returns |
| `dbg89s.dbg` | At START: the frame below the main routine (`SET SCOPE 1`, `/CURRENT 1` and 2); `SET SCOPE REC` before REC is active; `SET SCOPE 0` across a break; and after the program's end: SHOW SCOPE, SET SCOPE, `/CURRENT`, CANCEL SCOPE |
| `dbg89f.dbg` | The compiler's MACRO for FACT, at its base case, four levels deep: each scope's AP and FP, and N and K through them; EXAMINE/SOURCE; `2\%FP`, `FACT 2\%AP` |
| `dbg89.com` | Builds the images and runs each session, logging to `NAME.DLG`; `dbg89.dbg` runs again on GVDBG89, govax's image |
| `exchange.cmd`, `copyout.cmd` | govax's scripts: the volume, and the results back |

## Running it

1. Build the volume, from the repository root (it makes
   `testdata/disks/dbg89-exchange.dsk`, label DBG89, gitignored):

       go build -o govax ./cmd/govax
       ./govax console < testdata/dbg89/exchange.cmd

2. On VMS, with the disk attached to simh (no privileges needed):

       $ MOUNT DUA1: DBG89            ! or the unit you attached it as
       $ SET DEFAULT DUA1:[000000]
       $ @DBG89/OUTPUT=DBG89.LOG

3. Pause simh or detach the disk, then copy the results back:

       mkdir -p testdata/dbg89/vax
       ./govax console < testdata/dbg89/copyout.cmd

The same procedure ran on govax (a scratch copy, 2026-11-05) from end to end, to find
mistakes in the probe; govax had no SET SCOPE yet, so its sessions are mostly errors.

## Results (VMS 7.1, 2026-11-05)

`vax/` holds the run: `dbg89.log` (the procedure's log), the sessions `dbg89.dlg`,
`dbg89s.dlg`, `dbg89f.dlg`, and `gvdbg89.dlg`, and the listings, maps, objects, and
images. `gvdbg89.dlg` is `dbg89.dbg` run by VMS's debugger on govax's own image. It
answered as on VMS's image, except for the source lines, which it couldn't open on the
DUA2 govax had named.

`TestDebuggerSessionOracle` (`internal/debugger/session_oracle_test.go`) replays
`dbg89.dlg`, `dbg89s.dlg`, and `dbg89f.dlg` on VMS's images. What the logs settled is
in the phase doc's progress log for subtask 10b.

One mistake in the probe: the DEPOSIT cases were written with `^X`, which VMS's
debugger doesn't read (`%DEBUG-E-SYNERREXPR`), so they tell nothing. govax reads `^X`,
so its run of those commands differs from VMS's, as the oracle's list says.
