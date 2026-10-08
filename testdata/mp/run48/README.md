# The VAX runs of Phase 48, in one session

Everything Phase 48 asks VMS, on one exchange volume, run by one command
procedure. Each part has its own directory and README; this directory
only puts them together.

| Part | Directory | What it asks | Log |
| ---- | --------- | ------------ | --- |
| Definitions | `../defs` (`def_cli`, `def_lib`) | Every name and value `$CLIDEF` and `$LIBDEF` define, for govax's macro library | `DEFS.LOG` |
| The milestone | `..` (`msparent.mar`, `mschild.mar`, `milestone.com`) | The multiprocessing milestone on VMS, both ways: the output should be the lines in `msparent.mar`'s header, and the files' records the same as govax's | `MILESTONE.LOG` |
| Probe 4 | `../probe4` | `LIB$SPAWN`'s and DCL's behavior that govax guesses at: completion statuses, DCL's messages, which logical names and symbols a subprocess gets, the event flag, the default name, LOGINOUT, LIB$_NOCLI, DCL's SPAWN messages | `PROBE4.LOG` |

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mp/run48/exchange.cmd

   This makes `testdata/disks/mp-run48.dsk` (RD53 size, label MPRUN48,
   gitignored), holding every program and procedure.
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run, from the SYSTEM account (probe 4 defines an
   executive-mode logical name):

       @RUN48

   It ends with a directory of the logs. Nothing should hang: every wait
   is for a subprocess that ends by itself. If one does, Ctrl/Y and
   `STOP MS_CHILD` (the milestone) or `STOP P4_LOGINOUT` (probe 4) clear
   it.
3. Dismount, copy the container back, and run:

       govax console < testdata/mp/run48/copyout.cmd

   The definitions go to `../defs/vax/` (log `defs48.log`), the
   milestone's log to `../vax/milestone.log`, and probe 4's to
   `../probe4/vax/probe4.log`.

**Audit the definitions' log before anything reads it.** `defs48.log` is
MACRO's own output, and an error message could quote a line of a macro's
expansion. Claude's clean-room hook (`.claude/hooks/cleanroom.sh`) refuses
any tool call that names it until the author has checked it and taken its
entry out of the hook's `unaudited` list. The objects and analyses are
fine to read (they hold object data, not macro text), as are the
milestone's and probe 4's logs (program and DCL output).

## After the run

- Definitions: `go run testdata/mp/defs/decode.go` and
  `go run ./internal/vmsdef/gen -values testdata/mp/defs/defined.txt`,
  then `go generate ./internal/bootdata` for `$CLIDEF` and `$LIBDEF` in
  govax's macro library (`internal/bootdata/mkdefs`).
- The milestone: its log should show the parent's lines as
  `TestMilestone` does, each way (the `$CREPRC` child's two lines go to
  the terminal, not the log), and the same records in the typed files,
  SHARED.DAT's in whatever order the two processes wrote them.
- Probe 4: `go test ./internal/console -run TestProbe4 -v` prints govax's
  report and logs; each difference settles one of the phase doc's
  unconfirmed items.
