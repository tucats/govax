package isam

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// BlockSize is the size of a disk block, and the unit a bucket's size is
// counted in.
const BlockSize = 512

// Offsets in VBN 1 of the file-level fields, which follow key 0's
// descriptor (VMS's PLG$ names).
const (
	plgAreaVBN   = 0x66 // PLG$B_AVBN: the block holding the first area descriptor
	plgAreaCount = 0x67 // PLG$B_AMAX: how many areas
	plgDataVBN   = 0x68 // PLG$W_DVBN: the first VBN past the prolog
	plgMaxRecord = 0x6C // PLG$L_MRN
	plgEOF       = 0x70 // PLG$L_EOF
	plgVersion   = 0x74 // PLG$W_VER_NO: 3 for every file read here
	plgGlobalBuf = 0x76 // PLG$W_GBC: the global buffer count
	plgCollate   = 0x78 // PLG$W_COLVBN

	blockSumOffset = 0x1FE // the word holding each prolog block's sum
)

// Offsets within a key descriptor (VMS's KEY$ names).
const (
	keyNextVBN    = 0x00 // KEY$L_IDXFL: the block of the next key's descriptor
	keyNextOffset = 0x04 // KEY$W_NOFF: its offset there
	keyIndexArea  = 0x06 // KEY$B_IANUM
	keyLevel1Area = 0x07 // KEY$B_LANUM
	keyDataArea   = 0x08 // KEY$B_DANUM
	keyRootLevel  = 0x09 // KEY$B_ROOTLEV
	keyIndexSize  = 0x0A // KEY$B_IDXBKTSZ
	keyDataSize   = 0x0B // KEY$B_DATBKTSZ
	keyRootVBN    = 0x0C // KEY$L_ROOTVBN
	keyFlags      = 0x10 // KEY$B_FLAGS
	keyDataType   = 0x11 // KEY$B_DATATYPE
	keySegments   = 0x12 // KEY$B_SEGMENTS
	keyNullChar   = 0x13 // KEY$B_NULLCHAR
	keyTotalSize  = 0x14 // KEY$B_KEYSZ
	keyReference  = 0x15 // KEY$B_KEYREF
	keyMinRecord  = 0x16 // KEY$W_MINRECSZ
	keyIndexFill  = 0x18 // KEY$W_IDXFILL
	keyDataFill   = 0x1A // KEY$W_DATFILL
	keyPositions  = 0x1C // KEY$W_POSITION: 8 words
	keySizes      = 0x2C // KEY$B_SIZE: 8 bytes
	keyName       = 0x34 // KEY$T_KEYNAM: 32 bytes
	keyNameSize   = 32
	keyFirstData  = 0x54 // KEY$L_LDVBN
	keyTypes      = 0x58 // KEY$B_TYPE: 8 bytes
	keyDescSize   = 0x60 // KEY$C_BLN
)

// A key descriptor's flag bits (KEY$M_). The three compression bits are
// set when a part is compressed, the opposite of the XABKEY's XAB$V_
// bits in the same places, which are set when it isn't.
const (
	FlagDuplicates     = 0x01 // KEY$M_DUPKEYS
	FlagChanges        = 0x02 // KEY$M_CHGKEYS
	FlagNull           = 0x04 // KEY$M_NULKEYS
	FlagIndexCompress  = 0x08 // KEY$M_IDX_COMPR
	FlagNoIndexYet     = 0x10 // KEY$M_INITIDX: no record has made this key's index yet
	FlagKeyCompress    = 0x40 // KEY$M_KEY_COMPR
	FlagRecordCompress = 0x80 // KEY$M_REC_COMPR
)

// Offsets within an area descriptor (VMS's AREA$ names).
const (
	areaID         = 0x02 // AREA$B_AREAID
	areaBucketSize = 0x03 // AREA$B_ARBKTSZ
	areaVolume     = 0x04 // AREA$W_VOLUME
	areaAlignment  = 0x06 // AREA$B_ALN
	areaOptions    = 0x07 // AREA$B_AOP
	areaAvailable  = 0x08 // AREA$L_AVAIL
	areaExtentVBN  = 0x0C // AREA$L_CVBN
	areaExtentSize = 0x10 // AREA$L_CNBLK
	areaUsed       = 0x14 // AREA$L_USED
	areaNextVBN    = 0x18 // AREA$L_NXTVBN
	areaNextExtent = 0x1C // AREA$L_NXT
	areaNextSize   = 0x20 // AREA$L_NXBLK
	areaExtend     = 0x24 // AREA$W_DEQ
	areaLocation   = 0x28 // AREA$L_LOC
	areaTotal      = 0x32 // AREA$L_TOTAL_ALLOC
	areaDescSize   = 0x40 // AREA$C_BLN
)

// Errors a caller may need to tell apart. Each maps onto an RMS status
// in internal/rms.
var (
	// ErrBadProlog is a file with no usable prolog: not indexed, or left
	// empty by a $CREATE that failed (VMS's own E85 in testdata/idx88).
	ErrBadProlog = errors.New("isam: no valid prolog")

	// ErrNotFound means no record answers a Find.
	ErrNotFound = errors.New("isam: record not found")

	// ErrEnd means a sequence has no more records.
	ErrEnd = errors.New("isam: end of file")

	// ErrDeleted means an RFA names a record that's gone.
	ErrDeleted = errors.New("isam: record deleted")

	// ErrKeySize is a search key of the wrong size: empty, longer than
	// the key, or a part of a key that isn't a string.
	ErrKeySize = errors.New("isam: bad key size")

	// ErrKeyRef is a key of reference the file doesn't have.
	ErrKeyRef = errors.New("isam: no such key")
)

// ErrPrologLevel is a file whose prolog is a level this package doesn't
// read: Prologs 1 and 2 have another record layout, and the author chose
// Prolog 3 alone (the phase doc's "The review", answer 1).
type ErrPrologLevel struct {
	Version int
}

func (e *ErrPrologLevel) Error() string {
	return fmt.Sprintf("isam: prolog level %d isn't supported", e.Version)
}

// CorruptError is a structure that doesn't parse: a bucket's records
// running past its free space, say. Where says what was being read.
type CorruptError struct {
	VBN    uint32
	Offset int
	Where  string
}

func (e *CorruptError) Error() string {
	return fmt.Sprintf("isam: corrupt file at VBN %d, offset %X: %s", e.VBN, e.Offset, e.Where)
}

// Blocks is how the package reaches a file: by virtual block number,
// counted from 1, one 512-byte block at a time. rms gives it an ods2
// file, and tests a byte slice.
type Blocks interface {
	ReadBlock(vbn uint32, block []byte) error
}

// RecordFormat is the file's record format, which the file's header
// holds rather than the prolog.
type RecordFormat int

const (
	// Variable records carry their sizes (RMS's FAB$C_VAR).
	Variable RecordFormat = iota

	// Fixed records are all the size in Attributes.Size (FAB$C_FIX).
	Fixed
)

// Attributes is what the package must know from the file's header.
type Attributes struct {
	Format RecordFormat

	// Size is the longest record, MRS: every record's size if Fixed.
	Size int
}

// Segment is one piece of a key: bytes at a position in the record.
type Segment struct {
	Position int
	Size     int
	Type     Type // KEY$B_TYPE's byte for this segment
}

// Key is one key descriptor.
type Key struct {
	Ref                             int // the key of reference, 0 for the primary key
	IndexArea, Level1Area, DataArea int
	RootLevel                       int // the root's level; 0 with no index yet
	IndexBucketSize, DataBucketSize int // in blocks
	RootVBN                         uint32
	Flags                           uint8
	Type                            Type
	Null                            uint8 // the null character, for a string key with FlagNull
	Size                            int   // the whole key's bytes
	MinRecord                       int   // the shortest record that holds the whole key
	IndexFill, DataFill             int   // bytes to fill a bucket to, under ROP LOA
	Segments                        []Segment
	Name                            string
	FirstData                       uint32 // the first data bucket's VBN; 0 with no index yet
	DescriptorVBN                   uint32 // where the descriptor is
	DescriptorOffset                int
	NextDescriptorVBN               uint32
	NextDescriptorOffset            int
}

// HasIndex reports whether the key's tree exists. Until its first record
// it doesn't, and the root field holds the areas instead.
func (k *Key) HasIndex() bool {
	return k.Flags&FlagNoIndexYet == 0 && k.RootVBN != 0
}

// Area is one area descriptor.
type Area struct {
	ID              int
	BucketSize      int // in blocks
	Volume          int
	Alignment       uint8
	Options         uint8
	Available       uint32 // the first bucket of the reclaimed list
	ExtentVBN       uint32 // the current extent's first VBN
	ExtentSize      uint32 // its blocks
	Used            uint32 // its blocks in use
	NextVBN         uint32 // the next free VBN
	NextExtent      uint32 // an extent to come: its VBN
	NextExtentSize  uint32 // and its blocks
	ExtendSize      int    // the default extend quantity, DEQ
	Location        uint32
	TotalAllocation uint32
}

// File is an open indexed file.
type File struct {
	blocks Blocks
	attr   Attributes

	Version     int
	AreaVBN     uint32 // the block of the first area descriptor
	DataVBN     uint32 // PLG$W_DVBN
	MaxRecordNo uint32 // PLG$L_MRN
	EOF         uint32 // PLG$L_EOF
	GlobalBufs  int    // PLG$W_GBC
	Keys        []*Key
	Areas       []Area
}

// Open reads a file's prolog. Nothing else is read until asked for, and
// nothing is kept but the prolog, so a File stays right however the data
// buckets change.
func Open(blocks Blocks, attr Attributes) (*File, error) {
	if attr.Format == Fixed && attr.Size <= 0 {
		return nil, ErrBadProlog
	}

	first := make([]byte, BlockSize)
	if err := blocks.ReadBlock(1, first); err != nil {
		return nil, err
	}

	version := int(binary.LittleEndian.Uint16(first[plgVersion:]))
	if version == 1 || version == 2 {
		return nil, &ErrPrologLevel{Version: version}
	}

	if version != 3 || !blockSumRight(first) {
		return nil, ErrBadProlog
	}

	f := &File{
		blocks:      blocks,
		attr:        attr,
		Version:     version,
		AreaVBN:     uint32(first[plgAreaVBN]),
		DataVBN:     uint32(binary.LittleEndian.Uint16(first[plgDataVBN:])),
		MaxRecordNo: binary.LittleEndian.Uint32(first[plgMaxRecord:]),
		EOF:         binary.LittleEndian.Uint32(first[plgEOF:]),
		GlobalBufs:  int(binary.LittleEndian.Uint16(first[plgGlobalBuf:])),
	}

	if err := f.readKeys(first); err != nil {
		return nil, err
	}

	if err := f.readAreas(int(first[plgAreaCount])); err != nil {
		return nil, err
	}

	return f, nil
}

// blockSumRight checks a prolog block's sum: the block's last word holds
// the sum of its other 255 words, ignoring overflow.
func blockSumRight(block []byte) bool {
	var sum uint16

	for i := 0; i < blockSumOffset; i += 2 {
		sum += binary.LittleEndian.Uint16(block[i:])
	}

	return sum == binary.LittleEndian.Uint16(block[blockSumOffset:])
}

// readPrologBlock reads a block of descriptors and checks its sum.
func (f *File) readPrologBlock(vbn uint32) ([]byte, error) {
	block := make([]byte, BlockSize)
	if err := f.blocks.ReadBlock(vbn, block); err != nil {
		return nil, err
	}

	if !blockSumRight(block) {
		return nil, &CorruptError{VBN: vbn, Offset: blockSumOffset, Where: "the prolog block's sum"}
	}

	return block, nil
}

// readKeys follows the chain of key descriptors from key 0's, which
// starts VBN 1.
func (f *File) readKeys(first []byte) error {
	block := first
	vbn, offset := uint32(1), 0

	for {
		if offset+keyDescSize > BlockSize {
			return &CorruptError{VBN: vbn, Offset: offset, Where: "a key descriptor past its block"}
		}

		k := parseKey(block[offset : offset+keyDescSize])
		k.DescriptorVBN, k.DescriptorOffset = vbn, offset

		if k.Ref != len(f.Keys) {
			return &CorruptError{VBN: vbn, Offset: offset, Where: "a key descriptor out of order"}
		}

		f.Keys = append(f.Keys, k)

		if k.NextDescriptorVBN == 0 {
			return nil
		}

		if k.NextDescriptorVBN != vbn {
			next, err := f.readPrologBlock(k.NextDescriptorVBN)
			if err != nil {
				return err
			}

			block = next
		}

		vbn, offset = k.NextDescriptorVBN, k.NextDescriptorOffset
	}
}

// parseKey decodes one key descriptor.
func parseKey(d []byte) *Key {
	k := &Key{
		IndexArea:            int(d[keyIndexArea]),
		Level1Area:           int(d[keyLevel1Area]),
		DataArea:             int(d[keyDataArea]),
		RootLevel:            int(d[keyRootLevel]),
		IndexBucketSize:      int(d[keyIndexSize]),
		DataBucketSize:       int(d[keyDataSize]),
		RootVBN:              binary.LittleEndian.Uint32(d[keyRootVBN:]),
		Flags:                d[keyFlags],
		Type:                 Type(d[keyDataType]),
		Null:                 d[keyNullChar],
		Size:                 int(d[keyTotalSize]),
		Ref:                  int(d[keyReference]),
		MinRecord:            int(binary.LittleEndian.Uint16(d[keyMinRecord:])),
		IndexFill:            int(binary.LittleEndian.Uint16(d[keyIndexFill:])),
		DataFill:             int(binary.LittleEndian.Uint16(d[keyDataFill:])),
		FirstData:            binary.LittleEndian.Uint32(d[keyFirstData:]),
		NextDescriptorVBN:    binary.LittleEndian.Uint32(d[keyNextVBN:]),
		NextDescriptorOffset: int(binary.LittleEndian.Uint16(d[keyNextOffset:])),
	}

	k.Name = strings.TrimRight(string(d[keyName:keyName+keyNameSize]), " \x00")

	count := int(d[keySegments])
	for i := 0; i < count && i < 8; i++ {
		k.Segments = append(k.Segments, Segment{
			Position: int(binary.LittleEndian.Uint16(d[keyPositions+2*i:])),
			Size:     int(d[keySizes+i]),
			Type:     Type(d[keyTypes+i]),
		})
	}

	// Until the index is made, the root field holds the areas, not a VBN.
	if k.Flags&FlagNoIndexYet != 0 {
		k.RootVBN = 0
	}

	return k
}

// readAreas reads the area descriptors, 64 bytes each from the block the
// prolog names on, in as many blocks as they need.
func (f *File) readAreas(count int) error {
	if count == 0 || f.AreaVBN == 0 {
		return &CorruptError{VBN: 1, Offset: plgAreaVBN, Where: "a file with no areas"}
	}

	perBlock := BlockSize / areaDescSize

	for i := range count {
		vbn := f.AreaVBN + uint32(i/perBlock)

		block, err := f.readPrologBlock(vbn)
		if err != nil {
			return err
		}

		f.Areas = append(f.Areas, parseArea(block[(i%perBlock)*areaDescSize:][:areaDescSize]))
	}

	return nil
}

// parseArea decodes one area descriptor.
func parseArea(d []byte) Area {
	return Area{
		ID:              int(d[areaID]),
		BucketSize:      int(d[areaBucketSize]),
		Volume:          int(binary.LittleEndian.Uint16(d[areaVolume:])),
		Alignment:       d[areaAlignment],
		Options:         d[areaOptions],
		Available:       binary.LittleEndian.Uint32(d[areaAvailable:]),
		ExtentVBN:       binary.LittleEndian.Uint32(d[areaExtentVBN:]),
		ExtentSize:      binary.LittleEndian.Uint32(d[areaExtentSize:]),
		Used:            binary.LittleEndian.Uint32(d[areaUsed:]),
		NextVBN:         binary.LittleEndian.Uint32(d[areaNextVBN:]),
		NextExtent:      binary.LittleEndian.Uint32(d[areaNextExtent:]),
		NextExtentSize:  binary.LittleEndian.Uint32(d[areaNextSize:]),
		ExtendSize:      int(binary.LittleEndian.Uint16(d[areaExtend:])),
		Location:        binary.LittleEndian.Uint32(d[areaLocation:]),
		TotalAllocation: binary.LittleEndian.Uint32(d[areaTotal:]),
	}
}

// key returns the descriptor for a key of reference.
func (f *File) key(ref int) (*Key, error) {
	if ref < 0 || ref >= len(f.Keys) {
		return nil, ErrKeyRef
	}

	return f.Keys[ref], nil
}
