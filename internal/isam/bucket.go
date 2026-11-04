package isam

import (
	"encoding/binary"
)

// Offsets within a bucket's header (VMS's BKT$ names).
const (
	bktCheck    = 0x00 // BKT$B_CHECKCHAR: a count of the bucket's writes
	bktIndexNo  = 0x01 // BKT$B_INDEXNO: the key of reference
	bktSample   = 0x02 // BKT$W_ADRSAMPLE: the low word of the bucket's VBN
	bktFree     = 0x04 // BKT$W_FREESPACE: the first free byte
	bktNextID   = 0x06 // BKT$W_NXTRECID: the next record ID to give out
	bktNext     = 0x08 // BKT$L_NXTBKT: the next bucket at this level; a ring
	bktLevel    = 0x0C // BKT$B_LEVEL: 0 is the data level
	bktControl  = 0x0D // BKT$B_BKTCB
	bktOverhead = 0x0E // BKT$C_OVERHDSZ: where the records start

	bktLast      = 0x01 // BKT$M_LASTBKT: the last at its level
	bktRoot      = 0x02 // BKT$M_ROOTBKT
	bktPtrShift  = 3    // BKT$V_PTR_SZ: an index bucket's pointer size, less 2
	bktPtrMask   = 0x03
	bktEndOffset = 4 // BKT$C_ENDOVHD: what ends an index bucket
)

// A data record's control byte (VMS's IRC$ names).
const (
	ircPtrSize  = 0x03 // IRC$M_PTRSZ: the pointer's size, less 2
	ircDeleted  = 0x04 // IRC$M_DELETED
	ircRRV      = 0x08 // IRC$M_RRV
	ircNoPtr    = 0x10 // IRC$M_NOPTRSZ: no pointer follows
	ircFirstKey = 0x80 // IRC$M_FIRST_KEY: a SIDR's pointer
)

// bucket is one bucket as read: its header's fields and its bytes.
type bucket struct {
	vbn     uint32
	data    []byte
	check   uint8
	indexNo int
	sample  uint16
	free    int
	nextID  uint16
	next    uint32
	level   int
	control uint8
}

// isLast reports whether this is the last bucket at its level.
func (b *bucket) isLast() bool { return b.control&bktLast != 0 }

// pointerSize is an index bucket's pointer size: 2 bytes for every bucket
// seen so far.
func (b *bucket) pointerSize() int { return int(b.control>>bktPtrShift&bktPtrMask) + 2 }

// corrupt makes an error about this bucket.
func (b *bucket) corrupt(offset int, where string) error {
	return &CorruptError{VBN: b.vbn, Offset: offset, Where: where}
}

// readBucket reads the size blocks at vbn and checks what can be checked
// of any bucket: its first and last bytes match, and its VBN sample and
// free offset make sense.
func (f *File) readBucket(vbn uint32, size int) (*bucket, error) {
	if size <= 0 {
		return nil, &CorruptError{VBN: vbn, Where: "a bucket of no blocks"}
	}

	data := make([]byte, size*BlockSize)
	for i := range size {
		if err := f.blocks.ReadBlock(vbn+uint32(i), data[i*BlockSize:(i+1)*BlockSize]); err != nil {
			return nil, err
		}
	}

	b := &bucket{
		vbn:     vbn,
		data:    data,
		check:   data[bktCheck],
		indexNo: int(data[bktIndexNo]),
		sample:  binary.LittleEndian.Uint16(data[bktSample:]),
		free:    int(binary.LittleEndian.Uint16(data[bktFree:])),
		nextID:  binary.LittleEndian.Uint16(data[bktNextID:]),
		next:    binary.LittleEndian.Uint32(data[bktNext:]),
		level:   int(data[bktLevel]),
		control: data[bktControl],
	}

	switch {
	case data[len(data)-1] != b.check:
		return nil, b.corrupt(len(data)-1, "the check characters differ")
	case b.sample != uint16(vbn):
		return nil, b.corrupt(bktSample, "the VBN sample is wrong")
	case b.free < bktOverhead || b.free > len(data):
		return nil, b.corrupt(bktFree, "the free offset is out of the bucket")
	}

	return b, nil
}

// indexEntry is one entry of an index bucket: the highest key in the
// bucket a level down, and that bucket's VBN.
type indexEntry struct {
	offset  int
	key     []byte
	pointer uint32
}

// indexEntries decodes an index bucket. Keys run up from the header, and
// the pointers down from the bucket's end. The first key's pointer is
// nearest the end, with the bucket's last 4 bytes after it: a word holding
// the free offset below the pointers, a byte of 0, and the check
// character.
func (b *bucket) indexEntries(k *Key) ([]indexEntry, error) {
	var entries []indexEntry

	var previous []byte

	size := b.pointerSize()
	pointer := len(b.data) - bktEndOffset

	for at := bktOverhead; at < b.free; {
		key, used, err := b.readKey(k, at, b.free, previous, k.Flags&FlagIndexCompress != 0)
		if err != nil {
			return nil, err
		}

		pointer -= size
		if pointer < b.free {
			return nil, b.corrupt(pointer, "index pointers run into the keys")
		}

		entries = append(entries, indexEntry{
			offset:  at,
			key:     key,
			pointer: readPointer(b.data[pointer : pointer+size]),
		})

		previous = key
		at += used
	}

	return entries, nil
}

// readPointer reads a little-endian VBN of 2 to 5 bytes.
func readPointer(b []byte) uint32 {
	var v uint32

	for i := len(b) - 1; i >= 0; i-- {
		if i < 4 {
			v = v<<8 | uint32(b[i])
		}
	}

	return v
}

// readKey reads a key at an offset. With compression it's a byte of how
// many bytes are stored, a byte of how many leading bytes it shares with
// the previous key in the bucket, then the stored bytes, the last of them
// standing for itself repeated to the key's end: the Guide's front and
// rear compression. It returns the whole key and the bytes it took up.
func (b *bucket) readKey(k *Key, at, end int, previous []byte, compressed bool) ([]byte, int, error) {
	if !compressed {
		if at+k.Size > end {
			return nil, 0, b.corrupt(at, "a key past its end")
		}

		return b.data[at : at+k.Size : at+k.Size], k.Size, nil
	}

	if at+2 > end {
		return nil, 0, b.corrupt(at, "a compressed key past its end")
	}

	count, front := int(b.data[at]), int(b.data[at+1])
	if at+2+count > end || front > len(previous) || front+count > k.Size {
		return nil, 0, b.corrupt(at, "a compressed key's counts")
	}

	key := make([]byte, 0, k.Size)
	key = append(key, previous[:front]...)
	key = append(key, b.data[at+2:at+2+count]...)

	if len(key) == 0 {
		return nil, 0, b.corrupt(at, "a compressed key of no bytes")
	}

	last := key[len(key)-1]
	for len(key) < k.Size {
		key = append(key, last)
	}

	return key, 2 + count, nil
}

// dataRecord is one record of a primary key's data bucket: a live or
// deleted record, or an RRV forwarding one that a split moved.
type dataRecord struct {
	offset  int
	control uint8
	id      uint16
	rfa     RFA // for an RRV, where the record went
	key     []byte
	body    []byte // the bytes after the key, as stored: nil for an RRV
}

// isRRV reports whether this is a forwarding record.
func (r *dataRecord) isRRV() bool { return r.control&ircRRV != 0 }

// isDeleted reports whether this record was deleted.
func (r *dataRecord) isDeleted() bool { return r.control&ircDeleted != 0 }

// fixedStorage reports whether a data record leaves out its size: a fixed
// format file with nothing compressed, every record the same length.
func (f *File) fixedStorage(k *Key) bool {
	return f.attr.Format == Fixed && k.Flags&(FlagKeyCompress|FlagRecordCompress) == 0
}

// dataRecords decodes a primary key's data bucket. Every record is
// control, ID, and the RFA's ID and VBN; then, but in a fixed format file
// with nothing compressed, a size word; then the key and the rest. An RRV
// stops after the VBN. Each compressed key depends on the one before, so
// a bucket can only be read from its start.
func (f *File) dataRecords(b *bucket, k *Key) ([]dataRecord, error) {
	var records []dataRecord

	var previous []byte

	for at := bktOverhead; at < b.free; {
		r, next, err := f.dataRecord(b, k, at, previous)
		if err != nil {
			return nil, err
		}

		if !r.isRRV() {
			previous = r.key
		}

		records = append(records, r)
		at = next
	}

	return records, nil
}

// dataRecord decodes the record at an offset, returning the next offset.
func (f *File) dataRecord(b *bucket, k *Key, at int, previous []byte) (dataRecord, int, error) {
	if at+3 > b.free {
		return dataRecord{}, 0, b.corrupt(at, "a record's header past the free offset")
	}

	r := dataRecord{
		offset:  at,
		control: b.data[at],
		id:      binary.LittleEndian.Uint16(b.data[at+1:]),
	}

	cursor := at + 3

	if r.control&ircNoPtr != 0 {
		r.rfa = RFA{VBN: b.vbn, ID: r.id}
	} else {
		size := int(r.control&ircPtrSize) + 2
		if cursor+2+size > b.free {
			return dataRecord{}, 0, b.corrupt(at, "a record's pointer past the free offset")
		}

		r.rfa = RFA{
			ID:  binary.LittleEndian.Uint16(b.data[cursor:]),
			VBN: readPointer(b.data[cursor+2 : cursor+2+size]),
		}
		cursor += 2 + size
	}

	if r.isRRV() {
		return r, cursor, nil
	}

	end := 0

	if f.fixedStorage(k) {
		// A deleted record's stub is only its key, size word or none (G_G109).
		end = cursor + f.attr.Size
		if r.isDeleted() {
			end = cursor + k.Size
		}
	} else {
		if cursor+2 > b.free {
			return dataRecord{}, 0, b.corrupt(at, "a record's size past the free offset")
		}

		end = cursor + 2 + int(binary.LittleEndian.Uint16(b.data[cursor:]))
		cursor += 2
	}

	if end > b.free {
		return dataRecord{}, 0, b.corrupt(at, "a record past the free offset")
	}

	key, used, err := b.readKey(k, cursor, end, previous, k.Flags&FlagKeyCompress != 0)
	if err != nil {
		return dataRecord{}, 0, err
	}

	r.key = key
	r.body = b.data[cursor+used : end]

	return r, end, nil
}

// expand turns a record's stored bytes after the key back into what they
// were: with data compression they're fragments, each a word's length,
// that many bytes, and a byte of how many more times the last of them
// repeats.
func (r *dataRecord) expand(k *Key, b *bucket) ([]byte, error) {
	if k.Flags&FlagRecordCompress == 0 {
		return r.body, nil
	}

	out := make([]byte, 0, len(r.body))

	for at := 0; at < len(r.body); {
		if at+2 > len(r.body) {
			return nil, b.corrupt(r.offset, "a fragment's length past the record")
		}

		count := int(binary.LittleEndian.Uint16(r.body[at:]))
		if at+2+count+1 > len(r.body) {
			return nil, b.corrupt(r.offset, "a fragment past the record")
		}

		fragment := r.body[at+2 : at+2+count]
		out = append(out, fragment...)

		if repeat := int(r.body[at+2+count]); repeat > 0 && count > 0 {
			last := fragment[count-1]
			for range repeat {
				out = append(out, last)
			}
		}

		at += 2 + count + 1
	}

	return out, nil
}

// sidr is one secondary index data record: a key value, and pointers to
// the records having it.
type sidr struct {
	offset   int
	key      []byte
	pointers []sidrPointer
}

// sidrPointer is one pointer in a SIDR: where it is in the bucket, its
// control byte, and the RFA it holds.
type sidrPointer struct {
	offset  int
	control uint8
	rfa     RFA
}

// isDeleted reports whether the pointer's record was deleted, the pointer
// left behind (ROP FDL).
func (p sidrPointer) isDeleted() bool { return p.control&ircDeleted != 0 }

// sidrs decodes an alternate key's level 0 bucket. Each SIDR is a word's
// size, the key, and pointers to the end of that size, each a control
// byte, an ID, and a VBN.
func (f *File) sidrs(b *bucket, k *Key) ([]sidr, error) {
	var records []sidr

	var previous []byte

	for at := bktOverhead; at < b.free; {
		if at+2 > b.free {
			return nil, b.corrupt(at, "a SIDR's size past the free offset")
		}

		end := at + 2 + int(binary.LittleEndian.Uint16(b.data[at:]))
		if end > b.free {
			return nil, b.corrupt(at, "a SIDR past the free offset")
		}

		key, used, err := b.readKey(k, at+2, end, previous, k.Flags&FlagKeyCompress != 0)
		if err != nil {
			return nil, err
		}

		s := sidr{offset: at, key: key}

		for cursor := at + 2 + used; cursor < end; {
			// A pointer taken out from the middle of an array is its control byte
			// alone, IRC$M_DELETED and IRC$M_NOPTRSZ (testdata/vms88's changed files).
			p := sidrPointer{offset: cursor, control: b.data[cursor]}
			if p.control&ircNoPtr != 0 {
				cursor++

				s.pointers = append(s.pointers, p)

				continue
			}

			if cursor+3 > end {
				return nil, b.corrupt(cursor, "a SIDR pointer past its record")
			}

			id := binary.LittleEndian.Uint16(b.data[cursor+1:])
			cursor += 3

			size := int(p.control&ircPtrSize) + 2
			if cursor+size > end {
				return nil, b.corrupt(cursor, "a SIDR pointer past its record")
			}

			p.rfa = RFA{VBN: readPointer(b.data[cursor : cursor+size]), ID: id}
			cursor += size

			s.pointers = append(s.pointers, p)
		}

		previous = key
		records = append(records, s)
		at = end
	}

	return records, nil
}
