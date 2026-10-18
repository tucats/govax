! COPYOUT.CMD - copies Phase 65's probe results off pg65-exchange.dsk into
! the repository (README.md). Run from the repository root after @PG65:
!
!     govax console < testdata/pg65/copyout.cmd
!
MOUNT DUA1 "testdata/disks/pg65-exchange.dsk"
COPY DUA1:[000000]PG65.LOG "testdata/pg65/vax/pg65.log"/HOST/QUIET
COPY DUA1:[000000]PG65M.OUT "testdata/pg65/vax/pg65m.out"/HOST/QUIET
COPY DUA1:[000000]PG65G.OUT "testdata/pg65/vax/pg65g.out"/HOST/QUIET
COPY DUA1:[000000]PG65F.OUT "testdata/pg65/vax/pg65f.out"/HOST/QUIET
COPY DUA1:[000000]PG65D.OUT "testdata/pg65/vax/pg65d.out"/HOST/QUIET
COPY DUA1:[000000]PG65R.OUT "testdata/pg65/vax/pg65r.out"/HOST/QUIET
COPY DUA1:[000000]PG65P.OUT "testdata/pg65/vax/pg65p.out"/HOST/QUIET
COPY DUA1:[000000]PG65C.OUT "testdata/pg65/vax/pg65c.out"/HOST/QUIET
COPY DUA1:[000000]PG65U.OUT "testdata/pg65/vax/pg65u.out"/HOST/QUIET
COPY DUA1:[000000]PG65K.OUT "testdata/pg65/vax/pg65k.out"/HOST/QUIET
COPY DUA1:[000000]PG65S.OUT "testdata/pg65/vax/pg65s.out"/HOST/QUIET
COPY DUA1:[000000]PG65H.OUT "testdata/pg65/vax/pg65h.out"/HOST/QUIET
COPY DUA1:[000000]PG65_BUILD.LOG "testdata/pg65/vax/pg65_build.log"/HOST/QUIET
DISMOUNT DUA1
