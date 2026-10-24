! EXCHANGE.CMD - builds Phase 72's probe exchange volume with govax
! (README.md). Run from the repository root, with the default
! configuration (whose work volume, DUA0, holds [WORK.DUNGEON]):
!
!     govax console < testdata/legacy72/exchange.cmd
!
! The volume is DUA2, since the startup procedure may mount DUA1. The
! game's files are copied volume to volume, keeping their attributes.
INITIALIZE/CONTAINER "testdata/disks/legacy72-exchange.dsk" /DEVICE=RD51 LEGACY72
MOUNT/WRITE DUA2 "testdata/disks/legacy72-exchange.dsk"
COPY "testdata/legacy72/legacy72.com"/HOST DUA2:[000000]LEGACY72.COM
COPY "testdata/legacy72/dun72.dbg"/HOST DUA2:[000000]DUN72.DBG
COPY DUA0:[WORK.DUNGEON]DUNGEON.EXE DUA2:[000000]DUNGEON.EXE
COPY DUA0:[WORK.DUNGEON]DUNGEON_V552.EXE DUA2:[000000]DUNGEON_V552.EXE
COPY DUA0:[WORK.DUNGEON]DINDX.DAT DUA2:[000000]DINDX.DAT
COPY DUA0:[WORK.DUNGEON]DTEXT.DAT DUA2:[000000]DTEXT.DAT
DIRECTORY/SIZE/DATE DUA2:[000000]
DISMOUNT DUA2
