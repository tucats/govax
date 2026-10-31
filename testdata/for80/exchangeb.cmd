! EXCHANGEB.CMD - builds the Phase 80 probe's second exchange volume with govax.
! Written by testdata/for80/gen.go. Run from the repository root:
!
!     govax console < testdata/for80/exchangeb.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for80b-exchange.dsk" /DEVICE=RD51 FOR80B
MOUNT/WRITE DUA2 "testdata/disks/for80b-exchange.dsk"
COPY "testdata/for80/for80b.mar"/HOST DUA2:[000000]FOR80B.MAR
COPY "testdata/for80/for80d.mar"/HOST DUA2:[000000]FOR80D.MAR
COPY "testdata/for80/for80.opt"/HOST DUA2:[000000]FOR80.OPT
COPY "testdata/for80/for80br.com"/HOST DUA2:[000000]FOR80BR.COM
COPY "testdata/for80/for80b.com"/HOST DUA2:[000000]FOR80B.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
