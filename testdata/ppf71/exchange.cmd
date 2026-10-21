! EXCHANGE.CMD - builds Phase 71's probe exchange volume with govax
! (README.md). Run from the repository root:
!
!     govax console < testdata/ppf71/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/ppf71-exchange.dsk" /DEVICE=RD51 PPF71
MOUNT/WRITE DUA1 "testdata/disks/ppf71-exchange.dsk"
COPY "testdata/ppf71/ppf71.mar"/HOST DUA1:[000000]PPF71.MAR
COPY "testdata/ppf71/ppf71.com"/HOST DUA1:[000000]PPF71.COM
COPY "testdata/ppf71/ppf71s.com"/HOST DUA1:[000000]PPF71S.COM
COPY "testdata/ppf71/ppf71p.com"/HOST DUA1:[000000]PPF71P.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
