package isam

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// TestCheckEveryVMSFile runs Check over every file VMS made. Its rules are ones
// VMS's own files keep, or they aren't rules.
func TestCheckEveryVMSFile(t *testing.T) {
	for name, f := range readable(t) {
		if err := f.Check(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestCheckFindsDamage changes a file that Check passes, each change a thing
// Check promises to see.
func TestCheckFindsDamage(t *testing.T) {
	build := func() *growingBlocks {
		g := &growingBlocks{cluster: volumeCluster}

		spec := oneKey(0)
		spec.Keys = append(spec.Keys, KeySpec{Segments: []Segment{{Position: 8, Size: 4}}, Flags: FlagDuplicates})

		f, err := Create(g, spec)
		if err != nil {
			t.Fatal(err)
		}

		for _, n := range []int{50, 10, 90, 30, 70, 20, 60, 40, 80, 15, 25, 35, 45, 55, 65, 75, 85} {
			r := make([]byte, 150)
			copy(r, fmt.Sprintf("%08d%04d", n, n%3))

			if _, err := f.Put(r); err != nil {
				t.Fatal(err)
			}
		}

		if err := f.Check(); err != nil {
			t.Fatal(err)
		}

		return g
	}

	open := func(g *growingBlocks) *File {
		f, err := Open(g, Attributes{Format: Variable, Size: 200})
		if err != nil {
			t.Fatal(err)
		}

		return f
	}

	// change rewrites one level 0 bucket of a key through its parsed form.
	change := func(g *growingBlocks, ref int, edit func(*File, *bucket)) {
		f := open(g)
		k := f.Keys[ref]

		b, err := f.readBucket(k.FirstData, k.DataBucketSize)
		if err != nil {
			t.Fatal(err)
		}

		edit(f, b)

		if err := f.writeBucket(b); err != nil {
			t.Fatal(err)
		}
	}

	for name, damage := range map[string]func(*growingBlocks){
		"a ring cut short": func(g *growingBlocks) {
			change(g, 0, func(_ *File, b *bucket) { b.next = b.vbn; b.control |= bktLast })
		},
		"a chain that loops": func(g *growingBlocks) {
			change(g, 0, func(_ *File, b *bucket) { b.next = b.vbn })
		},
		"keys out of order": func(g *growingBlocks) {
			change(g, 0, func(f *File, b *bucket) {
				list, err := f.dataRecords(b, f.Keys[0])
				if err != nil {
					t.Fatal(err)
				}

				// The first record's key, written larger than all.
				copy(b.data[list[0].offset+11:], "99999999")
			})
		},
		"an RRV with no record": func(g *growingBlocks) {
			change(g, 0, func(f *File, b *bucket) {
				list, err := f.dataRecords(b, f.Keys[0])
				if err != nil {
					t.Fatal(err)
				}

				for i := range list {
					if list[i].isRRV() {
						binary.LittleEndian.PutUint16(b.data[list[i].offset+3:], 500)

						return
					}
				}

				t.Fatal("no RRV to damage")
			})
		},
		"a pointer to no record": func(g *growingBlocks) {
			change(g, 1, func(f *File, b *bucket) {
				list, err := f.sidrs(b, f.Keys[1])
				if err != nil {
					t.Fatal(err)
				}

				// The first pointer's ID, past the SIDR's size word and key.
				binary.LittleEndian.PutUint16(b.data[list[0].offset+2+4+1:], 500)
			})
		},
	} {
		g := build()
		damage(g)

		if err := open(g).Check(); err == nil {
			t.Errorf("%s: Check passed it", name)
		}
	}
}

// model is what a file should hold, for the randomized tests: each record by its
// RFA, in the order put.
type model struct {
	records []modelRecord
}

type modelRecord struct {
	rfa  RFA
	data []byte
}

// inOrder lists a key's entries as they should be: by key, then the order put.
func (m *model) inOrder(k *Key) []modelRecord {
	var list []modelRecord

	for _, r := range m.records {
		if _, belongs := k.Indexes(r.data); belongs {
			list = append(list, r)
		}
	}

	sort.SliceStable(list, func(i, j int) bool {
		a, _ := k.Indexes(list[i].data)
		b, _ := k.Indexes(list[j].data)

		return k.Compare(a, b) < 0
	})

	return list
}

// verify holds a file to the model: Check, each key's walk in order, each record
// by RFA, and each value found. Records with equal keys are put in order, so a
// walk gives them in that order too.
func (m *model) verify(t *testing.T, what string, f *File) {
	t.Helper()

	if err := f.Check(); err != nil {
		t.Fatalf("%s: %v", what, err)
	}

	for _, k := range f.Keys {
		want := m.inOrder(k)
		at := 0

		if err := f.Walk(k.Ref, func(e Entry) bool {
			if at >= len(want) {
				t.Errorf("%s: key %d has more than %d entries", what, k.Ref, len(want))

				return false
			}

			if e.RFA != want[at].rfa {
				t.Errorf("%s: key %d, entry %d: RFA %v, should be %v", what, k.Ref, at, e.RFA, want[at].rfa)

				return false
			}

			at++

			return true
		}); err != nil {
			t.Fatalf("%s: %v", what, err)
		}

		if at != len(want) && !t.Failed() {
			t.Errorf("%s: key %d has %d entries, not %d", what, k.Ref, at, len(want))
		}

		for i := range want {
			value, _ := k.Indexes(want[i].data)

			e, err := f.Find(k.Ref, value, Equal)
			if err != nil {
				t.Fatalf("%s: key %d, finding %X: %v", what, k.Ref, value, err)
			}

			if k.Compare(e.Key, value) != 0 {
				t.Errorf("%s: key %d, finding %X found %X", what, k.Ref, value, e.Key)
			}
		}
	}

	for _, r := range m.records {
		got, err := f.Record(r.rfa)
		if err != nil {
			t.Fatalf("%s: RFA %v: %v", what, r.rfa, err)
		}

		if !bytes.Equal(got, r.data) {
			t.Fatalf("%s: RFA %v holds another record", what, r.rfa)
		}
	}
}

// growthShape is a file for the randomized test, and how to make its records.
type growthShape struct {
	name    string
	spec    Spec
	maxSize int // records run from the keys' size up to this
	keys    int // distinct primary values; few means many duplicates
}

func growthShapes() []growthShape {
	string8 := func(pos int, flags uint8) KeySpec {
		return KeySpec{Segments: []Segment{{Position: pos, Size: 8}}, Flags: flags}
	}

	all := uint8(FlagKeyCompress | FlagIndexCompress | FlagRecordCompress)
	two := []AreaSpec{{BucketSize: 2}, {BucketSize: 1}}

	number := KeySpec{Segments: []Segment{{Position: 16, Size: 4, Type: Bin4}}, Type: Bin4, Flags: FlagDuplicates}
	falling := KeySpec{Segments: []Segment{{Position: 16, Size: 4, Type: Int4 | Descending}}, Type: Int4 | Descending, Flags: FlagDuplicates}

	return []growthShape{
		{"plain", Spec{Areas: []AreaSpec{{BucketSize: 1}}, Keys: []KeySpec{string8(0, 0)}}, 150, 100000},
		{"compressed", Spec{Areas: []AreaSpec{{BucketSize: 1}}, Keys: []KeySpec{string8(0, all)}}, 150, 100000},
		{"long records", Spec{Areas: []AreaSpec{{BucketSize: 1}}, Keys: []KeySpec{string8(0, 0)}}, 480, 100000},
		{"big buckets", Spec{Areas: []AreaSpec{{BucketSize: 4}}, Keys: []KeySpec{string8(0, all)}}, 900, 100000},
		{"duplicate primary", Spec{Areas: []AreaSpec{{BucketSize: 1}}, Keys: []KeySpec{string8(0, FlagDuplicates)}}, 150, 12},
		{"alternates", Spec{Areas: []AreaSpec{{BucketSize: 1}}, Keys: []KeySpec{string8(0, 0), string8(8, FlagDuplicates), number}}, 150, 100000},
		{"descending alternate", Spec{Areas: []AreaSpec{{BucketSize: 2}}, Keys: []KeySpec{string8(0, all), falling}}, 200, 100000},
		{"compressed alternates", Spec{Areas: []AreaSpec{{BucketSize: 1}}, Keys: []KeySpec{string8(0, all), string8(8, FlagDuplicates|FlagKeyCompress|FlagIndexCompress)}}, 120, 100000},
		{"two areas", Spec{Areas: two, Keys: []KeySpec{{Segments: []Segment{{Position: 0, Size: 8}}, DataArea: 0, IndexArea: 1, Level1Area: 1}}}, 300, 100000},
		{"three index areas", Spec{Areas: []AreaSpec{{BucketSize: 1}, {BucketSize: 1}, {BucketSize: 1}}, Keys: []KeySpec{{Segments: []Segment{{Position: 0, Size: 8}}, DataArea: 0, Level1Area: 1, IndexArea: 2}}}, 100, 100000},
	}
}

// TestGrowthAgainstAModel puts thousands of records of random sizes and keys in
// each shape of file, in several orders. Records of mixed sizes are what the
// probe never does, so three-bucket splits and buckets full of RRVs are reached
// here and nowhere else, and only held to being right, not to being VMS's.
func TestGrowthAgainstAModel(t *testing.T) {
	const records = 1500

	orders := map[string]func(*rand.Rand, int, int) int{
		"random":  func(r *rand.Rand, _, keys int) int { return r.Intn(keys) },
		"rising":  func(_ *rand.Rand, i, keys int) int { return i % keys },
		"falling": func(_ *rand.Rand, i, keys int) int { return (keys - 1 - i%keys) },
	}

	for _, shape := range growthShapes() {
		for orderName, order := range orders {
			t.Run(shape.name+"/"+orderName, func(t *testing.T) {
				r := rand.New(rand.NewSource(88))

				spec := shape.spec
				spec.Attributes = Attributes{Format: Variable, Size: shape.maxSize}

				g := &growingBlocks{cluster: volumeCluster}

				f, err := Create(g, spec)
				if err != nil {
					t.Fatal(err)
				}

				m := &model{}
				used := map[int]bool{}

				for i := range records {
					n := order(r, i, shape.keys)
					if shape.keys >= records && used[n] {
						continue
					}

					used[n] = true

					record := randomRecord(r, n, shape.maxSize)

					result, err := f.Put(record)
					if err != nil {
						t.Fatalf("record %d of key %d: %v", i, n, err)
					}

					m.records = append(m.records, modelRecord{rfa: result.RFA, data: record})

					if i%250 == 0 {
						m.verify(t, fmt.Sprintf("after %d", i), f)
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
}

// randomRecord makes a record: the primary key at 0, a value with duplicates at 8,
// a number at 16, then bytes with runs in them, to compress.
func randomRecord(r *rand.Rand, n, maxSize int) []byte {
	const least = 20

	record := make([]byte, least+r.Intn(maxSize-least+1))

	copy(record, fmt.Sprintf("%08d", n))
	copy(record[8:], fmt.Sprintf("A%07d", n/5))
	binary.LittleEndian.PutUint32(record[16:], uint32(n%40))

	for i := least; i < len(record); {
		run := 1 + r.Intn(30)
		value := byte(r.Intn(4)) + 'a'

		for ; run > 0 && i < len(record); run-- {
			record[i] = value
			i++
		}
	}

	return record
}

// TestLongRecordAlone puts a record no split of its bucket can make room for: it
// must go in a bucket of its own, with what was around it on either side.
func TestLongRecordAlone(t *testing.T) {
	g := &growingBlocks{cluster: volumeCluster}

	f, err := Create(g, Spec{
		Attributes: Attributes{Format: Variable, Size: 490},
		Areas:      []AreaSpec{{BucketSize: 1}},
		Keys:       []KeySpec{{Segments: []Segment{{Position: 0, Size: 8}}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	m := &model{}

	put := func(n, size int) RFA {
		t.Helper()

		record := make([]byte, size)
		copy(record, fmt.Sprintf("%08d", n))

		result, err := f.Put(record)
		if err != nil {
			t.Fatal(err)
		}

		m.records = append(m.records, modelRecord{rfa: result.RFA, data: record})

		return result.RFA
	}

	for n := 10; n <= 90; n += 20 {
		put(n, 80)
	}

	if err := f.Check(); err != nil {
		t.Fatal(err)
	}

	rfa := put(40, 480)

	if rfa.VBN == f.Keys[0].FirstData {
		t.Errorf("the long record is in the first bucket, %v", rfa)
	}

	m.verify(t, "after a long record", f)

	if levelZeroBuckets(t, f, 0) < 3 {
		t.Errorf("%d buckets, not 3", levelZeroBuckets(t, f, 0))
	}
}

// TestDuplicatesKeepTheirOrder fills several buckets with one key: they stay in
// the order put, and no index entry is spent on the continuation (G113's rule).
func TestDuplicatesKeepTheirOrder(t *testing.T) {
	g := &growingBlocks{cluster: volumeCluster}

	f, err := Create(g, oneKey(FlagDuplicates))
	if err != nil {
		t.Fatal(err)
	}

	m := &model{}

	for i := range 200 {
		record := make([]byte, 100)
		copy(record, "SAMESAME")
		binary.LittleEndian.PutUint32(record[8:], uint32(i))

		result, err := f.Put(record)
		if err != nil {
			t.Fatal(err)
		}

		if i > 0 && !result.Duplicate {
			t.Errorf("record %d wasn't reported a duplicate", i)
		}

		m.records = append(m.records, modelRecord{rfa: result.RFA, data: record})
	}

	m.verify(t, "after 200 duplicates", f)

	root, err := f.readBucket(f.Keys[0].RootVBN, f.Keys[0].IndexBucketSize)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := root.indexEntries(f.Keys[0])
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Errorf("%d index entries for one key", len(entries))
	}

	// A higher key after them must still be found, and in the right place.
	record := make([]byte, 100)
	copy(record, "ZZZZZZZZ")

	if _, err := f.Put(record); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Find(0, []byte("ZZZZZZZZ"), Equal); errors.Is(err, ErrNotFound) {
		t.Error("the key after the duplicates can't be found")
	}

	if err := f.Check(); err != nil {
		t.Error(err)
	}
}
