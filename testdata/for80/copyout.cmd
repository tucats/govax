! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/for80/gen.go. Run from the repository root:
!
!     govax console < testdata/for80/copyout.cmd
!
MOUNT DUA2 "testdata/disks/for80-exchange.dsk"
COPY DUA2:[000000]FOR80-VAX.TXT "testdata/for80/vax/for80-vax.txt"/HOST
COPY DUA2:[000000]FOR80P.OUT "testdata/for80/vax/for80p-vax.txt"/HOST
COPY DUA2:[000000]FOR80.LOG "testdata/for80/vax/for80.log"/HOST
DISMOUNT DUA2
