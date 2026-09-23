package rms

import (
	"errors"
	"io"
)

// SysGet implements SYS$GET: given a RAB's VAX address (argv[0] — SYS$GET
// takes exactly one argument, like SYS$PUT and SYS$CONNECT, and unlike
// SYS$CREATE/SYS$OPEN/SYS$CLOSE, which take a FAB), reads the next
// sequential record from whatever file the RAB is currently connected to
// (via SYS$CONNECT — see connect.go) and copies its bytes into the
// calling program's own record buffer.
//
// # A quick Go note for readers new to the language
//
// Like every other handler in this package, SysGet takes a []uint32
// (argv[0] is "the first, and in SYS$GET's case only, argument the
// calling VAX program passed") and returns (uint32, error): the uint32
// becomes register R0 back in the emulated VAX program (the real
// system-service completion-status convention), and a non-nil error means
// something went wrong at the Go/emulator level itself (for instance, an
// address pointing at unmapped VAX memory) rather than an ordinary
// RMS-level condition the calling program is expected to check for — see
// status.go's storeStatus doc comment for the fuller explanation of that
// split, and put.go's own doc comment (SysPut is SysGet's closest sibling
// — everything said there about argv/return shape and sequential-only
// scope applies equally here).
//
// # Two destinations for the record: RAB$L_RBF versus RAB$L_UBF
//
// A calling program almost always just supplies RAB$L_RBF (a record
// buffer address) and lets SYS$GET fill it — see rab.go's own doc comment
// on rabRBF/rabRSZ. But real RMS also lets a program supply a *separate*
// "user buffer" via RAB$L_UBF/RAB$W_USZ (rab.go's rabUBF/rabUSZ) when it
// specifically wants the record copied somewhere other than RMS's own
// default destination. This function honors that: if RAB$L_UBF is
// nonzero, the record is copied there instead of RAB$L_RBF, and — since
// the calling program also declared RAB$W_USZ as that buffer's exact
// capacity — a record too big to fit is a real error (RMS$_RSZ) rather
// than an overflow. The plain RAB$L_RBF path has no such capacity check:
// real RMS trusts the calling program to have sized that buffer itself
// (typically from the file's own FAB$W_MRS, which the program already
// knows), the same "trust the caller's own declared lengths" convention
// SysPut already relies on for its outgoing RAB$W_RSZ/RAB$L_RBF pair.
//
// Either way, RAB$W_RSZ is always written with the record's true length
// once SYS$GET succeeds — that field is how the calling program finds out
// how long the record it just read actually was, regardless of which
// buffer it ended up in.
func SysGet(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	// A RAB only ever refers to a file through the IFI SYS$CONNECT already
	// copied into its RAB$W_ISI field (rab.go's rabISI) — SYS$GET, like
	// SYS$PUT, never touches the RAB's related FAB directly.
	ifi, err := ctx.loadWord(rabAddr + rabISI)
	if err != nil {
		return 0, err
	}

	handle, ok := ctx.Files.Lookup(ifi)
	if !ok {
		// This RAB was never SYS$CONNECTed at all — a real, expected
		// RMS$_IFI condition, not a bug in this package (see SysPut's own
		// identical check for the fuller explanation).
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidIFI)
	}

	rac, err := ctx.loadByte(rabAddr + rabRAC)
	if err != nil {
		return 0, err
	}

	if rac != racSeq {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidRAC)
	}

	if handle.IsConsole() {
		// ctx.Console (ifi.go's FileHandle) is a plain io.Writer — this
		// emulator's console pseudo-device has no way to supply an input
		// record at all, so there's nothing for SYS$GET to read here.
		// Real, unmodified VMS RMS does support interactive terminal
		// input through SYS$GET, but that's out of this phase's scope
		// (docs/PHASE-22.md only carries the TTA0: special case forward
		// for its *output* side — SysCreate/SysPut). Reported the same
		// RMS$_PRV way as the "wrong direction" check just below, since
		// both boil down to the same thing: this stream isn't set up for
		// reading.
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsPrivilegeViolation)
	}

	if handle.Reader == nil {
		// SYS$CONNECT's own armForFAC (connect.go) already reports
		// RMS$_PRV at CONNECT time for a FAB that never asked for
		// FAB$V_GET access, so a nil Reader here means this RAB's FAB was
		// instead armed for writing only (FAB$V_PUT) and a GET was
		// attempted through it anyway — the same "wrong direction"
		// condition, just discovered one step later. Mirrors SysPut's own
		// equivalent nil-Writer check.
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsPrivilegeViolation)
	}

	record, err := handle.Reader.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			// Not a failure at all from the calling VAX program's point
			// of view — RMS$_EOF is how a real sequential-file reader
			// finds out it has read every record the file has, the same
			// role Go's own io.EOF plays for this package's underlying
			// odsrms.Reader.
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsEOF)
		}

		// Anything else (odsrms.ErrCorruptRecord: a truncated or
		// self-inconsistent record's on-disk framing) is a genuine
		// problem with the file's own content, not a Go-level bug in
		// this package, so it's reported as a generic RMS device error
		// rather than propagated as a Go error — the same convention
		// this package already uses for every other ods2-layer failure
		// (for instance, create.go's createOnVolume on a failing
		// vol.CreateFile).
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
	}

	destAddr, err := ctx.loadLongword(rabAddr + rabUBF)
	if err != nil {
		return 0, err
	}

	if destAddr == 0 {
		// No separate user buffer was supplied — the ordinary case — so
		// the record goes straight into RAB$L_RBF, with no capacity
		// check (see this function's own doc comment on why the RBF path
		// trusts the caller).
		destAddr, err = ctx.loadLongword(rabAddr + rabRBF)
		if err != nil {
			return 0, err
		}
	} else {
		usz, err := ctx.loadWord(rabAddr + rabUSZ)
		if err != nil {
			return 0, err
		}

		if len(record) > int(usz) {
			// A caller that did supply a separate user buffer also said
			// exactly how big it is (RAB$W_USZ) — unlike the plain RBF
			// path above, this really is a capacity real RMS enforces,
			// and a record too big for it is the same RMS$_RSZ condition
			// SysPut already reports for an outgoing record that doesn't
			// fit the file's own declared format.
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsRecordTooBig)
		}
	}

	for i, b := range record {
		if err := ctx.storeByte(destAddr+uint32(i), b); err != nil {
			return 0, err
		}
	}

	if err := ctx.storeWord(rabAddr+rabRSZ, uint16(len(record))); err != nil {
		return 0, err
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}
