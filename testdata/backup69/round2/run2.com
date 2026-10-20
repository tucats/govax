$ ! RUN2.COM - Phase 69's second round: VMS reads govax's save sets
$ ! (testdata/backup69/round2/README.md). Run it once, on the exchange
$ ! volume govax made, with its [000000] as the default directory:
$ !
$ !     @RUN2/OUTPUT=RUN2.LOG
$ !
$ ! [B69...] is round 1's tree as govax restored it, and [SETS] holds the
$ ! save sets govax made of it with round 1's Part 2 commands, and
$ ! HOST.BCK, one govax wrote to the host, copied here as fixed 512-byte
$ ! records. Part 1 looks at the tree, Part 2 lists every save set, Part
$ ! 3 restores them, Part 4 compares, Part 5 damages two and restores
$ ! them, and Part 6 checks the volume. Reports go to [RESULTS].
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ HERE = F$ENVIRONMENT("DEFAULT")
$ SHOW DEFAULT
$ !
$ ! ------------------------------------------------------------------
$ ! Part 1: the tree govax restored, as VMS sees its headers.
$ ! ------------------------------------------------------------------
$ !
$ DIRECTORY/FULL/OUTPUT=[RESULTS]TREE2.LST [B69...]
$ DIRECTORY/SIZE=ALL/DATE [B69...]
$ DIRECTORY/FULL/OUTPUT=[RESULTS]SETS2.LST [SETS]*.BCK
$ DIRECTORY/SIZE=ALL [SETS]
$ !
$ ! ------------------------------------------------------------------
$ ! Part 2: listings of govax's save sets.
$ ! ------------------------------------------------------------------
$ !
$ LIST = "B69,B69K8,B69G0,B69G3,B69CRC,B69NOCRC,B69COM,B69EXC,B69BYO,B69TXT,B69NONE,TEXT,B69OWN,LIST,B69IGN,HOST"
$ I = 0
$ LIST_LOOP:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO LIST_DONE
$   BACKUP/LIST [SETS]'F'.BCK/SAVE_SET
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO LIST_LOOP
$ LIST_DONE:
$ BACKUP/LIST/FULL [SETS]B69.BCK/SAVE_SET
$ SHOW SYMBOL $STATUS
$ BACKUP/LIST/FULL [SETS]B69COM.BCK/SAVE_SET
$ BACKUP/LIST=[RESULTS]B69FULL2.LIS/FULL [SETS]B69.BCK/SAVE_SET
$ !
$ ! ------------------------------------------------------------------
$ ! Part 3: restores, each with /LOG. The save sets written with /CRC
$ ! are restored with /CRC on the input, so their CRCs are checked.
$ ! ------------------------------------------------------------------
$ !
$ LIST = "B69,B69K8,B69G0,B69G3,B69NOCRC,B69COM,B69EXC,B69BYO,B69TXT,B69NONE,TEXT,B69OWN,LIST,B69IGN"
$ I = 0
$ RESTORE_LOOP:
$   F = F$ELEMENT(I, ",", LIST)
$   IF F .EQS. "," THEN GOTO RESTORE_DONE
$   BACKUP/LOG [SETS]'F'.BCK/SAVE_SET [V'F'...]/BY_OWNER=ORIGINAL
$   SHOW SYMBOL $STATUS
$   I = I + 1
$   GOTO RESTORE_LOOP
$ RESTORE_DONE:
$ BACKUP/LOG [SETS]B69CRC.BCK/SAVE_SET/CRC [VB69CRC...]/BY_OWNER=ORIGINAL
$ SHOW SYMBOL $STATUS
$ BACKUP/LOG [SETS]HOST.BCK/SAVE_SET/CRC [VHOST...]/BY_OWNER=ORIGINAL
$ SHOW SYMBOL $STATUS
$ DIRECTORY/FULL/OUTPUT=[RESULTS]VB69.LST [VB69...]
$ DIRECTORY/SIZE=ALL [VB69K8...],[VB69G0...],[VB69G3...],[VB69CRC...],[VHOST...]
$ !
$ ! ------------------------------------------------------------------
$ ! Part 4: compares. govax's B69.BCK against the tree it was made
$ ! from, and against VMS's restore of it.
$ ! ------------------------------------------------------------------
$ !
$ BACKUP/COMPARE/LOG [SETS]B69.BCK/SAVE_SET [*...]
$ SHOW SYMBOL $STATUS
$ BACKUP/COMPARE/LOG [SETS]B69.BCK/SAVE_SET [VB69...]
$ SHOW SYMBOL $STATUS
$ BACKUP/COMPARE/LOG [SETS]B69CRC.BCK/SAVE_SET/CRC [VB69CRC...]
$ SHOW SYMBOL $STATUS
$ !
$ ! ------------------------------------------------------------------
$ ! Part 5: govax's XOR blocks. SSCOPY complements byte 5000 of block 2
$ ! (data, not header) of B69.BCK and of B69CRC.BCK; each copy is listed
$ ! and restored, B69CRCD with /CRC so its CRC finds the damage. A
$ ! byte of block 2's header (100) is changed in B69HDRD.
$ ! ------------------------------------------------------------------
$ !
$ MACRO SSCOPY.MAR
$ LINK SSCOPY.OBJ
$ RUN SSCOPY
[SETS]B69.BCK
[SETS]B69D.BCK
0
2
5000
$ RUN SSCOPY
[SETS]B69CRC.BCK
[SETS]B69CRCD.BCK
0
2
5000
$ RUN SSCOPY
[SETS]B69.BCK
[SETS]B69HDRD.BCK
0
2
100
$ BACKUP/LIST [SETS]B69D.BCK/SAVE_SET
$ SHOW SYMBOL $STATUS
$ BACKUP/LOG [SETS]B69D.BCK/SAVE_SET [DB69D...]/BY_OWNER=ORIGINAL
$ SHOW SYMBOL $STATUS
$ BACKUP/LOG [SETS]B69CRCD.BCK/SAVE_SET/CRC [DB69CRCD...]/BY_OWNER=ORIGINAL
$ SHOW SYMBOL $STATUS
$ BACKUP/LOG [SETS]B69HDRD.BCK/SAVE_SET [DB69HDRD...]/BY_OWNER=ORIGINAL
$ SHOW SYMBOL $STATUS
$ BACKUP/COMPARE/LOG [SETS]B69CRCD.BCK/SAVE_SET/CRC [DB69CRCD...]
$ SHOW SYMBOL $STATUS
$ !
$ ! ------------------------------------------------------------------
$ ! Part 6: the volume, after govax's restores and VMS's.
$ ! ------------------------------------------------------------------
$ !
$ ANALYZE/DISK_STRUCTURE 'F$PARSE(HERE,,,"DEVICE")'
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE=ALL/DATE [SETS]
$ SET NOVERIFY
$ EXIT
