package rms

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
	if !handle.IsConsole() {
		if err := closeVolumeFile(handle); err != nil {
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
func closeVolumeFile(handle *FileHandle) error {
	if handle.Writer != nil {
		return handle.Writer.Close()
	}

	return handle.File.Close()
}
