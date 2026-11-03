$!  BUILDSTARLET.COM - builds STARLET.MLB, govax's system macro library, from
$!  its sources in SYS$SOURCES, and installs it as SYS$LIBRARY:STARLET.MLB
$!  (docs/PHASE-87 - system volume.md).
$!
$!  STARLET.MAR holds the system service and RMS macros, STARLETDEF.MAR
$!  the $xxxDEF definition macros. MACRO looks in SYS$LIBRARY:STARLET.MLB
$!  before the copy built into govax, so the library this builds is the one
$!  programs assemble with from then on. Each build is a new version of
$!  the file.
$!
$!  Usage: @SYS$SOURCES:BUILDSTARLET
$!
$ on error then goto failed
$ saved_default = f$environment("DEFAULT")
$ set default sys$sources
$ library/create/macro starlet.mlb starlet.mar,starletdef.mar
$ copy starlet.mlb sys$library:starlet.mlb
$ delete starlet.mlb;*
$ set default 'saved_default'
$ write sys$output "STARLET.MLB built and installed in SYS$LIBRARY"
$ exit
$!
$ failed:
$ status = $status
$ set default 'saved_default'
$ write sys$output "STARLET.MLB was not built"
$ exit status
