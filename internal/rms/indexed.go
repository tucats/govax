package rms

import (
	"errors"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// Indexed files (Phase 88; docs/PHASE-88 - keyed index files.md).
//
// An indexed file's structure, its prolog, buckets, and records, is
// internal/isam's, a package that knows nothing of FABs or VAX memory. This
// file is what RMS puts around it: turning a $CREATE's FAB and XABs into an
// isam.Spec, with VMS's statuses for what's wrong with them, and the blocks
// isam reads and writes, which are the volume file's. indexedxab.go reports
// a file's keys and areas in XABs, and extend.go is $EXTEND.
//
// The order of $CREATE's checks is VMS 7.1's, as far as testdata/idx88's
// refusals show it (the phase doc's "What round 1 settled"): what the FAB
// shows after each refusal says how far VMS had got. A key's own faults
// leave the FAB's BKS and ALQ alone; the areas' write ALQ as each XABALL is
// read; the keys' areas are checked once the allocation is known; and the
// fill sizes and the record's size once the bucket size is. KNM, the key
// name, is read only when the prolog is written, after the file exists, so
// a bad KNM leaves a file with an empty prolog behind (VMS's own E85).

// The statuses of an indexed $CREATE, $OPEN, and $DISPLAY. Most have the key
// of reference or the area as their STV, which the message's "for key = !UL"
// shows.
var (
	rmsNoPrimaryKey = vmsConst("RMS$_NPK")
	rmsKeyFlags     = vmsConst("RMS$_FLG")
	rmsKeyPosition  = vmsConst("RMS$_POS")
	rmsKeySize      = vmsConst("RMS$_SIZ")
	rmsKeyType      = vmsConst("RMS$_DTP")
	rmsKeySegments  = vmsConst("RMS$_SEG")
	rmsDataArea     = vmsConst("RMS$_DAN")
	rmsIndexArea    = vmsConst("RMS$_IAN")
	rmsLevel1Area   = vmsConst("RMS$_LAN")
	rmsDataFill     = vmsConst("RMS$_DFL")
	rmsIndexFill    = vmsConst("RMS$_IFL")
	rmsKeyName      = vmsConst("RMS$_KNM")
	rmsKeyRoom      = vmsConst("RMS$_KSI")
	rmsAreaBucket   = vmsConst("RMS$_BKZ")
	rmsPrologLevel  = vmsConst("RMS$_PLV")
	rmsPrologError  = vmsConst("RMS$_PLG")
	rmsKeyRef       = vmsConst("RMS$_REF")
	rmsAreaID       = vmsConst("RMS$_AID")
)

// XABKEY's fields (the RMS Reference Manual's chapter 14), and XABALL's
// beyond those xab.go names.
var (
	xabKeyIAN    = vmsConst("XAB$B_IAN")
	xabKeyLAN    = vmsConst("XAB$B_LAN")
	xabKeyDAN    = vmsConst("XAB$B_DAN")
	xabKeyLVL    = vmsConst("XAB$B_LVL")
	xabKeyIBS    = vmsConst("XAB$B_IBS")
	xabKeyDBS    = vmsConst("XAB$B_DBS")
	xabKeyRVB    = vmsConst("XAB$L_RVB")
	xabKeyFLG    = vmsConst("XAB$B_FLG")
	xabKeyDTP    = vmsConst("XAB$B_DTP")
	xabKeyNSG    = vmsConst("XAB$B_NSG")
	xabKeyNUL    = vmsConst("XAB$B_NUL")
	xabKeyTKS    = vmsConst("XAB$B_TKS")
	xabKeyREF    = vmsConst("XAB$B_REF")
	xabKeyMRL    = vmsConst("XAB$W_MRL")
	xabKeyIFL    = vmsConst("XAB$W_IFL")
	xabKeyDFL    = vmsConst("XAB$W_DFL")
	xabKeyPOS    = vmsConst("XAB$W_POS0")
	xabKeySIZ    = vmsConst("XAB$B_SIZ0")
	xabKeyKNM    = vmsConst("XAB$L_KNM")
	xabKeyDVB    = vmsConst("XAB$L_DVB")
	xabKeyPROLOG = vmsConst("XAB$B_PROLOG")

	xabAllALN = vmsConst("XAB$B_ALN")
	xabAllVOL = vmsConst("XAB$W_VOL")
	xabAllLOC = vmsConst("XAB$L_LOC")
	xabAllBKZ = vmsConst("XAB$B_BKZ")
	xabAllAID = vmsConst("XAB$B_AID")

	xabContiguousBestTry = byte(vmsConst("XAB$M_CBT"))
)

// An XABKEY's FLG bits. The three compression bits say what isn't
// compressed, where a key descriptor's say what is (isam's Flag constants).
var (
	xabFlagDUP          = byte(vmsConst("XAB$M_DUP"))
	xabFlagCHG          = byte(vmsConst("XAB$M_CHG"))
	xabFlagNUL          = byte(vmsConst("XAB$M_NUL"))
	xabFlagNoIndexCmpr  = byte(vmsConst("XAB$M_IDX_NCMPR"))
	xabFlagNoKeyCmpr    = byte(vmsConst("XAB$M_KEY_NCMPR"))
	xabFlagNoRecordCmpr = byte(vmsConst("XAB$M_DAT_NCMPR"))

	// xabCompression is the bits that are the inverse of the descriptor's.
	xabCompression = xabFlagNoIndexCmpr | xabFlagNoKeyCmpr | xabFlagNoRecordCmpr
)

// keySegments is how many POS and SIZ pairs an XABKEY has; keyNameSize the
// bytes of a key's name at KNM.
const (
	keySegments = 8
	keyNameSize = 32
)

// isIndexed reports whether f is an indexed file.
func isIndexed(f *volume.File) bool {
	return f != nil && byte(f.Header.RecordAttributes.Format)&0xF0 == orgIdx
}

// indexedBlocks is a volume file as isam reaches it: by block, and growing
// by whole clusters, as the volume rounds every extension.
type indexedBlocks struct {
	f *volume.File
}

func (b indexedBlocks) ReadBlock(vbn uint32, block []byte) error {
	return b.f.ReadBlock(vbn, block)
}

func (b indexedBlocks) WriteBlock(vbn uint32, block []byte) error {
	return b.f.WriteBlock(vbn, block)
}

// Size is the file's allocation, in blocks.
func (b indexedBlocks) Size() uint32 { return b.f.Blocks() }

// Extend adds at least count blocks at the file's end, rounded up by the
// volume's cluster.
func (b indexedBlocks) Extend(count uint32) (uint32, uint32, error) {
	old := b.f.Blocks()

	bm, err := b.f.Device.Bitmap()
	if err != nil {
		return 0, 0, err
	}

	ib, err := b.f.Device.IndexBitmap()
	if err != nil {
		return 0, 0, err
	}

	if err := volume.Extend(b.f, bm, ib, count); err != nil {
		return 0, 0, err
	}

	// Once RMS has extended an indexed file its end of file is the block
	// after the allocation (VMS 7.1: the phase doc's "What RMS's $CREATE
	// settled"); until then it's 0.
	b.f.SetEndOfFile(b.f.Blocks()+1, 0)

	return old + 1, b.f.Blocks() - old, nil
}

// readOnlyBlocks is a volume file isam may only read, so a File opened on it
// can't be changed by mistake.
type readOnlyBlocks struct {
	f *volume.File
}

func (b readOnlyBlocks) ReadBlock(vbn uint32, block []byte) error {
	return b.f.ReadBlock(vbn, block)
}

// indexedAttributes is what isam needs of a file's header.
func indexedAttributes(f *volume.File) isam.Attributes {
	ra := f.Header.RecordAttributes

	attr := isam.Attributes{Size: int(ra.MaxRecordSize)}
	if ondisk.RecordFormat(byte(ra.Format)&0x0F) == ondisk.RecordFormatFixed {
		attr.Format = isam.Fixed
	}

	return attr
}

// openIndexed reads an indexed file's prolog, for writing if writable.
func openIndexed(f *volume.File, writable bool) (*isam.File, error) {
	if writable {
		return isam.Open(indexedBlocks{f}, indexedAttributes(f))
	}

	return isam.Open(readOnlyBlocks{f}, indexedAttributes(f))
}

// prologStatus is the RMS status for a prolog isam won't read: RMS$_PLV for
// Prologs 1 and 2, which govax doesn't have (the author's answer 1), and
// RMS$_PLG for one that's damaged or missing.
func prologStatus(err error) uint32 {
	var level *isam.ErrPrologLevel
	if errors.As(err, &level) {
		return rmsPrologLevel
	}

	return rmsPrologError
}

// xabKey is one XABKEY of a $CREATE, as the program set it.
type xabKey struct {
	addr     uint32
	ref      int
	dtp      byte
	flg      byte
	nul      byte
	pos      [keySegments]uint16
	siz      [keySegments]byte
	ian, lan int
	dan      int
	dfl, ifl int
	knm      uint32
	prolog   byte
}

// readXABKey reads an XABKEY's input fields. XAB$B_PROLOG is read only from
// a block long enough to have it.
func (ctx *Context) readXABKey(x xabEntry) (xabKey, error) {
	k := xabKey{addr: x.Addr}

	bytes, err := ctx.loadBytes(x.Addr, int(xabKeyPROLOG)+1)
	if err != nil {
		return k, err
	}

	word := func(off uint32) uint16 { return uint16(bytes[off]) | uint16(bytes[off+1])<<8 }

	k.ref = int(bytes[xabKeyREF])
	k.dtp = bytes[xabKeyDTP]
	k.flg = bytes[xabKeyFLG]
	k.nul = bytes[xabKeyNUL]
	k.ian, k.lan, k.dan = int(bytes[xabKeyIAN]), int(bytes[xabKeyLAN]), int(bytes[xabKeyDAN])
	k.dfl, k.ifl = int(word(xabKeyDFL)), int(word(xabKeyIFL))
	k.knm = uint32(word(xabKeyKNM)) | uint32(word(xabKeyKNM+2))<<16

	for i := range keySegments {
		k.pos[i] = word(xabKeyPOS + uint32(2*i))
		k.siz[i] = bytes[xabKeySIZ+uint32(i)]
	}

	if bln, err := ctx.loadByte(x.Addr + xabBLN); err == nil && uint32(bln) > xabKeyPROLOG {
		k.prolog = bytes[xabKeyPROLOG]
	}

	return k, nil
}

// segments is the key's segments: its POS and SIZ pairs up to the first
// size of 0.
func (k *xabKey) segments() []isam.Segment {
	var segs []isam.Segment

	for i := range keySegments {
		if k.siz[i] == 0 {
			break
		}

		segs = append(segs, isam.Segment{Position: int(k.pos[i]), Size: int(k.siz[i]), Type: isam.Type(k.dtp)})
	}

	return segs
}

// numericSize is each numeric type's one size; a packed key's is 1 to 16.
var numericSize = map[isam.Type]int{
	isam.Type(vmsConst("XAB$C_IN2")): 2, isam.Type(vmsConst("XAB$C_BN2")): 2,
	isam.Type(vmsConst("XAB$C_IN4")): 4, isam.Type(vmsConst("XAB$C_BN4")): 4,
	isam.Type(vmsConst("XAB$C_IN8")): 8, isam.Type(vmsConst("XAB$C_BN8")): 8,
}

// maxPackedKey and maxKeyBytes are the longest packed decimal key and the
// longest key of any kind, in bytes (the RMS manual, XAB$B_SIZ).
const (
	maxPackedKey = 16
	maxKeyBytes  = 255
)

// shortestCompressedKey is the shortest string key whose key and index are
// compressed by default: VMS left a 5-byte key alone and compressed a
// 6-byte one (testdata/idx88's S4 and S5).
const shortestCompressedKey = 6

// checkKey refuses a key VMS refuses, in the order of its single-fault
// cases: the type (RMS$_DTP; COL and DCOL too, which govax leaves out by
// Decision 7), the primary key's flags (RMS$_FLG: no CHG or NUL), segments
// (RMS$_SEG: only a string may have more than one), sizes (RMS$_SIZ), and a
// key past the longest record (RMS$_POS). A primary key whose segments
// overlap would make VMS write Prolog 1 or 2 quietly; govax has only Prolog
// 3, so it's RMS$_PLV, govax's choice (answer 1). That an alternate key past
// MRS is RMS$_POS as the primary is is unconfirmed: no case tries one.
func checkKey(k *xabKey, format ondisk.RecordFormat, mrs uint16) uint32 {
	kind := isam.Type(k.dtp) &^ isam.Descending
	if k.dtp&^byte(isam.Descending) > byte(isam.Packed+2) || kind == isam.Collated {
		return rmsKeyType
	}

	if k.ref == 0 && k.flg&(xabFlagCHG|xabFlagNUL) != 0 {
		return rmsKeyFlags
	}

	segs := k.segments()
	if len(segs) == 0 {
		return rmsKeySize
	}

	if kind != isam.String && len(segs) > 1 {
		return rmsKeySegments
	}

	total, longest := 0, 0
	for _, s := range segs {
		total += s.Size
		longest = max(longest, s.Position+s.Size)
	}

	switch size, numeric := numericSize[kind]; {
	case numeric && total != size,
		kind == isam.Packed && total > maxPackedKey,
		total > maxKeyBytes:
		return rmsKeySize
	}

	if (format == ondisk.RecordFormatFixed || mrs != 0) && longest > int(mrs) {
		return rmsKeyPosition
	}

	if k.ref == 0 && overlapping(segs) {
		return rmsPrologLevel
	}

	return 0
}

// overlapping reports whether any two segments share a byte.
func overlapping(segs []isam.Segment) bool {
	for i, a := range segs {
		for _, b := range segs[i+1:] {
			if a.Position < b.Position+b.Size && b.Position < a.Position+a.Size {
				return true
			}
		}
	}

	return false
}

// descriptorFlags maps an XABKEY's FLG onto a key descriptor's: DUP, CHG,
// and NUL as they are, and each part compressed unless the XABKEY says not.
// Only a string key of 6 bytes or more has its key and index compressed,
// and only the primary key's records, the alternates having none (VMS
// 7.1: testdata/idx88's shape cases, the phase doc's "What the writer
// settled").
func descriptorFlags(k *xabKey, total int) uint8 {
	flags := k.flg & (xabFlagDUP | xabFlagCHG | xabFlagNUL)

	kind := isam.Type(k.dtp) &^ isam.Descending
	if kind == isam.String && total >= shortestCompressedKey {
		if k.flg&xabFlagNoKeyCmpr == 0 {
			flags |= isam.FlagKeyCompress
		}

		if k.flg&xabFlagNoIndexCmpr == 0 {
			flags |= isam.FlagIndexCompress
		}
	}

	if k.ref == 0 && k.flg&xabFlagNoRecordCmpr == 0 {
		flags |= isam.FlagRecordCompress
	}

	return flags
}

// xabFlags is a key descriptor's flags as an XABKEY reports them: the
// compression bits inverted, and KEY$M_INITIDX (10 hex, no index made yet)
// as it is, which VMS's $DISPLAY shows (S2).
func xabFlags(descriptor uint8) byte {
	return descriptor ^ xabCompression
}

// Bucket sizes and record sizes. A Prolog 3 bucket of b blocks holds a
// record of at most b*512-32 bytes: VMS took MRS 32224 in a 63-block bucket
// (S55), and the bucket's header (14 bytes), its end (4), and a compressed
// variable record's overhead (11, and 3 for compression) are 32. A record
// up to 9 bytes longer, b*512-23, fitted VMS's Prolog 1 (E92, MRS 32225 made
// a Prolog 1 file, and 32234 was RMS$_MRS), so govax, which has only Prolog
// 3, says RMS$_PLV for those (answer 1) and RMS$_MRS past them. That fixed
// records have the same limit is unconfirmed.
const (
	bucketOverhead       = 32
	prologOneOverhead    = 23
	smallestDefaultBlock = 2
)

// largestRecord is the longest record a Prolog 3 bucket of blocks holds.
func largestRecord(blocks int) int {
	return blocks*isam.BlockSize - bucketOverhead
}

// defaultBucket is the bucket size RMS picks when the FAB or XABALL gives
// none: the smallest that holds one record of MRS bytes, and at least 2
// blocks. VMS gave 2 to every small record and 10 to MRS 5000 (S54); the
// manual says both "one record" and "two", which these don't settle further.
func defaultBucket(mrs uint16) int {
	for b := smallestDefaultBlock; b < maxBucketBlocks; b++ {
		if largestRecord(b) >= int(mrs) {
			return b
		}
	}

	return maxBucketBlocks
}

// keyRoom reports whether an index bucket of blocks holds two entries of a
// key of size bytes, or RMS$_KSI. An entry is the key and a pointer of up to
// 4 bytes; that VMS counts it so is unconfirmed, no case reaching the limit.
func keyRoom(blocks, size int) bool {
	return 2*(size+4) <= blocks*isam.BlockSize-18
}

// indexedPlan is what $CREATE settled for an indexed file before making it.
type indexedPlan struct {
	spec  isam.Spec
	names []uint32 // each key's KNM, 0 for an unnamed key
	bks   byte     // area 0's bucket size: the header's, and the FAB's
	deq   uint16   // area 0's default extension: the header's
	aop   byte     // the areas' options together, for the header's contiguity
}

// checkIndexedCreate reads an indexed $CREATE's XABKEYs and XABALLs, makes
// its isam.Spec, and refuses what VMS refuses, writing FAB$L_ALQ and
// FAB$B_BKS as VMS does along the way. It returns the plan, or a status and
// its STV.
func (ctx *Context) checkIndexedCreate(fab uint32, format ondisk.RecordFormat, mrs uint16) (*indexedPlan, uint32, uint32, error) {
	if format != ondisk.RecordFormatFixed && format != ondisk.RecordFormatVariable {
		return nil, rmsInvalidRFM, 0, nil
	}

	bks, err := ctx.loadByte(fab + fabOffset("BKS"))
	if err != nil {
		return nil, 0, 0, err
	}

	if bks > maxBucketBlocks {
		return nil, rmsInvalidBKS, 0, nil
	}

	chain, _, _, err := ctx.xabChain(fab)
	if err != nil {
		return nil, 0, 0, err
	}

	keys, sts, stv, err := ctx.createKeys(chain, format, mrs)
	if err != nil || sts != 0 {
		return nil, sts, stv, err
	}

	plan := &indexedPlan{}
	plan.spec.Attributes = isam.Attributes{Size: int(mrs)}

	if format == ondisk.RecordFormatFixed {
		plan.spec.Attributes.Format = isam.Fixed
	}

	if sts, stv, err := ctx.createAreas(fab, chain, plan, keys); err != nil || sts != 0 {
		return nil, sts, stv, err
	}

	areas := plan.spec.Areas

	for i := range keys {
		k := &keys[i]

		switch {
		case k.dan >= len(areas):
			return nil, rmsDataArea, uint32(k.ref), nil
		case k.ian >= len(areas):
			return nil, rmsIndexArea, uint32(k.ref), nil
		case k.lan >= len(areas):
			return nil, rmsLevel1Area, uint32(k.ref), nil
		}
	}

	// An area with no bucket size gets RMS's default.
	for i := range areas {
		if areas[i].BucketSize == 0 {
			areas[i].BucketSize = defaultBucket(mrs)
		}
	}

	plan.bks = byte(areas[0].BucketSize)
	if err := ctx.storeByte(fab+fabOffset("BKS"), plan.bks); err != nil {
		return nil, 0, 0, err
	}

	if sts, stv := plan.addKeys(keys, mrs); sts != 0 {
		return nil, sts, stv, nil
	}

	return plan, 0, 0, nil
}

// createKeys reads a $CREATE's XABKEYs in key order, and checks each. An
// alternate key's XABKEY may come before the primary's (VMS took one: E71);
// keys must then be 0, 1, 2, ..., one of each: RMS$_NPK with no key 0 (its
// STV the first XABKEY, or 0 with none), and RMS$_IMX for a key twice (its
// STV the second of the two in chain order: E70) or a gap (its STV the key
// before the gap, as VMS gave key 0's for keys 0 and 2: E69).
func (ctx *Context) createKeys(chain []xabEntry, format ondisk.RecordFormat, mrs uint16) ([]xabKey, uint32, uint32, error) {
	var keys []xabKey

	for _, x := range chain {
		if x.Cod != xabKEY {
			continue
		}

		k, err := ctx.readXABKey(x)
		if err != nil {
			return nil, 0, 0, err
		}

		keys = append(keys, k)
	}

	if len(keys) == 0 {
		return nil, rmsNoPrimaryKey, 0, nil
	}

	sort.SliceStable(keys, func(i, j int) bool { return keys[i].ref < keys[j].ref })

	if keys[0].ref != 0 {
		return nil, rmsNoPrimaryKey, keys[0].addr, nil
	}

	for i := 1; i < len(keys); i++ {
		switch {
		case keys[i].ref == keys[i-1].ref:
			return nil, rmsDuplicateXAB, keys[i].addr, nil
		case keys[i].ref != i:
			return nil, rmsDuplicateXAB, keys[i-1].addr, nil
		}
	}

	for i := range keys {
		if sts := checkKey(&keys[i], format, mrs); sts != 0 {
			return nil, sts, uint32(i), nil
		}
	}

	// Only Prolog 3 (answer 1): PROLOG=1 or 2 would have VMS write that
	// level, and anything else is VMS's RMS$_PLV too (E102, PROLOG 9).
	if p := keys[0].prolog; p != 0 && p != 3 {
		return nil, rmsPrologLevel, 0, nil
	}

	return keys, 0, 0, nil
}

// createAreas reads a $CREATE's XABALLs, or the FAB for a file with none, into
// the plan's areas. The XABALLs must be areas 0, 1, 2, ... in chain order
// (RMS$_IMX; its STV is the primary key's XABKEY, as VMS gave for an AID 1
// alone and for AIDs 0 and 2, E99 and E100, though why isn't known), each with a bucket size of at most 63 blocks
// (RMS$_BKZ, STV the area). FAB$L_ALQ takes each area's allocation as it's
// read (E100: 4 after area 0, then RMS$_IMX at area 2), and at the end the
// whole, area 0 holding at least the prolog (E79: 2 for a file of one key
// and area). With no XABALL, the FAB's BKS, ALQ, DEQ, and FOP's CTG and CBT
// are area 0's; that its AOP follows FOP is unconfirmed.
func (ctx *Context) createAreas(fab uint32, chain []xabEntry, plan *indexedPlan, keys []xabKey) (uint32, uint32, error) {
	var total uint32

	for _, x := range chain {
		if x.Cod != xabALL {
			continue
		}

		bytes, err := ctx.loadBytes(x.Addr, int(xabAllAID)+1)
		if err != nil {
			return 0, 0, err
		}

		aid := int(bytes[xabAllAID])
		if aid != len(plan.spec.Areas) {
			return rmsDuplicateXAB, keys[0].addr, nil
		}

		bkz := int(bytes[xabAllBKZ])
		if bkz > maxBucketBlocks {
			return rmsAreaBucket, uint32(aid), nil
		}

		long := func(off uint32) uint32 {
			return uint32(bytes[off]) | uint32(bytes[off+1])<<8 | uint32(bytes[off+2])<<16 | uint32(bytes[off+3])<<24
		}

		a := isam.AreaSpec{
			BucketSize: bkz,
			Allocation: long(xabALQ),
			Extend:     int(bytes[xabDEQ]) | int(bytes[xabDEQ+1])<<8,
			Alignment:  bytes[xabAllALN],
			Options:    bytes[xabAOP],
			Volume:     int(bytes[xabAllVOL]) | int(bytes[xabAllVOL+1])<<8,
			Location:   long(xabAllLOC),
		}

		plan.spec.Areas = append(plan.spec.Areas, a)
		total += a.Allocation

		if err := ctx.storeLongword(fab+fabOffset("ALQ"), total); err != nil {
			return 0, 0, err
		}
	}

	if len(plan.spec.Areas) == 0 {
		a, err := ctx.fabArea(fab)
		if err != nil {
			return 0, 0, err
		}

		plan.spec.Areas = []isam.AreaSpec{a}
		total = a.Allocation
	}

	prolog := isam.PrologBlocks(len(keys), len(plan.spec.Areas))
	first := &plan.spec.Areas[0]

	if first.Allocation < prolog {
		total += prolog - first.Allocation
		first.Allocation = prolog
	}

	// The header's contiguity is any area's: VMS 7.1 marked C38.IDX
	// "Contiguous best try" for its area 2's CBT, areas 0 and 1 having
	// none (testdata/idx88's round 3).
	plan.deq = uint16(first.Extend)
	for _, a := range plan.spec.Areas {
		plan.aop |= a.Options
	}

	return 0, 0, ctx.storeLongword(fab+fabOffset("ALQ"), total)
}

// fabArea is the one area of a file with no XABALL, from its FAB.
func (ctx *Context) fabArea(fab uint32) (isam.AreaSpec, error) {
	var a isam.AreaSpec

	bks, err := ctx.loadByte(fab + fabOffset("BKS"))
	if err != nil {
		return a, err
	}

	alq, err := ctx.loadLongword(fab + fabOffset("ALQ"))
	if err != nil {
		return a, err
	}

	deq, err := ctx.loadWord(fab + fabOffset("DEQ"))
	if err != nil {
		return a, err
	}

	fop, err := ctx.loadLongword(fab + fabFOP)
	if err != nil {
		return a, err
	}

	a = isam.AreaSpec{BucketSize: int(bks), Allocation: alq, Extend: int(deq)}

	switch {
	case fop&fopCTG != 0:
		a.Options = xabContiguous
	case fop&fopCBT != 0:
		a.Options = xabContiguousBestTry
	}

	return a, nil
}

// addKeys makes the plan's key specs, once the areas' bucket sizes are
// known, checking what depends on them: the longest record against the
// primary key's data bucket (RMS$_MRS, or RMS$_PLV where only a lower
// prolog would hold it), each fill size against its bucket (RMS$_DFL,
// RMS$_IFL), and the index bucket's room for two keys (RMS$_KSI).
func (p *indexedPlan) addKeys(keys []xabKey, mrs uint16) (uint32, uint32) {
	areas := p.spec.Areas

	if data := areas[keys[0].dan].BucketSize; mrs != 0 && int(mrs) > largestRecord(data) {
		if int(mrs) <= data*isam.BlockSize-prologOneOverhead {
			return rmsPrologLevel, 0
		}

		return rmsInvalidMRS, 0
	}

	for i := range keys {
		k := &keys[i]
		data, index := areas[k.dan].BucketSize, areas[k.ian].BucketSize

		switch {
		case k.dfl > data*isam.BlockSize:
			return rmsDataFill, uint32(i)
		case k.ifl > index*isam.BlockSize:
			return rmsIndexFill, uint32(i)
		}

		segs := k.segments()

		total := 0
		for _, s := range segs {
			total += s.Size
		}

		if !keyRoom(index, total) {
			return rmsKeyRoom, uint32(i)
		}

		p.spec.Keys = append(p.spec.Keys, isam.KeySpec{
			Segments:   segs,
			Type:       isam.Type(k.dtp),
			Flags:      descriptorFlags(k, total),
			Null:       k.nul,
			IndexArea:  k.ian,
			Level1Area: k.lan,
			DataArea:   k.dan,
			IndexFill:  k.ifl,
			DataFill:   k.dfl,
		})
		p.names = append(p.names, k.knm)
	}

	return 0, 0
}

// initIndexed writes a new indexed file's prolog and first allocation, and
// the header's fields that follow from them: area 0's bucket size and
// default extension, contiguity by area 0's AOP (when the allocation came
// out in one piece; unconfirmed), and an end of file of 0, which VMS keeps
// until RMS extends the file past its first allocation (E71, S2). A key name
// RMS can't read is RMS$_KNM with the key as STV, and as VMS's E85, the file
// is left with its allocation and no prolog.
func (ctx *Context) initIndexed(f *volume.File, plan *indexedPlan) (uint32, uint32, error) {
	for i, knm := range plan.names {
		if knm == 0 {
			continue
		}

		name, err := ctx.loadFixedString(knm, keyNameSize)
		if err != nil {
			var total uint32
			for _, a := range plan.spec.Areas {
				total += a.Allocation
			}

			if _, _, err := (indexedBlocks{f}).Extend(total); err != nil {
				return rmsDeviceError, 0, nil
			}

			return rmsKeyName, uint32(i), nil
		}

		plan.spec.Keys[i].Name = strings.TrimRight(name, " \x00")
		plan.spec.Keys[i].Named = strings.Trim(name, "\x00") != ""
	}

	if _, err := isam.Create(indexedBlocks{f}, plan.spec); err != nil {
		return rmsDeviceError, 0, nil
	}

	err := volume.UpdateHeader(f, func(h *ondisk.FileHeader, _ *ondisk.Ident) {
		h.RecordAttributes.BucketSize = plan.bks
		h.RecordAttributes.DefaultExtend = plan.deq
		h.RecordAttributes.EndOfFileBlock = 0
		h.RecordAttributes.FirstFreeByte = 0

		// A file of more than one area ends past its allocation, as if
		// each area after the first were an extension: VMS 7.1's
		// CREATE/FDL files C38.IDX and C52.IDX, of three areas and no
		// records, end at block 52 and 22, where those of one area end at
		// 0 (testdata/idx88's round 3).
		if len(plan.spec.Areas) > 1 {
			h.RecordAttributes.EndOfFileBlock = h.RecordAttributes.HighestBlock + 1
		}

		if contiguous(f.Extents) {
			switch {
			case plan.aop&xabContiguous != 0:
				h.FileCharacteristics |= ondisk.FchContig
			case plan.aop&xabContiguousBestTry != 0:
				h.FileCharacteristics |= ondisk.FchContigB
			}
		}
	})
	if err != nil {
		return rmsDeviceError, 0, nil
	}

	return 0, 0, nil
}

// indexedRecords reports whether h is a stream of an indexed file's records
// (not a block stream, whose operations are any file's).
func (h *FileHandle) indexedRecords() bool {
	return h.indexed && !h.stream.blocks
}
