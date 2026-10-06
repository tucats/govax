# Performance studies

This file tracks performance audits of govax's emulator. Each audit is a
**study**: one workload, measured and profiled. Its **observations** record
where the time goes, with the evidence. Its **recommendations** are ranked by
expected gain against risk. Its **implementation log** records what was
changed, with before/after numbers. Later studies use other workloads and
reuse the method below. They also re-measure earlier studies' workloads, so
a fix that helps one program and hurts another shows up.

Contents:

- [Method](#method): tools, conventions, how to profile a run
- [Study index](#study-index)
- [Study 1: PI to 10,000 places](#study-1-pi-to-10000-places)
- [Deferred design issues](#deferred-design-issues)

## Method

### Measuring

`govax -s` prints emulation statistics when govax exits
(`cmd/govax/main.go`'s `printStats`). The ones a study cares about:

| Statistic | What it counts |
|---|---|
| Elapsed Time | Wall time since the process started. Startup (boot, `vax.init`, assembling `kernel.asm`) takes about 5 ms, so for a run of seconds this is effectively execution time. |
| Instructions | `Engine.Step` calls that ran an instruction. |
| Interrupts | Interrupts delivered (`deliverPendingInterrupt`). |
| Page Translations | Full **page-table walks**: TB misses that read a PTE from memory. |
| Single-Byte Reads | `LoadByte` calls. Most are instruction-stream bytes. |
| Multi-byte Reads/Writes | Word, longword, and quadword accesses that stayed within one page. |
| Sequential Cache Tries/Hits | Every virtual-address translation (the single-entry "STC" in front of the TB), and how many it satisfied. |
| TB Cache Tries/Hits | Translations that got past the STC to the 128-entry TB, and how many the TB satisfied. |

The headline metric for comparing studies is **ns per instruction**
(Elapsed ÷ Instructions), since different programs run very different
instruction counts.

### Profiling

`--cpu-profile FILE` (added for Study 1, `cmd/govax/profile.go`) runs Go's
sampling CPU profiler over the whole govax run and writes a pprof profile:

```sh
go build -o /tmp/govax ./cmd/govax
/tmp/govax -s --cpu-profile /tmp/pi.prof run pi 10000
go tool pprof -top -nodecount=40 /tmp/govax /tmp/pi.prof      # flat/cum by function
go tool pprof -list 'cpu.decodeInstruction$' /tmp/govax /tmp/pi.prof  # by source line
go tool pprof -peek 'runtime.mapaccess2_fast64$' /tmp/govax /tmp/pi.prof  # who calls it
go tool pprof -http=:8080 /tmp/govax /tmp/pi.prof             # flame graph
```

Pass the binary as well as the profile so pprof can resolve symbols. The
profiler costs a few percent, so take timings from runs **without**
`--cpu-profile`.

`internal/console`'s `BenchmarkSieve` is an older, in-process benchmark
(Phase 12). It calls `bench.asm`'s SIEVE directly, without VMS, the RTL, or
the debugger's run loop. Use it for quick A/B checks of the CPU core.

### Configuration profiles

govax's settings change what the emulator does per instruction. The most
important is `vax.hardware.clock` (see Study 1, O1). Studies record which
settings they ran under. The `perf` configuration profile
(`~/.govax/govax.json`, created for Study 1) holds audit settings. It is the
same as `default` except that `vax.hardware.clock` is `false`
(deterministic, quantum-driven clock):

```sh
govax --profile perf -s run pi 10000     # quantum clock
govax -s run pi 10000                    # default profile: hardware clock
```

### Temporary instrumentation

Some questions need counters the statistics don't have, such as *why* the
TB misses. Add them as a throwaway patch: build a scratch binary, then
`git checkout` the files. Record the patch's idea and its result in the
study, and don't commit it. If a counter turns out to be useful for every
study, promote it to `-s` as a recommendation.

### Recording a study

1. Record the commit, machine, Go version, command, and configuration profile.
2. Run the baseline at least twice without the profiler, and record the
   `-s` numbers.
3. Profile. Record top functions (flat and cum) and the line-level hot
   spots that matter.
4. Write **observations** (O1, O2, ...): each one a measured fact, with its
   evidence and cost in seconds or percent.
5. Write **recommendations** (R1, R2, ...): change, expected gain, risk,
   fidelity notes, ranked. An estimate is labeled as an estimate.
6. As fixes land, append to the **implementation log**: what changed (commit),
   before/after numbers for this study's workload, and a re-check of earlier
   studies' workloads.

## Study index

| # | Workload | Date | Baseline | Status |
|---|---|---|---|---|
| 1 | `pi.mar`, `run pi 10000` | 2026-10-06 | 9.6 s, 83.3 M instr, 115 ns/instr | R1–R3 done: 4.3–4.5 s, 52–54 ns/instr (quantum mode 22.1 → 4.5 s) |

---

## Study 1: PI to 10,000 places

### Workload

`testdata/mar/pi.mar` (also in the default container). It computes π with
multi-precision longword arithmetic (BASE 10⁹) and prints it. The work is in
small loops over longword arrays. The hottest is DIVIDE's:

```
10$:	EMUL	R4, #BASE, (R2)[R5], R6
	EDIV	12(AP), R6, (R3)[R5], R4
	AOBLSS	WORDS, R5, 10$
```

This loop reads an array element (`(R2)[R5]`), reads an argument from the
stack (`12(AP)`), reads a static (`WORDS`), and writes an array element
(`(R3)[R5]`). Several of the loops divide in place, with source and
destination the same array. The run time is O(n²) in the digit count:
1,000 digits take 0.93 M instructions, 3,000 take 7.7 M, and 10,000 take
83.3 M.

### Environment

- Commit `a2ba8b3` plus `--cpu-profile`; Go 1.26.0, darwin/arm64; Apple M5 Max.
- `govax -s run pi 10000`, `default` configuration profile
  (`vax.hardware.clock` = true). Comparison runs used the `perf` profile
  (quantum clock).

### Baseline

Two runs, default profile, no profiler:

| | Run 1 | Run 2 |
|---|---|---|
| Elapsed | 9.589 s | 9.599 s |
| Instructions | 83,323,723 | 83,323,743 |
| Interrupts | 1,065 | 1,067 |
| Page-table walks | 21,034,383 | 21,034,612 |
| Single-byte reads | 329,963,441 | 329,963,487 |
| Multi-byte reads / writes | 89.1 M / 16.1 M | same |
| STC tries / hits | 435.1 M / 314.9 M (72%) | same |
| TB tries / hits | 120.2 M / 99.2 M (82.5%) | same |

That is **115 ns per instruction** (8.7 M instructions/s). Per instruction:
5.2 translations, 4.0 single-byte reads, and 0.25 page-table walks.

With the `perf` profile (quantum clock) the same program takes **21.5 s,
222.2 M instructions, and 13.9 M interrupts** (see O2).

### Profile summary

Default profile, 9.43 s of samples:

| Function | flat | cum | Notes |
|---|---|---|---|
| `time.Now` (`runtime.walltime` + `nanotime1`) | 3.0 s | **3.12 s (33%)** | `Engine.Step`'s hardware clock (O1) |
| `cpu.decodeInstruction` | 0.96 s | 3.53 s (37%) | includes operand decode and I-stream reads |
| `cpu.decodeOperand` | 1.08 s | 2.47 s (26%) | |
| `Engine.Step` | 0.65 s | 8.82 s | |
| instruction handlers (all) | | ~1.10 s (12%) | the actual work |
| `vm.(*Memory).LoadByte` | 0.29 s | 0.92 s | |
| `vm.(*Memory).translate` | 0.45 s | 0.61 s | |
| `vm.(*Memory).phys` | 0.37 s | 0.37 s | |
| `cpu.(*Table).HandlerFor` (map lookup) | | 0.28 s (3%) | O6 |
| debugger `runLoop` checks | | ~0.35 s (4%) | O7 |

So of 115 ns per instruction, about 38 ns is reading the clock, about 40 ns
is decode, about 14 ns is the instruction itself, and the rest is memory
access and loop bookkeeping.

### Observations

**O1. The hardware clock reads the wall clock on every instruction: 33% of
the run.** With `vax.hardware.clock` set, `Engine.Step`
(`internal/cpu/engine.go`) calls `time.Now().UnixMilli()` before every
instruction to see whether a millisecond has passed. On macOS that is a
real `walltime` + `nanotime` pair, about 37 ns, and costs 3.1 s of the 9.4 s.
Once per millisecond it also does a second `time.Now()` and a `time.Date`
for TODR.
- *Experiment:* reading the clock only every 1024th instruction (about every
  90 µs at this speed, still well under the 1 ms tick) took the run to
  **7.2–7.9 s** with identical digits.
- `checkLimits` has the same pattern: with `--time-limit` set it calls
  `time.Now()` every instruction. This run didn't set one.
- In hardware-clock mode, the number of clock interrupts depends on how fast
  the host runs, so the instruction count varies slightly from run to run.

**O2. In deterministic (quantum) mode, about 62% of instructions are the
clock interrupt handler.** `vax.init` (inherited from eVAX's) does `set
quantum 1`, so `tickQuantum` ticks the interval clock on *every*
instruction. `kernel.asm` sets NICR = −11, so the interval clock interrupts
every 11 instructions. Instrumented counts for the `perf` run: 222,168,241
ticks; 13,885,514 interval interrupts (vector `0xC0`), the only interrupt
apart from one console write. Each interrupt runs `exe$interval`
(INCL, PUSHL, MFPR, TSTL, BEQL, INCL, MTPR, MTPR, MOVL, REI, about 10
instructions). That adds about 139 M instructions to PI's 83 M: 2.7× the
instructions, 2.2× the time. Each interrupt also allocates a `*Fault` (O8),
and each REI runs `InvalidateProtection` over all 128 TB entries (5% of that
profile).
- The handler only counts ticks and increments TODR. In hardware-clock
  mode, `Step` *also* overwrites TODR from the host clock every millisecond,
  so the handler's increment is redundant there.
- Per the author (2026-10-06), the interval clock is a holdover from eVAX's
  hardware emulation. govax's focus is now running VMS programs, and the
  clock mainly serves the microkernel's console output. See R1 and the
  deferred design issues.

**O3. Decode copies a 360-byte struct about three times per instruction:
about 1.2 s (13%).** `Decoded` holds six 56-byte `Operand`s (360 bytes).
`decodeInstruction` builds one on its stack and returns it by value
(`return d`: 0.43 s). `Step` copies it into `e.decoded` (0.21 s). Each
`decodeOperand` builds an `Operand` and returns it by value
(0.15 s + 0.24 s for the `d.Operands[i] = op` store), and `decodeGeneral`
and `decodePCRelative` take and return one by value too. Most instructions
use two or three operands, but all six slots are zeroed and copied every
time. `Operand` itself is padded: `Access`, `Kind`, `Reg`, and `Size` are
each an 8-byte `int` holding values below 256.

**O4. TB misses are almost all read/write thrashing of one entry: 99.6% of
21 M page-table walks.** A TB entry (`internal/vm/tb.go`) keeps the access
type it was filled for (`protMode`), and `translate` hits only when
`entry.protMode == access`. A page that is read and then written (or the
reverse) misses every time the access type changes, and does a full walk:
- the P0 PTE's own address is translated (a recursive S0 translation,
  through the TB and STC again),
- the PTE is read from physical memory and its protection checked,
- the entry is refilled for the new access type.

Instrumented miss reasons for this run: 20,954,872 access-type mismatches,
81,253 tag (page) mismatches, and 55 invalid entries. By region: P0
20.87 M, P1 77 K, S0 88 K. PI's loops read and write the same arrays, and
the in-place divide reads and writes the same element, so nearly every
iteration pays a walk.
- With access type out of the key, the TB's geometry (direct-mapped, 32
  entries per region, indexed by the low 5 bits of the page number) isn't
  a problem for this workload. Only 81 K misses are conflicts. Other
  workloads with larger working sets may disagree; recheck it in later
  studies.
- On a hit, `translate` doesn't re-check protection against the current
  mode. Correctness depends on `InvalidateProtection` running at every mode
  change (REI, CHMx, AST delivery). This is eVAX's design. It is safe, but
  every REI then costs a 128-entry sweep (O2).

**O5. The instruction stream is read a byte at a time through full
translation: 330 M `LoadByte` calls, about 4 per instruction.** The
opcode, each operand specifier byte, and each byte displacement is its own
`LoadByte`. Each one is a call chain
(`LoadByte` → `Translate` → `translate` → `phys`) that checks MAPEN and the
STC, bounds-checks RAM against ROM and NVRAM, and returns a one-byte slice.
`translate` is too large to inline. The specifier-byte read alone in
`decodeOperand` costs 0.76 s. The STC is a single entry shared by
instruction fetch and data, so any operand in another page evicts the code
page. As a result 120 M of 435 M translations (28%) miss the STC, all of
them page mismatches.

**O6. Handler dispatch is a map lookup: 0.28 s (3%).** `Table.HandlerFor`
looks the handler up in a `map[*Instruction]Handler` every instruction
(`runtime.mapaccess2_fast64` + `memhash64`). `Table.Lookup` has already
found the `*Instruction` by array index, so the handler could be a field on
it.

**O7. The debugger's run loop adds per-instruction checks: about 0.35 s
(4%).** A plain `RUN` runs through `debugger.runLoop`
(`internal/debugger/runcontrol.go`). It calls `traceHit`, `breakpointHit`,
`instructionBreakpointHit`, `returnDue`, a `trace(pc)` closure and its
`finish`, `watchHit`, `signalBreak`, and `unhandledBreak` around every
`Step`, even with no breakpoint, watch, or trace set. `Step` itself also
calls `checkLimits`, `checkAttention` (an atomic load), and `deliverAST`
(through `console.NextAST` → `corevms.NextAST`) every instruction. Each is
cheap, but together they are a few nanoseconds per instruction.

**O8. Each interrupt allocates.** `deliverPendingInterrupt` returns
`HandleFault(&Fault{...})`, a heap allocation. This doesn't matter at
1 K interrupts (default profile). At 13.9 M (quantum mode) it shows as
`mallocgc` and GC work (about 3%). Exceptions on the instruction path use
the same `*Fault` pattern.

**O9. Smaller items, noted but not costed:**
- `phys()` checks the ROM and NVRAM ranges only after the RAM range, so the
  common case is cheap. But it returns a slice, so every access pays a slice
  header and a bounds check.
- The `-s` counters (`singlebyteReadCount++` and so on) are plain
  increments on the hot path. They are cheap, and they are what made this
  study possible, so keep them.
- About 2% of samples are Go runtime preemption wakeups (`gopreempt_m` →
  `wakep`). That is the scheduler's normal treatment of a goroutine that
  never blocks; nothing to fix.

### Recommendations

Ranked by expected gain for this workload against risk. Gains are estimates
from the profile, except where an experiment measured them. Expect them to
overlap: removing one cost makes the others a larger share of what's left.

**R1. Stop reading the wall clock per instruction, and rethink the interval
clock (O1, O2). Gain: 20–33% (measured: 9.6 → 7.2–7.9 s). Risk: low for the
first step.**
*Status (2026-10-06): done, by way of step 3 below. The microkernel no
longer uses the interval clock or the console transmit interrupt, and the
engine keeps time without either. Step 2's quantum change turned out
unnecessary (see the implementation log).*
1. *Short term:* in hardware-clock mode, check the time only every N
   instructions (N a power of two, e.g. 1024, so the test is a mask). Better
   still, have a goroutine with a `time.Ticker` set an atomic "tick due"
   flag that `Step` tests, as `checkAttention` already does for Ctrl-C/Y.
   Compute TODR from a base time when it's read (MFPR) or on the tick, not
   with `time.Date` per tick. Do the same for `checkLimits`'s
   `--time-limit` check.
2. *Deterministic mode:* `set quantum 1` makes the interval clock run 1.6
   handler instructions for every program instruction. Raising the quantum
   (e.g. 1000 instructions per emulated millisecond) would cut that to
   noise. That changes how many instructions a fixed-length emulated
   interval takes, so check what depends on it first: `$GETTIM`/`clockTicks`
   (`internal/cpu/systime.go`), any oracle test that counts instructions or
   interrupts, and the console-output path.
3. *Longer term (the author's note, 2026-10-06):* the interval interrupt
   mostly serves the microkernel's console output. Replacing that with an
   XFC or another emulator exit, and dropping the per-instruction clock
   tick, would remove the clock from the hot path in both modes. This needs
   a design decision; see [Deferred design
   issues](#deferred-design-issues).

**R2. Decode in place, and shrink `Operand` (O3). Gain: about 10–13%.
Risk: low; mechanical, and covered by the CPU tests and oracles.**
*Status (2026-10-06): done. The measured gain was much larger than the
estimate: 37–40% (see the implementation log).*
- Have `decodeInstruction` fill `*Decoded` (`&e.decoded`) and
  `decodeOperand` fill `*Operand`, not return values. Reset only the fields
  a decode sets, not all six slots.
- Narrow `Operand`'s fields: `AccessKind`, `OperandKind`, and the size to
  `uint8`, and `Reg` in `Operand` to a byte-sized type. That takes it from
  56 bytes to about 32, which shrinks every remaining copy (handlers take
  `Operand` by value in `Load`/`Store`).

**R3. Make a TB entry valid for both reads and writes (O4). Gain: about
5–8% here (21 M page-table walks removed), more for write-heavy programs.
Risk: medium. This is the VM core; fidelity matters.**
*Status (2026-10-06): done. The walks are gone (21.0 M → 59), but the
measured gain was smaller than the estimate: 2–4%, since a walk cost only
about 7 ns. It also fixed a PROBE bug (see the implementation log).*
- Key entries on the page alone. Cache the PTE's protection code and its
  modify (M) bit in the entry.
- On a hit, check protection against the current mode and access with a
  precomputed table (protection code × mode × access → allowed: 16 × 4 × 2
  bits). This is cheaper than a walk, and it means a mode change no longer
  needs `InvalidateProtection`'s 128-entry sweep. That also closes the
  "hit doesn't re-check protection" dependence noted in O4. The sweep can
  stay as a harmless flush.
- A write hit on an entry whose M bit is clear takes the slow path once, to
  set M in the PTE, and then marks the entry modified.
- Fault behavior (which fault, and its parameters) must stay exactly as it
  is. The existing `internal/vm` tests cover the TB and translation, and
  `TestTB*` should gain read-then-write cases.
- Leave the TB's geometry alone for now (only 81 K conflict misses). Look
  again when a study shows conflicts.

**R4. Fetch the instruction stream through a cached page (O5). Gain: about
8–10%. Risk: medium.**
- Give the engine an instruction-fetch window: the physical base of the
  page holding the PC, the virtual page it maps, and a TB generation number
  (bumped on every TB invalidation) to tell when it is stale. Opcode,
  specifier, and displacement reads within the page then become
  `ram[base+offset]` with no call chain. Crossing a page, or a stale
  generation, refills the window through `translate`, so faults and access
  checks behave as they do now.
- A cheaper first step that captures part of the gain: split the STC into
  separate instruction and data entries, so data operands stop evicting the
  code page.

**R5. Add an inlinable fast path to the memory accessors (O5, O9). Gain:
about 3–5%. Risk: low.** `LoadByte`, `LoadLongword`, `StoreLongword`, and
the rest call `translate` and `phys` unconditionally. A small check that the
compiler can inline (STC hit, address inside `ram`) followed by a direct
`binary.LittleEndian` read on `ram` would skip both calls in the common
case, leaving the existing path as the slow path. Do this after R3 and R4,
since they change what the fast path checks.

**R6. Put each instruction's handler on its `Instruction` (O6). Gain:
about 3%. Risk: very low.** Register handlers into a field of
`Instruction` (or a parallel slice indexed by the table position) instead
of `Table.handlers`. Keep `HandlerFor` for callers outside the hot path.

**R7. Skip the debugger's per-instruction checks when nothing is armed
(O7). Gain: about 3–4%. Risk: low to medium; the debugger's oracle tests
cover it.** Keep a single "anything armed?" summary, updated when
breakpoints, watchpoints, traces, or step/return conditions change. When
it's false, run `Engine.Step` in a tight inner loop and leave only on a
stop condition: an error, a halt, Ctrl-C/Y, or a change to the armed
state. The same idea applies inside `Step`: fold the limit, attention, and
AST checks behind one "slow path pending" flag that their setters raise.

**R8. Avoid allocating on interrupt and exception delivery (O8). Gain:
negligible in the default profile; about 3% in quantum mode until R1 lands.
Risk: low.** Pass the `Fault` by value or use a per-engine scratch `Fault`
on the delivery path.

**Not recommended yet: a predecoded instruction cache.** Caching each
instruction's decode by physical PC is the usual next big step for
interpreters. Here decode has side effects (autoincrement and
autodecrement change registers; indexed and deferred modes read memory), so
only the *shape* of each specifier could be cached, not its resolved
operands. Code-page writes would also need invalidating. Reconsider once
R1–R7 have been measured, if decode still dominates.

**Instrumentation follow-ups:**
- Add a `BenchmarkPI` (`internal/console`, beside `BenchmarkSieve`) that runs
  `pi.exe` for a fixed, smaller digit count (e.g. 2,000), so each change can
  be A/B-tested with `go test -bench` and `benchstat`.
- Have `-s` print instructions per second and ns per instruction, and say
  which clock mode was used.
- Rename the `-s` labels to say what they count: "Page Translations" is
  page-table walks, and the "Sequential Cache Tries" line counts every
  translation. (Also fix the "Elapased" typo.)
- Consider a `-s` breakdown of TB misses by reason (tag, access type,
  invalid), like the temporary counters this study used.

### Implementation log

Instrumentation only so far:

- 2026-10-06: `--cpu-profile FILE` global option (`cmd/govax/profile.go`,
  `grammar.go`; stopped in `main.go` once `govaxApp.Run` returns).
- 2026-10-06: `perf` configuration profile in `~/.govax/govax.json`
  (local, not in git): the `default` profile's settings with
  `vax.hardware.clock` = false. `vax.debug.registers` was not copied over.

**R1: the microkernel off the clock; a synthetic clock (2026-10-06).**
Done as R1's step 3, in three commits, each tested on its own.

1. *Console output through an XFC* (`a01e8d0`). The microkernel's
   `exe$put_one` wrote the console a character at a time: a CHMK into
   kernel mode per character, `MTPR` to the transmit data register
   (TXDB), then a spin on `exe$tx_ready` until the console-transmit
   interrupt's handler (`exe$tx`) set it again. A queued interrupt is only
   admitted on a clock tick, so output needed the clock running. Now
   `exe$put_one` hands the emulator the whole string in one new XFC,
   `XFC$CONSOLE_PUT` (code 4, govax's own: R0 is a string descriptor's
   address; `internal/cpu/xfc.go`, `SystemServices.ConsoleWrite`), or a
   single character to `XFC$CONSOLE_WRITE`. CHMK 0 (`exe$$put_console`)
   uses `XFC$CONSOLE_WRITE` too. `exe$initialize` no longer enables the
   transmit interrupt, and `exe$tx_ready` is gone; `exe$tx` is left as a
   bare `REI` for a program that enables the interrupt itself. The TXCS/
   TXDB emulation in `procreg.go` is unchanged.
   - Output is byte-identical (ABOUT, the boot banner, PI).
   - Two regression fixtures, `input.asm` and `test.asm`, used to spin
     forever on the ready flag (their test doesn't run `exe$initialize`,
     so the interrupt was never enabled). They now complete, and
     `TestRegression_rtlDependentAsmFixtures` expects that.
     `TestAssemble_kernelThenHelloRunsBounded` no longer has to enable
     TXCS by hand.
2. *The interval clock stays stopped* (`482df1f`). `exe$initialize` no
   longer loads NICR, sets TODR, or starts ICCS. `exe$interval` stays in
   the SCB for a program that starts the clock itself. Nothing else
   depended on its interrupt: the RTL's timers and `$GETTIM` already run
   on `Engine.SystemTime` (`clockTicks` in quantum mode), not on the
   handler's tick count. PI to 2,000 places in quantum mode went from
   9.33 M instructions and 583 K interrupts to 3.50 M and none.
3. *A synthetic clock* (`5372e55`, `internal/cpu/clock.go`).
   - TODR is computed when it's read (`MFPR`, and the console's SHOW and
     SET through `Engine.ReadPR`/`TODR`/`SetTODR`): 10 ms units since
     January 1st by the emulated system time, plus an offset that a write
     sets. Nothing has to store it every millisecond. It also reads
     correctly now. Before, the console's `SHOW CLOCK` showed
     `JAN-01 00:00:00`, since TODR was written only while instructions ran
     (hardware mode) or counted from 1 (quantum mode).
   - In hardware-clock mode, `Step` reads the host clock only every 1,024
     instructions (a mask test on the instruction count), and then only
     while the interval clock is running or an interrupt is queued
     (`pollHostClock`). The `--time-limit` check in `checkLimits` is
     batched the same way.
   - Quantum mode is unchanged: `tickQuantum` still counts instructions
     into emulated milliseconds, which keeps timers deterministic.
   - New tests in `internal/cpu/clock_test.go` cover TODR, `MTPR`/`MFPR`
     of TODR, and when the host clock is polled.

Results, PI to 10,000 places (same machine, same session; the old build
is `0bdb2b3`, before R1):

| | Old, default | New, default | Old, `perf` (quantum) | New, `perf` (quantum) |
|---|---|---|---|---|
| Elapsed | 9.95 s | 7.22 s, 7.32 s | 22.10 s | 7.46 s, 7.78 s |
| Instructions | 83,324,133 | 83,313,083 | 222,168,223 | 83,313,083 |
| Interrupts | 1,106 | 0 | 13,885,515 | 0 |
| Page-table walks | 21.0 M | 21.0 M | 149.5 M | 21.0 M |
| ns per instruction | 119 | 87–88 | 99 | 90–93 |

- Default profile: 27% faster, within the 20–33% R1 estimated. `time.Now`
  no longer shows in the profile: of 6.7 s of samples, `Step`'s own
  decode and memory work is what's left (decode 52% cum, `LoadByte` 15%,
  the handler map lookup 6%), so R2–R6 now have larger shares.
- Quantum profile: 2.9× faster, and the program's instructions are now
  all it runs. The extra 128 M page-table walks that came with the
  interrupts are gone too; most likely they were refills after each
  handler REI's `InvalidateProtection` sweep (O2, O4), though this run
  didn't instrument that.
- Both modes now run exactly the same 83,313,083 instructions, run after
  run, with no interrupts. In hardware mode the count used to vary a
  little with host speed (O1).
- Quantum mode is still about 3% slower than default: with `set quantum 1`
  (`vax.init`), `tickQuantum` calls `tickIntervalClock` and
  `scanInterruptQueue` on every instruction, each of which now returns at
  once. Step 2 of R1 (a larger quantum) would remove that, but it also
  changes how fast emulated time runs for timers, so it isn't worth the
  behavior change for 3%. Folding these checks behind R7's "slow path
  pending" flag is the better route.
- The digits are identical in every run.
- Earlier workloads: `BenchmarkSieve` (Phase 12, quantum mode, no
  microkernel initialization, so no clock was ever running) is unchanged
  at 11.19 ms/op, old and new.

Found along the way, and fixed separately (2026-10-06): `govax --time-limit
1s run pi 10000` ran to completion, in the old build and the new. A one-shot
command runs from inside `vax.init` (`INCLUDE/COMMAND_LINE`), but
`cmd/govax` applied `--instruction-limit` and `--time-limit` only after
`vax.init` finished, to keep them off the boot sequence, so they never
reached a one-shot command. The console now holds the limits
(`Console.SetRunLimits`) and applies them (`ApplyRunLimits`) just before the
one-shot command and before the interactive prompt. With the fix the same
command stops at 1.0 s with `%VAX-I-TIMELIMIT`.
`TestRun_limitsStopAOneShotRun` covers both limits. A one-shot run that a
limit stops now also fails: govax exits with status 124 (timeout(1)'s
convention), showing the limit message only once.

**R2: decode in place; a 24-byte `Operand` (2026-10-06).** Two commits,
each tested on its own.

1. *Decode in place* (`f1933bb`). `decodeInstruction` fills in its
   caller's `*Decoded`, and `decodeOperand` (with `decodePCRelative`,
   `decodeGeneral`, `pcRelativeTarget`, and `displacementTarget`) an
   `*Operand`, rather than returning them by value. Each operand slot an
   instruction uses is reset as it is decoded. The slots past
   `OperandCount` are left as an earlier instruction left them, and
   `Decoded`'s comment says nothing may read them.
   - `Step` decodes straight into one of two buffers in the `Engine`
     (`decoded [2]Decoded`), the one that isn't current, and makes it
     current only if the decode succeeds. Decoding into a single buffer
     would have left a half-decoded instruction behind on a decode fault.
     `LastDecoded` (the debugger's `STEP`, the console's operand trace)
     would then show it, or, for an undefined opcode, an instruction with
     a nil `Instruction`. With two buffers it still shows the last
     instruction decoded whole, as it did when `Step` copied a decode into
     place only on success.
   - The tests keep the old value-returning shape through two helpers,
     `decodeInstructionValue` and `decodeOperandValue`
     (`internal/cpu/decodehelpers_test.go`).
2. *A narrower `Operand`* (`090cdd5`). `AccessKind`, `OperandKind`, and
   `vax.Reg` are now `uint8`, and `Operand.Size` is a `uint8` placed
   beside them, so Go packs the struct with no padding: 24 bytes, down from
   56 (`Decoded`: 168 bytes, down from 360). Where a size is passed on as
   an `int`, callers convert it with `int(op.Size)`. `TestOperandSize`
   keeps it at 24.

Results, PI to 10,000 places (same machine, same session; the old build
is `7b3e595`, after R1):

| | Old, default | New, default | Old, `perf` (quantum) | New, `perf` (quantum) |
|---|---|---|---|---|
| Elapsed | 7.27–7.99 s | 4.62 s, 4.62 s | 8.39 s, 8.44 s | 4.81 s, 4.83 s |
| Instructions | 83,313,083 | 83,313,083 | 83,313,083 | 83,313,083 |
| ns per instruction | 87–96 | 55 | 101 | 58 |

- Step 1 alone took the default run from 7.3–8.0 s to 4.4–4.6 s, about
  three times R2's estimate. O3 had costed only the lines that copy a
  `Decoded` or an `Operand` (1.2 s). The flat time in `decodeInstruction`
  and `Step` was nearly all that copying and the zeroing of a fresh
  360-byte `Decoded`, more than the line-level attribution showed. Flat
  time fell from 1.64 s to 0.14 s in `decodeInstruction`, and from 0.71 s
  to 0.20 s in `Step`.
- Step 2 is lost in the noise on PI (4.49–4.54 s before it, 4.49–4.57 s
  after). It mostly makes the remaining copies smaller (handlers pass an
  `Operand` by value to `Load` and `Store`), and those copies were already
  cheap.
- The output is byte-identical to the old build's.
- Earlier workloads: `BenchmarkSieve` went from 11.46 ms/op (old build) to
  6.40 ms/op after step 1, and to 6.28 ms/op after step 2.
- New profile, default run (4.27 s of samples): decode is 46% cum
  (`decodeOperand` 36%), `LoadByte` 19%, `translate` and `phys` 19% between
  them, and the handler map lookup 6%. So R4 and R5 (instruction fetch and
  the memory accessors) and R6 (the map) now have the largest shares.
  Page-table walks are unchanged at 21.0 M, which leaves R3's gain where
  it was in absolute terms and a larger share of the run.

**R3: one TB entry for reads and writes (2026-10-06).** One commit
(`b7be6f4`), done as R3 proposed.

- *The entry* (`internal/vm/tb.go`). A `tbEntry` caches the page's
  protection code and modify (M) bit, and `grants`: for each of the four
  access modes, how much a hit may let through, as a `tbGrant` (none,
  reads, or reads and writes). It comes from `protGrants`, the protection
  check precomputed for all 16 codes and 4 modes (`TestProtGrantsMatchesAllows`
  checks it against `Protection.allows`), and is held to reads while M is
  clear. The values are ordered so that a hit is one comparison,
  `tbGrant(access) < entry.grants[mode]`, and an empty entry (all zero)
  grants nothing.
- *The hit* (`translate`). The TB hits when the page matches and the
  grant for the CPU's current mode lets the access through. Everything else
  walks the page table, as before: a page not cached, a page's first write
  (the walk sets M in the PTE and refills the entry with M set), and an
  access the protection code denies, which the walk reports as the same
  fault, with the same parameters, it always has.
- *The STC*. It keeps one grant, the cached entry's for the mode at fill
  time, so a read hits a slot a write filled and the reverse once M is
  set. It doesn't check the mode on a hit, to stay as cheap as it was, so
  it must still be emptied at a mode change.
- *Mode changes*. Since a TB hit checks the current mode,
  `InvalidateProtection` no longer sweeps the 128 entries; it only empties
  the STC. Pages stay cached across REI, CHMx, and AST delivery.
- *A fidelity fix along the way*. PROBER/PROBEW lower the CPU's mode to
  the probed mode around their translations without
  `InvalidateProtection` (as eVAX's `emul_probe` does). With the old TB, an
  entry cached by a kernel-mode read hit any later read, whatever the
  mode, so probing a kernel-only page for user mode after the kernel had
  read it reported it accessible. Now the hit checks the probed mode.
  Logged in `docs/DEVIATIONS.md`; `TestProbeTranslateChecksProbedMode`
  fails on the old TB. The STC a probe fills under the lowered mode can
  only under-grant, since a less privileged mode never may do more.
- *SHOW TB*. Each entry's `MODE=` now says what a hit lets the current
  mode do (`KERNEL WRITE`, `KERNEL READ`, or `-NONE-`), not the access
  type it was last checked for (`vm.TBEntry.Permits`).
- *Tests*. New `internal/vm` tests cover read-then-write (one walk, to
  set M, then TB hits both ways), write-then-read (an STC hit), a hit
  checked against the current mode (a kernel-cached, kernel-only page
  faults from user mode, with the uncached fault's kind, address, and
  mask), and a write to a read-only page. The tests that expected a mode
  change to make the TB miss now expect the STC to empty and the TB to
  hit (`internal/vm`, `internal/cpu`'s `setModeStack`, the console's
  `SET PSL`).

Results, PI to 10,000 places (same machine, same session; the old build
is `3d107cf`, after R2):

| | Old, default | New, default | Old, `perf` (quantum) | New, `perf` (quantum) |
|---|---|---|---|---|
| Elapsed | 4.43–4.64 s | 4.28–4.50 s | 4.61 s, 4.63 s | 4.51 s, 4.52 s |
| Instructions | 83,313,083 | 83,313,083 | 83,313,083 | 83,313,083 |
| Page-table walks | 21,014,393 | 59 | | |
| Multi-byte reads | 89.1 M | 68.1 M | | |
| STC tries / hits | 435.1 M / 314.9 M | 414.1 M / 314.9 M | | |
| TB tries / hits | 120.2 M / 99.2 M | 99.2 M / 99.2 M | | |
| ns per instruction | 53–56 | 51–54 | 55–56 | 54 |

- In alternating runs of the two builds, the new one was faster every
  time, by 1–4% (old 4.55/4.56/4.64 s, new 4.50/4.46/4.45 s).
- That is less than R3's 5–8% estimate. The walks went (the 21 M
  multi-byte reads they did for PTEs, and the 21 M translations of the
  PTEs' own S0 addresses, went with them), but a walk was cheap: the PTE's
  S0 page was nearly always in the TB, so a walk cost about 7 ns. In the
  profile, `translate`'s cumulative time went only from 0.54 s to 0.52 s
  of about 4.1 s; its flat time went up a little (0.37 → 0.43 s) with
  the mode read and grant lookup on each TB hit.
- The TB now misses only 59 of its 99.2 M tries. What's left is the
  STC's 99.2 M misses (24% of translations), each now a TB hit. They are
  page mismatches (O5), mostly the single slot losing the code page to a
  data operand and back, which R4 (an instruction-fetch window, or
  separate instruction and data STC slots) addresses.
- The output is byte-identical to the old build's.
- Earlier workloads: `BenchmarkSieve` went from 6.60–6.69 ms/op to
  6.45–6.51 ms/op.
- The gain should be larger for programs that change mode often (system
  services, ASTs), since a mode change no longer costs a 128-entry sweep
  and the walks that followed it. PI changes mode only a few times.

---

## Deferred design issues

**Clock, console output, and context switching (from Study 1, 2026-10-06).**
*Console output and the clock: done (Study 1, R1). The microkernel writes
the console through `XFC$CONSOLE_PUT` and leaves the interval clock
stopped, and the engine keeps time itself (`internal/cpu/clock.go`).
Context switching is still deferred.*

The interval clock interrupt is a holdover from eVAX's goal of emulating VAX
hardware. govax's focus is now running VMS programs, and the clock
mainly serves the microkernel's console output. (Its handler,
`kernel.asm`'s `exe$interval`, itself only counts ticks and advances
TODR.) That service could be provided by an XFC or another exit
into the emulator instead, which would let the per-instruction clock tick go
entirely (Study 1, R1.3). Something will eventually have to gate context
switching once govax supports multiple processes. That can be a scheduler
quantum checked cheaply (a counter or an atomic flag), not an interrupt
path run every few instructions. The multi-process design is deferred.
