! EXCHANGE.CMD - builds the Phase 57 probe's exchange volume with
! govax. Written by testdata/lib57/gen.go. Run from the repository root:
!
!     govax console < testdata/lib57/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib57-exchange.dsk" /DEVICE=RD51 LIB57
MOUNT/WRITE DUA1 "testdata/disks/lib57-exchange.dsk"
COPY "testdata/lib57/lib57.mar"/HOST DUA1:[000000]LIB57.MAR
COPY "testdata/lib57/lib57b.mar"/HOST DUA1:[000000]LIB57B.MAR
COPY "testdata/lib57/def_libcli.mar"/HOST DUA1:[000000]DEF_LIBCLI.MAR
COPY "testdata/lib57/lib57.com"/HOST DUA1:[000000]LIB57.COM
COPY "testdata/lib57/lib57s.com"/HOST DUA1:[000000]LIB57S.COM
COPY "testdata/lib57/lib57c.com"/HOST DUA1:[000000]LIB57C.COM
COPY "testdata/lib57/lib57p.com"/HOST DUA1:[000000]LIB57P.COM
COPY "testdata/lib57/lib57i.com"/HOST DUA1:[000000]LIB57I.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
