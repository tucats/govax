# Phase 97 — host file access

**Status:** planned 2026-11-16 (subtask 1, the survey and this plan),
awaiting the author's review. This document is the development plan; as
the work goes on it becomes the implementation record: the progress log,
and at the end "Where things stand".

## Goal

A site can stop unprivileged users and their programs from reaching the
host's files through govax. A new configuration key,
**`vax.console.host.sysprv`** (boolean), turns the rule on:

- **Off** (the key missing or false, the default): govax behaves exactly
  as it always has. No check is made and nothing is traced.
- **On**: every access to a host file, whether the name is a host path by
  its syntax (`/tmp/x.txt`, `C:x`), a bare name taken as a host file
  because the default directory is on the host, a logical name that
  translates to one, or a name with govax's `/HOST` qualifier, is allowed
  only to a process whose current privileges include **SYSPRV**. Any
  other process is refused, as a volume file's protection refuses it
  (Decision 5).

The session below is what the plan expects, as a user without SYSPRV
with the key on (the messages are those each command already gives for
a protection refusal, Phase 93):

    $ TYPE /tmp/notes.txt
    %TYPE-W-OPENIN, error opening /tmp/notes.txt as input
    -RMS-E-PRV, insufficient privilege or file protection violation
    $ COPY DUA1:[WORK]A.TXT a.txt/HOST
    %COPY-E-OPENOUT, error opening a.txt as output
    -RMS-E-PRV, insufficient privilege or file protection violation

and as SYSTEM, which holds SYSPRV, nothing changes.

This is govax's own rule. VMS has no host, so there is nothing to probe;
SYSPRV is chosen because it is VMS's privilege for "access to objects as
the system", the one a site would expect to guard files outside the
system's own protection.

## Part A — the survey (2026-11-16)

### A.1 How a name becomes a host file

`rms.Session.Locate` (`internal/rms/location.go`) is the one place a
typed file name is placed on the host or on a mounted volume, by four
rules: an explicit `/HOST`; VMS syntax (a bracketed directory, a device
or logical name of two letters or more) on a volume; host syntax (`/`,
`\`, a one-letter drive) on the host; and a bare name on the default
volume when SET DEFAULT is on one, else on the host. Before them, a name
with no punctuation that is a logical name stands for its translation
(`locateLogical`). The result is a `FileLocation` with `Host` set.

Each process has its own `rms.Session` (DCL's `Shell.ContainerSession`,
`corevms.Environment.Session`), and `Environment.UseSession` gives it
the process's `Credentials`, so the session already knows whose it is
(Phase 93). Its `HostFallback` reads a bare host name not in the host's
current directory through the console's search-path resolver
(`respath.Resolver`: `--path` directories on the host, the system
volume, then govax's embedded files; `ReadFileFrom` says which).

### A.2 Who calls Locate

55 calls outside `location.go`. Most open the file they place; a few only
ask which side a name is on and then do something else:

| Area | Callers |
| --- | --- |
| DCL utilities (`internal/dclcmd`) | TYPE, COPY (source and destination), DUMP, DIRECTORY (`/OUTPUT`), CREATE (file, `/FDL`), CONVERT (and its exceptions file), BACKUP (save, restore, list: input, output, save set), MACRO, FORTRAN (sources and INCLUDE logicals), LINK (objects, options), LIBRARY (library, inputs), LNK$LIBRARY's user libraries, ANALYZE, HELP (`/LIBRARY`, `/OUTPUT`), INSTALL, `filenames.go`'s output names |
| DCL itself (`internal/dclshell`) | `@file`, the procedure's `/OUTPUT`, OPEN, the lexical functions (F$SEARCH, F$FILE_ATTRIBUTES, F$PARSE), RUN in a subprocess (one call only asks the side, `subcli.go:1006`), DBG$INPUT, AUTHORIZE's image name, SPAWN/HOST's `/INPUT` and `/OUTPUT` |
| The machine | RUN's image (`image.go`, `imagesource.go`, with `/HOST` as `runHost`), ASM and LOAD (`consolefile.go`), the debugger's source lines (`source.go`) |
| Programs | LBR$OPEN (`lbrrtl/control.go`), a created process's SYS$INPUT, SYS$OUTPUT, and SYS$ERROR (`corevms/ppfstart.go`, `outfile.go`, `cliprocess.go`) |
| RMS | EDIT (`rms/edit.go`), CONVERT, ANALYZE/RMS_FILE, F$FILE_ATTRIBUTES, SET and SHOW SECURITY (`secobject.go`), the page file (`pagefile.go`, from configuration) |

### A.3 Host access that doesn't go through Locate

| Path | Where | What it touches |
| --- | --- | --- |
| EXE$OPEN | `corevms/file.go`, XFC shim 4 | a host file a VAX program names, opened with `os.OpenFile` |
| SAVE/ROM, SAVE/NVRAM | `machine/rom.go` | `os.Create` of a host path |
| LOAD/ROM, LOAD/NVRAM | `machine/rom.go` through `readConsoleFile` | Locate, then the resolver |
| The debugger's SET LOG, SET OUTPUT/LOG | `debugger/dbglog.go`, `Console.CreateDebuggerLog` | the log file |
| Shareable images | `rms/shareable.go`'s `ImageLocator`, step 3 | SET SHARE_PREFIX and `vax.library` on the host |
| The resolver's host directories | `HostFallback`, `consolefile.go` | `--path` directories |
| SPAWN/HOST | `dclshell/spawnhost.go` | runs a host program with `exec` |
| MOUNT, INITIALIZE/CONTAINER, DEFINE/DEVICE | `dclcmd/mount.go`, `init_container.go`, `device.go` | a container file on the host (MOUNT already needs SYSNAM, INITIALIZE of an existing one VOLPRO) |
| EDIT | `dclcmd/edit.go`, `rms/edit.go` | a host editor on a scratch copy in `vax.editor.scratch` |

### A.4 What govax reaches on the host for itself

These are named by configuration or the command line, not by a user's
file name, and are read or written for the system: the authorization
file (`vax.authorization.file`), the system volume and `--mount`
containers, `vax.default.volume.file`, `vax.library` for STARLET.MLB,
IMAGELIB.OLB, and STARLET.OLB, a configured page file
(`vax.sysgen.paging.file`), host locks, grammars, profiles
(`--cpu-profile`), and the editor's scratch copy. govax's embedded files
and the system volume aren't host files at all.

### A.5 The privilege gate

Phase 94's gate (`corevms/privrules.go`): a `PrivilegeRule` per guarded
operation, asked through `requirePrivilege` (the caller's mode),
`requirePrivilegeAt`, or for DCL's own commands `CommandPrivilege`
(supervisor mode), each tracing under SET DEBUG PRIVILEGES.
`TestNoUncheckedPrivileges` fails on code that reads the current
privileges itself, and `TestEveryPrivilegeRuleUsed` on a rule nothing
names. `rms` can't import `corevms`, so RMS reaches the gate through a
function the process sets on its session, as it reaches the process's
credentials.

## Part B — decisions

1. **The key.** `vax.console.host.sysprv`, a boolean in `cmd/govax`'s
   `validConfigs` and HELP CONFIGURATION KEYS, read once at startup into
   the `corevms.System` (`HostRequiresSYSPRV`). Off, the gate isn't
   asked, so nothing is traced and every path behaves as before.

2. **What counts as a host file.** A file a process names, or a program
   a process runs, that ends up on the host: every Locate result with
   `Host` set; a file the resolver finds in a `--path` directory for such
   a name; EXE$OPEN; SAVE and LOAD of ROM and NVRAM; the debugger's log;
   a shareable image found on the host by step 3 of the locator; a
   container file named in MOUNT, INITIALIZE/CONTAINER, or
   DEFINE/DEVICE; and SPAWN/HOST's program. Not counted (A.4): what
   govax opens for itself by configuration or command line, the system
   volume, and the embedded files. EDIT of a volume file is not a host
   access (its scratch copy is govax's own); EDIT of a host file is,
   through Locate.

3. **One rule, one gate.** A `PrivilegeRule`, `ruleHostFile` ("host
   file: open, create, or run"), SYSPRV, no exempt modes, SS$_NOPRIV.
   `Environment.HostAccess(name string) error` asks it (nil when the key
   is off) and is the only way in: DCL's paths from supervisor mode, a
   program's from its own. `UseSession` sets it on the session as
   `rms.Session.HostAccess`, beside `Credentials`.

4. **Checked in Locate.** Locate asks `HostAccess` before returning a
   host location, and a refusal is a `*rms.HostDeniedError{Name}`, which
   is `ErrNoPrivilege` (`errors.Is`) and so already a denied open to
   `IsDenied`. A caller that only asks which side a name is on uses a new
   `Session.Side` (Locate without the check), so a question isn't
   refused; each of A.2's callers is audited in subtask 3 and said to be
   one or the other.

5. **Messages and statuses.** A refused access is shown as the command
   shows a protection refusal on a volume file (Phase 93's
   `rms.IsDenied` paths: its OPENIN, OPENOUT, or the like, then
   `-RMS-E-PRV`), with the same `$STATUS`. Where a command has no such
   path (SAVE/ROM, SPAWN/HOST, the debugger's log), `%SYSTEM-F-NOPRIV`,
   SS$_NOPRIV, the rule's status. EXE$OPEN returns -1, as for a failed
   open. A program's RMS paths (LBR$OPEN, a created process's files)
   return RMS$_PRV.

6. **Whose privilege, and when.** The privileges of the process making
   the access, at that moment: SET PROCESS/PRIVILEGES=NOSYSPRV counts at
   once, an installed image with SYSPRV in its privileges has it while it
   runs, a subprocess or detached process uses its own. A file opened
   before the privilege was turned off stays open (a DCL OPEN, a
   procedure being run); only new accesses are checked.

7. **Who is affected.** SYSTEM, the default login, holds SYSPRV, so
   STARTUP.COM and a session without `--user` are unchanged. The one-shot
   subcommands (`govax macro`, `govax run`, ...) run as the logged-in
   user and follow the same rule.

8. **A test that finds new paths.** `TestNoUncheckedHostAccess`, like
   Phase 93's `TestNoUncheckedAccess`: outside a named list of files
   (A.4's, and the gate's own), no package under `internal` may call
   `os.Open`, `os.OpenFile`, `os.Create`, `os.ReadFile`, `os.WriteFile`,
   or `exec.Command`.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The key and the gate** (Decisions 1, 3, 4): `validConfigs`, HELP
   CONFIGURATION KEYS, `System.HostRequiresSYSPRV`, `ruleHostFile`,
   `Environment.HostAccess`, `rms.Session.HostAccess`, `HostDeniedError`,
   the check in Locate and `Session.Side`; unit tests in `rms` and
   `corevms` (key off, on with and without SYSPRV, the trace).
3. **Locate's callers** (Decisions 4, 5): each of A.2's 55 calls audited
   (an access or a question), the questions moved to `Side`, each
   command's message for the refusal; console tests per command with the
   key on, as an account without SYSPRV, then with SYSPRV turned on, then
   with the key off.
4. **The other paths** (Decisions 2, 8): A.3's table (EXE$OPEN, ROM and
   NVRAM, the debugger's log, the shareable image locator's host step,
   the resolver's `--path` directories, SPAWN/HOST, the container
   commands), and `TestNoUncheckedHostAccess`.
5. **The close-out**: MANUAL.md (the key in the configuration chapter, a
   section on host files and who may reach them, `/HOST`'s and SPAWN/HOST's
   descriptions), HELP CONFIGURATION KEYS, HELP SPAWN `/HOST` and HELP
   HOST_FILES if there is one, VAX-VMS-INTERNALS.md (host files in the
   I/O and RMS sections, the gate in the privileges part), PLAN.md, and
   CLAUDE.md.

## The review

Questions for the author:

1. **Containers** (Decision 2): MOUNT, INITIALIZE/CONTAINER, and
   DEFINE/DEVICE name a host container file. Should they need SYSPRV as
   well under the key, beside the privileges they need today (SYSNAM for
   MOUNT, VOLPRO for an existing container)?
2. **SPAWN/HOST** (Decision 2): it runs any host program, a stronger
   access than reading a file. Should it need SYSPRV under the key, or
   be refused outright unless the key is off?
3. **EDIT** (Decision 2): EDIT of a volume file runs the host editor on a
   scratch copy. Leave it allowed, as the plan has it?
4. **`--path` directories and `vax.library`** (Decision 2): a bare name
   ASM, LOAD, or HELP finds in a `--path` directory, and a shareable
   image found in SET SHARE_PREFIX's or `vax.library`'s directory, are
   host files the configuration pointed at. Check them, as the plan has
   it, or treat them as the system's own?
5. **The key's name**: `vax.console.host.sysprv`, as asked, though it
   applies to programs and subprocesses too, not only the console?

## Progress log

- **2026-11-16** Subtask 1: read `rms/location.go` and its 55 callers,
  `rms.Session` (`Credentials`, `HostFallback`), `respath.Resolver`'s
  origins, the RMS services (which open no host files themselves),
  EXE$OPEN, ROM and NVRAM, the debugger's log, the shareable image
  locator, SPAWN/HOST, the container commands, and Phase 94's gate and
  command rules. Wrote Part A and this plan.
