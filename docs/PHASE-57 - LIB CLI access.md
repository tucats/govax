# Phase 57 — LIB$ access to the command language interpreter

**Status:** in progress (2026-10-13; planned 2026-10-12). Needs Phases
53–55, and Phases 48, 50, and 51 (DCL in every process). Part of the LIB$ program laid out in
`PHASE-53 - RTL LIB support.md` (Part A).

## Goal

The routines of the 5.0 manual's section 2.2: a program working with
the DCL that ran it. Symbols and supervisor-mode logical names; the
command line (GET_COMMAND, with GET_FOREIGN already done); chaining,
where DO_COMMAND and RUN_PROGRAM end the image and have DCL run a command
or another image; the CLI's common area (GET_COMMON, PUT_COMMON), which
keeps text from one image to the next; control-key interception
(DISABLE_CTRL, ENABLE_CTRL); and moving the terminal between processes
(ATTACH) or back to DCL for a moment (PAUSE).

Every one of these runs in a process whose DCL is govax's own: process
1's, or a subprocess's CLI (`internal/dclshell/subcli.go`). With no CLI
(a `$CREPRC` image), they return LIB$_NOCLI as the manual says. This
phase is mostly about the boundary between `internal/librtl` and
`internal/dclshell`. On VMS these routines go through the CLI callback
($CLI); govax needs an interface `corevms` can call that each process's
DCL implements, as `Machine.Hooks` does for the machine.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Symbols | GET_SYMBOL (219), SET_SYMBOL (343), DELETE_SYMBOL (116) |
| Logical names | SET_LOGICAL (340), DELETE_LOGICAL (114) |
| Command line | GET_COMMAND (199) |
| Chaining | DO_COMMAND (124), RUN_PROGRAM (332) |
| Common area | GET_COMMON (202), PUT_COMMON (309) |
| Control keys | DISABLE_CTRL (122), ENABLE_CTRL (138) |
| Terminal | ATTACH (17), PAUSE (299) |

## What earlier phases leave in place

- DCL's symbol tables (`dclshell/dclsym.go`: global and per-level local),
  and how a subprocess gets its parent's symbols (LIB$SPAWN, Phase 48).
- Logical names, supervisor mode, the process table (`internal/lnm`).
- LIB$GET_FOREIGN's `Environment.CommandLine`, LIB$GET_INPUT's terminal
  reads, and SYS$COMMAND (`dclshell/dclinput.go`).
- `$CLI` (`corevms/cli.go`), SET [NO]CONTROL and Ctrl/Y and Ctrl/C
  handling (`dclshell/dclcontroly.go`, Phase 50, subtask 18).
- SPAWN, ATTACH-style terminal handoff (`corevms.Environment.Attached`,
  Phase 51).

## Open questions

- ~~**The CLI interface**~~ (settled by the survey, 2026-10-13): a
  `corevms.CLI` interface (symbols, chaining, control keys, PAUSE,
  ATTACH) that dclshell sets on each process with a CLI, replacing
  `HasCLI`. Logical names go to `lnm` directly, and the common area is
  the process's, not the CLI's. See "The CLI interface".
- **Chaining**: the image exits (its exit handlers run), then DCL runs
  the command as if typed, in the procedure level that ran the image.
  DCL's `$STATUS` and verify echo are what the probe should check
  (the survey's "Chaining" lists the cases).
- ~~**ATTACH and PAUSE (author)**~~ (decided 2026-10-13): ATTACH in
  full, DCL's command included, over an explicit terminal holder per
  job; PAUSE reads the Ctrl/Y level wherever it's called, level 0
  included (the survey's "ATTACH" and "PAUSE").
- ~~**The common area (author)**~~ (decided 2026-10-13): 256 bytes (a
  longword length and 252 characters), kept in Go on the process until
  the probe shows where VMS keeps it (SYS$GL_COMMON).
- ~~**SET CONTROL per process (author)**~~ (decided 2026-10-13): the
  settings move from the `Interpreter` to each process's `Shell`
  (the survey's "Control keys").
- **`$CLI`** stays as it is: no manual at hand describes its request
  block, so the LIB$ routines don't go through it (the survey's
  "Sources").

## The survey (subtask 1)

Each routine's interface, from the 5.0 LIB$ manual, in the notation of
Phase 53's survey (`name.access-type.mechanism`; `[ ]` optional). Page
numbers are the manual's LIB-nnn where the entry starts. Section 2.2
(pages 2-2 to 2-9) adds what the entries share. Choices the manuals
leave open are marked **unconfirmed**, and each is a case for the probe
(subtask 10).

All 14 routines are in VMS 7.3's LIBRTL.EXE transfer vector
(`vmsdef.ImageSymbols`: ATTACH 0x318, DELETE_LOGICAL 0x320,
DELETE_SYMBOL 0x328, DISABLE_CTRL 0x330, ENABLE_CTRL 0x338, GET_SYMBOL
0x340, SET_LOGICAL 0x348, SET_SYMBOL 0x350, GET_COMMAND 0x408,
GET_COMMON 0x820, PUT_COMMON 0x828, DO_COMMAND 0x840, RUN_PROGRAM 0x8E8,
PAUSE 0xA68). Today each is a missing-routine stub. The section's other
two routines, GET_FOREIGN and SPAWN, are done (Phases 34 and 48).

### Sources

- **The 5.0 LIB$ manual** (`AA-LA76A-TE`, its OCR text) is the
  reference. The OCR's errors that matter here: "CU" and "Clls" for CLI
  and CLIs, "UB$" and "U8$" and "Ll8$" for LIB$, "SVMBOL" for SYMBOL,
  "DQ_CQMMAND" for DO_COMMAND, `1` for `I` in condition names
  (`LIB$_1NVARG`), and "LIB$-1NVARG" in SET_LOGICAL.
- **The VSI OpenVMS RTL Library (LIB$) Manual** (local-only, usable like
  DIGITAL's manuals). Its entries agree with the 5.0 text except where
  marked **VSI** below; each difference is a probe case, since it may
  postdate VAX VMS 7.3. It drops the MCR CLI throughout.
- **The OpenVMS 7.3-1 DCL Dictionary** (local text): ATTACH, SPAWN, SET
  CONTROL, CONTINUE, which these routines are defined by ("provides the
  same function as the DCL command ...").
- **No manual at hand describes `$CLI`'s request block.** On VMS these
  routines are callers of the `$CLI` service (the "CLI callback"), but
  its request codes and layout aren't in the 5.0 System Services manual
  or the VSI pages. So govax's routines reach DCL through a Go interface
  instead (below), and `corevms/cli.go`'s `$CLI` (a stub kept from the
  port: "get symbol" answers CLI$_UNDSYM, anything else halts) is left
  as it is. A program calling `$CLI` itself is out of scope.

### Codes

Every value is in `vmsdef.LibrarySymbols` (STARLET.OLB's definitions):

| Name | Value | Use |
| --- | --- | --- |
| LIB$K_CLI_LOCAL_SYM | 1 | table-type-indicator: local symbol table |
| LIB$K_CLI_GLOBAL_SYM | 2 | table-type-indicator: global symbol table |
| LIB$M_CLI_CTRLT | %X00100000 (bit 20) | Ctrl/T |
| LIB$M_CLI_CTRLY | %X02000000 (bit 25) | Ctrl/Y |
| LIB$_NOSUCHSYM | %X158364 | no such symbol |
| LIB$_INSCLIMEM | %X15836C | insufficient CLI memory |
| LIB$_AMBSYMDEF | %X158374 | ambiguous symbol definition |
| LIB$_NOCLI | %X15837C | no CLI present to perform function |
| LIB$_UNECLIERR | %X158384 | unexpected CLI error |
| LIB$_INVSYMNAM | %X15838C | invalid symbol name |
| LIB$_ATTREQREF | %X158394 | attach request refused |
| LIB$_INPSTRTRU | %X15821C | input string truncated |

The mask bits are the control characters' codes (Ctrl/T is 20, Ctrl/Y
25). The manuals name the macro `$LIBCLIDEF`. govax's macro library
(`starletdef.mar`) doesn't have it yet; it is generated from real
MACRO's output, so the probe records its symbols (below) and the probe
program defines the four values itself until then.

LIB$_INSCLIMEM, LIB$_UNECLIERR, LIB$_FATERRLIB, and LIB$_INSVIRMEM
are listed by most entries; govax's DCL has no limit that would give the
first, and no foreign CLI to give the second, so govax never returns
them (as Phase 55 did for FATERRLIB).

### General rules

- **Which processes have a CLI.** Process 1 (the console's DCL), and a
  process LIB$SPAWN or DCL's SPAWN created, or `$CREPRC` created to run
  LOGINOUT (the subprocess CLI, `dclshell/subcli.go`): today's
  `Environment.HasCLI`. A process `$CREPRC` created to run an image has
  none, and every routine that needs one returns LIB$_NOCLI there.
- **Which routines need one.** Eleven: the symbols, the logical names,
  the chaining pair, the control keys, ATTACH, and PAUSE. GET_COMMAND,
  GET_COMMON, and PUT_COMMON don't list LIB$_NOCLI: the first is an RMS
  read of SYS$COMMAND, and the common area is process storage, so they
  work without a CLI (**unconfirmed** for the common area; the probe's
  CLI-less process tries them).
- **Order of checks.** Whether LIB$_NOCLI comes before an argument's
  error (a bad descriptor, an invalid table indicator) isn't said.
  govax checks for the CLI first (**unconfirmed**).
- **Arguments.** None of the entries lists LIB$_WRONUMARG. An omitted
  optional argument is read as 0 (`arg`), as elsewhere in govax's
  LIBRTL. A required argument that's omitted is a probe case for
  GET_SYMBOL and SET_SYMBOL.
- **Success** is SS$_NORMAL (1) everywhere, but SET_LOGICAL's
  SS$_SUPERSEDE and SS$_BUFFEROVF and the strings' LIB$_STRTRU.
- **Strings** are read and written by `descriptor.go`'s rules (Phase
  53); a bad class is LIB$_INVSTRDES (GET_COMMAND's LIB$_INVARG for its
  output, as GET_INPUT's).

### Symbols (section 2.2.4)

| Routine (page) | Format |
| --- | --- |
| GET_SYMBOL (219) | `symbol.rt.dx, resultant-string.wt.dx [,resultant-length.ww.r] [,table-type-indicator.wl.r]` |
| SET_SYMBOL (343) | `symbol.rt.dx, value-string.rt.dx [,table-type-indicator.rl.r]` |
| DELETE_SYMBOL (116) | `symbol.rt.dx [,table-type-indicator.rl.r]` |

- **The name** is uppercased and its trailing blanks removed before use,
  at most 255 characters. What it may start with differs by entry and
  manual:
  - GET_SYMBOL: a letter or `$` (5.0); a letter, digit, `$`, `-`, or
    `_` (**VSI**).
  - SET_SYMBOL: a letter, `_`, or `$` (5.0); the VSI entry's condition
    says "did not begin with a letter".
  - DELETE_SYMBOL: a letter, `$`, or `_` (5.0); as GET_SYMBOL (**VSI**).

  Anything else, or a name longer than 255, is LIB$_INVSYMNAM. The probe
  tries `_X`, `$X`, `1X`, `-X`, a blank name, and 255 and 256
  characters. SET_SYMBOL's warning that "symbols created using
  LIB$SET_SYMBOL may be inaccessible by other CLI commands" suggests DCL
  takes names it can't parse; whether an `*` in the name makes an
  abbreviation (as DCL's `DIR*ECTORY :==` does) or is just a character
  is a probe case.
- **The tables.** SET_SYMBOL and DELETE_SYMBOL use the local table when
  the indicator is omitted; a value other than 1 or 2 is LIB$_INVARG
  (listed for DELETE_SYMBOL, and by the VSI entry for SET_SYMBOL).
  "Local" is the local table of the command level that ran the image:
  a symbol SET_SYMBOL defines in an image run from a procedure goes when
  the procedure ends. GET_SYMBOL "first searches the local symbol table
  ..., then searches the global symbol table", and writes 1 or 2 to the
  indicator. Whether it also looks in the outer levels' local tables, as
  DCL's own lookup does (`dclSymbolTable.searchOrder`), is a probe case,
  as is whether it takes an abbreviation (`DIR` for `DIR*ECTORY`).
- **Values.** GET_SYMBOL returns an integer symbol "converted to an
  ASCII representation of a signed decimal number" (negative numbers;
  the probe sets one with `=` in the procedure). A value too long for a
  fixed output is LIB$_STRTRU, a success, with resultant-length the
  stored length. SET_SYMBOL always makes a string symbol ("Integer
  values are not allowed"), keeping trailing blanks. Its value is at
  most 255 characters (5.0), or 4096 (**VSI**, LIB$_INVARG past it); the
  probe tries 255, 256, 1024, and 4097 and looks at what DCL then has.
- **LIB$_AMBSYMDEF**: SET_SYMBOL of a name that is "ambiguous when
  compared with existing symbol names", from abbreviations (`AB*CDEF`
  defined, then `ABC`?). DCL's own assignment has no such error in
  govax today; the probe settles when VMS gives it.
- **LIB$_NOSUCHSYM** from GET_SYMBOL, and from DELETE_SYMBOL of a name
  not in the table it names (listed by the 5.0 entry's status list, not
  its description).
- **DCL's own symbols.** `$STATUS` and `$SEVERITY` during the image (the
  status of the RUN command, or of the command before it?), SET_SYMBOL
  and DELETE_SYMBOL of them, and P1 in a procedure, are probe cases.

In govax the tables are DCL's (`dclshell/dclsym.go`'s
`dclSymbolTable`), one per process's `Shell`; DELETE/SYMBOL and the
assignments already do the work, so the routines are small callers of
it.

### Logical names (section 2.2.4)

| Routine (page) | Format |
| --- | --- |
| SET_LOGICAL (340) | `logical-name.rt.dx [,value-string.rt.dx] [,table.rt.dx] [,attributes.rlu.r] [,item-list.rz.ra]` |
| DELETE_LOGICAL (114) | `logical-name.rt.dx [,table-name.rt.dx]` |

- Both work in **supervisor mode** whatever the caller's mode ("does not
  require the caller to be executing in supervisor mode"), and default
  the table to LNM$PROCESS. Supervisor-mode names outlive the image.
  Logical names are case-sensitive (**VSI**; DCL's DEFINE uppercases,
  these don't, **unconfirmed** for 7.x).
- **SET_LOGICAL** needs value-string or item-list (neither: LIB$_INVARG);
  with both, value-string is ignored. The attributes are LNM$M_CONFINE
  and LNM$M_NO_ALIAS for the name, and, without an item list,
  LNM$M_CONCEALED and LNM$M_TERMINAL for the one value. The 5.0 manual
  calls the item list `item_list_2`, the VSI manual `item_list_3`, "item
  codes ... in module $LNMDEF": `$CRELNM`'s list (LNM$_STRING,
  LNM$_ATTRIBUTES, ...), which govax's `$CRELNM` already reads
  (`corevms/logicals.go`). govax reads it as `$CRELNM` does
  (**unconfirmed** which form VMS 7.x takes; the probe gives a
  three-longword list). Its statuses are `$CRELNM`'s (SS$_SUPERSEDE,
  SS$_IVLOGNAM, SS$_IVLOGTAB, SS$_NOPRIV, ...) plus LIB$_INVARG and
  LIB$_INVSTRDES.
- **DELETE_LOGICAL** deletes the supervisor-mode name only:
  SS$_NOLOGNAM if there's none at that mode (a user-mode name of the
  same name is a probe case).
- Neither has DEFINE's and DEASSIGN's side effect of opening or closing
  a process-permanent file for SYS$OUTPUT (**VSI**).
- In govax they need no help from DCL: `lnm.Database.Define` and
  `Delete` at `lnm.Supervisor` on the process's `Logicals`, with the
  table resolved as `$CRELNM` resolves one. The CLI is only checked for
  (LIB$_NOCLI).

### GET_COMMAND (199)

`resultant-string.wt.dx [,prompt-string.rt.dx] [,resultant-length.ww.r]`

LIB$GET_INPUT's twin, reading SYS$COMMAND: one record, the prompt shown
only when SYS$COMMAND is a terminal, LIB$_INPSTRTRU for a fixed output
too short (an error, with resultant-length the stored length), RMS$_EOF
at the end of the input, LIB$_INVARG for an output descriptor whose
class is 0 or unknown. It "opens file SYS$COMMAND on the first call",
and is "used when a program needs input from some source other than
the current input stream": in a command procedure, SYS$INPUT is the
procedure and SYS$COMMAND the terminal.

In govax, process 1's and a subprocess's terminal reads already split
the two (`corevms.Environment.ReadCommandLine`, the terminal even while
a procedure's data is SYS$INPUT), so GET_COMMAND is GET_INPUT's code
reading through it. A subprocess whose SYS$INPUT is a file or mailbox
(SPAWN/INPUT) reads its SYS$COMMAND, which govax defines as its input
(**unconfirmed**: VMS's SYS$COMMAND for such a subprocess).

### The common area (GET_COMMON 202, PUT_COMMON 309)

| Routine (page) | Format |
| --- | --- |
| GET_COMMON (202) | `resultant-string.wt.dx [,resultant-length.ww.r]` |
| PUT_COMMON (309) | `source-string.rt.dx [,resultant-length.ww.r]` |

- An area of "storage which remains defined across multiple image
  activations in a process", unique to each process. "The string length
  is taken from the first longword of the common area", and at most 252
  characters are kept: the area is 256 bytes, a longword length and 252
  bytes of text. PUT_COMMON of a longer string keeps 252 and returns
  LIB$_STRTRU; GET_COMMON into a shorter fixed string returns LIB$_STRTRU.
  resultant-length is the number of characters copied.
- PUT_COMMON's description gives the limit as "min(256, the length of
  the data in the common storage area) - 4", which reads as GET's rule;
  the probe tries PUT and GET of 0, 252, and 253 characters.
- Not listed: LIB$_NOCLI. So no CLI is needed (above).
- **Where VMS keeps it.** The P1 vector has a data cell, SYS$GL_COMMON
  at %X7FFEE114 (`vmsdef.P1VectorTable`), which may be the area's
  address or the area's start. In govax's P1 vector that address is
  inside SYS$CLRAST_2's trampoline (%X7FFEE110), so a pointer stored
  there would break it. The probe prints the longword at SYS$GL_COMMON
  and, if it is a P1 address, the area's 256 bytes after a PUT_COMMON.
- **Decided** (the author agreed, 2026-10-13): govax keeps the area in
  Go, on the process (`corevms.Environment`), where image rundown
  leaves it. If the probe shows SYS$GL_COMMON pointing to a P1 area, a
  later subtask moves it there (and SYS$CLRAST_2's trampoline out of
  the way).
- What the area holds when a process starts (zero length?), and whether
  a subprocess gets a copy of its parent's, are probe cases.

### Chaining (DO_COMMAND 124, RUN_PROGRAM 332; section 2.2.2, 2.2.3)

| Routine (page) | Format |
| --- | --- |
| DO_COMMAND (124) | `command-string.rt.dx` |
| RUN_PROGRAM (332) | `program-name.rt.dx` |

- Both end the image, and DCL then runs the command, or the program,
  "parsed using normal DCL rules". On success they don't return.
  RUN_PROGRAM: "the `$EXIT` system service is called, the new program
  image replaces the old image ..., and the command interpreter gives
  control to the new image"; "if unsuccessful, control returns to the
  command interpreter". So the image's exit handlers run (`$EXIT`), and
  a program that can't be activated is an error DCL shows at its
  prompt, as RUN's would be. The program's default type is .EXE.
- Text or name longer than 255 characters is LIB$_INVARG, the only error
  besides LIB$_NOCLI and LIB$_UNECLIERR.
- DO_COMMAND can run a procedure (`@file`). A command that isn't
  complete prompts for the rest ("$_Show what?:", the 5.0 manual's
  section 2.2.3).
- What the manuals don't say, all probe cases:
  - which status the image exits with, and `$STATUS` after the command
    (the command's own, in the cases tried);
  - the level the command runs at (the procedure that ran the image),
    and whether SET VERIFY echoes it;
  - an exit handler's output, before the command's;
  - a chained image that chains again, DO_COMMAND of an empty string,
    and of a command that fails;
  - RUN_PROGRAM of a program that isn't there (the message, and
    `$STATUS`), and what the new image's LIB$GET_FOREIGN returns
    (expected: nothing).
- In govax, the routine leaves the command with the process's DCL
  (the CLI interface's `Chain`) and exits the image as `$EXIT` does;
  the DCL command that ran the image (RUN, a foreign command, MCR),
  once the image has run down, dispatches it at its own level as a line
  read there. An image run under the debugger ends its session when it
  exits; its chained command runs when control returns to DCL
  (govax's choice: VMS has no such case to compare).

### Control keys (DISABLE_CTRL 122, ENABLE_CTRL 138; section 2.2.5)

| Routine (page) | Format |
| --- | --- |
| DISABLE_CTRL (122) | `disable-mask.rlu.r [,old-mask.wlu.r]` |
| ENABLE_CTRL (138) | `enable-mask.rlu.r [,old-mask.wlu.r]` |

- The same as SET NOCONTROL=(T,Y) and SET CONTROL=(T,Y): bits 20 and 25
  of the mask. A set bit for a character the CLI can't intercept is
  LIB$_INVARG; whether the valid bits are acted on anyway is a probe
  case, as are a 0 mask and both bits at once.
- **old-mask** is "of the same form" as the mask. The VSI manual adds,
  for DISABLE_CTRL, that it "indicates those control characters that
  were previously enabled. It may therefore be given to LIB$ENABLE_CTRL
  to reinstate the previous condition": so both routines write the
  previously *enabled* keys (**unconfirmed**; the probe passes each
  combination).
- The setting is DCL's, so it outlasts the image: F$ENVIRONMENT("CONTROL")
  after the image shows it. After DISABLE_CTRL of Y, "CTRL/Y is treated
  like CTRL/U followed by a carriage return". Whether it also cancels
  the level's ON CONTROL_Y, as SET NOCONTROL=Y does (`setControlCommand`),
  is a probe case.
- govax shows nothing for Ctrl/T, but keeps its setting, as SET CONTROL
  does.
- **One setting per process.** govax keeps SET CONTROL's settings on
  the `Interpreter` (`controlKeys`), shared by every process's DCL. On
  VMS each process's DCL has its own. Since a subprocess's image calling
  DISABLE_CTRL should change its own DCL's setting, the settings move to
  each process's `Shell`, and `ControlYEnabled` answers for the process
  the terminal is attached to (**decided**, the author agreed,
  2026-10-13; the probe checks a subprocess's SET NOCONTROL against its
  parent's).

### ATTACH (17; section 2.2.6)

`process-id.rlu.r`

- "Detach the terminal of the calling process and ... reattach it to a
  different process. The calling process then hibernates", as DCL's
  ATTACH. The process "must be currently detached (by means of a SPAWN
  or ATTACH command, or by a call to LIB$SPAWN or LIB$ATTACH) and must be
  part of the caller's job". The DCL Dictionary's ATTACH: it must
  "share the same input stream as your current process", and can't be
  the current process or a subprocess made with /NOWAIT. Its own
  messages: %DCL-S-ATTACHED (`CLI$_ATTACHED`) and, in the process that
  gets the terminal back, %DCL-S-RETURNED (`CLI$_RETURNED`), which
  govax's SPAWN already shows (`dclshell/spawncmd.go`).
- SS$_NONEXPR for a PID that names no process; LIB$_ATTREQREF for one
  that isn't detached or isn't in the job. Attaching to oneself is a
  probe case.
- When the call returns: when the terminal comes back to the caller
  (another process ATTACHes to it, or the process it attached to logs
  out?), with SS$_NORMAL. Which process gets the terminal when the
  attached one logs out is a probe case: SPAWN's entry says the parent
  "remains in hibernation until the subprocess terminates or returns
  control to the parent by way of the ATTACH command".
- **govax today.** The terminal's holder is found by following SPAWN
  waits down from process 1 (`corevms.Environment.Attached`: a process
  waiting for a subprocess it spawned has given it the terminal). There
  is no hibernating "detached" process and no DCL ATTACH. ATTACH needs
  the holder made explicit: each job's terminal held by one process,
  given to a subprocess by a waiting SPAWN, given anywhere in the job by
  ATTACH, and returned to the parent when the holder is deleted; a
  process that gave it away by ATTACH waits until it comes back (MWAIT
  or HIB in the scheduler). The Ctrl/Y routing (Phase 51) already asks
  `Attached` and keeps working.
- **Decided** (the author agreed, 2026-10-13): the whole of it in this
  phase: the explicit holder, DCL's ATTACH command (a process name, or
  /IDENTIFICATION=pid), and LIB$ATTACH over the same code.

### PAUSE (299)

No arguments.

- "Suspends program execution and returns control to the calling
  command level. The suspended image may be continued with the
  CONTINUE command, or it may be terminated with the EXIT or STOP
  commands. In the latter case, the image will not return to this
  routine." Batch mode: "it has no effect" (SS$_NORMAL). LIB$_NOCLI
  without DCL ("DCL only supports this function in INTERACTIVE mode").
- This is DCL's Ctrl/Y, asked for by the program. govax has that state
  already inside a procedure (the Ctrl/Y level, `controlYLevel`: the
  image paused beneath DCL, which reads the terminal; CONTINUE resumes;
  EXIT or STOP, or a command that runs an image or procedure, runs the
  image down). It doesn't have it at command level 0, where govax's
  Ctrl/Y ends govax and Ctrl/C stops the machine.
- **Decided** (the author agreed, 2026-10-13): PAUSE reads the Ctrl/Y
  level wherever it's called, level 0 included, with DCL's ordinary prompt: CONTINUE returns
  SS$_NORMAL to the program, EXIT and STOP run the image down (no exit
  handlers, as for a Ctrl/Y), and a command that would run an image, a
  procedure, or a SPAWN runs the image down first, then runs (the Ctrl/Y
  level's rule, `errLeaveControlY`; VMS lets SPAWN run with the image
  paused, the DCL Dictionary's SPAWN example, which govax can't while
  process 1 is inside a shim). In a subprocess, the shim hands the
  interpreter the CPU as the Ctrl/Y AST does (`resumeControlY`). A CLI
  reading a file or mailbox (LOGINOUT's) is batch: no effect.
- What VMS shows when PAUSE is called (an "Interrupt" line, as for
  Ctrl/Y, or nothing), and what a procedure's PAUSE reads (the terminal,
  as at the Ctrl/Y level, or the procedure's next line), are probe
  cases. Those need an interactive run (below).

### The CLI interface

What corevms and librtl need of a process's DCL, as Go:

```go
// CLI is a process's command language interpreter, as the LIB$ CLI
// routines reach it (docs/PHASE-57). dclshell gives each process whose
// DCL is govax's one (Environment.CLI); nil means none (LIB$_NOCLI).
type CLI interface {
    // Symbol looks name up (local tables, then global), returning its
    // value as text and the table it was in (1 local, 2 global).
    Symbol(name string) (value string, table int, err error)
    SetSymbol(name, value string, table int) error
    DeleteSymbol(name string, table int) error

    // Chain leaves a command (or, with program, an image to run) for
    // DCL to run once the image now running has exited.
    Chain(text string, program bool)

    // ControlKeys enables and disables Ctrl/T and Ctrl/Y (LIB$M_CLI_
    // masks), returning the keys enabled before.
    ControlKeys(enable, disable uint32) (old uint32)

    // Pause reads the Ctrl/Y level with the image paused; Attach gives
    // the terminal to target and waits for it to come back. Both may
    // return ErrWait, as a waiting service does.
    Pause() (uint32, error)
    Attach(target *Environment) (uint32, error)
}
```

- It replaces `Environment.HasCLI`: `env.CLI != nil` (LIB$SPAWN's test
  too). dclshell sets it on process 1 (`newProcessOne`) and on each
  subprocess (`cliHost.Start`), bound to that process's own `Shell`, not
  to the interpreter's current one: a subprocess's image calls it on
  the engine's goroutine while its interpreter's goroutine waits, by the
  strict handoff `subcli.go` keeps, so no lock is needed.
- The errors are vmserrors with LIB$ values (LIB$_NOSUCHSYM,
  LIB$_AMBSYMDEF, ...), turned into the routine's status by the caller.
- Logical names, GET_COMMAND, and the common area don't go through it
  (above).

### For the probe (subtask 10)

The probe is a command procedure, not just a program, since most of
what's open is what DCL does around an image: `LIB57.COM` sets symbols
and SET VERIFY, runs the program with a case selector (a foreign
command's parameter), and shows DCL's state after each run (SHOW SYMBOL,
`$STATUS`, F$ENVIRONMENT, SHOW LOGICAL). A second small program is
the chained one. What it should settle:

- **Symbols.** The name rules (`_X`, `$X`, `1X`, `-X`, blanks, case,
  `*`, 255 and 256 characters); GET of a local at an outer level, of an
  abbreviation, of an integer (negative), of P1, `$STATUS`, and
  `$SEVERITY`; SET's value lengths (255, 256, 1024, 4097); SET of
  `$STATUS`; AMBSYMDEF; table indicators 0, 1, 2, 3; DELETE of a missing
  name, and from the wrong table; SHOW SYMBOL of what SET made (local
  symbols of a level that then ends).
- **Logical names.** SET with a value, an item list of two values,
  both, and neither; attributes (CONFINE, CONCEALED); a lowercase name;
  SUPERSEDE; a named table; DELETE of a supervisor name, of a user-mode
  one, and of none; SHOW LOGICAL after the image.
- **GET_COMMAND** in a procedure (interactive: it reads the terminal)
  with a prompt, into a short fixed string.
- **The common area.** GET before any PUT; PUT and GET of 0, 252, and
  253 characters; a short fixed GET; the longword at SYS$GL_COMMON and
  what it points to; RUN_PROGRAM's next image reading it; a spawned
  subprocess's GET.
- **Chaining.** DO_COMMAND of SHOW SYMBOL, of `@` a procedure, of an
  empty string, of a failing command, of 256 characters; with an exit
  handler declared; under SET VERIFY; `$STATUS` after; RUN_PROGRAM of
  the second program (GET_FOREIGN, GET_COMMON there), of one that isn't
  there, and a chain of two.
- **Control keys.** Each mask (0, T, Y, both, an invalid bit, an
  invalid bit with Y) to each routine, with old-mask printed, and
  F$ENVIRONMENT("CONTROL") and ("NOCONTROL") after; ON CONTROL_Y after
  DISABLE_CTRL; a subprocess's SET NOCONTROL=Y, then its parent's
  F$ENVIRONMENT.
- **No CLI.** A copy of the program `$CREPRC` runs (as `testdata/mp`'s
  milestone does), trying each routine: LIB$_NOCLI or not.
- **Interactive** (an exchange script of typed lines, as `testdata/dcl50`'s
  round 6 terminal log): PAUSE at level 0 and in a procedure, then
  SHOW SYMBOL, CONTINUE, and again with STOP; ATTACH between process 1
  and a waiting SPAWN's subprocess, both ways, with SS$_NONEXPR,
  ATTREQREF (a /NOWAIT subprocess), and attach to self; LOGOUT of the
  attached process; GET_COMMAND's prompt.
- **`$LIBCLIDEF`**: a module assembled with `$LIBCLIDEF GLOBAL`, its
  symbols listed, for govax's macro library.

### The shape of the work

`internal/librtl/cli.go` (the symbol, chaining, control-key, ATTACH,
and PAUSE routines, over `Environment.CLI`), `logical.go` (SET and
DELETE_LOGICAL, over `lnm`), `common.go` (GET/PUT_COMMON), and
GET_COMMAND beside GET_INPUT in `input.go`. `corevms`: the `CLI`
interface (`cli.go`, beside `$CLI`), the common area on the process,
and the terminal holder (`spawn.go`). `dclshell`: the interface's
implementation per Shell (`libcli.go`), the chain run after an image
(`dclsym.go`'s foreign commands and RUN, `subcli.go`'s
`subprocessRun`), the control keys per Shell, the pause level
(`dclcontroly.go`), and ATTACH (`spawncmd.go`, with the grammar).

## Subtasks (provisional)

1. **Survey**, and the CLI interface's design. **Done** 2026-10-13.
2. **Symbols**: the `corevms.CLI` interface (replacing `HasCLI`) and
   its implementation per Shell; GET_SYMBOL, SET_SYMBOL, DELETE_SYMBOL
   (local and global tables; LIB$_NOSUCHSYM, LIB$_INVSYMNAM, LIB$_NOCLI).
   **Done** 2026-10-13.
3. **Logical names**: SET_LOGICAL, DELETE_LOGICAL. **Done** 2026-10-13.
4. **GET_COMMAND.**
5. **The common area**: GET_COMMON, PUT_COMMON.
6. **Chaining**: DO_COMMAND, RUN_PROGRAM.
7. **Control keys**: SET CONTROL's settings per process; DISABLE_CTRL,
   ENABLE_CTRL.
8. **PAUSE**: the Ctrl/Y level at any command level, and in a
   subprocess.
9. **ATTACH**: the terminal holder per job, DCL's ATTACH command, and
   LIB$ATTACH.
10. **The program** (`testdata/lib57/`, a command procedure running it,
    and an interactive script), its console test, and the VMS probe.
11. **Close-out**.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
- **2026-10-13** — Subtask 1, the survey. Each routine's interface from
  the 5.0 manual's entries and section 2.2, with the VSI manual's
  differences and the DCL Dictionary's ATTACH, SPAWN, and SET CONTROL.
  Every routine is in the transfer vector and every condition in
  `vmsdef.LibrarySymbols`; `$LIBCLIDEF` isn't in govax's macro library.
  The author decided the four open choices: ATTACH in full (an explicit
  terminal holder, DCL's ATTACH, LIB$ATTACH); PAUSE as the Ctrl/Y level
  at any command level; the common area kept in Go on the process until
  the probe shows where VMS keeps it; SET CONTROL's settings per
  process. The routines reach DCL through a `corevms.CLI` interface, not
  `$CLI`, whose request block no manual at hand describes. ATTACH and
  PAUSE became separate subtasks (8 and 9), the probe moved to 10.
- **2026-10-13** — Subtask 2, symbols. `corevms.CLI` (`corevms/cli.go`)
  replaces `Environment.HasCLI`: `Environment.CLI`, nil without a CLI
  (LIB$SPAWN's test is now `env.CLI == nil`). It has only the symbol
  methods so far; each later subtask adds its own. One change from the
  survey's sketch: the methods return VMS condition values (SS$_NORMAL,
  LIB$_NOSUCHSYM, LIB$_AMBSYMDEF), not errors, since the routines return
  them as they are. `dclshell/libcli.go`'s `shellCLI` is bound to the
  process's own `Shell`: process 1's set by `newProcessOne`, a
  subprocess's by `cliHost.Start` (so `startInterpreter` no longer sets
  anything itself). `librtl/cli.go` has the three routines (codes 149
  to 151). Choices, each **unconfirmed** and a probe case:
  - LIB$_NOCLI is checked before any argument.
  - The name rules are the 5.0 entries' as written: GET_SYMBOL's name
    starts with a letter or `$`, SET's and DELETE's also `_`; only the
    first character is checked; empty (or blank) and over 255 are
    LIB$_INVSYMNAM.
  - GET_SYMBOL follows DCL's own lookup (every level's local table from
    the current one out, then the global table, abbreviations taken);
    any local level's symbol reports table 1.
  - SET_SYMBOL: an `*` in the name marks the abbreviation, as in an
    assignment; a value past 255 characters is LIB$_INVARG; and
    LIB$_AMBSYMDEF when some abbreviation the new symbol allows is one
    another symbol of the same table allows (a symbol of the same full
    name is replaced). DCL's own assignments don't check that.
  - DELETE_SYMBOL takes an abbreviation, as DELETE/SYMBOL does.
  Tests: `librtl/cli_test.go` (a fake CLI: NOCLI, the name rules,
  STRTRU, the indicators, value length), `dclshell/libcli_test.go`
  (outer levels, abbreviations, the ambiguity rule), and
  `console/libcli_test.go` (a MACRO image run by process 1's DCL and by
  a LOGINOUT subprocess's, each changing its own DCL's symbols).
- **2026-10-13** — Subtask 3, logical names. `librtl/logical.go` has
  LIB$SET_LOGICAL and LIB$DELETE_LOGICAL (codes 152 and 153), over two
  new corevms entries, `Environment.SetSupervisorLogical` and
  `DeleteSupervisorLogical` (`corevms/logicals.go`), which are `$CRELNM`'s
  and `$DELLNM`'s own code at supervisor mode: `serviceSysCrelnm`'s item
  list walk is now `crelnmItems`, and its define `createName`;
  `$DELLNM`'s delete is `deleteNames`. The table privileges
  (`lnmTablePrivilege`) still apply; only the mode is the routines'
  own. DCL isn't asked for anything: `Environment.CLI` is only checked
  (LIB$_NOCLI first, as for the symbols). Choices, each **unconfirmed**
  and a probe case:
  - SS$_ACCVIO for an argument that can't be read is *returned*, since
    both manual entries list it among the statuses, not signaled as
    other LIB$ routines do (Phase 53's Decision 5).
  - Names, values, and table names are taken as they are: not
    uppercased, trailing blanks kept.
  - Without an item list, `attributes` may hold CONFINE and NO_ALIAS for
    the name and CONCEALED and TERMINAL for the value; with one, only the
    name's ("only logical name attributes are permitted"); any other bit
    is SS$_BADPARAM. The item list is `$CRELNM`'s (LNM$_STRING,
    LNM$_ATTRIBUTES, LNM$_TABLE, LNM$_CHAIN).
  - DELETE_LOGICAL deletes the user-mode name of the same name too, as
    DCL's DEASSIGN at supervisor mode does (the DCL Dictionary), so a
    name defined only at user mode is deleted, not SS$_NOLOGNAM as the
    5.0 entry's wording ("not defined as a supervisor-mode process
    logical name") might mean. An empty name is SS$_IVLOGNAM, not
    `$DELLNM`'s "every name".
  Tests: `librtl/logical_test.go` (NOCLI, supervisor mode and case,
  SUPERSEDE, a named table, INVARG, the attributes with and without an
  item list, ACCVIO returned, DELETE's modes and the empty name) and
  `console/libcli_test.go`'s `TestLibCLI_logicals` (an image's changes,
  seen by SHOW LOGICAL after it ends).
