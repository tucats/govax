! CELLS-OUT.CMD - copies the fixup cell follow-up's results into
! testdata/mar/round/vax/ (docs/PHASE-29.md, subtask 14). Run from the
! repository root after @CELLS/OUTPUT=CELLS.LOG:
!
!     govax console < testdata/mar/round/cells-out.cmd
!
MOUNT DUA1 "testdata/disks/round-exchange.dsk"
COPY DUA1:[000000]CELLS.LOG "testdata/mar/round/vax/cells.log"/HOST/QUIET
COPY DUA1:[000000]CELLS.LIS "testdata/mar/round/vax/cells.lis"/HOST/QUIET
COPY DUA1:[000000]CELLS.OBJ "testdata/mar/round/vax/cells.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]CELLSB.LIS "testdata/mar/round/vax/cellsb.lis"/HOST/QUIET
COPY DUA1:[000000]CELLSB.OBJ "testdata/mar/round/vax/cellsb.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]CELLS.EXE "testdata/mar/round/vax/cells.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]CELLS.MAP "testdata/mar/round/vax/cells.map"/HOST/QUIET
COPY DUA1:[000000]CELLS.ANI "testdata/mar/round/vax/cells.ani"/HOST/QUIET
COPY DUA1:[000000]CELLS2.EXE "testdata/mar/round/vax/cells2.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]CELLS2.MAP "testdata/mar/round/vax/cells2.map"/HOST/QUIET
COPY DUA1:[000000]CELLS2.ANI "testdata/mar/round/vax/cells2.ani"/HOST/QUIET
DISMOUNT DUA1
