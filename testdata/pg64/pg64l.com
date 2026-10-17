$ ! PG64L.COM - the subprocess Phase 64's probe spawns (PG64.COM): the
$ ! program's part T touches 300 pages, then LOGOUT/FULL shows the peak
$ ! page file size.
$ SET VERIFY
$ WRITE SYS$OUTPUT "sub pgflquota=", F$GETJPI("","PGFLQUOTA"), " pagfilcnt=", F$GETJPI("","PAGFILCNT")
$ DEFINE PG64_PART T
$ RUN PG64
$ WRITE SYS$OUTPUT "sub pgflquota=", F$GETJPI("","PGFLQUOTA"), " pagfilcnt=", F$GETJPI("","PAGFILCNT")
$ LOGOUT/FULL
