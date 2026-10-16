$ ! PG63X.COM - one SET WORKING_SET near or below the floor, in a
$ ! subprocess PG63.COM spawns for it (P1 holds the qualifiers). The
$ ! values are shown with F$GETJPI, which needs no image, since the SET
$ ! may leave the process unable to run one.
$ SET VERIFY
$ WRITE SYS$OUTPUT "x dfwscnt=", F$GETJPI("","DFWSCNT"), " wssize=", F$GETJPI("","WSSIZE"), -
	" quota=", F$GETJPI("","WSQUOTA"), " extent=", F$GETJPI("","WSEXTENT"), -
	" auth=", F$GETJPI("","WSAUTH"), " authext=", F$GETJPI("","WSAUTHEXT")
$ SET WORKING_SET'P1'
$ WRITE SYS$OUTPUT "x status=", $STATUS
$ WRITE SYS$OUTPUT "x dfwscnt=", F$GETJPI("","DFWSCNT"), " wssize=", F$GETJPI("","WSSIZE"), -
	" quota=", F$GETJPI("","WSQUOTA"), " extent=", F$GETJPI("","WSEXTENT"), -
	" auth=", F$GETJPI("","WSAUTH"), " authext=", F$GETJPI("","WSAUTHEXT")
$ SHOW WORKING_SET
$ EXIT
