# Macro facility and librarian fixtures (Phase 28)

These are the fixtures for `docs/PHASE-28.md`'s subtask 10. Like the
Phase 27 ladder one directory up, they're assembled by real VAX MACRO on
the user's simh VAX running VMS 7.3, and those results are what govax is
checked against.

| File | What it covers |
| --- | --- |
| `usermac.mar` | User-defined macros: every argument form, concatenation, a macro that defines a macro, redefinition and `.MDELETE`, `\symbol`, `.NARG`, `.NCHR`, `.MEXIT`, the string operators, repeat blocks, `.NTYPE` of every operand form, created local labels, `.PRINT` |
| `qiow.mar` | `$ASSIGN_S`, `$QIOW_S`, `$DASSGN_S`, `$EXIT_S`: "Hello, world!" (all in govax's own STARLET.MLB too) |
| `rmscopy.mar` | `$FAB`, `$RAB`, `$RMSDEF`, and the RMS service macros: copies itself to `RMSCOPY.OUT` |
| `fabalign.mar` | A misaligned `$FAB`, whose alignment check prints a GENINFO message |
| `libmac.mar` | A macro library's source (`LIBMAC.MLB`): comments to squeeze, a continued `.MACRO` line, created labels, `.IRP` and `.WARN` inside a macro, a macro that defines one |
| `uselib.mar` | A program calling `libmac`'s macros, from a library named on the command line |
| `libsub1.mar`, `libsub2.mar` | The modules of an object library (`LIBOBJ.OLB`) |
| `libmain.mar` | A program linked against that library |
| `extra.mar` | Macros real LIBRARIAN adds to a copy of govax's macro library |

`macros.com` does the VMS side (`@MACROS/OUTPUT=MACROS.LOG`): it assembles
and analyzes every fixture, links and runs the programs, and makes
`LIBMAC.MLB` and `LIBOBJ.OLB` with LIBRARIAN, listing each in full. For
govax's objects and libraries on the volume (`GV_NAME.OBJ`,
`GV_LIBMAC.MLB`, `GV_LIBOBJ.OLB`) it runs ANALYZE/OBJECT, links and runs
the programs, lists and extracts from the libraries, has real MACRO and
LINK use them, and has LIBRARIAN change copies of them. It ends with
`DIRECTORY/FULL` of the objects and libraries (`FILES.LST`), for their
file attributes.

## The exchange volume

`exchange.cmd` builds it with govax, run from the repository root:

    govax console < testdata/mar/macros/exchange.cmd

It makes `testdata/disks/mac-exchange.dsk` (RD51 size, label MACXCHG,
gitignored), copies the fixtures and `macros.com` on, and writes govax's
objects and libraries there, assembling the copies on the volume. It
needs `testdata/disks/rq0-ra92.dsk`, the VMS system disk, for its
STARLET.MLB; `GV_QIOW` is assembled before that is mounted, so it uses
govax's own STARLET.MLB (unless the `vax.library` setting names a
directory holding VMS's).

The user attaches the volume to simh, mounts it on VMS, sets it as the
default directory, and runs `@MACROS/OUTPUT=MACROS.LOG`. The results come
back into `vax/`, as the Phase 27 ones did.
