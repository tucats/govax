# ISA / hardware-definition deviations

This is a running log of places where the C reference source (`reference/eVAX/`)
appears not to match the VAX architecture/instruction-set spec, or not to do what its
own comments say it does — found in the natural course of porting, not part of a
dedicated audit.

This is distinct from `reference/eVAX/AUDIT.md`, which documents the C project's own
closed 32-vs-64-bit portability audit. Findings here are about ISA/behavioral fidelity,
not C portability.

## Policy

Per project guidance (see `CLAUDE.md`): the C source's fidelity to the VAX ISA is good
but not perfect. When a suspected ISA/behavior mismatch is found while porting:

- If it's a clear, obvious logic error unrelated to ISA semantics (e.g. an off-by-one,
  a copy-paste mistake, dead/unreachable code) — just fix it in the Go code, no need to
  log it here.
- If it's a genuine ISA/hardware-definition fidelity question (the emulated behavior
  doesn't match the spec, or doesn't match what the C code's own comments claim) — log
  it below with enough detail to revisit later, and **replicate the C source's current
  behavior in the Go port** rather than fixing it now, unless the fix is clear-cut and
  fits naturally in the current change's scope.
- When it's not obvious which of the above applies, ask before deciding unilaterally.

Entries get resolved (fixed or deliberately kept, with rationale) during Phase 12
(`PHASE-12.md`) or whenever the relevant subsystem gets a dedicated debugging pass.

## Phase 11 (assembler) findings

### [Phase 11] Three P1-vector table entries land close enough together that `p1_init()`'s own trampoline writes clobber each other's trailing `RET`/`XFC` bytes

- **Where**: `reference/eVAX/eVAX/Source/RTL/p1_vector.c`'s `p1_vector[]`
  initializer and its own `p1_init()` (ported as `internal/asm/pseudo.go`'s
  `pseudoP1Vector`, backed by `internal/p1vector.Table`, a verbatim copy of
  the same array). `SYS$CLRAST_2` (`0x7FFEE110`) and `SYS$GL_ASTRET` (also
  `0x7FFEE110` — the same address) both deposit their 5-byte mask+XFC+RET
  trampoline ending at `0x7FFEE114`; `SYS$GL_COMMON` (`0x7FFEE114`), later in
  the array, then writes its own 2-byte zero mask word starting at that exact
  address, clobbering the `RET` opcode (`0x04`) both earlier entries just
  wrote there with `0x00`. `SYS$GL_COMMON`'s own trailing `RET` (at
  `0x7FFEE118`) is in turn clobbered the same way by `SYS$SRCHANDLER`
  (`0x7FFEE118`, the array's one JMP-reached entry, processed right after
  it), whose XFC opcode byte (`0xFC`) lands on exactly that address.
- **What**: `p1_init()` (and this port's faithful line-for-line translation
  of it) writes every array entry's trampoline unconditionally, in array
  order, with no check for a following entry's address falling inside the
  5-byte span it just wrote. `SYS$GL_ASTRET`/`SYS$GL_COMMON` aren't really
  independent callable service entry points at all — real VMS documents them
  as plain longword "global location" data cells that happen to share this
  table purely to get a symbol defined at their address — so clobbering their
  own trailing bytes is likely harmless in practice (nothing legitimately
  `CALLS`/`JMP`s into a `GL_` cell), but `SYS$CLRAST_2` is a real,
  `CALL`-shaped entry whose `RET` this corrupts: a program that actually
  reached it via `CALLS` would fall through into whatever garbage follows
  once `XFC$P1VECTOR`'s handler returns, instead of unwinding cleanly.
  Confirmed present in the C source itself (not a porting slip): the same
  three addresses collide in `p1_vector.c` in the same order, so the real
  reference tool's own `p1_init()` would produce byte-for-byte the same
  corrupted output.
- **Status**: open, deferred, per this project's default policy for a
  finding rooted in the C source's own data/logic rather than a Go porting
  mistake — `pseudoP1Vector` replicates `p1_init()`'s unconditional,
  no-overlap-check write order exactly rather than special-casing these
  three names. No fixture in this project calls `SYS$CLRAST_2`,
  `SYS$GL_ASTRET`, or `SYS$GL_COMMON` (confirmed by inspection of every
  `testdata/asm/*.asm`/`testdata/exe/*` fixture's actual `SYS$` call sites),
  so this has no test-visible effect today. Revisit if a future fixture ever
  needs a working `SYS$CLRAST_2`.

### [Phase 11, found in Phase 26] MACRO-32 local labels (`n$`) aren't supported

- **Where**: `internal/asm` (operand value parsing and the symbol table).
- **What**: MACRO-32's local labels — `1$:`, `2$:`, ..., each valid only
  within a *local label block*, the code between two ordinary labels —
  aren't implemented. As a label definition, `2$:` defines an ordinary
  global symbol named `2$`, so a second `2$:` anywhere in the file is
  `VAX-E-DUPSYM`. In an operand, `2$` is read as the number 2, so
  `BNEQ 2$` branches to address 2 rather than to the label: found when
  a `docs/PHASE-26.md` subtask 34 fixture's handler branched into the
  middle of the next procedure.
- **Status**: open. The Phase 26 fixtures use ordinary labels. Fixing it
  means parsing `digits$` as a symbol reference and scoping it to the
  current local label block.

## Phase 22 (RMS / `ods2`) findings

Phase 22 (`PHASE-22.md`) has no `reference/eVAX` counterpart at all — its own
correctness reference is the real VMS RMS manual (`rms_manual.pdf`) and the
sibling `github.com/tucats/ods2` module's actual on-disk/record-format
behavior, not the C source. The entries below are that phase's own findings
in this same spirit (a real-RMS-vs-actual-implementation gap worth recording
rather than silently guessing at), kept in this file per that phase's own
"Bug-fixing policy"-style direction rather than starting a second log.

### [Phase 22] `FAB$C_UDF` (undefined record format, `FAB$B_RFM` = 0) is rejected outright instead of resolved to a default

- **Where**: `internal/rms/create.go`'s `createOnVolume` (`SYS$CREATE`'s
  real-volume path): `format := ondisk.RecordFormat(rfm); if format <
  ondisk.RecordFormatFixed || format > ondisk.RecordFormatStreamCR { return
  0, rmsInvalidRFM, nil }` — `ondisk.RecordFormatUndefined` (`FAB$C_UDF`,
  the real VMS value `0`) falls outside that range and is reported as
  `RMS$_RFM`, a hard failure.
- **What**: real RMS lets a calling program leave `FAB$B_RFM` unset (`0`,
  `FAB$C_UDF`) and picks a sensible default record format on its behalf
  (`rms_manual.pdf`'s own `$CREATE`/FAB description) rather than rejecting
  the call outright. This phase's own acceptance-test program and every
  unit-test fixture built so far always sets `FAB$B_RFM` explicitly (per
  `docs/PHASE-22.md` subtask 5's own progress-log note), so there's no
  concrete calling program yet to motivate *which* default `ods2` should
  pick (Fixed? Stream? something derived from `FAB$B_RAT`?) — a real,
  deliberate gap in what's implemented, not a bug in what is.
- **Status**: open, deferred. Left rejecting `FAB$C_UDF` with `RMS$_RFM`
  until a real calling program that relies on the default-format behavior
  shows up to motivate the right default, rather than guessing one now.

## Phase 23 (Files-11 console commands) findings

Phase 23 (`PHASE-23.md`) also has no `reference/eVAX` counterpart, like Phase
22 before it — its own correctness reference is `github.com/tucats/ods2`'s own
`cmd/ods2/internal/session` package's CLI behavior (read-only, per that
phase's own "behavioral reference, not code to link against" framing) compared
against real VMS DCL conventions. Most of this phase's own judgment calls
(the `INIT`/`INITIALIZE` verb unification, the `COPY`/`/HOST` direction
design, parameter-scoped qualifiers) are DCL-engine/UX decisions captured
directly in `PHASE-23.md`'s own "Design decisions" section rather than logged
here, per that doc's own subtask 12 note. The entries below are the two
places where `ods2`'s own CLI output shape was found to visibly diverge from
real VMS DCL's `DIRECTORY` conventions, left unresolved (not obviously a bug
to fix, not obviously acceptable forever either) rather than silently
guessed at.

### [Phase 23] `DIRECTORY`'s master-file-directory header prints `Directory DUA0:[]` instead of real VMS's `[000000]` form

- **Where**: `internal/rms/directory.go`'s `Session.Directory`, functionally
  matched against `ods2`'s own `cmd/ods2/internal/session/directory.go`
  (`formatDirectoryEntry`/`groupMatchesByDir`).
- **What**: real VMS DCL's own `DIRECTORY` command headers a volume's master
  file directory as `Directory DUA0:[000000]` (or whatever the device's MFD
  path is) — never an empty bracket pair. `ods2`'s own `formatDirectoryEntry`
  doc comment already flags this as a deliberate simplification on its own
  side: an empty joined `Dirs` slice prints as `[]`, with no `[000000]`
  fallback the way `filespec.Spec.String`'s own *error-message* formatting
  applies elsewhere in that module. `Session.Directory` (subtask 5) reproduces
  that same quirk rather than fixing it independently, matching this phase's
  own "closely enough to be recognizable, not necessarily byte-identical"
  framing for `DIRECTORY` output — but it is a real, visible divergence any
  operator familiar with genuine VMS DCL would notice immediately.
- **Status**: open, deferred. Kept matching `ods2`'s own current output rather
  than diverging from the behavioral reference unilaterally, or second-guessing
  `ods2`'s own documented simplification on this project's own initiative.
  Revisit if `ods2` itself ever fixes this, or if a future phase decides
  `DIRECTORY` output should target closer VMS-visual fidelity than `ods2`'s own
  CLI provides (see the next entry, the same open question either way).

### [Phase 23] `DIRECTORY`'s one-file-per-line output doesn't match real VMS's column-aligned multi-file-per-line listings

- **Where**: `internal/rms/directory.go`'s `Session.Directory`, and `ods2`'s
  own `formatDirectoryEntry` it was functionally matched against.
- **What**: real VMS's `DIRECTORY` command packs several short file names per
  line, column-aligned, once a directory has enough entries; both `ods2`'s own
  CLI and this port's `Session.Directory` print exactly one file per line
  regardless of how many entries there are. `ods2`'s own doc comment already
  acknowledges this as a deliberate simplification on its own side, inherited
  here rather than re-decided independently — see `PHASE-23.md`'s own "Open
  questions" section, where this was explicitly flagged during planning and
  left leaning toward inheriting the simplification (matching this phase's
  general "`ods2`'s CLI is the reference" framing) rather than investing in
  closer VMS visual fidelity.
- **Status**: open, deferred, per explicit flagging in `PHASE-23.md` itself
  rather than a silent, undocumented choice. No test in this project's own
  suite currently depends on either the single- or multi-file-per-line shape,
  so there is no compatibility cost either way today — revisit if an operator
  workflow, or a future phase's own acceptance criteria, actually needs the
  denser, real-VMS-style layout.

## Phase 25 (logical names) findings

Phase 25 (`PHASE-25.md`) replaced eVAX's logical-name support rather than
porting it. That was the user's explicit direction: the existing behavior was
treated as suspect, and VAX/VMS 7.3 is the target. The references are the VSI
OpenVMS User's Manual (chapter 11), the VMS 5.0 System Services Reference
Manual, and the CLRM §2.2. The first group of entries below records each eVAX
behavior the phase deliberately changed, so the difference from
`reference/eVAX` can be traced. The second group records the gaps that remain
against real VMS.

### [Phase 25] eVAX logical names: single-valued names in a flat table map, with `LNM$FILE_DEV` as a table

- **Where**: `reference/eVAX/eVAX/Source/RTL/logical_names.c` (`set_logical`/
  `get_logical`/`init_logicals`), ported in Phase 09 as `internal/io/logical.go`.
- **What**: each table mapped a name to exactly one value. `LNM$FILE_DEV` was
  a table holding `SYS$INPUT`/`OUTPUT`/`ERROR`/`COMMAND` = `TTA0:`, where in
  VMS it is a logical name (a search list of tables). There were no
  directories, search order, iterative translation, search lists, or access
  modes. Attributes were matched by equality, not as bits, and a lookup with
  no table fell back to a nonexistent `LNM$ROOT`.
- **Status**: fixed in Phase 25. `internal/lnm` models the VMS structure:
  directory tables, `LNM$FILE_DEV`/`LNM$DCL_LOGICAL` as logical names, per-mode
  entries, search lists, and `CONCEALED`/`TERMINAL`/`NO_ALIAS`/`CONFINE`.
  `internal/io/logical.go` is deleted. The process-permanent names are now
  `_TTA0:` at executive mode with `TERMINAL`, and `TT` is `_TTA0:`.

### [Phase 25] eVAX console `DEFINE/LOGICAL` wrote to a table nothing searched

- **Where**: `reference/eVAX/eVAX/Source/Console/define_logical.c`,
  `show_logical.c`; ported as `internal/console/device.go`'s
  `DefineLogical`/`ShowLogicals`.
- **What**: `DEFINE/LOGICAL name value` defaulted to the table `LNM_PROCESS`
  (underscore, not `$`), which neither RMS nor `$TRNLNM` ever searched, so an
  operator-defined name was invisible to programs. `DEFINE/LOGICAL` and `SHOW
  LOGICAL_NAMES` aren't VMS syntax, and the SHOW output wasn't VMS's format.
- **Status**: fixed in Phase 25. The VMS commands replace them: `DEFINE`,
  `ASSIGN`, `DEASSIGN`, `CREATE/NAME_TABLE`, `SHOW LOGICAL` in VMS format,
  and `SHOW TRANSLATION`. The eVAX forms were removed (open question 4).

### [Phase 25] eVAX RMS translated only a file spec that was, in its entirety, a logical name

- **Where**: `reference/eVAX/eVAX/Source/RTL/rms.c`'s spec lookup, carried
  into `internal/rms/create.go`/`open.go`.
- **What**: the whole spec string was looked up in the table `LNM$FILE_DEV`,
  so `DISK:FILE.DAT` never translated, nothing iterated, and search lists and
  `_` weren't handled. The console file commands did no translation at all.
- **Status**: fixed in Phase 25. The leftmost component is translated
  through `LNM$FILE_DEV`, iteratively, restarting the search order at each
  level. Search lists fan out per the User's Manual. `SYS$DISK` supplies the
  default device, the fields the user wrote take precedence over the
  translation's, and a translation loop is `RMS$_LNE`.

### [Phase 25] eVAX `SYS$TRNLNM` item codes, and no `SYS$CRELNM`/`DELLNM`/`CRELNT`

- **Where**: `reference/eVAX/eVAX/Source/RTL/logical_names.c`'s
  `sys_trnlnm`; `p1_vector.c`, where `SYS$CRELNM`/`SYS$DELLNM` have routine
  pointer 0.
- **What**: `LNM$_TABLE` wrote through a descriptor instead of a buffer, and
  returned the table name the caller passed in rather than the table holding
  the name. `LNM$_LENGTH`/`LNM$_MAX_INDEX` were stored as words (the manual
  says longwords). `LNM$_MAX_INDEX` was always 1. `LNM$_INDEX`/`LNM$_CHAIN`
  were ignored, and an unknown item code was silently skipped. More than 5
  arguments returned a status that isn't in the VAX `$SSDEF`. The other
  services didn't exist.
- **Status**: fixed in Phase 25. `$TRNLNM` follows the manual, and
  `$CRELNM`/`$DELLNM`/`$CRELNT` plus the pre-V4 `$CRELOG`/`$DELLOG`/`$TRNLOG`
  are implemented. `$ASSIGN`/`$GETDVIW` translate device names.

### [Phase 25] No privilege model: logical-name privileges aren't checked

- **Where**: `internal/lnm` (`Define`/`Delete`/`CreateTable`) and
  `internal/rtl/logicals.go`.
- **What**: VMS requires SYSNAM for executive or kernel mode names, GRPNAM
  for the group table, SYSNAM/SYSPRV for the system table, and SYSPRV for
  shareable tables. govax has no privileges, so every caller is treated as
  holding all of them. An explicit `acmode` to `$CRELNM`/`$DELLNM`/`$CRELNT` is
  used as-is (the SYSNAM rule) rather than maximized with the caller's mode.
  The old by-value services do maximize, since their 0 means "omitted".
  Structural rules still hold: a name can't be more privileged than its
  table, and the startup tables can't be deleted.
- **Status**: open, deliberate. Every write goes through
  `Database.Define`/`Delete`/`CreateTable`, so checks can be added there once
  a privilege model exists.

### [Phase 25] No job or cluster tables, no quotas or table protection

- **Where**: `internal/lnm.NewDatabase`; `$CRELNT`'s `quota`/`promsk`.
- **What**: VMS's `LNM$FILE_DEV` is process, job, group, system (and cluster).
  govax has one process and no cluster, so it has no `LNM$JOB` table, and
  MOUNT's `DISK$label` always goes in the system table (VMS uses the job
  table without `/SYSTEM`). Table quotas and protection masks are accepted and
  ignored.
- **Status**: open, deliberate (open question 1). The directory-based
  design makes adding `LNM$JOB` a few lines.

### [Phase 25] Process-permanent names carry no ESC/IFI prefix

- **Where**: `internal/lnm.Database.DefineProcessNames`.
- **What**: VMS stores `SYS$INPUT`/`OUTPUT`/`ERROR`/`COMMAND` with a hidden
  4-byte ESC/IFI header naming the process-permanent file. `$TRNLNM` returns
  it and RMS uses it. govax stores the plain device name, and RMS reaches
  the console through its `TTA0` device special case instead.
- **Status**: open, deliberate. Out of scope until RMS models
  process-permanent files.

### [Phase 25] Smaller DCL/RMS simplifications

- **Where**: `internal/console/logical.go`, `internal/rms/logicals.go`,
  `internal/rms/copy.go`, `internal/rms/directory.go`, `internal/console/run.go`.
- **What**:
  - `/TRANSLATION_ATTRIBUTES` applies to every equivalence string. In VMS it
    is positional.
  - COPY takes a search list's first element for its source, not the first
    element where the file exists.
  - DIRECTORY over several directories prints one `Total of` line, with no
    `Grand total` line.
  - User-mode names are run down only when RUN's image returns from its main
    routine. govax has no `SYS$EXIT`, so an image ending in HALT or a fatal
    exception isn't run down.
  - `$CRELOG`/`$DELLOG`/`$TRNLOG` follow the V4-era interface descriptions.
    The 7.3 source archive doesn't contain them to check against.
- **Status**: open, deferred. None affects an existing test or fixture.
  Revisit case by case if a workload needs the exact VMS behavior.

## Phase 26 (system services) findings

Phase 26 (`PHASE-26.md`) adds VMS system services eVAX never implemented, so
there is no C behavior to preserve; the reference is the VMS 5.0 System
Services Reference Manual. These entries record where govax's version is
knowingly simpler than real VMS, plus the one existing service whose behavior
changed as a result.

### [Phase 26] `$ADJSTK` doesn't probe the new stack segment

- **Where**: `internal/rtl/process.go`'s `serviceSysAdjstk`.
- **What**: the manual returns `SS$_ACCVIO` when "a portion of the new stack
  segment cannot be written by the caller". govax checks only that `newadr`
  can be read and written, not the memory the new stack pointer points at.
- **Status**: open, deliberate simplification.

### [Phase 26] Working-set limits are recorded, not enforced

- **Where**: `internal/rtl/process.go` (`Process.WSLimit`/`WSDefault`/
  `WSQuota`/`WSExtent`/`MinWSCount`, `serviceSysAdjwsl`).
- **What**: `$ADJWSL` adjusts and clamps the limit as documented, but govax
  has no paging or working set, so the limit has no effect on execution. The
  quota values are nominal, not from a real UAF or SYSGEN.
- **Status**: open, by design (the user asked for `$ADJWSL` for
  completeness).

### [Phase 26] `$ALLOC` simplifications

- **Where**: `internal/rtl/devices.go`'s `serviceSysAlloc`.
- **What**: condition values govax never returns, because it has nothing to
  check them against:
  - `SS$_DEVOFFLINE`: devices have no online/offline state.
  - `SS$_NOPRIV`: no privileges or device protection; no spooled devices
    (`ALLSPOOL`).
  - `SS$_TEMPLATEDEV`, `SS$_NONLOCAL`, and the `$ENQ` statuses: no template
    devices, cluster, or lock manager.
  - `SS$_DEVALLOC` for "other processes had channels assigned" to a
    shareable device: there is only one process.

  Also, a generic allocation (`flags` bit 0) matches `devnam` only against
  device-type names (`RA81`), not against a generic device name such as
  `DU:`; and `$ASSIGN` doesn't implicitly allocate a nonshareable device, as
  VMS does.
- **Status**: open, deliberate simplifications.

### [Phase 26] `$ASSIGN` now refuses a device allocated to another process

- **Where**: `internal/rtl/devices.go`'s `serviceSysAssign`.
- **What**: `$ASSIGN` returns `SS$_DEVALLOC` for a device allocated
  (`DEV$M_ALL`) to a different PID, as on VMS. eVAX's `sys_assign` had no
  allocation concept and always assigned the channel, overwriting the
  device's PID.
- **Status**: fixed in Phase 26 subtask 4.

### [Phase 26] eVAX event flags: no common clusters, no range check, no previous-state status

- **Where**: `reference/eVAX/eVAX/Source/RTL/service.c`'s `sys_clref`/
  `sys_setef`/`sys_readef`, ported into `internal/rtl/core.go`.
- **What**: flags 64-127 were stored in process-local longwords, so there
  were no common event flag clusters and no `SS$_UNASEFC`. The flag number
  was reduced `% 0xFF` (or `& 0xFF`), so flags 128-254 indexed past the
  four-longword array instead of returning `SS$_ILLEFC`. `$SETEF`/`$CLREF`
  always returned `SS$_NORMAL` rather than `SS$_WASSET`/`SS$_WASCLR`.
- **Status**: fixed in Phase 26 subtask 5, per the VMS 5.0 System Services
  Reference Manual. The services moved to `internal/rtl/eventflags.go`.
  Flags 64-127 reach the common cluster `$ASCEFC` associated, or fail with
  `SS$_UNASEFC`. A program relying on eVAX's local flags 64-127 now needs to
  call `$ASCEFC` first, as it would on VMS.

### [Phase 26] `$ASCEFC` simplifications

- **Where**: `internal/rtl/eventflags.go`'s `serviceSysAscefc`.
- **What**: no `TQELM` quota (`SS$_EXQUOTA`), no multiport shared memory
  (`SS$_EXPORTQUOTA`, `SS$_INTERLOCK`, `SS$_NOSHMBLOCK`,
  `SS$_SHMNOTCNCT`), and `PRMCEB` is always held. Cluster names are
  compared case-sensitively as given. `$DLCEFC` (subtask 7) never returns
  `SS$_NOPRIV`, since `PRMCEB` is always held.
- **Status**: open, deliberate simplifications.

### [Phase 26] eVAX `$GETJPIW`: two hard-coded items with the wrong lengths

- **Where**: `reference/eVAX/eVAX/Source/RTL/service.c`'s `sys_getjpiw`,
  ported into `internal/rtl/core.go`.
- **What**: only `JPI$_ACCOUNT` and `JPI$_CLINAME` were recognized.
  `ACCOUNT` returned the stand-in `"USER    "` with a return length of 4,
  though the manual defines an 8-byte blank-padded field. `CLINAME` always
  wrote 4 bytes (`DCL` and a NUL) whatever the buffer length. Any argument
  count other than exactly 7 was `SS$_INSFARG`.
- **Status**: fixed in Phase 26 subtask 8. `internal/rtl/getjpi.go`
  implements `$GETJPI`/`$GETJPIW` from the manual over `rtl.Process`, with
  21 items. `ACCOUNT` is the process's account, `SYSTEM`, with length 8.

### [Phase 26] `$GETJPI` simplifications

- **Where**: `internal/rtl/getjpi.go`.
- **What**:
  - Only 28 item codes are supported (21 until subtask 21 added the AST
    items, the priorities, and `STATE`). The others, for state govax
    doesn't model (`IMAGNAME`, most quotas, privileges, CPU and I/O
    accounting), are `SS$_BADPARAM`.
  - `JPI$_STATE` is always `SCH$C_CUR`, and the priorities are fixed
    values: there's one process and no scheduler.
  - The AST quota (`JPI$_ASTLM`/`ASTCNT`) is reported but not enforced.
  - (`astadr` is delivered as an AST since subtask 16.)
  - The wildcard context stored at `pidadr` is govax's own value.
  - `SS$_NOPRIV`/`SS$_SUSPENDED` can't happen, since there is one process.
  - `JPI$_WSSIZE` reports the working-set limit, since there is no real
    working set.
- **Status**: open, deliberate simplifications.

### [Phase 26] Event-flag waits re-execute the service instead of blocking

- **Where**: `internal/rtl/eventflags.go` (`$WAITFR`/`$WFLAND`/`$WFLOR`),
  `internal/cpu/xfc.go` (`ErrServiceWait`).
- **What**: an unsatisfied wait re-executes the service's `XFC` on every
  instruction step, instead of descheduling the process. Interrupts are
  still delivered between retries, so a timer interrupt handler can end
  the wait, and so are ASTs (since subtask 15), so an AST interrupts a
  wait and the wait resumes afterwards, as on VMS. Not modeled:
  - the `LEF`/`CEF` process state and `JPI$_EFWM`;
  - releasing the host CPU while waiting.
- **Status**: open, by design.

### [Phase 26] `$SETIMR`/`$CANTIM` simplifications

- **Where**: `internal/rtl/timers.go`, `internal/cpu/systime.go`.
- **What**:
  - The CPU-time flag is treated as elapsed time.
  - There is no `TQELM` quota.
  - (Resolved in subtasks 15-16: `astadr` is now delivered as an AST, and
    timers expire before every instruction rather than only at the next
    event-flag service.)
  - In quantum mode, emulated time is one millisecond per quantum tick
    (20 instructions by default), so a "second" is a fixed amount of
    execution, not wall-clock time. Set `vax.hardware.clock` for real
    time.
- **Status**: open, by design.

### [Phase 26] `$GETTIM`'s clock isn't rounded to 10ms

- **Where**: `internal/rtl/vmstime.go`, `internal/cpu/systime.go`.
- **What**: VMS updates its system time every 10ms, so `$GETTIM` returns
  multiples of 100,000. govax's system time moves in 1ms steps (one per
  interval-clock tick) and is returned as is.
- **Status**: open, by design.

### [Phase 26] `$ASCTIM`/`$BINTIM` details the manual leaves open

- **Where**: `internal/rtl/vmstime.go`.
- **What**:
  - `$ASCTIM` truncates hundredths rather than rounding them.
  - An absolute time past 31-DEC-9999 is `SS$_IVTIME` from `$ASCTIM`
    (it doesn't fit `yyyy`).
  - Bad addresses return `SS$_ACCVIO` instead of raising an access
    violation.
  - The manual's example `--1989 0:0:0.0` → `29-DEC-1989` is taken as a
    typo; govax uses today's day of the month, giving `30-DEC-1989`.
- **Status**: open, by design; revisit if a real VMS disagrees.

### [Phase 26] Hibernation simplifications

- **Where**: `internal/rtl/hibernate.go`, `internal/rtl/timers.go`.
- **What**:
  - `$WAKE`, `$SCHDWK`, and `$CANWAK` can only name the calling process
    (there are no others), so `SS$_NOPRIV` never happens.
  - No `ASTLM` quota (`SS$_EXQUOTA`) or `SS$_INSFMEM`.
  - `$HIBER` waits by re-executing its `XFC`, like the event-flag waits.
    There's no `HIB` process state.
- **Status**: open, by design.

### [Phase 26] eVAX `$SETAST`: one flag, always `SS$_NORMAL`

- **Where**: `reference/eVAX` `service.c` (`vms_ast_flag`), now
  `internal/rtl/ast.go`.
- **What**: eVAX recorded a single process-wide flag and returned
  `SS$_NORMAL`. The manual has one switch per access mode (the caller's),
  and returns `SS$_WASSET`/`SS$_WASCLR` for its previous state.
- **Status**: fixed in Phase 26 subtask 15, when ASTs began to be
  delivered.

### [Phase 26] AST delivery simplifications

- **Where**: `internal/rtl/ast.go`, `internal/cpu/ast.go`.
- **What**:
  - (Resolved in subtask 22: an inner-mode AST is now delivered to
    outer-mode code by switching into the AST's mode, as on VMS. Until
    then it waited for the CPU to enter that mode.)
  - Delivery is done by the RTL at instruction boundaries, not through
    the `ASTLVL` register, `REI`, and an IPL 2 software interrupt. The
    conditions checked (IPL < 2, not on the interrupt stack, enabled,
    none active) are VMS's.
  - The AST exit is the `SYS$CLRAST` vector entry, reached by the
    routine's `RET` rather than by VMS's own dispatcher code.
  - No `ASTLM` quota (`SS$_EXQUOTA`) or `SS$_INSFMEM`.
- **Status**: open, by design.

### [Phase 26] Terminal `$QIO` simplifications

- **Where**: `internal/rtl/qio.go`, `internal/rtl/ttdriver.go`.
- **What**:
  - Every terminal request completes before `$QIO` returns. A read with
    no input typed yet blocks the whole emulator until the host delivers
    a line. (Mailbox requests can wait, since subtask 29.)
  - Only terminals and mailboxes have drivers. A `$QIO` to any other
    device (a disk, say) is `SS$_ILLIOFUNC`, where VMS would perform the
    I/O.
  - Every terminal is the console: reads come from the host's input,
    which is line-buffered and already echoed by the host (or not echoed
    at all, when redirected). So `IO$M_NOECHO`/`TRMNOECHO` change
    nothing, there's no line editing, and a host newline stands for
    RETURN. The end of the host's input is `SS$_ENDOFFILE`.
  - `IO$M_TIMED` with a zero time limit reads only characters already
    buffered; a nonzero limit is ignored (the read waits indefinitely).
  - `IO$M_NOFILTR`, `REFRESH`, `ESCAPE`, `DSABLMBX`, and the write
    modifiers are accepted and have no effect. `IO$_SETMODE` with
    `IO$M_OUTBAND`, `HANGUP`, and the other modifiers succeeds without
    doing anything. (`IO$M_CTRLCAST` and `CTRLYAST` work since subtask
    27; see the CTRL/C entry below.)
  - `IO$_SENSEMODE`'s IOSB reports no line speeds, fill counts, or
    parity.
  - No `BIOLM`/`DIOLM`/`BYTLM`/`ASTLM` quotas (`SS$_EXQUOTA`),
    `SS$_INSFMEM`, `SS$_DEVOFFLINE`, or network functions.
- **Status**: open, by design.

### [Phase 26] Exit handler simplifications

- **Where**: `internal/rtl/exit.go`, `internal/cpu/exit.go`.
- **What**:
  - `$EXIT` calls only the exit handlers of the mode it's called from.
    VMS then runs the supervisor- and executive-mode handlers in their
    own modes during rundown.
  - An image that ends by an unhandled exception doesn't have its exit
    handlers called; the console reports the exception and stops.
  - `$EXIT` ends the image by returning from the console's call frame
    (or halting, without one), not by transferring to a command
    interpreter.
  - eVAX's `$DCLEXH` recorded one handler address and nothing called it;
    that stub is replaced.
- **Status**: open, by design.

### [Phase 26] `$GETSYI` simplifications

- **Where**: `internal/rtl/getsyi.go`.
- **What**:
  - The system is one node outside any cluster: only this node can be
    named, a wildcard scan finds only it, and the cluster items report
    no membership and CSID 0.
  - Only 10 item codes are supported, of the system's identity, boot
    time, and `MINWSCNT`. Other SYSGEN parameters, the `*_EMULATED`
    flags, and hardware model names are `SS$_BADPARAM`.
  - `SYI$_VERSION` reports `V7.3`, the release of the VMS definitions
    govax is built from, though the services follow the VMS 5.0 manual.
  - No `ASTLM` quota (`SS$_EXASTLM`).
- **Status**: open, by design.

### [Phase 26] `$FAO`/`$FAOL` simplifications

- **Where**: `internal/rtl/fao.go`.
- **What**:
  - The directives are the VMS 5.0 manual's, plus the VMS 7 size letters
    `A`, `I`, `H`, and `J` (all a longword on a VAX, which is how the VMS
    7.3 message texts' `!XH` reads) and `Q` (a quadword, by reference).
    VMS 7's `!%C`, `!%E`, and `!%F` (plural choices) are `SS$_BADPARAM`.
  - `!%I` has no rights database: it names only the process's own UIC
    (`[SYSTEM]`), writes other UICs as `!%U` does, and other identifiers
    as `%X` and eight hexadecimal digits.
  - `!%U` writes the group and member in octal without padding
    (`[1,4]`).
  - A `!n<` field whose text is longer than `n` is truncated to `n`.
  - A decimal number too wide for an explicit field fills it with
    asterisks for `!Zx` too (the manual's table gives the rule only for
    signed and unsigned decimal).
- **Status**: open, by design.

### [Phase 26] `$GETMSG`/`$PUTMSG` simplifications

- **Where**: `internal/rtl/message.go`, `internal/vmsdef/gen/msg.go`.
- **What**:
  - Only the CLI, LIB, MTH, OTS, RMS, and SYSTEM facilities of the VMS
    7.3 system message file are known. Images can't carry message
    sections, and there's no process message file or SET MESSAGE: the
    default flags are always all four parts.
  - The texts are VMS 7.3's, which differ in places from the VMS 5.0
    manual's (`SS$_DUPLNAM` is "duplicate name", not "duplicate process
    name").
  - Nine texts were cut at 132 columns in the listing they come from
    (CLI `WRGSUBSHSYN`, `UNTERMSUBSH`, `DUPREDSYN`, `INVCONCHAR`; RMS
    `DELJNS`; SYSTEM `SIG_ARGMISMATCH`, `HPARITH`, `PAGRDERRXM`,
    `ILLEGAL_SHADOW`) and are shown as far as they go.
  - A value without a message gets VMS 7.3's stand-in, `%FAC-S-NOMSG,
    Message number XXXXXXXX`, rather than the VMS 5.0 manual's
    `NONAME` form.
  - `$PUTMSG` writes to the console once; VMS writes to `SYS$ERROR` and
    also `SYS$OUTPUT` when they differ. It doesn't refuse a call from
    kernel mode.
- **Status**: open, by design.

### [Phase 26] `$CMKRNL`/`$CMEXEC` simplifications, and VMINIT's stacks

- **Where**: `internal/rtl/cmode.go`; VMINIT (`internal/console`).
- **What**:
  - The process holds every privilege, so neither service returns
    `SS$_NOPRIV`.
  - `$CMKRNL` doesn't load R4 with the address of a process control
    block: govax has none.
  - VMINIT's executive and supervisor stacks had the kernel stack's
    protection (only kernel mode could write them), and a caller passing
    0 pages put them at the kernel stack's own address, so a routine run
    by `$CMEXEC` faulted on its first push. Fixed at the user's request:
    each now has its own pages (8 by default), protected `EW`/`SW`, with a
    guard page below; see `docs/MODE-STACKS.md`. They're in S0, not P1
    as on VMS.
- **Status**: the services' simplifications are open, by design; the
  stacks are fixed.

### [Phase 26] eVAX `$GETDVIW`: three items, wrong sizes

- **Where**: `devices.c`'s `sys_getdviw`, ported to
  `internal/rtl/devices.go`.
- **What**: it knew `DVI$_DEVCLASS`, `DEVTYPE`, and `DEVBUFSIZ`, wrote
  the first two as single bytes (VMS returns longwords), required exactly
  eight arguments, and returned `SS$_IVCHAN` for an unassigned channel
  (the manual: `SS$_NOPRIV`). There was no `$GETDVI`.
- **Status**: fixed in Phase 26 subtask 28 (`internal/rtl/getdvi.go`).

### [Phase 26] `$GETDVI` simplifications

- **Where**: `internal/rtl/getdvi.go`.
- **What**:
  - One node: `FULLDEVNAM` and `ALLDEVNAM` carry its name, the allocation
    class is 0, and no device is remote or served (`SS$_NONLOCAL` can't
    happen).
  - No host, shadow-set, lock-name (`DEVLOCKNAM`), `LOGVOLNAM`,
    `NEXTDEVNAM`, `TRACKS`, `VPROT`, or `ACPTYPE` items (`SS$_BADPARAM`).
  - No secondary devices: `DVI$M_SECONDARY` is ignored.
  - No `ASTLM` quota (`SS$_EXASTLM`).
- **Status**: open, by design.

### [Phase 26] Mailbox simplifications

- **Where**: `internal/rtl/mailbox.go`, `internal/rtl/mbxdriver.go`,
  `internal/lnm/database.go`.
- **What**:
  - One process: a mailbox connects the process with itself (its AST
    routines, its parts). The IOSB's process IDs are always its own.
  - `LNM$TEMPORARY_MAILBOX` is `LNM$PROCESS`, not `LNM$JOB`: govax has
    no job table.
  - A write that doesn't fit in the mailbox's buffer space completes
    with `SS$_MBFULL`; VMS normally makes the writer wait (resource wait
    mode, `$SETRWM`).
  - `IO$_SETMODE` does nothing: no read or write attention ASTs
    (`IO$M_READATTN`, `WRTATTN`), no protection changes. `promsk` is
    recorded, not enforced.
  - No `BYTLM` quota (`SS$_EXBYTLM`), shared-memory mailboxes, or
    termination mailboxes.
  - `$DASSGN` cancels pending requests with `SS$_CANCEL`, as `$CANCEL`
    does.
  - INIT/VMINIT/ZERO delete every mailbox, permanent ones included.
- **Status**: open, by design.

### [Phase 26] Process-control simplifications

- **Where**: `internal/rtl/process.go`.
- **What**:
  - There is one process, so `$SETPRI`, `$FORCEX`, and `$DELPRC` of any
    other process are `SS$_NONEXPR`, and `$SETPRN` never finds a
    duplicate name (`SS$_DUPLNAM`).
  - `$SETPRI` sets the current priority to the new base priority; there
    is no scheduler to boost it, and the priority changes nothing.
  - `$DELPRC` of the caller ends the image without exit handlers, but
    the process isn't deleted: the console runs the next image in it.
- **Status**: open, by design.

### [Phase 26] CTRL/C and CTRL/Y AST simplifications

- **Where**: `internal/rtl/ctrlast.go`, `internal/cpu/attention.go`,
  `cmd/govax/attention.go`.
- **What**:
  - Only the host's Ctrl-C reaches the program. Host Ctrl-Y isn't
    intercepted (readline's yank; macOS's DSUSP), so a CTRL/Y AST runs
    only when Ctrl-C is typed with no CTRL/C AST enabled, as VMS does.
  - The terminal echoes nothing (VMS echoes `^C`, and `*INTERRUPT*` for
    a CTRL/Y the command interpreter takes).
  - A key typed while the program is blocked in a terminal read (waiting
    for the host) is only seen after the read returns.
  - The requests belong to the process's channels, not to a terminal
    device: every terminal is the console.
  - No `ASTLM` quota.
- **Status**: open, by design.

### [Phase 26] eVAX `chf()`: SCB offsets as condition values, depths from 1, reversed arguments

- **Where**: `reference/eVAX/eVAX/Source/CPU/interrupt.c`'s `chf()`, ported
  as `internal/console/chf.go` (Phase 20).
- **What**: the C search puts the exception's SCB offset (`0x20` for an
  access violation) where VMS puts a condition value (`SS$_ACCVIO`),
  numbers the first frame's depth 1 where VMS numbers it 0, pushes an
  access violation's parameters as virtual address then reason mask
  (VMS's signal array has the mask first), searches no exception
  vectors, and halts the machine when no handler continues, where VMS's
  catch-all reports the condition and continues or exits the image by
  severity. Its handlers also run through nested console `CALL`s, and a
  continued exception ends the console's run loop.
- **Status**: fixed in Phase 26 subtask 31 by a VMS-style dispatcher in
  the RTL (`internal/rtl/condition.go`), which the engine offers every
  `console$handler` exception first. `chf.go` is kept, unchanged, as the
  fallback for exceptions the RTL declines (see the dispatcher's
  simplifications below).

### [Phase 26] Condition dispatch simplifications

- **Where**: `internal/rtl/condition.go`, `internal/cpu/handlefault.go`.
- **What**:
  - Only exceptions kernel.asm's SCB sends to `console$handler` are
    dispatched (access violation, translation not valid, privileged and
    reserved instructions, reserved operand and addressing mode,
    arithmetic). Breakpoint, trace, and compatibility-mode exceptions
    aren't signaled.
  - Every exception is still delivered as a fault: the signal array's PC
    is the faulting instruction's, even for the arithmetic *traps*
    (types 1-7), whose architected PC is the next instruction's. (The
    engine's ordinary exception frames do the same.) A handler that
    continues a trap must change the PC itself, or the instruction runs
    again.
  - An exception on the interrupt stack, or one whose stack can't be
    written, goes to the console's report instead of VMS's fatal-error
    handling.
  - The catch-all prints no traceback, and writes its message to the
    console terminal (VMS's goes to `SYS$ERROR` and `SYS$OUTPUT`).
  - On VMS the kernel dispatches an exception in the mode it happened
    in only after checking that mode's stack; govax pushes the arrays
    on whatever stack is current.
  - `LIB$SIGNAL`/`LIB$STOP` (subtask 33) put 0 in the mechanism array's
    R0, where VMS puts the caller's R0: the shim stub's
    `MOVL #code, R0` has replaced it before the shim runs. So a handler
    that continues without setting it makes `LIB$SIGNAL` return 0.
  - `$UNWIND` (subtask 34) from a vectored handler with no `depadr`
    does nothing: the default depth is the establisher's plus one, and a
    vectored handler has no establisher frame. `newpc` is taken as the
    resume address itself.
- **Status**: open, by design.

## Open findings

_None yet._

## Resolved findings

### [Phase 26] The assembler's `.ASCIC` has a 16-bit count

- **Where**: `internal/asm/pseudo.go` (`pseudoAscii`, `asciiCounted`),
  from `asm_pseudo.c`'s case 9.
- **What**: MACRO-32's `.ASCIC` stores a counted string: one byte of
  length, then the text. govax's assembler (like eVAX's) stores a
  16-bit length. A program passing an `.ASCIC` string to `$FAO`'s `!AC`,
  which reads the one-byte count VMS defines, gets a leading NUL and
  loses the string's last character. Found while writing subtask 24's
  fixture, which at first built its counted string with `.BYTE` and
  `.ASCII` instead. No fixture or boot file used `.ASCIC`.
- **Status**: fixed in Phase 26 (after subtask 30, at the user's
  request). `.ASCIC` stores a one-byte count, and a string longer than
  255 characters is `VAX_DATARANGE`; the HELP text says so, and
  `testdata/asm/fao.asm` now uses `.ASCIC` for its `!AC` string.

### [Phase 08/16] `validate_page`'s free-page search (and `mapped_pages`'s count) run one slot past the last real physical page

- **Where**: `reference/eVAX/eVAX/Source/Initialization/initialization.c`'s
  `alloc_vax` (~line 390): `mapsize = ( physmem >> 9 ) + 1;` — one more than
  the actual number of physical pages (`physmem >> 9`). `vm.c`'s
  `validate_page` (~line 1069, `for( n = 1; n < mapsize; n++ )`) and
  `mapped_pages` (~line 1106, the same loop bound) both search/count up to
  and including index `mapsize - 1`, i.e. `physmem >> 9` — a page index one
  past the highest real physical page (valid PFNs are `0 .. (physmem>>9)-1`).
- **What**: once every real physical page is claimed, `validate_page`'s
  search can still find `page_map[ physmem>>9 ] == 0` (an array slot that
  exists — `getmem(mapsize)` allocated the extra byte — but corresponds to
  no real page of RAM) and hand it out as a PFN. Downstream, `PTE.pfn <<
  9` for that PFN addresses one page past the end of `vax.memory[]`, an
  out-of-bounds access on every subsequent read/write through that page.
  This is exactly the kind of off-by-one the project's bug-fixing policy
  treats as a plain, ISA-unrelated slip (a `+1` that belongs on neither
  side of the loop bound it pairs with), not a hardware-definition choice —
  nothing in the VAX architecture calls for a "reserved, past-the-end"
  physical page here, and reproducing it in the Go port would have meant
  deliberately building a memory model that can be told to touch bytes
  beyond its own backing array.
- **Status**: fixed in Go, immediately, while porting `validate_page`/
  `mapped_pages`/`page_map` as part of adding real DYNVM demand-paging
  support (per user direction 2026-09-15: this port should treat DYNVM as
  its normal, only mode rather than the eager pre-mapping stand-in Phase 08
  originally used — see `internal/console/vminit.go`'s own updated doc
  comment). `internal/vm.Memory`'s `pageMap` is sized to exactly
  `Size()/pageSize` physical pages (no extra slot), and `AllocatePage`'s
  search — like `MappedPages`'s count — is bounded by `len(m.pageMap)`, so
  neither can ever produce a PFN outside real physical memory. Verified by
  `internal/vm/memory_test.go`'s `TestAllocatePage_exhausted` (a
  2-page Memory correctly reports exhaustion after its one allocatable page
  is claimed, rather than handing out a third, out-of-range page) and
  `TestMappedPages_countsReservedAndAllocatedButNotPageZero`.

### [Phase 14] `emulChmx` (this port's own CHMK/CHME/CHMS/CHMU handler) never updates `e.instructionPC`, so its handler's return address is the CHMx instruction's own start, not the instruction after it

- **Where**: `internal/cpu/changemode.go`'s `emulChmx`, as it stood before this
  fix -- not a C-reference porting gap (`reference/eVAX/eVAX/Source/CPU/
  interrupt.c`'s `emul_chmx` gets this right: `vax.instruction_PC = vax.PC;`
  runs immediately before its own `set_fault` call), but a Go-port-only
  regression from Phase 07's original CHMK implementation, which never
  carried that line over.
- **What**: every *other* synchronous fault this port raises relies on
  `Engine.Step`'s generic, before-decode `e.instructionPC = e.cpu.GPR(vax.PC)`
  (matching `decode_opcode.c`'s own `vax.instruction_PC = pc = vax.PC;`) --
  correct for those, since re-executing the same faulting instruction once
  whatever's wrong is fixed up (a resized stack, a paged-in page) is exactly
  the right behavior for an access violation or reserved-operand fault. CHMx
  is architecturally different: like a system call, it's expected to
  complete once and resume at the *next* instruction, not retry itself. By
  the time `emulChmx` runs, decode has already advanced `e.cpu.GPR(vax.PC)`
  past the whole CHMx instruction (opcode and operand) -- but `Engine.raise`
  (the fault path every `Handler` error goes through) unconditionally resets
  `e.cpu.GPR(vax.PC)` back to `e.instructionPC` before calling `HandleFault`,
  discarding that already-correct advance and pushing the CHMx instruction's
  *own* address as the saved return PC. A CHMK handler that completes and
  returns (RET or REI) therefore resumes execution *at the CHMK instruction
  itself*, re-trapping it -- invisible for a CHMK issued exactly once, but an
  infinite loop for any caller that issues CHMK a second time after the
  first one's handler returns, e.g. `kernel.asm`'s own `LIB$PUT_ONE`
  (`_loop: movb (r2)+,r5; chmk #EXE$PUT_CONSOLE; ...; sobgtr r6,_loop`) --
  every byte after the first re-triggers the *first* byte's own CHMK
  indefinitely, never advancing `r2`/`r5`/`r6`. Found while building Phase
  14's own `LIB$PUT_OUTPUT`-prints-a-multibyte-string end-to-end test (the
  first fixture in this project to ever complete one CHMK handler and then
  issue a second CHMK from the same call site) -- symptom was the console
  repeating only the *first* character of any multi-character string
  forever, not a step-cap hang, which is what led here.
- **Status**: fixed in Go. `emulChmx` now sets `e.instructionPC =
  e.cpu.GPR(vax.PC)` immediately before returning its `*Fault`, mirroring
  `emul_chmx.c`'s own explicit line. Verified by
  `internal/cpu/changemode_test.go`'s
  `TestEmulChmxReturnsPastTheChmxInstruction` (two back-to-back CHMKs, each
  with a trivial REI-based handler, confirming the second one actually
  executes rather than re-trapping the first) and, end to end, by
  `internal/console/asm_test.go`'s `TestAssemble_kernelThenHelloRunsBounded`.

### [Phase 14] `handle_fault` saves the stale `instruction_PC` as an interrupt's return address

- **Where**: `reference/eVAX/eVAX/Source/CPU/vax.c`'s `execute_vax` (~lines
  294-321), the `if (vax.interrupt_pending)` block, and `interrupt.c`'s
  `set_fault`/`handle_fault`, which push `vax.fault.pc = vax.instruction_PC`.
- **What**: `vax.instruction_PC` is only updated by `decode_opcode.c`'s
  `decode_instruction`, at the *start* of decoding an instruction. The
  interrupt-delivery block runs *before* `decode_instruction` is called for
  the upcoming instruction, so at that point `vax.instruction_PC` still holds
  the *previous* instruction's start address, while `vax.PC` has already been
  advanced (by that previous instruction's own decode) to the address that
  should actually resume once the ISR returns via REI. Concretely: a `MTPR
  TXDB` instruction that admits a console-transmit interrupt as its own side
  effect, followed by `MOVL #..., R0`, would push the `MTPR`'s own address as
  the return PC, not the `MOVL`'s — causing the `MTPR` (and hence the
  interrupt admission itself) to re-run after every REI, rather than
  resuming at the intended next instruction. This is a genuine architectural
  bug (interrupts are precise and must resume exactly where execution left
  off), not an ISA judgment call — real VAX hardware's whole distinction
  between interrupts (asynchronous, between instructions) and synchronous
  faults (mid-instruction, where `instruction_PC` *is* the right address) is
  exactly what this conflates.
- **Status**: fixed in Go, immediately (a clear-cut correctness bug with no
  ambiguity about the correct behavior, not a case needing deferral).
  `internal/cpu/interrupt.go`'s `deliverPendingInterrupt` sets
  `e.instructionPC` from the *current* PC (already advanced past whatever
  last executed) immediately before calling `HandleFault`, rather than
  leaving whatever `Step`'s previous decode left there. Verified by
  `internal/cpu/interrupt_test.go`'s
  `TestInterruptDeliverySavesCurrentPCNotStale`, which admits an interrupt as
  a one-instruction side effect and checks the pushed return PC is the
  *following* instruction's address, not the side-effecting instruction's
  own.

### [Phase 14] `poll_keyboard` (console-terminal receive interrupt) is a permanent no-op outside Mac/Windows builds

- **Where**: `reference/eVAX/eVAX/Source/CPU/vax.c`'s `poll_keyboard()`: the
  entire body is guarded by `#if defined(macintosh) || defined(WIN)`; on
  every other platform (including `LINUX86`, the platform macro
  `reference/CLAUDE.md` says the Xcode target actually builds with
  regardless of host OS) the function is empty and always returns 0.
- **What**: this is the only call site that ever sets `vax.RXDB`/`vax.RXCS`'s
  DON bit from a real keystroke, so on the reference build's own target
  platform, console-terminal *receive* interrupts (`EXC_CONREAD` via a real
  keypress, as opposed to the RXCS-write-triggered case in
  `emul_procreg.c`'s case 32) have never actually fired — `EXE$$GET_CONSOLE`
  (CHMK 2)'s unconditional `mfpr RXDB, r0` reads whatever stale value
  happens to be sitting in the register. Found while auditing this phase's
  RXCS/RXDB scope, per explicit user direction (2026-09-15) to fix this if a
  real opportunity presented itself, rather than reproduce it.
- **Status**: fixed in Go, differently from a literal port (there is no
  working behavior to replicate — the C reference's own mechanism never
  fires on its own reference platform). `internal/cpu/procreg.go`'s
  `emulMfpr` now pulls a real byte from `SystemServices.ConsoleReadByte`
  (already used by `XFC$CONSOLE_READ`) as an RXDB read's own side effect,
  making `EXE$$GET_CONSOLE`'s plain `mfpr RXDB, r0` genuinely return live
  console input instead of garbage. `Engine.DeliverConsoleByte`
  (`internal/cpu/interrupt.go`) separately ports what `poll_keyboard`'s
  *intended* (Mac/Windows) behavior actually does -- deposit a byte, set
  RXCS's DON bit, and admit an `EXC$CONREAD` interrupt when RXCS's IE bit is
  set -- as a real, callable primitive rather than a platform keyboard-hit
  poll, for any future caller with a real asynchronous byte source. Verified
  by `internal/cpu/procreg_test.go`'s RXDB read tests and
  `internal/cpu/interrupt_test.go`'s `DeliverConsoleByte` tests.

### [Phase 05] `emul_float_math.c`'s D-floating arithmetic path calls `fpu_load` with its source longwords swapped

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_float_math.c`'s `emul_float_math()`,
  the `dsize == 3` (D_FLOAT) branch of the basic-math `switch`:
  `rc = fpu_load( d[1], d[0], &float1 );` (and the identical pattern for `float2`).
- **What**: `fpu_load`'s signature is `fpu_load(LONGWORD src1, LONGWORD src2, double
  *result)`; the correct convention — confirmed empirically, not by tracing `fpu.c`'s
  byte-shuffle loops (see `docs/PHASE-05.md`'s design notes) — is `src1` = the
  operand's low-address (first) longword, `src2` = its high-address (second) longword.
  `emul_mov.c`'s `emul_movd` (MOVD/MNEGD) calls `fpu_load(data[0], data[1], &dbl)`,
  the correct order; `emul_float_math.c`'s D-floating math path passes them reversed.
  Verified with a standalone C harness built against the real `fpu.c`
  (`clang -DLINUX86 -I eVAX/Headers probe.c fpu.c`): storing `3.14159265358979` as
  D_floating and reading it back via `fpu_load(data[1], data[0])` (this bug's order)
  returns `-5.4687269079054549e-19`; via `fpu_load(data[0], data[1])` (the correct
  order) it returns the original value exactly. This corrupts every D-floating source
  read the C reference actually dispatches: `ADDD2/3`, `SUBD3`, `MULD2/3`, `DIVD2/3`,
  and `CVTBD`/`CVTWD`/`CVTLD`.
- **Status**: fixed in Go. `internal/cpu/fpu.go`'s `fpuLoad` is a single, from-scratch
  implementation used uniformly by every D-floating consumer (arithmetic, conversion,
  MOV/MNEG, compare/test, ACB) — there is only one calling convention in the Go port,
  and it's the confirmed-correct one, so this bug has no way to resurface per-caller
  the way it did across `emul_float_math.c`/`emul_mov.c`'s two separate C
  implementations. Verified by `internal/cpu/fpu_test.go`'s
  `TestFpuStoreDoubleFloatingKnownValues`/`TestFpuLoadRoundTrip` (D_floating cases)
  using the harness's own known-good raw bit patterns.

### [Phase 05] `fpu_store`'s underflow flush-to-zero is dead code

- **Where**: `reference/eVAX/eVAX/Source/CPU/fpu.c`'s `fpu_store()`, the underflow
  branch (`if( expon < -127 )`): `local = 0.0; expon = 0;` when `PSL<FU>` is clear.
- **What**: `local` is a separate `double` from `source` (the function's actual
  parameter); every bit-extraction line below this assignment reads through `pd`,
  which was set up earlier as `pd = (ULONGWORD *) &source` — aliasing the original,
  un-zeroed parameter, never `local`. The intended flush to `0.0` therefore has no
  effect on the function's output. Confirmed with the same standalone C harness used
  for the finding above: storing `1e-100` with `PSL<FU>` clear returns `rc == 0` (no
  fault, as expected) but a nonzero raw longword (`0xF977405F`, not `0x00000000`) —
  the original tiny value's mantissa bits with only the exponent field forced to a
  fixed value, not a flush to zero.
- **Status**: fixed in Go. `internal/cpu/fpu.go`'s `fpuStore` returns a genuine `0` on
  underflow with `PSL<FU>` clear, matching the explicit `value == 0` short-circuit one
  branch above it in the same function. Verified by `internal/cpu/fpu_test.go`'s
  `TestFpuStoreUnderflowFlushesToZeroWhenFUClear`.

### [Phase 05] Seven D-floating opcodes have no working operand data or dispatch in the C reference

- **Where**: `reference/eVAX/eVAX/Headers/instruction_table.h` (`SUBD2` at 0x62,
  `CVTDB`/`CVTDW`/`CVTDL`/`CVTRDL` at 0x68-0x6B, `CMPD`/`TSTD` at 0x71/0x73) and
  `reference/eVAX/eVAX/Source/Initialization/init_emulators.c`.
- **What**: `SUBD2`'s table row has `OperandCount: 0` despite correctly populated
  scale/access columns (a pure data-transcription slip — its siblings `MULD2`/`DIVD2`
  are correct) and _does_ have a dispatch entry (`instruction[0x62].routine =
  emul_float_math`), so decode would read it as a zero-operand instruction despite the
  handler expecting two. `CVTDB`/`CVTDW`/`CVTDL`/`CVTRDL`/`CMPD`/`TSTD` are worse: an
  all-zero table row (count, scale, _and_ access all zero) _and_ no
  `init_emulators.c` dispatch entry at all — genuinely never wired up, not just
  mis-tabled. `emul_cmp.c`'s shared handler (which `CMPD`/`TSTD` would need) has no
  `case 3` (D_FLOAT) in its `switch(dsize)` either, so even a hypothetical dispatch fix
  would have no working logic behind it. Confirmed by reading the C source directly
  (not inferred); severity (non-functional, not just subtly wrong, unlike Phase 04's
  BISB3 finding) surfaced to the user rather than decided unilaterally, since it cuts
  into this phase's own named D-floating deliverables.
- **Status**: fixed, per user direction. `internal/cpu/gen/main.go`'s
  `knownTableFixes` patches these seven entries at `go generate` time (so the fix
  survives regeneration rather than being hand-edited into the generated,
  DO-NOT-EDIT `instructions_table.go` and later silently reverted), mirroring each
  entry's already-correct F-floating/sibling D-floating row. Since five of the seven
  have no working C implementation to port at all, their instruction handlers
  (`internal/cpu/`) are implemented from the ISA manual and by direct analogy to the
  F-floating/D-floating siblings that _do_ work, not ported from C. Verified by
  `internal/cpu/instruction_test.go`'s `TestInstructionTableDFloatingFixedEntries`
  (table-level) and by each opcode's own handler tests as they land.

### [Phase 05] Floating ADD/SUB/MUL/DIV never clear V/C, and F_floating overflow silently swallows the fault

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_float_math.c`'s `emul_float_math()`,
  the basic-math `switch(dsize)` block (func 0-3).
- **What**: two related omissions. First, the function sets `vax.pslw.n`/`vax.pslw.z`
  from the result but never explicitly clears `vax.pslw.v`/`vax.pslw.c` on the normal
  path — the manual's ADDF/SUBF/MULF/DIVF (and D-floating counterparts) entry specifies
  `V <- 0, C <- 0`, the same "forgot to clear" pattern already found and fixed
  repeatedly elsewhere in this project via the `SETCONDITIONBITS(x, 0L)` idiom (Phase
  04), just without even that idiom's incidental C-clearing here. Second, and more
  serious: on F_floating overflow specifically, `fpu_store`'s fault return sets
  `vax.pslw.v = 1` but — unlike the D_floating branch two lines below it in the same
  `if`/`else`, which correctly does `if( rc ) return rc;` — falls through to
  `put_operand`, writing whatever `fpu_store` left in its (never actually populated on
  the fault path) destination longword and returning success instead of propagating
  the synchronous arithmetic fault. A real VAX floating overflow is always a
  synchronous exception; this is an inconsistency between two nearly-identical
  branches of the same conditional, not a considered design choice.
- **Status**: fixed in Go. `internal/cpu/floatmath.go`'s `setFloatPSL` always sets
  `V <- 0, C <- 0` on the surviving path (genuine overflow already faults before this
  runs); `storeFloatResult`/`storeFloat`/`fpuStore` propagate a real `*Fault` uniformly
  for both F_floating and D_floating, with no special-casing that could reintroduce
  the asymmetry. Verified by `internal/cpu/floatmath_test.go`'s
  `TestEmulFAddZeroSetsZ` (C explicitly primed dirty beforehand) and
  `TestEmulFAddOverflowFaults`.

### [Phase 05] CVTRFL/CVTRDL (round-to-nearest float→long) always fault instead of rounding

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_float_math.c`'s `emul_float_math()`,
  the `func == 4` (float→integer) handler's `switch( dsize2 )`.
- **What**: the switch only has cases for `0` (Byte), `1` (Word), and `2` (Long);
  `dsize2 == 3` — `op & 0x03 == 3`, which is exactly `CVTRFL`/`CVTRDL` — falls to
  `default: set_fault( EXC_PRIV, 0 ); return VAX_FAULT;`. `CVTRFL`/`CVTRDL` are
  correctly tabled and dispatched (this is a gap in the shared handler's own switch,
  unlike this doc's D-floating table/dispatch finding above), so every use of either
  instruction in the C reference faults as a reserved/privileged-instruction violation
  rather than rounding — there is no working "truncate instead of round" behavior to
  preserve either, it simply never executes.
- **Status**: fixed in Go. `internal/cpu/cvtfloat.go`'s `emulCvtRoundFloatToInt`
  implements real round-to-nearest (ties away from zero, via `math.Round`, applied
  before the overflow bounds check since rounding can itself push an in-range value
  out of range) — implemented fresh from the manual's `CVTRFL`/`CVTRDL` entries, not
  ported from C. Verified by `internal/cpu/cvtfloat_test.go`'s
  `TestEmulCvtRoundVsTruncate` (rounding vs. truncation, including a tie case and a
  negative-direction case) and `TestEmulCvtRoundFloatToIntOverflow` (a value that only
  overflows after rounding).

### [Phase 03] Autoincrement Deferred (`@(Rn)+`) eagerly loads the operand's value instead of resolving its address

- **Where**: `reference/eVAX/eVAX/Source/CPU/decode_operand.c`, `decode_operand()`'s
  `case 0x09` (~lines 458-479), ported to `internal/cpu/operand.go`'s
  `decodeGeneral`.
- **What**: For every other deferred addressing mode in this function — byte/word/long
  Displacement Deferred (`case 0x0B/0x0D/0x0F`, ~lines 493-551) and the PC-relative
  Byte/Word/Long Relative Deferred forms in `decode_opcode.c`'s PC-mode branch — the
  pattern is consistent: compute a pointer address, dereference it _once_ to get the
  operand's real address, and store that address in `opcode->VAXaddr[n]` for
  `get_operand`/`put_operand` to read or write through at execute time (respecting
  `OP_WR`/`OP_MD` write-back). `case 0x09` (Autoincrement Deferred, `@(Rn)+`) breaks
  this pattern: it dereferences the pointer, then immediately dereferences the _result_
  a second time (`load_register(treg, vax.reg[reg], 4)` then
  `load_register(treg, vax.reg[treg], 4)`) and stores the loaded value's address via
  `opcode->address[n]` (the "already-resolved, no execute-time indirection needed"
  path) instead of `VAXaddr[n]`. This is inconsistent with the addressing mode's own
  definition (`Rn` holds the address of a longword containing the operand's address —
  one level of indirection, not two) and, more importantly, would silently break any
  instruction that _writes_ through this mode: `put_operand` would write into the
  scratch register `get_operand` eagerly loaded into, not back to the real VAX memory
  location the pointer addressed, e.g. `CLRL @(R0)+` or `MOVL R1,@(R2)+` as a
  destination would discard the write.
- **Status**: fixed in Go. `decodeGeneral`'s `case 0x09` dereferences the pointer once
  and resolves an `OperandMemory` operand (`Addr` = the dereferenced value), exactly
  mirroring the other deferred modes' pattern — internally consistent with sibling code
  in the same function, and the natural behavior of this port's value-based `Operand`
  type in the first place (see `docs/PHASE-03.md`'s design notes: nothing in this
  design _has_ a "scratch register to eagerly load into" the way the C source's
  pointer-aliasing mechanism does, so reproducing the bug would have taken deliberately
  added complexity, not less). Verified by
  `internal/cpu/operand_test.go`'s `TestDecodeOperandAutoincrementDeferred`.

### [Phase 03] Indexed-mode-nested-in-Indexed-mode rejection doesn't route through the normal fault mechanism

- **Where**: `reference/eVAX/eVAX/Source/CPU/decode_operand.c`, `case 0x04` (~lines
  403-431): `if (idx) return VAX_ILLADDRFAULT;`.
- **What**: The VAX architecture reserves Index mode as the base addressing mode of
  another Index mode operand specifier (a reserved addressing mode fault,
  `EXC_RESADDR`). The C source clearly intends to reject this — it's the only
  addressing-mode-structural check in the function — but signals it with
  `VAX_ILLADDRFAULT`, a raw return-code sentinel that never touches `vax.fault`/
  `set_fault` the way every other decode-time fault in this file does (e.g. the
  short-literal-write-access check just above it, which does call `set_fault`). Its
  caller (`execute_vax` in `vax.c`) unconditionally calls `handle_fault()` on any
  nonzero decode result, which reads `vax.fault.code` — left stale from whatever fault
  (if any) last set it, not this one. The clearly-intended behavior (reject with
  `EXC_RESADDR`) doesn't actually reach the exception vector it should.
- **Status**: fixed in Go. `decodeGeneral`'s `case 0x04` returns
  `&Fault{Code: ExcReservedAddr}` directly through this port's normal fault-return
  path (the same one every other decode-time fault uses), rather than a disconnected
  sentinel — this is fixing broken plumbing behind an already-clear intent, not
  second-guessing an ISA judgment call. Verified by
  `internal/cpu/operand_test.go`'s `TestDecodeOperandDoubleIndexedFaults`.

### [Phase 03] `set_mode_stack` sets `MAPEN = 1` on every non-interrupt-stack mode switch

- **Where**: `reference/eVAX/eVAX/Source/CPU/interrupt.c`, `set_mode_stack()`
  (~line 481): `vax.MAPEN = 1;        /* Not sure about this!! */`.
- **What**: the C source's own comment flags this write as uncertain — every switch to
  a KSP/ESP/SSP/USP-based mode (as opposed to the interrupt stack, which explicitly
  sets `MAPEN = 0`) unconditionally forces virtual memory _on_, regardless of whatever
  `MAPEN` held before the exception. This isn't obviously wrong (kernel-mode exception
  handlers on a running VMS-like OS would have VM enabled anyway), but it's also not
  obviously right for early boot / console-level fault handling before VM is set up,
  and the original author didn't resolve it either.
- **Status**: deliberately kept — replicated as-is in `Engine.setModeStack`
  (`internal/cpu/handlefault.go`), per this project's policy of not second-guessing an
  ISA judgment call the original author explicitly marked as unresolved. Re-checked
  at Phase 12 (this doc's own named revisit point): VM-disabled fault handling still
  hasn't been exercised end to end by anything in this port's test suite or the
  fixture-driven regression pass (VMINIT always leaves `MAPEN=1` set from the moment
  it runs), so there's still no concrete scenario to weigh this against. Staying
  deferred; revisit only if one actually comes up.

### [Phase 02, noticed in Phase 03, resolved Phase 12] `vm.TranslationFault` collapses `EXC_ACCVIO`'s two distinct signal subcodes

- **Where**: `reference/eVAX/eVAX/Source/CPU/vm.c`'s `vm()` (~lines 423-503): a region
  length/base violation signals `set_fault(EXC_ACCVIO, 2, addr, 0x0001)`, a protection
  violation signals `set_fault(EXC_ACCVIO, 2, addr, 0x0002)` — the second signal
  argument distinguishes *why* the access violation happened. `internal/vm/translate.go`
  (Phase 02) has a single `AccessViolation` `FaultKind` for both cases (see its
  `accessViolation` helper), losing that distinction.
- **What**: Phase 02 predates fault-signaling entirely (`docs/PHASE-02.md` explicitly
  left `set_fault`'s translation to Phase 03), so this wasn't a fidelity question until
  now — `wrapMemError`'s `vm.TranslationFault` → `cpu.Fault` mapping (`internal/cpu/
  dispatch.go`, called from `Engine.raise` in `internal/cpu/engine.go`) has no subcode
  to recover and reports `0x0001` (length/base violation) unconditionally for every
  `AccessViolation`, matching the more common of the two C call sites (and the same
  subcode `decode_opcode.c`/`decode_operand.c`'s own inlined physical-address-
  resolution faults already use).
- **Status**: fixed in Go, in Phase 12. `internal/vm/translate.go`'s `FaultKind` now
  has a distinct `ProtectionViolation` alongside `AccessViolation` (a new
  `protectionViolation` helper, used only by the one protection-check call site;
  every length/base-violation call site is unchanged); `wrapMemError`
  (`internal/cpu/dispatch.go`) reports subcode `0x0002` for `ProtectionViolation` and
  `0x0001` for `AccessViolation`, recovering the real distinction instead of always
  reporting the length/base subcode. Verified by `internal/vm/translate_test.go`'s
  updated `TestTranslateProtectionViolation` (now asserts `ProtectionViolation`, not
  `AccessViolation`) and `internal/cpu/dispatch_test.go`'s new
  `TestWrapMemErrorProtectionViolation`.

### [Phase 04] Register mode used where `OP_AD`/`OP_VA` access is required

- **Where**: `reference/eVAX/eVAX/Source/CPU/decode_operand.c`'s `decode_operand()`
  never rejects Register mode (`mode == 5`) for an operand whose access is `OP_AD`
  (e.g. `MOVAL`/`PUSHAL`, `JMP`/`JSB`) or `OP_VA` — a register has no VAX address. A few
  individual C handlers (`emul_mova.c`, half of `emul_push.c`) work around this ad hoc
  by testing `opcode->is_register[0]` themselves and faulting `EXC_RESADDR`, but decode
  itself never enforces it, so any future `OP_AD`/`OP_VA` consumer that forgets the
  check would silently accept an illegal encoding.
- **What**: per user direction (explicitly requested when starting Phase 04), this
  should fault, and generally rather than per-handler.
- **Status (Phase 04)**: fixed in Go, at decode time. `decodeOperand`'s register-mode
  fast path (`internal/cpu/operand.go`) returned `&Fault{Code: ExcReservedAddr}` for any
  `AccessAddress`/`AccessVarField` operand that resolves to Register mode — one fix
  covering every current and future `OP_AD`/`OP_VA` consumer (Phase 04's
  `MOVAx`/`PUSHAx`/`JMP`/`JSB` then, Phase 06's bitfield `OP_VA` operands later) rather
  than a check repeated in each handler.
- **[Phase 12] reverted**: this "fix" was itself wrong. Phase 12's first real
  end-to-end run of `testdata/asm/kernel.asm` (this project's own hand-written
  microkernel, assembled and executed for real via the new `ASM`/`CALL` console
  commands — never previously exercised this way) found that its CHMK dispatcher's
  `callg ap, (r0)` genuinely depends on `emul_call.c`'s own `is_register[0]` branch
  (`new_ap = vax.reg[opcode->regnum[0]]` — using the register's own *value* as the
  arglist address, a real tail-call idiom: the dispatcher reuses its caller's own AP
  register directly rather than rebuilding an arglist in memory) — confirmed not just
  by reading the C source but by the CPU jumping to a wild, garbage PC once decode's
  blanket reject was defeated by a *different* bug (see the assembler indexed-mode
  finding below) and this one was reached. `internal/cpu/bitfield.go`'s `loadField`/
  `storeField` were *also* already written expecting a register-mode base to be legal
  (a real, defined VAX feature for that instruction family — the field spans adjacent
  registers rather than memory — not a reserved encoding at all), and were silently
  unreachable in that shape for the same reason. Given two of the three affected
  consumers turned out to need the old behavior for real, reverted per user direction
  (2026-09-15) rather than narrowed to just CALLG: decode
  (`internal/cpu/operand.go`) no longer rejects Register mode for `AccessAddress`/
  `AccessVarField` at all (matching `decode_operand.c` exactly — it never did either).
  `MOVAx`/`PUSHAx` (`internal/cpu/mova.go`) now self-check `Operand.Kind ==
  OperandRegister` and fault, mirroring `emul_mova.c`/`emul_push.c`'s own
  `is_register[0]` checks; `CALLS`/`CALLG` (`internal/cpu/call.go`'s `emulCall`) uses
  the register's value as the new AP when its arglist operand is Register mode,
  mirroring `emul_call.c`. `JMP`/`JSB` never had an `is_register` check in the C source
  either (nor a meaningful fallback — `VAXaddr[n]` is simply left stale/uninitialized
  for that case), so they're deliberately left with no check here too, matching that
  gap rather than inventing a fallback the reference never had; no current fixture
  exercises a register-mode JMP/JSB operand. Verified by
  `internal/cpu/operand_test.go`'s `TestDecodeOperandAccessAddressAllowsRegisterMode`
  (decode no longer faults), `TestEmulMovaRegisterModeFaults`/
  `TestEmulPushaRegisterModeFaults` (mova_test.go, self-check still faults), and
  `TestEmulCallgRegisterArglistUsesRegisterValue` (call_test.go, the restored
  behavior) — plus, end to end, `internal/console/asm_test.go`'s
  `TestAssemble_kernelThenHelloRunsBounded`, which now gets past this exact
  instruction instead of faulting there.

### [Phase 04] `emul_movb_negated`'s stray extra `vax.pslw.v = 0` discards MNEGB's overflow flag

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_mov.c`'s `emul_movb_negated()`
  (~line 172): `vax.pslw.v = 0;` runs unconditionally right after the `n`/`z` are set,
  a few lines after the MNEGB overflow branch already set `vax.pslw.v = 1`.
  `emul_movw_negated`/`emul_movl_negated` don't have this extra line.
- **What**: a clear copy-paste bug, not an ISA judgment call — MNEGB can never actually
  report overflow in the C source, while MNEGW/MNEGL (same shape, same file) do.
- **Status**: fixed in Go. `emulMneg` (`internal/cpu/mov.go`), shared across
  MNEGB/MNEGW/MNEGL, sets V once and doesn't clobber it. Verified by
  `internal/cpu/mov_test.go`'s `TestEmulMneg/overflow_(largest_negative)`.

### [Phase 04] MNEGx's carry flag uses "source LSS 0" instead of the manual's "source NEQ 0"

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_mov.c`'s `emul_movb_negated`/
  `emul_movw_negated`/`emul_movl_negated`, the MNEG branch: `vax.pslw.c = (data < 0) ?
  1 : 0;` where `data` is the *original* source value.
- **What**: `vax_instr_set.pdf`'s MNEG entry defines `C <- dst NEQ 0`. Since negation
  maps zero to zero and nothing else to zero (including the overflow case, which
  leaves `dst` as the unchanged nonzero source), `dst NEQ 0` is equivalent to `source
  NEQ 0` — true for *any* nonzero source, not just a negative one. The C source's
  formula agrees only when the source is already negative or zero; for a positive
  source (e.g. negating 5) the manual says C should be 1 (0 − 5 borrows) but the C
  source computes 0.
- **Status**: fixed in Go. `emulMneg` (`internal/cpu/mov.go`) sets `C` from `source !=
  0`. Verified by `internal/cpu/mov_test.go`'s `TestEmulMneg` table (`"positive"` and
  `"negative"` cases both expect `C=true`).

### [Phase 04] The `SETCONDITIONBITS(x, 0L)` idiom incidentally forces C false and skips V

- **Where**: several `emul_*.c` handlers call `SETCONDITIONBITS(data, 0L)` purely to
  get N/Z ("is the result negative/zero") as a side effect of the macro's two-operand
  compare shape (`vax.h`'s `SETCONDITIONBITS`). Confirmed instances in this phase's
  scope: `emul_mov.c`'s `emul_movq`, `emul_ash.c`'s `emul_rotl`/`emul_ash` (both ASHL
  and ASHQ), and `emul_loop.c`'s `emul_aobleq`/`emul_aoblss`/`emul_sobgtr`/
  `emul_sobgeq`.
- **What**: the macro's C formula (`(ULONGWORD)(v1) < (ULONGWORD)(v2)`) is always false
  when `v2` is a literal 0 (nothing is less than unsigned zero), and the macro never
  touches V at all. `vax_instr_set.pdf` specifies `C <- C` (unchanged) for every one of
  these instructions, and a real V formula for ROTL (`V <- 0`, matching what the macro
  accidentally produces) and ASHL/ASHQ/the four loop instructions (`V <- {integer
  overflow}`, never computed by the C source at all — V is left as whatever the
  previous instruction set it to).
- **Status**: fixed in Go, all eight instances. N/Z computed explicitly, V computed per
  the manual's overflow definition where one applies (0 for MOVQ/ROTL; a real
  shift/increment overflow check for ASHL/ASHQ/AOBLEQ/AOBLSS/SOBGTR/SOBGEQ, the latter
  four reusing `addResult`/`subResult` since they're arithmetically exactly
  INCL/DECL), C left untouched throughout. MOVQ: `internal/cpu/mov.go`. ROTL/ASHL/ASHQ:
  `internal/cpu/ash.go`. AOBLEQ/AOBLSS/SOBGTR/SOBGEQ: `internal/cpu/loop.go`.

### [Phase 04] BIT's carry flag is force-cleared instead of left unchanged

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_cmp.c`'s `emul_cmp()`: `cbit` is
  initialized to 0 and only ever reassigned in the CMP case (`func == 0`); BIT and TST
  both fall through with `cbit == 0`, and `vax.pslw.c = cbit;` runs unconditionally
  after the switch.
- **What**: `vax_instr_set.pdf`'s BIT entry specifies `C <- C` (unchanged); its TST
  entry specifies `C <- 0`, which the C source gets right. Only BIT is wrong.
- **Status**: fixed in Go. `internal/cpu/cmp.go`'s `emulBit` uses `setLogicalPSL` (the
  same BIS/BIC/XOR-shaped "N/Z from result, V <- 0, C unaffected" helper from
  `internal/cpu/integermath.go`) instead of reusing CMP's C computation; `emulTst`
  explicitly sets both V and C to 0, matching the C source's (already-correct) TST
  behavior. Verified by `internal/cpu/cmp_test.go`'s `TestEmulBitLeavesCarryUnaffected`.

### [Phase 04, resolved Phase 12] ADWC/SBWC operate on word operands; the manual specifies longword

- **Where**: `reference/eVAX/eVAX/Headers/instruction_table.h`'s ADWC/SBWC entries
  (operand scale `{2, 2, ...}`, i.e. word) and `emul_integer_math.c`'s `emul_integer_
  math()`, which special-cases `op == 0xD8`/`0xD9` to `dsize = 5` (word) rather than
  falling through to the longword (`dsize == 6`) case its opcode range would otherwise
  select.
- **What**: `vax_instr_set.pdf`'s ADWC format line reads `add.rl, sum.ml` — longword
  operands (`.rl`/`.ml`), not word.
- **Status**: fixed in Go, in Phase 12, per user direction. `internal/cpu/gen/main.go`'s
  `knownTableFixes` patches both rows to `{4, 4, ...}` (the same mechanism already
  used for the D-floating/REMQUE/REMQHI/REMQTI table fixes), so `emulAdwc`/`emulSbwc`
  (`internal/cpu/integermath.go`) — which already derived their operand width
  generically from the table via `loadPair` — now operate at longword width with no
  handler code change needed. Verified by `internal/cpu/integermath_test.go`'s new
  `TestEmulAdwcOperatesOnLongwords` (a value that only produces the correct result at
  32 bits, not 16).

### [Phase 04] `emul_increment.c`/`emul_integer_math.c`'s N/Z/V/C formulas are broken for byte and longword sizes, and for BIS/BIC's C

This entry originally read "deferred" — logged below is the superseded reasoning,
followed by what actually shipped. Kept for the record rather than silently rewritten.

**Superseded plan**: the first pass through `emul_increment.c` found that its byte/word
carry computation reads the source as a *signed* `char`/`short` and tests
`data & 0x100`/`0x10000` for carry-out, which only matches the manual's "carry from the
most significant bit" when the operand's sign bit is clear (e.g. incrementing byte 0xFF:
C source computes `d1 = -1`, `data = 0`, reports C=0, but the true unsigned carry
(255+1=256) should set C). This looked like a pervasive, hard-to-fix-safely pattern
across INC/DEC and ADD/SUB/ADWC/SBWC, so the plan was to replicate it as-is and defer a
proper fix to Phase 12.

**What changed**: while deriving INC/DEC's overflow formula from the manual's own
explicit notes ("integer overflow occurs if the largest positive integer is
incremented"), a wider check of `emul_integer_math.c`'s V/C formulas against
`vax_instr_set.pdf`'s ADD/SUB/MUL/DIV/BIS/BIC entries turned up more, and worse,
confirmed defects than the carry-flag one alone:

- **Byte-size V range check uses the wrong constants.** `emul_increment.c`/
  `emul_integer_math.c`'s byte case tests `data > 255 || data < -256` for overflow. The
  analogous word case correctly tests `data > 32767 || data < -32768` — exactly
  `INT16_MAX`/`INT16_MIN`, the true signed-word bounds. The byte case's bounds (255,
  -256) are not the signed-byte bounds (127, -128) at all; concretely, incrementing byte
  0x7F (127, the largest positive byte) computes `data = 128`, and `128 > 255` is false,
  so no overflow is reported — but incrementing the largest positive integer is the
  textbook overflow case the manual's own INC note calls out by name. Comparing the
  (correct) word case to the (wrong) byte case in the same function makes this a clear,
  obvious constant error, not an ISA judgment call.
- **The longword V check via `data == udata` cannot detect anything, ever, now that
  `LONGWORD` is genuinely 32-bit.** This "compute the same add as both signed and
  unsigned, overflow iff they disagree" trick only works if the comparison happens in
  *wider* precision than the operands — which is exactly what the pre-audit-fix 64-bit
  `LONGWORD` accidentally provided. Post-fix, both `data` (`LONGWORD`) and `udata`
  (declared `LONGWORD` in `emul_increment.c`, `ULONGWORD` in `emul_integer_math.c`) are
  computed at the *same* 32-bit width as the inputs; two's-complement addition produces
  an identical bit pattern whether the intermediate type is signed or unsigned, so
  `(ULONGWORD)data == udata` is a tautology — always true, always reporting "no
  overflow." Verified concretely: `d1 = 0x7FFFFFFF`, `d2 = 1` — `data` and `udata` both
  come out `0x80000000`, so the check reports no overflow for `INT32_MAX + 1`, a
  textbook signed overflow. This is a real, confirmed regression: the
  `reference/eVAX/AUDIT.md` `LONGWORD`-width fix that closed the C project's own audit
  silently broke this particular overflow-detection idiom everywhere it's used at
  longword size, since the idiom was never widening on purpose — it was relying on
  `LONGWORD` accidentally already being wider than a VAX longword.
- **BIS's C is force-cleared, and BIC's uses the (already-broken) arithmetic carry
  formula.** `emul_integer_math()`'s byte/word cases special-case `func == 4` (BIS) to
  `vax.pslw.c = 0` and fall through to the generic (broken) `data & 0x100` computation
  for `func == 5` (BIC); the manual specifies `C <- C` (unchanged) for both — they're
  bitwise ops, carry has no meaning for them at all.
- **DIV's V doesn't cover the `MinInt / -1` overflow case** (only division-by-zero),
  and dividing by zero unconditionally before the `d1 == 0` check is undefined behavior
  in C (and a runtime panic in Go — the reason this file was touched in the first place;
  see below).

None of this needed guessing at ISA intent — every formula above (`ADD`'s "same-sign
operands, different-sign result", `SUB`'s "borrow" and "different-sign operands,
result differs from the minuend's sign", `MUL`'s "product doesn't fit the destination",
`DIV`'s "divisor is zero, or `MinInt / -1`", `BIS`/`BIC`'s "C unaffected") is stated
explicitly in `vax_instr_set.pdf`'s Condition Codes section for that instruction, and Go
has native 64-bit arithmetic to implement each one *correctly* by computing in wider
precision and checking for truncation — the exact technique the C source's own longword
path was reaching for and structurally couldn't achieve. Given that, and given that
INCx/DECx are specified as exactly equivalent to `ADDx S^#1`/`SUBx S^#1` (so shipping
INC with a correct formula while ADD keeps the broken one would make two opcodes
computing the identical operation disagree on their own condition codes), this was
reclassified from "defer, pervasive and risky" to "fix now, per-instruction, using
spec-derived formulas" — the same bar already used for MNEG's overflow/carry fixes in
sub-phase 2.

- **Status**: fixed in Go. `internal/cpu/condcodes.go` gained generic, size-parameterized
  `addResult`/`subResult`/`mulResult`/`divResult` helpers (wide-arithmetic overflow/carry
  per the formulas above); `internal/cpu/increment.go` (sub-phase 5) and
  `internal/cpu/integermath.go` (sub-phase 6) use them for INC/DEC and
  ADD/SUB/MUL/DIV/BIS/BIC/ADWC/SBWC respectively, replacing every one of the narrow/
  tautological/wrong-constant formulas described above. Divide-by-zero and
  `MinInt / -1` are guarded before the actual Go division (which would otherwise panic)
  rather than left as C's undefined behavior; see each sub-phase's progress log entry
  for the specific tests.

### [Phase 04, resolved Phase 12] BISB3's destination operand is declared longword-sized

- **Where**: `reference/eVAX/eVAX/Headers/instruction_table.h`'s `BISB3` entry
  (~line 1524-1533): operand scales `{1, 1, 4, 0, 0, 0}` — the third (destination)
  operand is 4 bytes. Every sibling instruction of the identical `Bxx3` shape (ADDB3,
  SUBB3, MULB3, DIVB3, BICB3, XORB3) correctly declares all three operands as
  byte-sized (`{1, 1, 1, ...}`).
- **What**: this looks like a plain transcription error in the reference table (a
  stray `4` where every neighboring entry has `1`), not a deliberate design choice —
  there's no ISA reading under which BISB3 alone would have a wider destination than
  BISB2 or its own siblings. Concretely, `BISB3 mask,src,Rn` with a register
  destination overwrites all 4 bytes of `Rn` (the byte OR result zero-extended)
  instead of only the low byte the way every other `Bxx3`/`Bxx2` form does.
- **Status**: fixed in Go, in Phase 12, per user direction, using the same
  `knownTableFixes` mechanism as the ADWC/SBWC finding above: the destination
  operand's scale is patched to `1`, matching every sibling `Bxx3` form.
  `internal/cpu/integermath.go`'s handlers already used each operand's own declared
  size generically (no special-casing), so no handler change was needed. Verified by
  `internal/cpu/integermath_test.go`'s `TestEmulBisb3DestinationScaleFixed` (renamed
  from `TestEmulBisb3DestinationScaleDeviation`, now asserting only the low byte of a
  register destination is written).

### [Phase 04] CVTxy computes N/Z from the source value instead of the truncated destination

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_integer_cvt.c`'s `emul_integer_cvt()`:
  `SETCONDITIONBITS(data, 0L)` runs right after `data` is read and sign-extended at the
  *source* size, before the second `switch` that truncates it to the destination size
  and writes it out. Its byte-destination V check also uses the same wrong constants
  (`255`/`-256` instead of `127`/`-128`) already found in `emul_increment.c`/
  `emul_integer_math.c`.
- **What**: `vax_instr_set.pdf`'s CVT entry specifies `N <- dst LSS 0`, `Z <- dst EQL 0`
  — the *destination* (post-truncation) value. These only disagree when the conversion
  overflows, but then they can disagree outright: converting long `0x00000080` (128,
  positive) to byte truncates to `0x80` (-128, negative) — the manual's N is true, the
  C source's is false.
- **Status**: fixed in Go, using the same wide-arithmetic approach as the other
  integer-arithmetic findings in this phase. `internal/cpu/condcodes.go`'s
  `convertResult` sign-extends the source and truncates to the destination size,
  returning the masked result and a truncation-based V (the manual's own definition:
  "any truncated bits not equal to the sign bit of the destination"); `internal/cpu/
  cvt.go`'s `emulCvt` computes N/Z from that result, not the pre-truncation source.
  Verified by `internal/cpu/cvt_test.go`'s `TestEmulCvtOverflowNZFromDestination` (the
  sign-flip case) and `TestEmulCvtByteOverflowRangeCheck` (the byte constant fix).

### [Phase 04] ACB's branch condition uses strict `<` where the manual specifies `<=`

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_branch.c`'s `emul_acb()`, the
  non-negative-addend branch condition (present identically in all four size
  variants): `if( addend_b >= 0 && index_b < limit_b ) branch = 1;`.
- **What**: `vax_instr_set.pdf`'s ACB entry: "If the addend operand is positive (or
  zero) and the comparison is less than or equal to zero [index <= limit] ... the
  branch displacement is added to the PC." The C source's strict `<` misses the
  boundary case where a counting-up loop's index lands exactly on the limit on its
  final iteration — e.g. `ACBL #10, #1, index, loop` with `index` reaching exactly
  10 fails to branch, silently dropping the last iteration. The negative-addend
  condition (`index >= limit`) was already correct — inclusive, matching the manual's
  "greater than or equal to zero."
- **Status**: fixed in Go. `internal/cpu/branchacb.go`'s `emulAcb` uses `<=` for the
  non-negative-addend case. Verified by `internal/cpu/branchacb_test.go`'s
  `TestEmulAcbPositiveAddendBoundary` (an index landing exactly on the limit) and
  `TestEmulAcbNotTaken` (confirming the loop still correctly exits once past it).
- **[Phase 05] update**: the same bug is present, identically, in `emul_acb.c`'s
  `ACBF` case (the one floating ACB variant the C reference actually implements —
  `case 0x4F`). Fixed the same way in `internal/cpu/branchacbfloat.go`'s
  `emulAcbFloat`, shared by both `ACBF` and the freshly-implemented `ACBD` (no C
  reference exists for `ACBD` at all — see this file's D-floating table/dispatch
  entry). Verified by `internal/cpu/branchacbfloat_test.go`'s
  `TestEmulAcbFloatPositiveAddendBoundary`/`TestEmulAcbFloatNotTaken`.

### [Phase 06] `get_register_field`/`set_register_field` split a cross-register field one bit short of the base register

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_bitfield.c`'s `get_register_field()`
  and `set_register_field()` (identical bug in both): the single-vs-two-register
  branch tests `position + size < 32` (so `position + size == 32` — a field that
  exactly fills the base register — wrongly takes the two-register path), and the
  two-register branch computes `size2 = (position + size) - 31` / `size3 = size -
  size2`, using `31` where the register width `32` belongs.
- **What**: a field genuinely spanning `base` and `base+1` should take `32 -
  position` bits from `base` and the remaining `size - (32 - position)` bits from
  `base+1`. The C source's `-31` constant is one bit too low: e.g. `position=30,
  size=4` should take 2 bits from `base` (bits 30-31) and 2 from `base+1` (bits 0-1),
  but the C formula computes `size2 = 3, size3 = 1` — 1 bit from `base`, 3 from
  `base+1`, silently misplacing every bit at and above the split point. This isn't an
  ISA judgment call — the manual's cross-register field layout is unambiguous, and
  the C source's own single-register branch already uses the correct `32` implicitly
  (by using plain `<< position`/`>> position` against a 32-bit register) — it's a
  transcription slip between two related magic numbers in the same function, the
  same class of "clear, obvious constant error" already fixed elsewhere in this
  project (e.g. Phase 04's byte-size V-range-check finding).
- **Status**: fixed in Go. `internal/cpu/bitfield.go`'s `getRegisterField`/
  `setRegisterField` split at `loBits := 32 - int(position)` (single-register
  whenever `size <= loBits`, i.e. inclusive of the exact-fit case) rather than
  porting the C formula. Verified by `internal/cpu/bitfield_test.go`'s
  `TestGetRegisterField` ("exactly fills the base register" and "spans base and
  base+1" subtests, the latter checked against hand-computed expected nibbles) and
  `TestSetRegisterField`'s round-trip subtest.

### [Phase 06] `emul_cmpc5`'s outer length gate skips fill-padding for a one-sided zero-length string

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_cmpc.c`'s `emul_cmpc5()`: the dual-
  string compare loop *and both* fill-padding loops are nested inside a single `if
  (tmp1 > 0 && tmp3 > 0)`.
- **What**: the manual describes CMPC5 as comparing two strings after conceptually
  extending the shorter one with the fill byte — which must include the case where
  one string's length is exactly 0 (an empty string, entirely fill). The outer gate
  skips all three loops whenever *either* length starts at 0, so e.g. `src1len=0,
  src2len=3` never compares string2 against fill at all — only the fallback
  length-vs-length condition-code default applies, not an actual fill comparison.
  The structurally identical `emul_movc5()` (same file) doesn't have this outer
  gate — its fill loops are each guarded only by their own remaining length — making
  this look like a stray extra condition added to one twin but not the other, not a
  deliberate design choice.
- **Status**: fixed in Go. `internal/cpu/cmpc.go`'s `emulCmpc5` drops the outer gate,
  each of the three loops using only its own natural condition, mirroring
  `emulMovc5`'s already-correct structure. Verified by `internal/cpu/cmpc_test.go`'s
  `TestEmulCmpc5/one-sided_zero_length_is_still_fill-padded`.

### [Phase 06] `emul_scanc`'s Z-bit polarity is backwards for both SCANC and SPANC

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_movc.c`'s `emul_scanc()` (shared by
  both SCANC and SPANC): `vax.pslw.z` is initialized to `0` and set to `1` only when
  a match is found (`if ((test && ch) || (!test && !ch)) { vax.pslw.z = 1; break; }`).
- **What**: both instructions' manual entries state the opposite verbatim — SCANC:
  "If a nonzero AND result is detected, the condition code Z-bit is cleared;
  otherwise, the Z-bit is set"; SPANC: the same with "zero" in place of "nonzero".
  I.e. Z should be *cleared* when a match is found and *set* when the string is
  exhausted without one — exactly backwards from what the C source does. This also
  produces a second, separately-documented symptom: both instructions' own Notes
  state a zero-length string should set Z ("just as though the entire string were
  scanned/spanned" — the no-match outcome), but a zero-length string never enters
  the loop at all, so the C source's un-fixed `z = 0` default leaves Z clear instead.
- **Status**: fixed in Go. `internal/cpu/scanc.go`'s `emulScanc` sets `Z <- !found`
  (found = a match was located before the string was exhausted), matching both
  manual entries and making the zero-length case correct for free (nothing found,
  so `!found` is true) without a separate special case. Verified by
  `internal/cpu/scanc_test.go`'s `TestEmulScanc` (match found, no match, and the
  zero-length regression) and `TestEmulSpanc`.

### [Phase 06] `emul_movtc`'s backward-copy branch indexes the table by the source address instead of the source byte

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_movc.c`'s `emul_movtc()`, the
  `else` (backward-copy, taken when `src <= dst`) branch's translate step:
  `load_byte(tmp2, &ch); load_byte(tbladdr + tmp2, &ch);` — the second call
  overwrites `ch` using `tmp2` (the *source address* being read from) as the table
  index, discarding the byte the first call just loaded.
- **What**: the forward-copy branch two loops above it in the same function gets
  this right (`load_byte(tmp2, &ch); load_byte(tbladdr + ch, &ch);` — indexes by
  `ch`, the loaded byte value). This is a variable-substitution slip between two
  otherwise-parallel code blocks in the same function, not an ISA judgment call:
  whenever MOVTC's overlap handling selects the backward-copy branch (source
  address at or below the destination address), every translated byte comes from
  `table[srcaddr & 0xFF]`-ish garbage instead of `table[source byte]`.
- **Status**: fixed in Go. `internal/cpu/movtc.go`'s `emulMovtc` shares one
  `translate` closure between both branches, so there's only one (correct) indexing
  expression to get right, not two to keep in sync. Verified by
  `internal/cpu/movtc_test.go`'s
  `TestEmulMovtcOverlappingBackwardCopyTranslatesCorrectly`.

### [Phase 06] `emul_movtuc` uses bitwise AND for its loop guard, and never zeroes R2

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_movc.c`'s `emul_movtuc()`:
  `while (tmp1 & tmp3) { ... }`, and no assignment to `vax.R2` anywhere in the
  function.
- **What**: `tmp1 & tmp3` is a bitwise AND of the two remaining lengths, true only
  when they happen to share a set bit — e.g. remaining lengths 2 (`0b10`) and 1
  (`0b01`) give `0`, falsely ending the loop even though both are nonzero. Every
  sibling string instruction in this file guards its loops with `!= 0 &&`/short-
  circuit logical AND; this one clear character (`&` for `&&`, or equivalently a
  missing `!= 0` on each side) is a plain typo, not an ISA reading. Separately, the
  manual's Notes list `R2 = 0` for MOVTUC same as every other instruction in this
  family, but the C source never writes `vax.R2` at all, leaving it stale.
- **Status**: fixed in Go. `internal/cpu/movtc.go`'s `emulMovtuc` loop guard is
  `len1 != 0 && len2 != 0`, and it sets `R2` to `0` unconditionally alongside the
  rest of its register outputs. Verified by `internal/cpu/movtc_test.go`'s
  `TestEmulMovtuc/remaining_lengths_sharing_no_set_bit_still_both_nonzero` and the
  `R2` assertion in `TestEmulMovtuc/translates_until_escape`.

### [Phase 06] `emul_insqhi.c`/`emul_insqti.c`'s emptiness test corrupts the queue on a second insertion

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_misc.c`'s `emul_insqhi()` and
  `emul_insqti()` (identical bug in both): `hf = header + load(header); hb = header +
  load(header+4); if (hf == hb) { /* treat as first entry */ }`.
- **What**: `hf == hb` is intended to detect an empty queue, but it's also true —
  wrongly — whenever the queue already holds *exactly one* real entry: after the
  first insertion, both of the header's link fields are set to the *same* offset
  (pointing at that one entry), so `hf` and `hb` both resolve to the entry's address,
  not the header's. A *second* insertion then re-enters the "first entry" branch,
  which overwrites the header's links to point only at the new entry — silently
  orphaning the first one, with no fault or other signal. Confirmed empirically (not
  just reasoned about): a standalone Python simulation of the literal algorithm
  inserting three entries loses the first two, producing a one-entry "queue" instead
  of three; using `hf == header` (comparing against the header's own address, which
  the correct check should be — a self-relative queue header is empty exactly when
  its own forward link points back to itself) produces the correct three-entry
  circular list, forward and backward, in the same simulation. `emul_remqhi.c`/
  `emul_remqti.c` (which this project's port confirms are otherwise correct) already
  use the unambiguous form of this check (comparing the *raw stored offset* to
  literal `0`, not two *resolved* addresses to each other), corroborating that
  `hf == header` (or equivalently, the raw offset at `header` being `0`) was the
  intended test here too.
- **Status**: fixed in Go. `internal/cpu/queue.go`'s `emulInsqhi`/`emulInsqti` check
  `hf == header`. Separately, neither C function explicitly clears `vax.pslw.z` on
  the non-empty insertion path (only the empty-queue branch sets it, to `1`) — left
  at whatever it held before the instruction, where the manual specifies `Z <- 0`
  whenever insertion doesn't produce a first entry; fixed the same way, explicitly.
  Verified by `internal/cpu/queue_test.go`'s `TestEmulInsqhi`/`TestEmulInsqti`,
  which insert three entries and walk the resulting queue both forward and
  backward, not just checking Z.

### [Phase 06] `emul_insqti.c`'s non-empty branch updates the wrong link field of the previous tail entry

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_misc.c`'s `emul_insqti()`, the
  non-empty branch's first store: `offset = entry - hb; store_memory(hb+4, &offset,
  4);`, commented `/* Fix current last entry's forward link to be new entry */`.
- **What**: the comment says "forward link," but `hb+4` is the *backward*-link
  field position (matching this file's own consistent `header`/`header+4` = forward/
  backward convention, used correctly everywhere else in the same function and in
  `emul_insqhi()`). A tail insertion needs the *previous* last entry's forward link
  updated to point to the new entry (so a forward traversal keeps working); writing
  to its backward link instead leaves that forward link stale, pointing past the
  new entry straight back to the header. Confirmed by the same kind of standalone
  simulation as the emptiness-test finding above: with only the emptiness check
  fixed, inserting three entries via INSQTI still collapses to a broken one-entry
  loop (`header -> first-entry -> header`, second and third entries unreachable);
  changing this one store's target from `hb+4` to `hb` produces a correct,
  bidirectionally-traversable three-entry queue in the same simulation. This is a
  second, independent bug from the shared emptiness-test one above — fixing only
  one of the two still leaves INSQTI broken for three or more entries.
- **Status**: fixed in Go. `internal/cpu/queue.go`'s `emulInsqti` stores to `hb`'s
  forward field (`storeLink(e, hb, 0, entry)`), not its backward field. Verified by
  `internal/cpu/queue_test.go`'s `TestEmulInsqti`, which checks both the forward and
  backward traversal of a three-entry queue (the backward check specifically
  regresses this finding, since the forward-only check from the emptiness-test fix
  alone wouldn't have caught it).

### [Phase 06] REMQUE/REMQHI/REMQTI's second operand is tabled as an address operand, not the write-longword destination the manual and the C handlers themselves use

- **Where**: `reference/eVAX/eVAX/Headers/instruction_table.h`'s REMQUE, REMQHI, and
  REMQTI rows all declare their second operand `OP_AD` (address access) — REMQHI/
  REMQTI's row is otherwise a byte-for-byte copy of INSQHI/INSQTI's (`{1, 8}` scale,
  `OP_AD, OP_AD` access), despite the two pairs of instructions having a different
  operand order and a genuinely different second-operand type.
- **What**: `vax_instr_set.pdf`'s format lines are unambiguous — `entry.ab, addr.wl`
  for REMQUE, `header.aq, addr.wl` for REMQHI/REMQTI — the second operand in every
  case is `addr.wl`, a *write longword* destination for the removed entry's address,
  not an address-yielding operand. `emul_remque.c`/`emul_remqhi.c`/`emul_remqti.c`'s
  own handlers already agree with the manual over their own table row: every one of
  them calls `put_operand(opcode, 1, OP_WR, ...)` explicitly. As tabled, this
  project's decoder (which, per the "Register mode used where OP_AD/OP_VA access is
  required" finding above, faults `AccessAddress` operands resolving to Register
  mode) would wrongly reject the common, legal case of writing the removed entry's
  address straight into a register (e.g. `REMQUE @h, R2`).
- **Status**: fixed in the generated table. `internal/cpu/gen`'s `knownTableFixes`
  patches all three rows' second operand to `OP_WR` (REMQUE's scale was already
  correct at 4; REMQHI/REMQTI's scale is corrected to `{8, 4}`, matching `header.aq`/
  `addr.wl`, rather than the copied-over `{1, 8}`). `internal/cpu/queue.go`'s
  `emulRemque`/`emulRemqhi`/`emulRemqti` use `Operand.Store` for this operand
  accordingly. Verified indirectly: `internal/cpu/queue_test.go`'s tests write the
  removed entry's address into a register destination (`REMQHI header, R0`, etc.),
  which would fault at decode time before this fix.

### [Phase 06, resolved Phase 12] `emul_cmpc5`'s fill-padding loops still run after the main loop finds an inequality

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_cmpc.c`'s `emul_cmpc5()`: the
  dual-string compare loop's only exit conditions are "both lengths exhausted" (the
  `while` condition) or `break` on the first unequal byte pair; either of its two
  fill-padding loops that follow runs unconditionally on whatever length is left
  over, regardless of *why* the main loop stopped.
- **What**: the manual states "comparison proceeds until inequality is detected or
  all bytes of the strings have been examined, [and] condition codes are affected by
  the result of the last byte comparison" — read plainly, once an inequality is
  found, comparison is over. But if the main loop stops due to inequality while
  *both* strings still have bytes left, the source-string fill loop (and,
  potentially, the destination-string one) still executes next, re-running
  `SETCONDITIONBITS` against the *fill* byte and the *same already-mismatched*
  source byte the main loop just examined — silently discarding the actual mismatch
  the manual says should be the final answer, in favor of an unrelated
  byte-vs-fill comparison.
- **Status**: fixed in Go, in Phase 12. Settled against `vax_instr_set.pdf`'s own
  CMPC entry directly (not guessed at): "comparison proceeds until inequality is
  detected **or** all the bytes of the strings have been examined[;] condition codes
  are affected by the result of the **last** byte comparison" — an inequality ends
  the whole operation, so the fill-padding loops must not run afterward and
  overwrite that result. `internal/cpu/cmpc.go`'s `emulCmpc5` now tracks whether the
  main loop's exit was due to a mismatch (`inequality`) and skips both fill loops
  when it was. Verified by `internal/cpu/cmpc_test.go`'s new
  `TestEmulCmpc5/inequality_found_with_bytes_remaining_on_both_sides_is_not_
  overwritten_by_fill_padding` (checks condition codes and R0-R3 all still reflect
  the true mismatch, not a comparison against the fill byte).

### [Phase 07, resolved Phase 12] ADAWI's condition codes don't match the manual in two ways

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_interlock.c`'s `emul_interlock()`
  (the `case 0x58` ADAWI branch), ported to `internal/cpu/interlock.go`'s
  `emulAdawi`.
- **What**: two separate gaps against the manual's "N <- sum LSS 0; ... C <- {carry
  from most-significant bit}":
  - C is supposed to be the addition's carry out, but the C source's
    `SETCONDITIONBITS(data, 0L)` call compares `data` against the constant `0`, both
    cast to `ULONGWORD` — an unsigned value is never less than zero, so this can only
    ever clear C, never set it.
  - N/Z are computed from `data`, the _untruncated_ 32-bit sum of the two sign-
    extended word operands, not from the word actually stored to the sum operand.
    The two disagree exactly in the overflow case: `32767 + 1 = 32768` is positive
    as a 32-bit sum (N clear) even though the word actually stored (`32768`
    truncated to a signed word) is `-32768` (would be N set, if N were computed from
    the truncated result instead).
- **Status**: fixed in Go, in Phase 12, using the same wide-arithmetic `addResult`
  helper ADD/ADWC already use (`internal/cpu/condcodes.go`) instead of either gap:
  N/Z now come from the truncated word result actually stored, and C from a real
  carry out of bit 15. Verified by `internal/cpu/interlock_test.go`'s
  `TestEmulAdawiOverflowSetsV` (updated to expect N set) and
  `TestEmulAdawiSetsRealCarry` (renamed from `TestEmulAdawiNeverSetsCarry`, now
  expecting a real carry).

### [Phase 07, resolved Phase 12] EDIV never detects quotient overflow

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_extended.c`'s `emul_ediv()`, ported
  to `internal/cpu/extended.go`'s `emulEdiv`.
- **What**: the manual lists two conditions for EDIV's `V` bit: the divisor is zero,
  or the quotient doesn't fit in 32 bits (both, per Note 2, fall back to "quotient <-
  bits 31:0 of the dividend, remainder <- 0"). `emul_ediv.c` only ever checks for a
  zero divisor; a genuine quotient overflow (e.g. a large quadword dividend divided
  by a small divisor) is computed and silently truncated with no V set and no
  fallback applied.
- **Status**: fixed in Go, in Phase 12. `emulEdiv` now checks the computed quotient
  against `longMin`/`longMax` (the same longword bounds `internal/cpu/fpu.go`'s
  float-to-integer conversions already use) and applies the manual's Note 3 fallback
  (quotient <- low 32 bits of the dividend, remainder <- 0, V set) on overflow, the
  same as the zero-divisor case. Verified by `internal/cpu/extended_test.go`'s new
  `TestEmulEdivQuotientOverflow`. Still true, and unchanged by this fix: no
  instruction in this codebase raises the architected arithmetic-trap fault for an
  integer-overflow V regardless of the `IV` PSL bit (see `emulDiv` in
  `internal/cpu/integermath.go`) — the whole trap-on-overflow mechanism is
  unimplemented project-wide, a separate gap from EDIV's own V-bit computation.

## Open questions carried forward (not yet findings)

### [Phase 03/04, noticed in Phase 06] PC-relative Immediate mode isn't rejected for an `AccessAddress` operand

- **Where**: `internal/cpu/operand.go`'s `decodePCRelative`, `case 0x08` (Immediate:
  `I^#n`): only faults `ExcReservedAddr` for `access == AccessModify ||
  access == AccessWrite`, unlike the short-literal path a few lines above in
  `decodeOperand` (`case mode < 4`), which faults for *any* non-`AccessRead` access
  — including `AccessAddress` — and unlike Register mode's own `AccessAddress`/
  `AccessVarField` check (the fix in this doc's "Register mode used where OP_AD/
  OP_VA access is required" finding).
- **What**: an `AccessAddress` operand (e.g. LOCC/SKPC/MATCHC's address operands
  this phase, or MOVAL/PUSHAL/JMP from Phase 04) that happens to be encoded as
  `I^#n` has no VAX address either — same underlying problem the Register-mode fix
  already solved — but decode doesn't currently catch this specific encoding of it.
  Noticed while confirming decode already covers `emul_locc.c`'s
  `is_register[2] != OP_MEMORY` check for this phase's LOCC (it does, for every
  addressing mode actually exercised by this phase's tests, but not this one).
- **Status**: not fixed, and — re-examined at Phase 12 (this doc's own named revisit
  point) — this finding's own cited precedent ("same underlying problem the
  Register-mode fix already solved") no longer holds: that Register-mode fix was
  itself reverted this same phase, once running `kernel.asm` for real (for the first
  time ever, via the new `ASM`/`CALL` console commands) found it broke CALLG's own
  legitimate use of Register mode for its arglist operand — see this doc's "Register
  mode used where OP_AD/OP_VA access is required" entry. That's a direct, concrete
  lesson about adding a *new* blanket decode-time reject for an addressing mode
  without a real fixture exercising it either way: still no current fixture uses
  Immediate mode for an `AccessAddress` operand, so there's no way to confirm this
  fault wouldn't have the same problem. Staying deliberately open rather than fixed
  unilaterally; revisit only once a real program is found that needs one behavior or
  the other, or ask.

### [Phase 06] Character-string length operands: signed `short` or unsigned word?

`emul_movc.c`/`emul_cmpc.c` (and, per its own file's shared shape, `emul_locc.c`/
`emul_matchc.c`/`emul_skpc.c`) declare every string-instruction length operand as a
signed `short`/`LONGWORD` sign-extended from one, and gate their main loops on
`> 0`. If a length operand's high bit is set (a count of 32768 or more), this
signed interpretation makes the count negative and skips the whole operation (and,
for `emul_cmpc3`/`emul_movc3`'s backward-copy branch specifically, produces an
arithmetically-consistent-but-strange result — see each handler's own comment).
The manual's operand notation (`len.rw` etc.) doesn't explicitly settle whether
these are meant to be a genuinely unsigned 16-bit count (as VAX byte counts
conventionally are) or a signed one exactly as ported. Not resolved either way —
`internal/cpu/movc.go`/`cmpc.go` replicate the C source's signed-`short` reading
faithfully rather than guessing. Given how large a string a real MACRO-32 program
would need to trigger this (32KB+ in one instruction), low priority to chase further
unless it turns out to matter for a real test fixture.

### [Phase 04] CASE's internal arithmetic width for byte/word selector, base, and limit

`emul_case.c` reads the byte/word `selector`/`base`/`limit` operands through a signed
`char`/`short` pointer, so they're sign-extended to full 32 bits before the
subtraction (`idx = selector - base`), the unsigned comparison against `limit`, and
the table-index/skip-distance arithmetic. `vax_instr_set.pdf`'s own note ("the
selector and base operands can both be considered as either signed or unsigned
integers") doesn't settle whether the *internal* arithmetic is meant to happen at the
operand's own declared width (the convention every other instruction in this phase
uses — CMPB, ADDB, etc. all compute purely within their declared byte/word/long size)
or genuinely widened to 32 bits the way the C source does. The two choices only
produce different results when an operand's own high bit is set (e.g. a `CASEB` with a
`selector` byte of 0x80 or above), which is a fairly unusual case values would take in
practice. Not resolved either way — replicated as the C source's sign-extend-then-
32-bit-arithmetic behavior in `internal/cpu/branchacb.go`'s `emulCase` rather than
guessed at. Re-checked directly against `vax_instr_set.pdf`'s own CASE entry at
Phase 12 (this doc's own named revisit point, "with the hardware/architecture
reference in hand"): Note 2 there is exactly the sentence quoted above, and the
manual has nothing else on the subject — the ambiguity is in the primary source
itself, not something a closer reading resolves. Staying deliberately kept, as the
C source's own (equally legitimate, per Note 2) reading of "signed integers,
widened."

<!--
Entry template:

### [Phase NN] Short title

- **Where**: `reference/eVAX/eVAX/Source/.../file.c` (function/lines), ported to
  `internal/.../file.go`.
- **What**: what the C code does vs. what the ISA manual / the C code's own comments
  say it should do.
- **Status**: deferred (Go replicates C behavior as-is) | fixed in Go (rationale).
-->
