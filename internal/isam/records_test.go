package isam

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// commonRecord is what the probe's MKREC makes from a number (see
// testdata/idx88/README.md), cut to a size: every field at a known place,
// the same bytes VMS was given to store.
func commonRecord(n, size int) []byte {
	r := make([]byte, 33000)

	for i := range r {
		r[i] = 'a' + byte(i%26)
	}

	for i := 60; i < 100; i++ {
		r[i] = ' '
	}

	copy(r[0:], fmt.Sprintf("P%07d", n))
	binary.LittleEndian.PutUint32(r[8:], uint32(n))
	binary.LittleEndian.PutUint32(r[12:], uint32(-int32(n)))
	copy(r[16:], fmt.Sprintf("A%07d", n/3))
	copy(r[24:], fmt.Sprintf("B%07d", 9999999-n))
	copy(r[32:], packedValue(n))
	binary.LittleEndian.PutUint64(r[36:], uint64(n))
	binary.LittleEndian.PutUint64(r[44:], uint64(-int64(n)))

	return r[:size]
}

// packedValue is n in 7 packed digits and a positive sign, as CVTLP
// writes it: a digit in each 4 bits, and C as the last 4.
func packedValue(n int) []byte {
	text := fmt.Sprintf("%07d", n)
	p := make([]byte, 4)

	for i := range 7 {
		p[i/2] |= (text[i] - '0') << (4 * uint(1-i%2))
	}

	p[3] |= 0x0C

	return p
}

// entriesOf lists a key's entries.
func entriesOf(t *testing.T, name string, f *File, ref int) []Entry {
	t.Helper()

	var list []Entry

	if err := f.Walk(ref, func(e Entry) bool {
		list = append(list, e)

		return true
	}); err != nil {
		t.Fatalf("%s key %d: %v", name, ref, err)
	}

	return list
}

// TestEveryRecordByEveryKey reads every record of every file VMS made by
// each key. Each key's entries must be in order, each must lead to a record
// holding that key's value, each alternate must list exactly the records
// that belong in it, and no duplicates where the key forbids them.
func TestEveryRecordByEveryKey(t *testing.T) {
	records := 0

	for name, f := range readable(t) {
		primary := entriesOf(t, name, f, 0)
		records += len(primary)

		holding := map[RFA][]byte{}

		for _, e := range primary {
			r, err := f.Record(e.RFA)
			if err != nil {
				t.Fatalf("%s: record %v: %v", name, e.RFA, err)
			}

			holding[e.RFA] = r
		}

		if len(holding) != len(primary) {
			t.Errorf("%s: %d entries with %d RFAs", name, len(primary), len(holding))
		}

		for _, k := range f.Keys {
			list := entriesOf(t, name, f, k.Ref)
			checkOrder(t, name, k, list)

			want := map[string]int{}

			for rfa, r := range holding {
				if value, ok := k.Indexes(r); ok {
					want[fmt.Sprintf("%X %v", value, rfa)]++
				}
			}

			for _, e := range list {
				id := fmt.Sprintf("%X %v", e.Key, e.RFA)
				if want[id] == 0 {
					t.Errorf("%s key %d: an entry no record explains: %s", name, k.Ref, id)
				}

				want[id]--
			}

			for id, n := range want {
				if n > 0 {
					t.Errorf("%s key %d: a record with no entry: %s", name, k.Ref, id)
				}
			}
		}
	}

	if records < 5000 {
		t.Errorf("read only %d records", records)
	}
}

// checkOrder checks a key's entries are in its order.
func checkOrder(t *testing.T, name string, k *Key, list []Entry) {
	t.Helper()

	for i := 1; i < len(list); i++ {
		c := k.Compare(list[i-1].Key, list[i].Key)

		switch {
		case c > 0:
			t.Errorf("%s key %d: %X before %X", name, k.Ref, list[i-1].Key, list[i].Key)
		case c == 0 && k.Flags&FlagDuplicates == 0:
			t.Errorf("%s key %d: %X twice, and no duplicates", name, k.Ref, list[i].Key)
		}
	}
}

// changedFile says which of the probe's files hold records other than what
// MKREC made: F120 and F121's keys were written over (the manual's
// tables), and U128 on are the $UPDATE and $DELETE cases, whose changes
// the log follows.
func changedFile(name string) bool {
	return name == "F120" || name == "F121" || strings.HasPrefix(name, "U") || name == "L140"
}

// TestRecordsAreVMSs rebuilds each record from its number, so a wrong byte
// anywhere in decompression, or in putting a key back where it belongs,
// shows up. The tail of a record is its number's letters and blanks, which
// makes data compression do real work.
func TestRecordsAreVMSs(t *testing.T) {
	compared := 0

	for name, f := range readable(t) {
		if changedFile(name) {
			continue
		}

		for _, e := range entriesOf(t, name, f, 0) {
			got, err := f.Record(e.RFA)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}

			n := int(binary.LittleEndian.Uint32(got[8:]))
			if want := commonRecord(n, len(got)); !bytes.Equal(got, want) {
				t.Errorf("%s: record %v isn't record %d of size %d:\n got %q\nwant %q", name, e.RFA, n, len(got), got, want)
			}

			compared++
		}
	}

	if compared < 5000 {
		t.Errorf("compared only %d records", compared)
	}
}

// logCase is one case of the probe's log, by its number.
type logCase struct {
	title string
	lines []string
}

// readLog splits VMS's log of the probe into its cases.
func readLog(t *testing.T) map[int]*logCase {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(vaxDir, "idx88-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	cases := map[int]*logCase{}

	var current *logCase

	head := regexp.MustCompile(`^#(\d+) (.*)$`)

	for _, line := range strings.Split(string(data), "\n") {
		if m := head.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			current = &logCase{title: m[2]}
			cases[n] = current

			continue
		}

		if current != nil {
			current.lines = append(current.lines, line)
		}
	}

	return cases
}

// fileOfCase names the file a case made. Files are numbered as cases are,
// but for the ten cases 123 to 132, which all use F122.
func fileOfCase(c int, files map[string]*File) (string, *File) {
	number := c
	if c > 132 {
		number = c - 10
	}

	// Case 103 wrote its records into the version a later $CREATE left
	// behind.
	if c == 103 {
		return "E103;1", files["E103;1"]
	}

	for _, prefix := range []string{"S", "E", "G", "F", "Q", "U", "L"} {
		name := prefix + strconv.Itoa(number)
		if f, ok := files[name]; ok {
			return name, f
		}
	}

	return "", nil
}

// Patterns in the log.
var (
	logWalkLine = regexp.MustCompile(`^  (.{8}) rfa=(\d+),(\d+) n=(\d+)$`)
	logWalkEnd  = regexp.MustCompile(`^ walk key (\d+): (\d+) read`)
	logChange   = regexp.MustCompile(`^=\$(PUT|UPDATE|DELETE|TRUNCATE)|^ puts `)
	logGot      = regexp.MustCompile(`^ got n=(\d+) rfa=(\d+),(\d+) sts=\w+ \|(.*)\|$`)
	logPut      = regexp.MustCompile(`^=\$PUT n=(\d+) size=(\d+) r=00010001$`)
	logRAB      = regexp.MustCompile(`^ rab rfa=(\d+),(\d+) `)
)

// TestWalksAgainstVMS replays the log's walks: a run of $GETs in a key's
// order, each record's first 8 bytes, RFA, and size. Only a key's last walk
// in a case with nothing written after it describes the file saved.
func TestWalksAgainstVMS(t *testing.T) {
	files := readable(t)
	compared := 0

	for number, c := range readLog(t) {
		name, f := fileOfCase(number, files)
		if f == nil {
			continue
		}

		var run []string

		last := map[int][]string{}

		for _, line := range c.lines {
			switch {
			case logChange.MatchString(line):
				last = map[int][]string{} // what came before doesn't describe the file now
			case logWalkLine.MatchString(line):
				run = append(run, line)
			default:
				if m := logWalkEnd.FindStringSubmatch(line); m != nil {
					ref, _ := strconv.Atoi(m[1])
					last[ref] = run
				}

				run = nil
			}
		}

		for ref, lines := range last {
			list := entriesOf(t, name, f, ref)
			if len(list) != len(lines) {
				t.Errorf("%s key %d: %d entries, VMS read %d", name, ref, len(list), len(lines))

				continue
			}

			for i, e := range list {
				got, err := f.Record(e.RFA)
				if err != nil {
					t.Fatal(err)
				}

				want := fmt.Sprintf("  %s rfa=%d,%d n=%d", shown(got[:8]), e.RFA.VBN, e.RFA.ID, len(got))
				if want != lines[i] {
					t.Errorf("%s key %d, record %d: we have %q, VMS %q", name, ref, i, want, lines[i])
				}

				compared++
			}
		}
	}

	if compared < 3000 {
		t.Errorf("compared only %d lines", compared)
	}
}

// shown is a record's bytes as the log prints them: $FAO's !AF rule as the
// run shows it, a period for a control character, for 7F to 9F, and for FF.
func shown(b []byte) string {
	out := make([]byte, len(b))

	for i, c := range b {
		if c < 0x20 || c >= 0x7F && c <= 0x9F || c == 0xFF {
			c = '.'
		}

		out[i] = c
	}

	return string(out)
}

// TestRFAsAgainstVMS holds Record to the RFA each logged $PUT got: splits
// came after, in the growth cases, and the same RFA still leads to the same
// record. Later changes can only matter in the cases that update or delete,
// which changedFile leaves out.
func TestRFAsAgainstVMS(t *testing.T) {
	files := readable(t)
	compared := 0

	for number, c := range readLog(t) {
		name, f := fileOfCase(number, files)
		if f == nil || changedFile(name) {
			continue
		}

		for i, line := range c.lines {
			m := logPut.FindStringSubmatch(line)
			if m == nil || i+1 >= len(c.lines) {
				continue
			}

			rab := logRAB.FindStringSubmatch(c.lines[i+1])
			if rab == nil {
				continue
			}

			n, _ := strconv.Atoi(m[1])
			size, _ := strconv.Atoi(m[2])
			vbn, _ := strconv.Atoi(rab[1])
			id, _ := strconv.Atoi(rab[2])

			got, err := f.Record(RFA{VBN: uint32(vbn), ID: uint16(id)})
			if err != nil {
				t.Errorf("%s: $PUT of %d got RFA %d,%d: %v", name, n, vbn, id, err)

				continue
			}

			if !bytes.Equal(got, commonRecord(n, size)) {
				t.Errorf("%s: $PUT of %d got RFA %d,%d, which now holds %q", name, n, vbn, id, got[:16])
			}

			compared++
		}
	}

	if compared < 300 {
		t.Errorf("compared only %d records", compared)
	}
}

// matchOf turns the probe's name for a ROP combination into a Match. The
// others are RMS's to refuse, with RMS$_ROP.
func matchOf(rop string) (Match, bool) {
	switch rop {
	case "0":
		return Equal, true
	case "KGE":
		return EqualOrNext, true
	case "KGT":
		return Next, true
	case "REV+KGE":
		return EqualOrPrevious, true
	case "REV+KGT":
		return Previous, true
	}

	return Equal, false
}

// oneGet is what a case's $GET gave: not found (RNF), refused (ROP), or a
// record with its RFA, size, and first bytes.
type oneGet struct {
	status string
	size   int
	rfa    RFA
	text   string
}

// getAt reads the $GET on a line of the log and the record that followed.
func getAt(lines []string, i int) (oneGet, bool) {
	if !strings.HasPrefix(lines[i], "=$GET r=") {
		return oneGet{}, false
	}

	g := oneGet{status: strings.TrimPrefix(lines[i], "=$GET r=")}

	if i+1 < len(lines) {
		if m := logGot.FindStringSubmatch(lines[i+1]); m != nil {
			g.size, _ = strconv.Atoi(m[1])
			vbn, _ := strconv.Atoi(m[2])
			id, _ := strconv.Atoi(m[3])
			g.rfa = RFA{VBN: uint32(vbn), ID: uint16(id)}
			g.text = m[4]
		}
	}

	return g, true
}

// RMS's status values, as the log prints them.
const (
	statusSuccess  = "00010001"
	statusNotFound = "000182B2"
	statusBadROP   = "0001867C"
)

// compareGet compares a Find's answer with VMS's $GET.
func compareGet(t *testing.T, what string, f *File, e Entry, err error, want oneGet) {
	t.Helper()

	switch {
	case want.status == statusNotFound:
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: got %v, %v; VMS has RNF", what, e, err)
		}

		return
	case want.status != statusSuccess:
		t.Errorf("%s: VMS has status %s, not expected here", what, want.status)

		return
	case err != nil:
		t.Errorf("%s: %v; VMS has %v", what, err, want.rfa)

		return
	case e.RFA != want.rfa:
		t.Errorf("%s: RFA %v; VMS has %v", what, e.RFA, want.rfa)

		return
	}

	record, err := f.Record(e.RFA)
	if err != nil {
		t.Errorf("%s: %v", what, err)

		return
	}

	if len(record) != want.size {
		t.Errorf("%s: size %d; VMS has %d", what, len(record), want.size)
	}

	if got := shown(record[:min(60, len(record))]); got != want.text {
		t.Errorf("%s: %q; VMS has %q", what, got, want.text)
	}
}

// TestFindAgainstVMS replays the log's cases of "a mark, then one $GET": the
// manual's Tables 7-3 and 7-4 on both directions, generic keys, and numeric
// keys. Each mark says the key, the value, and the ROP combination.
func TestFindAgainstVMS(t *testing.T) {
	log := readLog(t)
	f122 := openProbe(t, "F122")

	// Records' fields, as gen.go's getField takes them.
	field := func(n, off, size int) []byte { return commonRecord(n, off+size)[off:] }

	tableMark := regexp.MustCompile(`^ mark ROP ([A-Z0-9+]+), key (\w)$`)
	genericMark := regexp.MustCompile(`^ mark key \|(\w+)\| ([A-Z0-9+]+)$`)
	generic4Mark := regexp.MustCompile(`^ mark key 4 \|(\w+)\| ([A-Z0-9+]+)$`)
	revMark := regexp.MustCompile(`^ mark REV, generic (\w+), ([A-Z+]+)$`)
	numberMark := regexp.MustCompile(`^ mark (BN4|IN4|DIN8) near -?(\d+), ([A-Z0-9+]+)$`)

	type search struct {
		ref   int
		value []byte
		rop   string
	}

	read := func(line string) (search, bool) {
		if m := tableMark.FindStringSubmatch(line); m != nil {
			return search{0, []byte(m[2]), m[1]}, true
		}

		if m := genericMark.FindStringSubmatch(line); m != nil {
			return search{0, []byte(m[1]), m[2]}, true
		}

		if m := generic4Mark.FindStringSubmatch(line); m != nil {
			return search{4, []byte(m[1]), m[2]}, true
		}

		if m := revMark.FindStringSubmatch(line); m != nil {
			return search{0, []byte(m[1]), m[2]}, true
		}

		if m := numberMark.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])

			switch m[1] {
			case "BN4":
				return search{1, field(n, 8, 4), m[3]}, true
			case "IN4":
				return search{2, field(n, 12, 4), m[3]}, true
			}

			return search{7, field(n, 44, 8), m[3]}, true
		}

		return search{}, false
	}

	compared := 0

	for _, c := range []struct {
		number int
		file   *File
	}{
		{120, openProbe(t, "F120")}, {121, openProbe(t, "F121")},
		{126, f122}, {127, f122}, {129, f122},
	} {
		lines := log[c.number].lines

		for i := 0; i+1 < len(lines); i++ {
			s, ok := read(lines[i])
			if !ok {
				continue
			}

			want, ok := getAt(lines, i+1)
			if !ok {
				t.Fatalf("case %d: no $GET after %q", c.number, lines[i])
			}

			match, ok := matchOf(s.rop)
			if !ok {
				if want.status != statusBadROP {
					t.Errorf("case %d: %q: status %s", c.number, lines[i], want.status)
				}

				continue
			}

			e, err := c.file.Find(s.ref, s.value, match)
			compareGet(t, fmt.Sprintf("case %d:%s", c.number, lines[i]), c.file, e, err, want)

			compared++
		}
	}

	if compared < 90 {
		t.Errorf("compared only %d searches", compared)
	}
}

// TestExactAgainstVMS replays case 123: each key, record 100 and a record
// that isn't there.
func TestExactAgainstVMS(t *testing.T) {
	f := openProbe(t, "F122")
	field := func(n, off, size int) []byte { return commonRecord(n, off+size)[off:] }

	var gets []oneGet

	lines := readLog(t)[123].lines
	for i := range lines {
		if g, ok := getAt(lines, i); ok {
			gets = append(gets, g)
		}
	}

	searches := []struct{ ref, off, size int }{{0, 0, 8}, {1, 8, 4}, {2, 12, 4}, {3, 16, 8}, {4, 24, 8}, {5, 32, 4}, {6, 36, 8}, {7, 44, 8}}
	if len(gets) != 2*len(searches) {
		t.Fatalf("%d $GETs in the log", len(gets))
	}

	for i, s := range searches {
		for j, n := range []int{100, 100000} {
			e, err := f.Find(s.ref, field(n, s.off, s.size), Equal)
			compareGet(t, fmt.Sprintf("key %d, record %d", s.ref, n), f, e, err, gets[2*i+j])
		}
	}
}

// TestSequenceAgainstVMS replays case 124 (the first six records in each
// key's order) and the start of case 125 (a keyed read, then going on in
// that key's order, among duplicates).
func TestSequenceAgainstVMS(t *testing.T) {
	f := openProbe(t, "F122")
	log := readLog(t)

	var gets []oneGet

	for i, line := range log[124].lines {
		if g, ok := getAt(log[124].lines, i); ok && line != "" {
			gets = append(gets, g)
		}
	}

	if len(gets) != 48 {
		t.Fatalf("%d $GETs in case 124", len(gets))
	}

	for ref := range 8 {
		e, err := f.First(ref)

		for i := range 6 {
			compareGet(t, fmt.Sprintf("key %d, record %d", ref, i), f, e, err, gets[ref*6+i])

			e, err = f.Following(e)
		}
	}

	gets = nil

	for i := range log[125].lines {
		if g, ok := getAt(log[125].lines, i); ok {
			gets = append(gets, g)
		}
	}

	field := func(n, off, size int) []byte { return commonRecord(n, off+size)[off:] }

	e, err := f.Find(3, field(100, 16, 8), Equal)
	compareGet(t, "A value with duplicates", f, e, err, gets[0])

	e, err = f.Following(e)
	compareGet(t, "its second", f, e, err, gets[1])

	e, err = f.Following(e)
	compareGet(t, "its third", f, e, err, gets[2])

	e, err = f.Find(4, field(100, 24, 8), Equal)
	compareGet(t, "a descending key", f, e, err, gets[3])

	e, err = f.Following(e)
	compareGet(t, "going on down it", f, e, err, gets[4])
}

// bruteFind answers a search from the whole list of a key's entries, the
// long way, to check Find's use of the index.
func bruteFind(k *Key, list []Entry, value []byte, match Match) (Entry, error) {
	switch match {
	case EqualOrPrevious, Previous:
		var last *Entry

		for i := range list {
			c := k.comparePrefix(list[i].Key, value)
			if c == 0 && match == EqualOrPrevious {
				return list[i], nil
			}

			if c >= 0 {
				break
			}

			last = &list[i]
		}

		if last == nil {
			return Entry{}, ErrNotFound
		}

		for i := range list {
			if bytes.Equal(list[i].Key, last.Key) {
				return list[i], nil
			}
		}
	}

	for _, e := range list {
		c := k.comparePrefix(e.Key, value)

		switch {
		case c < 0, c == 0 && match == Next:
			continue
		case c == 0, match != Equal:
			return e, nil
		}

		return Entry{}, ErrNotFound
	}

	return Entry{}, ErrNotFound
}

// TestFindUsesTheIndex searches every file by every key, for each record's
// value and for values between them, generic ones too, and compares with
// the answer found by reading the key from its start. Going through the
// index buckets must come to the same record, at every level.
func TestFindUsesTheIndex(t *testing.T) {
	searches := 0

	for name, f := range readable(t) {
		for _, k := range f.Keys {
			list := entriesOf(t, name, f, k.Ref)

			step := 1 + len(list)/40
			for i := 0; i < len(list); i += step {
				for _, value := range valuesNear(k, list[i].Key) {
					for match := Equal; match <= Previous; match++ {
						want, wantErr := bruteFind(k, list, value, match)
						got, err := f.Find(k.Ref, value, match)

						if !errors.Is(err, wantErr) || got.RFA != want.RFA {
							t.Errorf("%s key %d, value %X, match %d: got %v, %v; want %v, %v",
								name, k.Ref, value, match, got.RFA, err, want.RFA, wantErr)
						}

						searches++
					}
				}
			}

			for i := range list {
				if i%step != 0 {
					continue
				}

				want := ErrEnd

				var next Entry

				if i+1 < len(list) {
					next, want = list[i+1], nil
				}

				got, err := f.Following(list[i])
				if !errors.Is(err, want) || got.RFA != next.RFA {
					t.Errorf("%s key %d: after %v: got %v, %v", name, k.Ref, list[i].RFA, got.RFA, err)
				}
			}
		}
	}

	if searches < 5000 {
		t.Errorf("did only %d searches", searches)
	}
}

// valuesNear gives search values around a key value: itself, the next
// higher and lower, and for a string key its leading bytes.
func valuesNear(k *Key, value []byte) [][]byte {
	near := [][]byte{value}

	last := len(value) - 1
	if k.Type.Kind() != String && k.Type.Kind() != Packed {
		last = 0 // the lowest byte of a VAX number
	}

	if value[last] < 0xFF {
		up := bytes.Clone(value)
		up[last]++
		near = append(near, up)
	}

	if value[last] > 0 {
		down := bytes.Clone(value)
		down[last]--
		near = append(near, down)
	}

	if k.Type.Kind() == String && len(value) > 1 {
		near = append(near, value[:1], value[:len(value)-1])
	}

	return near
}

// Patterns in ANALYZE's interactive walks, idx88w1.txt to 3.
var (
	walkSpec   = regexp.MustCompile(`File Spec: DUA1:\[000000\](\w+)\.IDX`)
	walkKey    = regexp.MustCompile(`^KEY DESCRIPTOR #(\d+) `)
	walkBucket = regexp.MustCompile(`^BUCKET HEADER \(VBN (\d+)\)`)
	walkRecord = regexp.MustCompile(`^(PRIMARY DATA|INDEX) RECORD \(VBN (\d+), offset %X'([0-9A-F]+)'\)`)
	walkField  = regexp.MustCompile(`^([A-Za-z -]+?): (%X'[0-9A-F]+'|\d+)(?:, 4-Byte Bucket Pointer: (\d+))?$`)
	walkIndexP = regexp.MustCompile(`^2-Byte Bucket Pointer: (\d+)$`)
)

// TestBucketsAgainstANALYZE holds the decoding of buckets and records to
// what VMS's ANALYZE/RMS_FILE/INTERACTIVE named in each, in the three
// scripted walks: every header's fields, every index record's pointer, and
// every data record's ID and RFA.
func TestBucketsAgainstANALYZE(t *testing.T) {
	files := readable(t)
	compared := 0

	for _, walk := range []string{"idx88w1.txt", "idx88w2.txt", "idx88w3.txt"} {
		text, err := os.ReadFile(filepath.Join(vaxDir, walk))
		if err != nil {
			t.Fatal(err)
		}

		m := walkSpec.FindSubmatch(text)
		if m == nil {
			t.Fatalf("%s: no file spec", walk)
		}

		f := files[string(m[1])]
		k := f.Keys[0]

		var b *bucket

		var data []dataRecord

		var index []indexEntry

		var record *dataRecord

		var entry *indexEntry

		number := func(s string) int { return anaNumber(s) }

		for _, line := range strings.Split(string(text), "\n") {
			line = strings.TrimSpace(line)

			if h := walkKey.FindStringSubmatch(line); h != nil {
				k = f.Keys[number(h[1])]

				continue
			}

			if h := walkBucket.FindStringSubmatch(line); h != nil {
				vbn := uint32(number(h[1]))

				// The report says a bucket's level a few lines on, and an
				// index bucket's size may differ from a data bucket's. The
				// level is in the first block, so look there first.
				peek := make([]byte, BlockSize)
				if err := f.blocks.ReadBlock(vbn, peek); err != nil {
					t.Fatalf("%s: %v", walk, err)
				}

				size := k.DataBucketSize
				if peek[bktLevel] != 0 {
					size = k.IndexBucketSize
				}

				b, err = f.readBucket(vbn, size)
				if err != nil {
					t.Fatalf("%s: %v", walk, err)
				}

				record, entry = nil, nil

				if b.level == 0 {
					data, err = f.dataRecords(b, k)
				} else {
					index, err = b.indexEntries(k)
				}

				if err != nil {
					t.Fatalf("%s: %v", walk, err)
				}

				continue
			}

			if h := walkRecord.FindStringSubmatch(line); h != nil {
				offset := anaNumber("%X'" + h[3] + "'")
				record, entry = nil, nil

				for i := range data {
					if data[i].offset == offset && h[1] == "PRIMARY DATA" {
						record = &data[i]
					}
				}

				for i := range index {
					if index[i].offset == offset && h[1] == "INDEX" {
						entry = &index[i]
					}
				}

				if record == nil && entry == nil {
					t.Errorf("%s: no %s record at %s", walk, h[1], h[3])
				}

				continue
			}

			if h := walkIndexP.FindStringSubmatch(line); h != nil && entry != nil {
				if int(entry.pointer) != number(h[1]) {
					t.Errorf("%s: index record at %X points to %d, VMS has %s", walk, entry.offset, entry.pointer, h[1])
				}

				compared++

				continue
			}

			h := walkField.FindStringSubmatch(line)
			if h == nil || b == nil {
				continue
			}

			want := number(h[2])
			got := -1

			switch h[1] {
			case "Check Character":
				got = int(b.check)
			case "Key of Reference":
				got = b.indexNo
			case "VBN Sample":
				got = int(b.sample)
			case "Free Space Offset":
				got = b.free
			case "Free Record ID":
				got = int(b.nextID)
			case "Next Bucket VBN":
				got = int(b.next)
			case "Level":
				got = b.level
			case "Bucket Pointer Size":
				got = b.pointerSize()
			case "Record ID":
				if record != nil {
					got = int(record.id)
				}
			case "RRV ID":
				if record != nil {
					got = int(record.rfa.ID)

					if int(record.rfa.VBN) != number(h[3]) {
						t.Errorf("%s: record at %X has RFA VBN %d, VMS %s", walk, record.offset, record.rfa.VBN, h[3])
					}
				}
			default:
				continue
			}

			if got == -1 && h[1] != "Record ID" && h[1] != "RRV ID" {
				t.Errorf("%s: %s: nothing to compare", walk, h[1])
			}

			if got != -1 && got != want {
				t.Errorf("%s: bucket %d, %s: %d, VMS has %d", walk, b.vbn, h[1], got, want)
			}

			compared++
		}
	}

	if compared < 50 {
		t.Errorf("compared only %d fields", compared)
	}
}
