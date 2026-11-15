! COPYOUT.CMD - copies VMS's results off Phase 93's exchange volume. Written by hand. From
! the repository root, after the VAX has dismounted it:
!
!     govax console < testdata/vms93/copyout.cmd
!
MOUNT/NOWRITE DUA5 "testdata/disks/vms93-exchange.dsk"
COPY/QUIET DUA5:[000000]VMS93.LOG "testdata/vms93/vax/vms93.log"/HOST
COPY/QUIET DUA5:[000000]DISK.LIS "testdata/vms93/vax/disk.lis"/HOST
COPY DUA5:[000000]VMS93C.EXE "testdata/vms93/vax/vms93c.exe"/HOST/BINARY/QUIET
COPY DUA5:[000000]VMS93U.EXE "testdata/vms93/vax/vms93u.exe"/HOST/BINARY/QUIET
DISMOUNT DUA5
