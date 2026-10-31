! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/mth77/gen.go. Run from the repository root:
!
!     govax console < testdata/mth77/copyout.cmd
!
MOUNT DUA2 "testdata/disks/mth77-exchange.dsk"
COPY DUA2:[000000]MTH77.OUT "testdata/mth77/vax/mth77-vax.txt"/HOST
COPY DUA2:[000000]MTH77.LOG "testdata/mth77/vax/mth77.log"/HOST
DISMOUNT DUA2
