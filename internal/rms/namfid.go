package rms

import (
	"strings"

	"github.com/tucats/ods2/ondisk"
)

// openByNAM is $OPEN with FAB$V_NAM set (docs/PHASE-33.md, subtask 4).
// From the RMS Reference Manual (FAB$V_NAM, and the $OPEN NAM tables),
// and VMS 7.3 (the oracle's NAMFID probe): the file is the one on the
// device NAM$T_DVI names whose ID is NAM$W_FID; or, when FID is zero, the
// one the FAB's file name names in the directory whose ID is NAM$W_DID.
// With neither, or no DVI, done is false and $OPEN goes by the file name
// as usual.
//
// Opened by FID, the NAM keeps its FID and DVI and gets nothing else: its
// DID is cleared. Opened by DID, it gets the resultant string, made of the
// DVI and the expanded name's directory (not the directory's own path),
// and no expanded string or FNB.
func (ctx *Context) openByNAM(fab, nam uint32, fac byte, chain []xabEntry, xabSts, xabSTV uint32) (sts uint32, done bool, err error) {
	fid, err := ctx.loadFid(nam + namFID)
	if err != nil {
		return 0, true, err
	}

	did, err := ctx.loadFid(nam + namDID)
	if err != nil {
		return 0, true, err
	}

	dvi, err := ctx.loadDVI(nam)
	if err != nil {
		return 0, true, err
	}

	if fid.IsZero() && did.IsZero() || dvi == "" {
		return 0, false, nil
	}

	if xabSts != 0 {
		sts, err := fabStatus(ctx, fab, xabSts, xabSTV)

		return sts, true, err
	}

	byFID := !fid.IsZero()

	if err := ctx.clearNAMOutputs(nam); err != nil {
		return 0, true, err
	}

	if err := ctx.storeFid(nam+namFID, fid); err != nil {
		return 0, true, err
	}

	if err := ctx.storeDVI(nam, dvi); err != nil {
		return 0, true, err
	}

	if !byFID {
		if err := ctx.storeFid(nam+namDID, did); err != nil {
			return 0, true, err
		}
	}

	fail := func(sts uint32) (uint32, bool, error) {
		sts, err := fabStatus(ctx, fab, sts, stvFor(sts))

		return sts, true, err
	}

	device := dviDevice(dvi)

	shr, err := ctx.loadByte(fab + fabSHR)
	if err != nil {
		return 0, true, err
	}

	vol, ok := ctx.Mounts.Lookup(device)
	if !ok {
		return fail(rmsDeviceError)
	}

	var (
		ifi   uint16
		found foundFile
		out   namOutputs
	)

	if byFID {
		if ifi, sts, err = openFID(ctx, fac, shr, device, vol, fid); err != nil || sts != 0 {
			if err != nil {
				return 0, true, err
			}

			return fail(sts)
		}

		found = foundFile{Device: device, FID: fid}
	} else {
		names, sts, err := ctx.expandFAB(fab, nam)
		if err != nil {
			return 0, true, err
		}

		if sts != 0 {
			return fail(sts)
		}

		p := names[0]
		if p.FNB&fnbWildcard != 0 {
			return fail(rmsWildcardError)
		}

		dir, err := vol.OpenDirectory(did)
		if err != nil {
			return fail(rmsDirNotFound)
		}

		spec := p.spec()

		entry, sts := lookupVersion(dir, specFileName(spec), spec.Version)
		if sts == 0 {
			if ifi, sts, err = openFID(ctx, fac, shr, device, vol, entry.Fid); err != nil {
				return 0, true, err
			}
		}

		if sts != 0 {
			return fail(sts)
		}

		name, typ := splitEntryName(entry.Name)
		p.Dev = dvi + ":"
		found = foundFile{
			Parsed: p, Device: device, Dirs: p.DirSpec.Elems,
			Name: name, Type: typ, Version: entry.Version,
			FID: entry.Fid, DID: did,
		}
		out.Resultant = true
	}

	if sts, err := ctx.reportOpened(fab, nam, ifi, found, chain, out, xabOpen); err != nil || sts != 0 {
		if err != nil {
			return 0, true, err
		}

		return fail(sts)
	}

	if err := ctx.storeWord(fab+fabIFI, ifi); err != nil {
		return 0, true, err
	}

	sts, err = storeStatus(ctx, fab, fabSTS, fabSTV, rmsNormal)

	return sts, true, err
}

// dviDevice is the device a NAM$T_DVI names: "_SIMVAX$DUA1" is DUA1.
func dviDevice(dvi string) string {
	dev := strings.TrimSuffix(strings.TrimPrefix(dvi, "_"), ":")
	if i := strings.LastIndexByte(dev, '$'); i >= 0 {
		dev = dev[i+1:]
	}

	return dev
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
