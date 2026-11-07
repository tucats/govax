! COPYOUT.CMD - copies round 1c's output off the exchange volume, into
! testdata/disks/help90-vms/round2/, which git ignores. README.md says how
! the logs are checked before any of them goes into vax/. Round 2's
! terminal log is the host's (simh's SET CONSOLE LOG); put it beside these.
! Written by testdata/help90/gen.go. Run from the repository root:
!
!     govax console < testdata/help90/copyout.cmd
!
MOUNT DUA5 "testdata/disks/help90c-exchange.dsk"
COPY DUA5:[000000]HELP90C.LOG "testdata/disks/help90-vms/round2/help90c.log"/HOST
DISMOUNT DUA5
