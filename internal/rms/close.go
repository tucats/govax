package rms

import (
	"time"

	"github.com/tucats/ods2/vmstime"
)

// SysClose implements SYS$CLOSE: given a FAB's VAX address (argv[0] — like
// SYS$CREATE, and unlike SYS$CONNECT/SYS$PUT, SYS$CLOSE operates on a FAB
// directly rather than a RAB), finalizes whatever file that FAB's
// FAB$W_IFI currently names and frees the IFI slot so a later SYS$CREATE/
// SYS$OPEN can reuse it.
//
// # Why closing a FAB is enough, without touching any RAB
//
// Real VMS lets more than one RAB be SYS$CONNECTed to the same FAB at once
// (rab.go's own doc comment), but a RAB has no independent lifetime of its
// own beyond its FAB — it's purely "a record-access stream into whatever
// file this FAB currently has open", with no separate close operation a
// real VAX program ever calls. Closing the FAB is what ends all of that
// FAB's RABs at once; this package doesn't need to track "which RABs point
// at this FAB" anywhere for that to be true, since nothing here holds a
// RAB's own state past the single SYS$PUT/SYS$GET call that used it.
//
// # A quick Go note for readers new to the language
//
// Like every other handler in this package, SysClose returns (uint32,
// error): the uint32 is what the calling VAX program sees in register R0
// (the real system-service completion-status convention), and a non-nil
// error means something went wrong at the Go/emulator level itself (for
// instance, fabAddr pointing at unmapped VAX memory) rather than an
// ordinary RMS-level condition the calling program is expected to check
// for — see status.go's storeStatus doc comment for the fuller
// explanation of that split.
func SysClose(ctx *Context, argv []uint32) (uint32, error) {
	fabAddr := argv[0]

	ifi, err := ctx.loadWord(fabAddr + fabIFI)
	if err != nil {
		return 0, err
	}

	handle, ok := ctx.Files.Lookup(ifi)
	if !ok {
		// Either this FAB's FAB$W_IFI was never set by a successful
		// SYS$CREATE/SYS$OPEN in the first place, or the file it once
		// named has already been SYS$CLOSEd once (Release, below,
		// removes the FileTable entry — a second close of the same FAB
		// lands here too). Both are real RMS$_IFI conditions, not a bug
		// in this package.
		return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsInvalidIFI)
	}

	// The console pseudo-device (ifi.go's FileHandle.Console) isn't a
	// real ODS-2 file at all — nothing on disk to finalize, and the
	// console itself stays open and usable for whatever the VAX program
	// does next (creating another TTA0: file, for instance). Only the
	// real-volume case below needs any actual close work done.
	if handle.IsRecordDevice() {
		handle.Device.Close()
	} else if !handle.IsConsole() {
		// A file opened for writing takes the revision date and number
		// of a XABRDT, and the protection of a XABPRO, as it's closed
		// (docs/PHASE-33.md, subtask 5).
		var in xabInputs

		if handle.Writable {
			chain, sts, stv, err := ctx.xabChain(fabAddr)
			if err != nil {
				return 0, err
			}

			if sts != 0 {
				return fabStatus(ctx, fabAddr, sts, stv)
			}

			if in, err = ctx.readXABInputs(chain); err != nil {
				return 0, err
			}
		}

		// The XABs apply to a file written through this FAB, and not to
		// one deleted while open, which is about to go.
		xabs := func() error {
			if !handle.Writable || handle.File.MarkedForDelete() {
				return nil
			}

			return applyCloseXABs(handle.File, in, vmstime.FromTime(time.Now()))
		}

		handle.locks.release()

		if err := closeVolumeFile(handle, xabs); err != nil {
			// A genuine underlying ods2/volume-layer failure while
			// finalizing the file's on-disk size — not a Go bug, so
			// it's reported as an ordinary (if generic) RMS device
			// error rather than propagated as a Go error, the same
			// convention create.go's createOnVolume already uses for
			// vol.CreateFile failures.
			return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsDeviceError)
		}
	}

	// Freeing the IFI slot happens last, only once the close itself has
	// actually succeeded: if closeVolumeFile had failed above, the
	// handle would still be released here anyway if this line ran
	// first, letting a calling VAX program believe the file was closed
	// (and its slot reusable) when it may not have been fully flushed to
	// disk.
	ctx.Files.Release(ifi)

	// The FAB is closed: FAB$W_IFI is cleared (RMS manual, the Close
	// service's output fields), so it can be used for another $OPEN,
	// $CREATE, or $ERASE, which require it 0.
	if err := ctx.storeWord(fabAddr+fabIFI, 0); err != nil {
		return 0, err
	}

	return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsNormal)
}

// closeVolumeFile finalizes handle's underlying *volume.File, the real
// ODS-2-backed case SysClose defers to once it has ruled out the console
// pseudo-device.
//
// Exactly one of two things happened to handle.File before this is ever
// called (ifi.go's FileHandle doc comment): either SYS$CONNECT armed it
// for writing (handle.Writer is set, via connect.go's armForFAC), or it
// was opened/connected for reading only (handle.Writer is nil — either
// handle.Reader is set instead, or the file was never armed for either
// direction at all, both of which need only the same plain, harmless
// close). Preferring Writer.Close over File.Close when a Writer exists
// matters because Writer.Close is what actually knows the file's true
// final byte length (records routinely end partway through a disk block —
// see the sibling ods2 module's rms.Writer.Close doc comment); calling
// File.Close directly on an armed-for-writing File instead would silently
// round its recorded size up to the next whole block.
//
// For every other case, *volume.File.Close is always safe to call even
// though this package never called volume.File.OpenForWrite on such a
// handle: ods2's own File.Close doc comment specifies it as a harmless
// no-op on a File that was never armed for writing at all — exactly what
// a read-only-opened file is — so there's no need for this function to
// separately detect and skip that case.
//
// A file opened through the file system's access (handle.Accessor, every
// $OPEN and $CREATE of a volume file: sharing.go) is shared with its
// other openers, so the Writer is flushed rather than closed: its last
// records go on the disk and the file's end of file past them, in the
// shared header, which the last opener's Deaccess writes back. That
// Deaccess also deletes a file deleted while open.
//
// finish, if not nil, is called once the records are written and before
// the file is deaccessed: $CLOSE's XABs.
func closeVolumeFile(handle *FileHandle, finish func() error) error {
	if finish == nil {
		finish = func() error { return nil }
	}

	if handle.Accessor == nil {
		var err error
		if handle.Writer != nil {
			err = handle.Writer.Close()
		} else {
			err = handle.File.Close()
		}

		if err != nil {
			return err
		}

		return finish()
	}

	var err error
	if handle.Writer != nil {
		err = handle.Writer.Flush()
	}

	if err == nil {
		err = finish()
	}

	if derr := handle.Accessor.Deaccess(); err == nil {
		err = derr
	}

	handle.claim.release()

	return err
}

// Rundown closes every file t has open and empties the table but for its
// terminal slot: what RMS's rundown does when a process is deleted
// (VAX/VMS Internals and Data Structures, section 22.2.1, step 3), so
// that a file the process was writing ends with its records on the
// volume, as if the program had closed it. No XABs are applied: no FAB
// is at hand, as none is on VMS. It returns how many files it closed and
// the first error a close met (every file is closed regardless).
func (t *FileTable) Rundown() (int, error) {
	var (
		closed   int
		firstErr error
	)

	for ifi, h := range t.handles {
		if h.IsConsole() {
			continue
		}

		h.locks.release()

		if h.IsRecordDevice() {
			h.Device.Close()
		} else if err := closeVolumeFile(h, nil); err != nil && firstErr == nil {
			firstErr = err
		}

		closed++

		delete(t.handles, ifi)
	}

	clear(t.searches)

	return closed, firstErr
}
