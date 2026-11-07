$!  BUILDHELP.COM - builds govax's help libraries from their help files,
$!  and installs them in SYS$HELP (docs/PHASE-90 - VMS help.md).
$!
$!  SYS$HELP:HELPLIB.HLB, DCL's HELP, is built from the help files in
$!  SYS$HELP:[.SOURCES], a file for each topic. SYS$HELP:DBG$HELP.HLB, the
$!  debugger's HELP, is built from SYS$HELP:[.SOURCES.DEBUG]DEBUG.HLP, one
$!  module, DEBUG, with each debugger command a topic below it. Each build
$!  is a new version of the library.
$!
$!  To change govax's help, edit a help file and run this procedure. To
$!  add help of your own, LIBRARY/HELP/INSERT a module into HELPLIB.HLB,
$!  or put it in a library of your own and define HLP$LIBRARY to name it.
$!  When a newer govax starts, it gives HELPLIB.HLB its new modules but
$!  keeps any module you changed or added.
$!
$!  Usage: @SYS$SOURCES:BUILDHELP
$!
$ on error then goto failed
$ saved_default = f$environment("DEFAULT")
$ set default sys$help
$ library/create/help helplib.hlb [.sources]*.hlp
$ library/create/help dbg$help.hlb [.sources.debug]*.hlp
$ set default 'saved_default'
$ write sys$output "HELPLIB.HLB and DBG$HELP.HLB built in SYS$HELP"
$ exit
$!
$ failed:
$ status = $status
$ set default 'saved_default'
$ write sys$output "The help libraries were not built"
$ exit status
