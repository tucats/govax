# Phase 03: Instruction decode engine

## Goal

Build the variable-length VAX instruction fetch/decode engine, operand resolution, and
the fetch-decode-execute loop — the scaffolding that Phases 04-07's instruction
families plug into. At the end of this phase the engine should be able to decode (not
yet execute) any instruction opcode and its operand specifiers.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/decode_opcode.c` — opcode fetch/dispatch.
- `reference/eVAX/eVAX/Source/CPU/decode_operand.c` — operand specifier parsing
  (addressing modes).
- `reference/eVAX/eVAX/Source/CPU/vax.c` — the main fetch-decode-execute loop.
- `reference/eVAX/eVAX/Source/CPU/interrupt.c`, `memory_io.c` — interrupt/fault queues
  and memory-mapped I/O hooks the loop interacts with.
- `reference/eVAX/eVAX/Headers/instruction_table.h` — opcode table structure.
- `reference/eVAX/eVAX/Source/Initialization/init_emulators.c` — builds the opcode
  table at startup; port as the Go equivalent's table construction (likely a static Go
  table/map rather than a runtime-built structure, since Go doesn't need C's
  init-time construction pattern).

## Deliverables

- An `internal/cpu` package with a decode step that can walk any VAX opcode + operand
  specifiers against `internal/vm` memory and `internal/vax.CPU` registers, and an
  execute dispatch mechanism ready for Phases 04-07 to register instruction handlers
  into.
- Unit tests: decode correctness for a representative opcode from each addressing mode
  and each opcode length/prefix pattern, cross-checked against
  `reference/eVAX/AUDIT.md` and the VAX ISA manual for any previously-fixed corner
  cases.

## Design notes (from C source inventory)

### Dispatch mechanism — resolved

Per user direction: a function dispatch table, matching the C source's own design
(`instruction[opcode.index].routine`, an array of function pointers built once by
`init_emulators.c`). No architectural or performance reason favors a big switch or
per-family sub-dispatch instead — a table is what Phases 04-07 will want to register
handlers into incrementally anyway (mirroring `init_emulators.c`'s "only the
implemented ones get overridden, the rest fault" pattern), so this isn't revisited.

### Instruction table: generated, not hand-transcribed

`instruction_table.h` is itself machine-generated in the C project (its own header
comment says so) and has ~284 entries in a rigidly consistent 9-field-per-entry format.
Hand-transcribing that into a Go literal risks silent transcription errors in opcode
values, operand scales, and access modes — exactly the kind of bug this project's
"comprehensive unit test suite the C version never had" goal exists to prevent, but
better avoided than caught. Instead: a small generator
(`internal/cpu/gen/main.go`, run via `go generate`) parses
`reference/eVAX/eVAX/Headers/instruction_table.h` directly and emits
`internal/cpu/instructions_table.go`. This keeps the Go table mechanically traceable to
the reference file rather than hand-copied, and can be re-run if upstream ever changes.

Table lookup: single-byte opcodes (`0x00`-`0xFC`) index a flat `[256]*Instruction`
array directly, matching the C source. Extended (two-byte, prefix `0xFD`/`0xFE`/`0xFF`)
opcodes use a `map[uint16]*Instruction` keyed by `extended<<8|opcode` instead of the C
source's linear scan from index 256 — same result, O(1) instead of O(n); this is a pure
lookup-strategy improvement with no behavioral difference (same category as Phase 01/02
not porting 1999-era speed hacks like `PSL_W` or the TB/STC cache), not logged to
`DEVIATIONS.md`.

### Operand representation: value-based, not pointer-based

The single biggest departure from the C source's mechanism, and worth explaining up
front since Phases 04-07's instruction handlers are built against it.

The C source's `decode_operand()` resolves each operand to a **raw pointer**
(`opcode->address[n]`) into either the register file or a scratch "temporary register"
slot (`vax.reg[16..]`), so that `get_operand()`/`put_operand()` can later
read/write through that pointer uniformly regardless of whether the operand is a
register, a memory location, or a literal. Short literals and immediates are copied
into scratch registers *purely* to get a pointer to dereference; quadword memory
operands are reassembled into a pair of scratch registers for the same reason. This
pointer-uniformity trick is also the direct cause of `AUDIT.md` finding C2 (quadword
operand reconstruction bug) — the scratch-register bookkeeping it requires is a real
source of bugs, not an incidental detail.

Go has no equivalent need: a method can provide the same uniform read/write interface
without aliasing through a pointer into shared state. So `cpu.Operand` instead carries
enough information to resolve its value **on demand**:

```go
type OperandKind int

const (
    OperandRegister  OperandKind = iota // value lives in a GPR (Reg)
    OperandMemory                       // value lives at a VAX virtual address (Addr)
    OperandImmediate                    // value is resolved at decode time (Value)
)

type Operand struct {
    Access AccessKind  // this operand's access mode from the instruction table
    Kind   OperandKind
    Reg    vax.Reg      // valid when Kind == OperandRegister
    Addr   uint32       // valid when Kind == OperandMemory; also the resolved
                         // address itself for OP_AD/OP_BR/OP_VA-access operands
                         // (used directly, never dereferenced — same as VAXaddr[n]
                         // in the C source)
    Value  uint64        // valid when Kind == OperandImmediate
    Size   int           // operand size in bytes: 1, 2, 4, or 8
}
```

Consequences of this choice, so later phases aren't surprised:

- No scratch/temporary-register bookkeeping (`vax.treg`, the two-counter dance between
  decode-time literal placement and execute-time quadword reassembly) is ported at
  all — there's nothing for it to do once operand access is value-based rather than
  pointer-based. `vax.CPU`'s register file (R0-R63) is not used as decode scratch
  space by this port; registers 16-63 remain available as ordinary addressable
  temporaries the way the C source's `T0`-`T5` mnemonics use them, just not as
  operand-decode plumbing.
- `Operand.Load`/`Operand.Store` (Phase 03 sub-phase 5, the `get_operand`/
  `put_operand` equivalents) take `*vax.CPU`/`*vm.Memory` and resolve the value fresh
  each call — no aliasing, so `AUDIT.md` C2's bug class can't recur here by
  construction.
- Autoincrement/autodecrement/deferred addressing still mutate the base register
  **once, at decode time**, exactly as the C source does — only the later
  "how do I get the value" step changed, not addressing-mode semantics.

### Short-literal floating operands: decoded structurally, resolved in Phase 05

`decode_operand.c`'s short-literal path (addressing modes 0-3) calls `fpu_store()` to
convert `short_double[]` table entries into VAX F/D-floating bit patterns when the
instruction's short-literal type is `OP_TYPE_FLOAT`. `fpu_store` is a nontrivial,
easy-to-get-wrong bit-twiddling routine (pointer-cast byte reordering, and its
`BIGENDIAN`-branch `LSB`/`MSB` macros disagree with their own inline comments — e.g.
`pd[ LSB /* 0 */ ]` where `LSB` is `1` on the non-`BIGENDIAN` branch this port
targets), and it's explicitly Phase 05's deliverable (`docs/PHASE-05.md`: "port F/D-
floating conversion and arithmetic"). Reverse-engineering it ahead of schedule for one
decode corner case, with no test yet built to verify it, is a good way to bake in a
silent wrong-bit-pattern bug.

Mode/PC-advancement is unaffected either way (a short literal is the addressing-mode
byte itself; no extra bytes are read regardless of int-vs-float interpretation), so
this doesn't block full addressing-mode coverage. Decode resolves this case
structurally — correct mode recognition, correct 6-bit index, correct
`OperandImmediate` classification — and carries the value as the *native* `float64`
from `short_double[]` (via `math.Float64bits`) rather than a VAX F/D-floating bit
pattern. Phase 05's float instruction handlers, which need the real `fpu_store`/
`fpu_load` port anyway, are what convert it the rest of the way. Not logged to
`DEVIATIONS.md` — this is a sequencing choice, not a suspected fidelity issue.

### Fault/exception handling is in scope; device interrupts are not

`interrupt.c`'s `set_fault`/`handle_fault`/`set_mode_stack` (build a fault frame,
consult the SCB vector at `SCBB + code`, push PC/PSL/signal-args on the
appropriately-chosen stack, switch access mode) is self-contained CPU/memory state
with no dependency on device interrupts, the console, or disassembly — it's what
`decode_operand.c`'s `set_fault(EXC_RESADDR, ...)` etc. already call into, so it has to
exist for decode-time faults to mean anything. This is ported now as `cpu.Fault` (the
`struct FAULT` equivalent: exception code, signal args, faulting PC) plus
`Engine.HandleFault`.

Explicitly **out of scope**, deferred to their already-assigned phases:

- The device interrupt queue (`vax.iqueue`, `interrupt()`, IPL-based interrupt
  admission, the periodic clock/`ICCS` handling in `execute_vax`'s "quantum" block) —
  Phase 09 I/O. Nothing in decode or fault-handling requires it; `handle_fault` only
  *consumes* an already-chosen IPL, it doesn't decide interrupt admission.
- `memory_io.c` (`load_io`/`store_io`, memory-mapped device register stubs) — Phase 09
  I/O; confirmed by reading it, it's device-specific stub code, not a CPU-loop
  dependency (`storage.c`'s I/O-space dispatch itself is Phase 09 territory, since the
  physical address ranges involved are device-specific).
- Breakpoints, single-step (`STEP_*`), disassembly output, and register-change
  tracking in `execute_vax` — Phase 08 console and Phase 11 disassembler.
- `format_exception`'s "no SCB handler installed" console fallback — Phase 08 console.

`set_mode_stack`'s `vax.MAPEN = 1` on a non-interrupt-stack mode switch is flagged by
the C source's own comment as uncertain (`/* Not sure about this!! */`). Per this
project's bug-fixing policy this is a suspected-fidelity question, not a clear-cut
error, so it's replicated as-is and logged to `docs/DEVIATIONS.md` when fault handling
lands (sub-phase 4) rather than second-guessed.

### New composing type: `cpu.Engine`

Fault handling and the fetch-decode-execute loop both need `*vax.CPU` and `*vm.Memory`
together, plus decode/execute-only state the C source keeps on the global `vax`
struct but Phase 01 deliberately deferred: `instruction_PC` (PC at the start of the
current instruction, used by fault reporting) and `halted`. These don't belong on
`vax.CPU` (registers/PSL only, per Phase 01's scope) or on `vm.Memory` (owns RAM, not
CPU-loop state), so Phase 03 introduces `cpu.Engine` to hold them alongside the
instruction dispatch table. This is a different type from the `vax.Machine` Phase 01's
naming note left room for — that name stays reserved for a possible later top-level
type composing CPU+memory+console+I/O (Phase 08+); `cpu.Engine` is scoped to exactly
what the decode/execute loop needs now.

Instruction handlers (Phases 04-07) have the signature
`type Handler func(e *Engine, d *Decoded) error`, returning `nil`, a `*Fault`, or the
`ErrHalted` sentinel (the Go equivalent of the C source's `VAX_HALT`/`vax.halted = 1`,
returned by the future HALT handler in Phase 04 rather than reaching into `Engine`
directly).

### Main loop: minimal core only

`execute_vax()` in `vax.c` is ~450 lines, but the large majority of it is console
concerns already ruled out above (breakpoints, STEP modes, disassembly output,
register-change tracking) or I/O concerns (the interrupt queue, the periodic clock).
Stripped to what's actually Phase 03's — fetch, decode, dispatch, fault handling, loop
— the core reduces to: decode one instruction, look up its handler in the dispatch
table, call it, and on a `*Fault` result call `Engine.HandleFault` and continue;
`ErrHalted` stops the loop. This becomes `Engine.Step()` (one instruction) and
`Engine.Run()` (loop until halted or a non-fault error), with Phase 08's console
expected to layer STEP/breakpoint semantics on top of `Step` rather than Phase 03
reimplementing them.

## Sub-phases

Each is one buildable, testable commit, following Phase 01/02's pattern.

1. **Core decode types & generated instruction table** — `AccessKind`/`OperandKind`
   constants, the `Instruction` table-entry type, `Handler`/dispatch `Table` type
   (`[256]*Instruction` + extended map), and `internal/cpu/gen`'s generator producing
   `instructions_table.go` from `instruction_table.h`. Unit tests: entry count matches
   the reference file, spot-checks of several opcodes across the single-byte and
   extended ranges (HALT, MOVL, INDEX, BUGL/BUGW), table lookup by (extended, opcode).

2. **Operand decode** — port of `decode_operand.c`: register mode fast path, short
   literals (int and float, per the design note above), all indexed/displacement/
   deferred/autoincrement/autodecrement modes, PC-relative modes (immediate, absolute,
   byte/word/long relative and deferred), indexed-mode recursion, and the
   reserved-addressing-mode fault for illegal write-to-literal. Unit tests: one
   representative opcode/operand encoding per addressing mode, indexed-mode
   composition, the illegal-write-to-literal fault case, register-mode PC/SP/FP/AP
   aliasing.

3. **Opcode fetch & full decode** — port of `decode_opcode.c`: single-byte vs.
   extended-opcode fetch, the operand-loop drive using sub-phase 2, and the
   unimplemented-opcode fault. Unit tests: decode across varying operand counts and
   opcode lengths, an extended opcode, a bad/reserved opcode faulting correctly.

4. **Fault/exception machinery** — `cpu.Fault`, `Engine.SetFault`/`Engine.HandleFault`/
   `set_mode_stack` equivalents (SCB vector fetch, stack push, access-mode switch), and
   mapping `vm.TranslationFault` into the right `EXC_ACCVIO`/`EXC_TNV` fault. Unit
   tests: vector fetch and PC/PSL/signal-arg stack push round-trip, mode switching
   across all four `CHMx` codes, a translation fault propagating as the right
   exception. `docs/DEVIATIONS.md` entry for `set_mode_stack`'s uncertain `MAPEN`
   write.

5. **Operand value access** — `Operand.Load`/`Operand.Store` (the `get_operand`/
   `put_operand` equivalents), built directly on `vm.Memory`'s typed accessors per the
   value-based design above (no scratch-register mechanism to port). Unit tests:
   load/store round-trip for each `OperandKind` at each size, illegal store to an
   immediate operand.

6. **Fetch-decode-execute loop** — `Engine.Step`/`Engine.Run`, the minimal core
   described above. Unit tests: `Step` dispatches to the correct (stub/unimplemented)
   handler, an unimplemented opcode faults `EXC_PRIV` the way `emul_unimplemented`
   does, a fault during `Step` is handled and execution continues, `ErrHalted` stops
   `Run`.

7. **Close-out** — gap review against this doc's Goal/Deliverables, any addressing-mode
   corner cases cross-checked against the VAX ISA manual, final `docs/DEVIATIONS.md`
   pass, progress log, full-repo `go build`/`go vet`/`go test` clean.

## Open questions / notes

- ~~Decide the instruction-dispatch mechanism~~ — resolved: function dispatch table,
  per user direction; see Design notes above.

## Progress Log

### 2026-09-14 — Sub-phase 1: core decode types & generated instruction table

- Added `internal/cpu/access.go`: `AccessKind` (`OP_NL`/`OP_RD`/`OP_WR`/`OP_MD`/
  `OP_AD`/`OP_VA`/`OP_BR`/`OP_IM`) and `ShortLiteralType` (`OP_TYPE_INT`/
  `OP_TYPE_FLOAT`).
- Added `internal/cpu/instruction.go`: `Opcode` (extended+function byte pair),
  `Instruction` (the static-data subset of `struct INSTRUCTION` — name, opcode,
  operand count/scale/access, short-literal type; `routine`/`use_count`/`debugdata`
  are deliberately not part of this type, see Design notes), and `Table`
  (`[256]*Instruction` for single-byte opcodes + a `map[uint16]*Instruction` for
  extended opcodes, replacing the C source's linear scan of the extended range).
- Added `internal/cpu/gen/main.go`: a `go generate`-driven parser for
  `instruction_table.h`'s fixed 9-field entry format, emitting
  `internal/cpu/instructions_table.go` (284 entries, matching the reference file
  exactly — a structural regex over the whole file, not a line-oriented parser, since
  the C header itself says it's machine-generated in this exact shape). Confirmed the
  single-byte opcode range (0x00-0xFF) has no gaps, including the `EXT_FD`/`EXT_FE`/
  `EXT_FF` filler entries at 0xFD-0xFF that exist only so the C source's array-
  position-equals-opcode-value convention holds (those three opcode values are always
  intercepted as extended-opcode prefixes before a single-byte lookup happens; ported
  in sub-phase 3).
- Added `internal/cpu/instruction_test.go`: entry count against the reference file,
  single-byte range completeness, `Lookup` for representative single-byte/extended/
  6-operand (`INDEX`) opcodes, `Lookup` returning nil for a defined prefix with an
  undefined function byte.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and `go test ./...` all
  clean.

### 2026-09-14 — Sub-phase 2: operand decode

- Added `internal/cpu/exception.go`: `Exception` (`EXC_*` codes) and a minimal `Fault`
  (`Code` + `Args`) — just enough for decode-time faults to have somewhere to go;
  fleshed out with PC/PSL/R0/R1 and `Engine.HandleFault` in sub-phase 4, same
  incremental-growth pattern as Phase 01's `CPU`/`PSL`.
- Added `internal/cpu/shortdouble.go`: `decode_operand.c`'s `short_double[64]` table,
  verbatim.
- Added `internal/cpu/operand.go`: `Operand`/`OperandKind` (the value-based design from
  this doc's Design notes) and `decodeOperand`, the port of `decode_operand.c` — the
  `OP_BR`/`OP_IM` access fast path, the register-mode fast path, short literals
  (int and float, per the design note on deferring F/D-floating bit-pattern resolution
  to Phase 05), all PC-relative modes (Immediate, Absolute, Byte/Word/Long Relative
  direct and deferred), and all general-register modes (Indexed, Register deferred,
  Autodecrement, Autoincrement [deferred], Byte/Word/Long displacement direct and
  deferred).
- Found and fixed two fidelity issues while porting (both logged to
  `docs/DEVIATIONS.md` with full rationale, since the fixes are clear-cut and fit
  naturally in this change's scope per the bug-fixing policy — not silently patched):
  Autoincrement Deferred (`@(Rn)+`) eagerly loading the operand's *value* during
  decode instead of resolving its address (would silently discard any write through
  this mode), and double-nested Indexed mode's rejection using a fault sentinel
  disconnected from the normal fault-signaling path. Also logged one genuinely open
  question (not a finding): whether Register mode should fault for `OP_AD`-access
  operands, deferred to whichever of Phase 04-07 implements the first such instruction.
- Added `internal/cpu/operand_test.go`: one test per addressing mode (register direct
  incl. PC/SP/FP/AP aliasing, short literal int/float, register deferred, autodecrement,
  autoincrement [deferred], byte/word/long displacement direct and deferred, indexed,
  all PC-relative forms, and the `OP_BR`/`OP_IM` direct-access cases), plus the
  illegal-write-to-literal fault, the double-indexed-mode fault, and sign-extension of
  a byte-sized PC-relative immediate.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and `go test ./...` all
  clean.

### 2026-09-14 — Sub-phase 3: opcode fetch & full decode

- Added `internal/cpu/decode.go`: `Decoded` (the value-based `struct OPCODE`
  equivalent — `Opcode`, `*Instruction`, up to 6 `Operand`s, `NextPC`) and
  `decodeInstruction`, the port of `decode_opcode.c`'s `decode_instruction` — single-
  byte vs. extended (two-byte) opcode fetch, instruction table lookup, driving
  `decodeOperand` (sub-phase 2) across all of the instruction's operands, and an
  `ExcPrivileged` fault for an undefined extended opcode (single-byte opcodes can't hit
  this, per sub-phase 1's confirmed full 0x00-0xFF table coverage, but `Table.Lookup`
  returning nil is still handled generically rather than assumed impossible).
  Deliberately does not port `vax.PC`'s several intermediate write-back points during
  decode: tracing through `execute_vax`'s fault paths shows they reset PC to the
  instruction's start address regardless of where mid-decode it got to, so the
  intermediate writes are never actually observed — `decodeInstruction` just returns
  `NextPC` and leaves PC bookkeeping to `Engine.Step` (sub-phase 6).
- Added `internal/cpu/decode_test.go`: a zero-operand instruction (HALT), a two-operand
  single-byte instruction (MOVL) with correct per-operand results and `NextPC`, an
  extended-opcode instruction (BUGL) with its longword immediate operand, an undefined
  extended opcode faulting `ExcPrivileged`, and an operand-decode fault (illegal write
  to a short literal) propagating with the partially-decoded `Instruction` still
  attached.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and `go test ./...` all
  clean.

### 2026-09-14 — Sub-phase 4: fault/exception machinery

- Added `internal/cpu/engine.go`: `Engine`, composing `*vax.CPU` + `*vm.Memory` + the
  instruction table with the decode/execute-only state Phase 01 deliberately deferred
  (`instructionPC`, `halted`) — see this doc's design notes on why it's a new type
  rather than growing `vax.CPU`.
- Added `internal/cpu/handlefault.go`: `Engine.HandleFault`, the port of
  `interrupt.c`'s `handle_fault` (SCB vector fetch with virtual memory forced off
  around it, matching `handle_fault`'s save/clear/restore of `MAPEN`; stack/access-mode
  switch; pushing the old PSL, the instruction's start PC, and up to two signal
  arguments; setting PC to the vector), and `Engine.setModeStack`, the port of
  `set_mode_stack` (per-mode stack pointer swap, `CUR_MOD`/`PRV_MOD`/`IS` PSL field
  updates — folded into the single canonical `vax.PSL` rather than C's separate
  `pslw` shadow, consistent with Phase 01). `ErrNoExceptionHandler` and
  `ErrUnhandledVector` cover the two "no real vector" cases (`0xFFFFFFFF` — this
  emulator's own "let the console handle it" convention, no-op here since Phase 08
  doesn't exist yet; and a zero vector, where — matching the C source exactly — the
  fault frame is still pushed and PC still set before the error is reported).
- Logged one more `docs/DEVIATIONS.md` entry while porting: `set_mode_stack`
  unconditionally sets `MAPEN = 1` on every non-interrupt-stack mode switch, which the
  C source's own comment flags as uncertain (`/* Not sure about this!! */`) — deferred
  rather than second-guessed, per policy, since the original author didn't resolve it
  either.
- Added `internal/cpu/handlefault_test.go`: the full push-frame round-trip (PSL/PC/
  args read back from memory), the SCB vector fetch bypassing translation (proved via
  a page table where the vector address would fault if not bypassed, while the
  fault-frame push target is backed by one valid S0 PTE so it succeeds regardless),
  `setModeStack`'s mechanics in isolation (mode/stack-pointer swap, `MAPEN` write, and
  the same-mode no-op case), the interrupt-stack path, and both no-real-vector cases.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and `go test ./...` all
  clean.

### 2026-09-14 — Sub-phase 5: operand value access

- Added `internal/cpu/operandaccess.go`: `Operand.Load`/`Operand.Store`, the port of
  `storage.c`'s `get_operand`/`put_operand` onto the value-based `Operand` design —
  each call resolves the value fresh against `*vax.CPU`/`*vm.Memory` rather than
  aliasing through a pointer, so there's no scratch-register bookkeeping to get wrong
  (see sub-phase 2's `AUDIT.md`-adjacent finding for what that bookkeeping cost the C
  source). A register operand smaller than a longword only touches its low bytes on
  both read and write, preserving the rest — matching `put_operand`'s byte-limited copy
  and `vm.Memory.LoadRegister`'s identical Phase 02 behavior. A quadword (8-byte)
  register operand reads/writes the register pair `Reg`/`Reg+1` (low/high longword) —
  made explicit here since the C source gets this "for free" from `vax.reg[]`'s
  contiguous array layout and a raw 8-byte pointer read, which this port's `Operand`
  has no pointer to alias through. `ErrImmutableOperand` is a defensive backstop for
  `Store` on an immediate operand — unreachable from real VAX code since decode already
  faults any write-access operand that resolves to a literal.
- Added `internal/cpu/operandaccess_test.go`: immediate load/store (including the
  immutability error), register round-trips at each size with the byte/word partial-
  write-preserves-upper-bytes behavior explicitly asserted, the quadword register-pair
  case, and memory round-trips at each size including a check that a byte store doesn't
  disturb neighboring memory.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and `go test ./...` all
  clean.
