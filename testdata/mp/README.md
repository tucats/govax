# Phase 45's multiprocessing fixtures

| Path | What it holds |
| ---- | ------------- |
| `defs/` | The `$PRCDEF`, `$PQLDEF`, `$ACCDEF`, and `$MSGDEF` definition probes (subtask 1) |
| `crechild.mar`, `child.mar` | Subtask 13's program: a parent that `$CREPRC`s a child with a termination mailbox, `$GETJPI`s it, wakes it, and prints its final status |
| `mbxpingpong.mar`, `mbxpong.mar` | Phase 46's subtask 8: a parent and child exchanging messages both ways through two temporary mailboxes, with a common event flag handshake |

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

The output should be the fifteen lines in `mbxpingpong.mar`'s header. The
pair hasn't been run on VMS yet.
