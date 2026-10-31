! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/copyout.cmd
!
MOUNT DUA2 "testdata/disks/for81-exchange.dsk"
COPY DUA2:[000000]FOR81-VAX.TXT "testdata/for81/vax/for81-vax.txt"/HOST
COPY DUA2:[000000]FOR81R-VAX.TXT "testdata/for81/vax/for81r-vax.txt"/HOST
COPY DUA2:[000000]FOR81.LOG "testdata/for81/vax/for81.log"/HOST
DISMOUNT DUA2
