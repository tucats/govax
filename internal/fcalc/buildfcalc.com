$!  BUILDFCALC.COM - builds FCALC.EXE, the scientific calculator written in
$!  Fortran 90, from its sources in the directory this procedure is in,
$!  SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC] on the system volume.
$!
$!  fcalc is govax's example of a whole Fortran 90 program: 16 files, each
$!  a module but the last (FCALC.F90, the main program), commented for a
$!  reader learning Fortran 90 and numerical programming. HELP FCALC
$!  describes it as a user sees it; govax's manual (chapter 14) describes
$!  it as a program.
$!
$!  The modules are compiled in the order below, each before the modules
$!  that USE it: compiling one writes its module file, NAME.F90$MOD, which
$!  the later ones read. fcalc must be compiled /G_FLOATING: its kind,
$!  SELECTED_REAL_KIND(15, 300), needs G_floating's range.
$!
$!  The image is left in the directory; with "INSTALL" as P1 it is also
$!  copied to SYS$SYSTEM:FCALC.EXE, where the FCALC symbol SYLOGIN.COM
$!  defines finds it (a new version of govax's own; delete yours to go
$!  back). To change fcalc, copy the directory's files to one of your own
$!  and run your copy of this procedure there.
$!
$!  Usage: @SYS$SYSROOT:[SYSHLP.EXAMPLES.FCALC]BUILDFCALC [INSTALL]
$!
$ on error then goto failed
$ saved_default = f$environment("DEFAULT")
$ here = f$environment("PROCEDURE")
$ set default 'f$parse(here,,,"DEVICE")''f$parse(here,,,"DIRECTORY")'
$ modules = "FCPLATFORM,FCKINDS,FCMESSAGES,FCVALUES,FCARRAYS,FCLEXER," + -
    "FCTREE,FCSYMBOLS,FCFUNCS,FCPOLY,FCMATRIX,FCEVAL,FCNUMERIC,FCPLOT," + -
    "FCSTATE,FCALC"
$ n = 0
$ compile:
$ module = f$element(n, ",", modules)
$ if module .eqs. "," then goto assemble
$ fortran/g_floating 'module'
$ macro/nolist 'module'
$ n = n + 1
$ goto compile
$ assemble:
$ link/executable=fcalc fcalc,fcplatform,fckinds,fcmessages,fcvalues,-
    fcarrays,fclexer,fctree,fcsymbols,fcfuncs,fcpoly,fcmatrix,fceval,-
    fcnumeric,fcplot,fcstate
$ delete *.mar;*,*.obj;*,*.f90$mod;*
$ if f$edit(p1,"UPCASE") .eqs. "INSTALL" then copy/quiet fcalc.exe sys$system:fcalc.exe
$ set default 'saved_default'
$ write sys$output "FCALC.EXE built"
$ exit
$!
$ failed:
$ status = $status
$ set default 'saved_default'
$ write sys$output "FCALC.EXE was not built"
$ exit status
