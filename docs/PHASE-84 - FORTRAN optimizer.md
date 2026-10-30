# Phase 84 — the FORTRAN optimizer

**Status:** planned (2026-10-29). Needs Phase 83 (the whole language
compiled, so the corpus is complete). Part of the FORTRAN program laid
out in `PHASE-74 - FORTRAN plan.md` (Decision 9).

## Goal

`/OPTIMIZE`: passes over the IR and the generated instructions that make
programs smaller and faster without changing what they print. Each pass
is its own file in `internal/fortran/opt`, with a doc comment that
teaches the technique on a small FORTRAN example. Proposed passes, in
order of payoff:

- **constant propagation and folding** across statements, and **dead
  code removal** (unreachable blocks, unused temporaries);
- **common subexpressions** within a basic block (array addresses above
  all);
- **loop-invariant code motion** out of DO loops;
- **induction variables and strength reduction:** an array subscript
  that grows by a constant each trip becomes an address that does;
- **register allocation:** R6–R11 for DO variables, trip counts, and
  addresses used in a loop, with liveness analysis; dummy argument
  addresses loaded once;
- **VAX loop instructions:** a counted loop closes with `SOBGTR`,
  `AOBLEQ`, or `ACBL` where its form allows;
- **a peephole pass** over the instructions: `MOVL #0` to `CLRL`, `ADDL2
  #1` to `INCL`, branches to branches, a move followed by a test.

## What earlier phases leave in place

- The IR (Phase 75), the whole language in it (Phases 76–83), and the
  corpus of programs with their expected output.

## Design notes

- **Correctness first:** every corpus program runs with `/OPTIMIZE` and
  `/NOOPTIMIZE` and must print the same; a test reports instruction
  counts (from govax's `--stats`) so improvements are measured.
- **VOLATILE** variables and variables whose address escapes (%LOC,
  EQUIVALENCE, COMMON across calls, arguments) are never kept in a
  register across a call or a reference through a pointer: the alias
  rules are FORTRAN's (a dummy argument may not be modified through
  another name), which the doc explains.
- **The default:** `/NOOPTIMIZE` until the whole corpus passes both
  ways, then `/OPTIMIZE`, as VAX FORTRAN's default (agreed in Phase 74's
  review, answer 7).

## Open questions

- **How far to go:** global CSE and partial redundancy elimination are
  more than the corpus may justify; the survey measures where the time
  goes in the corpus's larger programs first.

## Subtasks (provisional)

1. **Survey:** instruction counts and hot spots across the corpus; the
   pass list confirmed.
2. **The pass framework**, `/OPTIMIZE`, the both-ways test.
3. **Constant propagation, folding, dead code.**
4. **Local common subexpressions.**
5. **Loop-invariant motion.**
6. **Induction variables and strength reduction.**
7. **Register allocation.**
8. **Loop instructions and the peephole pass.**
9. **Close-out:** the measured gains; FORTRAN-INTERNALS.md's optimizer
   chapter; the default switched if the corpus passes.

## Progress log

- **2026-10-29:** Planned with Phase 74.
