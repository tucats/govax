# Phase 00: Project bootstrap & source import

## Goal

Turn `govax` from an empty shell into a scaffolded Go module with the C reference
source and its test fixtures imported and organized, so later phases can start writing
real code against a stable layout.

## Scope

- Initialize `go.mod` (module `github.com/tucats/govax`).
- Create the `cmd/`, `internal/`, `testdata/`, `reference/` directory layout.
- Import the eVAX C source tree (from `/Users/tom/Documents/Projects/eVAX`, GitHub
  `tucats/evax`) into `reference/eVAX/`, using `git archive` on that repo's `HEAD` so
  only tracked source comes across (Xcode derived-data/index caches, `.DS_Store`, etc.
  are excluded via that repo's own `.gitignore`).
- Organize the loose root-level fixtures from the C repo into `testdata/`:
  `.asm` sources → `testdata/asm/`, `.exe` binaries → `testdata/exe/`, `xdefault.rom` →
  `testdata/rom/`, `evax.dcl`/`vax.help`/`vax.init` → `testdata/dcl/`.
- Carry over `AUDIT.md` and `CLAUDE.md` from the C project into `reference/` as
  standing context for future sessions.
- Write all phase docs (`docs/PHASE-00.md` … `docs/PHASE-12.md`) and update
  `docs/PLAN.md` to link them and record the locked-in architecture decisions.

## Deliverables

- `go.mod`, `cmd/govax/main.go` (placeholder), one `doc.go` stub per `internal/*`
  package so `go build ./...` succeeds on an otherwise-empty module.
- `reference/eVAX/` — full copy of the C source tree, read-only reference for diffing
  during conversion. Not meant to be edited; if the upstream C repo changes, re-run the
  same `git archive` import.
- `testdata/{asm,exe,rom,dcl}/` — fixtures used by later phases' tests, particularly
  the Phase 12 integration/regression suite.
- `reference/AUDIT.md`, `reference/CLAUDE.md` — background on the C source's known-fixed
  bug history and architecture, useful when a Go port decision needs a "why does the C
  do it this way" answer.

## Open questions / notes

- Package boundaries under `internal/` (`vax`, `vm`, `cpu`, `console`, `io`, `rtl`,
  `asm`) are a starting point, not final — expect adjustment once Phase 01 is under way
  and the actual Go types take shape (e.g. `vax` and `vm` may end up merged, or `cpu`
  may need splitting once its real size is visible in Go).
- `reference/eVAX/` intentionally still contains the fixture files at its own root
  (as a faithful mirror of the C repo) even though `testdata/` also holds organized
  copies — the two serve different purposes (diff-reference vs. working test fixtures)
  and are expected to both exist.

## Progress Log

### 2026-09-14 — Phase started and completed

- Explored the C source tree at `/Users/tom/Documents/Projects/eVAX` to size the
  conversion (~48K lines across CPU/RTL/Console/Assembler/Initialization/Headers) and
  read `CLAUDE.md`/`AUDIT.md` for architecture and known-bug context.
- Ran a planning session with the user (see `docs/PLAN.md` for the locked-in
  decisions): instantiated-struct state model, full source import, bottom-up phase
  order, C source as primary correctness reference.
- Scaffolded `go.mod`, `cmd/govax/`, and `internal/{vax,vm,cpu,console,io,rtl,asm}/`
  with placeholder `doc.go` files.
- Imported the C source via `git archive HEAD | tar -x` from the eVAX repo (clean
  working tree at import time, no uncommitted changes lost) into `reference/eVAX/`.
  Removed one unrelated tracked file/dir that came along with the import,
  `go toys/` (a stray, unrelated Go experiment file with no connection to the VAX
  emulator), per the user's request.
- Organized fixtures into `testdata/asm/` (19 files), `testdata/exe/` (8 files,
  including the known-zero-byte `put1.exe` — see `AUDIT.md` "put1.exe's fate" for
  context, not fixed on the C side either), `testdata/rom/xdefault.rom`, and
  `testdata/dcl/` (`evax.dcl`, `vax.help`, `vax.init`).
- Copied `AUDIT.md` and `CLAUDE.md` into `reference/`.
- Wrote all 13 phase docs and updated `docs/PLAN.md`.
- Verified `go build ./...` succeeds against the scaffolded module.
