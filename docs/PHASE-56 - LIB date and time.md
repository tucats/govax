# Phase 56 — LIB$ date and time

**Status:** in progress (2026-10-13; subtasks 1 to 7 done). Needs Phases 53 and 54. Part of the
LIB$ program laid out in `PHASE-53 - RTL LIB support.md` (Part A).

## Goal

The 5.0 manual's chapter 3 and the date/time routines of its reference
section, in two parts:

- **Arithmetic and conversion**: adding, subtracting, and scaling
  times (the VMS 64-bit internal format: 100 ns units since
  17-Nov-1858, deltas negative), converting between internal times and
  operations' values (CVT_TO/FROM_INTERNAL_TIME and their F-floating forms,
  with the manual's operation codes: LIB$K_DELTA_SECONDS,
  LIB$K_DAY_OF_YEAR, ...), the seven-word vector (CVT_VECTIM), and the
  simple routines DATE_TIME, DAY, and DAY_OF_WEEK.
- **Formatting** (the manual's section 3.3): FORMAT_DATE_TIME and
  CONVERT_DATE_STRING with a context (INIT_DATE_TIME_CONTEXT,
  FREE_DATE_TIME_CONTEXT). Formats come from the logical names
  LIB$DT_FORMAT and LIB$DT_INPUT_FORMAT and the language's
  LIB$DATE_FORMAT_nnn and LIB$TIME_FORMAT_nnn, or are set at run time by
  mnemonic. Also GET_DATE_FORMAT and GET_MAXIMUM_DATE_LENGTH, and
  SYS$LANGUAGE. VMS defines these names in a startup procedure
  (LIB$DT_STARTUP), so govax will need a small definition set of its own,
  for English and the predefined formats.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Arithmetic | ADD_TIMES (5), SUB_TIMES (397), MULT_DELTA_TIME (297), MULTF_DELTA_TIME (298) |
| Conversion | CVT_TO_INTERNAL_TIME (72), CVTF_TO_INTERNAL_TIME (74), CVT_FROM_INTERNAL_TIME (67), CVTF_FROM_INTERNAL_TIME (70), CVT_VECTIM (78) |
| Simple | DATE_TIME (80), DAY (82), DAY_OF_WEEK (84) |
| Formatting | INIT_DATE_TIME_CONTEXT (231), FREE_DATE_TIME_CONTEXT (172), FORMAT_DATE_TIME (169), CONVERT_DATE_STRING (27), GET_DATE_FORMAT (204), GET_MAXIMUM_DATE_LENGTH (216) |

## What earlier phases leave in place

- `vmsdef/time.go`: the internal time format and its conversions;
  `$GETTIM`, `$NUMTIM`, `$ASCTIM`, `$BINTIM` (`corevms`); the quantum and
  host clocks (`vax.hardware.clock`).
- DCL's F$CVTIME and F$TIME (`dclshell/dcllextime.go`), whose parsing of
  absolute and delta times the conversion routines can share.
- `vaxfloat` for the F-floating forms.
- Logical names (`internal/lnm`) for the format names.

## Open questions

- ~~**The formatting logical names (author)**~~ (decided 2026-10-13): VMS's startup procedure
  defines them (in LNM$DT_FORMAT_TABLE). govax needs its own definitions,
  made from the manual's tables of mnemonics and predefined formats
  (clean room). Do they belong in `vax.init`, or are they built in?
  **Built in** (the author agreed, 2026-10-13). A user may run a `vax.init` of their own
  (vax.init's own header says so), and these names are system data, not
  boot steps. They'd be made with the shared tables when the logical-name
  database is created, beside LNM$SYSTEM_TABLE (`lnm/database.go`), from
  one Go table of the 60 predefined formats. Console DCL can already make
  them too (CREATE/NAME_TABLE, DEFINE/EXECUTIVE/TABLE=), so a user can
  add LIB$DATE_FORMAT_501 and later names, as the manual describes.
  Subtask 5 makes them.
- **Languages**: English only, or the manual's other predefined
  languages? **Proposed: English**, the routines built so others can be
  added. The manual says English spellings "are not taken from logical
  name translations, but rather are looked up in an internal table". A
  language given by a table of names (LNM$LANGUAGE_*name*, which a user
  can make with DCL) works through the same code.
- ~~**Which language the date routines use (author)**~~ (decided
  2026-10-13): Phase 55 gave
  LIB$GET_USERS_LANGUAGE two fallbacks, the `vax.language` key and the
  host's locale. The date routines, by the manual, translate
  SYS$LANGUAGE alone. **Decided** (the author agreed, 2026-10-13):
  they read SYS$LANGUAGE only, so a
  date's text doesn't change with the host's locale. A language with no
  table means English, with LIB$_ENGLUSED.
- ~~**CONVERT_DATE_STRING's defaults** for missing fields (the manual's
  defaults argument and flags) and its error statuses are subtle; a probe
  is worth it.~~ The survey ("Input") lays out the manual's rules. They
  are enough to write the routine, and the probe's cases are listed in
  "For the probe".

## The survey (subtask 1)

Each routine's interface, from the 5.0 LIB$ manual, in the notation of
Phase 53's survey (`name.access-type.mechanism`; `[ ]` optional). Page
numbers are the manual's LIB-nnn where the entry starts. Chapter 3
(pages 3-1 to 3-16) adds the rules the entries share: the format
mnemonics, the logical names, the predefined formats, and the context
components. Choices the manuals leave open are marked **unconfirmed**,
and each is a case for the probe (subtask 8).

All 17 routines are in VMS 7.3's LIBRTL.EXE transfer vector
(`vmsdef.ImageSymbols`: DATE_TIME 0x830, DAY 0x838, DAY_OF_WEEK 0x9F8,
ADD_TIMES 0xA88, SUB_TIMES 0xA90, MULT_DELTA_TIME 0xA98, MULTF_DELTA_TIME
0xAA0, FORMAT_DATE_TIME 0xAB8, FREE_DATE_TIME_CONTEXT 0xAC0,
INIT_DATE_TIME_CONTEXT 0xAC8, CVT_VECTIM 0xAE0, CVT_TO_INTERNAL_TIME
0xAE8, CVT_FROM_INTERNAL_TIME 0xAF0, CVTF_FROM_INTERNAL_TIME 0xAF8,
CVTF_TO_INTERNAL_TIME 0xB00, CONVERT_DATE_STRING 0xB08, GET_DATE_FORMAT
0xB10, GET_MAXIMUM_DATE_LENGTH 0xB18). Today each is a missing-routine
stub. No other date or time entry is in the vector: the timers and
SYS_ASCTIM were Phase 55's. Every condition value named here is in
`vmsdef.LibrarySymbols`.

### Sources

- **The 5.0 LIB$ manual** (`AA-LA76A-TE`, its OCR text) is the
  reference. The OCR's errors that matter here:
  - The delimiter `|` reads as `I` ("IJANIFEB...").
  - The digit 0 in mnemonics reads as `O` ("!DO", "!MO", "!SO",
    "!H04"). The VSI manual writes `!D0`, `!M0`, `!S0`, `!H04`, and
    `!H02`. The zero spelling is used throughout below, and `!MN0` by
    the same pattern (**unconfirmed**; the probe's dump of the format
    names, below, shows it).
  - "TQ" for "TO" (CVT_TQ_INTERNAL_TIME), blanks inside names
    ("!Y 4"), and a few format strings whose blanks the examples
    contradict (noted at Table 3-3).
- **The VSI OpenVMS RTL Library (LIB$) Manual** (local-only HTML, and
  its text). It is usable like DIGITAL's manuals (CLAUDE.md, 2026-10-13).
  Its entries for these routines agree with the 5.0 text except where
  marked **VSI** below. Each such difference is a probe case, since it
  may postdate VAX VMS 7.3. It adds IEEE S-floating twins (`CVTS_*`,
  MULTS_DELTA_TIME), which aren't in the VAX vector and aren't planned.
  It refers to the *Programming Concepts Manual* for the mnemonics,
  which isn't at hand. The 5.0 chapter 3 covers them.
- **Codes**: neither manual gives the LIB$K_ and LIB$M_ values.
  `vmsdef.Symbols` has VMS 7.3's (below).

### Codes

**Operations** (CVT[F]_FROM/TO_INTERNAL_TIME). In the table, "abs"
means an absolute time is required and "delta" a delta time; the
range is the 5.0 manual's.

| Code | Name | Kind | Range (5.0) |
| --- | --- | --- | --- |
| 1 | MONTH_OF_YEAR | abs | 1–12 |
| 2 | DAY_OF_YEAR | abs | 1–366 |
| 3 | HOUR_OF_YEAR | abs | 1–8784 |
| 4 | MINUTE_OF_YEAR | abs | 1–527,040 |
| 5 | SECOND_OF_YEAR | abs | 1–31,622,400 |
| 6 | DAY_OF_MONTH | abs | 1–31 |
| 7 | HOUR_OF_MONTH | abs | 1–744 |
| 8 | MINUTE_OF_MONTH | abs | 1–44,640 |
| 9 | SECOND_OF_MONTH | abs | 1–2,678,400 |
| 10 | DAY_OF_WEEK | abs | 1–7 (Monday is 1) |
| 11 | HOUR_OF_WEEK | abs | 1–168 ("since midnight on previous Monday") |
| 12 | MINUTE_OF_WEEK | abs | 1–10,080 |
| 13 | SECOND_OF_WEEK | abs | 1–604,800 |
| 14 | HOUR_OF_DAY | abs | 1–24 (**VSI**: 0–23) |
| 15 | MINUTE_OF_DAY | abs | 1–1440 (**VSI**: 0–1439) |
| 16 | SECOND_OF_DAY | abs | 1–86,400 (**VSI**: 0–86,399) |
| 17 | MINUTE_OF_HOUR | abs | 1–60 (**VSI**: 0–59) |
| 18 | SECOND_OF_HOUR | abs | 1–3600 (**VSI**: 0–3599) |
| 19 | SECOND_OF_MINUTE | abs | 1–60 (**VSI**: 0–59) |
| 20 | JULIAN_DATE | abs | "number of days since system zero time" |
| 21–25 | DELTA_WEEKS, `_DAYS`, `_HOURS`, `_MINUTES`, `_SECONDS` | delta | whole units |
| 26–30 | DELTA_WEEKS_F ... DELTA_SECONDS_F | delta | fractional units (F-floating) |

LIB$K_MAX_OPERATION is 31, one past the last.

**Components** (INIT_DATE_TIME_CONTEXT), with the number of elements
each takes: MONTH_NAME 0 (12), OUTPUT_FORMAT 1 (2), INPUT_FORMAT 2 (1),
MONTH_NAME_ABB 3 (12), LANGUAGE 4 (1, **VSI**: the 5.0 manual lacks
it), WEEKDAY_NAME 6 (7), WEEKDAY_NAME_ABB 9 (7), MERIDIEM_INDICATOR 12
(2), RELATIVE_DAY_NAME 15 (3), FORMAT_MNEMONICS 18 (9). The 5.0 manual
spells it MERIDIAN_INDICATOR, but VMS's definitions and the VSI manual
spell it MERIDIEM. The spelled components are three apart, which
suggests three slots each, for the upper, lower, and capitalized forms
of the LIB$..._[U|L|C] logical names. Codes 5, 7, 8, 10, 11, 13, 14,
16, and 17 have no names; govax gives them LIB$_ILLCOMPONENT
(**unconfirmed**), as it does codes past 18.

**Masks.** FORMAT_DATE_TIME's and GET_MAXIMUM_DATE_LENGTH's flags are
LIB$M_TIME_FIELDS 1 and LIB$M_DATE_FIELDS 2. CONVERT_DATE_STRING's
field bits follow $NUMTIM's order: LIB$M_YEAR 1, MONTH 2, DAY 4,
HOUR 8, MINUTE 0x10, SECOND 0x20, HUNDREDTHS 0x40.

### General rules

- **Times.** A positive quadword is absolute (100 ns units since
  17-Nov-1858 00:00, local time), and a negative one is a delta,
  negated. This is `corevms/vmstime.go`'s model. Zero counts as
  absolute (the base date). The 5.0 manual says "delta times must be
  less than 10,000 days" for the arithmetic and the conversions (the
  VSI manual drops the sentence); that is `corevms`'s `maxDeltaDays`,
  and a delta past it is LIB$_IVTIME.
- **Statuses.** The arithmetic, the conversions, and CVT_VECTIM return
  LIB$_NORMAL (0x158001). DATE_TIME, DAY, DAY_OF_WEEK, and the
  formatting routines return SS$_NORMAL (1). govax follows each entry
  (**unconfirmed**: the probe prints the values).
- **Argument counts.** LIB$_WRONUMARG is listed for the arithmetic, the
  conversions, CVT_VECTIM, CONVERT_DATE_STRING, and GET_DATE_FORMAT.
  govax returns it when there are fewer than the required arguments
  (`tooFewArgs`), or more than the format allows (**unconfirmed** for
  the "more" case, as in Phase 55). The others don't list it; a missing
  argument is read as 0, as elsewhere in govax's LIBRTL.
- **"Zero by value" and "zero by reference".** For DAY, DAY_OF_WEEK,
  and FORMAT_DATE_TIME, a time argument that is omitted or is 0 by
  value (address 0) means the current time (`Environment.Clock`). The
  address of a zero quadword means 17-Nov-1858, which was a Wednesday:
  day 0, day of week 3.
- **No reentrancy.** The formatting routines list LIB$_REENTRANCY, for
  a context two threads use at once. govax's routines run in Go without
  being interrupted, so they never return it. This is a deliberate
  difference, like Phase 55's LIB$_INSEF, and the probe needn't test it.

### Arithmetic

- **ADD_TIMES time1.rq.r, time2.rq.r, resultant-time.wq.r** (LIB-5).
  delta + delta is a delta; absolute + delta, in either order, is
  absolute; two absolute times are LIB$_ONEDELTIM. Adding a delta to
  an absolute time moves it later (its magnitude is added). A delta
  result of 10,000 days or more is IVTIME (**unconfirmed**, as is an
  absolute result past the year 9999).
- **SUB_TIMES time1, time2, resultant-time** (LIB-397). Three cases
  are allowed:
  - delta − delta is a delta, and time1 must be the longer interval;
  - absolute − absolute is a delta, and time1 must be the later time;
  - absolute − delta is absolute, and moves the time earlier
    (**unconfirmed**: the table says only "time1-time2").
  A delta time1 with an absolute time2 is LIB$_INVARGORD, and a
  negative result is LIB$_NEGTIM. **VSI**: equal times give the
  smallest delta, −1 (100 ns, shown as "0 00:00:00.00"), because a
  delta can't be zero. The 5.0 text asks for a later time, which
  leaves equal times open: NEGTIM or the 100 ns delta. A probe case.
- **MULT_DELTA_TIME multiplier.rl.r, delta-time.mq.r** (LIB-297).
  Multiplies the delta in place by the multiplier's absolute value. An
  absolute delta-time is IVTIME (**unconfirmed**), as is a product of
  10,000 days or more. The product of a zero multiplier would be a zero
  "delta", a probe case. **VSI** says "the initial delta-time argument
  must be greater than 0", which reads as "nonzero".
- **MULTF_DELTA_TIME multiplier.rf.r, delta-time.mq.r** (LIB-298). The
  same, with an F-floating multiplier, read by `loadFloat`. The product
  is rounded to a 100 ns tick (**unconfirmed**: rounded or truncated).
  A reserved operand is a probe case.

### Conversion

- **CVT_FROM_INTERNAL_TIME operation.rlu.r, resultant-time.wlu.r
  [,input-time.rq.r]** (LIB-67). Converts by one of operations 1–25,
  using the current time when input-time is omitted. An absolute
  operation on a delta is LIB$_ABSTIMREQ, and a delta operation on an
  absolute time is LIB$_DELTIMREQ, so 21–25 with no input-time is
  always DELTIMREQ. Any other code is LIB$_INVOPER, the _F codes 26–30
  included (**unconfirmed**). The delta operations return whole units,
  truncated. Three points are open:
  - **The ranges.** The VSI ranges for HOUR_OF_DAY through
    SECOND_OF_MINUTE start at 0 where the 5.0 ranges start at 1.
    govax's first reading: units of year, month, and week are 1-based
    ordinals (DAY_OF_YEAR of 1 January is 1, HOUR_OF_YEAR of 00:30 that
    day is 1); units of a day, an hour, and a minute are 0-based, as
    VSI says. The probe settles both.
  - **JULIAN_DATE.** The footnote says days since 17-Nov-1858, which
    would be LIB$DAY's number. VMS usage often means the day of the
    year by "Julian date" instead. govax follows the footnote; the
    probe confirms.
  - **The current time.** It is read once per call.
- **CVTF_FROM_INTERNAL_TIME operation, resultant-time.wf.r,
  input-time.rq.r** (LIB-70). Operations 26–30 only, others INVOPER.
  input-time is required (no brackets) and must be a delta (else
  DELTIMREQ). The result is the exact tick count divided by the unit,
  rounded once to F-floating by `vaxfloat`.
- **CVT_TO_INTERNAL_TIME operation, input-time.rl.r,
  resultant-time.wq.r** (LIB-72). Operations 21–25 only, others
  INVOPER. input-time is a signed longword of whole units. The result
  is the negated delta. The 5.0 manual says the input "must neither be
  negative nor greater than 10,000 days" (IVTIME); **VSI** says "must be
  greater than 0". So the open case is zero.
- **CVTF_TO_INTERNAL_TIME operation, input-time.rf.r, resultant-time**
  (LIB-74). Operations 26–30, with an F-floating input. The same limits
  apply, and the fraction of a tick is rounded (**unconfirmed**).
- **CVT_VECTIM input-time.rwu.ra, resultant-time.wq.r** (LIB-78).
  The inverse of $NUMTIM's seven words (year, month, day, hour, minute,
  second, hundredths). If year and month are both 0, the vector is a
  delta of *day* days plus the time of day; otherwise it's absolute.
  Fields out of range are IVTIME: month 1–12, the day within the month,
  hour < 24, minute < 60, second < 60, hundredths < 100, a delta under
  10,000 days, and a year from 1858 (on or after 17 November) to 9999
  (**unconfirmed**). A year of 0 with a nonzero month is absolute in the
  manual's words, so it is IVTIME (**unconfirmed**). This is new code:
  corevms has `numericTime` ($NUMTIM) but not its inverse. It belongs
  beside it (`vectorTime`, exported through `corevms/export.go`), since
  CONVERT_DATE_STRING builds its result the same way.

### Simple

- **DATE_TIME date-time-string.wt.dx** (LIB-80). The current time in
  $ASCTIM's 23-character form (`formatVMSTime`), with a blank-padded
  day: " 9-OCT-1988". The manual writes the form "dd-mmm-yyyy", but
  LIB$SYS_ASCTIM's output (Phase 55, checked on VMS) is blank-padded.
  It is written with `writeString` by the output's class (LIB$_STRTRU
  when truncated; LIB$_INVSTRDES, LIB$_INSVIRMEM).
- **DAY number-of-days.wl.r [,user-time.rq.r] [,day-time.wl.r]**
  (LIB-82). The number of days in the time, and optionally the 10 ms
  units since that day's midnight. The time "is not verified": a delta
  (negative) gives a negative day count. Whether that count is
  truncated toward zero or floored, and what day-time is then, are
  probe cases. SS$_INTOVF is for "a date past the year 8600". That fits
  a VAX division of the time by 10⁹ (100-second units) whose quotient
  must fit a longword: 2³¹ × 10⁹ ticks is 25-Dec-8663.
  govax returns INTOVF from that point (**unconfirmed**; **VSI**: "well
  beyond the year 9999", 64-bit arithmetic). The 5.0 example ("The
  daynumber is 46738") is 4-Nov-1986.
- **DAY_OF_WEEK [user-time.rq.r,] day-number.wlu.r** (LIB-84). 1 is
  Monday and 7 is Sunday, with the zero rules above. Only SS$_NORMAL is
  listed. What a delta gives is a probe case. govax uses the day of the
  week of the time's day count (**unconfirmed**).

### Formatting: mnemonics and predefined formats

**Mnemonics** (section 3.3.2.1). Each is written after a `!`. Anything
else in a format is punctuation, copied as written (output) or matched
(input). The spellings follow a pattern: a zero means zero-filled, a
doubled letter no fill, and B blank-filled.

- Date fields:
  - day: D0, DD, DB;
  - weekday: WU, WAU, WC, WAC, WL, WAL (U upper case, C capitalized, L
    lower case; A abbreviated);
  - alphabetic month: MAU, MAAU, MAC, MAAC, MAL, MAAL;
  - numeric month: MN0, MNM, MNB;
  - year: Y4, Y3, Y2, Y1 (the last n digits).
- Time fields:
  - hour: H04, HH4, HB4 (24-hour clock) and H02, HH2, HB2 (12-hour
    clock);
  - minute: M0, MM, MB;
  - second: S0, SS, SB;
  - fraction: C7 to C1 (digits of the fraction, C7 being whole ticks);
  - meridiem: MIU, MIC, MIL.

Matching is longest first, against the table: in "!H04H!M0m" the
mnemonics are H04 and M0, and "H" and "m" are punctuation. An unknown
mnemonic is LIB$_UNRFORCOD. These are **unconfirmed**:

- whether mnemonics may be written in lower case;
- that fractions are truncated, as $ASCTIM's are;
- that the 12-hour clock shows 0:xx as 12;
- that the meridiem is the first indicator from 00:00 to 11:59:59.99
  and the second from 12:00 (the manual's LIB$MI_ text says this).

**Predefined formats** (Tables 3-3 and 3-4), each a logical name in
LNM$DT_FORMAT_TABLE (executive mode). These are the date formats, with
the manual's example for 13-Jan-1988 09:13:25.14:

| nnn | LIB$DATE_FORMAT_nnn | Example |
| --- | --- | --- |
| 001 | `!DB-!MAAU-!Y4` | 13-JAN-1988 |
| 002 | `!DB !MAU !Y4` | 13 JANUARY 1988 |
| 003 | `!DB.!MAU !Y4` | 13.JANUARY 1988 |
| 004 | `!DB.!MAU.!Y4` | 13.JANUARY.1988 |
| 005 | `!DB !MAU !Y2` | 13 JANUARY 88 |
| 006 | `!DB !MAAU !Y2` | 13 JAN 88 |
| 007 | `!DB.!MAAU !Y2` | 13.JAN 88 |
| 008 | `!DB.!MAAU.!Y2` | 13.JAN.88 |
| 009 | `!DB !MAAU !Y4` | 13 JAN 1988 |
| 010 | `!DB.!MAAU !Y4` | 13.JAN 1988 |
| 011 | `!DB.!MAAU.!Y4` | 13.JAN.1988 |
| 012 | `!MAU !DD, !Y4` | JANUARY 13, 1988 |
| 013 | `!MN0/!D0/!Y2` | 01/13/88 |
| 014 | `!MN0-!D0-!Y2` | 01-13-88 |
| 015 | `!MN0.!D0.!Y2` | 01.13.88 |
| 016 | `!MN0 !D0 !Y2` | 01 13 88 |
| 017 | `!D0/!MN0/!Y2` | 13/01/88 |
| 018 | `!D0/!MN0-!Y2` | 13/01-88 |
| 019 | `!D0-!MN0-!Y2` | 13-01-88 |
| 020 | `!D0.!MN0.!Y2` | 13.01.88 |
| 021 | `!D0 !MN0 !Y2` | 13 01 88 |
| 022 | `!Y2/!MN0/!D0` | 88/01/13 |
| 023 | `!Y2-!MN0-!D0` | 88-01-13 |
| 024 | `!Y2.!MN0.!D0` | 88.01.13 |
| 025 | `!Y2 !MN0 !D0` | 88 01 13 |
| 026 | `!Y2!MN0!D0` | 880113 |
| 027 | `/!Y2.!MN0.!D0` | /88.01.13 |
| 028 | `!MN0/!D0/!Y4` | 01/13/1988 |
| 029 | `!MN0-!D0-!Y4` | 01-13-1988 |
| 030 | `!MN0.!D0.!Y4` | 01.13.1988 |
| 031 | `!MN0 !D0 !Y4` | 01 13 1988 |
| 032 | `!D0/!MN0/!Y4` | 13/01/1988 |
| 033 | `!D0-!MN0-!Y4` | 13-01-1988 |
| 034 | `!D0.!MN0.!Y4` | 13.01.1988 |
| 035 | `!D0 !MN0 !Y4` | 13 01 1988 |
| 036 | `!Y4/!MN0/!D0` | 1988/01/13 |
| 037 | `!Y4-!MN0-!D0` | 1988-01-13 |
| 038 | `!Y4.!MN0.!D0` | 1988.01.13 |
| 039 | `!Y4 !MN0 !D0` | 1988 01 13 |
| 040 | `!Y4!MN0!D0` | 19880113 |

And the time formats:

| nnn | LIB$TIME_FORMAT_nnn | Example |
| --- | --- | --- |
| 001 | `!H04:!M0:!S0.!C2` | 09:13:25.14 |
| 002 | `!H04:!M0:!S0` | 09:13:25 |
| 003 | `!H04.!M0.!S0` | 09.13.25 |
| 004 | `!H04 !M0 !S0` | 09 13 25 |
| 005 | `!H04:!M0` | 09:13 |
| 006 | `!H04.!M0` | 09.13 |
| 007 | `!H04 !M0` | 09 13 |
| 008 | `!HH4:!M0` | 9:13 |
| 009 | `!HH4.!M0` | 9.13 |
| 010 | `!HH4 !M0` | 9 13 |
| 011 | `!H02:!M0 !MIU` | 09:13 AM |
| 012 | `!HH2:!M0 !MIU` | 9:13 AM |
| 013 | `!H04!M0` | 0913 |
| 014 | `!H04H!M0m` | 09H13m |
| 015 | `kl !H04.!M0` | kl 09.13 |
| 016 | `!H04H!M0'` | 09H13' |
| 017 | `!H04.!M0 h` | 09.13 h |
| 018 | `h !H04.!M0` | h 09.13 |
| 019 | `!HH4 h !MM` | 9 h 13 |
| 020 | `!HH4 h !MM min !SS s` | 9 h 13 min 25 s |

The OCR's blanks are corrected from the examples (date 031's "!MNO
!DO!Y4" is "01 13 1988"; the OCR splits "01/13/ 1988" and "09: 13:25"
where the formats have no blank). Date 018 ("13/01-88") and 027
("/88.01.13") look odd, but the format and the example agree, so they
stay. The probe dumps VMS's LNM$DT_FORMAT_TABLE (SHOW LOGICAL/TABLE=;
this is VMS's output, so the clean-room rule allows it) to check every
string. **VMS's standard format** is the pair date 001 and time 001:
`!DB-!MAAU-!Y4` and `!H04:!M0:!S0.!C2` (the manual's
LIB$K_OUTPUT_FORMAT example).

### Formatting: the logical names and the context

- **Run time** (section 3.3.2.2). Three names are involved:
  - SYS$LANGUAGE names the language. A language other than English
    takes its spellings from the table LNM$LANGUAGE_*name*:
    LIB$WEEKDAYS_x, LIB$WEEKDAY_ABBREVIATIONS_x, LIB$MONTHS_x,
    LIB$MONTH_ABBREVIATIONS_x, LIB$MI_x, and LIB$RELATIVE_DAYS_x (x is
    U, L, or C, with Monday, January, and "yesterday, today, tomorrow"
    first), plus LIB$FORMAT_MNEMONICS. English is built in, and the
    manual's example defines LIB$FORMAT_MNEMONICS in
    LNM$LANGUAGE_ENGLISH too.
  - LIB$DT_FORMAT is a list of LIB$DATE_FORMAT_nnn and
    LIB$TIME_FORMAT_nnn names (001–040 and 001–020 are DIGITAL's,
    501–999 the user's). Each name is translated in LNM$DT_FORMAT_TABLE
    to its format, and the formats are output in the list's order with
    one blank between them.
  - LIB$DT_INPUT_FORMAT is a format string itself, not a name.

  A failed translation falls back:
  - SYS$LANGUAGE, or a spelling name: English is used, and the status
    is LIB$_ENGLUSED.
  - LIB$DT_FORMAT, LIB$DT_INPUT_FORMAT, or a format name: the standard
    format is used, and the status is LIB$_DEFFORUSE.

  Both statuses have severity 0, warning (ENGLUSED 0x159028, DEFFORUSE
  0x159030). Whether VMS returns DEFFORUSE when the names were simply
  never defined, as the text reads, is a probe case: that's the usual
  state.

  The user's names (SYS$LANGUAGE, LIB$DT_FORMAT, LIB$DT_INPUT_FORMAT)
  are translated through LNM$FILE_DEV, as GET_USERS_LANGUAGE translates
  SYS$LANGUAGE (**unconfirmed**). The rest must be executive mode in
  their own tables.
- **The context.** Every formatting routine takes an optional
  *user-context* longword: 0 at first, after which the program mustn't
  touch it. The first call allocates the context with LIB$GET_VM.
  Without the argument, a per-image static context is used, which is
  freed at image exit (FREE_DATE_TIME_CONTEXT, LIB-172, says so). The
  block's contents aren't documented. govax's is a heap block (so
  FREE's LIB$FREE_VM is real) whose first longword names Go-side state
  kept per process, beside the timers' (`Environment.ImageData`). The
  cell is zeroed by FREE (**unconfirmed**).
- **INIT_DATE_TIME_CONTEXT user-context.mlu.r, component.rl.r,
  init-string.rt.dx** (LIB-231). Sets one component per call, from a
  string that starts and ends with its own delimiter (its first
  character), with exactly the component's number of elements:
  - an unknown component is LIB$_ILLCOMPONENT;
  - a string that doesn't end with the delimiter is LIB$_ILLINISTR
    (as is an empty one, **unconfirmed**);
  - the wrong number of elements is LIB$_NUMELEMENTS;
  - an unknown mnemonic in a format is LIB$_UNRFORCOD.

  An initialized context is used *instead of* the logical names (5.0:
  "it is not expected that some items are preinitialized and other items
  are gathered through logical name translation"). **VSI** adds
  LIB$K_LANGUAGE, with which a context names only its language and
  takes the spellings from the logical names. Whether VMS 7.3 has it is
  a probe case; its code is in 7.3's definitions. An output format's
  two elements are a date part and a time part, in output order: govax
  tells them apart by their mnemonics. An element mixing date and time
  fields is LIB$_ILLFORMAT (**unconfirmed**). Whether a context missing
  a spelling that its format needs falls back to English is also
  **unconfirmed**.
- **FREE_DATE_TIME_CONTEXT [user-context.mlu.r]** (LIB-172). Frees the
  context, or the static one with no argument. Returns SS$_NORMAL, or
  LIB$FREE_VM's status.

### Output

- **FORMAT_DATE_TIME date-string.wt.dx [,date.rq.r]
  [,user-context.mlu.r] [,date-length.wl.r] [,flags.rlu.r]** (LIB-169).
  Formats an absolute time (the current one when *date* is omitted or 0
  by value; a delta is LIB$_ABSTIMREQ) by the context's output format or
  LIB$DT_FORMAT. With *flags*, LIB$M_DATE_FIELDS and LIB$M_TIME_FIELDS
  choose the parts. A part that LIB$DT_FORMAT doesn't give falls back
  to the standard format for that part, so with nothing defined, both
  flags give $ASCTIM's text. The output is written with `writeString`
  (LIB$_STRTRU when truncated), and *date-length* receives the bytes
  written (**unconfirmed**: written, or the whole text's length). Flags
  of 0, or with other bits set, are probe cases.
- **GET_MAXIMUM_DATE_LENGTH date-length.wl.r [,user-context]
  [,flags]** (LIB-216). The widest text the same format can produce: the
  longest spelling of each spelled field in the language (SEPTEMBER and
  WEDNESDAY, 9 each), each number at its widest, and the blank between
  parts. The manual's example (`!MAU !DD, !Y4` and `!HH2:!M0 !MIU`,
  "SEPTEMBER 21, 1988 11:24 PM") says 28, but that example string has
  27 characters (the VSI manual repeats it with 2000). So the probe
  must settle whether VMS adds one. It lists LIB$_ABSTIMREQ, though it
  takes no time; that is probably copied from FORMAT_DATE_TIME and
  can't arise.
- **GET_DATE_FORMAT format-string.wt.dx [,user-context]** (LIB-204).
  The input format in "legible" form: the punctuation, with whitespace
  compressed, and each field as the language's mnemonic (English
  YYYY, MM, DD, HH, MM, SS, CC, AM/PM, MONTH). The year and the
  fraction take their digit count as a suffix. The manual's examples:
  - `!MAU !DD, !Y4 !H04:!M0:!S0:!C2` gives
    `MONTH DD, YYYY4 HH:MM:SS:CC2`;
  - `!MN0-!D0-!Y2 !H04:!M0:!S0.!C2` gives `MM-DD-YYYY2 HH:MM:SS.CC2`;
  - `!MAAU !D0, !Y2 !H02:!M0:!S0.!C4 !MIU` gives
    `MONTH DD, YYYY2 HH:MM:SS.CC4 AM/PM`.

  It returns LIB$_ILLFORMAT for an input format that breaks the input
  rules below. With no LIB$DT_INPUT_FORMAT, it returns DEFFORUSE and
  the standard format's legible text (**unconfirmed**; the probe shows
  that text).

### Input

- **CONVERT_DATE_STRING date-string.rt.dx, date-time.wq.r
  [,user-context] [,flags.rlu.r] [,defaults.rwu.ra]
  [,defaulted-fields.wlu.r]** (LIB-27; section 3.3.3).
  - **The input format** (the context's, or LIB$DT_INPUT_FORMAT, or the
    standard one) must hold at least the first seven of: month
    (alphabetic or numeric), day, year, hour, minute, second, fraction,
    and meridiem. The meridiem is required with a 12-hour clock and
    illegal with a 24-hour one. The date fields and the time fields
    must form two separate groups. Otherwise the format is
    LIB$_ILLFORMAT. A weekday in an input format is ILLFORMAT too
    (**unconfirmed**: it's not in the list).
  - **Matching.** Case and abbreviation don't matter: "feb" matches
    `!MAU`, and "am" matches `!MIU`. Whitespace in the format is one
    synchronizing blank, so any run of blanks and tabs matches it, and
    leading and trailing blanks are ignored. Explicit punctuation
    matches exactly. A blank-filled number may begin with blanks. The
    string may instead be YESTERDAY, TODAY, or TOMORROW (or the
    language's words). That's the date, with the time from the defaults
    (**unconfirmed**). An abbreviation that matches two names ("JU") is
    LIB$_AMBDATTIM (**unconfirmed**: the manual gives no example of
    AMBDATTIM).
  - **Omitted fields.** A field is omitted when its punctuation is
    there and its value isn't (Table 3-2: "-MAR", "23:", "::23",
    ".01"). *flags* says which fields may be omitted (default 0x78, the
    time fields). *defaults*, a $NUMTIM vector, supplies their values;
    without it, the date fields are today's and the time fields 0. An
    omitted field that the flags don't allow is LIB$_INCDATTIM.
    *defaulted-fields* receives the mask of the fields that were
    omitted. Table 3-2 shows date fields omitted too, so its examples
    must run with date bits in the flags. Its first example,
    "31" → "31-FEB-1988", is a date that doesn't exist, so the probe
    should try it (IVTIME expected).
  - **Errors.** A value out of range is LIB$_IVTIME. A missing
    date-string or date-time is LIB$_INVARG, and a bad descriptor
    LIB$_INVSTRDES.
  - **The code.** DCL's F$CVTIME parsing (`dcllextime.go`) and $BINTIM's
    (`parseVMSTime`) have fixed forms. This routine is driven by the
    format, so it is new code: a matcher over the parsed format that
    shares the mnemonic table with the output side.

### The shape of the work

The mnemonic table, the format parser, and the output and input engines
are plain Go over strings and the seven $NUMTIM fields, with no memory
or process state, and they are tested on their own. The routines read
their arguments, find the context or the logical names, and call them.
Planned files are `internal/librtl/times.go` (arithmetic, conversion,
CVT_VECTIM), `days.go` (DATE_TIME, DAY, DAY_OF_WEEK), `dtformat.go`
(the mnemonics and the two engines), `dtcontext.go` (contexts and
names), and the definitions' table where subtask 5 puts it. The time
fields come from corevms's `numericTime` and the new `vectorTime`.

### For the probe (subtask 8)

What the VMS run should settle:

- **Arithmetic.**
  - ADD_TIMES: two absolutes, deltas at and past 10,000 days, and the
    direction of absolute + delta.
  - SUB_TIMES: equal times (NEGTIM or −1), absolute − delta's
    direction, and delta − absolute (INVARGORD).
  - MULT[F]_DELTA_TIME: by 0, by a negative multiplier, of an
    absolute time, past 10,000 days, MULTF's rounding, and a reserved
    operand.
- **Conversion.**
  - CVT_FROM_INTERNAL_TIME: every operation on one known time (the
    0- or 1-based ranges, JULIAN_DATE), at midnight and 23:59:59.99 of
    31 December in a leap year, the _F codes, 0 and 31.
  - CVT[F]_TO_INTERNAL_TIME: 0, negative inputs, 10,000 days, and
    rounding.
  - CVT_VECTIM: deltas, absolutes, every field one past its range, 29
    February in leap and common years, year 0 with a nonzero month,
    and a date before 17-Nov-1858.
- **Simple.**
  - DATE_TIME: to fixed, dynamic, and varying strings, and a short
    fixed one (STRTRU).
  - DAY: a delta (truncated or floored, and day-time), the zero rules,
    and the year-8600 INTOVF boundary.
  - DAY_OF_WEEK: the zero rules, and a delta.
- **Statuses.** Each routine's success value, LIB$_NORMAL or
  SS$_NORMAL.
- **Formatting.**
  - The names: SHOW LOGICAL/TABLE=LNM$DT_FORMAT_TABLE (every format
    string) and the LNM$LANGUAGE_ENGLISH table, if VMS has one.
  - FORMAT_DATE_TIME: with nothing defined (DEFFORUSE or not), with
    each predefined format on one time at 00:xx, 09:xx, 12:xx, and
    23:xx, and with LIB$DT_FORMAT naming time first or only one part.
    Also flags 0, 1, 2, 3, and 4; a delta (ABSTIMREQ); a short output
    (STRTRU, date-length); and an undefined SYS$LANGUAGE against
    SYS$LANGUAGE FRENCH with no French table (ENGLUSED).
  - GET_MAXIMUM_DATE_LENGTH for the manual's example (27 or 28) and a
    few others.
  - GET_DATE_FORMAT for the manual's three formats and for none.
  - INIT_DATE_TIME_CONTEXT: each component, the unnamed codes,
    NUMELEMENTS, ILLINISTR (no closing delimiter, empty), UNRFORCOD,
    a mixed output element, LIB$K_LANGUAGE, and a context missing its
    month names.
  - The context and the names (subtask 5's choices): LIB$DT_FORMAT
    redefined between two calls on one context (read once, or each
    call); ENGLUSED and DEFFORUSE together; FREE of a zero cell and of a
    stale copy; INIT of a cell holding garbage.
- **Input** (CONVERT_DATE_STRING).
  - Table 3-2's eight strings, with flags 0x7F.
  - The default flags with "-MAR" (INCDATTIM).
  - "feb 25, 1988 04:39:02:55 am" against the manual's 12-hour format.
  - "JU" (AMBDATTIM), "31" in February (IVTIME), YESTERDAY, TODAY,
    and TOMORROW.
  - A *defaults* vector, and defaulted-fields.
  - ILLFORMAT formats: too few fields, mixed groups, a meridiem with a
    24-hour clock, and a weekday.

Values that vary from run to run (the current time, a context's
address) print on lines marked with a tilde, as in Phase 55's probe.

## Subtasks (provisional)

1. **Survey**: chapter 3 and each routine's entry; the operation codes;
   the format mnemonics. **Done** 2026-10-13.
2. **Arithmetic**: ADD_TIMES, SUB_TIMES, MULT_DELTA_TIME, MULTF_DELTA_TIME.
   **Done** 2026-10-13.
3. **Conversion**: CVT(F)_TO/FROM_INTERNAL_TIME, CVT_VECTIM.
   **Done** 2026-10-13.
4. **Simple**: DATE_TIME, DAY, DAY_OF_WEEK.
   **Done** 2026-10-13.
5. **Format context and logical names**: INIT/FREE_DATE_TIME_CONTEXT, the
   names' definitions. **Done** 2026-10-13.
6. **Output**: FORMAT_DATE_TIME, GET_DATE_FORMAT, GET_MAXIMUM_DATE_LENGTH.
   **Done** 2026-10-13.
7. **Input**: CONVERT_DATE_STRING. **Done** 2026-10-13.
8. **The program** (`testdata/lib56/`), its console test, and the VMS
   probe. **Done** 2026-10-13 (two VMS runs), but for the open
   CONVERT_DATE_STRING question in the progress log.
9. **Close-out**.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
- **2026-10-13** — Subtask 1, the survey ("The survey" above). I read
  the 5.0 LIB$ manual's chapter 3 and all 17 entries, the VSI manual's
  entries for the same routines, and VMS 7.3's LIB$K_ and LIB$M_ values
  in `vmsdef.Symbols`. Findings that shape the plan:
  - All 17 routines are in LIBRTL's vector (none in LIBRTL2), and no
    other date or time entry is missing.
  - The manuals give no code values. VMS's definitions have them,
    including LIB$K_LANGUAGE (a VSI component the 5.0 manual lacks)
    and the MERIDIEM spelling. The component codes are spaced three
    apart, and govax refuses the unnamed ones.
  - The two manuals disagree in places, all probe cases: the 0- or
    1-based ranges of the time-of-day operations; CVT_TO's limits;
    SUB_TIMES of equal times (VSI: a 100 ns delta); and LIB$DAY's
    INTOVF year (8600 on the VAX, which fits a division by 10⁹ ticks).
    GET_MAXIMUM_DATE_LENGTH's example says 28 for a 27-character
    string.
  - The OCR spells the zero in mnemonics as O. VSI confirms D0, M0,
    S0, H04, and H02, and the survey's tables of the 60 predefined
    formats use the zero spelling, with the blanks corrected from the
    examples.
  - CVT_VECTIM needs the inverse of corevms's `numericTime`, and
    CONVERT_DATE_STRING a format-driven parser: neither $BINTIM's nor
    F$CVTIME's parsing fits.
  - The formatting routines never return LIB$_REENTRANCY: a deliberate
    difference, like Phase 55's INSEF.
  - Two questions for the author (Open questions): the predefined
    names are proposed built in, not in `vax.init`; and the date
    routines are proposed to read SYS$LANGUAGE only, without
    GET_USERS_LANGUAGE's `vax.language` and host-locale fallbacks.

  No code changed.
- **2026-10-13** — The author agreed to both proposals: the predefined
  format names are built in, and the date routines read SYS$LANGUAGE
  only (Open questions).
- **2026-10-13** — Subtask 2, the arithmetic (`internal/librtl/times.go`,
  shim codes 131–134). ADD_TIMES, SUB_TIMES, MULT_DELTA_TIME, and
  MULTF_DELTA_TIME follow the survey's rules. These are govax's
  choices, each marked unconfirmed in the code and left for the probe:
  - every delta, given or computed, must be under 10,000 days
    (LIB$_IVTIME), including the interval between two absolute times;
  - equal times subtract to the one-tick delta, as VSI says;
  - absolute − delta moves the time earlier, and a result before the
    base date is LIB$_NEGTIM;
  - an absolute time to multiply is IVTIME;
  - a multiplier of 0 stores 0;
  - MULTF's product is rounded to a tick, half away from zero, and a
    reserved multiplier signals SS$_ROPRAND;
  - any argument count but the format's is LIB$_WRONUMARG;
  - nothing is stored on an error.

  Tested in `times_test.go` and, through a linked program's stubs, by
  `TestLibrtlTimeArithmetic` (`internal/console/librtl_times_test.go`).
- **2026-10-13** — Subtask 3, the conversions
  (`internal/librtl/cvttime.go`, shim codes 135–139; a file of their
  own rather than `times.go`, which the survey planned).
  CVT_FROM_INTERNAL_TIME, CVTF_FROM_INTERNAL_TIME, CVT_TO_INTERNAL_TIME,
  CVTF_TO_INTERNAL_TIME, and CVT_VECTIM follow the survey's rules. The
  operation codes are one table (`timeOperations`, by LIB$K_ value): an
  absolute operation's field from a `calendar` of the time, a delta
  operation's unit in ticks. CVT_VECTIM's work is `corevms.VectorTime`
  (`vectorTime`, beside `numericTime`, whose output it takes back:
  `TestVectorTime`). These are govax's choices, each marked unconfirmed
  in the code and left for the probe:
  - the year, month, and week operations count from 1 (hour 1 of the
    year is 00:00–00:59 on 1 January), and the day, hour, and minute
    operations from 0, as VSI says;
  - JULIAN_DATE is days since 17-Nov-1858, LIB$DAY's number;
  - CVT_FROM_INTERNAL_TIME takes an input-time of 0 by value as the
    current time, and refuses the floating codes (INVOPER);
  - CVT_TO and CVTF_TO accept 0 units, storing 0, as the 5.0 manual
    allows (VSI asks for more than 0), and CVTF_TO rounds to a tick, half
    away from zero, signaling SS$_ROPRAND for a reserved operand, as
    MULTF_DELTA_TIME does;
  - CVT_VECTIM's limits: years 1858 (from 17 November) to 9999, a year
    of 0 with a nonzero month IVTIME, and seven zeros a zero delta,
    stored as 0.

  A note for the probe: a delta in F-floating units isn't exact unless
  the unit divides it by a power of two (5 hours is 5/24 of a day), so a
  round trip through CVTF_FROM and CVTF_TO can come back a fraction of a
  millisecond short. Tested in `cvttime_test.go` and, through a linked
  program's stubs, by `TestLibrtlTimeConversion`
  (`internal/console/librtl_times_test.go`).
- **2026-10-13** — Subtask 4, the simple routines
  (`internal/librtl/days.go`, shim codes 140–142): DATE_TIME, DAY, and
  DAY_OF_WEEK, returning SS$_NORMAL. DATE_TIME writes $ASCTIM's
  23 characters (`corevms.FormatTime`) by the output's class, with
  LIB$_STRTRU when they don't fit. DAY and DAY_OF_WEEK take the current
  time when the time is omitted or 0 by value, and the base date for a
  zero quadword. These are govax's choices, each marked unconfirmed in
  the code and left for the probe:
  - a delta is divided as EDIV divides, truncated toward zero: two days
    and an hour give a day count of −2 and a day-time of −360,000
    hundredths;
  - DAY's SS$_INTOVF is when the time divided by 10⁹ ticks doesn't fit
    a longword (from 25-Dec-8663, and a delta as long), with nothing
    stored;
  - DAY_OF_WEEK counts from the day count, day 0 a Wednesday, so a
    delta of 3 days is a Sunday and one under a day a Wednesday.

  Tested in `days_test.go` and, through a linked program's stubs, by
  `TestLibrtlDays` (`internal/console/librtl_times_test.go`).
- **2026-10-13** — Subtask 5, the context and the names.
  - **The names.** `internal/lnm/dtformat.go` holds the 60 predefined
    formats (`DateFormats`, `TimeFormats`), and `NewDatabase` makes
    LNM$DT_FORMAT_TABLE from them: shareable, under the system directory,
    executive mode, with executive-mode LIB$DATE_FORMAT_nnn and
    LIB$TIME_FORMAT_nnn names. A user can add their own with DEFINE/EXECUTIVE
    /TABLE=. The table now shows in SHOW LOGICAL/STRUCTURE
    (`TestCreateNameTable`).
  - **The mnemonics** (`internal/librtl/dtformat.go`). One table holds
    each mnemonic's field, fill, case, abbreviation, and digits.
    `parseDateFormat` matches the longest code after each `!`, and
    `partOf` says whether a format gives the date, the time, both, or
    neither. Subtasks 6 and 7 build their engines on these items.
  - **The context** (`internal/librtl/dtcontext.go`, shim codes 143 and
    144). A heap block tagged "DTCX" (8 bytes) whose state is kept in Go
    per image, found by the block's address; without a user-context, the
    image's static context. The components are a table (`dtComponents`)
    with English's spellings built in. `dateContext.settings` gives what
    the engines will use: three cases of each spelling, the output
    format's elements in order, the input format, and the ENGLUSED and
    DEFFORUSE warnings (`outputWarning`, `inputWarning`).
  - These are govax's choices, each marked unconfirmed in the code and
    left for the probe:
    - an uninitialized context reads the names on its first use and
      keeps them;
    - an undefined SYS$LANGUAGE is ENGLUSED, and an undefined
      LIB$DT_FORMAT or LIB$DT_INPUT_FORMAT is DEFFORUSE. ENGLUSED wins
      when both apply;
    - a language's table supplies each spelled component as _U, _L, and
      _C names (LIB$FORMAT_MNEMONICS as one name), and a component the
      table lacks is English's, with ENGLUSED;
    - INIT's spellings are given once, and the three cases are derived
      from them;
    - an initialized context takes the spellings it wasn't given from
      its LIB$K_LANGUAGE (else English), and the standard format, with
      DEFFORUSE, for a format it wasn't given;
    - INIT checks its string before touching the context, so a failed
      call allocates nothing;
    - an output element that gives neither part or both, two elements of
      one part, or an empty input format is ILLFORMAT. An empty output
      element is a part left out;
    - a LIB$K_LANGUAGE of blanks is ILLINISTR, and a one-character
      init-string is too;
    - INIT with a cell that names no context is LIB$_INVARG. FREE of one
      is LIB$_BADBLOADR, and FREE of a zero cell does nothing;
    - INIT without a user-context sets the static context;
    - mnemonics are upper case only.

  Tested in `dtformat_test.go`, `dtcontext_test.go`, and lnm's
  `TestDTFormatTable`, and, through a linked program's stubs, by
  `TestLibrtlDateContext` (`internal/console/librtl_times_test.go`).
- **2026-10-13** — Subtask 6, the output routines
  (`internal/librtl/dtoutput.go`, shim codes 145–147):
  LIB$FORMAT_DATE_TIME, LIB$GET_MAXIMUM_DATE_LENGTH, and
  LIB$GET_DATE_FORMAT, over subtask 5's settings and parsed formats.
  - **The engine.** A time is broken into its fields once (`fieldsOf`:
    $NUMTIM's, with the fraction in whole ticks for !C7, and the
    weekday), and each item of an element is written by its mnemonic
    (`formatItem`); the elements are joined by one blank. All 60
    predefined formats write the manual's examples for 13-Jan-1988
    09:13:25.14 (`TestPredefinedFormatsOutput`).
  - **The widest text** (`maximumLength`): each number at two digits
    whatever its fill, a year or fraction at its digits, a spelled field
    at its longest spelling in its case, and the blank between parts.
  - **The legible input format** (`legibleFormat`) uses the context's
    LIB$K_FORMAT_MNEMONICS spellings; the manual's three examples come
    out as it shows them.
  - **The input format's rules** (`checkInputFormat`, in `dtformat.go`,
    for subtask 7 too): the seven fields once each, one month and one
    hour, a meridiem exactly with the 12-hour clock, and the date's and
    time's fields in two groups. LIB$GET_DATE_FORMAT returns
    LIB$_ILLFORMAT for a format that breaks them.
  - These are govax's choices, each marked unconfirmed in the code and
    left for the probe:
    - a year shows its last n digits, zero-filled; a fraction is
      truncated; the 12-hour clock shows 00:xx as 12; AM is before noon;
    - with flags, the format's own parts come out in its order, and a
      part it lacks is the standard one (the date first, the time last),
      with LIB$_DEFFORUSE; other bits are ignored, and flags of 0 write
      nothing;
    - date-length is the bytes written (11 when a 23-byte text is cut to
      11, with LIB$_STRTRU), and a delta writes nothing (LIB$_ABSTIMREQ);
    - LIB$GET_MAXIMUM_DATE_LENGTH gives 27 for the manual's example, which
      says 28;
    - the legible format's runs of blanks and tabs become one blank, and
      none is kept at the ends;
    - LIB$GET_DATE_FORMAT takes one or two arguments (LIB$_WRONUMARG
      otherwise), and an omitted format-string is LIB$_INVARG;
    - an input format with a weekday, or a field twice, is
      LIB$_ILLFORMAT.

  Tested in `dtoutput_test.go` and, through a linked program's stubs, by
  `TestLibrtlDateOutput` (`internal/console/librtl_times_test.go`).
- **2026-10-13** — Subtask 7, the input routine
  (`internal/librtl/dtinput.go`, shim code 148):
  LIB$CONVERT_DATE_STRING, over subtask 5's settings and subtask 6's
  `checkInputFormat`.
  - **The matcher** (`dtMatcher`). The string, its outer blanks
    trimmed, matches one contiguous run of the input format's items,
    every field in the run read or omitted, and everything outside the
    run omitted. A field may also be left out with the punctuation
    before it. The reading that leaves out the least punctuation wins,
    then the earliest run, then the longest value. This reads all of
    Table 3-2 as the manual shows it ("23:" is an hour, "-1988" a year,
    "::23" seconds), and "4:39 pm" by a format with seconds and a
    fraction. Numbers take one digit up to their most (two, or a year's
    or fraction's digits), longest first, so "!Y4!MN0!D0" reads
    "19880113". A blank-filled field may begin with blanks. A fraction
    is a decimal fraction of a second, as $BINTIM reads one. Month names
    and meridiems match in full or abbreviated, case aside. An
    abbreviation of two names ("JU") is LIB$_AMBDATTIM when nothing else
    matches.
  - **Defaults** (`complete`). An omitted field takes the defaults
    vector's word, else today's date or 0. Its bit is set in
    defaulted-fields, and a bit the flags lack is LIB$_INCDATTIM. The
    time is made by `corevms.VectorTime` plus the fraction's ticks, so
    31 February is LIB$_IVTIME, as is a string that matches nothing.
  - These are govax's choices, each marked unconfirmed in the code and
    left for the probe:
    - the matching rules above: contiguous runs, the least punctuation
      left out, then the earliest run;
    - a year of fewer than four digits takes the rest of its digits from
      the default year: "88" read in 2026 is 2088;
    - a 12-hour hour without a meridiem is before noon, and a meridiem
      without an hour leaves the default hour unchanged;
    - a relative day name (YESTERDAY, TODAY, TOMORROW, abbreviated as
      far as unique, "T" ambiguous) must be the whole string. Its time
      fields are omitted (defaulted-fields 0x78), so flags of 0 make it
      LIB$_INCDATTIM;
    - flag bits past the seventh are ignored;
    - two to six arguments, else LIB$_WRONUMARG; an omitted date-string
      or date-time is LIB$_INVARG;
    - nothing is written on an error;
    - explicit punctuation matches case aside ("KL" for "kl").

  Tested in `dtinput_test.go` (Table 3-2, the flags, other formats,
  defaults, relative days, the routine's arguments) and, through a linked
  program's stubs, by `TestLibrtlDateInput`
  (`internal/console/librtl_times_test.go`).
- **2026-10-13** — Subtask 8, the program (`testdata/lib56/`, README.md)
  and its oracle, `TestLib56Oracle` (`internal/console/lib56_test.go`),
  which skips until VMS's output is in `vax/`.
  - **The program.** `gen.go` writes `lib56.mar` (395 cases),
    `lib56.com`, and `exchange.cmd`, after Phase 55's model. What's new
    is a time shown as `$ASCTIM` writes it, and logical names the
    program defines itself with `$CRELNM` (LIB$DT_FORMAT,
    LIB$DT_INPUT_FORMAT, SYS$LANGUAGE in the process table), a case at a
    time, each with a fresh context cell.
  - **The cases** follow the survey's "For the probe":
    - the arithmetic's edges;
    - every CVT_FROM_INTERNAL_TIME operation on 13-Jan-1988 09:13:25.14
      and on the first and last moments of 1988;
    - the conversions' limits, and CVT_VECTIM's fields one past their
      ranges;
    - DATE_TIME into each class;
    - DAY's zero rules, its deltas, and the INTOVF boundary;
    - each of the 60 predefined formats at 00:05, 09:13, 12:30, and
      23:45;
    - the names and their fallbacks, the flags, and INIT's components
      and errors;
    - every mnemonic, GET_MAXIMUM_DATE_LENGTH's 27 or 28, and
      GET_DATE_FORMAT's examples;
    - CONVERT_DATE_STRING on Table 3-2 (its current date from a
      defaults vector), the 12-hour format, abbreviations, ILLFORMAT's
      formats, and the relative days (shown as their difference from
      TODAY's result).
  - **`lib56.com`** shows the names VMS defines: LNM$DT_FORMAT_TABLE's
    predefined formats and any language tables.
  - **Fixing the program.** govax's run turned up four mistakes in the
    program before it went to VMS: Go's `time.Duration` can't span the
    years to 9999; a case whose output was longer than the 132-byte line;
    a VS string's CURLEN shown in a longword with text that varies; and
    lengths and differences that depend on the date. All are fixed in
    `gen.go`.
- **2026-10-13** — Subtask 8, the VMS runs (`testdata/lib56/README.md`,
  "The runs"). Run 1 hung after 301 cases. The 302nd was an INIT of a
  cell holding ^X200, the image's first page, which VMS took for a
  context; that case is dropped. Run 2 ran all 426 cases of round 2,
  after `lib56.com` ran VMS's LIB$DT_STARTUP. govax now matches run 2
  line for line, except for the CONVERT cases below. What VMS V7.1
  settled, and govax now does:
  - **Arithmetic.**
    - There is no 10,000-day limit on deltas anywhere: ADD, SUB, MULT[F],
      CVT_FROM/TO, and CVT_VECTIM all take and make longer ones.
    - ADD_TIMES is plain quadword arithmetic, wrapping (two deltas of
      2^62 ticks make ^X80000000 00000000). CVT_VECTIM wraps too
      (31-Dec-65535 comes out as 3-Aug-7080), and accepts years past
      9999.
    - A zero multiplier (MULT and MULTF) is LIB$_IVTIME, as is a count of
      0 for CVT[F]_TO.
    - MULTF rounds the length up: one tick times 0.5, 1.4, 1.5, and 2.5
      is one, two, two, and three ticks.
    - CVTF_TO follows `floatTicks`, a model fitted to VMS's six results
      and marked as such. The whole seconds are kept as a longword
      (wrapping), and the fraction of a second is multiplied by 10^7,
      rounded to F_floating, and truncated. Real MACRO assembles the
      probe's F_floating literals with the same bits as govax (checked
      in the VAX's LIB56.OBJ), so the model is about the routine, not
      the data.
    - $ASCTIM writes a year past 9999 as `****` (corevms).
  - **Conversion.**
    - Days, months, and weekdays count from 1, and hours, minutes, and
      seconds from 0, of the year, month, week, day, hour, and minute
      alike. So 00:30 on 1 January is hour 0 of the year.
    - JULIAN_DATE is LIB$DAY's day number.
    - CVT_FROM with an absolute time and any code but 1–20 is
      LIB$_DELTIMREQ (0 and 31 included). With a delta, 1–20 is
      LIB$_ABSTIMREQ, and anything else LIB$_INVOPER.
  - **LIB$DAY** of a delta gives a negative day count and a positive
    day-time.
  - **The names.** VMS defines LIB$DT_FORMAT and LIB$DT_INPUT_FORMAT in
    the system table as the standard format, so the oracle's console
    does too. A context INIT left a component out of takes it from the
    names (or the language from SYS$LANGUAGE), not from defaults.
    LIB$DT_FORMAT's first two names are the elements, a third is
    ignored, and a name that doesn't translate makes the whole format
    the standard one, with DEFFORUSE.
  - **INIT** checks little:
    - it allocates the context first, so the cell is set even when the
      call fails;
    - it returns LIB$_ILLCOMPONENT, LIB$_INVSTRDES (no init-string),
      LIB$_ELETOOLON (an empty string, an element over 255 bytes, or an
      output format of the delimiter alone), LIB$_ILLINISTR, or
      LIB$_NUMELEMENTS (too few elements; extras are ignored);
    - it doesn't check mnemonics or fields: those errors come when the
      format is used;
    - a user-context of 0 is an access violation.
    Spellings are used as given, whatever case the mnemonic asks for,
    and mnemonics are read in either case.
  - **Output.**
    - The first element is the time part if its first field is a time
      field, else the date part, and the second element is the other
      part, whatever its fields. An element mixing date and time fields
      is allowed.
    - With flags, a chosen part the format lacks is the standard one,
      appended, with no warning. An empty first element writes nothing,
      and an empty second element writes the standard format's part.
    - An unknown mnemonic is LIB$_UNRFORCOD, and GET_MAXIMUM_DATE_LENGTH
      writes 0 for it.
    - The manual's example is 27 characters, not 28.
  - **Input.**
    - A weekday in an input format is LIB$_UNRFORCOD.
    - A two-digit year takes its century from the current year, not
      from the defaults vector.
  - **CONVERT_DATE_STRING (open).** VMS returned SS$_ROPRAND as a status
    in 67 cases: nearly every string with a letter in it, or beginning
    with punctuation. That includes Table 3-2's own "-MAR", ":45:", and
    ".01", and YESTERDAY, TODAY, and TOMORROW. One case, a packed format
    after them, returned LIB$_BADBLOADR. Strings of digits that begin
    with a digit ("21", "23:", "13-01-1988", "01/13/88 09:13:25.14")
    converted as govax converts them. This looks like a fault inside the
    routine or in simh's VAX 8600, not a rule to follow, so govax keeps
    the manual's behavior, and `TestLib56Oracle` lists the 69 cases
    (`expectedLib56Cases`, by reason). A run on another simh model, or
    another VMS version, would tell which.
