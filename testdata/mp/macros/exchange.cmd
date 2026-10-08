! EXCHANGE.CMD - builds the system service macro probes' exchange volume
! with govax (testdata/mp/macros/README.md). Written by gen.go. Run from
! the repository root:
!
!     govax console < testdata/mp/macros/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-macros.dsk" /DEVICE=RD53 MPMACROS
MOUNT/WRITE DUA1 "testdata/disks/mp-macros.dsk"
COPY "testdata/mp/macros/r5_misc.mar"/HOST DUA1:[000000]R5_MISC.MAR
COPY "testdata/mp/macros/r6_crmpsc.mar"/HOST DUA1:[000000]R6_CRMPSC.MAR
COPY "testdata/mp/macros/r6_mgblsc.mar"/HOST DUA1:[000000]R6_MGBLSC.MAR
COPY "testdata/mp/macros/r6_dgblsc.mar"/HOST DUA1:[000000]R6_DGBLSC.MAR
COPY "testdata/mp/macros/r6_enq.mar"/HOST DUA1:[000000]R6_ENQ.MAR
COPY "testdata/mp/macros/r6_enqw.mar"/HOST DUA1:[000000]R6_ENQW.MAR
COPY "testdata/mp/macros/r6_deq.mar"/HOST DUA1:[000000]R6_DEQ.MAR
COPY "testdata/mp/macros/r6_getlki.mar"/HOST DUA1:[000000]R6_GETLKI.MAR
COPY "testdata/mp/macros/r6_getlkiw.mar"/HOST DUA1:[000000]R6_GETLKIW.MAR
COPY "testdata/mp/macros/macros.com"/HOST DUA1:[000000]MACROS.COM
COPY "testdata/mp/macros/macros5.com"/HOST DUA1:[000000]MACROS5.COM
COPY "testdata/mp/macros/macros6.com"/HOST DUA1:[000000]MACROS6.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
