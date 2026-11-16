! COPYOUT.CMD - copies the results of the system service macro probes off
! mp-macros.dsk into testdata/mp/macros/vax/ (README.md). Written by gen.go.
! Run from the repository root:
!
!     govax console < testdata/mp/macros/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-macros.dsk"
COPY DUA1:[000000]R5_MISC.OBJ "testdata/mp/macros/vax/r5_misc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R5_MISC.ANL "testdata/mp/macros/vax/r5_misc.anl"/HOST/QUIET
COPY DUA1:[000000]R6_CRMPSC.OBJ "testdata/mp/macros/vax/r6_crmpsc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_CRMPSC.ANL "testdata/mp/macros/vax/r6_crmpsc.anl"/HOST/QUIET
COPY DUA1:[000000]R6_MGBLSC.OBJ "testdata/mp/macros/vax/r6_mgblsc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_MGBLSC.ANL "testdata/mp/macros/vax/r6_mgblsc.anl"/HOST/QUIET
COPY DUA1:[000000]R6_DGBLSC.OBJ "testdata/mp/macros/vax/r6_dgblsc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_DGBLSC.ANL "testdata/mp/macros/vax/r6_dgblsc.anl"/HOST/QUIET
COPY DUA1:[000000]R6_ENQ.OBJ "testdata/mp/macros/vax/r6_enq.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_ENQ.ANL "testdata/mp/macros/vax/r6_enq.anl"/HOST/QUIET
COPY DUA1:[000000]R6_ENQW.OBJ "testdata/mp/macros/vax/r6_enqw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_ENQW.ANL "testdata/mp/macros/vax/r6_enqw.anl"/HOST/QUIET
COPY DUA1:[000000]R6_DEQ.OBJ "testdata/mp/macros/vax/r6_deq.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_DEQ.ANL "testdata/mp/macros/vax/r6_deq.anl"/HOST/QUIET
COPY DUA1:[000000]R6_GETLKI.OBJ "testdata/mp/macros/vax/r6_getlki.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_GETLKI.ANL "testdata/mp/macros/vax/r6_getlki.anl"/HOST/QUIET
COPY DUA1:[000000]R6_GETLKIW.OBJ "testdata/mp/macros/vax/r6_getlkiw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_GETLKIW.ANL "testdata/mp/macros/vax/r6_getlkiw.anl"/HOST/QUIET
COPY DUA1:[000000]R7_LOCK.OBJ "testdata/mp/macros/vax/r7_lock.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R7_LOCK.ANL "testdata/mp/macros/vax/r7_lock.anl"/HOST/QUIET
COPY DUA1:[000000]R8_PURGWS.OBJ "testdata/mp/macros/vax/r8_purgws.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R8_PURGWS.ANL "testdata/mp/macros/vax/r8_purgws.anl"/HOST/QUIET
COPY DUA1:[000000]R9_CHANGE_ACL.OBJ "testdata/mp/macros/vax/r9_change_acl.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R9_CHANGE_ACL.ANL "testdata/mp/macros/vax/r9_change_acl.anl"/HOST/QUIET
COPY DUA1:[000000]R9_PARSE_ACL.OBJ "testdata/mp/macros/vax/r9_parse_acl.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R9_PARSE_ACL.ANL "testdata/mp/macros/vax/r9_parse_acl.anl"/HOST/QUIET
COPY DUA1:[000000]R9_FORMAT_ACL.OBJ "testdata/mp/macros/vax/r9_format_acl.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R9_FORMAT_ACL.ANL "testdata/mp/macros/vax/r9_format_acl.anl"/HOST/QUIET
COPY DUA1:[000000]R9_CHKPRO.OBJ "testdata/mp/macros/vax/r9_chkpro.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R9_CHKPRO.ANL "testdata/mp/macros/vax/r9_chkpro.anl"/HOST/QUIET
COPY DUA1:[000000]R9_CHECK_ACCESS.OBJ "testdata/mp/macros/vax/r9_check_access.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R9_CHECK_ACCESS.ANL "testdata/mp/macros/vax/r9_check_access.anl"/HOST/QUIET
COPY DUA1:[000000]R9_GET_SECURITY.OBJ "testdata/mp/macros/vax/r9_get_security.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R9_GET_SECURITY.ANL "testdata/mp/macros/vax/r9_get_security.anl"/HOST/QUIET
COPY DUA1:[000000]R9_SET_SECURITY.OBJ "testdata/mp/macros/vax/r9_set_security.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R9_SET_SECURITY.ANL "testdata/mp/macros/vax/r9_set_security.anl"/HOST/QUIET
COPY DUA1:[000000]R9_SETDFPROT.OBJ "testdata/mp/macros/vax/r9_setdfprot.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R9_SETDFPROT.ANL "testdata/mp/macros/vax/r9_setdfprot.anl"/HOST/QUIET
COPY DUA1:[000000]R10_SNDJBC.OBJ "testdata/mp/macros/vax/r10_sndjbc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R10_SNDJBC.ANL "testdata/mp/macros/vax/r10_sndjbc.anl"/HOST/QUIET
COPY DUA1:[000000]R10_SNDJBCW.OBJ "testdata/mp/macros/vax/r10_sndjbcw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R10_SNDJBCW.ANL "testdata/mp/macros/vax/r10_sndjbcw.anl"/HOST/QUIET
COPY DUA1:[000000]R10_GETQUI.OBJ "testdata/mp/macros/vax/r10_getqui.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R10_GETQUI.ANL "testdata/mp/macros/vax/r10_getqui.anl"/HOST/QUIET
COPY DUA1:[000000]R10_GETQUIW.OBJ "testdata/mp/macros/vax/r10_getquiw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R10_GETQUIW.ANL "testdata/mp/macros/vax/r10_getquiw.anl"/HOST/QUIET
COPY DUA1:[000000]MACROS5.LOG "testdata/mp/macros/vax/macros5.log"/HOST/QUIET
COPY DUA1:[000000]MACROS6.LOG "testdata/mp/macros/vax/macros6.log"/HOST/QUIET
COPY DUA1:[000000]MACROS7.LOG "testdata/mp/macros/vax/macros7.log"/HOST/QUIET
COPY DUA1:[000000]MACROS8.LOG "testdata/mp/macros/vax/macros8.log"/HOST/QUIET
COPY DUA1:[000000]MACROS9.LOG "testdata/mp/macros/vax/macros9.log"/HOST/QUIET
COPY DUA1:[000000]MACROS10.LOG "testdata/mp/macros/vax/macros10.log"/HOST/QUIET
DISMOUNT DUA1
