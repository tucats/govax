# eVAX Source Audit — 32-bit → 64-bit Portability

**Scope:** `eVAX/Source/**`, `eVAX/Headers/**`, `eVAX.xcodeproj`. Audited by reading the
code, cross-checking against the real VAX/VMS architecture the emulator targets, and by
actually building the project and instrumenting `sizeof()` of the key types to confirm
suspicions rather than guessing. Several findings were verified concretely (compiling
test snippets, hex-dumping the repo's own fixture files) rather than left as theory.

**Method note:** this document is the *combined* output of four passes over the tree
(RTL; CPU; Console; Assembler/Initialization/Headers) done in this session. Findings are
organized by subsystem, most-severe first within each. Severity scale used throughout:
- **CRITICAL** — silently corrupts emulated program behavior or crashes broadly, is
  outright undefined behavior (OOB memory access), or is demonstrated broken *right now*
  against a real fixture in the repo.
- **HIGH** — wrong behavior under common/likely conditions.
- **MEDIUM** — real bug, but narrow trigger conditions.
- **LOW** — latent hazard, cosmetic, or fix-time note (not wrong today, but will become
  wrong the moment the root cause is fixed unless handled in the same change).

This file is a menu for follow-up tasks, not a patch — nothing has been changed in the
source tree. Several files in this codebase use classic Mac OS CR-only line endings (no
LF); line numbers below for those files were obtained by normalizing a private copy
before counting, so they may drift slightly from what a raw `grep -n` on the checked-out
file reports. See §4.

---

## 0. Root cause

`eVAX/Headers/arch.h` defines the emulator's core integer types like this:

```c
#ifdef HAS64BITLONGS
typedef long QUADWORD;
typedef int LONGWORD;
typedef unsigned int ULONGWORD;
#else
#ifdef WIN
typedef __int64 QUADWORD;
#else
typedef long long QUADWORD;
#endif
typedef long LONGWORD;
typedef unsigned long ULONGWORD;
#endif
```

`LONGWORD`/`ULONGWORD` are meant to be **the** 32-bit VAX longword type used everywhere
in the emulator (registers, addresses, opcode fields, RTL argument slots — essentially
every piece of emulated machine state). The `HAS64BITLONGS` branch exists specifically
because the original author already hit this problem once, porting to Alpha/TRU64 in
1999 (see the file's own history comment: *"there's no benefit in being a truly 64-bit
system, emulating a VAX ... I'm converting all long's to an explicit definition of a
32-bit integer"*). `HAS64BITLONGS` is only ever defined under `#ifdef TRU64` — nothing
defines it for a modern 64-bit Mac/Linux/x86_64/arm64 build.

The current Xcode project (`eVAX.xcodeproj/project.pbxproj`) builds with:

```
"GCC_PREPROCESSOR_DEFINITIONS[arch=*]" = LINUX86;
```

`arch.h`'s `LINUX86` branch does **not** define `HAS64BITLONGS`, so the build falls into
the `#else` branch above: `LONGWORD` = `long`, `ULONGWORD` = `unsigned long`, both **8
bytes** on 64-bit macOS. Confirmed by instrumenting the actual build:

```
sizeof(LONGWORD)          = 8   (should be 4)
sizeof(ULONGWORD)         = 8   (should be 4)
sizeof(QUADWORD)          = 8   (correct — meant to be 64-bit)
sizeof(struct VAX)        = 3104
sizeof(struct OPCODE)     = 432
sizeof(union MASKREG)     = 8   (its `.bits` member is a 32-bit bitfield struct)
sizeof(union PTE)         = 8   (Headers/pte.h — same pattern, see C8/V7)
sizeof(struct FAB)        = 184 (real VMS FAB is 80 bytes — FAB_K_BLN)
sizeof(struct RAB)        = 136 (real VMS RAB is 68 bytes — RAB_K_BLN)
```

This one typedef is the source of nearly every finding below. It also means the whole
project **compiles and links cleanly** (verified: all 89 `.c` files individually compile
with 0 errors under `clang -DLINUX86 -I Headers -std=gnu99`, and a full link of the whole
tree into one executable succeeds) — there is no compiler diagnostic that will point at
this class of bug. It only shows up as wrong emulated results, and in several places,
real memory corruption.

**Fix priority:** the highest-leverage single fix is giving `LONGWORD`/`ULONGWORD` a
true 32-bit definition on this build (e.g. `int32_t`/`uint32_t` from `<stdint.h>`, gated
on pointer width rather than on a hand-maintained platform list). That alone fixes most
of the CPU-side findings below "for free" — the sign-extension, condition-code,
quadword-register-pair, and ROTL tricks were all written assuming a real 4-byte type and
become correct again once the type is 4 bytes.

**It will not, by itself, fix two other classes of problem**, which need deliberate,
separate handling in the *same* change:
1. **Genuine native pointers stored in VAX-address-sized struct fields** (Finding R1) —
   a real 64-bit host pointer cannot be made to fit in 4 bytes by narrowing a typedef.
2. **Every pointer↔`LONGWORD` cast in the tree must be re-audited before narrowing the
   type**, because some of them are only "correct" today *because* `LONGWORD` happens to
   equal native pointer width. `Headers/memmap.h`'s `STROFF` macro (Finding V6) is a
   confirmed instance: it casts a pointer to `LONGWORD` and subtracts, which works now by
   coincidence and will silently truncate/corrupt every structure-mapping offset the
   moment `LONGWORD` becomes a true 32-bit type. Search for `(LONGWORD)` / `(ULONGWORD)`
   casts applied to pointers project-wide as part of the step-1 fix, not after it.

---

## 1. CPU subsystem (`Source/CPU/`) — the instruction engine

### C1. Sign-extension is lost for every longword operand → arithmetic and condition codes wrong for any negative value — **CRITICAL**
`Source/CPU/emul_integer_math.c` (ADD/SUB/MUL/DIV/BIS/BIC, longword case, ~line 188-230),
`Source/CPU/emul_cmp.c` (CMPL/BITL/TSTL, ~line 121-125), `Source/CPU/emul_increment.c`
(INCL/DECL, ~line 98-115) all do:

```c
d1 = *(( LONGWORD * ) src1 );   /* src1 points at a 4-byte VAX longword's home */
```

On the original 32-bit build this was correct: `LONGWORD` was a real 4-byte `long`, so
loading 4 raw bytes *as* a 4-byte signed type directly reinterprets a negative VAX
longword (high bit set) as a negative host value — no separate sign-extension step was
ever needed. On this build, `LONGWORD` is 8 bytes, and the *source* of that 4 bytes is
either `vax.reg[]` (an 8-byte slot that only ever gets its low 4 bytes written by
`put_operand`, or gets explicitly zeroed-then-filled by `load_register` for a
memory-sourced temp — see `Source/CPU/storage.c:364-366`) — either way, the upper 32
bits are **zero**, not sign-extended. A VAX longword like `0xFFFFFFFE` (-2) is loaded as
the *positive* 64-bit value `0x00000000FFFFFFFE`.

Every subsequent computation and flag test operates on this wrong, always-non-negative
value: `SETCONDITIONBITS` (`Headers/vax.h:75-78`) sets N via `(LONGWORD)(v1) < 0`, which
is now never true for what should be a negative 32-bit result; the `OVERFLOW()` macro
(`Headers/vax.h:116-118`), which detects 32-bit signed overflow by comparing the sign of
a 64-bit intermediate, is likewise broken for the same reason; `emul_cmp.c`'s CMPL does
`data = d1 - d2` and compares — comparisons between a "negative" and "positive" VAX
longword come out backwards. This breaks **any VAX program doing signed longword
arithmetic or comparison with a negative operand** — one of the most common things a
compiled VAX program does. It is silent: no crash, just wrong N/V/C flags and wrong
branch decisions downstream.

### C2. Quadword operand reconstruction is broken — MOVQ (and any other quadword-operand instruction) silently zeroes the high longword — **CRITICAL**
`Source/CPU/storage.c` `get_operand()` (~line 235-273) reconstructs an 8-byte VAX
quadword operand from two 4-byte halves by loading them into **two adjacent slots of
`vax.reg[]`** and returning a pointer to the pair, relying on the two slots being
contiguous 4-byte `long`s in memory (a classic 32-bit-era trick — two adjacent 4-byte
registers *are* a valid 8-byte quadword when concatenated). `Source/CPU/emul_mov.c`
`emul_movq()` (line 458) then does `data = *src;` with `src` cast to `QUADWORD*` (a
real 8-byte type) to read the reconstructed value.

Because `vax.reg[]` elements are now 8 bytes each (not 4), the two "adjacent" registers
are no longer contiguous at 4-byte granularity — `vax.reg[T-1]` alone is already 8
bytes wide. `*(QUADWORD*)&vax.reg[T-1]` reads entirely within that single register slot
(whose upper 4 bytes were zeroed by `load_register`) and never touches `vax.reg[T]` at
all. The result: **the high longword of every quadword value loaded from VAX memory is
silently replaced with zero.** The same problem exists for a register-mode quadword
*source* operand (`Source/CPU/decode_operand.c:222-245`, `mode==5` register-mode
addressing does not special-case `src_scale==8` to combine `Rn`/`Rn+1` — it just returns
`&vax.reg[reg]` for one register, so `MOVQ Rn,...` reads only `Rn` and never touches
`Rn+1`, whereas real VAX semantics use the `Rn:Rn+1` pair as the source). MOVQ,
CVT-to/from-quadword, and anything else reading a quadword operand is affected.

### C3. `fpu_store()`/`fpu_load()` perform out-of-bounds stack reads *and writes* on every call, and compute the wrong result even where they stay in-bounds — **CRITICAL**
`Source/CPU/fpu.c` converts between a native `double` and VAX F/D-floating longword
pairs by aliasing the `double`'s 8 raw bytes as an array of two 32-bit words (`LSB`/`MSB`
macros, lines 36-41) — a standard technique for splitting an IEEE double into its two
32-bit halves for exponent/mantissa manipulation, and it requires the word type to
actually be 4 bytes:

```c
ULONGWORD * pd, *pv;
...
pd = ( ULONGWORD * ) &source;     /* source is a genuine 8-byte native double */
...
expon = pd[ LSB /* 0 */ ] & 0x7FF00000;      /* fpu_store, line 99 */
pv[ MSB ] |= (pd[ MSB /* 1 */ ] & 0xE0000000) >> 29;   /* line 141 */
```

With `ULONGWORD` now 8 bytes, `pd` has 8-byte elements over an 8-byte `double`:
- `pd[0]` reads the **entire** double's 64-bit bit pattern in one shot — not a 32-bit
  half — so the exponent-extraction mask (`0x7FF00000`, meant to isolate bits 20-30 of
  the *high* word of the IEEE layout) is being applied to the wrong bit positions
  entirely.
- `pd[1]` (used for both `LSB`/`MSB` depending on the byte-order branch — on this
  little-endian `LINUX86` build, `LSB==1`) reads 8 bytes starting **8 bytes past the end
  of the `double source` parameter** — a genuine out-of-bounds read of whatever happens
  to sit next on the stack (undefined behavior; not merely "wrong value").

`fpu_load()` (line 205) has the mirror-image bug, and it is worse: it *writes* through
the same kind of alias —

```c
hvax2[ LSB ] = ( hvax[ MSB ] & 0x80000000) | ... ;   /* line 279 */
```

`hvax2 = (ULONGWORD*)&local2` where `local2` is a native `double` local; with `LSB==1`,
this **writes 8 bytes past the end of `local2` on the stack** on every single call —
an out-of-bounds stack *write*, i.e. real stack corruption, not just a wrong-answer bug.
Depending on stack layout and optimization this can silently corrupt an adjacent local
variable, corrupt saved registers, or crash. Because the two locals (`local`, `local2`)
happen to be zero-initialized right before this and are often adjacent on the stack, the
practical symptom is usually "every VAX floating-point load/store instruction computes
0.0 or faults with a bogus reserved-operand exception," but this cannot be relied upon —
it is unspecified compiler-dependent behavior.

This affects **every VAX F_floating/D_floating instruction** (and by extension every
compiled VAX program that touches floating point, including things that only
incidentally use floats, e.g. FORTRAN/BASIC/Pascal runtime helpers), and separately, it
is a genuine memory-safety bug independent of correctness.

### C4. Corrupted 64-bit values from RTL system-call returns get written wholesale into VAX registers — **CRITICAL** (cross-cutting with §2)
Every `SSDEF`-style system-service handler in `Source/RTL/*.c` returns its status via a
plain C assignment, e.g. `Source/RTL/p1_vector.c:440` `vax.R0 = rc;` and
`Source/RTL/shim.c:259` `vax.R0 = rc;`. `rc` is `LONGWORD` (8 bytes); as established in
§2 below, RTL argument marshaling routinely leaves the upper 32 bits of such variables
as **uninitialized garbage**. This assignment is a full 8-byte struct-field write (not
routed through `put_operand`'s byte-limited copy the way normal VAX instructions write
registers), so it plants that garbage directly into all 8 bytes of `vax.R0`. Any later
VAX instruction that reads R0 as an operand (`GET_OPERAND`/`get_operand` return the full
8-byte register slot, see Finding C1's discussion) picks up the garbage high bits and
treats the register as an enormous 64-bit value instead of a 32-bit one — wrong
comparisons, wrong branches, wrong displayed values, all downstream of a system call
that returned an ordinary small status code.

### C5. `union MASKREG` — checked, *not* a bug
`Headers/vax.h`'s `union MASKREG` has an 8-byte `longword` member unioned with a 32-bit
`bits` bitfield struct — a mismatch that looked suspicious going in. However,
`Source/CPU/emul_call.c` (CALLS/CALLG procedure entry, lines ~127-142) always transfers
exactly 4 bytes explicitly (`store_memory(vax.SP, &(mask_union.longword), 4)` /
`load_memory(vax.SP, &(mask_union.longword), 4)`), never `sizeof(union MASKREG)`. This
call-frame mask save/restore path is fine as written. Noted here so it isn't
re-flagged/re-investigated later.

### C6. Address-translation page masks in `vm.c` are incidentally safe
`Source/CPU/vm.c` masks like `addr & 0x3FFFFFFFL` and `addr & 0xFFFFFE00` are applied to
`ULONGWORD` addresses that could in principle carry garbage above bit 31 (per the
register-corruption findings above). Because these are literal bit masks rather than a
type-width-dependent operation, they still correctly strip any high garbage regardless
of `ULONGWORD`'s actual width — this code path is not a new bug introduced by the
widening. Noted so it isn't mistaken for one.

### C7. `emul_ash.c` ROTL — the original author's own 64-bit-long safety mask is compiled out on this build — **HIGH**
`Source/CPU/emul_ash.c`, `emul_rotl()` (~line 34-97), line ~93:
```c
#ifdef HAS64BITLONGS
value &= 0xFFFFFFFF;
#endif
```
This masking was added in 1999 specifically to keep ROTL correct on 64-bit-`long` hosts
(TRU64) — but per §0, `HAS64BITLONGS` is defined only for `TRU64`, never for
`LINUX86`/`MAC`, so on this build the mask is compiled out even though `LONGWORD`/
`ULONGWORD` are 8 bytes here too. A left rotate whose source has bit 31 set will carry
that bit into bit 32 and beyond across iterations; that pollutes the very next
`SETCONDITIONBITS(value, 0L)` call — Z becomes wrong whenever the true 32-bit rotated
result is 0 but upper bits aren't, and N is checked against bit 63 instead of bit 31.
Reproducible, not latent. **This is a live instance of the general "`#ifdef
HAS64BITLONGS` code is dead on this build" hazard — worth a project-wide grep for other
`HAS64BITLONGS`-gated blocks that should also be active here** (see also V7's `union
VALUE` and §0's fix guidance).

### C8. `union PTE` (Headers/pte.h) bitfield/longword size mismatch feeds a wrong access length into `vm()` — **HIGH**
`union PTE { struct PTEBITS bit; ULONGWORD longword; }` — `PTEBITS` totals exactly 32
bits (real VAX PTE format), but `ULONGWORD` is now 8 bytes, so `sizeof(union PTE)==8`
(confirmed by compiling a test). In `Source/CPU/vm.c` (~lines 437, 830, 921): `n =
sizeof(pte); rc = vm(&paddr, &n, TB_READ);` asks `vm()` to translate/validate an
**8-byte** access for what is actually always a 4-byte PTE fetch (the immediately
following manual copy loop is hardcoded to 4 bytes, so the actual data read is fine).
The wrong size only affects `vm()`'s own page-spanning/protection-check logic for the
recursive PTE lookup — real, but narrow: it only bites when a PTE's own address sits
within the last few bytes of a page. Same pattern independently confirmed in
`Source/Assembler/asm_pseudo.c`'s `union VALUE` — see V7.

### C9. Diagnostic output shows meaningless extra hex digits from uninitialized upper bits — **MEDIUM**
`Source/CPU/registers.c` prints `vax.psl.reg` with `%08lX` (lines ~85, ~292); `vm.c`
prints `pte.longword` with `%08lX` (lines ~468, ~598, ~856, ~939). Because these unions
are populated via 4-byte `load_memory`/assignment into an otherwise-uninitialized 8-byte
local, the upper 32 bits are stack garbage. Control flow is unaffected (reads go through
the bitfield view), but `SHOW PTE`/`SHOW REGISTERS`/`DEBUG(VM)` output will display
extra, meaningless high-order hex digits instead of the clean 8-digit VAX value —
confusing for anyone using the console to debug a running VAX program.

### C10. BIGENDIAN register-slot kludges are dead code today, but reproduce C1-class bugs on any future big-endian 64-bit port — **LOW**
`Source/CPU/decode_operand.c` (lines ~226-239, ~571-584) and `storage.c`
(`load_register`, lines ~392-411) do `opcode->address[n] += 2/3` or index `r[3]`/`r[2]`
assuming each register slot is exactly 4 bytes so the pointer nudge lands on the right
byte. These are `#if BIGENDIAN`-gated and dead on this little-endian build, so they're
not live bugs now, but they'd reproduce C1/C2's failure mode on any future big-endian
port (e.g. 64-bit PowerPC/ARM-BE) that doesn't separately define `HAS64BITLONGS`. Worth
fixing alongside the root cause rather than leaving for a future porter to rediscover.

### C11. ~30+ `%ld`/`%lX`/`%08lX` format specifiers across CPU files — fix-time hazard, not a live bug — **LOW**
Scattered across `vm.c`, `interrupt.c`, `registers.c`, `storage.c`, `vax.c`,
`decode_opcode.c`, `emul_call.c`, `emul_xfc.c` (and similarly in Console/Assembler files,
see V4). These are only "correct" today because `LONGWORD` happens to be `long`. The
moment `LONGWORD` is fixed to a true 32-bit type, every one of these becomes a
format/vararg-width mismatch and must be updated in the same commit — easy to miss given
the sheer count and spread. Recommend a project-wide grep for `%l` format specifiers
against `LONGWORD`/`ULONGWORD` arguments as a checklist item for the fix, rather than
fixing reactively file-by-file.

**CPU subsystem summary:** the instruction-execution core is pervasively affected. C1
alone likely breaks the majority of real-world compiled VAX programs (any signed
longword arithmetic); C2 and C3 break entire instruction classes (quadword moves,
floating point) outright, with C3 additionally being genuine undefined behavior
(out-of-bounds stack access) rather than "just" a wrong-answer bug. C4 shows the CPU and
RTL bugs actively compound each other via the register file. C7/C8 show the same
size-mismatch pattern recurring in less-obvious corners (rotate instructions, page
tables) beyond the "big" findings.

---

## 2. RTL subsystem (`Source/RTL/`) — VMS calling convention / system services

The RTL layer emulates the VMS calling convention: argument-list vectors, system-service
dispatch, string descriptors, and RMS file I/O (FAB/RAB blocks). It is dominated by one
repeating pattern: a native variable or struct field is `LONGWORD`/pointer-typed (now 8
bytes), but is only ever populated by `load_memory(addr, &var, 4)` — which writes just
the low 4 bytes, leaving the high 4 bytes as **uninitialized stack or heap garbage**
(the destination is either an uninitialized automatic variable or a `getmem()` block,
neither of which is zeroed). This is not four separate bugs; it is one systemic failure
mode that shows up at many call sites. The two worst instances cross from "wrong
emulated value" into outright memory-safety bugs.

### R1. FAB/RAB pointer-typed fields are 8-byte native pointers populated with only 4 VAX-side bytes → garbage high 32 bits on every RMS file operation — **CRITICAL**
`Headers/fab.h` (`fab_l_xab`, `fab_l_nam`, `fab_l_fna`, `fab_l_dna`) and `Headers/rab.h`
(`rab_l_ubf`, `rab_l_rbf`, `rab_l_rhb`, `rab_l_kbf`/`rab_l_pbf`, `rab_l_fab`,
`rab_l_xab`) are declared as genuine native pointers (8 bytes) standing in for what is a
4-byte VAX address in the real VMS layout. `Source/RTL/structure_mapping.c`'s `map()`
fills them via `load_memory(addr+v_offset, p, ml->size)` with `ml->size` hardcoded to 4
(set up in `Source/RTL/rms.c:53-111`, e.g. `map_int("FAB","FAB$L_FNA",
STROFF(fab,fab_l_fna), 4)` — note this offset is itself computed by the `STROFF` macro,
see V6), into `struct FAB fab;`/`struct RAB rab;` locals that are **never initialized**
(`Source/RTL/rms.c` `rms_create`/`rms_connect`/`rms_put`, lines 128-280). Confirmed:
`sizeof(struct FAB)==184` vs. the VMS-defined `FAB_K_BLN==80`; `sizeof(struct RAB)==136`
vs. `RAB_K_BLN==68`.

Concretely: `rms.c:144` `map("FAB", &fab, (LONGWORD) rab.rab_l_fab)` and `rms.c:234`
`load_string((LONGWORD) fab.fab_l_fna, fn, fab.fab_b_fns)` both dereference a "VAX
address" whose upper 32 bits are garbage stack bytes. Any nonzero garbage almost
guarantees the address fails the physical-memory bounds check, producing an access
violation, or reading unrelated memory. This hits **`SYS$CREATE`/`SYS$CONNECT`/`SYS$PUT`
— i.e. any VMS program that opens or writes a file.**

### R2. `SYS$CLI` can index a stack buffer with a garbage 64-bit length → real stack corruption, not just a wrong value — **CRITICAL**
`Source/RTL/cli.c` `sys_cli()` (lines 40-59): `len` and `ptr` are `LONGWORD` (8 bytes),
uninitialized, filled via `load_memory(req_addr+4, &len, 4)` (only the low 4 bytes).
`len` is then used directly as an array index: `b[len] = 0;` on `char b[256]` (line 69).
Because the high 32 bits of `len` are uninitialized garbage rather than guaranteed zero,
this is an out-of-bounds write at an **arbitrary** offset from `b` — a real stack-safety
bug, exercised by ordinary interactive DCL symbol lookups through the CLI callback.

### R3. Every system-service argument vector carries garbage high bits from the moment it's built — **CRITICAL, root cause for most of this section**
`Source/RTL/p1_vector.c` `call_service()` (lines 354-444) and `Source/RTL/shim.c`
`shim()` (lines 200-264) allocate `argv` as `LONGWORD*` sized `sizeof(LONGWORD)*argc`
(consistently 8 bytes/slot — the allocation size itself is fine), via plain `getmem()`
(not zeroing), then fill each slot with `load_memory(..., &(argv[n]), 4)` — 4 valid low
bytes, 4 bytes of heap garbage on top. This `argv` is handed to **every** `SSDEF`-based
service handler across `service.c`, `rms.c`, `devices.c`, `cli.c`, `logical_names.c` as
its entire argument list. Handlers that only mask a small flag out of `argv[n]` happen
to survive (e.g. `sys_clref`'s `argv[0] %= 0x00FF`); handlers that use `argv[n]`
directly as an address, length, or loop bound (the majority — `sys_expreg`,
`sys_getjpiw`, all of `rms.c`, `sys_cli`) do not. `argc` itself (loaded the same way) has
the same exposure while controlling the very loop that fills `argv`.

### R4. `str_get()`/`str_put()` — the core VMS string-descriptor accessors — resolve the descriptor's data pointer into a garbage-high-bits address — **HIGH**
`Source/RTL/librtl_utils.c` lines 201-280: `daddr` is `LONGWORD`, uninitialized, filled
via `load_memory(addr+4, &daddr, 4)`, then `daddr + n` is used to read/write through
`load_byte`/`store_memory`. These are the general-purpose descriptor accessors used
anywhere a `dsc$descriptor` is unpacked, e.g. `Source/RTL/service.c:167`
`str_get(argv[2], &size, prcname)` for `SYS$GETJPIW`'s process-name argument.

### R5. `SYS$GETDVIW`/`SYS$GETJPIW` item-list destination addresses have the same garbage-high-bits exposure — **HIGH**
`Source/RTL/devices.c:337-408` and `Source/RTL/service.c:141-246`: item-list
`buffaddr`/`retaddr` are `LONGWORD`, loaded via 4-byte `load_memory` into uninitialized
locals, then used directly as `store_memory` destination addresses. Likely symptom:
spurious ACCVIO on otherwise-correct, very commonly issued item-list queries.

### R6. Indirect-PID path in `SYS$GETJPIW` corrupts an already-valid value in place — **MEDIUM**
`Source/RTL/service.c:157-163`: `pid = (int) argv[1];` first produces a clean,
sign-extended 8-byte value; the subsequent `load_memory(pid, &pid, 4)` (indirect-PID
form) then overwrites only the low 4 bytes of that *same* variable, leaving the high 4
bytes at whatever sign-extension pattern the previous value had rather than anything
derived from the new value. Narrower trigger (only the by-reference PID form), but a
clean illustration that this failure mode also corrupts already-initialized values, not
only fresh stack garbage.

### R7. A known-but-unwired band-aid already exists in the tree — informational
`Source/CPU/storage.c:504` has `#ifdef HAS64BITLONGS \n if (count==4) *(LONGWORD*)dest =
0L; \n #endif` inside `load_memory()` — i.e. the original author already anticipated
exactly this "4-byte load into an oversized native slot" hazard and added a
zero-fill guard for it, gated on the same `HAS64BITLONGS` symbol that `arch.h` never
defines for this build. Zero-filling before a partial load would make the upper bits
deterministic (zero) rather than garbage — a much smaller, mechanical patch than fixing
every call site individually, though it papers over rather than fixes the underlying
type-width mismatch (and does nothing for R1's genuine-pointer fields, or for negative
values needing sign- rather than zero-extension per Finding C1).

**RTL subsystem summary:** nearly every VMS system call and RMS file-I/O path funnels a
4-byte VAX-memory value into an unconditionally-8-byte native slot via a 4-byte
`load_memory`, leaving the top 32 bits as uninitialized garbage. R1 (RMS pointer fields)
and R2 (CLI buffer index) cross into genuine memory-safety territory and should be
prioritized; R3 explains why point-fixing individual call sites won't fully hold unless
the root `LONGWORD` typedef (§0) is also fixed.

---

## 3. Console / Assembler / persistence / Headers

### V1. ROM and NVRAM binary file formats are already broken *right now* against the repo's own fixture files — **CRITICAL**
`Source/Console/console_load.c` `load_rom()` (lines ~703-816, esp. 725, 731, 758, 762)
and `load_nvram()`/`console_load()` (lines ~250, 254, 911, 917), and
`Source/Console/save_binary.c` `save_rom()`/`save_nvram()` (lines ~199-467, esp. 280,
287, 318, 324, 338, 437, 446), all use `fread(&x, sizeof(LONGWORD), 1, fp)` /
`fwrite(&x, 1, sizeof(LONGWORD), fp)` to read/write what is architecturally a **fixed
4-byte field** (base address, page count, size) in the on-disk ROM/NVRAM image format.

This was verified directly, not inferred: hex-dumping the repo's own checked-in fixture
`xdefault.rom` shows a header of `;ROMIMG\r` followed by a 4-byte big-endian `rom_base`
(`20 04 00 00`) and a 4-byte `rom_end`, then repeating 4-byte-address / 4-byte-count /
512-byte-data page records — a format written by the original 32-bit/BIGENDIAN build.
On this 64-bit build, `sizeof(LONGWORD)==8`, so `load_rom()` reads 8 bytes where the file
has 4, desyncing every subsequent field. **Running `LOAD/ROM xdefault.rom` today, on this
build, is reproducibly broken** — wrong base/end addresses, garbage page addresses, and
corrupted or truncated ROM contents. The same 8-vs-4 mismatch applies to any
legacy-format `.nvram` file. This is not a future-compatibility concern; it is a present,
demonstrable bug against a fixture already in the repository.

Round-tripping save→load *within this same 64-bit binary* still works (both sides now
agree on 8 bytes), which will mask the bug in a same-binary smoke test — it only
surfaces against `xdefault.rom` or any other pre-existing/foreign-build file, so a test
plan that only does save-then-load-in-the-same-run will not catch this.

Separately, `save_binary.c`/`console_load.c` also dump the entire `struct VAX` wholesale
(`fwrite(&vax, sizeof(struct VAX), 1, fp)` / `fread(&fvax, sizeof(struct VAX), 1, fp)`,
`save_binary.c:135`, `console_load.c:184`) including raw pointers. Both sides
consistently use `sizeof(...)` (never a hardcoded constant) so this round-trips correctly
*within one build*, and the loader is careful not to reuse the raw `fvax.memory` pointer
read from the file — it calls `alloc_vax()` and copies only specific fields
(`console_load.c:191-242`), so stale pointers from a saved file are not a live bug. The
problem here is purely a **cross-build compatibility hazard**: a `.VAX` save file from
this build (`sizeof(struct VAX)==3104`) cannot be read by the historical 32-bit emulator
or by a future correctly-fixed 64-bit build, and the reverse direction (a fixed future
build reading a file *this* build produced) will misalign every subsequent read. Less
urgent than the ROM/NVRAM case above (no fixture currently ships in this format), but
should be decided deliberately — version-tag the format, or key record sizes to a
literal `4` — rather than left implicit.

### V2. Assembler numeric encoding — checked, not a bug
`Source/Assembler/asm_expr.c`/`asm_opcode.c` always encode literal/computed values into
the emitted VAX instruction stream via `store_memory(addr, &var, <explicit count>)` with
explicit counts of 1 or 4 — never `sizeof(LONGWORD)`. Assembled machine code is
therefore emitted at the correct byte widths regardless of the native `LONGWORD` size.
Noted so it isn't re-investigated.

### V3. DCL result-element array walking — checked, not a bug
`Headers/dcldef.h`'s `struct DCL_RESULT_ELEMENT`/`DCL_RESULTS` and their use in
`Source/Console/dclrtl.c` consistently size allocations with `sizeof(struct
DCL_RESULT_ELEMENT)` and index with normal C array syntax (`element[result_count]`) —
never a hardcoded stride. This is a purely native, non-VAX-facing data structure, so
`LONGWORD` widening just makes it bigger, self-consistently. Not a bug. Separately,
`dclrtl.c` has several `long (*)(...)` function-pointer casts between mismatched
parameter-count/type signatures (e.g. line ~724, ~778, ~1111, ~1624, ~1641 —
`long(*)(void)` cast to `long(*)(char*)`, `long(*)(int,...)` cast to `long(*)(long,...)`)
flagged by the compiler as `-Wcast-function-type-mismatch`. These are pre-existing,
independent of the `LONGWORD` size bug, but worth a look: calling a function through an
incompatibly-typed pointer where a parameter's declared width differs (`int` vs. `long`)
is undefined behavior on platforms/ABIs where that changes how the argument is passed,
which is exactly the kind of thing that can silently break on a new architecture.

### V4. Format-string scan (Console/Assembler)
A broad `%d`-vs-`LONGWORD` scan across `Source/Console/*.c` and `Source/Assembler/*.c`
turned up nothing exploitable — the handful of `%d` uses found are against genuinely
`int`/`char`-sized values, and the codebase's habitual `%08lX`/`%ld` for
`LONGWORD`/`long` values happens to still match in argument size (since `LONGWORD` *is*
`long` here) even though the intent (`%08lX` implying a 32-bit value) is now cosmetically
misleading. No varargs-mismatch bugs found in this pass — **LOW**, cosmetic only. (The
CPU subsystem has the same pattern at larger scale — see C11 — and needs the same
fix-time pass.)

### V5. `asm_hex()`/`asm_dec()` don't truncate accumulated literals to 32 bits — **MEDIUM**
`Source/Assembler/asm_value.c`: both build up `ULONGWORD value` via `value =
(value<<4)+digit` (hex) or `value = value*10+digit` (decimal) with no masking. On the
original 32-bit-`long` host, native arithmetic overflow naturally wrapped results into
VAX-longword range; now, with a 64-bit accumulator, a literal wider than 8 hex digits (or
a decimal >2^32) parses to a different value than the original implementation would have
produced. Real-world impact is limited (VAX/MACRO-32 literals are conventionally ≤8 hex
digits, and final code bytes still get correctly truncated to 4 bytes by
`store_memory`'s explicit-count writes — this affects assembler-time constant-folding,
e.g. `.IF`/`LONG()` comparisons, not emitted machine code), but it's a genuine behavior
drift from the original implementation worth knowing about and fixing alongside the root
cause.

### V6. `Headers/memmap.h`'s `STROFF` macro: a pointer↔`LONGWORD` cast that is only correct by coincidence — **CRITICAL fix-time hazard**
```c
#define STROFF( structure, member ) \
    (( LONGWORD ) &( structure-> member )) - (( LONGWORD ) structure )
```
This computes a byte offset by casting two pointers to `LONGWORD` and subtracting. It
only "works" today because `LONGWORD` accidentally equals native pointer width (8 bytes)
on this build — the subtraction happens entirely within a 64-bit domain, so no precision
is lost. `memmap.h`'s own header comment describes this file's purpose as mapping
VAX-address-space structures onto native structures for image loading and page-table
management. **This is exactly the macro `structure_mapping.c`'s offset tables are built
with** (see R1's `map_int("FAB","FAB$L_FNA", STROFF(fab,fab_l_fna), 4)`), so it is not a
hypothetical corner — it underlies the RTL layer's entire field-mapping mechanism.

If the root-cause fix narrows `LONGWORD` to a true 32-bit type (the architecturally
correct fix, per `arch.h`'s own 1999 changelog stating that intent), this macro will
silently truncate 64-bit pointers to 32 bits before subtracting — producing wrong or
undefined offsets depending on where the structures happen to live in the 64-bit address
space. **Whoever fixes the root typedef must fix or replace this macro in the same
change** (e.g. using `offsetof()`, which sidesteps the pointer-cast entirely and is the
standard, portable way to compute this). Recommend a project-wide search for other
pointer-to-`LONGWORD`/`ULONGWORD` casts as part of the same audit — this is the one
confirmed instance found in Headers; the CPU/RTL audits did not turn up others, but were
not looking for this specific pattern exhaustively.

### V7. `union VALUE` (`Source/Assembler/asm_pseudo.c:219-223`) — same bitfield/longword union-size pattern as `MASKREG`/`PTE`, currently safe — **LOW / fix-time**
```c
union VALUE { unsigned char byte[4]; unsigned short word[2]; ULONGWORD longword; } value;
```
Used to assemble `.BYTE`/`.WORD`/`.LONG` pseudo-ops. Currently safe for the write paths
found (`store_memory` called with explicit byte counts, e.g. `asm_pseudo.c:812`), so this
is not a live bug — but it's the same latent pattern as `union MASKREG` (C5, checked OK)
and `union PTE` (C8, a real bug in its consumer). Worth including in whatever sweep fixes
those, since `sizeof()`-ing or bulk-copying this union in the future would reproduce
C8's failure mode. **At least three instances of this union pattern now confirmed
(`MASKREG`, `PTE`, `VALUE`) — a systematic pass across the codebase for `union { struct
{bitfields}; LONGWORD/ULONGWORD; }` is warranted** rather than treating each as isolated.

### V8. VMS image-header structures (`Headers/imgdef.h`) vs. loading the repo's own `.exe` test fixtures — **UNRESOLVED, needs follow-up investigation**
`struct IHD`, `IAF`, `SHR`, `IHI`, `ISD` in `Headers/imgdef.h` mix `short` (correctly 2
bytes) with `LONGWORD` (now 8, architecturally should be 4) for fields like `mask[2]`,
`flags`, `section_id`, `vbn`, `time[2]`. These model the real VAX/VMS image-activation
header format — the format of the `.exe` files this repository ships as test fixtures
(`cli.exe`, `dbl.exe`, `getvm.exe`, `put.exe`, `putc.exe`, `sieve.exe`, `simple.exe` —
confirmed via `file` to be genuine "VMS VAX executable" files; note `put1.exe` is an
empty/zero-byte file and can't be a valid fixture as-is).

`Source/Console/console_run.c` is the only consumer found via repo-wide search, but this
audit did not confirm *how* it uses these structs: if `console_run.c` overlays `struct
IHD`/etc. directly onto raw bytes read from an `.exe` file (e.g. `struct IHD *h = (struct
IHD*) buffer;`), then per V1's reasoning every field after the first `LONGWORD` is
misaligned and this is **CRITICAL** — loading any of the repo's own `.exe` fixtures would
be broken today, the same way `xdefault.rom` is broken. If instead it builds these
structs field-by-field via VAX-memory accessors (`load_memory` etc.), the severity is
much lower (same class as the RTL findings in §2, gated on how each field is populated).
**Action item: read `console_run.c`'s image-loading path directly and resolve which case
applies before triaging this further** — do not assume severity either way.

### V9. `Headers/fab.h`/`rab.h` bit-flag structs — good existing practice, worth copying
Both `struct FAB`/`RAB`'s bitfield members deliberately use plain `unsigned int` (not
`ULONGWORD`) for VMS FAB/RAB bit flags, so those specific sub-structures stay correctly
32-bit-wide regardless of the `LONGWORD` bug. This is the right pattern, and should be
the model used when fixing the union-size mismatches in C8/V7 and any others turned up by
the sweep V7 recommends.

---

## 4. Build/environment observations (not bugs, but relevant context)

- **The project currently compiles and links with zero errors** under
  `clang -DLINUX86 -I Headers -std=gnu99` (all 89 `.c` files individually, plus a full
  link of the whole tree into one executable). This confirms the bug class here is
  entirely semantic/runtime, not something a build failure will surface.
- `eVAX.xcodeproj/project.pbxproj` sets `"GCC_PREPROCESSOR_DEFINITIONS[arch=*]" =
  LINUX86;` for both Debug and Release of the single `eVAX` native target. `arch.h`'s
  `LINUX86` branch is the one missing `HAS64BITLONGS` (see §0) — flipping that is the
  natural place to start a fix.
- Several source files use classic **Mac OS CR-only line endings** (no LF) — a leftover
  from the original 1997-99 Mac development (`file <path>` reports "ASCII text, with CR
  line terminators" for e.g. `Source/RTL/service.c`, `Source/RTL/librtl_file.c`,
  most of `Source/CPU/*.c`, `Source/Console/parse.c` and `console_step.c`, most of
  `Source/Assembler/*.c`). Not a correctness bug, but some editors/tools (including
  `wc -l`, `grep -n`, and this audit's own tooling) will mishandle them silently —
  several line numbers cited above were obtained by normalizing a private copy first.
  Worth a one-time project-wide normalization pass before doing precise line-based
  patches.
- `Headers/vax.pch` is a **stale, unused duplicate** of `Headers/vax.h` (slightly
  out of sync — e.g. it still has the typo "pages are found to VM PTE's" that `vax.h`
  fixed to "pages are bound to"). Nothing in the current build includes it
  (`grep` confirms only `vax.h` is ever `#include`d); it's dead weight from the days
  when `vax.pch` was the project's precompiled header. Safe to delete, unrelated to the
  32/64-bit work.
- `Source/CPU/emul_locc.bak` is a stray backup file alongside `emul_locc.c`, also inert.
- `put1.exe` (top-level test fixture) is a zero-byte file — either a broken/incomplete
  fixture or leftover from a failed test run; worth checking against wherever the test
  suite expects to use it.

---

## 5. Suggested order of attack

1. **Fix the root typedef** (§0): give `LONGWORD`/`ULONGWORD` a true 32-bit definition
   on 64-bit builds (`int32_t`/`uint32_t`), keyed off pointer/platform width rather than
   a hand-maintained `#ifdef` list. **In the same change:**
   - Audit every pointer↔`LONGWORD`/`ULONGWORD` cast in the tree — `STROFF` in
     `memmap.h` (V6) is a confirmed instance that will silently break otherwise.
   - Fix or remove the `union MASKREG`/`PTE`/`VALUE` bitfield-size mismatches (C5 is
     fine as-is, but C8 and V7 are not) — replace with plain `unsigned int` bitfield
     containers per the `fab.h`/`rab.h` model (V9).
   - Re-enable (or replace) whatever logic is currently gated behind
     `#ifdef HAS64BITLONGS` and dead on this build — the ROTL mask (C7) is a confirmed
     live instance; grep for the symbol project-wide rather than fixing case-by-case.
   - Update the ~30+ `%l`-family format specifiers this exposes (C11, V4) in the same
     pass — they're invisible today only because `LONGWORD` happens to be `long`.
2. **Re-audit CPU and RTL after step 1** — rebuild and re-run findings C1-C4, C7, R2-R6
   with fresh `sizeof()` checks; expect most to disappear as a side effect, since the
   sign-extension, quadword register-pair, ROTL, and argument-marshaling tricks all
   become correct again once the type is genuinely 4 bytes. Confirm with targeted test
   programs (a simple negative-longword-compare VAX program for C1; a `MOVQ`/RMS
   file-open exercise for C2/R1) rather than assuming.
3. **Fix `fpu.c`'s float-conversion code as its own task, independent of step 1** (C3) —
   the `LSB`/`MSB` word-splitting trick needs to operate on a real 4-byte type
   regardless of what `LONGWORD` resolves to (e.g. an explicit `uint32_t` local pair, or
   `memcpy` into/out of a `uint32_t[2]`), since it's manipulating the bit-level layout of
   a `double`, not a VAX longword. This is also the one CRITICAL finding that is genuine
   memory-unsafe undefined behavior (OOB stack read/write) rather than "just" wrong
   arithmetic — worth prioritizing regardless of where it lands in the typedef-fix
   sequence.
4. **R1 (FAB/RAB pointer fields) needs a deliberate, separate architectural fix** — those
   fields are declared as real native pointers, not `LONGWORD`, so step 1 does not touch
   them. The right fix is to store a VAX address (a real 4-byte integer type) in these
   fields and resolve it through the existing memory-access path (`PHYADDR`/
   `load_memory`) on demand, rather than caching a native pointer that's only ever
   half-populated.
5. **Fix or version the ROM/NVRAM binary format now** (V1) — this is not a future
   concern, it is demonstrated broken today against the repo's own `xdefault.rom`.
   Simplest fix: change the `sizeof(LONGWORD)` reads/writes in `load_rom`/`save_rom`/
   `load_nvram`/`save_nvram` to a literal `4`, independent of whatever `LONGWORD`
   resolves to, so the on-disk format is pinned regardless of future typedef changes.
   Apply the same "pin to a literal 4" treatment to the whole-`struct VAX` SAVE/RESTORE
   format, or add a version tag to the file header, so a save file's provenance can be
   checked before trusting its layout.
6. **Resolve V8** — read `console_run.c`'s `.exe`-loading path to determine whether the
   `imgdef.h` structs are overlaid directly on file bytes (would make this a CRITICAL,
   immediately-reproducible bug against the repo's own `.exe` fixtures, same class as
   V1) or built up field-by-field (lower severity). This is the one finding in this
   report that needs a confirmation step before it can be prioritized correctly.
7. **Housekeeping** (optional, low-risk, unrelated to correctness): normalize CR-only
   line endings project-wide; remove `Headers/vax.pch` and `Source/CPU/emul_locc.bak`;
   investigate the zero-byte `put1.exe` fixture; consider replacing `dclrtl.c`'s
   mismatched function-pointer-type casts (V3) with correctly-typed function pointers.
8. **N1 (native host pointers cached in `LONGWORD` fields outside RMS)** — split out from
   item 4 rather than folded into it (see the 2026-09-14 item-4 progress entry below for
   why): `Source/Assembler/asm_symbols.c`'s `struct SYMBOL.value` (string-kind symbols),
   `Source/CPU/decode_opcode.c:78-79` (diagnostic name buffer, display-only), and
   `Source/Console/console_show.c:1110`/`console_dispatch.c:200`/`driver.c:199,215,220`
   (`set_symbol_direct` storing a native `char*` argument string). Needs the same kind of
   decision item 4 needed for FAB/RAB — either resolve through a real VAX address, or stop
   routing these through the VAX-facing `LONGWORD` field and keep them natively typed
   host-side — but is a separate subsystem (assembler symbol table / console display) with
   its own call sites and risk profile, so it's tracked as its own item rather than
   reopening item 4.

---

## 6. Progress log

This section is a living record of work actually done against the plan in §5, kept
up to date as tasks land. Earlier sections above are the original audit and are left
as-written (not edited in place) even where later work supersedes a detail; corrections
are noted here instead.

### 2026-09-14 — Task 5.1 (root typedef fix) and 5.2 (CPU/RTL reaudit) — done

**5.1 — Fixed the root typedef**

- `eVAX/Headers/arch.h`: `LONGWORD`/`ULONGWORD`/`QUADWORD` are now unconditionally
  `int32_t`/`uint32_t`/`int64_t` (`<stdint.h>`) on every platform, instead of tracking
  the host's native `long`. Retired the `HAS64BITLONGS` escape hatch entirely — it was
  only ever defined for `TRU64` (the root cause per §0) and is no longer needed now that
  the fix is unconditional rather than a hand-maintained platform list. Confirmed via an
  instrumented build: `sizeof(LONGWORD)==4`, `sizeof(ULONGWORD)==4`, `sizeof(QUADWORD)==8`,
  `sizeof(struct VAX)` dropped from 3104 to 1824 bytes, `sizeof(union PTE)` dropped from
  8 to 4 (now matching `PTEBITS` exactly, fixing C8 with no code change).
- `Headers/memmap.h`'s `STROFF` macro (V6): replaced the pointer→`LONGWORD`-cast-and-
  subtract trick with `offsetof()` (via `__typeof__`, since every call site passes a
  null-initialized typed pointer variable rather than a type name). This removes reliance
  on null-pointer-dereference UB and on `LONGWORD` happening to be pointer-width. In
  practice the old macro would likely have stayed correct even after narrowing — every
  call site's pointer is null, so the "subtraction" was always really just a small
  offset value well within 32 bits — but `offsetof()` is the standard, well-defined way
  to compute this and removes the doubt V6 raised.
- `Source/CPU/storage.c` (finding R7) and `Source/CPU/emul_ash.c` (finding C7): removed
  the dead `#ifdef HAS64BITLONGS` guards, making the partial-load zero-fill and the ROTL
  32-bit mask unconditional. Both are effectively no-ops now (the underlying type really
  is 4 bytes, so there's nothing to zero/mask) but are kept as cheap defensive belt-and-
  suspenders rather than deleted outright.
- **Fixed 3 real compile *errors*** (not warnings) that narrowing `LONGWORD` exposed —
  prototype/definition mismatches that only matched before because `LONGWORD` happened to
  equal `long`:
  - `storage.c`: `add_watchpoint`/`delete_watchpoint`/`watchpoint_hit` took
    `unsigned long addr`, but `vaxproto.h` declares `ULONGWORD addr`. Changed the
    definitions (and `struct WATCHPOINT.address`) to `ULONGWORD`, since these are VAX
    addresses.
  - `librtl_time.c`: `decc_time` returned `long`, but `shim.h` (and every other `decc_*`
    prototype in it) declares `LONGWORD`. Changed the definition to `LONGWORD`.
  - `console_vminit.c`: `console_vminit_dcl` took `LONGWORD id`, but `console_proto.h`
    and its 3 sibling `_dcl` handlers (`console_show_dcl`, `console_clear_dcl`,
    `console_test_dcl`) all use `long id` — these are native DCL callback ids (see V3),
    not VAX-facing values. Changed the definition to `long id` to match the majority
    convention instead of changing 4 other places.
  - Same class, found as plain `-Wincompatible-pointer-types` warnings rather than hard
    errors, fixed for consistency: `emul_bitfield.c` (`(long*)&result` →
    `(LONGWORD*)&result`, since `result` is `ULONGWORD`), `librtl_memory.c`
    (`long local_argv[3]` → `LONGWORD local_argv[3]`, passed to a `LONGWORD*`-expecting
    function), `librtl_print.c` (`decc_apply_format`'s `long * pos` parameter →
    `LONGWORD * pos`, matching its only caller's local).
  - `asm_operand.c` (V7 fallout): `if( value.longword == -1L )` was comparing a 32-bit
    `ULONGWORD` against a `long` `-1L`; the comparison promotes to 64 bits and is now
    always false (`-Wtautological-constant-out-of-range-compare`). Changed the literal to
    plain `-1` so the comparison happens in the correct 32-bit domain. Side note: this
    check was *already* unreliable before this fix too, for the unrelated reason that
    `union VALUE` was size-mismatched (V7) — now that the union is correctly sized, the
    literal width was the one remaining thing keeping it broken.
  - `console_load.c`: `base = ( unsigned long ) -1L` → `( ULONGWORD ) -1L`. The
    truncation to `0xFFFFFFFF` was already the intended 32-bit sentinel value; this only
    silences a `-Wconstant-conversion` warning from the stale 64-bit cast.
- Fixed the ~250+ `%l`-family format-string/argument-width mismatches C11/V4 predicted
  would appear, mechanically, via `clang -Xclang -fixit` run per affected file (34 files
  across Assembler/Console/CPU/RTL — much broader than V4's original "~30+ CPU-only"
  estimate, because V4's pre-fix pass correctly found *zero* live mismatches at the time
  — `LONGWORD` still equaled `long` — and could only warn what the fix would later
  expose). Confirmed via a pre-fix build that there were 0 such warnings before the
  `arch.h` change, so all of them are newly-exposed, not pre-existing. Each file's
  original line-ending convention was preserved: many source files here are CR-only (see
  the CLAUDE.md gotcha) and `-fixit` normally re-serializes output as LF, so each fixed
  file was round-tripped back (`\n`→`\r`) and diffed against the original to confirm only
  the format specifiers themselves changed.
- **Build status after 5.1**: `clang -DLINUX86 -I eVAX/Headers -std=gnu99
  $(find eVAX/Source -name "*.c") -o evax -lm` succeeds with **0 errors, 20 warnings**
  (down from 0 errors/0 format-relevant warnings before the fix, but with the bug class
  entirely invisible to the compiler per §0/§4 — now the remaining 20 warnings are all
  genuine, newly-visible native-pointer↔`LONGWORD` truncation casts; see "New findings"
  below). 42 files touched in total.

**5.2 — Re-audited CPU and RTL**

Re-verified findings against the fixed build both by re-reading the code against the
now-genuinely-4-byte type, and by exercising the actual emulator interactively (built
`/tmp/evax`, drove it via the console's `DO`/`SHOW REGISTERS`/`EXAMINE` commands, plus a
standalone harness linked against the real object files calling `fpu_store`/`fpu_load`
directly) — per §5 item 2's own recommendation not to just assume these disappeared.

- **C1 (sign extension) — CONFIRMED FIXED.** Live test: `DO MOVL #^XFFFFFFFE,R1` (-2)
  then `DO CMPL R1,#0` now correctly sets PSL N=1. Before this fix the same sequence
  would zero-extend -2 into a large positive 64-bit value and set N=0.
- **C2 (quadword register-pair reconstruction) — CONFIRMED FIXED.** Live test:
  `MOVQ R2,@addr` / `MOVQ @addr,R4` round-tripped R2:R3 = `12345678:ABCDEF01` through VAX
  memory and back out into R4:R5 intact. Before this fix, R5 (the high longword) would
  have come back zero.
- **C3 (fpu.c OOB stack read/write) — CONFIRMED FIXED, "for free."** This **corrects**
  §5 item 3, which treated `fpu.c` as needing a dedicated fix independent of the typedef
  change. In fact `fpu_store`/`fpu_load`'s `ULONGWORD * pd = (ULONGWORD*)&source`
  word-splitting trick needed exactly what C1/C2 needed: a real 4-byte word type, nothing
  fpu.c-specific. Verified with a standalone harness (linked against the real
  `fpu_store`/`fpu_load` objects) round-tripping 5 values — `2.0`, `-3.5`,
  `3.14159265358979`, `123456.789`, `-0.001` — through F/D-floating and back at full
  `double` precision with zero relative error. Previously this code read/wrote 8 bytes
  past the end of an 8-byte stack `double`. **§5 item 3 is superseded by this entry — no
  separate fpu.c change is needed**, it should be treated as done rather than pending.
- **C4 (RTL return values corrupting registers) — fixed for free**: `vax.R0 = rc` is a
  clean 4-byte assignment now that both sides are genuinely 32-bit.
- **C5 (MASKREG) — still correct**, and now also correctly *sized* (bonus; was already
  functionally fine per the original audit's C5 note).
- **C6 (vm.c bit masks) — unaffected**, as originally noted.
- **C7 (ROTL mask) — fixed** (unconditional now, see above), and moot regardless since
  `value` is genuinely 4 bytes so there's nothing for the mask to strip.
- **C8 (union PTE) — CONFIRMED FIXED**, no code change needed beyond the typedef:
  `sizeof(union PTE)` is now 4 (was 8), matching `PTEBITS`.
- **C9 (diagnostic extra hex digits) — fixed for free**, and cleaned up further by the
  format-specifier pass.
- **C10 (BIGENDIAN dead code) — unaffected**, still correctly inert on this
  little-endian build; unchanged risk profile for a hypothetical future big-endian port.
- **C11 (format specifiers) — fixed**, see the `-fixit` pass above.
- **R1 (FAB/RAB native pointer fields) — CONFIRMED STILL BROKEN, as expected** — this
  was explicitly deferred to §5 item 4 (a separate architectural fix), since narrowing
  `LONGWORD` does not by itself fix a field that's declared as a genuine native pointer.
  **The failure mode has changed, though**, worth noting for whoever picks up item 4:
  before this fix, these fields were populated with *garbage* upper bits (uninitialized
  memory), and any nonzero garbage would almost certainly access-violate immediately and
  visibly. Now, the fields are cleanly zero-then-4-byte-populated (per the `load_memory`
  partial-load zero-fill, R7), but the cast itself (`(LONGWORD) rab.rab_l_fab`,
  `rms.c:144` etc.) now *silently and deterministically* truncates a real 64-bit host
  pointer to 32 bits — which will happen to "work" for any pointer that lands in the low
  4GB of address space and corrupt/misdirect for one that doesn't. This is arguably
  *harder* to diagnose than before (not reliably reproducible run-to-run, depends on where
  malloc/the OS happens to place the FAB/RAB's storage). Confirmed via fresh compiler
  warnings (`-Wpointer-to-int-cast`, 10 instances, all in `rms.c`) that did not exist
  before this fix.
- **R2 (SYS$CLI stack index) — fixed for free**: `len`/`ptr` are genuinely 4 bytes now,
  fully populated by the existing 4-byte `load_memory` call, so no garbage upper bits are
  possible any more.
- **R3 (argv vector garbage) — fixed for free**, same mechanism as R2.
- **R4 (str_get/str_put daddr) — fixed for free**, same mechanism.
- **R5 (GETDVIW/GETJPIW item-list addresses) — fixed for free**, same mechanism.
- **R6 (indirect-PID in-place corruption) — fixed for free**: a 4-byte `load_memory`
  into a genuinely-4-byte `pid` now fully overwrites it; no stale high bytes possible.
- **R7 (storage.c band-aid) — addressed**, see the `HAS64BITLONGS` removal above (kept
  unconditional rather than deleted, even though it's now effectively a no-op).

**New findings surfaced by this fix** (not in the original audit; found via fresh
compiler warnings and live testing after the typedef change — confirmed absent from a
pre-fix build, so genuinely newly-exposed rather than missed the first time):

- **N1. Native pointers stored in `LONGWORD`/symbol-table `value` fields, beyond RMS's
  R1 — MEDIUM, same class as R1, needs the same kind of architectural decision.**
  `Source/Assembler/asm_symbols.c` (`struct SYMBOL`'s `value` field stores raw host
  pointers for some symbol kinds — lines 127, 183, 948), `Source/CPU/decode_opcode.c:78-79`
  (stores a native `char*` into the opcode's diagnostic name buffer via a `LONGWORD`
  cast — display-only, lower stakes), and `Source/Console/console_show.c:1110`,
  `console_dispatch.c:200`, `driver.c:199/215/220` (`set_symbol_direct` storing a native
  `char*` argument-string pointer into a `LONGWORD` symbol value) all now produce
  `-Wint-to-pointer-cast`/`-Wpointer-to-int-cast` warnings — confirmed truncating casts of
  real host pointers through a 32-bit `LONGWORD`, invisible before for the same reason R1
  was. These need the same decision R1 needs (store a real VAX address and resolve
  through `PHYADDR` on demand, or keep these entirely native/host-side and stop routing
  them through the VAX-facing `LONGWORD` symbol-table field) rather than a mechanical
  fix — recommend folding into §5 item 4's scope (or giving it its own numbered item)
  before touching it.
- **N2. `Source/CPU/emul_float_math.c`'s CVTFL/CVTRFL/CVTDL (float→longword conversion)
  has a wrong overflow bounds check — MEDIUM, pre-existing, unrelated to the 32/64-bit
  typedef issue, newly discovered by this session's live testing (not something a
  typedef fix could affect either way).** The `func == 4` (float-to-integer) handler's
  `case 2: /* Long */` branch reuses the *Byte* case's bounds
  (`if( float1 < -128.0 || float1 > 127.0 )`) instead of a longword-range check, so
  `CVTFL`/`CVTRFL`/`CVTDL` spuriously faults with `EXC_ARITH`/`TRAP_INT_OVF` for any
  value outside `[-128,127]`. Reproduced live: `DO MOVL #100,R1` / `DO CVTLF R1,R2` /
  `DO CVTFL R2,R3` raises an unhandled arithmetic exception, even though 100 is nowhere
  near a real 32-bit longword overflow bound. Looks like a copy/paste from the
  byte-conversion case above it. Flagged here only because this session's testing found
  it; it belongs in its own follow-up task, not folded into the 32/64-bit fix work.

**Build/verify status**: full build — 0 errors, 20 warnings (all N1/R1-class, itemized
above). Emulator boots and runs interactively against the repo's own `evax.dcl`/
`vax.init`/`vax.help`. 42 files touched; every file kept its original line-ending
convention (CR-only files stayed CR-only, LF files stayed LF).

**Status of §5's remaining items**: item 3 should now be considered **done** (see C3
above — the separate fpu.c fix it called for turned out to be unnecessary). Item 4 (R1's
architectural fix) is still open, and its scope has grown to include N1. Items 5 (V1,
ROM/NVRAM binary format), 6 (V8, `.exe`/`imgdef.h` loading path — still needs the
"does `console_run.c` overlay `struct IHD` directly on file bytes?" investigation), and 7
(housekeeping) are untouched. N2 is a new, independent follow-up item not tied to any
existing §5 numbering.

### 2026-09-14 — Task 5.4 (R1 architectural fix) — done; N1 split into its own item (§5.8)

**Fixed R1** per §5 item 4's own prescription: `Headers/fab.h` (`fab_l_xab`, `fab_l_nam`,
`fab_l_fna`, `fab_l_dna`) and `Headers/rab.h` (`rab_l_ubf`, `rab_l_rbf`, `rab_l_rhb`,
`rab_r_kbf_overlay.rab_l_kbf`/`rab_l_pbf`, `rab_l_fab`, `rab_l_xab`) now declare these
fields as `LONGWORD` (a real 4-byte VAX address) instead of genuine native pointers
(`char *`, `struct NAM *`, `struct FAB *`). `struct NAM` was only ever forward-declared
for this one field and is otherwise undefined anywhere in the tree, so narrowing
`fab_l_nam` to `LONGWORD` also removes the last reference to that incomplete type.

- No changes were needed to `Source/RTL/structure_mapping.c`'s `map()`/`store_field()` —
  they already fill/store these fields via `load_memory(addr, p, size)`/
  `store_memory(...)` at a raw byte offset with `size` hardcoded to 4 (`rms.c`'s
  `map_int(..., 4)` calls), so narrowing the field's declared width from 8 to 4 bytes is
  exactly what those call sites already assumed — a "write 4 bytes into a genuinely
  4-byte slot" instead of "write 4 bytes into the low half of an 8-byte slot".
- `Source/RTL/rms.c`: removed the now-unneeded `(LONGWORD)` truncating casts at every read
  of these fields (`rms_put`, `rms_connect`, `rms_create` — the exact sites the 5.2 reaudit
  flagged via fresh `-Wpointer-to-int-cast` warnings), since the fields are already
  `LONGWORD` and the casts were the architectural symptom being fixed. Also removed a dead
  `fab.fab_l_fna = fn;` assignment in `rms_create` (line ~239 pre-fix) that stored a real
  host `char*` — the local filename buffer — into what is now correctly a VAX-address
  field, and was never read back afterward in that function; it was vestigial even before
  this fix (the field held a truncated copy of the *original* VAX-side pointer at that
  point, immediately overwritten with an unrelated host pointer that nothing subsequently
  used).
- Deliberately did **not** touch `rms_put`'s unrelated `(LONGWORD) f == 0 || (LONGWORD) f
  == -1` check on `FILE * f` (the native `ifi[]` table entry, `rms.c:154/156`) — this isn't
  one of R1's FAB/RAB fields, `f` is purely host-side and never crosses into VAX memory, and
  the `-1` sentinel (`ifi[0] = (FILE*) -1`) is deliberate. It still produces 3
  `-Wpointer-to-int-cast` warnings post-fix; pre-existing and out of scope for item 4.
- **Did not fold in N1** despite the 5.2 note recommending it: N1 lives in a different
  subsystem (assembler symbol table + console display code, not RTL/RMS), touches a
  string-typed symbol-table field where the "right" fix is a genuine architectural choice
  (native-side vs. VAX-side) independent of R1's, and bundling it in risked scope creep
  into files this task didn't otherwise need to touch. Split out as §5 item 8 instead, per
  the audit's own noted alternative ("recommend folding into item 4's scope (or giving it
  its own numbered item)").

**Verification**:
- Full rebuild (`clang -DLINUX86 -I eVAX/Headers -std=gnu99 eVAX/Source/**/*.c -o evax
  -lm`): 0 errors. Warning count dropped from 20 to 13 — all 10 of the FAB/RAB
  `-Wpointer-to-int-cast`/`-Wint-to-pointer-cast` warnings the 5.2 reaudit attributed to R1
  are gone from `rms.c`; the 3 remaining `rms.c` warnings are the unrelated `FILE * f`
  pattern noted above, and the other 10 (N1, unrelated files) are unchanged as expected.
- Regression-tested against the repo's own fixtures: `RUN PUT`, `RUN PUTC`, and `RUN CLI`
  all still execute correctly with no crash or behavior change (`RUN PUT`/`RUN PUTC`
  produce their expected "This is a test..."/"Hello, world" output).
- **Caveat, noted for whoever revisits this**: none of the repo's `.exe`/`.asm` fixtures
  actually exercise `rms_create`/`rms_connect`/`rms_put` — `put.exe`/`putc.exe`'s output
  comes from the `decc_printf`/`decc_fprintf` CRTL shims (`Source/RTL/librtl_print.c`),
  which write directly rather than routing through RMS FAB/RAB. Confirmed via `SET DEBUG
  RMS` before `RUN PUT`/`RUN PUTC`: none of `rms.c`'s `DBG_RMS`-gated trace output ever
  printed, meaning `SYS$CREATE`/`SYS$CONNECT`/`SYS$PUT` were never actually invoked by
  either fixture. So this fix is verified by build/warning evidence and by confirming no
  regression in the fixtures that do run, but **not** by observing the fixed code path
  execute correctly end-to-end — there is currently no fixture in the repo that calls
  `SYS$CREATE` and would need a purpose-built VAX assembly test (a real FAB/RAB laid out
  in VAX memory and a `CHMK`-based `SYS$CREATE`/`SYS$CONNECT`/`SYS$PUT` sequence) to close
  that gap.

### 2026-09-14 — Task 5 (V1, ROM/NVRAM binary format) — done

**Note on scope at the time this was picked up**: task 5.1's root typedef fix already
made `sizeof(LONGWORD)==4` unconditionally, so the specific reproduction V1 described
(`LOAD/ROM xdefault.rom` desyncing because `sizeof(LONGWORD)==8`) was already fixed as a
side effect before this task started — confirmed live: `LOAD/ROM xdefault.rom` reads
correct `CONSOLE$ROM_BASE`/`CONSOLE$ROM_END` (`20040000`-`200BFFFF`, matching a hex-dump
of the fixture's header) even before today's change. What remained was V1's actual
prescription: stop relying on `sizeof(LONGWORD)` to *happen* to be 4 and pin the on-disk
format to a literal `4`, so a future change to `LONGWORD` (or a build on some other host)
can't silently desync these file formats again the way the original bug did.

- **`Source/Console/save_binary.c`** (`save_binary`/`save_rom`/`save_nvram`) and
  **`Source/Console/console_load.c`** (`console_load`/`load_rom`/`load_nvram`): every
  `fread`/`fwrite` of a ROM/NVRAM/whole-VAX-save page-record address or count field now
  uses a literal `4` instead of `sizeof( LONGWORD )`. These fields are an on-disk format
  contract (`;ROMIMG`/`;//EOF*` page records), not an in-memory `LONGWORD` value, and
  should never have been sized off the internal typedef in the first place.
- **Whole-`struct VAX` SAVE/LOAD format** (the other half of V1 — the "cross-build
  compatibility hazard" noted as needing a deliberate decision, not urgent but not left
  implicit either): added a pinned 4-byte `sizeof(struct VAX)` field to the file, written
  right after the existing 2-byte version number and read back before trusting the
  wholesale `fread(&fvax, sizeof(struct VAX), 1, fp)` that follows. `console_load()` now
  rejects the file outright (clear error message, no partial/garbled load) if the stored
  size doesn't match this build's `sizeof(struct VAX)`, rather than silently misreading a
  file written by a different build's layout — exactly the failure mode V1 flagged for
  this format. Bumped the version number `2.0` → `2.1` to go with the added field (no
  fixture in the repo uses this format, so there was nothing to stay compatible with).
- Left `struct VAX`'s field-by-field copy logic (`console_load.c:191-242` — VM regions,
  console state, registers, etc.) untouched; this task only addressed the on-disk framing
  (record sizes, layout provenance check), not that unpacking logic.

**Verification** (interactively, via `/tmp/evax` built with
`clang -DLINUX86 -I eVAX/Headers -std=gnu99 eVAX/Source/**/*.c -o evax -lm`, driven with
piped DCL commands):
- Full rebuild: 0 errors, 13 warnings — unchanged from the count after task 5.4 (all
  N1/R1-`FILE*`-class, none of which this task touched). No new warnings introduced.
- `LOAD/ROM xdefault.rom` against the repo's real fixture: correct
  `CONSOLE$ROM_BASE`/`CONSOLE$ROM_END`/`CONSOLE$ROM_SIZE` (`20040000`/`200BFFFF`/512K),
  matching the file's own header bytes.
- `SAVE/ROM` → `LOAD/ROM` round-trip (loaded `xdefault.rom`, saved it back out, reloaded
  the saved copy): identical base/end addresses recovered.
- Whole-VAX `SAVE` → `LOAD` round-trip: registers/PC/PSL correctly restored.
- Confirmed the new version/size guard actually rejects bad files instead of misreading
  them: patched a saved file's stored struct-size field to `3104` (the pre-5.1-fix
  `sizeof(struct VAX)`) — load refused with `"...was saved by a build with a different
  VAX layout (3104 bytes vs. 1824 expected); refusing to load."` rather than silently
  desyncing. Separately patched the version byte back to `2.0` — load refused with
  `"Unsupported file version 2.0"` as before.
- Note: `SAVE/ROM`/`LOAD/ROM`/`LOAD /path` do not accept a leading `/` in the filename
  (the DCL qualifier parser consumes it looking for a `/QUALIFIER`) — pre-existing
  behavior unrelated to this fix, worked around in testing by using relative paths.

**Status of §5's remaining items**: item 6 (V8, `.exe`/`imgdef.h` loading path) and item 7
(housekeeping) are still untouched. Item 8 (N1, split out from item 4) is also still
open.

### 2026-09-14 — Task 6 (V8, `.exe`/`imgdef.h` loading path) — resolved, checked, not a bug

**Read `Source/Console/console_run.c`'s image-loading path directly**, per V8's own
action item, to resolve which of its two hypothesized cases applies.

**Finding: the field-by-field case applies, not the raw-overlay case.** `image_load()`
(`console_run.c:478`) reads the `.exe`'s header blocks into `vax.memory` as raw bytes
(via `fread`), but never overlays `struct IHD`/`IHI`/`ISD`/`IAF` directly onto that
buffer. Instead it calls the same declarative `map()` field mapper already used (and
already fixed) for FAB/RAB in task 5.4:

- `init_ihd_maps()` (`console_run.c:1331-1509`) registers every `IHD`/`IHI`/`ISD`/`IAF`
  field individually via `map_add( map_name, field_name, vax_offset, STROFF( ptr, field
  ), size, kind )`, where `size` is a small literal byte count (1, 2, 4, 15, or 39) taken
  from the real VMS image-header format — never `sizeof(LONGWORD)` or any other
  typedef-derived value — and `STROFF` (already `offsetof()`-based since 5.1) gives the
  exact byte offset of that field inside the host struct regardless of compiler padding.
- `image_load()`/`image_fixup()` then call `map( "IHD", &ihd, base )` etc.
  (`console_run.c:658,694,719,948,1111`), and `map()` (`Source/RTL/structure_mapping.c:163`)
  walks that field list calling `load_memory( addr + v_offset, (char*)dest + p_offset,
  ml->size )` per field — a byte-exact copy of exactly `size` bytes into exactly the
  right struct offset, independent of the field's native C type or alignment.
- Confirmed `load_memory` (`Source/CPU/storage.c:457`) honors an arbitrary `count`
  (1/2/4 here) by writing exactly that many bytes starting at `dest[0]`
  (`storage.c:546-549`), so a `short`-sized field gets exactly 2 bytes and a
  `LONGWORD`-sized field exactly 4 — no cross-field spillover, no dependence on
  `sizeof(LONGWORD)`.
- Grepped the file for any `(struct IHD *)`/`(struct ISD *)`/`(struct IHI *)`/
  `(struct IAF *)`/`(struct SHR *)` cast that would indicate a raw overlay onto file
  bytes — none exist; every use of these structs is either a `map()` destination (a
  local/heap variable populated field-by-field) or a purely host-side bookkeeping
  structure (`struct ICB`/`SHR`'s linked-list fields) never read from file bytes at all.

This is exactly the "much lower severity" case V8 itself described for this pattern, and
it turns out to be the actual case — **not a bug**, same class as V2/V3/C5/C6/V9 (checked
patterns already using the "right" approach `V9` recommended as the model for others to
copy). No code change made.

**Verification**: built `/tmp/evax` and ran `RUN` with `SET DEBUG IMAGES` against all 7
usable `.exe` fixtures (`put1.exe` is still the known zero-byte non-fixture) —
`cli.exe`, `dbl.exe`, `getvm.exe`, `put.exe`, `putc.exe`, `sieve.exe`, `simple.exe`. All
parse their image header and Image Section Descriptors cleanly (sensible addresses,
page counts, and flags in the debug trace, no "Unable to read/resolve" errors). `RUN
put.exe`/`RUN putc.exe` reproduce their expected output ("This is a test..."/"Hello,
world"). `RUN simple.exe` additionally exercises the fixup-record path
(`image_fixup`/`IAF` map) end-to-end, up to failing on missing secondary shared images
(`DECC$SHR.EXE` etc. — those files don't exist in the repo, an unrelated environment gap,
not a header-parsing bug).

**Status of §5's remaining items**: item 7 (housekeeping) and item 8 (N1) are still
open. All CRITICAL/HIGH findings in the original audit (§1-§3) are now either fixed or
confirmed not-a-bug; what remains is cleanup and the N1 architectural decision.

### 2026-09-14 — Task 7 (housekeeping) — done

Worked through all four sub-items §5 item 7 listed.

**Normalized CR-only line endings project-wide.** 68 of the 114 `.c`/`.h` files under
`eVAX/` used classic Mac OS CR-only line endings (the CLAUDE.md gotcha); the other 46
were already LF. Confirmed first that every one of the 68 is *pure* CR with zero
embedded LF bytes (`grep -c $'\n'` style check via a Python byte scan), so a plain
`\r` → `\n` byte replacement is unambiguous — no risk of a stray real `\n` colliding
with a converted `\r` and producing a blank line. Converted all 68 in place and verified
byte-for-byte that the only change is CR→LF: for each file, `git show HEAD:<path>` with
`\r` replaced by `\n` compared identical to the new on-disk content (0 mismatches across
68 files). Top-level data files read by the running emulator at startup (`evax.dcl`,
`vax.init`, `vax.help`) were checked and are already LF — untouched, no risk there.

**Removed `Headers/vax.pch` and `Source/CPU/emul_locc.bak`.** Confirmed both were dead
before removing: `vax.pch` appears in `project.pbxproj` only as a `PBXFileReference`/
group-listing entry, never as `GCC_PREFIX_HEADER` or any other build setting — not
actually used as a precompiled header. `emul_locc.bak` likewise appears only in the file
group, never in the `Sources` build phase (only `emul_locc.c` is compiled), and a diff
against `emul_locc.c` shows its only difference is `#include "vax.pch"` vs `#include
"vax.h"` — a pre-refactor leftover, not an alternate implementation worth keeping.
Removed both files (`git rm`) and their `PBXFileReference` + `PBXGroup` entries in
`project.pbxproj` (`plutil -lint` confirms the edited project file is still well-formed)
so Xcode won't show dangling red references.

**Investigated the zero-byte `put1.exe` fixture.** It has been empty since the single
initial commit of this repository (before that, CVS metadata was removed per CLAUDE.md,
so there's no earlier history to recover a real version from). No `put1.asm` or other
source exists anywhere in the tree to regenerate it from, and nothing in the emulator
source or DCL scripts references it — only this audit's own notes do. Left as-is (not
deleted): it's inert clutter, not a bug, and there's no evidence either way about what
the user wants done with a fixture this old with no recovery path — flagging here rather
than guessing.

**Replaced `dclrtl.c`'s mismatched function-pointer casts (V3) where they were live
bugs, left one alone where it wasn't.** V3 flagged several `-Wcast-function-type-mismatch`
casts; on inspection they split into two different situations:

- **Fixed — `DCLdump`'s callback parameter type** (`dclrtl.c:619,1705`, call sites
  `1111,1624,1641`): `DCLdump` declared its callback parameter as
  `long(*)(long,char*,...)`, but every real function ever passed to it
  (`DCLcallback_dcl`, `DCLresults_counter`, `DCLresults_load`, and the publicly
  documented model callback `DCLcallback_debug`) actually takes `int kind` as the first
  parameter, forcing every call site to paper over the self-inflicted mismatch with a
  cast. Changed `DCLdump`'s parameter type to `int` to match reality and removed all 3
  now-unnecessary casts.
- **Fixed — `struct DCL_VERB.routine` dispatch field** (`dclrtl.c:421,1560,1592`): this
  one was a genuine live-risk bug, not just a style issue. The field was typed
  `long(*)(int)`, and `DCLdispatch()` explicitly truncated `v->id` (a `long`) down to
  `int` before calling through it (`(*(v->routine))((int) v->id)`) — but every function
  actually bound here via `DCLbind()` (`console_show_dcl`, `console_clear_dcl`,
  `console_exit_dcl`, `console_test_dcl`, `console_vminit_dcl`, per `console_proto.h`)
  is declared `LONGWORD f( long id )`, i.e. expects the *un*-truncated `long`. This is
  exactly the class of bug V3 warned about: works today only because x86-64 happens to
  zero-extend a 32-bit register write, and would silently misdispatch on any ABI that
  doesn't guarantee that. Changed the field to `long(*)(long)`, the bind-site cast at
  1560 to match, and the call site to pass `v->id` directly with no truncating cast.
- **Left alone — `DCLsignal`/`DCLparse`'s public `long(*)(void)` callback parameter**
  (`dclrtl.c:721,724,754,778`, and the `DCL_INPUT` sentinel in `dclrtl.h`): unlike the
  two above, this one is never actually exercised with a real callback anywhere in this
  tree — `DCLsignal()` has zero callers, and `DCLparse()`'s only caller
  (`console_dispatch.c:124`) passes the `DCL_INPUT` sentinel (`(long(*)(void))-1L`, never
  actually called through), not a real function. The cast at each site is a well-defined
  function-pointer round-trip (cast to the real type, stored, always later called only
  through the correctly-retyped static — never called through the mismatched type
  directly), so there's no live UB today, and "fixing" it would mean redesigning a
  documented public-header callback convention (`dclrtl.h`) for a path nothing in this
  codebase currently uses — judged out of scope for a housekeeping pass. Documented here
  in case a future caller actually wires up a real signal/read callback.

**New finding surfaced by this investigation** (not fixed, flagged for a future task):
while confirming the `DCL_VERB.routine` fix, found that the bound `_dcl`-style functions
disagree on *return* type, not just the parameter type that was fixed. `console_proto.h`
declares `console_show_dcl`/`console_clear_dcl`/`console_exit_dcl`/`console_test_dcl`/
`console_vminit_dcl` as returning `LONGWORD` (4 bytes), while `vaxproto.h` declares the
other four functions bound the same way (`define_logical`, `show_logical`,
`define_device`, `show_device`) as returning native `long` (8 bytes on this host) —
and `DCL_VERB.routine`'s declared return type (`long`) can only match one of the two
groups. This mismatch is invisible to the compiler here because `DCLbind()`'s `addr`
parameter is `void*`, so the cast to `long(*)(long)` at the bind site never triggers
`-Wcast-function-type-mismatch`. Not fixed in this pass: resolving it properly means
picking one canonical return type and updating prototypes (and possibly definitions) in
two separate headers, which is a bigger, more deliberate change than this housekeeping
task's scope — and, per the same x86-64 zero-extension reasoning as the parameter-type
fix above, is likely benign in practice today for the small non-negative return codes
these functions actually produce.

**Verification**: full rebuild after every sub-step (line endings, dead-file removal,
dclrtl.c casts) — 0 errors, 13 warnings throughout (unchanged from before this task;
these casts weren't warned on by the default build flags, only found via inspection
prompted by V3's `-Wcast-function-type-mismatch` note in the original audit). Exercised
interactively after all changes: `LOAD/ROM xdefault.rom` + `SHOW ROM` (exercises
`console_show_dcl` through the fixed dispatch path), `DEFINE/LOGICAL` + `SHOW LOGICAL`
(exercises `define_logical`/`show_logical`, the *other* return-type convention, through
the same dispatch path), `DO MOVL #5,R3` + `SHOW REGISTERS`, and `RUN` against
`cli.exe`/`put.exe`/`putc.exe`/`simple.exe`/`sieve.exe` — all behave identically to
their pre-task-7 runs (`sieve.exe` still reports "The last prime is 99991", `putc.exe`
still prints "Hello, world").

**Status of §5's remaining items**: only item 8 (N1) is still open, plus the two new
small follow-ups noted above (put1.exe's fate, the `_dcl`-return-type inconsistency) —
neither urgent, both flagged for whoever picks this up next.

### 2026-09-14 — Task 8 (N1 architectural fix) — done

**Fixed N1** per the same prescription R1 used (§5 item 4): for the string-valued
console symbols (`SYM_STRING`), stop truncating a native host `char*` through the
32-bit `LONGWORD value` field and give it a real, correctly-sized home instead.

- `Headers/vax.h`: added `char * svalue;` to `struct SYMBOL`, right before the trailing
  `name[1]` flexible-array member (so `make_symbol()`'s `size = len +
  sizeof(struct SYMBOL)` allocation trick still works unchanged). Documented as holding
  the string value when `SYM_STRING` is set, native/host-side, deliberately kept
  separate from `value`.
- `Source/Assembler/asm_symbols.c`: `make_symbol()` now initializes `svalue = 0L` for
  every new symbol (mirrors `value = 0L`; `getmem()` doesn't zero its memory, confirmed
  by reading its implementation, so this isn't optional). Added
  `set_symbol_string_direct( char * symbol, char * strvalue, int flags )`: reuses
  `set_symbol_direct()` for all the existing name-parsing/lookup/creation machinery
  (passing a dummy `0L` numeric value, since string symbols don't use one), then sets
  `svalue` on the resulting `vax.console.last_symbol` — confirmed via code reading that
  `set_symbol()`/`get_symbol()` reliably leave `last_symbol` pointing at the just-
  processed symbol on every `VAX_OK` return path, both "found existing" and "newly
  created". Fixed the two display sites (`dump_system_symbols`, `dump_symbols`) to read
  `p->svalue` directly instead of casting `p->value`. Also fixed an unrelated
  `-Wpointer-to-int-cast`-class issue on the same sweep: a debug `printf` in
  `clear_system_symbols()` was truncating a `struct SYMBOL*` through `(LONGWORD)` for
  display; changed to `%p`/`(void*)`.
- `Source/CPU/decode_opcode.c`: the `addr = (unsigned char*) pc; sprintf(...,
  (LONGWORD) addr)` pattern was a pointless round-trip -- `pc` (a VAX address, already a
  `LONGWORD`) was being cast to a host pointer and immediately back to a `LONGWORD` for
  display, never dereferenced as a pointer. Removed the dead `addr` variable and the
  round-trip; prints `pc` directly. Confirmed via grep this was `addr`'s only use in the
  file (a second variable, `pb`, sharing the original declaration line, is a real
  pointer used elsewhere and was kept).
- `Source/Console/driver.c`: the 3 call sites that create `CONSOLE$ARG_CMD`/
  `CONSOLE$ARG_<n>`/`CONSOLE$ARG_FILE` (native heap strings built from `argv[]`, never
  VAX memory) now call `set_symbol_string_direct()` instead of truncating the pointer
  through `set_symbol_direct()`'s `LONGWORD` parameter.
- `Source/Console/console_show.c` (`sym->svalue` display fix, SHOW SYMBOL case 149) and
  `Source/Console/console_dispatch.c` (`&&SYMBOL` substitution in `expand_command()`)
  updated to read `svalue` off the resolved symbol instead of casting the numeric
  `value`.

**Two additional truncating-cast sites found during this task, not in N1's original
list** -- found by testing (see below), then confirmed by re-grepping every
`get_symbol`/`get_symbol_direct` call site in the tree for a pointer-typed destination
cast through `LONGWORD*`:

- `Source/Console/console_include.c:87` (`INCLUDE/COMMAND_LINE`, the mechanism vax.init
  uses to run a command passed on eVAX's own command line): was
  `get_symbol_direct( "CONSOLE$ARG_CMD", ( LONGWORD * ) &cmdbuff )` where `cmdbuff` is a
  `char*` local -- writes only the low 4 of 8 bytes, leaving the upper 4 as whatever
  stack garbage preceded the call. Fixed to look up the symbol via `get_symbol_direct`
  (dummy output var) and then read `vax.console.last_symbol->svalue`.
- `Source/Console/console_show.c`, `SHOW COMMAND_ARGS` (case 159): two identical
  instances -- `get_symbol_direct( "CONSOLE$ARG_FILE", (LONGWORD*) &p )` and, in the
  `CONSOLE$ARG_<n>` loop, `get_symbol_direct( buff, (LONGWORD*) &p )`, both with `p`
  declared `char *`. Fixed the same way. (A third call in the same case,
  `CONSOLE$ARG_COUNT`, is a genuine numeric symbol into a `LONGWORD` destination --
  correct as-is, left untouched.)

Every other `get_symbol`/`get_symbol_direct` call in the tree was individually checked
against its destination variable's declared type (`asm_expr.c`, `console_show.c`'s
stringpool code, `console_clear.c`, `console_run.c`, `emul_xfc.c`, `asm_pseudo.c`,
`console_load.c`): all pass a `LONGWORD*`/`ULONGWORD*` onto a genuinely 4-byte-typed
local, holding a real VAX-side address, not a native pointer -- confirmed not part of
this bug class.

**A third, unrelated bug found and fixed along the way**: `driver.c`'s
`CONSOLE$ARG_FILE` path allocated `pq = getmem( strlen( pp ))` then `strcpy( pq, pp )`
-- a one-byte heap buffer overflow (`strcpy` also writes the null terminator, which
`strlen(pp)` bytes doesn't leave room for). Reproduced concretely: `SHOW COMMAND_ARGS`
printed `"Executing file COMMAND_ARGSD"` (a stray trailing byte) before this fix, clean
output after. Fixed by allocating `strlen(pp) + 1`. The sibling dash-argument branch
(`pq = getmem( strlen( pp )); strcpy( pq, pp+1 )`) was checked and is *not* off-by-one
-- `pp+1` is one byte shorter than `pp`, so `strlen(pp)` bytes is exactly right there --
left unchanged rather than "fixed" into an unnecessary extra byte.

**How this was actually found (worth recording as a cautionary note on this class of
fix)**: the `svalue` field change itself built cleanly with **zero new warnings** (all
10 `-Wint-to-pointer-cast`/`-Wpointer-to-int-cast` warnings N1 originally flagged were
gone after the fix, warning count dropped from 13 to 3 -- the remaining 3 are the
unrelated, already-documented `rms.c` `FILE*` sentinel checks from task 5.4). Despite
that clean build, `EXIT` alone crashed the emulator (SIGSEGV) immediately after this
change -- the two additional `console_include.c`/`console_show.c` sites above produce no
compiler warning at all (they cast through `void*`-adjacent `LONGWORD*` reinterpretation
of a `char**`, which past experience in this file suggested might not always trigger
`-Wint-to-pointer-cast` the way a direct `(LONGWORD)ptr` cast does), so grep-based
"finish the list" coverage would have missed them. Found by bisecting the diff file-by-
file against a working build, then adding a temporary `SIGSEGV`/`SIGBUS`/`SIGABRT`
handler in `driver.c` that prints a `backtrace()` (a normal `lldb`-attached-to-piped-
stdin session wasn't reproducing the crash reliably -- environment-dependent stdin/pty
handling), which pointed straight at `console_include`. All debug scaffolding (the
signal handler, `execinfo.h`/`signal.h`/`unistd.h` includes, and temporary `fprintf`
trace lines in `set_symbol_string_direct`) was removed before finalizing -- none of it
is present in the committed diff.

**Verification**: full rebuild -- 0 errors, 3 warnings (down from 13; all 10 removed
warnings were N1-class, confirming the fix's completeness on the compiler-visible
subset). Interactively: plain boot + `EXIT` (previously crashing, now clean); `evax RUN
put.exe` (the real command-line-argument-to-RUN path, exercising
`CONSOLE$ARG_CMD`/`console_include.c`'s fixed `INCLUDE/COMMAND_LINE` handler
end-to-end) reproduces `put.exe`'s expected output; `SHOW SYMBOL CONSOLE$ARG_CMD` and
`SHOW SYMBOL/SYSTEM` display string symbols correctly (`str="..."`, no crash, no
garbage); `SHOW COMMAND_ARGS` reports clean, correctly-terminated strings after the
buffer-overflow fix; SAVE/LOAD round-trip and all 5 working `.exe` fixtures
(`put`/`putc`/`sieve`/`cli`/`simple`) still behave identically to every prior task's
baseline.

**§5 is now fully closed.** All items (1-8) are done: items 1-6 and 8 with code fixes or
confirmed-not-a-bug resolutions, item 7 (housekeeping) done. Two small, non-urgent
follow-ups remain flagged from task 7 (put1.exe's fate, the `_dcl`-handler return-type
inconsistency); N2 (below) is now also fixed.

### 2026-09-14 — N2 (CVTFL/CVTFW overflow bounds) — done

**Fixed N2** (`Source/CPU/emul_float_math.c`, the `func == 4` float-to-integer
conversion handler shared by `CVTFB`/`CVTFW`/`CVTFL`/`CVTRFL` and their D_floating
counterparts `CVTDB`/`CVTDW`/`CVTDL`/`CVTRDL`): the `switch( dsize2 )` overflow-bounds
check had two of its three cases wrong, not just the one N2 originally flagged.
Cross-checked exact signed ranges against `vax_instr_set.pdf` §8.3 ("Data Types"),
which states them explicitly (Byte -128..+127, Word -32,768..+32,767, Longword
-2,147,483,648..+2,147,483,647):

- **Case 2 (Long)** — N2's original finding — reused the Byte case's `-128.0`/`127.0`
  bounds verbatim, so any value outside that tiny range (essentially any real longword
  conversion) spuriously faulted with `EXC_ARITH`/`TRAP_INT_OVF`. Fixed to
  `-2147483648.0`/`2147483647.0`.
- **Case 1 (Word)** — not in N2's original list, found independently by checking the
  reference manual's exact ranges while fixing case 2 — used `-65536.0`/`65535.0`,
  which doesn't match any correct interpretation (not the signed range, not the
  unsigned range either). Fixed to `-32768.0`/`32767.0`.
- **Case 0 (Byte)** (`-128.0`/`127.0`) was already correct and left unchanged.

Kept the same boundary-check style/idiom (`< MIN || > MAX` against exact integer bounds
as `double` literals, all exactly representable) as the pre-existing, audit-endorsed
Byte case, rather than redesigning the truncation/overflow algorithm's edge-case
precision — that's a separate question this task wasn't asked to open.

**Verification**: full rebuild — 0 errors, 3 warnings (unchanged, the pre-existing
`rms.c` `FILE*` cases). Reproduced N2's exact original repro (`DO MOVL #100,R1` / `DO
CVTLF R1,R2` / `DO CVTFL R2,R3`) — previously faulted, now round-trips cleanly with
`R3 == R1`. Additional targeted tests: `CVTLF`/`CVTFL` round-trip for `^d1000` (still
outside the old buggy `[-128,127]` Long bounds) — clean, no fault. `CVTFW` round-trip
for `^d1000` (fits new correct word bounds) — clean. `CVTFW` on `^d40000` (exceeds the
*correct* word max of 32767, but was *within* the old buggy 65535 ceiling) — now
correctly raises the arithmetic exception, proving the word-bounds fix is real and not
just cosmetically different. `CVTFB` still correctly faults on `^d200` (exceeds byte
range) and round-trips `^d100` cleanly, confirming no regression in the untouched byte
case. All 5 working `.exe` fixtures (`put`/`putc`/`sieve`/`cli`/`simple`) unchanged from
baseline.

**No open items remain from this audit.**
