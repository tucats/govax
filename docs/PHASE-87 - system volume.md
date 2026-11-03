# Phase 87 — the system volume

**Status:** planned 2026-11-03; the plan reviewed and accepted by the
author the same day ("The review", after Part B). Needs nothing beyond
what Phases 1–86 left in place.

## Goal

govax always mounts a **system volume**: an ODS-2 container it creates
from the files built into it (`internal/bootdata`, `internal/ftnrtl`,
`internal/fortran/sysdef`), mounted writable before the startup
procedure runs, so that:

- the startup procedure is a file on the volume, `STARTUP.COM` (today's
  `vax.init`), which the user can change;
- the grammars and help files the console and the debugger read are
  there, under the user's control;
- `SYS$LIBRARY` and `SYS$SHARE` lead to govax's own library files
  (STARLET.MLB, FTNRTL.EXE, FORSYSDEF.TLB), which a program's MACRO,
  LINK, FORTRAN, and RUN find as they would on VMS, and which the user
  can copy, rebuild, and replace;
- FTNRTL's sources, the microkernel's, and STARLET's are there, with
  procedures that rebuild them;
- the page file has a home, `PAGEFILE.SYS` on the system volume.

The author's request (2026-11-03), in brief:

| Item | Requested |
|---|---|
| Configuration | `vax.default.system.file=./govax-system.dsk`, `vax.default.system.device=DUA0`, `vax.default.system.type=RD54` |
| Label | GOVASYS, read as GOVAXSYS (Question 3) |
| Created | if it doesn't exist; writable |
| Options | `--system-file FILE`, `--system-device DEV`, `--system-initialize` (re-create it even if it exists) |
| Page file | `[SYSTEM]PAGEFILE.SYS` on it by default |
| `[SYSTEM.SYSMGR]` | `STARTUP.COM` (renamed `vax.init`), `CONSOLE.DCL`, `DEBUG.DCL` |
| `[SYSTEM.SYSHLP]` | `CONSOLE.HLP`, `DEBUG.HLP` |
| `[SYSTEM.SYSLIB]` | SYS$SHARE's files: STARLET.MLB, FTNRTL.EXE, ...; STARTUP.COM defines SYS$LIBRARY and the rest to lead here |
| `[SYSTEM.SOURCES]` | `FTN*.MAR` and `BUILDFTN.COM`; `SSDEF.ASM`, `STARLET.MAR`, `STARLETDEF.MAR`, `KERNEL.ASM` |
| Unit numbers (added mid-planning) | a configured device with no unit (`DUA`) means the next free unit: `vax.default.volume.device=DUA` is DUA1 when the system volume is DUA0 |

## Part A — the survey

### A.1 What govax does at startup today

`cmd/govax/main.go`'s `run`, in order:

1. A `respath.Resolver` over `--path`'s directories with `bootdata.FS`
   as its fallback (Phase 15): a name is tried as given (relative to the
   host's current directory), then in each `--path` directory, then
   embedded.
2. `console.dcl`, `console.help`, `debug.dcl`, `debug.help` are read
   through the resolver and parsed. A grammar that's missing or bad ends
   govax (VAX_GRAMMAR); a missing help file is a warning.
3. `console.New`, the terminal, and `c.Init(minimumVAXMemory)`: a
   machine of 2,048 pages, with a System and a process 1, so DCL and RMS
   work before vax.init's INIT/VAX and VMINIT.
4. The dispatcher and the debugger are installed.
5. `--mount`/`--mount-write`, then `MountDefaultVolume`
   (`internal/dclcmd/defvolume.go`, the `vax.default.volume.*` keys):
   created if missing, mounted writable if it can be, SET DEFAULT to it.
   Failure is a warning; startup goes on.
6. `RunHostProcedure("vax.init")`: a host file, through the resolver,
   at command level 1. vax.init sizes memory from `vax.memory.*`
   (F$CONFIG), does INIT/VAX and VMINIT, `LOAD/ROM/NOERROR`,
   `LOAD/NVRAM/NOERROR`, `ASM "kernel.asm"` (which `.INCLUDE`s
   `ssdef.asm`), runs `EXE$INITIALIZE` under the debugger, and defines
   TTA0: and NLA0:.
7. `StartPageFile`: with paging on, the page file named by
   `vax.sysgen.paging.file` is installed. **There is no default name**:
   unset, there is no page file (`corevms/pagefile.go`).
8. `vax.startup.procedure`, if set and the file exists: a volume or host
   procedure, the user's own.
9. The one-shot command, or the prompt.

Every subcommand (`run`, `macro`, `fortran`, `link`, `library`,
`analyze`) goes through the same `run`, with its command as the
one-shot command.

### A.2 What already works before VMINIT

- **Mounts outlive the machine.** The `MountTable` and devices are the
  Machine's; `newRTL` (every INIT, VMINIT, ZERO) shares them with the
  new System (`machine.go`). The default volume is mounted at step 5,
  before vax.init, and stays mounted through its VMINIT. So the system
  volume can be mounted at the same point, and mounting it needs no VAX
  state at all: RMS and ods2 are Go.
- **Logical names outlive every INIT** (`NewMachine`: process 1's
  database is made once). Names defined before VMINIT are there after.
- **A procedure on a volume can be run** (`RunProcedure`, used by
  `vax.startup.procedure`); `openProcedure` reads the file through RMS
  and keeps its whole text, so the procedure doesn't hold the file open
  across its own VMINIT. Its process-permanent SYS$INPUT entry belongs
  to the System current when it started; vax.init already runs across
  its own VMINIT this way, so nothing new is needed, but subtask 5 must
  check a data line after VMINIT (the debugger's `go exe$initialize`)
  still reaches the debugger from a volume procedure.

So "mount before the machine is started" is not the hard part: govax
already does it for the default volume. The hard parts are elsewhere
(A.5).

### A.3 Who reads which file, and how

| File (embedded name) | Read by | How it's found today |
|---|---|---|
| `console.dcl`, `debug.dcl` | `cmd/govax` at step 2 | resolver (host, `--path`, embedded) |
| `console.help`, `debug.help` | `cmd/govax` at step 2 | resolver |
| `vax.init` | `RunHostProcedure` | resolver (`procedure.go`'s `openProcedure`, host branch) |
| `kernel.asm`, `ssdef.asm` | ASM (`machine/asm.go`) | resolver; `.INCLUDE` relative to the including file's host directory (`WithDir`) |
| `default.rom`, `default.nvram` | LOAD/ROM, LOAD/NVRAM (`machine/rom.go`) | resolver; `default.nvram` isn't embedded |
| STARLET.MLB | MACRO (`dclcmd/macro.go`) | `SYS$LIBRARY:STARLET.MLB` on a mounted volume, then `vax.library`'s host directory, then the embedded copy |
| FORSYSDEF.TLB | FORTRAN's `INCLUDE '(module)'` (`dclcmd/fortran.go`) | `FORT$LIBRARY`, `SYS$LIBRARY:FORSYSDEF.TLB`, then `sysdef` in Go |
| IMAGELIB.OLB, STARLET.OLB | LINK (`dclcmd/linksource.go`) | `SYS$LIBRARY` on a volume, `vax.library`; else govax's own tables. These are VMS's licensed files and are **not** candidates for the system volume. |
| FTNRTL.EXE | RUN (`rms.ImageLocator`), LINK (`linksource.go`) | a logical name `FTNRTL`, then `SYS$SHARE:FTNRTL.EXE` (RMS's parse), then the host, then the image built into govax (`Builtin`) |
| LIBRTL, MTHRTL | RUN, LINK | emulated (shims); a logical name of the image's name sends RUN to a file instead. Not files: nothing to put on the volume. |
| `starlet.mar`, `starletdef.mar` | `go generate ./internal/bootdata` only | not read at run time |
| `ftn_*.mar`, `ftnrtl.opt` | `go generate ./internal/ftnrtl` only | `ftnrtl.Files` (embedded) |

So MACRO, FORTRAN, LINK, and RUN already look in `SYS$LIBRARY` and
`SYS$SHARE` first: defining those names to the system volume is all it
takes for them to use its files. **No logical name `SYS$LIBRARY`,
`SYS$SHARE`, `SYS$SYSTEM`, or `SYS$SYSROOT` is defined today.** Only
host files reach ASM, LOAD, the grammars, the help files, and the
startup procedure.

### A.4 Other facts the plan rests on

- **Containers are sparse.** `diskimage.Create` sizes the file with
  `Truncate`; an RD54 is 311,200 blocks (159 MB) of address space and a
  few hundred KB of disk until written.
- **Nothing locks a container on the host.** Neither ods2 nor govax
  takes a host lock; two govax sessions mounting one container writable
  will each keep their own idea of its bitmap and headers, and corrupt
  it.
- **Concealed logical names exist** (`lnm/filespec.go`, `rms/logicals.go`);
  whether RMS merges a **rooted** directory (`SYS$SYSROOT` =
  `DUA0:[SYSTEM.]`, then `SYS$SYSROOT:[SYSLIB]`) is not established by
  this survey. Subtask 5 finds out first.
- **Tests.** `internal/...` tests build consoles through their own
  harnesses, never `cmd/govax`'s `run`, so they won't get a system
  volume unless they ask for one. `cmd/govax`'s tests call `run`
  directly (`main_test.go`, `milestone_test.go`, ...) and would create a
  container in the package's directory with the requested default.
- **The clean-room hook** (`.claude/hooks/cleanroom.sh`) refuses any
  tool call whose text names the STARLET macro library's file name, even
  govax's own copy. Procedures and Go code that name
  `SYS$LIBRARY:STARLET.MLB` are written with Edit/Write; shell commands
  in this phase avoid the name. This doesn't change the design; it's a
  working note.
- **FTNRTL already builds from DCL.** `testdata/ftn86/ftn86.com` has VMS's
  MACRO and LINK build it from the `.MAR` files and `ftnrtl.opt`, and
  `TestFTN86Procedure` rehearses that procedure on govax. BUILDFTN.COM is
  that procedure's first part, pointed at the system volume.
- **The request names `SSDEF.ASM` twice** in `[SYSTEM.SOURCES]`; once is meant.

### A.5 Where the real difficulties are

1. **Upgrades.** Once govax copies its files to a volume, the volume's
   copies win. A new govax with a new verb in `console.dcl`, a new FTNRTL
   entry (GSMATCH minor id raised), or a fixed `kernel.asm` would keep
   running the old volume's copies, and some of those mismatches are
   fatal (a grammar bound to handlers the binary no longer has, a
   program linked against the new FTNRTL refused with SHRIDMISMAT by the
   old one). The volume must be refreshed by every new govax without
   losing what the user changed on purpose. Decision 3.
2. **The grammars are code, not data.** `console.dcl` and `debug.dcl` bind
   verbs to Go handlers. A user's edited copy that doesn't parse, or names
   a syntax the binary lacks, would leave govax without a command line.
   Decision 4.
3. **The default place.** A system disk in the host's current directory
   means one per directory govax is started in, each set up and
   refreshed separately, and a test run from `cmd/govax` makes one there.
   Question 1.
4. **Two sessions on one volume.** With the default shared by every
   session started in one directory (or under one home directory),
   a second session is a real case, and writable mounts in both corrupt
   the volume (A.4). Decision 7.
5. **Device names.** The system volume on DUA0 collides with today's
   default volume, also DUA0, and with every configuration and test that
   mounts something there. The author's unit-number rule (Decision 6)
   answers most of it.

## Part B — decisions

Each decision was proposed for review; the questions after them were
the places where the plan departs from the request or where the
author's answer changes the work. The author accepted every
recommendation ("The review"), so the decisions stand as written, with
the answers folded in.

**Decision 0 — where it is, and running without one** (Questions 1 to
3). The default container is `~/.govax/govax-system.dsk`
(`vax.default.system.file`), labeled GOVAXSYS, on DUA0 as an RD54.
`--system-file none` (or the key's value `none`) starts govax with only
its built-in files, as Decision 1's fallback does.

**Decision 1 — the volume is a working copy; govax's built-in files stay
the fallback.** Everything on the volume comes from files built into
govax. If the volume can't be created or mounted (a read-only directory,
an unknown device type, a damaged container), govax says so
(`%MOUNT-W-NOSYSVOL, system volume FILE not mounted: reason`, beside the
default volume's NODEFAULT) and starts as it does today,
from the built-in files, with no system logical names. `--system-initialize`
is the cure for a damaged volume; this keeps govax usable until the user
runs it.

**Decision 2 — the layout.** As requested, with `[SYSTEM.SYSEXE]` added
(SYS$SYSTEM, where VMS keeps the page file):

| Directory | Logical name | Files |
|---|---|---|
| `[SYSTEM.SYSEXE]` | `SYS$SYSTEM` | `PAGEFILE.SYS` (made by the page file's code, not copied), `DEFAULT.ROM` |
| `[SYSTEM.SYSMGR]` | `SYS$MANAGER` | `STARTUP.COM`, `CONSOLE.DCL`, `DEBUG.DCL`; `SYSTARTUP_VMS.COM` (Decision 3), created once |
| `[SYSTEM.SYSHLP]` | `SYS$HELP` | `CONSOLE.HLP`, `DEBUG.HLP` |
| `[SYSTEM.SYSLIB]` | `SYS$LIBRARY`, `SYS$SHARE` | STARLET.MLB, `FTNRTL.EXE`, `FORSYSDEF.TLB` (`sysdef.Library`) |
| `[SYSTEM.SOURCES]` | `SYS$SOURCES` (govax's name) | `FTN_*.MAR`, `FTNRTL.OPT`, `BUILDFTN.COM`; `KERNEL.ASM`, `SSDEF.ASM`, `STARLET.MAR`, `STARLETDEF.MAR`, `BUILDSTARLET.COM` |
| `[SYSTEM]` | `SYS$SYSROOT` (rooted if RMS allows, A.4) | `GOVAX$FILES.DAT`, the manifest (Decision 3) |

Text files are variable-length records with CR carriage control, as
CREATE makes them; `FTNRTL.EXE` and the libraries fixed 512-byte records,
as LINK and LIBRARY write them. Which built-in file goes where, under
which name and record format, is one table in Go (`internal/sysvol`),
read by creation, refresh, and the tests.

**Decision 3 — refresh by content, with a manifest.** The manifest,
`[SYSTEM]GOVAX$FILES.DAT`, lists each file govax supplied and a SHA-256
of what it wrote. At every start, for each supplied file:

- the volume's file matches the manifest (the user hasn't changed it)
  and govax's built-in copy differs: govax writes its copy as a **new
  version** (the old one stays, so `PURGE` or a version number gets it
  back), and updates the manifest;
- the volume's file doesn't match the manifest (the user changed it, or
  rebuilt FTNRTL): it's left alone, and if govax's copy has changed since
  the manifest's, `%MOUNT-I-SYSKEPT, DUA0:[SYSTEM.SYSLIB]FTNRTL.EXE was
  changed, govax's new version not installed` says so, once per new
  built-in content;
- missing: written again.

Comparing contents rather than build numbers means a developer's
`go build` with edited bootdata files refreshes the volume too. Reading
every supplied file at each start costs a few hundred KB of reads.

To give users a place for changes that a refresh never touches,
`STARTUP.COM` ends with `$ IF F$SEARCH("SYS$MANAGER:SYSTARTUP_VMS.COM")
.NES. "" THEN @SYS$MANAGER:SYSTARTUP_VMS`, VMS's site-specific startup.
govax creates a commented `SYSTARTUP_VMS.COM` once and never refreshes it.
`vax.startup.procedure` stays, run after STARTUP.COM as today.

**Decision 4 — the grammars and help files on the volume, with the
built-in ones behind them.** The resolver gets a second fallback ahead of
the embedded files: the system volume's directories (an `fs.FS` over RMS
mapping `console.dcl` to `SYS$MANAGER:CONSOLE.DCL`, `console.help` to
`SYS$HELP:CONSOLE.HLP`, and so on). `--path` and a file in the current
directory still come first, as they do now, for development. A volume
grammar that doesn't parse, or whose binding fails, is a warning naming
the file, and govax uses its built-in grammar.

**Decision 5 — govax defines the names it needs to find the volume;
STARTUP.COM defines the rest** (the author, Question 5: in STARTUP.COM,
not by govax, wherever practical). Mounting the system volume defines, in
the system table at executive mode as VMS's boot does, `SYS$SYSDEVICE`
(the device, so STARTUP.COM doesn't depend on the unit) and
`SYS$SYSROOT`. govax opens `STARTUP.COM` and the grammars by device and
directory, so it needs nothing more. `STARTUP.COM` defines `SYS$SYSTEM`,
`SYS$MANAGER`, `SYS$HELP`, `SYS$LIBRARY`, `SYS$SHARE`, and `SYS$SOURCES`
in plain DCL (`DEFINE/SYSTEM/EXECUTIVE_MODE`), as the request asks, so
the user can see and change them.

**Decision 6 — unit numbers, and the work volume on the next one.** A
configured device with no unit number (`DUA`, `DUB`, `MUA`) means the next
unit of that controller not defined or mounted, from 0, settled when the
volume is mounted. It applies to `vax.default.system.device`,
`vax.default.volume.device`, `--system-device`, `--mount`/`--mount-write`
(`--mount DUA=work.dsk`), and the MOUNT command (`MOUNT DUA "work.dsk"`;
the author, after subtask 3). The system volume is mounted first, so with the
defaults it is DUA0, and **`vax.default.volume.device`'s default becomes
`DUA`**: DUA1 beside it. A configuration naming DUA0 for the work volume
gets a clear message (`%MOUNT-W-NODEFAULT, ...: DUA0: is the system
volume`) and the default volume isn't mounted. MANUAL.md and HELP CONFIG
KEYS say the default changed. DISMOUNT of the system volume is refused
(VMS's `%DISM-...`, text from its message table, chosen in subtask 3);
`DismountAll` at session end still flushes it.

**Decision 7 — one writer per volume.** Mounting the system volume
writable takes an exclusive advisory lock on the container
(`flock`/`LockFileEx` behind a small `internal/hostlock`; Unix first). A
second session finding it locked mounts it read-only with
`%MOUNT-I-SYSREADONLY, system volume FILE is in use by another govax
session; mounted read-only`: no refresh, and no page file there (Decision 8). The lock
goes with the process, so a crashed session leaves none behind. The same
lock could protect the work volume; this phase does it for the system
volume only, and logs the other as follow-up.

**Decision 8 — the page file's default.** With `vax.sysgen.paging.file`
unset and the system volume writable, the page file is
`SYS$SYSDEVICE:[SYSTEM.SYSEXE]PAGEFILE.SYS` (by device and directory, not
`SYS$SYSTEM`, so it doesn't depend on STARTUP.COM), installed after
STARTUP.COM as now, `vax.sysgen.paging.size` blocks (32,768, 16 MB, of an
RD54's 159). With no writable system volume, there is no page file, as
today. A configured name wins, as now.

When the system volume is read-only (a second session, Decision 7),
govax doesn't try to open a page file there at all: there is no page
file, nothing fails, and no message beyond SYSREADONLY's. SHOW MEMORY's
page file section says why (the author, 2026-11-03): the default page
file wasn't used because the system volume is read-only, in place of
today's "no page file" report (`PageFileReport` gets the reason).

**Decision 9 — ASM and LOAD read volume files.** `ASM` and
`LOAD/ROM`/`LOAD/NVRAM` take a VMS file name as well as a quoted host
path, as RUN and @ do (`Session.Locate`), and ASM's `.INCLUDE` looks
beside the including file on the same side, volume or host. STARTUP.COM
then assembles `SYS$SOURCES:KERNEL.ASM`, and an edited kernel is what
boots. With no system volume (Decision 1), STARTUP.COM's built-in copy
runs as a host procedure, sees no `SYS$SYSDEVICE`, and uses the built-in
names it uses today (`F$TRNLNM` picks the spec).

**Decision 10 — `--system-initialize` keeps the old volume.** It renames
the existing container to `FILE.old` (replacing an older `.old`) and
creates a new one. A rename costs nothing for a sparse file and saves the
user's SYSTARTUP_VMS.COM and rebuilt files from one mistaken run.

**Decision 11 — what `vax.init` becomes.** `internal/bootdata/files/vax.init`
is renamed `startup.com` and rewritten for the volume (Decisions 3, 5, 9).
A host `vax.init` in the current directory or on `--path` is no longer
run: the old name is gone, and a user's customizations belong in
SYSTARTUP_VMS.COM or `vax.startup.procedure`. MANUAL.md says so. (Tests
that pass a `vax.init` of their own through `--path` are found and moved
in subtask 9.)

### The new startup sequence

1. Configuration and options; the resolver.
2. `console.New`, the terminal, `c.Init(minimumVAXMemory)`.
3. **The system volume:** settle its device (Decision 6); `--system-initialize`'s
   rename; create it if missing (label, directories, every supplied file,
   the manifest); lock and mount (Decision 7); refresh (Decision 3);
   define `SYS$SYSDEVICE` and `SYS$SYSROOT` (Decision 5). Any failure:
   Decision 1.
4. The grammars and help files, through the resolver with the volume
   (Decision 4); the dispatcher and the debugger.
5. `--mount`s and the default volume, units settled after the system
   volume's.
6. `STARTUP.COM` from the volume at level 1 (or the built-in one, Decision
   1), ending with SYSTARTUP_VMS.COM.
7. The page file (Decision 8).
8. `vax.startup.procedure`; the one-shot command or the prompt.

Creation's informational messages (`%MOUNT-I-CREATED`, ...) are shown
when the console is verbose (no one-shot command), as the default
volume's are, after the banner.

### Questions for the review

1. **The default file's place.** The request says `./govax-system.dsk`,
   in the host's current directory. I recommend `~/.govax/govax-system.dsk`,
   beside the history files in govax's profile directory: one system
   volume per user, set up once, refreshed once per upgrade, found the
   same wherever govax is started, and never made by accident in a source
   tree (`cmd/govax`'s tests). Either way `--system-file` and
   `vax.default.system.file` choose another. Which?
2. **A way to run without one.** The request says "always". Should
   `--system-file none` (or `vax.default.system.file=none`) start govax
   with only the built-in files, for read-only installs and CI? I
   recommend yes; Decision 1's fallback is the same code.
3. **The label.** The request says GOVASYS; I read it as GOVAXSYS (8
   characters, within ODS-2's 12). Which?
4. **The refresh policy** (Decision 3): new versions of files the user
   hasn't changed, a message for ones they have, SYSTARTUP_VMS.COM as
   their place. Acceptable, or should a changed file be replaced too (with
   the user's kept as a lower version)?
5. **The logical names** (Decision 5): govax defines two, STARTUP.COM the
   rest. Or should govax define them all, as VMS's boot does, leaving
   STARTUP.COM to the machine's set-up?
6. **A second session** (Decision 7): read-only, as proposed, or refused?
7. **`[SYSTEM.SYSEXE]`** (Decision 2) for PAGEFILE.SYS and DEFAULT.ROM,
   rather than `[SYSTEM]` as requested?
8. **The work volume's default device** becoming `DUA` (Decision 6), so
   existing configurations that leave it unset move from DUA0 to DUA1.

### The review

The author reviewed the plan on 2026-11-03 and accepted every
recommendation:

1. `~/.govax/govax-system.dsk` (Decision 0).
2. Yes: `--system-file none` (Decision 0).
3. GOVAXSYS.
4. The refresh as proposed (Decision 3).
5. The logical names in STARTUP.COM, not set up by govax outside it,
   wherever practical: govax defines only `SYS$SYSDEVICE` and
   `SYS$SYSROOT`, which STARTUP.COM's own names are built on (Decision 5).
6. A second session mounts the volume read-only (Decision 7).
7. `[SYSTEM.SYSEXE]` for PAGEFILE.SYS and DEFAULT.ROM (Decision 2).
8. The work volume's default device `DUA` (Decision 6).

## Subtasks

1. **The survey and plan** (this document's Parts A and B), and the
   author's review (done 2026-11-03).
2. (done 2026-11-03) **The volume's contents** (`internal/sysvol`): the table of supplied
   files (directory, name, record format, source: `bootdata.FS`,
   `ftnrtl.Files` and `ftnrtl.Image`, `sysdef.Library`), the new
   built-in files (`startup.com`, `systartup_vms.com`, `buildftn.com`,
   `buildstarlet.com`), the manifest's format and its reading and writing.
   Plain Go over an interface to RMS, tested without a console.
3. (done 2026-11-03) **Creating and mounting it:** the `vax.default.system.*` keys
   (`validConfigs`, HELP CONFIG KEYS), `--system-file`, `--system-device`,
   `--system-initialize` (Decision 10), the unit-number rule for every
   device setting and `--mount` (Decision 6), creation with its
   directories and files, the host lock (Decision 7), Decision 1's
   fallback, DISMOUNT refused; `cmd/govax`'s `run` reordered (the sequence
   above) and given a test seam (a system-volume option, so tests choose
   a temporary file or none).
4. (done 2026-11-03) **The refresh** (Decision 3): new versions, kept files and their
   message, missing files, the manifest updated; tests for each case and
   for an interrupted refresh (a manifest that doesn't match what's
   there).
5. (done 2026-11-03) **Names, grammars, and STARTUP.COM from the volume:** whether RMS
   merges rooted directories (and making it, if small; otherwise plain
   names); `SYS$SYSDEVICE` and `SYS$SYSROOT`; the resolver's volume layer
   and the grammar fallback (Decision 4); STARTUP.COM run from the volume,
   checking that its data lines still reach the debugger after its VMINIT.
6. (done 2026-11-03) **ASM and LOAD on volume files** (Decision 9), `.INCLUDE` beside the
   including file; STARTUP.COM assembling `SYS$SOURCES:KERNEL.ASM`; the
   built-in STARTUP.COM's path with no volume. A test that boots from an
   edited KERNEL.ASM on the volume.
7. (done 2026-11-03) **The page file on the volume** (Decision 8), both paging modes; no page
   file, and no attempt to open one, when the volume is read-only, with
   SHOW MEMORY saying the default page file wasn't used (read-only); SHOW
   MEMORY/FILES naming it.
8. **The libraries:** MACRO, FORTRAN, LINK, and RUN using SYS$LIBRARY's
   and SYS$SHARE's files from STARTUP.COM's names; BUILDFTN.COM building
   and installing FTNRTL.EXE (a new version in SYS$SHARE) from the sources,
   and a program linked and run against the rebuilt one (`TestFTNRTL`'s
   program); BUILDSTARLET.COM building STARLET.MLB with LIBRARY, compared
   with the built-in library (identical but for dates, or logged as
   unconfirmed).
9. **Tests and the one-shot subcommands:** every `cmd/govax` test on a
   temporary system volume or none; `govax run`, `macro`, `fortran`,
   `link`, `library`, `analyze` with the system volume; the time a first
   start takes to create the volume, measured and recorded; the tests that
   used a host `vax.init` (Decision 11).
10. **The close-out:** MANUAL.md (the command line, configuration, a
    section on the system volume: its layout, STARTUP.COM and
    SYSTARTUP_VMS.COM, the refresh, rebuilding FTNRTL, recovery with
    `--system-initialize`), HELP (CONFIG KEYS, the options, STARTUP),
    CLAUDE.md, PLAN.md; follow-up list (the work volume's lock, Windows
    locking if not done).

## Progress log

- **2026-11-03** — Phase planned: the survey (Part A) and proposed
  decisions (Part B), with eight questions for the author. The author
  added the unit-number rule while it was being written (Decision 6).
- **2026-11-03** — Subtask 1 done: the author accepted every
  recommendation ("The review"); Decision 0 records the first three
  answers. The author added (the same day) that a read-only system volume
  means no page file and no attempt at one, with SHOW MEMORY saying why
  (Decision 8, subtask 7).
- **2026-11-03** — Subtask 2 done: `internal/sysvol`.
  - `files.go` is the layout: `Directories` (each after its parent), and
    `Files()`, one `File` per file (directory, name, `Format`, `Kind`, and a
    reader of its built-in source), with `Spec` and `Lookup`. FTNRTL's
    modules are read from `ftnrtl.Files`' directory, so a new module goes on
    the volume with no change here. `Label`, `DefaultDevice`, and
    `DefaultType` are Decision 0's.
  - `contents.go`: one canonical form per format, which is what's written,
    compared, and summed. A `Text` file is its lines, each ended by a line
    feed, CRLF made LF. A `Binary` file is its bytes zero-padded to whole
    blocks: DEFAULT.ROM is 127,944 bytes, and the padding is harmless
    because LOAD/ROM stops at a zero block count. `Records` and `Join` go
    between canonical contents and records. `Sum` is SHA-256.
  - `manifest.go`: `Manifest`, `[SYSTEM]GOVAX$FILES.DAT`, which starts with
    comment records, then has one record per supplied file, its spec and
    digest, sorted, so the same manifest is always the same file. A damaged
    record is an error naming it, never a file left out.
  - `populate.go`: the `Volume` interface (`CreateDirectory`, `WriteFile` a
    new version, `ReadFile` the highest), `Populate` (directories, files, a
    seeded file only if missing, the manifest of supplied files), and
    `ReadManifest`/`WriteManifest`.
  - FORSYSDEF.TLB is built with a fixed time (`definitionTime`), as
    STARLET.MLB is, so its digest doesn't change from start to start
    (`TestContentsRepeat`).
  - New built-in files in `internal/bootdata/files`: `startup.com` (vax.init
    plus STARTUP.COM's definitions of SYS$SYSTEM, SYS$MANAGER, SYS$HELP,
    SYS$LIBRARY, SYS$SHARE, and SYS$SOURCES under SYS$SYSDEVICE; the kernel
    and ROM from the volume, or the built-in names with no SYS$SYSDEVICE;
    SYSTARTUP_VMS.COM last), `systartup_vms.com`, `buildftn.com`, and
    `buildstarlet.com`. `vax.init` is still the startup procedure until
    subtask 5, and `startup.com` isn't run yet. Subtasks 5, 6, and 8 run
    and test the procedures. `TestBuildProcedureModules` holds
    BUILDFTN.COM's MACRO lines to FTNRTL.OPT's objects and to the modules
    on the volume.
- **2026-11-03** — Subtask 3 done: the system volume made and mounted.
  - `internal/dclcmd/sysvolume.go`: `MountSystemVolume(SystemVolume)` returns a
    `SystemVolumeResult` (`Mounted`, `Device`, `Path`, `Writable`, `Shared`,
    `Created`).
    - **Defaults:** `DefaultSystemFile` (`~/.govax/govax-system.dsk`), and
      `NoSystemVolume` (`none`, in any case), which mounts nothing and
      says nothing.
    - **Finding or making the container:** `openSystemContainer` finds it
      or makes it (`InitializeContainer`, label GOVAXSYS), taking the host
      lock as soon as the file exists. A new container is locked right
      after it's made, so two sessions starting at once still end with one
      writer.
    - **`--system-initialize`:** renames the old container to `FILE.old`,
      unless another session holds it; then it is left alone and mounted
      read-only.
    - **Mounting:** for writing, or read-only when shared (SYSREADONLY) or
      when the host file can only be read (WRITELOCK).
    - **Refusals:** a label other than GOVAXSYS, a device already mounted,
      or an unknown type is `%MOUNT-W-NOSYSVOL, system volume FILE not
      mounted: cause`, and startup goes on.
    - **A new, writable volume** is filled by `sysvol.Populate` through
      `systemFiles`, the `sysvol.Volume` over RMS. Text files are written
      as `TextRecords`, binary files as `ImageBlocks` (FIX 512, LINK's
      attributes). A failure there is `%MOUNT-W-SYSFILES`, with the volume
      left mounted; subtask 4's refresh writes the missing files.
  - **New messages** (`vmserrors/codes_mount.go`): NOSYSVOL, SYSREADONLY,
    SYSINIT, and SYSFILES, all MOUNT facility like the default volume's.
    The plan's `%GOVAX-` names are corrected above.
  - **`internal/hostlock`:** `TryLock`, flock(2) on darwin, linux, and the
    BSDs; elsewhere it locks nothing (`lock_other.go`). Windows's
    LockFileEx is follow-up work.
  - **`rms.MountTable`:** `SetSystem`/`System`/`IsSystem` record the system
    device, and `Hold` keeps a closer, the lock, which is closed after the
    volume at dismount. Dismounting forgets the system device.
  - **DISMOUNT** of the system volume shows `%DISM-W-CANNOTDMT, DUA0:
    cannot be dismounted` and `%DISM-W-SYSDISK, device is the system
    volume`. The second line's wording is govax's: VMS's is unconfirmed.
  - **`internal/dclcmd/units.go`:** `ResolveUnit` takes three letters
    alone (`DUA`, `dua:`, `_DUA:`) as a controller and returns its lowest
    unit with nothing mounted on it. Anything else comes back as given,
    and so does a three-letter logical name, unless written with `_`.
    - It applies to the system volume's device, the default volume's,
      and `--mount`'s.
    - `DefaultVolumeDevice` is now `DUA`. A default volume configured on
      the system volume's device is NODEFAULT's "is the system volume".
  - **`cmd/govax`:**
    - `--system-file`, `--system-device`, and `--system-initialize` go in
      `systemOptions`. `systemVolume()` takes each option over its
      `vax.default.system.*` setting, and the settings are in
      `validConfigs`.
    - `run` mounts the system volume before the `--mount` options and the
      default volume. The grammars are still read first; subtask 5 moves
      them after the mount, when the resolver reads the volume.
    - `TestMain` gives every test in the package `--system-file none`
      unless a test sets its own (`withSystemVolume`), so no test touches
      `~/.govax`.
  - **HELP CONFIG KEYS** has "The system volume" and the default volume's
    new device. HELP's and MANUAL.md's `--mount DUA0=...` examples are now
    DUA1, since DUA0 is the system volume. The rest of MANUAL.md is the
    close-out's.
  - **Time:** making and filling an RD54 takes about 150 ms in a whole
    start (`TestRun_systemVolume` logs it), and an RD51 about 140 ms.
    The containers are sparse.
  - **Tests:** `internal/console/sysvolume_test.go` (made, reused, shared,
    the lock let go, initialized, not used, DISMOUNT refused,
    `ResolveUnit`, the default volume beside it) and
    `cmd/govax/sysvolume_test.go` (the options end to end, `--mount DUA`).
- **2026-11-03** — The author asked (after subtask 3) for the rule in the
  MOUNT command too: `MountCommand` resolves its device with `ResolveUnit`,
  so `MOUNT DUA "foo.dsk"` mounts on the first DUA unit with nothing on it,
  and `%MOUNT-I-MOUNTED` names the unit (`TestMountCommandNextUnit`).
  `Context.Mount` itself takes the device as given; its other callers
  resolve first. HELP MOUNT and MANUAL.md's MOUNT say so.
- **2026-11-03** — Subtask 4 done: the refresh.
  - `sysvol.Refresh` (`populate.go`) runs at every start on a system
    volume mounted for writing; a read-only one (Decision 7) is used as it
    is. It makes any missing directory, reads the manifest, and takes each
    file of `Files()` (`refreshFile`):
    - **missing:** written (`Written`);
    - **already govax's copy:** nothing written (`Current`); a manifest
      digest that differs is corrected;
    - **as the manifest says govax wrote it:** govax's newer copy written as
      a new version, the old one kept below it (`Updated`);
    - **anything else:** the user's (`Kept`), never written. `Notify` is
      set when govax's copy differs from the manifest's digest and the user
      hasn't been told about this copy yet.
  - **The manifest's new notes.** `KEPT=digest` records the copy the user
    was told about, so each new copy is reported once; a file that was
    never govax's (one there before the manifest) has a `KEPT=` record with
    no digest. `SEEDED` marks a seeded file govax has made or found, so
    SYSTARTUP_VMS.COM is made once and one the user deletes stays deleted
    (Decision 3's "created once"). A digest set again drops the `KEPT=` note:
    a user who goes back to govax's copy gets the next one.
  - **What's written, and when.** The manifest is written last, and only
    when it changed, so a start with nothing to do writes no version of
    anything. Records for files govax no longer supplies are kept, so a
    later govax that supplies one again still knows whether it was changed.
    `Populate` is now `Refresh` of an empty volume.
  - **Failures.** A write that fails ends the refresh with the file's name
    and the manifest unwritten. That does no harm: the files written before
    it hold govax's copies, which the next refresh finds current and records
    (the interrupted refresh). A damaged manifest stops the refresh before
    anything is written, so no file whose owner can't be told is replaced.
    The case a refresh can't recover from is a file left half written by a
    crash during the write: it matches neither digest and is kept as the
    user's, with SYSKEPT. `--system-initialize` or a DELETE of the version
    is the cure.
  - **Messages** (`vmserrors/codes_mount.go`, `dclcmd`'s
    `refreshSystemVolume`):
    - `%MOUNT-I-SYSKEPT, DUA0:[SYSTEM.SYSMGR]STARTUP.COM was changed,
      govax's new version not installed`, shown even when the console isn't
      verbose, since it comes once per copy;
    - `%MOUNT-I-SYSUPDATED, 3 files on system volume DUA0: updated by this
      govax`, verbose only, and not on a new volume, where CREATED says
      enough;
    - SYSFILES is now `system volume DUA0: could not be brought up to
      date`, its cause naming the file or the manifest's record.
  - **Time:** a start with nothing to refresh mounts and refreshes an RD51
    in about 8 ms (`TestSystemVolumeReused` logs it).
  - **Tests:**
    - `internal/sysvol`, with stand-in files whose contents a test changes
      as a later govax would (`refreshFiles`): `TestRefresh` (every case in
      turn on one volume), `TestRefreshInterrupted`, `TestRefreshNoManifest`,
      `TestRefreshDamagedManifest`, `TestRefreshBuiltIn`,
      `TestManifestNotes`;
    - `internal/console/sysrefresh_test.go`, on a real volume made to look
      as an older govax and a user left it, and a read-only and a damaged
      case.
- **2026-11-03** — Subtask 5 done: names, grammars, and STARTUP.COM from the
  volume.
  - **Rooted directories:** RMS has none. A logical name's equivalence is
    pasted into the specification as text (`lnm.TranslateFileSpec`), so
    `DUA0:[SYSTEM.]` followed by `[SYSLIB]` is `DUA0:[SYSTEM.][SYSLIB]`, not
    `[SYSTEM.SYSLIB]`. Doing it as VMS does would mean placing the root
    after the default directory is applied, and changing how expanded names
    are shown. That isn't small, so, as the plan allowed, the names are
    plain ones; rooted directories are follow-up work.
  - **The names govax defines** (`dclcmd/sysnames.go`, at mount, in the
    system table at executive mode, both terminal): `SYS$SYSDEVICE` is
    `_DUA0:`, and `SYS$SYSROOT` is `_DUA0:[SYSTEM]`, so
    `SYS$SYSROOT:GOVAX$FILES.DAT` works but `SYS$SYSROOT:[SYSLIB]` doesn't.
    VMS's attributes for these names (concealed or not) are unconfirmed;
    govax's aren't concealed, so expanded names show the device.
  - **STARTUP.COM:** bootdata's `vax.init` is gone (Decision 11).
    `Console.RunStartup` (`console/startup.go`) runs the volume's STARTUP.COM
    by device and directory, never a host file, or bootdata's `startup.com`
    through the resolver when there is no volume (or it has no STARTUP.COM),
    so `--path` and the current directory still override it then, as they did
    `vax.init`. The volume's procedure boots as `vax.init` did: its data
    lines after VMINIT (`go exe$initialize`, `exit`) reach the debugger
    (`TestStartupFromSystemVolume`: the kernel placed, the debugger exited,
    TTA0: defined, SYSTARTUP_VMS.COM run last). Until subtask 6 lets ASM and
    LOAD read volume files, STARTUP.COM assembles the built-in `kernel.asm`
    and loads no ROM file in both branches.
  - **The resolver's volume layer:** `respath.Resolver.Volume`, after the
    `--path` directories and before the embedded files, kept by `WithDir`.
    `OpenFrom`/`ReadFileFrom` say where a file came from (`Host`, `Volume`,
    `Embedded`); a failure reading the volume, other than a missing file, is
    a `VolumeError`; `ReadEmbedded` reads govax's own copy.
    `dclcmd.SystemFS` is the `fs.FS` over RMS. It covers only
    `sysvol.Resolved`'s four names (`console.dcl`, `debug.dcl`,
    `console.help`, `debug.help`). ASM's and LOAD's files are subtask 6's,
    by their VMS names.
  - **The grammar fallback** (`cmd/govax/grammarfiles.go`):
    - a volume grammar that can't be read or doesn't parse is
      `%VAX-W-SYSGRAMMAR, DUA0:[SYSTEM.SYSMGR]CONSOLE.DCL on the system
      volume not used; govax's own instead: cause`, and govax uses its own;
    - so is one that, once bound, lacks a syntax this govax binds
      (`dcl.Grammar.Missing`, "it has no EXIT"). Binding never fails in
      `dcl`, so this is what "whose binding fails" (Decision 4) became. Both
      built-in grammars lack none, so it never fires on govax's own copies;
    - a volume help file that can't be read falls back the same way;
    - a host grammar (current directory or `--path`) that fails is still
      VAX_GRAMMAR's error, as before, since a developer gave it on purpose.
  - **`run`'s order:** the system volume is mounted before the grammars are
    read, then the console's dispatcher and the debugger are installed,
    then the `--mount`s and the default volume (the new startup sequence's
    steps 3 to 5).
  - **Side effects to know about.** STARTUP.COM now defines `SYS$LIBRARY`
    and `SYS$SHARE`, so with a system volume MACRO, FORTRAN, LINK, and RUN
    already find the volume's macro library, FORSYSDEF.TLB, and FTNRTL.EXE
    (the same bytes as the built-in ones). Subtask 8 tests that. LINK finds
    no IMAGELIB.OLB there and goes on to `vax.library` and govax's tables, as
    before. IMAGELIB.OLB is VMS's licensed file, never govax's, and LINK gets
    its information from tables, so nothing is added to the volume for it
    (the author asked, 2026-11-03).
  - **Left for later:** MANUAL.md's mentions of `vax.init` (the
    close-out's), and test comments that still say `vax.init` (subtask 9).
    The tests' `RunHostProcedure("vax.init")` calls now name `startup.com`,
    and `cmd/govax`'s `--path` override tests test `startup.com`. HELP's
    mentions are updated, and HELP CONFIG KEYS describes STARTUP.COM,
    SYSTARTUP_VMS.COM, the grammars, and the refresh.
  - **Tests:**
    - `respath`: the volume's place, `VolumeError`;
    - `sysvol`: `TestResolvedFiles`;
    - `console`: `TestStartupFromSystemVolume`;
    - `cmd/govax/startup_test.go`: a host `startup.com` not run with a
      volume, the names, an edited STARTUP.COM run and booting, and a
      grammar that doesn't parse, one without EXIT, and a good edited one,
      all put on the volume by COPY.
- **2026-11-03** — Subtask 6 done: ASM and LOAD on volume files.
  - **Where a file is** (`machine/consolefile.go`'s `readConsoleFile`, for
    ASM, LOAD/ROM, and LOAD/NVRAM, in the console and the debugger): by
    `rms.Session.Locate`'s rules, as RUN and @ place theirs. A VMS
    specification is a volume file, a host path a host file read through
    the resolver as before, and a bare name a file in the default directory
    when SET DEFAULT is on a volume.
  - **One rule of the console's own:** a bare name that isn't in the
    default volume directory is then looked for through the resolver (the
    host's current directory, `--path`, govax's built-in files). RUN and @
    don't do this, their bare names being the user's files; ASM's and
    LOAD's are usually govax's, and without it STARTUP.COM's `ASM
    "kernel.asm"` would fail with no system volume but a work volume as the
    default. `rms.BareName` says which names are bare.
  - **`.INCLUDE`** (`includeReader`): a bare name beside the including file,
    on its side: the same device and directory of its volume
    (`LocateRelated`), or its host directory and then the resolver, as
    before; a bare name not beside a volume file then goes to the resolver
    too. An explicit name follows Locate's rules. Interactive ASM, with no
    file, sends bare names to the resolver and VMS specifications to their
    volumes.
  - **Messages:** a volume file that isn't there is `%SYSTEM-W-NOSUCHFILE`
    with its expanded name, and a device that isn't mounted DEVNOTMOUNT, as
    DCL's commands say (`volumeFileFailure`, the same mapping as
    `dclcmd.FileFailure`, which the machine doesn't import). A host file's
    message is the resolver's, unchanged. With `/NOERROR`, LOAD loads
    nothing for either.
  - **The ROM** on the volume has fixed 512-byte records, so its last block
    is padded with zeros; LOAD/ROM stops at the image's zero count before
    them, and the bytes loaded are the built-in ROM's
    (`TestLoadVolumeFiles`). SHOW ROM names the file it came from in full,
    `DUA0:[SYSTEM.SYSEXE]DEFAULT.ROM;1`; a host file is still named as
    given.
  - **STARTUP.COM** assembles `SYS$SOURCES:KERNEL.ASM` and loads
    `SYS$SYSTEM:DEFAULT.ROM` with a system volume, and govax's built-in
    `kernel.asm` and `default.rom` without one (the built-in procedure run
    as a host file, as before). The kernel's `.INCLUDE "ssdef.asm"` finds
    `SYS$SOURCES:SSDEF.ASM` beside it. Existing volumes get the new
    STARTUP.COM from subtask 4's refresh, unless the user changed theirs.
  - **Left as it was:** SAVE/ROM and SAVE/NVRAM still write host files
    only (Decision 9 named only LOAD); a SAVE to a volume is follow-up work.
    MANUAL.md's startup section, which still describes `vax.init`, is the
    close-out's; its section on the interactive assembler now says how ASM
    and LOAD find files, and so do HELP ASM (both grammars) and HELP LOAD.
  - **Tests:**
    - `internal/console/consolefile_test.go`: ASM of a volume file with an
      `.INCLUDE` found only beside it (`TestASMVolumeFile`), bare names
      from the default volume directory and from the built-in files
      (`TestASMBareName`), a missing volume file and an unmounted device
      (`TestASMVolumeFileMissing`), LOAD/ROM and LOAD/NVRAM of volume files
      (`TestLoadVolumeFiles`), and STARTUP.COM booting an edited KERNEL.ASM
      whose `.INCLUDE` names a file only on the volume
      (`TestBootFromEditedKernel`);
    - `cmd/govax/startup_test.go`'s `TestRun_systemVolumeKernel`: the ROM
      with and without a system volume, and a KERNEL.ASM edited by COPY
      booting.
- **2026-11-03** — Subtask 7 done: the page file on the system volume.
  - **The default** (`corevms/pagefile.go`'s `pageFileName`): with no
    `vax.sysgen.paging.file`, the page file is `_DUA0:[SYSTEM.SYSEXE]PAGEFILE.SYS`
    on the system volume (`sysvol.PageFile`, the layout's one place for it),
    named by the device's own name with an underscore, so neither STARTUP.COM's
    names nor a user's logical name can move it. It's decided when the page
    file is opened, from the mount table (`System`, `Writable`), so nothing
    new is passed from cmd/govax: the system volume is mounted before
    STARTUP.COM, and `StartPageFile` installs the file after it, as before.
    Each new System (VMINIT, INIT, ZERO) installs the same file again. A
    configured name wins.
  - **Paging off:** no page file and no PAGEFILE.SYS, as before.
  - **Read-only** (another session holds the volume, Decision 7): no page
    file, no attempt to open one (neither at startup nor at the pager's or
    `$GETSYI`'s need), and no message. `PageFileReport.Unused` says why, and
    SHOW MEMORY's page file section shows it under "Page file: none":
    `DUA0:[SYSTEM.SYSEXE]PAGEFILE.SYS not used: the system volume is
    read-only`; so does `--stats`. SHOW MEMORY/FILES lists no files, and
    `F$GETSYI("PAGEFILE_PAGE")` is 0. With no system volume at all
    (`--system-file none`) the report is a plain "none", as before.
  - **Names:** SHOW MEMORY shows `DUA0:[SYSTEM.SYSEXE]PAGEFILE.SYS;1`, and
    SHOW MEMORY/FILES `DISK$GOVAXSYS:[SYSTEM.SYSEXE]PAGEFILE.SYS`, as VMS's
    shows its volume's logical name.
  - **Size and time:** on the default RD54, a first start with paging on
    makes the volume and its 32,768-block page file in about 154 ms
    (`TestRun_systemPageFile` logs it). The page file's blocks are
    allocated, not written, so the 159 MB container took 2.9 MB of the
    host's disk (measured once while writing the test, on macOS's APFS;
    the test doesn't check it, the measure being Unix-only). The test volumes,
    RD51s, can't hold the default size, so the console's tests use 500
    blocks.
  - **Docs:** HELP CONFIG KEYS' `vax.sysgen.paging.file` and MANUAL.md's
    configuration table and 13.5 give the new default; their examples name
    DUA1, DUA0 being the system volume.
  - **Tests:** `internal/console/syspagefile_test.go` (STARTUP.COM run from
    a system volume, then the page file installed: made at the first start
    and found again by the next session, SHOW MEMORY and /FILES; paging off;
    read-only; a configured file instead), and `cmd/govax`'s
    `TestRun_systemPageFile` (the default RD54 and size, `--stats` and SHOW
    MEMORY/FILES).
