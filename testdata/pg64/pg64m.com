$ ! PG64M.COM - round 2's monitor (PG64X.COM spawns it): the time and
$ ! SHOW MEMORY/PHYSICAL_PAGES (the modified list's size is its last
$ ! column) once a second, until PG64_DONE is defined in the job table,
$ ! at most 300 times.
$ SET NOVERIFY
$ N = 0
$ LOOP:
$ WRITE SYS$OUTPUT "m ", F$TIME()
$ SHOW MEMORY/PHYSICAL_PAGES
$ IF F$TRNLNM("PG64_DONE","LNM$JOB") .NES. "" THEN EXIT
$ N = N + 1
$ IF N .GE. 300 THEN EXIT
$ WAIT 00:00:01
$ GOTO LOOP
