! COPYOUT.CMD - copies the second round's results off
! link68-round2.dsk into testdata/link68/vax/round2 (README.md), and
! into govax/ govax's own images and library, the ones VMS used (each
! the newest version). MAIN1V.EXE;2 is the link through MYSHR.OLB that
! RUN2.COM meant to call MAIN1W (it gave no /EXECUTABLE=). Run from the
! repository root after @RUN2 on the VAX:
!
!     govax console < testdata/link68/round2/copyout.cmd
!
MOUNT DUA1 "testdata/disks/link68-round2.dsk"
COPY DUA1:[000000]RUN2.LOG "testdata/link68/vax/round2/run2.log"/HOST/QUIET
COPY DUA1:[000000]SHR1.AN2 "testdata/link68/vax/round2/shr1.an2"/HOST/QUIET
COPY DUA1:[000000]SHR1A.AN2 "testdata/link68/vax/round2/shr1a.an2"/HOST/QUIET
COPY DUA1:[000000]SHR2.AN2 "testdata/link68/vax/round2/shr2.an2"/HOST/QUIET
COPY DUA1:[000000]SHR3.AN2 "testdata/link68/vax/round2/shr3.an2"/HOST/QUIET
COPY DUA1:[000000]SHR3P.AN2 "testdata/link68/vax/round2/shr3p.an2"/HOST/QUIET
COPY DUA1:[000000]SHR4.AN2 "testdata/link68/vax/round2/shr4.an2"/HOST/QUIET
COPY DUA1:[000000]SHR5.AN2 "testdata/link68/vax/round2/shr5.an2"/HOST/QUIET
COPY DUA1:[000000]SHR6.AN2 "testdata/link68/vax/round2/shr6.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN1.AN2 "testdata/link68/vax/round2/main1.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN1A.AN2 "testdata/link68/vax/round2/main1a.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN1N.AN2 "testdata/link68/vax/round2/main1n.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN1L.AN2 "testdata/link68/vax/round2/main1l.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN1U.AN2 "testdata/link68/vax/round2/main1u.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN2.AN2 "testdata/link68/vax/round2/main2.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN3.AN2 "testdata/link68/vax/round2/main3.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN3P.AN2 "testdata/link68/vax/round2/main3p.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN4.AN2 "testdata/link68/vax/round2/main4.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN6.AN2 "testdata/link68/vax/round2/main6.an2"/HOST/QUIET
COPY DUA1:[000000]EXE1.AN2 "testdata/link68/vax/round2/exe1.an2"/HOST/QUIET
COPY DUA1:[000000]MAIN1V.MAP "testdata/link68/vax/round2/main1v.map"/HOST/QUIET
COPY DUA1:[000000]MAIN2V.MAP "testdata/link68/vax/round2/main2v.map"/HOST/QUIET
COPY DUA1:[000000]MAIN1W.MAP "testdata/link68/vax/round2/main1w.map"/HOST/QUIET
COPY DUA1:[000000]MAIN1V.EXE "testdata/link68/vax/round2/main1v.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN2V.EXE "testdata/link68/vax/round2/main2v.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN1W.EXE "testdata/link68/vax/round2/main1w.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MYSHRV.OLB "testdata/link68/vax/round2/myshrv.olb"/HOST/BINARY/QUIET
COPY DUA1:[000000]MYSHRV.LIS "testdata/link68/vax/round2/myshrv.lis"/HOST/QUIET
COPY DUA1:[000000]MYSHRG.LIS "testdata/link68/vax/round2/myshrg.lis"/HOST/QUIET
COPY DUA1:[000000]LINK68.LOG "testdata/link68/vax/round2/govax-link68.log"/HOST/QUIET
COPY DUA1:[000000]MAIN1V.EXE;1 "testdata/link68/vax/round2/main1v-1.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN1V.OBJ "testdata/link68/vax/round2/main1v.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN2V.OBJ "testdata/link68/vax/round2/main2v.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]SHR1.EXE;8 "testdata/link68/vax/round2/govax/shr1.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]SHR1A.EXE;2 "testdata/link68/vax/round2/govax/shr1a.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]SHR2.EXE "testdata/link68/vax/round2/govax/shr2.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]SHR3.EXE "testdata/link68/vax/round2/govax/shr3.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]SHR3P.EXE "testdata/link68/vax/round2/govax/shr3p.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]SHR4.EXE;2 "testdata/link68/vax/round2/govax/shr4.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]SHR5.EXE "testdata/link68/vax/round2/govax/shr5.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]SHR6.EXE "testdata/link68/vax/round2/govax/shr6.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN1.EXE "testdata/link68/vax/round2/govax/main1.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN1A.EXE "testdata/link68/vax/round2/govax/main1a.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN1N.EXE "testdata/link68/vax/round2/govax/main1n.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN1L.EXE "testdata/link68/vax/round2/govax/main1l.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN1U.EXE "testdata/link68/vax/round2/govax/main1u.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN2.EXE "testdata/link68/vax/round2/govax/main2.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN3.EXE "testdata/link68/vax/round2/govax/main3.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN3P.EXE "testdata/link68/vax/round2/govax/main3p.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN4.EXE "testdata/link68/vax/round2/govax/main4.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MAIN6.EXE "testdata/link68/vax/round2/govax/main6.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]EXE1.EXE "testdata/link68/vax/round2/govax/exe1.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]MYSHR.OLB "testdata/link68/vax/round2/govax/myshr.olb"/HOST/BINARY/QUIET
DISMOUNT DUA1
