# Phase 34 — CREATE/DIRECTORY

**Status:** in progress (2026-10-01): expanded with LIB$CREATE_DIR and a
new `internal/librtl` package (subtasks 10-15). Subtasks 1-9, the
console's `CREATE/DIRECTORY`, are done and match VMS 7.3 on the oracle run
(`testdata/credir`).

## Goal

govax's console gets VMS DCL's `CREATE/DIRECTORY`. It makes directories on
a mounted Files-11 volume the way VMS 7.3 makes them:

```
CREATE/DIRECTORY directory-spec[,...]
    /OWNER_UIC=uic | /OWNER_UIC=PARENT
    /VERSION_LIMIT=n
    /PROTECTION=(S:RWED,O:RWED,G:RE,W)
    /ALLOCATION=n
    /LOG
```

- **`/OWNER_UIC`** sets the new directory's owner. Without it, the owner is
  the process UIC, `[1,4]` (`rtl.NominalUIC`, or the live process's UIC
  when one exists). `PARENT` means the parent directory's owner.
- **`/VERSION_LIMIT`** sets the default version limit for files created in
  the new directory. Without it, the limit is the parent directory's. 0
  means no limit.
- **`/PROTECTION`** and **`/ALLOCATION`** take the defaults VMS uses
  (parent's protection minus delete access, and 1 block) unless the oracle
  shows otherwise.
- **Several levels at once.** `CREATE/DIRECTORY [A.B.C]` makes each missing
  level, with the same attributes.
- **An existing directory** isn't an error: `%CREATE-I-EXISTS`. With
  `/LOG`, each directory made is reported with `%CREATE-I-CREATED`.
- The spec goes through Phase 25/33's name processing: logical names, the
  default device and directory, and relative directories (`[.SUB]`,
  `[-.X]`).

## What exists already

The work is smaller than it first looks. ods2 already has most of the
pieces:

| Piece | Where | What it does today | Gap |
| --- | --- | --- | --- |
| `Volume.CreateDirectory(parent, name, versionLimit, bm, ib)` | `ods2/volume/writefile.go` | Header with `FchDirectory`, VAR records, the given version limit, and an entry in the parent | Owner and protection always come from the home block. The version is `NextVersion`, so a second `;2` can be made. No initial allocation (0 blocks). No `FchContig`. No check on the name's length. |
| `NewFileHeader` / `CreateHeader` | `ods2/volume/writeheader.go` | Writes a new primary header | No `Owner` or `Protection` fields |
| `UpdateHeader`, `SetVersionLimit` | `ods2/volume/updateheader.go`, `delete.go` | Rewrite an existing header in place | Nothing; govax's `ACPCreate` already sets the owner this way |
| `filespec.ResolveDirectory` | `ods2/filespec/glob.go` | Walks `[A.B.C]` to a `Directory` | Fails at the first missing level; there's no "create as you go" walk |
| ods2 CLI `CREATE DIRECTORY` | `ods2/cmd/ods2/internal/session/create.go` | One level, `/VERSION` only | The VMS policy (defaults, several levels) lives in the CLI, not in the library |
| govax `CREATE` verb | `internal/bootdata/files/console.dcl` | `CREATE/NAME_TABLE` only (Phase 25) | No `/DIRECTORY` |
| govax `DIRECTORY` | `internal/rms/directory.go` | `/FULL /FILE /SIZE /DATE` | No owner or protection is shown anywhere, so the result can't be seen |
| UIC and protection parsing | — | — | Neither exists: no `[g,m]` parser, and no `(S:RWED,...)` parser |

## Design

### Where VMS's rules live

ods2 is meant to be a library first (its README). The rules for how VMS
lays out a directory file and for making several levels go in ods2, so its
own CLI and govax share them:

- **`volume`** knows how a *single* directory file is laid out: its
  characteristics, record attributes, version `;1`, initial allocation and
  first block, and the owner and protection it's given.
- **`filespec`** (which already walks directory paths) gets the walk that
  makes the missing levels.
- **govax** owns DCL: the grammar, parsing UICs and protection strings,
  the process UIC, name processing, the messages, and host-side specs.

### ods2 API (proposed)

```go
// volume
type DirectoryOptions struct {
    VersionLimit uint16       // stored as given; the caller resolves inheritance
    Owner        *ondisk.Uic  // nil: the volume's default (home block)
    Protection   *uint16      // nil: the volume's default (home block)
    Allocation   uint32       // blocks to allocate at once; 0 = VMS's default
}
func (vol *Volume) CreateDirectory(parent *Directory, name string,
    opts DirectoryOptions, bm *Bitmap, ib *IndexBitmap) (*Directory, error)

// NewFileHeader gains Owner *ondisk.Uic and Protection *uint16, so the
// header is written once with the right values instead of created and then
// rewritten.

// filespec
type CreatedDirectory struct {
    Path    []string // e.g. ["A", "B"]
    Created bool     // false: it already existed
}
func CreateDirectoryPath(vol *volume.Volume, dirs []string,
    opts func(parent *volume.Directory) volume.DirectoryOptions,
    bm *volume.Bitmap, ib *volume.IndexBitmap) ([]CreatedDirectory, error)
```

`opts` is a callback so that the defaults that depend on the parent
(version limit, `/OWNER_UIC=PARENT`, protection minus delete) are worked
out for each level against *that* level's parent. That's what VMS does when
it makes `[A.B.C]` from nothing. Changing `CreateDirectory`'s signature
breaks only ods2's own CLI. govax doesn't call it today.

### On-disk details to match VMS 7.3

These are what VMS's `CREATE/DIRECTORY` is believed to write. Each is
confirmed or corrected by the oracle (subtask 1) before ods2 is changed:

- file characteristics `FCH$M_DIRECTORY | FCH$M_CONTIG`;
- record format VAR, maximum record size 512, `FAT$M_NOSPAN`;
- version always `;1`, and a directory that already exists (any version)
  gives "exists" and not a new version;
- name at most 39 characters, type `.DIR`;
- `/ALLOCATION` default 1 block, contiguous, with the first block holding
  an empty directory (the `0xFFFF` end marker, as ods2's `cf18a63` already
  keeps for every directory block), and EFBLK/FFBYTE set as VMS sets them;
- the header's backlink is the parent, and IDENT's dates, revision, and
  name (`NAME.DIR;1`) are as for any file;
- protection default: the parent's protection with delete removed for
  every category, unless the oracle shows otherwise;
- whether VMS also puts an ACL or other reserved area on the header.

### govax pieces

- **`internal/rms`**:
  - UIC and protection parsing come from ods2's `ondisk` (`ParseUic`,
    `ParseProtection`, `FormatProtection`; moved there during subtask 3
    so ods2's CLI shares them); `PARENT` is handled by the caller;
  - `Session.CreateDirectory(specText string, opts CreateDirectoryOptions)
    ([]CreatedDirectory, error)`. It resolves the spec with the shared
    name processing and checks that the device is mounted for writing. It
    rejects a file name, type, version, or wildcard in the spec. Then it
    calls `filespec.CreateDirectoryPath` and flushes the bitmaps.
- **`internal/console`**:
  - `console.dcl`: `CREATE` gets `/DIRECTORY` with
    `/syntax=create_directory`, as `/NAME_TABLE` has `create_name_table`.
    It has a list parameter and `allocation`, `log`, `owner_uic`,
    `protection` (list), and `version_limit` qualifiers.
  - `create.go`: the binding prints VMS's messages and counts a failure
    for one-shot exit status as other commands do. A bare `CREATE` (a file
    from the terminal) keeps saying it's unsupported.
- **`DIRECTORY /OWNER /PROTECTION`**, and both in `/FULL`'s output, so a
  new directory's attributes can be seen.

## Method: the oracle

This follows Phase 33's model: write it from the manual (the *OpenVMS DCL
Dictionary*'s CREATE/DIRECTORY, and the *I/O User's Reference*'s file
header layout), then check it against VMS 7.3:

- a DCL procedure runs on the author's VMS 7.3 system against an exchange
  volume govax builds. It makes directories covering each qualifier and
  default: plain, `/OWNER_UIC=[200,201]`, `/OWNER_UIC=PARENT`,
  `/VERSION_LIMIT=3`, `/PROTECTION=(...)`, `/ALLOCATION=4`, three levels
  at once, one that already exists, and one under a parent with a
  non-default limit and protection. It sends its `/LOG` output and
  `DIRECTORY/FULL` to a log file on the volume;
- a govax test makes the same directories on a copy of the volume as it
  was before the run. It compares each new directory's header field by
  field with VMS's, and the messages too. File IDs, dates, and LBNs are
  masked, since they depend on the order of allocation.

## Subtasks

Each ends with `go build`/`go vet`/`go test` and golangci-lint clean (in
whichever repo it touches), and a commit, plus `build -i` when it changes
behavior. **ods2 commits follow ods2's CLAUDE.md**: no `Co-Authored-By`
line, comments written for a newcomer to ODS-2, and tests in the same
commit.

1. **The oracle** (govax). `testdata/credir/`: the VMS procedure
   (`CREDIR.COM`), the govax console script that builds the exchange
   volume (`exchange.cmd`), and a README. The author runs it on VMS while
   subtasks 2–5 go ahead; its findings may change the "On-disk details"
   above.
2. **ods2 `volume`: owner, protection, and the directory file's layout.**
   `NewFileHeader.Owner`/`Protection`. `DirectoryOptions` and the new
   `CreateDirectory`, which makes version `;1` only, checks the name's
   length, sets `FchContig`, allocates contiguously, and writes the empty
   first block. Update ods2's CLI caller. Tests: header fields, `;1` only,
   allocation, and that `List`/`Insert` work in a directory with an
   allocated empty block.
3. **ods2 `filespec.CreateDirectoryPath`** and ods2's own CLI.
   Make each missing level and report each level as made or existing.
   ods2's `CREATE DIRECTORY` uses it and gains `/OWNER`, `/PROTECTION`, and
   `/ALLOCATION` (`docs/COMMANDS.md`). Tests: several levels, a partly
   existing path, MFD-level (`[FOO]`) creation, and a full volume partway
   through (what's left behind).
4. **UIC and protection parsing**, with tests, including malformed input
   and octal limits. (Done in ods2's `ondisk` during subtask 3; see the
   log.)
5. **govax `Session.CreateDirectory`.** Name processing, checks, the
   defaults (process UIC, parent's limit, parent's protection), and errors
   for a read-only volume, an unknown device, a missing parent with
   `/NOLOG`, a wildcard, and a file name in the spec. Tests on a
   govax-initialized volume.
6. **Console `CREATE/DIRECTORY`.** Grammar, binding, messages, and HELP
   (`internal/bootdata/files/vax.help`). Tests through the console,
   including `[.SUB]` against `SET DEFAULT`, a logical name, and a comma
   list.
7. **`DIRECTORY /OWNER /PROTECTION`**, and in `/FULL`.
8. **Reconcile with the oracle.** A test runs `CREDIR.COM`'s commands
   under govax on a copy of the pre-run volume, and compares with VMS's
   headers and log. Each difference is fixed, or masked with the reason
   recorded. Deviations from VMS go in `DEVIATIONS.md`.
9. **Close-out.** This doc, PLAN.md's index, CLAUDE.md's package notes,
   ods2's README feature list and COMMANDS.md, and HELP.

## Decisions

The author accepted each proposal below on 2026-10-01.

1. **Host-side specs.** When the default directory is a host directory
   (`Session.Locate` says host), should `CREATE/DIRECTORY [.X]` make a
   host directory (`os.MkdirAll`, ignoring owner, protection, and version
   limit, perhaps with a warning), or refuse? Decided: make it, and warn
   only when one of those qualifiers is given.
2. **Default owner.** You asked for the process UIC `[1,4]`. VMS has a
   wrinkle: in some cases a privileged user's subdirectory takes the
   parent's owner. Decided: always the process UIC, as you asked, and
   note any difference the oracle shows in `DEVIATIONS.md` without acting
   on it.
3. **Named UICs** (`/OWNER_UIC=[SYSTEM]` or `SYSTEM`). Decided: only
   numeric `[g,m]` and `PARENT` in this phase, since govax has no SYSUAF or
   rights database to translate names (beyond Phase 26's process record).
4. **`LIB$CREATE_DIR`.** Programs make directories through this RTL
   routine, and it would sit on the same core (`Session.CreateDirectory`).
   Decided: out of scope, as a follow-on, unless you'd like it as a
   subtask here.
5. **Tracking the ods2 work.** ods2 has its own `docs/PHASE-0n.md` series.
   Decided: this doc is the plan for both repos, and ods2 gets a short
   `docs/PHASE-04.md` that points here and logs its own commits. The
   alternative is to keep it all here.

## Expansion: LIB$CREATE_DIR and `internal/librtl`

Added 2026-10-01 at the author's request. Programs make directories with
the RTL's LIB$CREATE_DIR, which can sit on the same core as
`CREATE/DIRECTORY`. Shims that emulate LIBRTL.EXE go in a new package,
`internal/librtl`, and later *RTL.EXE emulations each get a package of
their own.

**Clean room.** LIB$CREATE_DIR is written from its documentation, the
*VMS Run-Time Library Routines Volume: Library (LIB$) Manual* (VMS 5.0,
AA-LA76A-TE, pp. LIB-35 to LIB-39), and checked against VMS 7.3 by a probe
program. VMS's source is not read.

**What the manual says.**

```
LIB$CREATE_DIR device-directory-spec [,owner-UIC] [,protection-enable]
               [,protection-value] [,maximum-versions]
               [,relative-volume-number]
```

- *device-directory-spec* (descriptor): an RMS directory specification,
  with or without a device; no node, name, type, version, or wildcard;
  at most 255 characters.
- *owner-UIC* (longword by reference): 0 or omitted means the parent
  directory's owner (except a UIC-format directory, such as [123,321],
  whose UIC is used).
- *protection-enable*, *protection-value* (words by reference): set bits
  of the enable mask take the value's bits; clear bits take the parent's
  protection, except that the parent's delete access is never passed on.
  Enable omitted or 0: the parent's protection, less delete.
- *maximum-versions* (word by reference): omitted means the parent's
  default limit; 0 means no limit.
- *relative-volume-number* (word by reference): placement in a volume
  set; govax's volumes are single, so it's accepted and ignored.
- Returns SS$_CREATED (one or more made), SS$_NORMAL (all existed),
  LIB$_INVARG (spec missing or over 255 characters), LIB$_INVFILSPE (no
  explicit directory; a node, name, type, version, or wildcard; not a
  disk), or what $PARSE, $QIO, and the rest return.

**Decisions (2026-10-01, the author).**

5. *Move every LIBRTL shim into `internal/librtl` now*: LIB$ADAWI,
   STR$UPCASE, LIB$GET_VM, LIB$FREE_VM, LIB$DELETE_VM_ZONE, LIB$SIGNAL,
   LIB$STOP, LIB$ESTABLISH, LIB$REVERT, LIB$SIG_TO_RET, and
   LIB$MATCH_COND, not only the new one.
6. *Owners default to the parent's*, for LIB$CREATE_DIR (its manual) and
   for `CREATE/DIRECTORY` too (what VMS 7.3 did for `[OWNED.CHILD]`),
   replacing Decisions 2.
7. *A program's call means a mounted volume.* LIB$CREATE_DIR never makes
   a host directory; the console's host fallback stays the console's.
8. *A VAX probe* settles what the manual leaves open (statuses for an
   unmounted device or a too-deep path, the protection arithmetic,
   arguments past the sixth).

**Design.**

- `rtl` keeps the process-level machinery: the condition dispatcher, the
  call frames, and the heap that DECC$MALLOC shares with LIB$GET_VM. It
  exports what an RTL package needs: the CPU and memory, reading a string
  descriptor, starting a software signal, setting the caller's handler,
  turning a condition into a return, allocating and freeing VM, and the
  shim table to register into.
- `librtl` holds each routine's documented interface, and a table of
  routines (name, LIBRTL transfer-vector offset, shim code, function).
  The console registers the functions into each new RTL environment and
  builds the `SHIM$LIBRTL_<offset>` stubs from the same table. Existing
  shim codes are kept; LIB$CREATE_DIR gets the next, 39. A test checks
  each offset against LINK's captured LIBRTL symbol table
  (`vmsdef.ImageSymbols`).

**Subtasks.**

10. **`internal/librtl`, and the move.** The rtl export API; the
    routines and their tests move; the console wires the package in.
    `TestConditions*` and the other RUN tests keep passing unchanged.
11. **Owners default to the parent's** (Decision 6): `rms`,
    `CREATE/DIRECTORY`, HELP; the oracle mask and DEVIATIONS entry go.
12. **LIB$CREATE_DIR** in `librtl`, over `rms.Session.CreateDirectory`
    with the masks (enable and value), volume-only resolution, and the
    statuses above. Tests drive it through the shim.
13. **The probe.** `testdata/credir/libcrd.mar` calls LIB$CREATE_DIR for
    each case and writes R0 and the case to a dump file; a command
    procedure builds and runs it on VMS 7.3; a govax script builds its
    exchange volume. The author runs it.
14. **Reconcile** with the probe's run: statuses and directory headers.
15. **Close-out**, again.

## Out of scope

- Plain `CREATE` (a file from terminal input), `CREATE/FDL`, and the rest
  of CREATE's forms.
- `SET DIRECTORY`, `SET PROTECTION`, `SET FILE/OWNER` (an obvious follow-on
  using the same parsers and `UpdateHeader`), and `DELETE` of a directory
  (VMS needs it empty and its protection to allow delete; Phase 23's
  `DELETE` should refuse a non-empty one, which subtask 5's tests check).
- ACLs, and enforcing access checks against UIC and protection. govax
  runs everything as a fully privileged `[1,4]` process.
- Multi-volume sets (relative volume numbers).

## Progress log

- 2026-10-01: Plan written, after surveying ods2's `volume`/`filespec`
  and govax's console, `rms`, and DCL grammar.
- 2026-10-01: The author reviewed the plan and accepted every proposed
  answer to its open questions (now "Decisions"). Subtasks proceed with a
  commit after each.
- 2026-10-01: Subtask 1 done (`testdata/credir/`): `credir.com` covers
  each qualifier, its defaults, and the errors; `exchange.cmd` builds the
  exchange volume, which is waiting for the author's VAX run.
- 2026-10-01: Early facts from the Phase 33 container
  (`testdata/mar/rms3/vax/rms3-vax.dsk.gz`), whose `[TEST]`, `[OUT]`, and
  the rest VMS 7.3 made with plain `CREATE/DIRECTORY` as `[1,4]`:
  characteristics `0x2080` (DIRECTORY, CONTIG); VAR records, RAT `NOSPAN`,
  RSZ and MRS 512; HIBLK 1, EFBLK 2, FFB 0, high-water mark 2; first block
  `FFFF` then zeros; IDENT revision **0** (ordinary files VMS wrote read
  1); protection the parent's less delete (MFD `0xBA00`, directories
  `0xBA88`); owner `[1,4]`; the parent's entry carries the parent's
  default version limit (no limit, 32767), as ods2's `Insert` already
  writes it.
- 2026-10-01: Subtask 2 done, in ods2 (`eb28f6a`, with `docs/PHASE-04.md`
  there logging this phase's ods2 work). `volume.CreateDirectory` takes
  `DirectoryOptions` and writes the layout above; `NewFileHeader` gains
  `Owner` and `Protection`; a header with the directory characteristic
  starts at IDENT revision 0. A directory that exists in any version is
  `ErrExists`, not a new version. A failure part way frees the header and
  space before anything is entered in the parent. govax's tests that call
  `CreateDirectory` follow the new signature.
- 2026-10-01: Subtasks 3 and 4 done, in ods2 (`e6f8b1d`, `bc8784d`).
  - **Parsing moved to ods2.** `ondisk.ParseUic` (`[g,m]` or `<g,m>`,
    octal, group up to 37776 and member up to 177776), `ParseProtection`
    (categories by any abbreviation, `:` or `=`, a category left out keeps
    the base mask's field), `FormatProtection`, and `ProtectionAccess`.
    The plan had these in govax's `internal/rms`; in `ondisk`, ods2's CLI
    uses the same code, so subtask 4 was folded into subtask 3.
  - **`filespec.CreateDirectoryPath`** makes each missing level, asking a
    callback for each level's options given its own parent, and reports
    which levels it made, also on failure (levels made stay made).
    `volume.InheritedDirectoryOptions` gives the parent-derived defaults.
  - **ods2's CLI** `CREATE DIRECTORY` makes several levels, reports an
    existing directory (the MFD too) as `%CREATE-I-EXISTS`, and gains
    `/OWNER`, `/PROTECTION`, `/ALLOCATION` (COMMANDS.md, README).
- 2026-10-01: Subtask 5 done: `Session.CreateDirectory`
  (`internal/rms/createdir.go`).
  - It resolves the spec with `expandSpec`, so logical names, the
    default, and `[.SUB]`/`[-.X]` work. A search list creates in its first
    element, as `$CREATE` does. It refuses a name, type, version,
    wildcard, or `...` (`ErrNotDirectorySpec`), an unmounted device
    (`*NotMountedError`), a read-only volume, a version limit over 32767,
    and a bad protection, each before anything is made.
  - `CreateDirectoryOptions.forParent` is `CreateDirectoryPath`'s
    callback: the process UIC (or `/OWNER_UIC`, or `PARENT`'s owner), the
    parent's limit, and the parent's protection less delete with
    `/PROTECTION`'s categories laid over it.
  - Each level comes back with its display name (`DUA0:[A.B]`, through a
    concealed logical name if there was one) and whether it was made.
  - **Host directories** (decision 1): with no device and no default on a
    volume, or a host path, the directory is made on the host, relative
    to the current directory (`[.A.B]` and `[A.B]` are `A/B`, `[-.A]` is
    `../A`).
  - **Bug fixed in passing:** `expandSpec` probed each translation for a
    device by parsing it with an empty default directory, so any
    `[-.X]` failed as "goes above the master file directory", for
    DIRECTORY, DELETE, and every other command too. It now probes with
    the default directory.
- 2026-10-01: Subtask 6 done: the console's `CREATE/DIRECTORY`.
  - **Grammar** (`console.dcl`): `CREATE` gains `/DIRECTORY` with
    `syntax create_directory` (ids 1501-1506): a list of directories,
    `/OWNER_UIC` (`$any`, so `[200,201]` and `PARENT` pass through),
    `/VERSION_LIMIT`, `/PROTECTION` (a list, so `(S:RWED,...)` and
    `W:RE` both work), `/ALLOCATION`, and `/LOG`.
  - **Binding** (`internal/console/create.go`, bound in `logical.go` with
    `CREATE_NAME_TABLE`): the process UIC is the RTL process's, or
    `rtl.NominalUIC` before INIT. Bad qualifier values are
    `CLI_BADQUALIFIER`, before anything is made. In a list, a directory
    that fails is reported and the rest are still made; the command then
    fails with its message inhibited (exit status only).
  - **Messages:** a new CREATE facility (`vmserrors/codes_create.go`):
    `%CREATE-I-CREATED` (with `/LOG`), `%CREATE-I-EXISTS` (always, for an
    existing last level), `%CREATE-E-DIRNOTCRE` (with the cause). The
    texts are VMS's as best known; subtask 8 checks them against the log.
  - **HELP** (`vax.help`): `HELP CREATE` is now an overview, with
    `HELP CREATE /DIRECTORY` and `/NAME_TABLE` for the two forms. A bare
    `CREATE` says `/DIRECTORY or /NAME_TABLE` is required.
  - Tests (`create_test.go`) drive the real grammar: levels and `/LOG`,
    every qualifier, `[.SUB]`, `[-.X]`, a logical device, and errors.
  - **Noticed for subtask 7:** DIRECTORY's heading for the MFD prints
    `Directory DUA1:[]`, not `[000000]`.
- 2026-10-01: Subtask 7 done: `DIRECTORY /OWNER /PROTECTION`.
  - `rms.DirectoryOptions` gains `Owner` and `Protection`; `/FULL` shows
    both. The owner is written `[g,m]` (octal); VMS shows an identifier
    name such as `[SYSTEM]` where one exists, which govax has no rights
    database to look up. Protection is VMS's `(RWED,RWED,RE,)` form.
    Both columns are checked against the oracle log in subtask 8.
  - **Bug fixed:** DIRECTORY headed the MFD `Directory DUA1:[]`; it's now
    `[000000]`, as VMS writes it, in govax and in ods2's CLI (ods2
    `f3c474b`). Four govax tests expected the old heading.
  - HELP DIRECTORY lists the new qualifiers, and now says what `/FILE`
    shows (the file ID, not the record format).
- 2026-10-01: Subtask 8 started: `TestCreateDirectoryOracle`
  (`internal/console/credir_oracle_test.go`) replays `credir.com`'s
  commands under govax on a freshly built exchange volume and compares
  each directory with VMS's, field by field: IDENT name and revision,
  characteristics, owner, protection, record attributes, HIBLK/EFBLK/FFB,
  high-water mark, version limit, the version limit on its parent's
  entry, and its extent count. File IDs, LBNs, and dates aren't
  compared. It skips until `testdata/credir/vax/credir-vax.dsk.gz`
  exists; checked against a govax-made stand-in, it reports nothing.
  Comparing the messages with `CREDIR.LOG` waits for the log's format.
  **Waiting on the author's VAX run.** One expected difference to look
  for: govax makes a ninth level (`[L1...L9]`), which ODS-2 on VMS 7.3 is
  believed to refuse.
- 2026-10-01: Subtask 8 done. The author ran `credir.com` on VMS 7.3 on
  the exchange volume itself (`testdata/credir/vax/credir-vax.dsk.gz`).
  `TestCreateDirectoryOracle` now matches every directory's header and
  all 36 CREATE/DIRECTORY commands' 45 message lines. What the oracle
  changed:
  - **ods2** (`480534a`): a new directory's entry in its parent has no
    version limit, whatever the parent's default (VMS: 32767 under
    `[LIMITED]`, whose default is 3). A directory's new space is zeroed
    and its high-water mark set past it (`/ALLOCATION=4`: HWM 5), never
    lowered after. A path deeper than 8 levels, or a name over 39
    characters, is refused before anything is made
    (`volume.ErrDirectoryName`).
  - **Messages** (`internal/console/create.go`, `vmserrors`): `/LOG`
    reports only the directory asked for, not the levels made above it.
    EXISTS names the spec as typed (`[PLAIN] already exists`).
    DIRNOTCRE is `<spec> directory file not created`, then a secondary
    status from VMS's message table: `-RMS-F-DIR` (too deep, name too
    long), `-LIB-F-INVFILSPE` (file name, wildcard, `...`, unparsable),
    `-SYSTEM-W-NOSUCHDEV` (unknown device; `DEVNOTMOUNT` for a known
    one), `-SYSTEM-F-WRITLCK` (read-only). A bad `/OWNER_UIC` is
    `%CREATE-F-SYNTAX, error parsing '...'` and `-SYSTEM-F-IVIDENT`, and
    makes nothing; a bad `/PROTECTION` gets the same first line (VMS's
    wasn't probed).
  - **`/VERSION_LIMIT=40000`**: VMS reports
    `%CREATE-E-BADVALUE, '40000' is an invalid keyword value` and still
    creates the directory, ignoring the qualifier; govax does too (and
    fails the command's status, its message shown).
  - **Owner, by decision:** VMS gave `[OWNED.CHILD]` (no `/OWNER_UIC`,
    parent owned by `[200,201]`) its parent's owner. govax keeps the
    process UIC (Decisions 2); masked in the test and logged in
    DEVIATIONS.md. So is DIRECTORY/OWNER's `[1,4]` where VMS shows
    `[SYSTEM]`.
  - The replay now copies `[ALLOC]FIRST.DAT` in from a host file, so the
    directory's first entry is compared too; the log is read as text
    (it's VFC).
- 2026-10-01: Subtask 9, close-out (the phase was later expanded; see
  "Expansion"). PLAN.md marks the phase done;
  CLAUDE.md notes `Session.CreateDirectory` and the oracle; HELP CREATE
  /DIRECTORY says what `/LOG` reports and how a bad `/VERSION_LIMIT` is
  handled. ods2's README, COMMANDS.md, and `docs/PHASE-04.md` cover its
  side. Follow-ons left out of scope: `LIB$CREATE_DIR` (Decisions 4),
  `SET FILE/OWNER`/`SET PROTECTION` on the same parsers, and the default
  owner, should the author want VMS's rule (DEVIATIONS.md).
- 2026-10-01: Expanded with LIB$CREATE_DIR and `internal/librtl`
  (subtasks 10-15, Decisions 5-8), from the LIB$ manual's description.
- 2026-10-01: Subtask 10 done: `internal/librtl`, and every LIBRTL shim
  moved into it.
  - **rtl's export API** (`internal/rtl/export.go`): `CPU`, `Memory`,
    `Shims`, `StringDescriptor`, `Signal` (LIB$SIGNAL/LIB$STOP's
    dispatch), `SetCallerHandler`, `ConditionToReturn` (LIB$SIG_TO_RET's
    unwind), and the heap: `AllocateVM`, `FreeVM`, `FreeVMZone`. The
    dispatcher, frames, and heap stay in rtl; signal.go, unwind.go, and
    memory.go lost their shim functions, and math.go went.
  - **librtl** (`doc.go`, `routines.go`, `math.go`, `strings.go`, `vm.go`,
    `condition.go`): LIB$ADAWI, STR$UPCASE, LIB$GET_VM, LIB$FREE_VM,
    LIB$DELETE_VM_ZONE, LIB$SIGNAL, LIB$STOP, LIB$ESTABLISH, LIB$REVERT,
    LIB$SIG_TO_RET, LIB$MATCH_COND, with their old codes. `Routines` holds
    each one's offset and code; a test checks the offsets against
    `vmsdef.ImageSymbols` and that codes are distinct.
  - **Console:** `newRTL` registers librtl's routines; `shimTable` takes
    its LIBRTL rows from `librtl.Routines`. A new test checks every stub's
    code is distinct and registered.
  - **Tests:** the routine tests moved to librtl and drive the routines
    through the shim table; rtl's dispatcher tests call the exported
    methods. The RUN tests of conditions pass unchanged.
  - Found while moving: rtl's allocator takes a flag before the zone, so
    `AllocateVM` passes both; LIB$GET_VM still asks for zone 0, keeping
    eVAX's behavior (its zone-id was never applied).
  - **A limit to watch:** VMInit reserves one 512-byte page for shim stubs,
    room for 42; there are 38, 39 with LIB$CREATE_DIR. More LIBRTL routines
    will need a bigger reservation.
- 2026-10-01: Subtask 11 done: owners default to the parent's
  (Decisions 6). ods2's `InheritedDirectoryOptions` gives the parent's
  owner (ods2 `a1b26bc`, so its CLI follows); `rms.CreateDirectoryOptions`
  loses `ProcessUIC` and `OwnerParent` (`/OWNER_UIC=PARENT` is now the
  default spelled out), and the console its process-UIC lookup. The
  oracle's `[OWNED.CHILD]` mask and the DEVIATIONS entry are gone:
  `TestCreateDirectoryOracle` matches VMS with nothing masked. HELP
  CREATE /DIRECTORY says so.
- 2026-10-01: Subtask 12 done: LIB$CREATE_DIR (`internal/librtl/
  createdir.go`, shim code 39, LIBRTL+0xA28), from the LIB$ manual.
  - `rms.CreateDirectoryOptions` gains the manual's masks
    (`ProtectionEnable`, `ProtectionValue`: enabled bits from the value,
    the rest from the parent's protection less delete) and `VolumeOnly`
    (Decisions 7: no host directory; reaching no volume is
    `ErrNotDirectorySpec`).
  - The routine reads its arguments by reference (a 0 address or a short
    argument list is an omitted argument), and returns SS$_CREATED or
    SS$_NORMAL; LIB$_INVARG for a missing or over-255-character spec;
    LIB$_INVFILSPE for no explicit directory, a node, name, type,
    version, or wildcard, or no mounted volume; RMS$_DIR for a name too
    long or a path too deep; RMS$_DEV for a device not mounted;
    SS$_WRITLCK for a read-only volume; SS$_ACCVIO for an argument it
    can't read. relative-volume-number is read and ignored.
  - **UIC-format directories**, which the manual documents: `[123,321]`
    makes `123321.DIR` (each part padded to three octal digits), owned by
    that UIC unless owner-UIC says otherwise.
  - **Unsettled until the probe:** the status for a device that isn't
    mounted (RMS$_DEV is a guess from "any condition values returned by
    $PARSE"), whether a spec whose directory comes only from a logical
    name counts as explicit (govax requires a bracket in the text), and
    what VMS 7.3 does with arguments past the sixth.
- 2026-10-01: Subtask 13 done: the LIB$CREATE_DIR probe.
  `testdata/credir/gen.go` writes `libcrd.mar` (34 cases, each a CALLG
  with its own argument list; R0 per case to `LIBCRD.DMP`), `libcrd.com`
  (MACRO, LINK, RUN, with CRDDEV and CRDLOG defined, then DIRECTORY
  listings), and `libcrd.cmd` (the exchange volume, built and waiting for
  the author's VAX run). `TestLibCreateDirOracle` already assembles,
  links, and runs the probe under govax (govax's MACRO and LINK, its
  LIBRTL stubs) and compares with VMS's once the container is in
  `testdata/credir/vax/`; until then it logs govax's statuses and skips.
  The header comparison is shared with `TestCreateDirectoryOracle`
  (`compareDirectories`).
- 2026-10-01: Subtask 14: the probe's VMS 7.3 run
  (`testdata/credir/vax/libcrd-vax.dsk.gz`). It ended at case 33, a 0
  descriptor address: VMS **signals an access violation** (VA 4) rather
  than returning a status. Of the 32 cases before it, govax differed on
  five, now matching:
  - `[-.UP]` above the MFD is RMS$_DIR (ods2 `filespec.ErrAboveMFD`,
    `fa56922`, mapped to `volume.ErrDirectoryName` by `rms`).
  - `CRDLOG:`, a directory only through a logical name, counts as named
    and is created; `CRDDEV:`, a device only, is still LIB$_INVFILSPE.
    `rms` records whether a translation names a directory
    (`resolvedSpec.Explicit`) and `CreateDirectoryOptions.RequireDirectory`
    replaces librtl's bracket test.
  - `NOSUCH:[X]` is SS$_NOSUCHDEV (SS$_DEVNOTMOUNT for a known device that
    isn't mounted), not RMS$_DEV.
  - A **seventh argument** is the initial allocation: VMS gave `[P8]` 4
    blocks for a longword 4 there. govax reads it as a longword by
    reference.
  - An unreadable argument, a 0 descriptor address included, signals
    SS$_ACCVIO (reason 0, the address) through rtl's `Signal`, the
    hardware's signal array.
  - Every owner, protection (the manual's %XDBFF/%X37FF example too), and
    version limit matched. One field doesn't, masked and logged in
    DEVIATIONS.md: `[SUBREL]`'s MFD entry has a version limit of 1 on VMS.
  - **The probe** now runs "no arguments" next to last and the 0
    descriptor last, in a subroutine that establishes LIB$SIG_TO_RET, so
    the access violation comes back as SS$_ACCVIO (govax: 0xC). The test
    matches cases by name, from the `LIBCRD.MAR` on VMS's volume, so the
    first run still checks its 32; the two it never reached are logged.
    The exchange volume is rebuilt for a run of the current probe, which
    would check those two.
