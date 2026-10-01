package rms

import (
	"strings"

	"github.com/tucats/ods2/ondisk"
)

// openByNAM is $OPEN with FAB$V_NAM set (docs/PHASE-33.md, subtask 4).
// From the RMS Reference Manual (FAB$V_NAM, and the $OPEN NAM tables):
// the file is the one on the device NAM$T_DVI names whose ID is
// NAM$W_FID; or, when FID is zero, the one named by the FAB's file name in
// the directory whose ID is NAM$W_DID. With neither, done is false and
// $OPEN goes by the file name as usual.
//
// Opened by FID, the NAM gets no strings; by DID, it gets the resultant
// string only.
func (ctx *Context) openByNAM(fab, nam uint32, fac byte) (sts uint32, done bool, err error) {
	fid, err := ctx.loadFid(nam + namFID)
	if err != nil {
		return 0, true, err
	}

	did, err := ctx.loadFid(nam + namDID)
	if err != nil {
		return 0, true, err
	}

	if fid.IsZero() && did.IsZero() {
		return 0, false, nil
	}

	dvi, err := ctx.loadDVI(nam)
	if err != nil {
		return 0, true, err
	}

	device := strings.TrimSuffix(strings.TrimPrefix(dvi, "_"), ":")

	vol, ok := ctx.Mounts.Lookup(device)
	if !ok {
		sts, err := fabStatus(ctx, fab, rmsDeviceError, ssNoSuchDevice)

		return sts, true, err
	}

	var (
		ifi   uint16
		found foundFile
		out   namOutputs
	)

	if !fid.IsZero() {
		if ifi, sts, err = openFID(ctx, fac, device, vol, fid); err != nil || sts != 0 {
			if err == nil {
				sts, err = fabStatus(ctx, fab, sts, 0)
			}

			return sts, true, err
		}

		h, _ := ctx.Files.Lookup(ifi)
		found = foundFile{Device: device, FID: fid, DID: h.File.Header.Backlink}
	} else {
		names, sts, err := ctx.expandFAB(fab, nam)
		if err != nil {
			return 0, true, err
		}

		if sts != 0 {
			sts, err = fabStatus(ctx, fab, sts, 0)

			return sts, true, err
		}

		p := names[0]
		if p.FNB&fnbWildcard != 0 {
			sts, err = fabStatus(ctx, fab, rmsWildcardError, 0)

			return sts, true, err
		}

		dir, err := vol.OpenDirectory(did)
		if err != nil {
			sts, err = fabStatus(ctx, fab, rmsDirNotFound, ssNoSuchFile)

			return sts, true, err
		}

		spec := p.spec()

		entry, sts := lookupVersion(dir, specFileName(spec), spec.Version)
		if sts == 0 {
			ifi, sts, err = openFID(ctx, fac, device, vol, entry.Fid)
			if err != nil {
				return 0, true, err
			}
		}

		if sts != 0 {
			sts, err = fabStatus(ctx, fab, sts, 0)

			return sts, true, err
		}

		path, _ := dirPath(vol, did)
		name, typ := splitEntryName(entry.Name)
		p.Dev = dvi
		found = foundFile{
			Parsed: p, Device: device, Dirs: path,
			Name: name, Type: typ, Version: entry.Version,
			FID: entry.Fid, DID: did,
		}
		out.Resultant = true
	}

	if sts, err := ctx.fillNAM(fab, nam, found, out); err != nil || sts != 0 {
		ctx.Files.Release(ifi)

		if err == nil {
			sts, err = fabStatus(ctx, fab, sts, 0)
		}

		return sts, true, err
	}

	if err := ctx.storeWord(fab+fabIFI, ifi); err != nil {
		return 0, true, err
	}

	sts, err = storeStatus(ctx, fab, fabSTS, fabSTV, rmsNormal)

	return sts, true, err
}

// loadFid reads a file ID stored as storeFid writes it.
func (ctx *Context) loadFid(addr uint32) (ondisk.Fid, error) {
	var (
		f   ondisk.Fid
		err error
	)

	if f.Num, err = ctx.loadWord(addr); err != nil {
		return f, err
	}

	if f.Seq, err = ctx.loadWord(addr + 2); err != nil {
		return f, err
	}

	if f.Rvn, err = ctx.loadByte(addr + 4); err != nil {
		return f, err
	}

	f.Nmx, err = ctx.loadByte(addr + 5)

	return f, err
}

// loadDVI reads NAM$T_DVI's text.
func (ctx *Context) loadDVI(nam uint32) (string, error) {
	n, err := ctx.loadByte(nam + namDVI)
	if err != nil || n == 0 {
		return "", err
	}

	return ctx.loadFixedString(nam+namDVI+1, int(min(uint32(n), namDVISize-1)))
}
