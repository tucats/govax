# MACRO-32 object module fixtures (Phase 27)

These `.mar` files are the fixture ladder in `docs/PHASE-27.md`. Each step
adds one feature of the VAX object language, from an empty module (`empty`)
to a complete program (`hello`). They're assembled by real VAX MACRO on the
user's simh VAX 8600 running VMS 7.3, and those results are what govax's
object reader and MACRO command are checked against.

`assemble.com` does the VMS side in one step (`@ASSEMBLE`). For each
fixture it runs `MACRO/LIST` (giving `.OBJ` and `.LIS` files) and
`ANALYZE/OBJECT` (giving `.ANL`). It links and runs `entry` and `hello`,
the two complete programs, with maps, and writes `DIRECTORY/FULL` output
for the objects (their record attributes) to `OBJECTS.DIR`.

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
