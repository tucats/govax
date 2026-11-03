package isam

import "fmt"

// Check walks a whole file and reports the first thing wrong with it: what no
// operation may leave, whatever else it did. For every key, each level is a ring
// of buckets with the last flag at its end, and its keys are in order; each index
// entry names a bucket a level down whose keys aren't above the entry's. For the
// primary key, RFAs are unique, each RRV forwards to the record naming it, and
// each record can be found by its RFA. For an alternate, each pointer leads to a
// record with that key's value, and each record that belongs in the index has one.
func (f *File) Check() error {
	records, err := f.checkPrimary()
	if err != nil {
		return err
	}

	for _, k := range f.Keys {
		if k.HasIndex() {
			if err := f.checkTree(k); err != nil {
				return fmt.Errorf("key %d: %w", k.Ref, err)
			}
		}
	}

	for _, k := range f.Keys[1:] {
		if err := f.checkAlternate(k, records); err != nil {
			return fmt.Errorf("key %d: %w", k.Ref, err)
		}
	}

	return nil
}

// levelRing reads one level of a key's tree from its first bucket, checking the
// ring and each bucket's header. Level 0 buckets are the data bucket size.
func (f *File) levelRing(k *Key, first uint32, level int) ([]*bucket, error) {
	size := k.IndexBucketSize
	if level == 0 {
		size = k.DataBucketSize
	}

	var ring []*bucket

	seen := map[uint32]bool{}

	for vbn := first; ; {
		if seen[vbn] {
			return nil, &CorruptError{VBN: vbn, Where: "a level's chain loops before its last bucket"}
		}

		seen[vbn] = true

		b, err := f.readBucket(vbn, size)
		if err != nil {
			return nil, err
		}

		switch {
		case b.level != level:
			return nil, b.corrupt(bktLevel, "a bucket at the wrong level")
		case b.indexNo != k.Ref:
			return nil, b.corrupt(bktIndexNo, "a bucket of another key")
		}

		ring = append(ring, b)

		if b.isLast() {
			if b.next != first {
				return nil, b.corrupt(bktNext, "the last bucket doesn't point to the level's first")
			}

			return ring, nil
		}

		vbn = b.next
	}
}

// checkTree checks a key's levels, top down.
func (f *File) checkTree(k *Key) error {
	first := k.RootVBN

	for level := k.RootLevel; level > 0; level-- {
		ring, err := f.levelRing(k, first, level)
		if err != nil {
			return err
		}

		if level == k.RootLevel && (len(ring) != 1 || ring[0].control&bktRoot == 0) {
			return ring[0].corrupt(bktControl, "a root that isn't alone and marked")
		}

		var last []byte

		for at, b := range ring {
			entries, err := b.indexEntries(k)
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				return b.corrupt(bktOverhead, "an index bucket with no entries")
			}

			if at == 0 {
				first = entries[0].pointer
			}

			for i, e := range entries {
				top := b.isLast() && i == len(entries)-1
				if !top && last != nil && k.Compare(last, e.key) >= 0 {
					return b.corrupt(e.offset, "index keys out of order")
				}

				if !top {
					last = e.key
				}

				if err := f.checkEntry(k, b, e, top); err != nil {
					return err
				}
			}
		}
	}

	if first != k.FirstData {
		return &CorruptError{VBN: first, Where: "the index's first bucket isn't the key's first data bucket"}
	}

	ring, err := f.levelRing(k, first, 0)
	if err != nil {
		return err
	}

	var last []byte

	for _, b := range ring {
		z, err := f.readLevelZero(k, b.vbn)
		if err != nil {
			return err
		}

		for _, key := range z.keys() {
			if last != nil && k.Compare(last, key) > 0 {
				return b.corrupt(bktOverhead, "level 0 keys out of order")
			}

			last = key
		}
	}

	return nil
}

// keys lists a level 0 bucket's stored keys, deleted records' too.
func (z *levelZero) keys() [][]byte {
	var list [][]byte

	for i := range z.primary {
		if !z.primary[i].isRRV() {
			list = append(list, z.primary[i].key)
		}
	}

	for i := range z.alts {
		list = append(list, z.alts[i].key)
	}

	return list
}

// checkEntry checks that the bucket an index entry names has no key above it.
func (f *File) checkEntry(k *Key, b *bucket, e indexEntry, top bool) error {
	if top {
		return nil
	}

	if b.level > 1 {
		child, err := f.readBucket(e.pointer, k.IndexBucketSize)
		if err != nil {
			return err
		}

		entries, err := child.indexEntries(k)
		if err != nil {
			return err
		}

		if len(entries) > 0 && k.Compare(entries[len(entries)-1].key, e.key) > 0 {
			return b.corrupt(e.offset, "an index bucket's keys pass its entry")
		}

		return nil
	}

	z, err := f.readLevelZero(k, e.pointer)
	if err != nil {
		return err
	}

	if keys := z.keys(); len(keys) > 0 && k.Compare(keys[len(keys)-1], e.key) > 0 {
		return b.corrupt(e.offset, "a bucket's keys pass its entry")
	}

	return nil
}

// checkPrimary checks every record and RRV of the primary key and returns the
// live records' RFAs, with each record's bytes for the alternates' checks.
func (f *File) checkPrimary() (map[RFA][]byte, error) {
	k := f.Keys[0]
	records := map[RFA][]byte{}

	if !k.HasIndex() {
		return records, nil
	}

	ring, err := f.levelRing(k, k.FirstData, 0)
	if err != nil {
		return nil, err
	}

	type forward struct {
		from RFA
		b    *bucket
		at   int
	}

	var forwards []forward

	for _, b := range ring {
		list, err := f.dataRecords(b, k)
		if err != nil {
			return nil, err
		}

		ids := map[uint16]bool{}

		for i := range list {
			r := &list[i]

			if ids[r.id] {
				return nil, b.corrupt(r.offset, "two records with one ID")
			}

			ids[r.id] = true

			if r.isRRV() {
				forwards = append(forwards, forward{RFA{VBN: b.vbn, ID: r.id}, b, r.offset})

				continue
			}

			if r.isDeleted() {
				continue
			}

			if _, twice := records[r.rfa]; twice {
				return nil, b.corrupt(r.offset, "two records with one RFA")
			}

			rest, err := r.expand(k, b)
			if err != nil {
				return nil, err
			}

			records[r.rfa] = k.assemble(r.key, rest)
		}
	}

	for _, w := range forwards {
		if _, live := records[w.from]; !live {
			return nil, w.b.corrupt(w.at, "an RRV with no record naming it")
		}
	}

	for rfa, want := range records {
		got, err := f.Record(rfa)
		if err != nil {
			return nil, &CorruptError{VBN: rfa.VBN, Where: "a record not found by its RFA: " + err.Error()}
		}

		if string(got) != string(want) {
			return nil, &CorruptError{VBN: rfa.VBN, Where: "another record found by a record's RFA"}
		}
	}

	return records, nil
}

// checkAlternate holds an alternate's pointers to the records.
func (f *File) checkAlternate(k *Key, records map[RFA][]byte) error {
	pointed := map[RFA]bool{}

	if err := f.Walk(k.Ref, func(e Entry) bool {
		pointed[e.RFA] = true

		return true
	}); err != nil {
		return err
	}

	walkErr := error(nil)

	if err := f.Walk(k.Ref, func(e Entry) bool {
		record, ok := records[e.RFA]
		if !ok {
			walkErr = &CorruptError{VBN: e.RFA.VBN, Where: "a pointer to no record"}

			return false
		}

		if value, belongs := k.Indexes(record); !belongs || k.Compare(value, e.Key) != 0 {
			walkErr = &CorruptError{VBN: e.RFA.VBN, Where: "a pointer under the wrong value"}

			return false
		}

		return true
	}); err != nil {
		return err
	}

	if walkErr != nil {
		return walkErr
	}

	for rfa, record := range records {
		if _, belongs := k.Indexes(record); belongs && !pointed[rfa] {
			return &CorruptError{VBN: rfa.VBN, Where: "a record missing from an alternate index"}
		}
	}

	return nil
}
