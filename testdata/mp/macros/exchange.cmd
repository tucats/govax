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
COPY "testdata/mp/macros/r7_lock.mar"/HOST DUA1:[000000]R7_LOCK.MAR
COPY "testdata/mp/macros/r8_purgws.mar"/HOST DUA1:[000000]R8_PURGWS.MAR
COPY "testdata/mp/macros/r9_change_acl.mar"/HOST DUA1:[000000]R9_CHANGE_ACL.MAR
COPY "testdata/mp/macros/r9_parse_acl.mar"/HOST DUA1:[000000]R9_PARSE_ACL.MAR
COPY "testdata/mp/macros/r9_format_acl.mar"/HOST DUA1:[000000]R9_FORMAT_ACL.MAR
COPY "testdata/mp/macros/r9_chkpro.mar"/HOST DUA1:[000000]R9_CHKPRO.MAR
COPY "testdata/mp/macros/r9_check_access.mar"/HOST DUA1:[000000]R9_CHECK_ACCESS.MAR
COPY "testdata/mp/macros/r9_get_security.mar"/HOST DUA1:[000000]R9_GET_SECURITY.MAR
COPY "testdata/mp/macros/r9_set_security.mar"/HOST DUA1:[000000]R9_SET_SECURITY.MAR
COPY "testdata/mp/macros/r9_setdfprot.mar"/HOST DUA1:[000000]R9_SETDFPROT.MAR
COPY "testdata/mp/macros/r10_sndjbc.mar"/HOST DUA1:[000000]R10_SNDJBC.MAR
COPY "testdata/mp/macros/r10_sndjbcw.mar"/HOST DUA1:[000000]R10_SNDJBCW.MAR
COPY "testdata/mp/macros/r10_getqui.mar"/HOST DUA1:[000000]R10_GETQUI.MAR
COPY "testdata/mp/macros/r10_getquiw.mar"/HOST DUA1:[000000]R10_GETQUIW.MAR
COPY "testdata/mp/macros/macros.com"/HOST DUA1:[000000]MACROS.COM
COPY "testdata/mp/macros/macros5.com"/HOST DUA1:[000000]MACROS5.COM
COPY "testdata/mp/macros/macros6.com"/HOST DUA1:[000000]MACROS6.COM
COPY "testdata/mp/macros/macros7.com"/HOST DUA1:[000000]MACROS7.COM
COPY "testdata/mp/macros/macros8.com"/HOST DUA1:[000000]MACROS8.COM
COPY "testdata/mp/macros/macros9.com"/HOST DUA1:[000000]MACROS9.COM
COPY "testdata/mp/macros/macros10.com"/HOST DUA1:[000000]MACROS10.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
