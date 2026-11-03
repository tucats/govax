$!  SYSTARTUP_VMS.COM - your own startup commands, SYS$MANAGER:SYSTARTUP_VMS.COM
$!  on the system volume (docs/PHASE-87 - system volume.md).
$!
$!  SYS$MANAGER:STARTUP.COM runs this procedure last, after the machine is
$!  started and the system logical names are defined. govax created this
$!  file when it made the system volume, and never changes or replaces
$!  it, so it is the place for commands of your own: logical names,
$!  devices, volumes to mount, symbols, SET commands.
$!
$!  For example:
$!
$!     $ define/system work$disk dua1:
$!     $ set prompt="MYVAX> "
$!
$ exit
