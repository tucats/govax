# Phase 23: Console commands to support using Files-11 containers

## Goal

Round out govax's operator-facing command set for the ODS-2 containers Phase 22
made mountable, so a `govax` console session can do everything the separate
`ods2` module's own `cmd/ods2` interactive session can do, without needing that
second tool at all. Phase 22 already delivered `MOUNT`/`DISMOUNT`; this phase adds
the rest of `cmd/ods2`'s command surface:

- `INITIALIZE/CONTAINER` — format a new, empty container file (not yet mounted).
  `INITIALIZE` becomes a single, unified verb: `/CONTAINER` selects this new
  behavior, `/VAX` selects the existing VAX-memory-allocation behavior
  currently spelled `INIT` (see "`INITIALIZE`: unifying `INIT` and
  `INITIALIZE` under one verb" below) — `INIT` keeps working as DCL's own
  ordinary abbreviation of the full word, not as a second, separate verb.
- `DIRECTORY` — list files on a mounted volume.
- `SET DEFAULT` / `SHOW DEFAULT` — the operator's current default device/directory,
  so a partial file spec (`DIRECTORY *.TXT`, `TYPE FOO.TXT`) resolves the way it
  would in a real VMS session.
- `DELETE` — reclaim a file's storage.
- `PURGE` — trim old versions of a name down to a keep count.
- `COPY` — host-to-container, container-to-host, and container-to-container
  copying, disambiguated by a new `/HOST` qualifier (see "COPY direction and the
  `/HOST` qualifier" below).
- `TYPE` — write one file's content to the console.

**Status: planning only (this document). No implementation has started — see
"Subtasks" below for the intended breakdown once work begins.**

## Why this phase looks different from most others

Like Phase 22, this phase has no `reference/eVAX` counterpart at all: the real
eVAX C source never implemented ODS-2 volume access, so there is no C file to
port from. Its behavioral reference is instead `github.com/tucats/ods2`'s own
`cmd/ods2/internal/session` package (`directory.go`, `delete.go`, `purge.go`,
`copy.go`, `type.go`, `initialize.go`, `setshow.go`, `tokenize.go`) — the real,
already-tested implementation of every one of these commands against `ods2`'s
public `filespec`/`volume`/`ondisk`/`diskimage` API, read here purely as a
*behavioral* spec.

That package cannot be imported directly, though, even if it were otherwise
convenient: it lives at `cmd/ods2/internal/session`, and Go's own `internal/`
visibility rule confines it to code inside the `ods2` module tree. Every
operation this phase adds has to be a fresh implementation against `ods2`'s
public packages, living in `internal/rms` — the sole place in this project
already allowed to import `ods2` (per Phase 22's own package-boundary rule) —
with a thin, console-specific wrapper in `internal/console`, exactly mirroring
Phase 22's `MountTable`/`internal/console/mount.go` split. Reading `ods2`'s CLI
source is this phase's equivalent of Phase 22 reading `fab.h`/`rab.h`: the
authoritative description of what the finished behavior should look like, not
code to link against.

## Scope

- `github.com/tucats/ods2`'s `cmd/ods2/internal/session` package — read-only
  behavioral reference for every new command's semantics (matched functionally,
  not literally ported, per the `internal/` visibility note above).
- `github.com/tucats/ods2`'s public `filespec`, `volume`, `ondisk`, `diskimage`
  packages — the real API this phase's `internal/rms` code calls into, same as
  Phase 22.
- `internal/rms` — gains the actual container-operation implementations (a new
  `Session`-shaped type holding the operator's current default spec, plus
  per-command files) alongside its existing `MountTable`/RMS-service code.
- `internal/console` — gains one new thin wrapper file per command (mirroring
  `mount.go`), plus `dispatch.go` changes for `SET DEFAULT` and for unifying
  `INIT`/`INITIALIZE` onto the DCL grammar (below).
- `internal/console/dcl` — gains a new grammar-language/parser feature:
  parameter-scoped qualifiers, needed for `COPY`'s `/HOST` (see its own design
  section below). This is the one piece of this phase that isn't "add a
  command" — it's an engine capability none of the twelve existing verbs
  needed.
- `internal/bootdata/files/evax.dcl` — new verbs, in the same "govax-native
  extension, diverges from `testdata/dcl/evax.dcl`" block Phase 22 started
  (see that file's own divergence comment, right above `verb mount`).
- `internal/bootdata/files/vax.init` — the live boot script's bare
  `init ^d4096` line updates to `init/vax ^d4096` once `/VAX` becomes
  mandatory (see "Fallout" below); `testdata/dcl/vax.init` and
  `reference/eVAX/vax.init`
  (read-only upstream-import copies) are left untouched, same convention
  Phase 22 established for `evax.dcl`.

## Design decisions

### `INITIALIZE`: unifying `INIT` and `INITIALIZE` under one verb

`internal/console/dispatch.go`'s `Dispatch` checks a hardcoded `fixedCommands`
map, keyed by **the first 4 characters of the verb**, before ever consulting the
DCL grammar (`console_dispatch_table`'s own real-C-source split between
hand-written commands and DCL-driven ones — see `dispatch.go`'s own doc comment
at the top of the file). `INIT` (the VAX-memory-allocation command,
`Console.Init`, one of the oldest commands in this codebase, predating the DCL
grammar engine's own existence) is one of those fixed entries. Typing
`INITIALIZE/CONTAINER path size` today would truncate to `"INIT"`, match that
entry, and hand `/CONTAINER path size` to `cmdInit`'s VAX-memory-size expression
evaluator — never reaching the DCL grammar at all. Confirmed directly against
`dispatch.go`'s `readCommandVerb`/`verb4` truncation logic, and independently by
the user while this doc was being drafted.

The fix isn't just relocating `INIT` onto the DCL grammar as a second,
separate verb next to a new `INITIALIZE` — per the user's explicit direction,
**`INIT` and `INITIALIZE` are one and the same verb**, invokable under either
spelling, with the VAX-memory-allocation behavior and the new container-format
behavior selected by qualifier: `/VAX` for the former, `/CONTAINER` for the
latter. Neither is a default — a bare `INITIALIZE` (or `INIT`) with no
qualifier at all is invalid; there is no meaningful "do nothing in particular"
form of this verb. The **documented, canonical name is `INITIALIZE`**; `INIT`
is not a second verb or an explicit alias, it's simply DCL's ordinary
unambiguous-abbreviation matching doing its job — `Grammar.matchVerb`
(`internal/console/dcl/match.go`) already resolves any unambiguous prefix of a
declared verb name, and nothing else in this grammar starts with `INIT`, so the
4-letter form keeps working automatically once the verb is declared under its
full name. This is exactly the mechanism `/syntax=`-redirecting qualifiers
already use elsewhere (`DEFINE`'s `/LOGICAL` vs. `/DEVICE` split,
`define_logical`/`define_device` in `internal/bootdata/files/evax.dcl`) — no
new grammar-engine capability is needed here, only using the existing pattern
correctly: a bare verb with no parameters of its own and no bound handler,
whose only two qualifiers each redirect to their own fully-parameterized
syntax. Neither the parsing engine nor `Dispatch` needs to enforce "one of
`/VAX`/`/CONTAINER` is required" specially: since the top-level `initialize`
entry has no handler bound to it, typing the verb with no qualifier (so it
never redirects anywhere) falls through to `Grammar.Dispatch`'s existing
"no handler bound for this verb" error — the same fallback every other
currently-unbound syntax in this file already relies on (see Phase 22 subtask
2's own progress-log note on this mechanism).

Draft shape, refined during implementation:

```text
verb initialize

    qualifier   vax/id=.../syntax=initialize_vax
    qualifier   container/id=.../syntax=initialize_container

syntax initialize_vax
    parameter   pages/id=.../type=$rest_of_line

syntax initialize_container
    parameter   path/id=.../type=$string/prompt="Container file"
    parameter   size/id=.../type=$integer/prompt="Size in blocks"
    parameter   label/id=.../type=$string
    qualifier   cluster/id=.../type=$integer
```

`initialize_vax`'s `pages` parameter carries no `/prompt=` (so it isn't a
formally *required* parameter in the grammar's own sense) because the existing
`cmdInit` handler's behavior when nothing is typed is a specific, named error
(`CLI_NEEDPAGES`) rather than an interactive re-prompt — the new handler
checks `r.Present("PAGES")` explicitly and returns that same error when it's
absent, preserving today's wording/behavior exactly rather than letting a
formal `/prompt=` requirement produce a different message. `ZERO` (the only
other fixed command whose name could plausibly collide with something) needs
no change — its own name is exactly 4 characters, never at risk of shadowing a
longer verb the way `INIT`/`INITIALIZE` did.

**Fallout.** `INIT`'s existing test coverage moves onto the grammar-based path
the same way Phase 22 subtask 2 moved `internal/console/dcl`'s and
`dispatch_test.go`'s direct grammar-file loads. More importantly, this is a
real, user-visible behavior change, not just an internal refactor: **plain
`INIT <size>` (or `INITIALIZE <size>`) with no qualifier stops working** —
`/VAX` becomes mandatory. The one place in this tree that currently relies on
the old bare form is `internal/bootdata/files/vax.init`'s own boot script
(line 9, `init ^d4096` — confirmed by grep), which this port actually executes
on every boot; it needs updating to `init/vax ^d4096` (or the `INITIALIZE`
spelling) as part of this same subtask. Per Phase 22's own established
convention for this exact file pair, `testdata/dcl/vax.init` and
`reference/eVAX/vax.init` (the read-only upstream-import copies, confirmed
identical to each other and to `internal/bootdata/files/vax.init`'s
pre-Phase-23 line 9 by grep) are **left untouched** — only the live,
actually-executed `internal/bootdata/files/vax.init` copy changes, the same
divergence-going-forward situation Phase 22 already created for `evax.dcl`
itself. No other test or fixture in the tree currently dispatches a bare
`"INIT ..."` command line (checked by grep across `*_test.go` and
`testdata/`); existing tests that exercise VAX memory initialization call
`Console.Init` directly rather than through `Dispatch`'s string parsing, so
they're unaffected by the verb-spelling/qualifier change.

`internal/rms`'s handler calls `diskimage.Create` + `volume.Initialize` directly
(exactly `ods2`'s own `cmdInitialize`), and — matching real VMS's own
`INITIALIZE`, and `ods2`'s own version, which format a device without mounting
it — does **not** mount the result. An operator runs `MOUNT` separately
afterward, same as with any other container.

### The operator's current default device/directory (`SET DEFAULT`/`SHOW DEFAULT`)

`ods2`'s own CLI session carries one piece of state none of Phase 22's work
needed: `Session.Default filespec.Spec`, the "current working directory" a
partial file spec resolves against (`filespec.Parse(text, s.Default)`).
`DIRECTORY`, `DELETE`, `PURGE`, `COPY`, and `TYPE` all need the equivalent.

Since `internal/console` may not import `ods2` types directly (the same rule
that keeps `filespec.Spec` out of that package today), this state has to live
in `internal/rms`, behind an opaque handle `internal/console` just holds and
passes through. Proposed shape (naming to be finalized while implementing,
matching Phase 22's own "sketched, refined in the progress log" pattern):

```go
// internal/rms package
type Session struct {
    Mounts  *MountTable      // reuses Console's existing MountTable, not a copy
    Default filespec.Spec
}

func (s *Session) SetDefault(text string) error   // filespec.Parse(text, s.Default)
func (s *Session) DefaultString() string          // s.Default.String(), for SHOW DEFAULT
```

`Console` gains one new field — **resolved in subtask 3 as
`ContainerSession *rms.Session`** (not `Files`: that would risk colliding in
a reader's mind with `rms.Context.Files`, the *`VAX-program`* open-file
table from Phase 22, which is a genuinely different thing) — constructed
once alongside `Devices`/`Logicals`/`Mounts` and never reset by `ZERO` — same
lifetime rule Phase 22 gave `Mounts` (`machine.go`'s own doc comment on why).

Every other new command (`DIRECTORY`/`DELETE`/`PURGE`/`COPY`/`TYPE`) resolves
its file-spec argument(s) against `session.Default` via a small shared
`internal/rms` helper (`resolveVolume(specText string) (*volume.Volume,
filespec.Spec, error)`, parsing against `Default` and then looking the parsed
device up in `Mounts`), rather than each command re-deriving the same two-step
lookup independently.

`SET DEFAULT` is **not** a new DCL verb — `SET` is already one of
`dispatch.go`'s `fixedCommands` (a hand-parsed `NAME=value`/sub-verb command,
`cmdSet`), and adding a `case "DEFAULT":` there (calling
`d.Console.SetDefault(after)`, a thin console-layer wrapper around
`ContainerSession.SetDefault` per the "Status-code / error translation"
section below) is the natural, minimal extension —
mirroring exactly how `ods2`'s own `cmdSet` adds a `"default"` case alongside
its register/symbol assignment. `SHOW DEFAULT`, by contrast, **is** a new DCL
grammar entry: `verb show` already exists with a large family of `syntax
show_*` entries (`internal/bootdata/files/evax.dcl`'s `show_map`/`show_tb`/...),
and `show_default` slots in the same way, bound in `dispatch.go`'s
`bindGrammar` alongside the others.

### COPY direction and the `/HOST` qualifier

`ods2`'s own `COPY` disambiguates which direction a copy runs by inspecting
whether the *destination* argument names a location on a mounted volume
(`device:[dir]name.type`) or an ordinary host path (`volumeDestination` in
`copy.go`) — container-to-host is the default, container-to-container is
implicit once the destination turns out to be volume syntax, and `/HOST` (a
single, command-wide qualifier there) flips the *source* argument's own
interpretation for the fourth combination, host-to-container.

The user's requested govax syntax is more explicit and, per their examples,
resolves any ambiguity per-argument rather than through one global flag:

```text
COPY FOO.TXT BAR.TXT                          ! container -> container
COPY foo.txt/HOST BAR.TXT                     ! host -> container
COPY FOO.TXT BAR.TXT/HOST                     ! container -> host
COPY "/Users/tom/foo.txt" /HOST  FOO.TXT      ! same as line 2, host path quoted
```

The last line matters: it shows `/HOST` can trail its parameter with a space
between them (not just written directly attached, as in line 2) whenever the
parameter itself needed quoting — for instance because it starts with `/` and
so has to be quoted to keep it from ever being *mistaken* for a qualifier
introducer in the first place (see "Quoting" below). So the actual rule isn't
"no whitespace between the parameter and its qualifier"; it's "the qualifier
appears somewhere after that parameter's value and before the next parameter
begins" — real DCL's own notion of a qualifier scoped to the parameter it
follows, not to the whole command line.

**This is a capability the current `internal/console/dcl` engine does not
have.** Today, `Entry.Qualifiers` is one flat list per verb/syntax, and
`Result`'s internal map records only "was `/HOST` present anywhere on this
line" — with no way to tell "attached to parameter 1" from "attached to
parameter 2" apart, which `COPY` needs since either of its two file-spec
parameters can independently carry `/HOST`. Per the user's own explicit
direction ("extend DCL if it doesn't support parameter-specific qualifiers"),
this phase extends the grammar engine itself:

- `Parameter` gains its own `Qualifiers []*Qualifier` list, populated by
  `qualifier` statements nested under a `parameter` statement in the grammar
  text (a new, additive construct — every qualifier declared the existing way,
  directly under a `verb`/`syntax`, is entirely unaffected).
- `Parse`'s main loop (`internal/console/dcl/parse.go`) tracks the most
  recently filled positional `*Parameter` as it goes. When it encounters a
  `/name` token (its existing `pos[0] == '/'` check, today always routed to
  the *entry's* qualifier list), it tries matching `name` against that last
  parameter's own `Qualifiers` list **first**; only if that lookup misses does
  it fall back to today's entry-level `matchQualifier` call, exactly
  unchanged. This ordering is what makes the change purely additive: none of
  the twelve existing verbs declares any parameter-scoped qualifiers, so the
  new lookup always misses for them and every existing grammar file parses
  identically to today.
- `Result` gains a parallel, parameter-namespaced accessor
  (`ParamPresent(paramName, qualName string) bool` at minimum;
  `ParamString`/`ParamKeyword` if a future parameter-scoped qualifier ever
  needs a value, though `/HOST` itself is a plain switch) — kept in separate
  storage from the existing flat `values` map rather than key-mangled into
  it, so a qualifier name that happens to also exist at the entry level can't
  collide with its parameter-scoped namesake.
- `COPY`'s grammar declares `qualifier host` twice — once nested under its
  `SOURCE` parameter, once under `DESTINATION` — since which occurrence
  `/HOST` binds to is determined purely by *where the parser was* when it saw
  the token (which parameter it had just finished reading), not by any
  static, single grammar-level association. This only works because each
  parameter's own value token is read in full (including through a quoted
  string) before the parser goes looking for a following qualifier — already
  true of the existing loop structure, not a new invariant this change has
  to introduce.
- Deliberately out of scope: a qualifier written *before* the parameter it's
  meant to modify (real VMS DCL supports this in some cases; none of this
  phase's commands need it, since every example the user gave has `/HOST`
  trailing its file spec).

**Resolved during subtask 2: how the grammar *text* itself marks a qualifier
statement as nested, concretely.** The bullets above describe *Parse*'s
runtime resolution order, but say nothing about how `define.go`'s
grammar-definition parser is supposed to tell a parameter-scoped `qualifier`
statement apart from an ordinary entry-level one while reading the grammar
file — and it turns out mere textual adjacency ("this `qualifier` statement
immediately follows a `parameter` statement") can't be the signal:
`internal/bootdata/files/evax.dcl`'s own pre-existing `DEFINE/DEVICE` syntax
already has a `parameter name/prompt="Name"` immediately followed by twenty
more `qualifier` statements (`CLUSTER`, `CYLINDERS`, ...) that are genuinely
entry-level, never meant to be scoped to `NAME`. Using adjacency would have
silently misattributed all twenty. Implemented instead as an explicit,
opt-in `/parameter=<name>` switch on the `qualifier` statement itself, naming
the already-declared parameter it nests under — consistent with how every
other structural link in this grammar language (`/syntax=`, `/alias=`) is
already spelled out as a switch rather than inferred from position:

```text
parameter   source/id=.../type=$string/prompt="Source"
qualifier   host/id=.../parameter=source

parameter   destination/id=.../type=$string/prompt="Destination"
qualifier   host/id=.../parameter=destination
```

A `/parameter=` naming a parameter not yet declared on the current
verb/syntax is a grammar-definition-time error (`CLI_PARAMNOTFOUND`), not a
silent fallback to entry-level — matching how every other named
cross-reference in this file (`/syntax=`, `/alias=`, `/type=`) is validated.
`Qualifier.Alias` on a parameter-scoped qualifier resolves against that same
parameter's own qualifier list, not the whole entry's — aliasing across
scopes isn't supported and no command in this project needs it.

New `internal/console/dcl` unit tests (alongside `parse_test.go`) cover: a
parameter-scoped qualifier attached with no space, with a space, on the first
vs. second parameter of a two-parameter entry, a qualifier name absent from
that parameter's own list correctly falling through to entry-level matching
unaffected, and the existing full grammar file (`internal/bootdata/files/
evax.dcl`) continuing to parse identically once the new nested-`qualifier`
grammar-text construct is added to `define.go`/`ParseGrammar`.

### Quoting: already supported, needs test coverage, not a fix

Re-reading `internal/console/dcl/parse.go` while planning this phase: double-
quoted parameter/qualifier values are already handled (`readValueToken`'s
quote branch, `upcaseOutsideQuotes` preserving case and any embedded `/`
inside quotes). Critically, `Parse`'s per-iteration qualifier check
(`pos[0] == '/'`) only fires when the *current* character is a bare `/` — a
quoted value starting with `/` begins with `"` instead, so it is never
misread as a qualifier introducer in the first place; no engine change is
needed for `COPY "/Users/tom/foo.txt" ...` to parse correctly. This phase's
job here is verifying that with real test coverage (a quoted host path,
including one immediately followed by a space then `/HOST`, and one
containing an embedded space of its own), not fixing anything.

### COPY qualifier parity

Per the user's direction: full parity with `ods2`'s own `COPY` qualifier set
(`/QUIET`, `/VERBOSE`, `/TEST`, `/BINARY`, `/TIME`, `/IGNORE`, `/DIRS`,
`/STREAM`, `/VFC`, `/CRLF`, `/LF`, plus the new `/HOST`), landed incrementally
across two subtasks rather than all at once: core container-to-container
copying plus `/HOST` first (subtask 9), the remaining format/qualifier
behavior second (subtask 10) — see Subtasks. `/TIME` (preserving a host file's
mtime) only has meaning for a container-to-host copy, same restriction
`ods2`'s own `cmdCopy`/`cmdCopyFromHost` already apply; `/IGNORE`, `/DIRS`,
`/STREAM`, and the line-ending qualifiers are likewise host-file-format
concerns that `ods2`'s own doc comments already explain don't carry over to a
volume destination — this phase's implementation follows those same documented
restrictions rather than re-deciding them.

### Status-code / error translation

Matches Phase 22's `Mount`/`Dismount` convention (`internal/console/mount.go`):
each thin console-layer wrapper checks the relevant precondition itself
(file not found, device not mounted, version required for `DELETE`, ...)
before or after calling into `internal/rms`, and translates a plain Go error
into a real, literal VMS status via `vmserrors`. `internal/rtl/status.go`
already carries two unused-since-Phase-22-subtask-3 constants anticipating
exactly this need — `ssNoSuchFac`/`ssNoSuchFile` — good fits for `DELETE`/
`TYPE`/`COPY`'s own "not found" cases; any further codes this phase needs
(e.g. for `DIRECTORY`/`PURGE`) get added to that same literal table rather
than invented ad hoc. Unlike Phase 22's FAB/RAB-facing `RMS$_` codes, these
are all operator-console-facing `SS$_` statuses, the same class MOUNT/DISMOUNT
already use — there's no VAX-program-visible FAB/RAB status field involved in
any of this phase's commands at all (`DIRECTORY`/`DELETE`/`PURGE`/`COPY`/
`TYPE`/`INITIALIZE` are all pure operator/console actions, never called via a
`SYS$` system service from VAX code).

### Test fixtures

Same two-tier strategy Phase 22 established: the committed automated suite
builds its own throwaway containers per-test via `ods2`'s own
`diskimage.Create`/`volume.Initialize` (hermetic, no binary blob under
`testdata/`), while `testdata/disks/rq0-ra92.dsk`/`empty.dsk` back a small
number of opt-in, `t.Skip`-when-absent interop checks — most usefully here,
`DIRECTORY` listing the real VAX/VMS system disk's MFD read-only, since that
container was never created by anything this project wrote.

## Subtasks

1. **Done.** Unify `INIT`/`INITIALIZE` onto the DCL grammar as a single `verb
   initialize` with `/VAX` and `/CONTAINER` syntax-redirecting qualifiers (no
   handler bound to the bare verb, so a qualifier is effectively mandatory —
   see the design section above), moving `INIT`'s existing VAX-memory-
   allocation behavior onto `initialize_vax` unchanged. Update
   `internal/bootdata/files/vax.init`'s bare `init ^d4096` to `init/vax
   ^d4096` (`testdata/dcl/vax.init`/`reference/eVAX/vax.init` untouched).
   `INITIALIZE/CONTAINER`'s own `initialize_container` syntax is stubbed in
   the grammar here too (parameters only) but its `internal/rms` handler
   isn't implemented until subtask 4 — this subtask is scoped to the
   verb-unification/fallout fix, not new container-formatting behavior.
2. **Done.** `internal/console/dcl`: parameter-scoped qualifier support (`Parameter.
   Qualifiers`, `define.go`'s nested-`qualifier`-statement parsing, `parse.go`'s
   last-filled-parameter-aware qualifier resolution, `Result.ParamPresent`
   and friends). Grammar-engine-only; no console command uses it yet. Full
   unit test coverage per the "Quoting"/"COPY direction" design sections.
3. **Done.** `internal/rms`: the `Session` type (`Default filespec.Spec` plus the
   shared `resolveVolume` helper), `Console`'s new field, `SET DEFAULT`
   (`dispatch.go`'s `cmdSet`) and `SHOW DEFAULT` (new grammar `syntax
   show_default`, bound in `bindGrammar`) wired end to end.
4. **Done.** `INITIALIZE/CONTAINER`: `internal/rms` handler
   (`diskimage.Create`+`volume.Initialize`, no auto-mount) for the
   `initialize_container` syntax stubbed in subtask 1, console wrapper,
   tests.
5. **Done.** `DIRECTORY`: `internal/rms` implementation (glob, per-directory
   grouping, `/FULL`/`/FILE`/`/SIZE`/`/DATE` formatting, matching `ods2`'s
   own output shape closely enough to be recognizable, not necessarily
   byte-identical), grammar, console wrapper, tests — including the opt-in
   `rq0-ra92.dsk` read-only-listing interop check.
6. `DELETE`: required-version enforcement (matching `ods2`'s own "bare
   `DELETE FOO.TXT` never defaults to a version" rule), grammar, console
   wrapper, tests.
7. `PURGE`: `/LIMIT=n` (default 1), grammar, console wrapper, tests.
8. `TYPE`: single-match-only (no wildcards, matching `ods2`), record-format-
   aware text rendering (VFC/Fixed/Variable/Stream, reusing the same
   record-to-text logic `COPY`'s own text path needs — see subtask 10;
   whichever of `TYPE`/`COPY` lands first implements it, the other reuses
   it), grammar, console wrapper, tests.
9. `COPY`, core: grammar (`SOURCE`/`DESTINATION` parameters, `/HOST`
   declared under each per the parameter-scoped-qualifier design), all four
   direction combinations working for plain container-to-container and
   `/HOST` copying with no other qualifier, `internal/rms` implementation,
   console wrapper, tests (including the quoted-host-path cases from the
   "Quoting" design section).
10. `COPY`, full qualifier parity: `/BINARY`, `/QUIET`, `/VERBOSE`, `/TEST`,
    `/TIME`, `/IGNORE`, `/DIRS`, `/STREAM`, `/VFC`, `/CRLF`, `/LF`, each
    matched against `ods2`'s own documented semantics/restrictions per
    direction (see "COPY qualifier parity" above), tests per qualifier.
11. End-to-end acceptance pass: a scripted session exercising
    `INITIALIZE/CONTAINER` -> `MOUNT` -> `SET DEFAULT` -> file creation
    (reusing Phase 22's existing `SYS$CREATE`/`SYS$PUT` path, or `COPY/HOST`
    from a host fixture) -> `DIRECTORY` -> `TYPE` -> `COPY` (container ->
    host) -> `DELETE` -> `PURGE` -> `DISMOUNT`, plus whatever further opt-in
    `testdata/disks/` interop checks subtask 5 didn't already cover.
12. Docs: this file's progress log; `docs/PLAN.md` phase-table row + narrative
    paragraph; `docs/DEVIATIONS.md` entries for anything ambiguous found
    comparing `ods2`'s own CLI behavior against real VMS DCL conventions
    along the way (this phase's `AUDIT.md`-equivalent, per `CLAUDE.md`'s
    bug-fixing policy — though note that policy is framed around VAX ISA/
    hardware fidelity, and most of this phase's judgment calls are DCL-
    engine/UX decisions instead, which is why they're captured directly in
    this doc's own "Design decisions" rather than deferred to
    `DEVIATIONS.md`).

## Open questions

- ~~Exact final naming for the new `internal/rms.Session` type and the
  `Console` field that holds it~~ **Resolved in subtask 3:** the type is
  `internal/rms.Session` as sketched, and the `Console` field is
  `Console.ContainerSession *rms.Session` — chosen specifically to avoid
  reading as a collision with `rms.Context.Files` (a running VAX program's
  own open-file table, Phase 22, an unrelated concept).
- `DELETE`/`PURGE`'s inherited-from-`ods2` restriction to single-device
  volumes: not expected to matter in practice, since Phase 22's `MountTable`
  never mounts a multi-device volume set anyway, but worth a one-line
  confirmation in subtask 6's own progress-log entry once implemented.
- Whether `DIRECTORY`'s output formatting should aim for closer visual parity
  with real VMS `DIRECTORY` (column-aligned multi-file-per-line listings)
  rather than `ods2`'s own simpler one-file-per-line style — `ods2`'s own
  `formatDirectoryEntry` doc comment already flags this as a deliberate
  simplification on its side; this phase can either inherit that
  simplification (fastest, keeps the two tools' output comparable) or invest
  in closer VMS fidelity. Leaning toward inheriting it for now, matching this
  phase's general "ods2's CLI is the reference" framing, but flagging it since
  it's a visible, easily-second-guessed choice.

## Progress Log

### 2026-09-23 — Planning

- Surveyed `github.com/tucats/ods2/cmd/ods2/internal/session`'s `copy.go`/
  `directory.go`/`delete.go`/`purge.go`/`type.go`/`initialize.go`/
  `setshow.go`/`tokenize.go` for this phase's behavioral reference, and
  confirmed that package's Go `internal/` visibility rules out importing it
  directly — every command here is a fresh `internal/rms` implementation
  against `ods2`'s public API, not a call-through, mirroring Phase 22.
- Read `internal/console/dcl`'s full parser (`parse.go`/`define.go`/
  `grammar.go`/`match.go`) to design the parameter-scoped-qualifier
  extension `COPY`'s `/HOST` needs, and confirmed quoted-string handling
  (needed for a host path like `"/Users/tom/foo.txt"`) already works
  correctly with no engine change — see "Quoting" above.
- User flagged, and this doc's own drafting independently confirmed by
  tracing `dispatch.go`'s `Dispatch`/`readCommandVerb`, that `INITIALIZE`
  would silently collide with the existing fixed `INIT` command via
  `fixedCommands`'s blind first-4-characters lookup.
- Confirmed with the user via two targeted questions: an initial (later
  revised, see below) direction that `INITIALIZE` be a real DCL verb with a
  required `/CONTAINER` qualifier redirect (matching `DEFINE`'s
  `/LOGICAL`-vs-`/DEVICE` precedent), and that `COPY`'s qualifier set targets
  full parity with `ods2`'s own `COPY`, landed incrementally across two
  subtasks (core + `/HOST` first, the rest of the format/qualifier behavior
  second) — the `COPY` decision stands; see below for how the `INITIALIZE`
  question was refined.
- User clarified the actual intent was narrower than "just fix the
  collision": `INIT` and `INITIALIZE` are meant to be **one verb**, invokable
  under either spelling, with the VAX-memory-allocation behavior and the new
  container-format behavior selected by `/VAX`/`/CONTAINER` respectively —
  not two independent verbs that merely no longer collide. Revised the
  "`INITIALIZE`: unifying `INIT` and `INITIALIZE` under one verb" design
  section and merged the former subtasks 1/4 split accordingly (subtask 1 now
  does the verb unification, including stubbing `initialize_container`'s
  grammar; subtask 4 implements its handler). Confirmed the canonical,
  documented spelling is `INITIALIZE`, with `INIT` working purely as DCL's
  ordinary abbreviation matching, not a second verb or an explicit alias.
- User separately flagged that `internal/bootdata/files/vax.init`'s own boot
  script — confirmed by grep to contain a bare `init ^d4096` at line 9, and
  confirmed to be the live, actually-executed copy (identical today to the
  read-only `testdata/dcl/vax.init`/`reference/eVAX/vax.init` imports) —
  would break once `/VAX` becomes mandatory. Added as explicit fallout in
  both the design section and subtask 1, following the same "only the live
  bootdata copy changes" convention Phase 22 already established for
  `evax.dcl`.
- No code written yet; this document is the planning deliverable requested.
  Implementation starts at subtask 1 in a future session.

### 2026-09-23 — Subtask 1: unify INIT/INITIALIZE onto the DCL grammar

- `internal/bootdata/files/evax.dcl`: added the govax-native `verb
  initialize` block (following the same "diverges from `testdata/dcl/
  evax.dcl`" convention Phase 22 established for `MOUNT`/`DISMOUNT`,
  extended with its own comment) — `/VAX` and `/CONTAINER` qualifiers
  redirecting to `initialize_vax` and `initialize_container` respectively,
  neither a default. `initialize_vax` declares a single `PAGES $rest_of_line`
  parameter with no `/prompt=`, exactly as planned, so `INITIALIZE_VAX`'s
  handler can preserve `CLI_NEEDPAGES`'s original wording via an explicit
  `Result.Present` check rather than a formal-requirement error.
  `initialize_container` is stubbed with its four parameters/qualifier
  (`PATH`/`SIZE`/`LABEL`/`CLUSTER`) per the design doc, no handler bound yet.
- `internal/console/dispatch.go`: removed `INIT` from `fixedCommands` and
  deleted `cmdInit` entirely; added `bindGrammar`'s `INITIALIZE_VAX` bind,
  carrying `cmdInit`'s exact body forward (same `Evaluator` construction,
  same `v * 512` byte conversion, same `CLI_NEEDPAGES` wrap-on-eval-error).
  Also extended the file's own top-of-file doc comment: INIT was a genuine
  `console_dispatch_table` real-function (fixed) entry in the C source, so
  moving it onto the DCL grammar is a third deliberate deviation from that
  table's split, alongside the pre-existing DEPOSIT/RUN callouts.
- `internal/bootdata/files/vax.init`: line 9's `init ^d4096` became
  `init/vax ^d4096` — confirmed via `cmd/govax`'s existing
  `TestRun_startupBootsFromEmbeddedFilesAlone`-family tests that the real
  embedded boot script still boots end to end with the new mandatory `/VAX`
  qualifier. `testdata/dcl/vax.init` and `reference/eVAX/vax.init` left
  untouched, per Phase 22's established convention for this file pair.
- Confirmed by grep (as the design section anticipated) that no other test
  or fixture in the tree dispatches a bare `"INIT ..."`/`"INITIALIZE ..."`
  command line; every other call site that exercises VAX memory
  initialization goes through `Console.Init` directly, unaffected by the
  verb-spelling change.
- Tests added: `internal/console/dcl/define_test.go` (verb count 12→13,
  `INITIALIZE` added to `TestLoadEvaxGrammar`'s verb list, a new
  `TestLoadEvaxGrammar_initializeVaxContainer` structural check mirroring
  the existing `..._mountDismount` one) and `parse_test.go`
  (`TestParse_initializeVax`, `TestParse_initAbbreviatesInitialize`,
  `TestParse_initializeContainerStub`, `TestParse_initializeBareHasNoDefault`);
  `internal/console/dispatch_test.go` (`TestDispatch_initializeVax`,
  `TestDispatch_initAbbreviatesInitializeVax`,
  `TestDispatch_initializeVaxNeedsPages`, `TestDispatch_initializeBareErrors`,
  `TestDispatch_initializeContainerNotYetImplemented`). Full `go build ./...`,
  `go vet ./...`, and `go test ./...` all clean.
- No bugs found in the peer `ods2` module during this subtask (it wasn't
  touched — subtask 1 is grammar-engine/dispatch-only, no `internal/rms`
  work yet).

### 2026-09-23 — Subtask 2: parameter-scoped qualifier support in `internal/console/dcl`

- `grammar.go`: `Parameter` gains a `Qualifiers []*Qualifier` field (nil for
  the overwhelming majority of parameters, which declare none of their own)
  plus a `qualifier(name string)` lookup method mirroring `Entry.qualifier`.
- `define.go`: the `QUALIFIER` case gains an explicit, opt-in `/parameter=
  <name>` switch. When present, the new `findParameter` helper resolves
  `<name>` against the current entry's *already-declared* parameters (a
  forward reference — naming a parameter not yet seen — is not supported,
  matching every other name-based cross-reference in this file) and the
  qualifier is appended to that `Parameter`'s own list instead of the
  entry's; absent, behavior is completely unchanged. See the design
  section's new "Resolved during subtask 2" note above for why this had to
  be an explicit switch rather than inferred from adjacency in the grammar
  text — `DEFINE/DEVICE`'s own existing `NAME` parameter immediately
  followed by ~20 unrelated entry-level qualifiers would have been silently
  misattributed otherwise.
- `validate.go`: refactored the per-entry qualifier cross-reference
  resolution (`/type=`, `/syntax=`, `/alias=`) out into a shared
  `validateQualifiers` helper, called once for an entry's own `Qualifiers`
  and once per parameter for that parameter's `Qualifiers` — so
  parameter-scoped qualifiers get identical validation to entry-level ones,
  including a new `CLI_PARAMNOTFOUND` status
  (`internal/vmserrors/codes_cli.go`) for a `/parameter=` naming an
  undeclared parameter. A parameter-scoped qualifier's `/alias=` resolves
  against that same parameter's own list, not the whole entry's — aliasing
  across scopes isn't needed by anything in this project.
- `parse.go`: `Parse`'s main loop now tracks `lastParam *Parameter`, the
  most recently filled positional parameter, reset to `nil` whenever a
  `/syntax=` redirect switches `active` to a different entry entirely.
  `parseQualifier` tries matching a `/name` token against `lastParam`'s own
  qualifier list first, falling back to the entry-level list exactly as
  before only when that lookup misses. `Result` gained a parallel,
  parameter-namespaced value store (`paramValues`, keyed first by parameter
  name then by qualifier name — deliberately separate from the flat
  `values` map) and its own accessor family: `ParamPresent`, `ParamNegated`,
  `ParamString`, `ParamInt`, `ParamKeyword`. `checkRequirements` now also
  applies a parameter-scoped qualifier's `/default=` when it wasn't
  supplied, mirroring the existing entry-level default-filling loop.
- Confirmed the change is purely additive by running the full existing
  suite unmodified before writing any new tests (all green), then adding
  `TestLoadEvaxGrammar_unaffectedByParamQualifierFeature`, which walks every
  entry in the real `evax.dcl` grammar and asserts no parameter ended up
  with any parameter-scoped qualifiers, plus a spot check that
  `DEFINE_DEVICE`'s `CLUSTER` qualifier is still entry-level despite
  immediately following its `NAME` parameter in the file.
- New tests: `internal/console/dcl/param_qualifier_test.go`, built against a
  small self-contained grammar (not `evax.dcl`) shaped after `COPY`'s own
  `SOURCE`/`DESTINATION`/`HOST` design, covering: `/parameter=` correctly
  attaching each declaration to its named parameter
  (`TestLoadParamQualifierGrammar_structure`); `/parameter=` naming an
  undeclared parameter erroring at grammar-load time
  (`TestParseGrammar_qualifierParameterNotFound`); a parameter-scoped
  qualifier attached with no space, with a space, on the first vs. second
  parameter, both at once, in negated (`/NOHOST`) form, and after a quoted
  value containing an embedded `/` (the "Quoting" design section's own
  concern, now regressed against this specific code path); and a qualifier
  name absent from the just-filled parameter's own list correctly falling
  through to entry-level matching. `go build ./...`, `go vet ./...`, and
  `go test ./...` all clean across the whole module.
- No console command uses this capability yet, as planned — `COPY` (subtask
  9) is the first consumer.
- No bugs found in the peer `ods2` module during this subtask (not
  touched — this is `internal/console/dcl`-only, no `internal/rms` work).

### 2026-09-23 — Subtask 3: `internal/rms.Session`, `SET DEFAULT`/`SHOW DEFAULT`

- `internal/rms/session.go` (new): `Session` holds `Mounts *MountTable`
  (the same table a `Console` already owns — not a copy) and `Default
  filespec.Spec`. `NewSession(mounts)`, `SetDefault(text string) error`
  (`filespec.Parse` against the current `Default`, replacing it on
  success), `DefaultString() string` (`Default.String()`, never fails —
  even the zero-value `Default` renders as `"[000000]"`), and the shared
  `resolveVolume(specText string) (*volume.Volume, filespec.Spec, error)`
  helper every later subtask's command (`DIRECTORY`/`DELETE`/`PURGE`/
  `COPY`/`TYPE`) will call: parses `specText` against `Default`, then looks
  the resulting device up in `Mounts`, erroring clearly if nothing's
  mounted there.
- Resolved the design section's open naming question:
  `Console.ContainerSession *rms.Session`, constructed alongside `Mounts`
  in `machine.go`'s `New` (sharing the same `*rms.MountTable` — `mounts :=
  rms.NewMountTable(); ... Mounts: mounts, ContainerSession:
  rms.NewSession(mounts)`), never reset by `ZERO`/`Init`, matching
  `Mounts`'s own lifetime rule and doc comment.
- `internal/console/default.go` (new): `Console.SetDefault`/
  `Console.ShowDefault`, thin wrappers mirroring `mount.go`'s own
  `Mount`/`Dismount` pattern. `SetDefault` translates a malformed file
  spec into a new `CLI_BADFILESPEC` status (`internal/vmserrors/
  codes_cli.go`) rather than a bare Go error — chosen as a *CLI*-facility
  code, not a *SYS*-facility one like `MOUNT`'s `SS_DEVMOUNT`/`SS_NOMOUNT`,
  since this is plain console-command-argument validation (the same family
  `SET RADIX`/`SET MODE`'s own `CLI_BADRADIXVAL`/`CLI_BADMODE` already
  use), not an actual file-operation failure state — the design section's
  "operator-console-facing `SS$_` statuses" framing turned out to describe
  subtask 4 onward's actual file-operation commands (`INITIALIZE/
  CONTAINER`, `DIRECTORY`, ...), not `SET DEFAULT` itself, which has no
  device-mount precondition to check at all. `ShowDefault` never fails.
- `dispatch.go`: `cmdSet` gained a `case "DEFAULT":` (erroring
  `CLI_NEEDSETARG` on a bare `SET DEFAULT` with nothing after it, matching
  every other SET sub-form's own missing-argument check), and
  `bindGrammar` gained `g.Bind("SHOW_DEFAULT", ...)`.
- `internal/bootdata/files/evax.dcl`: `type show_types` gained a `DEFAULT`
  keyword redirecting to a new `syntax show_default/id=163` under `verb
  show` — no parameters or qualifiers of its own, following the same
  "govax-native extension" comment convention Phase 22 established at
  `MOUNT`'s own definition. `testdata/dcl/evax.dcl`/`reference/eVAX/
  evax.dcl` untouched, per that same established convention.
- Tests: `internal/rms/session_test.go` (new) covers `SetDefault`/
  `DefaultString`'s round trip, component inheritance across two calls,
  relative-directory parsing, rejecting-and-leaving-`Default`-unchanged on
  bad syntax, the fresh-session `"[000000]"` display, and `resolveVolume`'s
  happy path plus its two error paths (not mounted, bad syntax).
  `internal/console/default_test.go` (new) covers the same round trip
  through the `Console`-level wrappers, `CLI_BADFILESPEC` translation, and
  `SET DEFAULT`/`SHOW DEFAULT` dispatched through the real fixed-table/DCL
  paths respectively (including `SHOW DEF`'s abbreviation and a bare `SET
  DEFAULT`'s `CLI_NEEDSETARG`). `internal/console/dcl/define_test.go`
  gained `TestLoadEvaxGrammar_showDefault`, a structural check mirroring
  the existing `..._mountDismount`/`..._initializeVaxContainer` ones. Full
  `go build ./...`, `go vet ./...`, and `go test ./...` all clean across
  the whole module (including the peer `ods2` module, reachable via
  `go.work`).
- No bugs found in the peer `ods2` module during this subtask; its own
  `cmd/ods2/internal/session/setshow.go` was read purely as the intended
  behavioral reference (`filespec.Parse`/`.String()` round trip), not
  modified.

### 2026-09-23 — Subtask 4: `INITIALIZE/CONTAINER` handler

- `internal/rms/initialize.go` (new): `InitializeContainer(path string,
  blocks uint32, label string, clusterSize uint16) error`, a direct
  functional match for `ods2`'s own `cmdInitialize`
  (`diskimage.Create`+`volume.Initialize`, container closed via `defer`
  either way, no mount performed — matching real VMS's own INITIALIZE and
  this project's own MOUNT/INITIALIZE split per the design section).
  `label`/`clusterSize` pass straight through to
  `volume.InitializeOptions`; their own zero values (`""`/`0`) are already
  `volume.Initialize`'s documented "use the default" (`NONAME`/1-block
  clusters), so no default-filling logic was needed on this side.
- `internal/console/initialize.go` (new): `Console.InitializeContainer`,
  a thin wrapper mirroring `mount.go`/`default.go`'s own pattern —
  translates any `internal/rms.InitializeContainer` failure into a new
  `SS_BADPARAM` status (`internal/vmserrors/codes_sys.go`) rather than a
  bare Go error.
- New status code: `SS_BADPARAM` (real VMS `SS$_BADPARAM`, 20, confirmed
  against `reference/eVAX/eVAX/Headers/ss_def.h`) — chosen over inventing
  a non-real code because `ss_def.h` has no INIT-facility-specific status
  this project could reuse (real VMS reports INITIALIZE failures through
  a dedicated INIT facility this project doesn't model at all), and every
  one of `INITIALIZE/CONTAINER`'s own failure modes (bad path, zero/too-
  small block count, oversized label, ...) ultimately traces back to a
  bad argument value — the same story `SS$_BADPARAM` tells for any other
  system service. Kept in the SYS facility alongside `SS_DEVMOUNT`/
  `SS_NOMOUNT`, matching the design section's "operator-console-facing
  `SS$_` statuses, the same class MOUNT/DISMOUNT already use" framing.
- `dispatch.go`: `bindGrammar` gained `g.Bind("INITIALIZE_CONTAINER",
  ...)`, reading `PATH`/`SIZE`/`LABEL`/`CLUSTER` straight off the
  `Result` (`PATH`/`SIZE` are guaranteed present by the grammar's own
  `/prompt=`-driven requirement, unlike `INITIALIZE_VAX`'s `PAGES`; the
  grammar's `LABEL`/`CLUSTER` don't need an explicit `Present` check
  either, since `r.String`/`r.Int`'s own absent-value zero values already
  match `InitializeContainer`'s own "use the default" convention) and
  calling `Console.InitializeContainer`.
- Removed `internal/console/dispatch_test.go`'s now-obsolete
  `TestDispatch_initializeContainerNotYetImplemented` (subtask 1's "no
  handler bound yet" placeholder) — real dispatch coverage now lives in
  `internal/console/initialize_test.go`.
- Tests: `internal/rms/initialize_test.go` (new) covers the happy path
  (confirmed by mounting the result and reading its label back),
  `""`-label defaulting to `NONAME`, a non-zero `clusterSize` actually
  reaching the on-disk home block (read back via `Mount` +
  `Volume.Devices[0].Home.ClusterSize`), confirms nothing is left mounted
  afterward, and three failure paths (`blocks=0`, a 5-block volume too
  small for the reserved-file layout — matching the sibling `ods2`
  module's own `TestInitializeRejectsUndersizedVolume`'s choice of 5 —
  and a nonexistent parent directory). `internal/console/initialize_test.go`
  (new) covers the same happy path and two failure translations
  (`SS_BADPARAM`) through the `Console`-level wrapper, plus DCL-dispatched
  coverage: a full `INITIALIZE/CONTAINER "path" size label` command line
  mounting successfully afterward, `INIT/CONTAINER` (the 4-letter
  abbreviation) reaching the same handler, confirmation the command never
  auto-mounts, and a bare `INITIALIZE/CONTAINER "path" size` with no
  optional `LABEL`/`CLUSTER` dispatching successfully. Full `go build
  ./...`, `go vet ./...`, and `go test ./...` all clean across the whole
  module (including the peer `ods2` module, reachable via `go.work`).
- No bugs found in the peer `ods2` module during this subtask; its own
  `diskimage.Create`/`volume.Initialize`/`InitializeOptions` were read and
  called exactly as documented, not modified.

### 2026-09-23 — Subtask 5: `DIRECTORY`

- `internal/rms/directory.go` (new): `DirectoryOptions` (`Full`/`File`/
  `Size`/`Date`, `Full` implying the other three, matching `ods2`'s own
  `cmdDirectory`) and `Session.Directory(specText string, opts
  DirectoryOptions) (string, error)`. Functionally mirrors `ods2`'s own
  `cmdDirectory`/`groupMatchesByDir`/`formatDirectoryEntry` (`filespec.Glob`
  against the resolved spec, grouped by directory, one line per file, a
  trailing file/block-count summary) but reproduced fresh rather than
  imported — that package lives under `cmd/ods2/internal/session`, off
  limits per this phase's own "behavioral reference, not code to link
  against" framing. Deliberately hands back a plain `string` rather than
  writing to an `io.Writer` the way `ods2`'s own version writes straight to
  `s.Stdout`: `internal/rms` has no notion of `internal/console.Console`'s
  output stream (see `Session.SetDefault`/`DefaultString`'s own "rms
  computes, console prints" split), so `internal/console/directory.go`'s
  `Console.Directory` does the actual `Printf`. An empty `specText`
  defaults to `"*.*;*"`, matching `ods2`'s own default. Confirmed by direct
  observation that a freshly initialized volume's master file directory
  already contains `ods2`'s own reserved files (`INDEXF.SYS`, `BITMAP.SYS`,
  `BADBLK.SYS`, `BADLOG.SYS`, `CORIMG.SYS`, `VOLSET.SYS`, `CONTIN.SYS`,
  `BACKUP.SYS`, `000000.DIR` — nine files total from `volume.Initialize`
  alone), which every test below had to account for (either by scoping its
  file-spec pattern narrowly enough to exclude them, or by not asserting an
  exact total at all) rather than assuming a "bare, empty volume" starting
  point.
- Reused `ods2`'s own `cmdDirectory` quirk rather than "fixing" it: a
  directory header for the master file directory itself prints as
  `"Directory DUA0:[]"` (an empty joined `Dirs` slice, with no `[000000]`
  fallback the way `filespec.Spec.String`/`formatDirPath`'s *error-message*
  formatting applies elsewhere) — confirmed this is `ods2`'s own existing
  behavior, not a defect introduced here, and left it exactly as observed
  per this phase's own "matching `ods2`'s own output shape closely enough
  to be recognizable, not necessarily byte-identical" framing (not a
  license to diverge gratuitously in the one place a literal port would
  have been just as easy).
- `internal/rms/session.go`: `resolveVolume`'s "device not mounted" failure
  changed from a plain `fmt.Errorf` string to a new, exported
  `*NotMountedError{Device string}` type — needed so
  `internal/console/directory.go`'s `Console.Directory` can distinguish
  "bad file spec" from "device not mounted" (reported as `CLI_BADFILESPEC`
  vs. `SS_DEVNOTMOUNT` respectively, matching `SET DEFAULT`'s and `MOUNT`/
  `DISMOUNT`'s own conventions for the same two conditions) via `errors.As`
  rather than by inspecting error text, and can recover the exact device
  name for its own diagnostic without re-parsing `specText` itself.
  `TestSession_resolveVolumeNotMounted`/`..._resolveVolumeBadSyntax`
  (subtask 3) still pass unmodified — neither asserted on the previous
  error text, only that an error was returned.
- `internal/console/directory.go` (new): `Console.Directory`, translating
  `*rms.NotMountedError` to `SS_DEVNOTMOUNT` and everything else to
  `CLI_BADFILESPEC`, then printing the listing via `Console.Printf` on
  success — no new status codes were needed for this subtask after all
  (the design section's speculative `ssNoSuchFac`/`ssNoSuchFile` reuse
  turned out not to fit DIRECTORY's own two failure modes; per its own
  wording, those remain earmarked for `DELETE`/`TYPE`/`COPY`'s "not found"
  cases in later subtasks instead).
- `internal/bootdata/files/evax.dcl`: new govax-native `verb directory/
  id=900`, continuing the divergence block started at `MOUNT` — a single
  optional `SPEC` parameter (`$string`, no `/prompt=`: an omitted file spec
  is normal, not a missing argument) plus `FULL`/`FILE`/`SIZE`/`DATE`
  switch qualifiers. `testdata/dcl/evax.dcl`/`reference/eVAX/evax.dcl`
  untouched, per the established convention. `DIR` (3 letters) reaches
  `DIRECTORY` via `Grammar.matchVerb`'s ordinary unambiguous-prefix
  matching — confirmed no other verb in the grammar starts with those
  letters (`DISMOUNT` is the nearest, diverging at the third character).
- `dispatch.go`: `bindGrammar` gained `g.Bind("DIRECTORY", ...)`, building
  a `rms.DirectoryOptions` from `r.Present("FULL"/"FILE"/"SIZE"/"DATE")`
  and calling `Console.Directory(r.String("SPEC"), opts)`.
- Tests: `internal/rms/directory_test.go` (new) builds its own small,
  writable test volume and populates it with real files via `ods2`'s
  public `Volume.CreateFile`/`Directory.Insert` API directly (the same
  calls `internal/rms/create.go`'s own `createOnVolume` makes on behalf of
  a real `SYS$CREATE`, invoked here without going through a whole
  simulated VAX FAB/RAB, since these tests only need files to exist, not a
  running VAX program to create them through) — covers a bare listing
  containing created files, a wildcard name/type filter actually
  narrowing the results, an empty (zero-match) listing being success not
  an error, `/FULL` including the record format and implying
  `/FILE`+`/SIZE`+`/DATE`, `/SIZE` alone not including `/FULL`-only detail,
  and both `resolveVolume` failure modes surfacing as the right error
  shape (`*NotMountedError` vs. a plain error) for the console layer to
  key off of. `internal/console/directory_test.go` (new) covers the
  `Console`-level wrapper's happy path, both status-code translations, and
  DCL-dispatched coverage (`DIRECTORY` and its `DIR` abbreviation, an
  explicit file spec plus `/SIZE` threading through end to end).
  `internal/console/dcl/define_test.go` gained
  `TestLoadEvaxGrammar_directory` (verb count 13→14) mirroring the existing
  structural checks. `internal/rms/simh_interop_test.go` gained the
  subtask's own called-for opt-in interop check,
  `TestSimhInterop_directoryListsRealVMSDisk` (skips cleanly when
  `testdata/disks/rq0-ra92.dsk` isn't present): mounts the real VAX/VMS
  system disk read-only and runs `DIRECTORY/FULL` against its master file
  directory, confirmed by manual inspection of `t.Logf` output to produce
  a sensible 13-file, 6885-block listing (`INDEXF.SYS`, `SYS0.DIR`,
  `SYSEXE.DIR`, `VMS$COMMON.DIR`, ...) — proving the glob/formatting path
  handles a real system disk's MFD, not only this package's own hermetic
  fixtures. Full `go build ./...`, `go vet ./...`, and `go test ./...` all
  clean across the whole module (including the peer `ods2` module,
  reachable via `go.work`).
- No bugs found in the peer `ods2` module during this subtask; its own
  `filespec.Glob`/`Volume.CreateFile`/`Directory.Insert` were read and
  called exactly as documented, not modified.
