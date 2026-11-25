! COPYOUTK.CMD - copies round 3b's output off its exchange volume.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/copyoutk.cmd
!
MOUNT DUA2 "testdata/disks/for81k-exchange.dsk"
COPY DUA2:[000000]FOR81K-VAX.TXT "testdata/for81/vax/for81k-vax.txt"/HOST
COPY DUA2:[000000]FOR81K.LOG "testdata/for81/vax/for81k.log"/HOST
DISMOUNT DUA2
