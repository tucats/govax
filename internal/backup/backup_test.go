package backup

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The save sets VMS V7.1 wrote for Phase 69 (testdata/backup69/vax): pi.bck,
// the first, and the probe's twenty.
const vaxDir = "../../testdata/backup69/vax"

// probeSets are the probe's save sets, SSCOPY's damaged copies last.
var probeSets = []string{
	"b69", "b69k8", "b69g0", "b69g3", "b69crc", "b69nocrc", "b69com",
	"b69exc", "b69byo", "b69txt", "b69none", "text", "b69own", "list",
	"b69ign", "b69cut", "b69crcd", "b69g0d", "b69nocrcd", "b69hdrd",
}

// load reads a fixture.
func load(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(vaxDir, name))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// readSet reads a fixture save set.
func readSet(t *testing.T, name string, opts Options) *SaveSet {
	t.Helper()

	data := load(t, name+".bck")

	s, err := Read(bytes.NewReader(data), int64(len(data)), opts)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	return s
}

// file finds a save set's file by name.
func file(t *testing.T, s *SaveSet, name string) *File {
	t.Helper()

	for _, f := range s.Files {
		if f.Name == name {
			return f
		}
	}

	t.Fatalf("no %s in the save set", name)

	return nil
}

// TestPISaveSet reads the first save set: its summary is VMS's listing's,
// and its three files are the originals, attribute for attribute and
// byte for byte.
func TestPISaveSet(t *testing.T) {
	s := readSet(t, "pi", Options{})

	if s.BlockSize != 32256 || s.Blocks != 2 || len(s.Recovered)+len(s.Lost) != 0 {
		t.Errorf("block size %d, %d blocks, recovered %v, lost %v", s.BlockSize, s.Blocks, s.Recovered, s.Lost)
	}

	sum := s.Summary
	want := Summary{
		Name: "PI.BCK", Command: "BACKUP [.PI]*.* PI.BCK/SAVE", UserName: "SYSTEM      ",
		UserUIC: 0x00010004, OpSys: OpSysVAX, SystemVersion: "V7.1", CPUID: 0x040704D2,
		Device: "_SIMVAX$DUA1:", BackupVersion: "V6.2", BlockSize: 32256, GroupSize: 10, Buffers: 8,
	}

	sum.Date, sum.List = 0, nil
	if !reflect.DeepEqual(sum, want) {
		t.Errorf("summary\n got %+v\nwant %+v", sum, want)
	}

	// 20-OCT-2026 10:45:22.88, in 100 ns units since 17-NOV-1858.
	if got := s.Summary.Date / 100000 % (24 * 3600 * 100); got != (10*3600+45*60+22)*100+88 {
		t.Errorf("summary time of day %d centiseconds", got)
	}

	type expect struct {
		name            string
		fid             [3]uint16
		allocated, used uint32
		format, rattr   uint8
		rsize, extend   uint16
		revision        uint16
		characteristics uint32
		original        string
	}

	for _, e := range []expect{
		{"[XFER.PI]PI.EXE;1", [3]uint16{19, 1, 1}, 6, 6, 1, 0, 512, 0, 1, CharContiguousBestTry, "pi.exe"},
		{"[XFER.PI]PI.MAR;1", [3]uint16{18, 1, 1}, 24, 24, 2, 2, 79, 0, 2, 0, "pi.mar"},
		{"[XFER.PI]PI.OBJ;1", [3]uint16{16, 2, 1}, 6, 5, 2, 0, 214, 20, 1, 0, "pi.obj"},
	} {
		f := file(t, s, e.name)
		if f.FID != e.fid || f.BackLink != [3]uint16{17, 1, 0} || f.Allocated != e.allocated ||
			f.Used() != e.used || f.RecordFormat() != e.format || f.RecordAttr() != e.rattr ||
			f.RecordSize() != e.rsize || f.Extend() != e.extend || f.Revision != e.revision ||
			f.Characteristics != e.characteristics {
			t.Errorf("%s: FID %v back link %v, %d/%d blocks, format %d attributes %d size %d extend %d, revision %d, characteristics %#x",
				e.name, f.FID, f.BackLink, f.Used(), f.Allocated, f.RecordFormat(), f.RecordAttr(),
				f.RecordSize(), f.Extend(), f.Revision, f.Characteristics)
		}

		if f.Owner != 0x00010004 || f.Protection != 0xFA00 || f.DirProtection != 0xBA88 ||
			f.VersionLimit != 32767 || f.DirVersionLimit != 32767 || f.Highwater != e.allocated+1 ||
			f.StructureLevel != 0x0201 || f.Created == 0 || f.Revised < f.Created || f.Expires != 0 {
			t.Errorf("%s: owner %#x protection %#x directory's %#x, limits %d %d, highwater %d, level %#x, times %d %d %d",
				e.name, f.Owner, f.Protection, f.DirProtection, f.VersionLimit, f.DirVersionLimit,
				f.Highwater, f.StructureLevel, f.Created, f.Revised, f.Expires)
		}

		if !bytes.Equal(f.Data(), load(t, e.original)) || f.Missing() != nil {
			t.Errorf("%s: data differs from %s, or blocks are missing (%v)", e.name, e.original, f.Missing())
		}
	}
}

// TestChecks: every block VMS wrote passes its header checksum, and
// every block of B69CRC its data CRC, but for the two blocks SSCOPY
// damaged. The others say they have no CRC.
func TestChecks(t *testing.T) {
	for _, name := range append([]string{"pi"}, probeSets...) {
		data := load(t, name+".bck")
		size := int(binary.LittleEndian.Uint32(data[bhdBlockSize:]))

		for at := 0; at+size <= len(data); at += size {
			block := data[at : at+size]
			number := at/size + 1
			h := ParseHeader(block)

			if h.Number != uint32(number) && !(name == "b69hdrd" && number == 2) {
				t.Errorf("%s block %d: numbered %d", name, number, h.Number)
			}

			damagedHeader := name == "b69hdrd" && number == 2
			if headerIntact(block) == damagedHeader {
				t.Errorf("%s block %d: header checksum %#04x, computed %#04x", name, number, h.Checksum, HeaderChecksum(block))
			}

			withCRC := name == "b69crc" || name == "b69crcd"
			if h.HasCRC() != withCRC {
				t.Errorf("%s block %d: flags %#x", name, number, h.Flags)
			}

			damagedData := name == "b69crcd" && number == 2
			if withCRC && (h.CRC == DataCRC(block)) == damagedData {
				t.Errorf("%s block %d: CRC %#08x, computed %#08x", name, number, h.CRC, DataCRC(block))
			}
		}
	}
}

// listing is one save set's file lines in VMS's BACKUP/LIST.
type listing struct {
	names []string
	sizes []uint32
	total uint32
}

// vaxListings reads the probe log's plain listings of each save set,
// keyed by the save set's name in lowercase.
func vaxListings(t *testing.T) map[string]listing {
	t.Helper()

	command := regexp.MustCompile(`^\$ +BACKUP/LIST \[SETS\](\w+)\.BCK/SAVE_SET$`)
	line := regexp.MustCompile(`^(\[\S+;\d+)\s+(\d+)\s+\d\d-[A-Z]{3}-\d{4} `)
	total := regexp.MustCompile(`^Total of \d+ files?, (\d+) blocks?$`)

	listings := map[string]listing{}

	var (
		name    string
		current listing
	)

	scanner := bufio.NewScanner(bytes.NewReader(load(t, "backup69.log")))
	for scanner.Scan() {
		text := scanner.Text()

		if m := command.FindStringSubmatch(text); m != nil {
			name, current = strings.ToLower(m[1]), listing{}

			continue
		}

		if name == "" {
			continue
		}

		if m := line.FindStringSubmatch(text); m != nil {
			n, _ := strconv.Atoi(m[2])
			current.names = append(current.names, m[1])
			current.sizes = append(current.sizes, uint32(n))
		} else if m := total.FindStringSubmatch(text); m != nil {
			n, _ := strconv.Atoi(m[1])
			current.total = uint32(n)
		} else if text == "End of save set" {
			listings[name] = current
			name = ""
		}
	}

	return listings
}

// TestVAXListings reads every probe save set and finds the files VMS
// listed, in its order, with its sizes and totals.
func TestVAXListings(t *testing.T) {
	listings := vaxListings(t)

	for _, name := range probeSets {
		want, ok := listings[name]
		if !ok {
			t.Errorf("%s: no listing in the log", name)

			continue
		}

		s := readSet(t, name, Options{})

		var got listing
		for _, f := range s.Files {
			got.names = append(got.names, f.Name)
			got.sizes = append(got.sizes, f.Used())
			got.total += f.Used()
		}

		if !slices.Equal(got.names, want.names) || !slices.Equal(got.sizes, want.sizes) || got.total != want.total {
			t.Errorf("%s:\n got %v %v total %d\nwant %v %v total %d", name,
				got.names, got.sizes, got.total, want.names, want.sizes, want.total)
		}
	}
}

// TestSplitData: MEDIUM.DAT and BIG.DAT are larger than a save set block,
// so their data goes on from block to block. Every save set that holds
// them, whatever its block and group sizes, gives the same bytes, with
// none missing.
func TestSplitData(t *testing.T) {
	for _, name := range []string{"[B69.SUB1]MEDIUM.DAT;1", "[B69.SUB1.SUB2]BIG.DAT;1", "[B69]CONTIG.DAT;1"} {
		var first []byte

		for _, set := range []string{"b69", "b69k8", "b69g0", "b69g3", "b69crc", "b69nocrc"} {
			f := file(t, readSet(t, set, Options{}), name)
			data := f.Data()

			if f.Missing() != nil || len(data) != int(f.Used())*VBNSize || f.Used() < 70 {
				t.Errorf("%s in %s: %d blocks used, %d bytes, missing %v", name, set, f.Used(), len(data), f.Missing())
			}

			if first == nil {
				first = data
			} else if !bytes.Equal(first, data) {
				t.Errorf("%s in %s differs from B69's", name, set)
			}
		}
	}
}

// TestFileKinds: a directory's records carry the DIRECTORY flag, a
// NOBACKUP file is its header alone unless saved with /IGNORE=NOBACKUP,
// an ACL is two packed ACEs, and an empty file has no data.
func TestFileKinds(t *testing.T) {
	s := readSet(t, "b69", Options{})

	dir := file(t, s, "[B69]SUB1.DIR;1")
	if dir.Flags != RecordDirectory || !dir.Directory() || dir.DirStatus != 0x0F || len(dir.Data()) != VBNSize {
		t.Errorf("SUB1.DIR: flags %#x, characteristics %#x, status %#x, %d bytes",
			dir.Flags, dir.Characteristics, dir.DirStatus, len(dir.Data()))
	}

	if f := file(t, s, "[B69]TEXT.TXT;1"); f.Flags != 0 || f.Directory() {
		t.Errorf("TEXT.TXT: flags %#x, characteristics %#x", f.Flags, f.Characteristics)
	}

	noBackup := file(t, s, "[B69]NOBACK.TXT;1")
	if !noBackup.HeaderOnly() || noBackup.Data() != nil || noBackup.Missing() != nil || noBackup.Used() != 1 {
		t.Errorf("NOBACK.TXT: header only %v, %d bytes, missing %v", noBackup.HeaderOnly(), len(noBackup.Data()), noBackup.Missing())
	}

	saved := file(t, readSet(t, "b69ign", Options{}), "[B69]NOBACK.TXT;1")
	if saved.HeaderOnly() || len(saved.Data()) != VBNSize || !saved.NoBackup() {
		t.Errorf("NOBACK.TXT with /IGNORE=NOBACKUP: header only %v, %d bytes", saved.HeaderOnly(), len(saved.Data()))
	}

	// Two identifier ACEs: [1,4] READ+WRITE, and [200,201] READ (UICs
	// are octal).
	acl := file(t, s, "[B69]ACL.TXT;1").ACL
	if len(acl) != 24 || acl[0] != 12 || acl[1] != 1 || acl[12] != 12 ||
		binary.LittleEndian.Uint32(acl[8:]) != 0x00010004 || binary.LittleEndian.Uint32(acl[20:]) != 0x00800081 {
		t.Errorf("ACL.TXT's ACL: % x", acl)
	}

	if f := file(t, s, "[B69]EMPTY.DAT;1"); f.Used() != 0 || len(f.Data()) != 0 || f.Missing() != nil {
		t.Errorf("EMPTY.DAT: %d used, %d bytes", f.Used(), len(f.Data()))
	}

	if f := file(t, s, "[B69]OWNED.TXT;1"); f.Owner != 0x00800081 {
		t.Errorf("OWNED.TXT: owner %#x", f.Owner)
	}
}

// TestSummaries: /COMMENT's text, /BLOCK_SIZE's buffer count, and
// /GROUP_SIZE.
func TestSummaries(t *testing.T) {
	if s := readSet(t, "b69com", Options{}).Summary; s.Comment != "Phase 69's probe, with a comment" {
		t.Errorf("B69COM's comment %q", s.Comment)
	}

	if s := readSet(t, "b69k8", Options{}); s.BlockSize != 8192 || s.Summary.Buffers != 32 || s.Blocks != 28 {
		t.Errorf("B69K8: block size %d, buffers %d, %d blocks", s.BlockSize, s.Summary.Buffers, s.Blocks)
	}

	for name, group := range map[string]uint16{"b69": 10, "b69g0": 0, "b69g3": 3} {
		if s := readSet(t, name, Options{}); s.Summary.GroupSize != group {
			t.Errorf("%s: group size %d", name, s.Summary.GroupSize)
		}
	}
}

// TestCurrentFile: a block after the first names, in its header, the
// file whose data it continues, its file ID as the file record's, with
// RVN 1.
func TestCurrentFile(t *testing.T) {
	data := load(t, "b69.bck")
	block2 := ParseHeader(data[32256:])
	contig := file(t, readSet(t, "b69", Options{}), "[B69]CONTIG.DAT;1")

	if block2.Current.Name != contig.Name || block2.Current.FID != contig.FID || contig.FID[2] != 1 || block2.Current.Allocation != contig.Allocated {
		t.Errorf("block 2's current file %+v; CONTIG.DAT's FID %v", block2.Current, contig.FID)
	}

	if block1 := ParseHeader(data); block1.Current != (CurrentFile{}) {
		t.Errorf("block 1's current file %+v", block1.Current)
	}
}

// blocksOf splits a fixture save set into its blocks.
func blocksOf(t *testing.T, name string) [][]byte {
	t.Helper()

	data := load(t, name+".bck")
	size := int(binary.LittleEndian.Uint32(data[bhdBlockSize:]))

	var blocks [][]byte
	for at := 0; at+size <= len(data); at += size {
		blocks = append(blocks, data[at:at+size])
	}

	return blocks
}

// TestRecovery: SSCOPY's damaged copies. B69HDRD's damaged header is
// always found and its block rebuilt from its group, byte for byte the
// block B69 holds. B69CRCD's damaged data is found only with /CRC, and
// then rebuilt as B69CRC's; without it, the damage reaches the data, as
// it did on VMS. B69G0D and B69NOCRCD have no CRC to find it by.
func TestRecovery(t *testing.T) {
	cases := []struct {
		damaged, original string
		opts              Options
		recovered         []int
	}{
		{"b69hdrd", "b69", Options{}, []int{2}},
		{"b69hdrd", "b69", Options{CheckCRC: true}, []int{2}},
		{"b69crcd", "b69crc", Options{CheckCRC: true}, []int{2}},
		{"b69crcd", "b69crc", Options{}, nil},
		{"b69g0d", "b69g0", Options{CheckCRC: true}, nil},
		{"b69nocrcd", "b69nocrc", Options{CheckCRC: true}, nil},
	}

	for _, c := range cases {
		blocks := blocksOf(t, c.damaged)
		repaired := make([][]byte, len(blocks))

		for i, b := range blocks {
			repaired[i] = slices.Clone(b)
		}

		s := &SaveSet{BlockSize: len(blocks[0]), Blocks: len(blocks)}
		lost := s.repair(repaired, c.opts)

		if !slices.Equal(s.Recovered, c.recovered) || len(lost) != 0 || s.Lost != nil {
			t.Errorf("%s %+v: recovered %v, lost %v", c.damaged, c.opts, s.Recovered, s.Lost)
		}

		original := blocksOf(t, c.original)
		same := slices.EqualFunc(repaired, original, bytes.Equal)

		if same != (c.recovered != nil) {
			t.Errorf("%s %+v: repaired blocks equal %s's: %v", c.damaged, c.opts, c.original, same)
		}

		// Read gives the same files either way, and CONTIG.DAT, whose
		// data the changed byte is in, matches only when rebuilt.
		read := readSet(t, c.damaged, c.opts)
		contig := file(t, read, "[B69]CONTIG.DAT;1").Data()
		matches := bytes.Equal(contig, file(t, readSet(t, c.original, Options{}), "[B69]CONTIG.DAT;1").Data())

		if len(read.Files) != 25 || matches != (c.damaged == "b69hdrd" || c.recovered != nil) {
			t.Errorf("%s %+v: %d files, CONTIG.DAT matches %v", c.damaged, c.opts, len(read.Files), matches)
		}
	}
}

// TestLostBlock: a damaged block with no redundancy group (B69G0's
// block 2, its header changed) is lost, and the files whose data it
// held miss those blocks; the files whose records it held are gone.
func TestLostBlock(t *testing.T) {
	data := slices.Clone(load(t, "b69g0.bck"))
	data[32256+100] ^= 0xFF

	s, err := Read(bytes.NewReader(data), int64(len(data)), Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(s.Lost, []int{2}) || s.Recovered != nil {
		t.Errorf("lost %v, recovered %v", s.Lost, s.Recovered)
	}

	// Block 2 held CONTIG.DAT's blocks from 61 on, then the records of
	// EMPTY.DAT to MEDIUM.DAT's first ten blocks. Blocks 3 and 4 go on
	// with MEDIUM.DAT's data, which is dropped.
	contig := file(t, s, "[B69]CONTIG.DAT;1")
	if got := contig.Missing(); !slices.Equal(got, []Range{{61, 74}}) {
		t.Errorf("CONTIG.DAT is missing %v", got)
	}

	var names []string
	for _, f := range s.Files {
		names = append(names, f.Name)

		if f.Name != contig.Name && f.Missing() != nil {
			t.Errorf("%s is missing %v", f.Name, f.Missing())
		}
	}

	want := []string{
		"[B69]ACL.TXT;1", "[B69]CONTIG.DAT;1", "[B69.SUB1]SUB2.DIR;1",
		"[B69.SUB1.SUB2]B.TXT;1", "[B69.SUB1.SUB2]BIG.DAT;1", "[B69]TEXT.TXT;1",
		"[B69]VERLIM.TXT;2", "[B69]VERLIM.TXT;1", "[B69]VFC.TXT;1",
	}
	if !slices.Equal(names, want) {
		t.Errorf("files %v", names)
	}
}

// TestTruncated: B69CUT is B69's first two blocks. MEDIUM.DAT, whose
// data block 3 went on with, misses the rest (VBNMISSING, "file has
// missing blocks 11 through 74").
func TestTruncated(t *testing.T) {
	s := readSet(t, "b69cut", Options{})
	medium := file(t, s, "[B69.SUB1]MEDIUM.DAT;1")

	if got := medium.Missing(); !slices.Equal(got, []Range{{11, 74}}) {
		t.Errorf("MEDIUM.DAT is missing %v", got)
	}

	for _, f := range s.Files[:len(s.Files)-1] {
		if f.Missing() != nil {
			t.Errorf("%s is missing %v", f.Name, f.Missing())
		}
	}
}

// TestNotSaveSet: a text file, an image, and an empty file aren't save
// sets.
func TestNotSaveSet(t *testing.T) {
	for _, data := range [][]byte{load(t, "pi.mar"), load(t, "pi.exe"), nil, make([]byte, 40000)} {
		if _, err := Read(bytes.NewReader(data), int64(len(data)), Options{}); !errors.Is(err, ErrNotSaveSet) {
			t.Errorf("%d bytes: %v", len(data), err)
		}
	}
}

// TestCRC16: the header checksum's CRC is CRC-16/ARC, whose check value
// for "123456789" is %XBB3D.
func TestCRC16(t *testing.T) {
	if got := crc16([]byte("123456789")); got != 0xBB3D {
		t.Errorf("crc16 = %#04x", got)
	}
}
