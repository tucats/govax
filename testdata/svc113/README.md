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

It ran in Phase 113's probe round (subtask 12, `testdata/err113`); `vax/svc113-vax.txt` is
VMS 7.1's output. `TestSvc113Probe` (`internal/console/svc113_test.go`) holds govax's run to it
line for line, with each device buffer's DIB$L_OPCNT masked, but for `expectedSvc113Differences`:
the terminal and disk cases, which describe the VAX's own OPA0: and volume (govax's test console
has neither).

What the run settled (Phase 113's subtask 13b; `internal/corevms/utc.go` and `getchn.go`):

- The octaword is as govax took it: every `BINUTC` and `TIMCON 1` octaword matched.
- `$BINUTC` of a delta time writes the infinite inaccuracy before SS$_IVTIME; text it can't
  read writes nothing.
- `$TIMCON` takes any flag but 0 as 1, and an address of 0 is an access violation *signaled*
  at that address (write reason for the one it stores to), not a status.
- `$ASCUTC` into 12 bytes, the date and its blank, is SS$_NORMAL; 5 bytes is SS$_BUFFEROVF.
- `$NUMUTC`'s layout is govax's; an inaccuracy of 0 gives days 17 (a zero quadword read as
  17-NOV-1858) and the rest 0.
- The DIB is `$DIBDEF`'s (decoded from `vax/def_dib.obj`): the fixed fields to 36, then
  DIB$T_DEVNAME's 76 bytes for the counted names (the controller's, then the volume label
  blank-padded to 12), then DIB$L_MAXBLOCK at 112; 116 bytes in all. VMS writes the fixed
  fields, the names, and MAXBLOCK, leaving the rest of the names' room as it was.
- `$GETDEV` with fewer than five arguments is SS$_INSFARG; `$ASSIGN` to NLA0: leaves its owner
  (PID 0, [1,1]): a shareable device isn't the assigner's.
