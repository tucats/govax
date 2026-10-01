package rms

import (
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
)

// SysParse implements SYS$PARSE (docs/PHASE-33.md, subtask 2): it runs
// name processing on a FAB's file name (name.go) and reports the expanded
// specification in the FAB's NAM block, without looking for the file.
//
// From the RMS Reference Manual's $PARSE description. The inputs are the
// FAB's FNA/FNS, DNA/DNS, FOP's OFP bit, and NAM (which must be given),
// and the NAM's ESA/ESS, NOP, and RLF. The outputs:
//
//   - the expanded string in ESA, when ESA and ESS are both nonzero, its
//     length in ESL (RMS$_ESS if it doesn't fit);
//   - the component pointers and lengths, into the expanded string;
//   - FNB's status bits;
//   - DID and DVI, and the FAB's DEV and SDC, unless NOP's SYNCHK is
//     set, after checking the device and directory exist;
//   - RSL and FID cleared.
//
// The wildcard context (WCC) names the search state $SEARCH continues
// from (search.go).
func SysParse(ctx *Context, argv []uint32) (uint32, error) {
	if len(argv) < 1 {
		return ssInsufficientArgs, nil
	}

	fab := argv[0]

	if sts, err := checkFAB(ctx, fab); err != nil || sts != 0 {
		return sts, err
	}

	ifi, err := ctx.loadWord(fab + fabIFI)
	if err != nil {
		return 0, err
	}

	if ifi != 0 {
		return fabStatus(ctx, fab, rmsInvalidIFI, 0)
	}

	nam, sts, err := fabNAMBlock(ctx, fab)
	if err != nil {
		return 0, err
	}

	if sts == 0 && nam == 0 {
		sts = rmsInvalidNAM
	}

	if sts != 0 {
		return fabStatus(ctx, fab, sts, 0)
	}

	sts, stv, err := ctx.parseInto(fab, nam)
	if err != nil {
		return 0, err
	}

	return fabStatus(ctx, fab, sts, stv)
}

// parseInto is $PARSE's work, for a FAB and its NAM, both checked. It
// returns the status and STV value to report. $SEARCH's context for the
// parse is kept under the NAM's WCC.
func (ctx *Context) parseInto(fab, nam uint32) (sts, stv uint32, err error) {
	in, err := fabNames(ctx, fab, nam)
	if err != nil {
		return 0, 0, err
	}

	nop, err := ctx.loadByte(nam + namNOP)
	if err != nil {
		return 0, 0, err
	}

	// RSL, FID, DID, DVI, and WCC start clear.
	for _, f := range []struct {
		off  uint32
		size uint32
	}{{namRSL, 1}, {namFID, 6}, {namDID, 6}, {namDVI, namDVISize}, {namWCC, 4}} {
		for i := range f.size {
			if err := ctx.storeByte(nam+f.off+i, 0); err != nil {
				return 0, 0, err
			}
		}
	}

	names, sts := ctx.expandName(in)
	if sts != 0 {
		return sts, 0, nil
	}

	p := names[0]

	if err := ctx.storeLongword(nam+namFNB, p.FNB); err != nil {
		return 0, 0, err
	}

	ess, err := ctx.loadByte(nam + namESS)
	if err != nil {
		return 0, 0, err
	}

	esa, err := ctx.loadLongword(nam + namESA)
	if err != nil {
		return 0, 0, err
	}

	text := p.String()

	overflow, err := ctx.storeString(esa, ess, text)
	if err != nil {
		return 0, 0, err
	}

	if overflow {
		return rmsESSError, 0, nil
	}

	if esa != 0 && ess != 0 {
		if err := ctx.storeByte(nam+namESL, byte(len(text))); err != nil {
			return 0, 0, err
		}

		if err := ctx.storeComponents(nam, esa, p.fileName); err != nil {
			return 0, 0, err
		}
	}

	if nop&nopSynChk != 0 {
		return rmsNormal, 0, nil
	}

	did, sts, stv := ctx.checkParsed(p)
	if sts != 0 {
		return sts, stv, nil
	}

	if err := ctx.storeDVI(nam, deviceID(p.Lookup)); err != nil {
		return 0, 0, err
	}

	if err := ctx.storeFid(nam+namDID, did); err != nil {
		return 0, 0, err
	}

	if err := ctx.storeLongword(fab+fabDEV, diskDevChar); err != nil {
		return 0, 0, err
	}

	if err := ctx.storeLongword(fab+fabSDC, diskDevChar); err != nil {
		return 0, 0, err
	}

	if err := ctx.saveSearch(nam, names); err != nil {
		return 0, 0, err
	}

	return rmsNormal, 0, nil
}

// checkParsed confirms a parsed name's device is mounted and, when the
// directory has no wildcard, that the directory exists, returning its
// file ID. A wildcard directory's DID is zero. On failure it returns the
// status and STV to report.
func (ctx *Context) checkParsed(p parsedName) (did ondisk.Fid, sts, stv uint32) {
	vol, ok := ctx.Mounts.Lookup(p.Lookup)
	if !ok {
		return did, rmsDeviceError, ssNoSuchDevice
	}

	if p.FNB&fnbWildDir != 0 {
		return did, 0, 0
	}

	dir, err := filespec.ResolveDirectory(vol, p.DirSpec.names())
	if err != nil {
		return did, rmsDirNotFound, ssNoSuchFile
	}

	return dir.Header.Fid, 0, 0
}

// fabStatus stores sts in FAB$L_STS and stv in FAB$L_STV, and returns
// sts.
func fabStatus(ctx *Context, fab, sts, stv uint32) (uint32, error) {
	if err := ctx.storeLongword(fab+fabSTS, sts); err != nil {
		return 0, err
	}

	if err := ctx.storeLongword(fab+fabSTV, stv); err != nil {
		return 0, err
	}

	return sts, nil
}
