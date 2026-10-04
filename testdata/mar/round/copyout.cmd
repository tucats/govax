! COPYOUT.CMD - copies the results of Phase 29's VMS round off
! round-exchange.dsk into testdata/mar/round/vax/ (docs/PHASE-29.md,
! subtask 14). Run from the repository root after @ROUND/OUTPUT=ROUND.LOG:
!
!     govax console < testdata/mar/round/copyout.cmd
!
MOUNT DUA1 "testdata/disks/round-exchange.dsk"
COPY DUA1:[000000]GVLCTL.ANL "testdata/mar/round/vax/gvlctl.anl"/HOST/QUIET
COPY DUA1:[000000]GVBINARY.ANL "testdata/mar/round/vax/gvbinary.anl"/HOST/QUIET
COPY DUA1:[000000]GVSYMTAB.ANL "testdata/mar/round/vax/gvsymtab.anl"/HOST/QUIET
COPY DUA1:[000000]GVNOTITLE.ANL "testdata/mar/round/vax/gvnotitle.anl"/HOST/QUIET
COPY DUA1:[000000]GVNOTITLE2.ANL "testdata/mar/round/vax/gvnotitle2.anl"/HOST/QUIET
COPY DUA1:[000000]GVNOTITLE3.ANL "testdata/mar/round/vax/gvnotitle3.anl"/HOST/QUIET
COPY DUA1:[000000]GVXREF.ANL "testdata/mar/round/vax/gvxref.anl"/HOST/QUIET
COPY DUA1:[000000]GVTRACE.ANL "testdata/mar/round/vax/gvtrace.anl"/HOST/QUIET
COPY DUA1:[000000]GVFAILMAIN.ANL "testdata/mar/round/vax/gvfailmain.anl"/HOST/QUIET
COPY DUA1:[000000]GVFAILSUB.ANL "testdata/mar/round/vax/gvfailsub.anl"/HOST/QUIET
COPY DUA1:[000000]GVFAILSIG.ANL "testdata/mar/round/vax/gvfailsig.anl"/HOST/QUIET
COPY DUA1:[000000]GVTRACE.ANI "testdata/mar/round/vax/gvtrace.ani"/HOST/QUIET
COPY DUA1:[000000]GVTRNOTB.ANI "testdata/mar/round/vax/gvtrnotb.ani"/HOST/QUIET
COPY DUA1:[000000]GVFAIL.ANI "testdata/mar/round/vax/gvfail.ani"/HOST/QUIET
COPY DUA1:[000000]GVFAILNT.ANI "testdata/mar/round/vax/gvfailnt.ani"/HOST/QUIET
COPY DUA1:[000000]GVFSIG.ANI "testdata/mar/round/vax/gvfsig.ani"/HOST/QUIET
COPY DUA1:[000000]GVFSIGNT.ANI "testdata/mar/round/vax/gvfsignt.ani"/HOST/QUIET
COPY DUA1:[000000]RLTRACE.ANI "testdata/mar/round/vax/rltrace.ani"/HOST/QUIET
COPY DUA1:[000000]RLTRNOTB.ANI "testdata/mar/round/vax/rltrnotb.ani"/HOST/QUIET
COPY DUA1:[000000]RLFAIL.ANI "testdata/mar/round/vax/rlfail.ani"/HOST/QUIET
COPY DUA1:[000000]RLFAILNT.ANI "testdata/mar/round/vax/rlfailnt.ani"/HOST/QUIET
COPY DUA1:[000000]RLFSIG.ANI "testdata/mar/round/vax/rlfsig.ani"/HOST/QUIET
COPY DUA1:[000000]RLFSIGNT.ANI "testdata/mar/round/vax/rlfsignt.ani"/HOST/QUIET
COPY DUA1:[000000]RLTRACE.EXE "testdata/mar/round/vax/rltrace.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]RLTRACE.MAP "testdata/mar/round/vax/rltrace.map"/HOST/QUIET
COPY DUA1:[000000]RLTRNOTB.EXE "testdata/mar/round/vax/rltrnotb.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]RLTRNOTB.MAP "testdata/mar/round/vax/rltrnotb.map"/HOST/QUIET
COPY DUA1:[000000]RLFAIL.EXE "testdata/mar/round/vax/rlfail.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]RLFAIL.MAP "testdata/mar/round/vax/rlfail.map"/HOST/QUIET
COPY DUA1:[000000]RLFAILNT.EXE "testdata/mar/round/vax/rlfailnt.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]RLFAILNT.MAP "testdata/mar/round/vax/rlfailnt.map"/HOST/QUIET
COPY DUA1:[000000]RLFSIG.EXE "testdata/mar/round/vax/rlfsig.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]RLFSIG.MAP "testdata/mar/round/vax/rlfsig.map"/HOST/QUIET
COPY DUA1:[000000]RLFSIGNT.EXE "testdata/mar/round/vax/rlfsignt.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]RLFSIGNT.MAP "testdata/mar/round/vax/rlfsignt.map"/HOST/QUIET
COPY DUA1:[000000]NOTITLE2.ANL "testdata/mar/round/vax/notitle2.anl"/HOST/QUIET
COPY DUA1:[000000]NOTITLE2.LIS "testdata/mar/round/vax/notitle2.lis"/HOST/QUIET
COPY DUA1:[000000]NOTITLE2.OBJ "testdata/mar/round/vax/notitle2.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]NOTITLE3.ANL "testdata/mar/round/vax/notitle3.anl"/HOST/QUIET
COPY DUA1:[000000]NOTITLE3.LIS "testdata/mar/round/vax/notitle3.lis"/HOST/QUIET
COPY DUA1:[000000]NOTITLE3.OBJ "testdata/mar/round/vax/notitle3.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]ROUND.LOG "testdata/mar/round/vax/round.log"/HOST/QUIET
DISMOUNT DUA1
