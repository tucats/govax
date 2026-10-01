$ ! BUILD.COM - builds the Phase 33 oracle's test tree. Written by
$ ! testdata/mar/rms3/gen.go. Run it once, with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @BUILD/OUTPUT=BUILD.LOG
$ !
$ SET VERIFY
$ CREATE/DIRECTORY [TEST]
$ CREATE/DIRECTORY [TEST.SUB]
$ CREATE/DIRECTORY [TEST.EMPTY]
$ CREATE/DIRECTORY [OUT]
$ CREATE/DIRECTORY [CRE]
$ CREATE [TEST]A.DAT
A.DAT, the first version
$ CREATE [TEST]A.DAT
A.DAT, the second version
with two lines
$ CREATE [TEST]A.DAT
A.DAT, the third version
$ CREATE [TEST]B.TXT
B.TXT holds several lines,
so that its end-of-file block and
first free byte are worth reading.
The fourth line.
The fifth line, which is rather longer than the others before it.
$ SET FILE/PROTECTION=(S:RWED,O:RWED,G:RE,W) [TEST]B.TXT
$ CREATE [TEST]AB.DAT
AB.DAT
$ CREATE [TEST]C.DAT
[TEST]C.DAT
$ CREATE [TEST.SUB]C.DAT
[TEST.SUB]C.DAT
$ CREATE [TEST.SUB]D.DAT
[TEST.SUB]D.DAT
$ CREATE FIX.FDL
FILE
	ORGANIZATION	sequential
	ALLOCATION	10
	EXTENSION	5
RECORD
	FORMAT	fixed
	SIZE	80
	CARRIAGE_CONTROL	none
$ CREATE/FDL=FIX.FDL [TEST]F.DAT
$ DELETE FIX.FDL;*
$ DIRECTORY/FULL [TEST...]
$ DIRECTORY/FILE_ID [000000...]
