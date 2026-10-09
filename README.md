# govax

A VAX emulator, written in Go

This is a translation (not cross-compile) of the `evax` project on
[github](https://github.com/tucats/evax), originaly written in C.
The original C code was written by Tom Cole as a hobby
project starting in the late 90's when `VAX` was still _slightly_
cool, though Digital/Compaq were already working hard to replace it with
the new hot-ness of the `Alpha` architecture.

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
  the port to Go, as well as some deviations from the VAX ISA
  that had never been found in the C code.

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

The [docs](docs/) folder also contains multiple documents that
describe each sequential phase of the port, including adding new
features and updating the underlying code to strip out some of
the artifacts from once having run on a MacOS 7 in the 1990's...

## Current status

This isn't a complete set of featurees, but generally covers what's
been do so far with `govax`:

- CPU instruction fidelity (including VAX floating point, packed decimal,
  and octaword instructions).
- Virtual memory support (including $CRMPSC sections). There is currently
  no pager support outside mapped sections.
- Support for RMS services and ODS2 Files-11 container disks, as well as
  limited RMS support for accessing native files.
- Support for VMS-style processes, and system servcies for managing
  processes and inter-process communication.
- MACRO32 and LINK command support, allowing first-class VAX macro programs
  to be compiled, linked, and run.
- Clean-room implementation of STARLET.MLB and related files, so VAX macro
  programs can be run using VMS-style macro invocations.
- Debugger support for GST/TBT records in images, so debugging is familiar
  to a VAX/VMS user.

## Optional VMS files

govax builds and runs from this repository alone. It contains no files
that DIGITAL, Compaq, HP, or VSI hold copyright on. Where govax needs VMS
definitions, such as status codes, message texts, and the entry points of
the run-time library, it uses its own tables of those values. These are in
`internal/vmsdef`.

If you have a VAX/VMS system or distribution, a few of its files make
`MACRO` and `LINK` behave exactly as they do on VMS. Copy them (with
`COPY/BINARY`, or from a Files-11 container) into a directory, and point
govax at it:

```sh
govax config set vax.library=/path/to/vms/files
```

Names may be upper or lower case. On a mounted volume, govax also finds
these files through the `SYS$LIBRARY` and `SYS$SHARE` logical names, as VMS
does.

| File | Used by | Without it |
| ---- | ------- | ---------- |
| `STARLET.MLB` | `MACRO`: the system macros (`$FAB`, `$RAB`, `$QIOW_S`, ...) | govax's own STARLET, written from DIGITAL's manuals and checked against real VAX MACRO's output: the RMS macros (`$FAB`, `$RAB`, `$NAM`, the XABs, their `_STORE` forms, and `$OPEN`, `$GET`, `$PUT`, ...), the `$xxxDEF` definition macros (`$FABDEF`, `$SSDEF`, `$IODEF`, ...), and a few system service macros (`$ASSIGN_S`, `$DASSGN_S`, `$EXIT_S`, `$QIO_S`, `$QIOW_S`). Other system macros need the real one. |
| `IMAGELIB.OLB` | `LINK`: which shareable image defines each routine | govax's tables of LIBRTL's routines, and the routines govax's shims stand for |
| `LIBRTL.EXE` (and other shareable images) | `LINK`: routine offsets. `RUN`: the routines themselves | Linking works for every LIBRTL routine. Running one needs the image, or a govax shim. |
| `STARLET.OLB` | `LINK`: the system library's modules | govax's tables of STARLET's status codes and other definitions (SS$_, RMS$_, IO$_, ...), and its system-service vector. Routines STARLET holds as code (BAS$, MTH$, ...) need the real library. |

## What's next?

Here's a general list of next tasks:

- Addition of a virtual-memory pager, so processes support working sets properly.
- Support for supervisor-mode console command handling, to mimic how VAX/VMS
  maintains a DCL shell in P1 space.
- More accurate device operations and reference counting
- More complete DCL support (IF statement, lexicals)
- ANALYZE/DISK/REPAIR for ODS2 container volumes

Beyond that, this project was never aiming to emulate real hardware
(disk controllers, network controllers, etc.) or boot an unmodified
VAX/VMS distribution — for that, the excellent
[simh](https://simh.trailing-edge.com) emulator can be used today to
actually boot a functioning VAX/VMS system.
