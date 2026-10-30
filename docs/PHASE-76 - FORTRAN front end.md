# Phase 76 — FORTRAN's whole front end: every statement, every declaration

**Status:** in progress (subtask 1, the survey, done 2026-10-30, for
review). Needs Phase 75 (every pass for a subset). Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN
plan.md` (Decisions 1, 6, 7, 10, 16, and 19).

## Goal

Parse the whole of VAX FORTRAN V5.0, and give every declaration its
meaning and its storage, so that later phases only add code generation
and run-time support:

- **every statement** in the 1988 manual parses to the AST, including
  those whose code later phases generate (I/O statements' control lists
  and I/O lists, FORMAT, STRUCTURE blocks, NAMELIST, ENCODE/DECODE,
  OPTIONS, CDEC$ directives). A statement that parses but has no code
  yet is `%FORT-E-NOTIMPL` at the point semantic analysis reaches it;
- **declarations:** type statements for every type and length (`*n`,
  `CHARACTER*(*)`, `CHARACTER*(expr)`), with initial values in slashes
  (DEC's extension); IMPLICIT and IMPLICIT NONE; PARAMETER (both the
  standard form and appendix A.4's); DIMENSION with constant,
  adjustable, and assumed-size bounds and lower bounds; COMMON (named
  and blank); EQUIVALENCE; DATA with implied DO lists and repeat counts;
  SAVE; EXTERNAL; INTRINSIC; BLOCK DATA; the statement order rules
  (Figure 1-1);
- **constant expressions** folded with `vaxfloat` in every type;
- **the storage:** every program unit's `$LOCAL` layout, every COMMON
  block's psect and layout (with EQUIVALENCE extending it), and DATA's
  initial values in the `.MAR` file, in `$LOCAL` and in COMMON psects
  (BLOCK DATA among them), with the storage map comment.

## What earlier phases leave in place

- **Phase 75:** the lexer, parser, semantic analysis, IR, and code
  generator for the subset, and the end-to-end harness.
- **LINK's overlaid psects:** every contribution to an OVR psect starts
  at its base, so COMMON blocks declared in several modules share
  storage, and the psect is as long as the longest. Initialized
  contributions from BLOCK DATA in another module overlay the others.

## Design notes

- **Storage association** is the classic algorithm: EQUIVALENCE sets
  are merged (union-find) into groups, each member with an offset from
  the group's base; a group containing a COMMON member is placed in the
  block, which may lengthen it at the end and may not start before it
  (an error). Conflicts (one variable at two offsets) are errors. Its
  doc comment works an example through.
- **Alignment:** VAX FORTRAN aligns nothing within COMMON (the VAX
  allows unaligned data); variables in `$LOCAL` are laid out at their
  natural alignment, which is free on the VAX and faster.
- **DATA** writes initial values by offset into a psect's image: the
  generator emits `.BLKB` gaps and `.LONG`, `.WORD`, `.BYTE`, `.ASCII`,
  and floating values in hex (Decision 16), in offset order. Two values
  for one byte is an error.
- **Unresolved:** whether two modules' DATA initializing the same COMMON
  bytes is diagnosed. VMS LINK takes the last; ours does what our LINK
  does, noted in the manual.

## Open questions

- **Initial values in type statements** (`INTEGER I/5/`) are DEC's; they
  are DATA under another name. Confirm the manual's rules for arrays.
- **Hollerith constants in DATA** (appendix C.7) belong here, or with
  the other Hollerith uses in Phase 82.

Both are answered in Part A ("The open questions, answered").

# Part A — the survey (2026-10-30)

Section numbers are the 1988 *VAX FORTRAN Language Reference Manual*'s
(the OCR text beside its PDF). Table D-2 (appendix D.2, the manual's
statement summary) is the list of statements; chapters 1, 2, 4, 10, and
appendix A give the rules. What a rule means for a pass is in
brackets. Where the OCR garbled an example (4.6.3's figures, 5.6's
GO TO forms), the text around it and the standard settle it.

## Statement order (1.1.2, Figure 1-1)

Figure 1-1, read row by row (horizontal lines separate kinds that may
not be interspersed; vertical lines kinds that may):

1. **OPTIONS**, before the unit's header (1.4.3: "must be the first
   statement in a program unit, preceding the PROGRAM, SUBROUTINE,
   FUNCTION, and BLOCK DATA statements").
2. **The header**: PROGRAM, FUNCTION, SUBROUTINE, or BLOCK DATA
   (optional for a main program).
3. **IMPLICIT NONE**, then **IMPLICIT** statements, then the **other
   specification statements**, with **PARAMETER** allowed among both;
   then **statement function definitions** and **DATA**, then
   **executable statements**, with DATA allowed among them too.
4. **END**.

Beside all of rows 2 to 3, two columns may go anywhere: comment lines,
INCLUDE, and the general directives (CDEC$ IDENT, TITLE, SUBTITLE,
PSECT) in one; NAMELIST, FORMAT, and ENTRY in the other (NAMELIST is
still a specification statement in 4.10; the figure lets it sit
among the others). "As a VAX FORTRAN extension, DATA statements can be
freely interspersed with PARAMETER statements and other specification
statements." [So DATA may go anywhere after IMPLICIT; the one hard line
is specification statements before the first statement function or
executable statement.]

"Other specification statements" (the manual's list): COMMON,
DICTIONARY, DIMENSION, EQUIVALENCE, EXTERNAL, INTRINSIC, RECORD, SAVE,
structure declarations (STRUCTURE to END STRUCTURE, with UNION and MAP
inside), type declarations, VOLATILE (and the CPAR$ directives, which
govax reads as comments). The executable statements are the manual's
list too: ACCEPT, ASSIGN, assignment, BACKSPACE, CALL, CLOSE, CONTINUE,
DELETE, DO and END DO, ELSE, ELSE IF, END, ENDFILE, FIND, every GO TO,
every IF and END IF, INQUIRE, OPEN, PAUSE, PRINT, READ, RETURN, REWIND,
REWRITE, STOP, TYPE, UNLOCK, WRITE; ENCODE and DECODE (A.1) are
executable too, as statements that transfer data. [lex's `kinds` table
agrees, but for DEFINE FILE: A.2 says it "must be executed before the
first direct access I/O statement", so it is executable, and the table
should mark it so.]

Other order rules: type declarations "must precede all executable
statements" and a name's type may be declared once, and not after the
name was used with its implicit type (4.4); IMPLICIT NONE and IMPLICIT
may not both appear (D-2); a PARAMETER name's type comes from a
declaration *before* it (4.11); a variable in a bound expression can't
have its type changed by a later declaration (2.2.3.1); a structure
must be declared before a RECORD names it (4.13). BLOCK DATA may contain
only COMMON, DATA, DIMENSION, EQUIVALENCE, IMPLICIT, PARAMETER, RECORD,
SAVE, structure declarations, and type declarations, and no executable
statement (4.1). [sema checks the order with a state per unit: header,
implicit, specification, body; a statement of an earlier state after a
later one is an error naming both. Phase 75 checks only declarations
after executable statements.]

## Every statement, and its phase

"Parse" is the phase whose parser makes the statement's AST node; every
statement parses by the end of Phase 76's subtask 2. "Meaning and
code" is the phase that gives it its meaning in sema and its code
(Phase 74's Part C); until then sema reports `%FORT-E-NOTIMPL` for it
where it reaches it. "75" means Phase 75's subset already has it.

**Program units**

| Statement | Forms | § | Parse | Meaning and code |
|---|---|---|---|---|
| PROGRAM | `PROGRAM nam` | 4.12 | 75 | 75 |
| FUNCTION | `[typ] FUNCTION nam[*m][([p[,p]...])]` | 6.2.2 | 75 (INTEGER, REAL, LOGICAL); 76 every type and `*m` | 79 (functions of every type; 78 the types' arithmetic) |
| SUBROUTINE | `SUBROUTINE nam[([p[,p]...])]`, `*` dummies | 6.2.3 | 75; 76 the `*` dummies | 75; 79 alternate returns |
| BLOCK DATA | `BLOCK DATA [nam]` | 4.1 | 76 | 76 |
| ENTRY | `ENTRY nam[([p[,p]...])]` | 6.2.4 | 76 | 79 |
| Statement function | `f([p[,p]...]) = e` | 6.2.1 | 75 (as an assignment) | 76 recognized by sema; 79 code |
| END | `END` | 5.5 | 75 | 75 |

**Specification statements**

| Statement | Forms | § | Parse | Meaning and code |
|---|---|---|---|---|
| IMPLICIT | `IMPLICIT typ(a[,a]...)[,typ(a[,a]...)]...`, `a` a letter or `c1-c2`; `IMPLICIT NONE` | 4.8 | 76 | 76 |
| Numeric type | `type[*n] v[*n][/clist/][,v[*n][/clist/]]...`; BYTE, LOGICAL, INTEGER, REAL, DOUBLE PRECISION, COMPLEX, DOUBLE COMPLEX | 4.4.1 | 75 (INTEGER, REAL, LOGICAL, no `*n`, no clist); 76 the rest | 76 declarations, storage, initial values; 78 the new types in expressions |
| CHARACTER | `CHARACTER[*len[,]] v[*len][/clist/]...`; `len` n, `(expr)`, `(*)` | 4.4.2 | 76 | 76 storage and initial values; 78 expressions; 79 `*(*)` dummies |
| DIMENSION, VIRTUAL | `DIMENSION a(d)[,a(d)]...`, `d` `[dl:]du`, `du` may be `*` | 4.5, D-2 | 75 (constant bounds); 76 the rest | 76 constant bounds; 76 adjustable and assumed-size declared and checked, 79 their code |
| PARAMETER | `PARAMETER (p=c[,p=c]...)` | 4.11 | 76 | 76 |
| PARAMETER (old form) | `PARAMETER p=c[,p=c]...` | A.4 | 76 | 76 |
| COMMON | `COMMON [/[cb]/]nlist[[,]/[cb]/nlist]...` | 4.2 | 76 | 76 |
| EQUIVALENCE | `EQUIVALENCE (nlist)[,(nlist)]...` | 4.6 | 76 | 76 |
| DATA | `DATA nlist/clist/[[,]nlist/clist/]...`, implied DO, `n*c` | 4.3 | 76 | 76 |
| SAVE | `SAVE [a[,a]...]`, `a` a name or `/cb/` | 4.14 | 76 | 76 |
| EXTERNAL | `EXTERNAL v[,v]...` | 4.7 | 76 | 76 the declaration; 79 procedure arguments |
| EXTERNAL (/NOF77) | `EXTERNAL [*]v[,[*]v]...` | A.6 | 76 | an error: govax has no `/NOF77` |
| INTRINSIC | `INTRINSIC v[,v]...` | 4.9 | 76 | 76 the declaration; 79 procedure arguments |
| NAMELIST | `NAMELIST /g/ list[[,]/g/ list]...` | 4.10 | 76 | 80 |
| RECORD | `RECORD /s/ list[,/s/ list]...` | 4.13 | 76 | 82 |
| STRUCTURE ... END STRUCTURE | `STRUCTURE [/s/] [fields]`, field declarations, `END STRUCTURE` | 4.15.1 | 76 | 82 |
| UNION, MAP | `UNION`, `MAP`, `END MAP`, `END UNION` | 4.15.3 | 76 | 82 |
| VOLATILE | `VOLATILE nlist`, names and `/cb/` | 4.16 | 76 | 82 (no effect on code until 84's optimizer) |
| DICTIONARY | `DICTIONARY 'cdd-path[/[NO]LIST]'` | 1.4.1 | 76 | never (Decision 1: no CDD) |
| AUTOMATIC, STATIC | — | not in the manual | 76 | 82, if a source for their rules is found (see "Corrections") |

**Compilation control**

| Statement | Forms | § | Parse | Meaning and code |
|---|---|---|---|---|
| OPTIONS | `OPTIONS /qual[/qual]...`: `/[NO]G_FLOATING`, `/[NO]I4`, `/[NO]F77`, `/CHECK=...`, `/NOCHECK`, `/[NO]EXTEND_SOURCE` | 1.4.3 | 76 | 82 (see "For the passes") |
| INCLUDE | `INCLUDE 'file[/[NO]LIST]'`, `INCLUDE '[lib](module)[/[NO]LIST]'` | 1.4.2 | 74 (the source pass) | 74 files; 83 text libraries; 85 `/LIST` |
| CDEC$ IDENT | `CDEC$ IDENT 'string'` | 10.3.1 | 76 | 82 |
| CDEC$ PSECT | `CDEC$ PSECT /cb/ attr[,attr]...`: LCL, GBL, [NO]WRT, [NO]SHR, ALIGN=n | 10.3.2 | 76 | 82 |
| CDEC$ TITLE, SUBTITLE | `CDEC$ TITLE 'string'` | 10.3.3 | 76 | 85 (listings) |
| CPAR$ (eight directives) | | 10.2 | never: comments (Decision 1, the source pass) | never |
| FORMAT | `label FORMAT (spec)` | 8.2 | 76 (its text kept, Decision 6) | 80 (checked and compiled) |

**Executable statements**

| Statement | Forms | § | Parse | Meaning and code |
|---|---|---|---|---|
| Assignment | `v = e` | 3.1–3.3 | 75 | 75 numeric and logical; 78 every type, CHARACTER; 82 aggregates (3.4) |
| ASSIGN | `ASSIGN s TO v` | 3.5 | 76 | 79 |
| GO TO | `GO TO s` | 5.6.1 | 75 | 75 |
| Computed GO TO | `GO TO (slist)[,] e` | 5.6.2 | 76 | 79 |
| Assigned GO TO | `GO TO v[[,](slist)]` | 5.6.3 | 76 | 79 |
| Arithmetic IF | `IF (e) s1,s2,s3` | 5.7.1 | 76 | 79 |
| Logical IF | `IF (e) st` | 5.7.2 | 75 | 75 |
| Block IF | `IF (e) THEN`, `ELSE IF (e) THEN`, `ELSE`, `END IF` | 5.7.3 | 75 | 75 |
| DO | `DO [s[,]] v=e1,e2[,e3]` | 5.3.1 | 75 (with a label); 76 without | 75; 79 without a label, REAL variables |
| DO WHILE | `DO [s[,]] WHILE (e)` | 5.3.2 | 76 | 79 |
| END DO | `END DO` | 5.4 | 76 | 79 |
| CONTINUE | `CONTINUE` | 5.2 | 75 | 75 |
| CALL | `CALL sub[([a][,[a]]...)]`, `*s` and `&s` | 5.1 | 75; 76 alternate returns and null arguments | 75; 79 the rest |
| RETURN | `RETURN [i]` | 5.9 | 75; 76 `i` | 75; 79 `i` |
| STOP | `STOP [disp]` | 5.10 | 75 | 75 |
| PAUSE | `PAUSE [disp]` | 5.8 | 76 | 79 |
| READ (every form) | formatted, list-directed, namelist, unformatted; sequential, direct (`REC=` and `u'r`), keyed (`KEY`, `KEYEQ`, `KEYGE`, `KEYGT`, `KEYLE`, `KEYLT`, `KEYNXT`, `KEYNXTNE`, `KEYID=`), internal; `READ f[,list]`, `READ *[,list]`, `READ nl` | 7.2 | 76 | 80 formatted, list-directed, namelist, internal; 81 unformatted, direct; never keyed (indexed files deferred) |
| WRITE (every form) | as READ, without keys | 7.3 | 75 (list-directed, `(*,*)` and `(u,*)`); 76 the rest | 80, 81 as READ |
| PRINT, TYPE | `PRINT f[,list]`, `PRINT *[,list]`, `PRINT nl` | 7.6 | 75 (PRINT `*`); 76 the rest | 75 PRINT `*`; 80 the rest |
| ACCEPT | `ACCEPT f[,list]`, `ACCEPT *[,list]`, `ACCEPT nl` | 7.5 | 76 | 80 |
| REWRITE | `REWRITE ([UNIT=]u[,[FMT=]f][,ERR=s][,IOSTAT=ios]) [list]` | 7.4 | 76 | never (indexed files deferred) |
| OPEN | `OPEN (par[,par]...)`, 37 keywords (D-2) | 9.1 | 76 | 81 |
| CLOSE | `CLOSE ([UNIT=]u[,STATUS=p][,ERR=s][,IOSTAT=ios])` (DISPOSE, DISP) | 9.2 | 76 | 81 |
| INQUIRE | `INQUIRE (par[,par]...)`, by FILE= or UNIT= | 9.3 | 76 | 81 |
| REWIND, BACKSPACE, ENDFILE | `X ([UNIT=]u[,ERR=s][,IOSTAT=ios])`, `X u` | 9.4–9.6 | 76 | 81 |
| DELETE | `DELETE ([UNIT=]u[,REC=r]...)`, `DELETE (u'r...)` | 9.7 | 76 | 81 (relative files); never indexed |
| UNLOCK | `UNLOCK ([UNIT=]u...)`, `UNLOCK u` | 9.8 | 76 | 81 |
| ENCODE, DECODE | `ENCODE (c,f,b[,ERR=s][,IOSTAT=ios]) [list]` | A.1 | 76 | 80 |
| DEFINE FILE | `DEFINE FILE u(m,n,U,v)[,u(m,n,U,v)]...` | A.2 | 76 | 81 |
| FIND | `FIND (u'r...)`, `FIND ([UNIT=]u,REC=r...)` | A.3 | 76 | 81 |

The I/O statements share one grammar, the control list (7.1.1): `[UNIT=]u`
or an internal file first, `[FMT=]f`, `*`, or `[NML=]nl` second (the
keywords optional only in those places, and only if the first's is
omitted), `u'r` only right after a keyword-less unit, then keyword
specifiers in any order (REC, ERR, END, IOSTAT, KEY and its kinds,
KEYID); and the I/O list (7.1.2): expressions (in output), references,
whole arrays, and implied DO lists `(list, i=e1,e2[,e3])`, nested. OPEN,
CLOSE, and INQUIRE take only `keyword=value` (and OPEN's valueless
NOSPANBLOCKS, READONLY, SHARED), but for a first unit without `UNIT=`.
[One parser for the control list, a table of keywords per statement,
values kept as expressions; sema, in the phase that compiles them,
checks the types.]

## Declarations

- **Types and lengths** (2.1, Table 2-1, 4.4.1): BYTE (= LOGICAL*1);
  LOGICAL *1, *2, *4; INTEGER *2, *4; REAL *4, *8, *16; DOUBLE
  PRECISION (REAL*8); COMPLEX *8, *16; DOUBLE COMPLEX (COMPLEX*16);
  CHARACTER*len, 1 to 65535. LOGICAL and INTEGER are 4 bytes, or 2 with
  `/NOI4` (Phase 78). `*n` is not allowed on BYTE, DOUBLE PRECISION, and
  DOUBLE COMPLEX; another length not in the table is an error.
  [Phase 76 gives sema the whole type system (a type and a length) and
  every type's size and storage; 78 the arithmetic.]
- **Where a length goes** (4.4.1, 4.4.2): in a numeric statement "the
  `*n` specifier must be positioned immediately after the array name":
  `INTEGER*2 IVEC*4(10)`; in CHARACTER "the array declarator goes first":
  `CHARACTER*32 SOCSEC(100)*9`. A CHARACTER statement's `*len` may be
  followed by a comma (`CHARACTER*8, A`). A CHARACTER `len` is a
  constant, `(integer constant expression)` (`CHARACTER*(4+LENGTH)`), or
  `(*)`; a PARAMETER name as a length must be in parentheses (4.11).
  `*(*)` is for dummy arguments, functions, and PARAMETER names, which
  take the constant's length (4.4.2's STRING example).
- **Initial values in type statements** (4.4.1, 4.4.2): `/clist/` after
  a name initializes "only the variable or array that immediately
  precedes" it, as DATA would; more than one value only for an array,
  and then "a value for every element in the array"; none for a
  PARAMETER name. [The open question: these are the rules for arrays.
  Repeat counts are allowed, the clist being "as in a DATA statement".]
- **IMPLICIT** (4.8): a type, CHARACTER's with `*len`, and letters or
  ranges in order (`A-H`); it applies to names with no explicit type,
  not to intrinsic functions' own types. The default is `IMPLICIT
  INTEGER (I-N), REAL (A-H,O-Z)`. IMPLICIT NONE: every name must be
  declared. [Two IMPLICITs for one letter: an error (the standard's
  rule; the manual doesn't say). `$` and `_` can't start a name, so
  only letters matter.]
- **Names** (1.1.3, Table 1-1): one entity per name in a unit, except
  that a common block, structure, field, and a variable or array may
  share one. Unique across the program: function, subroutine, entry,
  common block, main program, and block data names. [sema checks a
  file's units against each other, as Phase 75 does for units.]

## PARAMETER and constant expressions (4.11, A.4)

- **The standard form** `PARAMETER (p=c,...)`: `p`'s type is its
  declared or implicit type, and `c` is converted to it (`PARAMETER
  (MU=1.23)` makes MU the integer 1 unless MU is declared REAL). A name
  is defined once; it may be used wherever a constant may, except in
  FORMAT (but inside `<>`) and as a Hollerith count; it may be either
  part of a complex constant, but no part of another constant.
- **The old form** `PARAMETER p=c,...` (A.4): no parentheses, and the
  type is the constant's own: it may not be declared, and its first
  letter means nothing.
- **Compile-time constant expressions**: arithmetic (constants, named
  constants, `+ - * / **` with `**` folded only for an integer exponent,
  and MIN, MAX, ABS, MOD, ICHAR, NINT, DIM, DPROD, CMPLX, CONJG, IMAG
  of constant arguments; integer, real, or complex); logical (Boolean
  and relational operators over logical or integer operands, IAND, IOR,
  NOT, IEOR, ISHFT, LGE, LGT, LLE, LLT); character (`//` and CHAR).
  [Phase 75's folding covers INTEGER, REAL, and LOGICAL operators; 76
  adds every type, the intrinsics, and real `**` integer. A real `**`
  real is not a constant expression: an error in PARAMETER.]
- **Where constant expressions are required**: PARAMETER values, array
  bounds in a main program and in COMMON, CHARACTER lengths, DATA's
  subscripts, substring bounds, implied DO limits, and repeat counts
  (named constants allowed), and EQUIVALENCE's subscripts and substrings.

## Arrays (2.2.3)

- One to seven dimensions; `[dl:]du`, `dl` defaulting to 1, `du >= dl`,
  `du - dl + 1` elements, any bound negative, zero, or positive. A
  bound is an integer expression of constants, dummy arguments, and
  COMMON variables, converted to integer; not array references or
  function references. Bounds that aren't constant make an adjustable
  array, only in a subprogram; `*` as the last upper bound makes an
  assumed-size array, only for a dummy argument. A name has one array
  declarator in a unit (in a type statement, DIMENSION, COMMON, or
  RECORD).
- Column-major order (2.2.3.3); a subscript per dimension, any
  arithmetic expression, truncated to integer. **EQUIVALENCE alone may
  use one subscript** for a multidimensional array, the element's number
  in storage order (4.6.1).
- An array's name alone: COMMON, DATA, EQUIVALENCE, NAMELIST, SAVE, I/O
  lists, type declarations, and arguments; nowhere else (2.2.3.5).
  [The element offset `sum((s_k - dl_k) * stride_k) * size` is sema's
  for constant subscripts (EQUIVALENCE, DATA) and the IR's otherwise, as
  Phase 75 does for one dimension; 76 makes it n-dimensional.]

## COMMON and EQUIVALENCE (4.2, 4.6, 10.3.2)

- **COMMON**: a named block or blank common (`//`, or no slashes for
  the first); a name may appear in several COMMON statements, each list
  continuing the block. Array declarators allowed. A block may share a
  name with a variable but not with a function, subroutine, or entry.
  Same-named blocks in different units share storage; nothing requires
  their contents to agree (4.2's CENTS/MONEY example). [Not stated in
  the manual, but the standard's: a dummy argument or a function name
  can't be in COMMON, and a name in one block only, once.]
- **Psect attributes** (Table 10-1): relocatable, overlaid, global,
  not executable, writable, readable, position-independent, shareable,
  no protection, longword alignment. [As Decision 10 has them: `PIC,
  OVR, REL, GBL, SHR, NOEXE, RD, WRT, LONG`. The block's psect is named
  for it; blank common's is `$BLANK` (Decision 10; the manual doesn't
  name it).]
- **EQUIVALENCE**: each list of two or more variables, arrays, elements,
  or substrings starts at one place; subscripts and substring bounds are
  integer constant expressions; no records, record fields, or dummy
  arguments. Two elements of one array may not be associated, nor two
  substrings of one variable starting at different positions (each a
  conflict: one name at two offsets). Different types may be associated
  (an INTEGER with a COMPLEX's real part). Character elements may
  overlap at any position (4.6.2's FIELDS and STAR).
- **The interaction** (4.6.3): EQUIVALENCE may extend a block past its
  end, never before its start. The manual's valid case is `DIMENSION
  A(4), B(6); COMMON A; EQUIVALENCE (A(2), B(1))` (B runs past A(4)),
  its invalid one `EQUIVALENCE (A(2), B(3))` (B(1) before A(1)). Two
  names in different blocks may not be associated (the standard's
  rule, implied by the offsets).
- **Alignment**: the manual says nothing about alignment inside a
  block, and COMMON's order is the program's. [As the design notes have
  it: nothing aligned inside COMMON, natural alignment in `$LOCAL`. An
  EQUIVALENCE group in `$LOCAL` is placed at its strictest member's
  alignment, when its offsets allow; ours, unconfirmed.]

## DATA, SAVE, BLOCK DATA (4.3, 4.14, 4.1)

- **DATA**: each `nlist/clist/` pairs names, array elements,
  substrings, whole arrays (every element, in storage order), and
  implied DO lists `(dlist, i=n1,n2[,n3])` (nested; limits integer
  constant expressions that may use outer implied DO variables) with
  constants, `n*c` repeating one (`n` a nonzero unsigned integer
  constant or a named integer constant; `c` a constant, possibly
  signed, or a named constant). The counts must match exactly.
- **Conversions**: a numeric constant to a numeric item is converted to
  its type; an octal or hexadecimal constant is extended on the left
  with zeros or cut on the left; a character constant to a character
  item is padded on the right with blanks or cut on the right; an
  integer, octal, or hexadecimal constant 0 to 255 to a CHARACTER*1
  item is that character code; a Hollerith or character constant to a
  numeric item fills its bytes, padded with blanks or cut on the right.
  [Every rule is about bytes, and none needs code: so Phase 76 does
  all of them, typeless and Hollerith constants included; see
  "Corrections".]
- **What DATA may initialize**: not dummy arguments, functions, or
  blank common (the standard's; the manual is silent), and named common
  only in BLOCK DATA in the standard; VAX FORTRAN's own rule is
  unstated. [We allow named COMMON in any unit, as LINK's overlaid
  psects make it work, and diagnose blank common: ours, unconfirmed.]
- **SAVE**: names, arrays, and `/cb/`; not blank common, nor names in
  a common block, procedures, or dummy arguments; with no list,
  everything. Storage is static (Decision 10), so SAVE changes nothing
  but is checked (4.14's NOTE: "The definitions of data entities are
  retained automatically").
- **BLOCK DATA**: the statements of "Statement order"; a DATA in it
  that initializes a named block needs the block's complete
  specification; one may initialize several blocks; its name may be in
  another unit's EXTERNAL to make LINK search libraries for it. [So a
  BLOCK DATA module defines a global symbol of its name, and EXTERNAL of
  it is a reference to that symbol. An unnamed BLOCK DATA's name is not
  given by the manual: 4.12 mentions one ("a name is not specified in a
  PROGRAM or BLOCK DATA statement") without saying it; we use
  `filename$DATA`, unconfirmed.]

## For the passes

Found reading the manual against Phase 75's code:

- **The old PARAMETER form looks like an assignment.** `PARAMETER
  PI=3.14` squeezes to `PARAMETERPI=3.14`, which has an `=` at level 0,
  so the lexer calls it an assignment to `PARAMETERPI`. The two can't be
  told apart by text. [Proposed: before a unit's first executable
  statement, an "assignment" whose text starts `PARAMETER` followed by a
  name and `=` at level 0 (no parenthesis before the `=`) is the A.4
  statement; after it, an assignment. Ours, unconfirmed. A statement
  function `PARAMETERF(X)=...` keeps its meaning, having a parenthesis
  first.]
- **ASSIGN's label runs into TO.** `ASSIGN 10 TO I` squeezes to
  `ASSIGN10TOI`, tokenized `10` and the name `TOI`; the parser splits
  `TO` off the name (and an error if the name doesn't start with TO).
  `GO TO` is already handled by the keyword.
- **`"nn` is an integer.** A.5: "Integer constants in octal form have
  integer data type and are treated as integers", "not the same as the
  typeless octal constants". The lexer makes `"17` a `TypelessConst`
  with radix 8; it should be an `IntegerConst` with a radix (or its
  value). [Subtask 2 or 4.]
- **DEFINE FILE is executable** (A.2), which `kinds` should say.
- **Slashes as brackets.** COMMON, SAVE, NAMELIST, RECORD, STRUCTURE,
  VOLATILE, and CDEC$ PSECT write names between slashes, and blank
  common as `//`, which the tokenizer makes the `//` operator; the
  parser takes `//` there as an empty name. DATA's `/clist/` and the
  type statements' initial values use slashes too, which a constant
  expression can't contain (constants in a clist are not expressions,
  so `/` there always closes the list).
- **NOTIMPL moves to sema.** The parser now reports NOTIMPL for
  statements and forms it can't build (the computed and assigned GO TO,
  initial values, implied DO lists, substrings, typeless, Hollerith, and
  complex constants, `%VAL`, unlabeled DO, `u'r`). The goal is an AST
  node for each, with sema reporting NOTIMPL "at the point semantic
  analysis reaches it". sema's `incomplete` rule (a unit with a NOTIMPL
  declaration gives only structural messages) carries over.
- **`PRINT X`** may name a namelist group or a run-time format (a
  character variable, array, or an integer variable assigned a FORMAT
  label); the parser keeps the name, and sema decides from its symbol.
- **OPTIONS changes the source pass.** `/EXTEND_SOURCE` in an OPTIONS
  statement changes the statement field from 72 to 132 for the unit that
  follows, but the source pass has joined the unit's lines before the
  lexer finds OPTIONS. [Phase 82 must either have the source pass
  recognize an OPTIONS line itself, or re-read the unit's lines; noted
  for its survey.]
- **FORMAT's apostrophes.** The squeeze rule takes an apostrophe after
  a name or digit inside parentheses for the `u'r` separator, so
  `FORMAT (1X'ABC')` (no comma before the constant) would go wrong, in
  the `!` search too (`source/comment.go`). The manual's 8.5 doesn't say
  a comma may be left out there, and FORMAT's text is kept, not
  tokenized; noted for Phase 80, which checks FORMAT.
- **STRUCTURE blocks are nested statements.** Inside STRUCTURE ... END
  STRUCTURE, type statements are field declarations (with `%FILL`), and
  UNION and MAP nest; the parser builds the block as one node with its
  fields, so a type statement there never declares a variable. [Subtask
  2; 82 gives it meaning.]

## The open questions, answered

- **Initial values in type statements:** 4.4.1 and 4.4.2 settle them:
  the values initialize the one item before them, one value for a
  scalar, one for every element of an array (repeat counts allowed),
  none for a PARAMETER name. They are DATA under another name, and
  sema turns them into DATA's pairs.
- **Hollerith constants in DATA:** here. 4.3's rules for a Hollerith or
  character constant given to a numeric item are byte rules that need
  no code, and the same holds for typeless constants (left extension or
  truncation). Phase 82 keeps their uses in expressions and arguments.
  This is the first proposed correction below.

## Corrections to the plan (for the author's review)

1. **Typeless and Hollerith constants in DATA and initial values move
   from Phase 82 to Phase 76** (above). Phase 82 keeps them in
   expressions, assignments, and arguments.
2. **AUTOMATIC and STATIC aren't in the 1988 manual.** Phase 74's
   language list and Phase 82 name them, and the lexer knows them, but
   V5.0's manual has neither (they came with a later VAX FORTRAN).
   Proposed: they stay known to the lexer and parse in Phase 76 (so a
   program using them gets NOTIMPL, not a syntax error), and Phase 82
   drops them unless a DIGITAL source for their rules is found.
3. **The A.6 form of EXTERNAL (`*v`) is an error**, since govax has no
   `/NOF77` (Decision 18's qualifiers don't include `/[NO]F77`). OPTIONS
   `/NOF77` is then NOTIMPL in Phase 82.
4. **Phase 76's NOTIMPL messages come from sema, not the parser**, so
   golden dumps of Phase 75's tests change where the parser's messages
   were (positions stay the statement's or the form's).
5. **Statement order is subtask 3's**, beside the declarations it
   orders, and statement functions are recognized there too (as sema
   must, to tell them from assignments to array elements, which
   Figure 1-1's order decides); their code stays Phase 79's.

Subtasks 2 to 9 below are revised to match.

## Subtasks (provisional)

1. **Survey** (done 2026-10-30): chapters 1, 2, and 4 and appendices A
   and D's statement table; a table of every statement form, with its
   phase (Part A).
2. **The parser's remaining statements**, with golden AST dumps for
   each form in the manual's examples: every statement in the tables,
   the I/O control list and I/O list grammar, STRUCTURE blocks, the A.4
   PARAMETER, ASSIGN's TO, `"nn` integers, DEFINE FILE executable; the
   parser's NOTIMPL messages moved to sema.
3. **Type declarations, IMPLICIT, and statement order**, every type and
   length; Figure 1-1's order; statement functions recognized.
4. **PARAMETER and constant folding** in every type, both forms, the
   constant-expression intrinsics.
5. **Arrays:** bounds, lower bounds, adjustable and assumed-size forms,
   element offsets.
6. **COMMON and EQUIVALENCE:** the layout algorithm and its errors.
7. **DATA, SAVE, BLOCK DATA:** initial values in the `.MAR` file,
   typeless and Hollerith constants among them.
8. **End to end:** programs sharing COMMON across units and files,
   EQUIVALENCE overlays, DATA-initialized tables.
9. **Close-out:** FORTRAN-INTERNALS.md's chapter on semantic analysis
   and storage.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-10-30:** Subtask 1 done, for review: the survey (Part A).
  - Read Table D-2 (every statement's forms), chapter 1 (Figure 1-1,
    the statement categories, names, DICTIONARY, INCLUDE, OPTIONS),
    chapter 2's data types, constants, and arrays, chapter 4 whole,
    7.1's control list and I/O list, chapter 10's directives and Table
    10-1, and appendix A.
  - Every statement's forms, section, and phase are in the tables; the
    order rules, declarations, PARAMETER, arrays, COMMON, EQUIVALENCE,
    DATA, SAVE, and BLOCK DATA have their rules beside them.
  - Both open questions answered: initial values follow 4.4's rules
    (one item, every element of an array); Hollerith constants in DATA
    are this phase's.
  - Five corrections proposed for review: typeless and Hollerith
    constants in DATA move here; AUTOMATIC and STATIC aren't V5.0's;
    `EXTERNAL *v` is an error without `/NOF77`; NOTIMPL moves to sema;
    statement order and statement functions are subtask 3's.
  - Found for the passes: the A.4 PARAMETER squeezes into an
    assignment, ASSIGN's label runs into TO, `"nn` is an integer (the
    lexer makes it typeless), DEFINE FILE is executable, OPTIONS
    `/EXTEND_SOURCE` must reach the source pass, and FORMAT's
    `X'...'` meets the `u'r` rule.
  - Choices of ours, unconfirmed: the A.4 PARAMETER rule above, two
    IMPLICITs for a letter an error, an EQUIVALENCE group's alignment in
    `$LOCAL`, DATA in named COMMON allowed in any unit and in blank
    common diagnosed, `filename$DATA` for an unnamed BLOCK DATA.
