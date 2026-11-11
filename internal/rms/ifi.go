package rms

import (
	"io"

	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// This file is a process's open files: the Go state behind RMS's
// "internal file index" (IFI), which SYS$CREATE and SYS$OPEN store in
// FAB$W_IFI, SYS$CONNECT copies into RAB$W_ISI, and every later service
// takes as its way of saying "this open file".

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
// ods2 module), and Reader or Writer (or both) is set once the file
// has actually been armed for record-by-record access: Reader once
// SYS$OPEN/SYS$CONNECT has set the file up for reading (backing
// SYS$GET), and Writer once SYS$CREATE/SYS$CONNECT has set it up for
// writing (backing SYS$PUT). Both are nil for a File that SYS$CREATE has
// just made but SYS$CONNECT hasn't armed yet — this package's IFI table
// tracks "this file is open" and "this stream is ready for GET/PUT" as
// two separate steps, the same two-step CREATE-then-CONNECT (or
// OPEN-then-CONNECT) sequence a real VAX program has to perform.
//
// A stream opened for both (FAB$B_FAC with GET and PUT) has both: it
// reads with Reader and appends with Writer, its context (stream.go)
// saying where (Phase 49).
type FileHandle struct {
	// Console is set only for the terminal-pseudo-device case. Check
	// IsConsole rather than comparing this field to nil directly, so
	// that which check to use isn't scattered across every caller.
	Console io.Writer

	// Device is set only for a record device's stream: a mailbox or NL:
	// (recdevice.go). Check IsRecordDevice.
	Device RecordDevice

	// PutPending is set while an asynchronous $PUT to Device waits for
	// its record to be read; $WAIT finishes it.
	PutPending bool

	// File is the underlying ODS-2 file, set only for the real-volume
	// case (nil for the console case).
	File *volume.File

	// Found is the file as $OPEN or $CREATE found it, for $DISPLAY's
	// NAM; Writable is whether it was opened for writing, so $CLOSE
	// takes its XABRDT and XABPRO (docs/PHASE-33.md).
	Found    *foundFile
	Writable bool

	// Access is the FAB$B_FAC access the file was opened or created with,
	// including what the service implied when the FAB asked for none
	// (facAccess). $CONNECT arms the file by it when the FAB asks for
	// none.
	Access byte

	// Accessor is the file system's access of File (ods2's volume.Access,
	// sharing.go), which $CLOSE ends; Mode is how it accesses the file,
	// from FAB$B_FAC and FAB$B_SHR.
	Accessor *volume.Access
	Mode     volume.AccessMode

	// Share is the FAB$B_SHR the file was opened with, after its
	// defaults (effectiveSharing); locks are the stream's record locks
	// (recordlock.go).
	Share byte
	locks streamLocks

	// claim is the open's place in its file's list of RMS openers
	// (sharing.go), released at $CLOSE.
	claim *openerClaim

	// Reader/Writer are set once File has actually been armed for
	// reading or writing respectively (see this type's own doc comment
	// for why that's a separate step from File being non-nil at all).
	Reader *odsrms.Reader
	Writer *odsrms.Writer

	// stream is the record stream's context: its current and next
	// records (stream.go).
	stream streamContext

	// rel is a relative file's layout, set when $CONNECT arms one, and
	// relStream what its stream keeps beside stream (relative.go). A
	// relative file has no Reader or Writer.
	rel       *relativeLayout
	relStream relativeStream

	// indexed is set when $CONNECT arms an indexed file's stream
	// (indexed.go), which has no Reader or Writer either, and idx is what
	// its stream keeps beside stream (indexedrecords.go).
	indexed bool

	// reviseAtClose says the file gets a new revision date and number when
	// it's closed without a FAB (closeRevision): DCL's OPEN of a file that
	// was there, to write.
	reviseAtClose bool
	idx           indexedStream

	// readBuffer and writeBuffer are the last multiblock buffers the
	// stream read and wrote, and written whether it wrote anything, for
	// the I/O counts (iocount.go).
	readBuffer, writeBuffer ioBuffer
	written                 bool
}

// wrote counts a record written at byte offset off (iocount.go).
func (h *FileHandle) wrote(ctx *Context, off int64) {
	h.written = true
	h.writeBuffer.touch(ctx, off)
}

// noteLongest records a record of n data bytes written to h's file: as
// RMS keeps it for a sequential file, the header's record size
// (FAT$W_RSIZE, XAB$W_LRL, DIRECTORY/FULL's "longest") is the longest
// record written, but for a fixed-length file, whose record size is the
// one size. The header goes to the volume when the file is closed.
func (h *FileHandle) noteLongest(n int) {
	if h.File == nil {
		return
	}

	ra := &h.File.Header.RecordAttributes
	if ra.Format != ondisk.RecordFormatFixed && n > int(ra.RecordSize) {
		ra.RecordSize = uint16(min(n, 0xFFFF))
	}
}

// IsConsole reports whether h is the terminal-pseudo-device case (Console
// set) rather than a real ODS-2-backed file.
func (h *FileHandle) IsConsole() bool {
	return h.Console != nil
}

// FileTable is one process's open files, by IFI ("internal file index"),
// the number $OPEN and $CREATE store in FAB$W_IFI and $CONNECT copies to
// RAB$W_ISI: RMS's rough equivalent of a Unix process's file descriptors.
// Each process's corevms.Environment has one.
//
// An image's files are in handles, from IFI 4 up (firstUserIFI); an IFI no
// open has given out, 0 to 3 among them, is RMS$_IFI. The process's
// permanent files (ppf.go) are a table of their own, outliving images,
// whose IFIs have the PPF bit set and never collide with these.
type FileTable struct {
	handles map[uint16]*FileHandle

	// searches is $PARSE and $SEARCH's wildcard contexts, by number
	// (search.go).
	searches map[uint32]*searchState

	// permanent is the process-permanent files, by index (ppf.go): not
	// the image's, so not in handles, which rundown closes.
	permanent map[uint16]*PermanentEntry

	// lineOpen is whether the terminal's last line is open: a FORTRAN
	// carriage control prompt left the cursor at its end (ftncc.go).
	lineOpen bool

	// console is the terminal's output, where Rundown ends an open line.
	console io.Writer
}

// NewFileTable returns an empty FileTable for a process whose terminal
// output is consoleOut (nil for one with none, as in tests).
func NewFileTable(consoleOut io.Writer) *FileTable {
	return &FileTable{handles: map[uint16]*FileHandle{}, console: consoleOut}
}

// firstUserIFI is the lowest IFI Alloc hands out. Which numbers VMS's RMS
// gives an image's files no probe has recorded (only process-permanent
// files' indirect IFIs, ^XC084 and the like, testdata/ppf71); govax starts
// at 4.
const firstUserIFI = 4

// Alloc registers h under the lowest IFI not in use, from firstUserIFI
// up, so an IFI $CLOSE freed is used again, and returns it.
func (t *FileTable) Alloc(h *FileHandle) uint16 {
	for id := uint16(firstUserIFI); ; id++ {
		if _, used := t.handles[id]; !used {
			t.handles[id] = h

			return id
		}
	}
}

// Lookup returns the FileHandle registered under ifi, and whether one is.
// A false result is RMS$_IFI for the calling program (a FAB never opened,
// or already closed), not a fault of this package.
func (t *FileTable) Lookup(ifi uint16) (*FileHandle, bool) {
	h, ok := t.handles[ifi]

	return h, ok
}

// Find returns the IFI h has in t, if it has one.
func (t *FileTable) Find(h *FileHandle) (uint16, bool) {
	for ifi, x := range t.handles {
		if x == h {
			return ifi, true
		}
	}

	return 0, false
}

// Release forgets ifi's entry, once $CLOSE has closed what it held.
// Releasing an IFI not in use does nothing.
func (t *FileTable) Release(ifi uint16) {
	delete(t.handles, ifi)
}
