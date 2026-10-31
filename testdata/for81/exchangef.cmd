! EXCHANGEF.CMD - builds the volume for round 2's FORRTL program, run again.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/exchangef.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for81f-exchange.dsk" /DEVICE=RD54 FOR81F
MOUNT/WRITE DUA2 "testdata/disks/for81f-exchange.dsk"
COPY "testdata/for81/for81c.mar"/HOST DUA2:[000000]FOR81C.MAR
COPY "testdata/for81/for81.opt"/HOST DUA2:[000000]FOR81.OPT
COPY "testdata/for81/for81f.com"/HOST DUA2:[000000]FOR81F.COM
COPY "testdata/for81/for81e.com"/HOST DUA2:[000000]FOR81E.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
