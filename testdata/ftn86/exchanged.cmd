! EXCHANGED.CMD - adds DUNGEON to the FORTRAN round's exchange volume, after EXCHANGE.CMD.
! Written by testdata/ftn86/gen.go. Run from the repository root:
!
!     govax console < testdata/ftn86/exchanged.cmd
!
MOUNT/WRITE DUA5: "testdata/disks/ftn86-exchange.dsk"
SET DEFAULT DUA5:[000000]
COPY/QUIET "testdata/ftn86/ftn86d.com"/HOST FTN86D.COM
CREATE/DIRECTORY DUA5:[DUNGEON]
SET DEFAULT DUA5:[DUNGEON]
COPY/QUIET "testdata/disks/dungeon-3.2A/dindx"/HOST DINDX.DAT
COPY/QUIET "testdata/disks/dungeon-3.2A/dtext"/HOST DTEXT.DAT/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=80
COPY/QUIET "testdata/disks/dungeon-3.2A/dparam.for"/HOST DPARAM.FOR
COPY/QUIET "testdata/dungeon86/fixtime.for"/HOST FIXTIME.FOR
COPY/QUIET "testdata/dungeon86/game.in"/HOST GAME.IN
COPY/QUIET "testdata/dungeon86/save.in"/HOST SAVE.IN
COPY/QUIET "testdata/dungeon86/restore.in"/HOST RESTORE.IN
COPY/QUIET "testdata/disks/dungeon-3.2A/dungeon.f"/HOST DUNGEON.F
COPY/QUIET "testdata/disks/dungeon-3.2A/game.f"/HOST GAME.F
COPY/QUIET "testdata/disks/dungeon-3.2A/gdt.f"/HOST GDT.F
COPY/QUIET "testdata/disks/dungeon-3.2A/parser.f"/HOST PARSER.F
COPY/QUIET "testdata/disks/dungeon-3.2A/subr.f"/HOST SUBR.F
COPY/QUIET "testdata/disks/dungeon-3.2A/rooms.f"/HOST ROOMS.F
COPY/QUIET "testdata/disks/dungeon-3.2A/verbs.f"/HOST VERBS.F
COPY/QUIET "testdata/disks/dungeon-3.2A/objects.f"/HOST OBJECTS.F
COPY/QUIET "testdata/disks/dungeon-3.2A/timefnc.f"/HOST TIMEFNC.F
FORT/NOCHECK/CONT=40 DUNGEON.F,GDT.F,PARSER.F,SUBR.F,ROOMS.F,VERBS.F,OBJECTS.F,TIMEFNC.F,GAME.F+FIXTIME.FOR
CREATE/DIRECTORY DUA5:[DUNGEONR]
SET DEFAULT DUA5:[DUNGEONR]
COPY/QUIET "testdata/disks/dungeon-3.2A/dindx"/HOST DINDX.DAT
COPY/QUIET "testdata/disks/dungeon-3.2A/dtext"/HOST DTEXT.DAT/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=80
COPY/QUIET "testdata/disks/dungeon-3.2A/dparam.for"/HOST DPARAM.FOR
COPY/QUIET "testdata/dungeon86/fixtime.for"/HOST FIXTIME.FOR
COPY/QUIET "testdata/dungeon86/game.in"/HOST GAME.IN
COPY/QUIET "testdata/dungeon86/save.in"/HOST SAVE.IN
COPY/QUIET "testdata/dungeon86/restore.in"/HOST RESTORE.IN
COPY/QUIET "testdata/disks/dungeon-3.2A/dungeon.f"/HOST DUNGEON.F
COPY/QUIET "testdata/disks/dungeon-3.2A/game.f"/HOST GAME.F
COPY/QUIET "testdata/disks/dungeon-3.2A/gdt.f"/HOST GDT.F
COPY/QUIET "testdata/disks/dungeon-3.2A/parser.f"/HOST PARSER.F
COPY/QUIET "testdata/disks/dungeon-3.2A/subr.f"/HOST SUBR.F
COPY/QUIET "testdata/disks/dungeon-3.2A/rooms.f"/HOST ROOMS.F
COPY/QUIET "testdata/disks/dungeon-3.2A/verbs.f"/HOST VERBS.F
COPY/QUIET "testdata/disks/dungeon-3.2A/objects.f"/HOST OBJECTS.F
COPY/QUIET "testdata/disks/dungeon-3.2A/timefnc.f"/HOST TIMEFNC.F
FORT/NOCHECK/CONT=40/RECURSIVE DUNGEON.F,GDT.F,PARSER.F,SUBR.F,ROOMS.F,VERBS.F,OBJECTS.F,TIMEFNC.F,GAME.F+FIXTIME.FOR
SET DEFAULT DUA5:[000000]
DISMOUNT DUA5:
EXIT
