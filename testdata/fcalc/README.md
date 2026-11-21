# fcalc, a scientific calculator in Fortran 90

The Fortran 90 program's milestone (`docs/planning/PHASE-111 - Fortran 90
milestone.md`; Part A is its design). fcalc reads lines at a `fcalc> ` prompt:
an expression's value is shown and kept as `ans`, `name = expr` assigns a
variable, and a line starting with a command word is a command. Every value is
complex, `COMPLEX(8)`.

## The sources

Each file is a module but the last, the main program, compiled in the order
`compiles.txt` gives, a FORTRAN command each:

| File | What it holds |
|---|---|
| `fcplatform.f90` | what differs by system: VMS's carriage control (`gfortran/fcplatform.f90` is gfortran's twin) |
| `fckinds.f90` | the kind `RK`, fcalc's limits (`LIMIT`, `SMALL`), the constants |
| `fcmessages.f90` | output (`SAY`, which keeps records to 132 characters) and the messages, `%FCALC-E-...` |
| `fcvalues.f90` | `TYPE VALUE`, its operators, guarded so nothing traps, and how a value is shown |
| `fclexer.f90` | the tokens |
| `fctree.f90` | `TYPE NODE`, a tree of pointers, and the recursive descent parser |
| `fcsymbols.f90` | the variables, a binary search tree, and the constants |
| `fceval.f90` | the evaluator |
| `fcalc.f90` | the main program: the prompt, the commands, assignment |

fcalc must be compiled `/G_FLOATING`: no kind of D_floating has the range of
300 `SELECTED_REAL_KIND(15, 300)` asks for. On govax, on a volume holding the
sources as `NAME.F90`:

```
$ FORTRAN/G_FLOATING FCPLATFORM
$ FORTRAN/G_FLOATING FCKINDS
  ... each file in compiles.txt's order ...
$ MACRO FCPLATFORM
  ... each file ...
$ LINK FCALC,FCPLATFORM,FCKINDS,FCMESSAGES,FCVALUES,FCLEXER,FCTREE,FCSYMBOLS,FCEVAL
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
- **Rounding.** Values are shown to 12 significant digits, where the two
  machines' last bits don't reach; a part of a complex value smaller than the
  other by more than the digits shown is shown as zero, and equality is to
  within 64 epsilons, so `0.1 + 0.2 == 0.3`.
