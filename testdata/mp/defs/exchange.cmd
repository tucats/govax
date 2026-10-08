! EXCHANGE.CMD - builds the definition probes' exchange volume with govax
! (testdata/mp/defs/README.md): Phase 48's two. Run from the repository
! root:
!
!     govax console < testdata/mp/defs/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-defs.dsk" /DEVICE=RD53 MPDEFS
MOUNT/WRITE DUA1 "testdata/disks/mp-defs.dsk"
COPY "testdata/mp/defs/def_cli.mar"/HOST DUA1:[000000]DEF_CLI.MAR
COPY "testdata/mp/defs/def_lib.mar"/HOST DUA1:[000000]DEF_LIB.MAR
COPY "testdata/mp/defs/defs.com"/HOST DUA1:[000000]DEFS.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
