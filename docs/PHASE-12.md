# Phase 12: Integration & regression

## Goal

Prove the full stack works end-to-end against every fixture, establish a durable
regression suite, and do a final polish pass before considering the port complete.

## Scope

- Assemble (via Phase 11) and run every `testdata/asm/*.asm` fixture; run every real
  `testdata/exe/*.exe` binary (except `put1.exe`, a zero-byte file not usable per
  `reference/eVAX/AUDIT.md`); exercise SAVE/LOAD against `testdata/rom/xdefault.rom`.
- Build a fixture-driven regression test suite (likely in `internal/console` or a
  top-level `integration_test.go`) that runs all of the above in CI-friendly form, so
  future changes can't silently regress behavior the C source got right.
- Cross-check final behavior against `reference/eVAX/AUDIT.md`'s own verification notes
  for each fixed finding (N1, N2, V1, V7, V8, R1, etc.) — those describe the exact
  expected behavior for several of these fixtures and are a ready-made acceptance
  checklist.
- Performance pass: profile the fetch-decode-execute loop under `bench.asm` (a fixture
  apparently intended for this purpose) and address any obvious hotspots.
- Documentation polish: update `README.md` and `docs/PLAN.md` to reflect the finished
  state; consider whether `reference/eVAX/` should be trimmed or kept as permanent
  historical reference.

## Deliverables

- A green, fixture-driven regression suite covering CPU, console, I/O, RTL, and
  assembler behavior together.
- Every entry in `docs/DEVIATIONS.md` resolved: either fixed in the Go port, or
  deliberately kept with rationale recorded there.

## Open questions / notes

- This phase's real scope depends heavily on what Phases 01-11 turn out to need —
  treat the above as a checklist to refine once those are underway, not a fixed plan.

## Progress Log

_Not started._
