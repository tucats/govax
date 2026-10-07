$ ! MACROSERR.COM - the forms that may not exist (README.md). Run it after
$ ! MACROS.COM, with the same default directory:
$ !
$ !     @MACROSERR/OUTPUT=ERRORS.LOG
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/NOLIST ERR_SVC
$ SET NOVERIFY
$ EXIT
