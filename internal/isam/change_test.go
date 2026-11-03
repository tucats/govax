package isam

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// The probe's record fields (testdata/idx88's gen.go): the primary key, the
// number, and the two string alternates.
const (
	fieldP   = 0
	fieldBN4 = 8
	fieldA   = 16
	fieldB   = 24
)

// changeRun follows one of the probe's "changing" cases: the same operations on
// a file made as VMS's was, each status held to VMS's, and the file held to each
// of the log's dumps of it in turn.
type changeRun struct {
	t     *testing.T
	name  string
	f     *File
	g     *growingBlocks
	attr  Attributes
	dumps []memoryBlocks
	exact bool // compare the bytes too, not only the dumps
}

// dumpsIn lists a case's dumps of its file, each from " vbn 1" to "=dump".
func dumpsIn(lines []string) []memoryBlocks {
	var dumps []memoryBlocks

	start := -1

	for i, line := range lines {
		switch {
		case line == " vbn 1" && start < 0:
			start = i
		case strings.HasPrefix(line, "=dump") && start >= 0:
			dumps = append(dumps, blocksIn(lines[start:i]))
			start = -1
		}
	}

	return dumps
}

// startChange makes a case's file and its first records, put the way the probe's
// PUTS put them: first, first+step, and so on.
func startChange(t *testing.T, log map[int]*logCase, c int, exact bool) *changeRun {
	t.Helper()

	name := "U" + strconv.Itoa(c-10)
	lines := log[c].lines

	m := logCreate.FindStringSubmatch(lines[1])
	if m == nil {
		t.Fatalf("%s: no $CREATE in its case", name)
	}

	alq, _ := strconv.Atoi(m[1])

	final := readable(t)[name]
	if final == nil {
		t.Fatalf("%s isn't in the save set", name)
	}

	r := &changeRun{
		t:     t,
		name:  name,
		g:     &growingBlocks{cluster: volumeCluster},
		attr:  final.attr,
		dumps: dumpsIn(lines),
		exact: exact,
	}

	f, err := Create(r.g, specOf(final, uint32(alq)))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	r.f = f

	return r
}

// puts writes records as PUTS does.
func (r *changeRun) puts(first, count, step, size int) {
	r.t.Helper()

	for i := range count {
		if _, err := r.f.Put(commonRecord(first+i*step, size)); err != nil {
			r.t.Fatalf("%s: $PUT %d: %v", r.name, first+i*step, err)
		}
	}
}

// put writes one record, which must get the RFA VMS's did.
func (r *changeRun) put(record []byte, want RFA, wantErr error) {
	r.t.Helper()

	result, err := r.f.Put(record)

	switch {
	case !errors.Is(err, wantErr):
		r.t.Errorf("%s: $PUT %q: %v, VMS had %v", r.name, record[:8], err, wantErr)
	case err == nil && result.RFA != want:
		r.t.Errorf("%s: $PUT %q: RFA %v, VMS had %v", r.name, record[:8], result.RFA, want)
	}
}

// find is a keyed $GET by a value, which must find the RFA VMS found.
func (r *changeRun) find(ref int, value string, want RFA) RFA {
	r.t.Helper()

	e, err := r.f.Find(ref, []byte(value), Equal)
	if err != nil {
		r.t.Fatalf("%s: finding %s by key %d: %v", r.name, value, ref, err)
	}

	if want != (RFA{}) && e.RFA != want {
		r.t.Errorf("%s: %s by key %d is %v, VMS found %v", r.name, value, ref, e.RFA, want)
	}

	return e.RFA
}

// missing is a keyed $GET that must find nothing, RMS$_RNF. A pointer a fast
// delete left is taken out, as RMS does when it finds one.
func (r *changeRun) missing(ref int, value string) {
	r.t.Helper()

	for range 4 {
		e, err := r.f.Find(ref, []byte(value), Equal)
		if errors.Is(err, ErrNotFound) {
			return
		}

		if err != nil {
			r.t.Fatalf("%s: finding %s by key %d: %v", r.name, value, ref, err)
		}

		if _, err := r.f.Record(e.RFA); !errors.Is(err, ErrDeleted) {
			r.t.Errorf("%s: %s by key %d found %v, which VMS didn't", r.name, value, ref, e.RFA)

			return
		}

		if err := r.f.DropPointer(e); err != nil {
			r.t.Fatalf("%s: dropping %v: %v", r.name, e, err)
		}
	}
}

// update is a $UPDATE, which must end as VMS's did.
func (r *changeRun) update(rfa RFA, record []byte, wantErr error, wantDup bool) {
	r.t.Helper()

	result, err := r.f.Update(rfa, record)

	switch {
	case !errors.Is(err, wantErr):
		r.t.Errorf("%s: $UPDATE of %v: %v, VMS had %v", r.name, rfa, err, wantErr)
	case err == nil && result.Duplicate != wantDup:
		r.t.Errorf("%s: $UPDATE of %v: duplicate %v, VMS said %v", r.name, rfa, result.Duplicate, wantDup)
	case err == nil && result.RFA != rfa:
		r.t.Errorf("%s: $UPDATE of %v moved its RFA to %v", r.name, rfa, result.RFA)
	}

	if err != nil {
		return
	}

	got, err := r.f.Record(rfa)
	if err != nil || string(got) != string(record) {
		r.t.Errorf("%s: after $UPDATE, %v holds %q (%v)", r.name, rfa, got, err)
	}
}

// del is a $DELETE, after which the RFA is RMS$_DEL.
func (r *changeRun) del(rfa RFA, fast bool) {
	r.t.Helper()

	if err := r.f.Delete(rfa, fast); err != nil {
		r.t.Fatalf("%s: $DELETE of %v: %v", r.name, rfa, err)
	}

	if _, err := r.f.Record(rfa); !errors.Is(err, ErrDeleted) {
		r.t.Errorf("%s: after $DELETE, %v: %v", r.name, rfa, err)
	}

	if err := r.f.Delete(rfa, fast); !errors.Is(err, ErrDeleted) {
		r.t.Errorf("%s: a second $DELETE of %v: %v", r.name, rfa, err)
	}
}

// dump holds the file, opened again from its bytes, to the log's next dump.
func (r *changeRun) dump() {
	r.t.Helper()

	if len(r.dumps) == 0 {
		r.t.Fatalf("%s: no dump left in the log", r.name)
	}

	vms := r.dumps[0]
	r.dumps = r.dumps[1:]

	want, err := Open(vms, r.attr)
	if err != nil {
		r.t.Fatalf("%s: VMS's: %v", r.name, err)
	}

	f, err := Open(r.g, r.attr)
	if err != nil {
		r.t.Fatalf("%s: %v", r.name, err)
	}

	if err := f.Check(); err != nil {
		r.t.Errorf("%s: %v", r.name, err)
	}

	compareFiles(r.t, r.name, f, want)

	if r.exact {
		sameBytes(r.t, r.name, r.g.data, vms)
	}

	r.f = f
}

// patched is a record with bytes written over at an offset.
func patched(record []byte, at int, with string) []byte {
	copy(record[at:], with)

	return record
}

// key is a field of record n, as the probe's MKKEY makes it.
func key(n, at int) string { return string(commonRecord(n, 40)[at : at+8]) }

// TestChangesAsVMS replays the probe's "changing" cases, 138 to 149: each
// $UPDATE and $DELETE, and the $PUTs among them, on a file made by the same
// $PUTs as VMS's, and holds the result to each of the log's dumps, by Dump and
// by Check, and where noted byte for byte.
func TestChangesAsVMS(t *testing.T) {
	log := readLog(t)

	rfa := func(vbn, id int) RFA { return RFA{VBN: uint32(vbn), ID: uint16(id)} }

	cases := map[int]func(r *changeRun){
		138: func(r *changeRun) { // the same, longer, shorter
			at := r.find(0, key(50, fieldP), rfa(4, 5))
			for _, size := range []int{100, 250, 60} {
				r.update(at, commonRecord(50, size), nil, false)
			}
		},
		139: func(*changeRun) {}, // RMS$_CUR and RMS$_RNF: RMS's, not isam's
		140: func(r *changeRun) { // the primary key, each alternate; CHG and DUP
			at := r.find(0, key(50, fieldP), rfa(4, 5))
			r.update(at, patched(commonRecord(50, 100), fieldP, "X0000000"), ErrKeyChanged, false)
			r.update(at, patched(commonRecord(50, 100), fieldA, "X0000000"), ErrKeyChanged, false)
			r.update(at, patched(commonRecord(50, 100), fieldB, "X0000000"), nil, false)

			at = r.find(0, key(60, fieldP), rfa(4, 6))
			r.update(at, patched(commonRecord(60, 100), fieldB, "X0000000"), ErrDuplicate, false)

			at = r.find(0, key(70, fieldP), rfa(4, 7))
			record := commonRecord(70, 100)
			record[fieldBN4], record[fieldBN4+1], record[fieldBN4+2], record[fieldBN4+3] = 50, 0, 0, 0
			r.update(at, record, nil, true)
		},
		141: func(r *changeRun) { // too short for the primary key, and for alternates
			at := r.find(0, key(50, fieldP), rfa(4, 5))
			r.update(at, commonRecord(50, 4), ErrRecordSize, false)
			r.update(at, commonRecord(50, 20), ErrKeyChanged, false)
			at = r.find(0, key(60, fieldP), rfa(4, 6))
			r.update(at, commonRecord(60, 10), ErrKeyChanged, false)
		},
		142: func(r *changeRun) { // $PUT with UIF: an update where the key is there
			at := r.find(0, key(50, fieldP), rfa(4, 5))
			r.update(at, patched(commonRecord(50, 150), fieldA, "X0000000"), nil, false)
			r.put(commonRecord(55, 150), rfa(4, 11), nil)
			r.put(commonRecord(50, 150), RFA{}, ErrDuplicate)
		},
		143: func(r *changeRun) { // $DELETE, then finding by each key, and putting it back
			at := r.find(0, key(50, fieldP), rfa(4, 5))
			r.del(at, false)
			r.missing(0, key(50, fieldP))
			r.missing(1, key(50, fieldA))
			r.missing(2, key(50, fieldB))
			r.put(commonRecord(50, 100), rfa(4, 11), nil)
			r.find(2, key(50, fieldB), rfa(4, 11))
		},
		144: func(r *changeRun) { // $DELETE with FDL, then by the alternate
			r.del(r.find(0, key(50, fieldP), rfa(4, 5)), true)
			r.missing(1, key(50, fieldA))
			r.missing(2, key(50, fieldB))
		},
		145: func(r *changeRun) { // a null alternate value, then given one
			r.put(patched(commonRecord(55, 100), fieldA, "        "), rfa(4, 11), nil)
			r.missing(1, "        ")
			at := r.find(0, key(55, fieldP), rfa(4, 11))
			r.update(at, commonRecord(55, 100), nil, false)
			r.find(1, key(55, fieldA), rfa(4, 11))
		},
		146: func(r *changeRun) { // $DELETE found through an alternate
			r.del(r.find(2, key(60, fieldB), rfa(4, 6)), false)
		},
		147: func(r *changeRun) { // RFAs across a split, and a record moved twice
			r.dump()
			at := r.find(0, key(20, fieldP), rfa(4, 2))
			r.update(at, commonRecord(20, 390), nil, false)
			r.dump()
			r.del(at, false)
		},
		148: func(r *changeRun) { // deleted records' space reused
			r.del(r.find(0, key(20, fieldP), rfa(3, 2)), false)
			r.dump()
			r.put(commonRecord(20, 150), rfa(3, 4), nil)
			r.put(commonRecord(21, 150), rfa(5, 1), nil)
		},
		149: func(r *changeRun) { // every record of a bucket deleted
			for n := 1; n <= 12; n++ {
				r.del(r.find(0, key(n, fieldP), RFA{}), false)
			}

			r.dump()

			if _, err := r.f.First(0); !errors.Is(err, ErrEnd) {
				r.t.Errorf("%s: a first record after all were deleted: %v", r.name, err)
			}

			r.puts(1, 4, 1, 150)
		},
	}

	for c := 138; c <= 149; c++ {
		t.Run(fmt.Sprintf("U%d", c-10), func(t *testing.T) {
			// U137 and U138 split buckets, whose stale bytes past a free offset
			// aren't followed (the phase doc's "What the splits settled"); the rest
			// are VMS's to the byte.
			r := startChange(t, log, c, c != 147 && c != 148)

			switch c {
			case 147:
				for _, n := range []int{10, 20, 30} {
					r.puts(n, 1, 1, 150)
				}

				replayPuts(t, r.name, r.f, putsOf(log[c].lines)[3:])
			case 148:
				r.puts(10, 3, 10, 150)
			case 149:
				r.puts(1, 12, 1, 150)
			default:
				r.puts(10, 10, 10, 100)
			}

			cases[c](r)
			r.dump()

			if len(r.dumps) > 0 {
				t.Errorf("%s: %d of VMS's dumps not compared", r.name, len(r.dumps))
			}
		})
	}
}

// changedRecord makes a record for the randomized test with a given primary
// key: its alternates' values made from alt, as randomRecord makes them from n,
// and size bytes long, so short enough at times to leave an alternate's index.
func changedRecord(r *rand.Rand, primary []byte, alt, size int) []byte {
	record := make([]byte, size)
	copy(record, primary)

	tail := make([]byte, 20)
	copy(tail[8:], fmt.Sprintf("A%07d", alt/5))
	binary.LittleEndian.PutUint32(tail[16:], uint32(alt%40))
	copy(record[8:], tail[8:])

	for i := 20; i < len(record); {
		run := 1 + r.Intn(30)
		value := byte(r.Intn(4)) + 'a'

		for ; run > 0 && i < len(record); run-- {
			record[i] = value
			i++
		}
	}

	return record
}

// TestChangesAgainstAModel puts, updates, and deletes thousands of records of
// random sizes in each shape of file the growth test uses, and one of fixed
// records, against a model: updates that grow past their bucket and split it,
// alternates that change or leave their index, deleted records' stubs and the
// RRVs that go with them, fast deletes and the pointers they leave, and keys put
// back where they were deleted. Every alternate here can change.
func TestChangesAgainstAModel(t *testing.T) {
	shapes := growthShapes()
	shapes = append(shapes, growthShape{
		name:    "fixed",
		spec:    Spec{Areas: []AreaSpec{{BucketSize: 1}}, Keys: []KeySpec{{Segments: []Segment{{Position: 0, Size: 8}}}, {Segments: []Segment{{Position: 8, Size: 8}}, Flags: FlagDuplicates}}},
		maxSize: 100,
		keys:    600,
	})

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(6))

			spec := shape.spec
			spec.Keys = append([]KeySpec(nil), spec.Keys...)

			for i := 1; i < len(spec.Keys); i++ {
				spec.Keys[i].Flags |= FlagChanges
			}

			fixed := shape.name == "fixed"

			spec.Attributes = Attributes{Format: Variable, Size: shape.maxSize}
			if fixed {
				spec.Attributes.Format = Fixed
			}

			size := func() int {
				if fixed {
					return shape.maxSize
				}

				return 8 + r.Intn(shape.maxSize-7)
			}

			g := &growingBlocks{cluster: volumeCluster}

			f, err := Create(g, spec)
			if err != nil {
				t.Fatal(err)
			}

			m := &model{gone: map[RFA]bool{}}
			clock := 0

			stampAll := func() []int {
				s := make([]int, len(spec.Keys))
				for i := range s {
					s[i] = clock
				}

				clock++

				return s
			}

			// holding says whether another live record has a key's value.
			holding := func(k *Key, value []byte, not RFA) bool {
				for _, rec := range m.records {
					if v, in := k.Indexes(rec.data); in && rec.rfa != not && k.Compare(v, value) == 0 {
						return true
					}
				}

				return false
			}

			dropLeftovers := func() {
				for _, k := range f.Keys[1:] {
					var stale []Entry

					if err := f.Walk(k.Ref, func(e Entry) bool {
						if m.gone[e.RFA] {
							stale = append(stale, e)
						}

						return true
					}); err != nil {
						t.Fatal(err)
					}

					for _, e := range stale {
						if err := f.DropPointer(e); err != nil {
							t.Fatalf("dropping %v: %v", e, err)
						}
					}
				}
			}

			for op := range 3000 {
				what := fmt.Sprintf("operation %d", op)

				switch choice := r.Intn(20); {
				case choice < 8 || len(m.records) < 20:
					n := r.Intn(shape.keys)
					record := changedRecord(r, []byte(fmt.Sprintf("%08d", n)), r.Intn(100000), size())

					k := f.Keys[0]
					value, _ := k.Value(record)
					dup := holding(k, value, RFA{})

					result, err := f.Put(record)

					switch {
					case dup && k.Flags&FlagDuplicates == 0:
						if !errors.Is(err, ErrDuplicate) {
							t.Fatalf("%s: a duplicate primary key put: %v", what, err)
						}
					case err != nil:
						t.Fatalf("%s: put: %v", what, err)
					default:
						m.records = append(m.records, modelRecord{rfa: result.RFA, data: record, stamp: stampAll()})
					}
				case choice < 15:
					i := r.Intn(len(m.records))
					rec := &m.records[i]

					alt := r.Intn(100000)
					if r.Intn(2) == 0 {
						alt = -1 // the alternates' values kept
					}

					record := changedRecord(r, rec.data[:8], 0, size())
					if alt < 0 {
						copy(record[8:], rec.data[8:min(20, len(rec.data))])
					} else {
						record = changedRecord(r, rec.data[:8], alt, len(record))
					}

					wantDup := false

					for _, k := range f.Keys[1:] {
						before, wasIn := k.Indexes(rec.data)
						after, willBeIn := k.Indexes(record)

						if willBeIn && (!wasIn || !bytes.Equal(before, after)) && holding(k, after, rec.rfa) {
							wantDup = true
						}
					}

					result, err := f.Update(rec.rfa, record)
					if err != nil {
						t.Fatalf("%s: update of %v: %v", what, rec.rfa, err)
					}

					if result.Duplicate != wantDup {
						t.Fatalf("%s: update of %v: duplicate %v, should be %v", what, rec.rfa, result.Duplicate, wantDup)
					}

					for _, k := range f.Keys[1:] {
						before, wasIn := k.Indexes(rec.data)
						after, willBeIn := k.Indexes(record)

						if wasIn != willBeIn || wasIn && !bytes.Equal(before, after) {
							rec.stamp[k.Ref] = clock
						}
					}

					clock++
					rec.data = record
				default:
					i := r.Intn(len(m.records))
					rfa := m.records[i].rfa

					if err := f.Delete(rfa, r.Intn(5) == 0); err != nil {
						t.Fatalf("%s: delete of %v: %v", what, rfa, err)
					}

					m.gone[rfa] = true
					m.records = append(m.records[:i], m.records[i+1:]...)
				}

				if op%25 == 24 {
					if err := f.Check(); err != nil {
						t.Fatalf("%s: %v", what, err)
					}
				}

				if op%500 == 499 {
					m.verify(t, what, f)
					dropLeftovers()
					m.verify(t, what+", leftovers dropped", f)
				}
			}

			reopened, err := Open(g, spec.Attributes)
			if err != nil {
				t.Fatal(err)
			}

			m.verify(t, "at the end", reopened)
		})
	}
}

// TestChangeRefuses covers what Update, Delete, and DropPointer refuse, each
// leaving the file as it was.
func TestChangeRefuses(t *testing.T) {
	g := &growingBlocks{cluster: volumeCluster}

	spec := oneKey(0)
	spec.Keys = append(spec.Keys, KeySpec{Segments: []Segment{{Position: 8, Size: 8}}})

	f, err := Create(g, spec)
	if err != nil {
		t.Fatal(err)
	}

	record := func(primary, alt string) []byte { return []byte(primary + alt + "rest of it") }

	first, err := f.Put(record("AAAAAAAA", "11111111"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.Put(record("BBBBBBBB", "22222222")); err != nil {
		t.Fatal(err)
	}

	before := append([]byte(nil), g.data...)

	for name, err := range map[string]error{
		"a primary key changed":            func() error { _, err := f.Update(first.RFA, record("CCCCCCCC", "11111111")); return err }(),
		"an alternate without CHG changed": func() error { _, err := f.Update(first.RFA, record("AAAAAAAA", "33333333")); return err }(),
		"too short for an alternate":       func() error { _, err := f.Update(first.RFA, []byte("AAAAAAAA1")); return err }(),
		"too short for the primary key":    func() error { _, err := f.Update(first.RFA, []byte("AAAA")); return err }(),
		"too long for a bucket": func() error {
			_, err := f.Update(first.RFA, append(record("AAAAAAAA", "11111111"), make([]byte, 2000)...))
			return err
		}(),
		"an RFA never given": func() error {
			_, err := f.Update(RFA{VBN: first.RFA.VBN, ID: 99}, record("AAAAAAAA", "11111111"))
			return err
		}(),
		"a delete of an RFA never given": f.Delete(RFA{VBN: first.RFA.VBN, ID: 99}, false),
		"a pointer of the primary key":   f.DropPointer(Entry{Ref: 0, Key: []byte("AAAAAAAA"), RFA: first.RFA}),
		"a pointer that isn't there":     f.DropPointer(Entry{Ref: 1, Key: []byte("11111111"), RFA: RFA{VBN: 9, ID: 9}}),
	} {
		want := map[string]error{
			"a primary key changed":            ErrKeyChanged,
			"an alternate without CHG changed": ErrKeyChanged,
			"too short for an alternate":       ErrKeyChanged,
			"too short for the primary key":    ErrRecordSize,
			"too long for a bucket":            ErrRecordSize,
			"an RFA never given":               ErrDeleted,
			"a delete of an RFA never given":   ErrDeleted,
			"a pointer of the primary key":     ErrKeyRef,
			"a pointer that isn't there":       ErrNotFound,
		}[name]

		if !errors.Is(err, want) {
			t.Errorf("%s: %v, not %v", name, err, want)
		}
	}

	if !bytes.Equal(before, g.data) {
		t.Error("a refused change wrote to the file")
	}

	if err := f.Check(); err != nil {
		t.Error(err)
	}

	// A fast delete leaves the alternate's pointer, which Check allows and
	// DropPointer takes out.
	if err := f.Delete(first.RFA, true); err != nil {
		t.Fatal(err)
	}

	if err := f.Check(); err != nil {
		t.Errorf("after a fast delete: %v", err)
	}

	e, err := f.Find(1, []byte("11111111"), Equal)
	if err != nil || e.RFA != first.RFA {
		t.Fatalf("the fast delete's pointer: %v %v", e, err)
	}

	if err := f.DropPointer(e); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Find(1, []byte("11111111"), Equal); !errors.Is(err, ErrNotFound) {
		t.Errorf("after DropPointer: %v", err)
	}

	// A value only a fast delete's pointer has isn't a duplicate.
	third, err := f.Put(record("CCCCCCCC", "33333333"))
	if err != nil {
		t.Fatal(err)
	}

	if err := f.Delete(third.RFA, true); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Put(record("DDDDDDDD", "33333333")); err != nil {
		t.Errorf("a value held only by a deleted record: %v", err)
	}

	reader, err := Open(memoryBlocks(g.data), spec.Attributes)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := reader.Update(e.RFA, nil); !errors.Is(err, ErrReadOnly) {
		t.Errorf("an update through a reader: %v", err)
	}

	if err := reader.Delete(e.RFA, false); !errors.Is(err, ErrReadOnly) {
		t.Errorf("a delete through a reader: %v", err)
	}
}
