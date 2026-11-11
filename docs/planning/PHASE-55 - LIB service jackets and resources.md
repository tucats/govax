# Phase 55 — LIB$ service jackets, process resources, and measurement

**Status:** done (2026-10-13). Needs Phases 53 and 54. Part of the
LIB$ program laid out in `PHASE-53 - RTL LIB support.md` (Part A).

## Goal

The routines the 5.0 manual groups as "system service access" (its
section 2.1), "processwide resource allocation" (2.4), and "performance
measurement" (2.5), with a few small interface routines (2.7). Most are
thin layers over services govax already has. The work is mostly
arguments (strings of any class, items by value) and the conventions the
manual adds.

- **Jackets** call a service and return its string through any class
  of descriptor (Phase 53's output helper), with the arguments in the
  service's order. LIB$SYS_FAO, LIB$SYS_FAOL, LIB$SYS_ASCTIM,
  LIB$SYS_GETMSG, and LIB$SYS_TRNLOG.
- **Item routines** LIB$GETJPI, LIB$GETDVI, and LIB$GETSYI take one item
  code and return its value either as a longword or as a string. They
  convert the numeric items to text as the manual describes, which is
  the same job the lexical functions' items do
  (`corevms/lexical.go`, F$GETJPI and its kind).
- **Allocators**: event flags (GET_EF, FREE_EF, RESERVE_EF) and logical
  unit numbers (GET_LUN, FREE_LUN), per process, with the manual's
  initial reservations and statuses (LIB$_INSEF, LIB$_EF_ALRFRE, ...).
- **Timers**: INIT_TIMER, SHOW_TIMER, STAT_TIMER, FREE_TIMER. These
  record and report elapsed time, CPU time, buffered and direct I/O,
  and page faults, from `$GETJPI` items govax keeps (Phase 49's I/O
  counts). SHOW_TIMER's line has a fixed layout, which the probe should
  check.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Jackets | SYS_ASCTIM (401), SYS_FAO (404), SYS_FAOL (406), SYS_GETMSG (408), SYS_TRNLOG (in Table 2-1; no reference entry in the 5.0 manual) |
| Item routines | GETJPI (186), GETDVI (181), GETSYI (196) |
| Event flags | GET_EF (206), FREE_EF (174), RESERVE_EF (327) |
| LUNs | GET_LUN (215), FREE_LUN (175) |
| Timers | INIT_TIMER (235), SHOW_TIMER (352), STAT_TIMER (388), FREE_TIMER (176) |
| Miscellaneous | AST_IN_PROG (15), ASN_WTH_MBX (12), WAIT (465), GET_USERS_LANGUAGE (222) |
| Protection text (7.x) | FORMAT_SOGW_PROT, PARSE_SOGW_PROT, PARSE_ACCESS_CODE: described by the VSI manual (the survey's "Sources") |

## What earlier phases leave in place

- The services: `$FAO`/`$FAOL` (`corevms/fao.go`), `$ASCTIM`,
  `$GETMSG` (`message.go`), `$TRNLOG`, `$GETJPI`, `$GETDVI`, `$GETSYI`,
  `$CREMBX`, `$ASSIGN`, `$SCHDWK`/`$HIBER`/`$SETIMR`, and the AST state
  (`ast.go`). The lexical functions' item tables (`corevms/lexical.go`)
  give each item's bytes; the text F$GETJPI and F$GETDVI make of them is
  in `internal/dclshell` (the survey's "Item routines" moves it down so
  the LIB$ routines can share it).
- Phase 49's per-process I/O counts and CPU time (`iocount.go`,
  `showsys.go`).
- `vaxfloat` for LIB$WAIT's F-floating seconds.

## Open questions

- **SYS_TRNLOG** has no reference entry in the 5.0 manual. Its arguments
  are presumably `$TRNLOG`'s in order (section 2.1's rule for jackets),
  and that rule is used, marked unconfirmed (the survey).
- ~~**The initial reservations**: which event flags and LUNs the manual
  says are reserved at image start.~~ Settled by the survey: flag 0 is
  never allocated, 1–23 are initially reserved (free once FREE_EF frees
  them), 24–31 are VMS's, 32–63 are initially free; the LUN pool is
  100–119. Which number GET_EF and GET_LUN hand out first isn't said,
  a probe case.
- ~~**LIB$WAIT** takes F-floating seconds; 7.x added a flags argument
  and float type.~~ The VSI manual describes both (flags bit 0
  LIB$K_NOWAKE; float types F, D, G, H, IEEE S, IEEE T), and govax does
  them (the survey); whether VMS 7.3 on a VAX takes them is a probe case.
- **SHOW_TIMER's** output layout and rounding, for the probe. The
  5.0 manual's table and its example disagree; the VSI manual's copy of
  the example keeps its blanks (two between fields), which govax follows
  (the survey).
- ~~**The protection-text routines (author)**: no DIGITAL document at
  hand describes FORMAT_SOGW_PROT, PARSE_SOGW_PROT, or
  PARSE_ACCESS_CODE.~~ The author supplied the VSI OpenVMS LIB$ manual
  (2026-10-13), which describes all three; subtask 7 stays in the plan
  (the survey's "Protection text").
- ~~**Page faults**: govax counts none, and `$GETJPI` lacks
  JPI$_PAGEFLTS, which the timers need.~~ Subtask 5 counts the faults
  `System.PageIn` resolves on P0 and P1 pages, and `$GETJPI` returns
  them.

## The survey (subtask 1)

Each routine's interface, from the 5.0 LIB$ manual, in the notation of
Phase 53's survey (`name.access-type.mechanism`; `[ ]` optional). Page
numbers are the manual's LIB-nnn where the entry starts; the chapter 2
sections (2.1, 2.4, 2.5, 2.7) add a few rules the entries don't
repeat. The OCR text was checked against a second extraction of the
PDF (`pdftotext -layout`) where its spacing mattered (SHOW_TIMER's
line); the PDF's text layer is itself OCR, so spacing stays the
probe's. Choices the manual leaves open are marked **unconfirmed**,
each a case for the probe (subtask 8). Every routine is in VMS 7.3's
LIBRTL.EXE transfer vector (`vmsdef.ImageSymbols`: SYS_ASCTIM 0x8F0
... GET_USERS_LANGUAGE 0xAB0, and the three 7.x protection routines
0xB98–0xBA8), so each is today a missing-routine stub. Every condition
value named here is in `vmsdef.LibrarySymbols`, and every service the
routines sit on is registered in `internal/corevms`.

### Sources

- **The 5.0 LIB$ manual** (`AA-LA76A-TE`, its OCR text) is the
  reference, as for the whole LIB$ program.
- **The VSI OpenVMS RTL Library (LIB$) Manual** (VMS Software, Inc.'s
  current edition, a public web page; local-only at `~/Documents/
  Technical Doc/VMS/VSI OpenVMS RTL Library (LIB$) Manual.html`), added
  by the author on 2026-10-13, who judged a public manual usable under
  the clean-room rule. It describes the 7.x routines the 5.0 manual
  lacks. It describes current OpenVMS (Alpha, Itanium, x86-64), so where
  it adds to the 5.0 text for a routine VAX VMS 7.3 already had, the
  addition is marked **VSI** and is a probe case: it may postdate 7.3.
  Its entries for this phase's 5.0 routines agree with the 5.0 text
  except where the bullets below say otherwise. It has no SYS_TRNLOG
  entry either.

### General rules

- **Argument counts.** The item routines and LIB$WAIT list
  LIB$_WRONUMARG. govax returns it for fewer than the required
  arguments or more than the format allows (**unconfirmed** for the
  "more" case; Phase 53's `tooFewArgs` is the "fewer" one). The other
  routines don't list it; a missing required argument is read as 0, as
  elsewhere in govax's LIBRTL.
- **No event flag of their own.** GETJPI, GETDVI, GETSYI, and
  ASN_WTH_MBX "call LIB$GET_EF to allocate a local event flag" for
  their synchronous service calls, so on VMS they return LIB$_INSEF
  when the pool is empty. govax's routines call the services' Go code,
  which completes at once, and take no flag: they never return INSEF
  (a deliberate difference, noted for the probe, which needn't test it).
- **Image-private state.** LIBRTL's own data (the event flag and LUN
  pools, the static timer block) is in the image's P0 space on VMS, so
  each image starts with the initial reservations. govax keeps it in Go
  per process and resets it when an image starts.

### Jackets (section 2.1)

"You call these routines exactly as you would the corresponding system
service, but you can pass an output argument of any valid string
class": the arguments are the service's, in its order, and the output
is written by `writeString` (Phase 53) with its class rules. Input
strings: "pass only fixed-length or dynamic input strings"; govax's
`readString` takes any class anyway.

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| SYS_ASCTIM (401) | `[resultant-length.wwu.r], time-string.wt.dx [,user-time.rq.r] [,flags.rlu.r]` | cond | SS$_NORMAL; LIB$_STRTRU; LIB$_FATERRLIB; LIB$_INSVIRMEM; LIB$_INVSTRDES; SS$_IVTIME |
| SYS_FAO (404) | `character-string.rt.dx, [resultant-length.wwu.r], resultant-string.wt.dx [,directive-argument.., ...]` | cond | SS$_NORMAL; SS$_BUFFEROVF; LIB$_STRTRU; SS$_BADPARAM; LIB$_INSVIRMEM; LIB$_INVSTRDES |
| SYS_FAOL (406) | `character-string.rt.dx, [resultant-length.wwu.r], resultant-string.wt.dx, directive-argument-address.rlu.ra` | cond | the same |
| SYS_GETMSG (408) | `message-id.rlu.r, [message-length.wwu.r], destination-string.wt.dx [,flags.rlu.r] [,unsigned-resultant-array.wbu.ra]` | cond | SS$_NORMAL; SS$_BUFFEROVF; SS$_MSGNOTFND; LIB$_STRTRU; LIB$_FATERRLIB; LIB$_INSVIRMEM; LIB$_INVSTRDES |
| SYS_TRNLOG (Table 2-1 only) | `logical-name.rt.dx, [resultant-length.wwu.r], resultant-string.wt.dx [,table.wbu.r] [,access-mode.wbu.r] [,disable-mask.rlu.v]` | cond | `$TRNLOG`'s, plus LIB$_STRTRU, LIB$_INSVIRMEM, LIB$_INVSTRDES |

- **Flags by reference.** SYS_ASCTIM's and SYS_GETMSG's flags are by
  reference "and changed to value for use by" the service; omitted is
  0 for ASCTIM (date and time) and 15 for GETMSG (every part: text,
  identifier, severity, facility).
- **SYS_ASCTIM's user-time**: "if zero or no address is specified, the
  current system date and time"; negative is a delta, which must be
  under 10,000 days (SS$_IVTIME). The manual's table of lengths
  (absolute: 23 date and time, 12 date, 11 with flags 1 the time; delta:
  16 days and time, 11 the time) is `$ASCTIM`'s own rule for a fixed
  buffer of that length. govax formats into the destination's length
  when it's fixed (S, Z, SD, A, NCA: `$ASCTIM` with that buffer) and the
  whole text (23 or 16, 11 with flags 1) when it's dynamic or varying
  (**unconfirmed**). resultant-length is what was written, the
  truncated length when cut.
- **SYS_FAO and SYS_FAOL**: "if called with other than a fixed-length
  string for output, the length of the resultant string is limited to
  256 bytes and truncation will occur." So a fixed destination is
  `$FAO`'s buffer itself (SS$_BUFFEROVF when it overflows, never
  STRTRU), and a dynamic or varying one gets the text of a 256-byte
  buffer, SS$_BUFFEROVF past it (**unconfirmed**: the manual says
  "truncation", not which status); a varying one's MAXSTRLEN may then
  cut it further (LIB$_STRTRU). SYS_FAO's directive arguments are the
  argument list's fourth and later longwords, `$FAO`'s p1 to pn; SYS_FAOL's
  fourth argument is the address of the longword array, `$FAOL`'s
  prmlst. The work is `corevms`'s `formatFAO`, with the same
  parameter sources the two services use.
- **SYS_GETMSG**: the 4-byte array gets byte 1 the FAO argument count
  and byte 2 the user value (`$GETMSG`'s outadr). SS$_MSGNOTFND is a
  success ("%NONAME-..." text, as `$GETMSG` writes it). A dynamic or
  varying destination gets the whole message, at most 256 bytes
  (**unconfirmed**, the FAO jackets' limit).
- **SYS_TRNLOG** (**unconfirmed**, Table 2-1's rule only): `$TRNLOG`'s
  lognam, rsllen, rslbuf, table, acmode, dsbmsk in order, rslbuf any
  class, the translation at most 255 characters. Statuses as
  `$TRNLOG` returns them (SS$_NORMAL, SS$_NOTRAN, ...); a success's
  truncation is LIB$_STRTRU.

### Item routines

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| GETJPI (186) | `item-code.rl.r [,process-id.mlu.r] [,process-name.rt.dx] [,resultant-value.wz.r] [,resultant-string.wt.dx] [,resultant-length.wwu.r]` | cond | SS$_NORMAL; LIB$_STRTRU; SS$_BADPARAM; LIB$_INSEF; LIB$_INVARG; LIB$_INVSTRDES; LIB$_WRONUMARG; `$GETJPI`'s |
| GETDVI (181) | `item-code.rl.r [,channel.rwu.r] [,device-name.rt.dx] [,longword-integer-value.wl.r] [,resultant-string.wt.dx] [,resultant-length.wwu.r]` | cond | the same, with SS$_IVDEVNAM and `$GETDVI`'s |
| GETSYI (196) | `item-code.rl.r [,resultant-value.wz.r] [,resultant-string.wt.dx] [,resultant-length.wwu.r] [,cluster-system-id.mlu.r] [,node-name.rt.dx]` | cond | the same, with `$GETSYI`'s |

- **One item, two forms.** A "string" item (in the services' tables)
  goes to resultant-string only, and LIB$_INVARG when that's absent; a
  numeric item's value goes to resultant-value (when given) and its
  text to resultant-string (when given). Neither given for a numeric
  item: SS$_NORMAL, nothing written (**unconfirmed**). Text is written
  left-justified; blank-padded items (JPI$_USERNAME, SYI$_VERSION) lose
  their trailing blanks. resultant-length is the significant
  characters.
- **Value size.** GETJPI and GETSYI write the item's own size: a
  quadword for the privilege masks and JPI$_LOGINTIM (Table LIB-5),
  SYI$_BOOTTIME, a longword otherwise. GETDVI's value is always a
  longword.
- **GETJPI's text** (Table LIB-5): the privilege masks a list of
  enabled privileges, uppercase, comma-separated; LOGINTIM an absolute
  date-time string; PID eight hex digits; STATE CEF, COM, COMO, CUR,
  COLPG, FPG, HIB, HIBO, LEF, LEFO, MWAIT, PFW, SUSP, SUSPO (SHOW
  SYSTEM's names, `System.ProcessState`); UIC "[group,member]", by
  identifiers where the rights database has them; MODE BATCH,
  INTERACTIVE, or NETWORK (JPI$K_OTHER: "OTHER", as F$GETJPI does,
  **unconfirmed**); every other numeric item an **unsigned** decimal
  integer. That last differs from F$GETJPI, whose integers are signed.
  The other process forms: process-id is modify (the PID used is
  written back; -1 is a wildcard scan, `$GETJPI`'s context), else
  process-name (exact, no trailing blanks), else the caller.
- **GETDVI's text** (Table LIB-4): DVI$_PID and DVI$_ACPPID eight hex
  digits; DVI$_ACPTYPE NONE, F11V1, F11V2, MTA, NET, REM, JNL; DVI$_OWNUIC
  a UIC as above; DVI$_VPROT `SYSTEM=RWLP,OWNER=RWLP,GROUP=RWLP,WORLD=RWLP`,
  with a category's "=" left out when it has no access (no blanks); a
  Boolean TRUE or FALSE by its low bit; the rest unsigned decimal. govax's
  `$GETDVI` lacks DVI$_ACPTYPE: subtask 3 adds it (F11V2, `DVI$C_ACP_F11V2`,
  for a mounted ODS-2 disk; NONE otherwise), so the table can be checked.
  The device is channel or device-name, "not both" (both given: govax
  uses the channel, **unconfirmed**), neither SS$_IVDEVNAM; a device name's
  colon and what follows are ignored; DVI$M_SECONDARY (code + 1) is
  `$GETDVI`'s.
- **GETSYI's text**: every numeric item unsigned decimal (no special
  forms in the 5.0 entry). cluster-system-id is modify: zero means use
  node-name and write its CSID back, -1 a wildcard scan; omitted,
  node-name; neither, this node. govax's single node (CSID 0) makes most
  of that `$GETSYI`'s existing rule.
- **Where the text code goes.** The bytes come from `corevms`'s item
  registries (by code: `jpiItems`, `syiItems`, `dviItem`), and the text
  forms are F$GETJPI's and F$GETDVI's (`internal/dclshell`'s
  `dcllexprocess.go`, `dcllexfile.go`), which `internal/librtl` can't
  import. Subtask 3 moves the conversions (privilege list, PID, UIC,
  time, state, mode, the DVI kinds) down into `corevms`, with the
  decimal form a choice (signed for DCL, unsigned for LIB$), and both
  callers use them.

### Event flags and LUNs (section 2.4)

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| GET_EF (206) | `event-flag-number.wl.r` | cond | SS$_NORMAL; LIB$_INSEF |
| FREE_EF (174) | `event-flag-number.rlu.r` | cond | SS$_NORMAL; LIB$_EF_ALRFRE; LIB$_EF_RESSYS |
| RESERVE_EF (327) | `event-flag-number.rlu.r` | cond | SS$_NORMAL; LIB$_EF_ALRRES; LIB$_EF_RESSYS |
| GET_LUN (215) | `logical-unit-number.wl.r` | cond | SS$_NORMAL; LIB$_INSLUN |
| FREE_LUN (175) | `logical-unit-number.rl.r` | cond | SS$_NORMAL; LIB$_LUNALRFRE; LIB$_LUNRESSYS |

- **The event flag pool** (GET_EF's table): 0 never used by the
  routines; 1–23 initially reserved, available once FREE_EF frees them;
  24–31 reserved to VMS; 32–63 initially free. FREE_EF and RESERVE_EF
  of a number outside 1–23 and 32–63 are LIB$_EF_RESSYS; so
  RESERVE_EF of 1–23 at image start is LIB$_EF_ALRRES, not RESSYS.
  GET_EF with none free writes -1 and returns LIB$_INSEF. Which free
  flag GET_EF gives first (lowest or highest) isn't said
  (**unconfirmed**: govax gives the lowest; the probe prints a
  sequence). The flags' states aren't touched (no `$CLREF`).
- **The LUN pool**: 100–119; FREE_LUN outside it is LIB$_LUNRESSYS;
  GET_LUN with none free writes -1 and returns LIB$_INSLUN. Order as
  for flags (**unconfirmed**; lowest first).
- **The statuses' severities.** LIB$_INSEF and the EF_ and LUN
  statuses are all errors or worse in `vmsdef` (INSEF 0x158294 and
  LUNRESSYS 0x1582C4 are severe); they're returned, not signaled.

### Timers (section 2.5)

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| INIT_TIMER (235) | `[context.mlu.r]` | cond | SS$_NORMAL; LIB$_INVARG; LIB$_INSVIRMEM |
| SHOW_TIMER (352) | `[handle-address.rlu.r] [,code.rl.r] [,user-action-procedure.flc.rp] [,user-argument-value.rz.v]` | cond | SS$_NORMAL; LIB$_INVARG; LIB$PUT_OUTPUT's or the action routine's |
| STAT_TIMER (388) | `code.rl.r, value-argument.wz.r [,handle-address.rlu.r]` | cond | SS$_NORMAL; LIB$_INVARG |
| FREE_TIMER (176) | `handle-address.mlu.r` | cond | SS$_NORMAL; LIB$_INVARG; LIB$_BADBLOADR |

- **The block.** INIT_TIMER with context omitted fills a static block;
  with context 0 it allocates a heap block, fills it, and writes its
  address to context; with a nonzero context, refills that block,
  LIB$_INVARG if it wasn't one INIT_TIMER allocated. The block holds
  the starting time (quadword), CPU time, BUFIO, DIRIO, and page
  faults. Its layout isn't documented: govax uses those five in that
  order, 24 bytes (**unconfirmed**; the probe dumps a heap block's
  first longwords to compare), and remembers in Go which addresses are
  timer blocks, the INVARG test. SHOW_TIMER and STAT_TIMER with the
  handle omitted use the static block, LIB$_INVARG if INIT_TIMER never
  filled it. FREE_TIMER frees a heap block and zeroes the handle;
  not a timer block, LIB$_INVARG.
- **The statistics**: 1 elapsed time, 2 CPU time (10 ms units), 3
  buffered I/O, 4 direct I/O, 5 page faults, each the difference from
  the block's values (`JPI$_CPUTIM`, `JPI$_BUFIO`, `JPI$_DIRIO`, Phase
  49's counts; the time from `$GETTIM`). STAT_TIMER writes a quadword
  for 1 ("in system time format") and a longword for the others; a code
  outside 1–5 is LIB$_INVARG. Whether the quadword is a delta
  (negative) or a positive count of 100 ns units isn't said; the
  manual's example passes it to `$ASCTIM` with flags 1, which prints
  either as a time (**unconfirmed**: govax writes the negative delta,
  which `$ASCTIM` reads as a delta; the probe prints the sign).
- **SHOW_TIMER's line.** Code omitted or 0: all five on one line; 1–5:
  that one; anything else LIB$_INVARG. The text goes to SYS$OUTPUT
  through LIB$PUT_OUTPUT, or, with an action routine, to it as its
  first argument (a class S descriptor, "formatted exactly as it would
  be if written to SYS$OUTPUT. The leading character is blank") with
  user-argument-value (0 when omitted) by value as its second; its
  status is SHOW_TIMER's. The manual disagrees with itself on the
  layout: the entry's table reads `ELAPSED= dddd hh:mm:ss.cc CPU =
  hhhh:mm:ss.cc BUFIO = nnnn DIRIO = nnnn PAGEFAULTS = nnnn`, while the
  example's output (one line, printed wrapped) reads
  ` ELAPSED: 0 00:00:00.44  CPU: 0:00:00.04  BUFIO: 1  DIRIO: 0  FAULTS: 18`,
  blanks as the VSI manual's copy of the example keeps them (a leading
  blank, two between fields; the 5.0 OCR collapses them). The VSI
  manual's table repeats the 5.0 one. govax follows the example, which
  is real output: the elapsed time as days, a blank, and hh:mm:ss.cc,
  CPU as hours:mm:ss.cc, and no padding of the numbers (**unconfirmed**
  whether larger values are padded, which the example can't show; the
  probe masks the values themselves, which differ from run to run, but
  not their widths).
- **Page faults.** govax has no page fault count: no working set, so
  no soft faults, and `$GETJPI` lacks JPI$_PAGEFLTS. Subtask 5 adds a
  per-process count of the faults on invalid pages that
  `System.PageIn` resolves (demand-zero pages and section page-ins,
  charged to `processForSpace`'s process), and JPI$_PAGEFLTS returns
  it. VMS's numbers are higher (they count working set faults); the
  probe masks them.

### Miscellaneous (section 2.7)

| Routine (page) | Format | Returns | Conditions |
| --- | --- | --- | --- |
| AST_IN_PROG (15) | none | 1 if an AST is in progress, else 0 | LIB$FREE_EF's, LIB$GET_EF's, `$GETJPI`'s |
| ASN_WTH_MBX (12) | `device-name.rt.dx, [maximum-message-size.rl.r], [buffer-quota.rl.r], device-channel.wwu.r, mailbox-channel.wwu.r` | cond | SS$_NORMAL; `$ASSIGN`'s, `$CREMBX`'s, `$GETDVI`'s; LIB$GET_EF's, LIB$FREE_EF's |
| WAIT (465) | `seconds.rf.r [,flags.rlu.r] [,float-type.rlu.r]` | cond | SS$_NORMAL; LIB$_INVARG; LIB$_WRONUMARG; `$SCHDWK`'s |
| GET_USERS_LANGUAGE (222) | `language.wt.dx` | cond | SS$_NORMAL; LIB$_ENGLUSED; LIB$SCOPY_R_DX's |

- **AST_IN_PROG** asks `$GETJPI` (its condition values say so), so
  it's JPI$_ASTACT. Which mode's bit: govax takes the caller's mode, the
  PSL's current mode at the call (**unconfirmed**; the probe calls it
  from a user-mode AST and from the main program). It returns a value,
  not a status; the VSI entry says an error in it or the routines it
  calls goes to LIB$STOP. govax's can't fail.
- **ASN_WTH_MBX**: `$CREMBX` a temporary mailbox (maximum-message-size,
  buffer-quota, no logical name; **VSI**: those two are optional, and
  omitted they're `$CREMBX`'s defaults) onto mailbox-channel, `$GETDVIW` its
  physical name (DVI$_DEVNAM), and `$ASSIGN` device-name onto
  device-channel with that name as mbxnam; device-name may be a logical
  name, "one level of logical name translation". If `$ASSIGN` fails
  the mailbox channel is deassigned, so the temporary mailbox goes
  (**unconfirmed**), and the failure is returned.
- **WAIT** rounds seconds to the nearest hundredth, `$SCHDWK`s a delta
  wakeup that long, and `$HIBER`s; outside 0.0 to 100,000.0, LIB$_INVARG.
  The process hibernates in the caller's mode, so ASTs are delivered.
  Not said: a wake already pending ends `$HIBER` at once (the 5.0 entry
  describes no loop; govax returns then, **unconfirmed**, and the
  scheduled wakeup stays pending, as it would on VMS); a reserved
  operand (an F-floating -0) is LIB$_INVARG (**unconfirmed**).
  **VSI**: flags bit 0, LIB$K_NOWAKE (1), waits with `$SETIMR` and
  `$SYNCH` instead, so a `$WAKE` doesn't end it ("you should not use
  ... NOWAKE ... at AST level"); float-type is LIB$K_VAX_F (0, the
  default), VAX_D (1), VAX_G (2), VAX_H (3), IEEE_S (4), or IEEE_T (5),
  the seconds argument's format. govax does both: the VAX formats
  through `vaxfloat`, the IEEE ones through Go's `math`; another type
  is LIB$_INVARG (**unconfirmed**). LIB$_WRONUMARG for none or more
  than three. The `LIB$K_` values come from the VSI entry (none is in
  `vmsdef`); whether VMS 7.3 on a VAX takes the two arguments at all is
  a probe case (NOWAKE with a pending wake; D-floating seconds).
- **GET_USERS_LANGUAGE** translates SYS$LANGUAGE (as RMS translates a
  name, `LNM$FILE_DEV`'s tables, **unconfirmed**) and writes the first
  equivalence; with none, "ENGLISH" and LIB$_ENGLUSED, a warning. A
  string failure or truncation (LIB$_STRTRU) "overrides the
  LIB$_ENGLUSED or SS$_NORMAL status".
  **The author's addition** (2026-10-13): with no SYS$LANGUAGE, govax
  asks a `vax.language` configuration key, then the host's locale,
  before falling back to ENGLISH (subtask 6's log).

### Protection text (7.x only; the VSI manual)

| Routine | Format | Returns | Conditions |
| --- | --- | --- | --- |
| FORMAT_SOGW_PROT (0xBA0) | `protection-mask.rwu.r [,access-names.rq.ra] [,ownership-names.rq.ra] [,ownership-separator.rt.dx] [,list-separator.rt.dx], protection-string.wt.dx [,protection-length.ww.r]` | cond | SS$_NORMAL; LIB$_INVARG; LIB$_WRONUMARG; STR$_TRU |
| PARSE_SOGW_PROT (0xB98) | `protection-string.rt.dx [,access-names.rq.ra], protection-mask.wwu.r, ownership-mask.wwu.r [,end-position.ww.r]` | cond | SS$_NORMAL; LIB$_INVARG; LIB$_WRONUMARG |
| PARSE_ACCESS_CODE (0xBA8) | `access-string.rt.dx [,access-names.rq.ra], ownership-category.rwu.r, access-mask.wwu.r [,end-position.ww.r]` | cond | SS$_NORMAL; LIB$_INVARG; LIB$_WRONUMARG |

(The offsets are the transfer vector's; the VSI manual has no page
numbers. Its condition lists spell two names `LIB$_IVARG` and
`LIB$_WRONGNUMARG`, which no definition file has: they're LIB$_INVARG
and LIB$_WRONUMARG.)

- **The protection mask** is the 16-bit SOGW word: four bits per
  category, System in bits 0–3, Owner 4–7, Group 8–11, World 12–15,
  and a set bit means **no** access. The ownership masks (an argument of
  PARSE_ACCESS_CODE, a result of PARSE_SOGW_PROT) are the same nibbles,
  all four bits set for a category (System 0x000F, ..., World 0xF000).
- **The access-name table** is "a vector of 32 quadword string
  descriptors", one per bit of an access mask, an undefined access a
  zero-length name; the routines use each name's first letter (access
  names "are always abbreviated to one letter"). Omitted, it's the FILE
  class's table: READ, WRITE, EXECUTE, DELETE for bits 0–3, CONTROL
  for bit 4, the rest empty (the DCL Dictionary's "read (R), write (W),
  execute (E), and delete (D)", in XAB$W_PRO's bit order; the names
  past the first letters are **unconfirmed** and don't change any
  output). A program normally gets a table from LIB$GET_ACCNAM, which
  is `$GET_SECURITY`'s and stays out of scope (Phase 53's Part A); a
  table the program builds itself works the same.
- **FORMAT_SOGW_PROT** writes, for System, Owner, Group, World in turn,
  the ownership name, then the ownership separator and the letters of
  the accesses granted (clear bits, in bit order) if there are any,
  with the list separator between categories: `System: RWED, Owner:
  RWED, Group: RW, World: R`. Defaults: the names `System`, `Owner`,
  `Group`, `World` (a 4-descriptor vector replaces them), ": ", and
  ", ". A category with no access is its name alone (the separator
  "introduce[s] a nonempty set of access names"): `Group` with nothing
  after it but the list separator (**unconfirmed**). Output by
  `writeString`; truncation is **STR$_TRU** (the entry's status, a
  warning, not LIB$_STRTRU); protection-length the length written.
  LIB$_INVARG when protection-mask or protection-string is missing.
- **PARSE_SOGW_PROT** reads `owner[:|=][letters]` items separated by
  commas: an ownership name "in full or truncated to any number of
  characters", case blind, blanks ignored; a missing or empty access
  part is no access. protection-mask gets the mask (bits set for the
  accesses not named), ownership-mask the nibbles of the categories
  named; a category not named has its nibble 0 in both (**unconfirmed**:
  "which ownership names were present" leaves the absent ones'
  protection bits unsaid). A letter that isn't some table entry's first
  letter, an unknown name, or a bad separator is LIB$_INVARG, and
  end-position is "the offset to the character that made the protection
  string invalid"; on success it's the characters processed (the whole
  string). Whether an ambiguous abbreviation (none among the four names)
  or a repeated category is an error isn't said (**unconfirmed**: a
  repeat's later value wins).
- **PARSE_ACCESS_CODE** reads letters only (`RWE`) into access-mask
  for the category ownership-category names, a set bit meaning access
  **granted** ("the opposite of the standard protection format"). That
  the bits land in the category's own nibble (`RWE` for Group is
  0x0700) rather than in bits 0–3 is govax's reading of "a mask for
  the requested ownership category" (**unconfirmed**). An
  ownership-category that isn't exactly one category's nibble is
  LIB$_INVARG (**unconfirmed**); end-position as for PARSE_SOGW_PROT.
- **Shape of the work.** Plain Go string and bit work over a table
  read from memory; a small file of its own (`protection.go`).

### For the probe (subtask 8)

What the VMS run should settle: GET_EF's and GET_LUN's order, and
RESERVE_EF of 1–23 at image start; SHOW_TIMER's line (blanks and field
names), STAT_TIMER's elapsed-time sign, and a heap timer block's first
longwords; GETJPI's text for each Table LIB-5 kind, GETDVI's for VPROT,
ACPTYPE, OWNUIC, and a Boolean, and GETSYI's for VERSION (blanks
removed) and BOOTTIME (a decimal quadword?); F$GETDVI's VPROT and
ACPTYPE (subtask 3's DCL changes); the item routines' INVARG and WRONUMARG cases; the jackets'
outputs to fixed, dynamic, and varying strings (lengths, STRTRU against
BUFFEROVF, SYS_FAO past 256 bytes, SYS_ASCTIM to a dynamic string);
SYS_TRNLOG's arguments; AST_IN_PROG in and out of an AST; LIB$WAIT's
INVARG cases, whether a pending wake ends it early, and whether 7.3
takes the VSI flags and float-type (NOWAKE with a pending wake,
D-floating seconds); GET_USERS_LANGUAGE with and without SYS$LANGUAGE;
and the protection routines: FORMAT_SOGW_PROT of masks with an empty
category and with custom names and separators, PARSE_SOGW_PROT of
abbreviations, mixed case, blanks, `=`, a missing category, and bad
strings (end-position), PARSE_ACCESS_CODE for each category, and
LIB$GET_ACCNAM's FILE table dumped, to compare with govax's default.
Values that change from run to run (times, counts, PIDs, channels)
are masked.

## Subtasks (provisional)

1. **Survey**: each routine's format, arguments, and condition values;
   the event flag and LUN pools; what the services and item tables
   already give. **Done** 2026-10-13.
2. **Jackets**: SYS_FAO, SYS_FAOL, SYS_ASCTIM, SYS_GETMSG, SYS_TRNLOG.
   **Done** 2026-10-13.
3. **Item routines**: GETJPI, GETDVI, GETSYI, numeric and string forms.
   **Done** 2026-10-13.
4. **Event flags and LUNs.** **Done** 2026-10-13.
5. **Timers.** **Done** 2026-10-13.
6. **Miscellaneous**: AST_IN_PROG, ASN_WTH_MBX, WAIT, GET_USERS_LANGUAGE.
   **Done** 2026-10-13.
7. **Protection text**: FORMAT_SOGW_PROT, PARSE_SOGW_PROT,
   PARSE_ACCESS_CODE, from the VSI manual. **Done** 2026-10-13.
8. **The program** (`testdata/lib55/`), its console test, and the VMS
   probe (SHOW_TIMER's layout, GETJPI's text forms). **Done**
   2026-10-13 (VMS 7.1's run, and govax changed to match it).
9. **Close-out**. **Done** 2026-10-13.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
- **2026-10-13** — Subtask 1, the survey ("The survey" above). Read
  every entry of the phase in the 5.0 LIB$ manual's OCR text, and its
  chapter 2 sections 2.1, 2.4, 2.5, and 2.7; looked for the three 7.x
  protection routines in every other DIGITAL manual at hand (none
  describes them). Findings that shape the plan:
  - The event flag pool is settled by GET_EF's table (1–23 initially
    reserved but freeable, 24–31 VMS's, 32–63 free, 0 never), and the
    LUN pool is 100–119; only the allocation order is left to the probe.
  - F$GETJPI's and F$GETDVI's text forms are in `internal/dclshell`,
    out of `internal/librtl`'s reach; subtask 3 moves them into
    `corevms` for both. LIB$'s numeric text is unsigned, DCL's signed.
    `$GETDVI` gains DVI$_ACPTYPE for Table LIB-4's ACP names.
  - govax counts no page faults, and `$GETJPI` lacks JPI$_PAGEFLTS;
    subtask 5 counts the faults `System.PageIn` resolves.
  - SHOW_TIMER's entry and its own example disagree on the line's
    layout; govax follows the example (real output) and the probe
    settles it.
  - The SYS_FAO and SYS_FAOL jackets limit a non-fixed output to 256
    bytes; a fixed output is `$FAO`'s buffer itself.
  - The item routines and ASN_WTH_MBX take no event flag in govax
    (they call Go), so never return LIB$_INSEF: a deliberate
    difference.
  - The protection-text routines (subtask 7) have no description;
    they stay missing unless the author has one (Open questions).

  No code changed.
- **2026-10-13** — The author supplied the VSI OpenVMS LIB$ manual (a
  public web page, judged usable under the clean-room rule; the
  survey's "Sources"). It describes the three protection-text routines,
  so subtask 7 stays: the survey's "Protection text" now gives their
  interfaces, the SOGW mask, the access-name table (FILE's by default;
  LIB$GET_ACCNAM stays out of scope), and its unconfirmed choices. It
  also settled or added to three other points: LIB$WAIT's flags
  (LIB$K_NOWAKE) and float-type arguments, now in scope and a probe
  case for 7.3; SHOW_TIMER's line, whose example the VSI copy prints
  with its blanks; and two smaller notes (AST_IN_PROG's errors go to
  LIB$STOP; ASN_WTH_MBX's size and quota are optional). FORMAT_SOGW_PROT
  truncates with STR$_TRU, not LIB$_STRTRU. No code changed.
- **2026-10-13** — Subtask 2, the jackets (`internal/librtl/jackets.go`,
  shim codes 106–110):
  - **Shape.** Each jacket runs the service's own Go code through a new
    `corevms.Environment.CallService(name, argv)`, so the statuses,
    `$FAO`'s directives, `$GETMSG`'s lines, and `$TRNLOG`'s tables are the
    services' exactly. The service writes into a scratch buffer the
    jacket reserves below SP (`scratchBuffer`: a class S descriptor, a
    length word, the buffer), in its own call frame as a VAX routine's
    locals would be, and its RET gives the space back; then
    `copyResult` copies it with `writeString`'s class rules and writes
    the result length. A failing service status is returned with
    nothing copied, and a truncated copy is LIB$_STRTRU, overriding the
    service's success status (SS$_BUFFEROVF, SS$_MSGNOTFND,
    SS$_NOTRAN): both **unconfirmed**, for the probe.
  - **SYS_FAO and SYS_FAOL**: an output of class Z, S, or SD goes to
    `$FAO` itself (unpadded, SS$_BUFFEROVF when cut); any other class
    through a 256-byte scratch buffer, SS$_BUFFEROVF past 256 bytes.
    Which classes count as "fixed-length" and the missing padding are
    **unconfirmed** (the probe pre-fills a fixed buffer).
  - **SYS_ASCTIM**: `$ASCTIM` into a 23-byte scratch buffer, so the whole
    text, then copied (its condition list has LIB$_STRTRU but not
    SS$_BUFFEROVF, which says it always copies): a 12-byte fixed string
    gets the date and LIB$_STRTRU. flags by reference.
  - **SYS_GETMSG**: message-id and flags by reference, flags omitted 15;
    a 256-byte scratch buffer. **SYS_TRNLOG**: `$TRNLOG`'s six
    arguments, the omitted ones 0, a 255-byte scratch buffer.
  - **Tests**: `jackets_test.go` (dynamic, fixed, and varying outputs,
    the 256-byte limit, failures, INVSTRDES, FAOL's array, ASCTIM's
    forms and IVTIME, GETMSG's flags and array, TRNLOG's table byte and
    NOTRAN, SP left below the caller's), and `TestLibrtlJackets` in
    `internal/console`, a MACRO program calling LIB$SYS_FAO and
    LIB$SYS_GETMSG through their stubs with a dynamic string.
- **2026-10-13** — Subtask 3, the item routines
  (`internal/librtl/items.go`, shim codes 111–113):
  - **The text forms moved down** (`corevms/itemtext.go`): an
    `ItemForm` per item (number, string, hexadecimal, privileges, time,
    UIC, Boolean, mode, state, ACP type, protection) in two `Style`s,
    DCL's and LIB$'s, and `Environment.ItemText` writes one.
    `JPIItemForm`, `SYIItemForm`, and `DVIItemForm` read the tables, which
    hold where the styles differ: LIB$ numbers unsigned, DCL's signed;
    JPI$_OWNER and JPI$_MASTER_PID hexadecimal in DCL only (Table LIB-5
    names only JPI$_PID); SYI$_BOOTTIME, SYI$_CLUSTER_MEMBER, and
    SYI$_NODE_CSID special in DCL only (LIB$GETSYI's entry names no
    special forms: the boot time's text is the quadword in decimal,
    **unconfirmed**); DVI$_ACPTYPE's code 0 ILLEGAL in DCL (the DCL
    Dictionary) and NONE in LIB$. F$GETJPI, F$GETSYI, F$GETDVI, and
    F$CONTEXT's selection items now use them (`lexItemValue`), so their
    keyword tables are just lists of names; F$GETJPI's STATE keeps SHOW
    SYSTEM's name (an MWAIT's resource), where LIB$GETJPI's is the state
    code's (MWAIT). `PrivilegeList` and the item byte readers are
    corevms's (`dclshell.PrivilegeList` stays, a wrapper).
  - **F$GETDVI changes**: ACPTYPE (new) is F11V2 for a mounted disk,
    ILLEGAL otherwise, and VPROT is now the protection text
    (`SYSTEM=RWLP,...`), no longer an integer. The DCL Dictionary calls
    VPROT a string without giving its form; that it's LIB$'s form is
    **unconfirmed**.
  - **`$GETDVI`'s DVI$_ACPTYPE**: DVI$C_ACP_F11V2 while a volume is
    mounted on the device (every govax volume is ODS-2), 0 otherwise.
  - **The lookups** (`corevms/libitems.go`): `ProcessItem`,
    `DeviceItem`, and `SystemItem` answer one item by its code with the
    services' own target rules and privilege check, but no item list,
    event flag, IOSB, or AST. The process, device, and node names come
    in as strings the routine read with `readString` (any class), so
    `processTarget`, `dviTarget`, and `nodeTarget` gained forms that
    take a name (`processTargetBy` reads it only when the name is
    used, as before).
  - **The routines**: item code by reference; WRONUMARG for no
    arguments or more than six (VSI's seventh, LIB$GETDVI's path name,
    postdates 7.3); a string item without resultant-string is
    LIB$_INVARG, its trailing blanks dropped; a numeric item's value is
    a longword, or a quadword for an 8-byte item in GETJPI and GETSYI;
    with neither output, SS$_NORMAL and nothing written. GETDVI's
    channel is a word by reference, and wins over device-name; GETSYI
    reads node-name only when cluster-system-id is omitted or 0.
  - **Tests**: `items_test.go` (numeric and special forms, the
    quadword masks, string items and INVARG, STRTRU, the PID written
    back, NONEXPR, BADPARAM and WRONUMARG, GETDVI by channel and by
    name on a mailbox, IVDEVNAM and NOSUCHDEV, GETSYI's strings, boot
    time, CSID, and NOSUCHNODE), `corevms/itemtext_test.go` (every form
    in both styles, the tables), F$GETDVI's ACPTYPE, VPROT, and PID
    (`TestLexicalGetDVI`), and `TestLibrtlItemRoutines`, a MACRO program
    calling all three through their stubs.
- **2026-10-13** — Subtask 4, the event flag and LUN allocators
  (`internal/librtl/resources.go`, shim codes 114–118: GET_EF, FREE_EF,
  RESERVE_EF, GET_LUN, FREE_LUN):
  - **Per-image state.** A new `corevms.Environment.ImageData(key,
    create)` keeps an RTL package's data for the running image, made on
    first use and forgotten by `ImageRundown`, as LIBRTL's own P0 data
    goes with the image on VMS. The pools are its first user; the
    timers' static block (subtask 5) will be the next.
  - **The pools** follow the survey: flags 1–23 start reserved, 32–63
    free; 0, 24–31, and 64 and up are LIB$_EF_RESSYS to FREE_EF and
    RESERVE_EF; LUNs 100–119 start free, any other LIB$_LUNRESSYS.
    GET_EF and GET_LUN give the lowest free number (so GET_EF prefers a
    freed flag from 1–23 to 32 and up; **unconfirmed**, for the probe),
    and write -1 with LIB$_INSEF or LIB$_INSLUN when none is free. An
    output that can't be written signals SS$_ACCVIO with nothing
    allocated. Flags' states aren't touched.
  - **Tests**: `resources_test.go` (each pool's order and exhaustion,
    every status, freed numbers handed out again, the pools reset by
    image rundown), and `TestLibrtlResources` in `internal/console`, a
    MACRO program calling all five through their stubs and showing the
    results with LIB$SYS_FAO.
- **2026-10-13** — Subtask 5, the timers (`internal/librtl/timers.go`,
  shim codes 119–122: INIT_TIMER, SHOW_TIMER, STAT_TIMER, FREE_TIMER):
  - **Page faults** (`corevms/filesec.go`): `System.PageIn` now counts
    each fault it resolves on a P0 or P1 page in the faulting process's
    new `Process.PageFaults` (`countFault`, through `processForSpace`);
    an S0 fault belongs to no process. That's demand-zero pages and
    section page-ins only (govax has no working set, so no soft
    faults). `$GETJPI` gains JPI$_PAGEFLTS, F$GETJPI the PAGEFLTS
    keyword, and the termination message ACC$L_PAGEFLTS.
  - **The blocks.** The static block is Go data per image
    (`ImageData`, as the subtask 4 pools); a heap block is 24 bytes from
    `AllocateVM` in the survey's layout (time, CPU in 10 ms units, BUFIO,
    DIRIO, page faults; **unconfirmed**), and the image's set of
    addresses INIT_TIMER allocated is the LIB$_INVARG test for every
    handle. INIT_TIMER of a handle holding 0 allocates (LIB$_INSVIRMEM
    with no room), of a known block refills it, of anything else is
    INVARG. FREE_TIMER zeroes the handle and frees the block;
    LIB$_BADBLOADR when the heap no longer has it (the program freed it
    with LIB$FREE_VM). SHOW_TIMER and STAT_TIMER with no handle use the
    static block, INVARG until INIT_TIMER fills it; a handle holding 0
    is INVARG (**unconfirmed**).
  - **The figures** are `Clock`, `CPUTime` (10 ms units, JPI$_CPUTIM's),
    and the process's BUFIO, DIRIO, and page faults, sampled before the
    line is written (so SHOW_TIMER's own write counts in the next
    call's BUFIO). STAT_TIMER's elapsed time is a negative (delta)
    quadword (**unconfirmed**), the others a longword; codes outside
    1–5 are INVARG.
  - **SHOW_TIMER's line** follows the manual's example:
    ` ELAPSED: d hh:mm:ss.cc  CPU: h:mm:ss.cc  BUFIO: n  DIRIO: n  FAULTS: n`,
    each field led by a blank and one more between fields; a single
    code's line is that field alone, with its blank. Hundredths are
    truncated and nothing is padded (**unconfirmed**, the probe).
  - **The action routine** is called as the shim's *tail call*: a new
    `Tail` flag on `corevms.CallRequest` (and `cpu.ServiceCall`) makes
    the routine return to the instruction after the XFC, the stub's
    RET, rather than to the XFC (a shim can't be re-entered that way:
    R0 selects it). SHOW_TIMER puts the argument list (2, a class S
    descriptor of the line, the user argument), descriptor, and text
    below SP in the stub's frame; the routine's R0 is SHOW_TIMER's
    status, and the stub's RET takes them off the stack.
  - **Tests**: `timers_test.go` (the static block and image rundown,
    the whole line and each code, STAT_TIMER's five forms and INVARG,
    a heap block's contents, refill, FREE_TIMER and the INVARG cases
    after it, BADBLOADR, the action routine's CallRequest and argument
    list), and `TestLibrtlTimers` in `internal/console`, a MACRO
    program that times three demand-zero page touches (STAT_TIMER's
    faults grow by exactly 3) and passes SHOW_TIMER's line to an action
    routine whose status SHOW_TIMER returns.
- **2026-10-13** — Subtask 6, the miscellaneous routines
  (`internal/librtl/misc.go`, shim codes 123–126: AST_IN_PROG,
  ASN_WTH_MBX, WAIT, GET_USERS_LANGUAGE):
  - **AST_IN_PROG** asks `ProcessItem` for JPI$_ASTACT and returns the
    bit of the PSL's current mode at the call (**unconfirmed**), 1 or 0.
  - **ASN_WTH_MBX** runs `$CREMBX` (temporary, no logical name; size and
    quota by reference, omitted or 0 the service's defaults), asks the
    mailbox's DVI$_DEVNAM, and `$ASSIGN`s device-name with it, the two
    names as class S descriptors in the routine's frame (`stackString`),
    so any class of device-name works. A failing `$ASSIGN` deassigns
    the mailbox's channel and is returned (**unconfirmed**). No event
    flag, as the survey says.
  - **WAIT** reads seconds in the float-type's format (VAX F, D, G, H by
    `loadFloat`; IEEE S and T by Go's `math`), rounds to hundredths half
    up, and checks 0 to 100,000 (a reserved operand, NaN or infinity, or
    an unknown type is LIB$_INVARG; none or more than three arguments
    LIB$_WRONUMARG). It then `$SCHDWK`s the delta (the quadword briefly
    below SP) and `$HIBER`s, or, with LIB$K_NOWAKE, takes a flag from
    LIB$GET_EF's pool (`nextEF`, factored out of GET_EF; LIB$_INSEF with
    none free), `$SETIMR`s it, and `$WAITFR`s, the flag going back to the
    pool at the end. The wait is the service's `corevms.ErrWait`, so the
    stub's XFC runs the routine again: a call already waiting is
    remembered per image by the stub frame's FP (an AST's own LIB$WAIT
    has a lower one), and only waits again. A wake already pending ends
    the wait at once, the scheduled one staying pending (**unconfirmed**).
  - **GET_USERS_LANGUAGE** translates SYS$LANGUAGE through LNM$FILE_DEV
    and writes its first equivalence. At the author's request it then
    asks govax's new `vax.language` configuration key (uppercased; in
    `validConfigs` and HELP CONFIG KEYS), then the host's locale (the
    first of LC_ALL, LC_MESSAGES, and LANG that is set, its ISO 639 code
    mapped to a VMS language name: `fr_FR.UTF-8` is FRENCH; C, POSIX,
    and codes not in the table give nothing). Only when none of them
    names a language is it ENGLISH with LIB$_ENGLUSED, so on a host with
    an English LANG govax returns ENGLISH and SS$_NORMAL where VMS,
    without SYS$LANGUAGE, returns LIB$_ENGLUSED (a deliberate difference;
    the probe defines SYS$LANGUAGE or masks the status).
  - **Tests**: `misc_test.go` (WAIT's hibernation, its retry without a
    second wakeup, a `$WAKE` ending it, a pending wake, zero seconds,
    NOWAKE's flag that `$WAKE` doesn't end and `$SETEF` does, INSEF,
    the INVARG and WRONUMARG cases, rounding in all six formats;
    AST_IN_PROG outside an AST; ASN_WTH_MBX on a mailbox by its logical
    name, and a failing `$ASSIGN` leaving no mailbox channel; each source
    of GET_USERS_LANGUAGE's answer in order, and STRTRU), and
    `TestLibrtlMisc` in `internal/console`, a MACRO program in which
    AST_IN_PROG is 0 in the main program and 1 in an AST that `$WAKE`s
    a 10-second LIB$WAIT, a quarter-second NOWAKE wait ends by its timer,
    ASN_WTH_MBX assigns a named mailbox, and GET_USERS_LANGUAGE with
    nothing set is ENGLISH and LIB$_ENGLUSED.
  - Noticed: govax's `$GETDVI` answers SS$_NOPRIV for a channel that
    isn't assigned, where VMS's would be SS$_IVCHAN (unchanged here; a
    case for a later probe).
- **2026-10-13** — Subtask 7, the protection-text routines
  (`internal/librtl/protection.go`, shim codes 127–129:
  FORMAT_SOGW_PROT, PARSE_SOGW_PROT, PARSE_ACCESS_CODE), as the
  survey's "Protection text" describes them:
  - **The access-name table.** Only its first four entries are read
    (each category has four bits), so a program's table needn't have 32;
    each name's first letter, uppercased, is the access's letter, an
    empty name none. Omitted, it's FILE's (READ, WRITE, EXECUTE,
    DELETE). An access letter in a parsed string matches case blind
    (**unconfirmed**).
  - **FORMAT_SOGW_PROT** writes each category's name, and the ownership
    separator and its granted letters only when it has any, so a
    category with no access is its name alone (**unconfirmed**). The
    names (a vector of four descriptors) and separators may be the
    program's. Truncation is STR$_TRU; LIB$_WRONUMARG for fewer than six
    or more than seven arguments, LIB$_INVARG with no mask or output.
  - **PARSE_SOGW_PROT** skips blanks and tabs anywhere; an ownership
    name is any abbreviation of System, Owner, Group, or World, and the
    error offset for a bad name is the first character that leaves no
    name it could abbreviate (`SYX` at 2, `SYSTEMS` at 6). An empty
    item (`S:R,,W`, a trailing comma) is an error at the comma or end; a
    character among the access letters that's no letter (`;`, a second
    `:`) is an error at it. An empty or blank string is SS$_NORMAL with
    both masks 0 (**unconfirmed**). On an error neither mask is written,
    only end-position (**unconfirmed**).
  - **PARSE_ACCESS_CODE** reads the letters (blanks skipped, a comma an
    error) into the category's nibble, set for granted; a category that
    isn't one nibble's four bits is LIB$_INVARG with nothing written.
  - **Tests**: `protection_test.go` (the default and custom names,
    separators, and tables, empty categories, STR$_TRU, every parse
    form and error offset, each category of PARSE_ACCESS_CODE, the
    argument checks), and `TestLibrtlProtection` in `internal/console`, a
    MACRO program calling all three through their stubs. The console
    tests' `runMacroProgram` now shows MACRO's messages when assembly
    fails.
- **2026-10-13** — Subtask 8, the probe, written (`testdata/lib55`,
  README.md; the VMS run is next). `gen.go` writes `lib55.mar`, 216
  cases over every routine of the phase and each choice the survey left
  for the probe, `lib55.com`, and `exchange.cmd`. Values that change
  from run to run go on `~` lines, whose names alone the oracle
  compares; SHOW_TIMER's line reaches an action routine and is shown
  with its digits made 9s, so its layout is compared; LIB$WAIT's
  elapsed time is a bucket. The procedure also shows F$GETDVI's VPROT
  and ACPTYPE and F$GETJPI's STATE, MODE, and OWNER. `TestLib55Oracle`
  runs the program on a twin of the exchange volume (an RD51 labeled
  LIB55 on DUA1, LIB55_DISK, NLA0: as vax.init defines it, the host
  locale C) and skips the comparison until `vax/lib55-vax.txt` exists.
  Two things govax's run already shows, for the VMS run to judge:
  `$ASSIGN` of an unknown device (ASN_WTH_MBX's last case) is
  SS$_IVDEVNAM where `$GETDVI` says SS$_NOSUCHDEV, and LIB$GET_ACCNAM
  is a missing routine (SS$_UNSUPPORTED). The VAX is VMS 7.1, which may
  lack the 7.x protection routines; LINK's messages in the log will say.
- **2026-10-13** — The exchange volume uncovered an ods2 bug: VMS listed
  LIB55.MAR but DIRECTORY/SIZE said "no such file". ods2's `Extend`
  added a retrieval pointer per allocation even for contiguous space,
  so COPY's block-at-a-time writes gave the 270-block file 270 one-block
  pointers, a primary header and two extension headers. Fixed in ods2
  (`volume/writeheader.go`'s `addExtent`, commit b1ab6d0): contiguous
  space grows the last pointer, so the file is now one format 2 pointer.
  Left open: VMS rejected the file, so something in ods2's extension
  headers is likely wrong by VMS's rules too (each extension header's
  back link names the segment before it, not the primary header; they
  have no identification area and zeroed record attributes). Only a file
  fragmented enough to need one shows it; settling it needs a VMS-written
  multi-header file to compare (a later probe).
- **2026-10-13** — Subtask 8, the VMS run: OpenVMS V7.1 on simh's VAX
  8600 ran `lib55.com` (`testdata/lib55/vax/`; README.md's "The run").
  The procedure showed SYS$LANGUAGE defined as ENGLISH in the system
  table, and every routine of the phase resolved at LINK, the 7.x
  protection routines and LIB$GET_ACCNAM included. govax now does what
  VMS did; `TestLib55Oracle` matches all 600 lines but the nine it lists
  in `expectedLib55Differences` (two unexplained jacket edge cases, the
  mounter's PID, the output device's I/O counts, a clock tick). What the
  run settled, against the survey's choices:
  - **Jackets.** LIB$SYS_FAO and SYS_FAOL format into a 256-byte buffer
    for every output class, a fixed string too, then copy it padded:
    a fixed string too short is LIB$_STRTRU, not SS$_BUFFEROVF, and 300
    bytes into a 400-byte fixed string is SS$_BUFFEROVF with 256.
    LIB$SYS_TRNLOG's disable mask is by reference (VMS read a "2" at
    address 2). SYS_ASCTIM of a 10,001-day delta returned SS$_IVTIME
    but wrote 42 unseen characters; FAO's overflow into a varying
    string left zeros in its text: both left as expected differences.
    LIB$SYS_GETMSG showed SS$_ACCVIO's VAX text (`virtual
    address=!XL, PC=!XL, PSL=!XL`); govax's message table had the
    Alpha's (`!XH`, `PS=`), now overridden (`vmsdef`'s `vaxMessages`).
  - **Item routines.** LIB$ writes JPI$_OWNER in hexadecimal, and
    F$GETSYI's special forms (BOOTTIME a date, CLUSTER_MEMBER a truth,
    NODE_CSID hexadecimal) in LIB$GETSYI too; a string item keeps its
    trailing blanks ("SYSTEM      ", "V7.1    "); DVI$_VPROT's letters are
    RWPL in LIB$ and RWED, with ", " between categories, in F$GETDVI;
    DVI$_ACPTYPE 0 is UNKNOWN in LIB$ (ILLEGAL in DCL, confirmed);
    F$GETJPI's OWNER is empty when there's none; LIB$GETJPI's privilege
    names are the newer ones for bits 9 and 13 (NOACNT, SETPRI) and both
    for bit 5 (IMPERSONATE,DETACH). Item code 0 is SS$_NORMAL with a
    value of 0 (it ends the item list); LIB$GETJPI with one or two
    arguments is LIB$_WRONUMARG (govax: fewer than reach the value). A
    privately mounted volume's DVI$_PID and DVI$_OWNUIC are the
    mounter's, which in govax is process 1 (`deviceOwner`). `$ASSIGN`
    of an unknown device is SS$_NOSUCHDEV (was SS$_IVDEVNAM), and
    `$GETDVI` of an unassigned channel SS$_IVCHAN (was SS$_NOPRIV).
  - **Event flags and LUNs** go highest first: flags 63 down to 32, then
    the freed ones of 1 to 23; LUNs 119 down.
  - **Timers.** SHOW_TIMER's whole line is
    ` ELAPSED:    0 00:00:00.00  CPU: 0:00:00.00  BUFIO: 0  DIRIO: 1  FAULTS: 2 `
    (the elapsed time `$ASCTIM`'s delta form, a blank at the end); one
    statistic's line is the manual's table's (` ELAPSED TIME = ...`,
    ` CPU TIME = ...`, ` BUFFERED I/O COUNT = n`, ` DIRECT I/O COUNT =
    n`, ` PAGE FAULT COUNT = n`). A heap block starts with the tag
    "TIMR", the start time at offset 20 (govax: four counts between,
    their order unconfirmed); the routines check the tag where the
    handle points, so a handle of 0 is an access violation. STAT_TIMER's
    elapsed time is a negative delta, -1 when no time passed.
  - **Miscellaneous.** LIB$WAIT takes all six float types and NOWAKE on
    7.1; a type past 5 is accepted (govax reads it as F); a reserved
    operand signals SS$_ROPRAND. A `$WAKE` already pending ends a wait
    at once (confirmed). LIB$GET_USERS_LANGUAGE's truncated copy is
    still SS$_NORMAL. LIB$GET_ACCNAM exists on 7.1 and its FILE table is
    READ, WRITE, EXECUTE, DELETE, CONTROL, the rest empty: govax now has
    the routine for the FILE class (shim code 130; SS$_NOCLASS for
    another).
  - **Protection text.** FORMAT_SOGW_PROT: an empty access name takes
    FILE's letter; protection-length is the whole text's length even
    when cut; fewer arguments are LIB$_INVARG. PARSE_SOGW_PROT reads
    names and access letters as runs of letters, so blanks are errors
    (the VSI manual's "spacing is ignored" isn't 7.1's); a category not
    named is denied everything; a category named twice is LIB$_INVARG;
    an unknown name LIB$_UNRKEY at its start; other faults
    LIB$_SYNTAXERR, bad letters reported at the start of the run; a
    comma at the end is accepted. PARSE_ACCESS_CODE checks each
    character (LIB$_INVARG at it), and puts the granted bits into the
    access mask where the category's bits are, keeping its other bits,
    at the category's lowest set bit, for any category word.
- **2026-10-13** — Subtask 9, close-out: DEVIATIONS.md's Phase 55 entry
  (the behavior changes beyond LIBRTL, and what's still govax's
  choice), librtl's package doc, PLAN.md's row, and CLAUDE.md. Phase 55
  is done.
