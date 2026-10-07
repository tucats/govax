# Phase 45 probe 1: a subprocess and its termination message on VMS

`docs/PHASE-45.md` records several things about `$CREPRC` and `$GETJPI` as
unconfirmed. This probe asks VMS for them. (The simh system answers `OpenVMS V7.1` in
SHOW SYSTEM, not 7.3: the earlier probes' "VMS 7.3" is the version of the
macro assembler and linker, which may differ from the running system.)

| File | What it holds |
| ---- | ------------- |
| `probe1.mar` | The parent. It creates a termination mailbox, a named information mailbox, and two subprocesses. |
| `../child.mar` | The first child (shared with `crechild.mar`): prints, hibernates, returns the status 7. It is created with `NL:` for output and the termination mailbox. |
| `info.mar` | The second child, created with **no** input, output, or error. It writes the translations of `SYS$INPUT`, `SYS$OUTPUT`, `SYS$ERROR`, and `SYS$COMMAND` to the information mailbox. |
| `probe1.com` | Assembles, links, and runs it. |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and copy the log back. |
| `vax/` | The VMS run's log |

What the log shows:

1. **`$GETJPI` of the child while it hibernates, and of the parent**: thirty
   longword items, each with its status. This settles `JPI$_STATE`,
   `JPI$_JOBTYPE`, and `JPI$_MODE` for a subprocess, and the nominal
   quotas, limits, and counts govax made up.
2. **The termination message**, as 21 longwords with their offsets: the
   job ID at +0C, the word after the message type, the times, and the
   counts govax leaves at 0.
3. **What a process created with no input, output, or error has** as
   `SYS$INPUT`, `SYS$OUTPUT`, `SYS$ERROR`, and `SYS$COMMAND`.

`TestProbe1` (`internal/console/probe1_test.go`) runs the same programs
under govax, so the two reports can be put side by side.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mp/probe1/exchange.cmd

   This makes `testdata/disks/mp-probe1.dsk` (RD53 size, label MPPROBE1,
   gitignored).
2. Attach it to the simh VAX, mount it, set it as the default directory
   (its `[000000]`), and run:

       @PROBE1/OUTPUT=PROBE1.LOG

   Run it from the SYSTEM account, or any account that may create a
   subprocess at a higher priority than its own (the programs use
   ALTPRI). If the account has a small PRCLM, no more than two
   subprocesses are made at a time.
3. Dismount the volume and copy the container back, then:

       govax console < testdata/mp/probe1/copyout.cmd

Run 1 (2026-10-07, `vax/probe1-run1.log`) ended at once with
`%SYSTEM-F-INSFARG`, before any output: a service was called with fewer
arguments than VMS 7.1 requires (suspects: `$GETDVIW` with 4, `$CREPRC`
with 12). The programs now print `step n` before each service so a log
that ends early shows where, and `$GETDVIW` is called with 8 arguments.
Run 2 (`vax/probe1-run2.log`) stopped at `step 2`: `$CREMBX` with 4
arguments is `INSFARG` too. VMS 7.1 wants all seven (through `lognam`);
govax accepts fewer, which is why its tests never noticed.

Runs 3 and 4 got the first child's report but nothing from the second.
Run 5 (`vax/probe1-run5.log`) read its termination message: final status
`00000114`, SS$_INSFARG. The second child itself called `$ASSIGN` with 2
arguments; VMS wants all 4.

Nothing else is needed. If a program hangs on VMS (for instance, waiting
for a termination message that doesn't come), press CTRL/Y and `STOP` the
subprocesses; the log up to that point is still worth copying.
