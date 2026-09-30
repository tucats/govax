! EXCHANGE.CMD - builds the Phase 28 exchange volume with govax
! (docs/PHASE-28.md, subtask 10). Run from the repository root:
!
!     govax console < testdata/mar/macros/exchange.cmd
!
! It makes testdata/disks/mac-exchange.dsk (RD51 size, label MACXCHG),
! copies the fixtures and MACROS.COM onto it, and writes govax's own
! objects (GV_NAME.OBJ) and libraries (GV_LIBMAC.MLB, GV_LIBOBJ.OLB)
! there, assembling the copies on the volume. GV_QIOW uses whatever
! STARLET.MLB MACRO finds first (govax's own, with no VMS library);
! the rest use VMS's, from the system disk.
!
INITIALIZE/CONTAINER "testdata/disks/mac-exchange.dsk" /DEVICE=RD51 MACXCHG
MOUNT/WRITE DUA1 "testdata/disks/mac-exchange.dsk"
COPY "testdata/mar/macros/usermac.mar"/HOST DUA1:[000000]USERMAC.MAR
COPY "testdata/mar/macros/qiow.mar"/HOST DUA1:[000000]QIOW.MAR
COPY "testdata/mar/macros/rmscopy.mar"/HOST DUA1:[000000]RMSCOPY.MAR
COPY "testdata/mar/macros/fabalign.mar"/HOST DUA1:[000000]FABALIGN.MAR
COPY "testdata/mar/macros/libmac.mar"/HOST DUA1:[000000]LIBMAC.MAR
COPY "testdata/mar/macros/uselib.mar"/HOST DUA1:[000000]USELIB.MAR
COPY "testdata/mar/macros/libsub1.mar"/HOST DUA1:[000000]LIBSUB1.MAR
COPY "testdata/mar/macros/libsub2.mar"/HOST DUA1:[000000]LIBSUB2.MAR
COPY "testdata/mar/macros/libmain.mar"/HOST DUA1:[000000]LIBMAIN.MAR
COPY "testdata/mar/macros/extra.mar"/HOST DUA1:[000000]EXTRA.MAR
COPY "testdata/mar/macros/macros.com"/HOST DUA1:[000000]MACROS.COM
SET DEFAULT DUA1:[000000]
MACRO QIOW/OBJECT=GV_QIOW
MOUNT DUA0 "testdata/disks/rq0-ra92.dsk"
DEFINE SYS$LIBRARY DUA0:[VMS$COMMON.SYSLIB]
MACRO USERMAC/OBJECT=GV_USERMAC
MACRO RMSCOPY/OBJECT=GV_RMSCOPY
MACRO FABALIGN/OBJECT=GV_FABALIGN
MACRO LIBSUB1/OBJECT=GV_LIBSUB1
MACRO LIBSUB2/OBJECT=GV_LIBSUB2
MACRO LIBMAIN/OBJECT=GV_LIBMAIN
LIBRARY GV_LIBMAC LIBMAC/CREATE/MACRO
MACRO USELIB/OBJECT=GV_USELIB/LIBRARY=GV_LIBMAC
LIBRARY GV_LIBOBJ GV_LIBSUB1,GV_LIBSUB2/CREATE
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
DISMOUNT DUA0
