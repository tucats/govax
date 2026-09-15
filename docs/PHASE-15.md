# Phase 15: UX / ease-of-use support

## Goal

A catch-all phase for user-experience improvements to the `govax` command itself —
things that make the emulator pleasant to run standalone (outside this repo's own
`testdata/` tree) without being tied to any single emulated subsystem the way
Phases 00-14 are. Unlike those phases, this one isn't mapped to a single C source
file or a fixed deliverable list: it starts with one concrete sub-phase (file search
path support, below) and is expected to grow additional sub-phases over time as more
ease-of-use gaps are found, the same way `docs/DEVIATIONS.md` accumulates findings
rather than being written once.

**Status: not started.**

## Sub-phase 1: `-path` search path for unqualified file names

### Problem

`cmd/govax/main.go`'s `run` currently locates its required startup files —
`evax.dcl` (command grammar), `vax.help` (HELP text), `vax.init` (the startup
script) — via a single `-data` flag naming one directory
(`cmd/govax/main.go:36`, default `testdata/dcl`), joined directly onto each
filename. `kernel.asm` and `ssdef.asm` (and any other file an `ASM`/`INCLUDE`
console command names) are resolved separately, relative to whatever directory the
*referencing* file lives in (`internal/console/asm.go`'s `SetIncludeResolver`,
`internal/console/misc.go`'s `Console.Include`) — there is no shared, overridable
search strategy across any of these call sites, and nothing today lets `govax` run
from an arbitrary working directory using its own bundled copies of these files
without the caller pointing `-data` at a checkout of this repo's `testdata/` tree.

### Proposed behavior

- A repeatable `-path <dir>` command-line flag (`-path` may be given more than
  once; order given is search order — the Linux `PATH`-variable analogy the user
  described, but as repeated flags rather than a single `:`-joined string, matching
  Go's own `flag` package idioms rather than inventing a delimiter-splitting
  convention).
- Resolving an unqualified file name always tries the name exactly as given first
  (so an absolute path or an explicit relative path the caller already qualified is
  never overridden), then each `-path` directory in order, then finally an embedded
  fallback (below). This mirrors how the C reference's own `driver.c` looks in the
  current working directory first, per `reference/CLAUDE.md`.
- Per the user's own direction, this is a blanket policy, not a fixed five-file
  list: **every** place the Go code does an `os.ReadFile`/`os.Open` (or equivalent)
  against a filename that ultimately came from a console command or startup flag —
  not just `vax.init`/`vax.help`/`evax.dcl`/`kernel.asm`/`ssdef.asm` — should go
  through the shared resolver helper rather than calling `os`'s file functions
  directly. New file-reading code added after this phase lands should use the
  helper as a matter of course, the same way this project already expects new
  instruction opcodes to go through the dispatch table rather than a fresh
  `switch`.
- An embedded `embed.FS` (a Go `//go:embed` of a new package-local copy of
  `vax.init`, `vax.help`, `evax.dcl`, `kernel.asm`, `ssdef.asm`, and whatever else
  ends up on this list) is always the last, implicit entry in the search order —
  never named on the command line, always present. `govax` with **no** `-path`
  flags at all must still run correctly purely off the embedded copies, so a bare
  `go install`ed binary works standalone with no `testdata/` checkout nearby. This
  is the part of `cmd/govax/main.go`'s own doc comment (`main.go:8-10`, "go:embed
  isn't usable here since testdata/dcl isn't a subdirectory of this package") that
  motivates the new local copy: `go:embed` can only embed files under the embedding
  package's own directory tree, so the embedded fixtures need to actually live
  under `cmd/govax/` (or a package it imports), not merely reference
  `testdata/dcl` in place.
- One or more `-path` directories let a user override *any subset* of the embedded
  files by placing their own copy in a searched directory — the per-file search
  order (named-as-given, then each `-path` dir, then embedded) means a directory
  with only a customized `vax.help` still falls through to the embedded
  `vax.init`/`evax.dcl`/etc. for everything else, rather than requiring a complete
  replacement set.

### Key design questions

- **Where does this resolver live?** Likely a small new type (e.g. a `filepath`-like
  `SearchPath` in a new `internal/` package, or a method on `Console`) that every
  current direct file-read call site is switched over to, rather than each site
  growing its own copy of the same named-then-searched-then-embedded logic. Per the
  user's blanket-policy direction above, that means all of the following found by
  grepping the tree for `os.ReadFile`/`os.Open` (confirm this list is complete
  again once this sub-phase actually starts — new call sites may have landed in
  the meantime):
  - `cmd/govax/main.go` — grammar (`evax.dcl`), help (`vax.help`), init
    (`vax.init`).
  - `internal/console/misc.go`'s `Console.Include` (the `INCLUDE` command and
    `vax.init` itself).
  - `internal/console/asm.go`'s `Assemble` (the top-level `ASM <filename>` source)
    and its `SetIncludeResolver` closure (assembler `.INCLUDE`, e.g.
    `kernel.asm`/`ssdef.asm`).
  - `internal/console/help.go`'s `LoadHelpFile`.
  - `internal/console/dcl/define.go`'s grammar-file reader.
  - `internal/console/image.go` (the `RUN <filename>` `.exe` loader) — a
    user-supplied program path rather than a fixed bootstrap file, but still an
    `os.ReadFile` against a possibly-unqualified name, so it falls under the
    blanket policy.
  - `internal/console/rom.go` (two call sites — ROM image loading, e.g.
    `xdefault.rom`) — a binary boot image rather than a DCL-adjacent text file,
    but likewise in scope under the blanket policy rather than excluded.
- **How does an embedded-FS "path" compose with `-path`-provided real directories
  in one search abstraction**, given `embed.FS` and `os.DirFS`/plain
  `os.ReadFile` have different APIs (`fs.FS` is the natural common interface —
  `os.DirFS(dir)` for each `-path` entry, the `embed.FS` itself for the fallback,
  each tried via `fs.ReadFile` in order)?
- **Does `-data` get replaced outright, or kept alongside `-path` for one release**
  as a deprecated synonym for `-path testdata/dcl`? The user's description treats
  `-path` as the new, more general mechanism; no explicit instruction was given on
  `-data`'s fate — worth confirming before implementation rather than assuming.
- **Error reporting when a file is genuinely missing everywhere** (named-as-given
  fails, every `-path` directory misses, and the embedded copy is either absent
  from the list or itself doesn't have it): should list which locations were tried,
  not just the last (or first) attempted path's own `os.ErrNotExist`, so a user
  debugging a bad `-path` isn't left guessing.

### Deliverables (draft — expect revision once this sub-phase starts)

- A shared search-path resolver (name TBD) implementing "as-given, then each
  `-path` dir in order, then embedded `fs.FS` fallback."
- A new embedded copy of the required startup files under `cmd/govax/` (or an
  importable package), wired via `//go:embed`, kept in sync with (or replacing)
  the existing `testdata/dcl`/`testdata/asm` fixtures used by tests. Files with no
  natural embedded fallback of their own (a user's `RUN <filename>.exe`, an
  arbitrary ROM image) still route through the same resolver for the
  as-given/`-path`-search behavior — the embedded-`fs.FS` leg of the search is
  simply never populated for those names, not a reason to bypass the helper.
- Every direct `os.ReadFile`/`os.Open` call site enumerated in "Where does this
  resolver live?" above switched to call the shared helper instead, with no
  remaining direct-`os` file read anywhere in the tree for a name that ultimately
  came from a console command or startup flag.
- `cmd/govax/main.go`'s `-path` flag (repeatable), threaded through to every
  file-resolution call site named above.
- Tests: resolver unit tests (as-given precedence, `-path` order, embedded
  fallback, not-found-anywhere error listing every location tried), plus an
  integration test that `govax` with zero `-path` flags and an empty/irrelevant
  CWD still boots off the embedded files alone.
- Update `cmd/govax/main.go`'s own doc comment (currently explains *why* embedding
  wasn't used) once this changes that.

## Future sub-phases

None yet planned — add here as further UX/ease-of-use gaps are identified, each as
its own `## Sub-phase N: ...` section with the same shape (Problem / Proposed
behavior / Key design questions / Deliverables) as sub-phase 1 above.

## Progress Log

_Not started._
