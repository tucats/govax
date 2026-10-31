# The Phase 80 probe: LIB$TPARSE, LIB$TABLE_PARSE, and $FILESCAN

`docs/PHASE-80 - FORTRAN formatted IO.md`'s subtask 10. One MACRO program runs
the parser and the service, case by case, on tables its own macros build, and
prints what each returned and stored. VMS's run settles what the manuals leave
open, and govax's macros are held to VMS's tables byte for byte.

`gen.go` writes everything here but this README and `vax/`:

    go run testdata/tpa80/gen.go

| File | What it holds |
| ---- | ------------- |
| `tpa80.mar` | The program. Sections: the `$TPADEF` and `$FSCNDEF` values, undefined names shown as such (TPA$_DECIMAL_64 and the like); a table's bytes, with every field and flag of a transition; which of the 256 characters each symbol type takes, and which end a keyword; every grammar under LIB$TPARSE and again under LIB$TABLE_PARSE; type bytes no macro makes; argument block counts; bad addresses; `$FILESCAN` |
| `tpa80b.mar` | Forms a macro call may trouble (`<';'>`, `'!'`, lowercase `tpa$_exit`, `'AB*'`), with no `$TPADEF`, to see if `$INIT_STATE` defines the symbols. Assembled, never run |
| `tpa80c.mar` | `'~'`, which govax's macro refuses, for VMS's answer. Assembled, never run |
| `tpa80.com` | Assembles, links, runs, and analyzes them |
| `exchange.cmd`, `copyout.cmd` | Build the exchange volume, on DUA2 since the startup procedure may mount DUA1, and copy the results off |

## How the program works

`CASE` fills in the argument block, the fields the parser may leave alone preset
to ^X55555555, and calls the routine under `HANDLER`, which records a condition
and unwinds. A case prints

    name
      action r0low=n argc=n block=n     once per call of an action routine
        rest=n@o tok=n@o char= num= param= opt=
      R0=status signal=condition
      tok=n@o "text"
      rest=n@o "text"
      char= num= param= opt=
      cell=value [desc @o]

where `n@o` is a length and an offset from the string's start, so nothing depends
on where the program is. `ACT` is the action routine most grammars use: it logs
its call and returns its argument, so `,ACT,,,1` accepts, `,ACT,,,0` rejects, and
`,ACT,,,H_1230` rejects with a status. `block=0` says it was given the argument
block, through AP (LIB$TPARSE, `argc=8`) or as its one argument (LIB$TABLE_PARSE,
`argc=1`). Hexadecimal arguments and numeric types are symbols, as round 1 showed
they must be.

## Round 1

`vax/round1/`. Real MACRO refused 15 lines of `tpa80.mar`, so nothing ran, but the
partial objects' analyses gave the psects' attributes. VMS's `$TRAN` refuses a
bracketed `<^X...>` as its last argument (UNTERMARG), a type starting with a digit,
`8` or `1+1` (ILLIFCOND, IFDIRSYNX), and `'<'` and `'>'` (a generated INVALID
CHARACTER error). govax's macro takes all of these. The logs of MACRO's messages,
`*.mlg`, may echo expansions and are for the author to audit before they're read.

## The VAX run

1. Build the volume from the repository root: `govax console < testdata/tpa80/exchange.cmd`.
2. Attach `testdata/disks/tpa80-exchange.dsk` to the simh VAX as a spare drive (any
   unit; the program doesn't name the device), log in as SYSTEM, and:

        $ MOUNT/OVERRIDE=IDENT DUA1:
        $ SET DEFAULT DUA1:[000000]
        $ @TPA80/OUTPUT=TPA80.LOG
        $ SET DEFAULT SYS$LOGIN
        $ DISMOUNT DUA1:

3. Copy the results off: `govax console < testdata/tpa80/copyout.cmd`.

`TPA80.MLG`, `TPA80B.MLG`, and `TPA80C.MLG` are MACRO's messages, in logs of their
own; look them over before they're read here. Nothing in the program should hang: a
looping grammar has input to use up, and each failure returns.

## On govax

`TestTPA80Oracle` (`internal/console/tpa80_test.go`) runs the program and matches
`vax/tpa80-vax.txt` line for line, and skips that comparison until the file is
here. `TestTPA80Manual` holds some cases to the manuals' words regardless.
`TestFor80RunsOnFORRTL` runs VMS's FORRTL over govax's LIB$TPARSE and holds the
namelist cases to VMS's `testdata/for80/vax/for80-vax.txt`, on the tables VMS's own
macros built.
