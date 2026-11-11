package isam

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"
)

// A fast load: what CONVERT does to fill an indexed file, by building each key's
// tree from records already in its order, rather than by a $PUT at a time
// (the Record Management Utilities manual's /FAST_LOAD; docs/planning/PHASE-88 - keyed
// index files.md, subtask 10c). The rules are from the files VMS 7.1's CONVERT
// made in round 3 of the probe (testdata/idx88's C6 to C9), which load_test.go
// compares with the ones made here:
//
//   - A key's first data bucket and its first index bucket are allocated
//     together, before anything is written (C78D stopped after one record, with
//     the file grown by both and nothing else changed). So the data's first
//     bucket is followed by the root, then by the rest of the data: F122's 8 keys
//     made data 5, root 7, data 9.
//   - Each key is finished before the next begins, and the prolog hears of a key
//     only then: C10B has key 0 whole and key 1 empty, and C78D an area that
//     knows nothing of what it gave out.
//   - A bucket fills as far as it can, or to the key's fill size unless
//     /FILL_BUCKETS says otherwise. When a record doesn't fit, its bucket's
//     highest key goes up to the index, which is where a full index bucket grows
//     the level above (a new root first, then the new bucket: G116's root 98, level
//     1's 99, data 100), and then the next data bucket is allocated.
//   - Nothing has been rewritten, so every bucket's check character is 0, and a
//     data bucket's records have IDs from 1 in order.
//   - An alternate's level 0 holds a SIDR for each value, with its pointers in the
//     primary key's order, the sort being stable (the manual's Jones example). A
//     SIDR isn't divided between buckets if it fits in one (C71's key 3), and its
//     pointers' VBNs are 4 bytes each.
//
// No VMS case loads a file with a fill smaller than its bucket (K50's fills had no
// bucket size to be a percentage of), so that rule follows ROP LOA's; nor a primary
// key's duplicates, nor a value with more pointers than a bucket holds, which are
// laid out as RMS's growth lays them (addPointer's continuation): unconfirmed.

// ErrOutOfOrder is a record a load can't take, its primary key lower than the
// last's: what CONVERT/NOSORT calls "record not in order".
var ErrOutOfOrder = errors.New("isam: record not in order")

// Load is a fast load in progress.
type Load struct {
	f    *File
	fill bool // /FILL_BUCKETS: fills are ignored

	primary *treeLoad

	// entries are what the alternates' levels will hold, by key, in the primary
	// key's order; seen is a no-duplicates alternate's values.
	entries [][]loadEntry
	seen    []map[string]bool

	previous []byte // the last record's primary key
}

// loadEntry is a pointer an alternate's level 0 will hold.
type loadEntry struct {
	value []byte
	rfa   RFA
}

// StartLoad begins a fast load of f, which must have no records. The prolog stays
// as it is until FinishPrimary, then until each alternate's end.
func (f *File) StartLoad(fillBuckets bool) *Load {
	f.holdProlog = true

	return &Load{
		f: f, fill: fillBuckets,
		entries: make([][]loadEntry, len(f.Keys)),
		seen:    make([]map[string]bool, len(f.Keys)),
	}
}

// Add puts the next record in, returning its RFA. ErrRecordSize is a record
// shorter than the primary key, or the wrong size for a fixed file;
// ErrOutOfOrder one lower than the last; ErrDuplicate one with a value a key
// doesn't allow twice, a primary key's or an alternate's.
func (l *Load) Add(record []byte) (RFA, error) {
	f := l.f
	k := f.Keys[0]

	value, ok := k.Value(record)
	if !ok || f.attr.Format == Fixed && len(record) != f.attr.Size {
		return RFA{}, ErrRecordSize
	}

	if l.previous != nil {
		switch c := k.Compare(l.previous, value); {
		case c > 0:
			return RFA{}, ErrOutOfOrder
		case c == 0 && k.Flags&FlagDuplicates == 0:
			return RFA{}, ErrDuplicate
		}
	}

	rest, err := f.encodeData(k, value, record)
	if err != nil {
		return RFA{}, err
	}

	for i := 1; i < len(f.Keys); i++ {
		alt := f.Keys[i]
		if alt.Flags&FlagDuplicates != 0 {
			continue
		}

		if v, ok := alt.Indexes(record); ok && l.seen[i][string(v)] {
			return RFA{}, ErrDuplicate
		}
	}

	if l.primary == nil {
		if l.primary, err = l.startTree(k); err != nil {
			return RFA{}, err
		}
	}

	t := l.primary

	stored := f.encodeRecord(k, t.data.nextID, RFA{VBN: t.data.vbn, ID: t.data.nextID}, value, t.last, rest)
	if !t.fits(len(stored)) {
		if err := t.nextData(); err != nil {
			return RFA{}, err
		}

		stored = f.encodeRecord(k, t.data.nextID, RFA{VBN: t.data.vbn, ID: t.data.nextID}, value, nil, rest)
	}

	rfa := RFA{VBN: t.data.vbn, ID: t.data.nextID}

	t.data.free += copy(t.data.data[t.data.free:], stored)
	t.data.nextID++
	t.last = value
	l.previous = value

	for i := 1; i < len(f.Keys); i++ {
		alt := f.Keys[i]

		v, ok := alt.Indexes(record)
		if !ok {
			continue
		}

		l.entries[i] = append(l.entries[i], loadEntry{value: v, rfa: rfa})

		if alt.Flags&FlagDuplicates == 0 {
			if l.seen[i] == nil {
				l.seen[i] = map[string]bool{}
			}

			l.seen[i][string(v)] = true
		}
	}

	return rfa, nil
}

// FinishPrimary ends the primary key's tree, and tells the prolog of it.
func (l *Load) FinishPrimary() error {
	return l.finish(l.primary)
}

// FinishAlternate builds alternate key i (1 up), and tells the prolog of it.
func (l *Load) FinishAlternate(i int) error {
	k := l.f.Keys[i]
	list := l.entries[i]

	sort.SliceStable(list, func(a, b int) bool { return k.Compare(list[a].value, list[b].value) < 0 })

	var t *treeLoad

	for at := 0; at < len(list); {
		end := at + 1
		for end < len(list) && k.Compare(list[at].value, list[end].value) == 0 {
			end++
		}

		if t == nil {
			var err error
			if t, err = l.startTree(k); err != nil {
				return err
			}
		}

		if err := t.addSIDR(list[at].value, list[at:end]); err != nil {
			return err
		}

		at = end
	}

	return l.finish(t)
}

// End lets the prolog be written as usual again, the load finished or not.
func (l *Load) End() { l.f.holdProlog = false }

// finish ends a key's tree, if it has one, and writes the prolog's areas and
// the key's descriptor. A key with no records has neither buckets nor an index.
func (l *Load) finish(t *treeLoad) error {
	if t != nil {
		if err := t.end(); err != nil {
			return err
		}
	}

	f := l.f
	f.holdProlog = false

	defer func() { f.holdProlog = true }()

	for i := range f.Areas {
		if err := f.updateArea(i); err != nil {
			return err
		}
	}

	if t == nil {
		return nil
	}

	return f.updateKey(t.k)
}

// treeLoad is one key's tree as it's built.
type treeLoad struct {
	l *Load
	f *File
	k *Key

	data      *bucket // the level 0 bucket being filled
	last      []byte  // its last value
	firstData uint32

	// levels are the index levels, 1 upward: the bucket being filled at each.
	levels []*levelLoad
}

// levelLoad is one index level as it's built.
type levelLoad struct {
	level int
	area  int
	vbn   uint32
	first uint32
	items []indexItem
}

// startTree allocates a key's first data and index buckets. As makeIndex has it,
// a level 1 area apart from the index area has a level 2 root over level 1 from
// the start.
func (l *Load) startTree(k *Key) (*treeLoad, error) {
	f := l.f

	vbn, err := f.allocate(k.DataArea)
	if err != nil {
		return nil, err
	}

	t := &treeLoad{l: l, f: f, k: k, data: newBucket(vbn, k.DataBucketSize, k.Ref, 0, 0), firstData: vbn}

	if _, err := t.addLevel(k.Level1Area); err != nil {
		return nil, err
	}

	if k.IndexArea != k.Level1Area {
		if _, err := t.addLevel(k.IndexArea); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// addLevel allocates the first bucket of a new level at the top.
func (t *treeLoad) addLevel(area int) (*levelLoad, error) {
	vbn, err := t.f.allocate(area)
	if err != nil {
		return nil, err
	}

	lv := &levelLoad{level: len(t.levels) + 1, area: area, vbn: vbn, first: vbn}
	t.levels = append(t.levels, lv)

	return lv, nil
}

// fits says whether the data bucket can take more bytes, up to the key's fill
// size unless /FILL_BUCKETS said otherwise.
func (t *treeLoad) fits(more int) bool {
	limit := t.k.DataFill
	if t.l.fill {
		limit = 0
	}

	return t.data.roomFor(limit, more)
}

// nextData ends the full data bucket: its highest key goes up to the index, then
// the next bucket is allocated and the chain goes on to it.
func (t *treeLoad) nextData() error {
	if err := t.addEntry(0, t.last, t.data.vbn); err != nil {
		return err
	}

	vbn, err := t.f.allocate(t.k.DataArea)
	if err != nil {
		return err
	}

	t.data.next = vbn

	if err := t.f.writeBucket(t.data); err != nil {
		return err
	}

	t.data = newBucket(vbn, t.k.DataBucketSize, t.k.Ref, 0, 0)
	t.last = nil

	return nil
}

// addEntry adds an entry to the index level at position i. If its bucket can't
// take it, the bucket's highest key goes up first, a level being added above if
// this was the top, and a new bucket at this level begins with the entry.
func (t *treeLoad) addEntry(i int, key []byte, pointer uint32) error {
	lv := t.levels[i]
	entry := indexItem{key: bytes.Clone(key), pointer: pointer}
	grown := append(lv.items[:len(lv.items):len(lv.items)], entry)

	if t.indexHolds(grown) {
		lv.items = grown

		return nil
	}

	if i+1 == len(t.levels) {
		if _, err := t.addLevel(t.k.IndexArea); err != nil {
			return err
		}
	}

	if err := t.addEntry(i+1, lv.items[len(lv.items)-1].key, lv.vbn); err != nil {
		return err
	}

	vbn, err := t.f.allocate(lv.area)
	if err != nil {
		return err
	}

	if err := t.writeLevel(lv, vbn, 0); err != nil {
		return err
	}

	lv.vbn, lv.items = vbn, []indexItem{entry}

	return nil
}

// indexHolds says whether an index bucket can hold these entries. VMS leaves room
// in a loaded index bucket for one key whole: each of G116's level 1 buckets
// stopped with at most 504 of 512 bytes used by keys, pointers, and its ending,
// and 507 was refused, with a key of 8 (unconfirmed for other sizes, the one file
// with two levels having that one).
func (t *treeLoad) indexHolds(items []indexItem) bool {
	k := t.k
	size := indexPointerSize(items, 2)
	used := bktOverhead + len(indexKeyBytes(k, items)) + len(items)*size

	if !t.l.fill && k.IndexFill > 0 && used > k.IndexFill {
		return false
	}

	return used+bktEndOffset+k.Size <= k.IndexBucketSize*BlockSize
}

// writeLevel writes a level's bucket, its chain going on to next.
func (t *treeLoad) writeLevel(lv *levelLoad, next uint32, control uint8) error {
	k := t.k

	b := newBucket(lv.vbn, k.IndexBucketSize, k.Ref, lv.level, control)
	b.next = next
	b.putIndexItems(k, lv.items, indexPointerSize(lv.items, 2))

	return t.f.writeBucket(b)
}

// end writes the last bucket at each level, whose chain goes round to the level's
// first and whose last index entry has the highest value of all, and tells the
// key where its tree is.
func (t *treeLoad) end() error {
	k := t.k

	t.data.control = bktLast
	t.data.next = t.firstData

	if err := t.f.writeBucket(t.data); err != nil {
		return err
	}

	high := k.highestValue()

	if err := t.addEntry(0, high, t.data.vbn); err != nil {
		return err
	}

	for i := 0; i < len(t.levels); i++ {
		lv := t.levels[i]

		control := uint8(bktLast)
		if i == len(t.levels)-1 {
			control |= bktRoot
		}

		if control&bktRoot == 0 {
			if err := t.addEntry(i+1, high, lv.vbn); err != nil {
				return err
			}

			// That can have added a level, but this one's still not its top.
		}

		if err := t.writeLevel(lv, lv.first, control); err != nil {
			return err
		}
	}

	top := t.levels[len(t.levels)-1]

	k.RootVBN, k.RootLevel, k.FirstData = top.vbn, top.level, t.firstData
	k.Flags &^= FlagNoIndexYet

	return nil
}

// loadPointer lays out a SIDR pointer as a load writes it: its VBN always 4 bytes,
// like a data record's, where a $PUT's is as few as hold it (control 82 and 02,
// beside sidrPointerBytes's 80 and 00).
func loadPointer(rfa RFA, first bool) []byte {
	control := uint8(dataPointerControl)
	if first {
		control |= ircFirstKey
	}

	out := append(make([]byte, 0, 3+dataPointerBytes), control)
	out = binary.LittleEndian.AppendUint16(out, rfa.ID)

	return binary.LittleEndian.AppendUint32(out, rfa.VBN)
}

// addSIDR adds a value's SIDR, with its pointers. It goes in the next bucket if it
// won't fit in this one but would in an empty one. One too long for any bucket
// is divided, a SIDR with the same value going on in the next bucket.
func (t *treeLoad) addSIDR(value []byte, list []loadEntry) error {
	var pointers []byte

	for i, e := range list {
		pointers = append(pointers, loadPointer(e.rfa, i == 0)...)
	}

	stored := sidrBytes(t.k, value, t.last, pointers)

	if !t.fits(len(stored)) && t.last != nil {
		if err := t.nextData(); err != nil {
			return err
		}

		stored = sidrBytes(t.k, value, nil, pointers)
	}

	if t.fits(len(stored)) {
		t.data.free += copy(t.data.data[t.data.free:], stored)
		t.last = value

		return nil
	}

	// More pointers than a bucket holds: as many as fit, then the rest.
	for i := 1; i <= len(list); i++ {
		var run []byte

		for j, e := range list[:i] {
			run = append(run, loadPointer(e.rfa, j == 0)...)
		}

		if t.fits(len(sidrBytes(t.k, value, nil, run))) {
			continue
		}

		var part []byte

		for j, e := range list[:i-1] {
			part = append(part, loadPointer(e.rfa, j == 0)...)
		}

		t.data.free += copy(t.data.data[t.data.free:], sidrBytes(t.k, value, nil, part))
		t.last = value

		if err := t.nextData(); err != nil {
			return err
		}

		return t.addSIDR(value, list[i-1:])
	}

	return nil
}
