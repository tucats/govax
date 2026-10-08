package rms

import (
	"strconv"
	"strings"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file is the NAM block on $OPEN and $CREATE (docs/PHASE-33.md,
// subtask 4), from the RMS Reference Manual's tables for the two services
// (RMS-7, RMS-8, RMS-51, RMS-52) and FAB$V_NAM.

// expandFAB runs name processing on a FAB's file name, with its default
// name and its NAM's related file (nam is 0 when it has none). It returns
// the names in search order, or the RMS status to fail with.
func (ctx *Context) expandFAB(fab, nam uint32) ([]parsedName, uint32, error) {
	in, err := fabNames(ctx, fab, nam)
	if err != nil {
		return nil, 0, err
	}

	names, sts := ctx.expandName(in)

	return names, sts, nil
}

// foundFile is the file $OPEN or $CREATE found or made, as its NAM
// reports it.
type foundFile struct {
	// Parsed is the expanded name it was found by.
	Parsed parsedName

	// Device is the volume's device name, and Dirs the file's directory.
	Device string
	Dirs   []string

	// Name, Type, and Version are its directory entry's.
	Name, Type string
	Version    uint16

	// FID and DID are the file's ID and its directory's.
	FID, DID ondisk.Fid

	// HighVer and LowVer are FNB's HIGHVER and LOWVER: higher and lower
	// versions of the file exist ($CREATE only).
	HighVer, LowVer bool
}

// resultant is f's resultant string's fields.
func (f foundFile) resultant() fileName {
	return fileName{
		Dev:  f.Parsed.Dev,
		Dir:  dirSpec{Elems: f.Dirs}.String(),
		Name: f.Name,
		Type: "." + f.Type,
		Ver:  ";" + strconv.Itoa(int(f.Version)),
	}
}

// namOutputs says what a NAM gets besides FID, DID, and DVI: the
// expanded and resultant strings, and FNB. By FAB$V_NAM, $OPEN by file ID
// writes none of them, and by directory ID only the resultant string (the
// oracle's NAMFID probe).
type namOutputs struct {
	Expanded, Resultant, FNB bool
}

// allOutputs is what an ordinary $OPEN or $CREATE writes.
var allOutputs = namOutputs{Expanded: true, Resultant: true, FNB: true}

// fillNAM reports f in the NAM: what out asks for, the component
// pointers (into the last string written), FID, DID, and DVI. When the
// resultant string doesn't fit it returns RMS$_RSS, having written FNB
// and the strings as VMS does but no FID, DID, or DVI (the oracle's OPEN
// case 10).
func (ctx *Context) fillNAM(fab, nam uint32, f foundFile, out namOutputs) (uint32, error) {
	_ = fab 
	
	if out.Expanded {
		if err := ctx.storeExpanded(nam, f.Parsed); err != nil {
			return 0, err
		}
	}

	if out.FNB {
		fnb := f.Parsed.FNB

		if f.HighVer {
			fnb |= fnbHighVer
		}

		if f.LowVer {
			fnb |= fnbLowVer
		}

		if err := ctx.storeLongword(nam+namFNB, fnb); err != nil {
			return 0, err
		}
	}

	if out.Resultant {
		rss, err := ctx.loadByte(nam + namRSS)
		if err != nil {
			return 0, err
		}

		rsa, err := ctx.loadLongword(nam + namRSA)
		if err != nil {
			return 0, err
		}

		overflow, err := ctx.storeNameString(nam, rsa, rss, namRSL, f.resultant())
		if err != nil {
			return 0, err
		}

		if overflow {
			return rmsRSSError, nil
		}
	}

	if err := ctx.storeFid(nam+namFID, f.FID); err != nil {
		return 0, err
	}

	if err := ctx.storeFid(nam+namDID, f.DID); err != nil {
		return 0, err
	}

	return 0, ctx.storeDVI(nam, ctx.deviceID(f.Device))
}

// stvFor is the STV value VMS reports with a failure to find or make a
// file: the file system's SS$_ reason.
func stvFor(sts uint32) uint32 {
	switch sts {
	case rmsFileNotFound, rmsDirNotFound:
		return ssNoSuchFile
	case rmsDeviceError, rmsDeviceNotReady:
		return ssNoSuchDevice
	case rmsFileExists:
		return ssDuplicateFileName
	}

	return 0
}

// lookupVersion finds name's entry in dir for a version as written after
// ";": none or 0 for the highest, n for that version, -n for the one n
// below the highest. It returns RMS$_VER for a version that isn't one,
// and RMS$_FNF when there's no such entry.
func lookupVersion(dir *volume.Directory, name, ver string) (ondisk.DirEntry, uint32) {
	n := 0

	if ver != "" {
		var err error
		if n, err = strconv.Atoi(ver); err != nil || n > 32767 || n < -32767 {
			return ondisk.DirEntry{}, rmsInvalidVersion
		}
	}

	entries, err := dir.List()
	if err != nil {
		return ondisk.DirEntry{}, rmsFileNotFound
	}

	// A name's entries are in descending version order.
	var versions []ondisk.DirEntry

	for _, e := range entries {
		if strings.EqualFold(e.Name, name) {
			versions = append(versions, e)
		}
	}

	switch {
	case n > 0:
		for _, e := range versions {
			if int(e.Version) == n {
				return e, 0
			}
		}

	case -n < len(versions):
		return versions[-n], 0
	}

	return ondisk.DirEntry{}, rmsFileNotFound
}

// versionsAround reports whether name has versions in dir above and below
// version.
func versionsAround(dir *volume.Directory, name string, version uint16) (higher, lower bool) {
	entries, err := dir.List()
	if err != nil {
		return false, false
	}

	for _, e := range entries {
		if !strings.EqualFold(e.Name, name) {
			continue
		}

		higher = higher || e.Version > version
		lower = lower || e.Version < version
	}

	return higher, lower
}

// splitEntryName splits a directory entry's name into its name and type.
func splitEntryName(s string) (string, string) {
	name, typ, _ := strings.Cut(s, ".")

	return name, typ
}

// reportOpened is what $OPEN and $CREATE do once the file is open: the
// NAM (when there is one) gets the file, and the FAB and the XAB chain
// get its attributes. The handle remembers the file for $DISPLAY. On
// RMS$_RSS the file is closed again and the status returned.
func (ctx *Context) reportOpened(fab, nam uint32, ifi uint16, found foundFile, chain []xabEntry, out namOutputs, mode xabMode) (uint32, error) {
	h, _ := ctx.Files.Lookup(ifi)
	h.Found = &found

	if nam != 0 {
		sts, err := ctx.fillNAM(fab, nam, found, out)
		if err != nil {
			return 0, err
		}

		if sts != 0 {
			ctx.Files.Release(ifi)

			return sts, nil
		}
	}

	for _, s := range []struct{ off, v uint32 }{{fabDEV, diskDevChar}, {fabSDC, diskDevChar}} {
		if err := ctx.storeLongword(fab+s.off, s.v); err != nil {
			return 0, err
		}
	}

	if err := ctx.fillFABAttributes(fab, h.File); err != nil {
		return 0, err
	}

	return 0, ctx.fillXABs(chain, h.File, mode)
}
