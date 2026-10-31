# Phase 80 — FORTRAN formatted and list-directed I/O

**Status:** in progress: subtask 1 (the survey, Part A) done 2026-10-31,
accepted by the author the same day; subtask 2 (the probe) done the same day, two
rounds on VMS 7.1; subtask 3 (FORMAT's check, every statement through the
compiler, FTNRTL's statement calls) done the same day. Needs Phase 79 (control flow, for
implied DO loops and ERR/END branches). Part of the FORTRAN program laid
out in `PHASE-74 - FORTRAN plan.md` (Decisions 12, 13, and 15).

## Goal

Everything about turning values into text and text into values, on the
terminal and in internal files:

- **FORMAT** and every edit descriptor in chapter 8: I, F, E, D, G, L,
  A, H, character constants, X, T, TL, TR, `/`, `:`, `$`, Q, P scale
  factors, BN and BZ, S, SP, SS, O and Z, repeat counts and nested
  groups, reversion, variable format expressions (`<n>`), and default
  field widths (Table 8-4);
- **run-time formats** in character variables, arrays, and expressions;
- **list-directed** input and output in full (7.2.1.2; Table 7-1's output
  formats are Phase 78's): value separators, repeat counts (`3*5`), null values, slash termination,
  complex and character input;
- **NAMELIST** input and output (4.10, 7.2.1.3, 7.3.1.3);
- **internal files** (READ and WRITE to CHARACTER variables and arrays),
  and ENCODE and DECODE (appendix A.1);
- **the terminal statements:** READ with a format and no unit, ACCEPT,
  TYPE, PRINT, and units 5 and 6 (to SYS$INPUT and SYS$OUTPUT);
- **carriage control** for formatted output (8.4, Table 8-5);
- **the error model:** ERR=, END=, IOSTAT=, the FOR$_ conditions
  signaled when there are none, the FOR message texts (Decision 15),
  and ERRSNS (D.4.3).

## What earlier phases leave in place

- **Phase 75:** FTNRTL's list-directed output, the I/O call protocol,
  the unit table, SYS$OUTPUT.
- **Phase 78:** list-directed output of every type (its Part A's
  corrections, accepted 2026-10-30).
- **LIBRTL:** OTS$CVT_T_F/D/G/H (text to floating), OTS$CVT_TI_L,
  TO_L, TZ_L, TL_L (text to integer), OTS$CVT_L_Tx, and
  `OTS$$CVT_x_T_R8` (floating to digits with FORTRAN's rounding).
- **RMS's FORTRAN carriage control** (`rms/ftncc.go`, Phase 72).

## Design notes

- **The format interpreter** is MACRO in FTNRTL: it parses a format's
  text into a compact list of operations once (cached in the FORMAT's
  control block, Decision 12) and walks it as items arrive. The
  compiler checks a FORMAT statement's syntax at compile time, so a
  malformed FORMAT is a compile-time error; a run-time format is parsed
  when used.
- **Item calls drive the format:** each `FTN$IO_ITEM` advances the
  format to the next data edit descriptor, doing the positional and
  literal ones on the way; `FTN$IO_END` finishes it (to a colon or the
  end).
- **Variable format expressions** (`<n>`) need the program's values at
  run time: the compiler emits a small routine per format that FTNRTL
  calls to evaluate them, the one place FTNRTL calls back into compiled
  code.
- **Errors:** FTNRTL records the first error in the statement's context
  and makes later item calls no-ops; `FTN$IO_END` returns it (with ERR,
  END, or IOSTAT) or signals it. With END= in an input statement with an
  implied DO, the compiled loop tests the status after each item.
- **FOR messages** go in a hand-written `internal/vmsdef` table, in our
  words, with FAO arguments for the unit and file.

## Open questions

*Since the survey, each goes to the probe (Part A, subtask 2).*

- **The exact text of the FOR messages** and their FAO arguments: a
  `$GETMSG` loop in the probe gives VMS's; Part A's question 2 asks whether
  to use them.
- **List-directed output's line length** and where records break: the trials
  show 80 characters of text, with the carriage control besides.
- **G editing** (Table 8-2) and E's exponent forms at large exponents: the
  trials settle G and E but for exponents past 99.

# Part A — the survey (2026-10-31)

Section numbers are the 1988 *VAX FORTRAN Language Reference Manual*'s (the
LRM). As in Phases 78 and 79, what a rule means for a pass is in brackets.
In the examples `_` is a blank; the manual's OCR garbles many of its
examples' blanks, and those here were re-derived from the rules and, where
marked, checked against FORRTL.

## A new source: FORRTL as a black box

**The author's ruling (2026-10-31):** a program may call VMS's FORRTL.EXE
to verify behavior and to create or validate requirements. Its code may not
be copied. This changes the Phase 74 line "FORRTL.EXE is not studied as a
model": its *behavior* may be, its *code* not, and FTNRTL stays our own
design. It settles most of what would have been marked unconfirmed, since
the VAX FORTRAN User Manual is missing and the simh VAX has no compiler.

**What can be called.** Trials on govax, with VMS 7.1's FORRTL.EXE linked
from the local system disk (2026-10-31, scratch programs not kept):

| Entry | Arguments found by trial | Status |
|---|---|---|
| `FOR$WRITE_SO`, `FOR$READ_SO` | unit by value; the address of the format's text, ending in a NUL | works |
| `FOR$WRITE_SL`, `FOR$READ_SL` | unit by value | works |
| `FOR$READ_IO` | descriptor of the internal file; format's address | works |
| `FOR$WRITE_IO`, `FOR$ENCODE_MO` | as READ_IO; count by value, format's address, buffer's address | reached the format: the one trial's format was 21 bytes for a 20-byte record, and each gave OUTSTAOVE, so no output was seen yet |
| `FOR$IO_L_V`, `_F_R`, `_D_R`, `_FC_R`, `_DC_R`, `_LU_V`, `_LU_R`, `_WU_R`, `_T_DS`, `_L_R` | a value, a reference, or a descriptor | work |
| `FOR$IO_END` | none | works |
| `FOR$READ_IL`/`WRITE_IL`, `FOR$DECODE_MO`, `FOR$ERRSNS` | internal record's descriptor; as ENCODE; five addresses | work (`testdata/for80`) |
| `FOR$READ_SN`/`WRITE_SN` | unit by value; a namelist block, found by trial (the author's answers, below) | WRITE works; READ needs `LIB$TPARSE`, which govax lacks |

The `_SO` entries are the key: an "object-time" format is plain text, so a
MACRO program can put any format to FORRTL, without knowing how a compiler
encodes one (the `_SF` entries'). `FOR$READ_SO` reproduced the manual's short-field example from a terminal
line
(`1,-2,1.0,35` under `(2I6,2F10.2)` gave 1, -2, 1.00, 0.35).

**How far to trust the trials.** FORRTL ran on govax, so it was calling
govax's LIBRTL (`OTS$$CVT_x_T_R8`, `OTS$CVT_L_TI`, RMS). The results below
are FORRTL's logic over govax's support, and are marked **[trial]** until a
VMS run confirms them (subtask 2). One thing visibly went wrong under govax:
list-directed integers came out as a blank field with OUTCONERR, though
`OTS$CVT_L_TI` is called with the documented arguments, and `(1X,I)` of the
same value works. (Since explained, and fixed: "List-directed integers
explained", after the questions.)

## Where the compiler stands

A program of every form was compiled with `govax fortran --no-macro`.

**Compiled today:** PRINT \* and WRITE (\*,\*) with a list of values and
whole arrays (FTN$PRINT_LIST, FTN$WRITE_LIST, FTN$IO_ITEM, FTN$IO_END).

**Parsed, and reported NOTIMPL:**

| Form | Reported by |
|---|---|
| FORMAT, NAMELIST | `handled` (`sema/notimpl.go`) |
| TYPE, ACCEPT, READ of any form | `handled` |
| ENCODE, DECODE | `handled` |
| a labeled format, or a format expression | `listDirected` (`sema/exec.go`) |
| a unit other than \*; unformatted; REC=; ERR=, END=, IOSTAT=; NML= | `executable` (`sema/exec.go`) |
| an implied-DO list | `formWhat` (`sema/notimpl.go`) |
| ASSIGN of a FORMAT label | `checkLabels` (`sema/labels.go`) |
| an assumed-size array in a list | NOSIZE, an error, and it stays one |

The parser needs little: `ast.FormatStmt.Text` is the specification as
written, `ast.FormatSpec` is `*`, a label, or an expression, a namelist named
by position is the `Format`, `ast.CodeStmt` is ENCODE and DECODE, and
`ast.ImpliedDo` is ready. [`parseFormat` takes the text from the lexer's
`Body`, so blanks in constants survive. Phase 76's note stands: a constant
right after a name or digit, `1X'ABC'`, confuses the `u'r` and `!` scans
(`source/comment.go`), to be settled with the probe's answer on whether
that form is legal at all.]

**FTNRTL today:** `ftn_listout.mar` is one static record of 80 bytes and
one statement at a time; `ftn_units.mar` is one unit, SYS$OUTPUT, with
LIB$STOP on any RMS error; `FTN$IO_ITEM` knows only output.

**VMS's side, already done:** RMS's FTN carriage control (`rms/ftncc.go`,
which the terminal statements' subtask only uses), the terminal's input entry takes PUT (Phase
72), `OTS$CVT_T_x`, `OTS$CVT_Tx_L`, `OTS$CVT_L_Tx`, `OTS$$CVT_x_T_R8`, and the
FOR$\_ condition values (`vmsdef.LibrarySymbols`). There are no FOR
message texts: today the condition prints `%NONAME-F-NOMSG`.

## The rules of the I/O statements (chapter 7)

**Control lists (7.1.1).** Done in the parser. A unit is an integer
expression, 0 to 99, or \*, or a CHARACTER scalar or array, an internal file.
[Sema tells them apart by type. A number outside 0 to 99 is a run-time
INVLOGUNI, as a unit is an expression.] A format is a label, an integer
variable assigned a FORMAT's label, an array or element holding a run-time
format, or a character expression; a namelist is a group's name. [`format`
in sema decides, in this order: `*`, label, namelist name, character value,
array name or element, integer scalar. A scalar of another type is an error.]

**IOSTAT (7.1.1.9):** zero, positive for an error, *negative* for end of
file. [Corrects Decision 15, which says `FOR$_ENDDURREA gives 24`: IOSTAT is
-1 at end of file, and 24 is ERRSNS's number. The -1 is ours, as the manual
says only "negative", and a probe may confirm it only if FORRTL's IOSTAT
convention can be called.]

**ERR= and END= (7.1.1.10):** END= is for a sequential or internal READ only
[sema: an error elsewhere; `transferSpecs` already keeps WRITE from it]. An
end of file goes to END=, else ERR=, else (with no IOSTAT) the program ends.
An error goes to ERR=; with neither ERR= nor IOSTAT it ends. [The labels are
jumps, checked by `refs` as any GO TO's, into the current block only.]

**I/O lists (7.1.2).** Output: any expression. Input: variables, elements,
substrings, whole arrays, implied-DO lists, and the subscripts may use
values read earlier in the list (`READ (1,1250) J,K,ARRAY(J,K)`). [So each
item's address is computed at its own call, as the IR's sequence of item
calls already does; a constant or expression read into is a new error.] A
function in an output list "must not attempt any further I/O operations on
the same logical unit": I/O on *another* unit, or an internal file, inside a
list's function is legal. [FTNRTL needs a stack of statements (Decision 6).
FOR$\_RECIO_OPE, 40, is the same unit's error.] Records in lists are
unformatted only (Phase 81).

**Implied-DO (7.1.2.2):** a DO's variable and parameters, each item called
in turn. After an abnormal end the variable is undefined. [Lowering reuses
`ir`'s DO count; Decision 7 drops the design note's status test.]

**Statements:** READ (sequential formatted, list, namelist; internal
formatted and list), `READ f`, ACCEPT, TYPE, PRINT, WRITE. Unformatted,
direct, keyed, and REWRITE are Phase 81's. Namelist isn't permitted with an
internal file [NMLINTERNAL, new].

**List-directed input (7.2.1.2).** A value is a constant of the item's kind
(a numeric item takes only a numeric constant, converted by Table 3-1; a
character item only an apostrophe-delimited one, `''` an apostrophe), a null
(`,,`, an initial or trailing comma, unchanged item, a whole complex but not
a part), `r*c`, or `r*`. Separators: blanks, tabs, comma, and slash, which
ends the statement. A record's end is a blank, except inside a character
constant, which continues with the next record's first character; leading
blanks aren't skipped there. Complex: `(a,b)`, blanks allowed around the
parts. Logical: `.TRUE.`, or anything beginning `T`, `.T`, `t`, `.t`; `F`
likewise. Hollerith, octal, and hexadecimal aren't permitted. Test case, the
manual's own (7-19), now also a probe case:

```
READ (1,*) I,R,D,E,L,M,J,K,S,T,C,A,B
4 6.3 (3.4,4.2), (3, 2)    T,F ,, 3*14.6 ,'ABC,DEF/GHI '' JK'/
```

gives 4, 6.3, (3.4,4.2), (3.0,2.0), .TRUE., .FALSE., J unchanged, K=14,
S=14.6, T=14.6D0, C=`ABC,DEF/GHI'JK`, and A and B unchanged. (Under govax,
FORRTL stopped this one with INPCONERR, 64, at the first complex item, after
reading I and R. Whether FORRTL or the trial's argument to `FOR$IO_FC_R` is
at fault is the probe's to say. Since the descriptor fix below it gives the
manual's values over govax.)

**List-directed output (7.3.1.2):** Table 7-1, Phase 78's. "Each output
record begins with a space", one or more complete records per statement, a
value whole in a record but for character values longer than a record and a
complex split after its comma. [See "What FORRTL shows" for three differences
from Phase 78's choices.]

**Namelist (4.10, 7.2.1.3, 7.3.1.3).** `NAMELIST /g/ list`: variables and
whole arrays, of any type, no dummy arguments, no elements or substrings.
Input: records up to one with `$` or `&` in column 2 and the group's name;
column 1 of every record ignored; `name = value[, ...]`, `name` a variable,
array, element, or substring (constants only as subscripts, no PARAMETERs);
ending `$` or `$END`. Values are as in list-directed input, a list filling
successive elements from the one named. Prompting: `?` lists the group's
entities, `=?` also their values, at a terminal. Output: ` $g`, then each
entity on a line of its own, `NAME = value,`, repeats as `3*0.0000000E+00,`,
character values in apostrophes, and ` $END`. The manual's example (7-38) is
a test case, garbled by the OCR in its columns. [Each group a table in
`$LOCAL`: names and descriptors, built by the compiler. An adjustable or
assumed-size array can't be in one, as dummies are refused.]

**Internal files (7.2.4, 7.3.4):** a CHARACTER scalar is one record, an
array's element each one record; formatted and list-directed only.
[Records are the elements' length: past the last is END=, 7.2.4's `end`, on
input and OUTSTAOVE-or-EOF on output, the probe's to say.]

**ENCODE and DECODE (A.1):** `ENCODE (c,f,b[,IOSTAT=ios][,ERR=s]) [list]`,
one record of c bytes; b a scalar or array of any type, its elements in
order; "if less than c characters are received, the remaining character
positions are filled with blank characters"; more than one record is an
error [MORONEREC?]. Test case A-3: `DECODE (12,100,A) K` with `100
FORMAT (3I4)`, A=`123456789012`, gives 1234, 5678, 9012, and `ENCODE
(12,100,B) K(3),K(2),K(1)` gives `901256781234`.

**ERRSNS (D.4.3):** `CALL ERRSNS(fnum,rmssts,rmssty,iunit,condval)`, any
argument null, INTEGER\*2 or \*4, the saved error cleared after each call.
fnum is 0 with no error since. [Phase 83 has the other system
subroutines. An argument by reference can't say its length, so the compiler
must pick the routine: Decision 13.]

**Terminal statements and units.** ACCEPT reads, TYPE and PRINT write, the
implicitly connected units only. READ `f`, ACCEPT, and unit 5 reach
SYS$INPUT; TYPE, PRINT, WRITE (\*,f), and unit 6 reach SYS$OUTPUT. Phase 72
saw FORRTL's unit 5: a `$CREATE` of `SYS$INPUT:` (default `FOR005.DAT`, RAT
FTN) with PUT as well as GET, since a program may write unit 5, as DUNGEON
does. The default names (FOR$READ, FOR$PRINT, FORnnn) are Phase 81's.

## FORMAT and the edit descriptors (chapter 8)

**The specification (8.1, 8.2):** `FORMAT (q1 f1 s1 f2 s2 ... fn qn)`, and
always labeled [FMTLABEL, new]. `r`, `w` are 1 to 32767, `m`, `d`, `e` up to
255 (the manual says both "greater than zero" and "zero"; `I4.0` and `F8.0`
are accepted [trial]), never PARAMETER names; `nP` is -128 to 127, sticky through reversion. Repeat counts
only on I, O, Z, F, E, D, G, L, A, and groups. In the 8.1 rule `w.d` go
together, or neither. A statement with a list needs a data descriptor in
its format, or a run-time error (INFFORLOO, 60 [trial]).

**Table: every descriptor, with the manual's cases.** Each is a probe case;
output cases are written with FORMAT `(1X,d)` so the terminal shows them.

| Descr. | Item | Output, manual's cases | Input, manual's cases |
|---|---|---|---|
| Iw[.m] | integer, logical | I3 284 `284`; I4 -284 `-284`; I4 0 `___0`; I5 174 `__174`; I2 3244 `**`; I3 -473 `***`; I4.0 0 `____`; I4.2 1 `__01`; I4.4 1 `0001` | I4 `2788`; I3 `-26` -26; I9 `______312` 312; blank 0; over range an error |
| Ow[.m] | any | O6 32767 `077777`; O6 -32767 (word) `100001`; O2 14261 `**`; O4 27 `__33`; O8 10.5 `___41050`; O4.2 7 `__07`; O4.4 7 `0007` | O5 `32767`; O4 `16234` 1623; O3 `970` error |
| Zw[.m] | any | Z4 32767 `7FFF`; Z5 -32767 (word) `_8001`; Z2 16 `10`; Z4 -10.5 `C228`; Z3.3 2708 `A94`; Z6.4 2708 `__0A94` | Z3 `A94`; Z5 `A23DEF` A23DE; Z5 `95.AF2` error |
| Fw.d | real, complex part | F8.5 2.3547188 `_2.35472`; F9.3 8789.7361 `_8789.736`; F2.1 51.44 `**`; F10.4 -23.24352 `__-23.2435`; F5.2 325.013 `*****`; F5.2 -.2 `-0.20` | F8.5 `123456789` 123.45678; `-1234.567` -1234.56; `24.77E+2` 2477.0; F5.2 `1234567.89` 123.45 |
| Ew.d[Ee] | same | E9.2 475867.222 `_0.48E+06`; E12.5 same `_0.47587E+06`; E12.3 0.00069 `___0.690E-03`; E10.3 -0.5555 `-0.556E+00`; E5.3 56.12 `*****`; E14.5E4 -1.001 `-0.10010E+0001`; E14.3E6 0.000123 `_0.123E-000003` | E9.3 `734.432E3` 734432.0; E12.4 `__1022.43E-6`; E15.3 `52.3759663_____`; E12.5 `210.5271D+10` |
| Dw.d | same | D14.3 0.0363 `_____0.363D-01`; D23.12 5413.87625793 `_____0.541387625793D+04`; D9.6 1.2 `*********` | BZ,D10.2 `12345_____` 12345000.; D10.2 `___123.45_`; D15.3 `367.4981763D+04` |
| Gw.d[Ee] | same | G13.6: 0.01234567 `_0.123457E-01`; -0.12345678 `-0.123457____`; 1.23456789 `__1.23457____`; 12.3456789 `__12.3457____`; 123.456789 `__123.457____`; -1234.56789 `_-1234.57____`; 12345.6789 `__12345.7____`; 123456.789 `__123457.____`; -1234567.89 `-0.123457E+07` | as F |
| complex | two fields | 2F8.5 `_2.35472_3.45673`; `E9.2,', ',E5.3` 47587.222,56.123 `_0.48E+05, *****` | F8.5,F8.5 `1234567812345.67`; E9.1,F9.3 |
| nP | F, E, D, G | 1PE12.3 -270.139 `___-2.701E+02`; 1PE12.2 `____-2.70E+02`; -1PE12.2 `____-0.03E+04`; `' ',F8.2,2PF8.2,F8.2` of 25.0 x6 (8-27) | 3PE10.5 `___37.614_` .037614; `__37.614E2` 3761.4; -3PE10.5 `____37.614` 37614.0 |
| Lw | integer, logical | L5 .TRUE. `____T`; L1 .FALSE. `F` | T, t, .T, .t; F, f, .F, .f; blank false; other an error |
| A[w] | any | A5 `OHMS` `_OHMS`; `VOLTS`; `AMPERES` `AMPER` | Table 8-3 by item size; A6 `PAGE #` to CHARACTER\*1,3,6,8, LOGICAL\*1, INTEGER\*2, REAL, REAL\*8 (8-31) |
| nH, '...' | none | text | H replaces the descriptor's text; a constant is the same |
| BN, BZ | none | no effect | blanks ignored, or zeros, for this statement |
| SP, SS, S | none | I, F, E, D, G only | no effect |
| Tn, TLn, TRn, nX | none | position; a record's length from what's written: `'K=',I6,5X` is 9 bytes (8-35) | T7,A3,T1,A3 of `ABC___XYZ` (8-35); TLn stops at the first column |
| Q | int/log | item skipped | characters left in the record (8-37) |
| $ | none | see carriage control | ignored |
| : | none | ends if no items are left (8-38) | same |
| / | none | a record; `n` slashes, n-1 blank between, n at either end (8-39) | n-1 records skipped |

**Blanks (8.3):** leading ignored always; embedded and trailing as zeros or
ignored. **A unit with no OPEN is BLANK='ZERO', after OPEN 'NULL',** and BN
and BZ change it for the statement. [So Phase 80's terminal units are ZERO,
surprising but the manual's. Internal files aren't stated: a probe case.]

**Field separators (8.6).** A comma ends a numeric or L field shorter than w;
the d part stands; two commas, or a comma after a full field, a null field:
0, 0.0, or .FALSE. A, H, and constants take no comma, but a record's end ends
them, the rest blank. [The field reader's one loop.] **Case 8-40:**
`(2I6,2F10.2)`, `1,-2,1.0,35`, is 1, -2, 1.0, 0.35 ([trial] matches).

**Table 8-4, default widths**, is right as printed for every case tried
[trial]; REAL\*16's and BYTE's rows are probe cases: `(1X,I)` `__________42`, `(1X,O)`, `(1X,Z)`;
`(1X,L)` `_T`; `(1X,F)` of REAL `______3.2500000`; `(1X,E)`
`__0.3250000E+01`; `(1X,G)` `___3.250000____`; `(1X,D)` of REAL\*8
`___0.3250000000000000D+01`; `(1X,A)` of an integer, its bytes.
Field sizes depend on the item, so a parsed `I` keeps "default" for the
item's descriptor to supply. [REAL\*16 is w 42, d 33, e 3 by the table.]

**Variable format expressions (8.3.2):** `<expr>` anywhere an integer may
be, but H's count, and not in run-time formats; re-evaluated at each scan,
any expression, function calls and dummies included. [Decision 5.] Test case
8-9 is on the manual's page.

**Carriage control (8.4, Table 8-5).** blank, `0`, `1`, `+`, `$`, NUL, and
anything else as a blank, by RMS already. **`$` (8.3.12.2):** a blank first
byte, no return; `+`, starts at the previous line's end and stays; `0`, `1`,
ignored. [So FTNRTL rewrites the first byte: blank to `$`, `+` to NUL.]

**Run-time formats (8.7).** Character scalar, array (elements concatenated),
or expression; or a numeric array, element, or field. Recompiled at every
use, and data read into one's H isn't copied back. Test case 8-42.

**Format control (8.8).** One record read at the start of input. The item
descriptors each take one element; others take none. The walk stops at the
data descriptor an item needs, a `:` with no items left, or the end. At the
last `)` with items left: new record, reversion to the group of the next-to-
last `)` with its repeat count, or the start. **Test case 8-44,** four READs
of a ten-record file, is Phase 81's, needing a file; run with SYS$INPUT here.

## What FORRTL shows beyond the manual

All **[trial]**. Each becomes a probe case and a requirement.

**Numeric output.** One case disagrees with the manual: E10.3 of -0.5555 is
`-0.556E+00` in 8-21, and FORRTL over govax's digits gave `-0.555` at E12.3,
since REAL's nearest value is below the half. VMS's run says which stands.
The manual's complex example, `0.48E+06` for 47587.222, is its own slip for
`E+05`, which the table above uses.

| Case | Result | Rule |
|---|---|---|
| F2.1 of 0.5 | `.5` | the leading 0 goes if there's no room |
| F4.2 of -0.2 | `-.20` | same; F5.2 `-0.20` |
| F1.0 of 0.0 | `*`, OUTCONERR | a field with no room for `0.` |
| F6.1 of -0.01 | `___0.0` | no minus sign when the result shows zero |
| F5.1 of 2.25 and 2.35 | `__2.3` both | half up on the value's exact digits (2.35 in F is below) |
| E8.3 of 0.5555 | `.555E+00` | E's leading 0 goes too; 0.5555's F value is below the half |
| E10.3 of 0.0 | `_0.000E+00` | |
| E12.4E1 of 1E5, of 1E12 | `___0.1000E+6`, stars | an exponent too wide is OUTCONERR, not an abort |
| E5.0 of 1.0 | `*****` | |
| 4PE12.3 of 270.139 | `___2701.E-01` | n = d+1 gives no fraction |
| 5PE12.3, -3PE12.2 | stars, OUTCONERR | n must satisfy -d < n < d+2; the manual gives only the upper |
| G13.6 of 0.0 | `_0.000000E+00` | zero is E form: **confirms Phase 78's reading** |
| G13.6 of 0.09999999 | `_0.100000____` | the form is chosen after rounding |
| G13.3 of 999.6 | `____0.100E+04` | the same, upward |
| 2PG13.3 of 5.0, 5E8 | `_____5.00____`, `____50.00E+07` | P is suspended in the F form |
| G8.3 of 5.0, G6.3 | `5.00____`, `******` | |
| I1 of 0 | `0` | |
| O12, Z9 of -1 | `_37777777777`, `_FFFFFFFF` | |
| Z8, O12 of REAL 1.0 | `____4080`, `_______40200` | the item's longword as stored (F's word order), leading zeros dropped |

**Type checks at run time:** I5 of a REAL (`16720`, its bits as an integer), F8.2
of an INTEGER (`____0.00`, its bits as a real), and L5 of a REAL (`____F`) each write the field
and signal **FORVARMIS, 61, fatal**. I5 and F8.2 of CHARACTER are stars and
OUTCONERR, 63. The manual's "I7 of 29.812, not permitted: error" is this.

**Conditions seen:** OUTCONERR 63 (E: stars, and on), FORVARMIS 61 (F),
SYNERRFOR 62 (F), INFFORLOO 60 (F: `(1X,'A')` with an item left),
INPCONERR 64 (F), OUTSTAOVE 66 (F: more than an internal record holds).

**Syntax at run time:** `(1X,I3,,I3)` is accepted; `(1X I3 I3)` and
`(1XI3I3)` are SYNERRFOR, so a comma isn't optional between two descriptors
(the manual's "generally"); blanks are ignored inside a descriptor, `( 1 X ,
I 3 )`; lowercase is accepted; `()` is one empty record; `0I3` and `J3` and
a missing `)` are SYNERRFOR; `I0` is OUTCONERR, not a syntax error; `0X` is
accepted. **Unknown, a probe case:** `1X'ABC'` and `'A'1X`, Phase 76's note.

**Positioning and records:** `'A',T5,'B',TL2,'C',TR2,'D',3X,'E',5X` is
`A__CB_D___E` with no trailing blanks. `(1X,I3,/,1X,I3)` of 3 items ends with
a record holding only its 1X. `(//,' A'//)` is 2 blank records, A, 2 blank;
`(/)` is 2; `(' X',/)` is X and 1 blank. `(1X,I3,2(1X,I2,1X,I4))` of 8 items
reverts to the inner group with its count, as the manual says.

**List-directed output, against Phase 78's choices.** Same values, FORRTL
then FTNRTL today:

```
FORRTL   |  3.400000       3.400000       3.400000       3.400000       3.400000    
FTNRTL   |   3.400000       3.400000       3.400000       3.400000       3.400000    
FORRTL   |(1.000000,2.000000) (1.000000000000000,-2.500000000000000)
FTNRTL   | (1.000000,2.000000) (1.000000000000000,-2.500000000000000)
FORRTL   |<80 of the 100 characters>     then <20>AB T T T
FTNRTL   |<79 of the 100 characters>     then <21>AB T T     -3
```

(The last column differs only because the trial passed that word by
`FOR$IO_WU_R`, a logical's entry.) Three differences from Phase 78's choices, all marked unconfirmed
there:

1. **A numeric field's first blank is the carriage control** at a record's
   start, so a record of numbers shows one column less than FTNRTL's. A
   character value at a record's start gets a carriage control of its own.
2. **A complex value has no blank before it at a record's start,** one
   between values. It is the same rule, so the "blank is govax's" in
   `ftn_listout.mar` goes.
3. **A record's text is 80 characters,** with the control byte 81 in all;
   FTNRTL's RECORD_MAX of 80 includes it.

The two that *agree*: a value that doesn't fit starts a new record; a
character value that doesn't fit starts a new record, then fills them (`AB`
alone, then the long value's records).

**Open items, each a probe case:**

- ~~list-directed integers under govax~~: govax's defect, fixed (below);
- what a list-directed record's limit is at a non-terminal, and the formatted
  output limit (OUTSTAOVE at a record of more than the unit's size);
- exponents past 99 (E+nnn), needing G or H values, which F and D can't reach;
- the record size for terminal READ, and INPRECTOO, 22;
- H in input, on a run-time format (the manual: not copied back);
- internal files' END= and overflow, their BLANK, and a null list;
- the DECODE/ENCODE count's limits and MORONEREC, 27;
- `$` on non-blank records; `:` in odd places; deep group nesting's limit;
- what each condition does after the statement (recursive I/O, 40).

## What the compiler has to change

- **A new `internal/fortran/format` package** [Decision 3], the FORMAT grammar
  in Go, used by `sema`, teaching its own `doc.go`; a list of `Item`s with
  repeat, width, and so on, its syntax messages, and `<expr>` positions, each
  parsed by the parser's expression code (`parse`'s, reached through a small
  exported entry, as `format` must not import `parse`).
- **`sema`:** FORMAT statements (their labels, `Unit.Formats`), `NAMELIST`
  (`Unit.Namelists`; a symbol's class doesn't change, the name is a group's),
  a typed `IOStmt` replacing `Print` (kind, unit, format, namelist, specs,
  items, an implied-DO's nodes), `items` taking input's rule (an lvalue,
  no expression) and implied-DO, ASSIGN's FORMAT case, ERR=/END= as refs,
  ENCODE and DECODE, ERRSNS's two lengths. New messages (FMTSYNTAX with a
  caret in the format, FMTLABEL, NOTFORMAT, BADINPUT, NMLINTERNAL,
  NOTNAMELIST, NMLDUMMY, BADFORMATTYPE).
- **`ir`:** `Control` grows the fields Decision 12 listed (unit, format, flags,
  IOSTAT's address, the internal file's descriptor, the namelist's table, the
  VFE routine); the begin routine by statement; `Switch` on `FTN$IO_END`'s R0
  for END=/ERR=; `lowerer.item` for an input item (a descriptor of where it
  is, as now, never a temporary); implied-DO; FORMAT's text and its cache cell;
  the VFE routine; namelist tables.
- **`codegen`:** a VFE routine, whose first instruction loads AP (Decision 5).
- **FTNRTL:** essentially all new, by module: `ftn_format.mar` (the parser and
  the walk), `ftn_edit.mar` (each descriptor's output), `ftn_input.mar`
  (fields, each descriptor's input), `ftn_listin.mar`, `ftn_namelist.mar`,
  `ftn_internal.mar`, `ftn_errors.mar` (the statement context, the conditions,
  IOSTAT, ERRSNS); `ftn_units.mar` a table; `ftn_listout.mar` reworked on the
  shared record. New vector entries at the end, GSMATCH raised.
- **`vmsdef`:** the FOR facility's texts (question 2). `corevms`'s message
  lookup needs no change: `$GETMSG` looks in `vmsdef`'s tables.
- **Tests:** about 35 of the 41 `testdata/fortran/*.out` files have a
  list-directed line that moves a column (question 4), and so do MANUAL.md's
  chapter 14 examples and FORTRAN-INTERNALS.md.

## Decisions

Proposed, for the author's review.

1. **FORRTL settles what the LRM leaves open,** by the author's ruling: VMS's
   run of a probe is the requirement, and a difference from it is listed in
   the oracle's table with its reason, as other phases' are. Where the manual
   and FORRTL disagree, FORRTL wins and the doc says so.
2. **One table drives both sides of the probe:** `testdata/for80/gen.go`
   writes `for80.mar` (calling `FOR$`, for VMS) and `for80.for` (the same
   cases as FORTRAN statements, for govax); the same line for each case. The
   MACRO's handler prints a condition's number; the FORTRAN's IOSTAT= prints
   the same number. `TestFor80Oracle` compiles and runs `for80.for` and
   matches `vax/for80-vax.txt`. Cases FORTRAN can't state (an arbitrary format
   mismatch without IOSTAT's route, say) run in a second MACRO file, against
   govax's FTN$ directly, if any turn up.
3. **One format grammar, written twice,** in Go for the compile-time check and
   in MACRO for FTNRTL, both held to the probe's syntax cases. So a FORMAT
   statement FORRTL would refuse is `%FORT-E-FMTSYNTAX` at compile time. A
   run-time format is checked when used: SYNERRFOR.
4. **A FORMAT is text in `$PDATA` and a cache cell in `$LOCAL`,** since `$PDATA`
   is read-only, correcting the design note. FTN$ parses the text at first use
   into a heap block (LIB$GET_VM) and stores its address in the cell. H on
   input writes to the parsed copy, so a later statement using that FORMAT
   sees the data, as 8.3.9.2 says, and a run-time format's copy is freed with
   its statement, so the data isn't kept, as 8.7's note says. ASSIGN stores
   the address of the FORMAT's control block (text and cell), `MOVAB`, Phase
   79's answer 1.
5. **Variable format expressions are one routine per FORMAT,** `.ENTRY`
   compiled in the unit, `F(n)` for the nth `<expr>`, returning its value in
   R0 (a CASEL on n). The design note's routine needs dummies, and they are
   reached through AP, so FTNRTL's begin call keeps its caller's AP (its own
   frame's saved AP) and passes it, and the routine, once it has n, does
   `MOVL 8(AP),AP`; its RET puts AP back. The routine's temporaries are the unit's static cells; a
   VFE's function that does I/O is 7.1.2's rule. A value outside its range
   is VFEVALERR, 68.
6. **Statements stack.** A context (the unit or internal file, the format's
   place, the record's position, the first error, the flags) is made by each
   begin call from a free list of heap blocks, pushed on FTNRTL's current
   statement; FTN$IO_ITEM and FTN$IO_END use the top. A begin on a unit
   already in use is RECIO_OPE, 40. A signaled error that unwinds past a
   statement leaves its context: a begin pops any whose frame is gone, by the
   FP each records.
7. **Implied-DO loops don't test status after an item,** correcting the design
   note: after the first error FTN$IO_ITEM does nothing (Decision 12), the
   variable's undefined anyway, and it keeps the loop an ordinary loop.
8. **FTN$IO_END returns in R0:** 0 for success, negative for end of file,
   positive for an error (the FOR number), which a statement with specifiers
   stores (IOSTAT) and branches on with a `Switch`; a statement with no
   specifiers never sees a failure, as FTN$ signals first. Flags in the control
   block say which were given. End of file with IOSTAT is -1 (ours).
9. **Signaling:** with no specifier to take it, the FOR$\_ value is signaled
   with the unit's number and the file's name as FAO arguments, by severity:
   LIB$SIGNAL for OUTCONERR's E, after which the statement goes on, LIB$STOP
   for the F conditions. A field is written before FORVARMIS, as FORRTL's is.
10. **Only the terminal units in this phase.** -1 (default input, default
    output), 5, and 6 are connected at first use, by `$CREATE` of SYS$INPUT or
    SYS$OUTPUT with RAT=FTN and FAC=GET,PUT, as Phase 72 saw; WRITE (5,...)
    works, as DUNGEON needs. Any other unit from 0 to 99 is a run-time NOTIMP,
    2, until Phase 81 adds the table's rest; outside that, INVLOGUNI, 32. The
    logical names (FOR$READ, FOR005...) are Phase 81's. Units 5 and 6 and the
    default ones share a RAB per file, so a `$` line stays open across them.
11. **The default for BLANK is the manual's:** ZERO with no OPEN, so a
    terminal READ of `1 2` under I3 is 102; surprising, and tested. BN or BZ
    in a FORMAT override it. Phase 81's OPEN will set NULL.
12. **One item call.** Decision 12's FTN$IO_ITEM stays, for input too: the descriptor says the type, the context the direction. A complex
    item takes two descriptors' turns; a CHARACTER one takes A.
13. **ERRSNS is two FTNRTL routines,** `FTN$ERRSNS` and `FTN$ERRSNS_W`, for
    INTEGER\*4 and INTEGER\*2 arguments, chosen by sema from the first non-null
    argument, as FORRTL's two entries suggest. Mixed lengths are an error,
    unconfirmed.

## Questions for the author

The author's answers follow the questions.

1. **The VMS run.** The trials ran FORRTL on *govax*, and one result is
   visibly affected by govax's LIBRTL. I'd build `testdata/for80`'s exchange
   volume and you'd run it on the simh VAX, as with earlier probes. It would be
   subtask 2, before any FTNRTL code, so the requirements are settled first; a
   second, shorter round can follow what the first turns up. **Recommended.**
   Without it, FTNRTL would be held to FORRTL under govax, which needs
   FORRTL.EXE on the local disk and carries a known defect.
2. **FOR message texts.** Decision 15 said our own wording, as the User Manual
   isn't here. With your ruling, a `$GETMSG` loop over the FOR$\_ values in
   the probe would give VMS's. (a) Take VMS's text and FAO counts into
   `vmsdef`, as other facilities' are, so a program's messages read as on VMS.
   (b) Keep ours and match only the identifiers. **Recommended: (a),** which
   revises Decision 15. These short strings are the program's visible
   behavior rather than its code, but it is your call whether they count
   as copying.
3. **The namelist block.** `FOR$WRITE_SN` takes a block whose layout no manual
   gives, and no program on hand shows one (DUNGEON has no NAMELIST). (a) Try
   a few likely layouts in the probe, as the `_SO` arguments were found. If
   none works, namelist follows the manual's example, its columns unconfirmed.
   (b) Don't try. Disassembling FORRTL to learn the layout is out under your
   ruling, as I read it. **Recommended: (a).**
4. **List-directed output moves a column.** Following FORRTL (differences 1 to
   3 above) changes the output of nearly every program in `testdata/fortran`,
   and MANUAL.md's and FORTRAN-INTERNALS.md's examples. **Recommended: change
   them, in the subtask that reworks list-directed output,** once VMS's run
   confirms it, since the trial could be govax-affected.

**The author's answers (2026-10-31):**

1. They will run the probe on the simh VAX: subtask 2, as recommended.
2. Reusing FORRTL's message texts is fine: **(a)**, VMS's texts and FAO counts
   go in `vmsdef`, which revises Decision 15's "our own wording".
3. Use the probe to find `FOR$WRITE_SN`'s block: **(a)**. Found the same day,
   before the VMS run; see below.
4. "As close to FORRTL as practical": FTNRTL's list-directed output follows
   FORRTL, with differences 1 to 3 applied once VMS's run confirms them. This
   is now the phase's general rule, wider than list-directed output.

**The namelist block, found by calling it.** A guard block of distinct bad
addresses shows which longword `FOR$WRITE_SN` follows: its access violation
names the address. No disassembly was involved. The second argument is:

```
        .ADDRESS  group's name, .ASCIC
        .LONG     n, the number of entries
        n times:
        .ADDRESS  entity's name, .ASCIC
        .ADDRESS  entity's descriptor: class S, or class A for an array
```

So a compiled namelist block is exactly this, and FTNRTL's can be too. Output
on govax for L, F, T, and an array, [trial] until VMS's run:

```
 $GRP
 VAL     =          42,
 R       =   2.500000    ,
 T       = 'HELLO',
 A       = 3*1
 $END
```

The name is padded to 8 columns, repeats are `r*value`, and the last entity
has no comma. `FOR$READ_SN` takes the same block; under govax it stops at a
`GOVAX-E-NOTIMPL` stub, so input waits for VMS's run.

**List-directed integers explained.** It was a defect in govax's LIBRTL, not in
FORRTL. For a list-directed value FORRTL passes `OTS$CVT_L_TI` a descriptor with
only its length and pointer set, the type and class being whatever the stack
held, and govax refused the class with `OTS$_INVSTRDES`, which FORRTL reports as
OUTCONERR. Phase 72 had already found this for floating input
(`readAnyString`); `otscvt.go`'s `readOTSOutput` is the same rule for output.
The complex-input failure above was the same defect, on the input side
(`textToInteger`): the manual's READ now gives the manual's values.

## VMS's first run (round 1, 2026-10-31)

OpenVMS V7.1 on simh's VAX 8600 (`testdata/for80/vax`). Run 1 reached case
#853 of 859; run 2 (unit 6 a process-permanent file) reached the same place;
run 3 and the terminal session ran whole. What it settles, against Part A:

- **The [trial] results stand.** Over govax's LIBRTL, FORRTL gave VMS's output
  in every case of the editing sections but those listed below, so Part A's
  tables are now confirmed, not trial.
- **E10.3 of -0.5555 is `-0.555E+00`:** the manual's `-0.556E+00` is wrong, as
  REAL's nearest value is below the half. FORRTL wins (Decision 1).
- **List-directed output: all three differences confirmed** (Part A's 1 to 3):
  the first blank of a numeric field is the carriage control (`|           1|`,
  12 characters), a complex value at a record's start has no blank of its own,
  and a record holds 80 characters of text after its control byte (81 `A`s are
  two records). By the author's answer 4, FTNRTL follows, in subtask 7. More
  from the run: INTEGER\*2 and \*1 take 7 and 5 columns (`      7    7`);
  logicals 2 (` T`); REAL a G field, `1000000.` but `1.0000000E+07`; REAL\*8
  `3.400000000000000` and `1.0000000000000000E+20`; G floating
  `3.40000000000000`, `1.000000000000000E+200`; H 33 digits after the point,
  a four-digit exponent; complex D `(1.000000000000000,-2.500000000000000)`;
  character values adjacent with no blank (`HELLOWORLD`, `1HELLO`); an empty
  list writes one empty record, its control byte only.
- **A record's size.** At a unit 6 file a formatted record is at most 133
  bytes, control byte included: T133 writes, T134, T200, I5000, and I32767 are
  OUTSTAOVE (66). Reading SYS$INPUT, 133 bytes read and 200 are INPRECTOO
  (22); round 2 narrows it. FORTRAN's default RECL for formatted files is 133;
  FTNRTL takes 133 for both directions. (FORRTL over govax wrote and read the
  long records: govax's RMS gives it a larger size than VMS's, an open item for
  Phase 81's units, not FTNRTL's.)
- **`$` and the first byte, as stored** (FOR80.DAT): with `$`, a blank becomes
  `$` and `+` becomes NUL; `0`, `1`, `$`, and others are stored unchanged.
  Without `$` nothing changes. Part A's rule is confirmed. The terminal session
  (`vax/for80t.log`) shows how a terminal renders them; `+` overprints the line
  before, `1` is a form feed.
- **Units.** WRITE to 0, 1, 99, 100, 101, and 119 creates `FORnnn.DAT`; 120,
  1000, and -5 are INVLOGUNI (32); -1 and -2 write to SYS$OUTPUT; WRITE to 5,
  -3, and -4 is ERRDURWRI (38) when SYS$INPUT is a procedure (RMS$_FAC). So
  FORRTL's unit numbers run 0 to 119 and -1 to -4: Decision 10 changes to
  match, with 100 to 119 Phase 81's like the rest. A write to unit 5 fails
  unless SYS$INPUT can take one, so DUNGEON's writes must go to a terminal.
- **Syntax at run time.** The table's [trial] answers hold. A lone `P`
  (`(1X,P)`) is accepted, then INFFORLOO for want of a data descriptor; counts
  past their limits (`I32768`, `32768I3`, `E12.4E256`, `128P`) are SYNERRFOR.
- **Input** of O and Z that fails stores 0 in the item (the low word of an
  INTEGER\*2's longword slot), where I leaves it: FTNRTL does the same.
  ERRSNS's third value (RMS STV) is 0 for every error the run had.
- **Namelist output,** with the found block: the name padded to 8 columns, to
  16 past 8, then `= `; each value followed by a comma but the last; repeats
  `3*1`; character values in apostrophes; complex `(1.500000,-2.500000)`.
- **Namelist input:** scalars, character variables, `&` for `$`, a lone `$` or
  `&` to end, lowercase, a group found after others, and values over several
  records all work. Column 1 is skipped: a group in column 1 isn't found, and
  `X$GRP` is. A `/` is SYNERRNAM (17); an unknown name, or text for a number,
  is (19). Every array input was INVARGFOR (48): round 1's array descriptor
  had A0 at the first element where the architecture's A0 is the address of
  element 0, the likely cause; round 2 tries the forms. `?` and `=?` do nothing
  at a non-terminal.
- **Messages** (run 3): `%FOR-E-OUTCONERR, output conversion error`, then
  `  unit 6  file SYS$OUTPUT:.;` and `  user PC 00000425`, the traceback after;
  an internal file is `unit -5  file ` with nothing after; an input error adds
  `-FOR-F-INVTEX, invalid text is "ABC"` (`INVTEXREC ... in record number 1` at
  a unit); ENDDURREA adds `-RMS-E-EOF`. OUTCONERR's statement goes on and the
  image ends with status 1; every F condition ends it with the condition as
  `$STATUS`. INVLOGUNI and RECIO_OPE give no file name.
- **Not reached:** `FOR$IO_END` called twice took an access violation inside
  FORRTL and ended the image, so the cases after it, and the `$GETMSG` table,
  didn't run. No compiled program does that; round 2 leaves it out.

**Round 2** (`for80b.mar`, 89 cases, on its own volume): the message table
first (FOR numbers 0 to 255), the SYS$INPUT record limit between 134 and 199,
reads of units -1 to -4, namelist input to arrays by six descriptor forms and a
two-dimensional array, namelist output of every type and long names, and the
end of SYS$INPUT that round 1 didn't reach.


## VMS's second run (round 2, 2026-10-31)

`for80b.mar` on VMS 7.1 ran whole, status 1 (`vax/for80b-vax.txt`,
`vax/for80b.log`).

- **FOR's message texts:** `$GETMSG` gave 57 of the numbers 0 to 255 (the rest
  NOMSG), each `text!/  unit !SL  file !AS!/  user PC !XL`, three parameters,
  or with `!+!+` in place of the unit and file for the conditions that have
  none (NOTFORSPE, NOTIMP, INVARGFOR, ...); SYNERRNAM, TOOMANVAL, and INVREFVAR
  take the text in error first, four; ADJARRDIM takes none. They are
  `vmsdef`'s `forMessages` (`formessages.go`), with INVTEX and INVTEXREC (numbers
  past 255) from round 1's run 3; so Decision 15's "our own wording" is now
  VMS's, by the author's answer 2. `TestFor80Messages` holds FORRTL's run 3
  over govax to VMS's log, message for message, which also found a `$PUTMSG`
  rule: after the first message a zero condition value is passed over (FORRTL
  leaves two zeros where an RMS status and value go); as the first it's NOMSG
  (`corevms/message.go`).
- **SYS$INPUT's record limit:** records of 134 to 199 bytes read; round 1's 200
  was INPRECTOO. So the limit reading a procedure is 199; at a terminal it
  wasn't probed. FTNRTL: 133 for writing, as round 1 showed, and a read buffer
  of 199, INPRECTOO past it (unconfirmed for the terminal).
- **Units -1 to -4:** READ from -1 or -2 is ERRDURREA (39, RMS$_FAC), from -3 or
  -4 reads SYS$INPUT; WRITE was the other way round. So -1 and -2 are output
  units (PRINT, TYPE, WRITE (\*,f)) and -3 and -4 input units (ACCEPT, READ f);
  which of each pair is which isn't shown, and FTNRTL needn't expose them.
- **Namelist input to arrays still fails,** INVARGFOR (48), with A0 at element
  0 or at the first element, with coefficients, bounds, both, or neither, class
  A or NCA, one dimension or two. Output with the same descriptors works. So
  VMS's FORRTL wants something more for input than the block found, which
  calling it hasn't shown. Not pursued further: FTNRTL's namelist input is its
  own, and follows the manual's rules for arrays, marked unconfirmed.
- **Namelist output,** every type: a name is padded to the next multiple of 8
  past its length (3 to 8, 8 and 9 to 16, 31 to 32), then `= `; a record holds
  80 characters, and a continuation record starts in column 3 at a value;
  equal neighbors are a repeat (`2*1.000000    , 3*2.500000    ,`), in
  character arrays too (`2*'TWO  '`), an apostrophe doubled (`'FOUR'''`); a
  descriptor of data type BU writes as an integer (`-1`, so LOGICAL\*1 needs
  another type code), WU and LU as T or F; BYTE takes 5 columns, INTEGER\*2 7;
  G 15 digits, H 33, complex D and G as in list-directed output.
- **The end of SYS$INPUT** is ENDDURREA (24) with RMS$_EOF, and stays so on the
  next read.

**Subtask 2 is done.** Two rounds settle what the manual left open, but for
namelist input to arrays. The cases become `for80.for` once subtask 3 compiles
FORMAT, held to `vax/for80-vax.txt` by Decision 2's oracle as the editing
subtasks land.


## Subtasks (provisional)

1. **Survey:** chapters 7 and 8, appendix A.1, D.4.3; the edit
   descriptors' rules as a table with the manual's examples as test
   cases; FORRTL's trials. Done, Part A.
2. **The probe** (`testdata/for80`, added by the survey): FORRTL on VMS,
   every case in Part A's table and its open items, and the FOR message
   texts; VMS's run, then a second round if the first calls for one. Done:
   two rounds, and FOR's texts in `vmsdef`.
3. **FORMAT's compile-time check** and control block: `internal/fortran/format`,
   every statement form through sema and the IR, ASSIGN of a FORMAT label and
   PRINT v, implied-DO; FTNRTL's begin and end calls, statement stack, and
   units 5 and 6 in outline. Done ("Subtask 3", below).
4. **The format interpreter:** parsing, the walk, reversion, the shared
   record, positional descriptors, carriage control and `$`, and the
   conditions these raise.
5. **Integer, logical, and character editing,** both directions, with BN and BZ,
   short fields, and the default widths.
6. **Floating editing:** F, E, D, G, P, S, complex, both directions.
7. **List-directed:** output on the shared record (Part A's three differences),
   and input.
8. **Variable format expressions and run-time formats.**
9. **Internal files, ENCODE, DECODE.**
10. **LIB$TPARSE,** brought in from Phase 60 by the author (2026-10-31):
    VMS's FORRTL needs it for namelist input, so a program linked against
    VMS's FORRTL, as DUNGEON-era images may be, needs it from govax. The
    routine, TABLE_PARSE, and govax's own `$INIT_STATE`/`$STATE`/`$TRAN`/
    `$END_STATE` macros, from the LIB$ manual (LIB-411 to LIB-452), clean room,
    with a probe of its own on VMS. Phase 60's trees stay there. FTNRTL's
    namelist input is its own code either way.
11. **NAMELIST.**
12. **The error model:** ERR, END, IOSTAT, FOR messages, signaling, ERRSNS.
13. **Terminal statements** end to end: TYPE, ACCEPT, READ f, units 5 and 6,
    and DUNGEON's use of them.
14. **End to end**, then **close-out** (FORTRAN-INTERNALS.md's chapter on
    FTNRTL; MANUAL.md's chapter 14 and its differences).

## Subtask 3: what was built (2026-10-31)

**The format package** (`internal/fortran/format`, its `doc.go` the
grammar). `Parse` turns a specification into a tree of `Item`s (code, repeat,
w, m, d, e, n, text, a group's items, the offset where each starts) or an
`*Error` with the offset of the bad character. Its rules are VMS's, from the
probe's syntax cases and every other output case: `TestVMSSyntax` reads
`vax/for80-vax.txt` and holds `Parse` to each case's SYNERRFOR or acceptance
(over 400 formats, all agreeing). What VMS showed, beyond Part A:

- a comma is needed between two descriptors but before and after `/`, `:`,
  `$`, and a character constant, and after `kP` (`2PI3`, `2P3I3`); `I3'A'`,
  `1X'A'I3`, `'A'1X'B'` are accepted, so Phase 76's question is settled: the
  form is legal;
- `+2P` is accepted (only P takes a sign), `X` alone is 1X, `P` alone 0P,
  `T0`, `TR0`, `0X`, `I0` accepted; `E8.2E` is E8.2 (an `E` with no digits
  after it); a character constant must hold a character (`''` is SYNERRFOR);
  `0(I2)`, `3$`, `3:`, `3/`, `3BN`, `3T5`, `2'A'`, `0HA`, `H`, and `T`, `TL`,
  `TR` without a count are errors;
- groups nest 8 deep inside the format's parentheses, and 16 is refused; the
  limit between them wasn't probed, so `MaxDepth` is 8 (unconfirmed);
- a comma is optional around an H constant too, govax's choice (unconfirmed:
  the probe has no H run into its neighbors).

A run-time format may have anything after its last parenthesis (VMS ignores
`(1X,I3)XYZ`) and no variable format expression; a FORMAT statement may have
neither, `FMTSYNTAX`.

**Semantic analysis** (`sema/io.go`). FORMAT and NAMELIST are read in the
declarations walk, so a statement may use either before it appears: a FORMAT
needs a label (`FMTLABEL`), its text is checked (`FMTSYNTAX`, the caret at the
character, through `ast.FormatStmt.Offset`, on a continuation line too), and
it goes in `Unit.Formats` by label; a variable format expression is NOTIMPL
until subtask 8. A NAMELIST group (`Unit.Namelists`) is checked once the
statements have said what its names are (`NMLDUMMY`, `NMLITEM`, `NMLTWICE`),
each entity given a reference (`Namelist.Refs`). Every READ, WRITE, PRINT,
TYPE, ACCEPT, ENCODE, and DECODE becomes one `IO` (replacing Phase 75's
`Print`):

- the unit: none (the statement's default unit), an INTEGER expression made
  INTEGER\*4, or a CHARACTER place, an internal file (`BADUNIT` otherwise;
  namelist I/O to one is `NMLINTERNAL`); ENCODE's and DECODE's buffer is a
  place of any type (`BADBUFFER`), its count an INTEGER (`BADCOUNT`);
- the format, in Part A's order: `*`; a label, which `checkLabels` holds to a
  FORMAT statement (`NOTFORMAT`, `UNDEFLBL`); a NAMELIST group's name, by
  position or `NML=` (`NOTNAMELIST`); a CHARACTER value; an array or array
  element of any type, a run-time format; an INTEGER\*4 variable, one ASSIGNed
  a FORMAT's label; anything else `BADFORMAT`;
- ERR= and END= are jumps from the statement (so `INTOBLOCK` and `NOTEXEC`
  apply), IOSTAT= an INTEGER variable or element (`BADIOSTAT`); KEY= and
  KEYID= are NOTIMPL, as are REC=, unformatted I/O, and REWRITE (Phase 81);
- an input list's items are places, whole arrays, and implied-DO lists
  (`BADINPUT`; reading a DO loop's variable inside it is `DOVARSET`); an
  implied-DO list's variable and parameters are a DO statement's
  (`ImpliedDo`).

The default units are VMS's numbers: -1 for PRINT and `WRITE (*,...)`, -2 for
TYPE, -3 for ACCEPT, -4 for `READ f` and `READ (*,...)` (which of each pair
VMS uses wasn't shown; govax's choice). ASSIGN of a FORMAT's label is allowed,
and a FORMAT can't end a DO loop (`DOTERM`).

**Lowering** (`ir/lowerio.go`). A statement is its begin call, an
`FTN$IO_ITEM` per item (an implied-DO list a loop, `countedLoop`, which
`do` now shares; its temporaries stay busy until the statement's end), and
`FTN$IO_END`. The begin call is named by the statement's kind, as FORRTL's
are: `FTN$READ_SF`, `FTN$WRITE_SL`, `FTN$READ_IF`, ..., `FTN$ENCODE`,
`FTN$DECODE`; PRINT \* is `FTN$WRITE_SL` now, so every golden file changed by
that call. Its arguments are the control block, the unit (by value, or an
internal file's descriptor, or for ENCODE and DECODE a text descriptor of the
count's length at the buffer), and, but for list-directed I/O, the format: a
FORMAT's block (`FormatBlock`: its text and a `$FCn` cell in `$LOCAL` for
FTNRTL's parsed form, Decision 4), the address an ASSIGNed variable holds, a
run-time format's descriptor (an element of a numeric array described to the
array's end), or a NAMELIST group's block (`NamelistBlock`, the layout found
by calling VMS's FORRTL). The control block (`Control`) is now its flags
alone: ERR=, END=, IOSTAT=, and a run-time format; the unit moved to the
begin call, since it may be any expression. With specifiers, the status is
kept in a temporary, stored in IOSTAT's variable, and a `Sign` goes to END=
(or ERR= without it) when negative and ERR= when positive. ASSIGN of a
FORMAT's label stores its block's address (MOVAB); an assigned GO TO without a
list leaves FORMAT labels out of its targets.

FTNRTL must read what it needs of the begin call's descriptors (an internal
file's, a run-time format's) at the begin call: the statement's descriptor
cells and a concatenation's dynamic string may be reused or freed before the
end, at an implied-DO loop's first branch.

**FTNRTL in outline** (`ftn_statement.mar`, new; `ftn_units.mar` and
`ftn_listout.mar` reworked; GSMATCH 1,8, twelve new vector entries from
offset 280).

- A context per statement (Decision 6): a `LIB$GET_VM` block on a stack,
  pushed by the begin call, used by the item calls, popped by
  `FTN$IO_END`, then kept on a free list; it holds the FP of the routine
  that began it, its unit, kind, flags, first error, stream, and record. A
  begin call first pops the contexts of routines that have gone (an FP at or
  below its caller's). Every begin entry shares one body in its own frame,
  each entry setting the kind and unit and branching to it.
- The units (Decision 10, as VMS's round 2 corrected it): two streams, OUTPUT
  (SYS$OUTPUT, written only) for -1, -2, and 6, and INPUT (SYS$INPUT, read
  and written) for -3, -4, and 5, each `$CREATE`d at first use with RAT=FTN;
  0 to 4 and 7 to 119 are NOTIMP until Phase 81, any other number INVLOGUNI,
  a read of an output unit ERRDURREA. A statement begun on a unit another
  statement is still using is RECIO_OPE.
- The first error is recorded and the rest of the statement passes over its
  items; `FTN$IO_END` returns 0, -1 for ENDDURREA, or the FOR number when a
  specifier takes it, and otherwise signals with VMS's FAO arguments (the
  unit, the file, for now always an empty name, the user PC where
  `FTN$IO_END` returns): LIB$STOP for a severe condition, LIB$SIGNAL and on
  for the rest. The message matches round 1's run 3 (cases I and J,
  `TestFortranIOSignals`), but for when it comes: after the statement's items
  have run, where FORRTL may signal at once (unconfirmed which).
- List-directed output builds its record in the context, so a function that
  writes another unit inside an item writes its own record first; a failed
  `$PUT` is ERRDURWRI. Every other kind of statement is NOTIMP (FOR number 2)
  until its subtask. `FTN$PRINT_LIST` and `FTN$WRITE_LIST` stay, for programs
  linked before (`TestFTNRTL` still calls them).

Tests: `format`'s, sema's `io.for` golden and its error table, the IR's and
code generator's `io.for` goldens, `testdata/fortran/iostmt.for` (both paging
modes), and `TestFortranIOSignals`.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-10-31, subtask 1, the survey.** Read chapters 7 and 8, 4.10, A.1,
  and D.4.3; compiled every statement form to see what is NOTIMPL. During it
  the author ruled that a program may call FORRTL.EXE to set or check
  requirements, without copying its code. Trials on govax found that MACRO can
  reach FORRTL's statement entries, `FOR$WRITE_SO` taking a format's text, and
  ran about 110 format cases and the list-directed forms. They settle G and
  E's open rules, show three differences from Phase 78's list-directed choices,
  and turn up one failure under govax not yet explained (list-directed
  integers). Part A has the rules, 13 decisions, and 4 questions. One slip: a
  trial's files went to the author's `xfer.dsk`, which the startup procedure
  mounts as DUA1 (TR.MAR, TR.OPT, TR.OBJ, TR.EXE in `[000000]`). They were
  deleted the same hour, and later trials used DUA2 on a scratch volume, the
  lesson `testdata/mth77` already records: a probe's script uses DUA2.
- **2026-10-31, subtask 2, the probe written.** The author accepted the survey
  (their answers follow Part A's questions). `testdata/for80/gen.go` writes
  `for80.mar`, 859 cases by subject and FOR's message texts by `$GETMSG`;
  `for80d.mar`, which prints the output file's records between bars so that first
  bytes and trailing blanks show; `for80e.mar`, ten conditions with no handler;
  `for80t.mar`, carriage control at a real terminal; and the procedures. Writing
  it found two things, recorded above: the namelist block, by a guard block's
  access violations, and the defect behind list-directed integers (fixed,
  `readOTSOutput`, with `TestOTSIntegerToText` and `TestOTSTextToIntegerAnyClass`).
  Over govax's LIBRTL the program runs to its end (`TestFor80RunsOnFORRTL`), but
  `LIB$TPARSE` is NOTIMPL, stopping FORRTL's namelist input, which FTNRTL doesn't
  need; `$GETMSG` has no FOR texts; and a `(1X,I32767)` record of 32,768 bytes is
  accepted, where VMS's limit should show. The table is plain data for Decision 2's
  `for80.for`, not written yet: nothing compiles FORMAT before subtask 3. Decision 2's
  oracle follows once VMS's output is here. No output in `vax/` yet.

- **2026-10-31, subtask 2, round 1 on VMS.** The author ran `@FOR80` and
  `FOR80T` on VMS 7.1. Findings in "VMS's first run" above: the trials confirmed,
  the manual's E10.3 case wrong, list-directed output as FORRTL lays it out, a
  133-byte record, units 0 to 119 and -1 to -4, namelist output's columns, and
  the messages. `FOR$IO_END` twice ended the image early, before the message
  table; namelist array input failed, probably on round 1's A0. Round 2,
  `for80b.mar`, asks those and narrows the input record limit.
- **2026-10-31:** the author put LIB$TPARSE in scope for this phase (it was
  Phase 60's), since FORRTL's namelist input needs it; it is subtask 10, and
  NAMELIST to the close-out are 11 to 14.
- **2026-10-31, subtask 2, round 2 on VMS, and done.** The author ran
  `@FOR80B`. Findings in "VMS's second run": FOR's message texts, now
  `vmsdef.forMessages`; the input record limit (199); units -1 to -4; namelist
  output of every type; namelist input to arrays unexplained, left to the
  manual. `TestFor80Messages` holds FORRTL's no-handler conditions over govax to
  VMS's log, which needed `$PUTMSG` to pass over a zero condition value after
  the first message.
- **2026-10-31, subtask 3, done.** FORMAT's check (`internal/fortran/format`,
  held to every output case of VMS's run), every READ, WRITE, PRINT, TYPE,
  ACCEPT, ENCODE, and DECODE through semantic analysis, lowering, and code
  generation, with NAMELIST, ERR=, END=, IOSTAT=, internal files, run-time and
  ASSIGNed formats, and implied-DO lists; FTNRTL's statement stack, begin and
  end calls, and units -4 to -1, 5, and 6, list-directed output running on
  them and every other statement NOTIMP. What was built, and the choices
  left unconfirmed, are in "Subtask 3" above. MANUAL.md's chapter 14 and
  FORTRAN-INTERNALS.md follow the new calls.
