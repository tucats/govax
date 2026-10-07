# Phase 45 probe 2: process-control edge statuses and the null device on VMS

`docs/PHASE-45.md` records several statuses as govax's guesses. This probe
asks VMS for them. It also captures a few system parameters.

| File | What it holds |
| ---- | ------------- |
| `probe2.mar` | The program: nine experiments, each printing `what: status` lines |
| `../child.mar` | The child: prints, hibernates, returns 7 |
| `probe2.com` | Assembles, links, and runs it, then `SYSGEN` and `SHOW DEVICE` |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and copy the log back |
| `vax/` | The VMS run's log |

The VMS 7.1 run is `vax/probe2.log` (2026-10-07), and `docs/PHASE-45.md`'s
progress log lists what it settled and what govax changed. The
experiments (`TestProbe2 -v` prints govax's whole report; the "govax"
notes below are what govax said *before* the run):

1. `$RESUME` of a process that isn't suspended (govax: SS$_NORMAL);
   `$SUSPND`; its `JPI$_STATE` (9, SUSP); `$SUSPND` of one already
   suspended (SS$_SUSPENDED, `000003A4`); `$RESUME`; `$RESUME` again.
2. `$SETPRI` of the child: its status, the previous priority, and
   `JPI$_PRI`/`JPI$_PRIB` afterwards (govax applies no boost).
3. `$FORCEX` with the code `10000`: the final status in the termination
   message (govax: the code).
4. `$DELPRC` of a hibernating child: the final status (govax: SS$_ABORT,
   `0000002C`).
5. `$CREPRC` of an image that doesn't exist: the final status (govax:
   RMS$_FNF).
6. `$CREPRC` errors: a duplicate name, a 16 character name, a reserved bit
   in `stsflg`.
7. `$WAKE`, `$RESUME`, and `$GETJPI` of a PID that doesn't exist.
8. The `$GETJPI` wildcard scan: the PID longword after each call, the
   names, the final status (govax keeps a context there).
9. The null device: `$GETDVI` of `NL:` (class, type, characteristics),
   `$ASSIGN`, a write, a read, a sense mode: statuses and IOSBs.

`PROBE2.COM` then runs `SYSGEN` for `SHOW/PQL` (the process quota
defaults and minimums, which govax made up), a few more parameters, and
`SHOW DEVICE/FULL NLA0:`.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mp/probe2/exchange.cmd

   (`testdata/disks/mp-probe2.dsk`, RD53 size, label MPPROBE2, gitignored.)
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run, from the SYSTEM account:

       @PROBE2/OUTPUT=PROBE2.LOG

3. Dismount, copy the container back, and run:

       govax console < testdata/mp/probe2/copyout.cmd

If the program stops early with a status (an argument count VMS 7.1 refuses,
say), the log shows the last line printed: send it back as for probe 1.
