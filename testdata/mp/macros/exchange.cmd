! EXCHANGE.CMD - builds the system service macro probes' exchange volume
! with govax (testdata/mp/macros/README.md). Written by gen.go. Run from
! the repository root:
!
!     govax console < testdata/mp/macros/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-macros.dsk" /DEVICE=RD53 MPMACROS
MOUNT/WRITE DUA1 "testdata/disks/mp-macros.dsk"
COPY "testdata/mp/macros/svc_creprc.mar"/HOST DUA1:[000000]SVC_CREPRC.MAR
COPY "testdata/mp/macros/svc_delprc.mar"/HOST DUA1:[000000]SVC_DELPRC.MAR
COPY "testdata/mp/macros/svc_wake.mar"/HOST DUA1:[000000]SVC_WAKE.MAR
COPY "testdata/mp/macros/svc_hiber.mar"/HOST DUA1:[000000]SVC_HIBER.MAR
COPY "testdata/mp/macros/svc_schdwk.mar"/HOST DUA1:[000000]SVC_SCHDWK.MAR
COPY "testdata/mp/macros/svc_canwak.mar"/HOST DUA1:[000000]SVC_CANWAK.MAR
COPY "testdata/mp/macros/svc_forcex.mar"/HOST DUA1:[000000]SVC_FORCEX.MAR
COPY "testdata/mp/macros/svc_suspnd.mar"/HOST DUA1:[000000]SVC_SUSPND.MAR
COPY "testdata/mp/macros/svc_resume.mar"/HOST DUA1:[000000]SVC_RESUME.MAR
COPY "testdata/mp/macros/svc_setpri.mar"/HOST DUA1:[000000]SVC_SETPRI.MAR
COPY "testdata/mp/macros/svc_setprn.mar"/HOST DUA1:[000000]SVC_SETPRN.MAR
COPY "testdata/mp/macros/svc_getjpi.mar"/HOST DUA1:[000000]SVC_GETJPI.MAR
COPY "testdata/mp/macros/svc_getjpiw.mar"/HOST DUA1:[000000]SVC_GETJPIW.MAR
COPY "testdata/mp/macros/svc_getdvi.mar"/HOST DUA1:[000000]SVC_GETDVI.MAR
COPY "testdata/mp/macros/svc_getdviw.mar"/HOST DUA1:[000000]SVC_GETDVIW.MAR
COPY "testdata/mp/macros/svc_crembx.mar"/HOST DUA1:[000000]SVC_CREMBX.MAR
COPY "testdata/mp/macros/svc_delmbx.mar"/HOST DUA1:[000000]SVC_DELMBX.MAR
COPY "testdata/mp/macros/svc_setimr.mar"/HOST DUA1:[000000]SVC_SETIMR.MAR
COPY "testdata/mp/macros/svc_cantim.mar"/HOST DUA1:[000000]SVC_CANTIM.MAR
COPY "testdata/mp/macros/svc_waitfr.mar"/HOST DUA1:[000000]SVC_WAITFR.MAR
COPY "testdata/mp/macros/svc_setef.mar"/HOST DUA1:[000000]SVC_SETEF.MAR
COPY "testdata/mp/macros/svc_clref.mar"/HOST DUA1:[000000]SVC_CLREF.MAR
COPY "testdata/mp/macros/svc_readef.mar"/HOST DUA1:[000000]SVC_READEF.MAR
COPY "testdata/mp/macros/err_svc.mar"/HOST DUA1:[000000]ERR_SVC.MAR
COPY "testdata/mp/macros/macros.com"/HOST DUA1:[000000]MACROS.COM
COPY "testdata/mp/macros/macroserr.com"/HOST DUA1:[000000]MACROSERR.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
