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

Not run yet.
