! EXCHANGEC.CMD - builds round 2's exchange volume with govax.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/exchangec.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for81c-exchange.dsk" /DEVICE=RD54 FOR81C
MOUNT/WRITE DUA2 "testdata/disks/for81c-exchange.dsk"
COPY "testdata/for81/for81c.mar"/HOST DUA2:[000000]FOR81C.MAR
COPY "testdata/for81/for81d.mar"/HOST DUA2:[000000]FOR81D.MAR
COPY "testdata/for81/for81x.mar"/HOST DUA2:[000000]FOR81X.MAR
COPY "testdata/for81/for81.opt"/HOST DUA2:[000000]FOR81.OPT
COPY "testdata/for81/for81c.com"/HOST DUA2:[000000]FOR81C.COM
COPY "testdata/for81/for81e.com"/HOST DUA2:[000000]FOR81E.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
