# Phase 11: Assembler / disassembler

## Goal

Port the MACRO-32-ish assembler/disassembler shared by the console's inline ASM/DISASM
commands and used generally for operand encoding — this unlocks assembling
`testdata/asm/*.asm` fixtures directly in Go rather than depending on pre-built
`.exe`/ROM files for new test cases.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/Assembler/asm.c`, `asm_expr.c`, `asm_label.c`,
  `asm_opcode.c`, `asm_operand.c`, `asm_pseudo.c`, `asm_register.c`, `asm_symbols.c`,
  `asm_value.c` — parsing/assembly.
- `reference/eVAX/eVAX/Source/Assembler/disasm_operand.c` — disassembly/formatting for
  display.
- `reference/eVAX/eVAX/Source/Initialization/init_symbols.c` — symbol table seeding at
  startup.

## Deliverables

- `internal/asm` package with `Assemble(source string) ([]byte, error)`-shaped API (or
  similar) reusing the opcode tables from `internal/cpu` (Phase 03) rather than
  duplicating them, plus a disassembler for console DISASM support.
- Tests: assemble every fixture in `testdata/asm/` and confirm the output matches
  expected encodings (cross-check smaller ones like `hello.asm`, `movc3.asm`,
  `insv.asm` by hand against the ISA manual; use the larger ones — `kernel.asm`,
  `forth.asm`, `bench.asm` — as broad regression coverage). Round-trip test:
  disassemble output should re-assemble to the same bytes where unambiguous.

## Open questions / notes

- `testdata/asm/ssdef.asm` (26KB) looks like a VMS system-service-definitions include
  file rather than a standalone program — confirm its role (likely a `.INCLUDE`
  target for other fixtures like `kernel.asm`) before writing a test harness around it.

## Progress Log

### 2026-09-14 — Phase 11 complete

- `internal/asm` implements a full single-pass MACRO-32-ish assembler and a
  disassembler, ported from `asm.c`/`asm_expr.c`/`asm_label.c`/`asm_opcode.c`/
  `asm_operand.c`/`asm_pseudo.c`/`asm_register.c`/`asm_symbols.c`/`asm_value.c`/
  `disasm_operand.c`, plus `init_symbols.c`'s startup symbol-table seeding
  (`builtins.go`). `Assembler.Assemble(source string) ([]byte, error)` is the
  top-level entry point; mnemonic/operand-encoding metadata comes from
  `internal/cpu`'s existing instruction table (`cpu.Instructions()`, plus two
  small new exports — `Table.ByName`, `EncodeFloat`/`DecodeFloat`/`ShortFloat`/
  `FindShortFloat` for float-literal encoding) rather than a duplicate copy, per
  this phase's own scope note.
- Batch-assembly design decision: unlike the reference tool (which deposits
  directly into a live VAX's virtual memory, sharing state with the running
  machine), `Assembler` writes to its own address-keyed sparse image
  (`image.go`), independent of `internal/vm.Memory`/`internal/vax.CPU`. This
  keeps the assembler usable standalone (no VM/page-table bootstrap chicken-
  and-egg problem for assembling `kernel.asm` itself) and matches
  `docs/PHASE-13.md`'s own finding that image activation doesn't need this
  package's live-deposit behavior anyway. `Bytes()` returns the P0-region
  program from the configured origin (default `0x200`, matching
  `initialization.c`); `BytesRange`/`S0Origin`/`S0End`/`ByteAt` reach data
  deposited elsewhere (S0, an SCB entry) for microkernel-style sources.
- Deliberate scope cuts, all because no `testdata/asm/*.asm` fixture exercises
  them: the archaic single-quoted symbol-name syntax; `@register`/`@PSL`
  live-register-indirect expressions and the `DEFINED`-adjacent
  `LONG()`/`WORD()`/`BYTE()`/`ULONG()`/`UWORD()`/`UBYTE()`/`FILEEXISTS()`
  expression functions (need live memory/filesystem access this batch
  assembler has no model for); the privileged-register pseudo-ops (`.KSP
  value`, etc.) and `.MODE`/`.PTE`/`.VECTOR`/`.CONSOLE`'s page-protection
  sub-forms (need a live mode stack / real page tables); dialect-restricted
  opcode aliases (the default `ASM_DIALECT_ANY` matches every alias
  regardless of its tagged dialect anyway, so the whole table is just applied
  unconditionally). `.P1VECTOR` is recognized but a no-op — building the real
  VMS P1 system-service vector needs `internal/rtl`'s service table, an
  assembler-depends-on-RTL direction that's backwards, and confirmed by
  `docs/PHASE-13.md`'s own investigation to not be needed for image
  activation either (Phase 13 synthesizes `SHIM$` stubs directly as bytes,
  not through `kernel.asm`'s pseudo-ops).
- Three bugs found and fixed rather than replicated (assembler tooling, not
  emulated VAX ISA/hardware behavior, so `docs/DEVIATIONS.md`'s policy for
  suspected fidelity issues doesn't apply — same reasoning Phase 10's
  closeout used for its own non-ISA findings):
  - The reference tool's expression grammar has no way to write a negative
    immediate constant at all (no unary-minus handling reachable from
    `asm_expr3`'s atom position) — `testdata/asm/forth.asm` uses `#-1`
    repeatedly. Added leading unary `+`/`-` at the atom level
    (`exprAtom` in `value.go`).
  - `asm_operand.c`'s D_FLOAT immediate-literal case advances the deposit
    pointer by 4 mid-branch and then *again* by the full scale (8) in the
    shared code every branch falls through to — an 4-byte over-advance
    double-count. Not exercised by any fixture (none use an 8-byte float
    immediate), but fixed rather than ported forward (`storeImmediateFloat`
    in `operand.go`).
  - `disasm_operand.c`'s PC-relative B^/W^/L^ relative-mode disassembly
    prints the raw displacement byte/word/longword rather than the resolved
    destination address — internally inconsistent with its own assembler,
    which parses `B^address` as an absolute address and computes the
    displacement itself, so reassembling the reference tool's own
    disassembly output for these modes reproduces the *wrong* bytes unless
    the raw displacement happens to also be a valid absolute address. Fixed
    to show the resolved destination (`formatPCRelative` in `disasm.go`),
    which is both more readable and actually round-trips through this
    package's own assembler — the property `docs/PHASE-11.md`'s test plan
    calls for.
  - (Replicated, not fixed, as a closely-related aside: `asm_dec()`'s sign
    parsing works only because reading a digit sets the same flag an
    explicit `+`/`-` prefix would — without that, `"2+3"` would consume the
    `+` as part of the first number instead of stopping for the caller to
    see it as an operator. Ported faithfully, since it's the correct
    behavior (matches the original's own intent), just documented — it
    tripped up this port's own first attempt at a from-scratch
    `decimalLiteral`, which used a separate flag that a digit didn't set.)
- Console integration: `DISASSEMBLE`/`DIS` are wired to `internal/asm`'s
  disassembler (`internal/console/disasm.go`, a small `vax.CPU`+`vm.Memory`→
  `asm.ByteReader` adapter), replacing their `cmdNotImplemented` stubs.
  `ASM`/`ASSEMBLE` are **not** wired: they need `internal/asm`'s output
  routed into a live `Console`'s `vm.Memory` (translated, protected) one
  statement at a time as it's typed, plus its symbol table merged into
  `Console.Symbols` for `SHOW SYMBOL` — a live-deposit redesign of
  `internal/asm`'s own address-space-agnostic image model (see above), not a
  small wiring step, and out of scope for this phase's own deliverables
  (`exam.go`'s `Deposit` doc comment already flagged this exact gap as
  Phase 11's to fill; filling it is left as explicit follow-up work rather
  than rushed here).
- Verified against every fixture in `testdata/asm/`: all 16 small/medium
  programs (`atoi`, `bench`, `dbl`, `ff`, `float1`, `fmt`, `foo`, `hello`,
  `input`, `insv`, `logname`, `movc3`, `movq`, `rotl`, `test`, `xor`)
  assemble cleanly with a fresh `Assembler`. `forth.asm` and `kernel.asm`
  (the two large/microkernel-style fixtures named in this phase's own test
  plan) also assemble cleanly, given the configuration a real console
  session would already have in place before either is typically assembled
  (`SetMicrokernel(true)` for `forth.asm`, matching it being written to run
  *after* `vax.init`'s own `kernel.asm` build already set `.MICROKERNEL`
  session-wide; an `.INCLUDE` resolver for `kernel.asm`'s conditional
  `ssdef.asm` pull-in). `ssdef.asm` itself is confirmed to be exactly what
  this phase's own open question suspected: a `.INCLUDE` target (system
  status-code `.SET`s), not a standalone program — it has no test of its own
  beyond being exercised via `kernel.asm`'s.
  `hello.asm`/`movc3.asm`/`insv.asm` (the fixtures this phase's plan calls
  out for hand cross-checking) are asserted byte-for-byte against
  hand-derived VAX ISA encodings, alongside a dedicated table-driven test
  covering every addressing mode independent of any named fixture
  (`operand_test.go`). Round-trip disassemble→reassemble tests
  (`disasm_test.go`, `fixtures_test.go`) cover every addressing mode and
  several whole fixtures' code sections.
- One real, if narrow, port-introduced-vs-original-tool default worth
  flagging for future debugging: this port's numeric-literal default radix
  is hex, matching `initialization.c`'s actual default (`vax.console.radix =
  16`) — confirmed by tracing `asm_hex`/`asm_dec`'s call graph, not assumed;
  `#1000000` in `hello.asm` is `0x1000000`, not one million. Recorded here
  because it's easy to eyeball a fixture's numeric literals and assume
  decimal.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test
  ./...` all clean. `go test ./internal/asm/... -cover`: 76.7%.
