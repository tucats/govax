! COPYOUT2.CMD - copies round 2 of Phase 70's probe off
! tb70-exchange.dsk into testdata/tb70/vax (README.md). Run from the
! repository root after @TB70B/OUTPUT=TB70B.LOG on the VAX:
!
!     govax console < testdata/tb70/copyout2.cmd
!
MOUNT DUA1 "testdata/disks/tb70-exchange.dsk"
COPY DUA1:[000000]TB70B.LOG "testdata/tb70/vax/tb70b.log"/HOST/QUIET
COPY DUA1:[000000]TB70B.MAP "testdata/tb70/vax/tb70b.map"/HOST/QUIET
COPY DUA1:[000000]TB70BNT.MAP "testdata/tb70/vax/tb70bnt.map"/HOST/QUIET
COPY/BINARY DUA1:[000000]TB70B.EXE "testdata/tb70/vax/tb70b.exe"/HOST/QUIET
COPY/BINARY DUA1:[000000]TB70BNT.EXE "testdata/tb70/vax/tb70bnt.exe"/HOST/QUIET
COPY DUA1:[000000]TB70B.OBJ "testdata/tb70/vax/tb70b.obj"/HOST/QUIET
DISMOUNT DUA1
