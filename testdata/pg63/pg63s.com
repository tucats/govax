$ ! PG63S.COM - the subprocess's part of Phase 63's probe (PG63.COM spawns
$ ! it): its own working set values, and SET WORKING_SET's bounds there.
$ SET VERIFY
$ SHOW WORKING_SET
$ WRITE SYS$OUTPUT "sub dfwscnt=", F$GETJPI("","DFWSCNT"), " wssize=", F$GETJPI("","WSSIZE"), -
	" quota=", F$GETJPI("","WSQUOTA"), " extent=", F$GETJPI("","WSEXTENT"), -
	" auth=", F$GETJPI("","WSAUTH"), " authext=", F$GETJPI("","WSAUTHEXT")
$ SET WORKING_SET/QUOTA=100000/LOG
$ SHOW WORKING_SET
$ EXIT
