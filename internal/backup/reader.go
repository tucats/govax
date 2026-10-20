package backup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// VBNSize is the size of a virtual block, the unit of a file's data.
const VBNSize = 512

// ErrNotSaveSet is the error for a file that isn't a save set: its first
// block doesn't have a BACKUP block header, or its first block holds no
// summary (BACKUP's NOTSAVESET).
var ErrNotSaveSet = errors.New("file is not a BACKUP save set")

// Options are a read's choices.
type Options struct {
	// CheckCRC is /CRC on the input save set: a block whose data CRC is
	// wrong is damaged, and rebuilt from its group like a block whose
	// header checksum is wrong. VMS V7.1 checks the CRC only when told
	// to, and so does Read. The header checksum is always checked.
	CheckCRC bool
}

// SaveSet is a save set's contents.
type SaveSet struct {
	// BlockSize is the save set's block size, from its first block's
	// header; Blocks how many blocks it holds, redundancy blocks too.
	BlockSize int
	Blocks    int

	Summary Summary
	Files   []*File

	// Recovered lists the numbers of the damaged blocks rebuilt from
	// their redundancy groups (BACKUP's XORERRS, which VMS reports when
	// it reaches the group's redundancy block). Lost lists the numbers
	// of damaged blocks that couldn't be rebuilt, whose records are
	// missing; a file whose data was in one has missing blocks
	// (File.Missing).
	Recovered []int
	Lost      []int
}

// File is one file saved in a save set: its attributes and its data.
type File struct {
	Attributes

	// Flags are the file record's flags: RecordDirectory for a directory
	// file saved by a [dir...] save.
	Flags RecordFlag

	// runs are the file's data records, in the save set's order.
	runs []run
}

// run is one data record: the virtual blocks from vbn on.
type run struct {
	vbn  uint32
	data []byte
}

// Range is a run of virtual block numbers, First through Last.
type Range struct {
	First, Last uint32
}

// HeaderOnly reports whether the save set holds only the file's header:
// a file marked NOBACKUP, saved without /IGNORE=NOBACKUP, has no data
// records (a restore says HEADCOPIED).
func (f *File) HeaderOnly() bool {
	return f.NoBackup() && len(f.runs) == 0
}

// Data is the file's data, its used blocks in VBN order (Attributes.Used,
// or further if the save set holds more). Blocks the save set lacks
// (Missing) read as zeros. A header-only file has none.
func (f *File) Data() []byte {
	if f.HeaderOnly() {
		return nil
	}

	blocks := f.Used()
	for _, r := range f.runs {
		blocks = max(blocks, r.vbn-1+uint32(len(r.data)/VBNSize))
	}

	data := make([]byte, int(blocks)*VBNSize)
	for _, r := range f.runs {
		copy(data[int(r.vbn-1)*VBNSize:], r.data)
	}

	return data
}

// Missing lists the used blocks the save set has no data for, in order:
// a truncated save set, or a lost block (BACKUP's VBNMISSING, "file has
// missing blocks 11 through 74"). A header-only file misses none.
func (f *File) Missing() []Range {
	if f.HeaderOnly() {
		return nil
	}

	used := f.Used()
	have := make([]bool, used+1)

	for _, r := range f.runs {
		for v := r.vbn; v < r.vbn+uint32(len(r.data)/VBNSize) && v <= used; v++ {
			have[v] = true
		}
	}

	var missing []Range

	for v := uint32(1); v <= used; v++ {
		if have[v] {
			continue
		}

		if n := len(missing); n > 0 && missing[n-1].Last == v-1 {
			missing[n-1].Last = v
		} else {
			missing = append(missing, Range{First: v, Last: v})
		}
	}

	return missing
}

// Read reads the save set in r, size bytes long. Its block size comes
// from its first block's header, not from what holds it, so a save set
// copied off a volume as plain bytes reads as well as one on a volume.
// A partial block at the end is ignored, as a truncated save set's
// missing blocks are.
func Read(r io.ReaderAt, size int64, opts Options) (*SaveSet, error) {
	if size < HeaderSize {
		return nil, ErrNotSaveSet
	}

	first := make([]byte, HeaderSize)
	if _, err := r.ReadAt(first, 0); err != nil {
		return nil, err
	}

	header := ParseHeader(first)
	if !header.Plausible() {
		return nil, ErrNotSaveSet
	}

	s := &SaveSet{BlockSize: int(header.BlockSize), Blocks: int(size / int64(header.BlockSize))}
	if s.Blocks == 0 {
		return nil, ErrNotSaveSet
	}

	blocks := make([][]byte, s.Blocks)
	for i := range blocks {
		blocks[i] = make([]byte, s.BlockSize)
		if _, err := r.ReadAt(blocks[i], int64(i)*int64(s.BlockSize)); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
	}

	lost := s.repair(blocks, opts)

	return s, s.decode(blocks, lost)
}

// intact reports whether a block's checks pass: its header checksum,
// and with CheckCRC its data CRC.
func intact(block []byte, opts Options) bool {
	return headerIntact(block) && (!opts.CheckCRC || crcIntact(block))
}

// repair rebuilds each damaged block it can from its redundancy group,
// in place, and returns the set of block indexes it couldn't rebuild.
//
// A group is up to GroupSize data blocks followed by their redundancy
// block, whose body is the XOR of theirs, and whose header is the XOR of
// theirs with its own size, kind, number, flags, and checks written over.
// So the XOR of the redundancy block and the group's other data blocks
// is the damaged block, but for those header fields, which repair writes
// back. The last group may be short; GroupSize 0 writes no redundancy
// blocks, and nothing can be rebuilt.
func (s *SaveSet) repair(blocks [][]byte, opts Options) map[int]bool {
	lost := map[int]bool{}

	var bad []int

	for i, b := range blocks {
		if !intact(b, opts) {
			bad = append(bad, i)
		}
	}

	if len(bad) == 0 {
		return lost
	}

	group := s.groupSize(blocks, opts)

	for _, i := range bad {
		if !s.rebuild(blocks, i, group, opts) {
			lost[i] = true
			s.Lost = append(s.Lost, i+1)
		}
	}

	return lost
}

// groupSize finds how many data blocks a redundancy group holds: the
// summary's group size when block 1 is intact, or else the number of
// blocks before the first intact redundancy block. 0 means none.
func (s *SaveSet) groupSize(blocks [][]byte, opts Options) int {
	if intact(blocks[0], opts) {
		if records, _ := Records(blocks[0]); len(records) > 0 && records[0].Type == RecordSummary {
			summary, _ := parseSummary(records[0].Body)

			return int(summary.GroupSize)
		}
	}

	for i, b := range blocks {
		if intact(b, opts) && ParseHeader(b).Application == ApplicationXOR {
			return i
		}
	}

	return 0
}

// rebuild rebuilds blocks[i] from its group, reporting whether it could.
// A damaged redundancy block holds no data, so losing it loses nothing.
func (s *SaveSet) rebuild(blocks [][]byte, i, group int, opts Options) bool {
	if group == 0 {
		return false
	}

	start := i / (group + 1) * (group + 1)
	xor := min(start+group, len(blocks)-1)

	if i == xor || ParseHeader(blocks[i]).Application == ApplicationXOR {
		return true
	}

	parity := blocks[xor]
	if !intact(parity, opts) || ParseHeader(parity).Application != ApplicationXOR {
		return false
	}

	rebuilt := make([]byte, s.BlockSize)
	for j := start; j <= xor; j++ {
		if j == i {
			continue
		}

		if !intact(blocks[j], opts) {
			return false
		}

		for k, b := range blocks[j] {
			rebuilt[k] ^= b
		}
	}

	// The fields the redundancy block wrote over its XOR of the headers
	// are a data block's own: its size, kind, and number, the save set's
	// operating system, subsystem, and flags, and its checks.
	le := binary.LittleEndian
	p := ParseHeader(parity)
	le.PutUint16(rebuilt[bhdSize:], HeaderSize)
	le.PutUint16(rebuilt[bhdOpSys:], p.OpSys)
	le.PutUint16(rebuilt[bhdSubsystem:], p.Subsystem)
	le.PutUint16(rebuilt[bhdApplication:], ApplicationData)
	le.PutUint32(rebuilt[bhdNumber:], uint32(i+1))
	le.PutUint32(rebuilt[bhdFlags:], p.Flags)
	le.PutUint32(rebuilt[bhdCRC:], 0)

	if p.HasCRC() {
		le.PutUint32(rebuilt[bhdCRC:], DataCRC(rebuilt))
	}

	le.PutUint16(rebuilt[bhdChecksum:], HeaderChecksum(rebuilt))

	blocks[i] = rebuilt
	s.Recovered = append(s.Recovered, i+1)

	return true
}

// decode walks the data blocks' records: the summary first, then each
// file's attribute record followed by its data records, which may go on
// in later blocks.
func (s *SaveSet) decode(blocks [][]byte, lost map[int]bool) error {
	if lost[0] {
		return ErrNotSaveSet
	}

	var (
		current  *File
		lastLost bool
	)

	for i, block := range blocks {
		if lost[i] {
			lastLost = true

			continue
		}

		header := ParseHeader(block)
		if header.Application != ApplicationData {
			continue
		}

		// After a lost block, data records continue the file the block's
		// header names; if that isn't the file being read, its records
		// went with the lost block, and the data is dropped.
		if lastLost && current != nil && header.Current.Name != current.Name {
			current = nil
		}

		lastLost = false

		records, err := Records(block)
		if err != nil {
			return fmt.Errorf("block %d: %w", i+1, err)
		}

		for n, r := range records {
			switch r.Type {
			case RecordSummary:
				if i == 0 && n == 0 {
					if s.Summary, err = parseSummary(r.Body); err != nil {
						return err
					}
				}

			case RecordFile:
				attributes, err := parseAttributes(r.Body)
				if err != nil {
					return fmt.Errorf("block %d: %w", i+1, err)
				}

				current = &File{Attributes: attributes, Flags: r.Flags}
				s.Files = append(s.Files, current)

			case RecordVBN:
				if current != nil && r.Address > 0 {
					current.runs = append(current.runs, run{vbn: r.Address, data: r.Body})
				}
			}
		}

		if i == 0 && (len(records) == 0 || records[0].Type != RecordSummary) {
			return ErrNotSaveSet
		}
	}

	return nil
}
