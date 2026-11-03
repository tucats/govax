package isam

import (
	"bytes"
	"encoding/binary"
)

// This file is what happens when a record doesn't fit its bucket. Every rule
// here was read from the probe's growth cases (testdata/idx88, cases 104 to
// 119), which dump the file after each $PUT; the phase doc's "What the splits
// settled" says which are VMS's and which are only reasonable choices.
//
// In outline: a new bucket always goes to the right of the old one, which
// keeps its VBN, so a key's first data bucket never changes. A record that
// moves keeps its RFA, and an RRV in the bucket the RFA names says where it
// went. The level above gets an entry for the old bucket's new high key.

// chainAfter links a new bucket into its level after an old one. The level is
// a ring, so the new bucket takes the old one's next pointer whatever it was,
// and the "last" flag if the old one had it.
func chainAfter(left, right *bucket) {
	right.next = left.next
	right.control |= left.control & bktLast
	left.next = right.vbn
	left.control &^= bktLast
}

// startRight makes the bucket a split moves records to. Its check character
// counts its one write so far.
func (f *File) startRight(area, blocks, ref, level int) (*bucket, error) {
	vbn, err := f.allocate(area)
	if err != nil {
		return nil, err
	}

	right := newBucket(vbn, blocks, ref, level, 0)
	right.touch()

	return right, nil
}

// putBytes replaces everything a bucket holds after its header with new bytes.
// What was past them stays, as VMS leaves it.
func (b *bucket) putBytes(records []byte) {
	copy(b.data[bktOverhead:], records)
	b.free = bktOverhead + len(records)
}

// fits says whether a bucket can hold this many bytes of records.
func fits(blocks, records int) bool {
	return bktOverhead+records <= blocks*BlockSize-1
}

// balancedSplit says how many of a list of sizes stay on the left of a split:
// the place where the two sides differ least, and where two places tie, the one
// further right. It's the Guide's "RMS tries to keep half of the records"; VMS's
// growth cases show that it's half by bytes, the new record counted, and G118's
// scattered load settles the ties.
func balancedSplit(sizes []int) int {
	return balancedPlace(len(sizes), func(n int) int {
		left, right := 0, 0

		for i, size := range sizes {
			if i < n {
				left += size
			} else {
				right += size
			}
		}

		return left - right
	})
}

// balancedPlace finds that place, given each place's difference between sides.
func balancedPlace(count int, difference func(int) int) int {
	best, least := 1, -1

	for n := 1; n < count; n++ {
		d := difference(n)
		if d < 0 {
			d = -d
		}

		if least < 0 || d <= least {
			best, least = n, d
		}
	}

	return best
}

// splitOrder lists the places to try to split a list, first the balanced one,
// then ever further from it, keeping a run of equal keys together where it can.
// equal(i) says that items i-1 and i have the same key, so a split there would
// leave a continuation. Those come last. A left side of nothing comes last of
// all, for a bucket so full of RRVs that nothing else will do.
func splitOrder(count, balanced int, equal func(int) bool) []int {
	balanced = max(1, min(balanced, count-1))

	var clean, broken []int

	add := func(n int) {
		if n < 1 || n >= count {
			return
		}

		if equal(n) {
			broken = append(broken, n)
		} else {
			clean = append(clean, n)
		}
	}

	add(balanced)

	for d := 1; d < count; d++ {
		add(balanced - d)
		add(balanced + d)
	}

	return append(append(clean, broken...), 0)
}

// dataItem is one record in a data bucket being split: an old record, or the
// new one, which has no ID until its place is settled.
type dataItem struct {
	old   *dataRecord
	key   []byte
	body  []byte
	isNew bool
}

// dataSplit is what splitting a data bucket works from.
type dataSplit struct {
	f     *File
	k     *Key
	left  *bucket
	items []dataItem
	rrvs  []dataRecord // the RRVs already in the bucket
	pos   int          // the new record's place in items
}

// encodeRRV lays out a forwarding record.
func encodeRRV(id uint16, to RFA) []byte {
	out := make([]byte, 0, 9)
	out = append(out, ircRRV|dataPointerControl)
	out = binary.LittleEndian.AppendUint16(out, id)
	out = binary.LittleEndian.AppendUint16(out, to.ID)

	return binary.LittleEndian.AppendUint32(out, to.VBN)
}

// encodeItems lays out a run of a split's items for a bucket, each key stored
// against the one before it. Every item takes its ID from id: it stays as it was
// on the left and counts from 1 on the right, where the new record's RFA is made
// from its place.
func (s *dataSplit) encodeItems(items []dataItem, vbn uint32, id func(*dataItem, int) uint16) []byte {
	var out []byte

	var previous []byte

	for i := range items {
		it := &items[i]
		n := id(it, i)

		rfa := RFA{VBN: vbn, ID: n}
		if it.old != nil {
			rfa = it.old.rfa
		}

		stored := s.f.encodeRecord(s.k, n, rfa, it.key, previous, it.body)
		if it.old != nil {
			stored[0] = it.old.control
		}

		out = append(out, stored...)
		previous = it.key
	}

	return out
}

// leftBytes lays out what stays when items[:n] do: those records, the RRVs
// already there, and one for each record leaving whose RFA names this bucket.
// Their IDs on the right are their places there, so the RRVs can be written now.
func (s *dataSplit) leftBytes(n int, rightVBN uint32) []byte {
	out := s.encodeItems(s.items[:n], s.left.vbn, func(it *dataItem, _ int) uint16 {
		if it.old != nil {
			return it.old.id
		}

		return s.left.nextID
	})

	for i := range s.rrvs {
		r := &s.rrvs[i]
		out = append(out, encodeRRV(r.id, r.rfa)...)
	}

	for i := n; i < len(s.items); i++ {
		if it := &s.items[i]; it.old != nil && it.old.rfa.VBN == s.left.vbn {
			out = append(out, encodeRRV(it.old.rfa.ID, RFA{VBN: rightVBN, ID: uint16(i - n + 1)})...)
		}
	}

	return out
}

// rightBytes lays out items[n:] for a new bucket, whose IDs count from 1.
func (s *dataSplit) rightBytes(n int, rightVBN uint32) []byte {
	return s.encodeItems(s.items[n:], rightVBN, func(_ *dataItem, i int) uint16 { return uint16(i + 1) })
}

// leavingRRVs counts the RRVs a split at n would add to the old bucket.
func (s *dataSplit) leavingRRVs(n int) int {
	count := 0

	for i := n; i < len(s.items); i++ {
		if it := &s.items[i]; it.old != nil && it.old.rfa.VBN == s.left.vbn {
			count++
		}
	}

	return count
}

// choose finds a split that both sides can hold, or says there is none, when the
// new record must have a bucket of its own.
func (s *dataSplit) choose() (int, bool) {
	sizes := make([]int, len(s.items))

	var previous []byte

	for i := range s.items {
		sizes[i] = len(s.f.encodeRecord(s.k, 0, RFA{}, s.items[i].key, previous, s.items[i].body))
		previous = s.items[i].key
	}

	// Each side as the split would leave it, but for the RRVs the bucket already
	// has, which VMS's choices don't count (G118 tells them apart).
	balanced := balancedPlace(len(s.items), func(n int) int {
		left, right := 9*s.leavingRRVs(n), 0

		for i, size := range sizes {
			if i < n {
				left += size
			} else {
				right += size
			}
		}

		return left - right
	})

	order := splitOrder(len(s.items), balanced, func(n int) bool {
		return s.k.Compare(s.items[n-1].key, s.items[n].key) == 0
	})

	blocks := s.k.DataBucketSize

	for _, n := range order {
		// The VBN changes nothing about a side's size.
		if fits(blocks, len(s.leftBytes(n, 0))) && fits(blocks, len(s.rightBytes(n, 0))) {
			return n, true
		}
	}

	return 0, false
}

// splitData puts a record in a data bucket that can't hold it. A record going
// past the end of a bucket whose last record is its newest has a new bucket to
// itself and nothing moves, which is what makes a load in key order leave its
// buckets full (G104, and G113 for a duplicate). Otherwise half the records move
// right.
func (f *File) splitData(b *bucket, k *Key, list []dataRecord, value, rest []byte) (RFA, error) {
	s := &dataSplit{f: f, k: k, left: b}

	placed := false

	for i := range list {
		r := &list[i]
		if r.isRRV() {
			s.rrvs = append(s.rrvs, *r)

			continue
		}

		if !placed && k.Compare(r.key, value) > 0 {
			s.pos = len(s.items)
			s.items = append(s.items, dataItem{key: value, body: rest, isNew: true})
			placed = true
		}

		s.items = append(s.items, dataItem{old: r, key: bytes.Clone(r.key), body: bytes.Clone(r.body)})
	}

	if !placed && s.lastIsNewest() {
		right, rfa, err := f.newRecordBucket(b, k, value, rest)
		if err != nil {
			return RFA{}, err
		}

		return rfa, f.linkNew(b, right, k, s.lastKey(len(s.items)), value)
	}

	if !placed {
		s.pos = len(s.items)
		s.items = append(s.items, dataItem{key: value, body: rest, isNew: true})
	}

	n, ok := s.choose()
	if !ok {
		return s.alone()
	}

	return s.split(n)
}

// lastIsNewest says whether the bucket's last record is also the last one it was
// given, or it has none: only then does a record going past the end start a
// bucket of its own. G104's load in order does that, where G108's bucket of 8, 9,
// 12, and 15, its 15 not its newest, takes 16 by splitting in half.
func (s *dataSplit) lastIsNewest() bool {
	return len(s.items) == 0 || s.items[len(s.items)-1].old.id+1 == s.left.nextID
}

// lastKey is the highest key among the first n items, nil if there are none.
func (s *dataSplit) lastKey(n int) []byte {
	if n == 0 {
		return nil
	}

	return s.items[n-1].key
}

// newRecordBucket makes the bucket a record goes in by itself, after a left one.
func (f *File) newRecordBucket(left *bucket, k *Key, value, rest []byte) (*bucket, RFA, error) {
	right, err := f.startRight(k.DataArea, k.DataBucketSize, k.Ref, 0)
	if err != nil {
		return nil, RFA{}, err
	}

	rfa := RFA{VBN: right.vbn, ID: 1}
	right.putBytes(f.encodeRecord(k, 1, rfa, value, nil, rest))
	right.nextID = 2

	chainAfter(left, right)

	return right, rfa, nil
}

// linkNew writes a split's two buckets and tells the level above. A left side
// with no key to name it, or one whose high key is also the right side's first,
// gets no entry: the right bucket is found through the chain, a continuation.
// That's how a run of duplicates goes on past a bucket (G113, G114).
func (f *File) linkNew(left, right *bucket, k *Key, leftHigh, rightFirst []byte) error {
	left.touch()

	if err := f.writeBucket(left); err != nil {
		return err
	}

	if err := f.writeBucket(right); err != nil {
		return err
	}

	if leftHigh == nil || k.Compare(leftHigh, rightFirst) == 0 {
		return nil
	}

	return f.addIndexEntry(k, 1, bytes.Clone(leftHigh), right.vbn)
}

// split moves items[n:] to a new bucket. A record that moved before has its RRV
// where its RFA says, in another bucket, and that RRV is changed; one still in
// the bucket its RFA names leaves a new RRV behind. The new record, if it stays,
// takes the bucket's next ID; if it moves, it's named by its place in the new
// bucket, and the old bucket spends no ID on it (G106).
func (s *dataSplit) split(n int) (RFA, error) {
	f, k, b := s.f, s.k, s.left

	right, err := f.startRight(k.DataArea, k.DataBucketSize, k.Ref, 0)
	if err != nil {
		return RFA{}, err
	}

	rfa := RFA{VBN: b.vbn, ID: b.nextID}
	if s.pos >= n {
		rfa = RFA{VBN: right.vbn, ID: uint16(s.pos - n + 1)}
	}

	moved := s.items[n:]

	b.putBytes(s.leftBytes(n, right.vbn))
	right.putBytes(s.rightBytes(n, right.vbn))
	right.nextID = uint16(len(moved) + 1)

	if s.pos < n {
		b.nextID++
	}

	chainAfter(b, right)

	if err := s.moveRRVs(moved, right.vbn); err != nil {
		return RFA{}, err
	}

	return rfa, f.linkNew(b, right, k, s.lastKey(n), s.items[n].key)
}

// alone gives the new record a bucket of its own, between what's before it and
// what's after: no split of the old records left room for it either side. No
// VMS case reaches this, each using one size of record, so it's the plainest
// thing that works: three buckets, each entry its own.
func (s *dataSplit) alone() (RFA, error) {
	f, k, b := s.f, s.k, s.left

	before := s.items[:s.pos]
	after := s.items[s.pos+1:]
	value, rest := s.items[s.pos].key, s.items[s.pos].body

	var right *bucket

	if len(after) > 0 {
		var err error

		right, err = f.startRight(k.DataArea, k.DataBucketSize, k.Ref, 0)
		if err != nil {
			return RFA{}, err
		}

		old := s.items
		s.items = append(append([]dataItem(nil), before...), after...)
		left := s.leftBytes(len(before), right.vbn)
		moved := s.rightBytes(len(before), right.vbn)
		s.items = old

		b.putBytes(left)
		right.putBytes(moved)
		right.nextID = uint16(len(after) + 1)

		chainAfter(b, right)

		if err := s.moveRRVs(after, right.vbn); err != nil {
			return RFA{}, err
		}
	}

	middle, rfa, err := f.newRecordBucket(b, k, value, rest)
	if err != nil {
		return RFA{}, err
	}

	if err := f.linkNew(b, middle, k, s.lastKey(len(before)), value); err != nil {
		return RFA{}, err
	}

	if right == nil {
		return rfa, nil
	}

	if err := f.writeBucket(right); err != nil {
		return RFA{}, err
	}

	return rfa, f.addIndexEntry(k, 1, bytes.Clone(value), right.vbn)
}

// moveRRVs changes the RRVs of records that moved before and have moved again,
// each in the bucket its RFA names. A bucket is written once in a split however
// many of its RRVs change, which its check character shows: G106's bucket 3 counts
// one write for one, and G118's bucket 9 one for two.
func (s *dataSplit) moveRRVs(moved []dataItem, rightVBN uint32) error {
	byBucket := map[uint32]map[uint16]RFA{}

	var order []uint32

	for i := range moved {
		it := &moved[i]
		if it.old == nil || it.old.rfa.VBN == s.left.vbn {
			continue
		}

		vbn := it.old.rfa.VBN
		if byBucket[vbn] == nil {
			byBucket[vbn] = map[uint16]RFA{}
			order = append(order, vbn)
		}

		byBucket[vbn][it.old.rfa.ID] = RFA{VBN: rightVBN, ID: uint16(i + 1)}
	}

	for _, vbn := range order {
		if err := s.f.rewriteRRVs(s.k, vbn, byBucket[vbn]); err != nil {
			return err
		}
	}

	return nil
}

// rewriteRRVs points a bucket's RRVs of the given IDs at their records' new places.
func (f *File) rewriteRRVs(k *Key, vbn uint32, to map[uint16]RFA) error {
	b, err := f.readBucket(vbn, k.DataBucketSize)
	if err != nil {
		return err
	}

	list, err := f.dataRecords(b, k)
	if err != nil {
		return err
	}

	found := 0

	for i := range list {
		r := &list[i]

		place, ok := to[r.id]
		if !r.isRRV() || !ok {
			continue
		}

		binary.LittleEndian.PutUint16(b.data[r.offset+3:], place.ID)
		writePointer(b.data[r.offset+5:r.offset+5+int(r.control&ircPtrSize)+2], place.VBN)
		found++
	}

	if found != len(to) {
		return b.corrupt(bktOverhead, "a moved record with no RRV where its RFA says")
	}

	b.touch()

	return f.writeBucket(b)
}

// lastFor goes along a level from the bucket an index led to, to the last one
// that can hold a value: a bucket after it is a continuation if its first key
// isn't above the value. With no duplicates that's always the bucket the index
// named, whose high key is below the next bucket's first.
func (f *File) lastFor(k *Key, vbn uint32, value []byte) (*bucket, error) {
	b, err := f.readBucket(vbn, k.DataBucketSize)
	if err != nil {
		return nil, err
	}

	for !b.isLast() {
		next, err := f.readBucket(b.next, k.DataBucketSize)
		if err != nil {
			return nil, err
		}

		first, err := f.firstKey(next, k)
		if err != nil {
			return nil, err
		}

		if first == nil || k.Compare(first, value) > 0 {
			break
		}

		b = next
	}

	return b, nil
}

// firstKey is the first key a level 0 bucket holds, nil if it holds none.
func (f *File) firstKey(b *bucket, k *Key) ([]byte, error) {
	if k.Ref != 0 {
		list, err := f.sidrs(b, k)
		if err != nil || len(list) == 0 {
			return nil, err
		}

		return list[0].key, nil
	}

	list, err := f.dataRecords(b, k)
	if err != nil {
		return nil, err
	}

	for i := range list {
		if !list[i].isRRV() {
			return list[i].key, nil
		}
	}

	return nil, nil
}

// sidrBytes lays out a SIDR with its key stored against the one before it. Its
// pointers are bytes already, kept as they are.
func sidrBytes(k *Key, value, previous, pointers []byte) []byte {
	key := storedKey(k, value, previous)

	out := binary.LittleEndian.AppendUint16(nil, uint16(len(key)+len(pointers)))
	out = append(out, key...)

	return append(out, pointers...)
}

// pointerRun is a SIDR's pointers as stored, for moving them whole.
func (b *bucket) pointerRun(k *Key, s *sidr) []byte {
	end := s.offset + 2 + int(binary.LittleEndian.Uint16(b.data[s.offset:]))

	used := k.Size
	if k.Flags&FlagKeyCompress != 0 {
		used = 2 + int(b.data[s.offset+2])
	}

	return bytes.Clone(b.data[s.offset+2+used : end])
}

// sidrItem is one SIDR in a bucket being split.
type sidrItem struct {
	key      []byte
	pointers []byte
}

// encodeSIDRs lays out a run of SIDRs.
func encodeSIDRs(k *Key, items []sidrItem) []byte {
	var out, previous []byte

	for i := range items {
		out = append(out, sidrBytes(k, items[i].key, previous, items[i].pointers)...)
		previous = items[i].key
	}

	return out
}

// sidrItems copies a bucket's SIDRs, whose bytes are about to be written over.
func (b *bucket) sidrItems(k *Key, list []sidr) []sidrItem {
	items := make([]sidrItem, len(list))

	for i := range list {
		items[i] = sidrItem{key: bytes.Clone(list[i].key), pointers: b.pointerRun(k, &list[i])}
	}

	return items
}

// splitSIDRs moves part of an alternate key's level 0 bucket to a new one, as a
// primary key's split does, only with no RRVs and no IDs to give out: pointers
// hold RFAs, which don't change. It returns the two buckets, written.
func (f *File) splitSIDRs(b *bucket, k *Key, items []sidrItem, n int) (*bucket, error) {
	right, err := f.startRight(k.DataArea, k.DataBucketSize, k.Ref, 0)
	if err != nil {
		return nil, err
	}

	b.putBytes(encodeSIDRs(k, items[:n]))
	right.putBytes(encodeSIDRs(k, items[n:]))

	chainAfter(b, right)

	var high []byte
	if n > 0 {
		high = items[n-1].key
	}

	return right, f.linkNew(b, right, k, high, items[n].key)
}

// chooseSIDRs finds a split of SIDRs that both sides can hold.
func chooseSIDRs(k *Key, items []sidrItem) (int, bool) {
	sizes := make([]int, len(items))

	for i := range items {
		var previous []byte
		if i > 0 {
			previous = items[i-1].key
		}

		sizes[i] = len(sidrBytes(k, items[i].key, previous, items[i].pointers))
	}

	order := splitOrder(len(items), balancedSplit(sizes), func(int) bool { return false })

	for _, n := range order {
		if n > 0 && fits(k.DataBucketSize, len(encodeSIDRs(k, items[:n]))) && fits(k.DataBucketSize, len(encodeSIDRs(k, items[n:]))) {
			return n, true
		}
	}

	return 0, false
}

// indexItem is one entry of an index bucket being changed, its key its own copy.
type indexItem struct {
	key     []byte
	pointer uint32
}

// indexItems copies an index bucket's entries.
func (b *bucket) indexItems(k *Key) ([]indexItem, error) {
	list, err := b.indexEntries(k)
	if err != nil {
		return nil, err
	}

	items := make([]indexItem, len(list))
	for i, e := range list {
		items[i] = indexItem{key: bytes.Clone(e.key), pointer: e.pointer}
	}

	return items, nil
}

// indexPointerSize is the size a run of entries' pointers need: the bucket's
// already, or wider for a VBN that won't fit. Wider pointers are unconfirmed:
// every index here has 2-byte ones.
func indexPointerSize(items []indexItem, least int) int {
	size := least
	for _, it := range items {
		size = max(size, pointerBytes(it.pointer))
	}

	return size
}

// indexKeyBytes lays out an index bucket's keys, each stored against the one
// before it when the index is compressed.
func indexKeyBytes(k *Key, items []indexItem) []byte {
	var out, previous []byte

	for _, it := range items {
		stored := it.key
		if k.Flags&FlagIndexCompress != 0 {
			stored = compressKey(it.key, previous)
		}

		out = append(out, stored...)
		previous = it.key
	}

	return out
}

// indexFits says whether an index bucket of some size can hold entries. The keys
// must stop short of the pointers, not merely reach them (G116's bucket 539).
func indexFits(size int, keys, count, pointer int) bool {
	return bktOverhead+keys < size-bktEndOffset-count*pointer
}

// putIndexItems writes entries into an index bucket's bytes: the keys up from the
// header, the pointers down from the end, and the word that says where they stop.
func (b *bucket) putIndexItems(k *Key, items []indexItem, size int) {
	keys := indexKeyBytes(k, items)
	copy(b.data[bktOverhead:], keys)
	b.free = bktOverhead + len(keys)

	b.control = b.control&^(bktPtrMask<<bktPtrShift) | uint8(size-2)<<bktPtrShift

	at := len(b.data) - bktEndOffset

	for _, it := range items {
		at -= size
		writePointer(b.data[at:at+size], it.pointer)
	}

	binary.LittleEndian.PutUint16(b.data[len(b.data)-bktEndOffset:], uint16(at-1))
}

// descendTo goes from a key's root down to the bucket at a level that a value
// belongs in. The level is above 0, so it's an index bucket.
func (f *File) descendTo(k *Key, value []byte, level int) (*bucket, error) {
	vbn := k.RootVBN

	for at := k.RootLevel; ; {
		b, err := f.readBucket(vbn, k.IndexBucketSize)
		if err != nil {
			return nil, err
		}

		list, err := b.indexEntries(k)
		if err != nil {
			return nil, err
		}

		next, found := uint32(0), false

		for i, e := range list {
			if i == len(list)-1 && b.isLast() || k.Compare(e.key, value) >= 0 {
				next, found = e.pointer, true

				break
			}
		}

		switch {
		case !found && b.isLast():
			return nil, b.corrupt(bktOverhead, "an index bucket with no entries")
		case !found:
			vbn = b.next
		case at == level:
			return b, nil
		default:
			vbn = next
			at--
		}
	}
}

// addIndexEntry records a split at the level below: a bucket's new high key,
// which keeps the pointer that led to it, while the entry that had that pointer
// goes on naming the same high key but now points at the new bucket. So the
// entry is a new one before the old, and the old one's pointer changes (G106:
// "5 -> 3" becomes "1 -> 3, 5 -> 6").
func (f *File) addIndexEntry(k *Key, level int, high []byte, right uint32) error {
	b, err := f.descendTo(k, high, level)
	if err != nil {
		return err
	}

	items, err := b.indexItems(k)
	if err != nil {
		return err
	}

	at := len(items) - 1

	for i := range items {
		if k.Compare(items[i].key, high) >= 0 {
			at = i

			break
		}
	}

	if !(at == len(items)-1 && b.isLast()) && k.Compare(items[at].key, high) < 0 {
		return b.corrupt(bktOverhead, "an index bucket lower than the key it was found for")
	}

	grown := make([]indexItem, 0, len(items)+1)
	grown = append(grown, items[:at]...)
	grown = append(grown, indexItem{key: high, pointer: items[at].pointer}, indexItem{key: items[at].key, pointer: right})
	grown = append(grown, items[at+1:]...)

	size := indexPointerSize(grown, b.pointerSize())
	keys := indexKeyBytes(k, grown)

	if f.loading && bktOverhead+len(keys)+len(grown)*size > k.IndexFill ||
		!indexFits(len(b.data), len(keys), len(grown), size) {
		return f.splitIndex(b, k, level, grown, at)
	}

	b.putIndexItems(k, grown, size)
	b.touch()

	return f.writeBucket(b)
}

// splitIndexAt says how many entries stay on the left of an index split: as
// many of the old ones as take up no more than half the bucket's room, and the
// new one too if it falls among them. That isn't a data bucket's rule, which
// counts the new record. So 49 entries and one new leave 24 and 26 (G115), and a
// compressed bucket is cut at 247 of 494 bytes (G116).
func splitIndexAt(k *Key, items []indexItem, newAt, pointer, room int) int {
	var previous []byte

	n, sum := 0, 0

	for i, it := range items {
		if i == newAt {
			continue
		}

		stored := k.Size
		if k.Flags&FlagIndexCompress != 0 {
			stored = len(compressKey(it.key, previous))
		}

		if sum+stored+pointer > room/2 {
			break
		}

		sum += stored + pointer
		n++
		previous = it.key
	}

	if newAt <= n {
		n++
	}

	return max(1, min(n, len(items)-1))
}

// splitIndex moves part of an index bucket to a new one, telling the level above,
// which a root doesn't have: then there's a new root over the two, one level
// higher. The old bucket keeps its VBN, losing only the root flag, and the new
// root is allocated after the new bucket (G115: 5 and 58, then root 59).
func (f *File) splitIndex(b *bucket, k *Key, level int, items []indexItem, newAt int) error {
	area := k.IndexArea
	if level == 1 {
		area = k.Level1Area
	}

	right, err := f.startRight(area, k.IndexBucketSize, k.Ref, level)
	if err != nil {
		return err
	}

	pointer := indexPointerSize(items, b.pointerSize())

	n := splitIndexAt(k, items, newAt, pointer, len(b.data)-bktOverhead-bktEndOffset)
	if f.loading && newAt == len(items)-2 && b.isLast() {
		n = len(items) - 1
	}

	isRoot := b.control&bktRoot != 0
	b.control &^= bktRoot

	chainAfter(b, right)

	b.putIndexItems(k, items[:n], pointer)
	right.putIndexItems(k, items[n:], pointer)
	b.touch()

	if err := f.writeBucket(b); err != nil {
		return err
	}

	if err := f.writeBucket(right); err != nil {
		return err
	}

	high := items[n-1].key

	if !isRoot {
		return f.addIndexEntry(k, level+1, high, right.vbn)
	}

	return f.newRoot(k, level+1, b.vbn, high, right)
}

// newRoot puts a new root over the two halves of the old one. Its check
// character starts at 1, where a file's first index bucket starts at 0, since
// that one is written before it holds a record (G115's level 2).
func (f *File) newRoot(k *Key, level int, left uint32, high []byte, right *bucket) error {
	root, err := f.startRight(k.IndexArea, k.IndexBucketSize, k.Ref, level)
	if err != nil {
		return err
	}

	root.control = bktLast | bktRoot

	items := []indexItem{{key: high, pointer: left}, {key: k.highestValue(), pointer: right.vbn}}
	root.putIndexItems(k, items, indexPointerSize(items, 2))

	if err := f.writeBucket(root); err != nil {
		return err
	}

	k.RootVBN, k.RootLevel = root.vbn, level

	return f.updateKey(k)
}
