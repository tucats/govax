! COPYOUT.CMD - copies the results of the multiprocessing program's last
! VAX run off mp-final.dsk into the repository (README.md). Run from the
! repository root after @FINAL:
!
!     govax console < testdata/mp/final/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-final.dsk"
COPY DUA1:[000000]R7_LOCK.OBJ "testdata/mp/macros/vax/r7_lock.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R7_LOCK.ANL "testdata/mp/macros/vax/r7_lock.anl"/HOST/QUIET
COPY DUA1:[000000]MACROS7.LOG "testdata/mp/macros/vax/macros7.log"/HOST/QUIET
COPY DUA1:[000000]PROBE5.LOG "testdata/mp/probe5/vax/probe5.log"/HOST/QUIET
DISMOUNT DUA1
