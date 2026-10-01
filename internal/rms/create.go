package rms

import (
	"errors"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// consoleDeviceName is the one VMS device name this package's SYS$CREATE
// (and, eventually, SYS$PUT — docs/PHASE-22.md's subtask 7) treats as the
// terminal pseudo-device rather than a real ODS-2 volume: real, unmodified
// VAX/VMS software genuinely opens "TTA0:" this way (it's ordinary VMS
// behavior, not a govax shortcut — see docs/PHASE-22.md's "Why this phase
// looks different" section), and it's the one piece of the project's now-
// deleted Phase 10 stopgap (internal/rtl/rms.go, see git history) worth
// carrying forward unchanged.
//
// This is a plain, hardcoded name check rather than a lookup against
// internal/io's own DeviceTable/DeviceClassTT (which would tell "any
// terminal-class device", not just this one) for two reasons: first, this
// package deliberately has no dependency on internal/io's device-class
// concept at all (mount.go's own doc comment explains why MountTable
// avoids that same dependency); second, Phase 10's predecessor used
// exactly this same hardcoded check and nothing in this phase's scope
// (docs/PHASE-22.md) calls for generalizing it to other terminal devices.
const consoleDeviceName = "TTA0"

// SysCreate implements SYS$CREATE: given a FAB's VAX address (argv[0]),
// creates a new file (or, for the TTA0: special case, simply binds an IFI
// to the console) for output access, and stores the resulting IFI back
// into the FAB's FAB$W_IFI field.
//
// Only sequential organization (fab.go's orgSeq) is implemented — anything
// else in the FAB's FAB$B_ORG field is a real RMS$_ORG error, not a panic
// or a silent misinterpretation, matching docs/PHASE-22.md's scope.
func SysCreate(ctx *Context, argv []uint32) (uint32, error) {
	fabAddr := argv[0]

	fac, err := ctx.loadByte(fabAddr + fabFAC)
	if err != nil {
		return 0, err
	}

	// A calling program has to ask for write access to create a file —
	// matching real RMS, which rejects a CREATE that only asked for GET
	// access.
	if fac&facPut == 0 {
		return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsPrivilegeViolation)
	}

	nam, failStatus, err := fabNAMBlock(ctx, fabAddr)
	if err != nil {
		return 0, err
	}

	if failStatus != 0 {
		return fabStatus(ctx, fabAddr, failStatus, 0)
	}

	chain, failStatus, stv, err := ctx.xabChain(fabAddr)
	if err != nil {
		return 0, err
	}

	if failStatus != 0 {
		return fabStatus(ctx, fabAddr, failStatus, stv)
	}

	fop, err := ctx.loadLongword(fabAddr + fabFOP)
	if err != nil {
		return 0, err
	}

	// The spec's logical names are translated ("SYS$OUTPUT" becomes the
	// console terminal) and the default device and directory applied.
	// A search list creates the file in its first element, as RMS does.
	names, failStatus, err := ctx.expandFAB(fabAddr, nam)
	if err != nil {
		return 0, err
	}

	if failStatus == 0 && names[0].FNB&fnbWildcard != 0 {
		failStatus = rmsWildcardError
	}

	if failStatus == 0 && nam != 0 {
		if failStatus, err = ctx.checkESS(nam, names[0]); err != nil {
			return 0, err
		}
	}

	if failStatus != 0 {
		return fabStatus(ctx, fabAddr, failStatus, 0)
	}

	p := names[0]

	var (
		ifi     uint16
		created bool
	)

	if normalizeDeviceName(p.Lookup) == consoleDeviceName {
		ifi = ctx.Files.Alloc(&FileHandle{Console: ctx.Console})
	} else {
		newIFI, found, opened, failStatus, err := createOnVolume(ctx, fabAddr, p, fop&fopCIF != 0)
		if err != nil {
			return 0, err
		}

		if failStatus != 0 {
			return fabStatus(ctx, fabAddr, failStatus, 0)
		}

		// A new file takes the XABs' attributes; then, new or opened by
		// CIF, its NAM, FAB, and XABs report it.
		if !opened {
			h, _ := ctx.Files.Lookup(newIFI)

			in, err := ctx.readXABInputs(chain)
			if err != nil {
				return 0, err
			}

			if err := applyCreateXABs(h.File, in); err != nil {
				ctx.Files.Release(newIFI)

				return fabStatus(ctx, fabAddr, rmsDeviceError, 0)
			}
		}

		sts, err := ctx.reportOpened(fabAddr, nam, newIFI, found, chain, namOutputs{Expanded: true, Resultant: true})
		if err != nil {
			return 0, err
		}

		if sts != 0 {
			return fabStatus(ctx, fabAddr, sts, 0)
		}

		ifi = newIFI
		created = !opened
	}

	if err := ctx.storeWord(fabAddr+fabIFI, ifi); err != nil {
		return 0, err
	}

	status := uint32(rmsNormal)
	if created {
		status = rmsCreated
	}

	return storeStatus(ctx, fabAddr, fabSTS, fabSTV, status)
}

// createOnVolume is SysCreate's real-ODS-2-volume path: everything after
// "this isn't the TTA0: console special case" — resolving spec.Device to
// a mounted volume, validating the FAB's own record-organization/-format
// fields, creating the file via ods2's volume.CreateFile, and allocating
// an IFI for the result.
//
// Its three-result shape keeps SysCreate itself simple: err is a genuine
// Go/VAX-memory-access failure (propagate it up unchanged, exactly like
// every direct ctx.load*/ctx.store* call already does); a nonzero
// failStatus is an ordinary RMS$_ failure this function detected on its
// own (the caller stores it into the FAB and returns it as R0, via
// storeStatus); and, when both are zero-valued, ifi is the freshly
// allocated handle for the newly created file.
func createOnVolume(ctx *Context, fabAddr uint32, p parsedName, cif bool) (ifi uint16, found foundFile, opened bool, failStatus uint32, err error) {
	spec := p.spec()

	vol, ok := ctx.Mounts.Lookup(spec.Device)
	if !ok {
		return 0, found, false, rmsDeviceNotReady, nil
	}

	if !ctx.Mounts.Writable(spec.Device) {
		return 0, found, false, rmsPrivilegeViolation, nil
	}

	org, err := ctx.loadByte(fabAddr + fabORG)
	if err != nil {
		return 0, found, false, 0, err
	}

	if org != orgSeq {
		return 0, found, false, rmsInvalidOrg, nil
	}

	rfm, err := ctx.loadByte(fabAddr + fabRFM)
	if err != nil {
		return 0, found, false, 0, err
	}

	// ondisk.RecordFormatUndefined (0, VMS's FAB$C_UDF) is deliberately
	// treated as unsupported here rather than resolved to some chosen
	// default: real RMS lets a calling program leave FAB$B_RFM at 0 and
	// picks a sensible format on its behalf, but this phase's own test
	// program (docs/PHASE-22.md subtask 14) always sets every FAB field
	// explicitly, so there's no concrete case yet to justify picking one
	// default over another — a real gap worth revisiting once a program
	// that actually relies on FAB$C_UDF shows up, not a bug in what's
	// implemented so far.
	format := ondisk.RecordFormat(rfm)
	if format < ondisk.RecordFormatFixed || format > ondisk.RecordFormatStreamCR {
		return 0, found, false, rmsInvalidRFM, nil
	}

	rat, err := ctx.loadByte(fabAddr + fabRAT)
	if err != nil {
		return 0, found, false, 0, err
	}

	mrs, err := ctx.loadWord(fabAddr + fabMRS)
	if err != nil {
		return 0, found, false, 0, err
	}

	if spec.Name == "" && spec.Type == "" {
		return 0, found, false, rmsFileNameError, nil
	}

	dir, err := filespec.ResolveDirectory(vol, spec.Dirs)
	if err != nil {
		return 0, found, false, rmsDirNotFound, nil
	}

	name := specFileName(spec)

	version, ok := parseOpenVersion(spec.Version)
	if !ok {
		return 0, found, false, rmsInvalidVersion, nil
	}

	// FAB$V_CIF: a file that's already there is opened instead.
	if cif {
		if entry, sts := lookupVersion(dir, name, spec.Version); sts == 0 {
			ifi, sts, err := openFID(ctx, facPut, spec.Device, vol, entry.Fid)
			if err != nil || sts != 0 {
				return 0, found, false, sts, err
			}

			n, t := splitEntryName(entry.Name)
			found = foundFile{
				Parsed: p, Device: spec.Device, Dirs: spec.Dirs,
				Name: n, Type: t, Version: entry.Version,
				FID: entry.Fid, DID: dir.Header.Fid,
			}

			return ifi, found, true, 0, nil
		}
	}

	if version == 0 {
		if version, err = dir.NextVersion(name); err != nil {
			return 0, found, false, rmsDeviceError, nil
		}
	}

	bm, err := dir.Device.Bitmap()
	if err != nil {
		return 0, found, false, 0, err
	}

	ib, err := dir.Device.IndexBitmap()
	if err != nil {
		return 0, found, false, 0, err
	}

	recAttr := ondisk.RecAttr{
		Format:        format,
		Attributes:    rat,
		MaxRecordSize: mrs,
	}

	f, err := vol.CreateFileVersion(dir, name, version, recAttr, bm, ib)
	if errors.Is(err, volume.ErrExists) {
		return 0, found, false, rmsFileExists, nil
	}

	if err != nil {
		// A real ods2/volume-layer failure (out of disk space, a
		// directory-lookup problem past what ResolveDirectory already
		// checked, ...) rather than a Go-level bug — reported as an
		// RMS device error, not propagated as a Go error, so the
		// calling VAX program sees an ordinary (if generic) RMS$_
		// completion code instead of the whole emulator raising a
		// fault.
		return 0, found, false, rmsDeviceError, nil
	}

	n, t := splitEntryName(name)
	found = foundFile{
		Parsed: p, Device: spec.Device, Dirs: spec.Dirs,
		Name: n, Type: t, Version: version,
		FID: f.Header.Fid, DID: dir.Header.Fid,
	}
	found.HighVer, found.LowVer = versionsAround(dir, name, version)

	return ctx.Files.Alloc(&FileHandle{File: f, Writable: true}), found, false, 0, nil
}

// loadFileSpecString reads the FAB$L_FNA/FAB$B_FNS pair out of the FAB at
// fabAddr: the VAX-memory address of the file-specification string, and
// its exact length (RMS file-spec strings are not NUL-terminated — see
// fabFNS's own doc comment).
func loadFileSpecString(ctx *Context, fabAddr uint32) (string, error) {
	fns, err := ctx.loadByte(fabAddr + fabFNS)
	if err != nil {
		return "", err
	}

	fna, err := ctx.loadLongword(fabAddr + fabFNA)
	if err != nil {
		return "", err
	}

	return ctx.loadFixedString(fna, int(fns))
}
