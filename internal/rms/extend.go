package rms

// SysExtend implements SYS$EXTEND (FAB at argv[0]), from the RMS Reference
// Manual's description of the service (Phase 88, subtask 7): more blocks for
// an open file. The file is the one FAB$W_IFI names (RMS$_IFI if none), and
// the FAB's access must include PUT or UPD (RMS$_FAC).
//
//   - With a XABALL in the chain, each one's ALQ is what's added. An indexed
//     file's XABALL names its area by AID (RMS$_AID past the file's areas),
//     and the blocks go to that area (isam's ExtendArea). FAB$L_STV is the
//     blocks added in all.
//   - With none, FAB$L_ALQ is what's added, and becomes what was: the
//     volume rounds a request up to its cluster. An indexed file opened for
//     records has its area 0 extended; opened for block I/O, the file alone.
//   - An ALQ of 0 is RMS$_ALQ (the message's own "0 on $EXTEND"), and a
//     volume without the room RMS$_FUL.
//
// The new blocks read as zeros. A relative file's end of file follows its
// allocation, as relative.go keeps it. An indexed file's becomes the block
// past its allocation, which VMS's are once RMS has extended them (S1's 0A
// over 9 blocks; it was 0 before); that $EXTEND sets it so is unconfirmed.
// That STV is also the blocks added when there's no XABALL is unconfirmed;
// so is FAB$L_ALQ's being left as it was with one. Contiguity (AOP's CTG and
// CBT, FOP's) and placement are accepted and not honored, as Decision 5 has
// it for areas.
func SysExtend(ctx *Context, argv []uint32) (uint32, error) {
	if len(argv) < 1 {
		return ssInsufficientArgs, nil
	}

	fab := argv[0]

	ifi, err := ctx.loadWord(fab + fabIFI)
	if err != nil {
		return 0, err
	}

	if _, ok := ctx.Files.indirectFile(ifi); ok {
		return fabStatus(ctx, fab, rmsIOP, 0)
	}

	h, ok := ctx.Files.Lookup(ifi)
	if !ok || h.IsConsole() || h.IsRecordDevice() {
		return fabStatus(ctx, fab, rmsInvalidIFI, 0)
	}

	if h.Access&(facPut|facUpd) == 0 || !h.Writable {
		return fabStatus(ctx, fab, rmsFACNotAllowed, 0)
	}

	chain, sts, stv, err := ctx.xabChain(fab)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fab, sts, stv)
	}

	var alls []xabEntry

	for _, x := range chain {
		if x.Cod == xabALL {
			alls = append(alls, x)
		}
	}

	indexed := isIndexed(h.File)
	byRecord := indexed && h.Access&facBIO == 0

	// What's asked for: each XABALL's ALQ and area, or the FAB's ALQ for
	// area 0 (an indexed file's) or the file.
	var requests []extendRequest

	for _, x := range alls {
		alq, err := ctx.loadLongword(x.Addr + xabALQ)
		if err != nil {
			return 0, err
		}

		aid, err := ctx.loadByte(x.Addr + xabAllAID)
		if err != nil {
			return 0, err
		}

		requests = append(requests, extendRequest{area: int(aid), count: alq})
	}

	if len(requests) == 0 {
		alq, err := ctx.loadLongword(fab + fabOffset("ALQ"))
		if err != nil {
			return 0, err
		}

		requests = []extendRequest{{area: 0, count: alq}}
	}

	for _, r := range requests {
		if r.count == 0 {
			return fabStatus(ctx, fab, rmsInvalidALQ, 0)
		}
	}

	added, sts, stv, err := ctx.extendFile(h, byRecord, requests)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fab, sts, stv)
	}

	if len(alls) == 0 {
		if err := ctx.storeLongword(fab+fabOffset("ALQ"), added); err != nil {
			return 0, err
		}
	}

	return fabStatus(ctx, fab, rmsNormal, added)
}

// extendRequest is one extension $EXTEND was asked for: count blocks, for
// area of an indexed file.
type extendRequest struct {
	area  int
	count uint32
}

// rmsInvalidALQ and rmsDeviceFull are $EXTEND's own statuses.
var (
	rmsInvalidALQ = vmsConst("RMS$_ALQ")
	rmsDeviceFull = vmsConst("RMS$_FUL")
)

// extendFile adds each request's blocks to h's file: to an indexed file's
// area, when byRecord, or to the file. It returns the blocks added, or a
// status and its STV.
func (ctx *Context) extendFile(h *FileHandle, byRecord bool, requests []extendRequest) (uint32, uint32, uint32, error) {
	f := h.File
	old := f.Blocks()

	var added uint32

	if byRecord {
		file, err := openIndexed(f, true)
		if err != nil {
			return 0, prologStatus(err), 0, nil
		}

		for _, r := range requests {
			if r.area >= len(file.Areas) {
				return 0, rmsAreaID, uint32(r.area), nil
			}

			got, err := file.ExtendArea(r.area, r.count)
			if err != nil {
				return 0, rmsDeviceFull, 0, nil
			}

			added += got
		}
	} else {
		for _, r := range requests {
			_, got, err := indexedBlocks{f}.Extend(r.count)
			if err != nil {
				return 0, rmsDeviceFull, 0, nil
			}

			added += got
		}
	}

	switch {
	case isRelative(f):
		if err := zeroBlocks(f, old+1, f.Blocks()); err != nil {
			return 0, rmsDeviceError, 0, nil
		}

		mrn, err := prologueMRNOf(f)
		if err != nil {
			return 0, rmsDeviceError, 0, nil
		}

		if err := writePrologue(f, mrn, f.Blocks()+1); err != nil {
			return 0, rmsDeviceError, 0, nil
		}

		f.SetEndOfFile(f.Blocks()+1, 0)
	case isIndexed(f):
		f.SetEndOfFile(f.Blocks()+1, 0)
	}

	return added, 0, 0, nil
}
