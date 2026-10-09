package rms

import (
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// SysErase implements SYS$ERASE (FAB at argv[0]): it deletes the file
// the FAB names (the highest version if none is given), from the RMS
// manual's description. The FAB mustn't be open (FAB$W_IFI 0: RMS$_IFI
// otherwise, unconfirmed). A file another process has open is deleted
// when its last opener closes it (ods2 marks it for delete), but one
// open for writing is refused, RMS$_FLK (the manual's FAB$V_ERL, which
// would allow it, isn't in VMS 7.3's FAB definitions). A wildcard is
// RMS$_WLD. Only a disk file can be erased; a name the console or a
// record device stands for is RMS$_FNF (unconfirmed). The NAM's
// expanded string is written, as $OPEN writes it.
func SysErase(ctx *Context, argv []uint32) (uint32, error) {
	fabAddr := argv[0]

	ifi, err := ctx.loadWord(fabAddr + fabIFI)
	if err != nil {
		return 0, err
	}

	if ifi != 0 {
		return fabStatus(ctx, fabAddr, rmsInvalidIFI, 0)
	}

	nam, sts, err := fabNAMBlock(ctx, fabAddr)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fabAddr, sts, 0)
	}

	names, sts, err := ctx.expandFAB(fabAddr, nam)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fabAddr, sts, 0)
	}

	p := names[0]

	if nam != 0 {
		if sts, err := ctx.expandedOrESS(nam, p); err != nil || sts != 0 {
			if err != nil {
				return 0, err
			}

			return fabStatus(ctx, fabAddr, sts, 0)
		}
	}

	if p.FNB&fnbWildcard != 0 {
		return fabStatus(ctx, fabAddr, rmsWildcardError, 0)
	}

	sts, err = eraseOnVolume(ctx, p)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fabAddr, sts, stvFor(sts))
	}

	ctx.countIO(1, 0)

	return fabStatus(ctx, fabAddr, rmsNormal, 0)
}

// eraseOnVolume deletes the file p names, returning a failure status, or
// 0.
func eraseOnVolume(ctx *Context, p parsedName) (uint32, error) {
	spec := p.spec()

	vol, ok := ctx.Mounts.Lookup(spec.Device)
	if !ok {
		return rmsFileNotFound, nil
	}

	if !ctx.Mounts.Writable(spec.Device) {
		return rmsPrivilegeViolation, nil
	}

	dir, err := filespec.ResolveDirectory(vol, spec.Dirs)
	if err != nil {
		return rmsDirNotFound, nil
	}

	entry, sts := lookupVersion(dir, specFileName(spec), spec.Version)
	if sts != 0 {
		return sts, nil
	}

	// Someone writing it? A read access denying writers can't join one.
	if vol.Accessed(entry.Fid) {
		probe, err := vol.Access(entry.Fid, volume.AccessMode{NoWrite: true})
		if err != nil {
			return accessStatus(err), nil
		}

		if err := probe.Deaccess(); err != nil {
			return 0, err
		}
	}

	bm, ib, err := deviceBitmaps(dir.Device)
	if err != nil {
		return 0, err
	}

	if err := volume.DeleteFile(dir, entry.Name, entry.Version, bm, ib); err != nil {
		return rmsDeviceError, nil
	}

	return 0, flushBitmaps(bm, ib)
}
