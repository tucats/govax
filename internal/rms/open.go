package rms

import (
	"strconv"

	"github.com/tucats/ods2/filespec"
)

// SysOpen implements SYS$OPEN: given a FAB's VAX address (argv[0]), finds
// an already-existing file (as opposed to SYS$CREATE, which always makes a
// brand-new one) named by the FAB's file specification, and stores the
// resulting IFI back into the FAB's FAB$W_IFI field — the same handback
// SYS$CREATE performs (create.go), just for a file that was already there.
//
// # A quick Go note for readers new to the language
//
// Like every other handler in this package, SysOpen returns (uint32,
// error): the uint32 is what the calling VAX program sees in register R0,
// and a non-nil error means something went wrong at the Go/emulator level
// itself (for instance, fabAddr pointing at unmapped VAX memory) rather
// than an ordinary RMS-level condition — see status.go's storeStatus doc
// comment for the fuller explanation.
//
// # What's genuinely new here versus SYS$CREATE
//
// Console handling, logical-name translation, and file-spec parsing all
// mirror SysCreate's own doc comment (create.go) exactly, since a file
// spec is resolved the same way regardless of which service is asking —
// see that function's comments for the fuller explanation of each. The one
// thing this function does that SysCreate never needs to is honor
// FAB$B_FAC's PUT/UPD bits (fab.go's facPut/facUpd) to decide whether the
// file it just found also needs arming for writing
// (volume.File.OpenForWrite) — SYS$CONNECT (connect.go's armForFAC) is
// what later actually constructs the Reader/Writer for record-by-record
// access, but it can only succeed at that for a file already armed this
// way, exactly mirroring how SYS$CREATE's own createOnVolume arms a
// brand-new file for writing via volume.Volume.CreateFile internally.
func SysOpen(ctx *Context, argv []uint32) (uint32, error) {
	fabAddr := argv[0]

	fac, err := ctx.loadByte(fabAddr + fabFAC)
	if err != nil {
		return 0, err
	}

	// A calling program has to ask for at least one kind of access to open
	// a file at all — matching SysCreate's own up-front FAB$B_FAC check
	// (create.go), just against the fuller set of access bits SYS$OPEN
	// itself recognizes (GET, in addition to CREATE's PUT/UPD).
	if fac&(facGet|facPut|facUpd) == 0 {
		return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsPrivilegeViolation)
	}

	fn, err := loadFileSpecString(ctx, fabAddr)
	if err != nil {
		return 0, err
	}

	// See create.go's own SysCreate for why a file spec that is itself a
	// defined logical name gets translated before being parsed.
	if ln, found := ctx.Logicals.Get("LNM$FILE_DEV", fn, 0); found {
		fn = ln.Value
	}

	spec, err := filespec.Parse(fn, filespec.Spec{})
	if err != nil {
		return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsFileNotFound)
	}

	var ifi uint16

	if normalizeDeviceName(spec.Device) == consoleDeviceName {
		ifi = ctx.Files.Alloc(&FileHandle{Console: ctx.Console})
	} else {
		newIFI, failStatus, err := openOnVolume(ctx, fac, spec)
		if err != nil {
			return 0, err
		}

		if failStatus != 0 {
			return storeStatus(ctx, fabAddr, fabSTS, fabSTV, failStatus)
		}

		ifi = newIFI
	}

	if err := ctx.storeWord(fabAddr+fabIFI, ifi); err != nil {
		return 0, err
	}

	return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsNormal)
}

// openOnVolume is SysOpen's real-ODS-2-volume path: everything after
// "this isn't the TTA0: console special case" — resolving spec.Device to a
// mounted volume, looking the file's directory entry up by name/version,
// opening it via ods2's volume.Volume.OpenFID, arming it for writing if
// fac asked for that, and allocating an IFI for the result.
//
// Its three-result shape matches create.go's createOnVolume exactly (see
// that function's own doc comment for the full explanation): err is a
// genuine Go/VAX-memory-access failure to propagate unchanged, a nonzero
// failStatus is an ordinary RMS$_ failure for the caller to store into the
// FAB and return as R0, and both zero means ifi holds the freshly
// allocated handle for the now-open file.
func openOnVolume(ctx *Context, fac byte, spec filespec.Spec) (ifi uint16, failStatus uint32, err error) {
	vol, ok := ctx.Mounts.Lookup(spec.Device)
	if !ok {
		return 0, rmsDeviceNotReady, nil
	}

	// Real RMS lets a program SYS$OPEN a file read-only even on a
	// read-only-mounted device — only actually asking to write is a
	// problem, unlike SYS$CREATE (create.go), which always implies
	// writing and so always has to check this.
	wantsWrite := fac&(facPut|facUpd) != 0
	if wantsWrite && !ctx.Mounts.Writable(spec.Device) {
		return 0, rmsPrivilegeViolation, nil
	}

	if spec.Name == "" {
		return 0, rmsFileNotFound, nil
	}

	version, ok := parseOpenVersion(spec.Version)
	if !ok {
		return 0, rmsInvalidVersion, nil
	}

	dir, err := filespec.ResolveDirectory(vol, spec.Dirs)
	if err != nil {
		return 0, rmsFileNotFound, nil
	}

	name := spec.Name
	if spec.Type != "" {
		name += "." + spec.Type
	}

	entry, err := dir.Lookup(name, version)
	if err != nil {
		// A well-formed spec naming a file that genuinely isn't in this
		// directory — Directory.Lookup's own error, not a Go-level bug.
		return 0, rmsFileNotFound, nil
	}

	f, err := vol.OpenFID(entry.Fid)
	if err != nil {
		// A directory entry pointing at a file header ods2 itself then
		// failed to read — a real ods2/volume-layer problem, not
		// something this package caused, so it's reported as a generic
		// RMS device error rather than propagated as a Go error (the same
		// convention create.go's createOnVolume uses for a failing
		// vol.CreateFile).
		return 0, rmsDeviceError, nil
	}

	if wantsWrite {
		bm, err := f.Device.Bitmap()
		if err != nil {
			return 0, 0, err
		}

		ib, err := f.Device.IndexBitmap()
		if err != nil {
			return 0, 0, err
		}

		if err := f.OpenForWrite(bm, ib); err != nil {
			return 0, rmsDeviceError, nil
		}
	}

	return ctx.Files.Alloc(&FileHandle{File: f}), 0, nil
}

// parseOpenVersion interprets a file spec's version field (spec.Version —
// the raw text after ';', e.g. "5", or "" if no version was written at
// all) into the uint16 volume.Directory.Lookup expects, and whether that
// interpretation succeeded at all.
//
// Only two shapes are supported: no version (or the literal "0", which
// isn't itself a legal VMS version number), meaning "the highest existing
// version" — Lookup's own convention for a 0 argument — and an explicit
// positive version number. Real VMS's fuller version syntax (";*" for
// every version, ";-1" for "N versions back from the highest") is a
// wildcard-matching concern package filespec's own Glob already implements
// for directory-listing use cases (internal/console's future DIRECTORY
// command, say); SYS$OPEN's job is resolving one specific file, so this
// function deliberately doesn't reuse that machinery — anything it doesn't
// recognize is reported back to the caller as rmsInvalidVersion rather
// than silently guessing what was meant.
func parseOpenVersion(v string) (uint16, bool) {
	if v == "" || v == "0" {
		return 0, true
	}

	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, false
	}

	return uint16(n), true
}
