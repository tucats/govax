# The Phase 83 probe of the STR$ routines

`docs/PHASE-83 - FORTRAN and VMS.md`'s subtask 2 (the author's answer 2). One MACRO program
calls the 26 string routines and STR$UPCASE, case by case (255 cases), and prints what each
returned, signaled, and stored: every kind of destination, positions at both ends, comparisons,
the JSB entries with R1 to R6, and `$GETMSG`'s texts for the facility's status codes. It is laid
out as `testdata/lib54`'s, whose README explains the output.

`gen.go` writes `str83.mar`:

    go run testdata/str83/gen.go

It runs on `testdata/for83`'s volume, by that directory's `for83.com`; its README has the steps.
`vax/str83-vax.txt` is VMS's output.

`TestStr83Oracle` (`internal/console/str83_test.go`) runs the program on govax and matches VMS's output line for
line but for `expectedStr83Differences`. **One flaw in the probe:** `gen.go` puts each case's label prefix in place of
every `%` in its data lines, so the WILD cases with `%` in a pattern really have `C123_` there. They hold govax to what
they asked, a plain mismatch, and `%` as a wildcard rests on the manual (`TestWildMatch`).
