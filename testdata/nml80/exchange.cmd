! EXCHANGE.CMD - builds the NAMELIST probe's exchange volume with govax.
! Written by testdata/nml80/gen.go. Run from the repository root:
!
!     govax console < testdata/nml80/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/nml80-exchange.dsk" /DEVICE=RD51 NML80
MOUNT/WRITE DUA2 "testdata/disks/nml80-exchange.dsk"
COPY "testdata/nml80/nml80.mar"/HOST DUA2:[000000]NML80.MAR
COPY "testdata/nml80/nml80r.com"/HOST DUA2:[000000]NML80R.COM
COPY "testdata/nml80/nml80e.mar"/HOST DUA2:[000000]NML80E.MAR
COPY "testdata/nml80/nml80e.com"/HOST DUA2:[000000]NML80E.COM
COPY "testdata/nml80/nml80.opt"/HOST DUA2:[000000]NML80.OPT
COPY "testdata/nml80/nml80.com"/HOST DUA2:[000000]NML80.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
