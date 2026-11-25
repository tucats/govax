$ ! ERR113B.COM - Phase 113's probe round 2, what round 1 and subtask 13 left
$ ! unconfirmed (testdata/err113's README, "Round 2"). Run it with the exchange
$ ! volume's [000000] the default directory, the scratch disk attached but not
$ ! mounted:
$ !
$ !     @ERR113B/OUTPUT=ERR113B.LOG [scratch-device] [section]
$ !
$ ! P1 is the scratch disk, DUA3: when not given; it is initialized here, twice.
$ ! P2, if given, is a section to start at (LNK113D, MAC113F, DCL2, FULL2,
$ ! HDRFULL). P3 KEEP (govax's rehearsal, which has no INITIALIZE of a
$ ! device) uses the scratch volume mounted as it is, its files deleted
$ ! between the sections.
$ !
$ SET NOON
$ KEEP = P3 .EQS. "KEEP"
$ SCRATCH = P1
$ IF SCRATCH .EQS. "" THEN SCRATCH = "DUA3:"
$ HOME = F$ENVIRONMENT("DEFAULT")
$ SET DEFAULT [ERR113]
$ HERE = F$ENVIRONMENT("DEFAULT")
$ IF P2 .NES. "" THEN GOTO START_'P2'
$ !
$ ! A shareable image's .LONG of its own address: LINK's message, and the
$ ! image (its fixup section) and map come back.
$ WRITE SYS$OUTPUT "==== LNK113L"
$ MACRO/NOLIST LNK113L
$ SHOW SYMBOL $STATUS
$ LINK/SHAREABLE/MAP=LNK113L/FULL LNK113L,LNK113L/OPTIONS
$ SHOW SYMBOL $STATUS
$ !
$ ! Sixteen shareable images whose debug symbol tables' records total
$ ! successive byte counts around 512 (gen.go): how many blocks does LINK
$ ! give each table? The images come back.
$ START_LNK113D:
$ WRITE SYS$OUTPUT "==== LNK113D"
$ @LNK113D
$ !
$ ! MACRO: a register name after an operator in a data item.
$ START_MAC113F:
$ WRITE SYS$OUTPUT "==== MAC113F"
$ MACRO/LIST MAC113F
$ SHOW SYMBOL $STATUS
$ !
$ ! Round 1 found CLI$_DIRECT, with the specification as its segment, for a
$ ! directory left open in PURGE's and COPY's file parameters. Which other
$ ! commands' parameters are file specifications to DCL?
$ START_DCL2:
$ WRITE SYS$OUTPUT "==== DCL2"
$ DIRECTORY DCL[X.TXT
$ SHOW SYMBOL $STATUS
$ TYPE DCL[X.TXT
$ SHOW SYMBOL $STATUS
$ DELETE DCL[X.TXT;1
$ SHOW SYMBOL $STATUS
$ RENAME DCL[X.TXT DCLY.TXT
$ SHOW SYMBOL $STATUS
$ RENAME DCLX.TXT DCL[Y.TXT
$ SHOW SYMBOL $STATUS
$ DUMP DCL[X.TXT
$ SHOW SYMBOL $STATUS
$ CREATE DCL[X.TXT
$ SHOW SYMBOL $STATUS
$ OPEN F DCL[X.TXT
$ SHOW SYMBOL $STATUS
$ PRINT DCL[X.TXT
$ SHOW SYMBOL $STATUS
$ SUBMIT DCL[X.COM
$ SHOW SYMBOL $STATUS
$ CONVERT DCL[X.TXT DCLY.TXT
$ SHOW SYMBOL $STATUS
$ MACRO DCL[X.MAR
$ SHOW SYMBOL $STATUS
$ LINK DCL[X.OBJ
$ SHOW SYMBOL $STATUS
$ LIBRARY/CREATE DCL[X.OLB
$ SHOW SYMBOL $STATUS
$ ANALYZE/OBJECT DCL[X.OBJ
$ SHOW SYMBOL $STATUS
$ RUN DCL[X.EXE
$ SHOW SYMBOL $STATUS
$ BACKUP DCL[X.TXT DCLY.BCK/SAVE_SET
$ SHOW SYMBOL $STATUS
$ SET DEFAULT DCL[X
$ SHOW SYMBOL $STATUS
$ SET DEFAULT 'HERE'
$ @DCL[X.COM
$ SHOW SYMBOL $STATUS
$ !
$ ! Round 1's FULL again, with the free blocks counted: VMS left one block
$ ! free when DCL's WRITE to REST.DAT failed. How big is REST.DAT then?
$ START_FULL2:
$ WRITE SYS$OUTPUT "==== FULL2"
$ IF KEEP THEN GOTO FULL2_MOUNTED
$ IF F$GETDVI(SCRATCH,"MNT") THEN DISMOUNT 'SCRATCH'
$ INITIALIZE/HEADERS=16/EXTENSION=1 'SCRATCH' SCR113
$ MOUNT/NOASSIST 'SCRATCH' SCR113
$ FULL2_MOUNTED:
$ OPEN/WRITE F BIG.DAT
$ N = 0
$ SEED:
$ N = N + 1
$ WRITE F F$FAO("!4ZL!246*X", N)
$ IF N .LT. 32 THEN GOTO SEED
$ CLOSE F
$ N = 0
$ DOUBLE:
$ N = N + 1
$ COPY BIG.DAT BIGC.DAT
$ CONVERT/APPEND BIGC.DAT BIG.DAT
$ DELETE BIGC.DAT;*
$ PURGE BIG.DAT
$ IF N .LT. 8 THEN GOTO DOUBLE
$ N = 0
$ FILL:
$ N = N + 1
$ COPY BIG.DAT 'SCRATCH'[000000]FILL'N'.DAT
$ S = $STATUS
$ IF S .AND. N .LT. 10 THEN GOTO FILL
$ WRITE SYS$OUTPUT "COPIES ", N - 1, " FREE ", F$GETDVI(SCRATCH,"FREEBLOCKS")
$ OPEN/WRITE F 'SCRATCH'[000000]REST.DAT
$ IF .NOT. $STATUS THEN GOTO REST_DONE
$ N = 0
$ REST:
$ N = N + 1
$ WRITE F F$FAO("!5ZL!245*Y", N)
$ S = $STATUS
$ IF S .AND. N .LT. 100000 THEN GOTO REST
$ WRITE SYS$OUTPUT "RECORDS ", N - 1, " FREE ", F$GETDVI(SCRATCH,"FREEBLOCKS")
$ CLOSE F
$ WRITE SYS$OUTPUT "CLOSED FREE ", F$GETDVI(SCRATCH,"FREEBLOCKS")
$ REST_DONE:
$ DIRECTORY/SIZE=ALL/DATE 'SCRATCH'[000000]REST.DAT
$ WRITE SYS$OUTPUT "REST.DAT EOF ", F$FILE_ATTRIBUTES(SCRATCH+"[000000]REST.DAT","EOF"), -
     " FFB ", F$FILE_ATTRIBUTES(SCRATCH+"[000000]REST.DAT","FFB"), -
     " ALQ ", F$FILE_ATTRIBUTES(SCRATCH+"[000000]REST.DAT","ALQ")
$ DIRECTORY/SIZE=ALL 'SCRATCH'[000000]INDEXF.SYS
$ ! One more block's worth: a file of one record, then another record to REST.
$ CREATE 'SCRATCH'[000000]MORE.TXT
A line for a full volume.
$ SHOW SYMBOL $STATUS
$ WRITE SYS$OUTPUT "FREE ", F$GETDVI(SCRATCH,"FREEBLOCKS")
$ DELETE 'SCRATCH'[000000]*.DAT;*,*.TXT;*
$ DELETE BIG.DAT;*
$ IF .NOT. KEEP THEN DISMOUNT 'SCRATCH'
$ !
$ ! The index file past its header, on a volume whose free space is all in
$ ! blocks of one. 8-block files fill the volume; every other one is deleted;
$ ! 7-block files take the holes, leaving a block of each; then 1-block files
$ ! need new headers, which the index file can take only a block at a time.
$ ! Does VMS give INDEXF.SYS an extension header, or refuse (HEADERFULL)?
$ START_HDRFULL:
$ WRITE SYS$OUTPUT "==== HDRFULL"
$ IF KEEP THEN GOTO HDRFULL_MOUNTED
$ IF F$GETDVI(SCRATCH,"MNT") THEN DISMOUNT 'SCRATCH'
$ INITIALIZE/HEADERS=16/EXTENSION=1/MAXIMUM_FILES=8000 'SCRATCH' SCR113
$ MOUNT/NOASSIST 'SCRATCH' SCR113
$ HDRFULL_MOUNTED:
$ ! A directory must be contiguous, so on a fragmented volume one can't grow:
$ ! each phase's is made big enough first, its names in order (A0001, ...).
$ CREATE/DIRECTORY/ALLOCATION=300 'SCRATCH'[A]
$ CREATE/DIRECTORY/ALLOCATION=150 'SCRATCH'[B]
$ CREATE/DIRECTORY/ALLOCATION=100 'SCRATCH'[C]
$ ! The templates: 8, 7, and 1 blocks of 128-byte records, each allocated
$ ! its size and no more (FIX8.FDL, FIX7.FDL, FIX1.FDL), on the scratch volume,
$ ! whose cluster is a block (the round's volume's is 3).
$ T = SCRATCH + "[000000]"
$ CREATE/FDL=FIX8.FDL 'T'H8.DAT
$ CREATE/FDL=FIX7.FDL 'T'H7.DAT
$ CREATE/FDL=FIX1.FDL 'T'H1.DAT
$ OPEN/APPEND F 'T'H8.DAT
$ N = 0
$ H8:
$ N = N + 1
$ WRITE F F$FAO("!4ZL!124*H", N)
$ IF N .LT. 32 THEN GOTO H8
$ CLOSE F
$ OPEN/APPEND F 'T'H7.DAT
$ N = 0
$ H7:
$ N = N + 1
$ WRITE F F$FAO("!4ZL!124*S", N)
$ IF N .LT. 28 THEN GOTO H7
$ CLOSE F
$ OPEN/APPEND F 'T'H1.DAT
$ N = 0
$ H1:
$ N = N + 1
$ WRITE F F$FAO("!4ZL!124*O", N)
$ IF N .LT. 4 THEN GOTO H1
$ CLOSE F
$ DIRECTORY/SIZE=ALL 'T'H%.DAT
$ ! Fill the volume with 8-block files.
$ N = 0
$ FILL8:
$ N = N + 1
$ COPY/CONTIGUOUS 'T'H8.DAT 'SCRATCH'[A]A'F$FAO("!4ZL",N)'.DAT
$ S = $STATUS
$ IF S .AND. N .LT. 3000 THEN GOTO FILL8
$ A = N - 1
$ WRITE SYS$OUTPUT "8-BLOCK FILES ", A, " STATUS ", S, " FREE ", F$GETDVI(SCRATCH,"FREEBLOCKS")
$ DIRECTORY/SIZE=ALL 'SCRATCH'[000000]INDEXF.SYS,A.DIR,B.DIR,C.DIR
$ DIRECTORY/SIZE=ALL 'SCRATCH'[A]A0001.DAT,A0002.DAT
$ ! Every other one deleted.
$ N = 1
$ HOLES:
$ DELETE 'SCRATCH'[A]A'F$FAO("!4ZL",N)'.DAT;*
$ N = N + 2
$ IF N .LE. A THEN GOTO HOLES
$ WRITE SYS$OUTPUT "HOLES FREE ", F$GETDVI(SCRATCH,"FREEBLOCKS")
$ ! 7-block files into the holes, each in one run (COPY/CONTIGUOUS), so a
$ ! block of each hole is left.
$ N = 0
$ FILL7:
$ N = N + 1
$ COPY/CONTIGUOUS 'T'H7.DAT 'SCRATCH'[B]B'F$FAO("!4ZL",N)'.DAT
$ S = $STATUS
$ IF S .AND. N .LT. 3000 THEN GOTO FILL7
$ B = N - 1
$ WRITE SYS$OUTPUT "7-BLOCK FILES ", B, " STATUS ", S, " FREE ", F$GETDVI(SCRATCH,"FREEBLOCKS")
$ DIRECTORY/SIZE=ALL 'SCRATCH'[000000]INDEXF.SYS
$ ! 1-block files, each needing a header.
$ N = 0
$ FILL1:
$ N = N + 1
$ COPY 'T'H1.DAT 'SCRATCH'[C]C'F$FAO("!4ZL",N)'.DAT
$ S = $STATUS
$ IF S .AND. N .LT. 1500 THEN GOTO FILL1
$ C = N - 1
$ WRITE SYS$OUTPUT "1-BLOCK FILES ", C, " STATUS ", S, " FREE ", F$GETDVI(SCRATCH,"FREEBLOCKS")
$ DIRECTORY/SIZE=ALL/FULL 'SCRATCH'[000000]INDEXF.SYS
$ DUMP/HEADER/BLOCK=COUNT:0 'SCRATCH'[000000]INDEXF.SYS
$ ANALYZE/DISK_STRUCTURE 'SCRATCH'
$ DELETE 'T'H%.DAT;*
$ IF .NOT. KEEP THEN DISMOUNT 'SCRATCH'
$ !
$ SET DEFAULT 'HOME'
$ WRITE SYS$OUTPUT "==== END"
