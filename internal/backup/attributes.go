package backup

import (
	"encoding/binary"
	"fmt"
)

// The bodies of the summary record and of a file's record are attribute
// lists: a structure level word (%X0101), then attributes, each a word
// giving the value's size, a word giving its type, and the value. The
// summary's list ends at an attribute of size and type zero; a file's at
// the record's end.

// AttributeType is an attribute's type code. The names are the public
// readers'; their meanings are confirmed by VMS's save sets where the
// comments say what a value means.
type AttributeType uint16

// The summary's attribute types.
const (
	AttrSaveSetName   AttributeType = 1  // the save set's name
	AttrCommand       AttributeType = 2  // the command that wrote it
	AttrComment       AttributeType = 3  // /COMMENT's text
	AttrUserName      AttributeType = 4  // who wrote it, blank-padded to 12
	AttrUserUIC       AttributeType = 5  // their UIC
	AttrDate          AttributeType = 6  // when
	AttrOpSys         AttributeType = 7  // the operating system's code
	AttrSystemVersion AttributeType = 8  // its version, "V7.1"
	AttrNodeName      AttributeType = 9  // the node's name
	AttrCPUID         AttributeType = 10 // the CPU's system ID register
	AttrDevice        AttributeType = 11 // the device written on
	AttrBackupVersion AttributeType = 12 // BACKUP's version, "V6.2"
	AttrBlockSize     AttributeType = 13 // the block size
	AttrGroupSize     AttributeType = 14 // the redundancy group's size
	AttrBuffers       AttributeType = 15 // the buffer count
)

// A file record's attribute types.
const (
	AttrFileName        AttributeType = 42 // the file's name, "[B69]A.TXT;1"
	AttrStrucLev        AttributeType = 43 // its volume's structure level
	AttrFID             AttributeType = 44 // its file ID
	AttrBackLink        AttributeType = 45 // its directory's file ID
	AttrFileSize        AttributeType = 46 // its allocation in blocks
	AttrOwner           AttributeType = 47 // its owner's UIC
	AttrProtection      AttributeType = 48 // its protection mask
	AttrRecordProt      AttributeType = 49 // its record protection
	AttrAccessLevel     AttributeType = 50 // its access level
	AttrCharacteristics AttributeType = 51 // FH2$L_FILECHAR
	AttrRecordAttrs     AttributeType = 52 // the header's 32-byte FAT area
	AttrRevision        AttributeType = 53 // the revision count
	AttrCreated         AttributeType = 54
	AttrRevised         AttributeType = 55
	AttrExpires         AttributeType = 56
	AttrBackedUp        AttributeType = 57
	AttrPlacement       AttributeType = 70
	AttrDirOwner        AttributeType = 71 // a directory's owner
	AttrDirProtection   AttributeType = 72 // a directory's protection
	AttrDirStatus       AttributeType = 73 // a directory file's status
	AttrDirVersionLimit AttributeType = 74 // a directory's version limit
	AttrVersionLimit    AttributeType = 75 // the file's version limit
	AttrACLSegment      AttributeType = 78 // ACEs, packed
	AttrHighwater       AttributeType = 79 // the highwater mark
	AttrJournal         AttributeType = 80 // FH2$B_JOURNAL
	AttrRUActive        AttributeType = 87 // active recovery units' facility
)

// Attribute is one entry of an attribute list.
type Attribute struct {
	Type  AttributeType
	Value []byte
}

// attributeList parses body, a record's attribute list, after its
// structure level word.
func attributeList(body []byte) ([]Attribute, error) {
	le := binary.LittleEndian

	if len(body) < 2 {
		return nil, fmt.Errorf("backup: an attribute list of %d bytes", len(body))
	}

	var list []Attribute

	for at := 2; at+4 <= len(body); {
		size := int(le.Uint16(body[at:]))
		kind := AttributeType(le.Uint16(body[at+2:]))

		if size == 0 && kind == 0 {
			break
		}

		at += 4
		if at+size > len(body) {
			return list, fmt.Errorf("backup: attribute %d's %d bytes run past its record", kind, size)
		}

		list = append(list, Attribute{Type: kind, Value: body[at : at+size]})
		at += size
	}

	return list, nil
}

// unsigned reads a little-endian value of up to eight bytes.
func unsigned(value []byte) uint64 {
	var n uint64
	for i := len(value) - 1; i >= 0; i-- {
		n = n<<8 | uint64(value[i])
	}

	return n
}

// fileID reads a three-word file ID (number, sequence, relative volume).
func fileID(value []byte) (id [3]uint16) {
	for i := 0; i < 3 && 2*i+1 < len(value); i++ {
		id[i] = binary.LittleEndian.Uint16(value[2*i:])
	}

	return id
}

// Summary is the save set summary, the first record of block 1.
type Summary struct {
	Name          string
	Command       string
	Comment       string
	UserName      string // as written: blank-padded to 12
	UserUIC       uint32 // group in the high word, member in the low
	Date          uint64
	OpSys         uint16
	SystemVersion string
	NodeName      string
	CPUID         uint32
	Device        string
	BackupVersion string
	BlockSize     uint32
	GroupSize     uint16
	Buffers       uint16

	// List is every attribute as read, in order.
	List []Attribute
}

// parseSummary decodes a summary record's body.
func parseSummary(body []byte) (Summary, error) {
	list, err := attributeList(body)
	s := Summary{List: list}

	for _, a := range list {
		v := a.Value
		switch a.Type {
		case AttrSaveSetName:
			s.Name = string(v)
		case AttrCommand:
			s.Command = string(v)
		case AttrComment:
			s.Comment = string(v)
		case AttrUserName:
			s.UserName = string(v)
		case AttrUserUIC:
			s.UserUIC = uint32(unsigned(v))
		case AttrDate:
			s.Date = unsigned(v)
		case AttrOpSys:
			s.OpSys = uint16(unsigned(v))
		case AttrSystemVersion:
			s.SystemVersion = string(v)
		case AttrNodeName:
			s.NodeName = string(v)
		case AttrCPUID:
			s.CPUID = uint32(unsigned(v))
		case AttrDevice:
			s.Device = string(v)
		case AttrBackupVersion:
			s.BackupVersion = string(v)
		case AttrBlockSize:
			s.BlockSize = uint32(unsigned(v))
		case AttrGroupSize:
			s.GroupSize = uint16(unsigned(v))
		case AttrBuffers:
			s.Buffers = uint16(unsigned(v))
		}
	}

	return s, err
}

// File characteristic bits (FH2$L_FILECHAR) the reader needs.
const (
	CharNoBackup          uint32 = 0x00000002
	CharContiguousBestTry uint32 = 0x00000020
	CharContiguous        uint32 = 0x00000080
	CharDirectory         uint32 = 0x00002000
)

// Attributes is a file record's attributes: what a restore gives the
// new file's header.
type Attributes struct {
	Name           string
	StructureLevel uint16
	FID, BackLink  [3]uint16
	Allocated      uint32 // blocks
	Owner          uint32 // group in the high word, member in the low
	Protection     uint16 // a bit set denies
	RecordProt     uint16
	AccessLevel    uint8

	// Characteristics are FH2$L_FILECHAR's bits.
	Characteristics uint32

	// RecordAttributes is the file header's record attributes area
	// (FAT), as the header holds it; the methods below read its fields.
	RecordAttributes [32]byte

	Revision                          uint16
	Created, Revised, Expires, Backup uint64

	// The directory's owner, protection, and version limit, which every
	// file record carries; DirStatus a directory file's own status.
	DirOwner        uint32
	DirProtection   uint16
	DirVersionLimit uint16
	DirStatus       uint32

	VersionLimit uint16
	Highwater    uint32
	Journal      uint8
	RUActive     uint8

	// ACL is the ACEs of every ACLSEGMENT attribute, packed in order.
	ACL []byte

	// List is every attribute as read, in order.
	List []Attribute
}

// parseAttributes decodes a file record's body.
func parseAttributes(body []byte) (Attributes, error) {
	list, err := attributeList(body)
	f := Attributes{List: list}

	for _, a := range list {
		v := a.Value
		switch a.Type {
		case AttrFileName:
			f.Name = string(v)
		case AttrStrucLev:
			f.StructureLevel = uint16(unsigned(v))
		case AttrFID:
			f.FID = fileID(v)
		case AttrBackLink:
			f.BackLink = fileID(v)
		case AttrFileSize:
			f.Allocated = uint32(unsigned(v))
		case AttrOwner:
			f.Owner = uint32(unsigned(v))
		case AttrProtection:
			f.Protection = uint16(unsigned(v))
		case AttrRecordProt:
			f.RecordProt = uint16(unsigned(v))
		case AttrAccessLevel:
			f.AccessLevel = uint8(unsigned(v))
		case AttrCharacteristics:
			f.Characteristics = uint32(unsigned(v))
		case AttrRecordAttrs:
			copy(f.RecordAttributes[:], v)
		case AttrRevision:
			f.Revision = uint16(unsigned(v))
		case AttrCreated:
			f.Created = unsigned(v)
		case AttrRevised:
			f.Revised = unsigned(v)
		case AttrExpires:
			f.Expires = unsigned(v)
		case AttrBackedUp:
			f.Backup = unsigned(v)
		case AttrDirOwner:
			f.DirOwner = uint32(unsigned(v))
		case AttrDirProtection:
			f.DirProtection = uint16(unsigned(v))
		case AttrDirStatus:
			f.DirStatus = uint32(unsigned(v))
		case AttrDirVersionLimit:
			f.DirVersionLimit = uint16(unsigned(v))
		case AttrVersionLimit:
			f.VersionLimit = uint16(unsigned(v))
		case AttrACLSegment:
			f.ACL = append(f.ACL, v...)
		case AttrHighwater:
			f.Highwater = uint32(unsigned(v))
		case AttrJournal:
			f.Journal = uint8(unsigned(v))
		case AttrRUActive:
			f.RUActive = uint8(unsigned(v))
		}
	}

	return f, err
}

// The record attributes area's fields (an ODS-2 header's FAT). The two
// block numbers are stored high word first, as the VAX's file system
// keeps them.
const (
	fatRecordType = 0  // byte: organization (high nibble), format (low)
	fatRecordAttr = 1  // byte: FAB$M_FTN, CR, PRN, BLK
	fatRecordSize = 2  // word: the longest record, or the fixed size
	fatHighBlock  = 4  // long: the highest block allocated
	fatEndBlock   = 8  // long: the end-of-file block
	fatFirstFree  = 12 // word: the first free byte in the end block
	fatBucketSize = 14 // byte
	fatVFCSize    = 15 // byte
	fatMaxRecord  = 16 // word
	fatExtend     = 18 // word: the default extension
)

// swapped reads a FAT block number, high word first.
func swapped(b []byte) uint32 {
	le := binary.LittleEndian

	return uint32(le.Uint16(b))<<16 | uint32(le.Uint16(b[2:]))
}

// RecordFormat is the record format (FAB$C_FIX 1, VAR 2, VFC 3, STM 4,
// STMLF 5, STMCR 6; 0 undefined); Organization the file organization
// (0 sequential, 1 relative, 2 indexed).
func (f Attributes) RecordFormat() uint8 { return f.RecordAttributes[fatRecordType] & 0x0F }
func (f Attributes) Organization() uint8 { return f.RecordAttributes[fatRecordType] >> 4 }

// RecordAttr is FAT$B_RATTRIB: carriage control and spanning.
func (f Attributes) RecordAttr() uint8 { return f.RecordAttributes[fatRecordAttr] }

// RecordSize is FAT$W_RSIZE; MaxRecord FAT$W_MAXREC.
func (f Attributes) RecordSize() uint16 {
	return binary.LittleEndian.Uint16(f.RecordAttributes[fatRecordSize:])
}

func (f Attributes) MaxRecord() uint16 {
	return binary.LittleEndian.Uint16(f.RecordAttributes[fatMaxRecord:])
}

// HighBlock, EndBlock, and FirstFreeByte place the end of file.
func (f Attributes) HighBlock() uint32 { return swapped(f.RecordAttributes[fatHighBlock:]) }
func (f Attributes) EndBlock() uint32  { return swapped(f.RecordAttributes[fatEndBlock:]) }

func (f Attributes) FirstFreeByte() uint16 {
	return binary.LittleEndian.Uint16(f.RecordAttributes[fatFirstFree:])
}

// BucketSize, VFCSize, and Extend are the remaining FAT fields a
// listing shows.
func (f Attributes) BucketSize() uint8 { return f.RecordAttributes[fatBucketSize] }
func (f Attributes) VFCSize() uint8    { return f.RecordAttributes[fatVFCSize] }

func (f Attributes) Extend() uint16 {
	return binary.LittleEndian.Uint16(f.RecordAttributes[fatExtend:])
}

// Used is how many blocks the file's data uses, as BACKUP/LIST and
// DIRECTORY/SIZE count them: the end-of-file block, less one when the
// data ends at that block's start.
func (f Attributes) Used() uint32 {
	end := f.EndBlock()
	if end > 0 && f.FirstFreeByte() == 0 {
		return end - 1
	}

	return end
}

// NoBackup and Directory are file characteristics.
func (f Attributes) NoBackup() bool  { return f.Characteristics&CharNoBackup != 0 }
func (f Attributes) Directory() bool { return f.Characteristics&CharDirectory != 0 }
