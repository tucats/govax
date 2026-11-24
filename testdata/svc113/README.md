# The Phase 113 probe of the services subtask 8 added

`docs/planning/PHASE-113 - Fortran errata.md`'s subtask 8. One MACRO program calls the UTC
services (`$GETUTC`, `$ASCUTC`, `$BINUTC`, `$NUMUTC`, `$TIMCON`) and the device information
services (`$GETCHN`, `$GETDEV`) case by case (57 cases), and prints what each returned,
signaled, and stored. It is laid out as `testdata/str83`'s, whose README explains the output:
a case's name and R0 (and a signaled condition), then each value it shows, in longwords.
Buffers start as `^X5A` bytes, so a case shows what a service wrote.

`gen.go` writes `svc113.mar` and `def_dib.mar`:

    go run testdata/svc113/gen.go

govax's services were written from VSI's System Services Reference Manual, which gives each
UTC service's arguments and statuses but not the octaword's layout, and names `$GETCHN` and
`$GETDEV` only as obsolete. What is left open, the cases ask:

- **The UTC octaword**: govax takes it to be the DCE time service's form, 100-nanosecond ticks
  since 15-Oct-1582 UTC (bytes 0 to 7), a 48-bit inaccuracy, all ones for infinite (8 to 13),
  and a word of the TDF in minutes (12 bits, signed) under a version of 1 (14 and 15). The
  `BINUTC` and `TIMCON 1` cases show VMS's octawords whole; `GETUTC now` shows the last eight
  bytes of the current time's.
- **Its limits**: a version other than 1, a time before 17-Nov-1858, a TDF at the 12 bits'
  end, a finite inaccuracy (`ASCUTC`, `NUMUTC`, `TIMCON 0` cases).
- **`$NUMUTC`'s 13 words**: govax writes the local time's seven, the inaccuracy's five (days,
  hours, minutes, seconds, hundredths; each `FFFF` for infinite), then the TDF. VSI's text lost
  the manual's figure.
- **`$TIMCON`'s other flags and missing addresses** (govax: SS$_BADPARAM, SS$_ACCVIO).
- **The device information buffer**: govax's layout (the comment in
  `internal/corevms/getchn.go`) is a reconstruction. The `GETDEV` and `GETCHN` cases dump both
  buffers whole for NLA0:, the terminal, and the default disk; short buffers show the status
  for a cut block. `def_dib.mar` is `$DIBDEF GLOBAL`: decoded as `testdata/for83`'s
  `decode.go` decodes `def_opc.obj`, its object gives `$DIBDEF`'s names and offsets.

The probe runs with the simh VAX's time zone, US Eastern (SYS$TIMEZONE_DIFFERENTIAL -18000):
the cases' octawords are made for a TDF of -300, and `GETUTC now` should show `1ED4`.

It runs in Phase 113's probe round (subtask 12, `testdata/err113`), with the round's other programs. Until then
`TestSvc113Probe` (`internal/console/svc113_test.go`) runs it on govax and holds it to what
the manuals settle; with VMS's output in `vax/svc113-vax.txt`, it holds govax to that line for
line, but for `expectedSvc113Differences`. The terminal and disk cases fail on govax's test
console, which has neither device; VMS's run will have both.
