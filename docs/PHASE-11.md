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
  unconditionally). `.P1VECTOR` was originally recognized but a no-op — see
  this doc's 2026-09-23 progress-log entry for why, and for its later real
  implementation once RMS system services actually needed it.
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

### 2026-09-16 — `.ENTRY` symbols merged into `Console.Symbols`; disassembler recognizes entry masks

- Reported by the user: assembling `testdata/asm/hello.asm` and disassembling
  the code at `main` didn't show its register-save mask word (`.entry main,
  ^m<>`, `hello.asm:1`) as a `.ENTRY` mask the way the C reference tool does
  — and `main` had no visible "entry" attribute in `SHOW SYMBOLS` either.
  Root cause was two separate gaps, both now fixed:
  1. `Assembler.Symbols()` (`internal/asm/symbol.go`) returned a bare
     `map[string]uint32`, discarding each symbol's `SymFlag` bits — so the
     `SymEntry` flag `pseudoEntry` (`pseudo.go`) correctly sets was lost at
     this API boundary before `Console.Assemble` (`internal/console/asm.go`)
     ever saw it. `Symbols()` now returns `map[string]SymbolInfo` (`{Value
     uint32; Entry bool}`); `Console.Assemble`'s merge loop calls the
     console's new `SymbolTable.SetEntry` instead of `Set` when `Entry` is
     true.
  2. `internal/console`'s `SymbolKind` (`symbols.go`) only ever distinguished
     user/system — the general "attribute set" gap `docs/PHASE-16.md`
     sub-phase 1c already flagged for `SHOW SYMBOL`. Added a standalone
     `Symbol.IsEntry bool` (independent of `Kind`, since an entry point can
     be either a user or a system symbol) plus `SymbolTable.EntryAt(addr)`,
     a linear reverse lookup matching `decode_opcode.c`'s own `SYM_ENTRY`-by-
     PC scan. `ShowSymbols`/`ShowSymbol` (`show.go`) now append an ", entry"
     tag to the existing kind label when set — this resolves the "entry"
     part of sub-phase 1c's deferred attribute list; "perm"/"label"/"local"/
     "string" remain unimplemented, as that document still notes.
  3. `internal/asm`'s `Disassemble` (`disasm.go`) never consulted a symbol
     table at all — a narrower gap than the "symbolic operand formatting is
     a display nicety, left to the caller" scope cut its own doc comment
     describes: misdecoding a data word as an opcode is a correctness bug,
     not a display nicety. Rather than widen `Disassemble`'s own signature
     (it deliberately stays symbol-table-agnostic, callable on a bare byte
     stream), the entry-mask check lives in a new `Console.decodeInstruction`
     (`internal/console/disasm.go`): if `Symbols.EntryAt(pc)` finds a match,
     the word there is decoded as a mask (`asm.FormatMask`, new — ports
     `console_disasm.c`'s `format_mask()` byte-for-byte, including its literal
     "R12"/"R13" for bits 12/13 that this package's own `maskLiteral` parser
     never actually emits) into a synthetic `.ENTRY name,mask` `Decoded`
     instead of calling `asm.Disassemble`. Both `Console.Disassemble`
     (DISASSEMBLE/DIS) and `traceStep` (STEP/TRACE) now go through
     `decodeInstruction`, matching the C reference's `decode_opcode.c`, where
     the same combined execute/disassemble entry point does this check for
     both console commands.
- New tests: `internal/asm/symbol_test.go`'s `TestSymbolsEntryFlag`;
  `internal/console/disasm_test.go`'s `TestDisassemble_entryMask`;
  `internal/console/asm_test.go`'s `TestAssemble_helloEntrySymbolAndMask`
  (end-to-end against the actual fixture the user reported this against).
- `go build ./...`, `go vet ./...`, `golangci-lint run` (only pre-existing,
  unrelated findings elsewhere in the tree), `go test ./...` all clean.

### 2026-09-23 — `.P1VECTOR` implemented for real

- Requested by the user: `testdata/asm/rms_roundtrip.asm` (docs/PHASE-22.md
  subtask 14) had been hand-defining its six `SYS$xxx` symbols via `.SET
  /PERM` and relying on `internal/console/rms_e2e_test.go` to deposit their
  CALLS trampolines by hand, exactly the work `.P1VECTOR` (a no-op since
  this phase's original pass) is supposed to do — now that RMS system
  services actually exist to call through it, worth finishing rather than
  deferring further.
- Root design question first: `.P1VECTOR` needs the same fixed VMS P1
  address table (~250 `SYS$xxx` entries) `internal/rtl/p1vector.go` already
  has for dispatch — but `internal/asm` deliberately doesn't depend on
  `internal/rtl` (this doc's own scope-cut note above explained the original
  no-op that way). Asked the user rather than deciding unilaterally; picked
  option (a) of three (duplicate the table in `internal/asm`; (b) export it
  from `internal/rtl` and import that; (c) a new shared leaf package): a new
  `internal/p1vector` package holding just the data (`Entry`/`Table`, copied
  verbatim from `p1_vector.c`'s own array), imported by both
  `internal/rtl` (`p1vector.go` now just builds its PC-match index over it)
  and `internal/asm` — single source of truth, no new backwards dependency
  either direction.
- `internal/asm/pseudo.go`'s `pseudoP1Vector` now ports `p1_vector.c`'s own
  `p1_init()` line-for-line (matching `asm_pseudo.c` case 40's `return
  p1_init();` — the reference tool calls straight into it from the pseudo-op
  itself, not from anywhere downstream of assembly, so despite this doc's
  original note there was never a real "backwards for an assembler package"
  problem with the *behavior*, just with where the *data* lived): for every
  `internal/p1vector.Table` entry, defines the `SYS$xxx` symbol (permanent;
  `SymEntry` for an ordinary CALL target, `SymLabel` for the one JMP-reached
  entry, `SYS$SRCHANDLER`) and deposits a CALLS-compatible trampoline (zero
  procedure-entry mask, `XFC #XFC$P1VECTOR`, `RET`) at its address, then
  defines `EXE$P1_VECTOR_BASE`/`END` from the real min/max addresses seen.
  Requires `.MICROKERNEL`, matching `.SCB`/`.SHIM`/`.REGION`. Not ported:
  `p1_init()`'s own `declare_services()` call (this port already registers
  every implemented `SYS$` handler statically at `internal/rtl` package
  init, independent of assembly) and its closing `setpte_multiple(...
  PROT=PTE$K_UR)` (no PTE-protection pseudo-op/enforcement this fine-grained
  exists here, and every caller through this trampoline already works
  without it).
- Since `.P1VECTOR` writes its trampolines directly at fixed P1 addresses
  via the assembler's own sparse image buffer (like every other pseudo-op —
  nothing here touches live VM memory directly, unlike the reference tool's
  own `store_memory`), `internal/console/asm.go`'s `depositAsmImage` needed
  a new case to actually copy that range into live memory, the same role
  its existing SCB-page special case already plays: a new
  `Assembler.P1VectorRange()` accessor (backed by new `p1VectorBase`/
  `p1VectorEnd`/`p1VectorSet` fields `pseudoP1Vector` populates) reports the
  real `[min, max+5)` span so `depositAsmImage` knows what to
  `BytesRange`/`storeBytes`.
- **Found, and documented rather than fixed** (`docs/DEVIATIONS.md`'s policy
  for a finding rooted in the C source's own data/logic, not a Go porting
  mistake): three P1-vector table entries (`SYS$CLRAST_2`/`SYS$GL_ASTRET`,
  both at the same address, and `SYS$GL_COMMON` right after them) sit close
  enough together that `p1_init()`'s own unconditional, no-overlap-check
  writes clobber each earlier entry's trailing `RET`/`XFC` byte with the
  next entry's leading mask/opcode byte — confirmed present in the C source
  itself (the same three addresses collide there, in the same order), not
  introduced by this port. See `docs/DEVIATIONS.md`'s new "Phase 11
  (assembler) findings" section for the full detail; `internal/asm/
  p1vector_test.go`'s `TestPseudoP1VectorDefinesSymbolsAndTrampolines`
  asserts the exact clobbered bytes rather than assuming every entry's
  trampoline is untouched, so a future accidental fix of this ordering
  quirk would be caught, not silently masked.
- `testdata/asm/rms_roundtrip.asm` updated per the user's own request: the
  six `.SET /PERM sys$xxx ...` lines replaced with a plain `.MICROKERNEL` /
  `.P1VECTOR` pair, and `internal/console/rms_e2e_test.go`'s
  `depositP1VectorTrampolines` helper (and its six duplicated address
  constants) deleted outright — `.P1VECTOR` now does that work for real.
  `TestRMSRoundTrip_assembledProgram` still passes unchanged otherwise.
- Fallout: several `internal/console` tests assemble `kernel.asm` (which has
  its own bare `.p1vector` statement, previously a no-op) against a
  deliberately small `newRunnableConsole` (100 P1 pages) sized for image-
  load/fixup tests that were never expected to touch real P1 addresses. Now
  that `.p1vector` genuinely deposits at `internal/p1vector.Table`'s real
  addresses (lowest one 145 pages below the top of P1 space), 100 pages
  wasn't enough and assembling `kernel.asm` access-violated. Fixed by
  raising `newRunnableConsole` (and two test-local inlined copies of its
  same `VMInit` call, in `dispatch_test.go`/`show_test.go`) to 200 P1 pages —
  comfortable headroom over the 145-page floor, not a tight fit.
- **Bug found and fixed same-day, flagged by the user**: the initial
  `pseudoP1Vector` passed `unique=true` to each `SYS$xxx` symbol's
  `setSymbol` call, modeled on `.SHIM`'s own uniqueness-checked pattern —
  wrong model. A real `govax` boot already runs `.p1vector` once via
  `vax.init`'s own "asm kernel.asm"; requiring a fixture like
  `rms_roundtrip.asm` to carry its own `.p1vector` line is purely an
  artifact of its test running outside full console boot (no kernel.asm
  assembled first to define those symbols for free) — so in a real,
  already-booted console session, ASMing a second file with its own
  `.p1vector` line must not blow up re-defining symbols kernel.asm's own
  boot-time `.p1vector` already set. Traced back to
  `reference/eVAX/eVAX/Source/Assembler/asm_symbols.c`'s `set_symbol`: the
  duplicate-symbol rejection only fires when the caller has separately
  raised the global `ASM_UNIQUE` flag ahead of the call, and
  `p1_init()`'s own `set_symbol_direct` call sites never do — so the real
  reference tool's `.P1VECTOR` was always naturally idempotent; this port's
  `unique=true` was the deviation, not the reference's behavior. Fixed by
  passing `unique=false` instead (no separate "already ran" guard needed —
  re-running the whole loop just redefines every symbol/byte to the same
  values). New tests: `TestPseudoP1VectorIsIdempotent`
  (`internal/asm/p1vector_test.go`) and
  `TestRMSRoundTrip_afterKernelAlreadyP1VectoredIsIdempotent`
  (`internal/console/rms_e2e_test.go`, the real ASM-kernel.asm-then-ASM-
  rms_roundtrip.asm shape).
- New tests: `internal/asm/p1vector_test.go`
  (`TestPseudoP1VectorRequiresMicrokernel`,
  `TestPseudoP1VectorDefinesSymbolsAndTrampolines`). `go build ./...`,
  `go vet ./...`, `gofmt -l .` (no new findings), `go test ./...` all clean
  across the whole module (including the peer `ods2` module, reachable via
  `go.work`).
