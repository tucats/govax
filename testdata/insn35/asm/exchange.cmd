! EXCHANGE.CMD - builds the Phase 35 assembler fixture's exchange volume
! with govax. Run from the repository root:
!
!     govax console < testdata/insn35/asm/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/asm35-exchange.dsk" /DEVICE=RD51 ASM35
MOUNT/WRITE DUA1 "testdata/disks/asm35-exchange.dsk"
COPY "testdata/insn35/asm/asm35.mar"/HOST DUA1:[000000]ASM35.MAR
COPY "testdata/insn35/asm/asm35.com"/HOST DUA1:[000000]ASM35.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
