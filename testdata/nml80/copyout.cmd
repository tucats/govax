! COPYOUT.CMD - copies the VAX's run off the exchange volume.
! Written by testdata/nml80/gen.go. Run from the repository root:
!
!     govax console < testdata/nml80/copyout.cmd
!
MOUNT DUA2 "testdata/disks/nml80-exchange.dsk"
COPY DUA2:[000000]NML80-VAX.TXT "testdata/nml80/vax/nml80-vax.txt"/HOST
COPY DUA2:[000000]NML80E-VAX.TXT "testdata/nml80/vax/nml80e-vax.txt"/HOST
COPY DUA2:[000000]NML80.LOG "testdata/nml80/vax/nml80.log"/HOST
DISMOUNT DUA2
