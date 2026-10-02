$ ! ASM35.COM - assembles the Phase 35 assembler fixture with VAX MACRO
$ ! (testdata/insn35/README.md). Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @ASM35/OUTPUT=ASM35.LOG
$ !
$ SET NOON
$ SET VERIFY
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ MACRO/NOLIST ASM35
$ DIRECTORY/SIZE/DATE ASM35.*
