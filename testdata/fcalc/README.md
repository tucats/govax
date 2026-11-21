# fcalc, a scientific calculator in Fortran 90

The Fortran 90 program's milestone (`docs/planning/PHASE-111 - Fortran 90
milestone.md`; Part A is its design). fcalc reads lines at a `fcalc> ` prompt:
an expression's value is shown and kept as `ans`, `name = expr` assigns a
variable, and a line starting with a command word is a command. Every value is
complex, `COMPLEX(8)`, or a vector or matrix of complex values: `[1, 2; 3, 4]`.

## The sources

Each file is a module but the last, the main program, compiled in the order
`compiles.txt` gives, a FORTRAN command each:

| File | What it holds |
|---|---|
| `fcplatform.f90` | what differs by system: VMS's carriage control (`gfortran/fcplatform.f90` is gfortran's twin) |
| `fckinds.f90` | the kind `RK`, fcalc's limits (`LIMIT`, `SMALL`), the constants |
| `fcmessages.f90` | output (`SAY`, which keeps records to 132 characters) and the messages, `%FCALC-E-...` |
| `fcvalues.f90` | `TYPE VALUE`, its operators, guarded so nothing traps, and how a value is shown |
| `fcarrays.f90` | `TYPE QUANTITY`, a scalar or an array through a pointer, its ownership rule, and how an array is shown |
| `fclexer.f90` | the tokens |
| `fctree.f90` | `TYPE NODE`, a tree of pointers, and the recursive descent parser |
| `fcsymbols.f90` | the variables and the user's functions, each a binary search tree, and the constants |
| `fcfuncs.f90` | the built-in functions: their table, and each function, real and complex, guarded so nothing traps |
| `fcmatrix.f90` | the matrix product, LU (solve, inverse, determinant), powers, rank, norms, eigenvalues (Hessenberg and shifted QR), and the functions that make and rearrange arrays |
| `fceval.f90` | the evaluator: element-by-element operations, matrix literals, indexes, and a frame of parameters for each call of a user's function |
| `fcalc.f90` | the main program: the prompt, the commands, assignment, function definitions |

The sources are commented for a reader learning Fortran 90 and numerical
programming: each Fortran 90 feature is explained where it is first used,
and each numerical method where it is applied (guarding against overflow
and underflow, principal branches, cancellation, series and continued
fractions, Lanczos's gamma function, LU with partial pivoting, Householder
reflections and the shifted QR iteration).

fcalc must be compiled `/G_FLOATING`: no kind of D_floating has the range of
300 `SELECTED_REAL_KIND(15, 300)` asks for. On govax, on a volume holding the
sources as `NAME.F90`:

```
$ FORTRAN/G_FLOATING FCPLATFORM
$ FORTRAN/G_FLOATING FCKINDS
  ... each file in compiles.txt's order ...
$ MACRO FCPLATFORM
  ... each file ...
$ LINK FCALC,FCPLATFORM,FCKINDS,FCMESSAGES,FCVALUES,FCLEXER,FCTREE,FCSYMBOLS,FCFUNCS,FCEVAL
$ RUN FCALC
```

On gfortran, with `gfortran/fcplatform.f90` in place of `fcplatform.f90`, in
the same order: `gfortran -std=f95 gfortran/fcplatform.f90 fckinds.f90 ...
fcalc.f90`. The sources keep to Fortran 90: nothing Fortran 95 added (`I0`,
`NULL()`, default component values, `ELEMENTAL`, `PURE`, `FORALL`).

## The scripts

Each `name.in` is what a user types, and `name.out` what fcalc prints, worked by
hand from the mathematics and fcalc's rules (Part A's A.1 and A.3), then
compared with govax's and gfortran's runs, which print the same lines. Each
script starts with `set echo`, so the transcript shows each line after its
prompt, as a terminal does.

| Script | What it covers |
|---|---|
| `basics` | numbers and how they are shown, every operator and its precedence, the constants, variables and `ans`, LIST, CLEAR, SET [NO]ECHO, comments, and the messages |
| `complex` | complex values: their display, arithmetic, comparisons, and powers |
| `functions` | every built-in function: roots, exponentials, logarithms, trigonometry in each angle mode (SET ANGLE), hyperbolic functions, complex parts, rounding, whole numbers, the special functions, lists, IF, and random numbers (only their properties), with their errors |
| `userfuncs` | functions the user defines: parameters and variables, calls between functions, recursion and its limit, redefinition, LIST and CLEAR, and the errors of definitions and calls |
| `vectors` | vectors: literals and joining, the functions that make them, indexes (ranges, vector subscripts), assigning elements, element-by-element operations and functions, list functions of arrays, sizes, reshape, cumsum, sort, dot, cross, norms, transposes, users' functions of vectors, long vectors' display and LIST, and the errors |
| `matrices` | matrices: literals and blocks, sections, element-by-element and matrix products, transposes, powers, determinants, inverses, solves, trace, rank, norms, kron, eigenvalues (SET DIGITS 10 for some), the display beside the largest element, guarded products out of the safe band, users' functions of matrices, and the errors |

The `functions` values were checked against Python's `math` and `cmath` as a
second method, and the `vectors` and `matrices` values by hand and in Python
(characteristic polynomials' roots for the eigenvalues, exact fractions for
the inverses). Each of the two ends with `SHOW MEMORY` after `CLEAR ALL`,
which must count no arrays and no tree nodes in use: a leak, of an
operation's operand or after an error, shows there.

`internal/console`'s `TestFcalc` builds fcalc and runs every script;
`internal/dclcmd`'s `TestFcalcCompiles` compiles it, quickly, without a console.

## What the two systems make fcalc do

- **Carriage control.** VMS treats a terminal record's first character as
  carriage control; gfortran prints it. `fcplatform.f90`'s `CC` is a blank on
  VMS and nothing on gfortran, and every line fcalc writes starts with it.
- **Record lengths.** VMS's FORTRAN writes terminal records of up to 133 bytes
  and reads lines of up to 199 characters. fcalc writes a longer line in records
  of 132 characters, and takes lines of up to 199 (`LINELONG` past that); a
  too-long line isn't echoed, since VMS gives none of it.
- **Arithmetic.** A VAX traps where IEEE makes an infinity, and its range is
  smaller: no value fcalc keeps is larger than 1e300 or, but for zero, smaller
  than 1e-300, and each operation checks its operands first (`fcvalues.f90`).
  Built under `/CHECK=UNDERFLOW` (in `/CHECK=ALL`), govax faults on floating
  underflow too, so no operation may compute a value below the VAX's smallest
  either; fcalc arranges its comparisons so none does.
- **Rounding.** Values are shown to 12 significant digits, where the two
  machines' last bits don't reach; a part of a complex value smaller than the
  other by more than the digits shown is shown as zero, and equality is to
  within 64 epsilons, so `0.1 + 0.2 == 0.3`. An array's elements are shown
  beside its largest, so the noise a computation leaves (`1e-17` beside `1`
  in `a * inv(a)`) is shown as zero on both; eigenvalues are sorted with
  real parts that differ only that much counted the same, so their order
  doesn't depend on either machine's last bits either.
