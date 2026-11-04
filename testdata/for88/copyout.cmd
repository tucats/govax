! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/for88/gen.go. Run from the repository root:
!
!     govax console < testdata/for88/copyout.cmd
!
MOUNT DUA5 "testdata/disks/for88-exchange.dsk"
COPY DUA5:[000000]FOR88-VAX.TXT "testdata/for88/vax/for88-vax.txt"/HOST
COPY DUA5:[000000]FOR88.LOG "testdata/for88/vax/for88.log"/HOST
COPY DUA5:[000000]FOR88.BCK "testdata/for88/vax/for88.bck"/HOST/BINARY
DISMOUNT DUA5
