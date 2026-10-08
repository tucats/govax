# Phase 48 probe 4: LIB$SPAWN and DCL's SPAWN

`docs/PHASE-48 - LIB_SPAWN.md` records what govax's `LIB$SPAWN` and its
subprocess CLI guess at. This probe asks VMS. It runs with the phase's
other VMS work (`../run48/README.md`); `probe4.com` is its part.

| File | What it holds |
| ---- | ------------- |
| `probe4.mar` | The program: eleven steps, each line `n what: result` |
| `p4child.mar` | The image the spawned commands run: it ends with the status 3 |
| `p4nocli.mar` | The image of step 11: it calls `LIB$SPAWN` in a process with no CLI |
| `p4cmds.com` | Step 10's commands, the SYS$INPUT of a `$CREPRC` of LOGINOUT |
| `probe4.com` | Assembles, links, and runs it, types the spawned processes' logs, then tries DCL's SPAWN, waiting and not |
| `exchange.cmd`, `copyout.cmd` | Build a volume of probe 4 alone (`testdata/disks/mp-probe4.dsk`) and copy its log back, for running it by itself |
| `vax/` | The VMS run's log, `probe4.log` (not yet run) |

`TestProbe4` (`internal/console`) runs the same program under govax;
`go test ./internal/console -run TestProbe4 -v` prints govax's report and
the spawned processes' logs, to set beside VMS's. (govax's CLI has no
SHOW command, so steps 5 to 7's logs are its unknown-verb message.)

## The steps

Each spawned command's output goes to `P4_n.LOG`, and `PROBE4.COM` types
them after the program ends.

1. `RUN P4CHILD`, waiting: `LIB$SPAWN`'s status and the completion status
   (govax: 3, the image's).
2. `EXIT 7`: what DCL's EXIT does in a spawned subprocess, and its
   completion status (govax: logs out with 7).
3. `FROBNICATE`: DCL's message and the completion status (govax:
   `%DCL-W-IVVERB` with the verb on a line of its own, CLI$_IVVERB).
4. `RUN NOSUCH`: DCL's messages and the status (govax: `%DCL-W-ACTIMAGE`
   and `-RMS-E-FNF`, RMS$_FNF).
5. `SHOW LOGICAL/PROCESS P4_*`, after defining P4_USER (user mode),
   P4_SUPER (supervisor), P4_EXEC (executive), and P4_CONF (user,
   CONFINE): which the subprocess has (govax copies user and supervisor
   names, not CONFINE ones).
6. `SHOW SYMBOL P4SYM`: the subprocess has the parent's DCL symbols.
7. The same with `CLI$M_NOCLISYM`: it hasn't.
8. A flag `LIB$SPAWN` doesn't define (^X200): its status (govax:
   LIB$_INVARG).
9. `CLI$M_NOWAIT` with event flag 11, set beforehand: whether `LIB$SPAWN`
   clears it (govax: yes), the default process name (govax: SYSTEM_1),
   and the completion status once the flag is set.
10. `$CREPRC` of `SYS$SYSTEM:LOGINOUT.EXE` with `P4CMDS.COM` as its input:
    the termination message's final status after `RUN P4CHILD`,
    `FROBNICATE`, and `LOGOUT` (govax: the last command's, CLI$_IVVERB).
11. `$CREPRC` of `P4NOCLI.EXE`, which calls `LIB$SPAWN` with no CLI: its
    final status (govax: LIB$_NOCLI).

After the program, DCL's own SPAWN: `SPAWN/OUTPUT=...` and
`SPAWN/NOWAIT/PROCESS=P4_NOWAIT/OUTPUT=...`, for the messages DCL prints
(govax's console prints `%DCL-S-RETURNED` and `%DCL-S-SPAWNED`), and
`$STATUS` after the first.
