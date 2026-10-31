! COPYOUTC.CMD - copies round 2's output off the exchange volume.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/copyoutc.cmd
!
MOUNT DUA2 "testdata/disks/for81c-exchange.dsk"
COPY DUA2:[000000]FOR81C-VAX.TXT "testdata/for81/vax/for81c-vax.txt"/HOST
COPY DUA2:[000000]FOR81D-VAX.TXT "testdata/for81/vax/for81d-vax.txt"/HOST
COPY DUA2:[000000]FOR81C.LOG "testdata/for81/vax/for81c.log"/HOST
DISMOUNT DUA2
