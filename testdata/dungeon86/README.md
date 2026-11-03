# The FORTRAN milestone's own files

DUNGEON V3.2A, the milestone of `docs/PHASE-86 - FORTRAN milestone.md`, stays out of
this repository: every file of its kit carries a copyright notice forbidding commercial
use. Its tests (`internal/console/dungeon_test.go`) find it in the author's folder and
skip without it. Only what govax adds is here, and none of it quotes the game.

| File | What it is |
| ---- | ---------- |
| `vmssubr.for` | ITIME, which VAX FORTRAN doesn't have. The kit's own VMSSUBR.FOR supplied it, and is missing from this copy. |

To build it by hand, from a directory holding the kit, in govax's console:

    INITIALIZE/CONTAINER "dun.dsk" /DEVICE=RD54 DUN
    MOUNT DUA1: "dun.dsk"
    SET DEFAULT DUA1:[000000]
    COPY "dindx"/HOST DINDX.DAT
    COPY "dtext"/HOST DTEXT.DAT/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=80
    COPY "dparam.for"/HOST DPARAM.FOR
    COPY "dungeon.f"/HOST DUNGEON.F

and the same for the other sources and `vmssubr.for`, then:

    FORT/NOCHECK/CONT=40 DUNGEON.F,GAME.F,GDT.F,PARSER.F,SUBR.F
    FORT/NOCHECK/CONT=40 ROOMS.F,VERBS.F,OBJECTS.F,TIMEFNC.F,VMSSUBR.FOR
    MACRO DUNGEON,GAME,GDT,PARSER,SUBR,ROOMS,VERBS,OBJECTS,TIMEFNC,VMSSUBR
    LINK DUNGEON,GAME,GDT,PARSER,SUBR,-
    ROOMS,VERBS,OBJECTS,TIMEFNC,VMSSUBR
    RUN DUNGEON
