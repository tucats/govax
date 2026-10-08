# Phase 45's multiprocessing fixtures

| Path | What it holds |
| ---- | ------------- |
| `defs/` | The `$PRCDEF`, `$PQLDEF`, `$ACCDEF`, and `$MSGDEF` definition probes (subtask 1) |
| `crechild.mar`, `child.mar` | Subtask 13's program: a parent that `$CREPRC`s a child with a termination mailbox, `$GETJPI`s it, wakes it, and prints its final status |
| `mbxpingpong.mar`, `mbxpong.mar`, `pingpong.com` | Phase 46's subtask 8: a parent and child exchanging messages both ways through two temporary mailboxes, with a common event flag handshake |
| `probe1/`, `probe2/` | Phase 45's runtime probes |
| `probe3/` | Phase 46's runtime probe: mailboxes between processes, global sections, RMS on a mailbox |
| `macros/` | The system service macro probes (Phase 45 rounds 1-5, Phase 46 round 6) |
| `run46/` | Every VAX run waiting at the end of Phase 46, on one volume |
| `vax/` | The VMS run of the ping-pong pair (`pingpong.log`, not yet run) |

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
`pingpong.com` does all of this; it runs with the other end-of-Phase-46
runs (`run46/README.md`), and its log goes to `vax/pingpong.log`. The pair
hasn't been run on VMS yet.
