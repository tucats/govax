# govax

VAX emulator, written in Go

This is a translation (not cross-compile) of the `evax` project on
[github](https://github.com/tucats/evax), originaly written in C.

The original C code was written entirely by Tom Cole as a hobby
project starting in the late 90's when `VAX` was still _slight_
cool, though Compaq was working to replace it with the `Alpha`
architecture aggressively.

This port to Go was intiailly completed entirely by Claude Code
using the Sonnet 5 model, with direction from the developer. This
port had the following objectives:

- Convert to a modern language with more expressive idioms that
  would make for code that was easier to read, understand, and
  modify.

- Use a toolchain with rich unit testing capabilities to be able
  to incrementally validate the work as the port happened, and to
  extend as further features are added in the future.

- The conversion also identified basic logic and coding errors
  in the original C code that were corrected on-the-fly during
  the port to Go.

- A written-from-scratch support for the `VAX` floating-point
  datatypes (d-float and f-float) which are different from the
  modern IEEE floating point data types.

- The port task was given access to a `VAX` instruction set
  manual, and identified obvious errors in the C implementation
  compared to the ISA and architecture descriptions. This was
  most commonly found in handling the `VAX` `PSL` register bits
  as well as exception handling during certain instruction decode
  faults.

- Introduction of more modern console features, such as using
  a proper `readline` library for command line input, etc.

## Project Plan

The [PLAN](docs/PLAN.md) file describes the overall porting plan,
the parameters given to Claude Code for the port, and a breakdown
of each of the major phases of the port. Each phase is documented
in much more detail in the [docs](docs/) directory, with each
phase describing it's sub-tasks, progress details, and issues
that were found. Finally, the [deviations](docs/DEVIATIONS.md)
file describes deviations from the machine architecture or ISA
specifications found and either addressed or left outstanding
from the port process.

## Current status

The port covers the full stack the original objectives called for: CPU
instruction set (including both VAX floating-point formats), virtual
memory, an interactive console with a DCL-style command language, RTL/
system-service simulation, a MACRO-32-style assembler/disassembler, and
real VMS image activation (`RUN` loads and executes the project's own
`.exe` test fixtures end to end, resolving sharable-image dependencies and
applying load-time fixups). Every phase in [PLAN](docs/PLAN.md)'s table is
built and covered by its own unit tests; Phase 12 added a fixture-driven
regression suite that assembles and runs every `testdata/asm/*.asm`
program and every real `testdata/exe/*.exe` binary the project ships,
alongside the ROM/NVRAM save-and-load round trip.

Known gaps, tracked in their own docs rather than silently left unstated:

- **Interrupt delivery.** The interval timer (ICCS/NICR/ICR) and console
  TTY (TXCS/TXDB/RXCS/RXDB) privileged registers have their bit-level
  semantics but no actual interrupt admission/delivery behind them yet, so
  a real VMS-style program that polls a device-ready flag in a loop (as
  the project's own microkernel, `kernel.asm`, does for console output)
  will spin rather than complete. See
  [PHASE-14.md](docs/PHASE-14.md) (not started — a planning placeholder).
- **A handful of deliberately-kept ISA judgment calls** — places where the
  VAX architecture manual itself is ambiguous, or where the original
  author's own C comments flagged something as unresolved — documented
  with their reasoning in [DEVIATIONS.md](docs/DEVIATIONS.md) rather than
  guessed at.

## What's next?

With the assembler, RTL, and image loader all in place, the natural next
step is Phase 14 (interrupt delivery) — it's what stands between the
current state and a real microkernel program running to completion rather
than a bounded, documented stop. Beyond that, this project was never aiming
to emulate real hardware (disk controllers, network controllers, etc.) or
boot an unmodified VMS distribution — for that, the excellent
[simh](https://simh.trailing-edge.com) emulator can actually boot a
functioning VMS system.
