# govax

**A VAX/VMS programming environment in a single executable.**

If you learned computing in the '70s or '80s, there's a fair chance you
learned it at a VT100 wired to a VAX. You'll remember the `$` prompt,
version numbers on your files, typing `PURGE` because you were over quota,
green-bar listings of your MACRO-32 program, and the first time you saw
`%SYSTEM-F-ACCVIO` and knew it was your fault.

govax brings that world back, on your own computer. It executes real VAX
machine code and gives it the system services, run-time library, and files
it expects, behind a DCL prompt, with an assembler, a FORTRAN compiler, a
linker, and a debugger ready to go. It runs on macOS, Linux, and Windows.
**You don't need a VMS license, a VMS distribution, or a single file from
DIGITAL.**

A VAX-11/780 executed about half a million instructions a second, and a
whole department shared it. On a current laptop govax executes 10 to 20
million, all of them yours. You're logged in as `SYSTEM`, with every
privilege, and nobody from operations is going to call.

## Hello, VAX

You'll need [Go](https://go.dev) 1.26 or later:

```sh
git clone https://github.com/tucats/govax.git
cd govax
go install ./cmd/govax
```

That's the whole installation. The first time it runs, govax makes itself
a system disk in `~/.govax`.

Now put this in a file called `hello.for`, with the statements starting in
column 7, as nature intended:

```fortran
      PROGRAM HELLO
      PRINT *, 'Hello from a VAX that isn''t there'
      END
```

Compile, assemble, link, and run it:

```text
$ govax fortran hello.for
$ govax macro hello.mar
$ govax link hello.obj
$ govax run hello.exe
Hello from a VAX that isn't there
```

Then have a look at `hello.mar`. The compiler writes ordinary MACRO-32,
with each FORTRAN statement above the instructions it became, so it's a
good way to see how a VAX gets things done.

Or skip the compiler and write the assembler yourself. The commands are
the same, starting from `govax macro`:

```asm
        .TITLE  HELLO   Hello, world
        .PSECT  DATA, WRT, NOEXE, LONG
MSG:    .ASCID  /Hello from a VAX that isn't there/
        .PSECT  CODE, NOWRT, EXE, LONG
        .ENTRY  HELLO, ^M<>
        PUSHAQ  MSG                     ; a string descriptor, by reference
        CALLS   #1, G^LIB$PUT_OUTPUT
        RET                             ; R0 is its status
        .END    HELLO
```

## Run VAX/VMS programs

govax runs VMS executable images, including ones linked on a real VAX. Here
is one from this repository:

```text
$ govax run testdata/exe/put.exe
This is a test
that I am performing.
```

The VAX instruction set is complete, down to the H-floating and packed
decimal instructions. Behind it are over ninety system services, about 170
`LIB$` routines, the whole `MTH$` library, several processes running at
once, mailboxes, global sections, the lock manager, and, if you'd like to
watch it work, VMS's paging.

Your old programs may well run. An image needs only the services and
routines it calls, and if one is missing, govax tells you its name.
Programs built by DEC's compilers also need their language's run-time
library from a VMS system.

And what's the point of all this if it can't play DUNGEON? Bring your own
copy of the source kit for the FORTRAN port of the game that became Zork:
govax's compiler builds it, and it plays.

## Take it apart

The best part of having a VAX all to yourself is being able to poke at it.
Give govax a disk to work on, then start it up. The disk is a Files-11
volume kept in a single file on your computer:

```sh
govax config set vax.default.volume.file="~/vax/work.dsk"
govax
```

Take this program:

```fortran
      PROGRAM FACT
C     Ten factorials, and a good place for a breakpoint.
      INTEGER I, N
      N = 1
      DO 10 I = 1, 10
         N = N * I
         WRITE (6, 100) I, N
10    CONTINUE
100   FORMAT (' ', I2, '! = ', I8)
      END
```

Copy it to the VAX disk, build it for the debugger, and stop it mid-loop:

```text
VAX> COPY "fact.for"/HOST *
%COPY-S-COPIED, fact.for copied to DUA1:[WORK]FACT.FOR;1
VAX> FORTRAN/DEBUG/NOOPTIMIZE FACT
VAX> MACRO/DEBUG FACT
VAX> LINK/DEBUG FACT
VAX> RUN FACT

         GOVAX DEBUG

%DEBUG-I-INITIAL, Language: MACRO, Module: FACT
DBG> SET BREAK FACT.$LINE7
DBG> GO
break at FACT\FACT.$LINE7
FORTRAN line 7: WRITE (6, 100) I, N
    71:         PUSHAB  FACT.$F100
DBG> GO
 1! =        1
break at FACT\FACT.$LINE7
...
DBG> EXAMINE I, N
FACT\FACT.I:    00000003
FACT\FACT.N:    00000006
```

The usual suspects are all here:

| | |
| --- | --- |
| **DCL** | A full command interpreter: command procedures, symbols, lexical functions, `ON`, `GOSUB`, `SPAWN`, file I/O, and `HELP` |
| **MACRO-32** | The assembler and macro facility, with its own system macro library, so `$QIOW_S` and `$FAB` just work |
| **FORTRAN** | VAX FORTRAN V5.0 and DEC's extensions: records, `NAMELIST`, keyed files, an optimizer, listings |
| **LINK and LIBRARY** | Executable and shareable images, and object, macro, text, and help libraries |
| **The debugger** | Modeled on the VMS debugger: breakpoints, stepping, `EXAMINE`, source lines, by symbol |
| **RMS on Files-11** | Sequential, relative, and indexed files on ODS-2 disk containers, laid out as VMS lays them out |
| **BACKUP** | Reads, restores, compares, and writes real VMS save sets |

None of these is a loose imitation. The objects, images, libraries, and
save sets govax writes are built the way VMS's own tools build them, and
VMS reads them. Its behavior has been checked against VMS 7.1 and 7.3
themselves, and the test suite replays those runs to keep it honest.

## What govax isn't

govax is not a full simulated VAX/VMS system. It doesn't emulate disk
controllers or a console boot ROM, and it can't boot VMS. Its operating
system is a small microkernel plus Go code that behaves, from a program's
point of view, as VMS does. It's a single-user system with no batch queues,
no DECnet or TCP/IP, and no clusters, and FORTRAN and MACRO-32 are the only
languages you can build with.

If you want the real operating system, booted on a simulated VAX, Bob
Supnik's [SIMH](https://simh.trailing-edge.com) is still the gold standard.
(Yes, the same Bob Supnik who wrote the FORTRAN DUNGEON.) The two get along
fine: SIMH can mount the disks govax makes.

## Go deeper

The [user's manual](docs/MANUAL.md) covers it all: the command line, every
DCL command, MACRO-32, FORTRAN, the linker, the debugger, and how govax
works inside. Its examples were run on govax. At the `VAX>` prompt, `HELP`
works the way you remember, and if you've ever wanted to write a compiler
for the VAX, [FORTRAN-INTERNALS](docs/FORTRAN-INTERNALS.md) follows one
through every pass.

## Tell us what you find

This project lives on bug reports and on people's memories of how VMS
actually behaved. Please
[open an issue](https://github.com/tucats/govax/issues) if a program won't
run, a routine you need is missing, a message is worded wrong, or you just
find yourself muttering "that's not how VMS did it." Feedback, ideas, and
stories from the machine room are all welcome.

---

govax is [MIT licensed](LICENSE) and contains no DIGITAL, Compaq, HP, or
VSI code or files.
