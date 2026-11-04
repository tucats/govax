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
// Sequential, relative (relative.go), and indexed (indexed.go) files are
// made; anything else in FAB$B_ORG is RMS$_ORG.
func SysCreate(ctx *Context, argv []uint32) (uint32, error) {
	fabAddr := argv[0]

	fac, err := ctx.loadByte(fabAddr + fabFAC)
	if err != nil {
		return 0, err
	}

	// A FAB that asks for no access is created for PUT. One that asks
	// for access, but not PUT, is refused (unconfirmed: govax's choice).
	fac = facAccess(fac, facPut)

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

	// FAB$V_PPF is for opens in executive mode, which no program's is:
	// VMS 7.1 refused it before processing the name (testdata/ppf71's
	// O5).
	if fop&fopPPF != 0 {
		return fabStatus(ctx, fabAddr, rmsPrivilegeViolation, 0)
	}

	// FAB$L_ALQ as the program set it: a user file open's end of file
	// (ufoEndOfFile), before the create writes back the allocation.
	alqInput, err := ctx.loadLongword(fabAddr + fabOffset("ALQ"))
	if err != nil {
		return 0, err
	}

	if shr, err := ctx.loadByte(fabAddr + fabSHR); err != nil {
		return 0, err
	} else if sts := ufoSharing(fop, shr); sts != 0 {
		return fabStatus(ctx, fabAddr, sts, 0)
	} else if sts := blockSharing(fac, shr); sts != 0 {
		return fabStatus(ctx, fabAddr, sts, 0)
	}

	// The spec's logical names are translated ("SYS$OUTPUT" becomes the
	// console terminal) and the default device and directory applied.
	// A search list creates the file in its first element, as RMS does.
	// The NAM's outputs start clear; the expanded string and FNB are
	// written even when the create then fails (the oracle's CREATE cases
	// 3, 8, and 9).
	if nam != 0 {
		if err := ctx.clearNAMOutputs(nam); err != nil {
			return 0, err
		}
	}

	names, failStatus, err := ctx.expandFAB(fabAddr, nam)
	if err != nil {
		return 0, err
	}

	if failStatus != 0 {
		return fabStatus(ctx, fabAddr, failStatus, 0)
	}

	p := names[0]

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

	// A process-permanent file is reached indirectly, $CREATE being
	// $OPEN then (ppfaccess.go).
	if p.Permanent != nil {
		return ctx.openIndirect(fabAddr, nam, p, fop)
	}

	var (
		ifi     uint16
		created bool
		device  string
	)

	devIFI, isDevice, devStatus, devSTV, err := ctx.openRecordDevice(fabAddr, p.Lookup, fac)
	if err != nil {
		return 0, err
	}

	switch {
	case isDevice && devStatus != 0:
		return fabStatus(ctx, fabAddr, devStatus, devSTV)
	case isDevice:
		ifi, device = devIFI, p.Lookup
	case normalizeDeviceName(p.Lookup) == consoleDeviceName:
		ifi, device = ctx.Files.Alloc(&FileHandle{Console: ctx.Console, Access: fac}), p.Lookup
	default:
		newIFI, found, opened, plan, failStatus, failSTV, err := createOnVolume(ctx, fabAddr, fac, p, fop&fopCIF != 0)
		if err != nil {
			return 0, err
		}

		if failStatus != 0 {
			if failSTV == 0 {
				failSTV = stvFor(failStatus)
			}

			return fabStatus(ctx, fabAddr, failStatus, failSTV)
		}

		if !opened {
			ctx.countIO(ioCreateBuffered, ioCreateDirect)
		}

		// A new file takes the XABs' attributes; then, new or opened by
		// CIF, its NAM, FAB, and XABs report it.
		if !opened {
			h, _ := ctx.Files.Lookup(newIFI)

			in, err := ctx.readXABInputs(chain)
			if err != nil {
				return 0, err
			}

			alq, err := ctx.loadLongword(fabAddr + fabOffset("ALQ"))
			if err != nil {
				return 0, err
			}

			if in.HasAll {
				alq = in.Alloc
			}

			// An indexed file's allocation is its areas', which its
			// prolog's writing asks for (indexed.go), and its default
			// extension area 0's.
			if plan != nil {
				alq, in.HasAll = 0, false
			}

			// A relative file has its prologue at least (relative.go).
			relative := isRelative(h.File)
			if relative {
				alq = max(alq, 1)
			}

			if err := applyCreateXABs(h.File, in, alq); err != nil {
				ctx.abandon(newIFI)

				return fabStatus(ctx, fabAddr, rmsDeviceError, 0)
			}

			// A sequential or relative file's FOP CTG or CBT marks its
			// header when it was given blocks in one run, as VMS 7.1's
			// CREATE/FDL files were (testdata/idx88's round 3: C29 and
			// C2A; C2G's CONTIGUOUS with no allocation has neither). An
			// indexed file's comes from its areas (indexed.go).
			if plan == nil && alq > 0 {
				if err := markContiguity(h.File, fop); err != nil {
					ctx.abandon(newIFI)

					return fabStatus(ctx, fabAddr, rmsDeviceError, 0)
				}
			}

			if plan != nil {
				sts, stv, err := ctx.initIndexed(h.File, plan)
				if err != nil {
					return 0, err
				}

				if sts != 0 {
					ctx.abandon(newIFI)

					return fabStatus(ctx, fabAddr, sts, stv)
				}
			}

			if relative {
				mrn, err := ctx.loadLongword(fabAddr + fabMRN)
				if err != nil {
					return 0, err
				}

				if err := initRelative(h.File, mrn); err != nil {
					ctx.abandon(newIFI)

					return fabStatus(ctx, fabAddr, rmsDeviceError, 0)
				}
			}
		}

		mode := xabCreate
		if opened {
			mode = xabOpen
		}

		sts, err := ctx.reportOpened(fabAddr, nam, newIFI, found, chain, allOutputs, mode)
		if err != nil {
			return 0, err
		}

		if sts != 0 {
			return fabStatus(ctx, fabAddr, sts, 0)
		}

		ifi = newIFI
		created = !opened
		device = found.Device
	}

	if err := ctx.storeWord(fabAddr+fabIFI, ifi); err != nil {
		return 0, err
	}

	// RMS$_CREATED says FAB$V_CIF made a new file; an ordinary create
	// is RMS$_NORMAL (the oracle's CREATE case 1).
	status := uint32(rmsNormal)
	if created && fop&fopCIF != 0 {
		status = rmsCreated
	}

	// FAB$V_UFO: the file goes to a channel instead (ufo.go), a new
	// one ending where FAB$L_ALQ does.
	if fop&fopUFO != 0 {
		if created {
			ctx.ufoEndOfFile(ifi, alqInput)
		}

		return ctx.userFileOpen(fabAddr, ifi, device, status)
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
//
// An indexed file's $CREATE is checked here, before the file exists
// (indexed.go): plan is what initIndexed makes of it, and failSTV a
// refusal's STV, 0 for stvFor's.
func createOnVolume(ctx *Context, fabAddr uint32, fac byte, p parsedName, cif bool) (ifi uint16, found foundFile, opened bool, plan *indexedPlan, failStatus, failSTV uint32, err error) {
	spec := p.spec()

	vol, ok := ctx.Mounts.Lookup(spec.Device)
	if !ok {
		return 0, found, false, nil, rmsDeviceNotReady, 0, nil
	}

	// The device's block size is the FAB's once the device is found,
	// whether the create then succeeds or not (the oracle's CREATE cases
	// 3 and 8).
	if err := ctx.storeWord(fabAddr+fabOffset("BLS"), ondisk.BlockSize); err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	if !ctx.Mounts.Writable(spec.Device) {
		return 0, found, false, nil, rmsPrivilegeViolation, 0, nil
	}

	org, err := ctx.loadByte(fabAddr + fabORG)
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	// Sequential, relative (relative.go), and indexed files (indexed.go).
	if org != orgSeq && org != orgRel && org != orgIdx {
		return 0, found, false, nil, rmsInvalidOrg, 0, nil
	}

	rfm, err := ctx.loadByte(fabAddr + fabRFM)
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	// FAB$C_UDF (0), undefined records, is a sequential file's only: VMS
	// 7.1's CREATE/FDL made one from FORMAT undefined (testdata/idx88's
	// round 3, C28). A relative or indexed file of them is RMS$_RFM
	// (unconfirmed: govax's choice).
	format := ondisk.RecordFormat(rfm)
	if format > ondisk.RecordFormatStreamCR || (format == ondisk.RecordFormatUndefined && org != orgSeq) {
		return 0, found, false, nil, rmsInvalidRFM, 0, nil
	}

	rat, err := ctx.loadByte(fabAddr + fabRAT)
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	mrs, err := ctx.loadWord(fabAddr + fabMRS)
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	// A fixed-length record needs a length, and one that fits a word's
	// positive values (VMS 7.1: testdata/for81, FOR81R and FOR81's #205).
	if format == ondisk.RecordFormatFixed && (mrs == 0 || mrs > maxFixedRecord) {
		return 0, found, false, nil, rmsInvalidMRS, 0, nil
	}

	// A VFC record's fixed control is FAB$B_FSZ bytes, 0 meaning 2 (RMS
	// manual, FAB$B_FSZ).
	var fsz byte

	if format == ondisk.RecordFormatVFC {
		if fsz, err = ctx.loadByte(fabAddr + fabOffset("FSZ")); err != nil {
			return 0, found, false, nil, 0, 0, err
		}

		if fsz == 0 {
			fsz = 2
		}
	}

	var bks byte

	if org == orgRel {
		var sts uint32
		if bks, sts, err = ctx.checkRelativeCreate(fabAddr, format, mrs, fsz); err != nil || sts != 0 {
			return 0, found, false, nil, sts, 0, err
		}
	}

	if org == orgIdx {
		var sts, stv uint32
		if plan, sts, stv, err = ctx.checkIndexedCreate(fabAddr, format, mrs); err != nil || sts != 0 {
			return 0, found, false, nil, sts, stv, err
		}

		bks = plan.bks
	}

	shr, err := ctx.loadByte(fabAddr + fabSHR)
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	// A name with neither a name nor a type is made as ".;1": VMS 7.1's
	// CREATE/FDL with no file named and no FILE NAME made DUA1:[R3].;1
	// (testdata/idx88's round 3, C2.C).
	dir, err := filespec.ResolveDirectory(vol, spec.Dirs)
	if err != nil {
		return 0, found, false, nil, rmsDirNotFound, 0, nil
	}

	name := specFileName(spec)

	version, ok := parseOpenVersion(spec.Version)
	if !ok {
		return 0, found, false, nil, rmsInvalidVersion, 0, nil
	}

	// FAB$V_CIF: a file that's already there is opened instead.
	if cif {
		if entry, sts := lookupVersion(dir, name, spec.Version); sts == 0 {
			ifi, sts, err := openFID(ctx, fac, shr, spec.Device, vol, entry.Fid)
			if err != nil || sts != 0 {
				return 0, found, false, nil, sts, 0, err
			}

			n, t := splitEntryName(entry.Name)
			found = foundFile{
				Parsed: p, Device: spec.Device, Dirs: spec.Dirs,
				Name: n, Type: t, Version: entry.Version,
				FID: entry.Fid, DID: dir.Header.Fid,
			}

			return ifi, found, true, nil, 0, 0, nil
		}
	}

	// FAB$V_MXV: the version is the higher of the one given and one more
	// than the directory's highest (the RMS manual's FAB$L_FOP), so FDL's
	// MAXIMIZE_VERSION, a default, made [IN]S35.IDX;2 from a NAME of
	// S35.IDX;1 (testdata/idx88's round 3, C13).
	fopIn, err := ctx.loadLongword(fabAddr + fabFOP)
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	if version == 0 || fopIn&fopMXV != 0 {
		next, err := dir.NextVersion(name)
		if err != nil {
			return 0, found, false, nil, rmsDeviceError, 0, nil
		}

		version = max(version, next)
	}

	bm, err := dir.Device.Bitmap()
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	ib, err := dir.Device.IndexBitmap()
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	// FAB$W_DEQ is the file's default extension quantity, kept in its
	// header (the RMS manual, FAB$W_DEQ); an XABALL's DEQ, applied later
	// with the XABs, takes its place.
	deq, err := ctx.loadWord(fabAddr + fabOffset("DEQ"))
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	// The header's record type byte holds the organization in its high
	// nibble, the record format in its low one.
	recAttr := ondisk.RecAttr{
		Format:        ondisk.RecordFormat(org | rfm),
		Attributes:    rat,
		MaxRecordSize: mrs,
		DefaultExtend: deq,
		VfcSize:       fsz,
		BucketSize:    bks,
	}

	// A fixed-length file's longest record is its one size (XAB$W_LRL), and
	// so is a relative file's, whatever its format (VMS 7.1: FOR81R).
	if format == ondisk.RecordFormatFixed || org == orgRel {
		recAttr.RecordSize = mrs
	}

	f, err := vol.CreateFileVersion(dir, name, version, recAttr, bm, ib)
	if errors.Is(err, volume.ErrExists) {
		return 0, found, false, nil, rmsFileExists, 0, nil
	}

	if err != nil {
		// A real ods2/volume-layer failure (out of disk space, a
		// directory-lookup problem past what ResolveDirectory already
		// checked, ...) rather than a Go-level bug — reported as an
		// RMS device error, not propagated as a Go error, so the
		// calling VAX program sees an ordinary (if generic) RMS$_
		// completion code instead of the whole emulator raising a
		// fault.
		return 0, found, false, nil, rmsDeviceError, 0, nil
	}

	// The creator is the new file's first accessor, so no one else can
	// have it: the access can't fail but by a Go-level error.
	mode := accessMode(fac, shr)

	a, err := vol.AccessFile(f, mode)
	if err != nil {
		return 0, found, false, nil, rmsDeviceError, 0, nil
	}

	fop, err := ctx.loadLongword(fabAddr + fabFOP)
	if err != nil {
		return 0, found, false, nil, 0, 0, err
	}

	n, t := splitEntryName(name)
	found = foundFile{
		Parsed: p, Device: spec.Device, Dirs: spec.Dirs,
		Name: n, Type: t, Version: version,
		FID: f.Header.Fid, DID: dir.Header.Fid,
	}
	found.HighVer, found.LowVer = versionsAround(dir, name, version)

	// FAB$V_TMP and FAB$V_TMD: the file has no directory entry, and its name
	// no version (fop.go).
	if fop&(fopTMP|fopTMD) != 0 {
		if err := makeTemporary(dir, name, version, f, fop); err != nil {
			return 0, found, false, nil, rmsDeviceError, 0, nil
		}

		found.Version = 0
	}

	h := &FileHandle{File: f, Accessor: a, Mode: mode, Share: effectiveSharing(fac, shr), Writable: true, Access: fac,
		claim: ctx.Mounts.claimOpen(spec.Device, fileIDFrom(f.Header.Fid), fac, shr)}

	return ctx.Files.Alloc(h), found, false, plan, 0, 0, nil
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

// maxFixedRecord is the longest record a FIX file may have.
const maxFixedRecord = 32767

// markContiguity sets f's header's FCH$V_CONTIG for FOP CTG, or
// FCH$V_CONTIGB for CBT, when its blocks are one run. A CTG file whose
// blocks aren't is left unmarked (unconfirmed: VMS's $CREATE would have
// refused it, and ods2 gives a new file's first blocks in one run when it
// can).
func markContiguity(f *volume.File, fop uint32) error {
	var bit uint32

	switch {
	case fop&fopCTG != 0:
		bit = ondisk.FchContig
	case fop&fopCBT != 0:
		bit = ondisk.FchContigB
	default:
		return nil
	}

	if !contiguous(f.Extents) {
		return nil
	}

	return volume.UpdateHeader(f, func(h *ondisk.FileHeader, _ *ondisk.Ident) {
		h.FileCharacteristics |= bit
	})
}
