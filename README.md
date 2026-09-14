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

## What's Next?

Once the basic port is complete and validated, the next major
steps are to continue support for RTL emulation such that basic
`VAX` executables can be loaded and run.

Future objectives:

- Write a functional MACRO assembler tool that can read `VAX`
  ".mar" assembly files and produce a `VAX` ".obj" object file.
- Write a functional LINKER that can assemble the object files
  into a runnable `VAX` ".exe" executable, using knowledge of
  the RTL emulation built into `govax`.
- Be able to run the resulting ".exe" executable files using
  `govax`
  
At this point, it isn't the plan to emulate all hardware (i.e.
disk controllers, network controllers, etc.) or to be able
to boot up VMS. If you want something that can do that, I
recommend starting with the excellent [simh](https://simh.trailing-edge.com)
emulator which can in fact boot up a functioning VMS system.
