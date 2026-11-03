$!  Copyright (c) 1997-2026 Forest Edge Software
$!                See LICENSE for applicable MIT license information.
$!
$!  STARTUP.COM - govax's startup procedure, SYS$MANAGER:STARTUP.COM on
$!  the system volume (docs/PHASE-87 - system volume.md). govax runs it at
$!  command level 1 when it starts, before any of your commands. It is a
$!  DCL command procedure: each command line starts with "$", and "$!"
$!  starts a comment. A line without a "$" is data, for whatever reads the
$!  procedure's input (here, the debugger).
$!
$!  govax supplies this file, and replaces it with its own newer copy when
$!  it has one, unless you have changed it. Put your own startup commands
$!  in SYS$MANAGER:SYSTARTUP_VMS.COM, which this procedure runs last and
$!  govax never replaces.
$!
$!  The system logical names. govax defines SYS$SYSDEVICE (the system
$!  volume's device) and SYS$SYSROOT (its [SYSTEM] directory) when it
$!  mounts the volume; the rest are defined here, so they can be changed.
$!  With no system volume (govax started with --system-file none, or a
$!  volume it couldn't mount), there is no SYS$SYSDEVICE, and govax's
$!  built-in files are used instead.
$!
$ system_volume = f$trnlnm("SYS$SYSDEVICE") .nes. ""
$ if system_volume
$ then
$    define/system/executive_mode sys$system  sys$sysdevice:[system.sysexe]
$    define/system/executive_mode sys$manager sys$sysdevice:[system.sysmgr]
$    define/system/executive_mode sys$help    sys$sysdevice:[system.syshlp]
$    define/system/executive_mode sys$library sys$sysdevice:[system.syslib]
$    define/system/executive_mode sys$share   sys$sysdevice:[system.syslib]
$    define/system/executive_mode sys$sources sys$sysdevice:[system.sources]
$ endif
$ kernel = """kernel.asm"""
$ rom = ""
$!
$!  The memory's sizes, all in 512-byte pages. Each is the default here
$!  unless govax's configuration sets its vax.memory.* item (see HELP
$!  CONFIG KEYS), so they can be changed without a STARTUP.COM of your
$!  own. A configured value is read as a DCL integer: decimal, or %X for
$!  hex.
$!
$!  SIZE is physical memory (16384 pages is 8MB). P0, P1, and S0 are the
$!  virtual address regions VMINIT maps (0 shares what physical memory is
$!  left among the regions given 0), and KSP, ESP, SSP, and ISP the
$!  kernel, executive, supervisor, and interrupt stacks (ESP, SSP, and
$!  ISP's defaults are VMINIT's own).
$!
$!  With paging on (vax.sysgen.paging.enable), the regions cost virtual
$!  space rather than memory, and P0, P1, and S0 are 0 unless configured:
$!  VMINIT's paging defaults, sized for processes rather than for memory
$!  (see HELP VMINIT).
$!
$ memory_size = 16384
$ if f$config("vax.memory.size", "EXISTS") then -
     memory_size = f$integer(f$config("vax.memory.size"))
$ memory_p0 = 16384
$ memory_p1 = 8192
$ memory_s0 = 8192
$ if f$config("vax.sysgen.paging.enable", "BOOLEAN")
$ then
$    memory_p0 = 0
$    memory_p1 = 0
$    memory_s0 = 0
$ endif
$ if f$config("vax.memory.p0", "EXISTS") then -
     memory_p0 = f$integer(f$config("vax.memory.p0"))
$ if f$config("vax.memory.p1", "EXISTS") then -
     memory_p1 = f$integer(f$config("vax.memory.p1"))
$ if f$config("vax.memory.s0", "EXISTS") then -
     memory_s0 = f$integer(f$config("vax.memory.s0"))
$ memory_ksp = 20
$ if f$config("vax.memory.ksp", "EXISTS") then -
     memory_ksp = f$integer(f$config("vax.memory.ksp"))
$ memory_esp = 8
$ if f$config("vax.memory.esp", "EXISTS") then -
     memory_esp = f$integer(f$config("vax.memory.esp"))
$ memory_ssp = 8
$ if f$config("vax.memory.ssp", "EXISTS") then -
     memory_ssp = f$integer(f$config("vax.memory.ssp"))
$ memory_isp = 4
$ if f$config("vax.memory.isp", "EXISTS") then -
     memory_isp = f$integer(f$config("vax.memory.isp"))
$!
$!  Initialize the physical memory. INIT/VAX's page count is an
$!  expression in the console's radix, so ^D marks it decimal.
$!
$ init/vax ^d'memory_size'
$!
$!   Create page tables for the VAX physical memory.
$!   P0 and P1 may together be larger than physical
$!   memory; their pages take memory only when touched.
$!   With paging off, S0 is mapped to physical memory
$!   and must fit in it; with paging on, it may be
$!   larger.
$!
$ vminit /p0='memory_p0' /p1='memory_p1' /s0='memory_s0' -
     /ksp='memory_ksp' /esp='memory_esp' /ssp='memory_ssp' /isp='memory_isp'
$!
$!	Load the console ROM into ROM storage, if there is one found.
$!	Also allocate the NVRAM storage area.
$!
$ load/rom/noerror 'rom'
$ load/nvram/noerror
$!
$!	Assemble the microkernel.
$!
$! ASM is the console's only until the microkernel is in place; after this,
$! it is a debugger command.
$!
$ asm 'kernel'
$!
$ set quantum 1
$!
$! GO is a debugger command: start the debugger, and give it, as data
$! lines, the commands that run the microkernel's initialization and
$! return to the console.
$!
$ debug
go exe$initialize
exit
$!
$ set debug nouserhalt
$! set debug process
$ set radix dec
$ print " "
$!
$! Create devices
$!
$! (201588743 is 0C040007 hex: REC, CCL, TRM, AVL, IDV, ODV, a terminal's
$! DEVCHAR; 65540 is [1,4], SYSTEM's UIC, the owner VMS 7.1 showed TTA0:.)
$ define/device tta0/devclass=term/devtype=vt100/devbufsiz=80/devchar=201588743/ownuic=65540
$!
$! The null device: writes are discarded and reads are at end of file.
$!
$! (202702849 is 0C150001 hex, the DEVCHAR VMS 7.1 gives NLA0:.)
$ define/device nla0/devclass=mailbox/devtype=null/devchar=202702849/ownuic=65537/devbufsiz=512
$!
$! TT (TTA0:) and SYS$INPUT/OUTPUT/ERROR/COMMAND (the terminal as
$! process-permanent files, _GOVAX$TTA0:) are predefined.
$!
$! Last, the site-specific startup: your own commands, in a file govax
$! creates once and never replaces.
$!
$ if system_volume
$ then
$    if f$search("SYS$MANAGER:SYSTARTUP_VMS.COM") .nes. "" then -
        @sys$manager:systartup_vms
$ endif
$!
$! A command left on govax's own command line (the one-shot command) is
$! run once this file is done, as the command interpreter's input after a
$! login command procedure; govax then exits.
