// Package rms is govax's implementation of VMS RMS (Record Management
// Services) — the part of the VMS runtime that a VAX program calls into
// (via SYS$CREATE, SYS$OPEN, SYS$CLOSE, SYS$GET, SYS$PUT, and SYS$CONNECT)
// to create, open, read, and write files. See docs/PHASE-22.md for the
// full design.
//
// # Why this package exists, for a reader new to the project
//
// A running VAX program never talks to a "file system" the way a modern Go
// program does (os.Open, os.Create, ...). Instead it calls one of the
// SYS$ services named above, which this emulator's RTL layer
// (internal/rtl) dispatches by name into whichever package registered a
// handler for it — see internal/rtl/service.go's ServiceTable. This
// package registers the handlers for exactly those six services (create.go,
// connect.go, open.go, close.go, get.go, put.go — added incrementally as
// docs/PHASE-22.md's subtasks land) and backs them with a real, on-disk
// ODS-2 ("Files-11 Structure Level 2") file system, using the sibling
// module github.com/tucats/ods2 to actually read and write the volume's
// bytes. That's a deliberate step up from Phase 10's original RMS
// stopgap (deleted — see docs/PHASE-22.md's "Removing Phase 10's
// host-passthrough RMS"), which just opened an arbitrary host file and
// called that "RMS": real VMS software expects a real VMS-shaped volume
// underneath it (directories, file headers, allocation bitmaps, and a
// specific on-disk byte layout), and this package is what makes that true
// here.
//
// # The pieces in this package
//
//   - mount.go's MountTable tracks which VAX device names (e.g. "DUA0:")
//     currently have a container file mounted as an ODS-2 volume — the Go
//     equivalent of real VMS's MOUNT command.
//   - fab.go and rab.go define the exact byte layout of the two VMS
//     control blocks (the FAB, "File Access Block", and RAB, "Record
//     Access Block") that a calling VAX program builds in its own memory
//     and hands to RMS by address. This package's service handlers read
//     and write specific fields inside those blocks directly out of
//     emulated VAX memory, so it has to know precisely where each field
//     lives, the same way the CPU package has to know a VAX instruction's
//     exact opcode encoding.
//   - status.go defines the real numeric RMS$_ completion-code values a
//     compiled VAX program checks FAB$L_STS/RAB$L_STS against after a
//     call — these have to be the genuine VMS values, not values this
//     project invented, or a real, unmodified VAX program would
//     misinterpret what happened.
//   - ifi.go's FileTable is this package's own "list of currently open
//     files", keyed by a small integer ("internal file index", or IFI)
//     the same way a Unix process's open files are keyed by small integer
//     file descriptors. SYS$CREATE/SYS$OPEN hand back an IFI; SYS$PUT/
//     SYS$GET/SYS$CLOSE take one as input to say which open file they mean.
package rms
