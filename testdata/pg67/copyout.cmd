! COPYOUT.CMD - copies Phase 67's probe results off pg67-exchange.dsk into
! the repository (README.md). Run from the repository root after @PG67:
!
!     govax console < testdata/pg67/copyout.cmd
!
MOUNT DUA1 "testdata/disks/pg67-exchange.dsk"
COPY DUA1:[000000]PG67.LOG "testdata/pg67/vax/pg67.log"/HOST/QUIET
COPY DUA1:[000000]PG67A.OUT "testdata/pg67/vax/pg67a.out"/HOST/QUIET
COPY DUA1:[000000]PG67S.OUT "testdata/pg67/vax/pg67s.out"/HOST/QUIET
COPY DUA1:[000000]PG67T.OUT "testdata/pg67/vax/pg67t.out"/HOST/QUIET
COPY DUA1:[000000]PG67T50.OUT "testdata/pg67/vax/pg67t50.out"/HOST/QUIET
COPY DUA1:[000000]PG67C.OUT "testdata/pg67/vax/pg67c.out"/HOST/QUIET
COPY DUA1:[000000]PG67Q.OUT "testdata/pg67/vax/pg67q.out"/HOST/QUIET
COPY DUA1:[000000]PG67_BUILD.LOG "testdata/pg67/vax/pg67_build.log"/HOST/QUIET
COPY DUA1:[000000]PG67L1.LOG "testdata/pg67/vax/pg67l1.log"/HOST/QUIET
COPY DUA1:[000000]PG67L2.LOG "testdata/pg67/vax/pg67l2.log"/HOST/QUIET
COPY DUA1:[000000]PG67.MAP "testdata/pg67/vax/pg67.map"/HOST/QUIET
COPY DUA1:[000000]PG67S.MAP "testdata/pg67/vax/pg67s.map"/HOST/QUIET
COPY DUA1:[000000]PG67.OBJ "testdata/pg67/vax/pg67.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]PG67.EXE "testdata/pg67/vax/pg67.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]PG67S.EXE "testdata/pg67/vax/pg67s.exe"/HOST/BINARY/QUIET
DISMOUNT DUA1
