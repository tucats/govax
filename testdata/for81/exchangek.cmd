! EXCHANGEK.CMD - builds round 3b's exchange volume with govax.
! Written by testdata/for81/gen.go. Run from the repository root:
!
!     govax console < testdata/for81/exchangek.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for81k-exchange.dsk" /DEVICE=RD54 FOR81K
MOUNT/WRITE DUA2 "testdata/disks/for81k-exchange.dsk"
COPY "testdata/for81/for81k.mar"/HOST DUA2:[000000]FOR81K.MAR
COPY "testdata/for81/for81w.mar"/HOST DUA2:[000000]FOR81W.MAR
COPY "testdata/for81/for81.opt"/HOST DUA2:[000000]FOR81.OPT
COPY "testdata/for81/for81k.com"/HOST DUA2:[000000]FOR81K.COM
COPY "testdata/for81/for81l.com"/HOST DUA2:[000000]FOR81L.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
