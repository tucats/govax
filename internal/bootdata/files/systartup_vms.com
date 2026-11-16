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
$!  A scratch volume on a RAM disk (HELP DEFINE /DEVICE, HELP MOUNT). A
$!  RAM disk is a volume kept in govax's own memory, empty when govax
$!  starts and gone when it ends: a quick place for temporary files, which
$!  SYLOGIN.COM gives each account a directory on. The lines between the
$!  two marks below make one, MDA0:, of 40000 blocks, labeled SCRATCH, and
$!  make its top directory writable by every account, so that each can
$!  make a directory of its own there. If you don't want it, put "$!" in
$!  front of each of them. All RAM disks together may total no more than
$!  vax.ramdisk.limit blocks (512000 unless set); a failure here is
$!  reported, and startup goes on.
$!
$!  -- scratch volume: begin --
$ set noon
$ define/device/nolog mda0 /maxblock=40000
$ if $status then mount/nolog mda0: scratch
$ if f$getdvi("MDA0","MNT") then -
      set security/protection=(s:rwe,o:rwe,g:rwe,w:rwe) mda0:[000000]000000.dir
$ set on
$!  -- scratch volume: end --
$!
$ exit
