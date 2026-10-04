! CELLS-IN.CMD - copies the fixup cell follow-up onto round-exchange.dsk
! (docs/PHASE-29.md, subtask 14). Run from the repository root, with simh
! paused or the disk detached:
!
!     govax console < testdata/mar/round/cells-in.cmd
!
MOUNT/WRITE DUA1 "testdata/disks/round-exchange.dsk"
COPY "testdata/mar/round/cells.mar"/HOST DUA1:[000000]CELLS.MAR
COPY "testdata/mar/round/cellsb.mar"/HOST DUA1:[000000]CELLSB.MAR
COPY "testdata/mar/round/cells.com"/HOST DUA1:[000000]CELLS.COM
DIRECTORY DUA1:[000000]CELLS*.*
DISMOUNT DUA1
