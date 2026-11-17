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
$!  A batch queue (HELP SUBMIT, HELP START /QUEUE). The queue manager
$!  keeps its queues and jobs in a host file (vax.queue.file, by default
$!  ~/.govax/qman.json) that outlives govax, and starts again by itself
$!  when govax starts, but every queue comes back stopped: these lines,
$!  once you take the "$! " from the front of each, start the manager the
$!  first time, make the batch queue SYS$BATCH if there is none, and start
$!  it, so that SUBMIT works without /QUEUE. They need a SYSTEM session
$!  (OPER and SYSNAM). If another govax session runs the queue manager,
$!  they say so, and startup goes on.
$!
$!  -- batch queue: begin --
$! $ set noon
$! $ start/queue/manager
$! $ if f$getqui("DISPLAY_QUEUE","QUEUE_NAME","SYS$BATCH") .eqs. "" then -
$!      initialize/queue/batch sys$batch
$! $ start/queue sys$batch
$! $ set on
$!  -- batch queue: end --
$!
$ exit
