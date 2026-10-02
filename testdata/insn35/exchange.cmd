! EXCHANGE.CMD - builds the Phase 35 instruction probes' exchange
! volume with govax. Written by testdata/insn35/gen.go. Run from the
! repository root:
!
!     govax console < testdata/insn35/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/insn35-exchange.dsk" /DEVICE=RD51 INSN35
MOUNT/WRITE DUA1 "testdata/disks/insn35-exchange.dsk"
COPY "testdata/insn35/p35fd.mar"/HOST DUA1:[000000]P35FD.MAR
COPY "testdata/insn35/p35g.mar"/HOST DUA1:[000000]P35G.MAR
COPY "testdata/insn35/p35h.mar"/HOST DUA1:[000000]P35H.MAR
COPY "testdata/insn35/p35o.mar"/HOST DUA1:[000000]P35O.MAR
COPY "testdata/insn35/p35p.mar"/HOST DUA1:[000000]P35P.MAR
COPY "testdata/insn35/insn35.com"/HOST DUA1:[000000]INSN35.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
