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
| `squares.for` | A main program with no PROGRAM statement (`SQUARES$MAIN`), DO, PRINT of a text and integers |
| `factorials.for` | INTEGER and REAL arithmetic, REAL output in G's F and E forms, mixed-mode conversions, truncating division, WRITE (\*,\*) |
| `primes.for` | Nested DO, logical IF, GO TO out of a loop, a labeled block IF, a whole array printed, a LOGICAL expression printed |
| `args.for` | SUBROUTINE, INTEGER and REAL FUNCTION, an untyped FUNCTION, arguments by reference: an actual changed, expressions copied, an array whole and from an element, RETURN in the middle, functions in expressions |
| `grades.for` | Block IF with ELSE IF and ELSE, a nested block IF, GO TO from a block to the loop's end, LOGICAL output, STOP with a display |
