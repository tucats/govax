# Local-only disk containers

This directory is for real `simh`-produced VAX/VMS ODS-2 disk-image containers,
used to validate `internal/rms`'s `ods2`-backed file-system support against a
"real" volume rather than only one `govax` created itself — see
`docs/PHASE-22.md`'s "Container format fidelity / `simh` interoperability"
section.

These containers hold licensed VAX/VMS code and can be large binary blobs, so
they are never committed: everything in this directory except this README is
`.gitignore`d (see the repo's `.gitignore`). Symlink or copy container files
in here directly; nothing else is needed.

Interop tests that use these containers glob this directory and skip cleanly
when nothing is present, so they run automatically once containers are here
but never fail (or even run) on a fresh clone or in CI.
