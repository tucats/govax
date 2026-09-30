# LINK fixtures (Phase 30)

These fixtures check govax's LINK against real VAX LINK on links of more
than one module (docs/PHASE-30.md, subtask 4c). `link.com` runs the VMS
side (`@LINK/OUTPUT=LINK.LOG`): it assembles the fixtures, links them with
maps, analyzes each image with `ANALYZE/IMAGE`, and runs the complete
programs.

- `defs.mar` defines the symbols the Phase 27 fixtures `extern`, `exprs`,
  `modes`, `general`, and `globals` (in `testdata/mar/`) refer to but
  don't define. `globals`'s weak reference `MAYBE` stays undefined.
- `extern` is also linked alone, to see what real LINK does with
  undefined symbols, and with `defs` in a user object library.
- `share1.mar` and `share2.mar` contribute to concatenated psects with
  different alignments and to an overlaid psect of different sizes, linked
  in both orders. The program exits with status `^X34`.
- `addr.mar` holds `LIB$PUT_OUTPUT`'s address with `.ADDRESS`, which
  needs a `.ADDRESS` fixup, and calls through it.
- `prog.opt` is an options file: the `share` modules, `STACK=`,
  `IDENTIFICATION=`, and `SYMBOL=`.

The files travel on an ODS-2 container govax builds, as in
`testdata/mar/README.md`. The results go in `vax/`.
