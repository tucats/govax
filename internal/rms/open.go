package rms

import (
	"strconv"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
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

	// A FAB that asks for no access is opened for GET.
	fac = facAccess(fac, facGet)

	shr, err := ctx.loadByte(fabAddr + fabSHR)
	if err != nil {
		return 0, err
	}

	nam, failStatus, err := fabNAMBlock(ctx, fabAddr)
	if err != nil {
		return 0, err
	}

	if failStatus != 0 {
		return fabStatus(ctx, fabAddr, failStatus, 0)
	}

	// The XAB chain is checked once the name is processed: VMS writes
	// the expanded string first (the oracle's XAB cases 5, 6, and 8).
	chain, xabSts, xabSTV, err := ctx.xabChain(fabAddr)
	if err != nil {
		return 0, err
	}

	fop, err := ctx.loadLongword(fabAddr + fabFOP)
	if err != nil {
		return 0, err
	}

	if sts := ufoSharing(fop, shr); sts != 0 {
		return fabStatus(ctx, fabAddr, sts, 0)
	}

	// FAB$V_NAM: open by the NAM's file ID, or by its directory ID and
	// the file name (namfid.go).
	if nam != 0 && fop&fopNAM != 0 {
		if sts, done, err := ctx.openByNAM(fabAddr, nam, fac, chain, xabSts, xabSTV); done || err != nil {
			return sts, err
		}
	}

	// The NAM's outputs start clear.
	if nam != 0 {
		if err := ctx.clearNAMOutputs(nam); err != nil {
			return 0, err
		}
	}

	// See SysCreate for how the spec is translated and defaulted. A
	// search list opens the first element's file that exists; if none
	// does, the status for the last element tried is returned (User's
	// Manual §11.7). Each element tried is the NAM's expanded string in
	// turn.
	names, failStatus, err := ctx.expandFAB(fabAddr, nam)
	if err != nil {
		return 0, err
	}

	if failStatus != 0 {
		return fabStatus(ctx, fabAddr, failStatus, 0)
	}

	var (
		ifi   uint16
		found foundFile
		ok    bool

		// deviceName is the device a record device's or the terminal's
		// open names (for a user file open, ufo.go).
		deviceName string
	)

	for _, p := range names {
		if nam != 0 {
			if failStatus, err = ctx.expandedOrESS(nam, p); err != nil || failStatus != 0 {
				if err != nil {
					return 0, err
				}

				return fabStatus(ctx, fabAddr, failStatus, 0)
			}
		}

		if xabSts != 0 {
			return fabStatus(ctx, fabAddr, xabSts, xabSTV)
		}

		if p.FNB&fnbWildcard != 0 {
			return fabStatus(ctx, fabAddr, rmsWildcardError, 0)
		}

		devIFI, isDevice, devStatus, devSTV, err := ctx.openRecordDevice(fabAddr, p.Lookup, fac)
		if err != nil {
			return 0, err
		}

		if isDevice {
			if devStatus != 0 {
				return fabStatus(ctx, fabAddr, devStatus, devSTV)
			}

			ifi, failStatus, deviceName = devIFI, 0, p.Lookup

			break
		}

		if normalizeDeviceName(p.Lookup) == consoleDeviceName {
			ifi, failStatus, deviceName = ctx.Files.Alloc(&FileHandle{Console: ctx.Console, Access: fac}), 0, p.Lookup

			break
		}

		ifi, found, failStatus, err = openOnVolume(ctx, fac, shr, p)
		if err != nil {
			return 0, err
		}

		if failStatus == 0 {
			ok = true

			break
		}

		if failStatus != rmsFileNotFound && failStatus != rmsDeviceNotReady && failStatus != rmsDirNotFound {
			break
		}
	}

	if failStatus != 0 {
		return fabStatus(ctx, fabAddr, failStatus, stvFor(failStatus))
	}

	if ok {
		if sts, err := ctx.reportOpened(fabAddr, nam, ifi, found, chain, allOutputs, xabOpen); err != nil || sts != 0 {
			if err != nil {
				return 0, err
			}

			return fabStatus(ctx, fabAddr, sts, 0)
		}
	}

	// FAB$V_UFO: the file goes to a channel instead (ufo.go).
	if fop&fopUFO != 0 {
		if !ok {
			found.Device = deviceName
		}

		return ctx.userFileOpen(fabAddr, ifi, found.Device, rmsNormal)
	}

	if err := ctx.storeWord(fabAddr+fabIFI, ifi); err != nil {
		return 0, err
	}

	return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsNormal)
}

// openOnVolume is SysOpen's real-ODS-2-volume path: everything after
// "this isn't the TTA0: console special case" — resolving the device to a
// mounted volume, looking the file's directory entry up by name and
// version, and opening it (openFID). It returns the new IFI and the file
// found, for the NAM.
//
// err is a genuine Go/VAX-memory-access failure to propagate unchanged,
// a nonzero failStatus is an ordinary RMS$_ failure for the caller to
// store into the FAB and return as R0, and both zero means ifi holds the
// freshly allocated handle for the now-open file.
func openOnVolume(ctx *Context, fac, shr byte, p parsedName) (ifi uint16, found foundFile, failStatus uint32, err error) {
	spec := p.spec()

	vol, ok := ctx.Mounts.Lookup(spec.Device)
	if !ok {
		return 0, found, rmsDeviceNotReady, nil
	}

	if spec.Name == "" && spec.Type == "" {
		return 0, found, rmsFileNotFound, nil
	}

	dir, err := filespec.ResolveDirectory(vol, spec.Dirs)
	if err != nil {
		return 0, found, rmsDirNotFound, nil
	}

	entry, sts := lookupVersion(dir, specFileName(spec), spec.Version)
	if sts != 0 {
		return 0, found, sts, nil
	}

	if ifi, sts, err = openFID(ctx, fac, shr, spec.Device, vol, entry.Fid); err != nil || sts != 0 {
		return 0, found, sts, err
	}

	name, typ := splitEntryName(entry.Name)
	found = foundFile{
		Parsed: p, Device: spec.Device, Dirs: spec.Dirs,
		Name: name, Type: typ, Version: entry.Version,
		FID: entry.Fid, DID: dir.Header.Fid,
	}

	return ifi, found, 0, nil
}

// openFID opens the file whose ID is fid on vol (mounted as device) for
// access fac with sharing shr, and allocates its IFI. The file system
// access (sharing.go) arms the file for writing when fac asks to write,
// and fails with RMS$_FLK if another opener's access or sharing conflicts.
func openFID(ctx *Context, fac, shr byte, device string, vol *volume.Volume, fid ondisk.Fid) (uint16, uint32, error) {
	// Real RMS lets a program SYS$OPEN a file read-only even on a
	// read-only-mounted device — only actually asking to write is a
	// problem, unlike SYS$CREATE (create.go), which always implies
	// writing and so always has to check this.
	wantsWrite := fac&facWrite != 0
	if wantsWrite && !ctx.Mounts.Writable(device) {
		return 0, rmsPrivilegeViolation, nil
	}

	mode := accessMode(fac, shr)

	claim := ctx.Mounts.claimOpen(device, fileIDFrom(fid), fac, shr)
	if claim == nil {
		return 0, rmsFileLocked, nil
	}

	// A file ID that names no file (a bad FAB$V_NAM open, or a directory
	// entry pointing at a header ods2 then failed to read) is RMS$_FNF,
	// as is a file deleted while another process has it open.
	a, err := vol.Access(fid, mode)
	if err != nil {
		claim.release()

		return 0, accessStatus(err), nil
	}

	h := &FileHandle{File: a.File, Accessor: a, Mode: mode, Share: effectiveSharing(fac, shr), Writable: wantsWrite, Access: fac, claim: claim}
	ctx.countIO(ioOpenBuffered, 0)

	return ctx.Files.Alloc(h), 0, nil
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
