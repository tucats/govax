# Phase 30: A govax LINK

## Goal

Add a `LINK` command that builds a VMS executable image (`.EXE`) from object
modules, so that a program assembled by govax's `MACRO` command can `RUN`
inside govax without a real VAX. It must link govax's own objects and real
VAX objects alike.

Split out of Phase 27's "later sub-phases" (docs/PHASE-27.md, subtask 12).

**Status: planned, not started.**

## What Phase 27 leaves in place

- **Reading objects.** `internal/obj` reads, decodes, and checks every
  record of the VAX object language. That includes all GSD subrecord types
  and all TIR commands, each with its stack effect. It reads both govax's
  objects and real ones: the twelve real objects in `testdata/mar/vax/`
  are its regression corpus.
- **Finding files.** `rms.Session.Locate`/`LocateRelated` and
  `ReadRecordFile`/`CreateRecordFile` find and read input objects, and
  write outputs, on the host or a mounted volume, with the same rules as
  `MACRO`.
- **Running images.** govax's `RUN` (Phase 13) activates real VMS images:
  it maps image sections, resolves sharable-image references, and applies
  fixups. So the image LINK writes has a consumer in govax already.
- **References to compare with.** Real `LINK/MAP` maps for `entry`,
  `hello`, and `psects`, from both real MACRO's objects and govax's, are
  in `testdata/mar/vax/` and `testdata/mar/vax/govax/`. The images real
  LINK built from them are on the (gitignored) exchange volume
  `testdata/disks/mar-exchange2.dsk`.

## Scope

- The linker's two passes over the objects:
  - collect psects (concatenated or overlaid by attribute, and aligned),
    global symbols, and entry points;
  - run each TIR program on the linker's stack machine to lay out and
    relocate the image.
- Resolving references against sharable images (`SYS$PUBLIC_VECTORS`,
  `LIBRTL`, and so on). govax's shim table (Phase 13/20) knows those
  entry points.
- The image header and image sections that govax's `RUN` (and ideally
  real VMS) accept.
- `/MAP` for a link map; `/EXECUTABLE[=file]`.

## References

- *VMS 5.0 Linker Utility Manual* (`AA-LA62A-TE`): the linking process,
  image layout, the object language (chapter 7), and maps.
- `vmssrc_archive/v73/linker/lis/`: the VAX linker's source listings.
  - `lnkobjps1_v.lis` and `lnkobjps2.lis`: its two object passes.
  - `lnkimgout.lis`: image output.
  - `lnkmaprtn.lis`: maps.
- `reference/eVAX`'s `console_run.c` and Phase 13's docs: the image format
  from the loader's side.

## Open questions

- **Image fidelity.** Must a govax-linked image run on real VMS, or only
  in govax? The Phase 27 answer for objects was "readable both ways";
  the equivalent here would be "govax runs real images, and real VMS
  runs govax's".
- **Sharable images.** Where do sharable-image symbol tables come from:
  real `.EXE` files copied from the VAX, or govax's shim table?
- **Scope of LINK's qualifiers** and options files.

## Subtasks

To be written when the phase starts.

## Progress Log

### 2026-09-30 — Planned

- Created from Phase 27's "later sub-phases" when that phase closed.
