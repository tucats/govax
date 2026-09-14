# Phase 06: String, bitfield & queue instructions

## Goal

Port the VAX's character-string, bitfield, and CRC instruction families — a
self-contained, well-testable group.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/emul_bitfield.c` — bitfield instructions
  (EXTV/EXTZV/CMPV/CMPZV/INSV/FFS/FFC), plus the bit-branch instructions
  (BBS/BBC/BBSS/BBCS/BBSC/BBCC/BBSSI/BBCCI) that live in the same file.
- `reference/eVAX/eVAX/Source/CPU/emul_cmpc.c`, `emul_locc.c`, `emul_matchc.c`,
  `emul_movc.c`, `emul_skpc.c` — character-string instructions (MOVC3/MOVC5,
  CMPC3/CMPC5, MOVTC/MOVTUC, SCANC/SPANC — all in `emul_movc.c` despite the
  filename — LOCC, SKPC, MATCHC).
- `reference/eVAX/eVAX/Source/CPU/emul_crc.c` — CRC instruction.
- `reference/eVAX/eVAX/Source/CPU/emul_misc.c` — *not* listed in the original scope
  for this phase, but it's where INSQUE/REMQUE/INSQHI/INSQTI/REMQHI/REMQTI (the
  "queue instructions" named in this phase's title) actually live in the C source;
  only those six handlers are pulled from this file into Phase 06, same precedent as
  HALT/NOP already having been split out of it into an earlier phase. The file's
  remaining, unrelated instructions (BUGL/BUGW, INDEX, BISPSW/BICPSW, PUSHR/POPR,
  BPT, PROBEx, MOVPSL) are Phase 07 territory ("everything else").

## Deliverables

- Instruction handlers with unit tests covering variable-length string operands,
  edge cases (zero-length strings, fill-byte behavior), and bitfield boundary
  conditions (field spanning byte/word boundaries).
- `testdata/asm/movc3.asm`, `insv.asm` in `testdata/asm/` are direct fixtures for this
  family and should be used once the assembler (Phase 11) or a hand-encoded byte
  sequence can exercise them.

## Open questions / notes

- **CRC**: `emul_crc.c`'s C implementation is a complete no-op stub (`return
  VAX_OK;`, no computation at all) — there is no existing C behavior to port. Per
  user direction (2026-09-14), the Go port implements real CRC from the VAX
  architecture manual's table-driven algorithm rather than replicating the stub.

## Progress Log

### 2026-09-14 — Sub-phase 1: bitfield field instructions

- Added `internal/cpu/bitfield.go`: EXTV/EXTZV, CMPV/CMPZV, INSV, FFS/FFC, porting
  `emul_bitfield.c`'s field-instruction handlers and their `get_register_field`/
  `set_register_field`/`get_memory_field`/`set_memory_field`/`bit_sext` helpers.
  Register-field access replaces the C source's byte-pointer arithmetic (the pointer
  trick `emul_bbstate`'s register path uses elsewhere in the same file, not these
  routines) with direct shift/mask arithmetic against the value-based `Operand`
  model; memory-field access replaces the C source's bit-by-bit loop with an
  equivalent load-span/shift/mask (at most 5 bytes for any 32-bit field at any
  sub-byte offset) — same defined semantics, no ISA behavior riding on the loop
  shape itself.
- Found and fixed a genuine off-by-one in `get_register_field`/`set_register_field`'s
  cross-register split arithmetic (uses `31` where `32`, the register width,
  belongs) — logged in `docs/DEVIATIONS.md`.
- `internal/cpu/bitfield_test.go` covers both field-storage backends directly
  (register including the exact-32-bit-fill boundary case the C source's own
  boundary check gets wrong, and memory including a byte-boundary-spanning field and
  a negative bit displacement reaching before the base address) and all six
  instructions end-to-end through `Engine.Step`, including the reserved-operand
  fault for an immediate (short-literal) base.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean.

### 2026-09-14 — Sub-phase 2: bit-branch instructions

- Added `internal/cpu/bitbranch.go`: BBS/BBC and BBSS/BBCS/BBSC/BBCC/BBSSI/BBCCI,
  porting `emul_bitfield.c`'s `emul_bb`/`emul_bbstate`. Both reuse sub-phase 1's
  `getRegisterField`/`setRegisterField`/`getMemoryField`/`setMemoryField` at size 1
  rather than porting `emul_bbstate`'s own separate byte-pointer-based bit access —
  same defined single-bit test/set/clear semantics, with no pointer/endianness
  mechanism to inherit in a value-based operand model. `BBSSI`/`BBCCI` are handled by
  normalizing to their non-interlocked equivalents before computing the shared
  test/set-value decode, matching this project not emulating multiple processors.
- `internal/cpu/bitbranch_test.go` covers BBS/BBC (register and memory base, plus the
  immediate-base reserved-operand fault) and all six BBSS-family opcodes' test/
  branch/write-back combinations, including confirming the interlocked variants
  behave identically to their plain counterparts.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean.

### 2026-09-14 — Sub-phase 3: MOVC3/MOVC5, CMPC3/CMPC5

- Added `internal/cpu/movc.go` (MOVC3/MOVC5) and `internal/cpu/cmpc.go`
  (CMPC3/CMPC5), porting `emul_movc.c`'s and `emul_cmpc.c`'s namesake handlers.
  Register-mode source/destination operands already fault at decode time
  (`AccessAddress`), superseding each handler's own `is_register[]` check.
- Found and fixed a bug in `emul_cmpc5`: its dual-string and both fill-padding
  compare loops are all nested inside one `if (tmp1 > 0 && tmp3 > 0)` gate, which
  the structurally identical `emul_movc5` doesn't have — skipping the manual's
  documented fill-padding behavior whenever either string starts at exactly zero
  length. Logged in `docs/DEVIATIONS.md`.
- Logged two related open questions (not fixed, C behavior replicated as-is):
  whether the whole string-instruction family's length operands should be treated
  as unsigned rather than the C source's signed `short`, and `emul_cmpc5`'s own
  fill-padding loops still running (against the same stale, already-mismatched
  byte) after its main loop stops due to an inequality rather than exhaustion.
- `internal/cpu/movc_test.go`/`cmpc_test.go` cover both instructions' overlap
  handling (MOVC3's forward/backward copy direction), fill-padding in both
  directions (MOVC5/CMPC5), the CMPC5 outer-gate regression specifically, and the
  zero-length edge cases the manual calls out by name.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean.
