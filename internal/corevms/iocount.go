package corevms

import iodev "github.com/tucats/govax/internal/io"

// Per-process I/O counts (docs/PHASE-49.md, subtask 11): the accounting
// statistics VMS keeps in the process header, PHD$L_BIOCNT (buffered
// I/O operations) and PHD$L_DIOCNT (direct I/O operations). VAX/VMS
// Internals and Data Structures, section 18.3.3, increments one of them
// as each I/O request completes for the process (in I/O
// postprocessing, so a cancelled request counts too, and a request
// $QIO rejects doesn't). They are $GETJPI's JPI$_BUFIO and JPI$_DIRIO,
// SHOW SYSTEM's I/O column (their sum), the termination message's
// ACC$L_BIOCNT and ACC$L_DIOCNT, and LOGOUT's report.
//
// Which kind a request is depends on its device's driver. A disk's
// transfers are direct I/O, and so, on VMS 7.3, is a write to the null
// device (testdata/probe49, step 8); a terminal's and a mailbox's are
// buffered.
//
// govax counts:
//
//   - every $QIO request that completes for the process (completeIO),
//     by its device;
//   - each record RMS reads from or writes to a mailbox or NL: (and
//     LIB$GET_INPUT's lines, and LIB$PUT_OUTPUT's to the terminal), as
//     one I/O of the device's kind, as RMS's own $QIO for it would be;
//   - RMS's I/O on volume files, by internal/rms's model
//     (rms/iocount.go: the file system's calls are buffered I/O, the
//     multiblock buffers direct), through rms.Context.CountIO.
//
// LIB$PUT_OUTPUT's lines to a file count nothing: VMS's RMS buffered
// such a short line (step 8: a line to the probe's log was 0 and 0). A
// section's page reads and writes are paging I/O, not counted, as on
// VMS.

// countIO counts one completed I/O operation for env's process: direct
// I/O when direct is true, buffered otherwise.
func (env *Environment) countIO(direct bool) {
	if direct {
		env.Process.DirectIO++
	} else {
		env.Process.BufferedIO++
	}
}

// isDirectIO reports whether I/O to d is direct, not buffered: a disk,
// or the null device.
func isDirectIO(d *iodev.Device) bool {
	return d != nil && (d.DevClass == iodev.DeviceClassDisk || d.DevType == iodev.DeviceTypeNull && d.DevClass == iodev.DeviceClassMailbox)
}
