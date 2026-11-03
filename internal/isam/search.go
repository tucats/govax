package isam

import (
	"errors"
)

// RFA is a record's file address: the bucket it was first written into,
// and its ID there. It stays the same for the record's whole life.
type RFA struct {
	VBN uint32
	ID  uint16
}

// Match is how a search value picks a record, by the key's own order: a
// descending key's "next" is a lower value. These are RMS's ROP bits,
// KGE and KGT, with REV, which turns "next" into "previous" (the RMS
// manual's Tables 7-3 and 7-4). Which combinations RMS refuses is
// internal/rms's to say.
type Match int

const (
	// Equal finds the first record with the value.
	Equal Match = iota

	// EqualOrNext finds that, or else the first record past it (KGE).
	EqualOrNext

	// Next finds the first record past the value (KGT).
	Next

	// EqualOrPrevious finds the first record with the value, or else the
	// nearest record before it; of records sharing a key, the first (REV
	// and KGE).
	EqualOrPrevious

	// Previous finds the nearest record before the value; of records
	// sharing a value, the first (REV and KGT).
	Previous
)

// Entry is one place in a key's order: the record's key value and RFA,
// the key of reference it was found by. It's all a sequence needs to go
// on from: nothing in it names a bucket, so a split can't lose it.
type Entry struct {
	Ref int
	Key []byte
	RFA RFA
}

// levelZero is a key's level 0 bucket as read: its records if the key is
// the primary key, its SIDRs if not.
type levelZero struct {
	bucket  *bucket
	primary []dataRecord
	alts    []sidr
}

// readLevelZero reads and decodes a key's level 0 bucket.
func (f *File) readLevelZero(k *Key, vbn uint32) (*levelZero, error) {
	b, err := f.readBucket(vbn, k.DataBucketSize)
	if err != nil {
		return nil, err
	}

	if b.level != 0 {
		return nil, b.corrupt(bktLevel, "a data bucket that isn't level 0")
	}

	z := &levelZero{bucket: b}

	if k.Ref == 0 {
		z.primary, err = f.dataRecords(b, k)
	} else {
		z.alts, err = f.sidrs(b, k)
	}

	if err != nil {
		return nil, err
	}

	return z, nil
}

// entries lists a level 0 bucket's live places, in order: the records
// that aren't deleted or RRVs, or each SIDR's live pointers.
func (z *levelZero) entries(k *Key) []Entry {
	var list []Entry

	for i := range z.primary {
		r := &z.primary[i]
		if r.isRRV() || r.isDeleted() {
			continue
		}

		list = append(list, Entry{Ref: k.Ref, Key: r.key, RFA: r.rfa})
	}

	for i := range z.alts {
		s := &z.alts[i]

		for _, p := range s.pointers {
			if p.isDeleted() {
				continue
			}

			list = append(list, Entry{Ref: k.Ref, Key: s.key, RFA: p.rfa})
		}
	}

	return list
}

// descend goes from a key's root down to the level 0 bucket that may hold
// the first entry for a search value: at each level, the first entry
// whose high key isn't before the value. The last entry at a level has
// the highest key of all, written as FF bytes, which isn't an ordinary
// value of a signed or descending key: it's passed to, not compared.
func (f *File) descend(k *Key, value []byte) (uint32, error) {
	vbn := k.RootVBN
	size := k.IndexBucketSize

	for level := k.RootLevel; level > 0; level-- {
		b, err := f.readBucket(vbn, size)
		if err != nil {
			return 0, err
		}

		if b.level != level {
			return 0, b.corrupt(bktLevel, "an index bucket at the wrong level")
		}

		list, err := b.indexEntries(k)
		if err != nil {
			return 0, err
		}

		next, found := uint32(0), false

		for i, e := range list {
			if i == len(list)-1 && b.isLast() {
				next, found = e.pointer, true

				break
			}

			if k.comparePrefix(e.key, value) >= 0 {
				next, found = e.pointer, true

				break
			}
		}

		switch {
		case found:
			vbn = next
		case b.isLast():
			return 0, b.corrupt(bktOverhead, "an index bucket with no entries")
		default:
			// Every key here is lower: go on along this level.
			vbn = b.next
			level++
		}
	}

	return vbn, nil
}

// scanFrom calls visit with each live entry of a key from a bucket on, in
// order, until visit returns false or the level ends.
func (f *File) scanFrom(k *Key, vbn uint32, visit func(Entry) bool) error {
	for {
		z, err := f.readLevelZero(k, vbn)
		if err != nil {
			return err
		}

		for _, e := range z.entries(k) {
			if !visit(e) {
				return nil
			}
		}

		if z.bucket.isLast() {
			return nil
		}

		vbn = z.bucket.next
	}
}

// Walk calls visit with every record's entry in a key's order, stopping
// if visit returns false.
func (f *File) Walk(ref int, visit func(Entry) bool) error {
	k, err := f.key(ref)
	if err != nil {
		return err
	}

	if !k.HasIndex() {
		return nil
	}

	return f.scanFrom(k, k.FirstData, visit)
}

// First returns a key's first entry, or ErrEnd if its index is empty.
func (f *File) First(ref int) (Entry, error) {
	var first Entry

	found := false

	err := f.Walk(ref, func(e Entry) bool {
		first, found = e, true

		return false
	})

	switch {
	case err != nil:
		return Entry{}, err
	case !found:
		return Entry{}, ErrEnd
	}

	return first, nil
}

// Following returns the entry after one in its key's order, or ErrEnd.
// It looks the entry up again by its key and RFA, so what else happened
// to the file in between doesn't matter. If the entry itself has gone,
// the next is the first entry past its key.
func (f *File) Following(at Entry) (Entry, error) {
	k, err := f.key(at.Ref)
	if err != nil {
		return Entry{}, err
	}

	if !k.HasIndex() {
		return Entry{}, ErrEnd
	}

	vbn, err := f.descend(k, at.Key)
	if err != nil {
		return Entry{}, err
	}

	var next Entry

	found, passed := false, false

	err = f.scanFrom(k, vbn, func(e Entry) bool {
		c := k.Compare(e.Key, at.Key)

		switch {
		case c < 0:
			return true
		case c > 0 || passed:
			next, found = e, true

			return false
		case e.RFA == at.RFA:
			passed = true
		}

		return true
	})

	switch {
	case err != nil:
		return Entry{}, err
	case !found:
		return Entry{}, ErrEnd
	}

	return next, nil
}

// Find searches a key for a value, which for a string key may be shorter
// than the key: a generic search, which only that many leading bytes
// compare. It returns ErrNotFound if no record answers, and ErrKeySize
// for a value of a size the key can't take.
func (f *File) Find(ref int, value []byte, match Match) (Entry, error) {
	k, err := f.key(ref)
	if err != nil {
		return Entry{}, err
	}

	if err := k.checkValue(value); err != nil {
		return Entry{}, err
	}

	if !k.HasIndex() {
		return Entry{}, ErrNotFound
	}

	if match == EqualOrPrevious || match == Previous {
		return f.findBackward(k, value, match)
	}

	start, err := f.descend(k, value)
	if err != nil {
		return Entry{}, err
	}

	var result Entry

	found := false

	err = f.scanFrom(k, start, func(e Entry) bool {
		c := k.comparePrefix(e.Key, value)

		switch {
		case c < 0, c == 0 && match == Next:
			return true
		case c == 0, match != Equal:
			result, found = e, true
		}

		return false
	})

	switch {
	case err != nil:
		return Entry{}, err
	case !found:
		return Entry{}, ErrNotFound
	}

	return result, nil
}

// findBackward does the two REV searches. EqualOrPrevious takes the first
// record answering the value, if there is one: VMS's run gave P0000100 for
// a generic P00001, the first of the set and not its last. Otherwise, and
// for Previous, it's the last entry before the value, and of the entries
// sharing that entry's whole key, the first (the manual's "a duplicate set
// found by REV gives only its first record"; no VMS case has such a set,
// so that part is the manual's alone). Level 0 is chained only forward,
// so it's read from its start.
func (f *File) findBackward(k *Key, value []byte, match Match) (Entry, error) {
	if match == EqualOrPrevious {
		e, err := f.Find(k.Ref, value, Equal)
		if !errors.Is(err, ErrNotFound) {
			return e, err
		}
	}

	var last Entry

	found := false

	err := f.scanFrom(k, k.FirstData, func(e Entry) bool {
		if k.comparePrefix(e.Key, value) >= 0 {
			return false
		}

		last, found = e, true

		return true
	})
	if err != nil {
		return Entry{}, err
	}

	if !found {
		return Entry{}, ErrNotFound
	}

	return f.Find(k.Ref, last.Key, Equal)
}

// Record reads the record an RFA names, following an RRV if a split moved
// it. ErrDeleted says it's gone: the record is marked deleted, or the
// bucket no longer has that ID.
func (f *File) Record(rfa RFA) ([]byte, error) {
	k, err := f.key(0)
	if err != nil {
		return nil, err
	}

	// An RRV always names the record's present place, but a chain is no
	// great cost to allow, and a loop must not hang.
	for range 8 {
		b, err := f.readBucket(rfa.VBN, k.DataBucketSize)
		if err != nil {
			return nil, err
		}

		list, err := f.dataRecords(b, k)
		if err != nil {
			return nil, err
		}

		var r *dataRecord

		for i := range list {
			if list[i].id == rfa.ID {
				r = &list[i]

				break
			}
		}

		switch {
		case r == nil, r.isDeleted():
			return nil, ErrDeleted
		case r.isRRV():
			rfa = r.rfa

			continue
		}

		rest, err := r.expand(k, b)
		if err != nil {
			return nil, err
		}

		return k.assemble(r.key, rest), nil
	}

	return nil, &CorruptError{VBN: rfa.VBN, Where: "a chain of RRVs that doesn't end"}
}
