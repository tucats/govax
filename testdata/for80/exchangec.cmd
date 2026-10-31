! EXCHANGEC.CMD - builds an exchange volume for the Phase 80 probe's third round,
! which has run only on VMS's FORRTL.EXE over govax. Written by
! testdata/for80/gen.go. Run from the repository root:
!
!     govax console < testdata/for80/exchangec.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for80c-exchange.dsk" /DEVICE=RD51 FOR80C
MOUNT/WRITE DUA2 "testdata/disks/for80c-exchange.dsk"
COPY "testdata/for80/for80c.mar"/HOST DUA2:[000000]FOR80C.MAR
COPY "testdata/for80/for80d.mar"/HOST DUA2:[000000]FOR80D.MAR
COPY "testdata/for80/for80.opt"/HOST DUA2:[000000]FOR80.OPT
COPY "testdata/for80/for80c.com"/HOST DUA2:[000000]FOR80C.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
