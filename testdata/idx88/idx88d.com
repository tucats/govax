$ ! IDX88D.COM - Phase 88's round 4: DCL on indexed files (subtask 9;
$ ! docs/PHASE-88 - keyed index files.md). Run it with the exchange volume's
$ ! [000000] as the default directory, after IDX88.COM or on its own:
$ !
$ !     @IDX88D/OUTPUT=IDX88D.LOG
$ !
$ ! IDX88D.MAR makes D1.IDX, D2.IDX, and D3.IDX with no records (their keys are
$ ! in its comments), and this procedure writes, reads, changes, and shows them:
$ ! WRITE, READ's /KEY, /INDEX, /MATCH, /DELETE, and /NOLOCK, WRITE/UPDATE,
$ ! two streams on one file, a fixed-length file, OPEN/APPEND, TYPE, DUMP/RECORD,
$ ! DIRECTORY/FULL, and F$FILE_ATTRIBUTES. Each command's $STATUS is shown, and
$ ! each READ's symbol, which is set to "-" first. Nothing reads the terminal.
$ SET NOON
$ SET VERIFY
$ DELETE D%.IDX;*,D4.DAT;*
$ ! MACRO's own output goes to a log of its own, which stays on the volume
$ ! until the author has read it, as round 1's did.
$ DEFINE/USER_MODE SYS$OUTPUT IDX88DM.LOG
$ DEFINE/USER_MODE SYS$ERROR NL:
$ MACRO/NOLIST IDX88D
$ LINK/NOMAP IDX88D
$ RUN IDX88D
$ SHOW SYMBOL $STATUS
$ !
$ ! D1: WRITE on an indexed file: records out of order (does DCL's $PUT go by
$ ! key, or in sequence?), a duplicate primary key, a duplicate alternate, a
$ ! record too short for the primary key, one too short for an alternate, and
$ ! one longer than the file's maximum.
$ M = "D1"
$ SHOW SYMBOL M
$ OPEN/READ/WRITE F D1.IDX
$ SHOW SYMBOL $STATUS
$ SHOW LOGICAL/FULL F
$ WRITE F "K00020GRPAbbMMM two"
$ SHOW SYMBOL $STATUS
$ WRITE F "K00010GRPBaaZZZ one"
$ SHOW SYMBOL $STATUS
$ WRITE F "K00040GRPAccAAA four"
$ SHOW SYMBOL $STATUS
$ WRITE F "K00030GRPBbbMMM three"
$ SHOW SYMBOL $STATUS
$ WRITE F "K00050GRPCddQQQ five"
$ SHOW SYMBOL $STATUS
$ WRITE F "K00010GRPZzzZZZ a second K00010"
$ SHOW SYMBOL $STATUS
$ WRITE F "K0006"
$ SHOW SYMBOL $STATUS
$ WRITE F "K00060GR"
$ SHOW SYMBOL $STATUS
$ WRITE F "K00070", F$FAO("!75*x")
$ SHOW SYMBOL $STATUS
$ WRITE F "K00080", F$FAO("!74*y")
$ SHOW SYMBOL $STATUS
$ CLOSE F
$ SHOW SYMBOL $STATUS
$ !
$ ! D2: READ in sequence: the primary key's order, to the end of the file,
$ ! then READ at the end without /END_OF_FILE.
$ M = "D2"
$ SHOW SYMBOL M
$ OPEN/READ F D1.IDX
$ SHOW SYMBOL $STATUS
$ D2LOOP:
$ R = "-"
$ READ/END_OF_FILE=D2END/ERROR=D2END F R
$ SHOW SYMBOL R
$ GOTO D2LOOP
$ D2END:
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ !
$ ! D3: READ/KEY on the primary key: exact, then on in sequence; generic; not
$ ! there; /MATCH=GE and GT, generic too; a key longer than the key, an empty
$ ! one, lowercase, unquoted; /MATCH's keywords, abbreviated and wrong.
$ M = "D3"
$ SHOW SYMBOL M
$ R = "-"
$ READ/KEY="K00030" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K0002" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00025" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00025"/MATCH=GE F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00030"/MATCH=GT F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K0001"/MATCH=GT F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K0008"/MATCH=GT F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K000300" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="k00030" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY=k00030 F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00040"/MATCH=EQ F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00040"/MATCH=G F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00040"/MATCH=LT F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00040"/MATCH F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/MATCH=GE F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ !
$ ! D4: /INDEX: an alternate key with duplicates, then on in its order; /KEY
$ ! with no /INDEX after it (the last /INDEX given, the manual says); /INDEX
$ ! with no /KEY; the descending key with each match; the binary key; a key
$ ! the file hasn't; /INDEX with no value.
$ M = "D4"
$ SHOW SYMBOL M
$ R = "-"
$ READ/INDEX=1/KEY="GRPB" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="GRPA" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=0/KEY="K00010" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=1 F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=3/KEY="MMM" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=3/KEY="MMM"/MATCH=GT F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=3/KEY="NNN"/MATCH=GE F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=3/KEY="M"/MATCH=GT F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=2/KEY="bb" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=2/KEY="b" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=4/KEY="K00010" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX/KEY="K00010" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=0/KEY="K00010" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ !
$ ! D5: READ/KEY and /INDEX on a file opened only to read are D1 to D4; on a
$ ! sequential file, and READ/DELETE on a file opened only to read.
$ M = "D5"
$ SHOW SYMBOL M
$ R = "-"
$ READ/DELETE/KEY="K00010" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/DELETE F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ CLOSE F
$ OPEN/WRITE S D4.DAT
$ WRITE S "K00010 a sequential record"
$ WRITE S "K00020 another"
$ CLOSE S
$ OPEN/READ S D4.DAT
$ R = "-"
$ READ/KEY="K00020" S R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=1 S R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ S R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/DELETE S R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ CLOSE S
$ OPEN/READ/WRITE S D4.DAT
$ R = "-"
$ READ/DELETE S R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ S R
$ SHOW SYMBOL R
$ CLOSE S
$ !
$ ! D6: WRITE/UPDATE: the same length, longer, shorter, an alternate changed,
$ ! the primary key changed, too short for the primary key, with no READ
$ ! before it, after a failed READ, and READ in sequence after an update.
$ M = "D6"
$ SHOW SYMBOL M
$ OPEN/READ/WRITE F D1.IDX
$ R = "-"
$ READ/KEY="K00020" F R
$ SHOW SYMBOL R
$ WRITE/UPDATE F "K00020GRPAbbMMM TWO"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00020" F R
$ SHOW SYMBOL R
$ WRITE/UPDATE F "K00020GRPAbbMMM two, made longer"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/KEY="K00020" F R
$ SHOW SYMBOL R
$ WRITE/UPDATE F "K00020GRPCbbMMM"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/KEY="K00020" F R
$ SHOW SYMBOL R
$ WRITE/UPDATE F "K00021GRPCbbMMM changed"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/KEY="K00020" F R
$ SHOW SYMBOL R
$ WRITE/UPDATE F "K0002"
$ SHOW SYMBOL $STATUS
$ WRITE/UPDATE F "K00020GRPCbbMMM again"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/KEY="K00099" F R
$ SHOW SYMBOL $STATUS
$ WRITE/UPDATE F "K00020GRPCbbMMM after RNF"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/INDEX=1/KEY="GRPC" F R
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ !
$ ! D7: READ/DELETE: by key, then the same key again; in sequence, and READ
$ ! after it; a deleted record's alternate; WRITE/UPDATE after READ/DELETE.
$ M = "D7"
$ SHOW SYMBOL M
$ R = "-"
$ READ/INDEX=0/DELETE/KEY="K00040" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00040" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00010" F R
$ SHOW SYMBOL R
$ R = "-"
$ READ/DELETE F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ WRITE/UPDATE F "K00030GRPBbbMMM after a delete"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/INDEX=3/KEY="AAA" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/INDEX=0/KEY="K" F R
$ SHOW SYMBOL R
$ D7LOOP:
$ R = "-"
$ READ/END_OF_FILE=D7END/ERROR=D7END F R
$ SHOW SYMBOL R
$ GOTO D7LOOP
$ D7END:
$ CLOSE F
$ !
$ ! D8: two streams on one file, each with /SHARE: a record one has read,
$ ! read by the other, then with /NOLOCK, then after the first has read on;
$ ! WRITE/UPDATE of a record READ/NOLOCK read; and a second OPEN without
$ ! /SHARE while one is open.
$ M = "D8"
$ SHOW SYMBOL M
$ OPEN/READ/WRITE/SHARE A D1.IDX
$ SHOW SYMBOL $STATUS
$ OPEN/READ/WRITE/SHARE B D1.IDX
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/KEY="K00050" A R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00050" B R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/NOLOCK/KEY="K00050" B R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ WRITE/UPDATE B "K00050GRPCddQQQ five, by B"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/KEY="K00060" A R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ/KEY="K00050" B R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ WRITE/UPDATE B "K00050GRPCddQQQ five, by B"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/KEY="K00050" A R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ OPEN/READ C D1.IDX
$ SHOW SYMBOL $STATUS
$ CLOSE A
$ CLOSE B
$ OPEN/READ C D1.IDX
$ SHOW SYMBOL $STATUS
$ CLOSE C
$ !
$ ! D9: the fixed-length file, whose primary key has duplicates: WRITE of
$ ! a record of the size, shorter, and longer; duplicates in order; READ/KEY
$ ! of a duplicate and on; WRITE/UPDATE shorter; TYPE.
$ M = "D9"
$ SHOW SYMBOL M
$ OPEN/READ/WRITE F D2.IDX
$ WRITE F "BBBB first of BBBB.."
$ SHOW SYMBOL $STATUS
$ WRITE F "AAAA the only AAAA.."
$ SHOW SYMBOL $STATUS
$ WRITE F "BBBB second of BBBB."
$ SHOW SYMBOL $STATUS
$ WRITE F "CCCC short"
$ SHOW SYMBOL $STATUS
$ WRITE F "CCCC far too long for twenty"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ/KEY="BBBB" F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ R = "-"
$ READ F R
$ SHOW SYMBOL R
$ WRITE/UPDATE F "BBBB updated"
$ SHOW SYMBOL $STATUS
$ WRITE/UPDATE F "BBBB second, updated"
$ SHOW SYMBOL $STATUS
$ CLOSE F
$ TYPE D2.IDX
$ !
$ ! D10: OPEN/APPEND of an indexed file, and WRITE of a lower key after a
$ ! higher one (does /APPEND mean RMS's EOF, and so a $PUT in sequence?).
$ M = "D10"
$ SHOW SYMBOL M
$ OPEN/APPEND F D3.IDX
$ SHOW SYMBOL $STATUS
$ WRITE F "M001 first"
$ SHOW SYMBOL $STATUS
$ WRITE F "A001 lower"
$ SHOW SYMBOL $STATUS
$ WRITE F "Z001 higher"
$ SHOW SYMBOL $STATUS
$ CLOSE F
$ OPEN/APPEND F D3.IDX
$ WRITE F "B001 lower again"
$ SHOW SYMBOL $STATUS
$ R = "-"
$ READ F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ CLOSE F
$ TYPE D3.IDX
$ !
$ ! D11: TYPE of an indexed file and of its open logical name, DUMP/RECORD,
$ ! DIRECTORY/FULL, and F$FILE_ATTRIBUTES.
$ M = "D11"
$ SHOW SYMBOL M
$ TYPE D1.IDX
$ OPEN/READ F D1.IDX
$ R = "-"
$ READ/INDEX=1/KEY="GRPC" F R
$ SHOW SYMBOL R
$ TYPE F
$ R = "-"
$ READ F R
$ SHOW SYMBOL $STATUS
$ SHOW SYMBOL R
$ CLOSE F
$ DUMP/RECORD D1.IDX
$ DUMP/RECORD D2.IDX
$ DIRECTORY/FULL D1.IDX,D2.IDX,D3.IDX
$ N = 0
$ D11LOOP:
$ I = F$ELEMENT(N, ",", "ORG,RFM,RAT,MRS,LRL,FSZ,BKS,GBC,KNOWN,NOK,NOA,PVN,MRN,EOF,ALQ,DEQ,FFB,CTG,CBT")
$ IF I .EQS. "," THEN GOTO D11END
$ V1 = F$FILE_ATTRIBUTES("D1.IDX", I)
$ V2 = F$FILE_ATTRIBUTES("D2.IDX", I)
$ V3 = F$FILE_ATTRIBUTES("D4.DAT", I)
$ WRITE SYS$OUTPUT I, ": ", V1, " ", V2, " ", V3
$ N = N + 1
$ GOTO D11LOOP
$ D11END:
$ !
$ ! The end.
$ M = "END"
$ SHOW SYMBOL M
$ SET NOVERIFY
$ EXIT
