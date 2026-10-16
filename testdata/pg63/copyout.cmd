! COPYOUT.CMD - copies Phase 63's probe results off pg63-exchange.dsk into
! the repository (README.md). Run from the repository root after @PG63:
!
!     govax console < testdata/pg63/copyout.cmd
!
MOUNT DUA1 "testdata/disks/pg63-exchange.dsk"
COPY DUA1:[000000]PG63.LOG "testdata/pg63/vax/pg63.log"/HOST/QUIET
COPY DUA1:[000000]PG63A.OUT "testdata/pg63/vax/pg63a.out"/HOST/QUIET
COPY DUA1:[000000]PG63B.OUT "testdata/pg63/vax/pg63b.out"/HOST/QUIET
COPY DUA1:[000000]PG63S.LOG "testdata/pg63/vax/pg63s.log"/HOST/QUIET
COPY DUA1:[000000]PG63_BUILD.LOG "testdata/pg63/vax/pg63_build.log"/HOST/QUIET
COPY DUA1:[000000]R8_PURGWS.OBJ "testdata/mp/macros/vax/r8_purgws.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R8_PURGWS.ANL "testdata/mp/macros/vax/r8_purgws.anl"/HOST/QUIET
COPY DUA1:[000000]MACROS8.LOG "testdata/mp/macros/vax/macros8.log"/HOST/QUIET
DISMOUNT DUA1
