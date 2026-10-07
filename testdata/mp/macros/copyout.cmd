! COPYOUT.CMD - copies the results of the system service macro probes off
! mp-macros.dsk into testdata/mp/macros/vax/ (README.md). Written by gen.go.
! Run from the repository root:
!
!     govax console < testdata/mp/macros/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-macros.dsk"
COPY DUA1:[000000]SVC_CREPRC.OBJ "testdata/mp/macros/vax/svc_creprc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_CREPRC.ANL "testdata/mp/macros/vax/svc_creprc.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_DELPRC.OBJ "testdata/mp/macros/vax/svc_delprc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_DELPRC.ANL "testdata/mp/macros/vax/svc_delprc.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_WAKE.OBJ "testdata/mp/macros/vax/svc_wake.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_WAKE.ANL "testdata/mp/macros/vax/svc_wake.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_HIBER.OBJ "testdata/mp/macros/vax/svc_hiber.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_HIBER.ANL "testdata/mp/macros/vax/svc_hiber.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_SCHDWK.OBJ "testdata/mp/macros/vax/svc_schdwk.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_SCHDWK.ANL "testdata/mp/macros/vax/svc_schdwk.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_CANWAK.OBJ "testdata/mp/macros/vax/svc_canwak.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_CANWAK.ANL "testdata/mp/macros/vax/svc_canwak.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_FORCEX.OBJ "testdata/mp/macros/vax/svc_forcex.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_FORCEX.ANL "testdata/mp/macros/vax/svc_forcex.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_SUSPND.OBJ "testdata/mp/macros/vax/svc_suspnd.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_SUSPND.ANL "testdata/mp/macros/vax/svc_suspnd.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_RESUME.OBJ "testdata/mp/macros/vax/svc_resume.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_RESUME.ANL "testdata/mp/macros/vax/svc_resume.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_SETPRI.OBJ "testdata/mp/macros/vax/svc_setpri.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_SETPRI.ANL "testdata/mp/macros/vax/svc_setpri.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_SETPRN.OBJ "testdata/mp/macros/vax/svc_setprn.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_SETPRN.ANL "testdata/mp/macros/vax/svc_setprn.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_GETJPI.OBJ "testdata/mp/macros/vax/svc_getjpi.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_GETJPI.ANL "testdata/mp/macros/vax/svc_getjpi.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_GETJPIW.OBJ "testdata/mp/macros/vax/svc_getjpiw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_GETJPIW.ANL "testdata/mp/macros/vax/svc_getjpiw.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_GETDVI.OBJ "testdata/mp/macros/vax/svc_getdvi.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_GETDVI.ANL "testdata/mp/macros/vax/svc_getdvi.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_GETDVIW.OBJ "testdata/mp/macros/vax/svc_getdviw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_GETDVIW.ANL "testdata/mp/macros/vax/svc_getdviw.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_CREMBX.OBJ "testdata/mp/macros/vax/svc_crembx.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_CREMBX.ANL "testdata/mp/macros/vax/svc_crembx.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_DELMBX.OBJ "testdata/mp/macros/vax/svc_delmbx.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_DELMBX.ANL "testdata/mp/macros/vax/svc_delmbx.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_SETIMR.OBJ "testdata/mp/macros/vax/svc_setimr.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_SETIMR.ANL "testdata/mp/macros/vax/svc_setimr.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_CANTIM.OBJ "testdata/mp/macros/vax/svc_cantim.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_CANTIM.ANL "testdata/mp/macros/vax/svc_cantim.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_WAITFR.OBJ "testdata/mp/macros/vax/svc_waitfr.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_WAITFR.ANL "testdata/mp/macros/vax/svc_waitfr.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_SETEF.OBJ "testdata/mp/macros/vax/svc_setef.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_SETEF.ANL "testdata/mp/macros/vax/svc_setef.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_CLREF.OBJ "testdata/mp/macros/vax/svc_clref.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_CLREF.ANL "testdata/mp/macros/vax/svc_clref.anl"/HOST/QUIET
COPY DUA1:[000000]SVC_READEF.OBJ "testdata/mp/macros/vax/svc_readef.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SVC_READEF.ANL "testdata/mp/macros/vax/svc_readef.anl"/HOST/QUIET
COPY DUA1:[000000]ERR_SVC.OBJ "testdata/mp/macros/vax/err_svc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]MACROS.LOG "testdata/mp/macros/vax/macros.log"/HOST/QUIET
COPY DUA1:[000000]ERRORS.LOG "testdata/mp/macros/vax/errors.log"/HOST/QUIET
DISMOUNT DUA1
