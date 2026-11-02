# Phase 84 — the FORTRAN optimizer

**Status:** in progress. Subtask 1, the survey, was reviewed by the author
on 2026-11-01. Planned 2026-10-29. Needs Phase 83 (the whole language
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

# Part A — the survey (2026-11-01)

Reviewed by the author on 2026-11-01. Their answers follow the questions below,
and one of them adds a subtask the survey had recommended against.

## Summary

- **The corpus can't measure an optimizer.** Its programs spend about 95% of
  their instructions in FTNRTL, which is hand-written MACRO the optimizer never
  touches. It remains the correctness test. The gains have to be measured on
  DUNGEON and on five new kernels (`testdata/opt84`).
- **The largest gains are in two places the plan ranked low.** Those are
  branches (a fifth of DUNGEON's instructions in its own code are `BRW`) and
  registers (most of the memory references in a loop). Constant propagation,
  ranked first in the plan, has little left to do.
- **Recommended order:** the branch peephole moves to the front and registers
  come before induction variables. Global CSE and PRE are not needed.
- **Measured reachable gain:** 25% fewer instructions in the whole DUNGEON run
  with the peephole and one routine done by hand, and 47% to 76% in the
  kernels. All were hand-applied, not yet the work of a real pass.

## How it was measured

A temporary hook in `Engine.Step` counted every instruction by PC, in a
throwaway worktree (`testdata/opt84/measure`; not in the tree). Each count is
bucketed by the image the PC is in. "Own code" means the program's main image.
Data references are the memory statistics of `--stats`, instruction fetches
excepted (they go through the fetch window). The runs used the copying loader,
paging off. Each hand-edited or peepholed program was assembled and linked by
govax's MACRO and LINK and run, and its output compared with the original's.

Instruction counts aren't time. A VAX's time depends heavily on memory
references, which registers remove, so the tables give both.

## Where the instructions go

**The corpus** (76 programs in `testdata/fortran`, every one still printing
its `.out`):

| | Instructions | Share |
|---|---:|---:|
| All programs | 7,881,993 | |
| Own code | 212,317 | 2.7% |
| FTNRTL | 7,475,527 | 94.8% |
| `astrace` alone | 6,311,616 | 80.1% of the total |
| All but `astrace`: own code | 82,287 of 1,570,377 | 5.2% |

The largest own-code counts are `astrace` (130,030), `unformatted` (18,673),
and `adjust` (12,354). The corpus's programs
are tests of the language: small, short, and mostly I/O.

**DUNGEON.** It already compiles (`/CONTINUATIONS=40`), assembles, links, and
runs, with an ITIME of our own and DTEXT.DAT made as a fixed-record file by a
small FORTRAN program. A scripted game of 31 lines ran to its end. Its output was only compared between builds, not checked against a
reference: that is Phase 86's job.

| Run | Total | Own code | FTNRTL |
|---|---:|---:|---:|
| Start and quit | 2,172,953 | 143,493 (6.6%) | 2,012,960 (92.6%) |
| The scripted game | 6,818,609 | 4,504,906 (66.1%) | 2,267,472 (33.3%) |

Within the game's own code:

- **One routine dominates:** `QHERE`, a small DO loop searching two arrays, is
  62.0% of it. `SCHLST`, `LIT`, `THISIT`, and `SPARSE` follow at 6.0%, 5.7%,
  5.0%, and 4.2%.
- **By opcode:** `BRW` 20.0%, `MOVL` 16.6%, `INCL` 9.8%, `BLEQ` 9.5%, `DECL`
  9.5%, `CMPL` 9.3%, `BEQL` 8.9%. `CALLS` is 1.2%, so calls aren't where the
  time goes.
- **Statically,** 3,081 of DUNGEON's 20,976 instructions are `BRW`, and 2,875
  of those (93%) are the second half of a byte branch around a `BRW`.

Startup is mostly FTNRTL, in character loops in its own MACRO: about 32
instructions for each byte of DINDX.DAT, if all of it is that file's reading. No compiler pass reaches that (question 3).

**The kernels** (`testdata/opt84/kernels`), nearly all in their own code:

| Kernel | What it is | Total | Largest opcodes |
|---|---|---:|---|
| `matmul` | 60×60 REAL multiply | 3,101,599 | `ADDL2` and `MULL3`, 28% each: subscripts |
| `sieve` | Sieve, ten times, a GO TO loop | 5,785,872 | `BRW` 19%, `MOVL` 16%, `BLEQ` 16% |
| `subs` | Bubble sort and dot product on adjustable dummies | 6,876,884 | `MOVL` 47%, `MOVF` 15% |
| `whet` | REAL mix with COMMON, dummies, MTH$ | 2,695,057 | `MOVL` 34%, `ADDF2` 18% |
| `textk` | Character loop on substrings | 485,525 | `MOVL` 21%, `BRW` 20% |

## What the generated code wastes

The inner loop of `matmul` shows most of it, 14 instructions per trip:

```
MATMUL.$7:
        MULL3   MATMUL.K,#60,R0         ; A's subscript
        ADDL2   MATMUL.I,R0
        MULL3   MATMUL.J,#60,R1         ; B's
        ADDL2   MATMUL.K,R1
        MULF3   MATMUL.A-244[R0],MATMUL.B-244[R1],R0
        MULL3   MATMUL.J,#60,R1         ; C's, on the right
        ADDL2   MATMUL.I,R1
        MULL3   MATMUL.J,#60,R2         ; C's again, on the left
        ADDL2   MATMUL.I,R2
        ADDF3   MATMUL.C-244[R1],R0,MATMUL.C-244[R2]
MATMUL.L30:
        INCL    MATMUL.I
        DECL    MATMUL.$C5
        BLEQ    MATMUL.$8
        BRW     MATMUL.$7
MATMUL.$8:
```

After the proposed passes it could be three:

```
MATMUL.$7:
        MULF3   (R6)+,R8,R0
        ADDF2   R0,(R7)+
        SOBGTR  R9,MATMUL.$7
```

The patterns, from the kernels and DUNGEON:

1. **A branch around a `BRW`** at every conditional branch and loop close: two
   instructions where one would do. The generator doesn't know distances, and
   MACRO doesn't lengthen a short branch.
2. **Index setup each time.** `MOVL I,R0` before each use of an array element,
   though the index is unchanged. `QHERE` loads it up to twice per trip.
3. **Every variable in memory.** Each operand is a 5-byte `L^` reference to
   `$LOCAL`, another psect, so registers save bytes as well as references.
4. **Whole subscripts recomputed**, as in `matmul`, including the same one
   twice in a statement.
5. **Adjustable arrays' A0** loaded from the descriptor cell for each element,
   two `MOVL`s per reference (`subs`, 47% `MOVL`).
6. **Dummy elements with constant subscripts:** `MOVL #1,R1` then
   `@4(AP)[R1]`, since `@n(AP)` has no room for an offset (`whet`, 34% `MOVL`).
   With the address in a register, this is `4(R6)`.
7. **A store and its reload at a function's end:** `CLRL F`, `MOVL F,R0`, `RET`,
   91 of DUNGEON's 1,053 `RET`s by a static count.
8. **`CMPC3` and `MOVC3` of one character** (`LINE(I:I)`, `textk`; DUNGEON runs
   28,856 `CMPC3`s), where `CMPB` and `MOVB` would do and leave R0 to R3 alone.

Little is left for constant propagation: PARAMETER names are already constants
in sema, and `arith` folds as it lowers. Unreachable code is nearly absent, with
only 3 instructions directly after a jump in DUNGEON.

## What the passes can gain

Hand-applied, then assembled, linked, and run; each printed what the original
printed. These are upper estimates of what real passes will do on this code.

**`matmul`, stage by stage (cumulative):**

| Stage | Instructions | | Data reads | Data writes |
|---|---:|---:|---:|---:|
| As generated | 3,101,599 | | 3,033,035 | 692,715 |
| Loop close (`SOBGTR` on the count in memory) | 2,673,199 | −13.8% | 3,033,035 | 692,715 |
| Local CSE | 2,025,199 | −34.7% | 2,385,271 | 683,228 |
| Invariants moved out, to memory cells | 1,391,599 | −55.1% | 2,160,122 | 694,027 |
| Registers and stepped addresses | 750,799 | −75.8% | 629,721 | 247,630 |

**All the passes together:**

| Program | Instructions before | After | Change | Data reads before → after |
|---|---:|---:|---:|---|
| `sieve` | 5,785,872 | 3,073,873 | −46.9% | 3.61M → 0.37M |
| `subs` | 6,876,884 | 2,463,514 | −64.2% | 7.60M → 1.37M |

**DUNGEON's game, as a whole run:**

| Build | Total | Change | Own code | Change | Data reads | Data writes |
|---|---:|---:|---:|---:|---:|---:|
| As generated | 6,818,609 | | 4,504,906 | | 5,919,438 | 2,641,419 |
| Branch peephole on every module | 5,667,990 | −16.9% | 3,354,287 | −25.5% | 5,679,438 | 2,641,379 |
| And `QHERE` in registers | 5,070,333 | −25.6% | 2,756,630 | −38.8% | 4,182,932 | 2,053,390 |

The peephole also shrank the image by 2.7% (191,487 to 186,367 bytes, whole
blocks). The peephole here is a real program, `measure/peep.py`, run over all
of DUNGEON's MACRO. Its output printed byte for byte what the original did.

**A warning from the prototype.** Its size estimate is crude, and it
changed almost nothing in `matmul` or `subs` (under 0.1% each): their loop
bodies looked too far for a byte branch. `sieve` gained 7.3% and `whet` 1.5%.
The hand-written `SOBGTR` in `matmul` was in range and gained 13.8%, so the real
pass needs operand sizes close to what MACRO will choose. `whet` and `textk`
have no hand-optimized version, so their full gains are unmeasured.

## The pass list, confirmed and reordered

| Pass | Plan's rank | Evidence | Proposal |
|---|:--:|---|---|
| Branch shortening, loop close, peephole | last | 25% of DUNGEON's own code, alone | **First.** Needs only an instruction list |
| Constant propagation, folding, dead code | first | Little left | **Smaller:** store-to-load forwarding, dead compiler cells, the function value |
| Local CSE | second | `matmul`: 21 points | Keep |
| Loop-invariant motion | third | `matmul`: 20 points, with hoisted array bases | Keep |
| Register allocation | fifth | `matmul`: 21 points and most of its references; `QHERE`; `sieve`'s reads −90% | Keep, **before** induction variables |
| Induction variables, strength reduction | fourth | Smaller than planned (below) | Keep, **last**, and only where a stride isn't the element's size |
| Global CSE, PRE | open question | No measured case needs them | **Leave out** (answers the phase's open question) |
| One-character strings as `CMPB`/`MOVB` | not listed | `textk`, DUNGEON | **Add**, in lowering |

Why induction variables shrink: the VAX's indexed mode already scales. Once I
is in a register and a column's base is hoisted, `A(I,K)` is `(R6)[R9]` with no
stepped address at all. Only a stride that isn't the element's size needs one:
a row-wise loop, or a CHARACTER\*n array.

## Decisions

1. **Analyses work from the control flow graph, never from DO statements.**
   `sieve`'s hottest loop is a GO TO loop, and the extended range of a DO
   (`loops.for`) leaves and re-enters a loop. Natural loops handle both: a block
   that jumps back in is part of the loop. Assigned GO TO lists its targets, and
   ENTRY points and VFE cases are further entries to the graph.
2. **The IR gets register cells, not virtual registers.** Every pass goes on
   working with Cells: CSE and hoisting make temporaries, as lowering does, and
   the allocator's result is a `Cell.Register` that codegen prints as `R8`.
   Trees and the Sethi–Ullman generator stay as they are, and a dump stays
   readable.
3. **R6 to R11 are the allocator's, R0 to R5 the expressions'.** In all of the
   corpus and DUNGEON, one routine touches R6 and none goes higher, and MOVC and
   CMPC clobber only R0 to R5, so values in R6 and up also survive them. The generator's
   pool shrinks to R0–R5 in a routine that uses a register cell; a routine that
   needs more, as H_floating might, gets fewer register cells rather than a
   failure, with `alloc`'s panic kept as the check. A CALLS preserves R6–R11
   only because each entry mask saves them, so a routine's mask must name them.
4. **Codegen gets an instruction list before text.** `emit` writes text
   straight to a `Writer`, so nothing can look at it afterward. It will collect
   a routine's instructions, labels, and comments, the peephole pass will work
   on those, and then they are written. **`/NOOPTIMIZE` output stays byte for
   byte what it is today**, so every golden file holds.
5. **Branch distances are computed, never hoped for.** Each instruction gets an
   upper bound on its length from its operands, kept true to MACRO's choices
   by a test against what MACRO assembles. A branch is shortened only if the
   bound says it fits, repeating until nothing changes. Output must also go
   through real MACRO (Phase 86's optional round), which won't relax a branch.
6. **Variables keep their values across calls.** The manual's note to SAVE
   (section 4.14) says VAX FORTRAN retains them automatically. So a variable in
   a register has its memory home written before a RETURN, and at a loop's
   exits if used after them. Only the compiler's own cells, and a function's
   value cell, can be dead. A local's value on entry is always unknown; DATA
   values aren't constants unless nothing in the unit can change them.
7. **What may change a place.** Two places conflict by their real extents, not
   by name: cells overlap by offset in their section, since EQUIVALENCE gives
   each name its own Cell at a shared offset, and a union's fields and
   substrings are byte ranges. A local is **address-taken** if it is ever passed
   by reference or descriptor, named by %LOC, in an I/O list, in a NAMELIST, or
   used by a VFE. Address-taken is for the whole unit and for good, because
   ASSOCIATEVARIABLE and a `$QIO`'s IOSB are written by calls that don't name
   them. Any call may change an address-taken local, COMMON, or a dummy. A local
   that is neither survives calls, as the language has no recursion (the manual
   never mentions it). VOLATILE (`Mem.Volatile`) is never moved, merged, or
   removed.
8. **Faults aren't moved onto paths that had none.** Division, floating
   operations, and floating loads (a reserved operand faults) are hoisted only
   from blocks that run on every trip. Otherwise a zero-trip or conditional case
   could fault in a program that was correct. Integer adds, multiplies, and
   address arithmetic may always move, with overflow not enabled. CSE may merge
   two identical operations, so a handler can see fewer conditions: the program
   asked for the same value twice. A constant that faults when folded is left
   to fault at run time. When Phase 85 adds `/CHECK`, those checks must stay.
9. **ASTs and handlers get no special treatment beyond this.** A variable an
   AST routine changes must be VOLATILE or in COMMON across a call, as the
   manual's section 4.16 implies. `corevms`'s handling of registers on an unwind
   and of LIB$FIXUP_FLT follows the architecture, so register cells need nothing
   more.
10. **Both-ways testing covers more than the corpus.** Every corpus program
    and module runs both ways (both paging modes for the existing subset), plus
    DUNGEON's scripted game where its source is present, skipped otherwise. New
    programs in `testdata/fortran` target each decision above: EQUIVALENCE
    overlaps, a DO variable after an exit by GO TO, COMMON changed by a callee in
    a loop, ASSOCIATEVARIABLE, zero-trip loops with a division in them, ENTRY
    into a loop's routine, and VFE reading a loop variable.
11. **Measuring stays out of the engine.** A nil check per instruction on the
    hottest path isn't worth keeping for one phase. The both-ways test logs
    `Engine.InstructionCount` and the memory statistics around each RUN, which
    exist already. Per-PC detail stays in `testdata/opt84/measure`'s patch.
    The kernels join the corpus, with outputs from an independent calculation
    as the harness requires (`whet` only if that can be done: its result is a
    floating chain).
12. **The MACRO file stays readable.** A register cell's name and register go
    in the routine's storage map, moved code keeps a comment naming its source
    line, and each pass's doc comment teaches on a small FORTRAN example, as
    planned. Debugging is at the `.MAR` level, so a `DBG>` user looking at a
    register-held variable needs that map (MANUAL.md, at the close-out).

## Questions for the author

1. **Dummy arguments that alias.** The manual has no rule on a dummy changed
   through another name; the plan's design note assumes the standard's rule,
   that a program may not do this. Later compilers added `/ASSUME=DUMMY_ALIASES`
   for programs that broke it. **Recommended:** follow the standard's rule, with
   no qualifier, as the plan says. The cost of being safe instead is high: in
   `matmul` written with dummy arrays, storing C(I,J) would force B(K,J) to
   reload every trip. It does mean an old program breaking the rule could print
   something different only with `/OPTIMIZE`.
2. **The default.** The plan switches to `/OPTIMIZE` at the close-out if the
   corpus passes both ways. With a new, less-tried optimizer, debugging at the
   `.MAR` level gets harder. **Recommended:** keep the plan, and require DUNGEON's
   game to pass as well as the corpus.
3. **FTNRTL's character loops.** They are a third of DUNGEON's game and 93% of
   its startup, which no compiler pass can touch. **Recommended:** not in this
   phase, whose goal is the compiler's output. Log them for a study in
   PERFORMANCE.md after Phase 86, unless you'd like them sooner.
4. **`CMPB` and `MOVB` for one-character strings.** This is a change to
   lowering rather than a pass. **Recommended:** do it only under `/OPTIMIZE`, so
   Decision 4's golden files hold, as part of subtask 3 or a small subtask of its
   own.

## The author's answers (2026-11-01)

1. **Dummy arguments that alias:** as recommended. The optimizer follows the
   standard's rule, with no qualifier.
2. **The default:** as recommended. `/OPTIMIZE` becomes the default at the
   close-out, if the corpus and DUNGEON's game both pass both ways.
3. **FTNRTL's character loops:** **in this phase**, against the recommendation:
   a subtask tuning them, as part of the general optimization work (subtask 11).
4. **`CMPB` and `MOVB` for one-character strings:** as recommended, only under
   `/OPTIMIZE`, as a small subtask of its own after loop-invariant motion
   (subtask 8; the author's choice, 2026-11-02).

The survey's decisions drew no objection, and stand.

## Subtasks (revised by the survey and the review)

1. **Survey** (done 2026-11-01): this Part A.
2. **The pass framework** (done 2026-11-01): `internal/fortran/opt`, `/[NO]OPTIMIZE` and
   `--optimize`, `ir.Check` after each pass, the both-ways test, its counts
   logged, the kernels.
3. **The instruction list, branch shortening, the peephole** (done 2026-11-01):
   `MOVL #0`, `ADDL2 #1`, branches to branches, inverted branches, `SOBGTR`,
   `AOBLEQ`, `ACBL` on counts in memory.
4. **The analyses** (done 2026-11-01): the control flow graph with every entry,
   dominators, natural loops, the places and effects of Decision 7. No code changes
   yet; tested on their own.
5. **Forwarding and dead code** (done 2026-11-01): store to load in a block, constants
   across statements, dead compiler cells, the function value, unreachable blocks.
6. **Local common subexpressions** (done 2026-11-01).
7. **Loop-invariant motion** (done 2026-11-02): by Decision 8's rules, with
   deferred indexes for descriptor A0s; hoisted array bases and A0s themselves wait for
   registers (subtask 9), having no gain in memory.
8. **One-character strings** (the author's answer 4): `CMPB` and `MOVB` where
   `CMPC3` and `MOVC3` of one character are lowered now, only under `/OPTIMIZE`.
9. **Register allocation:** register cells, liveness, write-backs by Decision
   6, dummy addresses loaded once, the loop instructions on registers.
10. **Induction variables and strength reduction,** where a stride isn't the
   element's size, and autoincrement.
11. **FTNRTL's loops** (the author's answer 3): its character loops tuned by
    hand, as DUNGEON's startup and the corpus measure them, held to the
    existing oracles. No new entry points, so no change to its GSMATCH.
12. **Close-out:** the measured gains against this survey's table;
    FORTRAN-INTERNALS.md's optimizer chapter; MANUAL.md; the default switched.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-11-01:** Subtask 1, the survey, written as Part A, for the author's
  review. Measured the corpus, DUNGEON, and five new kernels with a temporary
  per-PC hook; kernels, hand-optimized code, and measuring code are in
  `testdata/opt84`. DUNGEON compiled, linked, and ran a scripted game on the way,
  earlier than Phase 86 needed it to.
- **2026-11-01:** The author's review: answers 1, 2, and 4 as recommended, and
  FTNRTL's loops added to this phase as subtask 10 (the close-out is now 11).
- **2026-11-01:** Subtask 2, the pass framework. `internal/fortran/opt` is a list
  of passes (`Passes`, empty for now), each followed by `ir.Check`, a failure a
  panic naming the pass. `/[NO]OPTIMIZE` and `--optimize`/`--no-optimize` reach it
  through `fortran.Options.Optimize`, and only `/OPTIMIZE`'s MACRO file says
  anything of it, in a header line naming the passes. `TestFortranPrograms` and
  `TestFortranModules` run everything both ways, each way on a console of its
  own since a program may leave files; every program's two outputs must match
  each other and its `.out`, and `go test -v` logs the gain per program.
  - **The counts:** `Engine.BeginRun` zeroes the instruction count at each run, so
    after a RUN `InstructionCount` is that image's alone. It agreed exactly with
    the survey's per-PC hook on the kernels, so Decision 11's choice stands.
  - **The kernels** joined the harness from `testdata/opt84/kernels`. Their
    `.out` files come from a calculation outside govax (F_floating rounding worked
    in exact fractions for `subs`); `whet` has none and must only print the same
    both ways.
  - **New programs for Decision 10:** `optalias.for`, `optfaults.for`, and
    `optflow.for`, whose outputs were worked by hand and passed as written. They
    guard against passes not yet written, so passing now says only that the
    compiler without an optimizer gets them right.
  - **DUNGEON:** `TestFortranDungeonBothWays` plays `measure/game.in` each way, and
    skips without the source. Its baseline is 6,732,500 instructions, where the
    survey's 6,818,609 came after a first run in the same session (running that
    first here gives exactly 6,818,609; why a second run costs 1.3% more wasn't
    looked into). Later comparisons use this test's number. The game's output is
    only compared between builds and never kept.
  - **For Decision 7:** `asts.for` has an AST reentering a routine with static
    variables, so a local can change with no call in sight. Decision 7's "a local
    that is neither survives calls" assumes no such thing, and subtask 4 must
    settle it: the manual's VOLATILE would be the rule, and `asts.for` the check.
- **2026-11-01:** Subtask 3, the instruction list, branch shortening, and the
  peephole. `/NOOPTIMIZE`'s output is unchanged: every golden file holds as it was.
  - **The list** (Decision 4): `codegen/code.go`'s `Code`, a routine's `Item`s
    (instructions, labels, source lines), which `emit` and `label` now add to, and
    `Code.Write` writes. It also carries what a pass must ask about symbols: labels
    reached from outside the list (`Outside`: ENTRY's and FORMAT's code, written
    after it), displacements' values, and which operands are whole variables (`Cells`,
    `Params`). `Generate` takes `Options.Improve`, and `opt` imports `codegen`, not
    the reverse. `opt.CodePasses` sits beside `opt.Passes`, with `Code.Check` after
    each as `ir.Check` is after the IR's.
  - **The bounds** (Decision 5): `Code.Bound`, from an instruction's operand forms.
    `TestBoundsHold` puts a label either side of every instruction of every program
    to hand, assembles it, and holds each bound to the distance in MACRO's symbol
    table, both as generated and as the passes leave it: 226 compilations, 54,958
    instructions, 353,965 bytes by the bounds and 353,587 by MACRO, none too small.
    That closeness is what the survey's prototype lacked, and it is why `matmul` and
    `subs` now close their loops short. Real MACRO hasn't assembled this output yet
    (Phase 86's optional round); the bounds are held to govax's MACRO only.
  - **`opt/peephole.go`** is the replacements one instruction at a time. The generator
    already chose nearly all of them, so what's left is mostly a constant index's
    `MOVL #0,R1`. A move followed by a test wasn't added: the generator already leaves
    those out (`g.set`).
  - **`opt/branches.go`** is the rest, in rounds until nothing changes (its comment
    teaches it). Every rule only takes bytes out, so a distance found at a round's
    start stays good through it.
  - **What `AOBLEQ` and `ACBL` change, as settled by `cpu/loop.go` and
    `branchacb.go`:** they read the limit before stepping the index, read the index
    once, and set N and Z from the index rather than the comparison. So a rule needs
    a limit that can't overlap the index (Decision 7, by the cells' extents; two
    dummies by answer 1), neither VOLATILE, and no conditional branch next on either
    path. `SOBGTR` and `SOBGEQ` are exactly `DECL` and a branch but for C, which no
    branch after a `DECL` reads. These loop instructions only fire on GO TO loops: a DO
    loop has a count, so `SOBGTR`. A far DO loop keeps its `DECL`, `BLEQ`, and `BRW`.
    `ACBL #1,#-1,C,T` would do it, but is longer in bytes, and rules must not lengthen.
  - **Not done:** `.WORD` tables and the `BRW` in `writeEntry` aren't threaded or
    shortened, being rarely run. A conditional branch to another on the same condition
    isn't threaded either, since the generator never writes one that matters.
  - **Dead code** after a jump goes, though the plan gave that to subtask 5, since
    threading makes it. A FORTRAN label stays even when nothing goes to it, and code
    after one nothing goes to goes (the label naming the place).
  - **Results** (`go test -v`), instructions in the run:

    | Program | `/NOOPTIMIZE` | `/OPTIMIZE` | Change | Survey's prototype |
    |---|---:|---:|---:|---:|
    | `matmul` | 3,101,599 | 2,658,800 | −14.3% | under 0.1% (−13.8% by hand) |
    | `sieve` | 5,785,872 | 4,785,922 | −17.3% | −7.3% |
    | `subs` | 6,876,884 | 6,671,464 | −3.0% | under 0.1% |
    | `whet` | 2,695,057 | 2,655,457 | −1.5% | −1.5% |
    | `textk` | 485,525 | 371,025 | −23.6% | not measured |
    | DUNGEON's game, whole run | 6,818,609 | 5,596,938 | −17.9% | −16.9% |

    DUNGEON's own code goes from 4,504,906 to about 3,283,000, −27.1%, its FTNRTL part
    being unchanged. It printed the same. Its baseline here is the 6,818,609 the log
    above explains, since these ran after other tests in one session. Data reads
    barely move, as expected: registers come in subtask 9. `whet`'s reads rise by
    exactly 4,000 (0.1%) with only two `SOBGTR`s changed in its code; why wasn't looked
    into.
  - **A test changed:** `directives.for` prints a traceback, whose PCs follow from the
    code's length. So under `/OPTIMIZE`, and between the two ways, `TestFortranPrograms`
    compares output with `PC=` and traceback addresses masked (`maskCodeAddresses`);
    `/NOOPTIMIZE` is still held to the `.out` exactly.
  - **New program:** `optbranch.for`, each value worked by hand and matched on the first
    run both ways. An index and limit sharing storage by EQUIVALENCE, steps up and
    down stopping exactly at zero, a far loop, an arithmetic IF reading a step's
    condition codes, chains, and labels reached only through an ASSIGN or a table.
- **2026-11-01:** Subtask 4, the analyses. `opt/graph.go`, `loops.go`, `places.go`, and
  `effects.go`, each teaching its technique in its comment; no pass uses them yet, and
  the compiler's output is unchanged.
  - **The graph** (Decision 1): `Graph.Entries` are the first block, each ENTRY
    statement's block, and each variable format expression's first block, under an
    imaginary root. Dominators are Cooper, Harvey, and Kennedy's iteration. A natural
    loop is any back edge's, however it's written; two back edges to one header are one
    `Loop`. A retreating edge that isn't a back edge is listed in `Irreducible`, its cycle
    having a second way in, and no pass may treat that as a loop. Those are rare: 16
    edges against 318 loops, counting the 581 routines below, all in small programs about
    jumps (`jumps.for`, `flow.for`, and the golden `altreturn.for` and `jumps.for`), none in
    a kernel.
  - **Decision 1 was wrong about assigned GO TO, and the manual settles it.** The decision
    says it "lists its targets", and `ir.IndirectJump`'s comment agreed, but section 5.6.3
    says the list "does not affect statement execution" and can be omitted, and the
    generated `JMP @v` doesn't look at it. So `GO TO K, (20)` with K assigned 30 goes to 30.
    An `IndirectJump`'s edges in the graph are its list plus every label the unit ASSIGNs
    (the ASSIGN and the GO TO must be in one unit, the same section says). A label ASSIGNed
    where no jump can reach it counts as an entry, since its symbol must stay defined.
    `optflow.for` has the case, a block nothing else reaches.
  - **The survey was wrong about recursion, and that settles what subtask 2 left open.**
    Decision 7 says the manual never mentions recursion, but section 6.2 does: "A subprogram
    can refer to other subprograms, but it cannot refer to itself, either directly or
    indirectly." So a local that isn't taken survives a call, by the language's rule. An AST
    routine that reenters a routine, or changes a local or COMMON variable between two
    instructions, is outside what any analysis can see, and VOLATILE is how to say so
    (section 4.16). `asts.for`'s `SHARED` holds as it stands: STAT is in an I/O list, so
    taken, and the AST comes during the `$DCLAST` call (`TestAstsProgram`). The new
    `optasts.for` has what isn't caught that way: a static VOLATILE variable no call is
    given, changed by a reentry; VOLATILE COMMON changed during a call; and a loop with no
    call in it that only a timer's AST ends. Its output was worked by hand and matched on
    the first run both ways. As with the other guards, it says little until a pass exists.
  - **Places** (Decision 7): a `Place` is a region and a byte range. Regions are `$LOCAL`,
    the frame, one COMMON block, read-only data, one dummy, a CHARACTER dummy's descriptor,
    one dynamic string, and unknown. A place through a computed address is recognized by
    the forms lowering makes (`throughPointer`). Only `moveRecord`'s pieces, a record past
    65,535 bytes, are unknown: 4 of the corpus's 21,996 places. An AUTOMATIC variable read
    from a format expression through `$FP` lands on the same frame range as its cell.
  - **Choices the manual doesn't settle (ours, unconfirmed):** a subscript or substring that
    isn't constant is held to its array or variable's own cell, the FORTRAN standard's rule,
    which the manual doesn't state. So a program relying on running off the end of an array
    into its neighbor in COMMON may print something different under `/OPTIMIZE`, as with
    answer 1's aliased dummies. `/NOOPTIMIZE` still does what it did. A constant subscript's
    range is exact, wherever it falls. Also, a dummy conflicts with no other storage, not
    even a variable the caller could have passed, which is answer 1 taken at its word.
  - **What is taken** (`Storage.findTaken`): an address in an `Arg.Ref`, an `Addr` (but not
    one that only reaches a packed field in the same instruction, `Mem.Ptr`), a descriptor
    in read-only data, a FORMAT block's cache cell, and anything a format expression
    touches. It's computed over every block, reachable or not, so deleting a block can't
    change the answer. `IOSTAT=` isn't among them: FTNRTL returns the status and the
    statement's own code stores it. `TestTaken` has a case for each way in Decision 7's list.
    A descriptor's or namelist's datum exists only if used, so a NAMELIST never read or
    written takes nothing.
  - **Testing:** the analyses have small programs for each form, and `TestGraphsOnCorpus` and
    `TestStorageOnCorpus` run on 581 routines from 168 of the 171 files to hand (the module
    programs included, definition modules opened), the 3 others being `modules/ledger`'s, which
    INCLUDE a file beside them.
    Dominators are checked against their definition, by removing each block and seeing what
    can still be reached, and each loop against the definition of a natural loop.
- **2026-11-01:** Subtask 5, forwarding and dead code: three IR passes, the first in
  `Passes`, each teaching its technique in its comment. `/NOOPTIMIZE`'s output is
  unchanged.
  - **`opt/forward.go`** is constants, in every block and across blocks: a forward
    analysis over the graph, where a block starts with what all the blocks that can go to
    it agree on, and an entry with nothing. A known index becomes part of the offset, what
    comes out constant is folded by lowering's own rules (`ir.Fold`, and comparisons here), and
    a settled `Branch`, `Switch`, or `Sign` becomes a `Jump` (`ir.NewJump`), after which the
    analysis runs again, since a way in has gone. A store of what a place already holds goes.
    A place has facts only if it is known exactly and isn't VOLATILE, a fact ends at any store
    `Conflict` says can touch it and at a call by `ChangedByCall`, and a load uses one only of
    the same type, so a REAL read of an INTEGER's bits still reads them.
  - **`opt/unreachable.go`** removes what the graph can't reach, repeatedly, since a block
    removed may hold the last ASSIGN of another's label.
  - **`opt/deadcode.go`** is liveness over the cells that can die (`Storage.findDying`:
    temporaries, DO counts and steps, a function's value; not taken, not VOLATILE). A dead
    store goes unless its value may fault, a store just before the `RETURN` that reads it goes
    into the `RETURN` even if it may, and temporaries, counts, and steps nothing uses leave
    `$LOCAL`, the compiler's cells after them closing up.
  - **A choice the manual doesn't settle (ours):** a place live at an entry is treated as live at
    every exit. So a function with a path that doesn't set its value goes on returning the last
    call's, as static storage always did, though the standard calls that undefined; the same rule
    covers a DO count read after an ENTRY statement in the loop. Decision 6 says a function's value
    can die, and this is the one case where it doesn't. `optforward.for`'s `KEEP` is the check.
  - **Dead stores and faults:** Decision 8 speaks of moving and merging, not removing. Removing
    `F = 5/K` could lose a program's only condition, so this keeps it (`mayFault`: a division, a
    floating operation, a floating load). A Raw constant has no facts, for the same reason.
  - **Copies were tried and taken out.** The survey and the plan's "store to load" suggest that
    after `$T = N`, a read of `$T` could read `N`. It saves no memory reference, the temporary usually
    stays live, a dummy's address is read as well, and it can cost a `TST` the generator had left out.
    Leaving them out gained 413 instructions and 47,604 reads on DUNGEON, and no corpus program ran more
    instructions for it (`text.for` had been 2 worse than before the pass); other programs' reads moved by
    at most 131 either way. What copies were wanted for, a function's value, the `RETURN` does. So store-to-load for a value that
    isn't constant waits for registers (subtask 9).
  - **Not done, on purpose:**
    - A DATA value as a constant (Decision 6 allows it if nothing in the unit can change it): nothing
      measured asks for it.
    - An AUTOMATIC variable dying at return: it could, but Decision 6 lists only the compiler's cells
      and the value, and it is rare.
    - A dead `Call.Result`: dropping it would leave `R2` and `R3` out of the entry mask when an `OTS$..._R3`
      routine changes them, since `codegen`'s `call` notes them from the result's width.
    - Read-only data no statement uses any more: a removed statement's descriptors and control blocks
      stay in `$PDATA`, and a descriptor's target stays taken.
    - Duplicating a `RETURN` into the blocks that jump to it, which would let each arm of an IF forward
      its own value.
  - **New program:** `optforward.for`, worked by hand, matched on the first run both ways. Partial stores,
    types sharing storage, arms that agree and don't, loops, DO counts of no trips, what a call changes,
    each settled terminator, an ASSIGNed label, D and complex values, known character lengths, each
    shape of function value, and the faults that have to stay (`LIB$FIXUP_FLT` on a Raw constant's load).
  - **Testing:** `forward_test.go` has a small program for each rule. `TestPassesOnCorpus` runs the passes
    on 589 routines, with `ir.Check` after each; every block left can run (2,393 to 2,335 blocks), and
    a second run changes nothing. `TestBoundsHold` holds with them. A negative or too-long constant
    length isn't put in a `Text`, since `charMove` takes it for a length (`TestCharacterLengths`).
  - **Results**, instructions in the run, cumulative with subtask 3's:

    | Program | `/NOOPTIMIZE` | Subtask 3 | Now | Writes, subtask 3 to now |
    |---|---:|---:|---:|---:|
    | DUNGEON's game, whole run | 6,818,609 | 5,596,938 | 5,528,332 (−18.9%) | 2,641,371 to 2,581,016 |
    | `subs` | 6,876,884 | 6,671,464 | 6,669,464 (−3.0%) | 1,843,907 to 1,841,907 |
    | `matmul`, `sieve`, `whet`, `textk` | | | unchanged | unchanged |

    As the survey expected, little is left for this pass: 1.2% of DUNGEON's instructions and 2.3% of
    its writes, which weren't broken down further. The kernels' loops have no constants to find, and
    wait for subtasks 6 to 9. Against subtask 3's build no program ran more instructions or wrote more.
  - **Data reads move with layout, not only with code.** `subs`'s reads fell 808,280 with 2,000 fewer
    instructions, for a change of a few bytes in `FLT`, far from its loops, and `whet`'s rise of 4,000 in
    subtask 3 looks the same. The likely cause, not checked, is an instruction across a page's end, whose
    fetch misses the window and goes through the single-byte reads `Stats` counts. So the survey's read
    counts aren't all data, and a small change in reads says little: 11 corpus programs' reads rose by 3 to
    112, with no more instructions or writes than before.
- **2026-11-01:** Subtask 6, local common subexpressions: `opt/cse.go`, the fourth IR pass, teaching its
  technique in its comment. `/NOOPTIMIZE`'s output is unchanged.
  - **Finding a place once.** `C(I,J) = C(I,J) + X` turned out to be most of what the survey's "local CSE" stage
    gained, and it needs no temporary: `ADDF2 X,C-244[R1]`. `Assign.InPlace` asks the code generator for this
    (`storeInPlace`), and `ir.InPlaceOperand` says when it can be asked: either side of `+`, `*`, `.OR.`, and `IEOR`,
    the first of `-`, `/`, and `.AND.`. Check holds the mark to it, and Forward clears it, as what it finds may leave
    the statement no operation on its own place. Nothing VOLATILE is marked.
  - **Merging.** A value is one run of an expression through a block, ended by any store `Conflict` says can touch what
    it reads, or a call by `ChangedByCall`. `A+B` is `B+A`, by key. Nothing reading something VOLATILE merges. The value is
    computed just before the first statement using it, into `$Un`, a temporary in memory (Decision 2); temporaries go back
    for later blocks' use at a block's end.
  - **A short-circuited operand can't be a value's first use.** `IF (N.NE.0 .AND. M/N.GT.2)`: code generator's `branchTo`
    doesn't run the second operand if the first decides it, and `M/N` ahead of the test would fault where the original
    didn't (Decision 8). It can use a value already computed.
  - **A value already in a variable:** `K = J*5 + I` then `A(J*5 + I) = 0.0` reads K, with no store added, as long as K holds it.
  - **Decision 2 shows its cost here, and that's what the cost model is for.** A temporary in memory takes a store and a
    read per use, and an index has to be moved to a register too, so `MULL3 J,#60,R1; ADDL2 I,R1` twice is no better
    as `$U1`, and worse by a write. `worth` counts what each choice runs, and takes a temporary only if it saves an
    instruction and costs no more memory references than it saves instructions. That refuses `J*60` shared between
    `matmul`'s `B(K,J)` and `C(I,J)`, which the survey's hand version kept in a register for the statement: its third
    instruction of three, waiting for subtask 9. **Subtask 9 must change `worth`:** a temporary in a register costs none
    of that, and it should be the first thing looked at there.
  - **Left alone:**
    - A variable format expression's blocks, which run in the middle of another block's I/O statement and would share
      its temporaries.
    - `Mem.Ptr` itself, since `throughPointer` knows a place by its form. So `subs`'s descriptor A0s stay for subtask 7, and
      its `MOVL BUBBLE.$B1+16,R0` before every element, and the index with it, are unchanged: `IF` ends its block between
      `A(J)`'s uses, so a local pass sees little there.
    - A call's `Result` and `Through`, whose index is computed after the call, and `CharIndex.Base`, whose form `ir.Check`
      requires.
    - A comparison's value, which a test does far better as a branch.
    - Across blocks: the survey found no case that asks for it.
  - **New program:** `optcse.for`, worked by hand, matched on the first run both ways once its layout was right, with every
    value set through a CALL so no earlier pass settles it. Each operator in place and each that can't be; other lengths;
    each form of dummy; each thing that ends a value; a value held and a holder changed, in part, or by its own expression;
    both orders; a short-circuited operand with and without an earlier value; a format expression sharing a value with its list; and
    substrings.
  - **Testing:** `cse_test.go` has a small program for each rule. `TestPassesOnCorpus` runs the pass on every routine with
    `ir.Check` after it, and a second run changes nothing. `TestBoundsHold` holds with the new forms.
  - **Results**, instructions in the run, cumulative with subtasks 3 and 5:

    | Program | `/NOOPTIMIZE` | Subtask 5 | Now | Survey's hand CSE stage |
    |---|---:|---:|---:|---:|
    | `matmul` | 3,101,599 | 2,658,800 | 2,223,200 (−28.3%) | 2,025,199 (−34.7%) |
    | `whet` | 2,695,057 | 2,655,457 | 2,475,457 (−8.1%) | not measured |
    | DUNGEON's game, whole run | 6,818,609 | 5,528,332 | 5,526,804 (−18.9%) | not measured |
    | `sieve`, `subs`, `textk` | | | unchanged | not measured |

    `matmul` has two thirds of the hand stage's gain, nearly all of it from finding `C(I,J)` once. Its reads fall 435,601 and its
    writes rise 3,599, about a store per trip of its first loop, whose three uses of `J*60+I` are the one temporary it gets. `whet` trades: reads fall 180,000 and writes rise
    120,000, its elements of `E` read from two temporaries. DUNGEON has very little that a block repeats and that's worth a temporary
    in memory: its `QHERE` is a loop of comparisons whose gain is in registers. Its reads rose 74,198, with its writes down 40 and 1,528
    fewer instructions. A temporary would have added writes, so this looks like the layout effect subtask 5 noted, though it wasn't
    checked further.
  - **Not done, and not in any subtask yet:** the author's answer 4, `CMPB` and `MOVB` for one-character strings under
    `/OPTIMIZE`, which the survey said could go with subtask 3 or a small subtask of its own. DUNGEON runs 28,856 `CMPC3`s, and `textk`
    is built on them. It needs a place in the list.
- **2026-11-02:** The author placed answer 4 as a subtask of its own after loop-invariant motion: subtask 8, one-character strings.
  Register allocation is now 9, induction variables 10, FTNRTL's loops 11, and the close-out 12; this doc's earlier log entries that look
  ahead to those subtasks, and `cse.go`'s comment, use the new numbers, except the entry that first numbered FTNRTL's loops.
- **2026-11-02:** Subtask 7, loop-invariant motion: `opt/invariant.go` and `opt/deferred.go`, two more IR passes before common
  subexpressions, each teaching its technique in its comment; the walk through what a statement computes moved from `cse.go` to `walker.go`,
  which both use. `/NOOPTIMIZE`'s output is unchanged.
  - **The pass.** Each natural loop, outermost first, gets a new block before its header, its preheader, if anything moves. The largest
    expression that nothing in the loop changes goes into a temporary in memory, `$Hn`. A value moves as far out as it can: in `matmul`
    `J*60` goes before the K loop, further than the survey's hand version put it. An outer loop refusing a value leaves it for the loops inside.
  - **Stricter than Decision 8, on purpose.** The decision says what can fault moves only "from blocks that run on every trip". An exit taken on
    the first trip gets past that: in `DO: IF (Z.EQ.0.0) GO TO 90; X(I) = Y/Z`, the division's block is on every trip that goes round, and the
    program never divides by zero. So what can fault needs one use the first trip surely reaches (`surely`): in a block that every latch and every
    exit comes through, with no call anywhere before it, as a call may not come back (`IF (Z.EQ.0.0) CALL GIVEUP`). With that, the program ran it
    anyway with the same operands. A load through an index or a dummy argument counts as able to fault too, which Decision 8 doesn't list, since a
    test may be keeping it off a bad address or a dummy that wasn't passed (`unsafeToMove`). `optinvar.for` has both. A fault can still come sooner
    than it would have: a store before it on the first trip has then not run when a handler sees it. Output can't go
    this way, since writing it is a call.
  - **What it can't do:** a loop whose header an ENTRY statement reaches, or an assigned GO TO from outside (`canPrecede`), since nothing can be put on those
    paths. A loop that starts its routine gets a new first block.
  - **Moved code stays readable (Decision 12):** `; from line 17` on its first instruction, and its source is what comes before the loop, so the file's lines stay in
    order. The storage map says "a value computed before a loop". `Assign.Moved` holds the statement.
  - **Hoisted array bases and A0s are not done, and this changes what the subtask said.** In memory they gain nothing: `MOVL I,R0; @$H1[R0]` is one
    instruction and three reads, the same as `ADDL3 I,$H1,R0; A-244[R0]`, and an A0 in another cell costs what it costs where it is. Their gain is all in registers,
    `(R6)[R9]`, so they are subtask 9's, along with `J*60` used alone as an index, which `worth`'s sum (`movable`) refuses for the same reason. **Subtask 9 must revisit `movable`
    as well as `cse.go`'s `worth`.** `throughPointer` must also learn a hoisted address's form, or the place becomes `UnknownRegion`, which conflicts with every store.
  - **Deferred indexes, added:** what the survey blamed on A0 (pattern 5, two `MOVL`s per reference) can be helped now without moving anything. `@S.$B1+16[R1]` looks through the
    longword in memory and indexes what it finds, one instruction fewer per reference and the same reads. `Mem.Deferred` asks for it, `DeferredIndexes` sets it on every place with a
    `Ptr` and an index, and the code generator (`deferredIndex`, `direct`, `pointerMem`) uses it where the address is such a longword, an offset going into the index as a dummy's does. This is most of
    `subs`'s gain, and with fewer registers held it spills less, so its writes fall too.
  - **A bug the corpus test caught:** a value inside another found elsewhere (`K*3`, and `K+K*3`). Rewriting `K*3` first changed the larger one's key, so its uses weren't replaced and its temporary was left
    unread. `TestPassesOnCorpus`'s second run showed it, in `numeric`. The longest now go first (`hoist`), and each shorter one is replaced in the definitions already placed.
  - **New program:** `optinvar.for`, worked by hand; each value matched on the first run. Its expected file had two errors of mine: `STOP`'s `FORTRAN STOP`, and a trailing `1X`, which writes no blank. With the fault
    rule disabled it fails, under `/OPTIMIZE` only, in both paging modes.
  - **Testing:** `invariant_test.go` has a small program for each rule and each thing that stops a value. `TestPassesOnCorpus` holds a second run to no change.
  - **Results**, cumulative:

    | Program | `/NOOPTIMIZE` | Subtask 6 | Now | Survey's hand stage |
    |---|---:|---:|---:|---:|
    | `matmul` | 3,101,599 | 2,223,200 | 1,362,920 (−56.1%) | 1,391,599 (−55.1%) |
    | `subs` | 6,876,884 | 6,669,464 | 3,866,184 (−43.8%) | not measured at this stage |
    | DUNGEON's game, whole run | 6,818,609 | 5,526,804 | 5,512,277 (−19.2%) | not measured |
    | `sieve`, `whet`, `textk` | | | unchanged | not measured |

    `matmul` passes the hand stage in instructions, with reads and writes close to its (2,156,480 and 690,546 against 2,160,122 and 694,027). `subs`'s reads fall from 7,595,774 to 5,592,479 and its writes from 1,843,907 to 1,442,892.
    `sieve`, `whet`, and `textk` run the same instructions as before. `whet`'s loops read plain variables, which gain nothing from a temporary in memory; `sieve`'s and `textk`'s weren't looked into. DUNGEON's reads fall 519,789 against `/NOOPTIMIZE`, but its writes are 22,826 above subtask 5's; a store per
    loop entered would explain that, though it wasn't checked. Neither was why its gain is so small here.
