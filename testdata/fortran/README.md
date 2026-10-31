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
| `grades.for` | Block IF with ELSE IF and ELSE, a nested block IF, GO TO from a block to the loop's end, LOGICAL output, STOP with a display |

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
