! COPYOUT.CMD - copies the second round's results off
! link68-round2.dsk into testdata/link68/vax/round2 (README.md). Run
! from the repository root after @RUN2 on the VAX:
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
DISMOUNT DUA1
