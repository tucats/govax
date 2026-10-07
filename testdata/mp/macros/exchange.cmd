! EXCHANGE.CMD - builds the system service macro probes' exchange volume
! with govax (testdata/mp/macros/README.md). Written by gen.go. Run from
! the repository root:
!
!     govax console < testdata/mp/macros/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-macros.dsk" /DEVICE=RD53 MPMACROS
MOUNT/WRITE DUA1 "testdata/disks/mp-macros.dsk"
COPY "testdata/mp/macros/r5_misc.mar"/HOST DUA1:[000000]R5_MISC.MAR
COPY "testdata/mp/macros/macros.com"/HOST DUA1:[000000]MACROS.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
