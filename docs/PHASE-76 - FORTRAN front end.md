# Phase 76 — FORTRAN's whole front end: every statement, every declaration

**Status:** in progress (subtask 1, the survey, done and accepted
2026-10-30; subtask 2, the parser's remaining statements, done
2026-10-30; subtask 3, types, IMPLICIT, and statement order, done
2026-10-30; subtask 4, PARAMETER and constant folding, done
2026-10-30; subtask 5, arrays, done 2026-10-30; subtask 6, COMMON and EQUIVALENCE,
done 2026-10-30). Needs Phase 75 (every pass for a subset). Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN
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

## Corrections to the plan (accepted 2026-10-30)

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
2. **The parser's remaining statements** (done 2026-10-30), with golden
   AST dumps for each form in the manual's examples: every statement in
   the tables, the I/O control list and I/O list grammar, STRUCTURE
   blocks, the A.4 PARAMETER, ASSIGN's TO, `"nn` integers, DEFINE FILE
   executable; the parser's NOTIMPL messages moved to sema.
3. **Type declarations, IMPLICIT, and statement order** (done
   2026-10-30), every type and length; Figure 1-1's order; statement
   functions recognized.
4. **PARAMETER and constant folding** (done 2026-10-30) in every type,
   both forms, the constant-expression intrinsics.
5. **Arrays** (done 2026-10-30): bounds, lower bounds, adjustable and
   assumed-size forms, element offsets.
6. **COMMON and EQUIVALENCE** (done 2026-10-30): the layout algorithm and
   its errors.
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
- **2026-10-30:** The author accepted the survey and its five
  corrections. Subtask 2 is next.
- **2026-10-30:** Subtask 2 done: the parser's remaining statements.
  - **The lexer** (`internal/fortran/lex`): appendix A.4's PARAMETER
    p=c is its own statement before a unit's first executable statement
    (`oldParameter`; the IF's statement never), `"17` is an
    `IntegerConst` of `Radix` 8 (A.5), DEFINE FILE is executable, a
    CDEC$ directive's text is tokenized, `OPTIONS /CHECK=ALL` isn't an
    assignment for its =, and a length after a CHARACTER array's bounds
    (`A(10)*8`) is read as a length.
  - **The AST** (`ast/stmt.go`): a node for every statement (the
    manual's Table D-2), statements of one grammar sharing a node told
    apart by `StmtBase.Kind` (`AttrStmt`, `PrintStmt`, `TransferStmt`,
    `AuxIOStmt`, `DimensionStmt` for VIRTUAL, `StopStmt` for PAUSE);
    `NotImplStmt` is gone. Types are every type with a `Len` (`*n`,
    `*(e)`, `*(*)`); declarators have initial values; new expressions
    are `Substring`, `FieldRef`, `ImpliedDo`, `ComplexConst`,
    `HollerithConst`, `TypelessConst`, `BuiltinArg` (`%LOC`, `%VAL`,
    `%REF`, `%DESCR`), `AltReturn`, and `KeyList` (OPEN's KEY=). A
    unit's `Prologue` holds an OPTIONS statement and directives before
    it, `File.Trailing` those after the last.
  - **The parser**: every kind has a function (`TestEveryKindParses`).
    `decl.go` has the headers (BLOCK DATA, ENTRY, `*` dummies, typed
    functions of every type) and the type statements (lengths in both
    orders, as each type has them; `/clist/`), DIMENSION and VIRTUAL,
    IMPLICIT, and PARAMETER in both forms; `spec.go` COMMON (blank
    common by position, `//`, or `/ /`), EQUIVALENCE, DATA (implied DO
    lists, nested), SAVE and its kin, NAMELIST, RECORD, STRUCTURE,
    UNION, MAP, DICTIONARY, OPTIONS, the CDEC$ directives, and FORMAT
    (its text kept); `stmt.go` ASSIGN (TO split from the name it runs
    into), the computed and assigned GO TO, the arithmetic IF, DO
    without a label, DO WHILE, END DO, PAUSE, alternate return
    arguments; `io.go` READ, WRITE, REWRITE, PRINT, TYPE, ACCEPT, ENCODE,
    DECODE, OPEN, CLOSE, INQUIRE, REWIND, BACKSPACE, ENDFILE, UNLOCK,
    DELETE, FIND, and DEFINE FILE over one control-list grammar with a
    table of each statement's keywords (BADSPECIFIER, DUPSPECIFIER).
    Structure declarations nest (`blocks.go`: NOBLOCKEND, NOBLOCK,
    NOTINBLOCK, FIELDSTMT, FILLNAME). Other new messages: BADLETTERS,
    NOF77 (the survey's correction 3), BADDIRECTIVE.
  - **sema** (`notimpl.go`): NOTIMPL is semantic analysis's now (the
    survey's correction 4). `handled` is the statements it compiles;
    any other is reported where the walk reaches it and keeps its place
    in the blocks; the expression forms it doesn't compile are reported
    as their expressions are analyzed (`formWhat`). These NOTIMPLs are
    reported in incomplete units too (`notImplForm`), as the parser's
    were. A type statement of a type not compiled yet (CHARACTER,
    COMPLEX, ...) no longer makes its unit incomplete: the type is
    reported once, and its names are broken. BLOCK DATA units and
    prologues are reported and left out, until subtask 7.
  - **Tests**: the lexer's new cases; `TestStatements` has every
    statement's forms, most from the manual's examples (chapters 3 to
    9 and appendix A); `TestErrors` the parser's new messages;
    `TestBlocks` structure declarations and their errors; two golden
    programs, `parse/testdata/spec.for` and `exec.for`, from the
    manual's examples; sema's `TestErrors` a case for each statement or
    form that moved its NOTIMPL from the parser. Phase 75's goldens
    changed only where the parser had reported NOTIMPL: those
    statements now parse, and their NOTIMPLs come from sema, later in
    the message list.
  - Found along the way: a fixed-form test line longer than 72 columns
    loses its end, as it should (a test of ours was wrong, not the
    compiler).
- **2026-10-30:** Subtask 3 done: type declarations, IMPLICIT, and
  statement order (`internal/fortran/sema`).
  - **Every type and length** (`types.go`): a `Type` is a kind
    (INTEGER, REAL, LOGICAL, COMPLEX, CHARACTER) and a size, Table 2-1's
    lengths each its own type (BYTE is LOGICAL*1, DOUBLE PRECISION
    REAL*8, DOUBLE COMPLEX COMPLEX*16; CHARACTER's size is its length,
    0 for `*(*)`). `typeOf` checks a length against its type (BADLENGTH;
    NOLENGTH for BYTE, DOUBLE PRECISION, DOUBLE COMPLEX), and
    `characterType` a CHARACTER length, a constant or a parenthesized
    integer constant expression from 1 to 65535 (BADCHARLEN). A type
    statement's own length is checked once, at the statement; a name's
    after it, at the name. A passed length on a name that isn't a dummy
    argument, a function, or (subtask 4) a PARAMETER constant is
    PASSEDLEN.
  - **Using them:** declarations no longer stop at a type Phase 78
    compiles: every name gets its type and storage, and NOTIMPL ("the
    REAL*8 type") moves to the first expression that uses such a name
    (`uncompiledUse`, once per name), or to the FUNCTION statement of a
    function of such a type (`functionResult`). `Type.Compiled` is
    INTEGER, REAL, and LOGICAL longwords.
  - **Storage** at natural alignment (`Type.Align`): a longword's on a
    multiple of 4, a word's on 2, a COMPLEX's that of its parts,
    CHARACTER anywhere, never more than 4 (`$LOCAL` is LONG-aligned);
    `LocalSize` is rounded to a longword so the compiler's cells stay
    aligned. Nothing changed for Phase 75's programs, all longwords.
  - **IMPLICIT** (`implicit.go`): the unit's IMPLICIT statements are
    read first, so they type the header's names too; a letter typed
    twice is DUPIMPLICIT (the survey's choice), IMPLICIT NONE with
    another IMPLICIT IMPLNONE (D-2, 4.8). Under IMPLICIT NONE a name
    with no declaration is NOTYPE once the unit is read (`untyped`),
    and quiet until then (its type is Invalid); a subroutine's name
    needs none.
  - **Statement order** (`order.go`): Figure 1-1 as groups (IMPLICIT
    NONE, IMPLICIT, other specifications, statement functions,
    executable), each statement with the group it puts the unit in and
    the last it may come in: PARAMETER from IMPLICIT to the
    specifications, DATA from the specifications on, FORMAT, ENTRY,
    NAMELIST, and directives anywhere. Out of order is LATEDECL after an
    executable statement (as before), ORDER otherwise, naming the first
    statement that took the unit past its place.
  - **Statement functions** (`statementFunction`): F(p,...) = e before
    the executable statements, F not an array or dummy argument, each p
    a name, is a definition; F's class is `StmtFunction` (no storage;
    a type, which IMPLICIT NONE requires). A definition is NOTIMPL
    (Phase 79), a reference an error already reported; calling one,
    using it as a variable, or assigning to it is BADUSE; a second
    definition DUPSTMTFUNC; a dummy twice DUPDUMMY. A parenthesized
    item that isn't a name (AVG(A,B,3.) = ...) makes it an assignment
    to an array element, NOTARRAY.
  - **Messages:** ORDER, NOLENGTH, NUMLENGTH, BADCHARLEN, PASSEDLEN,
    IMPLNONE, DUPIMPLICIT, NOTYPE, DUPSTMTFUNC.
  - **Tests:** sema's `TestErrors` has cases for each type and length,
    IMPLICIT, the order, and statement functions; golden
    `sema/testdata/types.for` shows every type's symbol and `$LOCAL`
    place; `codegen` and `ir`'s `declare.for` compile mixed declarations
    and IMPLICIT NONE through to MACRO (gaps of `.BLKB` before aligned
    variables). The sema dump's type column is wider (13), so every
    golden's symbol lines moved.
  - Choices of ours, unconfirmed: `FUNCTION F*8` (a length with no type
    keyword) gives the length to F's implicit type; a CHARACTER*(*)
    external function is allowed (the survey's reading of 4.4.2); a
    statement function whose name is an intrinsic's is the statement
    function (6.2.1's own SINH example).
- **2026-10-30:** Subtask 4 done: PARAMETER and constant expressions
  (`internal/fortran/sema`).
  - **The evaluator** (`constant.go`, `constfuncs.go`) computes section
    4.11's constant expressions from the syntax tree, so a name that isn't
    a constant is never entered as a variable. Every type: INTEGER*2 and
    *4, REAL*4, *8, and *16 (F, D, and H through `vaxfloat`), COMPLEX*8
    and *16, LOGICAL, CHARACTER; every operator (`**` of an INTEGER
    exponent only, `//`, `.EQV.`, `.NEQV.`, `.XOR.`, character
    comparisons, COMPLEX equality); and section 4.11's functions, IMAG and
    AIMAG both. `Type.rank` and `higher` are section 2.3.1.2's whole
    table, COMPLEX*8 with REAL*8 or REAL*16 being COMPLEX*16. A real or
    COMPLEX literal keeps its exact value (`Const.literal`), so `1D0 +
    0.3333333` is `1D0 + 0.3333333000000000D0` as 2.3.1.2 says.
  - **PARAMETER** (`parameter.go`): both forms. The standard one converts
    the value to the name's type (`MU=1.23` is the INTEGER 1), a
    CHARACTER*(*) constant takes its value's length, and appendix A.4's
    takes the value's type whatever the name's letter. A constant is class
    `Constant`, with `Value` and no storage; the dump shows its value.
    `constInt` (bounds, CHARACTER lengths) is the evaluator's now, so
    those may use constants, `**`, and the functions.
  - **Executable statements:** a constant's name is its value; a type not
    compiled yet is NOTIMPL, once (`constantUse`). Forms compiled only as
    constants (`**`, the functions, `//`, `.EQV.` and its kin, REAL*8 and
    COMPLEX constants, comparing character constants) go to the evaluator
    (`folded`, first quietly to learn whether they are constant), and
    otherwise keep their NOTIMPL. Octal and hexadecimal constants stay
    NOTIMPL there (Phase 82), since an assignment's target decides their
    type.
  - **vaxfloat:** `Sqrt` (a COMPLEX's ABS) and `Text`.
  - **Messages:** UNDEFPOW, NOTCONST, DUPPARAM, TYPEAFTER, OLDPARAMTYPE,
    PARAMTYPE, BADCOMPLEX, INTRINARGS, BADARG, BADTYPELESS; INTRANGE and
    REALRANGE now name the type and its range (the same text for INTEGER
    and REAL).
  - **Tests:** `TestParameterValues` (values in every type, both forms),
    `TestErrors`' PARAMETER section, `TestExpressions`' folded forms,
    golden `sema/testdata/params.for` (section 4.11's and A.4's examples),
    and `testdata/fortran/params.for` end to end. The old NOTIMPL cases
    for what is now constant changed: `2.0 ** 3` and `'A' // 'B'` use a
    variable instead, `'A' .EQ. 'B'` moved to `TestExpressions`, and
    PARAMETER's two have no NOTIMPL.
  - Choices of ours, unconfirmed (no VAX FORTRAN to check): a COMPLEX
    product's or quotient's parts computed exactly and rounded once;
    powers of REAL and COMPLEX bases by repeated squaring, rounded at each
    product, from the reciprocal for a negative exponent (Phases 77 and 78
    must agree, so that a folded expression has the value the program
    would compute); zero to a power of zero or less an error; a REAL*16
    part making a COMPLEX constant COMPLEX*16; generic functions' arguments
    of one kind (BADARG); an octal or hexadecimal constant INTEGER*4 in the
    old form and beside no operand, and the other operand's type beside
    one; a literal's extra precision kept through a PARAMETER of its own
    type, and lost through a conversion.
- **2026-10-30:** Subtask 5 done: arrays (`internal/fortran/sema/arrays.go`).
  - **Bounds:** constant ones are computed by the evaluator and converted
    to INTEGER (2.2.3.1: "converted to an integer if necessary", so
    `DIMENSION A(2.5)` has two elements, where Phase 75 said BADBOUND);
    LOGICAL and CHARACTER ones are BADBOUND. More than seven dimensions
    is TOOMANYDIMS. An array over 2147483647 bytes is ARRAYSIZE, checked
    once the declarations are read, since its type may come after its
    dimensions.
  - **Adjustable arrays are declared and checked; their code is Phase
    79's.** A bound that isn't constant is kept as its syntax tree
    (`Dim.LowerExpr`, `UpperExpr`), and the array is NOTIMPL at that bound.
    It stays an array, so its uses' subscripts are still counted. On an
    array that isn't a dummy argument, or in a main program, it is
    ADJNOTDUMMY. The form is checked where it is read: the arithmetic
    operators, constants, and names, an array element or a function being
    BOUNDREF, anything else BADBOUND. Each name is checked after the
    declarations (`checkArrays`), since they may follow the bound: a dummy
    argument, INTEGER or REAL (BOUNDVAR, BOUNDREF for an array, BADBOUND).
    A type statement that changes the type of a name a bound has used is
    BOUNDTYPE, section 6.1.1.1's invalid example; one that repeats the type
    is fine. A variable in a common block is the other kind of name a bound
    may use: a unit with COMMON is incomplete until subtask 6, so these
    messages don't reach it, and subtask 6 must add the case to
    `checkBoundName`.
  - **Assumed-size arrays** take adjustable bounds too (4.5's `A2(N3:*)`,
    6.1.1.2's `A(1:N,1:*)`); `Symbol.AssumedSize` answers NOSIZE's
    question, since `Elements` is 0 for adjustable arrays as well.
  - **Element offsets:** `Symbol.ElementNumber` is an element's place in
    storage order for constant subscripts, or false if it is outside the
    array, for EQUIVALENCE and DATA; `Dim.Extent` is shared with the IR.
    Lowering's `mem` was n-dimensional already (Phase 75), and so were
    descriptors; goldens now show it.
  - **Messages:** ADJNOTDUMMY, BOUNDVAR, BOUNDREF, BOUNDTYPE, TOOMANYDIMS,
    ARRAYSIZE; BADBOUND's text.
  - **Tests:** `TestErrors`' cases, `TestElementNumber` (Figure 2-1's
    positions), goldens `sema/testdata/arrays.for` (2.2.3's, 4.5's, and
    6.1.1's forms), `ir/testdata/arrays.for` and `codegen/testdata/arrays.for`
    (offsets checked by hand), and `testdata/fortran/arrays.for` end to end,
    up to seven dimensions.
  - Choices of ours, unconfirmed (no VAX FORTRAN to check): a constant
    subscript outside an array's bounds isn't diagnosed in an executable
    statement, since `DIMENSION A(1)` for a dummy of any length is a common
    idiom and the manual leaves bounds to `/CHECK=BOUNDS` at run time; the
    2147483647-byte limit; a COMPLEX or LOGICAL bound rejected, though the
    manual's "arithmetic expression" might take COMPLEX; a bound's power of
    a variable (`N**2`) accepted, the manual's forbidding only array and
    function references.
- **2026-10-30:** Subtask 6 done: COMMON and EQUIVALENCE
  (`internal/fortran/sema/common.go`, `equivalence.go`).
  - **COMMON:** every form of 4.2: blank common, named blocks, a block
    named again continuing, array declarators. A unit's `Commons` are its
    `CommonBlock`s (name, psect, members, size); a name's `Symbol.Common`
    is its block and `Offset` its place there, with nothing aligned
    inside a block. Places wait until every declaration is read
    (`layoutCommon`), as a name's type and dimensions may follow.
    `checkBoundName` now takes a variable in a block as an adjustable
    array's bound, as subtask 5 left for this one, EQUIVALENCE's among
    them.
  - **EQUIVALENCE:** groups of names merged list by list, each name with
    its place in its group (the design notes' union-find, done by moving
    every member of one group, since the groups are small; the file's
    comment works an example through). A bare array is its first element,
    a multidimensional array's one subscript its element number from 1
    (4.6.1), a substring its first character's place in its variable or
    element. A group with a name in a block is placed there, lengthening
    the block at its end; any other goes in `$LOCAL` whole, when `layout`
    reaches its first name. Tables 4-1 and 4-2, Figure 4-1, and 4.6.3's two
    examples are in the tests.
  - **In the `.MAR` file:** a module can't let each unit contribute its own
    part of a block: MACRO goes on from where a psect was left, overlaid or
    not, so a second unit's part would follow the first's. So
    `writeCommons` writes each block once, at the top of the file, its
    psect with Table 10-1's attributes, a `BLK.$BASE` label, and a `.BLKB`
    of the longest declaration in the file; each unit's names are direct
    assignments, `UNIT.KILO = BLK.$BASE+4`. Subtask 7's DATA fits this: one
    image per block per module. In `$LOCAL`, the names of a group are one
    `.BLKB` labeled by the lowest, and assignments from that label. The IR's
    `Cell.Common` is the block; `Check` allows overlaps only between
    equivalenced names.
  - **Messages:** COMMONDUMMY, DUPCOMMON, COMMONNAME, COMMONSIZE, EQUIVONE,
    EQUIVFORM, EQUIVDUMMY, EQUIVCONST, EQUIVRANGE, EQUIVSUBSTR,
    EQUIVSUBRANGE, EQUIVCONFLICT, EQUIVBLOCKS, EQUIVCOMMON, EQUIVBEFORE.
    A dummy argument, function result, unit name, constant, or statement
    function in either statement is an error (BADUSE for those without a
    message of their own), and a name in COMMON or EQUIVALENCE is a variable
    from then on, so `F(1.0)` of it is NOTARRAY.
  - **What stays not implemented:** a type statement's initial values and
    DATA (subtask 7); BLOCK DATA units (subtask 7), so a block name isn't
    yet checked against one's name; RECORD in COMMON (the RECORD statement,
    Phase 82, leaves its unit incomplete); CDEC$ PSECT's changes to a
    block's attributes (Phase 82).
  - **Tests:** `TestErrors`' COMMON and EQUIVALENCE sections, goldens
    `sema/testdata/common.for` (the manual's examples; the offsets checked by
    hand against its tables and figures), `ir/testdata/common.for`,
    `codegen/testdata/common.for` (assembled by MACRO), and
    `testdata/fortran/common.for` end to end, its output worked out by hand:
    units sharing blocks under other names, a block lengthened by
    EQUIVALENCE and seen in full from another unit, a REAL's bits read as an
    INTEGER, and arrays overlaid in `$LOCAL`. A one-off run of two separately
    compiled modules, blocks of different lengths, shared correctly and took
    the longest; subtask 8 adds that as a test.
  - Choices of ours, unconfirmed (no VAX FORTRAN to check): the standard's
    rules the manual doesn't state (a dummy argument or function result in
    COMMON, a name in two blocks, two blocks associated); a block's name
    checked against the main program's, from Table 1-1, and against routines
    a unit references; a one-item list an error, from "must contain at least
    two"; a group in `$LOCAL` placed on its strictest member's boundary, the
    others where the lists put them; a one-subscript element number accepted
    only from 1 to the array's size, an assumed-size array being a dummy and
    so never in a list; a one-dimensional array's one subscript read as an
    ordinary subscript, by its lower bound; the symbol `BLK.$BASE`; the
    2147483647-byte limit on a block.
