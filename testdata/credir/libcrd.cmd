! LIBCRD.CMD - builds the Phase 34 LIB$CREATE_DIR probe's exchange
! volume with govax. Written by testdata/credir/gen.go. Run from the
! repository root:
!
!     govax console < testdata/credir/libcrd.cmd
!
INITIALIZE/CONTAINER "testdata/disks/libcrd-exchange.dsk" /DEVICE=RD51 LIBCRD
MOUNT/WRITE DUA1 "testdata/disks/libcrd-exchange.dsk"
COPY "testdata/credir/libcrd.mar"/HOST DUA1:[000000]LIBCRD.MAR
COPY "testdata/credir/libcrd.com"/HOST DUA1:[000000]LIBCRD.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
