$ ! RUN.COM - assembles, links, and runs the Phase 33 oracle's probes.
$ ! Written by testdata/mar/rms3/gen.go. Run it after BUILD.COM, with the
$ ! exchange volume's [000000] as the default directory:
$ !
$ !     @RUN/OUTPUT=RUN.LOG
$ !
$ ! It empties [OUT] and [CRE] first, so it can be run again.
$ !
$ SET NOON
$ SET VERIFY
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ DEFINE TST 'DEV'[TEST]
$ DEFINE TSL 'DEV'[TEST.SUB],'DEV'[TEST]
$ IF F$SEARCH("[CRE]*.*;*") .NES. "" THEN DELETE [CRE]*.*;*
$ IF F$SEARCH("[OUT]*.*;*") .NES. "" THEN DELETE [OUT]*.*;*
$ MACRO/NOLIST PARSE
$ LINK PARSE
$ RUN PARSE
$ MACRO/NOLIST SEARCH
$ LINK SEARCH
$ RUN SEARCH
$ MACRO/NOLIST OPEN
$ LINK OPEN
$ RUN OPEN
$ MACRO/NOLIST XAB
$ LINK XAB
$ RUN XAB
$ MACRO/NOLIST NAMFID
$ LINK NAMFID
$ RUN NAMFID
$ MACRO/NOLIST CREATE
$ LINK CREATE
$ RUN CREATE
$ DIRECTORY/FULL [CRE]
$ DIRECTORY/FILE_ID [OUT]
$ DEASSIGN TST
$ DEASSIGN TSL
