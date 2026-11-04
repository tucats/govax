package isam

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// This file changes and takes out records that are already in a file: RMS's
// $UPDATE and $DELETE (subtask 6). Its rules are read from the probe's
// "changing" cases (testdata/idx88, cases 138 to 149, files U128 to U139),
// whose dumps the tests replay; the phase doc's "What update and delete
// settled" says which are VMS's and which are only reasonable choices.
//
// In outline: an updated record is rewritten where it is, keeping its ID and
// RFA, and only the alternates whose values changed are touched, the old
// pointer taken out and a new one put last among the new value's. A record too
// long for its bucket now splits the bucket as a new one would. A deleted
// record's bytes close up, and if it lived away from the bucket its RFA names,
// the RRV there goes too; but the last record left in a bucket stays as a stub,
// its key and nothing more, marked deleted.

// ErrKeyChanged is an update that changes a key that can't change: the primary
// key, or an alternate without FlagChanges, which includes a record made too
// short to hold it (RMS$_CHG).
var ErrKeyChanged = errors.New("isam: key changed")

// place is a record found by its RFA: its bucket as read, the bucket's records,
// and which of them it is.
type place struct {
	b    *bucket
	list []dataRecord
	at   int
}

// record is the place's record.
func (p *place) record() *dataRecord { return &p.list[p.at] }

// locate finds where an RFA's record is, following an RRV if a split moved it,
// as Record does. ErrDeleted says it's gone.
func (f *File) locate(rfa RFA) (*place, error) {
	k := f.Keys[0]

	for range 8 {
		b, err := f.readBucket(rfa.VBN, k.DataBucketSize)
		if err != nil {
			return nil, err
		}

		list, err := f.dataRecords(b, k)
		if err != nil {
			return nil, err
		}

		at := -1

		for i := range list {
			if list[i].id == rfa.ID {
				at = i

				break
			}
		}

		switch {
		case at < 0, list[at].isDeleted():
			return nil, ErrDeleted
		case list[at].isRRV():
			rfa = list[at].rfa

			continue
		}

		return &place{b: b, list: list, at: at}, nil
	}

	return nil, &CorruptError{VBN: rfa.VBN, Where: "a chain of RRVs that doesn't end"}
}

// contents is the place's whole record, its key put back where it belongs.
func (f *File) contents(p *place) ([]byte, error) {
	k := f.Keys[0]
	r := p.record()

	rest, err := r.expand(k, p.b)
	if err != nil {
		return nil, err
	}

	return k.assemble(r.key, rest), nil
}

// keyChange is what an update does to one alternate key: its value before and
// after, and whether the record was and will be in its index at all.
type keyChange struct {
	k               *Key
	before, after   []byte
	wasIn, willBeIn bool
}

// changes lists the alternates an update changes, refusing it if one can't
// change. A record leaving an index (too short now, or null) or entering one
// changes that key as surely as a new value does: U131's record made too short
// for a key without CHG was RMS$_CHG, though it was long enough for the key that
// had CHG.
func (f *File) changes(old, record []byte) ([]keyChange, error) {
	var list []keyChange

	for _, k := range f.Keys[1:] {
		c := keyChange{k: k}
		c.before, c.wasIn = k.Indexes(old)
		c.after, c.willBeIn = k.Indexes(record)

		if c.wasIn == c.willBeIn && (!c.wasIn || bytes.Equal(c.before, c.after)) {
			continue
		}

		if k.Flags&FlagChanges == 0 {
			return nil, ErrKeyChanged
		}

		list = append(list, c)
	}

	return list, nil
}

// Update replaces the record an RFA names with another of the same primary key,
// as RMS's $UPDATE does: in place, its RFA kept. Its length may change; one too
// long for its bucket splits the bucket (U137). Errors: ErrDeleted if the RFA's
// record is gone, ErrRecordSize for a record too short for the primary key or
// too long for any bucket, ErrKeyChanged for a key that can't change, and
// ErrDuplicate for a new alternate value its key won't have twice, each before
// anything is written. A new value its key has already is the result's Duplicate,
// RMS$_OK_DUP.
func (f *File) Update(rfa RFA, record []byte) (PutResult, error) {
	if f.out == nil {
		return PutResult{}, ErrReadOnly
	}

	f.loading = false
	k := f.Keys[0]

	p, err := f.locate(rfa)
	if err != nil {
		return PutResult{}, err
	}

	old, err := f.contents(p)
	if err != nil {
		return PutResult{}, err
	}

	value, ok := k.Value(record)
	if !ok || f.attr.Format == Fixed && len(record) != f.attr.Size {
		return PutResult{}, ErrRecordSize
	}

	if !bytes.Equal(value, p.record().key) {
		return PutResult{}, ErrKeyChanged
	}

	changed, err := f.changes(old, record)
	if err != nil {
		return PutResult{}, err
	}

	for _, c := range changed {
		if !c.willBeIn || c.k.Flags&FlagDuplicates != 0 {
			continue
		}

		switch held, err := f.valueHeld(c.k, c.after); {
		case err != nil:
			return PutResult{}, err
		case held:
			return PutResult{}, ErrDuplicate
		}
	}

	rest, err := f.encodeData(k, value, record)
	if err != nil {
		return PutResult{}, err
	}

	split, err := f.rewrite(p, k, rest)
	if err != nil {
		return PutResult{}, err
	}

	result := PutResult{RFA: rfa, Split: split}

	// The new pointer goes in before the old one comes out: U130's stale bytes
	// are its new SIDR's, moved down over the old one's place.
	for _, c := range changed {
		if c.willBeIn {
			dup, err := f.insertAlternate(c.k, record, rfa)
			if err != nil {
				return result, err
			}

			result.Duplicate = result.Duplicate || dup
		}

		if c.wasIn {
			if err := f.removePointer(c.k, c.before, rfa); err != nil {
				return result, err
			}
		}
	}

	return result, nil
}

// rewrite puts a record's new bytes after its key where the old ones were. The
// key itself doesn't change, so the stored key, compressed against the record
// before, is the same, and so is the next record's. It reports whether the
// bucket had to split for them.
func (f *File) rewrite(p *place, k *Key, rest []byte) (bool, error) {
	b, r := p.b, p.record()

	var previous []byte

	for i := range p.at {
		if !p.list[i].isRRV() {
			previous = p.list[i].key
		}
	}

	stored := f.encodeRecord(k, r.id, r.rfa, r.key, previous, rest)
	stored[0] = r.control

	length := p.end(b) - r.offset
	if !b.room(len(stored) - length) {
		return true, f.splitForUpdate(b, k, p.list, p.at, rest)
	}

	b.replace(r.offset, length, stored)
	b.touch()

	return false, f.writeBucket(b)
}

// end is where the place's record ends: the next record, or the free offset.
func (p *place) end(b *bucket) int {
	if p.at+1 < len(p.list) {
		return p.list[p.at+1].offset
	}

	return b.free
}

// Delete takes out the record an RFA names, as RMS's $DELETE does, and its
// pointers in each alternate index unless fast, RMS's ROP FDL: then they're left
// for a later search to find and take out with DropPointer (the Guide's 8.2.4).
// ErrDeleted says the record was already gone.
//
// The record's bytes close up, the next record's key stored again against the one
// before. The last record left in a bucket is kept as a stub instead, marked
// deleted, with its key and none of its data: U139's buckets each kept the last
// record deleted from them, and U137's bucket the one record it held. A record a
// split moved away from the bucket its RFA names had an RRV there, which goes
// (U137), so the RFA's ID is simply missing, which is RMS$_DEL.
func (f *File) Delete(rfa RFA, fast bool) error {
	if f.out == nil {
		return ErrReadOnly
	}

	f.loading = false
	k := f.Keys[0]

	p, err := f.locate(rfa)
	if err != nil {
		return err
	}

	record, err := f.contents(p)
	if err != nil {
		return err
	}

	r := p.record()
	moved := r.rfa.VBN != p.b.vbn

	if f.lastLeft(p, k) {
		f.makeStub(p, k)
	} else {
		f.closeUp(p.b, k, p.list, p.at)
	}

	p.b.touch()

	if err := f.writeBucket(p.b); err != nil {
		return err
	}

	if moved {
		if err := f.removeRRV(k, rfa); err != nil {
			return err
		}
	}

	if fast {
		return nil
	}

	for _, k := range f.Keys[1:] {
		if value, belongs := k.Indexes(record); belongs {
			if err := f.removePointer(k, value, rfa); err != nil {
				return err
			}
		}
	}

	return nil
}

// lastLeft says whether the place's record is the bucket's last by key, whatever else
// is before it: the index's entry for the bucket has that key, so VMS keeps a stub of
// it (testdata/vms88's changed files, G_F120's key 51 with three records before it).
// Later records with its own key don't count (G_S31's duplicates).
func (f *File) lastLeft(p *place, k *Key) bool {
	for i := p.at + 1; i < len(p.list); i++ {
		if r := &p.list[i]; !r.isRRV() && k.Compare(r.key, p.record().key) != 0 {
			return false
		}
	}

	return true
}

// makeStub turns a record into a deleted one's stub: its control byte with
// IRC$M_DELETED, its ID and RFA, a size word if the file's records have one, and its
// key (U137, U139; testdata/vms88's G_G109 for a fixed file with none).
func (f *File) makeStub(p *place, k *Key) {
	b, r := p.b, p.record()

	var previous []byte

	for i := range p.at {
		if !p.list[i].isRRV() {
			previous = p.list[i].key
		}
	}

	stored := f.encodeRecord(k, r.id, r.rfa, r.key, previous, nil)
	stored[0] = r.control | ircDeleted

	b.replace(r.offset, p.end(b)-r.offset, stored)
}

// closeUp takes a record's bytes out of a bucket in memory, then stores the next
// record's key again against the one before, if keys are compressed. That order
// is a SIDR's (U136's stale bytes show it), and taken to be a data record's too,
// unconfirmed: no VMS case deletes a record before one whose key then grows. Its
// ID stays spent, and the old last bytes stay past the free offset, as VMS
// leaves them.
func (f *File) closeUp(b *bucket, k *Key, list []dataRecord, i int) {
	r := &list[i]

	end := b.free
	if i+1 < len(list) {
		end = list[i+1].offset
	}

	var previous []byte

	for j := range i {
		if !list[j].isRRV() {
			previous = list[j].key
		}
	}

	// The next record with a key: RRVs can come between, a record put past the
	// last going after them.
	next := -1

	for j := i + 1; j < len(list) && k.Flags&FlagKeyCompress != 0; j++ {
		if !list[j].isRRV() {
			next = j

			break
		}
	}

	keyAt, old := 0, 0
	if next >= 0 {
		keyAt, old = f.dataKeyPlace(b, k, &list[next])
	}

	b.replace(r.offset, end-r.offset, nil)

	if next >= 0 {
		f.restoreDataKey(b, keyAt-(end-r.offset), old, compressKey(list[next].key, previous))
	}
}

// removeRecord closes up a record just put in a data bucket, for Put's backing
// out (S41): no stub, the record having been there only a moment.
func (f *File) removeRecord(k *Key, rfa RFA) error {
	b, err := f.readBucket(rfa.VBN, k.DataBucketSize)
	if err != nil {
		return err
	}

	list, err := f.dataRecords(b, k)
	if err != nil {
		return err
	}

	for i := range list {
		if !list[i].isRRV() && list[i].rfa == rfa {
			f.closeUp(b, k, list, i)
			b.touch()

			return f.writeBucket(b)
		}
	}

	return nil
}

// removeRRV takes the RRV for an RFA out of the bucket the RFA names.
func (f *File) removeRRV(k *Key, rfa RFA) error {
	b, err := f.readBucket(rfa.VBN, k.DataBucketSize)
	if err != nil {
		return err
	}

	list, err := f.dataRecords(b, k)
	if err != nil {
		return err
	}

	for i := range list {
		r := &list[i]
		if !r.isRRV() || r.id != rfa.ID {
			continue
		}

		end := b.free
		if i+1 < len(list) {
			end = list[i+1].offset
		}

		b.replace(r.offset, end-r.offset, nil)
		b.touch()

		return f.writeBucket(b)
	}

	return b.corrupt(bktOverhead, "a moved record with no RRV where its RFA says")
}

// valueHeld says whether a live record has a value of an alternate key. A
// pointer whose record is gone, a fast delete's, doesn't count: the Guide's
// 8.2.4 has RMS follow such a pointer and find no record there.
func (f *File) valueHeld(k *Key, value []byte) (bool, error) {
	if !k.HasIndex() {
		return false, nil
	}

	vbn, err := f.descend(k, value)
	if err != nil {
		return false, err
	}

	held := false

	var readErr error

	err = f.scanFrom(k, vbn, func(e Entry) bool {
		switch c := k.Compare(e.Key, value); {
		case c < 0:
			return true
		case c > 0:
			return false
		}

		if _, err := f.Record(e.RFA); !errors.Is(err, ErrDeleted) {
			held, readErr = err == nil, err

			return false
		}

		return true
	})
	if err == nil {
		err = readErr
	}

	return held, err
}

// DropPointer takes an entry's pointer out of its alternate index: RMS's part
// when a search by an alternate key finds a pointer whose record is gone, which a
// fast delete leaves. U134's record, deleted with ROP FDL, was RMS$_RNF by each
// alternate key afterwards, and neither index had its pointer then.
func (f *File) DropPointer(e Entry) error {
	if f.out == nil {
		return ErrReadOnly
	}

	k, err := f.key(e.Ref)
	if err != nil {
		return err
	}

	if e.Ref == 0 {
		return ErrKeyRef
	}

	return f.removePointer(k, e.Key, e.RFA)
}

// removePointer takes a record's pointer out of its value's SIDR, which may be
// in any bucket of a value that goes on past one, and the SIDR out if that was
// its only pointer. ErrNotFound says there's no such pointer.
func (f *File) removePointer(k *Key, value []byte, rfa RFA) error {
	if !k.HasIndex() {
		return ErrNotFound
	}

	vbn, err := f.descend(k, value)
	if err != nil {
		return err
	}

	for {
		b, err := f.readBucket(vbn, k.DataBucketSize)
		if err != nil {
			return err
		}

		list, err := f.sidrs(b, k)
		if err != nil {
			return err
		}

		for i := range list {
			s := &list[i]

			switch c := k.Compare(s.key, value); {
			case c > 0:
				return ErrNotFound
			case c < 0:
				continue
			}

			for j := range s.pointers {
				if s.pointers[j].rfa == rfa {
					return f.removeFromSIDR(b, k, list, i, j)
				}
			}
		}

		if b.isLast() {
			return ErrNotFound
		}

		vbn = b.next
	}
}

// removeFromSIDR takes pointer j out of SIDR i and writes the bucket: by where it is
// in the array, as VMS's changed files have it (testdata/vms88). A SIDR left with none
// goes whole; if that leaves the bucket empty, it stays empty, which no VMS case shows.
func (f *File) removeFromSIDR(b *bucket, k *Key, list []sidr, i, j int) error {
	s := &list[i]

	live := 0

	for _, p := range s.pointers {
		if !p.isDeleted() {
			live++
		}
	}

	switch {
	case live <= 1 && i+1 == len(list):
		// The bucket's last value stays, as its last record does: the index's entry
		// for the bucket has it (D2, U134, L140). Its first pointer holds the place,
		// until a value goes in after it (insertSIDR).
		b.data[s.pointers[0].offset] |= ircDeleted

		if j != 0 {
			f.leavePointerByte(b, s, j)
		}

	case live <= 1:
		// The last one left: whatever deleted pointers it has go with it
		// (unconfirmed, no case having deleted the last after the first).
		f.removeSIDR(b, k, list, i)

	case j == 0:
		// A value's first pointer stays, with others after it, marked deleted
		// and the array its size (testdata/for88's D1, D2, and R1).
		b.data[s.pointers[j].offset] |= ircDeleted

	default:
		f.leavePointerByte(b, s, j)
	}

	b.touch()

	return f.writeBucket(b)
}

// leavePointerByte takes a pointer that isn't an array's first down to its control
// byte, IRC$M_DELETED and IRC$M_NOPTRSZ (testdata/vms88's R_PHONES, U137).
func (f *File) leavePointerByte(b *bucket, s *sidr, j int) {
	p := s.pointers[j]
	whole := int(binary.LittleEndian.Uint16(b.data[s.offset:]))

	end := s.offset + 2 + whole
	if j+1 < len(s.pointers) {
		end = s.pointers[j+1].offset
	}

	gone := end - p.offset - 1

	b.data[p.offset] = ircDeleted | ircNoPtr
	binary.LittleEndian.PutUint16(b.data[s.offset:], uint16(whole-gone))
	b.replace(p.offset+1, gone, nil)
}

// removeSIDR closes up a whole SIDR, then stores the next one's key again
// against the one before: in that order, U136's stale bytes say.
func (f *File) removeSIDR(b *bucket, k *Key, list []sidr, i int) {
	s := &list[i]

	end := b.free
	if i+1 < len(list) {
		end = list[i+1].offset
	}

	old := 0

	again := i+1 < len(list) && k.Flags&FlagKeyCompress != 0
	if again {
		old = 2 + int(b.data[list[i+1].offset+2])
	}

	b.replace(s.offset, end-s.offset, nil)

	if again {
		var previous []byte
		if i > 0 {
			previous = list[i-1].key
		}

		restoreSIDRKey(b, s.offset, old, compressKey(list[i+1].key, previous))
	}
}
