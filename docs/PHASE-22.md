# Phase 22: RMS system services backed by `github.com/tucats/ods2`

## Goal

Introduce real, functional VMS RMS (Record Management Services) system-service
support — `SYS$CREATE`, `SYS$OPEN`, `SYS$CLOSE`, `SYS$GET`, `SYS$PUT` (plus
`SYS$CONNECT`, already ported in Phase 10) — backed by genuine ODS-2 volume/file
access via the sibling Go module `github.com/tucats/ods2` (working directory
`/Users/tom/go/src/github.com/tucats/ods2`).

Phase 10's `internal/rtl/rms.go` is a stopgap that never implemented real RMS
semantics at all: it just `fopen`s an arbitrary host path (or, for `TTA0:`,
writes to the console) and calls that "RMS". That's incomplete, not faithful to
VMS, and — per the user's explicit direction — gets **removed**, not kept
alongside the new implementation. The whole point of this phase is a true
VMS-like file system: because `ods2` implements the real, on-disk ODS-2 format
(the same format `simh` and other VAX simulators use), a container built by
`govax`'s new `MOUNT`/file-creation path should be mountable by `simh` and
vice versa. Any file-system convenience that isn't genuine ODS-2 has no place
here.

A new console `MOUNT` command attaches a disk-image container file to a device
(e.g. `DUA0:`), known simultaneously to `govax`'s own device table and to `ods2`'s
volume layer. The phase's acceptance objective, verbatim from the request that
started it: mount a disk container, assemble a VAX program that creates a file in
the container, writes a few records, closes it, reopens it for reading, reads the
records back to verify they're correct, and closes it again.

Like Phase 15 (UX support) and Phase 18 (flow-of-control), this phase isn't a
straight port of one `reference/eVAX` source file — see "Why this phase looks
different" below — and is expected to grow further sub-phases (`INITIALIZE`, more
RMS services, indexed/relative file organizations) once this first slice lands.

**Status: planning.**

## Why this phase looks different from most others

`reference/eVAX/eVAX/Source/RTL/rms.c` never touches real ODS-2 structures — its
three operations (`rms_create`/`rms_connect`/`rms_put`) just `fopen`/`fwrite`
directly against the host filesystem, exactly matching what Phase 10's
`internal/rtl/rms.go` already ports (see that file's own doc comment). There is no
C-source counterpart for real volume/file-system semantics to diff against, and no
`MOUNT` command anywhere in `reference/eVAX` (confirmed by search — the only
"mount" hits in the whole C tree are `devices.c`'s `mountcount` field, initialized
to 0 at device-creation and never read or written again anywhere else, and a
handful of unrelated `SS$_` codes in `ss_def.h`). So this phase's correctness
reference is **not** `reference/eVAX`; it's:

- The real VMS RMS layout, from `reference/eVAX/eVAX/Headers/fab.h`/`rab.h` — the
  actual `$FABDEF`/`$RABDEF` struct layouts the original C project's own `rms.c`
  was built against (confirmed authoritative: `internal/rtl/rms.go`'s existing
  `fabIFI`/`fabFAC`/`fabFNA`/... offsets already derive from these headers, not
  guessed).
- `~/Documents/Technical Doc/VMS/rms_manual.pdf` — service semantics ($CREATE/
  $OPEN/$CLOSE/$GET/$PUT argument lists, completion codes, record-format rules)
  neither header captures.
- `github.com/tucats/ods2`'s own `volume`/`diskimage`/`ondisk`/`rms`/`filespec`
  packages — a from-scratch, already-tested ODS-2 implementation this phase shims
  onto rather than reimplementing. (Full API survey done during this doc's own
  planning; summarized in "Design decisions" below.)

Phase 10's existing three-op host-passthrough path is removed outright (see
"Removing Phase 10's host-passthrough RMS" below), not kept alongside the new
one. The one piece of it worth keeping is the `TTA0:` special case — that's
legitimate terminal-device I/O, not a native-filesystem shortcut, and real VMS
RMS genuinely does support `SYS$CREATE`/`SYS$PUT` against a terminal device —
so it's carried forward into the new package rather than discarded with the
rest.

## Scope

- `reference/eVAX/eVAX/Headers/fab.h`, `rab.h` — authoritative FAB/RAB field
  offsets and bit layouts.
- `reference/eVAX/eVAX/Headers/ss_def.h` — real `SS_DEVMOUNT` (108),
  `SS_DEVNOTMOUNT` (124), `SS_NOMOUNT` (10380) for `MOUNT`/`DISMOUNT`'s own
  operational errors.
- `~/Documents/Technical Doc/VMS/rms_manual.pdf` — RMS service semantics and
  status-code vocabulary.
- `github.com/tucats/ods2`'s `diskimage`, `ondisk`, `volume`, `rms`, `filespec`
  packages — the real mount/file/record engine.
- `internal/io/device.go` — `Device`/`DeviceTable`, already carrying (unused since
  Phase 09) `VolName`/`MediaName`/`MediaType`/`RootDevName`/`MountCount`/
  `DeviceClassDisk` fields this phase finally populates for real.
- `internal/bootdata/files/evax.dcl` — new `MOUNT`/`DISMOUNT` grammar (see the
  "Grammar file" decision below — **not** `testdata/dcl/evax.dcl`).
- `internal/rtl` — existing `ServiceTable`/`Environment`; `rms.go`'s three
  host-passthrough handlers are removed and their `ServiceTable` slots
  re-registered against the new package.
- New package: `internal/rms` — the sole RMS implementation from this phase on.

## Design decisions

### Grammar file: `internal/bootdata/files/evax.dcl`, not `testdata/dcl/evax.dcl`

`testdata/dcl/evax.dcl` is `git archive`-imported read-only from the upstream C
repo (`CLAUDE.md`: "if upstream changes, re-import"). `internal/bootdata/files/
evax.dcl` is a separate, byte-identical-today copy that's actually what `govax`
parses at runtime (`cmd/govax/main.go`'s `resolver.ReadFile("evax.dcl")`, Phase
15's embedded-fallback mechanism) — confirmed by diff (currently 0 bytes
different) and by grepping every non-comment reference to either path. `MOUNT`/
`DISMOUNT` have no upstream C-grammar counterpart to preserve, so per the user's
own direction: add them directly to `internal/bootdata/files/evax.dcl`, and leave
`testdata/dcl/evax.dcl` untouched as the pure historical import. The two files are
expected to diverge from this phase on — a comment block at the new grammar's
insertion point should say so explicitly, so a future "upstream changed, re-import
`testdata/dcl/evax.dcl`" doesn't get mistakenly copied over the bootdata file and
wipe govax-native commands.

Fallout: `internal/console/dcl/define_test.go` and `parse_test.go` currently load
`testdata/dcl/evax.dcl` directly for grammar-parser unit tests — switch both to
load `internal/bootdata/files/evax.dcl` instead, since that's now the grammar
source of truth their tests should track. Doc-comment references to
`testdata/dcl/evax.dcl` elsewhere (`internal/console/dcl/{grammar,doc,match}.go`,
`internal/io/device.go`, `internal/console/show.go`) that are describing
*behavior the C reference's grammar originally defined* (e.g. `dev_class`/
`dev_type` keyword tables) stay pointed at `testdata/dcl/evax.dcl` — those really
are describing the untouched import; only the two direct-load tests need to move.

Draft grammar shape (refined during implementation):

```text
syntax mount
    parameter device/id=.../type=$name/prompt="Device"
    parameter file/id=.../type=$string/prompt="Container file"
    qualifier write/id=...
    qualifier nowrite/id=...

verb mount

syntax dismount
    parameter device/id=.../type=$name/prompt="Device"

verb dismount
```

Kept deliberately minimal — device, container path, and a writable/read-only
switch. Real VMS `MOUNT`'s fuller qualifier set (`/FOREIGN`, `/OVERRIDE=
IDENTIFICATION`, `/SYSTEM`, volume-set qualifiers, ...) and `INITIALIZE` are
explicitly out of scope this phase (see "Deferred" below) but the grammar/`verb
mount` structure leaves room to add qualifiers later without restructuring.

### `INITIALIZE` is deferred to a later phase

Confirmed with the user: Phase 22's objective only needs an already-initialized
`.dsk` container to mount. `ods2`'s own `diskimage.Create` + `volume.Initialize`
(two calls, see `cmd/ods2/internal/session/initialize.go` for the reference usage)
fully covers building a fresh container from a Go test helper, so this phase's own
tests build fixtures that way rather than depending on a govax `INITIALIZE`
command or on `ods2`'s separate `cmd/ods2` CLI tool being installed. A govax
`INITIALIZE` console command (and the rest of the DCL-command parity with `ods2`'s
own CLI the user's notes mention — `COPY`, `DIRECTORY`, etc.) becomes its own
later phase.

### Dependency: `go.work`, not a `replace` directive

`ods2` is actively co-developed alongside this phase and has a real tag,
`v0.1.3` (the repo was private during this doc's initial planning; the user
has since made it public — see progress log). Verified experimentally: a
`go.work` file at the `govax` repo root —

```text
go 1.26.0

use .
use ../ods2
```

— resolves `github.com/tucats/ods2` straight from the local sibling checkout
(both repos already sit side by side under
`/Users/tom/go/src/github.com/tucats/`) with **no `require` line needed in
`govax/go.mod` at all** while the workspace is active: confirmed by building a
throwaway test package that imported `ods2/diskimage` with no `ods2` entry in
`go.mod` — `go build`/`go vet`/`go test` all succeeded offline, and neither
`go.mod` nor `go.sum` were touched. This is the mechanism for local
development regardless of the repo's visibility: nothing here ever needs to
reach GitHub or the module proxy for `ods2` at all.

Adding an explicit `require github.com/tucats/ods2 vX.Y.Z` pin to `go.mod`
was tried and failed in this session specifically (`go get`'s own internal
git fetch couldn't authenticate to GitHub even after the repo was made
public and a plain `git ls-remote` in the same shell succeeded — an
environment-specific restriction on this session's outbound package-fetch
path, not a real blocker). So for now: **no `require` entry for `ods2` in
`go.mod`.** `go.work`/`go.work.sum` are already in `.gitignore` (pre-dating
this phase), so this is invisible to anyone else building `govax` — which
also means, today, `govax` **cannot build at all without the workspace**
(`../ods2` present and the `go.work` file in place) once any code imports
`internal/rms`. That's fine for now (solo local dev); before `govax` ever
needs to build standalone (CI, a release, someone else cloning just
`govax`), run `go get github.com/tucats/ods2@v0.1.3` (or `go mod tidy`) from
a normal terminal to populate a proper `require` + `go.sum` entry — should
work cleanly now that the repo is public. `go.work`'s `use` directive still
wins for local source resolution either way.

### New package `internal/rms`: the sole RMS implementation

Mirrors `internal/rtl`'s own registry-over-switch convention
([[feedback_table_driven_dispatch]]) and its `ServiceFunc` shape, but lives
separately from `internal/rtl` because it owns real state (`ods2` handles) that
Phase 10's stopgap never needed. It replaces `internal/rtl/rms.go` entirely —
see "Removing Phase 10's host-passthrough RMS" below — and becomes the only
place `SYS$CREATE`/`SYS$CONNECT`/`SYS$OPEN`/`SYS$CLOSE`/`SYS$GET`/`SYS$PUT` are
implemented:

- `internal/rms/mount.go` — a `MountTable`: device name -> `{container
  diskimage.Container; vol *volume.Volume; writable bool}`. `Mount(device, path
  string, writable bool) error` (`diskimage.OpenWritable`/`Open` +
  `volume.Mount`), `Dismount(device string) error` (`vol.Dismount()` + remove),
  `Lookup(device string) (*volume.Volume, bool)`. Owned by `internal/console`'s
  `Console` (constructed once, alongside `Devices`/`Logicals`) and injected into
  `rtl.Environment` the same way those two already are.
- `internal/rms/fab.go`/`rab.go` — real `$FABDEF`/`$RABDEF` field offsets. Starts
  from `internal/rtl/rms.go`'s existing `fabIFI`/`fabFAC`/`fabFNA`/`fabFNS`/
  `fabSTS`/`fabSTV`/`rabFAB`/`rabISI`/`rabRAC`/`rabRSZ`/`rabRBF`/`rabSTS`/`rabSTV`
  set (moved here, not duplicated) plus the new fields this phase needs:
  `FAB$B_ORG`/`FAB$B_RFM`/`FAB$B_RAT`/`FAB$W_MRS` (record organization/format/
  attributes/max-size, needed by `SYS$CREATE` to pick an `ondisk.RecordFormat`),
  `RAB$L_UBF`/`RAB$W_USZ`/`RAB$W_RSZ` (user buffer/size/returned-size, needed by
  `SYS$GET`). Verify every offset directly against `fab.h`/`rab.h` while writing
  this file, not by re-deriving from the manual's prose.
- `internal/rms/status.go` — real, literal `RMS$_` symbol values (`RMS$_NORMAL`,
  `RMS$_EOF`, `RMS$_FNF`, ...) pulled from `rms_manual.pdf`, matching
  `internal/rtl/status.go`'s own literal-constant style (**not** routed through
  `internal/vmserrors`'s `RMSFacility` codes — those are govax's own internal
  diagnostic-message IDs under the real RMS facility number, a different
  numbering space from the fixed, well-known `$RMSDEF` values a real compiled VAX
  program checks against; writing a vmserrors-assigned ID into `FAB$L_STS` would
  be silently wrong for any real program). Exact values are an implementation-time
  lookup, not guessed at here — see "Open questions".
- `internal/rms/{create,connect,open,close,get,put}.go` — the service handlers,
  each reading FAB/RAB fields from VAX memory, resolving the target device via
  `filespec.Parse`, looking up the mounted `*volume.Volume`, calling into `ods2`'s
  `volume`/`rms`/`filespec` packages, translating results/errors to `RMS$_`/`SS$_`
  codes, and writing back FAB/RAB fields. `SYS$CREATE`/`SYS$PUT` additionally keep
  the one piece of Phase 10's implementation worth carrying forward: a `TTA0:`
  (and, generally, any `DeviceClassTT` device) target is real terminal I/O, not a
  file, and continues to write straight to the console — everything else must
  resolve to a mounted ODS-2 volume or fail with a real `RMS$_`/`SS$_` device
  error (no more silent arbitrary-host-path `fopen`).
- Its own IFI table (`allocIFI`/handle-by-IFI), generalized from Phase 10's
  "`io.Writer` only" (write-only, host-passthrough) shape to a handle that's
  either the console writer (the `TTA0:` case above) or an `ods2`-backed
  `*volume.File` + `rms.Reader`/`rms.Writer` pair — this phase's first-ever read
  support for RMS files.

### Removing Phase 10's host-passthrough RMS

`internal/rtl/rms.go`'s three handlers (`serviceSysCreate`/`serviceSysConnect`/
`serviceSysPut`, plus `allocIFI`/`ifiWriter`/`storeRMSStatus`/`openRMSFile` and
the `fab*`/`rab*` offset consts) are deleted outright, not kept as a fallback —
per the user's explicit direction: this support is incomplete and doesn't
represent real RMS behavior (arbitrary host-path `fopen` has no VMS analogue;
real RMS always operates against a mounted device/volume), so there's nothing
worth preserving behind a routing branch. `internal/rtl/rms_test.go` (if any
exists exercising the removed handlers) moves to `internal/rms` and gets
rewritten against the new implementation rather than kept as regression coverage
for code that no longer exists. `internal/rtl`'s `registerRMSServices` shrinks to
registering `internal/rms`'s handlers into the shared `ServiceTable`.

### Container format fidelity / `simh` interoperability

A hard non-negotiable for this phase: nothing `govax` writes to a mounted
container may be a `govax`-specific extension to the ODS-2 on-disk format. Since
`ods2` already implements the real format byte-for-byte (see its own README's
"why not a line-by-line port" section — deliberate implementation-strategy
deviations from a reference C tool, but not format deviations), this should hold
for free as long as `internal/rms` only ever goes through `ods2`'s public API
rather than writing raw blocks itself.

The user has supplied two real containers on this dev machine, at
`testdata/disks/` (gitignored wholesale except its `README.md` — see
`.gitignore` — licensed VAX/VMS code and large binaries, never committed):

- `rq0-ra92.dsk` (~152 MB) — a full, real VAX/VMS system disk. The primary
  fidelity check: mount it **read-only**, list the MFD and at least one
  subdirectory via `filespec.Glob`/`Directory.List`, open and read back one or
  more real files' records — exercising `internal/rms`'s read path
  (`SYS$OPEN`/`SYS$GET`, or the underlying `ods2` calls directly in an
  `internal/rms` test) against a volume `govax` had no part in creating. This
  is the container that actually proves ODS-2 read fidelity; `govax` reading
  its own writes back proves nothing about compatibility with anything else.
- `empty.dsk` (65 KB) — a minimal container `ods2` itself already initialized
  (real home block, `INDEXF.SYS`, `BITMAP.SYS`, MFD, no user files). Useful two
  ways: (a) as a quick local stand-in for the not-yet-built `INITIALIZE`
  command during interactive `govax` testing this phase — copy it to get a
  fresh empty volume instead of hand-running `ods2`'s own Go API — and (b) as
  a second, real (not test-generated) starting point for a write-path check:
  copy it to a scratch file, mount read/write, exercise `CREATE`/`PUT`/`CLOSE`
  through `govax`, dismount, remount read-only, and confirm the file reads
  back correctly.

A small opt-in interop test (`internal/rms/simh_interop_test.go` or similar)
looks for these two files by name under `testdata/disks/` and `t.Skip`s
cleanly when either is absent — so it runs automatically on this dev machine
but never on a fresh clone or in CI. This is separate from, and doesn't
replace, the committed automated test suite's own fixtures (see "Test
fixtures" below), which must stay fully portable and can't depend on anything
under this gitignored directory.

### Device model: `MOUNT` auto-creates the device record

`internal/io.Device` needs no structural change — `VolName`/`MediaName`/
`MediaType`/`RootDevName`/`MountCount`/`DeviceClassDisk` already exist from Phase
09, unused until now. Rather than requiring a separate `DEFINE/DEVICE DUA0
/DEVCLASS=DISK` ceremony before `MOUNT DUA0: foo.dsk` works, `MOUNT` auto-creates
the device (`Devices.Define(name, iodev.DeviceOptions{DevClass:
DeviceClassDisk})`) if it isn't already defined — matching how an operator would
expect `MOUNT` to just work, and consistent with `devices.c`'s own dead
`mountcount` field having anticipated exactly this without ever being wired up.
`SHOW DEVICE/FULL` (`internal/console/device.go`'s `ShowDevices`) gains a
mounted-volume line once a device has a live `MountTable` entry, using the
`VolName`/`FreeBlocks`/etc. fields it already prints, now finally populated from
the real mounted `*volume.Volume`.

`SS_DEVMOUNT`/`SS_DEVNOTMOUNT`/`SS_NOMOUNT` (real `ss_def.h` values 108/124/
10380) back `MOUNT`/`DISMOUNT`'s own operational-error reporting — add them to
`internal/rtl/status.go`'s existing literal `ssXxx` table alongside the ones
already there, rather than inventing new numbers.

### Test fixtures: generated on the fly, not committed binaries

The **committed** automated test suite builds its own throwaway `.dsk`
containers in `t.TempDir()` via `ods2`'s own `diskimage.Create` +
`volume.Initialize` directly from `internal/rms`'s Go tests, rather than
depending on a binary fixture under `testdata/`. Keeps `go test ./...` fully
hermetic and portable (works on a fresh clone, in CI, on any machine) with no
binary blob in git.

Separately, for interactive work on this dev machine during this phase (manual
`govax` sessions, quick iteration before the automated suite exists, the
opt-in `simh`-interop test above), `testdata/disks/empty.dsk` — a real
container the user already initialized via `ods2` — is a ready-made stand-in
for the not-yet-built `INITIALIZE` command: copy it to get a fresh empty
volume rather than writing a one-off Go program to call `ods2`'s API by hand.
This local convenience doesn't feed the committed test suite.

## Subtasks

1. **Done.** Dependency wiring: `go.work` (`use .` / `use ../ods2`) at the
   `govax` root, no `go.mod`/`go.sum` change — see "Dependency: `go.work`, not
   a `replace` directive". Verified with a real, disposable smoke package
   importing `ods2`; `go build ./...`/`go vet ./...`/`go test ./...` all clean.
2. **Done.** `internal/bootdata/files/evax.dcl`: add `verb mount`/`verb
   dismount`, with the divergence-from-`testdata` comment. Move
   `internal/console/dcl`'s two direct-load tests (plus
   `internal/console/dispatch_test.go`'s own direct load, found along the
   way) onto the bootdata copy.
3. **Done.** Delete `internal/rtl/rms.go` (and its test, if any) — `serviceSysCreate`/
   `serviceSysConnect`/`serviceSysPut`, `allocIFI`/`ifiWriter`/
   `storeRMSStatus`/`openRMSFile`, and the `fab*`/`rab*` offset consts all go;
   confirm nothing else in `internal/rtl` referenced them.
4. **Done.** `internal/rms`: mount table (`mount.go`), FAB/RAB offset tables
   (`fab.go`/`rab.go`, seeded from the deleted file's consts plus the new
   fields, verified against a real VMS system's `$FABDEF`/`$RABDEF`), status-
   code table (`status.go`, verified against a real VMS system's `rmsdef.h`),
   IFI table (generalized read/write handle, `ifi.go`).
5. **Done.** `internal/rms`: `SYS$CREATE` — resolve device/directory/name via `filespec`;
   a `DeviceClassTT` target (`TTA0:`) writes to the console as before; a mounted
   disk device does `volume.CreateFile` with an `ondisk.RecAttr` built from the
   FAB's `RFM`/`RAT`/`MRS` fields; anything else is a real device/file error, not
   a host-path fallback. Store the new IFI back into the FAB.
6. **Done.** `internal/rms`: `SYS$CONNECT` (binding a RAB to an already-open IFI, console
   or ODS2-backed).
7. **Done.** `internal/rms`: `SYS$PUT` — `rms.NewWriter`/`.Put` per record, matching
   `RAB$B_RAC` (sequential-only).
8. `internal/rms`: `SYS$CLOSE` — `rms.Writer.Close`/`volume.File.Close`
   (writer case) or a plain `volume.File` release (reader case); release the IFI
   slot.
9. `internal/rms`: `SYS$OPEN` — `filespec.Parse` + `Directory.Lookup`/
   `vol.OpenFID`, honoring `FAB$B_FAC` (GET vs. PUT vs. UPD) to decide whether to
   arm the file for writing (`File.OpenForWrite`) or just read.
10. `internal/rms`: `SYS$GET` — `rms.NewReader`/`.Next` per record, copying the
    record into the RAB's `RBF`/`RSZ` (and `UBF`/`USZ` if distinct) fields,
    `RMS$_EOF` on exhaustion.
11. `internal/rtl`: `registerRMSServices` shrinks to registering `internal/rms`'s
    handlers into the shared `ServiceTable`.
12. `internal/io`/`internal/console/device.go`: `MOUNT`/`DISMOUNT` console
    methods (auto-create device record, call `internal/rms.MountTable`); `SHOW
    DEVICE/FULL` mounted-volume line.
13. `internal/console/dispatch.go`: bind the new `MOUNT`/`DISMOUNT` grammar
    entries.
14. End-to-end acceptance test: build+mount a fresh container, assemble (via the
    existing `ASM`/Phase 11 tooling) a small MACRO-32 program exercising
    `CREATE`->`CONNECT`->`PUT` x N ->`CLOSE`->`OPEN`->`CONNECT`->`GET` x N ->
    `CLOSE`, run it, verify the records read back match what was written. FAB/
    RAB field values in the test program: hand-encoded literals rather than a
    general `$FABDEF`/`$RABDEF` `.INCLUDE` macro-expansion facility (out of
    scope this phase — no such `.asm` fixture exists yet; building one is
    plausible future `internal/asm` work, not needed for this acceptance test).
15. Opt-in `simh`-container interop test (`testdata/disks/`, gitignored —
    "Container format fidelity" above): `rq0-ra92.dsk` mounted read-only,
    list the MFD, read back at least one real file; `empty.dsk` copied to a
    scratch file for a write-then-reread round trip. Skips cleanly when either
    file is absent.
16. Docs: this file's progress log; `docs/PLAN.md` phase-table row + narrative
    paragraph; `CLAUDE.md` package-layout list gains `internal/rms`;
    `docs/DEVIATIONS.md` entries for anything ambiguous found comparing
    `rms_manual.pdf` against `ods2`'s actual record-format behavior along the
    way.

## Open questions

- ~~Exact real `RMS$_` symbol values needed this phase~~ **Resolved (subtask
  4)**: `rms_manual.pdf` turned out to have no numeric `$RMSDEF` table at all
  (just symbol names/prose), and neither did the `vmssrc_archive` v7.3 source
  tree the user separately made available. The user then extracted `rmsdef.h`
  (a real VMS system's own compiled `$RMSDEF` C header) directly from a live
  VMS system and placed it at the repo root (gitignored, never committed —
  see `.gitignore`); `internal/rms/status.go`'s values are transcribed
  straight from it.
- ~~Whether the FAB/RAB offsets beyond `internal/rtl/rms.go`'s existing set...
  need any further fields~~ **Resolved (subtask 4)**: the needed new fields
  (`FAB$B_ORG`/`FAB$B_RAT`/`FAB$B_RFM`/`FAB$W_MRS`, `RAB$L_UBF`/`RAB$W_USZ`)
  are exactly what docs/PHASE-22.md's "Design decisions" already anticipated,
  no more. All offsets — old and new — were confirmed against a real VMS
  system's own `starlet.req` (BLISS field definitions; also gitignored, never
  committed), which caught one real bug in the now-deleted Phase 10 code's
  own offset table: see `internal/rms/fab.go`'s `fabFNS` doc comment.
- Committed-fixture vs. generated-on-the-fly test containers (leaning generated;
  see "Design decisions") — not locked in until the end-to-end test is actually
  written.
- Any `ods2` bugs found while integrating belong to the user (their package) —
  report them rather than working around them silently in `govax`, per the
  original request.

## Progress Log

### 2026-09-22 — Planning

- Surveyed `github.com/tucats/ods2`'s public API (`diskimage`/`ondisk`/`volume`/
  `rms`/`filespec`) via a research pass over its source and `cmd/ods2`'s own CLI
  usage of it (the best real end-to-end usage examples: `initialize.go`,
  `mount.go`, `copy.go`, `type.go`), and confirmed it has no `git` tags yet
  (co-development, `replace` directive needed).
- Confirmed `reference/eVAX` has no `MOUNT` counterpart at all (only a dead
  `mountcount` field and unrelated `SS$_` codes) and that `rms.c`'s three
  existing operations are host-filesystem passthrough, not ODS-2-aware —
  settling this phase's correctness reference as the RMS manual + `fab.h`/
  `rab.h` + `ods2` itself, not `reference/eVAX`.
- Resolved, with the user: `internal/bootdata/files/evax.dcl` (not
  `testdata/dcl/evax.dcl`) is the grammar file to extend, since it's the actual
  runtime source and the testdata copy is meant to stay a pure, untouched
  upstream import; `INITIALIZE` is deferred to a later phase, keeping this one
  scoped to `MOUNT` + the five named RMS services (plus `SYS$CONNECT`, already
  present from Phase 10).
- User clarified mid-planning: Phase 10's `internal/rtl/rms.go` is a buggy,
  incomplete stopgap (arbitrary host-path `fopen` masquerading as RMS) with no
  real VMS fidelity, and gets removed outright rather than kept as a fallback
  path — the whole point of this phase is a true, `ods2`-backed VMS file system,
  faithful enough that a container should be interchangeable with `simh` and
  other VAX simulators. Revised "New package `internal/rms`" and added
  "Removing Phase 10's host-passthrough RMS" and "Container format fidelity /
  `simh` interoperability" design-decision sections accordingly; the one thing
  kept from the old file is its `TTA0:`-to-console special case, since that's
  real terminal I/O, not a filesystem shortcut.
- User offered real `simh`-produced VAX/VMS disk containers, available locally
  on this dev machine for validation but never committable (licensed VAX/VMS
  code, large binaries). Settled on `testdata/disks/`, added to `.gitignore`
  wholesale except a tracked `README.md` documenting the convention (verified:
  `git check-ignore` confirms the README stays tracked while any `.dsk` placed
  there is ignored) so nothing symlinked/copied there is ever at risk of being
  committed.
- User has since placed two real containers there: `rq0-ra92.dsk` (~152 MB, a
  full real VAX/VMS system disk — the primary read-fidelity check target) and
  `empty.dsk` (65 KB, a minimal `ods2`-initialized container with no user
  files — doubles as a local stand-in for the not-yet-built `INITIALIZE`
  command and as a real starting point for a write-path round-trip check).
  Updated "Container format fidelity / `simh` interoperability" and "Test
  fixtures" with the concrete filenames and roles, and clarified that these
  are separate from, and don't replace, the committed automated suite's own
  `ods2`-generated (fully portable) fixtures.
- `ods2` gained a real tag, `v0.1.3`. Settled the local-dependency mechanism
  (superseding this doc's earlier "add a `replace` directive" note): a
  `go.work` file at the `govax` root (`use .` / `use ../ods2`, already
  covered by the pre-existing `.gitignore` entry for `go.work`/
  `go.work.sum`) resolves `ods2` from the local sibling checkout with **no
  `require` line needed in `go.mod`** — verified by building a throwaway
  package that imported `ods2/diskimage` with no such entry present; `go
  build`/`go vet`/`go test` all succeeded fully offline. `ods2` was private
  at the time; the user made it public shortly after (a decision made
  independently of this phase). Attempting to add a real `require
  github.com/tucats/ods2 v0.1.3` pin via `go get` failed in this session
  specifically — its internal git fetch couldn't authenticate even after the
  repo went public and a plain `git ls-remote` in the same shell succeeded,
  including with the Bash tool's sandbox explicitly disabled — an
  environment-specific restriction on this session's outbound package-fetch
  path, not a real blocker (`go.work` needs no such fetch at all). See
  "Dependency: `go.work`, not a `replace` directive" for the full writeup,
  including the one-time `go get`/`go mod tidy` step (from a normal
  terminal) needed before `govax` can build standalone without the
  workspace.

### 2026-09-22 — Subtask 1 complete

- After a VS Code restart (unrelated attempt to clear the session's own
  `go get` git-auth restriction — didn't help; still the same failure even
  post-restart, confirming it's not session-state related), re-verified the
  `go.work`-based dependency wiring is solid: a fresh throwaway package under
  `internal/rms` importing `ods2/diskimage`, `ods2/ondisk`, and `ods2/volume`
  built, vetted, and tested cleanly with zero `go.mod`/`go.sum` changes, then
  removed (not a real deliverable, just verification — the actual
  `internal/rms` package doesn't exist yet, starts at subtask 3/4). Marked
  subtask 1 done in the "Subtasks" list.
- No other implementation started yet.

### 2026-09-22 — Subtask 2 complete

- Added `verb mount`/`verb dismount` to `internal/bootdata/files/evax.dcl`,
  simplified from this doc's own draft sketch: a plain `verb` with its
  `parameter`/`qualifier` statements directly underneath (matching `verb
  vminit`'s existing shape) rather than a separate `syntax mount` entry
  redirected into via `/id=` — no qualifier-driven sub-form redirect is
  needed here (unlike `DEFINE`'s `/LOGICAL` vs. `/DEVICE` split), so the
  extra indirection the draft sketched wasn't buying anything. `MOUNT` takes
  two required parameters (`DEVICE`, `FILE`) and one switch qualifier
  (`WRITE`); `/NOWRITE` reaches it for free via the grammar interpreter's
  existing automatic "NO"-prefix negation (`match.go`'s `matchQualifier`),
  so a second, separately declared `NOWRITE` qualifier (as the draft sketch
  had) would have been redundant. `DISMOUNT` takes one required `DEVICE`
  parameter. New IDs (700/701/702/703/710/711) picked clear of every ID
  already in use elsewhere in the file — grammar IDs turn out to have no
  uniqueness enforcement across the file at all (`define.go`'s `parseID` is
  a bare integer parse; nothing checks for collisions), but keeping them
  distinct avoids confusing a future reader.
- Added the divergence comment at the insertion point (this file's own
  "Grammar file" design decision, and `CLAUDE.md`'s general
  re-import-from-upstream guidance) so a future `testdata/dcl/evax.dcl`
  re-import doesn't get carelessly copied over this file and silently
  delete `MOUNT`/`DISMOUNT` (or anything added after them).
- Moved `internal/console/dcl/define_test.go` and `parse_test.go`'s shared
  `evaxGrammarPath` helper onto the bootdata copy, per the doc's own planned
  fallout. While doing so, found a third direct load of
  `testdata/dcl/evax.dcl` this doc hadn't enumerated:
  `internal/console/dispatch_test.go`'s `evaxGrammarPathForConsole`, used by
  every dispatch test via `newTestDispatcher`. Since dispatch tests exist to
  exercise the same grammar/handler wiring `cmd/govax` runs in production
  (which loads the bootdata copy, not the testdata one), moved it too for
  the same reason the doc gives for the other two — left out of the
  "Fallout" list only because the initial planning search apparently didn't
  turn it up, not because it's a different case.
- Added grammar-level regression coverage: `TestLoadEvaxGrammar_mountDismount`
  (structural: both verbs exist, right parameter/qualifier shapes) plus
  `TestParse_mount`/`TestParse_mountNowrite`/`TestParse_dismount`/
  `TestParse_mountMissingFile` (parse-level: happy path, automatic `/NOWRITE`
  negation, missing-required-parameter error). Updated
  `TestLoadEvaxGrammar`'s `wantVerbs` list and `TestLoadEvaxGrammar_verbCount`
  (10 → 12 verbs) for the two new verbs.
- `go build ./...`, `go vet ./...` clean. `go test ./...` clean except two
  pre-existing failures confirmed unrelated to this change (reproduced
  identically on a stashed pre-change tree, and confirmed flaky/timing-
  dependent rather than deterministic): `TestShowFault` and
  `TestExecute_stopsOnAttention` (the latter passes reliably in isolation,
  fails only under full-suite timing pressure).
- Nothing bound in `internal/console/dispatch.go` yet — `MOUNT`/`DISMOUNT`
  have no handler to bind to until `internal/rms` exists (subtask 13, much
  later); parsing them today just yields `Grammar.Dispatch`'s existing
  "no handler bound" error, exactly like every other currently-unbound
  syntax in this file.

### 2026-09-22 — Subtask 3 complete

- Deleted `internal/rtl/rms.go` and `internal/rtl/rms_test.go` outright, per
  "Removing Phase 10's host-passthrough RMS": `serviceSysCreate`/
  `serviceSysConnect`/`serviceSysPut`, `allocIFI`/`ifiWriter`/
  `storeRMSStatus`/`openRMSFile`, and the `fab*`/`rab*` offset/access consts
  are all gone, along with the six tests that exercised them.
- Confirmed via grep that `ifiFiles`/`nextIFI` (the `Environment` fields
  backing the removed IFI table) were referenced nowhere else in the tree,
  so removed both fields and their initialization from
  `internal/rtl/environment.go`, and updated that file's doc comments
  (struct field block, `NewEnvironment`'s own comment) to stop describing an
  IFI table that no longer lives here — it moves to `internal/rms` from
  subtask 4 on. `consoleOut` itself stays: `print.go`/`file.go` still write
  through it independently of RMS.
- `internal/rtl/service.go`'s `registerServices` no longer calls
  `registerRMSServices` (deleted with the rest of the file) — its doc
  comment now explains RMS registration moves to `internal/rms` once that
  package exists (subtask 11), rather than silently dropping the mention.
- Updated three stale `internal/rtl/rms.go` references found by grep in
  files this subtask didn't otherwise touch, so nothing in the tree points
  at a deleted file: `internal/rtl/file.go`'s doc comment (IFI table now in
  `internal/rms`), `internal/console/show.go`'s `ShowMap` (both its doc
  comment and its printed "Not applicable" text, now pointing at
  `internal/rms/fab.go`/`rab.go` — `TestShowMap` only asserts on the
  "Not applicable" substring, so the reworded message doesn't break it),
  and `internal/console/image.go`'s doc comment citing the FAB/RAB
  direct-offset-read precedent.
- Left `internal/rtl/status.go`'s `ssNoSuchFac`/`ssNoSuchFile` constants in
  place even though nothing currently references them post-deletion — real,
  generic `SS$_` codes (not RMS-specific) that subtask 5's `SYS$CREATE`
  device/file-error paths will need again almost immediately; removing and
  re-adding them within the same phase would be pure churn, and unused
  constants (unlike unused imports/locals) aren't a Go compiler error.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean (no flaky
  failures this run, unlike subtask 2's log entry).

### 2026-09-22 — Subtask 4 complete

- Spent most of this subtask nailing down the exact FAB/RAB byte offsets and
  real `RMS$_` status values, since neither of this phase doc's own named
  sources (`rms_manual.pdf`, `fab.h`/`rab.h`) turned out sufficient on their
  own — see the two "Open questions" entries above (now resolved) for the
  full story. Short version: `rms_manual.pdf`'s "Field Offset" table columns
  turned out to be symbol names, not numbers, for both FAB and RAB; and
  `reference/eVAX/eVAX/Headers/fab.h`'s literal C struct layout has an extra
  `fab_l_jnl_overlay` union that, taken completely literally with natural
  alignment, is internally inconsistent with that same header's own declared
  `FAB_K_BLN = 80` — per the user's explicit direction ("if you find
  discrepancies between the reference eVAX implementation [and] the RMS
  documentation, that almost certainly just means it was a bug in the C
  code"), this was treated as a C-reference bug, not reproduced. Resolved
  two ways: (1) hand-walking `fab.h`/`rab.h`'s struct declarations
  field-by-field with natural alignment, confirming the running total lands
  exactly on `FAB_K_BLN`/`RAB_K_BLN` (80/68); (2) the user separately
  supplied two real artifacts extracted from a live VAX/VMS 7.3 system —
  `starlet.req` (BLISS field definitions, gitignored) and `rmsdef.h` (the
  compiled `$RMSDEF` C header, gitignored) — which gave literal, authoritative
  byte offsets and status-code values directly. All three sources agree on
  every FAB/RAB offset. Along the way this caught a real bug in the
  now-deleted Phase 10 stopgap: its `fabFNS` offset (48) was wrong by exactly
  the width of the `FAB$L_DNA` field it missed; the real offset is 52 (see
  `internal/rms/fab.go`'s own doc comment).
- Added `internal/rms` (`go.mod`/`go.work` wiring from subtask 1 already
  covered this): `doc.go` (package overview), `fab.go`/`rab.go` (FAB/RAB
  offset and access-mode constants), `status.go` (real `RMS$_` completion
  codes), `mount.go` (`MountTable`: `Mount`/`Dismount`/`Lookup`/`Writable`,
  keyed by normalized VMS device name, opening containers via `ods2`'s
  `diskimage.Open`/`OpenWritable` and `volume.Mount`), and `ifi.go`
  (`FileTable`/`FileHandle`: the generalized IFI table replacing Phase 10's
  `io.Writer`-only version — a handle is either the console pseudo-device or
  a real `*volume.File` with an `ods2/rms.Reader` or `.Writer` armed once
  connected).
- `FileTable.Alloc` deliberately rescans from the lowest reserved-slot-free
  IFI on every call rather than keeping a monotonically increasing cursor
  (see its own doc comment) — a simplicity-over-micro-optimization choice,
  and it means a `Release`d IFI is available for reuse immediately, unlike
  the old code (which never implemented `Close`/`Release` at all).
- Test coverage: `mount_test.go` (mount/lookup/writable/dismount, including
  the "already mounted"/"not mounted"/missing-container-file error paths,
  using `ods2`'s own `diskimage.Create`+`volume.Initialize` to build
  throwaway fixtures — no binary blob under `testdata/`), `ifi_test.go`
  (console seeding, reserved-slot skipping, alloc uniqueness, release-and-
  reuse), `fab_test.go`/`rab_test.go` (structural: every offset fits inside
  its block and none overlap another field), `status_test.go` (every status
  value distinct/positive, plus a dedicated check that `rmsNormal`'s low 3
  bits really do encode `STS$K_SUCCESS` — the one bit a real VAX program's
  own `BLBC`-style success check actually branches on).
- No handler files yet (`create.go`/`connect.go`/`open.go`/`close.go`/
  `get.go`/`put.go` are subtasks 5-10) — `internal/rms` doesn't register
  anything into `internal/rtl`'s `ServiceTable` yet, so `go build`/`go vet`/
  `go test ./...` are clean but nothing in this new package is reachable
  from a running VAX program yet.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean.

### 2026-09-23 — Subtask 5 complete

- Added `internal/rms/create.go` (`SysCreate`, the SYS$CREATE handler) and
  `internal/rms/context.go` (`Context`, a new self-contained bundle of
  everything a handler needs — `*vm.Memory`/`*vax.CPU` for direct VAX-memory
  access, `*MountTable`, `*FileTable`, `*iodev.LogicalNameTable`, and the
  console `io.Writer` — plus small `ctx.load*`/`ctx.store*` forwarders and
  `loadFixedString`, the non-NUL-terminated string reader RMS file specs
  need). `Context` exists specifically so this package never has to import
  `internal/rtl`: `internal/rtl.Environment` bundles almost the same state,
  but its fields are unexported, and subtask 11 has `internal/rtl` register
  this package's handlers into its own `ServiceTable` (`rtl` -> `rms`) —
  the reverse dependency (`rms` -> `rtl`, needed only to spell
  `*rtl.Environment` as a handler parameter type) would make the two
  packages import each other, which Go refuses to build. Handler functions
  in this package therefore take `(*Context, []uint32)` rather than
  matching `rtl.ServiceFunc`'s literal signature; subtask 11 is where
  `internal/rtl` adapts between the two (small closures over its own
  private `mem`/`cpu`/`consoleOut` fields, calling into this package's
  exported handlers).
- Also added `status.go`'s `storeStatus` helper (forward-declared by that
  file's own doc comment back in subtask 4, not implemented until now):
  writes one `RMS$_` value into both of a FAB/RAB's status fields and
  returns that same value, so every failing branch in a handler can end
  with a single `return storeStatus(...)` line that's simultaneously
  "what R0 becomes" and "what FAB$L_STS/STV record" — matching real RMS,
  where a call's completion code is always both at once.
  `SysCreate` follows that pattern throughout: the TTA0: console case
  (unchanged from the deleted Phase 10 stopgap, down to the hardcoded
  device-name check — see `create.go`'s `consoleDeviceName` doc comment
  for why a literal check rather than an `internal/io` `DeviceClassTT`
  lookup) just allocates an IFI against the console writer; the real-volume
  case (`createOnVolume`) resolves the FAB's file spec through
  `ods2/filespec.Parse`/`ResolveDirectory`, validates `FAB$B_ORG` (sequential
  only) and `FAB$B_RFM` (one of the six defined non-`UDF` formats — see the
  code comment on why `FAB$C_UDF`/0 is left unsupported rather than
  resolved to a guessed default, a real but not-yet-motivated gap, not a
  bug), and calls `volume.CreateFile` with an `ondisk.RecAttr` built from
  `RFM`/`RAT`/`MRS`. A calling program's logical-name-translation step
  (`LNM$FILE_DEV`, e.g. `SYS$OUTPUT` -> `TTA0:`) is preserved from the
  deleted Phase 10 code unchanged.
- Test coverage (`create_test.go`, all against throwaway `ods2`-initialized
  fixtures via `mount_test.go`'s existing `newTestVolumeFile` helper — no
  new binary blob under `testdata/`): the console path (including that the
  allocated handle actually writes through to the console buffer); a real
  disk-file create, independently re-verified by looking the new file back
  up through the *same* mounted `*volume.Volume` `SysCreate` itself used
  (not just this package's own IFI table agreeing with itself); a
  read-only-mounted device (`RMS$_PRV`, and confirms `FAB$W_IFI` is left
  untouched rather than partially written); an unmounted device
  (`RMS$_DNR`); a FAB that never asked for `PUT` access (`RMS$_PRV`, before
  the mount table is even consulted); unsupported `FAB$B_ORG`/`FAB$B_RFM`
  values (`RMS$_ORG`/`RMS$_RFM`); a device-only spec with no file name at
  all, and a spec naming a nonexistent subdirectory (both `RMS$_FNF`); and
  logical-name translation reaching the console path indirectly through a
  made-up logical. No handler is wired into `internal/rtl`'s `ServiceTable`
  yet (still subtask 11) — `SysCreate` is only reachable by calling it
  directly, exactly as this subtask's own tests do.
- No bugs found in the sibling `ods2` module while implementing this
  subtask.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean.

### 2026-09-23 — Subtask 6 complete

- Added `internal/rms/connect.go` (`SysConnect`, the SYS$CONNECT handler):
  reads the RAB's `RAB$L_FAB` to find its related FAB, reads that FAB's
  `FAB$W_IFI` to find the already-open `FileTable` handle (`RMS$_IFI` if
  it's stale or was never opened), copies the IFI into the RAB's own
  `RAB$W_ISI`, and — for a real ODS-2-backed file (the console case needs
  no further work) — arms it for record access via a small helper,
  `armForFAC`, that inspects the FAB's `FAB$B_FAC` to decide direction:
  `FAB$V_PUT` set constructs an `ods2/rms.Writer` (`odsrms.NewWriter`);
  `FAB$V_GET` set constructs an `odsrms.Reader`; neither bit set is
  `RMS$_PRV`, matching SYS$CREATE's own access-mode check. A second
  SYS$CONNECT against an already-armed FAB (real VMS lets more than one
  RAB share a FAB) reuses the existing Reader/Writer instance rather than
  constructing a second, independent one over the same file, which would
  otherwise let two write positions race over one linear file.
- Test coverage (`connect_test.go`): the console path; a real disk-file
  CONNECT that arms a Writer and confirms it's immediately usable for a
  `Put` (a live look ahead at subtask 7's own write path); an invalid/
  stale IFI (`RMS$_IFI`, with the RAB's `RAB$W_ISI` confirmed untouched);
  a FAB with neither `FAB$V_PUT` nor `FAB$V_GET` set (`RMS$_PRV`); a
  second CONNECT reusing rather than replacing an already-armed Writer;
  and a GET-direction CONNECT arming a Reader — built by writing and
  closing a file directly through `ods2`'s own `volume` API and reopening
  it fresh via `vol.OpenFID`, since SYS$OPEN (subtask 9) doesn't exist
  yet to produce a read-only handle through this package's own services.
- No bugs found in the sibling `ods2` module while implementing this
  subtask.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean.

### 2026-09-23 — Subtask 7 complete

- Added `internal/rms/put.go` (`SysPut`, the SYS$PUT handler): reads the
  RAB's `RAB$W_ISI` (set by SYS$CONNECT) to find the open file, rejects
  anything other than sequential access (`RAB$B_RAC` != `racSeq`,
  `RMS$_RAC`) and any stream not armed for writing (`RMS$_PRV` — covers
  both "the FAB never asked for `FAB$V_PUT`", which `armForFAC` already
  catches at CONNECT time, and "this RAB was armed for GET instead"),
  reads the outgoing record's bytes out of VAX memory via `RAB$L_RBF`/
  `RAB$W_RSZ`, and either writes them straight to the console (the
  `TTA0:` case, with a trailing newline appended — carried forward
  unchanged from the deleted Phase 10 stopgap's own `SYS$PUT` behavior,
  confirmed by reading that code back out of git history before
  reimplementing it) or calls the SYS$CONNECT-armed `odsrms.Writer.Put`
  for a real ODS-2 file. A `Writer.Put` failure (which, reading ods2's
  own `rms.Writer` source, only ever happens for a record whose length
  doesn't fit the file's declared record format) is reported as
  `RMS$_RSZ` — `status.go`'s own `rmsRecordTooBig` doc comment had
  already anticipated exactly this use back in subtask 4.
- Test coverage (`put_test.go`): the console path (record + newline
  landing in the fixture's console buffer); a real disk-file PUT,
  independently re-verified by closing the SYS$CONNECT-armed `Writer`
  directly (SYS$CLOSE is subtask 8, doesn't exist yet) and reading the
  record back with a fresh `odsrms.Reader` over a freshly reopened
  `volume.File`; three successive PUTs through the same RAB landing as
  three distinct, correctly-ordered records; a RAB that was never
  SYS$CONNECTed (`RMS$_IFI`); a non-sequential `RAB$B_RAC` (`RMS$_RAC`);
  a RAB armed for reading only, built the same way
  `TestSysConnect_readArming` builds its own read-only fixture
  (`RMS$_PRV`); and a record whose length doesn't match the target
  file's declared Fixed-format size (`RMS$_RSZ`).
- No bugs found in the sibling `ods2` module while implementing this
  subtask.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean.
