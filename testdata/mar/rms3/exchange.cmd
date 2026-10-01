! EXCHANGE.CMD - builds the Phase 33 oracle's exchange volume with govax.
! Written by testdata/mar/rms3/gen.go. Run from the repository root:
!
!     govax console < testdata/mar/rms3/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/rms3-exchange.dsk" /DEVICE=RD51 RMSXCHG3
MOUNT/WRITE DUA1 "testdata/disks/rms3-exchange.dsk"
COPY "testdata/mar/rms3/parse.mar"/HOST DUA1:[000000]PARSE.MAR
COPY "testdata/mar/rms3/search.mar"/HOST DUA1:[000000]SEARCH.MAR
COPY "testdata/mar/rms3/open.mar"/HOST DUA1:[000000]OPEN.MAR
COPY "testdata/mar/rms3/xab.mar"/HOST DUA1:[000000]XAB.MAR
COPY "testdata/mar/rms3/namfid.mar"/HOST DUA1:[000000]NAMFID.MAR
COPY "testdata/mar/rms3/create.mar"/HOST DUA1:[000000]CREATE.MAR
COPY "testdata/mar/rms3/build.com"/HOST DUA1:[000000]BUILD.COM
COPY "testdata/mar/rms3/run.com"/HOST DUA1:[000000]RUN.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
