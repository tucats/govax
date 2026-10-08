# The multiprocessing fixtures (Phases 45-48)

| Path | What it holds |
| ---- | ------------- |
| `defs/` | The `$PRCDEF`, `$PQLDEF`, `$ACCDEF`, and `$MSGDEF` definition probes (subtask 1) |
| `crechild.mar`, `child.mar` | Subtask 13's program: a parent that `$CREPRC`s a child with a termination mailbox, `$GETJPI`s it, wakes it, and prints its final status |
| `mbxpingpong.mar`, `mbxpong.mar`, `pingpong.com` | Phase 46's subtask 8: a parent and child exchanging messages both ways through two temporary mailboxes, with a common event flag handshake |
| `msparent.mar`, `mschild.mar` | Phase 48's milestone: a parent and child, started by `$CREPRC` or `LIB$SPAWN`, exchanging messages through mailboxes and writing a shared file and one each on an ODS-2 volume |
| `probe1/`, `probe2/` | Phase 45's runtime probes |
| `probe3/` | Phase 46's runtime probe: mailboxes between processes, global sections, RMS on a mailbox |
| `macros/` | The system service macro probes (Phase 45 rounds 1-5, Phase 46 round 6) |
| `run46/` | Every VAX run waiting at the end of Phase 46, on one volume |
| `vax/` | The VMS run of the ping-pong pair (`pingpong.log`, 2026-10-08) |

`internal/console`'s `TestCreChild` assembles and links both with govax and
runs the parent under the scheduler. The programs use only system services
and RTL routines and no govax macros, so they assemble and run unchanged on
VMS:

    $ MACRO CHILD, CRECHILD
    $ LINK CHILD
    $ LINK CRECHILD

The parent reads the child's image name with `LIB$GET_FOREIGN`, so run it
through a foreign command:

    $ CRECHILD :== $DUA1:[DIR]CRECHILD.EXE
    $ CRECHILD DUA1:[DIR]CHILD.EXE

The output should be the five lines in `crechild.mar`'s header.

`mbxpingpong.mar` and `mbxpong.mar` (Phase 46, subtask 8) are built and run
the same way (`TestMbxPingPong`, at a long and a 5-instruction quantum):

    $ MACRO MBXPONG, MBXPINGPONG
    $ LINK MBXPONG
    $ LINK MBXPINGPONG
    $ MBXPINGPONG :== $DUA1:[DIR]MBXPINGPONG.EXE
    $ MBXPINGPONG DUA1:[DIR]MBXPONG.EXE

The output should be the fifteen lines in `mbxpingpong.mar`'s header.
`pingpong.com` does all of this; it ran with the other end-of-Phase-46
runs (`run46/README.md`), and its log is `vax/pingpong.log`. There, the
parent's ten lines are the same as govax's; the child's five went to the
terminal (`TT:`), not to the log, as `/OUTPUT` sends only the parent's.

## The milestone (Phase 48)

`msparent.mar` and `mschild.mar` are the multiprocessing program's
milestone (`docs/PHASE-43 - processes.md`, "The milestone"). The parent
starts the child by `$CREPRC` or by `LIB$SPAWN` (with `CLI$M_NOWAIT` and
the command `RUN image`), as its command line says; they exchange eight
rounds of messages through two mailboxes; both append each round's record
to `SHARED.DAT`, opened for shared writing, and to a file of their own
(`PARENT.DAT`, `CHILD.DAT`); and the parent learns of the child's end from
its termination mailbox or from `LIB$SPAWN`'s event flag and completion
status, then counts each process's records in `SHARED.DAT`.
`internal/console`'s `TestMilestone` runs both ways under three quanta and
checks the output, the three files, and the volume's structure;
`TestMilestone_schedulerOff` checks that both ways fail cleanly with the
scheduler off; and `cmd/govax`'s `TestRun_milestone` does it through
govax's own `macro`, `link`, and `run` subcommands.

On VMS, with the default directory on a volume where the three files may
be made:

    $ MACRO MSCHILD, MSPARENT
    $ LINK MSCHILD
    $ LINK MSPARENT
    $ MSPARENT :== $DUA1:[DIR]MSPARENT.EXE
    $ MSPARENT CREPRC DUA1:[DIR]MSCHILD.EXE
    $ MSPARENT SPAWN DUA1:[DIR]MSCHILD.EXE

The output should be the lines in `msparent.mar`'s header. The `$CREPRC`
child writes to `TT:`, so in a batch or `/OUTPUT` log its two lines go to
the terminal instead; the `LIB$SPAWN` child writes to its parent's
SYS$OUTPUT.

