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
(`%GOVAX-W-NOSYSVOL, ...`, with the reason) and starts as it does today,
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
  the manifest's, `%GOVAX-I-SYSKEPT, DUA0:[SYSTEM.SYSLIB]FTNRTL.EXE;3 was
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
`vax.default.volume.device`, `--system-device`, and `--mount`/`--mount-write`
(`--mount DUA=work.dsk`). The system volume is mounted first, so with the
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
`%GOVAX-I-SYSREADONLY, ...in use by another govax session, mounted
read-only`: no refresh, and no page file there (Decision 8). The lock
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
2. **The volume's contents** (`internal/sysvol`): the table of supplied
   files (directory, name, record format, source: `bootdata.FS`,
   `ftnrtl.Files` and `ftnrtl.Image`, `sysdef.Library`), the new
   built-in files (`startup.com`, `systartup_vms.com`, `buildftn.com`,
   `buildstarlet.com`), the manifest's format and its reading and writing.
   Plain Go over an interface to RMS, tested without a console.
3. **Creating and mounting it:** the `vax.default.system.*` keys
   (`validConfigs`, HELP CONFIG KEYS), `--system-file`, `--system-device`,
   `--system-initialize` (Decision 10), the unit-number rule for every
   device setting and `--mount` (Decision 6), creation with its
   directories and files, the host lock (Decision 7), Decision 1's
   fallback, DISMOUNT refused; `cmd/govax`'s `run` reordered (the sequence
   above) and given a test seam (a system-volume option, so tests choose
   a temporary file or none).
4. **The refresh** (Decision 3): new versions, kept files and their
   message, missing files, the manifest updated; tests for each case and
   for an interrupted refresh (a manifest that doesn't match what's
   there).
5. **Names, grammars, and STARTUP.COM from the volume:** whether RMS
   merges rooted directories (and making it, if small; otherwise plain
   names); `SYS$SYSDEVICE` and `SYS$SYSROOT`; the resolver's volume layer
   and the grammar fallback (Decision 4); STARTUP.COM run from the volume,
   checking that its data lines still reach the debugger after its VMINIT.
6. **ASM and LOAD on volume files** (Decision 9), `.INCLUDE` beside the
   including file; STARTUP.COM assembling `SYS$SOURCES:KERNEL.ASM`; the
   built-in STARTUP.COM's path with no volume. A test that boots from an
   edited KERNEL.ASM on the volume.
7. **The page file on the volume** (Decision 8), both paging modes; no page
   file when the volume is read-only; SHOW MEMORY/FILES naming it.
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
  answers.
