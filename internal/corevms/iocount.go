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
// Which kind an operation is depends on its device: a disk's transfers
// are direct I/O (the device reads and writes the program's buffer),
// and a terminal's, a mailbox's, or the null device's are buffered (the
// data goes through a system buffer).
//
// govax counts:
//
//   - every $QIO request that completes for the process (completeIO),
//     by its device;
//   - each record RMS reads from or writes to the terminal, a mailbox,
//     or NL: (and LIB$PUT_OUTPUT's and LIB$GET_INPUT's lines) as one
//     buffered I/O, as RMS's own $QIO for it would be;
//   - for an RMS service on a volume, the block reads and writes ods2
//     did for it (a mounted volume's operation count, before and after)
//     as direct I/Os: VMS's RMS reads and writes its buffers with one
//     $QIO a buffer, and ods2's block operations stand in for those.
//     *Unconfirmed approximation*: ods2's caching and transfer sizes
//     aren't RMS's, so the counts won't match a VMS run's.
//
// A section's page reads and writes are paging I/O, not counted, as on
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

// isDirectIO reports whether I/O to d is direct (a disk), not buffered.
func isDirectIO(d *iodev.Device) bool {
	return d != nil && d.DevClass == iodev.DeviceClassDisk
}

// countVolumeIO runs fn and counts as env's direct I/O the block
// operations every mounted volume did meanwhile (see above).
func (env *Environment) countVolumeIO(fn func()) {
	if env.Mounts == nil {
		fn()

		return
	}

	before := env.Mounts.TotalOperations()

	fn()

	if after := env.Mounts.TotalOperations(); after > before {
		env.Process.DirectIO += uint32(after - before)
	}
}
