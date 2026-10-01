# Phase 34 — CREATE/DIRECTORY

**Status:** in progress (2026-10-01).

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
