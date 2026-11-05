# FTNRTL under recursion and ASTs

`docs/PHASE-89 - reentrant FORTRAN.md`, subtask 8. The programs here need a MACRO helper or a
check `testdata/fortran`'s harness can't make, so each has a test of its own in
`internal/console/rtl89_test.go`. `testdata/fortran/recurrtl.for` is the rest of the subtask:
statements nested at depth, RECIO_OPE at depth, and unwinding across recursive levels.

| File | What it is |
| --- | --- |
| `fmtrace.for` | A FORMAT statement's parsed form under ASTs: the routine using it entered again by an AST at each third instruction of its first statement in turn, the FORMAT's cell emptied before each, so that some ASTs parse it while the main line is parsing it. Prints whether any did (`RACES`) and the heap's bytes lost over the run (`LOST`, 0). `TestRTL89FormatCache`, both optimizer modes. |
| `rtl89u.mar` | Its helpers: `testdata/for83`'s `TRACEON`, `TRACEOFF`, and `KEEPT`, and `CLEARFMT`, which frees a FORMAT block's parsed format and empties its cell (white-box: it knows FTNRTL's layouts). |
| `gone89.for` | FTNRTL's one known difference from FORRTL after an unwind (docs/DEVIATIONS.md, "A gone statement in an identical frame"), in the recursive form that makes it the usual case. `TestRTL89SameFrames` holds it. |

Before the fix (`ftn_format.mar`'s `FTN$$FMT_START`), `fmtrace.for` printed `LOST` of about 30,000
bytes: the parsed copy the AST's statement stored in the cell, overwritten by the main line's.
