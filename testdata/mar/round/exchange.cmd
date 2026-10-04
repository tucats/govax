! EXCHANGE.CMD - builds the exchange volume for Phase 29's VMS round
! (docs/PHASE-29.md, subtask 14) with govax. Run from the repository root:
!
!     govax console < testdata/mar/round/exchange.cmd
!
! It makes testdata/disks/round-exchange.dsk (RD51 size, label ROUNDXCHG,
! gitignored), copies the sources and ROUND.COM onto it, and has govax
! assemble each source into GV<name>.OBJ and link the three programs into
! GV<name>.EXE, all on the volume, for ROUND.COM to check on VMS. The
! probe's sources come from testdata/mar/list; NOTITLE2 and NOTITLE3 are
! this directory's. No source calls a system macro, so nothing on the
! volume comes from VMS's libraries.
!
INITIALIZE/CONTAINER "testdata/disks/round-exchange.dsk" /DEVICE=RD51 ROUNDXCHG
MOUNT/WRITE DUA1 "testdata/disks/round-exchange.dsk"
!
! The sources, and ROUND.COM.
!
COPY "testdata/mar/list/lctl.mar"/HOST DUA1:[000000]LCTL.MAR
COPY "testdata/mar/list/binary.mar"/HOST DUA1:[000000]BINARY.MAR
COPY "testdata/mar/list/symtab.mar"/HOST DUA1:[000000]SYMTAB.MAR
COPY "testdata/mar/list/notitle.mar"/HOST DUA1:[000000]NOTITLE.MAR
COPY "testdata/mar/list/xref.mar"/HOST DUA1:[000000]XREF.MAR
COPY "testdata/mar/list/trace.mar"/HOST DUA1:[000000]TRACE.MAR
COPY "testdata/mar/list/failmain.mar"/HOST DUA1:[000000]FAILMAIN.MAR
COPY "testdata/mar/list/failsub.mar"/HOST DUA1:[000000]FAILSUB.MAR
COPY "testdata/mar/list/failsig.mar"/HOST DUA1:[000000]FAILSIG.MAR
COPY "testdata/mar/round/notitle2.mar"/HOST DUA1:[000000]NOTITLE2.MAR
COPY "testdata/mar/round/notitle3.mar"/HOST DUA1:[000000]NOTITLE3.MAR
COPY "testdata/mar/round/round.com"/HOST DUA1:[000000]ROUND.COM
!
! govax's objects: GV<name>.OBJ, beside the source.
!
MACRO DUA1:[000000]LCTL.MAR/OBJECT=GVLCTL
MACRO DUA1:[000000]BINARY.MAR/OBJECT=GVBINARY
MACRO DUA1:[000000]SYMTAB.MAR/OBJECT=GVSYMTAB
MACRO DUA1:[000000]NOTITLE.MAR/OBJECT=GVNOTITLE
MACRO DUA1:[000000]XREF.MAR/OBJECT=GVXREF
MACRO DUA1:[000000]TRACE.MAR/OBJECT=GVTRACE
MACRO DUA1:[000000]FAILMAIN.MAR/OBJECT=GVFAILMAIN
MACRO DUA1:[000000]FAILSUB.MAR/OBJECT=GVFAILSUB
MACRO DUA1:[000000]FAILSIG.MAR/OBJECT=GVFAILSIG
MACRO DUA1:[000000]NOTITLE2.MAR/OBJECT=GVNOTITLE2
MACRO DUA1:[000000]NOTITLE3.MAR/OBJECT=GVNOTITLE3
!
! govax's images, as the probe's list.com linked real MACRO's objects:
! with traceback (the default) and with /NOTRACEBACK.
!
LINK DUA1:[000000]GVTRACE.OBJ/EXECUTABLE=DUA1:[000000]GVTRACE.EXE/MAP=DUA1:[000000]GVTRACE.MAP
LINK DUA1:[000000]GVTRACE.OBJ/EXECUTABLE=DUA1:[000000]GVTRNOTB.EXE/MAP=DUA1:[000000]GVTRNOTB.MAP/NOTRACEBACK
LINK DUA1:[000000]GVFAILMAIN.OBJ,DUA1:[000000]GVFAILSUB.OBJ/EXECUTABLE=DUA1:[000000]GVFAIL.EXE/MAP=DUA1:[000000]GVFAIL.MAP
LINK DUA1:[000000]GVFAILMAIN.OBJ,DUA1:[000000]GVFAILSUB.OBJ/EXECUTABLE=DUA1:[000000]GVFAILNT.EXE/MAP=DUA1:[000000]GVFAILNT.MAP/NOTRACEBACK
LINK DUA1:[000000]GVFAILSIG.OBJ/EXECUTABLE=DUA1:[000000]GVFSIG.EXE/MAP=DUA1:[000000]GVFSIG.MAP
LINK DUA1:[000000]GVFAILSIG.OBJ/EXECUTABLE=DUA1:[000000]GVFSIGNT.EXE/MAP=DUA1:[000000]GVFSIGNT.MAP/NOTRACEBACK
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
