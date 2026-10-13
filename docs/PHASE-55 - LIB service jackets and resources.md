# Phase 55 — LIB$ service jackets, process resources, and measurement

**Status:** in progress (subtask 1, the survey, done 2026-10-13). Needs Phases 53 and 54. Part of the
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
| Protection text (7.x) | FORMAT_SOGW_PROT, PARSE_SOGW_PROT, PARSE_ACCESS_CODE: only if an interface can be settled (Phase 53's rule for 7.x-only routines) |

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
- **LIB$WAIT** takes F-floating seconds; 7.x added a flags argument
  and float type. The 5.0 form is done, and the rest noted (the
  survey: later arguments are ignored).
- **SHOW_TIMER's** output layout and rounding, for the probe. The
  manual's table and its example disagree (the survey).
- **The protection-text routines (author)**: no DIGITAL document at
  hand describes FORMAT_SOGW_PROT, PARSE_SOGW_PROT, or
  PARSE_ACCESS_CODE (searched: the 5.0 LIB$ manual, the 7.3 STR$
  manual, the 7.3.1 Utility Routines and User's manuals, the 7.3 System
  Messages manual, the 5.0 System Services manual). By Phase 53's rule
  they stay missing-routine stubs and subtask 7 is dropped, unless the
  author has a 7.x LIB$ manual's entries for them.
- **Page faults**: govax counts none, and `$GETJPI` lacks
  JPI$_PAGEFLTS, which the timers need. The survey proposes counting
  the faults `System.PageIn` resolves (subtask 5).

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
  ` ELAPSED: 0 00:00:00.44 CPU: 0:00:00.04 BUFIO: 1 DIRIO: 0 FAULTS: 18`.
  govax follows the example, which is real output, with the elapsed
  time as `$ASCTIM` writes a delta (days right-aligned in 4, `dddd
  hh:mm:ss.cc`) and CPU as hours:mm:ss.cc; the blanks between fields
  are the probe's to settle (**unconfirmed**), and the probe masks the
  values themselves, which differ from run to run.
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
| ASN_WTH_MBX (12) | `device-name.rt.dx, maximum-message-size.rl.r, buffer-quota.rl.r, device-channel.wwu.r, mailbox-channel.wwu.r` | cond | SS$_NORMAL; `$ASSIGN`'s, `$CREMBX`'s, `$GETDVI`'s; LIB$GET_EF's, LIB$FREE_EF's |
| WAIT (465) | `seconds.rf.r` | cond | SS$_NORMAL; LIB$_INVARG; LIB$_WRONUMARG; `$SCHDWK`'s |
| GET_USERS_LANGUAGE (222) | `language.wt.dx` | cond | SS$_NORMAL; LIB$_ENGLUSED; LIB$SCOPY_R_DX's |

- **AST_IN_PROG** asks `$GETJPI` (its condition values say so), so
  it's JPI$_ASTACT. Which mode's bit: govax takes the caller's mode, the
  PSL's current mode at the call (**unconfirmed**; the probe calls it
  from a user-mode AST and from the main program). It returns a value,
  not a status, so the statuses listed can't reach the caller; govax's
  can't fail.
- **ASN_WTH_MBX**: `$CREMBX` a temporary mailbox (maximum-message-size,
  buffer-quota, no logical name) onto mailbox-channel, `$GETDVIW` its
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
  operand (an F-floating -0) is LIB$_INVARG (**unconfirmed**). 7.x's
  flags and float-type arguments aren't described in any manual at
  hand: later arguments are ignored (Phase 53's 7.x rule), and LIB$_WRONUMARG
  is only for none. The F-floating value is read with `vaxfloat`.
- **GET_USERS_LANGUAGE** translates SYS$LANGUAGE (as RMS translates a
  name, `LNM$FILE_DEV`'s tables, **unconfirmed**) and writes the first
  equivalence; with none, "ENGLISH" and LIB$_ENGLUSED, a warning. A
  string failure or truncation (LIB$_STRTRU) "overrides the
  LIB$_ENGLUSED or SS$_NORMAL status".

### Protection text (7.x only)

FORMAT_SOGW_PROT, PARSE_SOGW_PROT, and PARSE_ACCESS_CODE aren't in the
5.0 manual, and no document at hand describes them (Open questions).
Their names say what they do, but not their arguments, flags, or
access-name tables, and a probe couldn't find those without guessing
the calls. By Phase 53's rule they stay missing-routine stubs; subtask 7
waits on the author.

### For the probe (subtask 8)

What the VMS run should settle: GET_EF's and GET_LUN's order, and
RESERVE_EF of 1–23 at image start; SHOW_TIMER's line (blanks and field
names), STAT_TIMER's elapsed-time sign, and a heap timer block's first
longwords; GETJPI's text for each Table LIB-5 kind, GETDVI's for VPROT,
ACPTYPE, OWNUIC, and a Boolean, and GETSYI's for VERSION (blanks
removed); the item routines' INVARG and WRONUMARG cases; the jackets'
outputs to fixed, dynamic, and varying strings (lengths, STRTRU against
BUFFEROVF, SYS_FAO past 256 bytes, SYS_ASCTIM to a dynamic string);
SYS_TRNLOG's arguments; AST_IN_PROG in and out of an AST; LIB$WAIT's
INVARG cases and whether a pending wake ends it early; and
GET_USERS_LANGUAGE with and without SYS$LANGUAGE. Values that change
from run to run (times, counts, PIDs, channels) are masked.

## Subtasks (provisional)

1. **Survey**: each routine's format, arguments, and condition values;
   the event flag and LUN pools; what the services and item tables
   already give. **Done** 2026-10-13.
2. **Jackets**: SYS_FAO, SYS_FAOL, SYS_ASCTIM, SYS_GETMSG, SYS_TRNLOG.
3. **Item routines**: GETJPI, GETDVI, GETSYI, numeric and string forms.
4. **Event flags and LUNs.**
5. **Timers.**
6. **Miscellaneous**: AST_IN_PROG, ASN_WTH_MBX, WAIT, GET_USERS_LANGUAGE.
7. **Protection text** (if settled).
8. **The program** (`testdata/lib55/`), its console test, and the VMS
   probe (SHOW_TIMER's layout, GETJPI's text forms).
9. **Close-out**.

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
