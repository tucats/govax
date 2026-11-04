package isam

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// volumeCluster is the exchange volume's cluster size in the probe's run:
// VMS's files grow 3 blocks at a time however little is asked for.
const volumeCluster = 3

// growingBlocks is a file in memory that can be written and extended, on a
// volume of a given cluster size. An extension's blocks read as zeros.
type growingBlocks struct {
	data    []byte
	cluster uint32
}

func (g *growingBlocks) ReadBlock(vbn uint32, block []byte) error {
	return memoryBlocks(g.data).ReadBlock(vbn, block)
}

func (g *growingBlocks) WriteBlock(vbn uint32, block []byte) error {
	at := int(vbn-1) * BlockSize
	if vbn == 0 || at+len(block) > len(g.data) {
		return fmt.Errorf("VBN %d is past the file's end", vbn)
	}

	copy(g.data[at:], block)

	return nil
}

func (g *growingBlocks) Size() uint32 { return uint32(len(g.data) / BlockSize) }

func (g *growingBlocks) Extend(count uint32) (uint32, uint32, error) {
	got := count
	if rest := got % g.cluster; rest != 0 {
		got += g.cluster - rest
	}

	first := uint32(len(g.data)/BlockSize) + 1
	g.data = append(g.data, make([]byte, int(got)*BlockSize)...)

	return first, got, nil
}

// specOf makes the Spec a VMS-made file was created from, out of what its
// descriptors say. Allocation is the first extent asked for: alq, the
// $CREATE's FAB, for a file of one area, and with more an area's extent,
// none of the cases that use them having grown one.
func specOf(f *File, alq uint32) Spec {
	spec := Spec{Attributes: f.attr}

	for _, a := range f.Areas {
		spec.Areas = append(spec.Areas, AreaSpec{
			BucketSize: a.BucketSize,
			Allocation: a.ExtentSize,
			Extend:     a.ExtendSize,
			Alignment:  a.Alignment,
			Options:    a.Options,
			Volume:     a.Volume,
			Location:   a.Location,
		})
	}

	if len(spec.Areas) == 1 {
		spec.Areas[0].Allocation = alq
	}

	for _, k := range f.Keys {
		spec.Keys = append(spec.Keys, KeySpec{
			Segments:   k.Segments,
			Type:       k.Type,
			Flags:      k.Flags &^ FlagNoIndexYet,
			Null:       k.Null,
			IndexArea:  k.IndexArea,
			Level1Area: k.Level1Area,
			DataArea:   k.DataArea,
			IndexFill:  k.IndexFill,
			DataFill:   k.DataFill,
			Name:       k.Name,
		})
	}

	return spec
}

var (
	logCreate = regexp.MustCompile(`^ fab org=20 .* alq=(\d+) `)
	logAnyPut = regexp.MustCompile(`^=\$PUT n=(\d+) size=(\d+) r=([0-9A-F]+)$`)
	logVBN    = regexp.MustCompile(`^ vbn (\d+)$`)
	logBytes  = regexp.MustCompile(`^   ([0-9A-F]{3}) ([0-9A-F]+)$`)
	logMark   = regexp.MustCompile(`^ mark after record \d+, the (\d+)th$`)
)

// RMS's statuses for a $PUT, in the log.
const (
	rmsSuccess   = "00010001"
	rmsOKDup     = "00018011"
	rmsDuplicate = "000184EC"
	rmsSequence  = "000186AC"
)

// putLine is one $PUT in the log.
type putLine struct {
	n, size int
	status  string
	rfa     RFA
}

// putsOf reads a stretch of a case's log for its $PUTs, each followed by its
// RAB's line. A RAC SEQ failure isn't isam's: RMS checks the order.
func putsOf(lines []string) []putLine {
	var puts []putLine

	for i, line := range lines {
		m := logAnyPut.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		n, _ := strconv.Atoi(m[1])
		size, _ := strconv.Atoi(m[2])

		p := putLine{n: n, size: size, status: m[3]}

		if r := logRAB.FindStringSubmatch(lines[i+1]); r != nil {
			vbn, _ := strconv.Atoi(r[1])
			id, _ := strconv.Atoi(r[2])
			p.rfa = RFA{VBN: uint32(vbn), ID: uint16(id)}
		}

		puts = append(puts, p)
	}

	return puts
}

// blocksIn rebuilds a file from one of the log's dumps: its " vbn n" lines
// and their hex, all-zero lines left out.
func blocksIn(lines []string) memoryBlocks {
	var data []byte

	base := 0

	for _, line := range lines {
		if m := logVBN.FindStringSubmatch(line); m != nil {
			vbn, _ := strconv.Atoi(m[1])
			base = (vbn - 1) * BlockSize
			data = append(data, make([]byte, vbn*BlockSize-len(data))...)

			continue
		}

		if m := logBytes.FindStringSubmatch(line); m != nil {
			offset, _ := strconv.ParseInt(m[1], 16, 32)
			raw, _ := hex.DecodeString(m[2])
			copy(data[base+int(offset):], raw)
		}
	}

	return data
}

// replayPuts does a stretch of the log's $PUTs on a file, comparing each
// status and RFA.
func replayPuts(t *testing.T, name string, f *File, puts []putLine) {
	t.Helper()

	for _, p := range puts {
		if p.status == rmsSequence {
			continue
		}

		result, err := f.Put(commonRecord(p.n, p.size))
		what := fmt.Sprintf("%s: $PUT n=%d", name, p.n)

		switch p.status {
		case rmsSuccess, rmsOKDup:
			if err != nil {
				t.Errorf("%s: %v, VMS said %s", what, err, p.status)

				continue
			}

			if result.RFA != p.rfa {
				t.Errorf("%s: RFA %v, VMS has %v", what, result.RFA, p.rfa)
			}

			if result.Duplicate != (p.status == rmsOKDup) {
				t.Errorf("%s: duplicate %v, VMS said %s", what, result.Duplicate, p.status)
			}
		case rmsDuplicate:
			if !errors.Is(err, ErrDuplicate) {
				t.Errorf("%s: %v, VMS said RMS$_DUP", what, err)
			}
		default:
			t.Errorf("%s: a status this test doesn't know, %s", what, p.status)
		}
	}
}

// compareFiles holds a file to VMS's: first by Dump, whose lines show what
// differs, then byte for byte. The phase doc's answer 3 asks only for the
// first, unused bytes being undefined, but a file written in one go has
// none but those Put leaves, and VMS leaves the same.
func compareFiles(t *testing.T, name string, ours *File, vms *File) {
	t.Helper()

	var mine, theirs bytes.Buffer

	if err := ours.Dump(&mine); err != nil {
		t.Errorf("%s: dump: %v", name, err)

		return
	}

	if err := vms.Dump(&theirs); err != nil {
		t.Errorf("%s: VMS's dump: %v", name, err)

		return
	}

	if mine.String() == theirs.String() {
		return
	}

	a, b := strings.Split(mine.String(), "\n"), strings.Split(theirs.String(), "\n")

	for i := range a {
		if i >= len(b) || a[i] != b[i] {
			other := ""
			if i < len(b) {
				other = b[i]
			}

			t.Errorf("%s: line %d of the dump:\n ours %s\n VMS  %s", name, i+1, a[i], other)

			return
		}
	}

	t.Errorf("%s: VMS's dump is longer", name)
}

// sameBytes compares two files' every block.
func sameBytes(t *testing.T, name string, ours []byte, vms []byte) {
	t.Helper()

	blocks := max(len(ours), len(vms)) / BlockSize

	for vbn := range blocks {
		at := vbn * BlockSize

		var mine, theirs [BlockSize]byte

		if at < len(ours) {
			copy(mine[:], ours[at:])
		}

		if at < len(vms) {
			copy(theirs[:], vms[at:])
		}

		for i := range mine {
			if mine[i] != theirs[i] {
				t.Errorf("%s: VBN %d, offset %X: ours %02X, VMS has %02X", name, vbn+1, i, mine[i], theirs[i])

				return
			}
		}
	}
}

// TestShapesAsVMS makes each file of the probe's "key shapes" cases, every
// kind of key and area with no splits, by the same $PUTs VMS was given. The
// result must match VMS's file byte for byte, and the Dump first so a
// difference can be read.
func TestShapesAsVMS(t *testing.T) {
	log := readLog(t)
	files := readable(t)
	raw := probeFiles(t)
	made := 0

	bytesOf := map[string]memoryBlocks{}
	for _, v := range raw {
		bytesOf[v.name] = v.blocks
	}

	for c := 1; c <= 65; c++ {
		name := "S" + strconv.Itoa(c)

		vms, ok := files[name]
		if !ok {
			continue // a lower prolog
		}

		lines := log[c].lines

		m := logCreate.FindStringSubmatch(lines[1])
		if m == nil {
			t.Fatalf("%s: no $CREATE in its case", name)
		}

		alq, _ := strconv.Atoi(m[1])

		g := &growingBlocks{cluster: volumeCluster}

		f, err := Create(g, specOf(vms, uint32(alq)))
		if err != nil {
			t.Errorf("%s: %v", name, err)

			continue
		}

		replayPuts(t, name, f, putsOf(lines))

		reopened, err := Open(g, vms.attr)
		if err != nil {
			t.Errorf("%s: reopening: %v", name, err)

			continue
		}

		compareFiles(t, name, reopened, vms)
		sameBytes(t, name, g.data, bytesOf[name])

		made++
	}

	if made < 60 {
		t.Errorf("made only %d files", made)
	}
}

// growthCases are the probe's cases that dump the file after every $PUT.
var growthCases = []int{104, 105, 106, 107, 108, 109, 110, 111, 112, 113}

// TestGrowthStepsAsVMS follows the growth cases a step at a time, each
// $PUT on a file closed and opened again, through every split. After each
// step the file must be what VMS's dump of it was then, so it tests a first
// record, an index made, IDs, and check characters at every step, on files
// that were reopened from their bytes.
func TestGrowthStepsAsVMS(t *testing.T) {
	log := readLog(t)
	steps := 0

	for _, c := range growthCases {
		name := fmt.Sprintf("G%d", c)
		lines := log[c].lines

		m := logCreate.FindStringSubmatch(lines[1])
		if m == nil {
			t.Fatalf("%s: no $CREATE in its case", name)
		}

		alq, _ := strconv.Atoi(m[1])

		// The descriptors come from the file VMS finished with, since no step
		// changes what the spec reads.
		final := readable(t)[name]
		if final == nil {
			t.Fatalf("%s isn't in the save set", name)
		}

		g := &growingBlocks{cluster: volumeCluster}

		f, err := Create(g, specOf(final, uint32(alq)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		start, split := 0, false

		for i := range lines {
			if !logMark.MatchString(lines[i]) {
				continue
			}

			replayPuts(t, name, f, putsOf(lines[start:i]))

			end := i
			for end < len(lines) && !strings.HasPrefix(lines[end], "=dump") {
				end++
			}

			vms := blocksIn(lines[i:end])
			step := fmt.Sprintf("%s, %s", name, strings.TrimSpace(lines[i]))

			want, err := Open(vms, final.attr)
			if err != nil {
				t.Errorf("%s: VMS's: %v", step, err)

				break
			}

			f, err = Open(g, final.attr)
			if err != nil {
				t.Errorf("%s: %v", step, err)

				break
			}

			compareFiles(t, step, f, want)

			// Before the first split nothing has moved, and the bytes past a free
			// offset are the same too. A split leaves VMS's working bytes there,
			// which the phase doc's answer 3 sets aside.
			if !split {
				if f.Areas[0].TotalAllocation != want.Areas[0].TotalAllocation {
					t.Errorf("%s: allocation", step)
				}

				if levelZeroBuckets(t, want, 0) > 1 {
					split = true
				} else {
					sameBytes(t, step, g.data, vms)
				}
			}

			steps++
			start = end
		}
	}

	if steps < 30 {
		t.Errorf("only %d steps compared", steps)
	}
}

// TestCompressionIsVMSs compresses every record and key of every file VMS
// made, each against the one before it, and must get the bytes VMS stored.
func TestCompressionIsVMSs(t *testing.T) {
	compared := 0

	for name, f := range readable(t) {
		n, err := f.CheckCompression()
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}

		compared += n
	}

	if compared < 5000 {
		t.Errorf("compared only %d", compared)
	}
}

// oneKey is a Spec for the unit tests below.
func oneKey(flags uint8) Spec {
	return Spec{
		Attributes: Attributes{Format: Variable, Size: 200},
		Areas:      []AreaSpec{{BucketSize: 2}},
		Keys:       []KeySpec{{Segments: []Segment{{Position: 0, Size: 8}}, Flags: flags}},
	}
}

// TestPutRefuses covers the records Put must leave the file alone for.
func TestPutRefuses(t *testing.T) {
	g := &growingBlocks{cluster: volumeCluster}

	f, err := Create(g, oneKey(FlagKeyCompress|FlagIndexCompress|FlagRecordCompress))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.Put([]byte("short")); !errors.Is(err, ErrRecordSize) {
		t.Errorf("a record short of its key: %v", err)
	}

	if _, err := f.Put(bytes.Repeat([]byte{'x'}, 8)); err != nil {
		t.Fatal(err)
	}

	before := bytes.Clone(g.data)

	if _, err := f.Put(bytes.Repeat([]byte{'x'}, 8)); !errors.Is(err, ErrDuplicate) {
		t.Errorf("a duplicate: %v", err)
	}

	if !bytes.Equal(before, g.data) {
		t.Error("a refused duplicate of the primary key changed the file")
	}

	big := make([]byte, 2000)
	for i := range big {
		big[i] = byte(i * 7)
	}

	if _, err := f.Put(big); !errors.Is(err, ErrRecordSize) {
		t.Errorf("a record past its bucket: %v", err)
	}
}

// TestBackOutOfAlternates puts a record that's refused at its second
// alternate, so its pointer must come out of the first, and the record out
// of the data bucket. No VMS case has two alternates here, so what's held is
// only that the file reads the same as before but for what the attempt
// spent, an ID and the check characters.
func TestBackOutOfAlternates(t *testing.T) {
	g := &growingBlocks{cluster: volumeCluster}

	spec := oneKey(FlagKeyCompress | FlagIndexCompress | FlagRecordCompress)
	spec.Attributes.Size = 100
	spec.Keys = append(spec.Keys,
		KeySpec{Segments: []Segment{{Position: 8, Size: 8}}, Flags: FlagDuplicates | FlagKeyCompress | FlagIndexCompress},
		KeySpec{Segments: []Segment{{Position: 16, Size: 8}}, Flags: FlagKeyCompress | FlagIndexCompress})

	f, err := Create(g, spec)
	if err != nil {
		t.Fatal(err)
	}

	record := func(a, b, c string) []byte { return []byte(a + b + c) }

	if _, err := f.Put(record("AAAAAAA1", "BBBBBBB1", "CCCCCCC1")); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Put(record("AAAAAAA2", "BBBBBBB1", "CCCCCCC2")); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Put(record("AAAAAAA3", "BBBBBBB1", "CCCCCCC1")); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("a duplicate of the second alternate: %v", err)
	}

	if _, err := f.Put(record("AAAAAAA4", "BBBBBBB2", "CCCCCCC2")); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("a duplicate, alone in the first alternate: %v", err)
	}

	for ref, want := range []int{2, 2, 2} {
		count := 0

		if err := f.Walk(ref, func(Entry) bool {
			count++

			return true
		}); err != nil {
			t.Fatal(err)
		}

		if count != want {
			t.Errorf("key %d has %d entries, not %d", ref, count, want)
		}
	}

	r, err := f.Find(1, []byte("BBBBBBB2"), Equal)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("a backed-out value is still there: %v, %v", r, err)
	}
}

// TestUnwritable shows Put needs a file it can write.
func TestUnwritable(t *testing.T) {
	g := &growingBlocks{cluster: volumeCluster}

	if _, err := Create(g, oneKey(0)); err != nil {
		t.Fatal(err)
	}

	f, err := Open(memoryBlocks(g.data), Attributes{Size: 200})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.Put(make([]byte, 20)); !errors.Is(err, ErrReadOnly) {
		t.Errorf("%v", err)
	}
}

// TestBadSpecs covers what Create refuses.
func TestBadSpecs(t *testing.T) {
	for name, change := range map[string]func(*Spec){
		"no keys":      func(s *Spec) { s.Keys = nil },
		"no areas":     func(s *Spec) { s.Areas = nil },
		"bucket 0":     func(s *Spec) { s.Areas[0].BucketSize = 0 },
		"bucket 64":    func(s *Spec) { s.Areas[0].BucketSize = 64 },
		"no segments":  func(s *Spec) { s.Keys[0].Segments = nil },
		"missing area": func(s *Spec) { s.Keys[0].DataArea = 1 },
		"long name":    func(s *Spec) { s.Keys[0].Name = strings.Repeat("N", 33) },
	} {
		spec := oneKey(0)
		change(&spec)

		if _, err := Create(&growingBlocks{cluster: 1}, spec); !errors.Is(err, ErrBadSpec) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestPrologLayout checks where descriptors go with many keys: 5 to a block
// from VBN 2 is as many as fit, past what VMS's files show, and whatever the
// count Open must read back what Create wrote.
func TestPrologLayout(t *testing.T) {
	spec := oneKey(0)
	for i := 1; i <= 12; i++ {
		spec.Keys = append(spec.Keys, KeySpec{Segments: []Segment{{Position: i * 8, Size: 8}}, Flags: FlagDuplicates})
	}

	for i := 1; i < 10; i++ {
		spec.Areas = append(spec.Areas, AreaSpec{BucketSize: i, Allocation: 3})
	}

	g := &growingBlocks{cluster: volumeCluster}

	if _, err := Create(g, spec); err != nil {
		t.Fatal(err)
	}

	f, err := Open(g, spec.Attributes)
	if err != nil {
		t.Fatal(err)
	}

	if len(f.Keys) != 13 || len(f.Areas) != 10 {
		t.Fatalf("%d keys and %d areas", len(f.Keys), len(f.Areas))
	}

	if f.AreaVBN != 5 {
		t.Errorf("areas at VBN %d", f.AreaVBN)
	}

	if a := f.Areas[0]; a.Used != 6 || a.NextVBN != 7 {
		t.Errorf("area 0: %d used, next %d: its prolog is 6 blocks", a.Used, a.NextVBN)
	}

	if got := binary.LittleEndian.Uint32(g.data[keyNextVBN:]); got != 2 {
		t.Errorf("key 0's next is VBN %d", got)
	}
}

// levelZeroBuckets counts a key's level 0 buckets.
func levelZeroBuckets(t *testing.T, f *File, ref int) int {
	t.Helper()

	k := f.Keys[ref]
	count := 0

	for vbn := k.FirstData; ; count++ {
		z, err := f.readLevelZero(k, vbn)
		if err != nil {
			t.Fatal(err)
		}

		if z.bucket.isLast() {
			return count + 1
		}

		vbn = z.bucket.next
	}
}

// bulkCases are the probe's cases that load many records and dump the file only
// at the end: the record numbers as gen.go's puts makes them. exact says the
// file comes out as VMS's in every structure. For the two scattered loads it
// doesn't, as the phase doc's "What the splits settled" says, and what's held
// instead is below.
var bulkCases = []struct {
	c                             int
	first, count, step, mod, size int
	exact                         bool
}{
	{114, 1, 400, 1, 0, 150, true},
	{115, 1, 1200, 1, 0, 200, true},
	{116, 1, 1500, 1, 0, 200, true},
	{117, 1, 500, 211, 997, 100, false},
	{118, 1, 500, 211, 997, 100, false},
}

// makeBulk loads one of the bulk cases' files.
func makeBulk(t *testing.T, log map[int]*logCase, vms *File, i int) *File {
	t.Helper()

	b := bulkCases[i]

	m := logCreate.FindStringSubmatch(log[b.c].lines[1])
	if m == nil {
		t.Fatalf("case %d: no $CREATE", b.c)
	}

	alq, _ := strconv.Atoi(m[1])
	g := &growingBlocks{cluster: volumeCluster}

	f, err := Create(g, specOf(vms, uint32(alq)))
	if err != nil {
		t.Fatal(err)
	}

	for n := 0; n < b.count; n++ {
		v := b.first + n*b.step
		if b.mod != 0 {
			v = v%b.mod + 1
		}

		if _, err := f.Put(commonRecord(v, b.size)); err != nil {
			t.Fatalf("case %d: record %d: %v", b.c, v, err)
		}
	}

	reopened, err := Open(g, vms.attr)
	if err != nil {
		t.Fatal(err)
	}

	return reopened
}

// TestBulkLoadsAsVMS makes the large cases' files: three index levels, root
// splits, index compression, and SIDR arrays past a bucket. The in-order ones
// must be VMS's in every structure. Every one must pass Check, and hold just
// what VMS's holds by each key.
func TestBulkLoadsAsVMS(t *testing.T) {
	log := readLog(t)
	files := readable(t)

	for i, b := range bulkCases {
		name := fmt.Sprintf("G%d", b.c)
		vms := files[name]
		f := makeBulk(t, log, vms, i)

		if b.exact {
			compareFiles(t, name, f, vms)
		}

		if err := f.Check(); err != nil {
			t.Errorf("%s: %v", name, err)
		}

		for ref := range vms.Keys {
			ours, theirs := keysIn(t, f, ref), keysIn(t, vms, ref)
			if ours != theirs {
				t.Errorf("%s: key %d's values differ from VMS's", name, ref)
			}
		}
	}
}

// keysIn lists a key's values in order, as text to compare.
func keysIn(t *testing.T, f *File, ref int) string {
	t.Helper()

	var out strings.Builder

	if err := f.Walk(ref, func(e Entry) bool {
		fmt.Fprintf(&out, "%X\n", e.Key)

		return true
	}); err != nil {
		t.Fatal(err)
	}

	return out.String()
}

// TestScatteredDataAsVMS holds what the scattered load without compression does
// match: every data bucket of its primary key has VMS's records, IDs, RRVs,
// and check character, in VMS's order, whatever their VBNs. Its SIDR buckets
// don't all match, which moves the VBNs: the phase doc has it.
func TestScatteredDataAsVMS(t *testing.T) {
	const scattered = 4

	vms := readable(t)["G118"]
	f := makeBulk(t, readLog(t), vms, scattered)

	ours, theirs := primaryShape(t, f), primaryShape(t, vms)
	if len(ours) != len(theirs) {
		t.Fatalf("%d data buckets, VMS has %d", len(ours), len(theirs))
	}

	for i := range ours {
		if ours[i] != theirs[i] {
			t.Errorf("data bucket %d:\n ours %s\n VMS  %s", i, ours[i], theirs[i])
		}
	}
}

// primaryShape describes each of the primary key's data buckets without any VBN.
func primaryShape(t *testing.T, f *File) []string {
	t.Helper()

	var shape []string

	k := f.Keys[0]

	for vbn := k.FirstData; ; {
		z, err := f.readLevelZero(k, vbn)
		if err != nil {
			t.Fatal(err)
		}

		line := fmt.Sprintf("check %02X free %X next ID %d:", z.bucket.check, z.bucket.free, z.bucket.nextID)

		for i := range z.primary {
			r := &z.primary[i]
			if r.isRRV() {
				line += fmt.Sprintf(" RRV %d", r.id)
			} else {
				line += fmt.Sprintf(" %d=%X", r.id, r.key)
			}
		}

		shape = append(shape, line)

		if z.bucket.isLast() {
			return shape
		}

		vbn = z.bucket.next
	}
}

// TestLoadAsVMS is ROP LOA, case 119: records in order only up to the fill size,
// and later records, without LOA, filling past it.
func TestLoadAsVMS(t *testing.T) {
	lines := readLog(t)[119].lines
	final := readable(t)["G119"]

	var dumps [][]string

	start := 0

	for i, line := range lines {
		if strings.HasPrefix(line, "=dump") {
			dumps = append(dumps, lines[start:i])
			start = i
		}
	}

	if len(dumps) != 2 {
		t.Fatalf("%d dumps", len(dumps))
	}

	g := &growingBlocks{cluster: volumeCluster}

	f, err := Create(g, specOf(final, 3))
	if err != nil {
		t.Fatal(err)
	}

	for n := 1; n <= 15; n += 2 {
		if _, err := f.PutLoad(commonRecord(n, 100)); err != nil {
			t.Fatal(err)
		}
	}

	want, err := Open(blocksIn(dumps[0]), final.attr)
	if err != nil {
		t.Fatal(err)
	}

	compareFiles(t, "after the load", f, want)

	for n := 2; n <= 16; n += 2 {
		if _, err := f.Put(commonRecord(n, 100)); err != nil {
			t.Fatal(err)
		}
	}

	compareFiles(t, "after the rest", f, final)
}

// TestExtendArea is $EXTEND's way into an area: the blocks become the extent
// to come, buckets are taken from it once the current extent is used, and the
// file doesn't grow again until it's used too.
func TestExtendArea(t *testing.T) {
	g := &growingBlocks{cluster: 1}

	f, err := Create(g, oneKey(0))
	if err != nil {
		t.Fatal(err)
	}

	got, err := f.ExtendArea(0, 20)
	if err != nil || got != 20 {
		t.Fatalf("ExtendArea: %d, %v", got, err)
	}

	a := f.Areas[0]
	if a.NextExtentSize != 20 || a.TotalAllocation != a.ExtentSize+20 {
		t.Fatalf("area after ExtendArea: %+v", a)
	}

	next, size := a.NextExtent, a.NextExtentSize
	blocks := len(g.data) / BlockSize

	// Records of 150 bytes, six to a 2-block bucket, until a bucket comes
	// from the extent to come.
	for n := 1; f.Areas[0].ExtentVBN != next; n++ {
		record := []byte(fmt.Sprintf("K%07d%s", n, strings.Repeat("x", 142)))
		if _, err := f.Put(record); err != nil {
			t.Fatalf("Put %d: %v", n, err)
		}

		if n > 200 {
			t.Fatal("no bucket came from the extent to come")
		}
	}

	if f.Areas[0].NextExtent != 0 || f.Areas[0].ExtentSize != size || len(g.data)/BlockSize != blocks {
		t.Errorf("area %+v, file %d blocks, was %d", f.Areas[0], len(g.data)/BlockSize, blocks)
	}

	reopened, err := Open(g, f.attr)
	if err != nil {
		t.Fatal(err)
	}

	if reopened.Areas[0] != f.Areas[0] {
		t.Errorf("the descriptor says %+v, the file %+v", reopened.Areas[0], f.Areas[0])
	}

	if err := reopened.Check(); err != nil {
		t.Error(err)
	}

	// The extent to come lengthens when the next extension follows it; one
	// that doesn't is refused while it's waiting.
	if _, err := f.ExtendArea(0, 4); err != nil {
		t.Fatal(err)
	}

	if _, err := f.ExtendArea(0, 4); err != nil || f.Areas[0].NextExtentSize != 8 {
		t.Errorf("a second extension: %v, %+v", err, f.Areas[0])
	}

	if _, _, err := g.Extend(1); err != nil {
		t.Fatal(err)
	}

	if _, err := f.ExtendArea(0, 4); !errors.Is(err, ErrBadSpec) {
		t.Errorf("an extension apart from the one waiting: %v", err)
	}
}
