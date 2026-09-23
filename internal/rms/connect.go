package rms

import (
	odsrms "github.com/tucats/ods2/rms"
)

// SysConnect implements SYS$CONNECT: given a RAB's VAX address (argv[0]),
// binds it to the IFI ("internal file index" — see ifi.go's FileTable)
// its related FAB already holds, and — for a real ODS-2-backed file, as
// opposed to the TTA0: console pseudo-device — arms that file for
// record-by-record access in whichever direction the FAB's FAB$B_FAC
// field asked for back when the file was created or opened.
//
// # Why "binding a RAB" is a separate step from SYS$CREATE/SYS$OPEN at all
//
// This is a real VMS concept, not something this package invented: a FAB
// (fab.go) represents one open *file*, but SYS$GET/SYS$PUT operate
// through a RAB (rab.go), a separate "record stream" object that has to
// be explicitly connected to a FAB before it can be used. A real,
// unmodified VAX program always does SYS$CREATE (or SYS$OPEN), then
// SYS$CONNECT, then however many SYS$GET/SYS$PUT calls it needs — see
// rab.go's own doc comment for the fuller RAB-versus-FAB explanation.
//
// # Why arming happens here, not at SYS$CREATE/SYS$OPEN
//
// ifi.go's FileHandle deliberately tracks "this file is open" (File
// non-nil) and "this stream is ready for GET/PUT" (Reader or Writer
// non-nil) as two separate pieces of state, matching this same two-step
// sequence a real VAX program performs. SYS$CREATE (create.go) only ever
// gets as far as the first step; this function is what performs the
// second one for a real file. The console case never needs arming at all
// — a FileHandle's Console writer is already exactly what SYS$PUT needs,
// with nothing further to set up.
func SysConnect(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	fabAddr, err := ctx.loadLongword(rabAddr + rabFAB)
	if err != nil {
		return 0, err
	}

	ifi, err := ctx.loadWord(fabAddr + fabIFI)
	if err != nil {
		return 0, err
	}

	handle, ok := ctx.Files.Lookup(ifi)
	if !ok {
		// The FAB's FAB$W_IFI doesn't name a currently open file — either
		// the calling program never actually ran a successful
		// SYS$CREATE/SYS$OPEN on this FAB first, or it's reusing a FAB
		// whose file has since been SYS$CLOSEd. Either way, this is a
		// real, expected RMS$_IFI error to report back, not a bug in
		// this package (see ifi.go's FileTable.Lookup doc comment).
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidIFI)
	}

	if !handle.IsConsole() {
		if failStatus, err := armForFAC(ctx, handle, fabAddr); err != nil {
			return 0, err
		} else if failStatus != 0 {
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, failStatus)
		}
	}

	if err := ctx.storeWord(rabAddr+rabISI, ifi); err != nil {
		return 0, err
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// armForFAC arms handle's underlying ODS-2 file for reading or writing,
// according to whichever access bit the FAB at fabAddr's own FAB$B_FAC
// field has set — matching how SYS$CREATE/SYS$OPEN recorded, back when
// the file was first opened, what the calling program said it wanted to
// do with it.
//
// If handle is already armed in the requested direction (a second
// SYS$CONNECT for the same file — real VMS allows more than one RAB to
// share a FAB, see rab.go's own doc comment), the existing Reader/Writer
// is reused rather than replaced: constructing a second, independent
// Writer over the same *volume.File would let two separate write
// positions race over one linear file, and a second Reader would
// silently restart from the beginning rather than continuing wherever
// the first one left off. Neither is what a real VAX program connecting
// a second RAB to an already-active stream would expect.
//
// The three-result shape matches createOnVolume's own convention
// (create.go): err is a genuine VAX-memory-access failure, to propagate
// unchanged; a nonzero failStatus is an ordinary RMS$_ failure for the
// caller to store into the RAB and return as R0; both zero means success.
func armForFAC(ctx *Context, handle *FileHandle, fabAddr uint32) (failStatus uint32, err error) {
	fac, err := ctx.loadByte(fabAddr + fabFAC)
	if err != nil {
		return 0, err
	}

	switch {
	case fac&facPut != 0:
		if handle.Writer == nil {
			w, err := odsrms.NewWriter(handle.File)
			if err != nil {
				// handle.File wasn't armed for writing (volume.File.
				// OpenForWrite never called on it) or has an
				// unsupported record format — a real ods2-layer
				// problem, not a Go bug, so it's reported as an
				// ordinary RMS device error rather than propagated.
				return rmsDeviceError, nil
			}

			handle.Writer = w
		}
	case fac&facGet != 0:
		if handle.Reader == nil {
			r, err := odsrms.NewReader(handle.File)
			if err != nil {
				return rmsDeviceError, nil
			}

			handle.Reader = r
		}
	default:
		// Neither FAB$V_PUT nor FAB$V_GET was ever asked for — nothing
		// for CONNECT to arm this stream to do.
		return rmsPrivilegeViolation, nil
	}

	return 0, nil
}
