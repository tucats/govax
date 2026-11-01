package rms

import (
	"encoding/binary"
	"fmt"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// Relative files (Phase 81, subtask 9; docs/PHASE-81 - FORTRAN files.md).
//
// A relative file is a row of numbered cells, one record a cell, the first
// cell record 1. A program reaches a record by its number (RAB$B_RAC
// RAB$C_KEY, the number in the longword RAB$L_KBF points to), or in
// sequence, which passes over the cells that hold no record. The Guide to
// OpenVMS File Applications (chapter 2) describes the organization; how VMS
// lays it out on the disk comes from files VMS 7.1 made (testdata/for81's
// vax/for81-blocks.txt and for81c-blocks.txt), not from any manual:
//
//   - Virtual block 1 is the prologue. Its longword at 68 is the first data
//     block (2), at 6C the maximum record number (MRN; 7FFFFFFF when the FAB
//     asked for 0, no limit), at 70 the end of file block, and its word at 74
//     the prologue's version (1). The word at 1FE is the sum of the block's
//     other 255 words. The rest is zeros.
//   - The data are buckets of FAB$B_BKS blocks from block 2. A cell never
//     crosses a bucket, though it may cross a block, so a bucket holds as many
//     whole cells as its bytes allow and the rest of it is unused.
//   - A cell is a control byte, then the record: for fixed-length records the
//     record's bytes (no padding to a word, as a sequential FIX file has); for
//     variable-length ones a word's length and room for FAB$W_MRS bytes; for
//     VFC a word's length, which counts the fixed control too, the FAB$B_FSZ
//     bytes of fixed control, and room for MRS bytes. The control byte is 00
//     for a cell never used, 08 for a record, and 0C for a deleted one, whose
//     bytes stay where they were. Writing a record writes its control byte,
//     length, and bytes, and leaves the rest of the cell as it was.
//   - The file's allocation is the prologue and the buckets written so far,
//     rounded up to the volume's cluster, and its end of file always the
//     block after the allocation (first free byte 0), in the header and the
//     prologue alike.
//
// The record operations' rules are VMS 7.1's (FOR81R's and FOR81D's runs),
// beside the guide's Table 8-3; relativeStream keeps a stream's context.

// The cell control byte's bits, as VMS's files show them.
const (
	cellUsed    = 0x08 // the cell holds a record, or held one
	cellDeleted = 0x04 // the record was deleted
)

// The prologue's fields: their byte offsets in virtual block 1.
const (
	prologueDataVBN  = 0x68
	prologueMRN      = 0x6C
	prologueEOF      = 0x70
	prologueVersion  = 0x74
	prologueChecksum = 0x1FE

	firstDataVBN = 2
	noMRN        = 0x7FFFFFFF
)

// maxBucketBlocks is the most blocks a bucket may have (RMS$_BKS past it).
const maxBucketBlocks = 63

var (
	rmsRecordExists = vmsConst("RMS$_REX")
	rmsInvalidMRN   = vmsConst("RMS$_MRN")
	rmsInvalidBKS   = vmsConst("RMS$_BKS")
	rmsOKReadAnyway = vmsConst("RMS$_OK_RRL")

	rabRHB = rabOffset("RHB")
	fabMRN = fabOffset("MRN")
)

// relativeLayout is a relative file's geometry: everything that says where a
// record's cell is, fixed when the file is created.
type relativeLayout struct {
	format ondisk.RecordFormat // the record format alone: FIX, VAR, or VFC
	mrs    int                 // the longest record, FAB$W_MRS
	fsz    int                 // a VFC record's fixed control
	bks    uint32              // blocks a bucket
	cell   int                 // bytes a cell
	cells  uint32              // cells a bucket
	mrn    uint32              // the highest record number
}

// cellSize is the size of a cell for records of format with at most mrs
// bytes, and fsz bytes of fixed control for VFC.
func cellSize(format ondisk.RecordFormat, mrs, fsz int) int {
	switch format {
	case ondisk.RecordFormatVariable:
		return 1 + 2 + mrs
	case ondisk.RecordFormatVFC:
		return 1 + 2 + fsz + mrs
	}

	return 1 + mrs
}

// relativeBucket returns the bucket size a new relative file gets: bks, or
// with bks 0 the fewest blocks that hold a cell of cell bytes. It returns
// RMS$_BKS for a bucket past 63 blocks, or too small for a cell (VMS 7.1:
// FOR81D's E9, E12, and E13).
func relativeBucket(bks byte, cell int) (byte, uint32) {
	need := (cell + ondisk.BlockSize - 1) / ondisk.BlockSize

	switch {
	case bks == 0 && need > maxBucketBlocks:
		return 0, rmsInvalidBKS
	case bks == 0:
		return byte(need), 0
	case bks > maxBucketBlocks || int(bks) < need:
		return 0, rmsInvalidBKS
	}

	return bks, 0
}

// checkRelativeCreate is $CREATE's checks of a relative file's FAB, after the
// ones every organization has: the record format must be FIX, VAR, or VFC
// (RMS$_RFM), a VAR or VFC file needs a longest record (RMS$_MRS), and the
// bucket must hold a cell (RMS$_BKS). It returns the bucket size, or the
// status. VMS 7.1 left an allocation of 1 in the FAB when these failed, and
// 0 when a fixed-length size did (FOR81R's #15, #18 to #20), so this writes
// it first.
func (ctx *Context) checkRelativeCreate(fabAddr uint32, format ondisk.RecordFormat, mrs uint16, fsz byte) (byte, uint32, error) {
	alq, err := ctx.loadLongword(fabAddr + fabOffset("ALQ"))
	if err != nil {
		return 0, 0, err
	}

	if err := ctx.storeLongword(fabAddr+fabOffset("ALQ"), max(alq, 1)); err != nil {
		return 0, 0, err
	}

	switch format {
	case ondisk.RecordFormatFixed, ondisk.RecordFormatVariable, ondisk.RecordFormatVFC:
	case ondisk.RecordFormatStreamCRLF, ondisk.RecordFormatStreamLF, ondisk.RecordFormatStreamCR:
		// A stream file's records get carriage return control: VMS 7.1 had
		// set FAB$B_RAT so when it refused a relative one (FOR81R's #19).
		// Whether it does for a sequential stream file with none is
		// unconfirmed, and govax doesn't.
		rat, err := ctx.loadByte(fabAddr + fabRAT)
		if err != nil {
			return 0, 0, err
		}

		if err := ctx.storeByte(fabAddr+fabRAT, rat|ratCR); err != nil {
			return 0, 0, err
		}

		return 0, rmsInvalidRFM, nil
	default:
		return 0, rmsInvalidRFM, nil
	}

	if mrs == 0 {
		return 0, rmsInvalidMRS, nil
	}

	bks, err := ctx.loadByte(fabAddr + fabOffset("BKS"))
	if err != nil {
		return 0, 0, err
	}

	bks, sts := relativeBucket(bks, cellSize(format, int(mrs), int(fsz)))

	return bks, sts, nil
}

// isRelative reports whether f is a relative file.
func isRelative(f *volume.File) bool {
	return byte(f.Header.RecordAttributes.Format)&0xF0 == orgRel
}

// layoutOf reads f's geometry from its header and prologue.
func layoutOf(f *volume.File) (*relativeLayout, error) {
	ra := f.Header.RecordAttributes

	l := &relativeLayout{
		format: ondisk.RecordFormat(byte(ra.Format) & 0x0F),
		mrs:    int(ra.MaxRecordSize),
		fsz:    int(ra.VfcSize),
		bks:    uint32(max(ra.BucketSize, 1)),
	}

	l.cell = cellSize(l.format, l.mrs, l.fsz)
	l.cells = l.bks * ondisk.BlockSize / uint32(l.cell)

	if l.cells == 0 {
		return nil, fmt.Errorf("rms: a relative file's cell of %d bytes doesn't fit its bucket", l.cell)
	}

	block := make([]byte, ondisk.BlockSize)
	if err := f.ReadBlock(1, block); err != nil {
		return nil, err
	}

	l.mrn = binary.LittleEndian.Uint32(block[prologueMRN:])

	return l, nil
}

// prologueMRNOf is the highest record number f's prologue allows.
func prologueMRNOf(f *volume.File) (uint32, error) {
	block := make([]byte, ondisk.BlockSize)
	if err := f.ReadBlock(1, block); err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint32(block[prologueMRN:]), nil
}

// writePrologue writes f's prologue for a highest record number mrn (0 for
// none) and an end of file at block eof.
func writePrologue(f *volume.File, mrn, eof uint32) error {
	if mrn == 0 {
		mrn = noMRN
	}

	block := make([]byte, ondisk.BlockSize)
	binary.LittleEndian.PutUint32(block[prologueDataVBN:], firstDataVBN)
	binary.LittleEndian.PutUint32(block[prologueMRN:], mrn)
	binary.LittleEndian.PutUint32(block[prologueEOF:], eof)
	binary.LittleEndian.PutUint16(block[prologueVersion:], 1)

	var sum uint16
	for i := 0; i < prologueChecksum; i += 2 {
		sum += binary.LittleEndian.Uint16(block[i:])
	}

	binary.LittleEndian.PutUint16(block[prologueChecksum:], sum)

	return f.WriteBlock(1, block)
}

// initRelative lays a new relative file out: every block it has allocated
// zeros (so a block the volume used before reads as no records), the
// prologue for mrn, and the end of file after the allocation.
func initRelative(f *volume.File, mrn uint32) error {
	if err := zeroBlocks(f, 1, f.Blocks()); err != nil {
		return err
	}

	if err := writePrologue(f, mrn, f.Blocks()+1); err != nil {
		return err
	}

	f.SetEndOfFile(f.Blocks()+1, 0)

	return nil
}

// zeroBlocks writes zeros over f's blocks first to last.
func zeroBlocks(f *volume.File, first, last uint32) error {
	zero := make([]byte, ondisk.BlockSize)

	for vbn := first; vbn <= last; vbn++ {
		if err := f.WriteBlock(vbn, zero); err != nil {
			return err
		}
	}

	return nil
}

// cellAt is where record n's cell starts, as a byte offset in the file, and
// the last block of its bucket.
func (l *relativeLayout) cellAt(n uint32) (int64, uint32) {
	bucket := (n - 1) / l.cells
	first := firstDataVBN + bucket*l.bks
	off := int64(first-1)*ondisk.BlockSize + int64((n-1)%l.cells)*int64(l.cell)

	return off, first + l.bks - 1
}

// lastCell is the highest record number whose cell f has room for now.
func (l *relativeLayout) lastCell(f *volume.File) uint32 {
	blocks := f.Blocks()
	if blocks < firstDataVBN {
		return 0
	}

	last := uint64((blocks-1)/l.bks) * uint64(l.cells)

	return uint32(min(last, uint64(l.mrn)))
}

// readCell returns record n's cell, or nil when the file has no room for it
// yet.
func (l *relativeLayout) readCell(f *volume.File, n uint32) ([]byte, error) {
	off, lastVBN := l.cellAt(n)
	if lastVBN > f.Blocks() {
		return nil, nil
	}

	return readAt(f, off, l.cell)
}

// readAt reads n bytes of f from byte offset off.
func readAt(f *volume.File, off int64, n int) ([]byte, error) {
	out := make([]byte, 0, n)
	block := make([]byte, ondisk.BlockSize)

	for len(out) < n {
		vbn := uint32(off/ondisk.BlockSize) + 1

		if err := f.ReadBlock(vbn, block); err != nil {
			return nil, fmt.Errorf("rms: reading virtual block %d: %w", vbn, err)
		}

		at := int(off % ondisk.BlockSize)
		k := min(n-len(out), ondisk.BlockSize-at)
		out = append(out, block[at:at+k]...)
		off += int64(k)
	}

	return out, nil
}

// holdsRecord reports whether cell holds a record: used and not deleted.
func holdsRecord(cell []byte) bool {
	return len(cell) > 0 && cell[0]&cellUsed != 0 && cell[0]&cellDeleted == 0
}

// cellRecord is the record in cell, and a VFC record's fixed control.
func (l *relativeLayout) cellRecord(cell []byte) (record, control []byte) {
	switch l.format {
	case ondisk.RecordFormatVariable:
		n := min(int(binary.LittleEndian.Uint16(cell[1:])), l.mrs)

		return cell[3 : 3+n], nil
	case ondisk.RecordFormatVFC:
		n := min(max(int(binary.LittleEndian.Uint16(cell[1:]))-l.fsz, 0), l.mrs)

		return cell[3+l.fsz : 3+l.fsz+n], cell[3 : 3+l.fsz]
	}

	return cell[1 : 1+l.mrs], nil
}

// sizeOK reports whether a record of n bytes fits the file: exactly MRS for
// fixed-length records, at most MRS for the others.
func (l *relativeLayout) sizeOK(n int) bool {
	if l.format == ondisk.RecordFormatFixed {
		return n == l.mrs
	}

	return n <= l.mrs
}

// cellBytes is what writing record (with a VFC record's fixed control) puts
// at the start of its cell.
func (l *relativeLayout) cellBytes(record, control []byte) []byte {
	out := []byte{cellUsed}

	switch l.format {
	case ondisk.RecordFormatVariable:
		out = binary.LittleEndian.AppendUint16(out, uint16(len(record)))
	case ondisk.RecordFormatVFC:
		out = binary.LittleEndian.AppendUint16(out, uint16(len(record)+l.fsz))

		fixed := make([]byte, l.fsz)
		copy(fixed, control)
		out = append(out, fixed...)
	}

	return append(out, record...)
}

// writeCell writes data at the start of record n's cell, extending the file
// first if its bucket is past the allocation.
func (l *relativeLayout) writeCell(f *volume.File, n uint32, data []byte) error {
	off, lastVBN := l.cellAt(n)

	if lastVBN > f.Blocks() {
		if err := extendRelative(f, lastVBN); err != nil {
			return err
		}
	}

	return writeAt(f, off, data)
}

// extendRelative makes f's allocation reach block last: by what it lacks, or
// by the file's default extension if that's more (the volume's, or 5, when
// the file has none), rounded up to the volume's cluster. The new blocks are
// zeros, and the end of file follows them, in the header and the prologue.
// VMS 7.1's files fit this rule (FOR81R's and FOR81D's allocations), which
// is govax's reading of them, not a rule from a manual.
func extendRelative(f *volume.File, last uint32) error {
	old := f.Blocks()

	more := uint32(f.Header.RecordAttributes.DefaultExtend)
	if more == 0 {
		more = uint32(f.Device.Home.DefaultExtendSize)
	}

	if more == 0 {
		more = 5
	}

	more = max(more, last-old)
	cluster := uint32(max(f.Device.Home.ClusterSize, 1))
	more = (more + cluster - 1) / cluster * cluster

	bm, err := f.Device.Bitmap()
	if err != nil {
		return err
	}

	ib, err := f.Device.IndexBitmap()
	if err != nil {
		return err
	}

	if err := volume.Extend(f, bm, ib, more); err != nil {
		return err
	}

	if err := zeroBlocks(f, old+1, f.Blocks()); err != nil {
		return err
	}

	mrn, err := prologueMRNOf(f)
	if err != nil {
		return err
	}

	if err := writePrologue(f, mrn, f.Blocks()+1); err != nil {
		return err
	}

	f.SetEndOfFile(f.Blocks()+1, 0)

	return nil
}

// relativeStream is a stream's context on a relative file, beside the
// streamContext every stream has (whose current, hasCurrent, afterFind, and
// next hold record numbers here, next 0 meaning record 1):
//
//   - A $GET or $FIND by key makes the record current; a $GET also moves the
//     next record to the one after it, a $FIND leaves it (Table 8-3).
//   - A sequential $GET or $FIND reads the next cell that holds a record, and
//     its number goes in RAB$L_BKT; at the end of the file it is RMS$_EOF,
//     BKT 0, and the next record stays there.
//   - A sequential $PUT writes the next record, RMS$_REX if one is there, and
//     the next record moves past it.
//   - A sequential $GET right after a $FIND reads the record the $FIND found,
//     and so does one right after a $PUT by key read the record the $PUT
//     wrote (FOR81R's #21: after records 1, 3, and 5, a $PUT of record 2 by
//     key, then a sequential $GET read 2, then 3), leaving BKT as it was.
//     That the $PUT acts as a $FIND would is govax's reading of that run.
type relativeStream struct {
	// afterPut is the record a $PUT by key last wrote, while the stream's
	// next operation hasn't happened.
	afterPut uint32
}

// relativeRecords reports whether h is a stream of a relative file's records
// (not a block stream, whose record operations are refused as any file's).
func (h *FileHandle) relativeRecords() bool {
	return h.rel != nil && !h.stream.blocks
}

// relativeNext is the stream's next record number.
func (h *FileHandle) relativeNext() uint32 {
	return uint32(max(h.stream.next, 1))
}

// relativeKey is the record number RAB$L_KBF names: RMS$_KEY for 0 (or a
// RAB$B_KSZ other than 0 or 4, RMS$_KSZ), RMS$_MRN past the file's highest.
func (ctx *Context) relativeKey(rabAddr uint32, l *relativeLayout) (uint32, uint32, error) {
	ksz, err := ctx.loadByte(rabAddr + rabKSZ)
	if err != nil {
		return 0, 0, err
	}

	if ksz != 0 && ksz != 4 {
		return 0, rmsInvalidKeySize, nil
	}

	kbf, err := ctx.loadLongword(rabAddr + rabKBF)
	if err != nil {
		return 0, 0, err
	}

	n, err := ctx.loadLongword(kbf)
	if err != nil {
		return 0, 0, err
	}

	switch {
	case n == 0:
		return 0, rmsInvalidKey, nil
	case n > l.mrn:
		return 0, rmsInvalidMRN, nil
	}

	return n, 0, nil
}

// relativeScan returns the first record from record n on, or 0 at the end of
// the file.
func (h *FileHandle) relativeScan(n uint32) (uint32, []byte, error) {
	l := h.rel
	last := l.lastCell(h.File)

	for ; n >= 1 && n <= last; n++ {
		cell, err := l.readCell(h.File, n)
		if err != nil {
			return 0, nil, err
		}

		if holdsRecord(cell) {
			return n, cell, nil
		}
	}

	return 0, nil, nil
}

// relativeLocate is $GET's and $FIND's work on a relative file (find says
// which): it chooses the record by RAB$B_RAC and the stream's context, locks
// it if the stream locks records, moves the context on, and for a $GET
// returns the record. It returns the status, stored in the RAB, or err for a
// failed access or a wait for a lock.
func (ctx *Context) relativeLocate(rabAddr uint32, h *FileHandle, find bool) (uint32, error) {
	s, l := &h.stream, h.rel

	// A failed $GET leaves a size of 0, and a failed $GET or $FIND no RFA
	// (VMS 7.1: FOR81R's #21).
	fail := func(sts uint32) (uint32, error) {
		s.lost()
		h.relStream.afterPut = 0

		if !find {
			if err := ctx.storeWord(rabAddr+rabRSZ, 0); err != nil {
				return 0, err
			}
		}

		if err := ctx.storeRFA(rabAddr, rfa{}); err != nil {
			return 0, err
		}

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	if s.fac&facReads == 0 {
		return fail(rmsFACNotAllowed)
	}

	rac, err := ctx.loadByte(rabAddr + rabRAC)
	if err != nil {
		return 0, err
	}

	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return 0, err
	}

	var (
		n    uint32
		cell []byte

		// sequential says the record was reached in sequence, and so goes in
		// RAB$L_BKT.
		sequential bool
	)

	switch rac {
	case racSeq:
		held := uint32(0)

		switch {
		case s.afterFind && s.hasCurrent && !find:
			held = uint32(s.current)
		case h.relStream.afterPut != 0 && !find:
			held = h.relStream.afterPut
		}

		if held != 0 {
			if cell, err = l.readCell(h.File, held); err != nil {
				return fail(rmsDeviceError)
			}

			if holdsRecord(cell) {
				n = held
			}
		}

		if n == 0 {
			sequential = true

			if n, cell, err = h.relativeScan(h.relativeNext()); err != nil {
				return fail(rmsDeviceError)
			}

			if n == 0 {
				if err := ctx.storeLongword(rabAddr+rabBKT, 0); err != nil {
					return 0, err
				}

				return fail(rmsEOF)
			}
		}
	case racKey, racRFA:
		if rac == racKey {
			var sts uint32
			if n, sts, err = ctx.relativeKey(rabAddr, l); err != nil {
				return 0, err
			} else if sts != 0 {
				return fail(sts)
			}
		} else {
			// By RFA: the record number and an offset of 0 (unconfirmed:
			// what VMS returns for an RFA that isn't).
			r, err := ctx.loadRFA(rabAddr)
			if err != nil {
				return 0, err
			}

			if r.vbn == 0 || r.offset != 0 || r.vbn > l.mrn {
				return fail(rmsInvalidRFA)
			}

			n = r.vbn
		}

		if cell, err = l.readCell(h.File, n); err != nil {
			return fail(rmsDeviceError)
		}

		if !holdsRecord(cell) {
			return fail(rmsRecordNotFound)
		}
	default:
		return fail(rmsInvalidRAC)
	}

	result := rmsNormal
	r := rfa{vbn: n}

	if ctx.locksRecords(h) {
		lockSts, err := ctx.lockFound(rabAddr, h, r, rop)
		if err != nil || lockSts == 0 {
			return 0, err
		}

		if lockSts&1 == 0 {
			return fail(lockSts)
		}

		result = lockSts

		// The record as it is now that it's locked.
		if result == rmsOKWaited {
			if cell, err = l.readCell(h.File, n); err != nil || !holdsRecord(cell) {
				return fail(rmsRecordNotFound)
			}
		}
	}

	h.readBuffer.touch(ctx, int64(n))

	// The context moves on (Table 8-3).
	s.current, s.hasCurrent, s.afterFind = int64(n), true, find
	h.relStream.afterPut = 0

	if rac == racSeq || !find {
		s.next = int64(n) + 1
	}

	if err := ctx.storeRFA(rabAddr, r); err != nil {
		return 0, err
	}

	if sequential {
		if err := ctx.storeLongword(rabAddr+rabBKT, n); err != nil {
			return 0, err
		}
	}

	if find {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, result)
	}

	record, control := l.cellRecord(cell)

	if control != nil {
		rhb, err := ctx.loadLongword(rabAddr + rabRHB)
		if err != nil {
			return 0, err
		}

		if rhb != 0 {
			for i, b := range control {
				if err := ctx.storeByte(rhb+uint32(i), b); err != nil {
					return 0, err
				}
			}
		}
	}

	return storeRecordStatus(ctx, rabAddr, record, result)
}

// relativePut is $PUT on a relative file: record goes in the cell RAB$B_RAC
// names, the next one in sequence or the one RAB$L_KBF's number names. As
// VMS 7.1's did (FOR81R's #3 to #21, FOR81D): a record of the wrong size is
// RMS$_RSZ, a number past MRN RMS$_MRN, a cell holding a record RMS$_REX
// (with RAB$V_UIF and by key, the record is replaced instead), and a deleted
// cell takes a record as an empty one does. A failed $PUT leaves no RFA.
func (ctx *Context) relativePut(rabAddr uint32, h *FileHandle, record []byte) (uint32, error) {
	s, l := &h.stream, h.rel

	fail := func(sts uint32) (uint32, error) {
		s.lost()
		h.relStream.afterPut = 0

		if err := ctx.storeRFA(rabAddr, rfa{}); err != nil {
			return 0, err
		}

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	if s.fac&facPut == 0 {
		return fail(rmsFACNotAllowed)
	}

	rac, err := ctx.loadByte(rabAddr + rabRAC)
	if err != nil {
		return 0, err
	}

	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return 0, err
	}

	var n uint32

	if rac == racKey {
		var sts uint32
		if n, sts, err = ctx.relativeKey(rabAddr, l); err != nil {
			return 0, err
		} else if sts != 0 {
			return fail(sts)
		}
	} else {
		n = h.relativeNext()
		if n > l.mrn {
			return fail(rmsInvalidMRN)
		}
	}

	if !l.sizeOK(len(record)) {
		return fail(rmsRecordTooBig)
	}

	cell, err := l.readCell(h.File, n)
	if err != nil {
		return fail(rmsDeviceError)
	}

	if holdsRecord(cell) && (rac != racKey || rop&ropUIF == 0) {
		return fail(rmsRecordExists)
	}

	var control []byte

	if l.format == ondisk.RecordFormatVFC {
		rhb, err := ctx.loadLongword(rabAddr + rabRHB)
		if err != nil {
			return 0, err
		}

		if rhb != 0 {
			text, err := ctx.loadFixedString(rhb, l.fsz)
			if err != nil {
				return 0, err
			}

			control = []byte(text)
		}
	}

	off, _ := l.cellAt(n)
	h.wrote(ctx, off)

	if err := l.writeCell(h.File, n, l.cellBytes(record, control)); err != nil {
		return fail(rmsDeviceError)
	}

	s.lost()
	h.relStream.afterPut = 0

	if rac == racKey {
		h.relStream.afterPut = n
	} else {
		s.next = int64(n) + 1

		if err := ctx.storeLongword(rabAddr+rabBKT, n); err != nil {
			return 0, err
		}
	}

	if err := ctx.storeRFA(rabAddr, rfa{vbn: n}); err != nil {
		return 0, err
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// relativeUpdate is $UPDATE on a relative file: the current record is
// replaced by the record at RAB$L_RBF, which may be shorter or longer than it
// was if the records are of variable length. VMS 7.1 checked for a current
// record before the size (FOR81R's #21: RMS$_CUR for a record of the wrong
// size after an $UPDATE), and a failure left no RFA.
func (ctx *Context) relativeUpdate(rabAddr uint32, h *FileHandle) (uint32, error) {
	s, l := &h.stream, h.rel

	fail := func(sts uint32) (uint32, error) {
		s.lost()
		h.relStream.afterPut = 0

		if err := ctx.storeRFA(rabAddr, rfa{}); err != nil {
			return 0, err
		}

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	rsz, err := ctx.loadWord(rabAddr + rabRSZ)
	if err != nil {
		return 0, err
	}

	n := uint32(s.current)

	switch {
	case s.fac&facUpd == 0:
		return fail(rmsFACNotAllowed)
	case !s.hasCurrent:
		return fail(rmsNoCurrent)
	case ctx.locksRecords(h) && h.locks.records[rfa{vbn: n}] == nil:
		return fail(rmsRecordNotLocked)
	case !l.sizeOK(int(rsz)):
		return fail(rmsRecordTooBig)
	}

	rbf, err := ctx.loadLongword(rabAddr + rabRBF)
	if err != nil {
		return 0, err
	}

	record, err := ctx.loadFixedString(rbf, int(rsz))
	if err != nil {
		return 0, err
	}

	cell, err := l.readCell(h.File, n)
	if err != nil {
		return fail(rmsDeviceError)
	}

	// The record went, deleted by another stream (unconfirmed: RMS$_CUR).
	if !holdsRecord(cell) {
		return fail(rmsNoCurrent)
	}

	var control []byte
	if l.format == ondisk.RecordFormatVFC {
		_, control = l.cellRecord(cell)
	}

	off, _ := l.cellAt(n)
	h.wrote(ctx, off)

	if err := l.writeCell(h.File, n, l.cellBytes([]byte(record), control)); err != nil {
		return fail(rmsDeviceError)
	}

	if err := ctx.storeRFA(rabAddr, rfa{vbn: n}); err != nil {
		return 0, err
	}

	s.lost()
	h.relStream.afterPut = 0

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// relativeDelete is $DELETE on a relative file: the current record's cell is
// marked deleted, its bytes left as they were (FOR81D's D1 to D4). It needs
// FAB$V_DEL access and a current record; unlike the other operations, a
// failure leaves RAB$W_RFA alone (FOR81R's #21).
func (ctx *Context) relativeDelete(rabAddr uint32, h *FileHandle) (uint32, error) {
	s, l := &h.stream, h.rel

	fail := func(sts uint32) (uint32, error) {
		s.lost()
		h.relStream.afterPut = 0

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	n := uint32(s.current)

	switch {
	case s.fac&facDel == 0:
		return fail(rmsFACNotAllowed)
	case !s.hasCurrent:
		return fail(rmsNoCurrent)
	case ctx.locksRecords(h) && h.locks.records[rfa{vbn: n}] == nil:
		return fail(rmsRecordNotLocked)
	}

	cell, err := l.readCell(h.File, n)
	if err != nil {
		return fail(rmsDeviceError)
	}

	if !holdsRecord(cell) {
		return fail(rmsNoCurrent)
	}

	off, _ := l.cellAt(n)
	h.wrote(ctx, off)

	if err := writeAt(h.File, off, []byte{cell[0] | cellDeleted}); err != nil {
		return fail(rmsDeviceError)
	}

	if err := ctx.storeRFA(rabAddr, rfa{vbn: n}); err != nil {
		return 0, err
	}

	s.lost()
	h.relStream.afterPut = 0

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}

// relativeEnd is the next record of a stream connected with RAB$V_EOF: the
// one after the file's last record (unconfirmed: VMS's run had no such case).
func (h *FileHandle) relativeEnd() int64 {
	l := h.rel

	for n := l.lastCell(h.File); n >= 1; n-- {
		cell, err := l.readCell(h.File, n)
		if err != nil {
			break
		}

		if holdsRecord(cell) {
			return int64(n) + 1
		}
	}

	return 1
}
