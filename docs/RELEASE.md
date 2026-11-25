# govax Release Notes

Releases are listed newest first.

---

# govax v1.1 — "A STAR Is Born"

DIGITAL's engineers called the first VAX, the VAX-11/780, "STAR" while
they were building it. The name fits this release. Version 1.0 was a VAX
processor: a faithful instruction set, memory management, and a console
where you could assemble a few instructions and step through them. Version
1.1 is what grew around that processor. A VAX program running under govax
now sees **VMS**, from the user's side of the terminal: a DCL prompt, files
on Files-11 disks, logical names, processes, a run-time library, compilers,
a linker, a debugger, accounts, and batch jobs.

These notes describe what to expect. The planning documents in
`docs/planning/` record each piece in detail, and the user's manual,
`docs/MANUAL.md`, tells you how to use it.

## What it feels like now

Start govax and you get a `$` prompt. The first time it runs, govax makes
itself a system disk (`~/.govax/govax-system.dsk`, mounted as `DUA0:`)
holding `STARTUP.COM`, the help libraries, the system macro library, the
run-time libraries, and their sources. A startup procedure runs, then your
login procedure, and you're at DCL, logged in as `SYSTEM`, or as any
account you've made.

From there it should feel familiar to anyone who used VMS:

```text
$ CREATE/DIRECTORY [.WORK]
$ SET DEFAULT [.WORK]
$ FORTRAN HELLO
$ MACRO HELLO
$ LINK HELLO
$ RUN HELLO
Hello from a VAX that isn't there
$ DIRECTORY/SIZE
```

Every one of those commands is govax's own, written from DIGITAL's
manuals. None of it comes from VMS. Where the manuals didn't settle a
question, the answer came from running the same thing on real VMS 7.1 or
7.3, under SIMH, and govax's test suite replays those runs to keep it
honest.

## The big pieces

### DCL, the command language

The console is a real DCL interpreter. Command procedures run with `@`,
with parameters P1 to P8, command levels and local symbols, symbol
substitution, expressions, `$STATUS` and `ON`, labels and `GOTO`, block
`IF`, `GOSUB`, `CALL`/`SUBROUTINE`, and the lexical functions. `OPEN`,
`READ`, `WRITE`, and `INQUIRE` work on files. `SET VERIFY`, Ctrl/Y, `STOP`,
`SPAWN`, `ATTACH`, and `WAIT` all behave as VMS's do. The file commands
(`DIRECTORY`, `TYPE`, `COPY`, `RENAME`, `DELETE`, `PURGE`, `DUMP`,
`DIFFERENCES`, `CREATE`, `EDIT`) and the `SHOW` commands lay out their
output the way VMS does. `HELP` is VMS help, read from `.HLB` help
libraries and organized the same way.

### Files and RMS

RMS reads and writes real ODS-2 volumes: the same container files SIMH
uses, so you can move files between govax and a real VMS system. Logical
names are VMS's, with tables, search lists, concealed devices, and rooted
directories. Sequential, relative, and indexed files are laid out on disk
exactly as VMS lays them out, so VMS's own `ANALYZE/RMS_FILE` accepts
govax's indexed files. Processes can share files with record locking.
`BACKUP` reads and writes VMS save sets, `CONVERT` and `CREATE/FDL` build
files from FDL, and RAM disks (`MDA0:`) give you fast scratch space.

### Programs and processes

Several processes run at once under VMS's scheduling rules: subprocesses,
detached processes, mailboxes, common event flags, global sections, and a
lock manager. A VMS paging system is available as an option, with working
sets, the free and modified page lists, a page file, and images that page
in from their files. You can watch it from `SHOW MEMORY` and
`SHOW WORKING_SET`.

### The run-time library

About 170 `LIB$` routines are here (strings, descriptors, conversions,
dates and times, memory zones, condition handling, file searches, the CLI
routines, `LIB$SPAWN`), along with the whole mathematics library (`MTH$`
and the `OTS$` powers), much of `STR$`, the librarian routines (`LBR$`),
and the security services. All of them are written in Go and reached from
VAX code exactly as the real libraries are. An unhandled error prints a
proper `%TRACE` traceback.

### Building programs

- **MACRO-32** assembles to VMS object modules, with macros, listings, and
  cross references. govax's own `STARLET.MLB` is written from the manuals.
- **LINK** builds executable and shareable images: transfer vectors,
  `GSMATCH`, options files, clusters, maps, and `/DEBUG`.
- **LIBRARY** maintains object, macro, text, help, and shareable-image
  libraries.
- **ANALYZE** reports on objects, images, and RMS files.

These tools write their output byte for byte as VMS's do. Objects and
images built by govax run on a real VAX, and objects and images built on a
VAX run under govax.

### FORTRAN

This is the biggest single addition. govax has its own FORTRAN compiler.
It reads **VAX FORTRAN** (FORTRAN-77 with DEC's extensions: structures and
records, `INCLUDE`, NAMELIST, indexed and relative file I/O, the system
subroutines), **Fortran 90** (free form, modules, derived types, array
syntax, allocatable arrays, pointers, interfaces and generics, recursion,
nonadvancing I/O), and the **Fortran 95** additions (`FORALL`, `PURE` and
`ELEMENTAL`, `NULL()`, default initialization).

The compiler writes annotated MACRO-32, which MACRO and LINK turn into an
image. If you want to see how a compiler targets the VAX, you can read
every instruction it produced next to the line of source that produced it.
It has an optimizer, listings, cross references, `/CHECK`, `/STANDARD`,
and `/DEBUG`. Its run-time library, FTNRTL, is itself written in MACRO-32
and built by govax's own tools. Two milestones prove it out: the classic
**DUNGEON** adventure game (local only, never shipped) plays move for move
as it does on a VAX, and **FCALC**, a scientific calculator of about
11,000 lines of Fortran 90, ships on the system disk. Type `FCALC` at the
prompt to try it.

### The debugger

A debugger modeled on the VMS debugger starts with `RUN/DEBUG` and gives
you a `DBG>` prompt. It provides breakpoints, tracepoints, watchpoints,
`STEP`, `EXAMINE`/`DEPOSIT` with symbolic disassembly, `SHOW CALLS`, and
`SET SCOPE`. It reads MACRO and FORTRAN symbols, so you can examine
FORTRAN arrays, records, and pointers by name.

### Users, security, and batch

govax keeps an authorization file. `AUTHORIZE` manages it, `ADDUSER.COM`
makes an account, and `govax --user NAME` logs in as one. Owners,
protection codes, ACLs, and privileges are checked on every access, so you
can see what an unprivileged user is refused. You can also restrict access
to host files with the SYSPRV privilege if you want to. Batch queues work
too: `SUBMIT` runs a job as a detached process with a log file, and queues
and jobs persist from one govax session to the next.

## Fidelity, and how it's checked

The goal throughout has been "the same as VMS," and that claim is tested.
Most subsystems have a probe: a program or procedure that was run on real
VMS under SIMH, with VMS's output committed under `testdata/`. Oracle tests
then run the same thing on govax and compare the results line by line, and
sometimes byte by byte. Where govax still knowingly differs from VMS,
`docs/DEVIATIONS.md` says so.

The project is clean room. govax contains no DIGITAL, Compaq, HP, or VSI
code or files, and needs none to run.

## What it still isn't

govax doesn't boot VMS and doesn't emulate disk controllers or terminal
hardware. Use SIMH for that. It has no networking, no print queues, no
clusters, and no compilers besides MACRO-32 and FORTRAN. It's a
single-user VMS for programs, which is enough to write, run, study, and
debug them, and to learn how VMS worked underneath.

## Where to read more

- `README.md`: installing and a first program.
- `docs/MANUAL.md`: the user's manual, covering every command, MACRO,
  LINK, the debugger, and FORTRAN in chapter 14.
- `docs/VAX-VMS-INTERNALS.md`: a tour of how govax implements a VAX and
  presents VMS to a program.
- `docs/FORTRAN-INTERNALS.md`: a tour of the compiler and FTNRTL, for
  anyone who wants to write a compiler for the VAX.
- `docs/planning/PLAN.md`: the phase-by-phase history behind all of the
  above.

---

# govax v1.0 "Just Getting STARted"

This is the first official release of govax!

- This is a port of the C version (named eVAX)
- Lots of fixes to improve emulator fidelity to the VAX instruction set
  architecture (ISA)
- Initial support for VAX/VMS RMS operations against Files-11 disk
  container images (the same containers used by `simh`, for example).
