package lbr

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// This file changes a library in place, a block at a time, as the LBR$
// routines do (LBR$PUT_RECORD, LBR$INSERT_KEY, and the rest;
// docs/planning/PHASE-90 - VMS help.md, subtask 11). Where a Builder lays a whole
// library out at once, an Updater keeps the file's blocks and changes them
// as each call asks, so that a library made by a sequence of calls is the
// one VMS 7.1's LBRSHR made from the same calls (testdata/help90's round 3,
// whose libraries are VMS's). What round 3 showed of the rules:
//
//   - Records go into data blocks as LIBRARY writes them (dataWriter's
//     layout), a module's header written by its first record. A session
//     (an open of the library) starts its data in a new block, at the
//     header's next VBN, not linked to the last session's, unless the
//     block at the header's next RFA is the data block a routine last read
//     (NoteRead); an end of module with no records before it starts a new
//     block too. A block a record runs on past is linked to the next one,
//     which the next VBN gives.
//   - Index blocks come from the free list the library was created with,
//     and when it's empty from the next VBN, between data blocks. A leaf
//     that a new entry won't fit splits: it keeps the first entries, the
//     new block after it the rest, and a root that splits gets a new root,
//     taken after the new block. A block isn't cleared past its entries.
//   - A module's header counts the keys that point at it. INSERT_KEY
//     enters its key before it finds the module, so a key with a bad RFA
//     goes in with the error.
//   - DELETE_DATA takes a module's records off each block's count, and a
//     block left with none goes on the deleted blocks' list. It doesn't
//     look at whether it was done before: twice is twice.
//
// docs/planning/PHASE-90 - VMS help.md's progress log (subtask 11b) has the cases
// that showed each rule, and which parts are unconfirmed.
//
// Only the V3 format's variable-length ASCII keys are changed; binary keys
// and the V2 format are later work.

// The LBR$ conditions an Updater reports. The LBR$ routines turn each into
// its status.
var (
	ErrDuplicateKey = errors.New("lbr: duplicate key in index")  // LBR$_DUPKEY
	ErrInvalidKey   = errors.New("lbr: invalid key")             // LBR$_INVKEY
	ErrKeyNotFound  = errors.New("lbr: key not found")           // LBR$_KEYNOTFND
	ErrInvalidRFA   = errors.New("lbr: invalid RFA")             // LBR$_INVRFA
	ErrPastEOF      = errors.New("lbr: RFA past end of file")    // LBR$_RFAPASTEOF
	ErrNoRecord     = errors.New("lbr: no module header there")  // RMS$_EOF
	ErrRecordLength = errors.New("lbr: illegal record length")   // LBR$_RECLNG
	ErrStillKeys    = errors.New("lbr: keys still point at data") // LBR$_STILLKEYS
	ErrIndexNumber  = errors.New("lbr: illegal index number")    // LBR$_ILLIDXNUM
	ErrCreate       = errors.New("lbr: illegal create options")  // LBR$_ILLCREOPT
)

// Updater is a library being created or changed in place.
type Updater struct {
	data []byte

	// cur is where the next record goes; open is false when the next
	// record starts a new block (at the start of a session, and for an
	// end of module with no records).
	cur  RFA
	open bool

	// inModule says a record was written since the last end of module:
	// the next record isn't a new module's.
	inModule bool
	module   RFA // the current module's header

	// lastRead is the data block a routine last read, which a session's
	// first record may continue (NoteRead).
	lastRead uint32

	// Now gives the time a module's header records. It's the caller's, so
	// that tests can fix it.
	Now func() uint64
}

// Index layout limits (unconfirmed beyond what round 3's W9 shows: 44
// entries of 11 bytes fit a block and the 45th split it, 32 entries kept).
const (
	indexFull  = 494 // the most bytes of entries a block takes
	splitShare = 7   // the old block keeps 7/10 of the entries' bytes, the entry crossing it included
)

// UpdateOptions are LBR$OPEN's create options (the manual's CRE$ block).
type UpdateOptions struct {
	Type      Type
	KeyLength int // CRE$L_KEYLEN: the longest key
	Indexes   int // CRE$L_IDXMAX
	UserData  int // CRE$L_UHDMAX: a module header's user data
	Entries   int // CRE$L_ENTALL: index entries to preallocate blocks for
	History   int // CRE$L_LUHMAX
	IndexOpts int // CRE$L_IDXOPT: CRE$M_NOCASECMP, NOCASENTR, UPCASNTRY
	// V2 makes the library in the VMS V2 format (CRE$C_VMSV2): its own
	// sanity longword, and ASCII index entries of a fixed length.
	V2        bool
	Librarian string
	Created   uint64
}

// binaryKeyLength is a binary key's length: CRE$L_KEYLEN 0 asks for
// binary keys, the manual's "32-bit unsigned keys". (Unconfirmed: VMS 7.1's
// LBR$INSERT_KEY failed with an access violation in round 3's W13, the one
// library with binary keys and an index; W15, with none, left index 1's
// descriptor all zeros, a key length of 0.)
const binaryKeyLength = 4

// preallocated is how many index blocks a new library starts with, for
// opts.Entries entries: as many as blocks of 500 bytes hold entries of the
// RFA and the longest key, at least 1. In V3, keys of 31 took 1 block for
// 1 entry and 8 (round 3b's X7, round 3's W9) and 7 for 96 (W6), keys of 15
// 4 for 96 (X8), and binary keys (KEYLEN 0, entries of 6 bytes) 1 for 96
// (W13). The V2 format took 9 for 96 keys of 31 (W14), 5 for 96 of 15 (X9),
// and 1 for 8 of 31 (X10), its fixed entries taken here at 4/5 of the 500
// bytes. 0 entries is defaultEntries: keys of 31 took 23 blocks (X6), and
// binary keys 3 (W15, X1).
func preallocated(opts UpdateOptions) int {
	entries := opts.Entries
	if entries == 0 {
		entries = defaultEntries
	}

	if opts.V2 && opts.KeyLength > 0 {
		return max(entries/(indexSpace*4/5/(keyOverhead+opts.KeyLength)), 1)
	}

	return max(entries/(indexSpace/(opts.KeyLength+rfaLength)), 1)
}

// defaultEntries is the entries a library is preallocated for when
// CRE$L_ENTALL is 0: X6's 23 blocks for keys of 31 fit 299 to 311, and
// W15's 3 for binary keys 249 to 331, so 300.
const defaultEntries = 300

// NewUpdater makes a new, empty library with opts: the header, and the
// index blocks the entries need, on a free list.
func NewUpdater(opts UpdateOptions) (*Updater, error) {
	if opts.KeyLength < 0 || opts.KeyLength > maxKeySize || opts.Indexes < 0 ||
		lhdIdxDesc+opts.Indexes*iddLength > blockSize || opts.UserData < 0 || opts.UserData > 255 ||
		opts.History < 0 || opts.History > 0xFFFF || opts.Entries < 0 {
		return nil, ErrCreate
	}

	prealloc := preallocated(opts)
	first := uint32(prealloc) + 2

	data := make([]byte, int(first-1)*blockSize)
	h := data[:blockSize]
	h[lhdType] = byte(opts.Type)
	h[lhdNIndex] = byte(opts.Indexes)
	binary.LittleEndian.PutUint32(h[lhdSanity:], saneID3)
	if opts.V2 {
		binary.LittleEndian.PutUint32(h[lhdSanity:], saneID2)
	}

	binary.LittleEndian.PutUint16(h[lhdMajorID:], 3)
	h[lhdLbrVer] = byte(copy(h[lhdLbrVer+1:lhdLbrVer+32], opts.Librarian))
	binary.LittleEndian.PutUint64(h[lhdCreDat:], opts.Created)
	binary.LittleEndian.PutUint64(h[lhdUpdTim:], opts.Created)
	h[lhdMhdUsz] = byte(opts.UserData)
	binary.LittleEndian.PutUint32(h[lhdNextRFA:], first)
	binary.LittleEndian.PutUint32(h[lhdNextVBN:], first)
	binary.LittleEndian.PutUint32(h[lhdFreIdxBlk:], uint32(prealloc))
	binary.LittleEndian.PutUint32(h[lhdFreeIdx:], 2)
	binary.LittleEndian.PutUint32(h[lhdHiPreAl:], first-1)
	binary.LittleEndian.PutUint16(h[lhdMaxLUHRec:], uint16(opts.History))

	// An ASCII index's key length counts the key's count byte; a binary
	// one's is the key's. The V2 format's ASCII entries are all of the key
	// length (W14's flags 1, no IDD$V_VARLENGTH).
	flags, keyLen := uint16(IndexASCII|IndexVarLength), uint16(opts.KeyLength+1)

	switch {
	case opts.KeyLength == 0:
		flags, keyLen = 0, binaryKeyLength
	case opts.V2:
		flags = IndexASCII
	}

	for i := range opts.Indexes {
		d := h[lhdIdxDesc+i*iddLength:]
		binary.LittleEndian.PutUint16(d, flags|uint16(opts.IndexOpts&7)<<3)
		binary.LittleEndian.PutUint16(d[2:], keyLen)
	}

	for v := uint32(2); v < first-1; v++ {
		binary.LittleEndian.PutUint32(data[(v-1)*blockSize:], v+1)
	}

	return &Updater{data: data}, nil
}

// OpenUpdater starts a change to the library data holds, which it takes
// over. It must be a library Open reads.
func OpenUpdater(data []byte) (*Updater, error) {
	if _, err := Open(data); err != nil {
		return nil, err
	}

	return &Updater{data: data}, nil
}

// Bytes returns the library's file as it stands.
func (u *Updater) Bytes() []byte { return u.data }

// Library reads the library as it stands.
func (u *Updater) Library() (*Library, error) { return Open(u.data) }

func (u *Updater) hdr(off int) uint32 { return le32(u.data, off) }

func (u *Updater) setHdr(off int, v uint32) { binary.LittleEndian.PutUint32(u.data[off:], v) }

func (u *Updater) addHdr(off int, n int) { u.setHdr(off, uint32(int(u.hdr(off))+n)) }

// SetUpdated sets the library's update time.
func (u *Updater) SetUpdated(t uint64) { binary.LittleEndian.PutUint64(u.data[lhdUpdTim:], t) }

func (u *Updater) blocks() uint32 { return uint32(len(u.data) / blockSize) }

func (u *Updater) block(vbn uint32) []byte { return u.data[(vbn-1)*blockSize : vbn*blockSize] }

// newBlock takes the next VBN for a block, zeroed.
func (u *Updater) newBlock() uint32 {
	v := u.hdr(lhdNextVBN)
	for u.blocks() < v {
		u.data = append(u.data, make([]byte, blockSize)...)
	}

	clear(u.block(v))
	u.setHdr(lhdNextVBN, v+1)

	return v
}

func (u *Updater) userSize() int { return int(u.data[lhdMhdUsz]) }

// The data.

// startBlock starts a new data block, linked from the last one if the
// record goes on into it.
func (u *Updater) startBlock(recs byte, link bool) {
	v := u.newBlock()
	if link && u.open {
		binary.LittleEndian.PutUint32(u.block(u.cur.VBN)[dataLink:], v)
	}

	u.block(v)[0] = recs
	u.cur, u.open = RFA{VBN: v, Offset: dataData}, true
}

// advance moves past n bytes, word-aligned, starting the next block when
// this one is full: the record goes on into it, or if done is set it's
// empty so far.
func (u *Updater) advance(n int, done bool) {
	off := (int(u.cur.Offset) + n + 1) &^ 1
	if off < blockSize {
		u.cur.Offset = uint16(off)

		return
	}

	if done {
		u.startBlock(0, true)
	} else {
		u.startBlock(1, true)
	}
}

// write writes rec where the next record goes, and returns where it
// starts.
func (u *Updater) write(rec []byte) RFA {
	if !u.open {
		next := RFA{VBN: u.hdr(lhdNextRFA), Offset: le16(u.data, lhdNextRFA+4)}

		if u.lastRead != 0 && u.lastRead == next.VBN && next.Offset >= dataData && next.VBN < u.hdr(lhdNextVBN) {
			u.cur, u.open = next, true
		} else {
			u.startBlock(0, false)
		}
	}

	u.block(u.cur.VBN)[0]++

	at := u.cur
	binary.LittleEndian.PutUint16(u.block(at.VBN)[at.Offset:], uint16(len(rec)))
	u.advance(2, false)

	for left := rec; ; {
		n := copy(u.block(u.cur.VBN)[u.cur.Offset:], left)
		left = left[n:]

		u.advance(n, len(left) == 0)

		if len(left) == 0 {
			break
		}
	}

	u.setHdr(lhdNextRFA, u.cur.VBN)
	binary.LittleEndian.PutUint16(u.data[lhdNextRFA+4:], u.cur.Offset)

	return at
}

// PutRecord writes a record of the current module, the module's header
// first if it's the module's first, and returns where the module starts.
func (u *Updater) PutRecord(rec []byte) (RFA, error) {
	if len(rec) > maxRecSize {
		return RFA{}, ErrRecordLength
	}

	if !u.inModule {
		mhd := make([]byte, mhdLen+u.userSize())
		mhd[1] = mhdID

		if u.Now != nil {
			binary.LittleEndian.PutUint64(mhd[8:], u.Now())
		}

		u.module = u.write(mhd)
		u.inModule = true
		u.addHdr(lhdModHdrs, 1)
	}

	u.write(rec)

	return u.module, nil
}

// PutEnd ends the current module. With no module, its end of module record
// starts a new block, as VMS 7.1's did.
func (u *Updater) PutEnd() {
	if !u.inModule {
		u.open, u.lastRead = false, 0
	}

	u.write(eot)
	u.inModule = false
}

// span is part of a record: bytes at an offset of a block.
type span struct {
	vbn uint32
	off int
	n   int
}

// record finds the record at rfa: its bytes' places, the blocks it counts
// in (every block with any of it, and the block after a length word that
// ends its block), and where the next record starts.
func (u *Updater) record(rfa RFA) ([]span, []uint32, RFA, error) {
	if rfa.VBN == 0 || rfa.VBN == 1 && rfa.Offset != 0 {
		return nil, nil, rfa, ErrNoRecord
	}

	if rfa.VBN > u.blocks() {
		return nil, nil, rfa, ErrPastEOF
	}

	off := int(rfa.Offset)
	if off == 0 {
		off = dataData
	}

	if off > blockSize-2 {
		return nil, nil, rfa, ErrNoRecord
	}

	vbn := rfa.VBN
	count := int(le16(u.block(vbn), off))
	touched := []uint32{vbn}

	// next moves to the next block of the chain.
	next := func() error {
		link := le32(u.block(vbn), dataLink)
		if link == 0 || link > u.blocks() {
			return ErrNoRecord
		}

		vbn, off = link, dataData

		return nil
	}

	if count > maxRecSize {
		return nil, nil, rfa, ErrNoRecord
	}

	off = (off + 2 + 1) &^ 1
	if off >= blockSize {
		if err := next(); err != nil {
			return nil, nil, rfa, err
		}

		touched = append(touched, vbn)
	}

	var spans []span

	for left := count; ; {
		n := min(left, blockSize-off)
		if n > 0 {
			spans = append(spans, span{vbn: vbn, off: off, n: n})
		}

		left -= n
		off = (off + n + 1) &^ 1

		if off >= blockSize {
			if err := next(); err != nil && left > 0 {
				return nil, nil, rfa, err
			} else if err != nil {
				return spans, touched, RFA{}, nil
			}

			if left > 0 {
				touched = append(touched, vbn)
			}
		}

		if left == 0 {
			return spans, touched, RFA{VBN: vbn, Offset: uint16(off)}, nil
		}
	}
}

func (u *Updater) gather(spans []span) []byte {
	var b []byte
	for _, s := range spans {
		b = append(b, u.block(s.vbn)[s.off:s.off+s.n]...)
	}

	return b
}

func (u *Updater) scatter(spans []span, b []byte) {
	for _, s := range spans {
		b = b[copy(u.block(s.vbn)[s.off:s.off+s.n], b):]
	}
}

// moduleHeader finds the module header at rfa.
func (u *Updater) moduleHeader(rfa RFA) ([]span, []byte, error) {
	spans, _, _, err := u.record(rfa)
	if err != nil {
		return nil, nil, err
	}

	h := u.gather(spans)
	if len(h) != mhdLen+u.userSize() || h[1] != mhdID {
		return nil, nil, ErrNoRecord
	}

	return spans, h, nil
}

// refer adds n to the reference count of the module at rfa.
func (u *Updater) refer(rfa RFA, n int) error {
	spans, h, err := u.moduleHeader(rfa)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(h[4:], uint32(int(le32(h, 4))+n))
	u.scatter(spans, h)
	u.lastRead = spans[len(spans)-1].vbn

	return nil
}

// DeleteData deletes the module at rfa, which no key may point at: its
// records come off their blocks' counts, and a block left with none goes
// on the deleted blocks' list.
func (u *Updater) DeleteData(rfa RFA) error {
	_, h, err := u.moduleHeader(rfa)
	if err != nil {
		return err
	}

	if le32(h, 4) != 0 {
		return ErrStillKeys
	}

	for at := rfa; ; {
		spans, touched, next, err := u.record(at)
		if err != nil {
			return err
		}

		u.lastRead = touched[len(touched)-1]

		for _, v := range touched {
			b := u.block(v)
			if b[0] > 0 {
				b[0]--
			}

			if b[0] == 0 {
				binary.LittleEndian.PutUint32(b[dataLink:], u.hdr(lhdFreeVBN))
				u.setHdr(lhdFreeVBN, v)
				u.addHdr(lhdFreeBlk, 1)
			}
		}

		if at != rfa && bytes.Equal(u.gather(spans), eot) || next.VBN == 0 {
			break
		}

		at = next
	}

	u.addHdr(lhdModHdrs, -1)

	return nil
}

// The indexes.

// entry is an index entry: an RFA and a key. An upper level's entry's RFA
// is its child block's VBN, with offset RFA$C_INDEX.
type entry struct {
	rfa RFA
	key []byte
}

// layout is how the library's index entries are laid out: index 1's flags
// and key length. (Every index of a library the routines make is laid out
// alike.)
func (u *Updater) layout() (flags, keyLen uint16) {
	return le16(u.data, lhdIdxDesc), le16(u.data, lhdIdxDesc+2)
}

// entrySize is the bytes an entry with key takes in an index block.
func (u *Updater) entrySize(key []byte) int {
	flags, keyLen := u.layout()
	if flags&IndexASCII == 0 || flags&IndexVarLength == 0 {
		return rfaLength + int(keyLen)
	}

	return keyOverhead + len(key)
}

func (u *Updater) entriesSize(es []entry) int {
	n := 0
	for _, e := range es {
		n += u.entrySize(e.key)
	}

	return n
}

func (u *Updater) readBlock(vbn uint32) (uint32, []entry) {
	b := u.block(vbn)
	end := min(indexEntries+int(le16(b, 0)), blockSize)
	flags, keyLen := u.layout()

	var es []entry

	for pos := indexEntries; pos < end; {
		r, key, size, err := readEntry(b[:end], pos, flags, keyLen)
		if err != nil {
			break
		}

		es = append(es, entry{rfa: r, key: append([]byte(nil), key...)})
		pos += size
	}

	return le32(b, indexParent), es
}

// writeBlock writes an index block's entries. What was past them stays,
// as it does in VMS's blocks: an entry moved out by a split, or the last
// entry's old place after a deletion, is still there past the used count.
func (u *Updater) writeBlock(vbn, parent uint32, es []entry) {
	b := u.block(vbn)
	binary.LittleEndian.PutUint32(b[indexParent:], parent)

	flags, _ := u.layout()

	pos := indexEntries
	for _, e := range es {
		size := u.entrySize(e.key)
		if pos+size > blockSize {
			break
		}

		binary.LittleEndian.PutUint32(b[pos:], e.rfa.VBN)
		binary.LittleEndian.PutUint16(b[pos+4:], e.rfa.Offset)

		if flags&IndexASCII == 0 {
			clear(b[pos+rfaLength : pos+size])
			copy(b[pos+rfaLength:pos+size], e.key)
		} else {
			// A fixed-length entry's key is followed by zeros to its end.
			b[pos+rfaLength] = byte(len(e.key))
			clear(b[pos+keyOverhead : pos+size])
			copy(b[pos+keyOverhead:pos+size], e.key)
		}

		pos += size
	}

	binary.LittleEndian.PutUint16(b, uint16(pos-indexEntries))
}

// indexBlock takes a block for an index: the free list's first, or the
// next VBN.
func (u *Updater) indexBlock() uint32 {
	var v uint32

	if u.hdr(lhdFreIdxBlk) > 0 && u.hdr(lhdFreeIdx) != 0 {
		v = u.hdr(lhdFreeIdx)
		u.setHdr(lhdFreeIdx, le32(u.block(v), 0))
		u.addHdr(lhdFreIdxBlk, -1)
		clear(u.block(v))

		if v <= u.hdr(lhdHiPreAl) && v > u.hdr(lhdHiPrUsd) {
			u.setHdr(lhdHiPrUsd, v)
		}
	} else {
		v = u.newBlock()
	}

	u.addHdr(lhdIdxBlks, 1)

	return v
}

// index returns index ix's descriptor, 0 the first. It may be past the
// header's count of indexes: VMS 7.1's LBR$INSERT_KEY entered a key in
// index 1 of a library made with none (round 3's W15).
func (u *Updater) index(ix int) ([]byte, error) {
	if ix < 0 || lhdIdxDesc+(ix+1)*iddLength > blockSize {
		return nil, ErrIndexNumber
	}

	return u.data[lhdIdxDesc+ix*iddLength:][:iddLength], nil
}

// Key returns key as index ix enters it: up to its first blank, in
// uppercase unless the index keeps case.
func (u *Updater) Key(ix int, key string) ([]byte, error) {
	d, err := u.index(ix)
	if err != nil {
		return nil, err
	}

	k := []byte(key)
	if i := bytes.IndexByte(k, ' '); i >= 0 {
		k = k[:i]
	}

	if len(k) == 0 || len(k) > int(le16(d, 2)) {
		return nil, ErrInvalidKey
	}

	if le16(d, 0)&IndexNoCaseEnt == 0 {
		k = bytes.ToUpper(k)
	}

	return k, nil
}

// Binary reports whether index ix's keys are binary.
func (u *Updater) Binary(ix int) bool {
	d, err := u.index(ix)

	return err == nil && le16(d, 0)&IndexASCII == 0
}

// BinaryKey returns the binary key v as index ix enters it: its key
// length's bytes of v, little-endian.
func (u *Updater) BinaryKey(ix int, v uint32) ([]byte, error) {
	d, err := u.index(ix)
	if err != nil {
		return nil, err
	}

	k := binary.LittleEndian.AppendUint32(nil, v)
	n := int(le16(d, 2))

	if n <= len(k) {
		return k[:n], nil
	}

	return append(k, make([]byte, n-len(k))...), nil
}

// compare orders two keys as index flags compares them.
func compare(flags uint16, a, b []byte) int {
	if flags&IndexASCII == 0 {
		return compareBinary(a, b)
	}

	if flags&IndexUpcase != 0 {
		return bytes.Compare(bytes.ToUpper(a), bytes.ToUpper(b))
	}

	return bytes.Compare(a, b)
}

// compareBinary orders two binary keys of one length as the unsigned
// numbers they are, little-endian: from their last bytes.
func compareBinary(a, b []byte) int {
	for i := max(len(a), len(b)) - 1; i >= 0; i-- {
		var x, y byte
		if i < len(a) {
			x = a[i]
		}

		if i < len(b) {
			y = b[i]
		}

		if x != y {
			return int(x) - int(y)
		}
	}

	return 0
}

// path is the blocks from the root to the leaf where key goes, and in each
// upper block the entry followed.
type step struct {
	vbn uint32
	at  int
}

func (u *Updater) find(flags uint16, root uint32, key []byte) []step {
	var path []step

	for v := root; ; {
		_, es := u.readBlock(v)
		if len(es) == 0 || es[0].rfa.Offset != rfaIndex {
			return append(path, step{vbn: v})
		}

		at := len(es) - 1
		for i, e := range es {
			if compare(flags, e.key, key) >= 0 {
				at = i

				break
			}
		}

		path = append(path, step{vbn: v, at: at})
		v = es[at].rfa.VBN
	}
}

// Lookup returns where index ix's key points.
func (u *Updater) Lookup(ix int, key []byte) (RFA, bool) {
	d, err := u.index(ix)
	if err != nil || le32(d, 4) == 0 {
		return RFA{}, false
	}

	flags := le16(d, 0)
	path := u.find(flags, le32(d, 4), key)

	_, es := u.readBlock(path[len(path)-1].vbn)
	for _, e := range es {
		if compare(flags, e.key, key) == 0 {
			return e.rfa, true
		}
	}

	return RFA{}, false
}

// InsertKey enters key, as Key made it, in index ix, pointing at rfa, and
// counts it in the module's header. A key with a bad RFA is entered all
// the same, with the error.
func (u *Updater) InsertKey(ix int, key []byte, rfa RFA) error {
	if err := u.enter(ix, key, rfa); err != nil {
		return err
	}

	return u.refer(rfa, 1)
}

func (u *Updater) enter(ix int, key []byte, rfa RFA) error {
	d, err := u.index(ix)
	if err != nil {
		return err
	}

	if _, ok := u.Lookup(ix, key); ok {
		return ErrDuplicateKey
	}

	u.addHdr(lhdIdxCnt, 1)
	if ix == 0 {
		u.addHdr(lhdModCnt, 1)
	}

	root := le32(d, 4)
	if root == 0 {
		v := u.indexBlock()
		u.writeBlock(v, 0, []entry{{rfa: rfa, key: key}})
		u.setRoot(ix, v)

		return nil
	}

	path := u.find(le16(d, 0), root, key)
	u.add(ix, path, len(path)-1, entry{rfa: rfa, key: key})

	return nil
}

// setRoot makes vbn index ix's root. (The descriptor is found again each
// time: taking a block may move the file's bytes.)
func (u *Updater) setRoot(ix int, vbn uint32) {
	d, _ := u.index(ix)
	binary.LittleEndian.PutUint32(d[4:], vbn)
}

// add puts e in the block at path[level], splitting it if it doesn't fit.
func (u *Updater) add(ix int, path []step, level int, e entry) {
	d, _ := u.index(ix)
	flags := le16(d, 0)
	v := path[level].vbn
	parent, es := u.readBlock(v)

	pos := len(es)
	for i, x := range es {
		if compare(flags, x.key, e.key) > 0 {
			pos = i

			break
		}
	}

	es = append(es[:pos], append([]entry{e}, es[pos:]...)...)

	if u.entriesSize(es) <= indexFull {
		u.writeBlock(v, parent, es)

		if pos == len(es)-1 {
			u.raise(flags, path, level, e.key)
		}

		return
	}

	// The split: the old block keeps the entries up to the one that takes
	// it past its share of their bytes, as many as fit.
	total, kept, p := u.entriesSize(es), 0, 0
	for p < len(es)-1 && kept*10 < total*splitShare && kept+u.entrySize(es[p].key) <= indexFull {
		kept += u.entrySize(es[p].key)
		p++
	}

	p = max(p, 1)

	// The whole list goes into the block first, as VMS's insertion put it
	// there, and the split then shortens the block's count.
	u.writeBlock(v, parent, es)

	low, high := es[:p], append([]entry(nil), es[p:]...)
	nv := u.indexBlock()

	u.reparent(high, nv)

	if level == 0 {
		nr := u.indexBlock()
		u.writeBlock(v, nr, low)
		u.writeBlock(nv, nr, high)
		u.writeBlock(nr, 0, []entry{
			{rfa: RFA{VBN: v, Offset: rfaIndex}, key: low[len(low)-1].key},
			{rfa: RFA{VBN: nv, Offset: rfaIndex}, key: high[len(high)-1].key},
		})
		u.setRoot(ix, nr)
		u.addHdr(lhdIdxOvh, 2)

		return
	}

	u.writeBlock(v, parent, low)
	u.writeBlock(nv, parent, high)

	pp, pes := u.readBlock(parent)
	pes[path[level-1].at].key = low[len(low)-1].key
	u.writeBlock(parent, pp, pes)

	u.addHdr(lhdIdxOvh, 1)
	u.add(ix, path, level-1, entry{rfa: RFA{VBN: nv, Offset: rfaIndex}, key: high[len(high)-1].key})
}

// reparent makes vbn the parent of the blocks upper-level entries name.
func (u *Updater) reparent(es []entry, vbn uint32) {
	for _, e := range es {
		if e.rfa.Offset == rfaIndex {
			binary.LittleEndian.PutUint32(u.block(e.rfa.VBN)[indexParent:], vbn)
		}
	}
}

// raise makes key the highest key of the block at path[level] in the
// entries above it, when it's higher than they have.
func (u *Updater) raise(flags uint16, path []step, level int, key []byte) {
	for l := level - 1; l >= 0; l-- {
		pp, pes := u.readBlock(path[l].vbn)
		at := path[l].at

		if compare(flags, key, pes[at].key) <= 0 {
			return
		}

		pes[at].key = key
		u.writeBlock(path[l].vbn, pp, pes)

		if at != len(pes)-1 {
			return
		}
	}
}

// DeleteKey takes key out of index ix, and the module's count of it.
func (u *Updater) DeleteKey(ix int, key []byte) error {
	rfa, err := u.remove(ix, key)
	if err != nil {
		return err
	}

	_ = u.refer(rfa, -1)

	return nil
}

func (u *Updater) remove(ix int, key []byte) (RFA, error) {
	d, err := u.index(ix)
	if err != nil {
		return RFA{}, err
	}

	root := le32(d, 4)
	if root == 0 {
		return RFA{}, ErrKeyNotFound
	}

	flags := le16(d, 0)
	leaf := u.find(flags, root, key)
	v := leaf[len(leaf)-1].vbn
	parent, es := u.readBlock(v)

	for i, e := range es {
		if compare(flags, e.key, key) == 0 {
			u.writeBlock(v, parent, append(es[:i], es[i+1:]...))
			u.addHdr(lhdIdxCnt, -1)

			if ix == 0 {
				u.addHdr(lhdModCnt, -1)
			}

			return e.rfa, nil
		}
	}

	return RFA{}, ErrKeyNotFound
}

// ReplaceKey points index ix's key at rfa where it pointed at old, and
// moves the module's count of it; a key the index doesn't have is entered
// (round 3's W4, whose counts say a failed replacement had taken the old
// module's count first; unconfirmed beyond that).
func (u *Updater) ReplaceKey(ix int, key []byte, old, rfa RFA) error {
	at, ok := u.Lookup(ix, key)
	if !ok {
		return u.InsertKey(ix, key, rfa)
	}

	if at != old {
		return ErrInvalidRFA
	}

	if err := u.refer(old, -1); err != nil {
		return err
	}

	if _, _, _, err := u.record(rfa); err != nil {
		return err
	}

	if _, _, err := u.moduleHeader(rfa); err != nil {
		return err
	}

	u.repoint(ix, key, rfa)

	return u.refer(rfa, 1)
}

func (u *Updater) repoint(ix int, key []byte, rfa RFA) {
	d, _ := u.index(ix)
	flags := le16(d, 0)
	path := u.find(flags, le32(d, 4), key)
	v := path[len(path)-1].vbn
	parent, es := u.readBlock(v)

	for i := range es {
		if compare(flags, es[i].key, key) == 0 {
			es[i].rfa = rfa
		}
	}

	u.writeBlock(v, parent, es)
}

// ModuleHeader returns the bytes of the module header at rfa: its fixed
// part and the library's user data.
func (u *Updater) ModuleHeader(rfa RFA) ([]byte, error) {
	_, h, err := u.moduleHeader(rfa)

	return h, err
}

// SetUserData replaces the user data of the module header at rfa with
// data, cut or zero-filled to the library's size for it (LBR$SET_MODULE's
// update).
func (u *Updater) SetUserData(rfa RFA, data []byte) error {
	spans, h, err := u.moduleHeader(rfa)
	if err != nil {
		return err
	}

	user := h[mhdLen:]
	clear(user)
	copy(user, data)
	u.scatter(spans, h)

	return nil
}

// NoteRead tells the updater a reading routine read the module at rfa: a
// session's first record continues the last data block only if that
// block was the last read (round 3: W3's update, after LOOKUP_KEY and
// DELETE_DATA read its block, went on at the header's next RFA; W2's,
// after only GET_HEADER, started a new block; unconfirmed beyond that).
func (u *Updater) NoteRead(rfa RFA) {
	if rfa.VBN > 1 && rfa.VBN <= u.blocks() {
		u.lastRead = rfa.VBN
	}
}
