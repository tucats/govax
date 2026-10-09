package rms

import (
	"errors"
	"strconv"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// SysRename implements SYS$RENAME: it gives an existing file a new name,
// version, or directory -- or any mix of the three -- without copying it.
//
// # The call
//
// $RENAME takes four arguments: the old file's FAB (argv[0]), an error
// and a success completion routine (argv[1]/argv[2], which this package
// never calls -- every service here completes synchronously, and like
// SYS$OPEN and the rest it ignores them), and the new name's FAB
// (argv[3]). Each FAB names its file through FAB$L_FNA/FAB$B_FNS, resolved
// exactly as SYS$OPEN and SYS$CREATE resolve theirs (logical names, the
// SYS$DISK device, the process default directory). The completion status
// goes in the *old* FAB's STS/STV; the new FAB's are cleared.
//
// # What real RMS checks, and in what order
//
// This follows RMS's own RMS$RENAME (VMS V7.3, RMS0RENAM.MAR) step by
// step, so a program sees the same status for the same mistake:
//
//  1. Both FABs must be FABs: FAB$B_BID must say so (RMS$_FAB), and
//     FAB$B_BLN must be long enough (RMS$_BLN).
//  2. The old name is parsed. It may not contain a wildcard (RMS$_WLD) --
//     DCL's RENAME command handles wildcards itself, by searching and
//     calling $RENAME once per file -- and must name a disk (RMS$_IOP;
//     the terminal can't be renamed). Its device must be mounted
//     (RMS$_DNR), and its directory must exist (RMS$_DNF).
//  3. The old file is looked up (RMS$_FNF). No version means the highest.
//  4. The new name is parsed, under the same rules as the old one.
//  5. The two names must be on the same device (RMS$_DEV): a rename only
//     moves a directory entry, and an entry can only point at a file on
//     its own volume. "The same device" is decided after logical names
//     are translated, so two different logical names for one disk are
//     fine.
//  6. The new directory must exist (RMS$_DNF), and, when the file being
//     renamed is itself a directory, must not be that directory or lie
//     beneath it (RMS$_IDR).
//  7. The old entry is removed and the new one entered. If the enter
//     fails, the old entry is put back and RMS$_ENT returned, with the
//     file system's reason in STV (SS$_DUPFILENAME when the new name and
//     version already exist -- $RENAME never replaces a file); if even
//     putting it back fails, the file is left without a name and
//     RMS$_REENT returned. A volume mounted read-only fails the removal:
//     RMS$_RMV, STV SS$_WRITLCK.
//
// No version on the new name means the next one: one more than the
// highest version of the new name already in the new directory, or 1.
// (DCL's RENAME command is what makes the new name default to the old
// version; RMS itself doesn't.) A negative new version is treated as no
// version, as the file system does; one above 32767 is RMS$_VER. The
// volume-level rules -- what happens to the header, version limits,
// moving directories -- are ods2's volume.Rename's, which documents them.
//
// # Not implemented
//
// NAM blocks (FAB$L_NAM: the expanded and resultant name strings, and
// related-file defaults) and default file names (FAB$L_DNA/FAB$B_DNS) are
// not supported by any service in this package yet, so $RENAME neither
// reads nor fills them. A search list in the old name is searched for the
// file, as SYS$OPEN does; one in the new name uses its first element, as
// SYS$CREATE does. Network (node::) names aren't supported.
func SysRename(ctx *Context, argv []uint32) (uint32, error) {
	if len(argv) < 4 {
		return ssInsufficientArgs, nil
	}

	oldFAB, newFAB := argv[0], argv[3]

	// Step 1. A bad old FAB has nowhere trustworthy to put a status; a
	// bad new FAB's status goes into the old one, like every other.
	if sts, err := checkFAB(ctx, oldFAB); err != nil || sts != 0 {
		return sts, err
	}

	if sts, err := checkFAB(ctx, newFAB); err != nil || sts != 0 {
		if err != nil {
			return 0, err
		}

		return storeStatus(ctx, oldFAB, fabSTS, fabSTV, sts)
	}

	sts, stv, err := renameFile(ctx, oldFAB, newFAB)
	if err != nil {
		return 0, err
	}

	if err := ctx.storeLongword(oldFAB+fabSTS, sts); err != nil {
		return 0, err
	}

	if err := ctx.storeLongword(oldFAB+fabSTV, stv); err != nil {
		return 0, err
	}

	return sts, nil
}

// renameSide is one of $RENAME's two names, resolved: the volume it's on,
// the device as the caller named it (after translation), and the parsed
// specification.
type renameSide struct {
	vol  *volume.Volume
	spec filespec.Spec
}

// renameFile does SysRename's work once both FABs have checked out,
// returning the RMS status and STV to report: RMS$_NORMAL on success. err
// is a VAX memory access failure.
func renameFile(ctx *Context, oldFAB, newFAB uint32) (sts, stv uint32, err error) {
	// Each name goes through name processing with its FAB's default name
	// and related file (docs/PHASE-33.md).
	oldSpecs, sts, err := ctx.resolveFAB(oldFAB)
	if err != nil || sts != 0 {
		return sts, sts, err
	}

	newSpecs, sts, err := ctx.resolveFAB(newFAB)
	if err != nil || sts != 0 {
		return sts, sts, err
	}

	_, sts, stv = renameSpecs(ctx, oldSpecs, newSpecs)

	return sts, stv, nil
}

// renameText is $RENAME on two file specifications, steps 2 through 7 of
// SysRename's list: the part that doesn't touch a FAB. It returns what
// was renamed (the new version, in particular, when the new name gave
// none) and the RMS status and STV, RMS$_NORMAL on success. The console's RENAME command
// (Session.Rename) calls it too, once per file, as DCL's RENAME calls
// $RENAME through LIB$RENAME_FILE.
func renameText(ctx *Context, oldText, newText string) (r volume.Renamed, sts, stv uint32) {
	oldSpecs, sts := ctx.resolveFileSpec(oldText)
	if sts != 0 {
		return r, sts, sts
	}

	newSpecs, sts := ctx.resolveFileSpec(newText)
	if sts != 0 {
		return r, sts, sts
	}

	return renameSpecs(ctx, oldSpecs, newSpecs)
}

// renameSpecs is renameText once both names are resolved.
func renameSpecs(ctx *Context, oldSpecs, newSpecs []resolvedSpec) (r volume.Renamed, sts, stv uint32) {
	// Steps 2 and 3: the old name, and the old file.
	oldSide, oldDir, oldVersion, sts := findRenameSource(ctx, oldSpecs)
	if sts != 0 {
		return r, sts, sts
	}

	// Step 4: the new name.
	newSide, sts := parseRenameTarget(newSpecs)
	if sts != 0 {
		return r, sts, sts
	}

	newVersion, sts := parseRenameVersion(newSide.spec.Version)
	if sts != 0 {
		return r, sts, sts
	}

	// Step 5: the same device. Two device names mounted to one volume
	// are the same device; anything else -- including a new device that
	// isn't mounted at all -- isn't.
	newSide.vol, _ = ctx.Mounts.Lookup(newSide.spec.Device)
	if newSide.vol != oldSide.vol {
		return r, rmsDeviceError, rmsDeviceError
	}

	// Step 6: the new directory. (The loop check is volume.Rename's.)
	newDir, err := filespec.ResolveDirectory(newSide.vol, newSide.spec.Dirs)
	if err != nil {
		return r, rmsDirNotFound, rmsDirNotFound
	}

	// Step 7.
	if !ctx.Mounts.Writable(oldSide.spec.Device) {
		return r, rmsRemoveFailed, ssWriteLocked
	}

	bm, ib, err := deviceBitmaps(newDir.Device)
	if err != nil {
		return r, rmsSystemError, rmsSystemError
	}

	r, renameErr := oldSide.vol.Rename(oldDir, specFileName(oldSide.spec), oldVersion,
		newDir, specFileName(newSide.spec), newVersion, bm, ib)

	// Whatever happened, what changed is on the disk once the bitmaps
	// are: a failed rename may still have extended a directory.
	flushErr := flushBitmaps(bm, ib)

	if renameErr != nil {
		sts, stv := renameStatus(renameErr)

		return r, sts, stv
	}

	if flushErr != nil {
		return r, rmsSystemError, rmsSystemError
	}

	ctx.countIO(1, 0) // the file system's call (iocount.go)

	return r, rmsNormal, 0
}

// findRenameSource resolves $RENAME's old name (steps 2 and 3 of
// SysRename's list), returning where it is, its directory, and its
// version (0: the highest), or the status to fail with. A search list is
// searched: the first element holding the file wins, and if none does,
// the status for the last element tried is returned (User's Manual
// §11.7), as SYS$OPEN does.
func findRenameSource(ctx *Context, specs []resolvedSpec) (renameSide, *volume.Directory, uint16, uint32) {
	sts := rmsFileNotFound

	for _, r := range specs {
		if sts = checkRenameSpec(r.Spec); sts != 0 {
			return renameSide{}, nil, 0, sts
		}

		version, ok := parseOpenVersion(r.Spec.Version)
		if !ok || version > maxRenameVersion {
			return renameSide{}, nil, 0, rmsInvalidVersion
		}

		vol, ok := ctx.Mounts.Lookup(r.Spec.Device)
		if !ok {
			sts = rmsDeviceNotReady

			continue
		}

		dir, err := filespec.ResolveDirectory(vol, r.Spec.Dirs)
		if err != nil {
			sts = rmsDirNotFound

			continue
		}

		if r.Spec.Name == "" && r.Spec.Type == "" {
			sts = rmsFileNotFound

			continue
		}

		if _, err := dir.Lookup(specFileName(r.Spec), version); err != nil {
			sts = rmsFileNotFound

			continue
		}

		return renameSide{vol: vol, spec: r.Spec}, dir, version, 0
	}

	return renameSide{}, nil, 0, sts
}

// parseRenameTarget resolves $RENAME's new name (step 4), using a search
// list's first element as SYS$CREATE does.
func parseRenameTarget(specs []resolvedSpec) (renameSide, uint32) {
	spec := specs[0].Spec

	if sts := checkRenameSpec(spec); sts != 0 {
		return renameSide{}, sts
	}

	if spec.Name == "" && spec.Type == "" {
		return renameSide{}, rmsFileNameError
	}

	return renameSide{spec: spec}, 0
}

// checkRenameSpec applies RMS's CHECK_PARSE to one of $RENAME's names:
// no wildcards (RMS$_WLD), and not the terminal (RMS$_IOP).
func checkRenameSpec(spec filespec.Spec) uint32 {
	if hasWildcard(spec) {
		return rmsWildcardError
	}

	if normalizeDeviceName(spec.Device) == consoleDeviceName {
		return rmsInvalidOperation
	}

	return 0
}

// hasWildcard reports whether spec contains any wildcard: "*" or "%" in
// any field, or "..." (spec.Recursive) in its directory.
func hasWildcard(spec filespec.Spec) bool {
	if spec.Recursive {
		return true
	}

	fields := append([]string{spec.Name, spec.Type, spec.Version}, spec.Dirs...)
	for _, f := range fields {
		if strings.ContainsAny(f, "*%") {
			return true
		}
	}

	return false
}

// maxRenameVersion is the highest version a directory entry can hold.
const maxRenameVersion = 32767

// parseRenameVersion interprets the new name's version: none (or 0, or a
// negative one, which the file system treats the same way) means the
// next version, 0 to volume.Rename; a positive one is used as given, up
// to 32767 (RMS$_VER beyond that, or for anything that isn't a number).
func parseRenameVersion(v string) (uint16, uint32) {
	if v == "" {
		return 0, 0
	}

	n, err := strconv.Atoi(v)

	switch {
	case err != nil || n > maxRenameVersion:
		return 0, rmsInvalidVersion
	case n <= 0:
		return 0, 0
	default:
		return uint16(n), 0
	}
}

// specFileName is spec's NAME.TYP as a directory entry spells it, the way
// SYS$OPEN and SYS$CREATE form it.
func specFileName(spec filespec.Spec) string {
	if spec.Type == "" {
		return spec.Name
	}

	return spec.Name + "." + spec.Type
}

// renameStatus maps a volume.Rename failure to the RMS status and STV
// RMS's $RENAME reports for it (see SysRename's list).
func renameStatus(err error) (sts, stv uint32) {
	switch {
	case errors.Is(err, volume.ErrRenameLost):
		return rmsReenterFailed, rmsReenterFailed
	case errors.Is(err, volume.ErrDirectoryLoop):
		return rmsInvalidDirRename, rmsInvalidDirRename
	case errors.Is(err, volume.ErrCrossVolume):
		return rmsDeviceError, rmsDeviceError
	case errors.Is(err, volume.ErrNotFound):
		return rmsFileNotFound, rmsFileNotFound
	case errors.Is(err, volume.ErrExists):
		return rmsEnterFailed, ssDuplicateFileName
	case errors.Is(err, volume.ErrBadVersion):
		return rmsEnterFailed, ssBadFileVersion
	default:
		return rmsEnterFailed, rmsSystemError
	}
}

// checkFAB is RMS's NEWFAB1: it confirms fab is a FAB -- FAB$B_BID says
// so (RMS$_FAB) and FAB$B_BLN is long enough (RMS$_BLN) -- and then clears
// its STS and STV. err is a VAX memory access failure.
func checkFAB(ctx *Context, fab uint32) (uint32, error) {
	bid, err := ctx.loadByte(fab + fabBID)
	if err != nil {
		return 0, err
	}

	bln, err := ctx.loadByte(fab + fabBLN)
	if err != nil {
		return 0, err
	}

	switch {
	case bid != fabBIDValue:
		return rmsInvalidFAB, nil
	case bln < fabBLNValue:
		return rmsInvalidBLN, nil
	}

	if err := ctx.storeLongword(fab+fabSTS, 0); err != nil {
		return 0, err
	}

	return 0, ctx.storeLongword(fab+fabSTV, 0)
}
