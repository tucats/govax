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

// namOutputs says which strings a NAM gets: by FAB$V_NAM, $OPEN writes
// the expanded string only when it had neither a DID nor a FID, and the
// resultant string only when it had no FID.
type namOutputs struct {
	Expanded, Resultant bool
}

// checkESS reports RMS$_ESS when the NAM's expanded string buffer is
// given but too small for p, before a service does anything.
func (ctx *Context) checkESS(nam uint32, p parsedName) (uint32, error) {
	ess, err := ctx.loadByte(nam + namESS)
	if err != nil {
		return 0, err
	}

	esa, err := ctx.loadLongword(nam + namESA)
	if err != nil {
		return 0, err
	}

	if esa != 0 && ess != 0 && len(p.String()) > int(ess) {
		return rmsESSError, nil
	}

	return 0, nil
}

// fillNAM reports f in the NAM: the strings out asks for, the component
// pointers (into the resultant string when RSS is nonzero, the expanded
// string otherwise), FNB, FID, DID, and DVI, and the FAB's DEV and SDC.
// It returns RMS$_RSS when the resultant string doesn't fit.
func (ctx *Context) fillNAM(fab, nam uint32, f foundFile, out namOutputs) (uint32, error) {
	ess, err := ctx.loadByte(nam + namESS)
	if err != nil {
		return 0, err
	}

	esa, err := ctx.loadLongword(nam + namESA)
	if err != nil {
		return 0, err
	}

	rss, err := ctx.loadByte(nam + namRSS)
	if err != nil {
		return 0, err
	}

	rsa, err := ctx.loadLongword(nam + namRSA)
	if err != nil {
		return 0, err
	}

	if out.Expanded && esa != 0 && ess != 0 {
		text := f.Parsed.String()
		if _, err := ctx.storeString(esa, ess, text); err != nil {
			return 0, err
		}

		if err := ctx.storeByte(nam+namESL, byte(len(text))); err != nil {
			return 0, err
		}

		if err := ctx.storeComponents(nam, esa, f.Parsed.fileName); err != nil {
			return 0, err
		}
	}

	if out.Resultant && rsa != 0 && rss != 0 {
		r := f.resultant()
		text := r.Dev + r.Dir + r.Name + r.Type + r.Ver

		overflow, err := ctx.storeString(rsa, rss, text)
		if err != nil {
			return 0, err
		}

		if overflow {
			return rmsRSSError, nil
		}

		if err := ctx.storeByte(nam+namRSL, byte(len(text))); err != nil {
			return 0, err
		}

		if err := ctx.storeComponents(nam, rsa, r); err != nil {
			return 0, err
		}
	}

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

	if err := ctx.storeFid(nam+namFID, f.FID); err != nil {
		return 0, err
	}

	if err := ctx.storeFid(nam+namDID, f.DID); err != nil {
		return 0, err
	}

	if err := ctx.storeDVI(nam, deviceID(f.Device)); err != nil {
		return 0, err
	}

	if err := ctx.storeLongword(fab+fabDEV, diskDevChar); err != nil {
		return 0, err
	}

	return 0, ctx.storeLongword(fab+fabSDC, diskDevChar)
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

// dirPath returns the path of the directory whose file ID is did, by its
// back links, and whether it could be found.
func dirPath(vol *volume.Volume, did ondisk.Fid) ([]string, bool) {
	var path []string

	for fid := did; !fid.Equal(ondisk.MasterFileDirectoryFid); {
		if len(path) > maxDirDepth {
			return nil, false
		}

		f, err := vol.OpenFID(fid)
		if err != nil || !f.Header.IsDirectory() {
			return nil, false
		}

		id, err := f.Header.Ident()
		if err != nil {
			return nil, false
		}

		name, _, _ := strings.Cut(strings.TrimSpace(id.Filename), ".")
		path = append([]string{name}, path...)
		fid = f.Header.Backlink
	}

	return path, true
}

// splitEntryName splits a directory entry's name into its name and type.
func splitEntryName(s string) (string, string) {
	name, typ, _ := strings.Cut(s, ".")

	return name, typ
}
