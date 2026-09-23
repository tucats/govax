package rms

import (
	"io"

	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// This file is this package's own "list of currently open files" — the Go
// state behind RMS's "internal file index" (IFI) concept, which fab.go's
// fabIFI field and rab.go's rabISI field both ultimately point at (an IFI
// is what SYS$CREATE/SYS$OPEN hand back, stored into a FAB; SYS$CONNECT
// copies that same number into a RAB; SYS$PUT/SYS$GET/SYS$CLOSE all take
// an IFI as their real way of saying "this open file").
//
// # Generalized from Phase 10's stopgap
//
// The project's now-deleted Phase 10 RMS stopgap (internal/rtl/rms.go)
// had an IFI table too, but every entry in it could only ever be a plain
// io.Writer — enough for host-file "create and write" support, but with
// no way to represent an open-for-reading file at all (SYS$OPEN/SYS$GET
// didn't exist yet). This package's FileHandle (below) generalizes that
// to whichever of two shapes an open file actually is: either the
// console pseudo-device (unchanged from Phase 10 — see FileHandle's own
// doc comment), or a real ODS-2-backed file, connected for either reading
// or writing via the sibling ods2 module's own rms.Reader/rms.Writer.

// FileHandle is one entry in a FileTable: either the console
// pseudo-device, or a real file on a mounted ODS-2 volume.
//
// # The console case
//
// Real VMS RMS genuinely supports opening a terminal device (like
// "TTA0:") for SYS$CREATE/SYS$PUT — this isn't a govax-specific shortcut,
// it's ordinary VMS behavior (see docs/PHASE-22.md's "Why this phase
// looks different" section). When a FileHandle represents that case,
// Console is set to wherever console output should go and every other
// field is left at its zero value.
//
// # The ODS-2 file case
//
// Otherwise, File holds the underlying ods2 volume.File (opened via
// volume.Volume.CreateFile or volume.Volume.OpenFID — see the sibling
// ods2 module), and exactly one of Reader or Writer is set once the file
// has actually been armed for record-by-record access: Reader once
// SYS$OPEN/SYS$CONNECT has set the file up for reading (backing
// SYS$GET), or Writer once SYS$CREATE/SYS$CONNECT has set it up for
// writing (backing SYS$PUT). Both are nil for a File that SYS$CREATE has
// just made but SYS$CONNECT hasn't armed yet — this package's IFI table
// tracks "this file is open" and "this stream is ready for GET/PUT" as
// two separate steps, the same two-step CREATE-then-CONNECT (or
// OPEN-then-CONNECT) sequence a real VAX program has to perform.
//
// Never both Reader and Writer at once: this phase implements sequential
// organization only, where a given connection is inherently one
// direction or the other (real RMS lets a file be simultaneously open
// for GET and PUT through *different* RABs on the same FAB, but not
// through the same RAB — this phase doesn't need that generality).
type FileHandle struct {
	// Console is set only for the terminal-pseudo-device case. Check
	// IsConsole rather than comparing this field to nil directly, so
	// that which check to use isn't scattered across every caller.
	Console io.Writer

	// File is the underlying ODS-2 file, set only for the real-volume
	// case (nil for the console case).
	File *volume.File

	// Reader/Writer are set once File has actually been armed for
	// reading or writing respectively (see this type's own doc comment
	// for why that's a separate step from File being non-nil at all).
	Reader *odsrms.Reader
	Writer *odsrms.Writer
}

// IsConsole reports whether h is the terminal-pseudo-device case (Console
// set) rather than a real ODS-2-backed file.
func (h *FileHandle) IsConsole() bool {
	return h.Console != nil
}

// FileTable is this package's registry of currently open files, keyed by
// IFI ("internal file index") — RMS's rough equivalent of a Unix
// process's table of open file descriptors. One FileTable belongs to one
// VAX "process" (see internal/rtl.Environment's own doc comment on what
// "process" means in this emulator), the same way that Environment's own
// (now-removed) Phase 10 IFI table used to be per-Environment.
//
// # Why IFIs 0-3 are reserved
//
// Real VMS seeds a process's IFI table with four fixed, pre-assigned
// slots before any file is ever opened: 0 (an always-invalid sentinel),
// 1 (SYS$OUTPUT, the process's default output — the console, in this
// emulator), 2 (SYS$INPUT), and 3 (SYS$ERROR). This package only
// actually wires up slot 1, matching Phase 10's own scope (nothing this
// emulator implements yet reads from SYS$INPUT or writes to SYS$ERROR
// through RMS specifically) — but slots 0-3 are still reserved and never
// handed out by Alloc, so that a real VAX program's own assumptions about
// those numbers being special stay true even though slots 2 and 3 aren't
// individually usable yet.
type FileTable struct {
	handles map[uint16]*FileHandle
}

// NewFileTable returns a FileTable with IFI 1 (SYS$OUTPUT) pre-seeded to
// write to consoleOut — typically the same io.Writer the owning
// Environment already uses for its own non-RMS console output (see
// internal/rtl.NewEnvironment's consoleOut parameter). Passing a nil
// consoleOut is valid and simply leaves slot 1 unusable (Lookup(1) will
// report "not found"), matching how a FileTable used purely in a test
// that never touches the console case doesn't need a real writer.
func NewFileTable(consoleOut io.Writer) *FileTable {
	t := &FileTable{handles: map[uint16]*FileHandle{}}

	if consoleOut != nil {
		t.handles[1] = &FileHandle{Console: consoleOut}
	}

	return t
}

// firstUserIFI is the lowest IFI number Alloc will ever hand out — the
// four slots below it (0-3) are reserved (see FileTable's own doc
// comment) and never allocated, even if they're currently unused (for
// instance, if NewFileTable was given a nil consoleOut, leaving slot 1
// empty — Alloc still skips over it rather than handing it out).
const firstUserIFI = 4

// Alloc registers h under a fresh, currently-unused IFI (never one of the
// four reserved slots — see FileTable's own doc comment) and returns that
// IFI, matching what SYS$CREATE/SYS$OPEN hand back to a calling VAX
// program and store into the FAB's fabIFI field.
//
// Alloc always scans upward from firstUserIFI looking for the lowest free
// slot, rather than remembering a monotonically increasing "next" counter
// across calls. This is deliberately the simple, obviously-correct choice
// over a slightly faster stateful cursor: it costs a handful of map
// lookups per call (this table only ever holds as many entries as a VAX
// program has files open at once — never a performance-sensitive count),
// and in exchange it automatically reuses an IFI that Release freed,
// rather than burning through ever-larger numbers for the lifetime of the
// process the way a cursor that only ever increases would.
func (t *FileTable) Alloc(h *FileHandle) uint16 {
	for id := uint16(firstUserIFI); ; id++ {
		if _, used := t.handles[id]; !used {
			t.handles[id] = h

			return id
		}
	}
}

// Lookup returns the FileHandle registered under ifi, and whether one is.
// SYS$CONNECT/SYS$PUT/SYS$GET/SYS$CLOSE all call this to turn the IFI
// they were given (read out of a FAB or RAB) back into the actual open
// file it refers to; a false result is a real RMS$_IFI error to report
// back to the calling program (status.go's rmsInvalidIFI), not a bug —
// it means the VAX program supplied a stale or fabricated IFI.
func (t *FileTable) Lookup(ifi uint16) (*FileHandle, bool) {
	h, ok := t.handles[ifi]

	return h, ok
}

// Release forgets ifi's entry, called by SYS$CLOSE once it has finished
// flushing/closing whatever the handle pointed at. Releasing an IFI that
// isn't currently allocated (including one of the reserved 0-3 slots) is
// a harmless no-op — Go's built-in delete on a map behaves the same way
// whether or not the key was present, so there's nothing extra for this
// method to check.
func (t *FileTable) Release(ifi uint16) {
	delete(t.handles, ifi)
}
