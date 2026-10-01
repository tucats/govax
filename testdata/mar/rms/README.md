# The Phase 32 oracle: real MACRO's RMS macros, observed

These fixtures are `docs/PHASE-32.md`'s subtask 3. govax's own RMS macros
are written from the RMS Reference Manual (`docs/RMS-MACROS.md`). Where
the manual leaves a detail open (oracle questions O1–O14), the answer
comes from what real VAX MACRO makes of these programs: their objects
and `ANALYZE/OBJECT` reports. No listing of a macro expansion is made,
and nothing is taken from VMS's macro library.

Everything here except `vax/` is written by `gen.go`, from govax's own
tables (`internal/vmsdef`) and the manual:

    go run testdata/mar/rms/gen.go

| Files | Question | What they hold |
| ----- | -------- | -------------- |
| `def_*.mar` | O9, O13 | Each `$xxxDEF` alone, then a `.LONG` of every candidate name for its family. A name the macro defines is data in the object; one it doesn't is an external reference in the GSD. |
| `def_twice.mar` | O13 | `$FABDEF` called twice |
| `init_*.mar` | O2, O3, O6–O8 | Each block macro with defaults only, then a use of one of its symbols |
| `init_*_all.mar` | O3, O4 | Each block macro with every keyword, with distinctive values |
| `init_*_opt.mar` | O5, O10, O11 | One block per option or keyword value, and the special cases (XABKEY's FLG defaults, XABPRO's PRO forms) |
| `store_*.mar` | O12 | Each `_STORE` macro, each argument kind and register form |
| `services.mar` | O14 | Every service macro in each form |
| `err_*.mar` | O1, O5, O13 | Probes that may not assemble: unknown keywords, post-7.3 options, a global form |
| `rms.com`, `rmserr.com` | | The command procedures that assemble and analyze them |
| `exchange.cmd` | | The govax console script that builds the exchange volume |

Each probe was checked with govax's own assembler before the VAX run,
using empty stub macros whose arguments are Appendix A's keywords: all 73
assemble.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mar/rms/exchange.cmd

   This makes `testdata/disks/rms-exchange.dsk` (RD53 size, label
   RMSXCHG, gitignored) holding the programs and the two procedures.
2. Attach it to the simh VAX, mount it, set it as the default directory,
   and run:

       @RMS/OUTPUT=RMS.LOG
       @RMSERR/OUTPUT=ERRORS.LOG

3. Copy the results back into `vax/`, as for Phase 28:
   - every `.OBJ`, in the host variable-length record layout;
   - every `.ANL`;
   - `RMS.LOG` and `ERRORS.LOG`.

   Names are lowercased.

**Before committing, audit `vax/errors.log`.** MACRO's error messages may
quote the line in error, which for these probes could be a line of a
macro's expansion. Remove any such line (or the whole log) before
committing. Claude's clean-room hook (`.claude/hooks/cleanroom.sh`)
refuses that log, and the `vax/` directory as a whole, until the hook's
entry for it is removed. `rms.log` holds only the commands and MACRO's
informational messages; skim it too.

## The second round

The first round left three store-macro details open (docs/DEVIATIONS.md,
the Phase 32 entries). `gen.go` writes a second round of probes to settle
them:

| Files | What they hold |
| ----- | -------------- |
| `r2_order_*.mar` | Each store macro with every keyword in one call, in the manual's order and then reversed (and `$XABKEY_STORE` with all of POS0–POS7 and SIZ0–SIZ7), to show the order its moves come in |
| `r2_dvi_*.mar` | `$NAM_STORE DVI=` as a register, a register deferred, and an immediate |
| `r2_pro_*.mar`, `r2_uic_*.mar` | `$XABPRO_STORE PRO=` as a one-class list and as an address named with protection letters, and a one-element UIC= list, stored and initialized |

Build its volume, from the repository root:

    govax console < testdata/mar/rms/exchange2.cmd

That makes `testdata/disks/rms2-exchange.dsk` (RD53 size, label
RMSXCHG2). On VMS, with it as the default directory:

    @RMS2/OUTPUT=RMS2.LOG
    @RMSERR2/OUTPUT=ERRORS2.LOG

Copy the `.OBJ` and `.ANL` files and both logs into `vax/` as before, after
auditing `ERRORS2.LOG` (and skimming `RMS2.LOG`) for macro expansion
text: the probes in `RMSERR2.COM` may draw errors.
