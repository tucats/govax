# govax User's Manual

govax is a VAX/VMS environment that runs on your own computer. It runs
VAX machine code, provides the VMS system services and run-time library
that VAX programs call, and gives you a DCL command line with the tools
to build programs: a MACRO-32 assembler, a linker, a librarian, and a
debugger modeled on the VMS debugger.

This manual is for people who know VMS as users (DCL, file
specifications, logical names, `RUN`, `MACRO`, `LINK`) and want to run
existing VAX programs, or write new ones, without a VAX. It doesn't
assume you know VAX system internals; the last part of the manual
explains as much of them as you need to understand what govax is doing.

The examples in this manual were run on govax, and the output shown is
what govax printed (sometimes shortened, where `...` shows). Dates,
times, node names, and version numbers will differ on your system.

## Contents

1. [What govax is, and what it isn't](#1-what-govax-is-and-what-it-isnt)
2. [Getting started](#2-getting-started)
3. [The govax command line](#3-the-govax-command-line)
4. [Configuration](#4-configuration)
5. [Files, volumes, and devices](#5-files-volumes-and-devices)
6. [DCL command reference](#6-dcl-command-reference)
7. [Command procedures and the DCL language](#7-command-procedures-and-the-dcl-language)
8. [Programming in MACRO-32](#8-programming-in-macro-32)
9. [The linker](#9-the-linker)
10. [The librarian](#10-the-librarian)
11. [Running programs](#11-running-programs)
12. [The debugger](#12-the-debugger)
13. [Internals](#13-internals)
14. [Programming in FORTRAN](#14-programming-in-fortran)
15. [Appendices](#15-appendices)

---

## 1. What govax is, and what it isn't

### What it is

govax emulates a VAX processor and, around it, enough of VMS for VAX
programs to run as they would under VMS:

- **The VAX instruction set, complete.** Every VAX instruction is
  implemented, including the F, D, G, and H floating-point formats (with
  VAX rounding, not IEEE), packed decimal, the string and character
  instructions, the queue instructions, and the octaword instructions.
  Memory management, access modes, exceptions, and interrupts follow the
  VAX architecture.
- **VMS system services.** Over ninety `SYS$` services: I/O (`$QIO`,
  `$ASSIGN`), logical names, time, event flags, ASTs, condition handling,
  processes (`$CREPRC`, `$HIBER`, `$WAKE`), mailboxes, global sections,
  the lock manager (`$ENQ`, `$DEQ`), memory (`$EXPREG`, `$CRETVA`,
  `$CRMPSC`), and information (`$GETJPI`, `$GETSYI`, `$GETDVI`). Appendix
  A lists them.
- **RMS**, on real Files-11 (ODS-2) disk volumes: `$OPEN`, `$CREATE`,
  `$CONNECT`, `$GET`, `$PUT`, `$UPDATE`, `$FIND`, `$DELETE`, `$PARSE`,
  `$SEARCH`, and the rest, with FABs, RABs, NAM blocks, and XABs, file
  sharing between processes, and record locks.
- **The run-time library.** About 170 `LIB$` routines (strings,
  descriptors, conversions, dates and times, memory zones, condition
  handling, file searches, the CLI routines, `LIB$SPAWN`), and the whole
  mathematics library, `MTH$` and its `OTS$` powers, written in Go
  and reached from VAX code exactly as the real LIBRTL is reached.
- **Processes.** Several VMS processes run at once, scheduled by VMS's
  priority rules: subprocesses, detached processes, mailboxes between
  them, shared files, common event flags.
- **Optional VMS paging.** With paging turned on, govax runs VMS's
  memory management: working sets, page faults, the free and modified
  page lists, a page file, and images mapped from their files.
- **A DCL command interpreter.** Command procedures, symbols,
  expressions, lexical functions, `IF`/`THEN`/`ELSE`, `GOSUB`, `CALL`,
  `ON`, file I/O with `OPEN`/`READ`/`WRITE`, `SPAWN`, and the usual file
  commands (`DIRECTORY`, `COPY`, `TYPE`, `DELETE`, `RENAME`, ...).
- **BACKUP save sets.** `BACKUP` lists, restores, compares, and writes
  VMS save sets, on a volume or on the host. Save sets made on a VAX
  restore in govax with every file attribute, and VMS restores govax's.
- **Program development tools.** `MACRO` assembles MACRO-32 source into
  VMS object modules. `LINK` links them into VMS executable and
  shareable images. `LIBRARY` makes and maintains object, macro, and
  shareable image libraries. `ANALYZE/OBJECT` and `ANALYZE/IMAGE`
  describe object and image files. The debugger debugs programs at the
  source line and instruction level.

These are not loose imitations. The objects govax's `MACRO` writes, the
images its `LINK` writes, and the libraries its `LIBRARY` writes are laid
out byte for byte as VMS's tools lay them out, so they work on a real VAX,
and VAX-built objects, libraries, and images work in govax. Most of the
system's behavior (message texts, statuses, DCL's rules, the layout of
`DIRECTORY` and `SHOW` output, the debugger's output) has been checked
against VMS 7.1 and 7.3 running on a VAX emulator, and govax's test suite
replays those VMS runs to keep it that way.

### What it isn't

- **It isn't a hardware emulator that boots VMS.** govax doesn't emulate
  disk controllers, terminal multiplexers, or a console boot ROM, and it
  can't boot a VMS distribution. If you want to run the real VMS
  operating system, use [SIMH](https://simh.trailing-edge.com), which
  does that very well. govax is VMS-compatible from the program's point
  of view: its "operating system" is a small microkernel plus Go code
  that behaves as VMS's services do.
- **It doesn't include VMS's compilers or utilities.** There is no C,
  BASIC, or Pascal compiler; MACRO-32 is the language you can build
  programs in, and govax's own FORTRAN compiler, which writes MACRO-32,
  is being built ([14. Programming in FORTRAN](#14-programming-in-fortran)).
  Images built by those compilers on a real VAX will
  run if every run-time library routine they call is one govax provides
  (see [Running programs built on VMS](#running-programs-built-on-vms)).
- **It contains no DIGITAL, Compaq, HP, or VSI code or files.** govax was
  written clean-room, from DIGITAL's manuals and from the output real VMS
  produced. Its system macro library (`STARLET.MLB`) and its tables of
  VMS symbols are its own. If you have VMS's own library files you can
  point govax at them (see [`vax.library`](#files-and-libraries)), but
  you don't need them.
- **It is a single-user system.** You are the user `SYSTEM`, with all
  privileges. There is no login, no user authorization file, no batch
  or print queues, no networking (DECnet or TCP/IP), and no cluster.
- **Some things VMS does are missing or simplified.** For example,
  there is no `INSTALL`, and there is no swapping. Where
  govax knowingly differs from VMS, this manual says so, and
  `docs/DEVIATIONS.md` keeps the detailed list.

### Who it's for

- You have VAX programs (MACRO-32 sources, or images and objects from a
  VAX) and want to run, study, or debug them.
- You want to learn or teach VAX assembly language and VMS programming
  with a fast edit-assemble-run cycle on a modern machine.
- You're curious how VMS works inside: govax lets you watch page faults,
  working sets, the scheduler, and system services from the debugger.

---

## 2. Getting started

### Building govax

govax is written in Go. With Go installed (1.26 or later), from a clone
of the repository:

```sh
go build ./cmd/govax          # makes ./govax
go install ./cmd/govax        # or installs govax in your Go bin directory
```

govax needs no other files: the files it reads at startup (its command
grammar, help text, startup procedure, and microkernel) are built into the
executable. govax runs on macOS, Linux, and Windows.

### Your first session

Type `govax` with no arguments to get the console's prompt:

```text
$ govax
govax 1.0-280
%SYSTEM-S-INIT, Kernel initialized

VAX>
```

Before the prompt appears, govax runs its startup procedure, `vax.init`
(see [What happens at startup](#what-happens-at-startup)): it configures
8 MB of memory, builds the page tables, loads and starts the microkernel,
and defines the terminal and null devices. `%SYSTEM-S-INIT` says that has
been done.

At the `VAX>` prompt you type DCL commands, as at VMS's `$` prompt. Some
things to try:

```text
VAX> SHOW SYSTEM
GOVAX 1.0-280  on node mymac  20-OCT-2026 09:56:01.44  Uptime  0 00:00:00
  Pid    Process Name    State  Pri      I/O       CPU       Page flts  Pages
00000301 SYSTEM          CUR      4        0   0 00:00:00.04         7      7
VAX> X = 6 * 7
VAX> SHOW SYMBOL X
  X = 42   Hex = 0000002A  Octal = 00000000052
VAX> WRITE SYS$OUTPUT "Answer: ", X, " at ", F$TIME()
Answer: 42 at 20-OCT-2026 09:56:01.44
VAX> HELP
```

`HELP` lists every command and topic; `HELP DIRECTORY`, `HELP LINK`,
`HELP LEXICAL`, and so on describe each one in detail. The help text and
this manual cover the same ground; the help text is the place to look up
a qualifier quickly.

To leave govax, type `EXIT` (or `QUIT`, or `LOGOUT`) at the `VAX>`
prompt, or press **Ctrl/Y**.

The prompt has a line editor: the arrow keys move through the line and
through earlier commands, which govax keeps between sessions in
`~/.govax/history.txt`. The debugger's `DBG>` and the assembler's `ASM>`
prompts recall their own commands, kept in `debug_history.txt` and
`asm_history.txt` beside it, so the console's history holds only
console commands.

### Hello, world

Here is the classic first program, in MACRO-32. Put it in a file called
`hello.mar` in your current directory:

```asm
        .TITLE  HELLO   Hello, world
        .IDENT  /V1.0/

        .PSECT  DATA, WRT, NOEXE, LONG
MSG:    .ASCID  /Hello, world!/         ; a string descriptor

        .PSECT  CODE, NOWRT, EXE, LONG
        .ENTRY  HELLO, ^M<>             ; the main routine
        PUSHAQ  MSG                     ; LIB$PUT_OUTPUT(MSG)
        CALLS   #1, G^LIB$PUT_OUTPUT
        RET                             ; R0 is its status: success

        .END    HELLO                   ; HELLO is the transfer address
```

Assemble, link, and run it, either from your host shell:

```text
$ govax macro hello.mar
$ govax link hello.obj
$ govax run hello.exe
Hello, world!
```

or at the `VAX>` prompt:

```text
VAX> MACRO "hello.mar"
VAX> LINK "hello"
VAX> RUN "hello.exe"
Hello, world!
```

Notice the quotes at the `VAX>` prompt. DCL makes everything you type
uppercase unless it's in quotes, and your host's file names are often
lowercase (and on Linux, case-sensitive). Section 5 explains how govax
decides whether a name means a host file or a file on a VAX disk.

### Working on a VAX disk

You can keep your work in host files, as above, but VAX programs that
use RMS (`$OPEN`, `$GET`, `$PUT`, ...) read and write files on Files-11
disk volumes, and VMS-style file names, versions, and directories only
exist there. govax keeps a Files-11 volume in a single host file, called
a *container*. The easiest way to get one is to let govax make it for
you each time it starts:

```sh
govax config set vax.default.volume.file="~/vax/work.dsk"
```

The next time govax starts, it creates the container (the first time
only), mounts it as `DUA0:`, and sets your default directory to
`DUA0:[WORK]`:

```text
$ govax
govax 1.0-280
%MOUNT-I-CREATED, container /Users/me/vax/work.dsk created as an RD54 volume labeled WORK
%MOUNT-I-MOUNTED, WORK mounted on _DUA0:
%SYSTEM-S-INIT, Kernel initialized

VAX> SHOW DEFAULT
  DUA0:[WORK]
VAX> COPY "hello.mar"/HOST *
%COPY-S-COPIED, hello.mar copied to DUA0:[WORK]HELLO.MAR;1
VAX> MACRO HELLO
VAX> LINK HELLO
VAX> RUN HELLO
Hello, world!
VAX> DIRECTORY

Directory DUA0:[WORK]

HELLO.EXE;1         HELLO.MAR;1         HELLO.OBJ;1

Total of 3 files.
```

Now names work as they do on VMS: `HELLO` means `DUA0:[WORK]HELLO.MAR`
to `MACRO` and `DUA0:[WORK]HELLO.EXE` to `RUN`, and each new file is a
new version. `COPY ... /HOST` moves files between your host and the
volume in either direction. The volume is an ordinary ODS-2 disk image;
SIMH and other VAX emulators can mount it too.

### What happens at startup

Each time govax starts, it:

1. Mounts the containers named by `--mount` and `--mount-write` options.
2. Mounts the default volume, if `vax.default.volume.file` is set.
3. Runs `vax.init`, govax's equivalent of a system startup and login
   procedure. It is a DCL command procedure that:
   - sets the size of physical memory (`INITIALIZE/VAX`),
   - builds the page tables for the P0, P1, and S0 address regions and
     the access modes' stacks (`VMINIT`),
   - loads a console ROM and NVRAM if there are any (`LOAD/ROM/NOERROR`),
   - assembles the microkernel into memory (`ASM "kernel.asm"`) and runs
     its initialization under the debugger,
   - defines the terminal `TTA0:` and the null device `NLA0:`.
4. Installs the page file, if paging is on.
5. Runs your own startup procedure, if `vax.startup.procedure` names one.
6. Runs the command given on govax's command line and exits, or else
   shows the `VAX>` prompt.

The memory sizes `vax.init` uses can be changed with configuration
settings (see [Memory](#memory)), so you rarely need a `vax.init` of your
own. If you do, put one in a directory and give it to govax with `--path`
(see [Global options](#global-options)).

---

## 3. The govax command line

### Forms

```text
govax [global-options]                          interactive session
govax [global-options] command [args...]        one DCL command, then exit
govax [global-options] subcommand [options] [args...]
```

With no command, govax starts an interactive session. Anything else on
the command line that isn't one of govax's subcommands is a DCL command:
govax starts, runs `vax.init`, runs that one command, and exits.

```text
$ govax show system
GOVAX 1.0-280  on node mymac  20-OCT-2026 10:10:37.70  Uptime  0 00:00:00
  Pid    Process Name    State  Pri      I/O       CPU       Page flts  Pages
00000301 SYSTEM          CUR      4        0   0 00:00:00.01         6      6
$ govax console 'write sys$output f$getsyi("HW_NAME")'
VAX (govax)
```

(`console` is the name of that default subcommand; you only need to type
it when the DCL command's verb is the same as a govax subcommand's.)

Remember that your host shell sees the command first: quote anything
with `$`, quotes, brackets, or other characters your shell treats
specially, as in the second example.

### Subcommands

The subcommands are shortcuts for DCL commands, with host-style options,
for use from scripts and makefiles. Each one quotes its file names for
you, so their case is kept.

| Subcommand | DCL command it runs | Purpose |
| --- | --- | --- |
| `govax console CMD...` | `CMD` | Run one DCL command (the default). |
| `govax run IMAGE [text...]` | `RUN IMAGE` | Run an image. Any words after the image's name are its command text, which it reads with `LIB$GET_FOREIGN`. |
| `govax fortran SOURCE...` | `FORTRAN SOURCE,...` | Compile FORTRAN source files into MACRO-32 source files. |
| `govax macro SOURCE` | `MACRO SOURCE` | Assemble a MACRO-32 source file. |
| `govax link OBJECT...` | `LINK OBJECT,...` | Link object modules into an image. |
| `govax library LIB [INPUT...]` | `LIBRARY LIB INPUT,...` | Create, change, list, or extract from a library. |
| `govax analyze FILE...` | `ANALYZE/OBJECT` or `/IMAGE` | Describe object or image files. |
| `govax config ...` | | View and change the configuration (section 4). |
| `govax help` | | Show the command-line help. |
| `govax version` | | Show govax's version. |

Each subcommand's options mirror the DCL qualifiers; `govax SUBCOMMAND
--help` lists them:

```text
govax run [--debug | --no-debug] IMAGE [text...]

govax fortran [--macro FILE | --no-macro] [--extend-source] [--d-lines]
              [--continuations N] [--no-i4] [--g-floating] SOURCE...

govax macro [--object FILE | --no-object] [--list | --list-file FILE]
            [--show OPTS] [--no-show OPTS] [--cross-reference]
            [--cross-reference-kinds KINDS] [--enable FUNCS]
            [--disable FUNCS] [--debug OPTS | --no-debug]
            [--library MLB]... SOURCE

govax link [--executable FILE | --no-executable] [--shareable]
           [--symbol-table FILE] [--debug] [--no-traceback]
           [--no-syslib] [--no-userlibrary]
           [--map | --map-file FILE] [--brief | --full]
           [--library OLB]... [--options OPT]... OBJECT...

govax library [--create] [--insert | --replace] [--delete MODS]
              [--extract MODS] [--output FILE] [--list | --list-file FILE]
              [--full] [--names] [--macro | --object | --share]
              [--no-squeeze] [--selective-search] [--log]
              LIBRARY [INPUT...]

govax analyze [--image [--header] [--fixup-section]]
              [--mhd] [--gsd] [--tir] [--tbt] [--dbg] [--lnk] [--eom]
              [--include MODS] [--output | --output-file FILE] FILE...
```

For example:

```text
$ govax macro --list --cross-reference prog.mar
$ govax link --map --full --options prog.opt prog.obj
$ govax run echo.exe Hello there, '"Mixed Case"'
Hello there, "Mixed Case"
```

### Global options

Global options go before the subcommand or command.

| Option | Meaning |
| --- | --- |
| `--mount DEV=FILE`, `-m` | Mount the container `FILE` read-only on device `DEV` before anything else. May be repeated. |
| `--mount-write DEV=FILE` | Mount it for reading and writing. May be repeated. |
| `--path DIR,...` | Host directories to search for files govax and its commands name (its startup files, `@` procedures, MACRO sources, ...), after the name as given. govax's built-in copies of its startup files are always searched last. |
| `--profile NAME` | Use the configuration profile `NAME` instead of the default one (section 4). |
| `--instruction-limit N`, `-i` | Stop a program after it has executed `N` VAX instructions. Each run (a `RUN`, or the debugger's `GO`, `CALL`, or `STEP`) starts with a fresh budget. |
| `--time-limit D`, `-t` | Stop a run after the elapsed time `D` (`10s`, `2m`, ...). |
| `--stats`, `-s` | When govax exits, show statistics: instructions executed, memory accesses, translation buffer use, and (with paging on) the paging counts. |
| `--cpu-profile FILE` | Write a Go CPU profile of govax itself (for govax's developers; see `docs/PERFORMANCE.md`). |
| `--version`, `-v` | Show the version. |
| `--help`, `-h` | Show help. |

The limits apply only to your commands, not to `vax.init`. They are a
safety net for a program that loops forever when govax runs unattended,
in a test script for instance.

```text
$ govax --mount-write DUA0=work.dsk macro 'DUA0:[SRC]PROG.MAR'
$ govax --mount DUA0=ex.dsk run 'DUA0:[DEMO]GREET'
What is your name? Grace
Hello, Grace!
$ govax --instruction-limit 5000000 run "./loop.exe"
```

### Exit status

| Status | Meaning |
| --- | --- |
| 0 | The command succeeded (for `RUN`, the image's exit status was a success). |
| 1 | The command failed: its message has been shown. |
| 124 | `--instruction-limit` or `--time-limit` stopped it. (124 is the status the Unix `timeout` command uses.) |

So a makefile or shell script can use govax like any other tool:

```sh
govax macro prog.mar && govax link prog.obj && govax run prog.exe
```

### Control keys

govax gives the control keys their VMS meanings:

| Key | At the `VAX>` prompt | While a program or command runs |
| --- | --- | --- |
| **Ctrl/C** | Cancels the line being typed. | Interrupts it, back to the prompt it was started from (`VAX>` or `DBG>`). A program that enabled a Ctrl/C AST gets the AST instead. |
| **Ctrl/Y** | Ends govax. | Ends govax (a second Ctrl/Y exits at once if the first doesn't). A program with a Ctrl/Y AST gets it instead. In a command procedure, Ctrl/Y is DCL's: see [Ctrl/Y in procedures](#ctrly-in-procedures). |
| **Ctrl/Z** | Echoes `*Exit*`, as DCL does. At `DBG>` it's `EXIT`. | End of file for the program's read (RMS's `RMS$_EOF`). |

`SET NOCONTROL=Y` turns Ctrl/Y off. `HELP KEYS` has the details,
including how the keys go to a subprocess that `SPAWN` is waiting for.

---

## 4. Configuration

govax keeps settings in a *profile*: a file in the `.govax` directory of
your home directory, read each time govax starts. You change them from
your host shell with `govax config`, not from the `VAX>` prompt.

### Setting, showing, and deleting

```text
govax config set KEY=VALUE      set a key
govax config set KEY            set a key to true
govax config show               show every key in the profile
govax config show KEY           show one key's value
govax config delete KEY         delete a key, so its default applies again
```

For example:

```text
$ govax config set vax.default.volume.file="~/vax/work.dsk"
Configuration key vax.default.volume.file written
$ govax config set vax.sysgen.paging.enable=true
Configuration key vax.sysgen.paging.enable written
$ govax config show
Key                         Value
vax.default.volume.file     ~/vax/work.dsk
vax.sysgen.paging.enable    true
$ govax config show vax.sysgen.paging.enable
true
$ govax config delete vax.sysgen.paging.enable
```

Keys are lowercase. govax warns, as it exits, about any key in the profile
it doesn't recognize; that usually means a misspelling. It also warns
about a value it can't use (a page count that isn't a number, say) and
uses the default instead.

Quote values your shell would otherwise change. In particular, zsh and
some other shells treat `[...]` as a file name pattern:

```sh
govax config set "vax.sysgen.paging.file=DUA0:[WORK]PAGEFILE.SYS"
```

### Profiles

You can keep several profiles, for different projects or for testing, and
choose one with the global `--profile` option:

```text
govax --profile test config set vax.quantum=50     set a key in "test"
govax --profile test                               run govax with "test"
govax config list                                  list the profiles
govax --profile test config set-description "For the test suite"
govax config remove test                           delete the "test" profile
```

Without `--profile`, govax uses the profile named `default`.

### Reading the configuration from DCL

The lexical function `F$CONFIG`, which is govax's own, reads a key from a
DCL procedure:

```text
$ IF F$CONFIG("vax.library", "EXISTS") THEN -
     WRITE SYS$OUTPUT "Libraries in ", F$CONFIG("vax.library")
$ PAGING = F$CONFIG("vax.sysgen.paging.enable", "BOOLEAN")
```

Its second argument is `VALUE` (the default: the value, or `""`),
`EXISTS` (1 if the key is set), or `BOOLEAN` (1 if the value is TRUE, T,
YES, Y, or 1 in any case). `vax.init` uses it to read the memory sizes.

### Configuration keys

#### The default volume

| Key | Meaning | Default |
| --- | --- | --- |
| `vax.default.volume.file` | A container to mount at startup and make the default device. A leading `~/` is your home directory. govax creates it (initialized, with the default directory in it) if it doesn't exist. | none |
| `vax.default.volume.label` | The label a new container gets. When set, an existing container's label must match or it isn't used. | `WORK` |
| `vax.default.volume.device` | The device to mount it on. | `DUA0:` |
| `vax.default.volume.type` | The disk type that sizes a new container (`RX50`, `RD54`, `RA81`, ...; see [Disk types](#disk-types)). | `RD54` (311,200 blocks) |
| `vax.default.volume.directory` | The default directory, created in a new container. | `[WORK]` |

If the volume can't be used, govax says why (`%MOUNT-W-NODEFAULT`) and
starts without it. A container that can only be read is mounted
read-only.

#### Files and libraries

| Key | Meaning |
| --- | --- |
| `vax.path` | Host directories to search for files, separated by commas, as with `--path`. |
| `vax.library` | A host directory holding VMS's own `STARLET.MLB`, `STARLET.OLB`, `IMAGELIB.OLB`, and shareable images, if you have them. `MACRO` and `LINK` look there when `SYS$LIBRARY` and `SYS$SHARE` don't lead to the files (see [Using VMS's own libraries](#using-vmss-own-libraries)). |
| `vax.link.library` | The older name for `vax.library`. |

#### The console

| Key | Meaning | Default |
| --- | --- | --- |
| `vax.startup.procedure` | A DCL procedure to run after `vax.init` each time govax starts: a volume file (`DUA0:[WORK]LOGIN.COM`) or a host file. If it doesn't exist, nothing happens. | none |
| `vax.console.prompt` | The prompt. `SET PROMPT` changes it and saves it here. | `VAX> ` |
| `vax.console.lines` | The screen height `TYPE/PAGE` uses when it can't ask the terminal. | 24 |
| `vax.disassemble.symbolic` | Whether the debugger shows instructions with symbolic names by default. | true |
| `vax.language` | The user's language, as `LIB$GET_USERS_LANGUAGE` returns it when `SYS$LANGUAGE` isn't defined. | the host's locale |

A login procedure is a good place for your own symbols and logical
names:

```text
$! DUA0:[WORK]LOGIN.COM
$ SD :== SET DEFAULT
$ DIR :== DIRECTORY/SIZE/DATE
$ DEFINE SRC DUA0:[WORK.SRC]
$ DEFINE SYS$SHARE DUA0:[SYSLIB]
```

```sh
govax config set "vax.startup.procedure=DUA0:[WORK]LOGIN.COM"
```

#### The machine

| Key | Meaning | Default |
| --- | --- | --- |
| `vax.hardware.clock` | true: the system time and clock follow the host's real time. false: time advances with the instructions executed, so every run is exactly repeatable. | false |
| `vax.quantum` | In instruction-count mode, the instructions between clock ticks. (`vax.init` sets it to 1 with `SET QUANTUM 1`, so each instruction is one emulated millisecond.) | 20 |
| `vax.runtime.random` | true: `MTH$RANDOM`'s numbers come from the host's cryptographic generator, and can't be predicted or repeated; the seed is still rewritten, but its old value plays no part. false: each number follows from the seed by the VMS manual's formula, so setting the same seed repeats a sequence, run after run. | false |

#### Memory

These sizes are read by `vax.init`. Each is a count of 512-byte pages,
decimal or `%X` hexadecimal.

| Key | Meaning | Default |
| --- | --- | --- |
| `vax.memory.size` | Physical memory. | 16384 (8 MB) |
| `vax.memory.p0` | The P0 (program) region's pages. | 16384; with paging, 19456 |
| `vax.memory.p1` | The P1 (control) region's pages. | 8192; with paging, 5120 |
| `vax.memory.s0` | The S0 (system) region's pages. | 8192; with paging, 16384 |
| `vax.memory.ksp` | The kernel stack. | 20 |
| `vax.memory.esp` | The executive stack. | 8 |
| `vax.memory.ssp` | The supervisor stack. | 8 |
| `vax.memory.isp` | The interrupt stack. | 4 |

The defaults suit most programs. A program that needs more than about
8 MB of P0 space needs a larger `vax.memory.p0` (and, without paging,
more physical memory). Section 13 explains the regions.

#### Processes

| Key | Meaning | Default |
| --- | --- | --- |
| `vax.process.scheduler` | Whether programs can create processes (`$CREPRC`, `LIB$SPAWN`, `SPAWN`). Off, only the console's own process runs. | true |
| `vax.process.quantum` | Instructions a process runs before another of the same priority gets the CPU. | 20000 |
| `vax.process.preempt` | The access modes in which a process can lose the CPU: `all` (as VMS), `user`, or `none` (only when it waits). | all |

#### System parameters

The `vax.sysgen.*` keys are govax's SYSGEN parameters, named after the
VMS parameters they stand for. Counts are in pages. Most only matter with
paging on; section 13 explains what they control.

| Key | VMS parameter | Default |
| --- | --- | --- |
| `vax.sysgen.paging.enable` | (govax's) turns VMS paging on | false |
| `vax.sysgen.paging.file` | (govax's) the page file: a volume file, or a quoted host path | none |
| `vax.sysgen.paging.size` | (govax's) the page file's size in blocks | 32768 |
| `vax.sysgen.paging.defer` | (govax's) open the page file only when first needed | false |
| `vax.sysgen.pql_dwsdefault` | PQL_DWSDEFAULT: a new process's default working set | 294 |
| `vax.sysgen.pql_dwsquota` | PQL_DWSQUOTA: its working set quota | 588 |
| `vax.sysgen.pql_dwsextent` | PQL_DWSEXTENT: its working set extent | 16400 |
| `vax.sysgen.pql_dpgflquota` | PQL_DPGFLQUOTA: a new job's page file quota | 16400 |
| `vax.sysgen.minwscnt` | MINWSCNT: the smallest working set | 20 |
| `vax.sysgen.tbskipwsl` | TBSKIPWSL | 8 |
| `vax.sysgen.growlim` | GROWLIM | 63 |
| `vax.sysgen.freelim` | FREELIM | 32 |
| `vax.sysgen.freegoal` | FREEGOAL | 200 |
| `vax.sysgen.mpw_hilimit` | MPW_HILIMIT | 500 |
| `vax.sysgen.mpw_lolimit` | MPW_LOLIMIT | 32 |
| `vax.sysgen.mpw_wrtcluster` | MPW_WRTCLUSTER (a multiple of 8, 16 to 120) | 120 |
| `vax.sysgen.pfcdefault` | PFCDEFAULT (0 to 127) | 32 |
| `vax.sysgen.gblsections` | GBLSECTIONS | 128 |
| `vax.sysgen.gblpages` | GBLPAGES | 4096 |
| `vax.sysgen.gblpagfil` | GBLPAGFIL | 4096 |
| `vax.sysgen.virtualpagecnt` | VIRTUALPAGECNT | P0 + P1 |

The console's own process has the `SYSTEM` account's quotas rather than
the PQL defaults: a working set default of 512, quota 1024, and page
file quota 40960.

#### Debugging switches

`vax.debug.NAME` (true or false) sets the initial state of the `SET
DEBUG` switch `NAME`: `vm`, `tb`, `symbols`, `exceptions`, `interrupts`,
`chm`, `registers`, `fulldisasm`, `userhalt`, `keyboard`, `images`,
`services`, `dcl`, `command`, `logicals`, `devices`, `process`,
`libinit`, `rms`, or `userstep`. These trace govax's own workings, and
are mainly for govax's developers.

---

## 5. Files, volumes, and devices

govax works with two kinds of files: **host files**, the ordinary files
of the computer govax runs on, and **volume files**, files on a Files-11
(ODS-2) disk volume that govax has mounted. They behave differently:

| | Host files | Volume files |
| --- | --- | --- |
| Names | your host's: `./src/prog.mar`, `/tmp/x.txt` | VMS's: `DUA0:[WORK.SRC]PROG.MAR;3` |
| Versions, directories, protection | the host's | VMS's |
| Used by the console's commands (`MACRO`, `LINK`, `LIBRARY`, `RUN`, `TYPE`, `COPY`, `@`, ...) | yes | yes |
| Used by a running program's RMS calls (`$OPEN`, `$CREATE`, `$GET`, ...) | no | yes |
| Listed by `DIRECTORY`, `DUMP`, `DELETE`, `PURGE`, `RENAME`, `F$SEARCH` | no | yes |

The important line is the fourth: **a VAX program can only open files on a
mounted volume.** The console's commands accept both, so you can keep
sources on your host and still assemble, link, and run them, but a
program that reads or writes files with RMS needs a volume.

### How govax decides which kind a name is

Each console command applies the same rules to a file name, in order:

1. A name followed by the qualifier `/HOST` is a host file:
   `TYPE "notes.txt"/HOST`.
2. A name with a directory in brackets (`[SRC]PROG.MAR`), or with a
   device or logical name of two or more characters and a colon
   (`DUA0:PROG.MAR`, `SRC:PROG.MAR`), is a volume file.
3. A name with `/` or `\` in it, or a Windows drive letter (`C:`), is a
   host file: `"./prog.mar"`, `"/Users/me/prog.mar"`.
4. Any other name (`PROG.MAR`, `PROG`) is a volume file in your default
   directory if `SET DEFAULT` names a mounted volume, and a host file in
   the host's current directory otherwise.

DCL makes unquoted text uppercase, so **quote a host name** whenever its
case matters: `MACRO "hello.mar"`, not `MACRO hello.mar` (which looks for
`HELLO.MAR`, and on a case-sensitive host file system doesn't find
`hello.mar`). Quotes also keep a `/` in a host path from being read as a
qualifier.

Without a default volume, your default directory is the host's current
directory, and a bare `HELLO.MAR` means the host file `HELLO.MAR`.

### Containers

A container is a host file that holds a complete ODS-2 volume, block for
block, the same "disk image" format SIMH and other emulators use. You
make one with `INITIALIZE/CONTAINER`, attach it to a device with `MOUNT`,
and detach it with `DISMOUNT`:

```text
VAX> INITIALIZE/CONTAINER "project.dsk" /DEVICE=RD54 PROJECT
VAX> MOUNT DUA1: "project.dsk"
%MOUNT-I-MOUNTED, PROJECT mounted on _DUA1:
VAX> CREATE/DIRECTORY DUA1:[SRC]
VAX> SET DEFAULT DUA1:[SRC]
VAX> ...
VAX> DISMOUNT DUA1:
```

`MOUNT` defines `DISK$label` in the system table, as VMS does, so the
volume can also be named `DISK$PROJECT:[SRC]`. A volume is mounted for
writing unless you say `/NOWRITE`. Writes reach the container when you
dismount it or leave govax; govax dismounts every volume as it exits, so
you don't have to.

A container made elsewhere (on SIMH, say, or one holding a VMS
distribution's files) mounts the same way. Mount someone else's volume
with `/NOWRITE` if you don't want it changed.

From the host shell, `--mount DEV=FILE` and `--mount-write DEV=FILE`
mount containers before anything else, which is how a script runs a
program that needs a volume:

```sh
govax --mount-write DUA0=project.dsk run 'DUA0:[SRC]PROG.EXE'
```

#### Disk types

`INITIALIZE/CONTAINER /DEVICE=type` sizes a new volume as a DEC disk of
that type (or give `/SIZE=blocks`). Some useful ones:

| Type | Blocks | Size |
| --- | ---: | ---: |
| RX50 | 800 | 400 KB |
| RD51 | 21,600 | 10.5 MB |
| RD53 | 138,672 | 68 MB |
| RD54 | 311,200 | 152 MB |
| RA81 | 891,072 | 435 MB |
| RA92 | 2,940,951 | 1.4 GB |

`docs/DISK-DEVICES.md` lists them all.

### Directories, versions, and the default directory

Volume files have VMS's names: `device:[directory.subdirectory]name.type;version`.
Every file you create is a new version; commands that read a file take
the highest version unless you give one. `SET DEFAULT` sets the default
device and directory, which fill in any part of a name you leave out:

```text
VAX> SET DEFAULT DUA0:[WORK]
VAX> SET DEFAULT [.SRC]           ! DUA0:[WORK.SRC]
VAX> SET DEFAULT [-]              ! back up to DUA0:[WORK]
VAX> SHOW DEFAULT
  DUA0:[WORK]
```

`CREATE/DIRECTORY` makes directories (and any missing ones above them);
`DIRECTORY` lists files; `PURGE` removes old versions; `DELETE` needs a
version (`;`, `;3`, or `;*`), as on VMS.

### Moving files between the host and a volume

`COPY` copies between the host and a volume, or between two volume
locations. `/HOST` after either file name marks that one as a host path.
Without it, each name follows the rules of
[How govax decides which kind a name is](#how-govax-decides-which-kind-a-name-is),
so a quoted path with a `/` in it is a host file too:

```text
VAX> COPY "prog.mar"/HOST DUA0:[WORK]       ! host to volume, same name
VAX> COPY "prog.mar"/HOST *                 ! the same, into the default directory
VAX> COPY PROG.LIS "prog.lis"/HOST          ! volume to host
VAX> COPY PROG.LIS "./prog.lis"             ! the same: the "/" makes it a host path
VAX> COPY *.MAR [BACKUP]*.SAV               ! volume to volume, wildcards
```

How the contents are copied:

- **Text** goes to a volume as variable-length records, one per line
  (host line endings removed), and comes back as host lines (LF, or CRLF
  with `/CRLF`).
- **Object modules** (`.OBJ`) are always copied record by record. On the
  host they're kept in ODS-2's record layout (a 2-byte length, then the
  record, padded to even), so a `.OBJ` can go to the host and back
  intact.
- **Anything else binary**, such as an image (`.EXE`) or a library, needs
  `/BINARY`, which copies the bytes exactly. A copy from one volume
  location to another always keeps the file's record format.

```text
VAX> COPY/BINARY "fromvax.exe"/HOST DUA0:[WORK]PROG.EXE
```

To move a whole directory tree, with each file's record format, dates,
protection, and other attributes, use a VMS save set: `BACKUP` writes
one to the host or reads one from it (see [BACKUP](#backup)), and a
save set made on a VAX and fetched by binary FTP can be restored
directly.

### Logical names

Logical names work as on VMS: `DEFINE`, `ASSIGN`, `DEASSIGN`, `SHOW
LOGICAL`, `SHOW TRANSLATION`, and `CREATE/NAME_TABLE`, with process, job,
group, and system tables, access modes, search lists, and the CONCEALED
and TERMINAL attributes. File names are translated through them, by the
console's commands and by RMS in a running program:

```text
VAX> DEFINE SRC DUA0:[WORK.SRC]
VAX> MACRO SRC:PROG
VAX> DEFINE/TRANSLATION=CONCEALED PROJ DUA1:
VAX> DIRECTORY PROJ:[PROJECT.SRC]

Directory PROJ:[PROJECT.SRC]
...
VAX> DEFINE LIBS DUA0:[LIB1],DUA0:[LIB2]      ! a search list
VAX> TYPE LIBS:HELLO.MAR                      ! the first one that has it
```

One VMS feature is missing: *rooted* directory logical names, whose
equivalence ends in `.]` (`DEFINE/TRANSLATION=CONCEALED PROJ
DUA1:[PROJECT.]`, then `PROJ:[SRC]`). govax doesn't yet treat such a name
as the root of a directory tree.

govax predefines `SYS$INPUT`, `SYS$OUTPUT`, `SYS$ERROR`, and
`SYS$COMMAND`, the terminal opened as process-permanent files, as VMS's
LOGINOUT opens them: each name's equivalence string is the special form
that leads RMS to the open file, which `SHOW LOGICAL` and `F$TRNLNM` show
as the device, `_GOVAX$TTA0:`. DCL defines its own `SYS$OUTPUT` in
supervisor mode, naming the same file, and while a command procedure
runs it defines `SYS$INPUT` as the procedure and `SYS$OUTPUT` as its
`/OUTPUT` file (see "Command procedures"), so `SHOW LOGICAL/PROCESS`
lists `SYS$OUTPUT` twice, `[super]` and `[exec]`. It also predefines `TT` (`TTA0:`,
concealed, in executive mode),
`SYS$DISK` (set by `SET DEFAULT`), `DISK$label` for each mounted volume, and the date-format
names `LIB$DT_FORMAT` and `LIB$DT_INPUT_FORMAT`. It doesn't define
`SYS$LIBRARY`, `SYS$SHARE`, `SYS$SYSTEM`, or `SYS$LOGIN`, because there is
no system disk; define them yourself (in a login procedure) if you keep
libraries or shareable images on a volume.

### Devices

`SHOW DEVICES` lists the devices: the terminal `TTA0:`, the null device
`NLA0:` (writes vanish, reads are at end of file), and each disk you've
mounted. `MOUNT` creates a disk device the first time it's named; any
disk name works (`DUA0`, `DUA1`, `DKA0`). Programs can also create mailboxes
(`$CREMBX`), which appear as `MBAn:`. `SHOW DEVICE/FULL DUA0` shows a
disk's details, read from the volume.

`DEFINE/DEVICE` defines a device with given characteristics; `vax.init`
uses it for `TTA0:` and `NLA0:`. You won't normally need it.

---

## 6. DCL command reference

This section describes every command the `VAX>` prompt accepts, grouped
by purpose. For each, it gives the syntax and the qualifiers you're
likely to use; `HELP command` at the prompt has the complete list.

General rules, as in VMS's DCL:

- Verbs, qualifiers, and keywords can be abbreviated, as long as the
  abbreviation is unique: `DIREC/SIZ`, `SHO DEF`, `SH SYS`.
- Unquoted text is made uppercase; quoted text keeps its case and its
  blanks. `""` inside quotes is a quote.
- `!` starts a comment, outside quotes.
- A line ending in `-` continues on the next line.
- Qualifiers can go after the verb or after a parameter. A qualifier
  after a file name may apply to that file only (`/HOST`, `/LIBRARY`).
- Each command sets `$STATUS` (see [Statuses](#statuses-status-and-severity)).

### 6.1 Files and directories

#### DIRECTORY

```text
DIRECTORY [file-spec[,...]]   (or file-spec+file-spec)
    /SIZE[=USED|ALLOCATION|ALL]  /DATE  /OWNER  /PROTECTION  /FILE
    /FULL  /VERSIONS=n  /[NO]HEADING  /[NO]TRAILING  /OUTPUT[=file]
```

Lists files on a volume, laid out as VMS lays them out. With no file
spec, every version of every file in the default directory. Wildcards `*`
and `%` work in every field, and `[...]` searches a directory tree.

A list of specs is listed in order, as on VMS. Each spec takes the
device and directory of the one before it unless it gives its own (`[]`
is the default directory), and files found in the directory just
listed go under the same heading and total:

```text
VAX> DIRECTORY/SIZE DUA0:[WORK.DUNGEON]*.EXE,*.DAT

Directory DUA0:[WORK.DUNGEON]

DUNGEON.EXE;1            210
DUNGEON_V552.EXE;1       209
DINDX.DAT;1               26
DTEXT.DAT;1              462

Total of 4 files, 907 blocks.
```

```text
VAX> DIRECTORY/SIZE/DATE

Directory DUA0:[HELLO]

HELLO.EXE;1                5  20-OCT-2026 09:56:08.37
HELLO.LIS;1                8  20-OCT-2026 09:56:08.37
HELLO.MAP;1                8  20-OCT-2026 09:56:08.37
HELLO.MAR;1                1  20-OCT-2026 09:56:08.36
HELLO.OBJ;1                1  20-OCT-2026 09:56:08.36

Total of 5 files, 23 blocks.
```

#### TYPE

```text
TYPE [/PAGE] [/HOST] file-spec
```

Displays a file (one file; no wildcards that match several). Volume files
are shown record by record, whatever their record format; host files line
by line. `/PAGE` stops after each screenful. `TYPE SYS$INPUT` types the
lines that follow it in a command procedure.

#### COPY

```text
COPY source[/HOST] destination[/HOST]
    /BINARY  /QUIET  /VERBOSE  /TEST
    (to the host only) /CRLF  /LF  /TIME  /STREAM  /DIRS  /IGNORE
```

Copies files between the host and a volume, or between volume locations
(see [Moving files](#moving-files-between-the-host-and-a-volume)). A
destination that's only a directory, or whose name is `*`, takes each
source file's name; a `*` type takes the source's type. A wildcarded
source copies every match. A copy never replaces an existing version:
with no version in the destination, the copy is the next version.

#### CREATE

```text
CREATE [/LOG] file-spec[/HOST]
CREATE/DIRECTORY [/LOG] [/OWNER_UIC=uic] [/PROTECTION=(...)]
                 [/VERSION_LIMIT=n] [/ALLOCATION=n] directory[,...]
CREATE/NAME_TABLE [/PARENT_TABLE=table] table-name
```

`CREATE` makes a text file from what follows: at the terminal, the lines
you type up to Ctrl/Z; in a procedure, the data lines after it.

```text
$ CREATE SETUP.DAT
ITEM ONE
ITEM TWO
$ TYPE SETUP.DAT
```

`CREATE/DIRECTORY` makes a directory and any missing ones above it.
`CREATE/NAME_TABLE` makes a logical name table.

#### DELETE

```text
DELETE [/LOG] file-spec;version[,...]
```

Deletes volume files. A version is required (`;3`, `;*`, or `;` for the
newest), as on VMS. Several specifications, separated by commas, take
missing fields from the one before: `DELETE A.TXT;1,;2,B;1`.

#### PURGE

```text
PURGE [/LIMIT=n] [file-spec]
```

Deletes all but the newest `n` versions (default 1) of each file.

#### RENAME

```text
RENAME [/LOG] [/[NO]NEW_VERSION] input[,...] output
```

Renames or moves files within a volume. `*` in the output keeps that
field of each input's name: `RENAME *.TXT *.OLD`. Directories can be
moved too, with everything in them.

#### DUMP

```text
DUMP [/BLOCK[=(START=n,END=n,COUNT=n)] | /RECORD[=(...)]] file-spec
```

Shows a volume file in hexadecimal and ASCII, a block or a record at a
time, as VMS's DUMP does.

#### BACKUP

```text
BACKUP/LIST[=file] [/FULL] save-set/SAVE_SET          list a save set
BACKUP [/LOG] save-set/SAVE_SET output-spec           restore files
BACKUP/COMPARE [/LOG] save-set/SAVE_SET file-spec     compare
BACKUP [/LOG] file-spec[,...] save-set/SAVE_SET       save files
```

Works with VMS BACKUP save sets, as VMS V7.1's BACKUP does: the save
sets govax writes are laid out as that VMS lays them out, and VMS reads
them. A save set can be a file on a mounted volume (VMS keeps one as
fixed-length records the size of its block) or a host file holding the
same bytes, such as one fetched from a VAX by binary FTP. Image,
physical, and incremental operations and tapes aren't supported.

Which specifier has `/SAVE_SET` says what BACKUP does: a save set as
input restores (or, with `/COMPARE`, compares); as output, saves. As in
VMS, *where* a qualifier stands matters. After `BACKUP` it's a command
qualifier (`/LIST`, `/FULL`, `/LOG`, `/COMPARE`). After the input
specifier it's about the input: `/SELECT`, `/EXCLUDE`, `/CRC` for a save
set, or `/SINCE`, `/BEFORE`, `/BY_OWNER`, `/EXCLUDE` for the files being
saved. After the output specifier it's about the output: `/BLOCK_SIZE`,
`/GROUP_SIZE`, `/CRC`, `/COMMENT` for a new save set, or `/NEW_VERSION`,
`/OVERLAY`, `/REPLACE`, `/BY_OWNER` for restored files. `HELP BACKUP`
lists them all.

**Saving.** A save goes through each directory in order, every version
of each file unless the specifier names one, and with `[dir...]` into
each subdirectory:

```text
VAX> SET DEFAULT [WORK]
VAX> BACKUP/LOG [.PROJ...]*.* PROJ.BCK/SAVE_SET
%BACKUP-S-COPIED, copied DUA0:[WORK.PROJ]DOC.DIR;1
%BACKUP-S-COPIED, copied DUA0:[WORK.PROJ.DOC]README.TXT;1
%BACKUP-S-COPIED, copied DUA0:[WORK.PROJ]NOTES.TXT;2
%BACKUP-S-COPIED, copied DUA0:[WORK.PROJ]NOTES.TXT;1
```

A quoted host path (or `/HOST`) writes the save set to the host. Here
it also has 8192-byte blocks and a CRC in each block, and `;0` saves
only each file's newest version:

```text
VAX> BACKUP/LOG [.PROJ...]*.TXT;0 "./proj.bck"/SAVE_SET/CRC/BLOCK_SIZE=8192
```

**Listing.** `/LIST` shows the save set's summary and its files, each
with its size in blocks and its creation date; `/FULL` shows each file's
dates, owner, protection, and record attributes, as DIRECTORY/FULL does.
`/LIST=file` writes the listing to a file.

```text
VAX> BACKUP/LIST PROJ.BCK/SAVE_SET
Listing of save set(s)

Save set:          PROJ.BCK
Written by:        SYSTEM
UIC:               [000001,000004]
Date:              20-OCT-2026 19:36:08.20
Command:           BACKUP/LOG [.PROJ...]*.* PROJ.BCK/SAVE_SET
Operating system:  OpenVMS VAX version V7.3
BACKUP version:    V6.2
Written on:        _GOVAX$DUA0:
Block size:        32256
Group size:        10
Buffer count:      8

[WORK.PROJ]DOC.DIR;1                                        1  20-OCT-2026 19:36
[WORK.PROJ.DOC]README.TXT;1                                 1  20-OCT-2026 19:36
[WORK.PROJ]NOTES.TXT;2                                      1  20-OCT-2026 19:36
[WORK.PROJ]NOTES.TXT;1                                      1  20-OCT-2026 19:36

Total of 4 files, 4 blocks
End of save set
```

As in VMS V7.1, a listing shows every file in the save set; `/SELECT`
and `/EXCLUDE` don't shorten it.

**Restoring.** The output specifier names where the files go:
`[*...]` puts each file back in the directory it was saved from;
`[dir...]` puts them in a tree under `[dir]`; `[dir]` puts them all in
that one directory. Under `[dir...]` a file keeps its whole directory
(`[WORK.PROJ]NOTES.TXT` goes to `[dir.WORK.PROJ]`), unless `/SELECT`
names a tree, in which case the part its `...` matched is kept:

```text
VAX> BACKUP/LOG PROJ.BCK/SAVE_SET/SELECT=[WORK.PROJ...] [RESTORED...]
%BACKUP-S-CREDIR, created directory DUA0:[RESTORED.DOC]
%BACKUP-S-CREATED, created DUA0:[RESTORED.DOC]README.TXT;1
%BACKUP-S-CREATED, created DUA0:[RESTORED]NOTES.TXT;2
%BACKUP-S-CREATED, created DUA0:[RESTORED]NOTES.TXT;1
```

A restored file keeps its version and every saved attribute: record
format, dates, revision count, owner (with `/BY_OWNER=ORIGINAL` after the
output specifier; by default you own it), protection, characteristics,
version limit, allocation, and access control list. Missing directories
are made. A file that already exists isn't restored unless the output
specifier has `/NEW_VERSION` (restore it as the next version),
`/OVERLAY` (write over it in place), or `/REPLACE` (delete it and
restore it again):

```text
VAX> BACKUP PROJ.BCK/SAVE_SET/SELECT=[...]NOTES.TXT;1 [*...]
%BACKUP-E-OPENOUT, error opening DUA0:[WORK.PROJ]NOTES.TXT;1 as output
-RMS-E-FEX, file already exists, not superseded
%BACKUP-W-NOFILES, no files selected from DUA0:[WORK]PROJ.BCK;
VAX> BACKUP PROJ.BCK/SAVE_SET/SELECT=[...]NOTES.TXT;1 [*...]/NEW_VERSION/LOG
%BACKUP-S-CREATED, created DUA0:[WORK.PROJ]NOTES.TXT;3
```

A host directory can be the output too (`"out"`, or `"out/..."` for a
tree); its files are written as COPY writes them to the host, with
their versions in their names:

```text
VAX> BACKUP PROJ.BCK/SAVE_SET/SELECT=[WORK.PROJ...] "out/..."/LOG
%BACKUP-S-CREDIR, created directory out/DOC
%BACKUP-S-CREATED, created out/DOC/README.TXT;1
%BACKUP-S-CREATED, created out/NOTES.TXT;2
%BACKUP-S-CREATED, created out/NOTES.TXT;1
```

**Comparing.** `/COMPARE` reads each file in the save set and the file
it would be restored to, the output specifier's rules deciding which,
and reports only a difference or a missing file (with `/LOG`, each file
compared too). After `out/DOC/README.TXT;1` was deleted and a line added
to `out/NOTES.TXT;2`:

```text
VAX> BACKUP/COMPARE PROJ.BCK/SAVE_SET/SELECT=[WORK.PROJ...] "out/..."
%BACKUP-E-OPENIN, error opening out/DOC/README.TXT;1 as input
-RMS-E-FNF, file not found
%BACKUP-E-VERIFYERR, verification error for block 1 of out/NOTES.TXT;2
```

**Damaged save sets.** Every save set block carries a checksum of its
header, and with `/CRC` (on the output specifier, when saving) a CRC of
its data. Every ten blocks (`/GROUP_SIZE`) come with a redundancy
block, from which BACKUP rebuilds one damaged block of the group as VMS
does. A block's header is always checked; its data CRC only when the
input specifier has `/CRC`, as in VMS V7.1, whose default is `/NOCRC`.
A file the save set holds only part of (one cut short, say) is restored
with zeros for the blocks missing, and a VBNMISSING message.

#### SET DEFAULT and SHOW DEFAULT

```text
SET DEFAULT [device:][directory]
SHOW DEFAULT
```

Sets and shows the default device and directory. Omitted parts are kept
from the current default; `[.SUB]` and `[-]` are relative.

### 6.2 Volumes and devices

#### INITIALIZE/CONTAINER

```text
INITIALIZE/CONTAINER "file" [label] [/DEVICE=type | /SIZE=blocks] [/CLUSTER=n]
```

Creates a new, empty ODS-2 volume in a host file. It isn't mounted; use
`MOUNT`.

#### MOUNT and DISMOUNT

```text
MOUNT [/NOWRITE] [/NOLOG] device "file"
DISMOUNT device
```

`MOUNT` attaches a container to a device (creating the device if it's
new). `DISMOUNT` writes pending changes and detaches it; it refuses a
volume with files open (a process still using them, say).

#### SHOW DEVICES

```text
SHOW DEVICES [/FULL] [name]
```

Lists devices, or those whose names begin with `name`.

#### DEFINE/DEVICE

Defines a pseudodevice and its characteristics (`/DEVCLASS`, `/DEVTYPE`,
`/DEVCHAR`, ...). `vax.init` uses it for the terminal and null device.

### 6.3 Logical names

```text
DEFINE [/PROCESS|/JOB|/GROUP|/SYSTEM|/TABLE=t] [/USER_MODE|/SUPERVISOR_MODE|/EXECUTIVE_MODE]
       [/TRANSLATION_ATTRIBUTES=(CONCEALED,TERMINAL)] [/[NO]LOG] name value[,...]
ASSIGN [same qualifiers] value[,...] name
DEASSIGN [/table qualifiers] [/mode] name
DEASSIGN/ALL [/table qualifiers]
SHOW LOGICAL [/table qualifiers] [/FULL] [/STRUCTURE] [name[,...]]
SHOW TRANSLATION [/TABLE=t] name
CREATE/NAME_TABLE [/PARENT_TABLE=t] [/mode] table
```

These follow VMS exactly. The default table is the process table, and
the default mode supervisor. The job table is shared by the console's
process and its subprocesses. A user-mode process name is deleted when
the next image you `RUN` exits. `SHOW LOGICAL` follows a name through
each level of translation:

```text
VAX> DEFINE MYDISK WORK4
VAX> DEFINE/SYSTEM WORK4 $255$DUA17:
VAX> SHOW LOGICAL MYDISK
   "MYDISK" = "WORK4" (LNM$PROCESS_TABLE)
1  "WORK4" = "$255$DUA17:" (LNM$SYSTEM_TABLE)
```

### 6.4 Symbols

```text
name = expression        name == expression      (local, global)
name := string           name :== string
SHOW SYMBOL [/LOCAL|/GLOBAL] [/ALL] [name]
DELETE/SYMBOL [/LOCAL|/GLOBAL] [/ALL] [name]
```

DCL symbols hold integers and strings; section 7 describes them.

```text
VAX> SD :== SET DEFAULT
VAX> FO*RTH :== $DUA0:[TOOLS]FORTH.EXE
VAX> N = 42
VAX> SHOW SYMBOL *
  N = 42   Hex = 0000002A  Octal = 00000000052
  $SEVERITY == "1"
  $STATUS == "%X00030001"
  FO*RTH == "$DUA0:[TOOLS]FORTH.EXE"
  SD == "SET DEFAULT"
```

Local symbols are listed first, then global ones.

### 6.5 Program development

These commands are described fully in their own sections:

| Command | Section |
| --- | --- |
| `FORTRAN source[,...] [qualifiers]` | [FORTRAN](#fortran), below |
| `MACRO source [qualifiers]` | [8. Programming in MACRO-32](#8-programming-in-macro-32) |
| `LINK file[,...] [qualifiers]` | [9. The linker](#9-the-linker) |
| `LIBRARY library [input[,...]] [qualifiers]` | [10. The librarian](#10-the-librarian) |
| `ANALYZE/OBJECT` and `ANALYZE/IMAGE` | [Analyzing objects and images](#analyzing-objects-and-images) |

#### FORTRAN

```text
FORTRAN source[,...][/HOST]  /MACRO[=file] | /NOMACRO
    /[NO]EXTEND_SOURCE  /[NO]D_LINES  /CONTINUATIONS=n  /[NO]I4
    /[NO]G_FLOATING
```

Compiles VAX FORTRAN source files (type `.FOR` by default) into MACRO-32
source files, which MACRO assembles and LINK links. govax's FORTRAN
compiler is being built a phase at a time (`docs/PHASE-74 - FORTRAN
plan.md`); so far it compiles a subset of the language (main
programs, subroutines, functions, and BLOCK DATA; every type's
declaration and storage, PARAMETER, IMPLICIT, arrays, COMMON,
EQUIVALENCE, DATA, SAVE, EXTERNAL, INTRINSIC; expressions in every type
and length, INTEGER, LOGICAL, REAL, COMPLEX, and CHARACTER, with every
intrinsic function; IF, DO in every form, DO WHILE, END DO, GO TO in every form, ASSIGN, CALL, RETURN, STOP, PAUSE; READ, WRITE,
PRINT, TYPE, ACCEPT, ENCODE, DECODE, FORMAT, and NAMELIST, though FTNRTL doesn't run namelist statements or internal files yet), and reports any
other statement as not implemented yet. The MACRO file holds every source line as a comment,
with its number, before the instructions it became. Sources and the MACRO file follow MACRO's
rules for a source and its object: the MACRO file has the source's name
with the type `MAR`, beside the source, unless `/MACRO=` names another;
`/NOMACRO` writes none. `/EXTEND_SOURCE` lets fixed-format statements
run to column 132, `/D_LINES` compiles the lines with a `D` in column 1,
`/CONTINUATIONS=n` allows a statement n continuation lines (0 to 99;
19 by default), `/NOI4` makes INTEGER and LOGICAL with no length 2
bytes rather than 4, and `/G_FLOATING` makes REAL\*8 G_floating rather
than D_floating. `INCLUDE 'file'` reads a file (type `.FOR` by default)
found relative to the file that includes it, in its host directory or on
its volume's device and directory. The compiler's messages show the line
and column:

```text
%FORT-E-NOTIMPL, the OPEN statement is not implemented yet
  at line 7 of DUA0:[WORK]PROG.FOR;1
        OPEN (UNIT=1, FILE='DATA')
  ------^
```

After an error no MACRO file is written, and the command fails with
`DCL-E-FORERRORS`; after warnings the file is written and `$STATUS` is a
warning. [Chapter 14](#14-programming-in-fortran) describes the
compiler, its source forms, the MACRO file, and FTNRTL, the run-time
library programs link against.

### 6.6 Running programs and processes

#### RUN

```text
RUN [/DEBUG | /NODEBUG] [/NOEXECUTE] image[/HOST]
```

Runs an image (default type `.EXE`). `/DEBUG` runs it under the debugger;
`/NODEBUG` runs an image linked `/DEBUG` without it; `/NOEXECUTE` loads it
without running it. `$STATUS` is the image's exit status. See
[11. Running programs](#11-running-programs).

#### Foreign commands and MCR

```text
symbol :== $image-file
symbol [text]
MCR image [text]
```

A symbol whose value starts with `$` is a foreign command: typing it runs
the image, which reads the rest of the line with `LIB$GET_FOREIGN`. `MCR`
does the same without a symbol.

```text
VAX> ECHO :== $DUA0:[TOOLS]ECHO.EXE
VAX> ECHO one   two "Three"
ONE TWO "Three"
```

#### SPAWN, ATTACH, and LOGOUT

```text
SPAWN [/NOWAIT] [/INPUT=file] [/OUTPUT=file] [/PROCESS=name]
      [/PROMPT=string] [/NOSYMBOLS] [/NOLOGICAL_NAMES] [command]
ATTACH process-name | ATTACH/IDENTIFICATION=pid
LOGOUT
```

`SPAWN` creates a subprocess running DCL, with copies of your symbols and
process logical names, and your default directory. With a command, it
runs that command and logs out; without one, it reads commands from its
input (you get a `$ ` prompt) until `LOGOUT`. Unless `/NOWAIT`, your
process waits for it, and the terminal is the subprocess's meanwhile.
Its `/INPUT` and `/OUTPUT` (a file, `NL:`, a mailbox, or the terminal)
are its own process-permanent files, so its DCL, its images, and its
`WRITE SYS$OUTPUT` all write one log, in order; your `OPEN` files are not
passed to it.
`SPAWN`'s status is the subprocess's final status.

```text
VAX> SPAWN DIRECTORY/SIZE
%DCL-S-SPAWNED, process SYSTEM_1 spawned
%DCL-S-ATTACHED, terminal now attached to process SYSTEM_1

Directory DUA0:[HELLO]
...
%DCL-S-RETURNED, control returned to process SYSTEM
VAX> SPAWN/NOWAIT @NIGHTLY
```

A `/NOWAIT` subprocess runs alongside you: between your commands, while
you type, and while your programs run. What it writes appears above the
line you're typing. Commands that work on the machine itself (`DEBUG`,
`VMINIT`, `INITIALIZE/VAX`, `SAVE`, `LOAD`, ...) aren't available in a
subprocess (`%VAX-E-NOTSUBPROC`).

`ATTACH` gives the terminal to another process of your job that's
waiting for it. `LOGOUT` ends a subprocess's DCL; in the console's own
process it ends govax.

#### STOP

```text
STOP [process-name] [/IDENTIFICATION=pid] [/IMAGE] [/EXIT[=mode] | /NOEXIT]
```

With a process, deletes it (and its subprocesses), or with `/IMAGE` ends
only the image it's running. Without one, in a command procedure, ends
every procedure and returns to the prompt. The console's own process
(PID `00000301`) can't be stopped.

#### SHOW SYSTEM and SHOW PROCESS

```text
SHOW SYSTEM
SHOW PROCESS [process-name] [/IDENTIFICATION=pid]
```

List the processes, with their states, priorities, CPU time, and page
use, and show one process's details.

#### WAIT

```text
WAIT delta-time                  ! WAIT 00:00:05
```

Waits; other processes run meanwhile. Ctrl/C or Ctrl/Y ends the wait.

### 6.7 Command procedure control

These are described in [section 7](#7-command-procedures-and-the-dcl-language).

| Command | Purpose |
| --- | --- |
| `@file [p1 ... p8]` | Run a command procedure. |
| `EXIT [status]` | End the procedure (at the prompt, end govax). |
| `IF expr THEN command`, `IF`/`THEN`/`ELSE`/`ENDIF` | Conditional commands and blocks. |
| `GOTO label` | Jump to a label. |
| `GOSUB label`, `RETURN [status]` | Call a subroutine in the same procedure. |
| `CALL label [p1 ... p8]`, `SUBROUTINE`, `ENDSUBROUTINE` | Call a subroutine at a new command level. |
| `ON condition THEN command` | Choose what happens when a command fails, or on Ctrl/Y. |
| `SET [NO]ON` | Turn status checking off and on. |
| `CONTINUE` | Resume after Ctrl/Y; otherwise do nothing. |
| `STOP` | End every procedure. |
| `DECK`, `EOD` | Data lines that start with `$`. |
| `OPEN`, `CLOSE`, `READ`, `WRITE`, `INQUIRE` | File and terminal I/O. |
| `SET VERIFY`, `SET PREFIX` | Show a procedure's lines as they run. |
| `SET CONTROL=Y`, `SET NOCONTROL=Y` | Turn Ctrl/Y on and off. |
| `QUIT` | End govax, even from inside a procedure. |

### 6.8 Information

| Command | Shows |
| --- | --- |
| `SHOW DEFAULT` | The default device and directory. |
| `SHOW DEVICES` | The devices. |
| `SHOW LOGICAL`, `SHOW TRANSLATION` | Logical names. |
| `SHOW SYMBOL` | DCL symbols. |
| `SHOW SYSTEM`, `SHOW PROCESS` | Processes. |
| `SHOW MEMORY [/FILES [/FULL]]` | Physical memory, the address regions, and with paging on the page lists, faults, and page file. |
| `SHOW WORKING_SET` | The process's working set limit, quota, and extent. |
| `SHOW VERSION` | The microkernel's version. |
| `SHOW RADIX` | The console's numeric radix. |
| `SHOW DEBUG` | The debugging switches. |
| `SHOW STRINGS` | The console's string pool. |
| `HELP [topic ...]` | The help text. |

### 6.9 The machine and the console

These commands work on the emulated machine itself, rather than on VMS
objects. Most of them are used by `vax.init`; you need them only to
change how the machine is set up.

| Command | Purpose |
| --- | --- |
| `INITIALIZE/VAX pages` | Set physical memory to `pages` 512-byte pages, clearing the machine. The count is in the console's radix, hexadecimal by default: write `^D16384` for decimal. |
| `VMINIT [/P0=n] [/P1=n] [/S0=n] [/KSP=n] [/ESP=n] [/SSP=n] [/ISP=n] [/[NO]PAGING]` | Build page tables for the address regions, and turn memory management on (section 13). |
| `CLEAR MEMORY`, `ZERO` | Clear physical memory (and so turn memory management off). |
| `SAVE file`, `LOAD file` | Save and restore memory and registers. |
| `SAVE/ROM`, `LOAD/ROM`, `INIT/NVRAM`, `SAVE/NVRAM`, `LOAD/NVRAM` | The console ROM and NVRAM areas. |
| `DEBUG` | Start the debugger on the machine as it is (section 12). |
| `SET WORKING_SET [/LIMIT=n] [/QUOTA=n] [/EXTENT=n]` | Change your process's working set values. |
| `SET QUANTUM n` | Instructions between clock ticks in instruction-count mode. |
| `SET RADIX DECIMAL\|HEXADECIMAL` | The console's radix for `PRINT` and machine expressions (hexadecimal unless set). |
| `SET PROMPT="text"` | Change the prompt (and save it in the configuration). |
| `SET DEBUG [NO]switch` | govax's internal tracing switches. |
| `PRINT item[,...]` | Print quoted text and machine expressions, numbers in the console's radix (`PRINT "Base: ", 200+10`). A name is a DCL symbol's value first, if it's an integer or a string holding a number (`COUNT = 5`, then `PRINT COUNT+1`), then the machine's symbol. For DCL string values use `WRITE SYS$OUTPUT`. |
| `TIME command` | Run a command and say how long it took. |
| `TEST n` | govax's internal test sequences. |

`BOOT` and `ROM` are reserved and not implemented.

---

## 7. Command procedures and the DCL language

govax's command interpreter is DCL, and command procedures work as they
do on VMS. This section is a complete guide to the language as govax
implements it. If you already write DCL procedures, skim the headings
and read [Differences from VMS DCL](#differences-from-vms-dcl).

### Running a procedure

```text
@file-spec[/OUTPUT=file] [p1 [p2 ... p8]]
```

The file's type defaults to `.COM`. It may be on a volume or the host
(quote a host name: `@"build.com"`). The procedure runs at a new
*command level*, one deeper than the one that ran it; the terminal is
level 0, and procedures may nest 32 deep. `/OUTPUT` sends its output to
a file (`NL:` discards it). A volume file is made when the procedure
starts, a print file written a line at a time, so the procedure's own
DIRECTORY lists it.

As on VMS, the procedure and its `/OUTPUT` file are process-permanent
files: while it runs, `SYS$INPUT` names the procedure (an image it runs
reads the data lines after the command, by `LIB$GET_INPUT` or RMS) and
`SYS$OUTPUT` the `/OUTPUT` file (an image's output goes there too).
`SYS$ERROR` is still the terminal, so an image's error messages, which
go to both, are also shown at the terminal, as are DCL's.

Up to eight parameters become the local symbols `P1` to `P8` (those not
given are `""`). Unquoted parameters are made uppercase; quotes keep
blanks and case, and `""` passes an empty one:

```text
VAX> @SETUP "Test Run" fast
```

gives `P1` the value `Test Run` and `P2` the value `FAST`.

### The lines of a procedure

| Line | Meaning |
| --- | --- |
| `$ command` | A command. Blanks after the `$` are ignored. |
| `$! text` | A comment. `!` also starts a comment after a command. |
| `$ label:` or `$ label: command` | A label, for `GOTO`, `GOSUB`, and `CALL`. |
| `... -` | A line ending in `-` continues on the next line, which has no `$`. |
| a line without `$` | **Data**, for whatever reads the procedure's input (see [Data lines](#data-lines-and-sysinput)). |

### Symbols

A symbol holds an integer or a string.

```text
$ COUNT = 0                      ! local integer
$ NAME = "Ada"                   ! local string
$ TOTAL == COUNT + 1             ! global (==)
$ GREETING := Hello there        ! string assignment, := (value uppercased: HELLO THERE)
$ MSG :== "Mixed Case kept"      ! global string assignment
$ SD :== SET DEFAULT             ! a symbol as a command abbreviation
$ FO*RTH :== $DUA0:[TOOLS]FORTH.EXE   ! foreign command; FO, FOR, FORT, FORTH all work
```

- `=` and `==` take an *expression* (below). `:=` and `:==` take the rest
  of the line as a string: unquoted text is uppercased and its blanks
  compressed, quoted text is kept.
- **Local** symbols (`=`, `:=`) belong to their command level and go
  when the procedure ends. A procedure sees the local symbols of the
  levels that ran it. **Global** symbols (`==`, `:==`) last until you
  delete them. A local symbol hides a global one of the same name.
- A `*` in the name marks the shortest abbreviation that still means it.
- When the first word of a command is a symbol, its value replaces it.
  So a symbol can rename or abbreviate a command, and a value starting
  with `$` makes a foreign command (the image runs with the rest of the
  line as its command text).
- `SHOW SYMBOL` displays symbols; `DELETE/SYMBOL` deletes them.

### Expressions

The value of an `=` or `==` assignment, and the condition of an `IF`, is
an expression.

**Operands:**

| Operand | Examples |
| --- | --- |
| integers (32-bit, wrapping) | `13`, `-5`, `%X1F` (hex), `%O17` (octal), `%D10` |
| strings | `"in quotes"`, with `""` for a quote |
| symbols | `COUNT`, `P1` |
| lexical functions | `F$LENGTH(NAME)` |

**Operators**, from first done to last (parentheses group):

| Operators | Meaning |
| --- | --- |
| `+` `-` (unary) | sign |
| `*` `/` | multiply, divide (integer) |
| `+` `-` | add, subtract; on strings, concatenate and remove the first occurrence |
| `.EQ.` `.NE.` `.LT.` `.LE.` `.GT.` `.GE.` | compare integers |
| `.EQS.` `.NES.` `.LTS.` `.LES.` `.GTS.` `.GES.` | compare strings |
| `.NOT.` | bitwise not |
| `.AND.` | bitwise and |
| `.OR.` | bitwise or |

A comparison is 1 when true and 0 when false. **A value is true when it
is odd**: an odd integer, a string that holds an odd number, or a string
that starts with `T` or `Y` (either case). Where an integer is wanted, a
string holding a number is that number, and any other string is 0 (or 1
if it starts with T or Y). Where a string is wanted, an integer becomes
its decimal digits.

```text
$ BALANCE = 142 * 14
$ WEEKEND = "Saturday, " + "Sunday"
$ BIG = COUNT .GT. 10 .AND. NAME .EQS. "TOTAL"
$ SHORT = "ABCDEF" - "CD"            ! "ABEF"
```

### Symbol substitution

Anywhere in a command, a symbol's name between apostrophes is replaced by
its value before DCL reads the command:

```text
$ FILE = "REPORT.LIS"
$ TYPE 'FILE'
$ @NEXT 'P1' 'P2'
$ COPY 'NAME'.MAR [BACKUP]         ! the closing apostrophe may be left off at a word's end
```

Inside quotes, use two apostrophes before the name and one after:

```text
$ WRITE SYS$OUTPUT "Creating ''NAME'.DAT"
```

A lexical function can be substituted too: `'F$LENGTH(X)'`. An undefined
symbol is replaced by nothing. Substitution happens first, so a symbol can
supply any part of a command, the verb included; and a value that itself
contains `'NAME'` is substituted again.

`&NAME` is a second kind of substitution, done later, after the command
is uppercased: the value keeps its case and stays a single parameter even
with blanks in it. It's useful for passing a value through unchanged:

```text
$ COUNT = 1
$ TYPE &P'COUNT'                   ! 'COUNT' first, giving &P1, then P1's value
```

Data lines and debugger commands are never substituted.

### Statuses: $STATUS and $SEVERITY

Every command ends with a VMS condition value, which DCL keeps in the
global symbol `$STATUS`, with its severity (the low three bits) in
`$SEVERITY`:

| Severity | Meaning |
| --- | --- |
| 0 | warning |
| 1 | success |
| 2 | error |
| 3 | informational |
| 4 | severe (fatal) error |

So `$STATUS` is odd (true) after a command that succeeded:

```text
$ COPY 'P1' SAVE.DAT
$ IF .NOT. $STATUS THEN WRITE SYS$OUTPUT "The copy failed"
```

After `RUN`, `$STATUS` is the image's exit status (its `R0` at exit, or
what it passed to `$EXIT`); after `@`, the procedure's status. Bit 28
(`%X10000000`) set means the message has already been shown. `SHOW
SYMBOL`, `IF`, `GOTO`, and `CONTINUE` leave `$STATUS` as it was, so you
can test the status of the command before them. `F$MESSAGE($STATUS)`
gives the message text.

### Error handling: ON and SET NOON

By default, a procedure ends when one of its commands fails with an
error or a severe error; a warning doesn't end it. An error also ends
the procedures that ran this one. `ON` changes that:

```text
ON WARNING THEN command          ! act on warnings, errors, and severe errors
ON ERROR THEN command            ! act on errors and severe errors
ON SEVERE_ERROR THEN command     ! act on severe errors only
ON CONTROL_Y THEN command        ! act on Ctrl/Y
```

The command runs in place of the default action (usually `EXIT`,
`CONTINUE`, or `GOTO` a cleanup label), once: after it, the default is
back until the next `ON`. (`ON CONTROL_Y` lasts until it's changed.)
Each command level has its own setting. `SET NOON` turns checking off
entirely; `$STATUS` is still set. `SET ON` turns it back on.

```text
$ ON ERROR THEN GOTO CLEAN_UP
$ ON WARNING THEN CONTINUE
```

`EXIT status` ends a procedure with a status of your choosing, which
becomes `$STATUS` in the level that ran it; a failure status's message is
shown there unless bit 28 says it was shown already.

### IF, THEN, ELSE, ENDIF

```text
$ IF expression THEN [$] command

$ IF expression
$ THEN
$     commands
$ [ELSE
$     commands]
$ ENDIF
```

The single-line form runs one command when the expression is true. In
the block form, `THEN` must be the command right after the `IF`. Blocks
nest up to 15 deep. The commands of the branch not taken are skipped
without being read, so their symbols aren't substituted.

```text
$ IF P1 .EQS. "" THEN INQUIRE P1 "File name"
$ IF F$SEARCH(P1) .EQS. ""
$ THEN
$     WRITE SYS$OUTPUT "No such file: ", P1
$     EXIT 1
$ ELSE
$     TYPE 'P1'
$ ENDIF
```

### GOTO and labels

```text
$ COUNT = 0
$ LOOP:
$   COUNT = COUNT + 1
$   WRITE SYS$OUTPUT "Pass ", COUNT
$   IF COUNT .LT. 3 THEN GOTO LOOP
```

A label is a name and a colon at the start of a command line. `GOTO`
goes to the label it has passed most recently, or else searches forward.
A label inside an `IF` block or a subroutine that the `GOTO` isn't in
can't be reached. A missing label is `%DCL-W-USGOTO`, and ends the
procedure.

### Subroutines: GOSUB and CALL

`GOSUB` calls a subroutine in the same procedure at the *same* command
level; it sees the procedure's local symbols. `RETURN` comes back:

```text
$ GOSUB SHOW_COUNT
$ EXIT
$ SHOW_COUNT:
$   WRITE SYS$OUTPUT "Count is ", COUNT
$   RETURN
```

`CALL` runs a subroutine at a *new* command level, as `@` runs a file:
it gets its own `P1` to `P8`, its own local symbols and labels, and its
own `ON` setting. The subroutine is the lines between `label: SUBROUTINE`
and `ENDSUBROUTINE`, which are skipped when the procedure runs through
them:

```text
$ CALL GREET "Hello there" Ada
$ EXIT
$ GREET: SUBROUTINE
$     WRITE SYS$OUTPUT P1, ", ", P2
$ ENDSUBROUTINE
```

`CALL/OUTPUT=file` sends the subroutine's output to a file. `RETURN
status` and `EXIT status` set the status returned.

### Data lines and SYS$INPUT

A line without a `$` is data. A program a procedure runs reads them as
its input (`SYS$INPUT`), whether it reads with `LIB$GET_INPUT`, RMS
`$GET`, or a terminal `$QIO`. The program reaches end of file at the next
`$` line. `TYPE SYS$INPUT` and `CREATE` read data lines the same way, and
so do the debugger and the interactive assembler when a procedure starts
them:

```text
$ RUN GREET
Ada Lovelace
$ CREATE NOTES.TXT
First line
Second line
$ TYPE NOTES.TXT
```

Data nothing reads is skipped after an image, with a `%DCL-W-SKPDAT`
warning.

To give a program data lines that start with `$`, put them between
`DECK` and `EOD`:

```text
$ TYPE SYS$INPUT
$ DECK
$ This line starts with a dollar sign, and is typed.
$ EOD
```

`DECK/DOLLARS=string` ends the data at a line starting with `string`
instead.

### File I/O: OPEN, READ, WRITE, CLOSE, INQUIRE

```text
OPEN [/READ] [/WRITE] [/APPEND] [/SHARE[=READ|WRITE]] [/ERROR=label] logical-name file
READ [/END_OF_FILE=label] [/ERROR=label] [/PROMPT=string] logical-name symbol
WRITE [/ERROR=label] [/UPDATE] [/SYMBOL] logical-name expression[,...]
CLOSE [/ERROR=label] logical-name
INQUIRE [/LOCAL|/GLOBAL] [/[NO]PUNCTUATION] symbol [prompt]
```

`OPEN` opens a file (default type `.DAT`) and defines a process logical
name for it. `/READ` (the default) reads an existing file; `/WRITE` makes
a new version; `/APPEND` adds to the end; `/READ/WRITE` reads and
updates. `READ` reads a record into a symbol, keeping its case and
blanks. `WRITE` writes one record made of its expressions' values.
`CLOSE` closes the file. A file stays open until it's closed, even after
the procedure ends. As on VMS, the file is a process-permanent file: a
program that opens the logical name (`$OPEN` or `$CREATE`) reads or
writes the same file, at the same place, as `READ` and `WRITE` do.

`SYS$INPUT`, `SYS$OUTPUT`, `SYS$ERROR`, and `SYS$COMMAND` need no `OPEN`.
`READ SYS$COMMAND` and `INQUIRE` read the terminal, even inside a
procedure.

```text
$ OPEN IN NAMES.DAT
$ LOOP:
$   READ/END_OF_FILE=DONE IN NAME
$   WRITE SYS$OUTPUT "Hello, ", NAME
$   GOTO LOOP
$ DONE:
$ CLOSE IN
$
$ OPEN/WRITE OUT RESULTS.DAT
$ WRITE OUT "Count is ", COUNT
$ CLOSE OUT
$
$ INQUIRE ANSWER "Continue (Y/N)"
$ IF .NOT. ANSWER THEN EXIT
```

`/END_OF_FILE`, `/ERROR`, and the `OPEN` and `CLOSE` `/ERROR` labels
branch without a message and without taking the `ON` action.

### Lexical functions

A lexical function returns an integer or a string. Use it in an
expression, or between apostrophes. Arguments are expressions separated
by commas; an optional argument may be left out, keeping its comma. A
function with no arguments still takes `()`. Names may be abbreviated
while unique (`F$LEN` is `F$LENGTH`).

**Strings and data**

| Function | Returns |
| --- | --- |
| `F$LENGTH(string)` | its length |
| `F$EXTRACT(start, length, string)` | a substring, from offset 0 |
| `F$LOCATE(substring, string)` | its offset, or the string's length if absent |
| `F$ELEMENT(n, delimiter, string)` | the nth delimited element, from 0; the delimiter itself past the end |
| `F$EDIT(string, edits)` | edited: `COLLAPSE`, `COMPRESS`, `TRIM`, `UPCASE`, `LOWERCASE`, `UNCOMMENT` (a list) |
| `F$FAO(control, arg, ...)` | `$FAO` formatting: `!AS`, `!UL`, `!XL`, `!2UL`, ... |
| `F$INTEGER(expr)`, `F$STRING(expr)` | the value as an integer, or a string |
| `F$TYPE(symbol)` | `INTEGER`, `STRING`, `PROCESS_CONTEXT`, or `""` |
| `F$CVSI(bit, size, string)`, `F$CVUI(...)` | a bit field, signed or unsigned |

**The process and its environment**

| Function | Returns |
| --- | --- |
| `F$ENVIRONMENT(item)` | `DEFAULT`, `DEPTH`, `PROCEDURE`, `PROMPT`, `ON_SEVERITY`, `VERIFY_PROCEDURE`, `INTERACTIVE`, `MAX_DEPTH`, `CONTROL`, ... |
| `F$DIRECTORY()` | the default directory |
| `F$GETJPI(pid, item)` | a `$GETJPI` item (`""` for this process): `PID`, `PRCNAM`, `USERNAME`, `UIC`, `STATE`, `CURPRIV`, `FREP0VA`, `VIRTPEAK`, ... |
| `F$GETSYI(item)` | a `$GETSYI` item: `NODENAME`, `VERSION`, `BOOTTIME`, `HW_NAME`, the paging parameters, `PAGEFILE_FREE`, ... |
| `F$PID(context)` | each process's PID in turn, then `""` |
| `F$CONTEXT("PROCESS", context, item, value, op)` | selects the processes `F$PID` returns |
| `F$PROCESS()` | the process's name |
| `F$MODE()` | `INTERACTIVE` |
| `F$USER()` | the UIC, `[SYSTEM]` |
| `F$PRIVILEGE(privs)`, `F$SETPRV(privs)` | test, and set, privileges |
| `F$IDENTIFIER(id, type)` | converts an identifier's name and value |
| `F$VERIFY([proc] [,image])` | the verification settings, and sets them |
| `F$CONFIG(key [,op])` | govax's own: a configuration key (section 4) |

**Files, devices, names, messages, and time**

| Function | Returns |
| --- | --- |
| `F$SEARCH(file [,stream])` | the next file matching a wildcard spec, then `""` |
| `F$PARSE(file [,default] [,related] [,field] [,type])` | the full spec, or a field: `NODE`, `DEVICE`, `DIRECTORY`, `NAME`, `TYPE`, `VERSION`; type `SYNTAX_ONLY` doesn't check that the device and directory exist |
| `F$FILE_ATTRIBUTES(file, item)` | from the file header: `ORG`, `RFM`, `RAT`, `MRS`, `EOF`, `ALQ`, `CDT`, `RDT`, `PRO`, `UIC`, `FID`, ... |
| `F$GETDVI(device, item)` | a `$GETDVI` item: `EXISTS`, `MNT`, `VOLNAM`, `FREEBLOCKS`, ... |
| `F$DEVICE([name] [,class] [,type])` | the next matching device, then `""` |
| `F$TRNLNM(name [,table] [,index] [,mode] [,case] [,item])` | a logical name's translation, or an item about it |
| `F$LOGICAL(name)` | the older form of `F$TRNLNM` |
| `F$MESSAGE(status)` | a status's message text |
| `F$TIME()` | the date and time now |
| `F$CVTIME([time] [,format] [,field])` | a time converted to `COMPARISON`, `ABSOLUTE`, or `DELTA` form, or one field: `DATE`, `TIME`, `YEAR`, `WEEKDAY`, ... |

`F$CSID`, `F$GETQUI`, and `F$LICENSE` aren't provided (no cluster,
queues, or licenses). Here is a procedure that tours some of them:

```text
$ ! LEX.COM -- a tour of lexical functions
$ WRITE SYS$OUTPUT "Process:  ", F$PROCESS(), "  PID ", F$GETJPI("","PID")
$ WRITE SYS$OUTPUT "Node:     ", F$GETSYI("NODENAME"), " running ", F$GETSYI("VERSION")
$ NOW = F$TIME()
$ WRITE SYS$OUTPUT "Today is  ", F$CVTIME(NOW,,"WEEKDAY"), ", ", F$CVTIME(NOW,"ABSOLUTE","DATE")
$ LIST = "RED,GREEN,BLUE"
$ I = 0
$ LOOP:
$   COLOR = F$ELEMENT(I, ",", LIST)
$   IF COLOR .EQS. "," THEN GOTO DONE
$   WRITE SYS$OUTPUT F$FAO("!2UL. !AS (!UL letters)", I+1, F$EDIT(COLOR,"LOWERCASE"), F$LENGTH(COLOR))
$   I = I + 1
$   GOTO LOOP
$ DONE:
$ SPEC = "DUA0:[WORK.SUB]REPORT.LIS;3"
$ WRITE SYS$OUTPUT F$PARSE(SPEC,,,"NAME","SYNTAX_ONLY"), " / ", F$PARSE(SPEC,,,"DIRECTORY","SYNTAX_ONLY")
$ WRITE SYS$OUTPUT F$MESSAGE(%X1C)
$ WRITE SYS$OUTPUT "Hex: ", F$FAO("!XL", 255), "  Upcase: ", F$EDIT("  mixed   Case ","COMPRESS,TRIM,UPCASE")
```

```text
VAX> @LEX
Process:  SYSTEM  PID 00000301
Node:     GOVAX running V7.3
Today is  Tuesday, 20-OCT-2026
 1. red (3 letters)
 2. green (5 letters)
 3. blue (4 letters)
REPORT / [WORK.SUB]
%SYSTEM-F-EXQUOTA, process quota exceeded
Hex: 000000FF  Upcase: MIXED CASE
```

### Verification: SET VERIFY

`SET VERIFY` shows each line of a procedure as it runs (after `'SYMBOL'`
substitution); `SET NOVERIFY` stops it. `SET VERIFY=(PROCEDURE,IMAGE)`
chooses command lines, data lines, or both. Skipped branches of an `IF`
aren't shown. `SET PREFIX "(!5%T) "` puts the time before each line. A
procedure that begins with

```text
$ ! 'F$VERIFY(0)'
```

turns verification off without that line being shown.

### Ctrl/Y in procedures

In a procedure, Ctrl/Y (and Ctrl/C, if no program takes it) is DCL's:

- If the procedure has an `ON CONTROL_Y THEN command`, the command runs.
- Otherwise the procedure is interrupted and you get the prompt. Commands
  you type run in the procedure's context (with its symbols). `CONTINUE`
  resumes it (and the image it was running, if any); `EXIT` or `STOP`
  ends every procedure; a command that runs an image or a procedure ends
  them and then runs.

`SET NOCONTROL=Y` makes Ctrl/Y do nothing.

### A complete example: BUILD.COM

This procedure assembles, links, and runs a MACRO program, keeps a log
of what it built, and reports failures. It uses parameters, `ON`,
lexical functions, an `IF` block, `GOSUB`, and file I/O.

```text
$! BUILD.COM -- assemble, link, and run a MACRO program.
$!
$! Usage:  @BUILD name [DEBUG]
$!
$ ON ERROR THEN GOTO FAILED
$ IF P1 .EQS. "" THEN INQUIRE P1 "Program name"
$ NAME = F$PARSE(P1,,,"NAME")
$ SOURCE = F$SEARCH(NAME + ".MAR")
$ IF SOURCE .EQS. ""
$ THEN
$     WRITE SYS$OUTPUT "No source file for ", NAME
$     EXIT 1
$ ENDIF
$ WRITE SYS$OUTPUT "Building ", SOURCE
$ QUALS = ""
$ IF P2 .EQS. "DEBUG" THEN QUALS = "/DEBUG"
$ MACRO 'NAME''QUALS'
$ LINK 'NAME''QUALS'
$ GOSUB LOG_IT
$ RUN/NODEBUG 'NAME'
$ WRITE SYS$OUTPUT "''NAME' exited with status ", $STATUS
$ EXIT
$!
$ LOG_IT:
$ OPEN/APPEND/ERROR=NEW_LOG LOG BUILD.LOG
$ GOTO WRITE_LOG
$ NEW_LOG:
$ OPEN/WRITE LOG BUILD.LOG
$ WRITE_LOG:
$ WRITE LOG F$TIME(), " built ", NAME, QUALS
$ CLOSE LOG
$ RETURN
$!
$ FAILED:
$ WRITE SYS$OUTPUT "Build of ''NAME' failed: ", F$MESSAGE($STATUS)
$ EXIT $STATUS
```

```text
VAX> @BUILD HELLO
Building DUA0:[HELLO]HELLO.MAR;1
Hello, world!
HELLO exited with status %X00000001
VAX> @BUILD NOSUCH
No source file for NOSUCH
VAX> TYPE BUILD.LOG
20-OCT-2026 09:57:39.58 built FACT
20-OCT-2026 09:57:40.15 built HELLO
```

With `SET VERIFY`, the same run shows each line as it's executed. Note
that the false `IF` block's lines are skipped, and the `GOSUB`'s lines
appear where they run:

```text
VAX> SET VERIFY
VAX> @BUILD HELLO
...
$ IF SOURCE .EQS. ""
$ ENDIF
$ WRITE SYS$OUTPUT "Building ", SOURCE
Building DUA0:[HELLO]HELLO.MAR;1
$ QUALS = ""
$ IF P2 .EQS. "DEBUG" THEN QUALS = "/DEBUG"
$ MACRO HELLO
$ LINK HELLO
$ GOSUB LOG_IT
$ LOG_IT:
$ OPEN/APPEND/ERROR=NEW_LOG LOG BUILD.LOG
$ GOTO WRITE_LOG
$ WRITE_LOG:
$ WRITE LOG F$TIME(), " built ", NAME, QUALS
$ CLOSE LOG
$ RETURN
$ RUN/NODEBUG HELLO
Hello, world!
$ WRITE SYS$OUTPUT "HELLO exited with status ", $STATUS
HELLO exited with status %X00000001
$ EXIT
```

### Differences from VMS DCL

govax's DCL was checked against VMS 7.3 with probe procedures, and
behaves as VMS's in the cases they cover. The differences to know about:

- The prompt is `VAX>` (change it with `SET PROMPT` or
  `vax.console.prompt`); a subprocess's is `$`.
- At the terminal, `EXIT` ends govax, as `LOGOUT` would end a VMS session.
- Commands that aren't VMS's: the machine commands of section 6.9, `PRINT`,
  `TIME`, and `F$CONFIG`.
- Commands VMS has that govax doesn't include `SUBMIT`, `PRINT` (VMS's,
  for queues), `SET TERMINAL`, `SET PROCESS`, `SET FILE`, `SORT`, `DIFFERENCES`, `SEARCH`, `EDIT`, `INSTALL`, `ANALYZE/DISK`, and
  the compilers.
- `F$MODE()` is always `INTERACTIVE`; there's no batch.
- govax's `@` reads a procedure from a host file as well as a volume.

---

## 8. Programming in MACRO-32

govax's `MACRO` command is a MACRO-32 assembler that behaves as VAX
MACRO does: the same source language and directives, the same macro
facility, the same object modules (byte for byte), and the same listing
layout. This section shows how to write and build programs with it,
through a series of complete examples. If MACRO-32 is new to you, read it
in order; if you know it, the examples and the qualifier reference are
what you need.

### The MACRO command

```text
MACRO source[/HOST] [/OBJECT[=file] | /NOOBJECT] [/LIST[=file] | /NOLIST]
      [/LIBRARY=(mlb,...)] [/SHOW=(...) | /NOSHOW=(...)]
      [/CROSS_REFERENCE[=(...)]] [/DEBUG[=(...)] | /NODEBUG]
      [/ENABLE=(...)] [/DISABLE=(...)]
```

| Qualifier | Meaning |
| --- | --- |
| `/OBJECT[=file]` | Write the object module (the default). It's named after the source, with type `.OBJ`, beside it. |
| `/NOOBJECT` | Check the source without writing an object. |
| `/LIST[=file]` | Write a listing (`.LIS`): the source with the generated code, the symbol table, the psect synopsis, and statistics. |
| `/SHOW=(...)`, `/NOSHOW=(...)` | Listing options: `BINARY`, `CALLS`, `CONDITIONALS`, `DEFINITIONS`, `EXPANSIONS`. By default all but `BINARY` and `EXPANSIONS` are on. `/SHOW=EXPANSIONS` lists every line macros generate. |
| `/CROSS_REFERENCE[=(...)]` | End the listing with a cross reference of `SYMBOLS`, `MACROS`, `OPCODES`, `DIRECTIVES`, `REGISTERS`, or `ALL` (default: symbols and macros). |
| `/DEBUG[=(...)]`, `/NODEBUG` | What the object carries for the debugger: `TRACEBACK` (the default), `SYMBOLS`, `ALL` (what `/DEBUG` alone means), or `NONE`. Use `/DEBUG` for source-line debugging. |
| `/ENABLE=(...)`, `/DISABLE=(...)` | Assembler functions, as `.ENABLE`/`.DISABLE`: `ABSOLUTE`, `DEBUG`, `GLOBAL`, `SUPPRESSION`, `TRACEBACK`, `TRUNCATION`, `VECTOR`. |
| `/LIBRARY=(...)` | Macro libraries (`.MLB`) to search before the system macro library (VMS writes this as `MACRO PROG+LIB/LIBRARY`). |

Every error in the source is reported, with its line, and then no object
is written, so a failed assembly never replaces a good object:

```text
%DCL-E-ASSEMBLING, Assembling lines.mar: line 27: VAX-E-BRANCHRANGE, Branch displacement 135 out of range
govax: DCL-E-ASMERRORS, 1 error(s) assembling lines.mar; no object file written
```

Warnings are reported, and the object is still written.

### Example 1: Hello, world, explained

```asm
        .TITLE  HELLO   Hello, world
        .IDENT  /V1.0/

        .PSECT  DATA, WRT, NOEXE, LONG
MSG:    .ASCID  /Hello, world!/

        .PSECT  CODE, NOWRT, EXE, LONG
        .ENTRY  HELLO, ^M<>
        PUSHAQ  MSG
        CALLS   #1, G^LIB$PUT_OUTPUT
        RET

        .END    HELLO
```

Line by line:

- `.TITLE` names the module (`HELLO`) and gives the listing a title.
  `.IDENT` is its version, which the linker's map and `ANALYZE` show.
- `.PSECT` starts a *program section*. Sections with the same name, from
  all the modules in a link, are gathered together. The attributes say
  what the section is: `WRT` (writable) and `NOEXE` for data, `NOWRT` and
  `EXE` for code. `LONG` aligns it on a longword. Keeping code and data
  apart lets the linker make the code read-only.
- `.ASCID` stores a string and, in front of it, a *string descriptor*:
  the 8-byte structure (length, type, class, address) that VMS routines
  take for a string argument. `MSG` is the descriptor's address.
- `.ENTRY HELLO, ^M<>` defines a procedure entry point. Its first word
  is the *entry mask*: the registers (R2 to R11) the procedure uses, which
  `CALLS` saves and `RET` restores. `^M<>` saves none. (R0 and R1 are
  scratch registers, never saved.)
- `PUSHAQ MSG` pushes the descriptor's address: the routine's one
  argument.
- `CALLS #1, G^LIB$PUT_OUTPUT` calls the run-time library routine with
  one argument from the stack. `G^` (general addressing) lets the linker
  point the call into a shareable image, here LIBRTL.
- `RET` returns. The routine's status is in R0: `LIB$PUT_OUTPUT` left
  `SS$_NORMAL` (1) there, so the image exits with success.
- `.END HELLO` ends the source, and names the *transfer address*, where
  the image starts.

The VAX calling standard behind this is worth knowing, because every
example uses it:

- A caller pushes the arguments in reverse order, then `CALLS #n, routine`.
  (`CALLG arglist, routine` passes a prebuilt argument list instead.)
- In the called routine, `AP` (the argument pointer) addresses the
  argument list: `4(AP)` is the first argument, `8(AP)` the second, and
  `(AP)` holds the count.
- `FP` (the frame pointer) addresses the call frame that `CALLS` built;
  `(FP)` is the routine's condition handler slot.
- A routine returns a value, or a status, in R0 (R0 and R1 for 64 bits).
  A status is a success when its low bit is set, so `BLBC R0, error`
  ("branch on low bit clear") is the usual test.
- Strings are passed by descriptor, most other values by reference
  (their address), and some by value.

### Example 2: Loops, recursion, and $FAO

This program prints a table of factorials. It shows a loop with local
labels, a recursive procedure that takes an argument, and the `$FAO_S`
system service macro, which formats text as `printf` does.

```asm
        .TITLE  FACT    Print a table of factorials
        .IDENT  /V1.0/

; Prints N! for N = 1 to 10, formatting each line with $FAO.

        .PSECT  DATA, WRT, NOEXE, LONG
CTRSTR: .ASCID  /!2UL!! = !UL/          ; $FAO control string ("!!" is a "!")
BUFFER: .BLKB   80                      ; where $FAO formats a line
OUTDSC: .LONG   80                      ; descriptor of BUFFER
        .ADDRESS BUFFER
COUNT:  .LONG   0                       ; N
RESULT: .LONG   0                       ; N!

        .PSECT  CODE, NOWRT, EXE, LONG

        .ENTRY  FACT, ^M<R2>
        MOVL    #1, R2                  ; N = 1
10$:    MOVL    R2, COUNT
        PUSHL   R2                      ; FACTORIAL(N)
        CALLS   #1, FACTORIAL
        MOVL    R0, RESULT
        MOVL    #80, OUTDSC             ; reset the buffer's length
        $FAO_S  CTRSTR=CTRSTR, OUTLEN=OUTDSC, OUTBUF=OUTDSC, -
                P1=COUNT, P2=RESULT
        BLBC    R0, 90$
        PUSHAQ  OUTDSC
        CALLS   #1, G^LIB$PUT_OUTPUT
        AOBLEQ  #10, R2, 10$            ; next N, through 10
        MOVL    #1, R0                  ; SS$_NORMAL
90$:    RET

; FACTORIAL(n): returns n! in R0, by recursion.
        .ENTRY  FACTORIAL, ^M<>
        MOVL    4(AP), R0               ; n
        CMPL    R0, #1
        BLEQ    20$                     ; 0! = 1! = 1
        SUBL3   #1, R0, -(SP)           ; FACTORIAL(n-1)
        CALLS   #1, FACTORIAL
        MULL2   4(AP), R0               ; times n
        RET
20$:    MOVL    #1, R0
        RET

        .END    FACT
```

```text
$ govax macro fact.mar && govax link fact.obj && govax run fact.exe
 1! = 1
 2! = 2
 3! = 6
 4! = 24
 5! = 120
 6! = 720
 7! = 5040
 8! = 40320
 9! = 362880
10! = 3628800
```

Things to notice:

- **Local labels** like `10$` and `90$` are known only between two
  ordinary labels (here, within `FACT`), so each routine can reuse them.
- `FACT` uses R2, so its entry mask `^M<R2>` saves and restores it.
  `FACTORIAL` uses only R0, which needs no saving.
- `SUBL3 #1, R0, -(SP)` computes `n-1` straight onto the stack: the
  argument for the recursive call.
- `$FAO_S` is a system service macro. The `_S` form pushes its arguments
  on the stack and calls `SYS$FAO`. Its `P1`, `P2`, ... are the
  directives' arguments, pushed as longwords: `P1=COUNT` pushes the
  contents of `COUNT`. `OUTLEN=OUTDSC` makes `$FAO` write the formatted
  length into the descriptor's own length word, a common VMS idiom.
- `AOBLEQ` ("add one and branch if less or equal") is the VAX's loop
  instruction.
- If `$FAO` fails, the routine returns its status. The control string
  must be right: an unknown directive is `SS$_BADPARAM`, and the image
  would exit with `%SYSTEM-F-BADPARAM`.

### Example 3: Reading a file with RMS

Programs read and write files through RMS, by filling in a *file access
block* (FAB) and a *record access block* (RAB) and calling the RMS
services. The `$FAB` and `$RAB` macros build the blocks; `$OPEN`,
`$CONNECT`, `$GET`, and `$CLOSE` call the services. Remember that RMS
works on volume files, so run this on a mounted volume.

```asm
        .TITLE  LINES   Number the lines of a file
        .IDENT  /V1.0/

; Reads the file LINES.DAT a record at a time with RMS, and writes
; each record to the terminal with its line number in front.

        .PSECT  DATA, WRT, NOEXE, LONG

INFAB:  $FAB    FNM=<LINES.DAT>, -      ; the file to read
                FAC=<GET>               ; read access only
INRAB:  $RAB    FAB=INFAB, -            ; the record stream on it
                UBF=RECORD, -           ; where $GET puts a record
                USZ=RECSIZ              ; and how big that buffer is

RECSIZ = 132
RECORD: .BLKB   RECSIZ
LINENO: .LONG   0
CTRSTR: .ASCID  /!4UL  !AF/             ; line number, then counted text
OUTBUF: .BLKB   160
OUTDSC: .LONG   160
        .ADDRESS OUTBUF

        .PSECT  CODE, NOWRT, EXE, LONG

        .ENTRY  LINES, ^M<R2>
        $OPEN   FAB=INFAB               ; open the file
        BLBS    R0, 5$
        BRW     90$                     ; (too far for a BLBC)
5$:     $CONNECT RAB=INRAB              ; and connect a stream to it
        BLBS    R0, 10$
        BRW     90$

10$:    $GET    RAB=INRAB               ; the next record
        BLBS    R0, 20$
        BRW     80$                     ; RMS$_EOF at the end
20$:    INCL    LINENO
        MOVZWL  INRAB+RAB$W_RSZ, R2     ; its length
        MOVL    #160, OUTDSC
        $FAO_S  CTRSTR=CTRSTR, OUTLEN=OUTDSC, OUTBUF=OUTDSC, -
                P1=LINENO, P2=R2, P3=#RECORD
        PUSHAQ  OUTDSC
        CALLS   #1, G^LIB$PUT_OUTPUT
        BRB     10$

80$:    CMPL    R0, #RMS$_EOF           ; the end of the file is success
        BNEQ    90$
        $CLOSE  FAB=INFAB
        MOVL    #1, R0
90$:    RET

        .END    LINES
```

```text
VAX> SET DEFAULT DUA0:[DEMO]
VAX> COPY "lines.mar"/HOST *
VAX> COPY "lines.dat"/HOST *
VAX> MACRO LINES
VAX> LINK LINES
VAX> RUN LINES
   1  The VAX-11/780 was announced in October 1977.
   2  It was the first VAX.
   3  Its successors ran VMS for decades.
VAX> RENAME LINES.DAT GONE.DAT
VAX> RUN LINES
%RMS-E-FNF, file not found
VAX> SHOW SYMBOL $STATUS
  $STATUS == "%X00018292"
```

Notes:

- The program returns RMS's status when something fails, so the image's
  exit status is the RMS error, and DCL shows its message.
- `RAB$W_RSZ` (the length of the record `$GET` read) and `RMS$_EOF` come
  from govax's tables of VMS symbols, which the linker uses; you don't
  have to define them. (`$RABDEF` and the other definition macros are
  available too, to define such symbols in the module itself.)
- The RMS and `$FAO_S` macros expand to a good deal of code, so a
  byte-displacement branch (`BLBC`, `BRB`) over them can be out of range.
  The assembler says so (`BRANCHRANGE`); branch around with `BRW`, as
  here.
- `!AF` formats counted text: a length (`P2`) and an address (`P3`).

### Example 4: Terminal input, dynamic strings, and your own macros

This example reads a line with `LIB$GET_INPUT`, into a *dynamic string*
(a class D descriptor, whose memory the run-time library manages), and
uses two macros from a macro library of its own.

The macros, in `MYMAC.MAR`:

```asm
; MYMAC.MAR -- macros for MYMAC.MLB

; PRINT desc: writes the string a descriptor describes to SYS$OUTPUT.
        .MACRO  PRINT   DESC
        PUSHAQ  DESC
        CALLS   #1, G^LIB$PUT_OUTPUT
        .ENDM   PRINT

; CHECK: returns from the routine with R0 if R0 is a failure status.
        .MACRO  CHECK   ?OK
        BLBS    R0, OK
        RET
OK:
        .ENDM   CHECK
```

`?OK` makes `OK` a *created local label*: each use of `CHECK` gets a
label of its own (`30000$`, `30001$`, ...), so the macro can be used many
times in one routine.

The program, `GREET.MAR`:

```asm
        .TITLE  GREET   Ask for a name, and greet it
        .LIBRARY /MYMAC/                ; search MYMAC.MLB for macros

        .PSECT  DATA, WRT, NOEXE, LONG
PROMPT: .ASCID  /What is your name? /
NAME:   .WORD   0                       ; a dynamic string descriptor:
        .BYTE   14                      ;   DSC$K_DTYPE_T, text
        .BYTE   2                       ;   DSC$K_CLASS_D, so LIB$GET_INPUT
        .LONG   0                       ;   gets the memory for it
CTRSTR: .ASCID  /Hello, !AS!!/
BUFFER: .BLKB   100
OUTDSC: .LONG   100
        .ADDRESS BUFFER

        .PSECT  CODE, NOWRT, EXE, LONG
        .ENTRY  GREET, ^M<>
        PUSHAQ  PROMPT                  ; LIB$GET_INPUT(NAME, PROMPT)
        PUSHAQ  NAME
        CALLS   #2, G^LIB$GET_INPUT
        CHECK                           ; RMS$_EOF if Ctrl/Z was typed
        $FAO_S  CTRSTR=CTRSTR, OUTLEN=OUTDSC, OUTBUF=OUTDSC, P1=#NAME
        CHECK
        PRINT   OUTDSC
        RET
        .END    GREET
```

Build the macro library, then the program:

```text
VAX> LIBRARY/CREATE/MACRO MYMAC MYMAC
VAX> LIBRARY/LIST MYMAC.MLB
Directory of MACRO library DUA0:[DEMO]MYMAC.MLB;1 on 20-OCT-2026 10:09:28
Creation date:  20-OCT-2026 10:09:28      Creator:  govax Librarian
...
CHECK
PRINT
VAX> MACRO GREET
VAX> LINK GREET
VAX> RUN GREET
What is your name? Ada Lovelace
Hello, Ada Lovelace!
```

Instead of `.LIBRARY` in the source, you can name the library on the
command line: `MACRO GREET/LIBRARY=MYMAC`. For a one-off macro, just
define it in the source with `.MACRO` ... `.ENDM`.

`$DSCDEF` isn't among govax's own system macros (see
[System macros](#system-macros)), which is why this example writes the
descriptor's type and class as numbers.

### Example 5: A condition handler

VMS reports errors such as access violations as *conditions*, which a
procedure can catch by establishing a *condition handler*: storing the
handler's address in its call frame, at `(FP)`. This program reads
address 0, which is never accessible; the handler catches the access
violation and unwinds the call, so `RISKY` returns to `MAIN` with
`SS$_ACCVIO` in R0.

```asm
        .TITLE  HANDLER Catch an access violation with a condition handler
        $SSDEF                          ; SS$_ symbols

        .PSECT  DATA, WRT, NOEXE, LONG
CAUGHT: .ASCID  /Handler: caught an access violation, unwinding/
DONE:   .ASCID  /Main: back from RISKY, R0 = SS$_ACCVIO/

        .PSECT  CODE, NOWRT, EXE, LONG

        .ENTRY  MAIN, ^M<>
        CALLS   #0, RISKY               ; returns with SS$_ACCVIO in R0
        CMPL    R0, #SS$_ACCVIO
        BNEQ    10$
        PUSHAQ  DONE
        CALLS   #1, G^LIB$PUT_OUTPUT
        MOVL    #SS$_NORMAL, R0
10$:    RET

; RISKY establishes HANDLER, then reads address 0.
        .ENTRY  RISKY, ^M<>
        MOVAB   HANDLER, (FP)           ; establish the condition handler
        CLRL    R1
        MOVL    (R1), R0                ; access violation
        RET

; HANDLER(signal-array, mechanism-array): for ACCVIO, unwind to RISKY's
; caller with the condition in R0; otherwise resignal.
        .ENTRY  HANDLER, ^M<>
        MOVL    4(AP), R0               ; the signal array
        CMPL    4(R0), #SS$_ACCVIO      ; its condition
        BNEQ    20$
        PUSHAQ  CAUGHT
        CALLS   #1, G^LIB$PUT_OUTPUT
        MOVL    8(AP), R0               ; the mechanism array
        MOVL    #SS$_ACCVIO, 12(R0)     ; R0 for the routine unwound to
        $UNWIND_S                       ; to RISKY's caller
        RET
20$:    MOVL    #SS$_RESIGNAL, R0
        RET
        .END    MAIN
```

```text
$ govax run handler.exe
Handler: caught an access violation, unwinding
Main: back from RISKY, R0 = SS$_ACCVIO
```

Without the handler (take out the `MOVAB`), the access violation would
go unhandled: the traceback handler ends the image with its message and
a symbolic stack dump, innermost call first, each line a frame's module,
routine, and PC (relative to the routine, and absolute):

```text
%SYSTEM-F-ACCVIO, access violation, reason mask=00, virtual address=00000000, PC=00000423, PSL=03C00004
%TRACE-F-TRACEBACK, symbolic stack dump follows
module name     routine name                     line       rel PC    abs PC

HANDLER         RISKY                                      00000004  00000423
HANDLER         MAIN                                       00000009  00000409
```

The line column is filled in for an image linked `/DEBUG` (section 9).
An image linked `/NOTRACEBACK` has no traceback handler, so the
catch-all takes the condition. For a hardware exception's condition
(an access violation, a reserved instruction or operand, an arithmetic
trap), however it was raised, it follows the message with VMS's
"improperly handled condition" dump: the signal array beside ten
longwords of the stack from the program's SP, then the registers at
the exception. Linked `/NOTRACEBACK`, the program without its handler
prints:

```text
%SYSTEM-F-ACCVIO, access violation, reason mask=00, virtual address=00000000, PC=00000423, PSL=03C00004

  Improperly handled condition, image exit forced.

        Signal arguments              Stack contents

        Number = 00000005                00000000
        Name   = 0000000C                20000000
                 00000000                7FDFFFE4
                 00000000                7FDFFFD0
                 00000423                00000409
                 03C00004                00000000
                                         00000000
                                         20000000
                                         7FDFFFFC
                                         7FDFFFE8

        Register dump

        R0 = 00000000  R1 = 00000000  R2 = 00000000  R3 = 00000000
        R4 = 00000000  R5 = 00000000  R6 = 00000000  R7 = 00000000
        R8 = 00000000  R9 = 00000000  R10= 00000000  R11= 00000000
        AP = 7FDFFF7C  FP = 7FDFFF3C  SP = 7FDFFFB8  PC = 00000423
        PSL= 03C00004
```

Any other condition gets only its message. The FP and AP shown aren't
the program's: as on VMS, they are where VMS's own frames below the
condition's arrays would be.

`LIB$SIGNAL` and `LIB$STOP` raise conditions of your own, and
`LIB$ESTABLISH`, `LIB$REVERT`, and `LIB$SIG_TO_RET` work as on VMS.

### Example 6: A program that traces itself

The PSL's *T bit* (trace, bit 4) makes the processor take a *trace
fault* after each instruction, which VMS reports to the program as the
condition `SS$_TBIT`. A program can trace itself with it: set T, and a
condition handler is called once per instruction. This one counts the
instructions a loop runs:

```asm
        .TITLE  TRACER  Count the instructions a loop runs
        $SSDEF                          ; SS$_ symbols

        .PSECT  DATA, WRT, NOEXE, LONG
COUNT:  .LONG   0                       ; traces taken
SUM:    .LONG   0
FORMAT: .ASCID  /Sum !UL, !UL instruction!%S traced/
LINE:   .ASCID  /                                        /

        .PSECT  CODE, NOWRT, EXE, LONG

        .ENTRY  MAIN, ^M<>
        MOVAB   TRACER, (FP)            ; the handler that takes SS$_TBIT
        MOVL    #5, R1
        BISPSW  #^X10                   ; set PSL<T>
        CLRL    R0                      ; the first instruction traced
10$:    ADDL2   R1, R0
        SOBGTR  R1, 10$
        BICPSW  #^X10                   ; clear PSL<T>: the last one traced
        MOVL    R0, SUM
        $FAO_S  CTRSTR=FORMAT, OUTLEN=LINE, OUTBUF=LINE, P1=SUM, P2=COUNT
        PUSHAQ  LINE
        CALLS   #1, G^LIB$PUT_OUTPUT
        MOVL    #SS$_NORMAL, R0
        RET

; TRACER(signal-array, mechanism-array): count SS$_TBIT and continue;
; resignal anything else.
        .ENTRY  TRACER, ^M<>
        MOVL    4(AP), R0               ; the signal array
        CMPL    4(R0), #SS$_TBIT        ; its condition
        BNEQ    20$
        INCL    COUNT
        MOVL    #SS$_CONTINUE, R0       ; go on at the next instruction
        RET
20$:    MOVL    #SS$_RESIGNAL, R0
        RET
        .END    MAIN
```

```text
$ govax run tracer.exe
Sum 15, 12 instructions traced
```

The twelve are the `CLRL`, five each of `ADDL2` and `SOBGTR`, and the
`BICPSW`. The rules, which are the VAX architecture's and VMS's:

- Tracing starts with the second instruction after the one that sets
  T, and the instruction that clears it is the last traced.
- `SS$_TBIT`'s signal array has no arguments of its own: the PC is the
  next instruction to run, and the PSL has T set. A handler that
  returns `SS$_CONTINUE` goes on there; it can change the signal
  array's PC to go elsewhere, or clear T in its PSL (bit 4) to stop the
  tracing.
- Handlers, and AST routines, run untraced, and the program's T comes
  back when they return.
- `CALLS` and `CALLG` save T clear in the frame and `RET` restores it,
  so tracing stops at the `RET` of a routine called while tracing,
  unless a handler sets T in that frame's saved PSW.
- Integer overflow and the other arithmetic traps come first; the trace
  of that instruction follows the trap's handler.
- With no handler, `SS$_TBIT` is severe and ends the image:
  `%SYSTEM-F-TBIT, T-bit pending trap at PC=00000409, PSL=03C00014`,
  with the traceback dump.
- `LIB$SIGNAL` or `LIB$STOP` called with T set ends the image with the
  condition's message and the condition as its status, its handlers
  never called, as VMS 7.1 does.

A system service or run-time library call is traced as govax runs it,
which isn't VMS's way: VMS's routines are VAX code, and a traced program
sees each of their instructions (about 15 for a LIBRTL routine), where
govax's routines are Go behind a stub of three instructions. A service's
vector entry gives three traces on both, but at different offsets
(`SYS$GETTIM+4` on govax, `+6` on VMS, whose `CHMK` is four bytes).
`docs/DEVIATIONS.md` has the details.

### The listing

`/LIST` writes a listing laid out as VAX MACRO's. Here is the first page
of `HELLO.LIS`:

```text
HELLO                           Fixture 9: hello, world                  20-OCT-2026 09:56:08  govax MACRO V0.0-0          Page   1
V1.0                                                                     20-OCT-2026 09:56:08  DUA0:[HELLO]HELLO.MAR;1           (1)

                                     0000     1         .TITLE  HELLO   Fixture 9: hello, world
                                     0000     2         .IDENT  /V1.0/
                                     0000     3
                                 00000000     6         .PSECT  DATA, WRT, NOEXE, LONG
2C 6F 6C 6C 65 48 00000008'010E0000' 0000     7 MSG:    .ASCID  /Hello, world!/
               21 64 6C 72 6F 77 20  000E
                                     0015     8
                                 00000000     9         .PSECT  CODE, NOWRT, EXE, LONG
                               0000  0000    10         .ENTRY  HELLO, ^M<>
                   00000000'EF   7F  0002    11         PUSHAQ  MSG
              00000000'GF   01   FB  0008    12         CALLS   #1, G^LIB$PUT_OUTPUT
                                 04  000F    13         RET
                                     0010    14
                                     0010    15         .END    HELLO
```

The generated bytes are on the left, read right to left as the VAX
stores them (lowest address on the right), then each line's offset in
its psect. An apostrophe marks a value the linker will relocate; `G`
marks a reference to a global symbol. Later pages hold the symbol table,
the psect synopsis, and statistics.

### Source language summary

govax's MACRO accepts VAX MACRO's language. A quick reference:

**Statements** have the form `label: opcode operands ; comment`. A line
ending in `-` (before any comment) continues on the next.

**Symbols**: `NAME = value` (local), `NAME == value` (global), `NAME:`
(a label), `NAME::` (a global label), `10$:` (a local label).

**Addressing modes:**

| Syntax | Mode |
| --- | --- |
| `R3` | register |
| `(R3)` | register deferred |
| `(R3)+`, `-(R3)`, `@(R3)+` | autoincrement, autodecrement, autoincrement deferred |
| `#5`, `S^#5`, `I^#500` | literal (short literal or immediate) |
| `8(R3)`, `@8(R3)`, `B^8(R3)` | displacement, displacement deferred, with a size |
| `LABEL`, `@LABEL` | relative (PC-relative), relative deferred |
| `@#^X200` | absolute |
| `G^NAME` | general: relative or absolute, as the linker decides |
| `TABLE[R4]` | indexed |

**Expressions** use `+ - * /`, `@` (shift), `&` (and), `!` (or), `\`
(exclusive or), `^C` (complement), angle brackets for grouping, and
radix operators `^X`, `^D`, `^O`, `^B`. Numbers are decimal by default.
Binary operators are applied left to right with no precedence: write
`1+<2*3>` for 7. `^M<R2,R3>` is a register mask; `^A/text/` is ASCII.

**Directives** (VAX MACRO's), among them:

| Kind | Directives |
| --- | --- |
| Module | `.TITLE`, `.IDENT`, `.SBTTL`, `.END` |
| Sections | `.PSECT`, `.SAVE_PSECT`, `.RESTORE_PSECT`, `.ALIGN` |
| Data | `.BYTE`, `.WORD`, `.LONG`, `.QUAD`, `.OCTA`, `.ADDRESS`, `.ASCII`, `.ASCIC`, `.ASCID`, `.ASCIZ`, `.F_FLOATING`, `.D_FLOATING`, `.G_FLOATING`, `.H_FLOATING`, `.PACKED`, `.BLKB`, `.BLKW`, `.BLKL`, `.BLKQ`, ... |
| Procedures | `.ENTRY`, `.MASK`, `.TRANSFER` |
| Symbols | `.GLOBAL`, `.EXTERNAL`, `.WEAK`, `.DEFAULT`, `.ENABLE`, `.DISABLE` |
| Conditionals | `.IF`, `.IIF`, `.IF_TRUE`, `.IF_FALSE`, `.IF_TRUE_FALSE`, `.ENDC` |
| Macros | `.MACRO`, `.ENDM`, `.MEXIT`, `.MDELETE`, `.MCALL`, `.LIBRARY`, `.NARG`, `.NCHR`, `.NTYPE`, `.IRP`, `.IRPC`, `.REPEAT`, `.ENDR`, `.INCLUDE` |
| Listing | `.SHOW`, `.NOSHOW`, `.LIST`, `.NLIST`, `.PAGE`, `.CROSS`, `.NOCROSS`, `.ERROR`, `.WARN`, `.PRINT` |

### System macros

When the source calls a macro it doesn't define, MACRO searches, in
order: the libraries named by `.LIBRARY` (the last named first), those
named by `/LIBRARY`, then the system macro library, STARLET. govax has
its own STARLET, written from DIGITAL's manuals and checked against VAX
MACRO's output; its macros generate the same code VAX MACRO's would. It
contains:

- **RMS**: `$FAB`, `$RAB`, `$NAM`, the XABs (`$XABALL`, `$XABDAT`,
  `$XABFHC`, `$XABITM`, `$XABKEY`, `$XABPRO`, `$XABRDT`, `$XABSUM`,
  `$XABTRM`), their `_STORE` forms, and the services `$OPEN`, `$CREATE`,
  `$CLOSE`, `$CONNECT`, `$DISCONNECT`, `$GET`, `$PUT`, `$FIND`,
  `$UPDATE`, `$DELETE`, `$TRUNCATE`, `$FLUSH`, `$FREE`, `$RELEASE`,
  `$REWIND`, `$READ`, `$WRITE`, `$SPACE`, `$WAIT`, `$PARSE`, `$SEARCH`,
  `$DISPLAY`, `$ERASE`, `$RENAME`, `$REMOVE`, `$ENTER`, `$EXTEND`,
  `$NXTVOL`.
- **System services**, in the `_S` form (arguments on the stack), and
  for most the plain form (an argument list in line) and the `_G` form
  (`CALLG`): `$ASSIGN_S`, `$DASSGN_S`, `$QIO_S`, `$QIOW_S`, `$EXIT_S`,
  `$HIBER_S`, `$FAO`, `$FAOL`, `$GETMSG`, `$PUTMSG`, `$ASCTIM`, `$BINTIM`,
  `$NUMTIM`, `$GETTIM`, `$SETIMR`, `$CANTIM`, `$SCHDWK`, `$WAKE`,
  `$SETEF`, `$CLREF`, `$READEF`, `$WAITFR`, `$WFLOR`, `$WFLAND`,
  `$ASCEFC`, `$DACEFC`, `$DLCEFC`, `$DCLAST`, `$SETAST`, `$DCLEXH`,
  `$CANEXH`, `$CREPRC`, `$DELPRC`, `$SUSPND`, `$RESUME`, `$FORCEX`,
  `$SETPRI`, `$SETPRN`, `$SETPRV`, `$GETJPI(W)`, `$GETSYI(W)`,
  `$GETDVI(W)`, `$CREMBX`, `$DELMBX`, `$ALLOC`, `$DALLOC`, `$CANCEL`,
  `$BRKTHRU(W)`, `$SNDOPR`, `$CRELNM`, `$DELLNM`, `$TRNLNM`, `$CRELNT`,
  `$CRELOG`, `$DELLOG`, `$TRNLOG`, `$EXPREG`, `$CRETVA`, `$DELTVA`,
  `$CNTREG`, `$ADJSTK`, `$ADJWSL`, `$SETPRT`, `$LCKPAG`, `$ULKPAG`,
  `$LKWSET`, `$ULWSET`, `$PURGWS`, `$CRMPSC`, `$MGBLSC`, `$DGBLSC`,
  `$UPDSEC(W)`, `$ENQ(W)`, `$DEQ`, `$GETLKI(W)`, `$CMKRNL`, `$CMEXEC`,
  `$SETEXV`, `$UNWIND`, `$SYNCH`, `$SETRWM`, `$ASCTOID`, `$IDTOASC`,
  `$CANWAK`.
- **Definitions**: `$ACCDEF`, `$ATRDEF`, `$BRKDEF`, `$CLIDEF`, `$DCDEF`,
  `$DEVDEF`, `$DVIDEF`, `$FABDEF`, `$FIBDEF`, `$IODEF`, `$JPIDEF`,
  `$LCKDEF`, `$LIBDEF`, `$LKIDEF`, `$LNMDEF`, `$MSGDEF`, `$NAMDEF`,
  `$PQLDEF`, `$PRCDEF`, `$PRTDEF`, `$PRVDEF`, `$PSLDEF`, `$RABDEF`,
  `$RMSDEF`, `$SECDEF`, `$SSDEF`, `$SYIDEF`, `$TTDEF`, and the XAB
  definitions.

If you have VMS's own system macro library, govax uses it instead (see
below), and every VMS system macro is available.

Symbols such as `SS$_NORMAL`, `RMS$_EOF`, `IO$_WRITEVBLK`, and the FAB
and RAB field offsets are also known to the linker from govax's own
tables, so a module can refer to them without defining them; the linker
supplies their values, as VMS's LINK does from its system object
library.

#### Using VMS's own libraries

If you have a VAX/VMS system or distribution, its library files make
`MACRO` and `LINK` behave exactly as on VMS. Put them in a host directory
and point `vax.library` at it, or keep them on a volume and define
`SYS$LIBRARY` and `SYS$SHARE` to lead to them:

| File | Used by | Without it |
| --- | --- | --- |
| the system macro library (STARLET's `.MLB`) | `MACRO`: every system macro | govax's own STARLET (above) |
| the system object library, `STARLET.OLB` | `LINK`: the system library's modules | govax's tables of its symbols; routines it holds as code (`BAS$`, ...) are missing |
| `IMAGELIB.OLB` | `LINK`: which shareable image defines each routine | govax's tables of LIBRTL's routines |
| `LIBRTL.EXE` and other shareable images | `LINK`: routine offsets | govax's tables |

These are copyrighted files and aren't part of govax; the project's
README lists them.

### Mixing in other languages

Besides MACRO (and govax's FORTRAN compiler, which writes MACRO:
[chapter 14](#14-programming-in-fortran)), govax has no compilers, but
object modules compiled on a VAX (FORTRAN, C, Pascal, BASIC) can be
linked by govax's `LINK` with your MACRO
modules, given the libraries above, and run if the run-time routines they
call are ones govax provides ([Appendix B](#appendix-b-run-time-library-routines)).

---

## 9. The linker

`LINK` combines object modules into an *image*: an executable image that
`RUN` runs, or a shareable image whose routines other images call. Its
images and maps are laid out as VMS's LINK lays them out, so an image
linked by govax runs on VMS, and VMS's objects and libraries link in
govax.

```text
LINK file[,...] [/EXECUTABLE[=file] | /SHAREABLE[=file] | /NOEXECUTABLE]
     [/MAP[=file] [/BRIEF | /FULL] | /NOMAP] [/[NO]TRACEBACK] [/DEBUG]
     [/SYMBOL_TABLE[=file]] [/[NO]SYSLIB] [/[NO]USERLIBRARY[=(tables)]]

file:   object[.OBJ]
        library/LIBRARY                  search an object or shareable image library
        library/INCLUDE=(module,...)     take these modules from a library
        object/SELECTIVE_SEARCH          take only the symbols referred to
        options/OPTIONS                  an options file (.OPT)
```

| Qualifier | Meaning |
| --- | --- |
| `/EXECUTABLE[=file]` | Write an executable image (the default), named after the first object, type `.EXE`. |
| `/SHAREABLE[=file]` | Write a shareable image instead. |
| `/NOEXECUTABLE` | Link and report errors, without writing an image. |
| `/MAP[=file]` | Write a link map (`.MAP`). `/BRIEF` makes it short; `/FULL` adds the image sections and what each library contributed. |
| `/TRACEBACK` | Start the image through `SYS$IMGSTA` and give it a debug symbol table from the objects' traceback records (the default). `/NOTRACEBACK` starts it at its own transfer address. |
| `/DEBUG` | Also include the objects' debugger records, and mark the image to start under the debugger. |
| `/SYMBOL_TABLE[=file]` | Also write the image's global symbols as an `.STB` object. |
| `/NOSYSLIB` | Don't search the system libraries. |
| `/NOUSERLIBRARY` | Don't search the libraries `LNK$LIBRARY`, `LNK$LIBRARY_1`, ... name. |

The simplest link is one object:

```text
VAX> LINK HELLO
```

and several objects and a library:

```text
VAX> LINK MAIN,SUBS,DUA0:[LIB]MATHLIB/LIBRARY
VAX> LINK/EXECUTABLE=PROG/MAP/FULL MAIN,PROG/OPTIONS
```

### How symbols are resolved

Each global symbol a module refers to must be defined by some module.
`LINK` looks, in order, in:

1. the object modules you named;
2. the libraries you named with `/LIBRARY`;
3. the user libraries `LNK$LIBRARY`, `LNK$LIBRARY_1`, ... `LNK$LIBRARY_999`
   (logical names, searched in turn until one isn't defined);
4. the system libraries: `IMAGELIB.OLB` (which shareable image defines a
   routine) and `STARLET.OLB` (whose modules are added to the image), if
   you have them;
5. govax's own tables: every LIBRTL routine and the other run-time
   routines govax provides, in their shareable images; STARLET's symbols
   (`SS$_...`, `RMS$_...`, `IO$_...`, the FAB/RAB/NAM offsets, and more);
   and the system services (`SYS$...`) at their addresses in the system
   service vector.

So without any VMS files, a call to `LIB$PUT_OUTPUT` or `SYS$QIOW`, or a
reference to `SS$_NORMAL`, links just as it would on VMS.

A symbol nothing defines is a warning: it's 0, each reference to it is
reported, and the image is still written. A routine in a shareable image
is reached with a general-mode operand (`CALLS #1, G^LIB$PUT_OUTPUT`) or
an `.ADDRESS`; the image activator connects it at run time.

### The map

`/MAP` writes VMS's map: the object modules, the program sections and
each module's part of them, the global symbols, and the image synopsis.
`/BRIEF` keeps only the first and last; for example:

```text
VAX> LINK/EXECUTABLE=MAIN3/MAP=MAIN3/BRIEF MAIN,MATHLIB/LIBRARY
VAX> TYPE MAIN3.MAP
                                             +------------------------+
                                             ! Object Module Synopsis !
                                             +------------------------+

Module Name     Ident              Bytes      File                                Creation Date      Creator
-----------     -----              -----      -----                               -------------      -------
MAIN            0                     198 DUA0:[SHR]MAIN.OBJ;1                 20-OCT-2026 09:59  govax MACRO V0.0-0
ROUTINES        0                      18 DUA0:[SHR]MATHLIB.OLB;1              20-OCT-2026 09:59  govax MACRO V0.0-0
...
                                                 +----------------+
                                                 ! Image Synopsis !
                                                 +----------------+

Virtual memory allocated:                         00000200 000007FF 00000600 (1536. bytes, 3. pages)
Stack size:                                             20. pages
...
User transfer address:                            00000414
Image type:                                       EXECUTABLE.
```

### Options files

An options file (`/OPTIONS`) gives the linker more input files and
options, a line each. `!` starts a comment and `-` continues a line. In a
command procedure, `SYS$INPUT/OPTIONS` reads the options from the data
lines after the `LINK` command. govax supports every VAX LINK option but
`UNSUPPORTED=`:

| Option | Meaning |
| --- | --- |
| `file/SHAREABLE` | Link against a shareable image (found as `RUN` would find it). |
| `file/LIBRARY`, `/INCLUDE=`, `/SELECTIVE_SEARCH` | As on the command line. |
| `STACK=pages` | The user stack's size (default 20). |
| `IDENTIFICATION=id`, `NAME=name` | The image's ident and name. |
| `SYMBOL=name,value` | Define a global symbol. |
| `UNIVERSAL=name,...` | Make symbols visible to images that link against this shareable image. |
| `GSMATCH=ALWAYS\|EQUAL\|LEQUAL,major,minor` | A shareable image's version match rule. |
| `CLUSTER=name[,base][,pfc][,file,...]` | Place files' sections together, in order, optionally at an address. |
| `COLLECT=cluster,psect,...` | Move program sections into a cluster. |
| `PSECT_ATTR=psect,attribute,...` | Change a program section's attributes. |
| `BASE=address` | The image's base address. |
| `DZRO_MIN=pages`, `ISD_MAX=count` | When writable zero pages become demand-zero sections. |
| `PROTECT=YES\|NO`, `IOSEGMENT=pages` | As on VMS. |

### Shareable images

A shareable image is a library of routines and data that other images
call, linked once and used by many programs, as LIBRTL is. govax links,
uses, and runs them as VMS does. Here is a complete example.

A shareable image usually starts with a *transfer vector*, so that its
routines keep their addresses from one version to the next. `VECTOR.MAR`:

```asm
        .TITLE  VECTOR  Transfer vector for MYSHR
        .PSECT  VECTOR, PIC, SHR, NOWRT, EXE, QUAD
        .TRANSFER       DOUBLE          ; DOUBLE is a universal symbol
        .MASK           DOUBLE          ; its entry mask
        JMP             L^DOUBLE+2      ; and a jump past it
        .TRANSFER       SQUARE
        .MASK           SQUARE
        JMP             L^SQUARE+2
        .END
```

The routines, `ROUTINES.MAR`:

```asm
        .TITLE  ROUTINES        Arithmetic routines for MYSHR
        .PSECT  $CODE, PIC, SHR, NOWRT, EXE, LONG
; DOUBLE(n): returns 2*n in R0
        .ENTRY  DOUBLE, ^M<>
        ADDL3   4(AP), 4(AP), R0
        RET
; SQUARE(n): returns n*n in R0
        .ENTRY  SQUARE, ^M<>
        MULL3   4(AP), 4(AP), R0
        RET
        .END
```

Its options, `MYSHR.OPT`, put the vector first and set its version rule:

```text
! MYSHR.OPT -- options for the shareable image
CLUSTER=TRANSFER_VECTOR,,,VECTOR
GSMATCH=LEQUAL,1,1000
```

A program that uses it, `MAIN.MAR`:

```asm
        .TITLE  MAIN    Calls MYSHR's routines
        .PSECT  DATA, WRT, NOEXE, LONG
CTR:    .ASCID  /double(!UL) = !UL, square = !UL/
BUF:    .BLKB   80
DSC:    .LONG   80
        .ADDRESS BUF
        .PSECT  CODE, NOWRT, EXE, LONG
        .ENTRY  MAIN, ^M<R2,R3>
        PUSHL   #7
        CALLS   #1, G^DOUBLE
        MOVL    R0, R2
        PUSHL   #7
        CALLS   #1, G^SQUARE
        MOVL    R0, R3
        $FAO_S  CTRSTR=CTR, OUTLEN=DSC, OUTBUF=DSC, P1=#7, P2=R2, P3=R3
        PUSHAQ  DSC
        CALLS   #1, G^LIB$PUT_OUTPUT
        RET
        .END    MAIN
```

and its options file, `MAIN.OPT`, one line: `MYSHR/SHAREABLE`.

Build and run them:

```text
VAX> MACRO VECTOR
VAX> MACRO ROUTINES
VAX> LINK/SHAREABLE=MYSHR VECTOR,ROUTINES,MYSHR/OPTIONS
VAX> MACRO MAIN
VAX> DEFINE MYSHR DUA0:[SHR]MYSHR.EXE
VAX> LINK MAIN,MAIN/OPTIONS
VAX> RUN MAIN
double(7) = 14, square = 49
```

The logical name `MYSHR` tells both `LINK` and `RUN` where the image is.
(Without it, both look in `SYS$SHARE`, which you can define to a
directory of shareable images.) At run time, the image activator finds
`MYSHR`, checks its `GSMATCH` against the one `MAIN` was linked with,
places it in the process's memory, and connects `MAIN`'s calls to it.

`GSMATCH` decides which later versions of the shareable image a program
accepts: `ALWAYS` any; `EQUAL` the same major and minor ids; `LEQUAL` the
same major id and a minor id at least the program's. A mismatch stops the
activation with `%SYSTEM-F-SHRIDMISMAT`. Without `GSMATCH`, the ident
comes from the link time, so programs must be relinked whenever the
shareable image is.

`LINK` can also search a *shareable image library* for the images that
define symbols, so programs needn't name them:

```text
VAX> LIBRARY/CREATE/SHARE SHRLIB MYSHR.EXE
VAX> DEFINE LNK$LIBRARY DUA0:[SHR]SHRLIB.OLB
VAX> LINK/EXECUTABLE=MAIN2 MAIN
VAX> RUN MAIN2
double(7) = 14, square = 49
```

A shareable image can't be run itself: `RUN MYSHR` is `%DCL-E-NOTFR`,
"no transfer address", as on VMS.

### Traceback and debugging

By default (`/TRACEBACK`), the image carries a debug symbol table built
from the objects' traceback records: module and routine names and line
numbers, which the debugger's `SHOW CALLS` uses. With `MACRO/DEBUG` and
`LINK/DEBUG`, it also carries the full debugger records (every symbol,
its type, the source line table), and `RUN` starts it under the debugger.

The traceback records also give a failing program its symbolic stack
dump: a warning, error, or severe condition that no handler of the
program's takes is shown with a line for each call frame, from the
innermost to the main routine, naming the module, the routine, and,
for an image linked `/DEBUG`, the listing line (section 8, example 5).
A warning or an error then goes on; a severe condition ends the image.
As on VMS, a PC more than 3FF bytes past the routine before it is named
by its psect instead, and only the main image's PCs are named: one in a
shareable image or LIBRTL has blank columns. Under the debugger an
unhandled condition breaks into it instead, with no dump.

A condition's message and its dump go where VMS's `$PUTMSG` writes:
to `SYS$ERROR`, and to `SYS$OUTPUT` too when the two name different
places. A success message goes to `SYS$OUTPUT` only, and a `SYS$OUTPUT`
file the program has written to itself gets none of them:

```
$ DEFINE/USER_MODE SYS$OUTPUT FAIL.OUT
$ DEFINE/USER_MODE SYS$ERROR FAIL.ERR
$ RUN FAIL
```

leaves the message and dump in both files if the program wrote nothing
to `SYS$OUTPUT`, and in `FAIL.ERR` alone if it did.

---

## 10. The librarian

`LIBRARY` creates and maintains the three kinds of VMS library, laid out
as VMS's librarian lays them out, so libraries move freely between govax
and VMS:

- **Macro libraries** (`.MLB`), which `MACRO` searches for macros;
- **Object libraries** (`.OLB`), which `LINK` searches for modules;
- **Shareable image libraries** (`.OLB`, made with `/SHARE`), which `LINK`
  searches for the shareable images that define symbols.

```text
LIBRARY library [input[,...]]
        /CREATE  /INSERT  /REPLACE  /DELETE=(module,...)  /EXTRACT=(module,...)
        /OUTPUT=file  /LIST[=file]  /FULL  /NAMES  /WIDTH=n
        /MACRO  /OBJECT  /SHARE  /NOSQUEEZE  /SELECTIVE_SEARCH  /LOG
```

| Qualifier | Meaning |
| --- | --- |
| `/CREATE` | Make a new library (a new version on a volume), then insert any input files. |
| `/INSERT` | Insert modules; one already there is a warning and is left as it was. |
| `/REPLACE` | Replace modules of the same names, inserting the rest. The default when input files are given. |
| `/DELETE=(...)` | Delete modules (wildcards `*` and `%` allowed). |
| `/EXTRACT=(...)` | Copy modules out to `/OUTPUT=file` (default: the library's name, type `.MAR` or `.OBJ`). |
| `/LIST[=file]` | List the library's header and modules. `/FULL` adds idents, insertion times, and symbol counts; `/NAMES` lists each object module's global symbols. |
| `/MACRO`, `/OBJECT`, `/SHARE` | The kind of new library (object by default). An existing library keeps its own kind. |
| `/NOSQUEEZE` | Keep macros' comments and trailing blanks, which are removed by default. |
| `/SELECTIVE_SEARCH` | Mark inserted object modules for selective search. |

The input files are macro sources (`.MAR`: one module per macro), objects
(`.OBJ`: one module per object module), or shareable images (`.EXE`).

The librarian builds the whole library and writes it only when every
step has succeeded, so an error leaves the library as it was.

```text
VAX> LIBRARY/CREATE/LOG MATHLIB ROUTINES
%DCL-S-INSERTED, module ROUTINES inserted in DUA0:[SHR]MATHLIB.OLB;1
VAX> LIBRARY/LIST/FULL/NAMES MATHLIB
Directory of OBJECT library DUA0:[SHR]MATHLIB.OLB;1 on 20-OCT-2026 10:10:06
Creation date:  20-OCT-2026 10:10:06      Creator:  govax Librarian
Revision date:  20-OCT-2026 10:10:06      Library format:   3.0
Number of modules:      1                 Max. key length:  31
Other entries:          2                 Preallocated index blocks:     49
Recoverable deleted blocks:      0        Total index blocks used:        2
Max. Number history records:      20      Library history records:        0

Module ROUTINES         Ident 0                Inserted 20-OCT-2026 10:10:06 2 symbols
DOUBLE                           SQUARE

VAX> LINK MAIN,MATHLIB/LIBRARY
VAX> LIBRARY/EXTRACT=ROUTINES/OUTPUT=COPY.OBJ MATHLIB
VAX> LIBRARY/DELETE=ROUTINES MATHLIB
```

A macro library:

```text
VAX> LIBRARY/CREATE/MACRO MYMAC MYMAC.MAR
VAX> LIBRARY/REPLACE/MACRO MYMAC NEWMACS.MAR      ! update some macros
VAX> MACRO PROG/LIBRARY=MYMAC
```

### Analyzing objects and images

`ANALYZE/OBJECT` and `ANALYZE/IMAGE` describe object and image files in
the same words and page layout as VMS's `ANALYZE`, which is useful for
seeing exactly what the assembler or linker produced.

```text
ANALYZE/OBJECT file[,...] [/OUTPUT[=file]] [/INCLUDE=(module,...)]
               [/MHD] [/GSD] [/TIR] [/TBT] [/DBG] [/LNK] [/EOM]
ANALYZE/IMAGE  file[,...] [/OUTPUT[=file]] [/HEADER] [/FIXUP_SECTION]
```

`ANALYZE/OBJECT` shows each record of an object module (or of an object
library's modules): headers, the global symbol directory, every text and
relocation command, traceback and debugger records, and end of module.
The record-type qualifiers show only those kinds. `ANALYZE/IMAGE` shows
an image's header, its image sections, and its fixup section (the
shareable images it uses and its references to them). Errors in a file
are flagged with `***` and counted at the end.

```text
VAX> ANALYZE/OBJECT/GSD ROUTINES
Analyze Object File                          20-OCT-2026 10:10:06.53   Page 1
DUA0:[SHR]ROUTINES.OBJ;1
ANALYZ V07-04

This is an OpenVMS VAX object file
...
        1)  Entry Point Symbol and Mask Definition (GSD$C_EPM)
                data type: DSC$K_DTYPE_Z (0)
                ...
                entry mask: <>
                symbol: "DOUBLE"
```

From the host: `govax analyze routines.obj`, `govax analyze --image
--header prog.exe`.

---

## 11. Running programs

### RUN

```text
RUN [/DEBUG | /NODEBUG] [/NOEXECUTE] image[/HOST]
```

`RUN` finds the image (default type `.EXE`) by the file name rules of
section 5, activates it and the shareable images it uses, and runs it
from its transfer address. When the image exits, `$STATUS` is its exit
status: the R0 its main routine returned, or the status it passed to
`$EXIT`. A failure's message is shown.

```text
VAX> RUN HELLO
Hello, world!
VAX> RUN "/Users/me/vax/fact.exe"
VAX> RUN/DEBUG FACT
```

`RUN` gives the image no command text. To pass it some, use a foreign
command or `MCR`, or from the host shell, `govax run image text...`:

```text
VAX> ECHO :== $DUA0:[TOOLS]ECHO.EXE
VAX> ECHO one   two "Three"
ONE TWO "Three"
VAX> MCR DUA0:[TOOLS]ECHO via mcr
VIA MCR
$ govax run echo.exe Hello there, '"Mixed Case"'
Hello there, "Mixed Case"
```

(A foreign command's text is treated as DCL treats parameters:
uppercased and compressed unless quoted. `govax run` passes the host
shell's words as they are.)

While an image runs, **Ctrl/C** stops it and returns to the prompt (or
delivers a Ctrl/C AST, if the program asked for one). `STOP` at the next
prompt ends an image Ctrl/C left stopped.

### How an image is activated

When `RUN` starts an image, govax does what VMS's image activator does:

1. **Reads the image header**: its image sections (code, data,
   demand-zero, the user stack), transfer addresses, and the list of
   shareable images it was linked against.
2. **Finds each shareable image**, and the ones they use in turn:
   1. A shareable image govax provides itself (LIBRTL, LIBRTL2, MTHRTL, the C
      run-time library, and the other system images) is govax's own
      routines, unless you've defined a logical name of the image's name
      (`DEFINE LIBRTL DUA1:[X]LIBRTL.EXE`), which asks for that file
      instead.
   2. Otherwise its name is looked up with the default `SYS$SHARE:.EXE`,
      as on VMS: a logical name of the image's name, or `SYS$SHARE`
      (which you define; each element of a search list in turn).
   3. Otherwise, the host: the `vax.library` directory, then the name as
      given.

   An image no rule finds stops the activation:

   ```text
   %DCL-W-ACTIMAGE, error activating image MYSHR
   -CLI-E-IMAGEFNF, image file not found DUA0:[SYSLIB]MYSHR.EXE;
   ```

3. **Checks each shareable image's ident** against the program's `GSMATCH`
   (`%SYSTEM-F-SHRIDMISMAT` if they don't match). govax's own routines
   always match.
4. **Places the images** in the process's P0 space: the program at its
   base, each shareable image after it (or at its own base, if it was
   linked with one).
5. **Does the fixups**: fills in each image's references to the others,
   and sets each page's protection (code read-only).
6. **Creates the user stack** in P1 (`STACK=` pages, 20 by default) and
   calls the image at its transfer address (through `SYS$IMGSTA`, for an
   image linked `/TRACEBACK`).

With paging off, the image's pages are copied into memory. With paging
on, they're *mapped* from the image file and read in as they're touched,
as on VMS (section 13).

When the image exits, its exit handlers run, its user-mode logical names
and locks go, and its address space (P0, and the P1 pages it made) is
deleted, ready for the next image.

### How a program reaches VMS

A VAX program calls the system and the run-time library through fixed
addresses, as on VMS:

- **System services** are called at their entries in the *system service
  vector* (`SYS$QIOW`, `SYS$EXIT`, ...), in P1 space at the addresses VMS
  uses. Each entry holds a special instruction, `XFC` (the VAX's
  "extended function call", reserved for customer extensions), that
  passes control from the emulated VAX to govax's Go code for that
  service. The service reads its arguments from the program's argument
  list and returns a status in R0, exactly as VMS's would.
- **Run-time library routines** (`LIB$`, `STR$`, ...) are reached through
  the shareable image's transfer vector. govax puts a small *shim* at
  each entry: a stub that, again with `XFC`, hands the call to the Go
  routine. A few routines are written in VAX code in govax's microkernel
  instead.

To the program none of this is visible: it links and calls as on VMS.
The debugger's `SHOW SHIM` lists the shims.

### Running programs built on VMS

An image linked on VMS runs in govax if:

- it calls only system services and run-time routines govax provides
  (Appendices A and B);
- or it uses shareable images you can supply as files (your own, or VMS's
  own if you have them), which run as VAX code.

Copy the image into govax with `COPY/BINARY` (or mount the VAX disk it's
on), and run it:

```text
VAX> COPY/BINARY "prog.exe"/HOST DUA0:[WORK]PROG.EXE
VAX> RUN PROG
```

A program that calls a run-time routine govax doesn't have yet links and
starts, and the call signals:

```text
%SYSTEM-E-UNSUPPORTED, unsupported operation or function
-GOVAX-E-NOTIMPL, routine LIB$TPARSE is not implemented in govax
```

Programs from VAX compilers also need their language's run-time library
(`FOR$`, `BAS$`, `PAS$`, `DECC$`); govax implements part of the C
library and none of the others, so such programs need those libraries'
images from a VMS system. govax uses a library's image when a logical
name of the library's own name leads to it (`DEFINE FORRTL
SYS$SHARE:FORRTL.EXE`). Without one, a call of a routine of the library
govax lacks signals as above, naming the library and the routine's
offset in its transfer vector:

```text
%SYSTEM-E-UNSUPPORTED, unsupported operation or function
-GOVAX-E-NOTIMPL, routine at BASRTL+^X518 is not implemented in govax
```

Images linked by VMS V1 (such as a FORTRAN program from 1978) run too.
They call the system through V1's system service vector at
`^X80000000`, which govax provides. They reach VMSRTL through global
sections at fixed addresses, so `SYS$SHARE:VMSRTL.EXE` (VMS 7's, from a
VMS system) is mapped there, with the libraries it uses after it.
`RUN/DEBUG` of one stops at its main program, as for any image; the
debugger names the language from the image's debug symbols (FORTRAN),
and so works in decimal.

A FORTRAN program run with VMS's FORRTL reads and writes the terminal
through its units 5 and 6 (SYS$INPUT and SYS$OUTPUT), with FORTRAN
carriage control: a `$` record is a prompt, which leaves the cursor at
its end for the reply, and a `+` record continues the line. Its
integer and floating-point edit descriptors (`I`, `O`, `Z`, `L`, `F`,
`E`, `D`, `G`) use govax's OTS$ conversions, as do the routines a
program may call itself (`OTS$CNVOUT`, `OTS$CVT_T_D`, `_F`, `_G`, and
`_H`). `SET DEBUG RMS` shows each RMS call
the program makes, with its file or record and status, which is the
quickest way to see why such a program's I/O fails.

### Processes

Each `RUN` runs the image in your own process, the console's (named
`SYSTEM`, PID `00000301`). Programs can create more processes, which run
alongside it:

- `SPAWN` and `LIB$SPAWN` create subprocesses that run DCL;
- `$CREPRC` creates a subprocess or detached process that runs an image
  (or, given `LOGINOUT.EXE` and an input file, DCL).

They communicate through mailboxes, common event flags, global sections,
the lock manager, and shared files, as on VMS. `SHOW SYSTEM` lists them,
`STOP` deletes one, and the debugger's `SET PROCESS` lets you look at
one. Section 13 describes how they're scheduled.

### Limits

From the host shell, `--instruction-limit` and `--time-limit` stop a
program that runs too long (exit status 124). At the prompt, Ctrl/C does.

---

## 12. The debugger

govax's debugger is modeled on the VMS debugger: its commands, its
syntax, and its output follow VMS's, and it was checked against VMS 7.3
debugger sessions. It also has commands of its own for looking at the
machine underneath (registers, page tables, the translation buffer).

The debugger has its own prompt, `DBG>`, and its own commands. DCL's
commands aren't available there; `EXIT` returns to `VAX>`.

### Starting the debugger

There are three ways in:

- **`RUN/DEBUG image`**, or `RUN` of an image linked `/DEBUG`: the image is
  loaded and stopped at the first instruction of its main routine.
  (`RUN/NODEBUG` runs a `/DEBUG` image without the debugger.)
- **`DEBUG`** at the `VAX>` prompt: a session on the machine as it is,
  with nothing running, for examining memory, setting breakpoints, or
  assembling code.
- **A breakpoint** reached while a program runs from the debugger.

Each `RUN` starts its image with a fresh debugger, as VMS gives each
image a debugger of its own: the breakpoints, tracepoints, and
watchpoints an earlier session set (every kind, `SET TRACE/INSTRUCTION`
and `SET BREAK/FAULT` among them), and its `SET STEP` defaults, are
gone, whether or not the new image runs under the debugger. The display
settings (`SET MODE`, `SET TYPE`) stay. The radix is the main module's
language's, as on VMS: hexadecimal for MACRO, decimal for FORTRAN, so a
FORTRAN program's instructions show `MOVAL L^11720,R11`; `SET RADIX`
changes it and `CANCEL RADIX` goes back to it. Before the new image
starts, an earlier one that was stopped and never exited is run down.

For source-line debugging, assemble with `/DEBUG` and link with
`/DEBUG`:

```text
VAX> MACRO/DEBUG FACT
VAX> LINK/DEBUG FACT
VAX> RUN FACT

         GOVAX DEBUG

%DEBUG-I-INITIAL, Language: MACRO, Module: FACT
DBG>
```

### A session

Using the factorial program from section 8, stop in `FACTORIAL` when its
argument is 3, look around, and step:

```text
DBG> SET BREAK FACTORIAL WHEN (.(AP+4) EQL 3)
DBG> SHOW BREAK
breakpoint at routine FACT\FACTORIAL
   when (.(AP+4) EQL 3)
DBG> GO
 1! = 1
 2! = 2
break at routine FACT\FACTORIAL
    34:         MOVL    4(AP), R0               ; n
DBG> EVALUATE/DECIMAL .(AP+4)
3
DBG> SHOW CALLS
 module name     routine name      line                rel PC           abs PC
*FACT            FACTORIAL           34               00000002         00000466
*FACT            FACT                20               00000015         00000415
DBG> STEP
stepped to FACT\FACTORIAL\%LINE 35
    35:         CMPL    R0, #1
DBG> STEP
stepped to FACT\FACTORIAL\%LINE 36
    36:         BLEQ    20$                     ; 0! = 1! = 1
DBG> EXAMINE/INSTRUCTION .PC
FACT\FACTORIAL\%LINE 36:        BLEQ     FACT\FACTORIAL\%LINE 41
DBG> STEP/RETURN
stepped on return from FACT\FACTORIAL\%LINE 36 to FACT\FACTORIAL\%LINE 40: RET
    40:         RET
DBG> EXAMINE/DECIMAL R0
FACT\FACTORIAL\%R0:     6
DBG> SHOW REGISTERS

    Registers:

    R0:  00000006      R4:  00000000     R8:  00000000     AP:  7FDFFFC4
    R1:  00000000      R5:  00000000     R9:  00000000     FP:  7FDFFFB0
    R2:  00000003      R6:  00000000     R10: 00000000     SP:  7FDFFFB0
    R3:  00000000      R7:  00000000     R11: 00000000     PC:  0000047B

    PSL: 03C00000
         PSW:  C=0, V=0, Z=0, N=0, T=0, IV=0, FU=0, DV=0
         PRIV: IPL=0, CUR_MOD=USER, PRV_MOD=USER, FPD=0, TP=0, CM=0
DBG> CANCEL BREAK/ALL
DBG> SET WATCH RESULT
DBG> GO
watch of FACT\RESULT at FACT\%LINE 21
    21:         MOVL    R0, RESULT
   old value: 00000002
   new value: 00000006
break at FACT\%LINE 22
    22:         MOVL    #80, OUTDSC             ; reset the buffer's length
DBG> CANCEL WATCH/ALL
DBG> GO
 3! = 6
 ...
10! = 3628800
%DEBUG-I-EXITSTATUS, is '%SYSTEM-S-NORMAL, normal successful completion'
DBG> EXIT
VAX>
```

After the program exits, the debugger stays, as VMS's does, until you
`EXIT`.

### Expressions and locations

An address or value is an expression:

| Form | Meaning |
| --- | --- |
| `200`, `^D512`, `%HEX 10` | numbers, in the input radix (the language's, hexadecimal for MACRO, unless `SET RADIX` says otherwise); `^X`, `^D`, `^O`, `^B` force one |
| `COUNT`, `FACTORIAL` | a program's symbol (data, label, or routine) |
| `FACT\FACTORIAL\20$` | a path name: module, routine, label |
| `%LINE 34`, `FACT\%LINE 34` | the code of a source line |
| `R0` ... `R11`, `AP`, `FP`, `SP`, `PC`, `PSL` | registers |
| `.address`, `.(AP+4)` | the contents of a location |
| `+ - * /`, `EQL NEQ LSS LEQ GTR GEQ`, `AND OR NOT` | operators |
| `.` `%CURLOC`, `^` `%PREVLOC`, `\` `%CURVAL` | the current location, the one before it, and the last value shown |
| `A:B` | a range, for `EXAMINE` |

### Commands

**Running**

| Command | Meaning |
| --- | --- |
| `GO [address]` | Run until a breakpoint, the end, or Ctrl/C. |
| `STEP [n]` | Step a line (or `n`); `/INSTRUCTION` a single instruction; `/INTO` into calls (the default is `/OVER`); `/RETURN` to the routine's `RET`. |
| `SET STEP keyword` | Change `STEP`'s defaults: `LINE`/`INSTRUCTION`, `OVER`/`INTO`, `[NO]SILENT`, `[NO]SOURCE`. |
| `CALL routine [(args)]` | Call a routine, as `CALLS` would; it runs until it returns, a breakpoint stops it, or Ctrl/C. |

**Eventpoints**

| Command | Meaning |
| --- | --- |
| `SET BREAK address` | Stop before the instruction at `address` (a routine's name means after its entry mask). |
| `SET BREAK/CALL`, `/BRANCH`, `/LINE`, `/RETURN routine`, `/INSTRUCTION[=(op,...)]`, `/EXCEPTION` | Stop at each instruction of a kind, or when an exception is signaled. |
| `SET TRACE ...` | As `SET BREAK`, but report and go on. |
| `SET WATCH location` | Stop after an instruction changes it. |
| `... /AFTER:n`, `/TEMPORARY`, `WHEN (cond)`, `DO (cmd;...)` | Skip the first `n`, remove after the first stop, stop only when true, run commands at the stop. |
| `SHOW BREAK`, `SHOW TRACE`, `SHOW WATCH` | List them. |
| `CANCEL BREAK [/ALL] [address]`, ... | Remove them. |

**Examining and changing**

| Command | Meaning |
| --- | --- |
| `EXAMINE [/type] location[,...]` | Show data, typed by the program's symbols or a qualifier: `/BYTE`, `/WORD`, `/LONGWORD`, `/QUADWORD`, `/OCTAWORD`, `/FLOAT`, `/D_FLOAT`, `/G_FLOAT`, `/H_FLOAT`, `/PACKED:n`, `/ASCII:n`, `/ASCIC`, `/ASCIZ`, `/ASCID`, `/DATE_TIME`, `/CONDITION_VALUE`, `/PSL`, `/PTE`; and `/HEX`, `/DECIMAL`, `/OCTAL`, `/BINARY`. |
| `EXAMINE/INSTRUCTION location` | Disassemble, with symbolic names; `/OPERANDS` explains each operand. With no location, the instruction after the last one shown. An address in a shareable image with no symbols is named `SHARE$image+offset`. |
| `DEPOSIT [/type] location = value` | Change memory or a register. `DEPOSIT/INSTRUCTION .PC = 'MOVL #1,R0'` assembles an instruction in place. `DEPOSIT/WORD PSL = 10` changes the PSL's low word, here setting T; DEPOSIT changes only the low word. |
| `EVALUATE [/radix] expr` | Show a value; `/ADDRESS` an address; `/CONDITION_VALUE` a status's message. |
| `SYMBOLIZE address` | The names for an address. |
| `SHOW CALLS`, `SHOW STACK` | The call frames. |
| `SHOW REGISTERS`, `SHOW PSL`, `SHOW name` | Registers, including privileged ones (`SHOW P0BR`). |
| `SHOW IMAGE [/FULL]` | The loaded images; `/FULL` shows their sections, protections, and page states. |
| `SHOW MODULE`, `SHOW SYMBOL pattern`, `SHOW SCOPE` | The program's debug symbols; `SHOW MODULE` has a language column when the modules' languages differ. |

**Settings**

| Command | Meaning |
| --- | --- |
| `SET RADIX [/INPUT\|/OUTPUT] radix` | Decimal, hexadecimal, octal, or binary. |
| `SET MODE [NO]SYMBOLIC`, `OPERANDS`, `[NO]LINE` | Display modes. |
| `SET TYPE [/OVERRIDE] type` | The default type for `EXAMINE` and `DEPOSIT`. |
| `SET SOURCE dir,...` | Where to find source files. |
| `@file` | Run debugger commands from a file (`.COM` when it has no type); with `SET OUTPUT VERIFY` each line is echoed as it runs. |
| `SET LOG file`, `SET OUTPUT [NO]LOG,[NO]VERIFY,[NO]TERMINAL` | Log the session (to `DEBUG.LOG` unless `SET LOG` names another file): each command as read, and every output line after a `!`, so the log can be run with `@`. |
| `SHOW LOG`, `SHOW OUTPUT` | Whether the session is logged, and the other output settings. |
| `EXIT`, `QUIT` | End the session, and a program still running under it. |

A command procedure can give the debugger its commands, as on VMS: a
`DEFINE/USER_MODE DBG$INPUT file` before `RUN/DEBUG` makes the debugger
read that file instead of the terminal, to its end, which ends the
session (and RUN). For example, run the program under the debugger with
`TRACE.DBG`'s commands, logging the session to `TRACE.DLG`:

```
$ OPEN/WRITE DBGF RUN.DBG
$ WRITE DBGF "SET LOG TRACE.DLG"
$ WRITE DBGF "SET OUTPUT LOG,VERIFY"
$ WRITE DBGF "@TRACE.DBG"
$ WRITE DBGF "EXIT"
$ CLOSE DBGF
$ DEFINE/USER_MODE DBG$INPUT RUN.DBG
$ RUN/DEBUG PROG
```

### A program that traces itself

The debugger steps a program from outside the machine, never with the T
bit, so a program that traces itself (section 8, example 6) can be
debugged. It behaves as VMS 7.1's debugger does:

- A breakpoint at an instruction that a trace is pending before takes
  the trace: the program's handler doesn't see it, and `EXAMINE PSL`
  shows T set and TP clear.
- Going on from a breakpoint or a step, by `GO` or `STEP`, runs one
  instruction and then clears the program's T, so the program takes no
  further traces. A T set with `DEPOSIT` goes the same way.
- A breakpoint in the program's trace handler doesn't disturb the
  tracing: `GO` from it, and the program gets every trace.
- `SET BREAK/EXCEPTION` stops at each `SS$_TBIT`, "at" the instruction
  the trace comes before; `GO` passes the condition to the program's
  handler, and the tracing goes on.
- An unhandled `SS$_TBIT` stops as any unhandled condition does;
  `GO` says `%DEBUG-I-CONFROMEXC, WARNING: continuing after severe
  error; integrity not guaranteed` and continues the program.

Here the example's program stops at its first trace by `SET
BREAK/EXCEPTION`, and its handler gets it; the breakpoint on the
`SOBGTR` takes the next trace, and the `GO` from there ends the tracing:

```text
VAX> MACRO/DEBUG TRACER
VAX> LINK/DEBUG TRACER
VAX> RUN TRACER

         GOVAX DEBUG

%DEBUG-I-INITIAL, Language: MACRO, Module: TRACER
DBG> SET BREAK %LINE 18
DBG> SET BREAK/EXCEPTION
DBG> GO
%SYSTEM-F-TBIT, T-bit pending trap at PC=00000410, PSL=03C00014
break on exception at TRACER\MAIN\%LINE 17
    17: 10$:    ADDL2   R1, R0
DBG> GO
break at TRACER\MAIN\%LINE 18
    18:         SOBGTR  R1, 10$
DBG> EXAMINE PSL
TRACER\MAIN\%PSL:
        CMP TP FPD IS CURMOD PRVMOD IPL DV FU IV T N Z V C
         0   0  0   0  USER   USER    0  0  0  0 1 0 0 0 0
DBG> CANCEL BREAK/ALL
DBG> GO
Sum 15, 1 instruction traced
%DEBUG-I-EXITSTATUS, is '%SYSTEM-S-NORMAL, normal successful completion'
DBG> EXIT
```

### govax's machine commands

These go beyond VMS's debugger, to the machine itself:

| Command | Shows or does |
| --- | --- |
| `SHOW CPU_STATUS` | Every register, privileged ones included, and each mode's stack pointer. |
| `SHOW PAGE address` | The page table entry for an address, and its form (valid, demand zero, in transition, in the page file, ...). |
| `SET PTE address field=value` | Change a page table entry. |
| `SHOW MEMORY [/STATISTICS]` | Memory regions, and with paging the page lists and faults. |
| `SHOW TB` | The translation buffer. |
| `SHOW SCB` | The system control block: the exception and interrupt vectors. |
| `SHOW EXCEPTIONS` | Recent machine faults and exceptions. |
| `SET BREAK/FAULT=code` | Stop when a machine fault is about to be delivered. |
| `SHOW SHIM` | The stubs that stand for run-time library routines. |
| `SHOW PROCESS`, `SET PROCESS pid` | List processes, and look at another one's registers and memory. |
| `SHOW KSP`, `ESP`, `SSP`, `USP`, `ISP` | Dump a mode's stack. |
| `SET MODE KERNEL` (or `EXECUTIVE`, `SUPERVISOR`, `USER`) | Switch the CPU's access mode. |
| `SET PSL field=value`, `SET VM`, `SET NOVM` | Change the PSL; turn memory management on or off. |
| `SHOW INSTRUCTIONS[/PROFILE]` | The instruction set; with `/PROFILE`, how often each instruction ran. |
| `SHOW SYMBOL/SYSTEM pattern` | The machine's predefined symbols (`PTE$K_UR`, `VAX$PR_SBR`, ...). |

### The interactive assembler

`ASM` at the `DBG>` prompt assembles VAX instructions straight into
memory, a line at a time (the prompt becomes `ASM>`) or from a file. It
uses MACRO syntax, with directives of its own such as `.BASE` (where to
assemble). `.END` returns to `DBG>`; `.END name` also calls the routine
`name`. This makes the debugger a laboratory for trying instructions:

```text
VAX> DEBUG
DBG> ASM
ASM> .BASE ^X1000
ASM> START:: MOVL #5, R1
ASM>  CLRL R0
ASM> LOOP: ADDL2 R1, R0
ASM>  SOBGTR R1, LOOP
ASM> DONE:: NOP
ASM> .END
DBG> EXAMINE/INSTRUCTION START:DONE
START:  MOVL     S^#05,R1
00001003:       CLRL     R0
LOOP:   ADDL2    R1,R0
00001008:       SOBGTR   R1,LOOP
DONE:   NOP
DBG> SET BREAK DONE
DBG> DEPOSIT PC = START
DBG> STEP/INSTRUCTION 3
stepped to 00001008: SOBGTR R1,^X00001005
DBG> EXAMINE R0,R1
%R0:    00000005
%R1:    00000005
DBG> GO
break at 0000100B
DBG> EVALUATE/DECIMAL R0
15
DBG> EXIT
```

Numbers in the assembler are decimal unless marked (`^X1000`), as in
MACRO. Code assembled this way runs in user mode; privileged instructions
such as `HALT` fault there, as on a real VAX (`%SYSTEM-F-OPCDEC`), so end
a test with a breakpoint, as above. `HELP ASM` at `DBG>` describes the
assembler's directives.

The debugger's `HELP` covers every command: `HELP SET BREAK`, `HELP
EXAMINE /INSTRUCTION`, `HELP EXPRESSIONS`, and so on.

---

## 13. Internals

This section explains how govax works inside: how it emulates the VAX
processor, how VMS's services fit around it, and how memory, processes,
and paging are managed. You don't need any of it to use govax, but it
explains what you see in `SHOW MEMORY`, `SHOW SYSTEM`, and the debugger,
and it's a gentle introduction to the VAX architecture and VMS's
internals for readers who know VMS only from the outside.

Two references are used throughout: DIGITAL's *VAX Architecture
Reference Manual* for the processor, and Kenah and Bate's *VAX/VMS
Internals and Data Structures* for how VMS works. govax's own design
notes are in `docs/`, a document per development phase.

### 13.1 The overall design

govax is one Go program. Inside it:

```text
  +---------------------------------------------------------------+
  |  Front end: the VAX> console, the DBG> debugger, cmd/govax     |
  +---------------------------------------------------------------+
  |  DCL interpreter (one per process that runs DCL)               |
  |  MACRO, LINK, LIBRARY, ANALYZE, DIRECTORY, COPY, ...           |
  +---------------------------------------------------------------+
  |  "VMS" in Go:  system services   RMS     run-time library      |
  |                processes, scheduler, lock manager, pager       |
  +-------------------------------------+-------------------------+
  |  The emulated VAX:  CPU (instruction set, exceptions,         |
  |  interrupts, access modes)  +  memory (page tables, TB)       |
  |  running the microkernel and your programs                    |
  +---------------------------------------------------------------+
  |  Host: files, terminal, clock                                  |
  +---------------------------------------------------------------+
```

The emulated VAX runs real VAX machine code: your programs, and a small
*microkernel* (`kernel.asm`, assembled into memory at startup) that sets
up the system control block, handles change-mode instructions, and
provides a few routines in VAX code. Everything else that VMS would do
in its executive (system services, RMS, the scheduler, the pager, the
run-time library) is Go code that the emulated machine calls into with
the `XFC` instruction, or that runs between instructions.

That split is the key to govax: from inside the machine, a program sees
a VAX running VMS (the same addresses, calling conventions, data
structures in its own memory, statuses, and side effects), while govax
avoids emulating the whole VMS executive in VAX code.

The Go packages follow the same layers; `CLAUDE.md` in the repository
describes each one. The most important are `internal/cpu` (the
processor), `internal/vm` (memory and address translation),
`internal/corevms` (system services and processes), `internal/rms` (RMS,
on the `ods2` module's Files-11 implementation), `internal/librtl` (the
run-time library), `internal/paging` and `internal/sched` (VMS's paging
and scheduling rules), `internal/dclshell` (DCL), `internal/asm`,
`internal/link`, `internal/lbr` (MACRO, LINK, LIBRARY), `internal/backup`
(BACKUP's save sets), and `internal/debugger`.

### 13.2 The processor

#### Registers and the PSL

The VAX has sixteen 32-bit general registers. R0 to R11 are for general
use (R0 and R1 also return results); R12 is AP, the argument pointer;
R13 is FP, the frame pointer; R14 is SP, the stack pointer; and R15 is
PC, the program counter.

The *processor status longword* (PSL) holds the processor's state:

| Bits | Field | Meaning |
| --- | --- | --- |
| 0-3 | C, V, Z, N | condition codes: carry, overflow, zero, negative |
| 4 | T | trace (single-step) |
| 5-7 | IV, FU, DV | trap enables: integer overflow, floating underflow, decimal overflow |
| 16-20 | IPL | interrupt priority level, 0 to 31 |
| 22-23 | PRV_MOD | the previous access mode |
| 24-25 | CUR_MOD | the current access mode: 0 kernel, 1 executive, 2 supervisor, 3 user |
| 26 | IS | running on the interrupt stack |
| 27 | FPD | first part done (an interrupted long instruction) |
| 30 | TP | trace pending |
| 31 | CM | compatibility mode (running PDP-11 code) |

The debugger's `SHOW PSL`, `SHOW REGISTERS`, and `SHOW CPU_STATUS` show
them, with the privileged registers (the page table registers P0BR, P0LR,
P1BR, P1LR, SBR, SLR; the stack pointers; SCBB; PCBB; and others).

#### Access modes

The VAX has four access modes, from most privileged to least: kernel,
executive, supervisor, and user. Your programs run in user mode. Each
mode has its own stack, and each page of memory says which modes may
read or write it. A program enters a more privileged mode only through a
*change mode* instruction (`CHMK`, `CHME`, `CHMS`), which goes to a
handler the system chose, and returns with `REI`. Privileged instructions
(`HALT`, `MTPR`, `LDPCTX`, ...) fault in any mode but kernel. This is the
protection VMS is built on, and govax enforces it as the architecture
defines.

#### Instructions

A VAX instruction is an opcode (one or two bytes) followed by operand
*specifiers*, each a byte giving the addressing mode and register,
followed by any displacement or literal. govax decodes instructions from
a table generated from the architecture manual: each opcode's operands,
with the data type and access of each (read, write, modify, address,
branch). The decoder evaluates the specifiers, and the instruction's
handler does the work. Every one of the VAX's instructions has a handler,
and a test checks that none is missing:

- **Integer and logical**: byte, word, longword, quadword, and octaword
  moves, arithmetic, logic, shifts, rotates, bit fields (`EXTV`, `INSV`,
  `FFS`, ...).
- **Floating point** in all four VAX formats: F (32-bit), D and G
  (64-bit), and H (128-bit). These aren't IEEE formats; govax implements
  them from scratch with exact arithmetic (Go's `math/big`), rounding
  once to the destination, as the hardware does, and raising the VAX's
  floating overflow, underflow, divide by zero, and reserved operand
  faults. `EMOD` and `POLY` are included.
- **Packed decimal**: `ADDP`, `MULP`, `DIVP`, `CVTLP`, `CVTPT`, `ASHP`,
  `EDITPC`, and the rest.
- **Character strings**: `MOVC3`, `MOVC5`, `CMPC`, `LOCC`, `SKPC`,
  `SCANC`, `SPANC`, `MATCHC`, `MOVTC`, `MOVTUC`, `CRC`.
- **Procedures and branches**: `CALLS`, `CALLG`, `RET`, `JSB`, `RSB`,
  `CASE`, `AOBLEQ`, `SOBGTR`, `ACBx`, `BBS`, ...
- **Queues**: `INSQUE`, `REMQUE`, and the interlocked `INSQHI`, `REMQHI`,
  `INSQTI`, `REMQTI`.
- **Privileged**: `MTPR`, `MFPR`, `PROBER`, `PROBEW`, `CHMx`, `REI`,
  `LDPCTX`, `SVPCTX`, `HALT`.

Fidelity is checked by tests that run hundreds of instruction cases and
compare them with the same cases run on VMS. Where govax knowingly
differs from the architecture, `docs/DEVIATIONS.md` says how.

#### Exceptions and interrupts

An *exception* is something an instruction causes: an access violation,
a reserved operand, an arithmetic error, a change-mode instruction. A
*fault* (an access violation, say) is taken with the PC pointing at the
faulting instruction, so it can be restarted; a *trap* (integer overflow,
say) with the PC after it. Either way the processor pushes the PC and
PSL, and goes to the routine whose address is in the *system control
block* (SCB), a page of vectors. The debugger's `SHOW SCB` lists them.

For a user program, VMS turns an exception into a *condition*: it builds
a signal array and a mechanism array on the user stack and calls the
condition handlers established in the call frames, innermost first (the
handler in the program's frame at `(FP)`, then its caller's, ...).
Below the program's frames is the image startup's. In an image linked
with traceback, its handler prints the message and the symbolic stack
dump of a warning, error, or severe condition. Any other condition
goes on to the catch-all, which prints its message, and for an
exception's condition that ends the image the "improperly handled
condition" dump of the registers and stack. A severe condition ends
the image. The last-chance vector is searched only when the frames
can't be read. govax does this in Go, calling the program's handlers as VAX
procedures, so `LIB$SIGNAL`, `$UNWIND`, `LIB$ESTABLISH`, and handlers
written in MACRO behave as on VMS (section 8, example 5).

The *trace fault* is taken at the start of an instruction when the
PSL's trace-pending bit (TP) is set; at each instruction's start the
processor copies the T bit into TP, so the fault comes after each
instruction that ran with T set. A fault that restarts its instruction
clears TP, so the instruction is traced when it completes; a trap keeps
it, so the trace follows the trap's handler. Every exception handler
starts with T and TP clear. VMS signals the trace fault as `SS$_TBIT`
(section 8, example 6).

*Interrupts* come from outside the instruction stream (the clock,
devices, software interrupts requested with `SIRR`) and are taken when
their priority level is above the processor's IPL. VMS uses software
interrupts at IPL 2 to deliver *ASTs* (asynchronous system traps):
routines a program asks to have called when an I/O completes, a timer
expires, or Ctrl/C is pressed. govax delivers ASTs between instructions,
in the access mode they were requested in, at the moments the
architecture allows.

#### Time

govax has two clocks to choose from (`vax.hardware.clock`):

- **Instruction-count time** (the default). Time advances with the
  instructions executed: after `vax.init`'s `SET QUANTUM 1`, each
  instruction is one emulated millisecond. When every process is waiting
  (for a timer, say), time jumps straight to the next timer. Every run of
  a program is therefore exactly repeatable: the same output, the same
  timings, the same interleaving of processes. govax's tests rely on this.
- **Host time**: the system time and the VAX's interval clock follow the
  host's real clock.

The time of day starts from the host's when govax starts, in both modes.

#### Speed

govax is an interpreter, and runs on the order of tens of millions of
VAX instructions per second on a modern computer: much faster than any
real VAX. It caches the current instruction page (the *fetch window*)
and recent address translations (the translation buffer, as the
hardware does). `--stats` shows the counts; `docs/PERFORMANCE.md`
describes how the emulator was profiled and tuned.

#### Extended function codes

The VAX reserves the `XFC` instruction for customer extensions. govax
uses it as the doorway from emulated code to the emulator: `XFC`
followed by a code byte calls into govax. The codes include console
character I/O (`XFC$CONSOLE_WRITE`, `XFC$CONSOLE_READ`), running a
console command from a program (`XFC$CONSOLE_CMD`), the system service
vector, the run-time library shims, and DCL's own entry points. `HELP
XFC` at `VAX>` lists the ones a program may use. A program built on VMS
never contains `XFC`; govax puts it only in the memory it manages (the
system service vector and the shims).

### 13.3 Virtual memory

#### The address space

A VAX address is 32 bits, so each process sees 4 GB of *virtual*
address space, in four regions of 1 GB:

| Region | Addresses | Holds |
| --- | --- | --- |
| P0, the program region | `00000000`-`3FFFFFFF` | the image and its data; grows up |
| P1, the control region | `40000000`-`7FFFFFFF` | the user stack (growing down from `7FE00000`), and system data per process: in govax, the system service vector at the top |
| S0, the system region | `80000000`-`BFFFFFFF` | the system: page tables, the privileged stacks, the microkernel, pool; the same in every process |
| S1 | `C0000000`-`FFFFFFFF` | reserved |

P0 and P1 are private to each process: the same address in two
processes is two different pages. S0 is shared by all.

Memory is divided into 512-byte *pages*. An address is a page number and
a byte within the page. Physical memory is divided into 512-byte *page
frames*, numbered by their *page frame number* (PFN).

#### Page tables

Each page's mapping is a 32-bit *page table entry* (PTE):

| Bits | Field | Meaning |
| --- | --- | --- |
| 31 | V | valid: the page is in memory, at the PFN |
| 27-30 | PROT | protection: which modes may read and write |
| 26 | M | modified: the page has been written |
| 0-20 | PFN | the page frame |

When V is clear, the hardware faults on any access (a *translation not
valid* fault), and the rest of the PTE belongs to the operating system,
which VMS uses to record where the page is (on a list, in a file, or to
be created as zeros). The protection codes name the most privileged
mode needed to read and to write the page:

| Code | Read | Write |
| --- | --- | --- |
| `NA` | no access | no access |
| `KR`, `KW` | kernel | none, kernel |
| `ER`, `EW`, `ERKW` | executive | none, executive, kernel |
| `SR`, `SW`, `SREW`, `SRKW` | supervisor | none, supervisor, executive, kernel |
| `UR`, `UW`, `URSW`, `UREW`, `URKW` | user | none, user, supervisor, executive, kernel |

The *system page table*, in physical memory, maps S0; the registers SBR
and SLR give its address and length. Each process's *P0 and P1 page
tables* live in S0, so they are themselves virtual (and, with paging on,
pageable in part); P0BR/P0LR and P1BR/P1LR give their addresses and
lengths. (P1's length register holds the lowest valid page, since P1
grows down.)

The processor caches recent translations in a *translation buffer*. Each
process switch flushes its process entries, as the `LDPCTX` instruction
does. The debugger's `SHOW TB` shows it, and `SHOW PAGE address` shows a
page's PTE and what its form means:

```text
DBG> SHOW PAGE 7FDFFE00
    Virtual address:   7FDFFE00
    Region:            01
    PTE Address:       8001CBFC
    PTE Longword:      A5802006
    PTE Form:          valid, PFN 00002006
        VALID: 1
        PROT:  04 (PTE$K_ALL)
        M:     1
        OWNER: 3
        S:     0
        PFN:   00002006
    Physical address:  00400C00
```

#### VMINIT: setting up memory

At startup, `vax.init` runs `INITIALIZE/VAX` (physical memory, 8 MB by
default) and then `VMINIT`, which writes VMS V1's system service vector
at the start of S0 (80000000: an entry per service, at its P1 vector
entry's offset, that jumps to that entry, for images linked before VMS
V4 that call services there), builds the system page table and the
console process's P0 and P1 tables, places the privileged stacks in S0
(kernel, executive, and supervisor, the latter two with no-access *guard
pages* below them so that an overflow faults instead of overwriting
something; and the interrupt stack), loads the page table registers, and
turns memory management on (`MAPEN`). `SHOW MEMORY` shows the result:

```text
VAX> SHOW MEMORY

    Physical Memory (8.00Mb)
        00004000 (16384 decimal) pages
        Addresses  00000000 - 007FFFFF

    Virtual Memory (currently enabled)
    There are 8198 physical pages mapped, 8186 free.

        P0 Region
            Size = 00004000 (16384 decimal) pages
            PFN database = 128 pages  P0BR = 80008C00    P0LR = 00004000
            Physical addresses = 00400000-00BFFFFF
            Virtual addresses  = 00000000-007FFFFF

        P1 Region
            Size = 00002000 ( 8192 decimal) pages
            ...
            Virtual addresses  = 7FC00000-7FFFFFFF

        S0 Region
            Size = 00002000 ( 8192 decimal) pages
            ...
            Virtual addresses  = 80000000-803FFFFF
```

With paging off (the default), memory management is simple. Every page of
S0 is mapped to physical memory. P0 and P1 pages are given a page frame
the first time they're touched, filled with zeros (*demand zero*), and
keep it until the image exits; nothing is ever written out. A program can
therefore use more P0 and P1 address space than there is memory, as long
as it doesn't touch more pages than there are frames. An image's pages
are copied into memory when it's activated.

The sizes of the regions and stacks are the `vax.memory.*` settings
(section 4). `docs/MODE-STACKS.md` describes the stacks' layout.

### 13.4 Processes and scheduling

#### Processes

A VMS *process* is a program's context: its address space (P0 and P1),
its registers, its privileged stacks, its quotas and privileges, its
logical names and channels. govax keeps two kinds of state:

- **The system**: what every process shares: devices and mounted volumes,
  mailboxes, common event flags, global sections, the lock database, the
  process table, the system logical name tables, and the S0 pool.
- **Each process**: its own P0 and P1 page tables, kernel, executive, and
  supervisor stacks (taken from the S0 pool, with guard pages), a
  hardware *process control block* (the 96-byte PCB that `LDPCTX` loads
  the registers from), and its VMS context: PID, name, UIC, privileges,
  quotas, event flags, ASTs, channels, logical names, and image.

The console's own process is process 1, PID `00000301`, named `SYSTEM`,
with VMINIT's tables and stacks. Other processes are created by
`$CREPRC`, `LIB$SPAWN`, and `SPAWN`, in a *job* (the console's process
and its subprocesses share the job logical name table and the job's
quotas, such as the page file quota). A detached process is a job of its
own.

Switching processes is what VMS's `SVPCTX` and `LDPCTX` instructions do,
and govax uses the same code: save the outgoing process's registers in
its PCB, load the incoming one's, and flush the process half of the
translation buffer.

#### Scheduling

Only one process runs at a time; govax schedules them by VMS's rules
(from *VAX/VMS Internals and Data Structures*, chapter 10):

- Every process has a base priority and a current priority, from 0 to
  31. Priorities 0 to 15 are *normal* (time-sharing), 16 to 31
  *real-time*. The console's process and its subprocesses have base
  priority 4.
- Computable processes wait in a queue per priority; the scheduler runs
  the first process in the highest non-empty queue.
- A normal process runs until it waits, uses up its *quantum*
  (`vax.process.quantum` instructions, 20,000 by default), or is
  preempted by a computable process of higher or equal priority. Then it
  goes to the back of its queue, so equal priorities take turns.
- When a waiting normal process's event happens (an I/O completes, a
  timer expires, an event flag is set), its priority is *boosted*, by an
  amount depending on the event, and then decays by one each time it's
  scheduled, back to its base. So interactive processes respond quickly,
  and compute-bound ones get the CPU when others are waiting.
- A real-time process is never boosted and has no quantum: it runs until
  it waits.

A process that calls a service that must wait (`$WAITFR`, `$HIBER`,
`$QIOW` to a terminal or mailbox, `$ENQW` for a held lock, ...) is put in
the corresponding VMS wait state, and another process runs. `SHOW SYSTEM`
shows each process's state:

| State | Meaning |
| --- | --- |
| `CUR` | current: it has the CPU |
| `COM` | computable: ready, waiting for the CPU |
| `LEF` | waiting for a local event flag (most I/O and timer waits) |
| `CEF` | waiting for a common event flag |
| `HIB` | hibernating (`$HIBER`) |
| `SUSP` | suspended (`$SUSPND`) |
| `MWAIT` | waiting for a resource, such as room in a full mailbox (`RWMBX`) |

Preemption happens at instruction boundaries, in the access modes
`vax.process.preempt` allows (all, as on VMS, by default), never at IPL
3 or above or on the interrupt stack. When every process is waiting, the
machine idles to the next timer.

The console's DCL is itself a process's command interpreter. Between DCL
commands, while you type at the prompt, and during `WAIT`, process 1
lends the CPU to the others, so `SPAWN/NOWAIT` subprocesses and detached
processes keep running. A subprocess's DCL runs in that process,
alternating with its images as VMS's DCL does.

#### Interprocess communication

All of VMS's mechanisms work between processes:

- **Mailboxes** (`$CREMBX`): message queues read and written with `$QIO`
  or RMS, with the termination mailbox a creator can ask for.
- **Common event flags** (`$ASCEFC`): flags shared by processes of a
  group.
- **Global sections** (`$CRMPSC`, `$MGBLSC`): memory shared between
  processes, backed by a file or the page file.
- **The lock manager** (`$ENQ`, `$DEQ`, `$GETLKI`): VMS's six lock modes,
  conversion queues, value blocks, blocking ASTs, and deadlock detection.
- **Shared files**: RMS's file sharing (FAB `FAC` and `SHR`) and record
  locking, with one shared file control block per open file.

### 13.5 Paging

With paging on, govax manages memory as VMS does: each process has a
*working set* of pages in memory, pages that leave it go to lists from
which they can be reclaimed, modified pages are written to a *page file*,
and images are mapped from their files and read in as they're used.

Turn it on with:

```sh
govax config set vax.sysgen.paging.enable=true
govax config set "vax.sysgen.paging.file=DUA0:[WORK]PAGEFILE.SYS"
```

The page file may be a volume file, as above, or a host file in quotes
(`'vax.sysgen.paging.file="./pagefile.sys"'`). `vax.init` then runs
`VMINIT/PAGING`, and govax creates the page file (`vax.sysgen.paging.size`
blocks, 32,768 by default) before your first command. Without a page
file, paging still works, but pages with no file of their own can't be
written out.

#### The page frame database

govax keeps a record for every physical page frame (VMS's *PFN
database*): its state, the virtual page it holds, its *backing store*
(where its contents live when it isn't in memory: an image file, a
section file, the page file, or nothing yet), and its share and
reference counts. A frame is in one of three states:

- **Active**: in a working set (or a global section's, or the system's).
- **On the free list**: available for reuse, but still holding its old,
  unmodified contents.
- **On the modified list**: holding contents that must be written to its
  backing store before the frame can be reused.

A page that leaves a working set isn't lost: its frame goes to the end
of the free or modified list, and its PTE is put in the *transition*
form, still naming the frame. If the page is touched again before the
frame is reused, the fault is resolved with no I/O at all: a *soft*
fault. The lists act as a cache of recently used pages.

#### Working sets

Each process has a *working set list*: the pages it has in memory. Its
size is bounded by three values, set from the process's quotas and
changed with `SET WORKING_SET` or `$ADJWSL`:

- the **limit** (the default working set size, which each image starts
  with);
- the **quota**: the size the process is guaranteed;
- the **extent**: the most it can grow to while memory is plentiful
  (`GROWLIM` free pages or more).

When a process faults in a page and its working set is full, the oldest
page in the list's dynamic part is replaced (passing over pages still in
the translation buffer, `TBSKIPWSL`, and locked pages: `$LKWSET`,
`$LCKPAG`). `$PURGWS` removes pages. When an image exits, its working set
goes back to the limit.

```text
VAX> SHOW WORKING_SET
  Working Set      /Limit=512   /Quota=1024    /Extent=16400
  Adjustment enabled      Authorized Quota=1024  Authorized Extent=16400
```

#### Page faults

When a program touches a page whose PTE isn't valid, the processor
faults, and govax's pager resolves it, by the PTE's form:

| Kind | The page was | Resolved by |
| --- | --- | --- |
| Demand zero | never used | taking a free frame and zeroing it |
| Soft (free or modified list) | in transition | taking its frame back off the list |
| Global valid | a global section page another process has in memory | sharing that frame |
| Hard (read) | in a file: the image, a section file, or the page file | reading it, and some of its neighbors (*read clustering*: the section's cluster size, or `PFCDEFAULT` pages) |

#### The modified page writer and the reclaimer

When the modified list grows past `MPW_HILIMIT` pages (500), the
*modified page writer* writes pages to their backing store, in clusters
of up to `MPW_WRTCLUSTER` (120), until the list is a cluster below the
high limit. Pages with no file of their own (demand-zero pages a program
modified, copy-on-reference pages) get blocks in the page file then. A
written page's frame moves to the free list, still holding its contents.

When the free list falls below `FREELIM` (32), the *reclaimer* frees
memory: it has the writer write the modified list down to `MPW_LOLIMIT`,
and trims working sets that grew past their quotas, until `FREEGOAL`
(200) pages are free.

#### The page file and quotas

The page file's space is allocated in runs, and released when a page is
deleted or modified again. govax warns on the console when the page file
passes 65% (`PAGEFRAG`) and 90% (`PAGECRIT`) full. Each job has a *page
file quota* (`PGFLQUOTA`): every page a process creates that could need
the page file (demand-zero pages, writable image pages, the user stack,
heap pages) is charged against it when it's created, and `$CRETVA`,
`$EXPREG`, and `LIB$GET_VM` fail with `SS$_EXQUOTA` past it. `SHOW
MEMORY/FILES` shows the page file as VMS does.

#### Sections and images

- A **private file section** (`$CRMPSC` with a channel) maps a file into
  the process's address space; its pages are read when first touched and
  written back to the file when modified (or with `$UPDSEC`).
- A **global section** is shared: its pages are counted by how many
  working sets hold them, stay on the lists when none does, and come back
  with a *global valid* fault in any process that maps it.
- A **page-file section** has no file: its pages are demand-zero and
  backed by the page file.
- A **copy-on-reference** page is read from its file, then becomes the
  process's own copy, backed by the page file.

`RUN` uses the same mechanisms to activate an image: each of the image's
sections is mapped from the image file (code read-only, writable data
copy-on-reference), and its demand-zero sections are created as
demand-zero pages. Nothing is read until it's touched. The image file
stays open while the image runs, so others may only read it. The
debugger's `SHOW IMAGE/FULL` shows each section's pages and where they
are.

#### The address space under paging

With paging on, a process's address space contains only the pages that
have been *created*: by image activation, by `$EXPREG`, `$CRETVA`,
`$CRMPSC`, or the run-time library's heap. P0LR and P1LR describe just
those pages, and grow and shrink as pages are created and deleted. An
access beyond them is an access violation, except below the user stack,
which grows by itself when user-mode code pushes past its bottom. Each
process's page tables take room in proportion to the pages it has
(`VIRTUALPAGECNT` bounds the total); only the parts of the tables in use
take memory.

#### Watching it work

This program touches 2,000 pages, twice, in a 1 MB demand-zero array:

```asm
        .TITLE  TOUCH   Touch 2000 pages, twice
        .PSECT  BIG, WRT, NOEXE, PAGE
ARRAY:  .BLKB   512*2000                ; 2000 pages, demand zero
        .PSECT  CODE, NOWRT, EXE, LONG
        .ENTRY  TOUCH, ^M<R2,R3>
        MOVL    #2, R3                  ; two passes
10$:    MOVAB   ARRAY, R2
        MOVL    #2000, R1
20$:    INCL    (R2)                    ; touch (and modify) each page
        ADDL2   #512, R2
        SOBGTR  R1, 20$
        SOBGTR  R3, 10$
        MOVL    #1, R0
        RET
        .END    TOUCH
```

Run with paging on, a page file, and a working set of 200 pages:

```text
VAX> SET WORKING_SET/LIMIT=200/QUOTA=200
VAX> RUN TOUCH
VAX> SHOW MEMORY
    ...
    Page faults                4010
        Free list              2000
        Modified list             0
        Global valid              0
        Demand zero            2008
        Read                      2
            Pages read            2
            Page file             0
    Pages written              3360
        Write I/Os               28
```

On the first pass, each page is a demand-zero fault. With only 200 pages
in the working set, each new page pushes an old, modified one out to the
modified list. When that list passes 500 pages, the modified page writer
writes it to the page file, in clusters of 120, and the frames go to the
free list. On the second pass, every page is still on the free list, so
each fault is a soft fault: no I/O. (The two reads are the image's own
pages.)

With much less memory (`vax.memory.size=1536`), the free list can't hold
the pages, their frames are reused, and the second pass must read them
back. `govax --stats` reports:

```text
  Paging:
    Page Faults:                    4,010
      Demand Zero:                  2,008
      Read:                         2,002
        Pages Read:                 2,002
          From Page File:           2,000
    Pages Written:                  3,360
      Write I/Os:                      28
```

#### How govax's paging differs from VMS's

- There is no swapping: processes are never moved out of memory as a
  whole.
- Page reads and writes happen at once; a process never waits for them
  (no `PFW`, `COLPG`, or `FPG` waits).
- The page tables and process header aren't in the working set list, so
  govax's smallest working set (`MINWSCNT`) is smaller than VMS's.
- S0 is not paged.
- There is one page file and no swap file.

The phase documents `docs/PHASE-62` to `PHASE-67` describe the design
in detail, and list every difference found by running the same probe
programs on VMS.

---

## 14. Programming in FORTRAN

govax is gaining a FORTRAN compiler, built a phase at a time
(`docs/PHASE-74 - FORTRAN plan.md` lays out the program, Phases 74 to
86). This chapter describes what is there so far, and grows with each
phase. The compiler accepts VAX FORTRAN V5.0, FORTRAN-77 with DEC's
extensions, as the 1988 *VAX FORTRAN Language Reference Manual*
describes it. It doesn't write object modules: it writes a MACRO-32
source file, which `MACRO` assembles and `LINK` links against FTNRTL,
govax's FORTRAN run-time library:

```text
$ FORTRAN SQUARES          ! SQUARES.FOR -> SQUARES.MAR
$ MACRO SQUARES            ! SQUARES.MAR -> SQUARES.OBJ
$ LINK SQUARES             ! SQUARES.OBJ + FTNRTL.EXE -> SQUARES.EXE
$ RUN SQUARES
```

The goal is programs that compile and run correctly, as the manual
defines the language and its run-time behavior. Matching DIGITAL's
compiler's object code, messages, or listings is not a goal.

**Where things stand (Phase 80, in progress, 2026-10-31):** the compiler reads
every statement of the language and gives every declaration its meaning
and its storage, compiles every expression in every type with every
intrinsic function, all of the language's control statements and
subprograms, and every formatted, list-directed, and namelist I/O
statement, with FORMAT checked as it compiles; FTNRTL runs formatted
input and output, with every edit descriptor but a variable format
expression, and list-directed input and output, and stops a program at any other I/O
statement with `%FOR-F-NOTIMP` (section 14.2). Any other statement draws
`%FORT-E-NOTIMPL`, naming it ("the OPEN statement is not implemented
yet"); the later phases widen the language.
`docs/FORTRAN-INTERNALS.md` is a tour of how the compiler works.

### 14.1 A first program

This program fills an array and prints the mean of its values, which a
function computes:

```text
      PROGRAM AVERAGE
      REAL V(4), MEAN
      DO 10 I = 1, 4
   10 V(I) = I * 1.5
      PRINT *, 'Mean:', MEAN(V, 4)
      END
C
      REAL FUNCTION MEAN(A, N)
      REAL A(*)
      MEAN = 0
      DO 10 I = 1, N
   10 MEAN = MEAN + A(I)
      MEAN = MEAN / N
      END
```

Compiled, assembled, linked, and run (FORTRAN, MACRO, and LINK say
nothing when all is well):

```text
VAX> FORTRAN AVERAGE
VAX> MACRO AVERAGE
VAX> LINK AVERAGE
VAX> RUN AVERAGE
Mean:   3.750000
```

The compiler checks the units of one file against each other. Without
`MEAN` in the main program's REAL statement, its first letter would make
it an INTEGER function there:

```text
VAX> FORTRAN AVERAGE
%FORT-E-FUNCTYPE, the function MEAN is REAL (line 8), but is used here as INTEGER
  at line 5 of DUA0:[WORK]AVERAGE.FOR;1
        PRINT *, 'Mean:', MEAN(V, 4)
  ------------------------^
%DCL-E-FORERRORS, 1 error(s) compiling DUA0:[WORK]AVERAGE.FOR;1; no MACRO file written
```

A program may be several files, each compiled and assembled on its own.
They share data through common blocks: a block of one name is the same
storage in every unit that declares it, whatever names each gives its parts,
and a block's initial values may come from a BLOCK DATA in a file of its own.
EXTERNAL says the program needs that unit, which no code calls. With these
three files,

```text
      PROGRAM STORE                   ! STORE.FOR
      EXTERNAL START
      COMMON /SHARED/ COUNT, TOTAL(3)
      INTEGER COUNT
      COUNT = 7
      CALL SHOW
      END

      SUBROUTINE SHOW                 ! SHOW.FOR
      COMMON /SHARED/ N, T(3)
      PRINT *, N, T
      END

      BLOCK DATA START                ! START.FOR
      COMMON /SHARED/ COUNT, TOTAL(3)
      INTEGER COUNT
      DATA TOTAL /3*2.5/
      END
```

```text
VAX> FORTRAN STORE,SHOW,START
VAX> MACRO STORE
VAX> MACRO SHOW
VAX> MACRO START
VAX> LINK STORE,SHOW,START
VAX> RUN STORE
          7   2.500000       2.500000       2.500000
```

More programs are in govax's `testdata/fortran` directory, each with the
output it prints, and programs of several files in its `modules` directory.

### 14.2 What the compiler accepts so far

- **Program units:** a main program, with or without a PROGRAM statement;
  SUBROUTINE; FUNCTION of any type, typed in its statement, by a type
  statement, or by its name's first letter, a CHARACTER one of a fixed
  length or `CHARACTER*(*)`, taking its length from the unit that references
  it; ENTRY in a SUBROUTINE or FUNCTION, each entry point with a dummy
  argument list of its own (a function's of any types that can share a
  value, section 6.2.4.1 of the language manual); `*` alternate returns in
  a SUBROUTINE's or ENTRY's dummy list; BLOCK DATA, named or not, which holds declarations
  and initial values for named common blocks; END. A file may hold several
  units.
- **Declarations:** type statements of every type and length (BYTE;
  LOGICAL `*1`, `*2`, `*4`; INTEGER `*2`, `*4`; REAL `*4`, `*8`, `*16`;
  DOUBLE PRECISION; COMPLEX `*8`, `*16`; DOUBLE COMPLEX; CHARACTER`*n`,
  `*(expr)`, and `*(*)` for a dummy argument, a function's own name, or a
  PARAMETER constant), and
  DIMENSION and VIRTUAL, of variables and arrays of up to seven dimensions
  with constant bounds (a lower bound with a colon, `V(0:9)`; a bound may
  be a constant expression, and a REAL one is truncated); a dummy array's
  last upper bound may be `*`, as in `A(3,*)`. A dummy array may be
  adjustable, `A(M,N)` or `X(-I/2:I/2,J)`, with bounds from dummy arguments
  or variables in common blocks, computed once each time the subprogram is
  entered, and may have an assumed size too, `A(1:N,1:*)`, and a passed
  length, `CHARACTER*(*) A(N)`. An entry point computes the bounds of the
  arrays its own list defines: those in its list whose bounds use only its
  other dummy arguments and common variables.
  COMMON, blank and named: units that declare a block of one name share its
  storage, each with names of its own, laid out in order with no gaps, and
  the block is as long as the longest declaration. EQUIVALENCE, with arrays,
  elements (a multidimensional array's by one subscript, its element number,
  too), and substrings: it may lengthen a common block at its end, never
  before its start. SAVE is accepted and checked; it changes nothing, since
  every variable keeps its value from call to call. EXTERNAL says a name is
  a routine outside the unit, even one with an intrinsic function's name
  (and naming a BLOCK DATA makes LINK find it in a library); INTRINSIC says
  it is FORTRAN's own. Either name as an argument passes the routine (below).
  Every variable gets its storage, and names of every type can be used in
  statements. Undeclared names have the implicit
  types: I to N INTEGER, the rest REAL, unless IMPLICIT says otherwise;
  with IMPLICIT NONE every name must be declared. The statements must
  come in the order the language sets (IMPLICIT, then the other
  declarations, then statement function definitions, then the executable
  statements).
- **Initial values:** DATA, with implied-DO lists and repeat counts
  (`DATA (B(I), I=1,10,2) /5*1/`), and values in slashes in a type statement
  (`INTEGER M(3)/1, 2*7/`). Numbers are converted to the item's type; octal
  and hexadecimal constants are extended or cut on the left; character and
  Hollerith constants, for a number too, are padded with blanks or cut on the
  right; an integer from 0 to 255 is a CHARACTER\*1 item's code. Any named
  common block may be initialized from any unit, but not blank common, dummy
  arguments, or a byte twice in one file. Two separately compiled modules
  initializing one byte of a block get whatever LINK leaves, the last one's.
- **Arguments:** a CALL or a function reference passes a number by reference
  and a CHARACTER value by descriptor. An argument may be left out, `CALL
  S(A, , B)`, which passes a longword 0. A name in an EXTERNAL or INTRINSIC
  statement passes the routine, which the routine given it calls through its
  dummy argument, `CALL P(X)` or `Y = F(X)`, of any type, with alternate returns,
  and passes on, in an EXTERNAL statement or after a call of it; a CHARACTER
  function goes with the length the passing unit declares, so a dummy declared
  `CHARACTER*(*) F` is that long. A Hollerith constant, `4HABCD`, passes its bytes
  by reference, for a numeric dummy argument. `%VAL(X)` passes a value as a
  longword (an integer or logical one shorter than that sign-extended, or a
  REAL\*4), `%REF(X)` by reference (a CHARACTER value's text), `%DESCR(X)` by
  descriptor, as Table 6-1 of the language manual allows, for routines not written
  in FORTRAN; `%LOC(X)` is the INTEGER\*4 address of a variable, array, element,
  substring, or procedure, in any expression, and 0 for a null argument. A FORTRAN
  routine is an ordinary procedure of the VAX calling standard, so it can also be
  a condition handler, an AST routine, or what `LIB$CALLG` calls.
- **Constants:** PARAMETER in both forms: `PARAMETER (N=10)`, where N has
  its declared or implicit type and the value is converted to it, and
  `PARAMETER N=10`, where N has its value's type. A value is a constant
  expression of any type, using `+ - * /`, `**` with an INTEGER exponent,
  `//`, the relational and logical operators, and MIN, MAX, ABS, MOD,
  ICHAR, NINT, DIM, DPROD, CMPLX, CONJG, IMAG, IAND, IOR, IEOR, NOT,
  ISHFT, LGE, LGT, LLE, LLT, and CHAR of constants. A constant's name may
  be used in declarations and statements.
- **Expressions:** INTEGER, REAL, COMPLEX, LOGICAL, and character constants;
  `+ - * /` and unary minus, on INTEGER, REAL, and COMPLEX of every length
  (REAL\*4 F_floating, REAL\*8 D_floating or with `/G_FLOATING` G_floating,
  REAL\*16 H_floating; COMPLEX\*8 a pair of F_floating values, COMPLEX\*16 of
  D_floating or G_floating ones), mixed by the usual conversions (INTEGER
  division truncates; two INTEGER\*2s are computed in a word, and a word that
  overflows wraps; a REAL\*4 beside a REAL\*8 is widened exactly, and a
  REAL\*8 assigned to a REAL\*4 rounded; a number beside a COMPLEX is its real
  part, with an imaginary part of 0.0, and a COMPLEX assigned to a number gives
  its real part; a COMPLEX\*8 beside a REAL\*8 is a COMPLEX\*16); the
  relational operators (`.EQ.` and the rest, and only `.EQ.` and `.NE.` on
  COMPLEX values, which are equal if both parts are); `.NOT.`, `.AND.`, `.OR.`, `.EQV.`, `.NEQV.`,
  `.XOR.`, on LOGICALs of every length or bit by bit on INTEGERs; array
  elements and
  function references. CHARACTER variables, array elements, and substrings,
  `S(2:5)`, `S(I:)`, `NAMES(K)(:J)`, whose bounds may be expressions (a
  constant bound must be within the variable); a character value compared
  with another, in ASCII's order (the codes as unsigned bytes, so 200 is
  after `Z`), the shorter as though filled with blanks; concatenation, `//`,
  whose value is built in a temporary before it is used, so `S = S(2:) //
  S(1:1)` is right (a temporary as long as the value can be, or, when that
  isn't known when compiling, as with a passed-length dummy argument, a
  dynamic string from LIB$SGET1_DD, freed by LIB$SFREE1_DD at the statement's
  end). `**` on every numeric type: an INTEGER, REAL, or COMPLEX base, an
  INTEGER, REAL, or COMPLEX exponent.
- **Intrinsic functions,** all of Table D-3's, by their generic names (chosen
  by the arguments' type, a small constant beside an INTEGER\*2 being one) or
  their specific ones. Written in line: ABS, IABS, MAX, MIN, MAX0, MIN0, MAX1, MIN1, AMAX0,
  AMIN0, DIM, IDIM, SIGN, ISIGN, and MOD of INTEGERs (each of every length);
  INT, IFIX, NINT, IDNINT, IQNINT, ZEXT, REAL, FLOAT, SNGL, DBLE, DFLOAT,
  QEXT, QFLOAT and their specific names, which return the default integer
  (INTEGER\*2 under `/NOI4`) where Table D-3's footnote 10 says so; CMPLX,
  DCMPLX, AIMAG, DIMAG, DREAL, CONJG, DCONJG, DPROD; IAND, IOR, IEOR, NOT,
  ISHFT by a constant count, IBITS, IBSET, IBCLR, BTEST, and ISHFTC (by FTNRTL's
  `FTN$IISHFTC` and `FTN$JISHFTC`); LEN, INDEX, ICHAR, CHAR, LLT, LLE, LGT,
  LGE; NWORKERS (1) and SIZEOF. Called in MTHRTL, with the arguments by
  reference: SQRT, EXP, LOG, LOG10, the trigonometric functions in radians
  and in degrees, their inverses, and the hyperbolic functions, of every REAL
  and (where Table D-3 has them) COMPLEX type; AINT, ANINT, and MOD of REALs;
  CABS and CDABS; and ISHFT by a count that isn't constant (`MTH$JISHFT`,
  `MTH$IISHFT`). The routine is the specific name's, with the format's letter
  where the name's differs: `MTH$SQRT`, `MTH$DSQRT` (`MTH$GSQRT` under
  `/G_FLOATING`), `MTH$HSQRT` for QSQRT, `MTH$ATAND2` for ATAN2D. A REAL\*16
  or COMPLEX\*16 value comes back through a hidden first argument, the
  address of the place for it.
- **Intrinsic functions' names as arguments:** a name in an INTRINSIC
  statement given as an actual argument passes the address of its routine,
  which the routine called may call with CALLS: MTHRTL's for most names
  (`MTH$SIN`, `MTH$JIABS`), and FTNRTL's for the 28 MTHRTL lacks (LEN, INDEX,
  ICHAR, CHAR, the lexical comparisons, BTEST, IBITS, IBSET, IBCLR, ISHFTC,
  ZEXT, SNGLQ, DBLEQ, QEXT, QEXTD, QFLOAT, NWORKERS, and the G_floating forms
  of DBLEQ and QEXTD; the FTNRTL table below). A generic name passed is its
  specific name of the default types: IABS is JIABS (IIABS under `/NOI4`), MAX
  JMAX0, LOG ALOG. SIZEOF has no routine, and is an error as an argument
  (`%FORT-E-NOTPASSABLE`).
- **Statement functions:** `AREA(R) = 3.14*R*R`, of any type, CHARACTER too,
  defined after the declarations and before the executable statements, and
  expanded in line at each reference. A dummy argument's name gives it only its
  type, whatever else the unit calls that name. Each actual argument must have
  its dummy's type (`%FORT-E-STMTFUNCARG`), but for a CHARACTER value's length
  and a constant that fits a shorter INTEGER. A definition may use only the
  statement functions before it (`%FORT-E-STMTFUNCLATE`), and they can't be
  called, passed, or assigned.
- **Statements:** assignment, of a character value to a character place cut
  or filled with blanks on the right to the place's length, and right even
  when the two overlap (`S(2:) = S` moves S one place on); logical IF; block IF with ELSE IF and
  ELSE; DO with a label, ending at CONTINUE or another statement, with
  any step, and a REAL control variable too; DO without a label, ended by
  END DO (`DO I = 1, N` ... `END DO`), and a labeled END DO as a labeled
  loop's terminal statement (`20 END DO`), which nested loops may not
  share (`%FORT-E-SHAREDENDDO`); DO WHILE (`DO WHILE (K .LT. 100)` or
  `DO 10 WHILE (...)`), its condition tested before every pass; a jump out
  of a loop and back into it (the extended range); GO TO in every form: the
  unconditional GO TO, the computed GO TO (`GO TO (10, 20, 30), K`, which
  goes on to the next statement when K is out of range; a REAL index is
  truncated), and the assigned GO TO (`GO TO K` or `GO TO K, (10, 20)`,
  the list naming where it may go) after `ASSIGN 10 TO K`, which stores the
  statement's address, so K must be INTEGER\*4 (an INTEGER\*2 can't hold
  one: `%FORT-E-ASSIGNVAR`); the arithmetic IF (`IF (X - Y) 10, 20, 30`) of
  an INTEGER or REAL value of any length; CALL, with alternate return
  specifiers (`CALL CHECK(A, B, *10, &20, C)`); RETURN, and RETURN i, which
  goes to the i-th specifier, or to the statement after the CALL if there is
  no i-th; STOP, with or
  without a display; PAUSE, with or without one, which writes the display
  (or `FORTRAN PAUSE`) and suspends the program at DCL's prompt: `CONTINUE`
  goes on at the next statement, `EXIT` or `STOP` end the program, and in
  a batch job or a process with no CLI the program just goes on.
- **Input and output** (chapters 7 and 8 of the language manual): READ,
  WRITE, PRINT, TYPE, ACCEPT, ENCODE, and DECODE, formatted (a FORMAT
  statement's label, an INTEGER variable ASSIGNed one, or a run-time format
  in a CHARACTER value or an array), list-directed (`*`), and namelist
  (NAMELIST groups, `NML=`); a unit number, `*`, or an internal file (a
  CHARACTER variable, element, substring, or array); ERR=, END=, and
  IOSTAT=; implied-DO lists, nested. FORMAT statements are checked as they
  compile, by the rules VMS's FORTRAN run-time library applies
  (`%FORT-E-FMTSYNTAX` points at the character in error). So far FTNRTL
  runs formatted and list-directed input and output, to the
  default units and units 5 and 6 (SYS$INPUT and SYS$OUTPUT). A format's
  text, H constants, X, T, TL, TR, slashes, groups, reversion, the colon,
  and the dollar sign work as section 8 of the language manual says, as do
  I, O, Z, F, E, D, G, L, A, and Q, both ways, with P, S, SP, SS, BN, and
  BZ, a comma ending an input field early, and Table 8-4's default widths,
  checked against VMS's library. Where it adds to the manual: F shows no
  minus sign on a value that rounds to zeros; E and D keep `-d < n < d+2`
  for a scale factor n, and past that, or with d 0 and n 0, are asterisks;
  an exponent past 99 with no `Ee` drops its letter (`0.1000+101`), and past
  999 is asterisks; and G writes zero in E form. A value too wide for its
  field is asterisks and `OUTCONERR` (63), after which the statement goes
  on; an input field the item can't hold is `INPCONERR` (64), the item set
  to 0; and I or L of an item that isn't an integer or a logical, or F, E,
  D, or G of one that isn't floating, edits its bytes as one and goes on,
  and the statement fails at its end with `FORVARMIS` (61), its records
  written, as VMS's library does. A unit with no OPEN takes blanks in a number as zeros, so `1 2`
  read by I3 is 102 (the manual's rule; BN reads it as 12). Every other I/O
  statement is FOR's `NOTIMP`, and so are units 0 to 4 and 7 to 119, which later phases
  add. A run-time format is checked when it is used: one VMS's FORTRAN
  run-time library would refuse is `SYNERRFOR` (62). A format that runs
  out of data edit descriptors with items left is `INFFORLOO` (60), and a
  record past column 133 `OUTSTAOVE` (66); reading, a record longer than
  199 characters is `INPRECTOO` (22). Unformatted, direct access, and keyed I/O, REWRITE,
  OPEN, CLOSE, and the like, and variable format expressions (`I<N>`), are
  `%FORT-E-NOTIMPL`.
- **I/O errors:** a statement with ERR=, END=, or IOSTAT= gets FTNRTL's
  status: IOSTAT's variable is set to 0, -1 at an end of file, or the
  error's FOR number (32 for an invalid unit, 40 for an I/O statement
  started in an item of another on the same unit); END= is taken at an end
  of file (ERR= without it), ERR= at an error. A statement with none of
  them stops the program, as on VMS:

  ```text
  %FOR-F-INVLOGUNI, invalid logical unit number
    unit 1000  file
    user PC 00000650
  %TRACE-F-TRACEBACK, symbolic stack dump follows
  ...
  ```
- **Arguments** are passed by reference: a routine that assigns to its
  dummy argument changes the caller's variable. An expression, or a
  variable in parentheses `(K)`, passes a copy. A CHARACTER value (a
  variable, element, substring, constant, or expression) is passed by
  descriptor, class S, as the VAX calling standard says; a whole CHARACTER
  array by a class A descriptor, but an assumed-size one by its first
  element's. A CHARACTER dummy argument declared `CHARACTER*(*)` has its
  actual argument's length, each time the routine is called, and is passed on
  with the descriptor it came with; one declared with a length has that
  length, which must be no more than the actual argument's. A passed-length
  array's elements have the length of the actual argument's.

### 14.3 The FORTRAN command

The command's syntax and qualifiers are in
[section 6.5](#fortran): the source files, `/MACRO` and `/NOMACRO`,
`/EXTEND_SOURCE`, `/D_LINES`, `/CONTINUATIONS=n`, `/[NO]I4`, and
`/[NO]G_FLOATING`. An `OPTIONS` statement before a unit may give `/I4`,
`/NOI4`, `/G_FLOATING`, or `/NOG_FLOATING` for that unit alone. From the
host shell, `govax fortran` runs it.

### 14.4 Source lines

A FORTRAN source is laid out in columns, from its days on punched
cards (the language manual's section 1.3):

| Columns | Field |
| --- | --- |
| 1-5 | the statement label, 1 to 5 digits (blanks and leading zeros don't count) |
| 6 | the continuation indicator: anything but a blank or 0 makes the line part of the statement before it |
| 7-72 | the statement (to 132 with `/EXTEND_SOURCE`) |
| 73-80 | a sequence number, ignored (none with `/EXTEND_SOURCE`) |

A tab within the first six columns ends the label field
(*tab-format* lines): after it, a digit from 1 to 9 is the continuation
indicator, and anything else starts the statement. A C, `*`, or `!` in
column 1, or a blank line, is a comment; a `!` elsewhere starts a
comment to the end of the line, unless it's in a character or Hollerith
constant. A D in column 1 marks a debugging line, compiled with
`/D_LINES` and a comment otherwise. A statement may have 19
continuation lines, or as many as `/CONTINUATIONS=n` says (to 99).
`CDEC$` lines are compiler directives (Phase 82 compiles them).

`INCLUDE 'file'` reads a file (type `.FOR` unless given) in the
statement's place. A file is found relative to the file that includes
it: on the host in its directory, on a volume on its device and in its
directory. Included files may include others, ten deep.
`INCLUDE '(module)'`, a text library's module, comes in Phase 83.

A problem with a line's layout is reported with the line and column;
after an error no MACRO file is written, and a fatal error (an INCLUDE
file that can't be read) stops the compilation:

```text
VAX> FORTRAN BAD
%FORT-E-BADLABEL, statement label field holds "X", which is not a label of 1 to 5 digits
  at line 2 of DUA1:[WORK]BAD.FOR;1
    X   Y = 2
  --^
%FORT-F-INCLOPEN, can't read the INCLUDE file 'NOWHERE': SYSTEM-W-NOSUCHFILE, File DUA1:[WORK]NOWHERE.FOR not found
  at line 5 of DUA1:[WORK]BAD.FOR;1
        INCLUDE 'NOWHERE'
  --------------^
%DCL-E-FORERRORS, 2 error(s) compiling DUA1:[WORK]BAD.FOR;1; no MACRO file written
```

### 14.5 The MACRO file

The MACRO file is meant to be read beside the program it came from. It
starts with what made it, and holds every line of the source as a
comment, with its number, just before the instructions it became, so
comments and declarations appear where they are in the source; a line
from an INCLUDE file has its include depth before the number, between
comments naming the file. For this program and its INCLUDE file,
`LIMITS.FOR`, which holds `INTEGER N`:

```text
      PROGRAM SQUARES
C     Squares of the first N integers
      INCLUDE 'limits'
      DO 10 I = 1, N
	PRINT *, I,
	1  I*I
10    CONTINUE
      END
```

FORTRAN writes this `SQUARES.MAR`:

```text
	.TITLE	SQUARES
	.IDENT	/01/
;
; Compiled by govax FORTRAN V1.0-612 on 30-OCT-2026 08:49:36.21
; from DUA0:[WORK]SQUARES.FOR;1
; by the command FORTRAN SQUARES
;
; Each line of the FORTRAN source appears as a comment, with its line number, before
; the instructions it became; a line from an INCLUDE file has its include depth
; before the number. Each routine's storage map says where its variables are.
;
; Descriptor codes (the OpenVMS Calling Standard's DSC$K_ values)
DSC$K_DTYPE_L = 8			; longword integer
DSC$K_CLASS_S = 1			; scalar or fixed-length string
;

; Storage map for SQUARES, the main program
;   N     INTEGER  $LOCAL+0  SQUARES.N
;   I     INTEGER  $LOCAL+4  SQUARES.I
;

	.PSECT	$LOCAL,PIC,CON,REL,LCL,NOSHR,NOEXE,RD,WRT,LONG
SQUARES.N:
	.BLKL	1			; N
SQUARES.I:
	.BLKL	1			; I
SQUARES.$C1:
	.BLKL	1			; DO 10's count
SQUARES.$T1:
	.BLKL	1			; temporary

	.PSECT	$PDATA,PIC,CON,REL,LCL,SHR,NOEXE,RD,NOWRT,LONG
SQUARES.$IO1:
	.WORD	8			; PRINT *'s control block: its size
	.WORD	0			; flags
	.LONG	-1			; unit *
SQUARES.$D1:
	.WORD	4			; length
	.BYTE	DSC$K_DTYPE_L,DSC$K_CLASS_S
	.ADDRESS SQUARES.I
SQUARES.$D2:
	.WORD	4			; length
	.BYTE	DSC$K_DTYPE_L,DSC$K_CLASS_S
	.ADDRESS SQUARES.$T1

	.PSECT	$CODE,PIC,CON,REL,LCL,SHR,EXE,RD,NOWRT,LONG
;    1        PROGRAM SQUARES
	.ENTRY	SQUARES,^M<>
;    2  C     Squares of the first N integers
;    3        INCLUDE 'limits'
; INCLUDE file DUA0:[WORK]LIMITS.FOR;1:
;1   1        INTEGER N
; end of DUA0:[WORK]LIMITS.FOR;1
;    4        DO 10 I = 1, N
	MOVL	SQUARES.N,SQUARES.$C1	; DO 10's count
	MOVL	#1,SQUARES.I
	TSTL	SQUARES.$C1
	BGTR	SQUARES.$2
	BRW	SQUARES.$1
SQUARES.$2:
SQUARES.$3:				; DO 10's body
;    5  	PRINT *, I,
;    6  	1  I*I
	PUSHL	#-1
	PUSHAB	SQUARES.$IO1
	CALLS	#2,G^FTN$WRITE_SL
	PUSHAQ	SQUARES.$D1
	CALLS	#1,G^FTN$IO_ITEM
	MULL3	SQUARES.I,SQUARES.I,SQUARES.$T1
	PUSHAQ	SQUARES.$D2
	CALLS	#1,G^FTN$IO_ITEM
	CALLS	#0,G^FTN$IO_END
;    7  10    CONTINUE
SQUARES.L10:
	INCL	SQUARES.I
	DECL	SQUARES.$C1		; one pass fewer
	BLEQ	SQUARES.$4
	BRW	SQUARES.$3
SQUARES.$4:
SQUARES.$1:
;    8        END
	MOVL	#1,R0			; SS$_NORMAL
	RET

	.END	SQUARES
```

Each source line's prefix is one tab stop wide, so tab-format lines keep
their layout. What to look for in the code:

- **The module** is named for the file's first program unit (here the
  PROGRAM statement's name; a main program without one is named
  `file$MAIN`, as section 4.12 of the language manual says), and `.END`
  names the main program, the image's transfer address. Each program
  unit is a routine, a `.ENTRY` with its register save mask, called by
  `CALLS` under the VAX calling standard.
- **Storage is static.** Each routine's variables, arrays, and the
  compiler's cells are in the `$LOCAL` program section, named
  `routine.name` (`SQUARES.I`), so `MACRO/DEBUG` and the debugger can
  name them; constants, descriptors, and I/O control blocks are in
  `$PDATA`, read-only; the code is in `$CODE`. The compiler's own names
  have a `$`: `$Cn` a DO loop's count, `$Sn` its step, `$Tn` a
  temporary, `$Kn` and `$Fn` constants, `$Dn` descriptors, `$IOn` I/O
  control blocks, `$Xn` part of an expression set aside, and `$n` a
  label. FORTRAN's label 10 is `SQUARES.L10`. A name longer than MACRO's
  31 characters becomes a numbered one, `.$Nn`, listed in the storage
  map.
- **Common blocks** come first in the file, each once, as long as the
  longest declaration of it in the file, and holding every unit's initial
  values: a program section of the block's name (blank common's is
  `$BLANK`), overlaid, so that LINK starts every module's part at one
  place. A unit's names in it are assignments, `STORE.TOTAL =
  SHARED.$BASE+4`, as are names that EQUIVALENCE overlays, and take no room.
- **Initial values** are in the storage itself: `.LONG`, `.WORD`, `.BYTE`,
  and `.ASCII`, with `.BLKB` for the gaps. A floating value is in
  hexadecimal, its source spelling in the comment; items in a row with one
  value are a repetition, `.LONG 0[1000]`.
- **Dummy arguments** are reached through the argument list:
  `ISQ.N = 4` names N's offset, and `@ISQ.N(AP)` is N.
- **A DO loop** computes its iteration count first, skips the body if
  it is zero or less, and counts down at the end (section 5.3.1.1 of
  the manual); the variable ends one step past its last value.
- **Every branch reaches** however far its target is: a conditional
  branch is the opposite condition's byte branch around a `BRW`.

### 14.6 FTNRTL, the run-time library

FTNRTL.EXE holds the routines compiled programs call (I/O, STOP, and so
on). It is a shareable image written in MACRO-32, its sources in
`internal/ftnrtl` (`mar/*.mar` and the options file `ftnrtl.opt`), and
built into govax, so it is always there. RUN and LINK look for it as for
any shareable image (a logical name `FTNRTL`, `SYS$SHARE:FTNRTL.EXE`,
the host library directory) and take govax's own last, so `LINK`
resolves its routines with no options file. Its routines are named
`FTN$...`, and its transfer vector keeps each at a fixed offset, so a
program linked against one FTNRTL runs with any later one.

So far it has these routines, which MACRO programs can call too:

| Routine | What it does |
| --- | --- |
| `FTN$VERSION([text])` | Returns FTNRTL's version in R0 (`^X00010000` for V1.0) and, given a string descriptor, writes `FTNRTL V1.0` there, blank-padded or cut short |
| `FTN$STOP([display])` | FORTRAN's `STOP`: writes the display (or `FORTRAN STOP` without one) to `SYS$OUTPUT` and ends the image with `SS$_NORMAL` |
| `FTN$PAUSE([display])` | FORTRAN's `PAUSE`: writes the display (or `FORTRAN PAUSE` without one) to `SYS$OUTPUT`, then calls `LIB$PAUSE`, and returns when the program is to go on |
| `FTN$READ_SL(icb, unit)`, `FTN$WRITE_SL(icb, unit)` | Start a list-directed READ or ACCEPT, or WRITE, PRINT, or TYPE statement: `icb` is its control block (a word of size, 8; a word of flags, bit 0 ERR=, bit 1 END=, bit 2 IOSTAT=, bit 3 a run-time format; a longword, 0), `unit` the unit by value (-1 for PRINT and `WRITE (*,...)`, -2 for TYPE, -3 for ACCEPT, -4 for `READ f` and `READ (*,...)`) |
| `FTN$READ_SF(icb, unit, format)`, `FTN$WRITE_SF(icb, unit, format)` | Start a formatted READ, or WRITE, PRINT, or TYPE: `format` is a FORMAT statement's block (the address of a longword where FTNRTL keeps the parsed format, a word of the text's length, a word of flags, the text's address) or, with the control block's bit 3, a run-time format's descriptor |
| `FTN$READ_SN`, `FTN$WRITE_SN`, `FTN$READ_IF`, `FTN$WRITE_IF`, `FTN$READ_IL`, `FTN$WRITE_IL`, `FTN$DECODE`, `FTN$ENCODE` | Start the other I/O statements (namelist, an internal file), given a format too; so far each is `FOR$_NOTIMP` |
| `FTN$PRINT_LIST(icb)`, `FTN$WRITE_LIST(icb)` | Start `PRINT *` and `WRITE (*,*)` as the compiler called them before Phase 80, kept for programs linked then |
| `FTN$IO_ITEM(desc)` | Reads or writes one value, described by a descriptor: a scalar (class S) or a whole array (class A) of `BYTE` (`DSC$K_DTYPE_BU`), `LOGICAL*2` and `*4` (`WU`, `LU`), `INTEGER*2` and `*4` (`W`, `L`), `REAL*4` (`F`), `REAL*8` (`D` or `G`), `REAL*16` (`H`), `COMPLEX*8` (`FC`), `COMPLEX*16` (`DC` or `GC`), or text (`T`) |
| `FTN$IO_END()` | Ends the statement, writing its last record, and returns its status in R0: 0, -1 at an end of file, or the error's FOR number; signals the error if the control block names none of ERR=, END=, and IOSTAT= |
| `FTN$JISHFTC(i, shift, size)` | The intrinsic function ISHFTC of INTEGER\*4s (by reference): the low `size` bits of `i` shifted `shift` places to the left (`-shift` to the right), circularly, the other bits as they were, in R0 |
| `FTN$IISHFTC(i, shift, size)` | The same of INTEGER\*2s, its word sign-extended to an INTEGER\*4 in R0, as Table D-3 gives IISHFTC's result |
| `FTN$LEN(s)`, `FTN$INDEX(s, t)`, `FTN$ICHAR(s)` | LEN, INDEX, and ICHAR of strings by descriptor, an INTEGER\*4 in R0 |
| `FTN$CHAR(result, i)` | CHAR: the character whose code is `i`'s low byte, into the string `result`'s descriptor describes |
| `FTN$LLT(s, t)`, `FTN$LLE`, `FTN$LGT`, `FTN$LGE` | The lexical comparisons, the shorter string padded with blanks: -1 (.TRUE.) or 0 in R0 |
| `FTN$BITEST(i, pos)`, `FTN$BJTEST(i, pos)` | BTEST of an INTEGER\*2 or \*4: -1 or 0 in R0 |
| `FTN$IIBITS(i, pos, len)`, `FTN$JIBITS` | IBITS, an INTEGER\*4 in R0 (IIBITS's word sign-extended) |
| `FTN$IIBSET(i, pos)`, `FTN$JIBSET`, `FTN$IIBCLR`, `FTN$JIBCLR` | IBSET and IBCLR in R0 (IIBCLR's word sign-extended to an INTEGER\*4) |
| `FTN$IZEXT(i)`, `FTN$JZEXT(i)` | ZEXT of the word at `i`, in R0 |
| `FTN$SNGLQ(q)`, `FTN$DBLEQ(q)`, `FTN$GBLEQ(q)` | A REAL\*16 rounded to a REAL\*4 in R0, or a REAL\*8 in D_floating or G_floating in R0 and R1 |
| `FTN$QEXT(result, x)`, `FTN$QEXTD`, `FTN$QEXTG`, `FTN$QFLOAT` | A REAL\*4, a REAL\*8 in D_floating or G_floating, or an INTEGER\*4 made a REAL\*16 at `result` |
| `FTN$NWORKERS()` | 1 in R0 |

Every routine but ISHFTC's is the routine of an intrinsic function MTHRTL
hasn't, for a program that passes the function's name as an argument; a
reference to the function is written in line.

A list-directed statement writes `SYS$OUTPUT`, with FORTRAN carriage
control: each record begins with a blank (a new line at the terminal),
and holds at most 80 bytes after it. A number's field begins with a blank of its
own, which at a record's start is that carriage control; a text has none, so
one that starts a record gets a blank first. These layouts are those of VMS's
FORTRAN run-time library, checked against it. Each value is in its type's format (the
*Language Reference Manual*'s Table 7-1): a LOGICAL\*2 or \*4 as ` T` or
` F` (L2), a LOGICAL\*1 or BYTE as an integer in 5 columns (I5), an
INTEGER\*2 in 7 columns (I7), an INTEGER\*4 in 12 (I12), a REAL as
`1PG15.7E2`, a REAL\*8 as `1PG24.16E2` (`1PG24.15E3` in G_floating), a
REAL\*16 as `1PG43.33E4`, a COMPLEX as its two parts in `1PG14.7E2`
(`1PG23.16E2` for COMPLEX\*16, `1PG23.15E3` in G_floating) between `(`, `,`,
and `)`, and text as itself. `1PG15.7E2` shows 7 significant digits: a magnitude from 0.1
up to 10\*\*7 in fixed-point form, followed by 4 blanks; any other, zero
included, with an exponent. The wider formats are the same with their own
digits, and an exponent of 2, 3, or 4 digits:

```text
      PRINT *, 42, .TRUE., 'IT''S', 3.4, 0.25, 1234567.0, 1.0E-5, 0.0
         42 TIT'S   3.400000      0.2500000       1234567.      9.9999997E-06
 0.0000000E+00
```

(1.0E-5 shows as `9.9999997E-06` because the F_floating value nearest
it is just below it.) The same at REAL\*8:

```text
      PRINT *, 1D0/3D0, 1D16, 0D0
 0.3333333333333333      1.0000000000000000E+16  0.0000000000000000E+00
```
 A value that doesn't fit in what's left of the
record starts the next one: the first record above, at 78 bytes, had no
room for the last REAL. Only a text longer
than a record is split, filling as many records as it needs. With
`SYS$OUTPUT` a print file, such as a command procedure's `/OUTPUT=`
log, each record's carriage control becomes the record's print control
(a line, a blank line first, a new page, overprinting, a prompt), as a
terminal shows it, rather than a character of the record.

A COMPLEX value has no blanks inside it (the manual's 7.3.1.2), so each part's
field is written without its blanks, with one blank before the value, as
every other number's field begins with one:

```text
      PRINT *, (1.0, -1.5), (0.0, 0.08)
(1.000000,-1.500000) (0.0000000E+00,7.9999998E-02)
```

A list-directed READ or ACCEPT takes values separated by blanks, tabs, record
ends, and at most one comma between two. A comma with no value before it is a
null value, which leaves its item as it was; a slash ends the statement, leaving
the rest; `3*5` is three values and `3*` three nulls. A number goes to any
numeric item, truncated for an integer (14.6 is 14); `T`, `F`, `.T`, or `.F`, in
either case with anything after them, goes to a logical, as does a number;
`(re,im)` goes to a COMPLEX, as does a number alone, the real part; `'text'`
goes to a CHARACTER item, cut off or padded with blanks, and may run across
records. What can't start a value (`ABC`) is `%FOR-F-LISIO_SYN`; a value of the
wrong kind for its item, or a number too large for it, is `%FOR-F-INPCONERR`.
Either way the item keeps its value.

```text
VAX> TYPE STOPPER.MAR
	.TITLE	STOPPER
	.PSECT	DATA,NOWRT,NOEXE,LONG
TEXT:	.ASCID	/ALL DONE/
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	PUSHAQ	TEXT
	CALLS	#1,G^FTN$STOP	; STOP 'ALL DONE'
	.END	START
VAX> MACRO STOPPER
VAX> LINK STOPPER
VAX> RUN STOPPER
ALL DONE
VAX> SHOW SYMBOL $STATUS
  $STATUS == "%X00000001"
```

To change FTNRTL, build your own from its sources and point the logical
name at it:

```text
VAX> MACRO FTN_VECTOR
VAX> MACRO FTN_CONTROL
VAX> MACRO FTN_UNITS
VAX> MACRO FTN_LISTOUT
VAX> MACRO FTN_INTRINSICS
VAX> LINK/SHAREABLE=FTNRTL/NOSYSLIB FTNRTL/OPTIONS
VAX> DEFINE FTNRTL DUA0:[WORK]FTNRTL.EXE
```

### 14.7 Differences from VAX FORTRAN

Where the manuals at hand don't settle a rule, govax makes a choice
(each phase's document lists its own, unconfirmed on VMS, since no VAX
FORTRAN compiler was available to check them). So far:

- `/CONTINUATIONS` allows 0 to 99, and 19 without it.
- In a tab-format line, the tab stands for the columns before column 7,
  so the statement still ends at column 72 (or 132).
- A line whose first nonblank character is `!` is a comment line, and
  doesn't end a statement whose continuation lines follow.
- Lines aren't padded with blanks to column 72: a character constant
  continued from a short line goes straight on to the next.
- `CPAR$` lines (parallel directives) are comments.
- INCLUDE files are found relative to the including file, ten deep at
  most.
- `FTN$STOP` and `FTN$PAUSE` write to `SYS$OUTPUT`.
- The statements without a unit, and a unit `*`, use VMS's default units
  -1 to -4 (-1 for PRINT and `WRITE (*,...)`, -2 for TYPE, -3 for ACCEPT,
  -4 for `READ f` and `READ (*,...)`); which statement uses which isn't
  known from VMS.
- An I/O error is signaled when its statement ends, after the statement's
  items (and any function they call) have run; VMS's run-time library may
  signal it sooner.
- A FORMAT's groups may nest 8 deep inside its own parentheses (VMS takes
  8 and refuses 16), and a comma is optional around an H constant, as it
  is around a character constant.
- At `PAUSE`, DCL's `DEBUG` is govax's console `DEBUG` (as at any
  `LIB$PAUSE`), not the VMS debugger attached to the program.
- Where one file holds both a routine and a reference to it, the
  compiler checks them against each other: a FUNCTION's type, a
  SUBROUTINE called and not referenced as a function, the number of
  arguments, and that alternate return specifiers are in the places of the
  routine's `*`s. VAX FORTRAN compiled each unit alone and couldn't.
- An alternate return specifier has no longword in the argument list
  (`CALL CHECK(A, B, *10, *20, C)` is `CALLS #3`), and the number RETURN i
  chooses comes back in R0, 0 from a plain RETURN; a MACRO routine a FORTRAN
  program calls with specifiers does the same. The language manual doesn't
  say how VAX FORTRAN laid these out.
- A statement function's actual argument is assigned to a temporary of its
  dummy argument's type and length, as an assignment would, unless it is a
  constant or a variable of that very type and length, which is used where it
  is; so a CHARACTER one is cut or padded to the dummy's length, and the value
  to the function's. The language manual doesn't say. A dummy argument named
  for a `CHARACTER*(*)` name is an error (`%FORT-E-STMTFUNCLEN`), FORTRAN-77's
  rule.
- Changing a DO loop's control variable inside the loop is an error
  (the manual forbids it; govax checks the loop's own statements).
- `.TRUE.` is the longword -1 and `.FALSE.` 0 (only bit 0 counts); on
  LOGICAL values `.AND.`, `.OR.`, and `.NOT.` work bit by bit, as they
  do on INTEGERs.
- A name only declared, never used, is a variable, and gets storage.
- A program unit's name names nothing else in its file: with a main
  program called TEXT, TEXT can't be a variable or dummy argument of
  another unit in the file (`%FORT-E-BADUSE`), which FORTRAN-77 seems to
  allow in a unit that doesn't refer to the program.
- A COMPLEX\*8 product is computed in line, as
  `(ac - bd, ad + bc)` with each product and sum rounded to F_floating, since
  MTHRTL has no routine for it; a COMPLEX\*16 product calls `OTS$MULCD_R3`
  (or `OTS$MULCG_R3`), and every COMPLEX quotient `OTS$DIVC`, `OTS$DIVCD_R3`,
  or `OTS$DIVCG_R3`, whose parts are each computed exactly and rounded once.
  A constant expression's product or quotient is computed the same way, so it
  has the value the program would compute; a COMPLEX\*8 product can differ
  from the exactly rounded one in a part's last bit.
- A power is computed by MTHRTL's `OTS$` routines (`OTS$POWJJ`, `OTS$POWRJ`,
  `OTS$POWDD`, `OTS$POWCJ`, and the rest), but for an INTEGER or REAL base
  with a constant exponent from 1 to 8, which is multiplied out in line:
  `X**2` is `X*X`. Either way, and in a constant expression too, a power to an
  INTEGER exponent is repeated squaring, rounded at each product, and for a
  negative exponent the product of the squares' reciprocals, as the `OTS$`
  routines' are, so `X**7` is `(X * X**2) * X**4`, each product rounded, and
  `2.0 ** (-127)` is in range though `2.0 ** 127` isn't. A COMPLEX power is
  always called, its products' parts rounded once. An INTEGER\*2 to an
  INTEGER\*2 power (or to a small constant) is an INTEGER\*2, so with `I2` at
  10, `I2**5` wraps to -31072; in line, an INTEGER power that overflows wraps,
  where the routine signals `SS$_INTOVF`. A REAL\*4 and a REAL\*8 are
  computed by `OTS$POWRD` or `OTS$POWDR` in D_floating, and both in G_floating
  under `/G_FLOATING`, which has no such routine. Zero to a power of zero or
  less is an error in a constant expression, and `MTH$_UNDEXP` when the
  program runs. MIN, MAX, MOD, DIM, IAND, IOR, and IEOR need arguments of
  one kind.
- An intrinsic function's arguments must be of one kind (but CMPLX's two),
  and a generic name selects by the highest of their types, as in a mixed
  operation: `MAX(I2, 5)` is IMAX0, an INTEGER\*2, since the constant fits a
  word, and `MAX(I2, J)` with J an INTEGER\*4 is JMAX0. A specific name for
  INTEGER\*2 arguments takes an INTEGER constant that fits one (`IIABS(5)`), but
  no other argument of another type. LOGICAL arguments are taken only where
  Table D-3 lists them (ZEXT, CHAR).
- IIBITS, IIBCLR, and IISHFTC return an INTEGER\*4, as Table D-3 prints them,
  computing in a word and sign-extending it: with `I2` at -1, `IBCLR(I2, 0)` is
  the INTEGER\*4 -2. IIBSET returns an INTEGER\*2, and BITEST a LOGICAL\*2.
- The intrinsic functions that choose a value (ABS, MAX, MIN, DIM, SIGN) are
  computed before the statement that uses them, as a function's call is, so in
  `IF (I .NE. 0 .AND. MAX(10/I, 1) .GT. 2)` the quotient is computed even when
  `I` is 0. MOD of INTEGERs is `X - (X/Y)*Y`, which a zero Y ends with the
  divide-by-zero trap. ISHFT by a constant count is written in line, a count
  of the integer's width or more giving 0.
- A generic intrinsic function's name passed as an argument is its specific
  name for the default integer, or else for a REAL\*4 (`IABS` is JIABS, `INT`
  JINT), by `/[NO]I4`. JZEXT and IZEXT passed read a word whatever they are
  given, so a passed JZEXT given an INTEGER\*4 loses its high word; CHAR passed
  reads a byte. SIZEOF can't be passed (`%FORT-E-NOTPASSABLE`).
- Variables are placed in `$LOCAL` at their natural alignment, at most
  a longword's: a `LOGICAL*1` or a CHARACTER variable anywhere, an
  `INTEGER*2` on a word, the rest on a longword.
- An integer constant from -32768 to 32767 beside an INTEGER\*2 or a
  shorter logical is a word, as the language manual's 2.2.1.1 says, so with
  `I2` at 100, `J = I2*1000` wraps to -31072. Two constants are computed in
  the default integer, so `30000+30000` is 60000. Two LOGICAL\*1s or BYTEs in
  arithmetic are an INTEGER\*2; a LOGICAL beside an INTEGER takes that
  INTEGER's length.
- A constant assigned to a short variable may have the variable's unsigned
  range too (`B = 255` in a BYTE is -1); past that it is an error.
- A letter given a type by two IMPLICIT rules is an error.
- A common block's names have no gaps between them, whatever their types.
  In `$LOCAL`, an EQUIVALENCE group goes on the boundary of its strictest
  member, the others where the lists put them.
- The standard's rules the manual doesn't state are errors: a dummy argument
  or function result in COMMON or EQUIVALENCE, a name in two blocks, two blocks
  associated, DATA for a dummy argument, a function result, or blank common.
- DATA may initialize a named common block in any unit, not only BLOCK DATA,
  though a BLOCK DATA unit may initialize only names in common blocks.
- Blank common's program section is `$BLANK`, and an unnamed BLOCK DATA's name
  is `file$DATA`, so a file holds at most one.
- An EXTERNAL name is a reference in the object module whether or not code
  uses it, so LINK reports one that nothing defines.
- A character or Hollerith constant may initialize a LOGICAL item as it may a
  number, and INTEGER and LOGICAL constants initialize each other's items, as
  assignment converts them. A REAL literal for a `REAL*8` item is rounded from
  its exact value: `DATA D /0.1/` is 0.1 in D_floating.
- A REAL literal in a REAL\*8 expression or assigned to a REAL\*8 keeps its
  exact value, as the manual's 2.3.1.2 says of a literal in a higher-precision
  expression: `D = 0.1` is 0.1D0, not the REAL\*4 value widened.
- `FUNCTION F*8(X)`, a length with no type keyword, gives the length to
  the type F has from its first letter (here `REAL*8`).
- Integer overflow isn't detected (the result wraps, as with VAX
  FORTRAN's `/CHECK=NOOVERFLOW`); an integer division by zero ends the
  program with the VAX's divide-by-zero trap.
- A constant passed as an argument is read-only: a routine that assigns
  to its dummy argument gets an access violation.
- A substring's bounds that aren't constants aren't checked when the program
  runs (as with VAX FORTRAN's `/CHECK=NOBOUNDS`): a substring past its
  variable's end reaches the bytes after it, and one whose last position is
  before its first moves or compares the characters an unsigned word of its
  length says, so `S(5:4)` is empty but `S(5:3)` is 65535 characters long.
  Nor is a substring of a passed-length dummy argument checked against its
  length when compiling, but for a constant bound less than 1 or a first
  position after the last.
- A character constant passed as an argument is passed by descriptor. VAX
  FORTRAN's compiler and linker together pass one given to a numeric dummy
  argument as a Hollerith constant, wherever the routine is. govax's LINK can't,
  so that happens only when the file holds the routine; otherwise a routine with
  a numeric dummy argument is given the descriptor (`%REF('ABCD')` says which is
  meant). The file's routine is also checked: a Hollerith constant can't go to
  its CHARACTER dummy argument (`%FORT-E-HOLLCHAR`).
- A procedure's `%DESCR`, and a CHARACTER function passed by default, is a class
  S descriptor of data type `DSC$K_DTYPE_ZEM`, its length the function's, or 0 for
  any other procedure; the manuals don't give these. An EXTERNAL name that a
  type statement or IMPLICIT makes CHARACTER is taken for a CHARACTER function,
  even if it is a subroutine, by caller and dummy alike.
- `%DESCR` of an assumed-size array describes its first element, as passing a
  CHARACTER one does.
- A CHARACTER array passed whole is described by a class A descriptor, and an
  assumed-size one by its first element's class S descriptor (a routine reads
  only the length and the address of either). A concatenation longer than
  65535 characters isn't reported.
- List-directed output's records hold 80 bytes after their carriage
  control; values follow one another with no blanks between them but
  their fields' own; a text that doesn't fit begins a new record; zero
  is written with an exponent (`0.0000000E+00`). `PRINT *` and `WRITE
  (*,*)` both write `SYS$OUTPUT`. All of this is VMS's library's.
- List-directed input takes what the language manual refuses, as VMS's
  library does: a number alone for a COMPLEX item, and a number for a
  LOGICAL item (`1` stores 1). `0*7` stores the 7, then is
  `%FOR-F-LISIO_SYN`.

---

## 15. Appendices

### Appendix A: System services

govax provides these system services. Their arguments, statuses, and
side effects follow DIGITAL's *System Services Reference Manual*.

| Area | Services |
| --- | --- |
| I/O | `$ASSIGN`, `$DASSGN`, `$QIO`, `$QIOW`, `$CANCEL`, `$ALLOC`, `$DALLOC`, `$BRKTHRU(W)`, `$SNDOPR` |
| Logical names | `$CRELNM`, `$DELLNM`, `$TRNLNM`, `$CRELNT`, `$CRELOG`, `$DELLOG`, `$TRNLOG` |
| Time and timers | `$GETTIM`, `$ASCTIM`, `$BINTIM`, `$NUMTIM`, `$SETIMR`, `$CANTIM`, `$SCHDWK`, `$CANWAK` |
| Event flags | `$SETEF`, `$CLREF`, `$READEF`, `$WAITFR`, `$WFLOR`, `$WFLAND`, `$ASCEFC`, `$DACEFC`, `$DLCEFC` |
| ASTs and synchronization | `$DCLAST`, `$SETAST`, `$SYNCH` |
| Conditions and exit | `$EXIT`, `$DCLEXH`, `$CANEXH`, `$UNWIND`, `$SETEXV`, `$GETMSG`, `$PUTMSG`, `$FAO`, `$FAOL` |
| Processes | `$CREPRC`, `$DELPRC`, `$HIBER`, `$WAKE`, `$SUSPND`, `$RESUME`, `$FORCEX`, `$SETPRI`, `$SETPRN`, `$SETPRV`, `$SETRWM`, `$CMKRNL`, `$CMEXEC` |
| Information | `$GETJPI(W)`, `$GETSYI(W)`, `$GETDVI(W)`, `$GETLKI(W)`, `$ASCTOID`, `$IDTOASC` |
| Mailboxes | `$CREMBX`, `$DELMBX` |
| Memory | `$EXPREG`, `$CRETVA`, `$DELTVA`, `$CNTREG`, `$ADJSTK`, `$SETPRT`, `$ADJWSL`, `$PURGWS`, `$LKWSET`, `$ULWSET`, `$LCKPAG`, `$ULKPAG` |
| Sections | `$CRMPSC`, `$MGBLSC`, `$DGBLSC`, `$UPDSEC(W)` |
| Locks | `$ENQ`, `$ENQW`, `$DEQ` |
| Command language | `$CLI` |
| RMS | `$OPEN`, `$CREATE`, `$CLOSE`, `$CONNECT`, `$GET`, `$PUT`, `$FIND`, `$UPDATE`, `$DELETE`, `$TRUNCATE`, `$REWIND`, `$FLUSH`, `$FREE`, `$RELEASE`, `$WAIT`, `$ERASE`, `$RENAME`, `$PARSE`, `$SEARCH`, `$DISPLAY` |

Other VMS services aren't available yet.

### Appendix B: Run-time library routines

govax's run-time library has these routines, written in Go from
DIGITAL's *Run-Time Library* manuals and checked against VMS 7.1 by probe
programs. Every other entry point of LIBRTL exists too, so programs link
and run, but signals `GOVAX-E-NOTIMPL` if called.

| Area | Routines |
| --- | --- |
| Output and input | `LIB$PUT_OUTPUT`, `LIB$GET_INPUT`, `LIB$GET_COMMAND`, `LIB$GET_FOREIGN` |
| Strings and descriptors | `STR$UPCASE`, `LIB$SCOPY_DXDX`, `LIB$SCOPY_R_DX`, `LIB$SGET1_DD`, `LIB$SFREE1_DD`, `LIB$SFREEN_DD`, `LIB$ANALYZE_SDESC`, `LIB$LEN`, `LIB$CHAR`, `LIB$ICHAR`, `LIB$INDEX`, `LIB$LOCC`, `LIB$SKPC`, `LIB$SCANC`, `LIB$SPANC`, `LIB$MATCHC`, `LIB$MOVC3`, `LIB$MOVC5`, `LIB$MOVTC`, `LIB$MOVTUC`, `LIB$TRA_ASC_EBC`, `LIB$TRA_EBC_ASC`, `LIB$LOOKUP_KEY`, `LIB$CVT_DX_DX` |
| Conversion | `LIB$CVT_DTB`, `LIB$CVT_HTB`, `LIB$CVT_OTB` |
| Arithmetic and bits | `LIB$ADDX`, `LIB$SUBX`, `LIB$EMUL`, `LIB$EDIV`, `LIB$ADAWI`, `LIB$EMODF`/`D`/`G`/`H`, `LIB$POLYF`/`D`/`G`/`H`, `LIB$EXTV`, `LIB$EXTZV`, `LIB$INSV`, `LIB$FFC`, `LIB$FFS`, `LIB$BBSSI`, `LIB$BBCCI`, `LIB$CRC`, `LIB$CRC_TABLE` |
| Queues | `LIB$INSQHI`, `LIB$INSQTI`, `LIB$REMQHI`, `LIB$REMQTI` |
| Conditions | `LIB$SIGNAL`, `LIB$STOP`, `LIB$ESTABLISH`, `LIB$REVERT`, `LIB$SIG_TO_RET`, `LIB$MATCH_COND`, `LIB$CALLG` |
| Memory | `LIB$GET_VM`, `LIB$FREE_VM`, `LIB$GET_VM_PAGE`, `LIB$FREE_VM_PAGE`, `LIB$CREATE_VM_ZONE`, `LIB$CREATE_USER_VM_ZONE`, `LIB$DELETE_VM_ZONE`, `LIB$RESET_VM_ZONE`, `LIB$FIND_VM_ZONE`, `LIB$SHOW_VM_ZONE`, `LIB$VERIFY_VM_ZONE`, `LIB$STAT_VM`, `LIB$SHOW_VM`, `LIB$VM_MALLOC`, `LIB$VM_CALLOC`, `LIB$VM_REALLOC`, `LIB$VM_FREE` |
| Dates and times | `LIB$DATE_TIME`, `LIB$DAY`, `LIB$DAY_OF_WEEK`, `LIB$ADD_TIMES`, `LIB$SUB_TIMES`, `LIB$MULT_DELTA_TIME`, `LIB$MULTF_DELTA_TIME`, `LIB$CVT_TO_INTERNAL_TIME`, `LIB$CVT_FROM_INTERNAL_TIME`, `LIB$CVTF_TO_INTERNAL_TIME`, `LIB$CVTF_FROM_INTERNAL_TIME`, `LIB$CVT_VECTIM`, `LIB$SYS_ASCTIM`, `LIB$FORMAT_DATE_TIME`, `LIB$CONVERT_DATE_STRING`, `LIB$INIT_DATE_TIME_CONTEXT`, `LIB$FREE_DATE_TIME_CONTEXT`, `LIB$GET_DATE_FORMAT`, `LIB$GET_MAXIMUM_DATE_LENGTH` |
| System service jackets and items | `LIB$SYS_FAO`, `LIB$SYS_FAOL`, `LIB$SYS_GETMSG`, `LIB$SYS_TRNLOG`, `LIB$GETJPI`, `LIB$GETSYI`, `LIB$GETDVI` |
| Resources and timers | `LIB$GET_EF`, `LIB$FREE_EF`, `LIB$RESERVE_EF`, `LIB$GET_LUN`, `LIB$FREE_LUN`, `LIB$INIT_TIMER`, `LIB$SHOW_TIMER`, `LIB$STAT_TIMER`, `LIB$FREE_TIMER`, `LIB$WAIT` |
| The command language | `LIB$GET_SYMBOL`, `LIB$SET_SYMBOL`, `LIB$DELETE_SYMBOL`, `LIB$SET_LOGICAL`, `LIB$DELETE_LOGICAL`, `LIB$GET_COMMON`, `LIB$PUT_COMMON`, `LIB$DO_COMMAND`, `LIB$RUN_PROGRAM`, `LIB$DISABLE_CTRL`, `LIB$ENABLE_CTRL`, `LIB$PAUSE`, `LIB$ATTACH`, `LIB$SPAWN` |
| Files | `LIB$FIND_FILE`, `LIB$FIND_FILE_END`, `LIB$FILE_SCAN`, `LIB$FILE_SCAN_END`, `LIB$DELETE_FILE`, `LIB$RENAME_FILE`, `LIB$FID_TO_NAME`, `LIB$TRIM_FILESPEC`, `LIB$CREATE_DIR` |
| Other | `LIB$AST_IN_PROG`, `LIB$ASN_WTH_MBX`, `LIB$GET_USERS_LANGUAGE`, `LIB$GET_ACCNAM`, `LIB$FORMAT_SOGW_PROT`, `LIB$PARSE_SOGW_PROT`, `LIB$PARSE_ACCESS_CODE`, `LIB$CURRENCY`, `LIB$DIGIT_SEP`, `LIB$RADIX_POINT`, `LIB$LP_LINES` |

The tables `LIB$AB_ASC_EBC`, `LIB$AB_EBC_ASC`, and `LIB$AB_UPCASE` are
there too, and the JSB entry points of the string routines. govax also
provides part of the C run-time library (`DECC$`) for the microkernel's
use.

#### The mathematics library

MTHRTL's 404 names all resolve, so any program can call them, whatever it's
written in: every `MTH$` routine in the VMS manual in F, D, G, and H, the complex
functions in F, D, and G, `MTH$RANDOM`, the JSB entries, and the `OTS$` powers and
complex multiplication and division. Only `MTH$$REDUCE_`, internal to VMS's library,
signals `GOVAX-E-NOTIMPL`. An `MTH$` routine takes its arguments by reference, an
`OTS$` routine by value. An F result comes back in R0, a D or G one in R0 and R1,
and an H or a D or G complex one through a first argument (`OTS$`: R0 to R3):

```text
	.TITLE	ROOT
	.PSECT	DATA,WRT,NOEXE,LONG
TWO:	.FLOAT	2.0
BAD:	.FLOAT	-1.0
SEED:	.LONG	1
CTL:	.ASCID	/square root !XL, random !XL/
BUF:	.BLKB	80
OUT:	.LONG	80
	.ADDRESS BUF
LEN:	.BLKL	1
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<R2>
	PUSHAF	TWO
	CALLS	#1,G^MTH$SQRT		; the argument by reference, the result in R0
	MOVL	R0,R2
	PUSHAL	SEED
	CALLS	#1,G^MTH$RANDOM
	$FAO_S	CTRSTR=CTL,OUTLEN=LEN,OUTBUF=OUT,P1=R2,P2=R0
	MOVL	LEN,OUT
	PUSHAQ	OUT
	CALLS	#1,G^LIB$PUT_OUTPUT
	PUSHAF	BAD
	CALLS	#1,G^MTH$SQRT		; no handler: a severe condition ends the image
	RET
	.END	START
```

```text
square root 04F340B5, random 80003886
%MTH-F-SQUROONEG, square root of negative value
  user PC 00000461
%TRACE-F-TRACEBACK, symbolic stack dump follows
module name     routine name                     line       rel PC    abs PC

ROOT            START                                      00000061  00000461
```

Every format is computed in arbitrary precision with guard bits and rounded once, so
a result is the same on every host, and within a last bit or so of the true value. VMS
7.1's are often a last bit or two away from that, and more in complex logarithms and
powers, so a program comparing results exactly with a VAX's will see differences there.
What the routines return and signal otherwise was checked against VMS 7.1.

`MTH$RANDOM` follows the manual, `SEED = (69069 * SEED + 1)` modulo 2^32, with the new
seed's high 24 bits as the result, so a seed repeats its sequence. Setting
`vax.runtime.random` makes each new seed come from the host's cryptographic generator
instead, for numbers that can't be predicted or repeated.

A handler can return `SS$_CONTINUE` from one of the library's conditions, and may correct
the result in the mechanism array's R0 and R1, as the manual says. One difference: VMS's
routine goes on from there, and some then fault on the reserved operand they were given, or
store the corrected value in an H result. govax's return to the caller with what the signal
left.

### Appendix C: Where govax differs from VMS

The main differences, gathered from the rest of the manual:

- No VMS boot, executive, or compilers; MACRO-32 is the only language you
  can build in (section 1).
- One user, `SYSTEM`, with all privileges; no login, batch, print
  queues, networking, or clusters.
- A program's RMS calls reach only files on mounted Files-11 volumes,
  not host files (section 5).
- No rooted-directory logical names (`DEV:[ROOT.]`).
- govax's own system macro library lacks some of VMS's definition macros
  (`$DSCDEF`, for one); VMS's own library can be used instead (section 8).
- Some LIBRTL routines aren't implemented yet (Appendix B).
- A handler that continues from a condition of the mathematics library gets
  what the first signal left, where VMS's routine may go on, and signal again
  (Appendix B).
- With paging on: no swapping, no paging waits, a smaller minimum working
  set (section 13.5).
- BACKUP works with save sets of files only: no image, physical, or
  incremental operations, no tapes, and no copies of files to files. It
  reads a save set stored as 512-byte records, which VMS refuses.
- The console has commands VMS's DCL doesn't (`VMINIT`, `PRINT`,
  `DEBUG` without an image, ...), and lacks some VMS has (section 7).

`docs/DEVIATIONS.md` is the full, detailed list, including every known
difference in instruction behavior.

### Appendix D: Further reading

- `HELP` at the `VAX>` and `DBG>` prompts: every command and qualifier.
- `docs/PLAN.md`: the project's plan and the list of development phases.
- `docs/PHASE-nn - topic.md`: a design document for each phase, with its
  decisions, the VMS behavior it reproduces, and the probe programs used
  to check it against VMS.
- `docs/DEVIATIONS.md`: known differences from the VAX architecture and
  from VMS.
- `docs/PERFORMANCE.md`: how govax was profiled and made faster.
- `docs/MODE-STACKS.md`, `docs/DISK-DEVICES.md`, `docs/RMS-MACROS.md`,
  `docs/DEBUG-RECORDS.md`: notes on particular subsystems.
- DIGITAL's manuals: the *VAX Architecture Reference Manual*, the *VAX
  MACRO and Instruction Set Reference Manual*, the *System Services
  Reference Manual*, the *Run-Time Library* manuals, the *Guide to VMS
  File Applications*, the *DCL Dictionary*, the *Linker* and *Librarian*
  manuals, and the *Debugger Manual*.
- Kenah and Bate, *VAX/VMS Internals and Data Structures*: how VMS works
  inside, and the source of govax's scheduling and paging rules.
