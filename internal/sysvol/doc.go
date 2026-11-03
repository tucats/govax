// Package sysvol describes govax's system volume (docs/PHASE-87 - system
// volume.md): the ODS-2 volume govax mounts, as DUA0: by default, every
// time it starts, holding the files a VMS system disk would hold for it.
//
// On a real VMS system the system disk is where the system's own files
// live: SYS$SYSTEM's images, SYS$MANAGER's startup procedures,
// SYS$LIBRARY's macro and object libraries, SYS$HELP's help libraries.
// govax carries its own versions of such files built into the program
// (internal/bootdata's startup procedure, grammars, and help; FTNRTL.EXE;
// the FORTRAN definition library). The system volume is a copy of them on
// a volume, where the user can read, change, rebuild, and replace them
// with DCL's own commands, as on VMS.
//
// This package knows what goes on the volume and nothing about how a
// volume is written: it is plain Go, over the small Volume interface,
// which govax's RMS implements and the tests implement with a map. It
// has:
//
//   - the layout (files.go): each file's directory, name, record format,
//     and where its contents come from, one table, read by everything
//     else;
//   - the file contents' one canonical form (contents.go), so that what
//     govax wrote and what it reads back can be compared;
//   - the manifest (manifest.go), the volume's record of what govax
//     supplied, by which later versions of govax tell a file the user
//     changed from one they didn't;
//   - Populate (populate.go), which fills a newly made volume.
//
// Two kinds of file go on the volume. A supplied file (Supplied) is
// govax's: listed in the manifest, and replaced by a newer govax's copy
// unless the user has changed it. A seeded file (Seeded) is the user's
// from the start: govax writes it once, when it's missing, as a starting
// point (SYSTARTUP_VMS.COM, the site-specific startup), and never again.
package sysvol
