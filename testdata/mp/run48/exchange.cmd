! EXCHANGE.CMD - builds the exchange volume for every VAX run of Phase 48
! (testdata/mp/run48/README.md). Run from the repository root:
!
!     govax console < testdata/mp/run48/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-run48.dsk" /DEVICE=RD53 MPRUN48
MOUNT/WRITE DUA1 "testdata/disks/mp-run48.dsk"
COPY "testdata/mp/defs/def_cli.mar"/HOST DUA1:[000000]DEF_CLI.MAR
COPY "testdata/mp/defs/def_lib.mar"/HOST DUA1:[000000]DEF_LIB.MAR
COPY "testdata/mp/defs/defs.com"/HOST DUA1:[000000]DEFS.COM
COPY "testdata/mp/msparent.mar"/HOST DUA1:[000000]MSPARENT.MAR
COPY "testdata/mp/mschild.mar"/HOST DUA1:[000000]MSCHILD.MAR
COPY "testdata/mp/run48/milestone.com"/HOST DUA1:[000000]MILESTONE.COM
COPY "testdata/mp/probe4/probe4.mar"/HOST DUA1:[000000]PROBE4.MAR
COPY "testdata/mp/probe4/p4child.mar"/HOST DUA1:[000000]P4CHILD.MAR
COPY "testdata/mp/probe4/p4nocli.mar"/HOST DUA1:[000000]P4NOCLI.MAR
COPY "testdata/mp/probe4/p4cmds.com"/HOST DUA1:[000000]P4CMDS.COM
COPY "testdata/mp/probe4/probe4.com"/HOST DUA1:[000000]PROBE4.COM
COPY "testdata/mp/run48/run48.com"/HOST DUA1:[000000]RUN48.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
