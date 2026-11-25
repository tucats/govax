$ ! FOR81M.COM - Phase 113's probe round 2's FORRTL part (testdata/err113's
$ ! README, "Round 2"): FOR81M, the associated variable after FIND, and FOR81W,
$ ! built here and run by hand at the terminal. Run it from [FOR81M]:
$ !
$ !     @FOR81M/OUTPUT=FOR81M.LOG
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST FOR81M
$ LINK/NOMAP FOR81M,FOR81/OPTIONS
$ MACRO/NOLIST FOR81W
$ LINK/NOMAP FOR81W,FOR81/OPTIONS
$ ! Through a procedure with /OUTPUT, so that unit 6's records join the cases'
$ ! lines in one file.
$ @FOR81N/OUTPUT=FOR81M-VAX.TXT
$ SHOW SYMBOL $STATUS
$ EXIT
