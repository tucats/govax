# Phase 96 — RAM disks

**Status:** planned 2026-11-16 (subtask 1, the survey and this plan).
The author reviewed it and accepted it the same day, with the answers
recorded in "The review". This document is the development plan; as the
work goes on it becomes the implementation record: the progress log, and
at the end "Where things stand".

## Goal

govax gets RAM disks: ODS-2 volumes whose blocks are held in govax's
own Go memory, not in a host container file and not in the emulated
VAX's memory. They are named as VMS's DECram disks are, `MDcu`: MDA0,
MDA1, and so on.

- **`DEFINE/DEVICE MDA`** makes the next free unit of the MDA controller
  (MDA0 first), a disk of device type DT$_RAM_DISK, its size in blocks
  given by `/MAXBLOCK`. `DEFINE/DEVICE MDA3` names the unit itself. It
  needs CMKRNL.
- **`MOUNT MDA0: [label]`** mounts the device. The first mount gives it
  a newly initialized, empty ODS-2 volume. Later mounts find the volume
  as the last DISMOUNT left it. No container file is named.
- **The contents last as long as govax runs.** DISMOUNT keeps them, and
  so does mounting again. Only defining the unit again, or the end of
  govax, throws them away. Nothing is written to the host.
- **The total of all RAM disks is limited** by `vax.ramdisk.limit`,
  512,000 blocks (250 MB) unless it's set, so that a user can't use up
  too much host memory by mistake.
- **A scratch volume** is something the user builds in the startup and
  login procedures. The manual gives the recipe, and the procedures
  govax supplies carry it as comments.

The session below is what the plan expects; the messages are govax's own
(Decision 8):

    $ DEFINE/DEVICE MDA /MAXBLOCK=20000
    %VAX-I-RAMDISK, _MDA0: defined, 20000 blocks
    $ MOUNT MDA0: SCRATCH
    %MOUNT-I-MOUNTED, SCRATCH mounted on _MDA0:
    $ CREATE/DIRECTORY MDA0:[WORK]
    $ COPY LOGIN.COM MDA0:[WORK]
    $ SHOW DEVICE MDA

    Device                  Device           Error    Volume         Free  Trans Mnt
     Name                   Status           Count     Label        Blocks Count Cnt
    MDA0:                   Mounted              0  SCRATCH          19876     0   1
    $ DISMOUNT MDA0:
    $ MOUNT MDA0: SCRATCH
    %MOUNT-I-MOUNTED, SCRATCH mounted on _MDA0:
    $ DIRECTORY MDA0:[WORK]

    Directory MDA0:[WORK]

    LOGIN.COM;1

    Total of 1 file.

The work is in two repositories. **ods2** gets a container that keeps
its blocks in memory (`diskimage.NewMemory`), with views that can be
closed without freeing it. **govax** gets RMS's RAM disks, MD devices in
DEFINE/DEVICE, MOUNT, DISMOUNT, and SHOW DEVICE, the limit, and the
scratch recipe.

## Part A — the survey (2026-11-16)

### A.1 What VMS has

DECram is DIGITAL's RAM disk product for VMS. It is a layered product,
not part of the base system, and none of our references covers it. The
author's simh VAX doesn't have it. What follows is from memory and
**unconfirmed**:

- Its devices are disks named **MDcu** (MDA0, ...), served by the driver
  SYS$MDDRIVER. Each is a file-structured disk (DEV$M_FOD, DEV$M_RND,
  DEV$M_DIR, ...) of device type **DT$_RAM_DISK**. That symbol is in
  `vmsdef` with value 58 (`%X3A`), so that much is certain.
- In the older releases the operator connected the device with SYSGEN
  (`CONNECT MDA0/NOADAPTER/DRIVER=SYS$MDDRIVER`), gave it a size with
  `INITIALIZE/SIZE=n MDA0: label`, and mounted it with `MOUNT/SYSTEM`.
  The later releases have a utility of their own (`DECRAM> CREATE ...`).
- The disk's memory belongs to the device, not to the mount, so DISMOUNT
  and MOUNT keep what's on it. Its contents are lost at shutdown or after
  a failure. People used it for scratch files, compiler work files, and
  sort work files.

govax keeps VMS's **names**, its **device type**, and the rule that the
**contents belong to the device**. It doesn't copy DECram's commands,
since we have no manual for them. The author asked for DEFINE/DEVICE,
which govax already has, as the way to make a device.

### A.2 What govax has today

- **DEFINE/DEVICE** (`console.dcl`'s `define_device` syntax, bound in
  `internal/console/dispatch.go`; `dclcmd/device.go`'s `DefineDevice`)
  adds a record to `iodev.DeviceTable` with the qualifiers' values:
  `/MAXBLOCK`, `/CLUSTER`, `/VOLNAME`, `/DEVCLASS`, `/DEVTYPE`, and the
  rest. It says nothing, needs no privilege, and a name defined twice
  resolves to the newer record (`DeviceTable.Define` prepends). A name
  that is a known disk type (`KnownDeviceOptions`) gets that disk's
  geometry. There is no way to delete a device.
- **MOUNT** (`dclcmd/mount.go`'s `MountCommand` and `Mount`) needs SYSNAM
  (`corevms/commandrules.go`, Phase 94). It resolves a bare controller
  to its lowest unmounted unit (`ResolveUnit`, `dclcmd/units.go`: the
  regular expression `^[A-Z]{3}$`, so `MDA` already counts as a
  controller). It mounts a host container file through
  `rms.MountTable.Mount`, defines a device record for a device that has
  none, defines `DISK$label`, and reports `%MOUNT-I-MOUNTED`. The
  grammar's second parameter is the **container file**, with a prompt.
  On VMS, MOUNT's second parameter is the volume label.
- **`rms.MountTable.Mount`** (`internal/rms/mount.go`) opens a path with
  `diskimage.Open` or `OpenWritable`, then calls `volume.Mount`. The
  mounted volume's container is reached again only by the logical I/O
  functions (`acplogical.go`) and by `Dismount`, which calls
  `volume.Dismount`. That call flushes the bitmaps and **closes each
  container**. `DismountAll`, run by `cmd/govax` when a session ends,
  dismounts every volume.
- **INITIALIZE/CONTAINER** (`rms.InitializeContainer`) makes a host file
  with `diskimage.Create` and formats it with `volume.Initialize`. The
  same two steps, over a memory container, are a RAM disk's first mount.
- **`$GETDVI`** (`corevms/getdvi.go`) answers from the device record and
  the mount table. It adds DEV$M_MNT and DEV$M_SWL for a mounted volume,
  and gives DVI$_ACPTYPE F11V2 for every volume.
- **SHOW DEVICE** names the device type from `iodev.deviceTypeNames`,
  which has no entry for 58. Its disk lines and `/FULL` layout read the
  mount table's live statistics.
- **SYS$SCRATCH** is defined at login, in the job table at executive
  mode, as the home directory (`dclshell/dcllogin.go`'s
  `SetLoginDirectory`, Phase 95). A supervisor-mode definition in the
  same table (SYLOGIN.COM's `DEFINE/JOB`) comes first in a translation,
  so a login procedure can point it elsewhere. Nothing in govax writes
  files there on its own.
- **SYSTARTUP_VMS.COM** and **SYLOGIN.COM** are on the system volume.
  Both are seeded (Phase 87, Phase 95): written only when missing, and
  never replaced, so a new example in them reaches only new volumes.

### A.3 What ods2 has today

- **`diskimage.Container`** has three methods (`ReadBlock`, `Blocks`,
  `Close`), and **`WritableContainer`** adds `WriteBlock`. The two
  implementations, `plainImage`/`writablePlainImage` and `rawCDImage`,
  sit over an `*os.File`. `Create` makes a sparse host file.
- **`volume.Initialize(c WritableContainer, InitializeOptions)`** formats
  any writable container. It takes a label, owner UIC, volume and file
  protection, cluster size, maximum files, and headers. **`volume.Mount`**
  takes any containers. Neither one cares where the blocks live.
- **`Volume.Dismount`** writes back what's held in memory, then closes
  every container.
- govax pins ods2 at v0.1.30, and a local `go.work` uses the sibling
  checkout (CLAUDE.md).

So ods2 needs two things: a container that isn't a file, and a way for
`Volume.Dismount` to close it without freeing the blocks.

### A.4 What the references leave open

1. DECram's SHOW DEVICE type name: "RAM_DISK", "DECRAM", or something
   else. govax will say `RAM_DISK`, from the symbol's name
   (**unconfirmed**).
2. DECram's DEVCHAR bits beyond those of an ordinary disk. govax will use
   an ordinary mounted disk's (**unconfirmed**).
3. How DECram's MOUNT treats a label that doesn't match the volume's.
   govax follows MOUNT's rule for any disk, `%MOUNT-F-INCVOLLABEL`
   (**unconfirmed** for DECram).

The simh VAX has no DECram, so these stay unconfirmed and the phase has
**no VMS probe**. The behavior is govax's own, written up in
DEVIATIONS.md.

## Part B — decisions

**Decision 1 — ods2: a memory image, sparse, with views.** In ods2,
`diskimage/memory.go` adds:

    // NewMemory returns a memory image of blocks logical blocks, all
    // zero, held in Go memory.
    func NewMemory(blocks uint32) (*MemoryImage, error)

    // View returns a WritableContainer over m's blocks. Closing the
    // view ends that view only; m and its blocks stay.
    func (m *MemoryImage) View() WritableContainer

    func (m *MemoryImage) Blocks() uint32
    func (m *MemoryImage) Resident() int64 // bytes held now
    func (m *MemoryImage) Release()        // frees the blocks; m is done

A **view** is what `volume.Initialize` and `volume.Mount` are given. When
`Volume.Dismount` closes it, the image keeps its blocks for the next
mount, which is how the contents outlive a dismount without changing
ods2's dismount. Reads and writes through a closed view, and every call
on a released image, return `ErrClosed`. `MemoryImage` also implements
`WritableContainer` itself, its `Close` the same as `Release`, for
ods2's own tests and any user that wants a plain memory disk.

The blocks are stored in **chunks of 128 blocks (64 KB)**, kept in a map
by chunk number and allocated on the first write of a nonzero block.
Writing a block of zeros to a chunk that doesn't exist allocates
nothing, and reading a missing chunk gives zeros. A large RAM disk
therefore costs only what has been written to it, as a sparse host file
does. This matters because `volume.Initialize` writes zeros over the
index file's and bitmap's blocks. A chunk is never given back while the
image lives, even after the files in it are deleted: ods2 can't know
which blocks are free, and a scratch volume's high-water mark is the
right cost. The usual errors apply: a block number past the end, and a
buffer shorter than a block (`ErrBufferTooSmall`).

A `sync.Mutex` guards the map. govax calls ods2 from one goroutine at a
time (strict handoff), but ods2 is a library of its own, and the lock
costs nothing that matters next to a 512-byte copy. The memory comes
from Go's heap, never from the VAX's memory (`vm.Memory`). That is the
author's requirement, and it's why the image lives in ods2 and not in
govax's `vm`.

**Decision 2 — ods2 is released before govax uses it.** ods2's work is
one commit with its tests, in ods2's own style. Per ods2's CLAUDE.md,
its commit messages carry **no AI-attribution lines**, unlike govax's.
It is then tagged (v0.1.31) and pushed by the author. govax moves to it
with `GOWORK=off go get github.com/tucats/ods2@v0.1.31`, and
`GOWORK=off go build ./...` is checked before govax's commit (CLAUDE.md).
Until the tag exists, govax's subtasks build through `go.work`.

**Decision 3 — RMS keeps the RAM disks.** A RAM disk's memory belongs to
its device, so it can't live in a mount table entry, which goes away at
DISMOUNT. `rms.MountTable` gains a second map, `ramDisks`, from device
name to:

    type ramDisk struct {
        image  *diskimage.MemoryImage
        blocks uint32
        formatted bool // a first mount has initialized it
    }

and these methods:

    // DefineMemory makes device a RAM disk of blocks blocks, discarding
    // any volume it held. It fails if the device is mounted, or if the
    // RAM disks would then total more than limit blocks.
    func (t *MountTable) DefineMemory(device string, blocks, limit uint32) error

    // MountMemory mounts device's RAM disk: on its first mount a new
    // volume made with opts, after that the volume it holds.
    func (t *MountTable) MountMemory(device string, writable bool, opts MemoryVolume) error

    type MemoryVolume struct {
        Label       string       // the volume label
        ClusterSize uint16       // 0 for INITIALIZE's default
        Owner       security.UIC
        Protection  uint16       // the volume's; 0 for none
    }

    func (t *MountTable) IsMemory(device string) bool
    func (t *MountTable) MemoryInUse(device string) int64
    func (t *MountTable) MemoryTotal() (defined uint32, resident int64)
    func (t *MountTable) ReleaseMemory() // every RAM disk, at the session's end

`Mount` is split. Its open step stays where it is, and a new
`mountContainer(key, container, writable)` does the rest (`volume.Mount`,
the table entry). `MountMemory` uses it with a view of the image, after
`volume.Initialize` of another view on the first mount. The entry
remembers it's a RAM disk, so `Dismount` leaves the image alone. Its
flush writes the bitmaps into the image, and the next `volume.Mount`
reads them back. `rms.InitializeContainer` and `MountMemory` share one
`initializeOptions` helper, so INITIALIZE's defaults are the same for
both.

The logical I/O functions, `VerifyVolume`, and every ACP and RMS service
work unchanged, since they see only a `*volume.Volume` and a `Container`.
`cmd/govax` calls `ReleaseMemory` after `DismountAll` when the session
ends. The process ending would free the memory anyway, but releasing it
explicitly keeps tests that start many sessions from holding it.

**Decision 4 — the contents last until govax ends.** The first MOUNT of
an MD device initializes it. Each later one mounts what's there. DISMOUNT
refuses a volume with files open (`FilesOpenError`), as it does for any
volume, and otherwise keeps the contents. A user who wants an empty
volume again defines the unit again (Decision 5), which discards the old
one. govax's end discards every RAM disk. Nothing ever reaches the host.

**Decision 5 — MD devices in DEFINE/DEVICE.** A DEFINE/DEVICE name of
the form `MDc` or `MDcu` (`^MD[A-Z][0-9]*$`, after removing `_` and `:`)
is a RAM disk:

- `MDc` alone is that controller's **lowest unit not yet defined**.
  This differs from `ResolveUnit`'s lowest *unmounted* unit, because a
  RAM disk must be defined before it can be mounted. `MDcu` is that unit.
- The record is class disk, type 58 (DT$_RAM_DISK), with DEVCHAR an
  ordinary disk's (FOD, DIR, RND, AVL, SHR, IDV, ODV). `/MAXBLOCK` is its
  size, and `/CLUSTER` and `/VOLNAME` are the defaults for its first
  mount's cluster size and label. A `/DEVCLASS` or `/DEVTYPE` that says
  otherwise is refused (`%CLI-W-CONFLICT`). `dev_type` gains the keyword
  `RAM_DISK` (`/id=58`), and `deviceTypeNames` gains 58, `RAM_DISK`.
- With no `/MAXBLOCK`, the size is `vax.ramdisk.size`, or else 65,536
  blocks (32 MB). A disk must have at least 100 blocks.
- **The limit is on the total.** The sizes of all the RAM disks defined,
  mounted or not, may not add up to more than `vax.ramdisk.limit`
  blocks. The default, when the key isn't set, is **512,000 blocks
  (250 MB)**. Sizes are counted as defined, not as used. The container
  is sparse, so a disk costs only what has been written to it, but
  counting definitions means a disk can never grow past what was allowed
  when it was made, whatever is later copied onto it. A disk defined
  again counts at its new size in place of its old one. A definition
  that would go over the total is refused, and nothing changes:

      %VAX-E-RAMLIMIT, RAM disks would total 540000 blocks, over the limit of 512000
      -VAX-I-RAMKEY, the limit is configuration key vax.ramdisk.limit

  The key is read when a device is defined, so lowering it doesn't
  affect disks that already exist. It only stops new ones until the
  total is under it. A value the key can't take (not a number, or under
  100) is reported at startup, as other bad `vax.sysgen.*` values are,
  and the default is used.
- Defining a unit that is **mounted** is refused (`%SYSTEM-F-DEVMOUNT`).
  Defining an unmounted one again **discards its volume** and replaces
  its size. The message says so: `%VAX-I-RAMDISK, _MDA0: defined, 20000
  blocks, previous contents discarded`.
- Defining an MD device needs **CMKRNL**, as VMS's SYSGEN CONNECT does:
  it consumes host memory, a resource shared by the whole system. A new
  `commandRules` entry (`"DEFINE_DEVICE_MD"`) and a `PrivilegeRule` cover
  it (Phase 94's gate, `TestEveryPrivilegeRuleUsed`). Other devices'
  DEFINE/DEVICE stays as it is.
- `define_device` gains `/[NO]LOG`. Unless it's `/NOLOG`, and when the
  console is verbose, an MD device's definition reports `%VAX-I-RAMDISK`,
  because a procedure that said only `MDA` needs to see which unit it
  got. A procedure can also find the unit with `F$GETDVI` or
  `F$DEVICE("MDA*")`, which already work on the device table.

**Decision 6 — MOUNT and DISMOUNT of an MD device.** In
`MountCommand`, after `ResolveUnit` (where `MDA` gives the lowest
unmounted unit, which must also be defined):

- An MD device that isn't defined is `%SYSTEM-F-NOSUCHDEV`. MOUNT
  doesn't make one, as it does for DU devices, because it wouldn't know
  the size.
- The second parameter, if given, is the **volume label**, as on VMS.
  On the first mount it names the new volume. Otherwise the label is the
  record's `/VOLNAME`, and otherwise the device name (`MDA0`). On a later
  mount the label must match the volume's, or MOUNT fails with
  `%MOUNT-F-INCVOLLABEL` (A.4, 3). With no label, any volume mounts.
- The grammar's FILE parameter loses its `/prompt=`, and the handler
  asks for "Container file" itself (through `Context.ReadReply`) for
  any other device, so `MOUNT DUA1` still prompts as it does now.
- On the first mount, the volume's owner is the mounting process's UIC,
  and its protection is (S:RWED, O:RWED, G:RWED, W:RWED). Each
  directory's own protection then decides access, as on a VMS scratch
  disk. `/NOWRITE` is refused on a first mount (`%VAX-W-RAMNOWRITE`),
  because a read-only empty volume has no use. On a later mount it is
  allowed, as for any disk.
- The rest is MOUNT's code as it stands: SYSNAM, `DISK$label`, the
  MOUNTED message.

DISMOUNT needs nothing new. The device's record and its contents stay
(Decision 4).

**Decision 7 — what a RAM disk shows.** SHOW DEVICE's brief line is a
disk's, mounted or not. `/FULL` names the type `RAM_DISK`, and adds two
lines of govax's own after the volume block:

    Host memory in use          10240 KB    RAM disks total     20000 of 512000 blocks

The first figure is `MemoryInUse`, and the second is `MemoryTotal`
against the limit. `$GETDVI` and F$GETDVI give DEVTYPE 58 and MAXBLOCK,
and the rest as for any disk. SHOW MEMORY is about the VAX's memory, so
it doesn't count RAM disks. The machine's SAVE and LOAD of its state
don't include RAM disks. A page file may be put on a RAM disk
(`vax.sysgen.paging.file`). That works, though the pages are still in
host memory, and nothing refuses it.

**Decision 8 — messages.** The new messages are govax's own, in the VAX
facility (`vmserrors`):

- `VAX-I-RAMDISK`: a unit defined, and whether old contents were
  discarded.
- `VAX-E-RAMLIMIT`, followed by `VAX-I-RAMKEY`: the total would be over
  the limit, and the key that sets it.
- `VAX-E-RAMSIZE`: a disk under 100 blocks.
- `VAX-W-RAMNOWRITE`: MOUNT/NOWRITE on a RAM disk's first mount.

The rest are VMS's existing statuses: NOSUCHDEV, DEVMOUNT, NOPRIV,
INCVOLLABEL, and CLI-W-CONFLICT.

**Decision 9 — the scratch volume is a recipe.** govax doesn't make a
scratch volume itself. MANUAL.md gives the recipe. A block in
SYSTARTUP_VMS.COM makes and mounts the disk at startup:

    $ DEFINE/DEVICE/NOLOG MDA0 /MAXBLOCK=40000
    $ MOUNT/NOLOG MDA0: SCRATCH

and a block in SYLOGIN.COM gives each login a directory there:

    $ IF F$GETDVI("MDA0","MNT")
    $ THEN
    $     USER = F$EDIT(F$GETJPI("","USERNAME"),"TRIM")
    $     IF F$SEARCH("MDA0:[000000]''USER'.DIR") .EQS. "" THEN -
              CREATE/DIRECTORY/PROTECTION=(S:RWED,O:RWED,G,W) MDA0:['USER']
    $     DEFINE/JOB SYS$SCRATCH MDA0:['USER']
    $ ENDIF

The bootdata copies of both procedures carry these blocks as comments,
so a new system volume has them ready to uncomment. Existing volumes keep
their own copies, since both files are seeded. Subtask 6 runs the recipe
in a test, which checks that `SYS$SCRATCH` translates to the new
directory for SYSTEM and for a second account (Phase 95's test
accounts), that each account can write its own directory and not the
other's, and that a subprocess sees the same name. Writing that test may
show that the recipe needs something govax doesn't yet do, such as the
directory's owner when a non-privileged user creates it at the volume's
top. If so, the fix is made in the same subtask.

**Decision 10 — tests.**

- ods2: the memory image's reads, writes, zeros, range and buffer
  errors, views (a closed view leaving the image whole, a second view
  reading what the first wrote), `Release`, sparseness (`Resident` after
  writes of zeros and of data), and `Initialize`, `Mount`, `Dismount`,
  `Mount` again over views, finding the file written before.
- `rms`: `DefineMemory` (the total, refusals, discarding on redefinition),
  `MountMemory` (first and later mounts, label, cluster, owner), files
  surviving DISMOUNT, `VerifyVolume` clean after a remount,
  `DismountAll` keeping the images, and `ReleaseMemory` freeing them.
- `console`: `TestRAMDisk`, the session in "Goal" end to end. It covers
  unit choice (MDA, then MDA1; MDA3 explicit); the refusals of Decisions
  5 and 6; the total limit at its default, lower, and higher, across
  several disks, and with a redefinition; contents surviving DISMOUNT
  and lost on redefinition; INCVOLLABEL; SHOW DEVICE in both forms;
  F$GETDVI; CMKRNL; and a FORTRAN or MACRO program writing a file there.
  `TestScratchRecipe` is Decision 9's.
- The help checks, `TestEveryQualifierHasHelp` (`/LOG` for DEFINE/DEVICE)
  and `TestConfigKeysHaveHelp` (`vax.ramdisk.size` and
  `vax.ramdisk.limit`), hold the documentation.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey** (done 2026-11-16): Part A, this plan.
2. **ods2's memory image** (done 2026-11-16) (Decisions 1 and 2): `diskimage/memory.go`
   (the image, views, `Release`) and its tests, ods2's package
   documentation (`diskimage/doc.go`), and a line in ods2's README.
   Committed in ods2 with no attribution lines, for the author to tag
   and push as v0.1.31.
3. **RMS's RAM disks** (Decision 3): `ramDisks`, `DefineMemory`,
   `MountMemory`, `mountContainer`, `MemoryVolume`, `IsMemory`,
   `MemoryInUse`, `MemoryTotal`, `ReleaseMemory`, the shared
   `initializeOptions`, and `cmd/govax`'s release at the session's end.
   `go.mod` is moved to ods2 v0.1.31 once it's tagged.
4. **MD devices** (Decision 5): DEFINE/DEVICE's MD rule (unit choice,
   size, the total limit, redefinition, the record, `/LOG`), `dev_type`'s
   `RAM_DISK` and `deviceTypeNames`, the CMKRNL rule, the configuration
   keys `vax.ramdisk.size` and `vax.ramdisk.limit` in `validConfigs`, and
   the messages.
5. **MOUNT, DISMOUNT, and SHOW** (Decisions 4, 6, and 7): first and later
   mounts of an MD device, the label parameter and INCVOLLABEL, the FILE
   parameter's prompt moved into the handler, owner and protection,
   `/NOWRITE`; SHOW DEVICE's type name and memory lines; `$GETDVI`
   checked; `TestRAMDisk`.
6. **The scratch recipe** (Decision 9): the commented blocks in
   bootdata's `systartup_vms.com` and `sylogin.com` (and the system
   volume's manifest for the supplied side, if either is supplied), and
   `TestScratchRecipe`, with any fix it needs.
7. **The close-out**: MANUAL.md (section 5's volumes: "RAM disks" and
   the scratch recipe; DEFINE/DEVICE and MOUNT; the two keys), HELP
   DEFINE `/DEVICE`, HELP MOUNT, and HELP CONFIGURATION KEYS,
   VAX-VMS-INTERNALS.md's I/O and RMS sections (a container need not be
   a file), DEVIATIONS.md (A.4's unconfirmed points), PLAN.md, and
   CLAUDE.md.

## The review

Questions for the author:

1. **Contents across DISMOUNT** (Decision 4): every MOUNT starts with an
   empty volume and DISMOUNT frees the memory, as the request says, or
   should the device keep its volume until govax ends, as DECram does?
2. **MOUNT's second parameter** (Decision 6): for an MD device, the
   volume label, as on VMS, while for other devices it stays the
   container file?
3. **Privilege** (Decision 5): CMKRNL to define an MD device, or no
   privilege, as DEFINE/DEVICE needs today?
4. **Sizes** (Decision 5): a limit per disk, or a limit on the total of
   all RAM disks?
5. **The scratch volume** (Decision 9): configuration keys that make it
   in Go at startup, or only a DCL recipe in the manual?
6. **The probe**: does the simh VAX have DECram?

The answers (2026-11-16):

1. A RAM disk keeps its contents for as long as govax runs (Decision 4).
   Defining the unit again is the way to start over (Decision 5), and
   ods2's image gets views so that a dismount doesn't free it
   (Decision 1).
2. Yes, the label (Decision 6). A later mount checks it (INCVOLLABEL).
3. CMKRNL (Decision 5).
4. The limit is on the total of all RAM disks: `vax.ramdisk.limit`,
   512,000 blocks (250 MB) unless set, counting each disk's defined size
   (Decision 5).
5. Only a DCL recipe. Users make RAM disks in the startup procedures
   (Decision 9), and the `vax.scratch.*` keys are dropped.
6. The simh VAX has no DECram, so there is no probe and A.4 stays
   unconfirmed.

## Progress log

- **2026-11-16** Subtask 1: read DEFINE/DEVICE (`console.dcl`'s
  `define_device`, `dclcmd/device.go`, `iodev.DeviceTable`), MOUNT and
  DISMOUNT (`dclcmd/mount.go`, `units.go`'s `ResolveUnit`),
  `rms.MountTable` and `InitializeContainer`, `$GETDVI`'s DEVCHAR, the
  session's end (`cmd/govax`'s `DismountAll`), `SetLoginDirectory`'s
  SYS$SCRATCH, and ods2's `diskimage` containers, `volume.Initialize`'s
  options, and `Volume.Dismount`. Found DT$_RAM_DISK (58) in `vmsdef`
  and no DECram documentation among the references. Wrote Part A and
  this plan. While the plan was being written, the author asked for
  VMS's DECram names, MDcu, in place of the RAMu first proposed, and for
  a configuration key limiting RAM disks' memory, 250 MB unless set.
- **2026-11-16** The review: the author accepted the plan and answered
  the six questions. Contents now last until govax ends (Decisions 1,
  3, and 4 reworked: views, RMS's `ramDisks`, redefinition to start
  over). The limit is on the total. The scratch volume is a DCL recipe
  (Decision 9), not configuration keys. There is no probe.
- **2026-11-16** Subtask 2: ods2's `diskimage/memory.go`, as Decision 1
  has it. `NewMemory` returns a `MemoryImage`, whose blocks are 64 KB
  chunks in a map, allocated by the first write of a block that isn't all
  zeros. `View` returns a `WritableContainer` whose `Close` ends only that
  view. `Release` frees the chunks, and every later call on the image or
  its views gives `ErrClosed`. `Resident` is the bytes held. The image is
  itself a `WritableContainer`, its `Close` the same as `Release`. A mutex
  guards it. `diskimage/memory_test.go` covers zeros, writes across
  chunks, overwriting with zeros, sparseness, the argument errors, views,
  and release; `volume/memory_test.go` initializes a 20,000-block image,
  makes a directory, dismounts, mounts a new view, finds the directory,
  and checks that the image holds under a quarter of its size. ods2's
  tests pass, with `-race` for `diskimage`. The package documentation
  and README name the third kind of container. Committed in ods2 as
  `e3f96d4` (no attribution lines, per ods2's CLAUDE.md), with the build
  number at 1.0-31, then tagged v0.1.31 and pushed with the author's
  permission. govax's `go.mod` now pins v0.1.31, a step the plan gave
  subtask 3, taken here to check the tag: `GOWORK=off go build ./...` and
  `internal/rms`'s tests pass on it.
