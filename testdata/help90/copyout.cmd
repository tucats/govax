! COPYOUT.CMD - copies round 3b's output off the exchange volume, into
! testdata/disks/help90-vms/round3b/, which git ignores. README.md says how
! the log is checked before it goes into vax/.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/copyout.cmd
!
MOUNT DUA5 "testdata/disks/help90x-exchange.dsk"
COPY DUA5:[000000]HELP90X.LOG "testdata/disks/help90-vms/round3b/help90x.log"/HOST
COPY DUA5:[000000]H90X.BCK "testdata/disks/help90-vms/round3b/h90x.bck"/HOST/BINARY
DISMOUNT DUA5
