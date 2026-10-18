! COPYOUT2.CMD - copies round 2 of Phase 64's probe off pg64-exchange.dsk
! into the repository (README.md). Run from the repository root after
! @PG64X:
!
!     govax console < testdata/pg64/copyout2.cmd
!
MOUNT DUA1 "testdata/disks/pg64-exchange.dsk"
COPY DUA1:[000000]PG64X.LOG "testdata/pg64/vax/round2/pg64x.log"/HOST/QUIET
COPY DUA1:[000000]PG64X.OUT "testdata/pg64/vax/round2/pg64x.out"/HOST/QUIET
COPY DUA1:[000000]PG64M.LOG "testdata/pg64/vax/round2/pg64m.log"/HOST/QUIET
COPY DUA1:[000000]PG64X_BUILD.LOG "testdata/pg64/vax/round2/pg64x_build.log"/HOST/QUIET
DISMOUNT DUA1
