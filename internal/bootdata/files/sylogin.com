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
$ exit
