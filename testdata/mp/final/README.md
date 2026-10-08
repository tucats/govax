# The multiprocessing program's last VAX run

Everything Phases 43–48 still ask VMS, on one exchange volume, run by
one command procedure (prepared 2026-10-08, at the program's close-out;
docs/PHASE-48.md; run by the author the same day, its logs audited). Each part has its own directory and README.

| Part | Directory | What it asks | Log |
| ---- | --------- | ------------ | --- |
| Macros, round 7 | `../macros` (`r7_lock.mar`, `macros7.com`) | The keywords of `$ENQ`'s twelfth and thirteenth arguments and `$GETLKI`'s seventh (govax calls them ARG12, ARG13, and ARG7) | `MACROS7.LOG` |
| Probe 5 | `../probe5` | A subprocess's priority, working set, and AST limit; PID reuse; what a subprocess inherits; `$ENQ` NOQUEUE and value blocks; `$OPEN` of a mailbox; `$ERASE` of an open file; services' argument-count minimums; DCL's verb abbreviations, statuses, and logical-name modes; device characteristics and SHOW DEVICE | `PROBE5.LOG` |

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mp/final/exchange.cmd

   This makes `testdata/disks/mp-final.dsk` (RD53 size, label MPFINAL,
   gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run, from the SYSTEM account:

       @FINAL

   Nothing should hang: probe 5's subprocesses either end or are deleted
   by the probe. If one doesn't, Ctrl/Y and `STOP P5_SLEEP` (or
   `P5_LOCK`) clear it.
3. Dismount, copy the container back, and run:

       govax console < testdata/mp/final/copyout.cmd

   Round 7's object, analysis, and log go to `../macros/vax/`, probe 5's
   log to `../probe5/vax/probe5.log`.

**Audit the macro round's log before anything reads it.** `macros7.log`
is MACRO's own output, and round 7's wrong keywords are errors whose
messages could quote a line of a macro's expansion. Claude's clean-room
hook (`.claude/hooks/cleanroom.sh`) refuses any tool call that names it
until the author has checked it and taken its entry out of the hook's
`unaudited` list. The object and analysis are fine to read, as is
probe 5's log.

## After the run

- Round 7: `go run testdata/mp/macros/dumpcode.go -calls
  testdata/mp/macros/r7_lock.calls testdata/mp/macros/vax/r7_lock.anl`
  shows each call; a candidate that is the real keyword puts ADR9 in
  its argument's place. Rename ARG12, ARG13, and ARG7 in govax's
  `starlet.mar`, and add `r7_` to `TestServiceMacroObjects`' probes.
- Probe 5: `go test ./internal/console -run TestProbe5 -v` prints govax's
  side; each difference settles one of DEVIATIONS.md's unconfirmed
  rules.

## What it answered (2026-10-08)

- Round 7: `$ENQ`'s thirteenth argument is PRIORITY and `$GETLKI`'s
  seventh RESERVED; no candidate was `$ENQ`'s twelfth.
- Probe 5: see `../probe5/README.md`, "What VMS answered". Step 8 stopped
  at `$ASCTIM`, which signaled an access violation, so the second run
  below asks the rest.

## The second run

Probe 5's step 8 for the services the first run didn't reach
(`../probe5/p5args.mar`):

    govax console < testdata/mp/final/exchange2.cmd

attach `testdata/disks/mp-final2.dsk`, set its `[000000]` as the default,
`@P5ARGS/OUTPUT=P5ARGS.LOG`, then

    govax console < testdata/mp/final/copyout2.cmd

The log goes to `../probe5/vax/p5args.log`; `TestProbe5Args` prints
govax's side. It holds program output only, no MACRO listing.
