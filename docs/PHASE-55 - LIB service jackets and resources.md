# Phase 55 — LIB$ service jackets, process resources, and measurement

**Status:** planned (2026-10-12). Needs Phases 53 and 54. Part of the
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
  convert items to text, and the jackets share them where the manual
  agrees.
- Phase 49's per-process I/O counts and CPU time (`iocount.go`,
  `showsys.go`).
- `vaxfloat` for LIB$WAIT's F-floating seconds.

## Open questions

- **SYS_TRNLOG** has no reference entry in the 5.0 manual. Its arguments
  are presumably `$TRNLOG`'s in order (section 2.1's rule for jackets),
  and that rule is used, marked unconfirmed.
- **The initial reservations**: which event flags and LUNs the manual
  says are reserved at image start (flags 1–23 for DIGITAL's use; LUNs
  100–119 for the allocator), checked in the survey.
- **LIB$WAIT** takes F-floating seconds; 7.x added a flags argument
  and float type. The 5.0 form is done, and the rest noted.
- **SHOW_TIMER's** output layout and rounding, for the probe.

## Subtasks (provisional)

1. **Survey.**
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
