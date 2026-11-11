// Package rms is govax's RMS (Record Management Services): the part of
// VMS a program calls (SYS$OPEN, SYS$CREATE, SYS$CONNECT, SYS$GET,
// SYS$PUT, SYS$PARSE, SYS$SEARCH, and the rest) to name, open, read, and
// write files, and the file work govax's own commands share with it.
//
// # How a program reaches it
//
// A program builds its control blocks (the FAB for a file, the RAB for a
// record stream, NAMs and XABs beside them) in its own memory and calls a
// service with the block's address. internal/corevms's service table
// sends each RMS service to its handler here (SysOpen, SysGet, ...) with
// a Context: the process's memory and CPU, its open files, its logical
// names, the mounted volumes, and what it needs of the rest of VMS (the
// terminal, mailboxes, the lock database). A handler reads and writes the
// blocks' fields by their VMS offsets (fab.go, rab.go, nam.go, xab.go) and
// returns the RMS status, which it has also stored in the block's STS.
//
// # Where the files are
//
// Files are on ODS-2 volumes, container files mounted on device names
// (mount.go's MountTable), read and written through the sibling module
// github.com/tucats/ods2, which this is the only package to import. A
// program's RMS reaches only volumes, the terminal, mailboxes, and NL:;
// the DCL utilities and the tools reach host files too, through Session
// (location.go, recordfile.go).
//
// # The pieces
//
//   - name.go and logicals.go: name processing, one for every caller;
//     parse.go and search.go, $PARSE and $SEARCH.
//   - ifi.go: a process's open files (FileTable) and each one's state
//     (FileHandle); ppf.go and ppfaccess.go, its process-permanent files.
//   - open.go, create.go, connect.go, close.go: opening; sharing.go, the
//     arbitration between openers; ufo.go, user file open.
//   - get.go, put.go, stream.go, recordops.go: sequential records;
//     relative.go, relative files; indexed.go and indexedrecords.go,
//     indexed files over internal/isam; keyed.go, keyed access to a
//     fixed-length sequential file; blockio.go, $READ and $WRITE;
//     recordlock.go, record locks in the lock database.
//   - terminal.go and recdevice.go: the terminal, mailboxes, and NL:.
//   - acp*.go: the disk ACP's functions, for a program's $QIO on a disk.
//   - session.go, location.go, recordfile.go, and the files named for
//     commands (directory.go, copy.go, ...): what the DCL utilities use.
package rms
