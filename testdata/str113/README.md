# The Phase 113 probe of STR$'s arithmetic

`docs/planning/PHASE-113 - Fortran errata.md`'s subtask 7. One MACRO program calls STR$ADD,
STR$MUL, STR$DIVIDE, STR$RECIP, and STR$ROUND case by case (104 cases), the undocumented
STR$ELEMENT_R8 twice, and STR$MATCH_WILD with `%` (12 cases), and prints what each returned,
signaled, and stored. It is laid out as
`testdata/str83`'s, whose README explains the output, with one addition: a result's sign and
exponent start as 99 each, so `CS:` shows whether the routine wrote them (the sign first, then
the exponent).

`gen.go` writes `str113.mar`:

    go run testdata/str113/gen.go

govax's routines were written from the VMS 5.0 and OpenVMS 7.3 STR$ manuals, which give each
routine's arguments and one example. What the manuals leave open, the cases ask:

- **ADD and MUL**: a result's leading and trailing zeros, the sign and exponent of a zero
  result, an empty digit string, digits that aren't digits (a letter, a blank, `+`, `.`), a sign
  other than 0 or 1, exponents that overflow, the destination kinds (fixed, varying, a class
  of 99), too few arguments, and a result written over an operand.
- **DIVIDE**: total-digits (positive, 0, negative), the round-truncate-indicator's bits, a zero
  divisor by every spelling, a negative zero.
- **RECIP**: what the second number means. The manual calls it "the precision limit" and shows
  1 / -1000 with .0002 as `1 -3 1`; govax takes only its exponent, truncates there, and moves the
  trailing zeros into the exponent. The cases vary B's digits, sign, and exponent, and round
  numbers whose reciprocals end early (8, .25) or never (3, 6, 7).
- **ROUND**: places below 1, flags other than 0 and 1, a carry past the places (95 to one
  place), zeros.
- **ELEMENT_R8**, which no manual describes: R1 as the element number by value (govax's guess,
  as STR$LEFT_R8 takes its position), then as an address.
- **MATCH_WILD with `%`**, which `testdata/str83`'s probe couldn't ask: its label prefix
  replaced every `%` in a case's data. Here a `%` in a text is written `<37>`, which the prefix
  leaves alone (Phase 113's subtask 12).

VMS 7.1's STR$RECIP never returned from one of its 118 cases, `RECIP 6 0e-4` (a precision limit whose
digits are `0`), so the round took it out (`gen.go` says where it was). `vax/str113-vax.txt` is
the second run, of 117 cases; VMS's DCL wrote it after the first run's lines in the same file
(`DEFINE/USER SYS$OUTPUT` of an existing file), which were taken off.

It ran in Phase 113's probe round (subtask 12, `testdata/err113`), with the round's other
programs. `TestStr113Probe` (`internal/console/str113_test.go`) runs it on govax and holds it to
`vax/str113-vax.txt` line for line, but for `expectedStr113Differences` (ROUND to -1 places,
whose VMS result is 65,535 bytes from beyond the digits, and the registers STR$ELEMENT_R8
leaves). Subtask 13 made govax's routines VMS's; `internal/librtl/strmath.go` says what the run
showed.
