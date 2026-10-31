# The FORTRAN end-to-end programs

`docs/PHASE-75 - FORTRAN first light.md`'s subtask 8: each `name.for` is
a FORTRAN program and `name.out` what running it prints at the terminal,
followed by SHOW SYMBOL's line for `$STATUS`. `TestFortranPrograms`
(`internal/console/fortranprograms_test.go`) copies each source to a
volume, runs FORTRAN, MACRO, LINK, and RUN on govax, checks that the
first three said nothing, and compares RUN's output with `name.out`.
Every program runs with paging off; those in `fortranPagingPrograms`
run with paging on too.

The expected output is written from the language's rules (the *VAX
FORTRAN Language Reference Manual*, and the phase doc's choices for
list-directed output), not copied from a run: there is no VAX FORTRAN
to check against. To add a program, add both files; this directory's
`.gitignore` keeps `*.out` from the repository's own rule.

| Program | What it exercises |
| ------- | ----------------- |
| `average.for` | The manual's first program (14.1) and the internals tour's: a REAL function given an array, a function reference in a PRINT list |
| `squares.for` | A main program with no PROGRAM statement (`SQUARES$MAIN`), DO, PRINT of a text and integers |
| `factorials.for` | INTEGER and REAL arithmetic, REAL output in G's F and E forms, mixed-mode conversions, truncating division, WRITE (\*,\*) |
| `primes.for` | Nested DO, logical IF, GO TO out of a loop, a labeled block IF, a whole array printed, a LOGICAL expression printed |
| `args.for` | SUBROUTINE, INTEGER and REAL FUNCTION, an untyped FUNCTION, arguments by reference: an actual changed, expressions copied, an array whole and from an element, RETURN in the middle, functions in expressions |
| `params.for` | PARAMETER in both forms, constants in array bounds and expressions, a CHARACTER\*(\*) constant, `**`, `//`, comparison, and intrinsic functions of constants |
| `arrays.for` | Arrays of up to seven dimensions: lower bounds of every sign, bounds from constants and REAL expressions, column-major order seen through whole-array output and one-dimensional dummies, assumed-size dummies with lower bounds, an array passed from an element, a REAL subscript |
| `common.for` | COMMON, blank and named, under other names in other units; a block lengthened by EQUIVALENCE and seen whole from another unit; a REAL's bits read through an INTEGER; arrays overlaid in `$LOCAL` |
| `data.for` | DATA and initial values in type statements, in `$LOCAL` and in COMMON from a BLOCK DATA named in EXTERNAL; a value kept by SAVE from call to call; section 4.3's byte rules read through EQUIVALENCE (numbers converted, octal, hexadecimal, Hollerith, and character constants, codes for CHARACTER\*1 items); implied-DO lists over two dimensions, with steps |
| `overlay.for` | EQUIVALENCE overlays: a matrix filled through a vector, a multidimensional array's element by its number, two lists chained into one group, a LOGICAL's and a REAL's bits read and changed through an INTEGER, an overlay's neighbors left alone, a common block under an overlay in two units |
| `tables.for` | Tables set by DATA and put to use: days in the months, a binary search, an interpolation, tax brackets in a common block from a BLOCK DATA, a matrix product of matrices given by rows and by columns |
| `store.for` | The internals tour's program for storage (section 8): PARAMETER in a bound, COMMON from a BLOCK DATA, EQUIVALENCE, DATA and initial values |
| `lengths.for` | INTEGER and LOGICAL of every length: unsigned constants in short variables, word arithmetic wrapping, a small constant beside a word, mixed lengths, the logical operators bit by bit, conversions, short conditions, a word DO variable and function, short arguments with subscripts, OPTIONS /NOI4 for one unit |
| `reals.for` | REAL\*8 in D_floating and REAL\*16: a REAL\*8 constant's digits and a REAL\*4's widened, rounding back to REAL\*4, mixed types, G editing's two forms at each width, a REAL\*8 DO variable, truncation to an integer, a REAL\*8 function, a REAL\*8 array changed by a subroutine, expressions that spill |
| `realsg.for` | `reals.for` in G_floating, each unit's OPTIONS /G_FLOATING saying so: 15 digits and three-digit exponents, and a value past D_floating's range |
| `complex.for` | COMPLEX\*8 and COMPLEX\*16 in D_floating: constants and their output, sums, differences, negation, mixed with REAL and INTEGER, products in line and by OTS$MULCD_R3, quotients by OTS$DIVC and OTS$DIVCD_R3, conversion to the real part, .EQ. and .NE., arrays with subscripts that aren't constant, a COMPLEX function, a COMPLEX argument changed by a subroutine, COMPLEX\*8 with REAL\*8, a record too full for the next value |
| `complexg.for` | `complex.for` in G_floating, each unit's OPTIONS /G_FLOATING saying so, and a product past D_floating's range |
| `chars.for` | CHARACTER values: assignment filling with blanks and cutting, substrings with constant bounds and with bounds that aren't (as values, targets, and output items), overlapping moves, a whole array and its elements' substrings, comparisons as values and as conditions with the shorter padded, ASCII's order past 127, a CHARACTER's bytes through EQUIVALENCE, a common block another unit changes |
| `strings.for` | CHARACTER arguments and concatenation: variables, constants, substrings, elements, and expressions passed by descriptor; passed-length dummies and one of a declared length; a dummy changed, and a parenthesized argument's copy; arrays passed whole, from an element, passed on, and seen with shorter elements; concatenations in a temporary, one overlapping its target, and ones of a passed-length dummy in a dynamic string, as values, arguments, output items, and conditions |
| `powers.for` | `**` on every type: INTEGER, REAL, and REAL\*8 and REAL\*16 powers to constant exponents from 1 to 8 in line, in the `OTS$` routines' order; larger, negative, and zero exponents and ones that aren't constant by `OTS$POWJJ`, `POWII`, `POWRJ`, `POWDJ`, `POWHJ_R3`; REAL exponents by `POWRR`, `POWDD`, `POWHH_R3`, and a REAL\*4 with a REAL\*8 by `POWRD` and `POWDR`; an INTEGER\*2 power wrapping in a word; a base computed once, `-X**2`, powers in subscripts and a condition; COMPLEX powers always by `POWCJ`, `POWCC`, `POWCDJ_R3`, `POWCDCD_R3`; and a unit under OPTIONS /G_FLOATING with `POWGJ`, `POWGG` (a REAL\*4 with a REAL\*8 too), `POWCGJ_R3`, `POWCGCG_R3` |
| `intrinsics.for` | The intrinsic functions written in line: ABS, MAX, MIN, DIM, SIGN, and MOD of INTEGERs, REALs, and words, generic and specific; INT, NINT, IFIX, ZEXT, REAL, DBLE, and footnote 10's default integer (a unit under OPTIONS /NOI4); COMPLEX parts, CMPLX, DCMPLX, CONJG, DPROD; IAND, IOR, IEOR, NOT, ISHFT, IBITS, IBSET, IBCLR, BTEST on words and longwords, and ISHFTC by FTNRTL's `FTN$IISHFTC` and `FTN$JISHFTC`; LEN, INDEX, ICHAR, CHAR, and the lexical comparisons, of passed-length dummies too; the choosing functions in a condition, a DO loop's limit, a subscript, and a statement with a dynamic string |
| `jumps.for` | The jumps of sections 5.6 and 5.7.1: the computed GO TO in range and out of it, with INTEGER, INTEGER\*2, and REAL indexes; ASSIGN and the assigned GO TO with a list and without, in the main program and through a dummy argument; the arithmetic IF of INTEGER, INTEGER\*2, REAL, and REAL\*8 values, with three labels and with two the same |
| `stops.for` | STOP from a subroutine, its display of digits kept as written; RETURN and END in a subroutine and in a function, whose value is the one last assigned to its name |
| `stopdo.for` | STOP as a DO loop's terminal statement, which ends the program on the first pass |
| `loops.for` | The loops of sections 5.3 and 5.4: DO without a label, ended by END DO, with positive, negative, and REAL steps, nested, and run no times; DO WHILE ended by END DO, by a labeled terminal statement, and by a labeled END DO, and run no times; a jump to a labeled END DO for the next pass; a DO and a DO WHILE sharing a terminal statement; a jump out of a loop, and the extended range, out and back in every pass |
| `grades.for` | Block IF with ELSE IF and ELSE, a nested block IF, GO TO from a block to the loop's end, LOGICAL output, STOP with a display |
| `mathfns.for` | The intrinsic functions MTHRTL computes, called with their arguments by reference: square roots, exponentials, logarithms, the trigonometric functions in radians and degrees, and the hyperbolic ones in F, D, and H; AINT, ANINT, and MOD of REALs; the complex functions, F values in R0 and R1 and D values through a hidden argument, as REAL\*16 values come back too; ISHFT by a count that isn't constant (`MTH$JISHFT`, `MTH$IISHFT`); a function of a function, in a subscript, and in a condition. Its output was worked out to 80 digits apart from govax, each value rounded to its format's bits |
| `mathfnsg.for` | `mathfns.for`'s REAL\*8 and COMPLEX\*16 functions in G_floating, the unit's OPTIONS statement saying so (`MTH$GSQRT`, `MTH$CGSQRT`, ...), and values past D_floating's range |
| `mixed.for` | Mixed arithmetic end to end (Phase 78, subtask 10): a value carried through every type and back by Table 3-1 (a byte keeping an INTEGER\*2's low bits); operations between types of every rank, LOGICAL values as integers, COMPLEX\*8 with REAL\*8 and COMPLEX\*16 with REAL\*16; a REAL\*4 constant keeping its digits in a REAL\*8 expression where a REAL\*4 variable doesn't; REAL subscripts and DO limits, an INTEGER\*2 and a REAL\*8 array summed by a subroutine; mixed types in conditions; one polynomial by Horner's rule in REAL\*8, REAL\*16, and COMPLEX |
| `mixedg.for` | `mixed.for` in G_floating, each unit's OPTIONS statement saying so |
| `text.for` | Character handling end to end: a line split into words by INDEX, sorted by LLT, a word reversed in place, palindromes, lowercase and a Caesar cipher through ICHAR, CHAR, and MOD, the letters counted, a number written as its digits, a title centered by LEN, a sentence built of substrings and cut to its target's length |
| `numeric.for` | Numerical methods end to end, each result checked against a tolerance or rounded where its last digits depend on the rounding along the way: Newton's method beside SQRT, a quadratic's real and complex roots by CSQRT, Simpson's rule over SIN, pi from ATAN in F, D, and H, degrees, logarithms, a hyperbolic identity, a mean and standard deviation, compound interest, a linear system by elimination, Euclid's algorithm |

## Programs of several files

Each directory of `modules/` is one program, run by `TestFortranModules`
in both paging modes. Its `.for` files are compiled and assembled
separately, and any other file but the two below (an INCLUDE file) is
only copied to the volume. `links.txt` has one LINK command's modules per
line: the program is linked each way, and must print `name.out`, named
for the directory, every time.

| Program | What it exercises |
| ------- | ----------------- |
| `modules/store` | The manual's example of a program of several files (14.1): one block seen from two files, its values from a BLOCK DATA in a third |
| `modules/ledger` | Four files sharing common blocks, blank and named: one block's layout from an INCLUDE file, another's declared at three lengths and in another file lengthened by EQUIVALENCE, its values from a BLOCK DATA in a file of its own that EXTERNAL names; the modules linked in three orders, so that the main program's short declaration, the module with the values, or another view of the block comes first |
