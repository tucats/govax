# Phase 29's VMS round

docs/PHASE-29.md, subtask 14. Subtask 1's probe (`testdata/mar/list`)
gave real MACRO's and real LINK's output for govax to match. This round
goes the other way: VMS checks what govax makes, and runs it.

- Does `ANALYZE/OBJECT` find govax's objects free of errors?
- Does `ANALYZE/IMAGE` find govax's images free of errors?
- Does real LINK link govax's objects?
- Do the failing programs print the same traceback, whether govax or
  real LINK linked them, as real MACRO's did in the probe's `list.log`?
- What title header record does a module with no `.TITLE` get?

Debugger records were deferred (subtask 12), so nothing here uses
`/DEBUG`.

| File | What it is |
| --- | --- |
| `notitle2.mar` | No `.TITLE`, `.IDENT`, or `.SBTTL`. Does the probe's `notitle.obj` title header record (`"\x01 "`) come from its `.SBTTL`? |
| `notitle3.mar` | No `.TITLE` or `.IDENT`, and a `.SBTTL` after the first statement |
| `exchange.cmd` | govax console commands that build the volume |
| `round.com` | The VMS side: `@ROUND/OUTPUT=ROUND.LOG` |
| `copyout.cmd` | govax console commands that copy the results into `vax/` |

The other sources are the probe's own, copied from `testdata/mar/list`:
`LCTL`, `BINARY`, `SYMTAB`, `NOTITLE`, `XREF`, `TRACE`, `FAILMAIN`,
`FAILSUB`, and `FAILSIG`. They're every probe source govax assembles,
except `DBGSRC`, which is about debugger records. No source calls a
system macro, so nothing on the volume comes from VMS's libraries, and
the clean-room hook needs no entry for these files.

## The exchange volume

`exchange.cmd` builds it with govax, run from the repository root:

    govax console < testdata/mar/round/exchange.cmd

It makes `testdata/disks/round-exchange.dsk` (RD51 size, label
ROUNDXCHG, gitignored) and copies the sources and `round.com` onto it.
Then govax makes these, all on the volume:

- **Objects.** govax's MACRO assembles each source into
  `GV<name>.OBJ`, traceback records and all.
- **Images.** govax's LINK links the three programs, each with
  traceback (the default) and with `/NOTRACEBACK`, as the probe's
  `list.com` did:

  | Image | Linked from | Traceback |
  | --- | --- | --- |
  | `GVTRACE.EXE` | `GVTRACE` | yes |
  | `GVTRNOTB.EXE` | `GVTRACE` | no |
  | `GVFAIL.EXE` | `GVFAILMAIN`, `GVFAILSUB` | yes |
  | `GVFAILNT.EXE` | `GVFAILMAIN`, `GVFAILSUB` | no |
  | `GVFSIG.EXE` | `GVFAILSIG` | yes |
  | `GVFSIGNT.EXE` | `GVFAILSIG` | no |

  Each has a map (`GV<name>.MAP`).

On VMS, with the volume mounted and set as the default directory, run:

    @ROUND/OUTPUT=ROUND.LOG

It:

1. Runs `ANALYZE/OBJECT` on every `GV*.OBJ` (`.ANL`, with `$STATUS`
   after each).
2. Links govax's objects with real LINK into `RL*.EXE`, mirroring the
   table above, with maps. It runs `ANALYZE/IMAGE` on all twelve images
   (`.ANI`).
3. Runs all twelve images, with `$STATUS` after each, so `ROUND.LOG`
   keeps what each printed. `GVTRACE` and `RLTRACE` should print the
   message `TRACE` prints. The others fail, with a traceback or
   without one.
4. Assembles `NOTITLE2` and `NOTITLE3` with real MACRO (`/LIST`) and
   runs `ANALYZE/OBJECT` on them.

With simh paused (or the disk detached), copy the results into `vax/`:

    govax console < testdata/mar/round/copyout.cmd

That copies the analyses, real LINK's images and maps, `NOTITLE2` and
`NOTITLE3`'s listings, objects, and analyses, and `ROUND.LOG`. govax's
own objects and images aren't copied back; `exchange.cmd` remakes them.

## What came back (`vax/`)

From the user's run of `@ROUND/OUTPUT=ROUND.LOG` on 4-OCT-2026.
`copyout.cmd` copied all 47 results.

- **Every govax object and image is clean.** `ANALYZE/OBJECT` found 0
  errors in each of the 11 `GV*.OBJ`, and `ANALYZE/IMAGE` 0 in each of
  the 6 `GV*.EXE`. It also found 0 in the 6 `RL*.EXE` real LINK made
  of them.
- **Real LINK links govax's objects.** Each `RL*.EXE` differs from the
  image real LINK made of real MACRO's objects in the probe only in the
  header's link time, image name, and linker ID.
- **The tracebacks are the same.** Each program printed the same thing
  three ways: from govax's objects linked by govax (`GV`), the same
  objects linked by real LINK (`RL`), and real MACRO's objects linked by
  real LINK (the probe's `list.log`). That covers the traceback tables
  (module, routine, rel PC, abs PC), the "Improperly handled condition"
  register dumps without traceback, and `$STATUS`. `GVTRACE` and
  `RLTRACE` printed `TRACE: three routines ran`.
- **govax's images against real LINK's.**
  - `GVTRACE`, `GVTRNOTB`, `GVFAIL`, and `GVFAILNT` are the `RL` images
    byte for byte, but for the header's identification fields.
  - `GVFSIG` and `GVFSIGNT` also differ in the order of their two fixup
    cells. FAILSIG calls `LIB$SIGNAL` (LIBRTL offset `4F0`), then
    `LIB$STOP` (`4F8`), with `G^`. govax gives them cells in that order;
    real LINK gives `LIB$STOP` the first. Each call's displacement
    follows its cell, so the code differs by those two displacements.
    Several rules fit one example, so `cells.mar` and `cellsb.mar`
    follow up (below).
- **The title header.** Real MACRO's `NOTITLE2` (no `.SBTTL`) and
  `NOTITLE3` (a `.SBTTL` after the first statement) both have a title
  header record of `"\x01 "`, as `NOTITLE` (a `.SBTTL` first) does. So
  it comes from having no `.TITLE`, and govax now writes it.

## The fixup cell follow-up

`cells.mar` calls `LIB$WAIT`, `LIB$ADDX`, `LIB$GET_INPUT`, and
`LIB$ADDX` again with `G^`. Each rule that fits FAILSIG gives these
cells a different order (the source's comment lists them).
`cellsb.mar` is a second module, linked after it, that calls one of
those routines and one new one.

    govax console < testdata/mar/round/cells-in.cmd     (done)
    @CELLS/OUTPUT=CELLS.LOG                              (on VMS)
    govax console < testdata/mar/round/cells-out.cmd

`cells-in.cmd` copied the sources and `CELLS.COM` onto
`round-exchange.dsk`. `CELLS.COM` assembles both sources, links `CELLS`
alone and `CELLS,CELLSB`, with maps, and analyzes the images.
`cells-out.cmd` copies the results into `vax/`.

**What came back** (the user's run, 4-OCT-2026). `ANALYZE/IMAGE` found
no errors.

- `CELLS.EXE` gives the cells in the order `LIB$ADDX`, `LIB$GET_INPUT`,
  `LIB$WAIT`.
- `CELLS2.EXE` gives `LIB$ADDX`, `LIB$GET_INPUT`, `LIB$PUT_OUTPUT`,
  `LIB$WAIT`.

That's the names' order, whatever order the code calls them in, and
whichever module calls them. But FAILSIG's `LIB$STOP` comes before its
`LIB$SIGNAL`, which no rule tried fits along with these:

- reverse of first or last call;
- offset order, either way;
- name order;
- name length;
- GSD order (which is name order).

Real LINK's order may come from its hash table, or from LIBRTL's own
global symbol table, and its output shows neither. govax gives cells in
name order (`orderCells`, `internal/link/fixup.go`). Its images of
CELLS and CELLS2 match real LINK's byte for byte. FAILSIG's are a known
difference, cosmetic only: the programs run the same.
