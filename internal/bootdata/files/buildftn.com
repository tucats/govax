$!  BUILDFTN.COM - builds FTNRTL.EXE, govax's FORTRAN run-time library, from
$!  its sources in SYS$SOURCES, and installs it as SYS$SHARE:FTNRTL.EXE
$!  (docs/PHASE-87 - system volume.md).
$!
$!  RUN and LINK find SYS$SHARE:FTNRTL.EXE before the copy built into
$!  govax, so the library this builds is the one programs use from then
$!  on. Each build is a new version of the file; to go back to govax's,
$!  delete the versions you made (or PURGE, then rename), or start govax
$!  once with --system-initialize.
$!
$!  The modules are assembled in the order FTNRTL.OPT links them. To add a
$!  module, add its MACRO line here and its object to FTNRTL.OPT; to add an
$!  entry point, add it to FTN_VECTOR.MAR's transfer vector and raise the
$!  minor number of FTNRTL.OPT's GSMATCH.
$!
$!  Usage: @SYS$SOURCES:BUILDFTN
$!
$ on error then goto failed
$ saved_default = f$environment("DEFAULT")
$ set default sys$sources
$ macro/nolist ftn_vector
$ macro/nolist ftn_control
$ macro/nolist ftn_units
$ macro/nolist ftn_statement
$ macro/nolist ftn_format
$ macro/nolist ftn_edit
$ macro/nolist ftn_float
$ macro/nolist ftn_listout
$ macro/nolist ftn_listin
$ macro/nolist ftn_namelist
$ macro/nolist ftn_internal
$ macro/nolist ftn_open
$ macro/nolist ftn_inquire
$ macro/nolist ftn_position
$ macro/nolist ftn_unformatted
$ macro/nolist ftn_direct
$ macro/nolist ftn_record
$ macro/nolist ftn_intrinsics
$ macro/nolist ftn_system
$ link/shareable=ftnrtl/nosyslib ftnrtl/options
$ copy ftnrtl.exe sys$share:ftnrtl.exe
$ delete ftn_*.obj;*,ftnrtl.exe;*
$ set default 'saved_default'
$ write sys$output "FTNRTL.EXE built and installed in SYS$SHARE"
$ exit
$!
$ failed:
$ status = $status
$ set default 'saved_default'
$ write sys$output "FTNRTL.EXE was not built"
$ exit status
