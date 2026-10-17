! COPYOUT.CMD - copies Phase 64's probe results off pg64-exchange.dsk into
! the repository (README.md). Run from the repository root after @PG64:
!
!     govax console < testdata/pg64/copyout.cmd
!
MOUNT DUA1 "testdata/disks/pg64-exchange.dsk"
COPY DUA1:[000000]PG64.LOG "testdata/pg64/vax/pg64.log"/HOST/QUIET
COPY DUA1:[000000]PG64Q1.OUT "testdata/pg64/vax/pg64q1.out"/HOST/QUIET
COPY DUA1:[000000]PG64Q2.OUT "testdata/pg64/vax/pg64q2.out"/HOST/QUIET
COPY DUA1:[000000]PG64Q3.OUT "testdata/pg64/vax/pg64q3.out"/HOST/QUIET
COPY DUA1:[000000]PG64C.OUT "testdata/pg64/vax/pg64c.out"/HOST/QUIET
COPY DUA1:[000000]PG64V.OUT "testdata/pg64/vax/pg64v.out"/HOST/QUIET
COPY DUA1:[000000]PG64W.OUT "testdata/pg64/vax/pg64w.out"/HOST/QUIET
COPY DUA1:[000000]PG64S.OUT "testdata/pg64/vax/pg64s.out"/HOST/QUIET
COPY DUA1:[000000]PG64L.LOG "testdata/pg64/vax/pg64l.log"/HOST/QUIET
COPY DUA1:[000000]PG64_BUILD.LOG "testdata/pg64/vax/pg64_build.log"/HOST/QUIET
DISMOUNT DUA1
