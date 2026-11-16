$!  SYLOGIN.COM - the system's login procedure, SYS$MANAGER:SYLOGIN.COM on
$!  the system volume, which the logical name SYS$SYLOGIN names.
$!
$!  When an account logs in to govax, DCL runs this procedure first, then
$!  the account's own login procedure: the one its LGICMD names, or
$!  LOGIN.COM in its home directory (SYS$LOGIN:LOGIN.COM). It runs for
$!  every account, SYSTEM's too, so it is the place for commands every
$!  user should have: symbols, logical names, SET TERMINAL, a message of
$!  the day.
$!
$!  govax created this file when it made the system volume, and never
$!  changes or replaces it. Every account can read it (it must, to run
$!  it), and only SYSTEM can change it.
$!
$!  For example:
$!
$!     $ dir :== directory/size/date
$!     $ if f$mode() .eqs. "INTERACTIVE" then write sys$output "Welcome to govax"
$!
$!  A scratch directory for each account on the RAM disk SYSTARTUP_VMS.COM
$!  makes (its "scratch volume" lines): when MDA0: is mounted, the lines
$!  between the two marks below give the account a directory there,
$!  MDA0:[username], that only it (and the system) can use, and point
$!  SYS$SCRATCH at it. If you don't want them, put "$!" in front of each.
$!
$!  -- scratch directory: begin --
$ if f$getdvi("MDA0","EXISTS")
$ then
$     if f$getdvi("MDA0","MNT")
$     then
$         user = f$edit(f$getjpi("","USERNAME"),"TRIM")
$         if f$search("MDA0:[000000]''user'.DIR") .eqs. "" then -
              create/directory/protection=(s:rwed,o:rwed,g,w) mda0:['user']
$         define/job/nolog sys$scratch mda0:['user']
$     endif
$ endif
$!  -- scratch directory: end --
$!
$ exit
