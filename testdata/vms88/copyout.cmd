! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by hand. From the
! repository root, after the VAX has dismounted it:
!
!     mkdir -p testdata/vms88/vax/round2
!     govax console < testdata/vms88/copyout.cmd
!
MOUNT/NOWRITE DUA5 "testdata/disks/vms88-exchange.dsk"
COPY/QUIET DUA5:[000000]VMS88.LOG "testdata/vms88/vax/round2/vms88.log"/HOST
COPY/QUIET/BINARY DUA5:[000000]RESULTS.BCK "testdata/vms88/vax/round2/results.bck"/HOST
COPY/QUIET/BINARY DUA5:[000000]CHANGED.BCK "testdata/vms88/vax/round2/changed.bck"/HOST
COPY/QUIET DUA5:[000000]DISK.LIS "testdata/vms88/vax/round2/disk.lis"/HOST
DISMOUNT DUA5
