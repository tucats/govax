# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

eVAX is a C emulator for a DEC VAX/VMS system: it emulates the VAX CPU, a VMS-like
runtime library/system-service layer, and an interactive console front-end, and
includes its own small MACRO-32-style assembler/disassembler. Originally written in
1997-99 (Forest Edge Software) for 32-bit hosts and ported across many platforms over
the years (Mac, MachTen, LinuxPPC, HP-UX, Windows, Linux/x86, FreeBSD, Alpha/TRU64,
Alpha/VMS — see `eVAX/Headers/arch.h`); now built as a single Xcode target.

## Build

- **Primary build**: open `eVAX.xcodeproj` in Xcode and build the single native target
  `eVAX` (Debug or Release). Note: this sandbox only has the Command Line Tools
  installed, not full Xcode, so `xcodebuild` cannot run here (`error: tool 'xcodebuild'
  requires Xcode`) — building must happen in the Xcode GUI or on a machine with full
  Xcode installed.
- **Manual/CLI build** (verified working; useful for a quick compile-check without
  Xcode):
  ```
  clang -DLINUX86 -I eVAX/Headers -std=gnu99 $(find eVAX/Source -name "*.c") -o /tmp/evax -lm
  ```
  One of `arch.h`'s platform macros (`LINUX86`, `MAC`, `TRU64`, `HPUX`, `WIN`,
  `FREEBSD`, `MACHTEN`, `LINUXPPC`, `VMS`, `IPHONE`) **must** be defined via `-D`, or the
  build fails with `#error Architecture not defined!`. The Xcode project currently
  builds this target with `LINUX86` regardless of host OS
  (`GCC_PREPROCESSOR_DEFINITIONS[arch=*]` in `project.pbxproj`) — match that when
  compiling manually so behavior matches the real build.
  - `CMakeLists.txt` at the repo root is a non-functional stub (`project()` +
    `include_directories()` only, no targets/sources) — not currently usable to build.
  - `.vscode/tasks.json` only builds whatever single file is open in the editor; it's
    not a project-wide build task.
- **Running**: the built binary must be run with its **working directory set to the
  repo root** — at startup it opens `evax.dcl` (DCL grammar), `vax.init` (initial
  commands; sets up a 2MB VAX and initializes VM), and `vax.help` (HELP text) by
  relative filename (`Source/Console/driver.c`), and these live at the repo root.

## Testing

There is no automated test suite. `console_test.c` implements the interactive `TEST`
DCL console command (a grab-bag of low-level, partly-undocumented console hooks), not a
unit-test harness. In practice, verify changes by building, running the emulator
interactively, and exercising it against the fixtures at the repo root: `*.asm` (VAX
assembly, fed to the built-in assembler), `*.exe` (real VMS VAX executables — verified
via `file`; `put1.exe` is a zero-byte file and not currently usable), and
`xdefault.rom`. For a fast sanity check on a single changed file without a full link:
`clang -c -DLINUX86 -I eVAX/Headers -Wall -Wextra <file>.c`.

## Architecture

Everything funnels through **`struct VAX`** (`eVAX/Headers/vax.h`) — the single global
machine-state object (registers, privileged registers, PSL, VM regions, fault/interrupt
queues, plus console- and assembler-specific sub-state). Almost no function passes a
`VAX*` explicitly; code reads/writes the global `vax` instance directly (declared via
the `GLOBALINIT` macro in `vax.h`, defined once in `Source/CPU/vax.c`). `vax.h` is
included (directly or transitively) by essentially every `.c` file and is the header to
read first when re-orienting.

- **`eVAX/Source/CPU/`** — the instruction engine.
  `decode_opcode.c`/`decode_operand.c` walk the VAX variable-length instruction format
  and resolve operands; `storage.c` is the load/store-to-VAX-memory primitive layer
  (`load_memory`/`store_memory`/`load_register`/`get_operand`/`put_operand`) that
  everything else calls through; `vm.c` does virtual→physical address translation
  (page tables, `Headers/pte.h`); `emul_*.c` implement instruction families, one file
  per related group (`emul_mov.c`, `emul_integer_math.c`, `emul_bitfield.c`,
  `emul_call.c` for CALLS/CALLG stack frames, etc.); `fpu.c` handles F/D-floating
  conversion to/from native `double`; `vax.c` is the main fetch-decode-execute loop.
- **`eVAX/Source/RTL/`** — emulates the VMS runtime-library/system-service calling
  convention on top of the CPU. `p1_vector.c`/`shim.c` dispatch system-service calls by
  reading an argument-list vector out of emulated VAX memory; `service.c`, `devices.c`,
  `logical_names.c`, `cli.c` implement individual `SYS$`/`LIB$` services; `rms.c` +
  `structure_mapping.c` implement RMS file I/O by mapping FAB/RAB struct fields
  (`Headers/fab.h`/`rab.h`) onto VAX memory addresses via a declarative offset table
  (see `structure_mapping.c`'s `map()` and the `STROFF` macro in `Headers/memmap.h`).
- **`eVAX/Source/Console/`** — the interactive monitor front-end. `driver.c` is
  `main()`; `parse.c`/`console_dispatch.c` parse and route commands;
  `console_*.c` implement individual commands (EXAMINE, DEPOSIT, RUN, STEP,
  SAVE/LOAD, VMINIT, SHOW, ...); `dclrtl.c` (+ `Headers/dclrtl.h`) is a small
  grammar-driven command-language parser whose grammar is defined in the top-level
  `evax.dcl`; `save_binary.c`/`console_load.c` implement the binary memory-image/ROM/
  NVRAM file formats.
- **`eVAX/Source/Assembler/`** — a MACRO-32-ish assembler/disassembler shared by the
  console's inline ASM/DISASM commands and by operand encoding generally:
  `asm_opcode.c`/`asm_operand.c`/`asm_pseudo.c`/`asm_expr.c`/`asm_value.c` parse;
  `disasm_operand.c` formats for display.
- **`eVAX/Source/Initialization/`** — one-time startup setup (instruction table
  construction, symbol table seeding) run before the console loop starts.
- **`eVAX/Headers/arch.h`** defines per-platform macros and the core integer typedefs
  `LONGWORD`/`ULONGWORD`/`QUADWORD` that stand in for VAX longword/quadword values
  throughout the codebase — see the critical issue below before changing anything that
  touches these types.

## Reference material

`~/Documents/Technical Doc/VMS/vax_instr_set.pdf` is a VAX architecture/instruction-set
reference manual (assembly language, opcodes, addressing modes) covering the ISA this
emulator implements — incompletely, per the user. Consult it when verifying or fixing
instruction semantics in `Source/CPU/emul_*.c` or the assembler in
`Source/Assembler/`.

## Known critical issue — read AUDIT.md first

`LONGWORD`/`ULONGWORD` are meant to be a fixed **32-bit** VAX longword type, but on the
current build they resolve to native `long`/`unsigned long` — **8 bytes** on this
64-bit host — because the `LINUX86` branch of `arch.h` used by this project's Xcode
target never defines the `HAS64BITLONGS` escape hatch the original author added in
1999 for exactly this problem. This one typedef is the root cause of most bugs
catalogued in `AUDIT.md`: wrong signed arithmetic/condition codes, broken quadword
operand handling, out-of-bounds stack access in `fpu.c`'s float conversion, corrupted
RTL argument marshaling, and a ROM-file loader that's demonstrably broken against the
repo's own `xdefault.rom` fixture. It produces no compiler warning or error — the
project compiles and links cleanly.

**Read `AUDIT.md` before starting any bug-fix work here** — it has full findings with
severities and file/line references, and a suggested fix order. Update it (mark
findings fixed, or add new ones) as that work proceeds, rather than letting it go stale.

## Gotchas

- Many source files use classic Mac OS **CR-only line endings** (no LF) — a holdover
  from the original 1997-99 Mac development. Standard line-oriented tools (`wc -l`,
  `grep -n`) misreport these files (often as a single line); normalize a working copy
  (`tr '\r' '\n'`) before trusting line numbers.
- Git history here starts fresh — this project previously used CVS (metadata removed)
  and had no `.git` until recently. `.gitignore` excludes Xcode derived-data/index
  caches that have previously accumulated to ~270MB inside `eVAX/eVAX/`
  (`*.noindex` directories, hashed `eVAX-<hash>/` index folders, a stray `Debug/`
  build-output directory); if similar directories reappear after opening the project
  in Xcode, they're safe to delete — they're pure build/index byproducts.
