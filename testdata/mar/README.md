# MACRO-32 object module fixtures (Phase 27)

These `.mar` files are the fixture ladder in `docs/PHASE-27.md`. Each step
adds one feature of the VAX object language, from an empty module (`empty`)
to a complete program (`hello`). They're assembled by real VAX MACRO on the
user's simh VAX 8600 running VMS 7.3, and those results are what govax's
object reader and MACRO command are checked against.

`assemble.com` does the VMS side in one step
(`@ASSEMBLE/OUTPUT=ASSEMBLE.LOG`). For each fixture it runs `MACRO/LIST`
(giving `.OBJ` and `.LIS` files) and `ANALYZE/OBJECT` (giving `.ANL`). It
links and runs `entry`, `hello`, and `psects`, the complete programs, with
maps, and writes `DIRECTORY/FULL` output for the objects (their record
attributes) to `OBJECTS.LST`.

Fixtures 1 to 9 are the ladder.

`forth.mar` isn't on the ladder: it's a FORTH interpreter (a MACRO-32
port of `testdata/asm/forth.asm`), a larger program for ANALYZE and the
debugger. It hasn't been through real MACRO yet, so `internal/asm`'s
ladder tests skip it (`notLadder`); `internal/console`'s `TestForth`
tests assemble, link, and run it. Its `input` and `output` words read
and write `.FTH` and `.LIS` files on a mounted volume. Fixtures 10 to 12 (`modes`, `psects`,
`general`) were added in subtask 11 for the encoding choices the ladder
didn't settle.

When the volume also holds `GV_NAME.OBJ`, govax's own object for
`NAME.MAR` written straight onto the volume by govax's MACRO command, the
script checks each one with `ANALYZE/OBJECT` (`GV_NAME.ANL`) and links
and runs the complete programs, as extra evidence that real VMS accepts
govax's objects.

The real VAX results go in `vax/`, with the objects in the host
variable-length record layout that `internal/obj.ReadRecords` reads.
`internal/obj`'s `TestRealObjects` checks every `vax/*.obj` found there.

## Moving files to and from the VAX

The files travel on an ODS-2 disk container that govax builds and simh
mounts:

1. govax creates the container (`testdata/disks/mar-exchange.dsk`,
   gitignored like every container in `testdata/disks/`) with
   `INITIALIZE/CONTAINER` and copies the fixtures in with `COPY .../HOST`.
2. The user attaches it to simh, then mounts it on VMS and runs `@ASSEMBLE`
   there.
3. With simh paused (or the disk detached), govax mounts the container
   read-only and copies the results out into `vax/`.

`vax/assemble.log` is the log of the second run (subtask 11), and
`vax/objects.lst` the objects' `DIRECTORY/FULL`. `vax/govax/` holds what
VMS made of govax's own objects in that run: `ANALYZE/OBJECT` output
(`gv_*.anl`, 0 errors each) and the link maps of the three programs,
which linked and ran.
