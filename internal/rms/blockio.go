package rms

import "github.com/tucats/ods2/ondisk"

// Block I/O (Phase 81): $READ and $WRITE transfer a file's virtual blocks
// with no records. A stream does it when the FAB asked for FAB$V_BIO, or for
// FAB$V_BRO with RAB$V_BIO in the $CONNECT. RAB$L_BKT names the first block,
// 0 meaning the block after the last transfer; the transfer is RAB$W_USZ bytes
// into RAB$L_UBF ($READ) or RAB$W_RSZ bytes from RAB$L_RBF ($WRITE). The RMS
// manual gives the rules.
//
// VMS 7.1 refused a block I/O $OPEN that lets others write, without FAB$V_UPI,
// with RMS$_UPI (testdata/for81, FOR81R's first run). Unconfirmed: which status
// a record operation on a block stream gets, and a block operation
// on a record stream (RMS$_IOP here), and that a read stops at the end of the block
// holding the end of file.

var (
	facBIO = byte(vmsConst("FAB$M_BIO"))
	facBRO = byte(vmsConst("FAB$M_BRO"))
	ropBIO = vmsConst("RAB$M_BIO")
	ropUIF = vmsConst("RAB$M_UIF")

	rabBKT = rabOffset("BKT")

	rmsNeedsUPI     = vmsConst("RMS$_UPI")
	rmsBadUserSize  = vmsConst("RMS$_USZ")
	rmsBadUserBuf   = vmsConst("RMS$_UBF")
	rmsBadRecordBuf = vmsConst("RMS$_RBF")
)

// blockSharing is the status for block I/O access fac with sharing shr: others
// can't write while this file is read by block unless FAB$V_UPI says the
// program looks after that.
func blockSharing(fac, shr byte) uint32 {
	if fac&(facBIO|facBRO) != 0 && shr&(shrPut|shrUpd|shrDel) != 0 && shr&shrUPI == 0 {
		return rmsNeedsUPI
	}

	return 0
}

// blockStream reports whether a stream with access fac, connected with options
// rop, transfers blocks.
func blockStream(fac byte, rop uint32) bool {
	return fac&facBIO != 0 || fac&facBRO != 0 && rop&ropBIO != 0
}

// blockHandle is what $READ and $WRITE share: the RAB's stream, if it can
// transfer blocks, and the first block to use. A nonzero status has been stored.
func (ctx *Context) blockHandle(rabAddr uint32) (*FileHandle, uint32, uint32, error) {
	h, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return nil, 0, sts, err
	}

	fail := func(status uint32) (*FileHandle, uint32, uint32, error) {
		sts, err := storeStatus(ctx, rabAddr, rabSTS, rabSTV, status)

		return nil, 0, sts, err
	}

	if h.IsConsole() || h.IsRecordDevice() {
		return fail(rmsIOP)
	}

	if !h.stream.blocks {
		return fail(rmsIOP)
	}

	vbn, err := ctx.loadLongword(rabAddr + rabBKT)
	if err != nil {
		return nil, 0, 0, err
	}

	if vbn == 0 {
		vbn = max(h.stream.nextBlock, 1)
	}

	return h, vbn, 0, nil
}

// lastDataBlock is the last block of h's file that holds data.
func (h *FileHandle) lastDataBlock() uint32 {
	ra := h.File.Header.RecordAttributes
	if ra.FirstFreeByte == 0 && ra.EndOfFileBlock > 0 {
		return ra.EndOfFileBlock - 1
	}

	return ra.EndOfFileBlock
}

// SysRead implements SYS$READ (RAB at argv[0]): whole blocks from the file into
// the user's buffer, RAB$W_RSZ saying how many bytes came and RAB$L_RBF where.
// The read stops at the end of the file, and starting past it is RMS$_EOF.
func SysRead(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	h, vbn, sts, err := ctx.blockHandle(rabAddr)
	if err != nil || h == nil {
		return sts, err
	}

	if h.stream.fac&(facGet|facBIO|facBRO) == 0 {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsFACNotAllowed)
	}

	usz, err := ctx.loadWord(rabAddr + rabUSZ)
	if err != nil {
		return 0, err
	}

	ubf, err := ctx.loadLongword(rabAddr + rabUBF)
	if err != nil {
		return 0, err
	}

	switch {
	case usz == 0:
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsBadUserSize)
	case ubf == 0:
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsBadUserBuf)
	}

	if err := h.flushWriter(); err != nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
	}

	last := h.lastDataBlock()
	if vbn > last {
		if err := ctx.storeWord(rabAddr+rabRSZ, 0); err != nil {
			return 0, err
		}

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsEOF)
	}

	want := min(int64(usz), int64(last-vbn+1)*ondisk.BlockSize)
	data := make([]byte, 0, want)
	block := make([]byte, ondisk.BlockSize)

	for b := vbn; int64(len(data)) < want; b++ {
		if err := h.File.ReadBlock(b, block); err != nil {
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
		}

		data = append(data, block[:min(want-int64(len(data)), ondisk.BlockSize)]...)
	}

	for i, b := range data {
		if err := ctx.storeByte(ubf+uint32(i), b); err != nil {
			return 0, err
		}
	}

	h.stream.nextBlock = vbn + uint32((want+ondisk.BlockSize-1)/ondisk.BlockSize)
	ctx.countIO(0, 1)

	for _, store := range []func() error{
		func() error { return ctx.storeWord(rabAddr+rabRSZ, uint16(want)) },
		func() error { return ctx.storeLongword(rabAddr+rabRBF, ubf) },
		func() error { return ctx.storeRFA(rabAddr, rfa{vbn: vbn}) },
	} {
		if err := store(); err != nil {
			return 0, err
		}
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// SysWrite implements SYS$WRITE (RAB at argv[0]): RAB$W_RSZ bytes from
// RAB$L_RBF to the file from the block named, the last block's rest zeros. The
// file grows as it needs to, and the end of file follows a write past it.
func SysWrite(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	h, vbn, sts, err := ctx.blockHandle(rabAddr)
	if err != nil || h == nil {
		return sts, err
	}

	if h.stream.fac&facWrite == 0 {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsFACNotAllowed)
	}

	rsz, err := ctx.loadWord(rabAddr + rabRSZ)
	if err != nil {
		return 0, err
	}

	rbf, err := ctx.loadLongword(rabAddr + rabRBF)
	if err != nil {
		return 0, err
	}

	if rbf == 0 {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsBadRecordBuf)
	}

	text, err := ctx.loadFixedString(rbf, int(rsz))
	if err != nil {
		return 0, err
	}

	if err := h.flushWriter(); err != nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
	}

	blocks := (uint32(rsz) + ondisk.BlockSize - 1) / ondisk.BlockSize
	data := make([]byte, int(blocks)*ondisk.BlockSize)
	copy(data, text)

	off := int64(vbn-1) * ondisk.BlockSize
	if err := writeAt(h.File, off, data); err != nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
	}

	if end := off + int64(len(data)); end > h.endOfFile() {
		h.File.SetEndOfFile(uint32(end/ondisk.BlockSize)+1, 0)
	}

	h.written = true
	h.stream.nextBlock = vbn + blocks
	ctx.countIO(0, 1)

	if err := ctx.storeRFA(rabAddr, rfa{vbn: vbn}); err != nil {
		return 0, err
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// flushWriter puts what h's Writer holds on the disk, for an operation that goes
// to the blocks itself, and has it start again from the end of file each time
// from here on.
func (h *FileHandle) flushWriter() error {
	if h.Writer == nil {
		return nil
	}

	if err := h.Writer.Flush(); err != nil {
		return err
	}

	h.Writer.SetShared(true)
	h.stream.fresh = true

	return nil
}

// SysDisconnect implements SYS$DISCONNECT (RAB at argv[0]): the stream ends and
// the file stays open, for another $CONNECT. What it holds goes to the disk, and
// its record locks are given up.
func SysDisconnect(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	isi, err := ctx.loadWord(rabAddr + rabISI)
	if err != nil {
		return 0, err
	}

	if isIndirect(isi) {
		if err := ctx.storeWord(rabAddr+rabISI, 0); err != nil {
			return 0, err
		}

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
	}

	h, sts, err := ctx.streamOf(rabAddr)
	if err != nil || sts != 0 {
		return sts, err
	}

	if !h.IsConsole() && !h.IsRecordDevice() {
		h.locks.release()

		if err := h.flushWriter(); err != nil {
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
		}
	}

	if err := ctx.storeWord(rabAddr+rabISI, 0); err != nil {
		return 0, err
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}
