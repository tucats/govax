# Phase 82 — FORTRAN: DEC's data extensions

**Status:** in progress: subtask 1, the survey (Part A below), done
2026-11-01 and reviewed by the author the same day ("The author's
answers"); subtask 2, structure declarations, done 2026-11-01. Needs Phase 79 (subprograms).
Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN plan.md`
(Decisions 1 and 10).

## Goal

DEC's extensions to FORTRAN-77's data and declarations, which VMS
programs use heavily (above all to build item lists for system
services):

- **STRUCTURE, RECORD, UNION, MAP** (4.13, 4.15, 2.2.5): structure
  declarations, substructures, unions of maps, `%FILL`, field
  references (`REC.FIELD.SUB(3)`), aggregate assignment (3.4), records
  as arguments, records in COMMON and EQUIVALENCE, arrays of records,
  and records in I/O lists (unformatted);
- **typeless constants:** octal and hexadecimal (`'777'O`, `'FF'X`,
  `"777`), Hollerith (`5HHELLO`) and their typing by context (2.2.1.4,
  2.2.1.7, appendix A.5), RADIX-50 constants (appendix B.3);
- **VOLATILE** (4.16): every reference goes to memory;
- **AUTOMATIC and STATIC**: a variable in the stack frame, or static
  explicitly;
- **OPTIONS** (1.4.3) and the **CDEC$ directives** (chapter 10): PSECT
  (COMMON block attributes, Table 10-1), IDENT (`.IDENT`), TITLE and
  SUBTITLE (listings, Phase 85).

## What earlier phases leave in place

- **Phase 76:** STRUCTURE blocks parsed; COMMON and EQUIVALENCE layout.
- **Phase 78:** every type's code, character moves (aggregate assignment
  is a byte move).

## Design notes

- **Structures are laid out** as the manual's 2.2.5.1 says (fields in
  order, no padding unless `%FILL`); a union is as long as its longest
  map. Field references lower to a base address plus a constant offset,
  plus subscripts.
- **AUTOMATIC variables** live in a frame area the entry sequence
  reserves (`SUBL2 #n,SP`), reached through FP. A routine with them is
  still not recursive in VAX FORTRAN V5's sense, but it becomes
  reentrant for AST use.
- **VOLATILE** turns off the optimizer's (Phase 84) caching of the
  variable; before Phase 84 every reference is to memory anyway, so the
  statement is recorded and checked.

## Open questions

- **RADIX-50 constants:** rare in VMS programs; include for
  completeness, or leave as NOTIMPL until a program needs them?
  **Answer** (the author, 2026-11-01, with the request for the survey):
  include them.
- **Alignment of records** under `/ALIGN` (a later VAX FORTRAN's
  qualifier): out, as V5.0 has none. **Answer** (the author,
  2026-11-01): support `/ALIGN`. Part A's "Alignment" section says from
  what source, and Decision 8 how.

# Part A — the survey (2026-11-01)

Section numbers are the 1988 *VAX FORTRAN Language Reference Manual*'s
(OCR text `~/Documents/Technical Doc/VMS/vax-fortran-lrm-1988.txt`).
"The HP manual" is the *HP Fortran for OpenVMS Language Reference
Manual* (January 2005, V8.0; `VMS-FORTRAN-LANGUAGE.txt` in the same
folder), read for what V5.0's manual lacks: `/ALIGN` (its 14.14, the
OPTIONS directive) and AUTOMATIC and STATIC (its 5.3). It describes a
Fortran 95 compiler on Alpha and Itanium, so only its rules for the
DEC extensions are taken, and where it disagrees with the 1988 manual
the 1988 manual wins (Decision 1 of Phase 74). As in earlier surveys,
what a rule means for a pass is in brackets, and a rule the manuals
leave open is our choice, marked unconfirmed, for MANUAL.md's list of
differences at the close-out. There is no VAX FORTRAN to check against
(Phase 74's answer 5), and nothing here calls FORRTL, so this phase has
no VMS probe.

## Where the compiler stands

Nine programs of every form in the goal were compiled through
`fortran.Compile` on 2026-11-01. Everything parses (Phase 76), and
nothing past that but what is listed below:

- **Done already:** typeless, Hollerith, and character constants in DATA
  and in type statements' initial values (Phase 76's correction 1, with
  BADTYPELESS); a **Hollerith constant as an actual argument** (Phase 79,
  subtask 9: no type, its length as written); `"177`, the octal integer
  of A.5, as an `IntegerConst` of radix 8 (no message, and right: it is
  an integer, not typeless); `OPTIONS /[NO]I4` and `/[NO]G_FLOATING`
  (Phase 78); structure declarations' nesting and their errors (the
  parser's `blocks.go`: NOBLOCKEND, NOBLOCK, NOTINBLOCK, FILLNAME for a
  `%FILL` outside a structure); field references in the syntax tree
  (`ast.FieldRef`, `REC.F(3).G`, from `parse/expr.go`'s
  `referenceAfter`).
- **NOTIMPL** (`sema/notimpl.go`), and each leaves its unit
  `incomplete` (only structural messages after it):

| Form | Reported by |
|---|---|
| STRUCTURE (with its UNION, MAP, `%FILL`, nested STRUCTURE, RECORD, PARAMETER) | `handled`: no case for `StructureStmt` |
| RECORD | `handled`: no case for `RecordStmt` |
| a field reference, scalar or aggregate | `formWhat`, "a record's field" |
| an octal or hexadecimal constant outside DATA, as an operand or an argument | `formWhat` |
| a Hollerith constant outside DATA and argument lists; a character constant in a numeric context | `formWhat`; `sema/expr.go` |
| VOLATILE, AUTOMATIC, STATIC | `handled`'s `AttrStmt` case |
| CDEC$ IDENT, PSECT, TITLE, SUBTITLE, in a unit or before one | `handled` (`statementWhat`, "a CDEC$ directive"); `prologue` (`sema/analyze.go`) |
| OPTIONS `/CHECK`, `/NOCHECK`, `/EXTEND_SOURCE`, `/[NO]F77` | `prologue` |
| DICTIONARY | `handled` |

- **Wrong today:**
  - **A Hollerith constant after `+`, `-`, or a `*` not after a digit**
    isn't one: `JCOUNT = ICOUNT + 2HXY` and `I = 1HC - 1HA` (the
    manual's own examples, 2.2.1.7) get EXTRATEXT ("the statement should
    end before the name HXY"). The rule of where a count may start
    (`countMayFollow`, in both `lex/squeeze.go` and
    `source/comment.go`, which must agree) takes only `( , / = .` and a
    `*` after a digit (a DATA repeat count). A value may also follow
    `+`, `-`, and any `*` (`N*2HAB`, `X**1HA` mean nothing but parse as
    values), and digits followed by H there can't be anything but a
    count. [Subtask 5: add them to both scanners, with tests in each.]
  - **A RADIX-50 constant** (`6RABCDEF`) isn't lexed: SYNTAX ("expected
    '/', but found the name RABCDEF").
  - **CDEC$ OPTIONS and END OPTIONS** are BADDIRECTIVE, as V5.0's
    manual has them.

## Records (2.2.5, 3.4, 4.13, 4.15)

**Structure declarations (4.15).** `STRUCTURE [/name/] [field-list]`,
field declarations, `END STRUCTURE`. The outermost structure needs a
name and no field list; a nested one needs a field list and may have a
name (then a RECORD statement may name it). Structure names are unique
among structures but may also be a variable's, a field's, or a common
block's: their own name space. Structure, field, and record names are
local to the unit. A field declaration is a type statement (any type,
dimensioned in it, with initial values), a RECORD statement (a
previously declared structure as a substructure), a nested STRUCTURE,
a UNION, or a PARAMETER statement ("the same effect as if [it]
appeared outside the block"). Fields must be explicitly typed: IMPLICIT
has no effect inside. No DIMENSION for fields; no adjustable or
assumed-size dimensions and no `CHARACTER*(*)`. Field names are unique
at one level; an inner structure may reuse an outer one's. A structure
can't contain itself at any level. [sema: a `Structure` per
declaration, in a per-unit table by name (nested unnamed ones held by
their field), each a list of `Field`s (name, type or structure,
dimensions, offset, initial values). New messages for each rule; the
parser already refuses what isn't a field declaration.]

**Layout (2.2.5.1, 4.15.1).** "Each field offset is the sum of the
lengths of the previous fields ... The structure is packed": no gaps,
by design (`%FILL` gives alignment by hand). A record array has no gaps
between elements. A union's size is its largest map's, each map laid
from the union's start, and the union then is one field of that size.
The manual's examples (APPOINTMENT, 89 bytes; STRB's NRD; STR with a
union, 11 bytes; WORDS_LONG, 6 bytes) are the layout's tests. [A
layout function that takes the alignment rule (Decision 8); packed is
V5.0's and the default.]

**Union field names.** A map's fields are referenced as the
structure's own (`R.FLT`, not through a map name, which has none), so
they share the structure's level, and must be unique across the maps
and the structure's other fields [ours, from the reference syntax; the
manual doesn't say it in words].

**Initial values (4.15.1, 4.15.3).** A field's initial values apply to
every record of the structure; fields without them, and `%FILL` fields
(which can't have them), are undefined. Initial values in several maps
of one union are applied in statement order, the last winning for the
bytes they share. [Each record in `$LOCAL` or a named COMMON block
gets the structure's `Initial`s at its offset, every element of a
record array likewise; overlaps from a union aren't DATATWICE, and
later ones replace earlier ones. A record's initial values are DATA's
for the COMMON rules Phase 76 chose (a named block, in any unit, once
per byte across the file's units).]

**RECORD (4.13).** `RECORD /s/ list[, /s/ list]...`, list items
variables, arrays, or array declarators. Record names may be in COMMON
and DIMENSION, and not in DATA, EQUIVALENCE, NAMELIST, or SAVE.
"Records and record fields cannot be used in EQUIVALENCE statements"
(2.2.5.2, 4.5): the plan's "records in COMMON and EQUIVALENCE" is wrong
about EQUIVALENCE, which is an error (union is the way). [A record is a
`Symbol` of class variable whose type is the structure (a new `Type`
kind, its length the structure's), placed in `$LOCAL`, a COMMON block,
or an argument. SAVE of a record is an error by the manual's words,
although storage is static anyway; a SAVE of the whole unit or of a
common block holding records is fine. A type statement naming a record
is an error (ours).]

**Field references (2.2.5.2).** `record[.aggregate]...` (aggregate) and
`record[.aggregate]....scalar`, each part with subscripts if it is an
array, and a CHARACTER scalar with a substring. A scalar field
reference is like a variable or array element anywhere in an executable
statement and in statement functions, but not in COMMON, SAVE,
NAMELIST, EQUIVALENCE, or as a DO variable (or an implied DO's, ours).
Type conversion is as for variables. A field named like an operator or
logical constant (`.EQ.`, `.TRUE.`) can't be referenced; the manual
says not to declare one, and the parser's rule that `.` followed by a
name is a field already makes `R.EQ.X` a comparison [a warning at the
declaration, ours]. [The offset of a reference is a constant plus, for
each subscripted level, (subscript − lower) × element length.
Constant parts fold into `Mem.Offset`. A variable part is a byte count,
and because a packed field's offset needn't be a multiple of its size,
VAX indexed mode (which scales by the operand's size) can't in general
take it: the place is `Mem.Ptr`, an address computed as the record's
address plus the byte index (MOVAB base[Rn]), as Phase 78 does for a
CHARACTER dummy. An element of a record array whose length and field
offset are both multiples of the field's size may use indexed mode
directly; that is the optimizer's to find (Phase 84), not this
phase's.]

**Aggregate assignment (3.4).** `v = e`, both aggregate references
"declared with the same structure". No other operation on aggregates
(no arithmetic, no comparison). [A byte move, MOVC3 of the structure's
length (MOVC3's length is a word, so a structure longer than 65,535
bytes is moved in pieces; ours). "The same structure" is the same
declaration (ours, unconfirmed): two structures declared alike aren't
the same for assignment.]

**Records as arguments (2.2.5.2, 6.1.1).** Aggregate references are
actual and dummy arguments, by reference; "the declaration of the dummy
record in the subprogram must match the form of the record declared in
the calling program unit; that is, each structure must have the same
number and types of fields in the same order. The ordering of map fields
within a union declaration is irrelevant." Adjustable arrays may be
records' (`RECORD /S/ R(N)` of a dummy R). `%LOC` takes an aggregate
reference (6.1.2.2). [By reference: the address, as any variable's. An
adjustable record array is Phase 79's descriptor cell with the
structure's length as the element size. Across units of one file,
`checkReferences` compares a record argument's structure with the
dummy's, field by field (types, dimensions, offsets), as it compares
argument counts; a structure argument against a non-record dummy is the
same message (ours). `%VAL` of an aggregate is an error; so is
`%DESCR` (ours: the manual gives no descriptor for a record, and a guess
would be worse than a message). `%REF` is the default's.]

**Records in COMMON** (4.13): an entity of the block, its length the
structure's, packed by Phase 76's layout. [`sema/common.go` takes a
record's length; nothing else changes.]

**Records in I/O (7.1.2, 2.2.5.2).** Aggregate references only in
unformatted I/O lists (one record whatever the list has); not
formatted, list-directed, or NAMELIST. [A class S descriptor of the
record's length, type DSC$K_DTYPE_Z (ours). FTNRTL's unformatted item
routine looks only at a descriptor's class (`ftn_unformatted.mar`), so
it needs nothing new. sema gives the other forms a message.]

## Constants (2.2.1.4, 2.2.1.7, A.5, B.3)

**Octal and hexadecimal constants (2.2.1.4)** have no type until used.
With a binary operator (assignment included), the other operand's type;
where a type is required (generally integer: a subscript, a DO
parameter, ...), that type; as an actual argument, none, and four bytes;
elsewhere (both operands typeless, a unary `.NOT.`, an arithmetic IF)
INTEGER\*4, or INTEGER\*2 under `/NOI4`. Up to 16 bytes; shorter than
the type, zero bytes on the left (the high end); longer, truncated on
the left, an error if a nonzero digit goes. [In sema, a typeless
operand takes its type when the expression around it is typed, its
bytes from `BADTYPELESS`'s DATA code. A REAL or COMPLEX value is the
bits, which the code must move as an integer of its width (MOVL, MOVQ,
MOVO), never with a floating instruction, which would fault on a
reserved operand (`'8000'X` as REAL\*4). An argument of more than four
bytes' digits: truncated to four, BADTYPELESS if a nonzero digit goes
(ours: "a length of four bytes is always used").]

**Hollerith constants (2.2.1.7)** have no type, and never CHARACTER.
The same typing rules as typeless constants, but an actual argument has
its own length (done), and the padding is blanks on the right and the
truncation on the right, an error if a character other than a blank
goes. A **character constant in a numeric context** is a Hollerith
constant (2.2.1.6): `X = 'AB'`. [As typeless constants, in the same
sema code, with the bytes from the text; the lexer fix above.]

**RADIX-50 constants (B.3)**, `nRc1c2...cn`, n from 1 to 12, c from the
RADIX-50 set (Table B-2: space 0, A to Z 1 to 26, `$` 27, `.` 28, 29
unassigned, 0 to 9 30 to 39; octal 0, 1–32, 33, 34, 35, 36–47 in the
manual), three characters a word, `(i*40+j)*40+k` (the manual's
`((i*50+j)*50+k)` is octal), so at most 8 bytes; n counts blanks, which
are characters here. "Only in DATA statements." Assigned to a component
shorter than the constant, the rightmost bytes are dropped (no error,
unlike Hollerith); longer, zero bytes are added on the right. [The
lexer: `nR` is a count as `nH` is, with the same `countMayFollow` rule,
in both scanners (blanks after it are the constant's). Outside DATA and
initial values it is an error. A count that isn't a multiple of 3 pads
the last word with blanks (code 0), which the manual's `4RABCD` implies
(ours). A character outside the set is an error, BADRAD50 (the manual's
`4RDK0:`); lowercase letters are taken as uppercase (ours, unconfirmed:
source is otherwise case-blind outside character and Hollerith
constants). Initial values in type statements and structure fields take
them as DATA does (ours: they are DATA under another name, Phase 76).
MACRO-32 has no `.RAD50` (it was MACRO-11's), so the value is written
as `.WORD`s, each with its characters in a comment.]

**The octal integer `"nn` (A.5)** is an integer constant: done.

## VOLATILE, AUTOMATIC, STATIC

**VOLATILE (4.16).** `VOLATILE nlist`: variables, arrays, and `/cb/`
common blocks (the whole block); the attribute spreads through
EQUIVALENCE, directly or not (the manual's P1 and P4). Its meaning is
the optimizer's: before Phase 84 every reference goes to memory anyway.
[sema: a `Volatile` flag on each symbol named, every member of a named
block, and each symbol an equivalence group joins to one; a name that
isn't a variable, array, or block (a constant, a procedure, a statement
function) is an error. Records aren't in 4.13's list of statements that
may name them, but a record is a variable, and the item lists of
`$GETJPI` are exactly what VOLATILE is for; ours: allowed. The IR
carries the flag on a `Mem` for Phase 84. A dummy argument may be
VOLATILE (the manual doesn't forbid it).]

**AUTOMATIC and STATIC** are not in the 1988 manual (Phase 76's
correction 2: dropped unless a DIGITAL source is found). The HP manual's
5.3 is one (HP inherited DEC's compilers; the statements came with VAX
FORTRAN after V5.0): `AUTOMATIC v[,v]...` puts a variable in "the stack
storage area", `STATIC v[,v]...` in "the static storage area"; SAVE is
still what keeps a value between calls; a variable with initial values,
in COMMON, or SAVE'd is always static; neither for a dummy argument or a
function result, AUTOMATIC not for a COMMON item; a name at most once in
either. (Its `type, AUTOMATIC :: v` attribute form is Fortran 90's, not
VAX FORTRAN's.) Question 1 is whether to take them. [If taken: a frame
area the entry sequence reserves (`SUBL2 #n,SP`, or MOVAB -n(SP),SP),
the variables at negative offsets from FP in the storage map, every
entry point (ENTRY) reserving the same area; a CHARACTER or array
variable's descriptor (`$Dn`) gets its address stored at run time, as
a dummy's does. AUTOMATIC with SAVE, DATA, initial values (a record
whose structure has any), or COMMON is an error, and so is a name in
both statements. STATIC changes nothing (storage is static, Decision 10)
but its checks.]

**A correction to the design notes:** an AUTOMATIC variable doesn't make
its routine reentrant, because the compiler's own temporaries, DO counts,
and descriptor cells (`$Tn`, `$Cn`, `$Dn`) stay in `$LOCAL`. Two
activations at once (an AST routine that is also called from the main
line) still share them. Moving those to the frame too is RECURSIVE,
which no VAX FORTRAN V5 had; not this phase.

## OPTIONS and the CDEC$ directives (1.4.3, chapter 10)

**OPTIONS (1.4.3).** V5.0's qualifiers: `/[NO]G_FLOATING`, `/[NO]I4`
(done), `/[NO]F77`, `/CHECK=...`, `/NOCHECK`, `/[NO]EXTEND_SOURCE`.
Phase 85's doc claims OPTIONS's `/CHECK` and `/EXTEND_SOURCE` and the
`/F77` question, which corrects Phase 76's note that `/EXTEND_SOURCE`'s
reach into the source pass is this phase's. The HP OPTIONS statement
(its 13.3) has no `/ALIGN`: alignment is the command's qualifier and the
directive's (below). [Nothing for OPTIONS here.]

**General rules (10.1).** `CDEC$` in columns 1 to 5, a blank or tab in
column 6, blanks insignificant after it, no continuation lines, blank
common written `//` [the source and lex passes do these already].

**CDEC$ IDENT 'string' (10.3.1):** up to 31 printable characters, into
the object module's ident; only the first in a unit counts. One source
file is one module (Phase 74's Decision 3), so the module's `.IDENT`
is the first IDENT directive of the file, wherever it is, and later
ones are ignored (ours, unconfirmed: VAX FORTRAN made a module per
unit). [`writeHeader` (`compile.go`) takes it in place of `/01/`, with
a delimiter the string doesn't contain. More than 31 characters is an
error (ours).]

**CDEC$ PSECT /cb/ attr[,attr]... (10.3.2):** LCL or GBL, [NO]WRT,
[NO]SHR, ALIGN=0 to 9 (a power of two; 9 a page), over Table 10-1's
defaults (`codegen`'s `commonAttributes`: PIC, OVR, REL, GBL, SHR,
NOEXE, RD, WRT, LONG). Every unit that names the block must change it
the same way. [sema keeps a block's attributes on its `CommonBlock`;
codegen writes them in its `.PSECT` (govax's MACRO takes an alignment
from 0 to 9). Since a file is one module, two units of it with
different attributes for one block would make MACRO's PSECTCONFLICT,
so sema reports it first (the manual's rule, checked where we can). LCL
with GBL is an error; so is a directive for a block the unit doesn't
declare (ours). Before a unit, a PSECT directive belongs to the next
unit (ours, as OPTIONS does).]

**CDEC$ TITLE and SUBTITLE (10.3.3)** take effect only with `/LIST`,
which is Phase 85's. [Here: accepted and kept, with no effect, so that
programs using them compile; Phase 85 puts them in the listing.]

**DICTIONARY (1.4.1)** reads the Common Data Dictionary, which govax
doesn't have: it stays NOTIMPL (ours).

## Alignment: `/ALIGN` and CDEC$ OPTIONS

V5.0 has no alignment control: its structures and common blocks are
packed. The author asked for `/ALIGN` (2026-11-01); its rules come from
the HP manual's 14.14, the OPTIONS directive, which the qualifier
shares:

- `CDEC$ OPTIONS option [option]` ... `CDEC$ END OPTIONS`, nesting up to
  100 levels, after the unit's OPTIONS and header statements and before
  its executable part, each OPTIONS ended by an END OPTIONS. An inner
  group's setting changes only the classes it names ("COMMONS retains
  the previous setting").
- `/[NO]ALIGN[=p]`, p `class=rule`, `(class=rule,...)`, a rule alone
  (every class), `ALL` (`/ALIGN`, NATURAL for both), or `NONE`
  (`/NOALIGN`, PACKED for both). Classes COMMONS, RECORDS, and
  STRUCTURES (RECORDS's synonym); rules PACKED (arbitrary byte
  boundaries), NATURAL ("up to 64-bit boundaries", every type,
  COMPLEX included), and STANDARD ("up to 32-bit", COMMONS only).
  Keywords abbreviate (`/ALIGN=RECO=NATU`, its example).
- `/WARN=[NO]ALIGNMENT`: messages for data not naturally aligned.
- A natural boundary is "a multiple of the size of its data type".

What the HP manual leaves out, because it is its *User Manual*'s (which
we don't have) or the machine's: the command qualifier's form and
default, natural boundaries for COMPLEX, CHARACTER, and records, whether
a structure's length is rounded up, how a COMMON statement inside a
group is laid out, and what EQUIVALENCE does to an aligned block. These
are Decision 8's choices and question 2. CDEC$ PACK (14.15) is a
further Alpha-era directive; out, unless the author wants it.

## What has to change

- **lex and source:** `countMayFollow` in both scanners (Hollerith after
  `+`, `-`, `*`); `nR` RADIX-50 constants (a new token kind). CDEC$
  OPTIONS and END OPTIONS in `parseDirective`, with the qualifier
  grammar `parse/spec.go` already has for OPTIONS.
- **sema:** structures and their layout (a new file, `structure.go`),
  a record `Type`, field references (`FieldRef` to a typed `Field`
  expression), aggregate assignment, record arguments and their checks,
  COMMON's record entities and PSECT attributes, typeless and Hollerith
  typing by context (out of `formWhat`), RADIX-50 in DATA, VOLATILE,
  AUTOMATIC and STATIC if taken, the directives, the alignment groups.
  Each form leaves `handled`'s and `formWhat`'s "not yet" side.
- **ir:** a field's `Mem` (offset, or `Ptr` for a variable part), record
  moves (a `CharMove` of the length, or a byte-move operation of its
  own), record items in unformatted I/O, typeless REAL values moved as
  integers, a frame `Cell` kind for AUTOMATIC, `Mem.Volatile`.
- **codegen:** FP-relative cells and the frame reservation; PSECT
  attributes in a block's `.PSECT`; the module's `.IDENT`; the storage
  map's records and fields.
- **The FORTRAN command:** `/[NO]ALIGN[=...]` in `console.dcl` and
  `dclcmd/fortran.go`, `--align` in `govax fortran`, `fortran.Options`
  to `sema.Options`; HELP FORTRAN and MANUAL.md's chapter 14.
- **FTNRTL:** nothing.

## Decisions

1. **Structures are laid out packed by default**, V5.0's rule, the
   manual's examples their tests (APPOINTMENT 89 bytes, WORDS_LONG 6).
2. **A field reference's offset folds its constant parts**; a variable
   part makes the place a computed address (`Mem.Ptr`), since packed
   offsets defeat indexed mode. Phase 84 may do better.
3. **Aggregate assignment is a byte move** between references of the
   same structure declaration.
4. **Records go by reference**, are checked against a dummy's structure
   across the units of a file, and refuse `%VAL` and `%DESCR`.
5. **Typeless and Hollerith constants take their type in sema** from
   the expression around them; floating values with typeless bits move
   as integers.
6. **RADIX-50 constants** (the author's answer): DATA and initial values
   only, blanks padding the last word, a character outside the set an
   error, lowercase taken as uppercase.
7. **VOLATILE is recorded and checked**, spread through equivalence and
   blocks, carried to the IR for Phase 84; records and dummies may be
   VOLATILE.
8. **`/ALIGN`** (the author's answer), on the FORTRAN command and in
   CDEC$ OPTIONS groups, with the HP manual's grammar:
   - **The default is `/NOALIGN`**: packed, V5.0's rule, so programs
     written for V5.0 lay out as they did (the HP compiler's defaults
     are an Alpha's and not taken). `/ALIGN` alone is `ALL`, NATURAL for
     records and commons.
   - **Natural boundaries** (ours, unconfirmed): an INTEGER, LOGICAL, or
     REAL its length, at most 8 under NATURAL and 4 under STANDARD (so
     REAL\*16 at 8); a COMPLEX at 16 under NATURAL (the author's answer
     2; proposed was its part's); CHARACTER and BYTE 1; a substructure or union the largest of
     its fields'; an array its element's. Under RECORDS=NATURAL each
     field goes to its next boundary, and a structure's length is
     rounded up to its own boundary, so the elements of a record array
     stay aligned. A record in COMMON under COMMONS=NATURAL goes to its
     structure's boundary.
   - **COMMON under a group:** each COMMON statement's entities are
     placed with the setting where the statement is, padding before an
     entity as needed (ours; the HP text speaks of "common blocks within
     Group B"). EQUIVALENCE extending an aligned block uses the block's
     offsets as they are, padding included. Two units of a file laying
     one block out differently is EXTCOMMON's existing message (if its
     length differs) or a new one.
   - **The block's psect alignment** is at least its largest natural
     boundary under COMMONS=NATURAL (QUAD for an 8-byte item), so that
     an offset's alignment is an address's; a CDEC$ PSECT ALIGN= that
     is smaller is an error (ours).
   - **`/WARN=[NO]ALIGNMENT`** in CDEC$ OPTIONS: off unless asked for
     (ours: under V5.0's packing nearly every structure would warn), an
     informational message per field or common item not on its natural
     boundary. The command's `/WARNINGS` is Phase 85's.
   - **CDEC$ PACK is out.**
9. **CDEC$ IDENT** gives the file's module its `.IDENT` (the first in
   the file); **PSECT** changes a block's `.PSECT`, checked across the
   file's units; **TITLE and SUBTITLE** are accepted with no effect
   until Phase 85; **DICTIONARY** stays NOTIMPL.
10. **OPTIONS's remaining qualifiers are Phase 85's**, as its doc says.

## Questions for the author

1. **AUTOMATIC and STATIC:** Phase 76 proposed dropping them unless a
   DIGITAL source turned up. The HP manual's 5.3 describes them (the
   statement forms, which VAX FORTRAN gained after V5.0). Take them from
   it, as `/ALIGN` is taken from its 14.14? Recommended: yes, the
   statement forms only, as described under "VOLATILE, AUTOMATIC,
   STATIC" (a frame area, not reentrancy).
2. **`/ALIGN`'s choices** in Decision 8: the default packed; natural
   boundaries with COMPLEX by its part and records by their largest
   field, lengths rounded up; COMMON statements by the setting where
   each is; alignment warnings off unless asked. Any you'd rather have
   otherwise? In particular COMPLEX\*8 at 4 (its part) or at 8 (its
   size, the glossary's literal reading)?
3. **The command qualifier's form:** `/ALIGN[=(RECORDS=rule,
   COMMONS=rule)]` and `/NOALIGN` on FORTRAN, and `--align[=...]` on
   `govax fortran`, taking the directive's grammar. Should OPTIONS (the
   statement) also take `/ALIGN`, as a per-unit override? The HP
   manual's OPTIONS doesn't; recommended: no.

## The author's answers (2026-11-01)

1. **AUTOMATIC, STATIC, and VOLATILE: the HP manual's versions** (its
   5.3 and 5.19), statement forms only. A later phase, not yet defined,
   is to make routines really reentrant, and the frame area is
   groundwork for it. VOLATILE is then the 1988 manual's 4.16 with the
   HP manual's additions: a procedure, a function result, or a namelist
   group can't be named (each an error), and a record's fields are
   volatile with it. A function result can't be VOLATILE, so a
   FUNCTION's name (or an ENTRY's in a function) in the list is an
   error, which Decision 7 didn't say.
2. **COMPLEX aligns on an octaword boundary** under NATURAL: COMPLEX\*8
   and COMPLEX\*16 both at 16 (past the HP manual's "up to 64-bit",
   by the author's choice). Under COMMONS=STANDARD ("up to 32-bit") a
   COMPLEX is at 4. A structure or common block holding a COMPLEX under
   NATURAL is then octaword-aligned: a block's psect is OCTA, and a
   record's place in `$LOCAL` is a multiple of 16, which needs `$LOCAL`
   OCTA-aligned in a unit where `/ALIGN` places such a record (subtask 8).
   The rest of Decision 8 stands.
3. **No `/ALIGN` on the OPTIONS statement.** The command qualifier and
   the CDEC$ OPTIONS groups only.

## Subtasks (provisional, revised by the survey)

1. **Survey** (done 2026-11-01): Part A.
2. **STRUCTURE and its layout** (done 2026-11-01): structures, UNION, MAP, `%FILL`,
   PARAMETER inside, nested structures and RECORD fields, every rule's
   message, the packed layout with the manual's examples as tests, the
   storage map's structures.
3. **RECORD and field references:** records in `$LOCAL` with the
   structure's initial values, scalar field references in every
   executable context (folded offsets, computed addresses), substrings
   of fields, the restrictions (DO variable, EQUIVALENCE, DATA, SAVE,
   NAMELIST); goldens and a program run end to end.
4. **Aggregates:** aggregate assignment, records as actual and dummy
   arguments (adjustable record arrays, the cross-unit check, `%LOC`,
   `%REF`), records in COMMON (with their initial values), records in
   unformatted I/O (and the messages for the other forms).
5. **Typeless, Hollerith, and RADIX-50 constants:** the lexer's count
   rule in both scanners, RADIX-50 tokens and their bytes, typing by
   context in expressions, assignments, arguments, and IFs; character
   constants in numeric contexts.
6. **VOLATILE, AUTOMATIC, and STATIC** (the HP manual's, answer 1): the
   frame area, FP-relative cells, ENTRY's reservations, descriptors.
7. **The CDEC$ directives:** IDENT, PSECT (with the cross-unit check),
   TITLE and SUBTITLE accepted.
8. **`/ALIGN`:** the command's qualifier (`console.dcl`,
   `dclcmd/fortran.go`, `govax fortran`), CDEC$ OPTIONS and END OPTIONS
   groups, the natural layouts of records and commons, the psect
   alignment, `/WARN=ALIGNMENT`.
9. **End to end:** a `$GETJPIW` item list built from a structure, its
   layout checked byte by byte (run in Phase 83 once services are
   reachable), and programs in `testdata/fortran` for records, unions,
   aggregates, constants, AUTOMATIC, and `/ALIGN`; then the
   **close-out** (FORTRAN-INTERNALS.md's section on records and
   storage, MANUAL.md's 14, HELP FORTRAN, the unconfirmed choices in
   MANUAL.md's differences, CLAUDE.md).

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-11-01:** Subtask 1, the survey (Part A). The author answered
  two open questions with the request: RADIX-50 constants and `/ALIGN`
  are in. Read the 1988 manual's 1.4.3, 2.2.1.4–2.2.1.7, 2.2.5, 3.4,
  4.13, 4.15, 4.16, 10, A.5, and B.3, and the HP manual's 5.3 (AUTOMATIC,
  STATIC), 13.3 (OPTIONS), and 14.14–14.15 (the OPTIONS and PACK
  directives). Compiled nine programs of every form through
  `fortran.Compile`: everything parses; the forms are NOTIMPL where
  the table says; found that a Hollerith constant after `+` or `-` is
  EXTRATEXT (the count rule, `countMayFollow`, in both scanners) and
  that RADIX-50 constants aren't lexed. Corrections to the plan: records
  can't be in EQUIVALENCE (the goal said they could); AUTOMATIC doesn't
  make a routine reentrant (the design notes said it did); OPTIONS's
  `/CHECK`, `/EXTEND_SOURCE`, and `/F77` are Phase 85's, not this
  phase's (Phase 76's close-out said otherwise). Ten decisions, three
  questions; subtasks revised from seven to nine.
- **2026-11-01:** The author's answers: AUTOMATIC, STATIC, and VOLATILE
  by the HP manual (groundwork for a later reentrancy phase); COMPLEX on
  octaword boundaries under `/ALIGN`; no `/ALIGN` on OPTIONS.
- **2026-11-01:** Subtask 2, structure declarations. `sema/structure.go`:
  a `Structure` per declaration (nested ones too) in the unit's own table
  (`Unit.Structures`, and `structs` by name, which RECORD statements will
  look in), its `Field`s in declaration order (a union's maps' fields
  among them, at the structure's level, each with its `Union` and `Map`;
  `%FILL` fields named so), laid out packed by `structBuilder` (a union
  as long as its longest map, each map from the union's offset), and its
  `FieldInitial`s from the field declarations' values (by data.go's
  `dataValues` and `initialBytes`, so DATA's rules and messages hold; a
  later map's values replace an earlier one's bytes). PARAMETER inside a
  structure defines the unit's constant. A STRUCTURE statement is now
  `handled`, so a unit with only structures compiles; RECORD and field
  references stay NOTIMPL until subtask 3. New messages: STRUCTNAME,
  STRUCTFIELDS, DUPSTRUCT, STRUCTEMPTY, NOSTRUCT, RECURSTRUCT, DUPFIELD,
  FIELDDIM, FIELDLEN, FIELDOP (a warning: a field named EQ, TRUE, ...),
  FILLINIT, STRUCTSIZE. A CDEC$ directive inside a structure is NOTIMPL
  (subtask 8's OPTIONS groups). The storage map lists each structure's
  fields with their offsets (`codegen`'s `writeStructures`), and sema's
  dump each structure. Tests: `sema/testdata/structures.for`, the manual's
  examples, whose sizes match its own (STRA 5, STRB 18, STR 11, DATE 4,
  APPOINTMENT 89, WORDS_LONG 6), with initial values, `%FILL`, a nested
  unnamed structure, and a union in a map; 31 `TestErrors` cases; the
  codegen golden `structures.for`. Choices of ours, unconfirmed: a union
  with one map, or a map with no fields, is allowed (the manual's syntax
  shows two or more maps); a structure with no field declarations at all
  is an error (4.15's "one or more"); a field's initial values follow
  DATA's conversions (Phase 76's choices).
