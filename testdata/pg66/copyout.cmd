! COPYOUT.CMD - copies Phase 66's probe results off pg66-exchange.dsk into
! the repository (README.md). Run from the repository root after @PG66:
!
!     govax console < testdata/pg66/copyout.cmd
!
MOUNT DUA1 "testdata/disks/pg66-exchange.dsk"
COPY DUA1:[000000]PG66.LOG "testdata/pg66/vax/pg66.log"/HOST/QUIET
COPY DUA1:[000000]PG66R1.OUT "testdata/pg66/vax/pg66r1.out"/HOST/QUIET
COPY DUA1:[000000]PG66R2.OUT "testdata/pg66/vax/pg66r2.out"/HOST/QUIET
COPY DUA1:[000000]PG66R3.OUT "testdata/pg66/vax/pg66r3.out"/HOST/QUIET
COPY DUA1:[000000]PG66W.OUT "testdata/pg66/vax/pg66w.out"/HOST/QUIET
COPY DUA1:[000000]PG66D.OUT "testdata/pg66/vax/pg66d.out"/HOST/QUIET
COPY DUA1:[000000]PG66_BUILD.LOG "testdata/pg66/vax/pg66_build.log"/HOST/QUIET
COPY DUA1:[000000]PG66L1.LOG "testdata/pg66/vax/pg66l1.log"/HOST/QUIET
COPY DUA1:[000000]PG66L2.LOG "testdata/pg66/vax/pg66l2.log"/HOST/QUIET
COPY DUA1:[000000]PG66.MAP "testdata/pg66/vax/pg66.map"/HOST/QUIET
COPY DUA1:[000000]PG66C.MAP "testdata/pg66/vax/pg66c.map"/HOST/QUIET
COPY DUA1:[000000]PG66.OBJ "testdata/pg66/vax/pg66.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]PG66.EXE "testdata/pg66/vax/pg66.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]PG66C.EXE "testdata/pg66/vax/pg66c.exe"/HOST/BINARY/QUIET
DISMOUNT DUA1
