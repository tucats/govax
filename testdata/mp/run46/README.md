# The VAX runs waiting at the end of Phase 46, in one session

**Run 2026-10-08.** Everything worked but probe 3, which stopped at once
(SS$_MBTOOSML) and left its child waiting; see `../probe3/README.md`,
which has scripts to run it again by itself. What the rest settled is in
`docs/PHASE-46 - interprocess comm.md`'s progress log.

Everything Phases 45 and 46 left for VMS to answer, on one exchange
volume, run by one command procedure. Each part has its own directory,
README, and scripts; this directory only puts them together.

| Part | Directory | What it asks | Log |
| ---- | --------- | ------------ | --- |
| Macro probe, round 5 | `../macros` (`r5_misc.mar`) | `$IDTOASC`'s third argument, `$TRNLOG`'s and `$CRELNT`'s argument sizes (left from Phase 45) | `MACROS5.LOG` |
| Macro probe, round 6 | `../macros` (`r6_*.mar`) | The keywords and argument forms of `$CRMPSC`, `$MGBLSC`, `$DGBLSC` (Phase 46 has no macros for them) and `$ENQ(W)`, `$DEQ`, `$GETLKI(W)` (for Phase 47) | `MACROS6.LOG` |
| Definitions | `../defs` (`def_sec`, `def_lck`, `def_lki`, `def_psl`, `def_dc`) | Every name and value `$SECDEF`, `$LCKDEF`, `$LKIDEF`, `$PSLDEF`, and `$DCDEF` define | `DEFS.LOG` |
| Probe 3 | `../probe3` | Phase 46's unconfirmed behavior: mailbox IOSBs between processes, `$GETDVI` of a mailbox, global section results and statuses, RMS on a mailbox and NL: | `PROBE3.LOG` |
| Ping-pong | `..` (`mbxpingpong.mar`, `mbxpong.mar`) | Phase 46's MACRO test, on VMS: the output should be the fifteen lines in `mbxpingpong.mar`'s header | `PINGPONG.LOG` |

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mp/run46/exchange.cmd

   This makes `testdata/disks/mp-run46.dsk` (RD53 size, label MPRUN46,
   gitignored), holding every program and procedure.
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run, from the SYSTEM account (probe 3 makes global
   sections and uses the privileges SYSTEM has):

       @RUN46

   It ends with a directory of the logs. If probe 3 or the ping-pong
   hangs (neither should: probe 3 gives up on any wait after 60 seconds),
   Ctrl/Y and `STOP PROBE3_CHILD` or `STOP PONG_1` clear it.
3. Dismount, copy the container back, and run:

       govax console < testdata/mp/run46/copyout.cmd

   Each result goes to its own directory's `vax/`: the macro probes'
   objects, analyses, and logs to `../macros/vax/`, the definitions' to
   `../defs/vax/` (log `defs46.log`), `../probe3/vax/probe3.log`, and
   `../vax/pingpong.log`.

**Audit the MACRO logs before anything reads them.** `macros5.log`,
`macros6.log`, and `defs46.log` are MACRO's own output, and its error
messages may quote a line of a macro's expansion. Claude's clean-room hook
(`.claude/hooks/cleanroom.sh`) refuses any tool call that names them until
the author has checked them and taken their entry out of its `unaudited`
list. The objects and analyses are fine to read (they hold object data,
not macro text), as are the probe 3 and ping-pong logs (program output).

## After the run

- Macros: see `../macros/README.md`, "After the run". Rounds 5 and 6 are
  added to `internal/asm`'s `TestServiceMacroObjects`, and govax's macros
  in `starlet.mar` are written for `$CRMPSC`, `$MGBLSC`, `$DGBLSC`, and
  (in Phase 47) the lock services, until every object matches.
- Definitions: `go run testdata/mp/defs/decode.go` and
  `go run ./internal/vmsdef/gen -values testdata/mp/defs/defined.txt`,
  then `$SECDEF` and the rest for govax's macro library
  (`internal/bootdata/mkdefs`).
- Probe 3: `go test ./internal/console -run TestProbe3 -v` prints govax's
  report, line for line beside VMS's. Each difference settles one of
  `docs/PHASE-46 - interprocess comm.md`'s unconfirmed items.
- Ping-pong: the log should show the same fifteen lines as
  `TestMbxPingPong`.
