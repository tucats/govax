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

**Status: sub-phases 1-2 complete.**

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

## Sub-phase 2: `-instruction-limit`/`-time-limit` runaway-program guards

### Problem

Nothing bounds how long a single `GO`/`CALL`/`STEP` command can run the emulated
CPU. A program under development with a genuine infinite loop (or one that's
merely much slower than expected) hangs the whole interactive session — the only
recourse is killing the process, losing whatever console state (breakpoints,
deposited memory, symbol table) had accumulated. Requested by the user, inspired
directly by Phase 14's own testing, where several dead-end debugging runs needed
an artificial step cap (`internal/console/run_test.go`'s `callBounded`) that has
no interactive-session equivalent.

### Proposed behavior

- Two new `govax` flags, both optional and defaulting to unlimited ("infinite"):
  `-instruction-limit <n>` (maximum instructions a single run may execute) and
  `-time-limit <d>` (maximum wall-clock time, a standard Go duration string —
  `5s`, `15ms`, etc.). `0` (the flag's own zero value when unset) means
  unlimited for both, rather than a separate "was this flag given" tracking
  field — the ordinary convention for this kind of CLI limit flag.
- Applies to every top-level "run the CPU" console operation (`GO`/`EXEC`,
  `CALL`, `STEP`, and `RUN` — which funnels through `CALL`) uniformly, each
  getting its own fresh budget rather than a single pool shared across the
  whole session: a runaway program in one command shouldn't consume the budget
  of an unrelated later one, and time genuinely spent stopped at a breakpoint
  or typing the next command must never count against the time limit.
- When a limit is reached, the run stops cleanly and reports it (`%VAX-I-
  INSTRLIMIT`/`%VAX-I-TIMELIMIT`, matching this project's existing `%VAX-`
  message style), the same way a `HALT` or a breakpoint already does — not a
  crash, not a Go-level error propagated up through the dispatcher.
- Both flags are applied only *after* `vax.init`'s own boot sequence finishes,
  not during it — a limit meant to catch a runaway *user* program shouldn't
  also cut short the emulator's own startup script.
- Explicit non-goal, per the user: avoid any `time` package cost when
  `-time-limit` isn't set at all — no `time.Now()` call on the hot per-
  instruction path unless a time limit is actually configured.

### Key design questions

- **Where does the budget reset?** Not globally per `govax` process (see above)
  — each call into `Console.Execute`/`Call`/`Step` resets it, matching "a
  runaway program's own debt doesn't roll over."
- **What, precisely, counts as "engine time"?** Wall-clock time only across the
  actual `Engine.Step` loop inside one `Execute`/`Call`/`Step` call — console-
  side work (breakpoint formatting, `Printf` output, waiting for the next
  command at the prompt) never starts the clock. This also means the time
  limit measures real process wall-clock time, not "CPU time spent emulating"
  in some more precise sense — pausing the *Go process itself* in a debugger
  mid-run elapses the deadline in the background, so a resumed run reports the
  time limit as reached almost immediately; see `Engine.BeginRun`'s own doc
  comment for this caveat in full. `-instruction-limit` has no such issue and
  is the better choice when debugging `govax`'s own Go code with a debugger.

### Deliverables

- `internal/cpu/limits.go`: `Engine.SetLimits`/`BeginRun`, `ErrInstructionLimitExceeded`/
  `ErrTimeLimitExceeded`, wired into `Engine.Step` as its own first check.
- `internal/console/execute.go`'s new `Console.reportStopReason` (shared by
  `Execute`/`Step`/`Call`): prints the matching message and returns `nil` for
  `ErrHalted`/`ErrInstructionLimitExceeded`/`ErrTimeLimitExceeded` alike,
  propagates anything else unchanged. `Execute`/`Step`/`Call` each call
  `Engine.BeginRun` once at the start of their own loop.
- `cmd/govax/main.go`'s `-instruction-limit`/`-time-limit` flags (`flag.Int`/
  `flag.Duration`), applied via `Engine.SetLimits` right after `vax.init`
  finishes, before the interactive prompt loop starts.
- Tests: `internal/cpu/limits_test.go` (instruction/time limits stop a genuine
  infinite loop, `0` means unlimited for either, `BeginRun` gives each run a
  fresh budget); `internal/console/execute_test.go`'s two new cases (the
  console-level message and per-command-fresh-budget behavior);
  `cmd/govax/main_test.go`'s `TestRun_instructionLimitStopsARunawayProgram`/
  `TestRun_timeLimitStopsARunawayProgram` (full CLI-flag-to-console path, an
  interactively deposited infinite loop actually gets stopped).

## Future sub-phases

None yet planned — add here as further UX/ease-of-use gaps are identified, each as
its own `## Sub-phase N: ...` section with the same shape (Problem / Proposed
behavior / Key design questions / Deliverables) as sub-phase 1 above.

## Progress Log

### 2026-09-15 — Sub-phase 1 complete

- Resolved the two open design questions before implementing (see "Key design
  questions" above): the user confirmed `-data` is removed outright rather than
  kept as a synonym or left alongside `-path`; the resolver lives as a small,
  nil-receiver-safe `*Resolver` type in a new `internal/respath` package (an
  `fs.FS`-based common interface — `os.Open`/`os.DirFS`-shaped real directories,
  the embedded `embed.FS` fallback — rather than a `Console` method, so it's usable
  from `cmd/govax/main.go` before a `Console` exists).
- `internal/respath.Resolver`: `Open`/`ReadFile` implement the as-given → `Dirs` in
  order → `Fallback` `fs.FS` search; `NotFoundError` lists every location tried.
  A nil `*Resolver` (every method's receiver is nil-safe) behaves as a plain,
  unwrapped `os.ReadFile`/`os.Open` with no search/fallback — this is what keeps
  the large existing body of `console.New(...)`-based tests working unchanged,
  since none of them assign a `Console.Paths`. `WithDir` returns a copy that
  additionally searches one more directory ahead of the configured list, used by
  the assembler `.INCLUDE` resolver below to keep searching alongside the
  including file's own directory (kernel.asm's own `ssdef.asm` `.include`) without
  losing `-path`/embedded fallback for names that aren't there.
- `internal/bootdata`: a `//go:embed files` of copies of `vax.init`, `vax.help`,
  `evax.dcl`, `kernel.asm`, `ssdef.asm` (copied from `testdata/dcl`/`testdata/asm`
  — go:embed can only embed files under its own package directory, confirming the
  reason `main.go`'s old doc comment gave for not embedding), exposed as
  `bootdata.FS fs.FS` via `fs.Sub` so names match the bare filenames a resolver
  looks up (`"vax.init"`, not `"files/vax.init"`).
- Every call site the "Where does this resolver live?" survey found now goes
  through `Console.Paths` (a new `*respath.Resolver` field) or, in `main.go`
  before a `Console` exists, the resolver built there directly: `cmd/govax/main.go`
  (grammar/help/init — now `resolver.ReadFile` + `dcl.ParseGrammar`/
  `console.ParseHelp` directly rather than through `LoadGrammarFile`/
  `LoadHelpFile`, which stay as plain literal-path convenience wrappers unchanged,
  still used by existing tests), `internal/console/misc.go`'s `Console.Include`,
  `internal/console/asm.go`'s `Assemble` and its `SetIncludeResolver` closure
  (via `c.Paths.WithDir(filepath.Dir(path))`), `internal/console/image.go`'s
  `imageLoad` (the path `findImage` already resolved to an existing file, so this
  always hits the as-given leg — wrapped for policy consistency, not because it
  changes behavior), `internal/console/rom.go`'s `LoadROM`/`LoadNVRAM` (`Open`, not
  `ReadFile`, to keep their existing incremental `io.ReadFull` reads).
  `internal/console/dcl/define.go`'s `LoadGrammarFile` and
  `internal/console/help.go`'s `LoadHelpFile` were deliberately left as
  plain-path-only utilities (no resolver parameter) rather than threading a
  resolver through every test call site that already passes a concrete
  `testdata/...` path — `main.go`'s own grammar/help loading calls their
  underlying `ParseGrammar`/`ParseHelp` directly against resolver-read bytes
  instead of calling through them, which was enough to keep the "no direct `os`
  read for a name that came from a console command or startup flag" bar met
  without changing either function's signature.
- `cmd/govax/main.go`: `-data` removed; `-path` is a repeatable flag (a small
  `flag.Value` implementation appending each occurrence, in order given) threaded
  into `respath.New(paths, bootdata.FS)`, assigned to `Console.Paths` before
  `Init`/`Include` run. Updated the package doc comment (previously explained why
  embedding *wasn't* used).
- Verified end-to-end by hand, not just unit tests: built the `govax` binary and
  ran it from `/private/tmp` (no repo, no `testdata/`, no `-path`) — `evax.dcl`,
  `vax.help`, `vax.init`, and (via `vax.init`'s own `asm "kernel.asm"`, which
  `.include`s `ssdef.asm`) both assembler source files all resolved purely from
  `internal/bootdata`'s embedded copies, reaching "Building Microkernel..." and an
  actual (unrelated) assembly-deposit failure — i.e. real file discovery, not just
  a grammar-load success. The remaining mid-script errors on that run (VM size vs.
  the minimal machine's default memory, `LOAD/ROM/NOERROR`'s `/NOERROR` qualifier
  not being stripped before the filename argument, `exe$initialize` undefined) are
  pre-existing script-execution gaps unrelated to file *discovery* — the same ones
  `TestRun_startupBootsFromEmbeddedFilesAlone`'s own doc comment already expected
  and tolerates (matching `TestRun_startupDoesNotFatallyFail`'s original comment,
  which this test replaces) — not something this sub-phase's scope covers.
- Tests: `internal/respath/respath_test.go` (as-given precedence over both
  `-path` dirs and the fallback, dir search order, fallback, not-found-anywhere
  `NotFoundError.Tried` listing every location, nil-receiver plain-read behavior,
  `WithDir` precedence). `cmd/govax/main_test.go` replaced its old
  `-data`-based tests with `TestRun_startupBootsFromEmbeddedFilesAlone` (this
  sub-phase's own named deliverable: zero `-path` flags still boots),
  `TestRun_pathOverridesEmbeddedForThatFileOnly` (a `-path` dir holding only a
  customized `vax.init` is used for `vax.init` while `evax.dcl`/`vax.help` still
  fall through to the embedded copies — the "partial override" behavior called out
  in "Proposed behavior" above), and `TestRun_asGivenPathWinsOverPathFlag` (a
  `vax.init` in the working directory beats a `-path`-supplied one).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean.

### 2026-09-15 — Sub-phase 2 complete

- `internal/cpu/limits.go`/`engine.go`: `Engine.SetLimits`/`BeginRun`, new
  `ErrInstructionLimitExceeded`/`ErrTimeLimitExceeded` sentinels, `Step`'s own
  `checkLimits` as its first action (refuses to start a new instruction once
  either budget is exhausted; nothing about machine state changes when it
  does). `BeginRun` costs nothing (no `time.Now()`) when no time limit is
  configured, per the user's own explicit requirement.
- `internal/console/execute.go`'s `reportStopReason` centralizes the "print a
  message, return nil" handling `Execute`/`Step`/`Call` previously duplicated
  for `ErrHalted` alone, now covering the two new sentinels too
  (`%VAX-I-INSTRLIMIT`/`%VAX-I-TIMELIMIT`); each of those three now calls
  `Engine.BeginRun()` once before its own `Engine.Step` loop.
- `cmd/govax/main.go`: `-instruction-limit`(`flag.Int`)/`-time-limit`
  (`flag.Duration`) added; `run`'s own signature grew the two values,
  applying them via `Engine.SetLimits` right after `vax.init` finishes
  (deliberately not during boot — see "Proposed behavior" above), before the
  interactive prompt loop starts.
- Tests: `internal/cpu/limits_test.go` (a genuine NOP+BRB self-loop stopped
  by either limit, `0` meaning unlimited for both, `BeginRun` giving a
  second run its own fresh budget rather than inheriting the first's
  exhaustion); `internal/console/execute_test.go`'s
  `TestExecute_stopsAtInstructionLimit`/
  `TestExecute_beginRunGivesEachCommandAFreshBudget`; `cmd/govax/main_test.go`'s
  `TestRun_instructionLimitStopsARunawayProgram`/
  `TestRun_timeLimitStopsARunawayProgram`, each driving a real interactive
  session (embedded `vax.init` boots, then `D`/`GO` commands injected via the
  same `io.ReadCloser` mechanism the rest of this file's tests use) that
  deposits an infinite loop and confirms `GO` returns control instead of
  hanging the test.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean.

## Follow-up ideas not pursued in this sub-phase

- `internal/console/image.go`'s `findImage` (the `RUN <filename>` `.exe` loader's
  own `.exe`-suffix/`SharePrefix`/lowercase-fallback search) is a distinct,
  purpose-built resolution mechanism predating this phase — it wasn't folded into
  `respath.Resolver`'s own search order, since its semantics (suffix guessing,
  `SharePrefix`) don't map cleanly onto "as-given / `-path` dirs / embedded" and
  the C source's own `find_image` doesn't work that way either. Worth a look if a
  future sub-phase wants `RUN` to honor `-path` too (today it only checks the
  literal name, `SharePrefix`+name, and lowercase variants of both — not `-path`).
- The pre-existing `LOAD/ROM/NOERROR` qualifier bug noticed during hand-testing
  above (`/NOERROR` isn't stripped before the filename argument, so
  `parseRomOrNvramArg` treats the literal text `"/NOERROR"` as the file name) is
  unrelated to file *discovery* and wasn't touched here — worth its own fix or a
  `docs/DEVIATIONS.md` entry (it's console/DCL parsing, not emulated VAX ISA
  behavior, so this project's bug-fixing policy in `CLAUDE.md` doesn't obviously
  require the latter, but it's not this sub-phase's own scope either way).
