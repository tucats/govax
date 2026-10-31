# The Phase 80 probe of NAMELIST I/O

`docs/PHASE-80 - FORTRAN formatted IO.md`'s subtask 11. One table of cases in
`gen.go` becomes two programs that print the same lines: `nml80.mar` calls VMS's
FORRTL.EXE (only what it does is used, never its code, by the author's ruling),
and `nml80f.for` is the same cases as FORTRAN statements, compiled by govax and
run on FTNRTL. FORRTL's output is FTNRTL's requirement.

`gen.go` writes everything here but this README and `vax/`:

    go run testdata/nml80/gen.go

| File | What it holds |
| ---- | ------------- |
| `nml80.mar` | The cases, on FORRTL: 786, 536 READs (two at the input's end) and 250 WRITEs |
| `nml80f.for` | The same cases as FORTRAN, for FTNRTL |
| `nml80r.com` | Runs either image (its parameter) with the records the READs take |
| `nml80e.mar`, `nml80e.for` | One READ with no handler, for a condition's message |
| `nml80e.com` | Runs either once per set of records: 44 conditions, each with its status |
| `nml80.opt`, `nml80.com` | Build and run everything on VMS |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts for a VMS exchange volume |
| `vax/nml80-forrtl.txt` | `NML80`'s run on VMS's FORRTL over govax |
| `vax/nml80e-forrtl.txt` | `NML80E.COM`'s run on FORRTL over govax: the messages and statuses |
| `vax/nml80t-forrtl.txt` | `NML80E` at a terminal, given `?` and `=?` in the search and in the group |

## How the program works

A read case resets one group of every type (scalars, arrays of one and two
dimensions, one with negative bounds, character arrays, a long name), reads its
lines (`SYNC` passes over the records up to the case's number, as in
`testdata/for80`), and prints

    #n <the case's records, / between them>
    =n e=c                     c is FOR$ERRSNS's FOR number, 0 if none
    $GRP ... $END              the group, written by a namelist WRITE

so the stored values show in FORRTL's own output. A write case writes a group of
its own. Each case's lines are followed by ` $GRP $END`, ` '`, and ` $END`, which
end a group, a group still being searched for, or a character constant still open.

The sections: finding the group; ending it; separators between assignments;
scalars and how many values; names; arrays' values and subscripts; character
values and substrings; numbers, logicals, and complex values to each type;
questions at a file; G_floating; round 2 (what ends a value, complex parts,
references, repeats, questions within the group); and writes: every type, names
of 1 to 31 characters, arrays and repeats, where records break (texts of every
length around the record's end, repeats, complex values), and G_floating.

## On govax

`TestNML80RunsOnFORRTL` (`internal/console/nml80_test.go`) runs `nml80.mar` and
`nml80e.mar` on FORRTL.EXE from a local-only volume (or skips) and holds them to
`vax/`; with `NML80_WRITE` set it rewrites those files. `TestNML80Oracle` holds
FTNRTL's run of `nml80f.for` to `vax/nml80-forrtl.txt` line for line,
`TestNML80Messages` the first line of each message and each status to
`vax/nml80e-forrtl.txt`, and `TestNML80Prompt` the terminal's answers to
`vax/nml80t-forrtl.txt`.

The probe has run only on FORRTL over govax, as `testdata/for80`'s round 3 has:
FORRTL gave VMS's output over govax in every case of `testdata/for80`'s namelist
sections, and its namelist input runs on govax's `LIB$TPARSE`, which
`testdata/tpa80` holds to VMS. `nml80.com` runs it on VMS when wanted:
`exchange.cmd`, `@NML80/OUTPUT=NML80.LOG` with the volume's `[000000]` the
default directory, then `copyout.cmd`.
