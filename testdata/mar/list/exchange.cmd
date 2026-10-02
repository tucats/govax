! EXCHANGE.CMD - builds the Phase 29 probe's exchange volume with govax
! (docs/PHASE-29.md, subtask 1). Run from the repository root:
!
!     govax console < testdata/mar/list/exchange.cmd
!
! It makes testdata/disks/list-exchange.dsk (RD51 size, label LISTXCHG,
! gitignored) and copies the probe's sources and LIST.COM onto it.
!
INITIALIZE/CONTAINER "testdata/disks/list-exchange.dsk" /DEVICE=RD51 LISTXCHG
MOUNT/WRITE DUA1 "testdata/disks/list-exchange.dsk"
COPY "testdata/mar/list/lctl.mar"/HOST DUA1:[000000]LCTL.MAR
COPY "testdata/mar/list/binary.mar"/HOST DUA1:[000000]BINARY.MAR
COPY "testdata/mar/list/symtab.mar"/HOST DUA1:[000000]SYMTAB.MAR
COPY "testdata/mar/list/notitle.mar"/HOST DUA1:[000000]NOTITLE.MAR
COPY "testdata/mar/list/errors.mar"/HOST DUA1:[000000]ERRORS.MAR
COPY "testdata/mar/list/errend.mar"/HOST DUA1:[000000]ERREND.MAR
COPY "testdata/mar/list/xref.mar"/HOST DUA1:[000000]XREF.MAR
COPY "testdata/mar/list/trace.mar"/HOST DUA1:[000000]TRACE.MAR
COPY "testdata/mar/list/dbgsrc.mar"/HOST DUA1:[000000]DBGSRC.MAR
COPY "testdata/mar/list/failmain.mar"/HOST DUA1:[000000]FAILMAIN.MAR
COPY "testdata/mar/list/failsub.mar"/HOST DUA1:[000000]FAILSUB.MAR
COPY "testdata/mar/list/failsig.mar"/HOST DUA1:[000000]FAILSIG.MAR
COPY "testdata/mar/list/list.com"/HOST DUA1:[000000]LIST.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
