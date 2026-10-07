# The Phase 45 system service macro probes

govax's own system macros (`internal/bootdata/files/starlet.mar`) have
`$ASSIGN_S`, `$DASSGN_S`, `$EXIT_S`, `$QIO_S`, and `$QIOW_S` and the RMS
macros, but none for the process-management services. The ones added for
Phase 45 (`$CREPRC`, `$DELPRC`, `$WAKE`, `$HIBER`, `$SCHDWK`, `$CANWAK`,
`$FORCEX`, `$SUSPND`, `$RESUME`, `$SETPRI`, `$SETPRN`, `$GETJPI(W)`,
`$GETDVI(W)`, `$CREMBX`, `$DELMBX`, `$SETIMR`, `$CANTIM`, `$WAITFR`,
`$SETEF`, `$CLREF`, `$READEF`) are written from the System Services
Reference Manual's argument lists, and checked against what real VAX MACRO
makes of these probes (the same method as `testdata/mar/rms`, Phase 32):
the objects and `ANALYZE/OBJECT` reports, never a listing of an expansion.

Everything here except `vax/` is written by `gen.go`:

    go run testdata/mp/macros/gen.go

| Files | What they hold |
| ----- | -------------- |
| `svc_*.mar` | One program per service. It calls the service's macro in the short form (`$NAME_S`, `CALLS`) and the long form (`$NAME`, an argument list), with no arguments, only the required ones, all of them, each optional one left out in turn, each one alone, and each one in every addressing form the macros must tell apart (`(R6)`, `4(R6)`, `@4(R6)`, `@#ADR1`, `ADR1[R7]`, `(R6)+`, `-(R6)`, `R6`, `#5`, `0`, ...) |
| `err_svc.mar` | Forms that may not exist: `$CREPRC_G` and the other `_G` forms, `ITEMLST` and `NODE` for `$CREPRC`, `NULLARG` for `$GETDVI`, `FLAGS` for `$CREMBX`, and a keyword that doesn't exist. MACRO may report errors; they go to `ERRORS.LOG` |
| `macros.com`, `macroserr.com` | Assemble and analyze them |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and copy the results back |

Each probe was checked with govax's own assembler before the VAX run, with
empty stub macros whose keywords are the ones in the program: all assemble.

## Round 1 and round 2

Round 1 (2026-10-07; its objects, analyses, and logs are in `vax/round1/`)
showed that the `_S` forms follow one model, that the unsuffixed forms
build an argument list in line (a `.LONG` count, then a `.LONG` for each
argument), and that several services have required arguments the probe had
wrong (`SCHDWK`'s `DAYTIM`, `SETPRI`'s `PRI`, `GETJPI`'s and `GETDVI`'s
`ITMLST`, `CREMBX`'s `CHAN`, `SETIMR`'s `DAYTIM`, ...), so a call without
one made a broken stream that could not be matched to its source line.
Round 2 (this directory's generator) names the true required arguments and
ends every call with a marker, `.LONG ^X7A7Axxxx`, that `dumpcode.go`
finds in the object. `svc_*.calls` lists the calls in order. To see what
real MACRO did for a probe:

    go run testdata/mp/macros/dumpcode.go -calls testdata/mp/macros/svc_wake.calls \
        testdata/mp/macros/vax/svc_wake.anl

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mp/macros/exchange.cmd

   This makes `testdata/disks/mp-macros.dsk` (RD53 size, label MPMACROS,
   gitignored) holding the programs and the two procedures.
2. Attach it to the simh VAX, mount it, set it as the default directory,
   and run:

       @MACROS/OUTPUT=MACROS.LOG
       @MACROSERR/OUTPUT=ERRORS.LOG

3. Copy the results back with

       govax console < testdata/mp/macros/copyout.cmd

   (every `.OBJ` in the host variable-length record layout, every `.ANL`,
   and the two logs).

**Before committing, audit `vax/macros.log` and `vax/errors.log`.**
MACRO's error messages may quote the line in error, which for these probes
could be a line of a macro's expansion. Remove any such line (or the whole
log) before committing. Claude's clean-room hook
(`.claude/hooks/cleanroom.sh`) refuses `vax/` until its entry for this
directory is added or the logs are audited.

## After the run

`internal/asm`'s `TestServiceMacroObjects` assembles each probe with
govax's own macros and requires the same object. govax's macros are written
first from the manual, and changed, probe by probe, until every object
matches.
