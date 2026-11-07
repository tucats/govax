! COPYOUT.CMD - copies the VAX run's output off the exchange volume,
! into testdata/disks/help90-vms/round1b/, which git ignores. README.md says how
! the logs are checked before any of them goes into vax/.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/copyout.cmd
!
MOUNT DUA5 "testdata/disks/help90b-exchange.dsk"
COPY DUA5:[000000]HELP90.LOG "testdata/disks/help90-vms/round1b/help90.log"/HOST
COPY DUA5:[000000]H90.BCK "testdata/disks/help90-vms/round1b/h90.bck"/HOST/BINARY
COPY DUA5:[000000]HELP90F.LOG "testdata/disks/help90-vms/round1b/help90f.log"/HOST
COPY DUA5:[000000]DBGH.DLG "testdata/disks/help90-vms/round1b/dbgh.dlg"/HOST
COPY DUA5:[000000]DBGV.DLG "testdata/disks/help90-vms/round1b/dbgv.dlg"/HOST
DISMOUNT DUA5
