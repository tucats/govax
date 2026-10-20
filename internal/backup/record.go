package backup

import (
	"encoding/binary"
	"fmt"
)

// RecordHeaderSize is the size of a record header; the record's body
// follows it.
const RecordHeaderSize = 16

// The record header's fields, by byte offset.
const (
	brhSize       = 0  // word: the body's size, not counting this header
	brhType       = 2  // word: the record's type
	brhFlags      = 4  // long: RecordFlag bits
	brhAddress    = 8  // long: a data record's first VBN
	brhBlockFlags = 12 // word: a per-block error mask
)

// RecordType is a record's type. A save of files uses Null, Summary,
// File, and VBN; the rest belong to image and physical saves and other
// systems, and are skipped.
type RecordType uint16

// The record types.
const (
	RecordNull       RecordType = 0  // fills the rest of a block
	RecordSummary    RecordType = 1  // the save set summary, block 1 only
	RecordVolume     RecordType = 2  // a volume summary (image saves)
	RecordFile       RecordType = 3  // a file's attributes
	RecordVBN        RecordType = 4  // a run of a file's virtual blocks
	RecordPhysVolume RecordType = 5  // physical volume attributes
	RecordLBN        RecordType = 6  // logical blocks
	RecordFileID     RecordType = 7  // a file identification
	RecordFileExtend RecordType = 8  // a file extension
	RecordLBN576     RecordType = 9  // 576-byte logical blocks
	RecordRSTSDir    RecordType = 10 // RSTS directory attributes
	RecordAliasEntry RecordType = 11 // a file alias entry
)

// RecordFlag is a record header's flag bits. Only RecordDirectory has
// been seen in VMS's save sets (on a directory's records); the others'
// bits are the public readers' order, unconfirmed.
type RecordFlag uint32

// The record flags.
const (
	RecordBadData       RecordFlag = 0x01
	RecordDirectory     RecordFlag = 0x02
	RecordNonSequential RecordFlag = 0x04
	RecordBlockErrors   RecordFlag = 0x08
	RecordAlias         RecordFlag = 0x10
	RecordHeadOnly      RecordFlag = 0x20
)

// Record is one record of a block.
type Record struct {
	Type    RecordType
	Flags   RecordFlag
	Address uint32 // a VBN record's first virtual block
	Body    []byte // shares the block's bytes
}

// Records splits the records out of block, which starts with its block
// header. A null record ends the block; so does its end. A record whose
// size runs past the block's end is an error.
func Records(block []byte) ([]Record, error) {
	le := binary.LittleEndian

	var records []Record

	for at := HeaderSize; at+RecordHeaderSize <= len(block); {
		size := int(le.Uint16(block[at+brhSize:]))
		r := Record{
			Type:    RecordType(le.Uint16(block[at+brhType:])),
			Flags:   RecordFlag(le.Uint32(block[at+brhFlags:])),
			Address: le.Uint32(block[at+brhAddress:]),
		}

		if r.Type == RecordNull {
			break
		}

		body := at + RecordHeaderSize
		if body+size > len(block) {
			return records, fmt.Errorf("backup: a type %d record at offset %d runs past the block's end", r.Type, at)
		}

		r.Body = block[body : body+size]
		records = append(records, r)
		at = body + size
	}

	return records, nil
}
