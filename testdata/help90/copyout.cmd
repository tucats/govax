! COPYOUT.CMD - copies round 3's output off the exchange volume, into
! testdata/disks/help90-vms/round3/, which git ignores. README.md says how
! the log is checked before it goes into vax/.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/copyout.cmd
!
MOUNT DUA5 "testdata/disks/help90w-exchange.dsk"
COPY DUA5:[000000]HELP90W.LOG "testdata/disks/help90-vms/round3/help90w.log"/HOST
COPY DUA5:[000000]H90W.BCK "testdata/disks/help90-vms/round3/h90w.bck"/HOST/BINARY
DISMOUNT DUA5
