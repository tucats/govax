! EXCHANGE.CMD - builds the exchange volume for Phase 68's second round
! (testdata/link68/round2/README.md): govax runs LINK68.COM on it, so it
! holds govax's objects, images, maps, and library, then RUN2.COM. Run
! from the repository root:
!
!     govax console < testdata/link68/round2/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/link68-round2.dsk" /DEVICE=RD51 LINK68
MOUNT/WRITE DUA1 "testdata/disks/link68-round2.dsk"
COPY "testdata/link68/xfr1.mar"/HOST DUA1:[000000]XFR1.MAR
COPY "testdata/link68/shr1.mar"/HOST DUA1:[000000]SHR1.MAR
COPY "testdata/link68/shr2.mar"/HOST DUA1:[000000]SHR2.MAR
COPY "testdata/link68/shr3.mar"/HOST DUA1:[000000]SHR3.MAR
COPY "testdata/link68/shr6.mar"/HOST DUA1:[000000]SHR6.MAR
COPY "testdata/link68/main1.mar"/HOST DUA1:[000000]MAIN1.MAR
COPY "testdata/link68/main2.mar"/HOST DUA1:[000000]MAIN2.MAR
COPY "testdata/link68/main3.mar"/HOST DUA1:[000000]MAIN3.MAR
COPY "testdata/link68/main6.mar"/HOST DUA1:[000000]MAIN6.MAR
COPY "testdata/link68/exe1.mar"/HOST DUA1:[000000]EXE1.MAR
COPY "testdata/link68/exe1b.mar"/HOST DUA1:[000000]EXE1B.MAR
COPY "testdata/link68/shr1.opt"/HOST DUA1:[000000]SHR1.OPT
COPY "testdata/link68/link68.com"/HOST DUA1:[000000]LINK68.COM
SET DEFAULT DUA1:[000000]
@LINK68/OUTPUT=LINK68.LOG
COPY "testdata/link68/round2/run2.com"/HOST DUA1:[000000]RUN2.COM
SET DEFAULT DUA0:[000000]
DIRECTORY DUA1:[000000]*.EXE
DISMOUNT DUA1
