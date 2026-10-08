# Phase 46 probe 3: mailboxes between processes, global sections, and RMS on a mailbox

`docs/PHASE-46 - interprocess comm.md` records several behaviors as
govax's guesses. This probe asks VMS for them. It is run with the other
end-of-Phase-46 runs (`../run46/README.md`); `probe3.com` is its part.

| File | What it holds |
| ---- | ------------- |
| `probe3.mar` | The program: three parts, each line `what: result` |
| `probe3c.mar` | The child of part 1: it reads messages from one mailbox and reports each read's IOSB on another |
| `probe3.com` | Assembles, links, and runs it |
| `exchange.cmd`, `copyout.cmd` | Build a volume of probe 3 alone (`testdata/disks/mp-probe3.dsk`) and copy its log back, for running it again by itself |
| `vax/` | The VMS runs' logs: `probe3-run1.log` (2026-10-08, with `../run46`), and `probe3.log`, the second run's |

## The first run

The first run (`vax/probe3-run1.log`) stopped at its first read of P3_B
with SS$_MBTOOSML: VMS refuses a mailbox read whose buffer is longer than
the mailbox's largest message, as it refuses such a write. The parent
ended, and its child, waiting for its report to be read, had to be
stopped by hand. govax accepted the read; it now refuses it too
(`corevms/mbxdriver.go`). The probe now reads no more than the largest
message, and deletes its child whenever it ends early.

## Running it again

From the repository root, `govax console < testdata/mp/probe3/exchange.cmd`;
on the VAX, from SYSTEM, with the volume as the default directory,
`@PROBE3/OUTPUT=PROBE3.LOG`; then
`govax console < testdata/mp/probe3/copyout.cmd`.

A PID prints as PARENT, CHILD, or ZERO, then its value, so lines compare
between runs and systems. `TestProbe3` (`internal/console`) runs the same
program under govax; `go test ./internal/console -run TestProbe3 -v`
prints govax's report.

## The experiments

**Part 1, mailboxes between two processes.** The parent makes two
temporary mailboxes, P3_A and P3_B, and creates the child, which
assigns them by name.

- 1.1, 1.2: `$GETDVI` of P3_A from both sides: DVI$_PID and DVI$_OWNUIC
  (govax: the creator's, from either side), DVI$_REFCNT, DVI$_DEVDEPEND
  and DVI$_DEVDEPEND2 (govax: the message count in DEVDEPEND's low word),
  DVI$_DEVBUFSIZ; and the IOSB of a read of the child's plain write.
- 1.3: a plain write handed to a waiting read: both IOSBs' PIDs.
- 1.4: with the child busy, an IO$M_NOW write and a plain `$QIO` write
  queued; `$GETDVI` with two messages queued; then each read's IOSB, and
  the plain write's IOSB once it was read (govax: a queued IO$M_NOW
  write's IOSB has PID 0, the read gets the writer's PID).
- 1.5: an IO$M_NOW write handed to a waiting read (govax: the write's
  IOSB gets the reader's PID; unconfirmed).
- 1.6: an end-of-file message to a waiting read (SS$_ENDOFFILE).
- 1.7: the child's final status.

**Part 2, global sections** (one process; the services are called with
`CALLS`, since govax has no macros for them yet). Page-file sections
mapped with SEC$M_EXPREG:

- 2.1-2.4: `$CRMPSC` of a new section, `$MGBLSC` of it (the two mappings
  share their pages), `$CRMPSC` of the same name again and bigger
  (govax: SS$_CREATED for a new section, SS$_NORMAL for an existing
  one, mapped at its own size); `retadr` each time.
- 2.5-2.13: the errors: a name that doesn't exist, pagcnt 0, relpag past
  the end, SEC$M_CRF with SEC$M_PAGFIL, SEC$M_PAGFIL, SEC$M_PERM, or
  SEC$M_SYSGBL without SEC$M_GBL, and SEC$M_GBL without SEC$M_PAGFIL
  (a file section, which govax doesn't support).
- 2.14-2.16: idents: a version 1.5 section mapped with each match control
  and version, and `$CRMPSC` with an ident no section of the name
  matches.
- 2.17: a section created read-only, mapped writable and read-only.
- 2.18-2.20: `$DGBLSC` of a mapped section (and whether the name is gone
  and the mapping still works), of a name that doesn't exist, and with
  SEC$M_SYSGBL for a group section.

**Part 3, RMS on a mailbox and on NL:.**

- 3.1: `$OPEN` of a mailbox: FAB$L_DEV, FAB$W_MRS (govax: the mailbox's
  largest message; unconfirmed), FAB$B_RFM, FAB$B_RAT, FAB$B_ORG.
- 3.2, 3.3: whether an asynchronous `$PUT` (RAB$V_ASY) with no one
  reading returns RMS$_PENDING (it waits for the message to be read) or
  succeeds at once (govax's choice); then a read of the mailbox, `$WAIT`,
  and RAB$L_STS. Asynchronous, so the probe can't deadlock if `$PUT`
  does wait.
- 3.4, 3.5: `$GET` of a message (RAB$W_RSZ) and of an end-of-file
  message (its status and RAB$L_STV).
- 3.6, 3.7: `$CLOSE`; `$OPEN` of NL:, its FAB$L_DEV and FAB$W_MRS, and
  `$GET` from it.
