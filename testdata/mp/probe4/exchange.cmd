! EXCHANGE.CMD - builds a volume of probe 4 alone (README.md), for running
! it by itself. Run from the repository root:
!
!     govax console < testdata/mp/probe4/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-probe4.dsk" /DEVICE=RD53 MPPROBE4
MOUNT/WRITE DUA1 "testdata/disks/mp-probe4.dsk"
COPY "testdata/mp/probe4/probe4.mar"/HOST DUA1:[000000]PROBE4.MAR
COPY "testdata/mp/probe4/p4child.mar"/HOST DUA1:[000000]P4CHILD.MAR
COPY "testdata/mp/probe4/p4nocli.mar"/HOST DUA1:[000000]P4NOCLI.MAR
COPY "testdata/mp/probe4/p4cmds.com"/HOST DUA1:[000000]P4CMDS.COM
COPY "testdata/mp/probe4/probe4.com"/HOST DUA1:[000000]PROBE4.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
