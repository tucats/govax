package backup

import "encoding/binary"

// HeaderSize is the size of a block header; a block's records start
// after it.
const HeaderSize = 256

// The block header's fields, by byte offset (the public readers' names).
const (
	bhdSize        = 0   // word: the header's size, 256
	bhdOpSys       = 2   // word: the operating system that wrote it
	bhdSubsystem   = 4   // word: 1, BACKUP
	bhdApplication = 6   // word: ApplicationData or ApplicationXOR
	bhdNumber      = 8   // long: the block's number in the save set, from 1
	bhdStrucLev    = 32  // word: the block's structure level, %X0101
	bhdVolume      = 34  // word: the volume number
	bhdCRC         = 36  // long: the data CRC, or 0
	bhdBlockSize   = 40  // long: the block's size
	bhdFlags       = 44  // long: FlagNoCRC
	bhdSaveSetName = 48  // 32 bytes: a counted string
	bhdFID         = 80  // 3 words: the current file's ID
	bhdDID         = 86  // 3 words: its directory's ID
	bhdFileName    = 92  // 128 bytes: its name, a counted string
	bhdRecordType  = 220 // byte: its FAT$B_RTYPE
	bhdRecordAttr  = 221 // byte: its FAT$B_RATTRIB
	bhdRecordSize  = 222 // word: its FAT$W_RSIZE
	bhdBucketSize  = 224 // byte
	bhdVFCSize     = 225 // byte
	bhdMaxRecord   = 226 // word
	bhdFileSize    = 228 // long: its allocation
	bhdChecksum    = 254 // word: the header checksum
)

// A block's application code: a block of records, or a group's
// redundancy block.
const (
	ApplicationData = 1
	ApplicationXOR  = 2
)

// SubsystemBackup is the subsystem code of every BACKUP block.
const SubsystemBackup = 1

// OpSysVAX and its kin are the operating system codes a block header and
// the summary's OPSYS attribute carry.
const (
	OpSysVAX   = 0x0400
	OpSysAlpha = 0x0800
	OpSysI64   = 0x1000
)

// FlagNoCRC, in a block header's flags, says the block has no data CRC.
const FlagNoCRC = 0x00000001

// The block sizes BACKUP allows for a disk save set.
const (
	MinBlockSize = 2048
	MaxBlockSize = 65535
)

// BlockHeader is a block header's fields.
type BlockHeader struct {
	Size        uint16
	OpSys       uint16
	Subsystem   uint16
	Application uint16
	Number      uint32
	StrucLev    uint16
	Volume      uint16
	CRC         uint32
	BlockSize   uint32
	Flags       uint32
	SaveSetName string
	Checksum    uint16

	// Current is the file BACKUP was saving when it wrote the block:
	// zero in block 1, and in a later block the file whose records it
	// starts with or continues.
	Current CurrentFile
}

// CurrentFile is the part of a block header that names the file being
// saved: its file ID (with the relative volume number, 1, as the file
// record's has), its directory's, its name, and its record attributes.
type CurrentFile struct {
	FID, DID   [3]uint16
	Name       string
	RecordType uint8
	RecordAttr uint8
	RecordSize uint16
	BucketSize uint8
	VFCSize    uint8
	MaxRecord  uint16
	Allocation uint32
}

// ParseHeader decodes the header at block's start. It checks nothing;
// see Valid.
func ParseHeader(block []byte) BlockHeader {
	le := binary.LittleEndian
	h := BlockHeader{
		Size:        le.Uint16(block[bhdSize:]),
		OpSys:       le.Uint16(block[bhdOpSys:]),
		Subsystem:   le.Uint16(block[bhdSubsystem:]),
		Application: le.Uint16(block[bhdApplication:]),
		Number:      le.Uint32(block[bhdNumber:]),
		StrucLev:    le.Uint16(block[bhdStrucLev:]),
		Volume:      le.Uint16(block[bhdVolume:]),
		CRC:         le.Uint32(block[bhdCRC:]),
		BlockSize:   le.Uint32(block[bhdBlockSize:]),
		Flags:       le.Uint32(block[bhdFlags:]),
		SaveSetName: counted(block[bhdSaveSetName : bhdSaveSetName+32]),
		Checksum:    le.Uint16(block[bhdChecksum:]),
	}

	c := &h.Current
	for i := range 3 {
		c.FID[i] = le.Uint16(block[bhdFID+2*i:])
		c.DID[i] = le.Uint16(block[bhdDID+2*i:])
	}

	c.Name = counted(block[bhdFileName : bhdFileName+128])
	c.RecordType = block[bhdRecordType]
	c.RecordAttr = block[bhdRecordAttr]
	c.RecordSize = le.Uint16(block[bhdRecordSize:])
	c.BucketSize = block[bhdBucketSize]
	c.VFCSize = block[bhdVFCSize]
	c.MaxRecord = le.Uint16(block[bhdMaxRecord:])
	c.Allocation = le.Uint32(block[bhdFileSize:])

	return h
}

// counted returns the counted string (a length byte, then the text) at
// field's start, cut at the field's end.
func counted(field []byte) string {
	n := min(int(field[0]), len(field)-1)

	return string(field[1 : 1+n])
}

// HasCRC reports whether the block carries a data CRC.
func (h BlockHeader) HasCRC() bool { return h.Flags&FlagNoCRC == 0 }

// Plausible reports whether h looks like a BACKUP block header at all,
// whatever its checksum says: the right size and subsystem, and a block
// size BACKUP allows. A file whose first block fails it isn't a save
// set.
func (h BlockHeader) Plausible() bool {
	return h.Size == HeaderSize && h.Subsystem == SubsystemBackup &&
		h.BlockSize >= MinBlockSize && h.BlockSize <= MaxBlockSize
}

// headerIntact reports whether block's header checksum is right.
func headerIntact(block []byte) bool {
	return binary.LittleEndian.Uint16(block[bhdChecksum:]) == HeaderChecksum(block)
}

// crcIntact reports whether block's data CRC is right, or it has none.
func crcIntact(block []byte) bool {
	h := ParseHeader(block)

	return !h.HasCRC() || h.CRC == DataCRC(block)
}
