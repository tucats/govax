# testdata/opt84: Phase 84's survey measurements

What `docs/PHASE-84 - FORTRAN optimizer.md`, Part A, measured with, kept so
its numbers can be made again. Nothing here is built or run by `go test`.

- `kernels/` are five small compute programs. The corpus in
  `testdata/fortran` spends about 95% of its instructions in FTNRTL, so it
  can't show what an optimizer gains. These spend almost all of theirs in
  their own loops.
- `hand/` is generated MACRO with loops rewritten by hand, as the proposed
  passes would rewrite them: `m1peep` to `m4reg` are `matmul` after each
  stage in turn, and `fsieve` and `fsubs` are `sieve` and `subs` after all
  of them.
- `measure/` is the measuring code:
  - `measure.patch` adds a per-instruction hook to `Engine.Step`. It isn't
    in the tree, so the hot path pays nothing.
  - `survey84_test.go.txt` and `survey84d_test.go.txt` are the tests that use
    the hook, one for the kernels and corpus and one for DUNGEON. Rename them
    to `.go` in `internal/console` in a scratch worktree, and set `SURVEY_OUT`
    and `SURVEY_DIRS`, or `SURVEY_KIT` and `SURVEY_MARX` for DUNGEON.
  - `peep.py` is the prototype of the branch peephole.
  - `mkdtx.for`, `itime.for`, `game.in`, and `quit.in` are what the DUNGEON
    run needs besides DUNGEON. Its source and data stay outside the
    repository, and no DUNGEON-derived MACRO is kept here.
