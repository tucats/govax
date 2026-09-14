# Project Plan

This document describes the *high-level* plan for the project, including project phases created
through pre-planning, and progress logging.

This is a conversion (from C to Go) of the VAX emulator project currently hosted on the local
development system at /Users/tom/Documents/Projects/eVAX. This is not a cross-compilation, but
a careful evaluation of the C version of the emulator followed by a rewriting it from scratch
as Go code, using the benefits of the Go language to implement features constructed in the
reference system using C mechanisms like macros and other C language constructs.

This project will then take advantage of Go's superior ability to have integrated testing to
create a comprehensive suite of unit tests for each of the main components of the project:

- CPU hardware definition
- Virtual memory support
- VAX instruction set emulation
- Console functionality
- Integerated I/O capabilityies
- Runtime Library (RTL) simulators

