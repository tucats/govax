package rtl

import (
	"encoding/binary"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Attribute lists (docs/PHASE-26.md subtask 42): how a disk $QIO reads
// and writes what a file's header records about it.
//
// # The list
//
// The p5 argument of IO$_ACCESS, IO$_MODIFY, IO$_DEACCESS, and IO$_CREATE
// is the address of an *attribute list* ($ATRDEF): an array of 8-byte
// entries, ended by a zero longword, each saying "this attribute, this
// many bytes of it, at this address":
//
//	+0  ATR$W_SIZE  word      bytes to read or write
//	+2  ATR$W_TYPE  word      the attribute code (ATR$C_RECATTR, ...)
//	+4  ATR$L_ADDR  longword  the buffer
//
// IO$_ACCESS *reads*: each attribute is copied into its buffer. The other
// three *write*: each buffer's bytes replace the attribute's. So a program
// finds a file's end of file by reading ATR$C_RECATTR (the 32-byte record
// attribute area, $FATDEF: the end of file block is FAT$L_EFBLK at offset
// 8, the first free byte in it FAT$W_FFBYTE at 12) and sets it by
// writing that area back when it deaccesses the file, which is exactly
// what RMS does on every close.
//
// A size smaller than the attribute reads or writes just its first bytes
// (the rest of a written attribute stays as it was); a larger one, up to
// the attribute's $ATRDEF maximum (ATR$S_), reads the attribute followed
// by zeros (blanks for the name). An unknown code, a size of zero or past
// the maximum, or writing an attribute that can only be read, fails the
// request with SS$_BADATTRIB, before anything is written. A list or buffer
// the program can't access is $QIO's SS$_ACCVIO.
//
// # The attributes govax knows
//
// diskAttributes, below, a registry keyed by attribute code (in the style
// of the instruction table): each entry says how to get the attribute's
// bytes from rms's ACPAttributes and, for the writable ones, how to put
// them back.

// diskAttribute is one attribute code's handling.
type diskAttribute struct {
	// get returns the attribute's bytes.
	get func(*rms.ACPAttributes) []byte

	// set stores the attribute's bytes (as long as get returns); nil for
	// an attribute that can only be read.
	set func(*rms.ACPAttributes, []byte)

	// max is the largest size a list entry may give (ATR$S_), and fill
	// the byte a read pads with past the attribute's own bytes.
	max  int
	fill byte
}

// atrCode returns the $ATRDEF value called name, panicking for a name
// $ATRDEF doesn't have, like ioCode.
func atrCode(name string) uint32 {
	v, ok := vmsdef.ATRConstants[name]
	if !ok {
		panic("rtl: no $ATRDEF symbol " + name)
	}

	return v
}

// Field accessors for the attribute table.
func longBytes(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }
func quadBytes(v uint64) []byte { return binary.LittleEndian.AppendUint64(nil, v) }
func wordBytes(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }
func fidBytes(f rms.FileID) []byte {
	return append(append(wordBytes(f.Num), wordBytes(f.Seq)...), f.Rvn, f.Nmx)
}

// dateAttribute is the table entry for one of the four dates, reached
// through field.
func dateAttribute(field func(*rms.ACPAttributes) *uint64) diskAttribute {
	return diskAttribute{
		get: func(a *rms.ACPAttributes) []byte { return quadBytes(*field(a)) },
		set: func(a *rms.ACPAttributes, b []byte) { *field(a) = binary.LittleEndian.Uint64(b) },
		max: 8,
	}
}

// diskAttributes is every attribute code the disk driver reads or writes.
// Others (the RAD-50 name fields, access control lists, journaling, ...)
// are SS$_BADATTRIB.
var diskAttributes = map[uint32]diskAttribute{
	atrCode("ATR$C_UCHAR"): {
		get: func(a *rms.ACPAttributes) []byte { return longBytes(a.Characteristics) },
		set: func(a *rms.ACPAttributes, b []byte) { a.Characteristics = binary.LittleEndian.Uint32(b) },
		max: int(atrCode("ATR$S_UCHAR")),
	},
	atrCode("ATR$C_RECATTR"): {
		get: func(a *rms.ACPAttributes) []byte { return a.RecordAttributes[:] },
		set: func(a *rms.ACPAttributes, b []byte) { copy(a.RecordAttributes[:], b) },
		max: int(atrCode("ATR$S_RECATTR")),
	},
	atrCode("ATR$C_FPRO"): {
		get: func(a *rms.ACPAttributes) []byte { return wordBytes(a.Protection) },
		set: func(a *rms.ACPAttributes, b []byte) { a.Protection = binary.LittleEndian.Uint16(b) },
		max: int(atrCode("ATR$S_FPRO")),
	},
	atrCode("ATR$C_UIC"): {
		get: func(a *rms.ACPAttributes) []byte { return longBytes(a.Owner) },
		set: func(a *rms.ACPAttributes, b []byte) { a.Owner = binary.LittleEndian.Uint32(b) },
		max: int(atrCode("ATR$S_UIC")),
	},
	atrCode("ATR$C_CREDATE"): dateAttribute(func(a *rms.ACPAttributes) *uint64 { return &a.Created }),
	atrCode("ATR$C_REVDATE"): dateAttribute(func(a *rms.ACPAttributes) *uint64 { return &a.Revised }),
	atrCode("ATR$C_EXPDATE"): dateAttribute(func(a *rms.ACPAttributes) *uint64 { return &a.Expires }),
	atrCode("ATR$C_BAKDATE"): dateAttribute(func(a *rms.ACPAttributes) *uint64 { return &a.Backup }),
	atrCode("ATR$C_ASCNAME"): {
		get:  func(a *rms.ACPAttributes) []byte { return []byte(strings.ToUpper(a.Name)) },
		max:  int(atrCode("ATR$S_ASCNAME")),
		fill: ' ',
	},
	atrCode("ATR$C_HEADER"): {
		get: func(a *rms.ACPAttributes) []byte { return a.Header[:] },
		max: int(atrCode("ATR$S_HEADER")),
	},
	atrCode("ATR$C_BACKLINK"): {
		get: func(a *rms.ACPAttributes) []byte { return fidBytes(a.Backlink) },
		max: int(atrCode("ATR$S_BACKLINK")),
	},
	atrCode("ATR$C_HIGHWATER"): {
		get: func(a *rms.ACPAttributes) []byte { return longBytes(a.HighWater) },
		max: int(atrCode("ATR$S_HIGHWATER")),
	},
}

// ssBadAttrib is SS$_BADATTRIB: an attribute list entry govax can't do.
var ssBadAttrib = vmsdef.SSConstants["SS$_BADATTRIB"]

// maxAttributeEntries bounds how many entries a list is read for, so a
// list missing its terminating zero can't run through all of memory.
const maxAttributeEntries = 64

// attributeEntry is one decoded attribute list entry.
type attributeEntry struct {
	size uint32
	attr diskAttribute
	addr uint32
}

// readAttributeList decodes the attribute list at addr (0 for none).
// writing says which way the attributes go: true to write them to the
// file (the buffers are read), false to read them (the buffers are
// written). The returned status is SS$_ACCVIO for a list or buffer the
// program can't access that way, SS$_BADATTRIB for an entry govax can't
// do (see this file's opening comment), and otherwise SS$_NORMAL.
func (env *Environment) readAttributeList(addr uint32, writing bool) ([]attributeEntry, uint32) {
	var entries []attributeEntry

	for addr != 0 && len(entries) < maxAttributeEntries {
		sizeType, err1 := env.mem.LoadLongword(env.cpu, addr)
		buf, err2 := env.mem.LoadLongword(env.cpu, addr+4)

		switch {
		case err1 != nil:
			return nil, ssAccVio
		case sizeType == 0:
			return entries, ssNormal
		case err2 != nil:
			return nil, ssAccVio
		}

		size, code := sizeType&0xFFFF, sizeType>>16

		attr, ok := diskAttributes[code]
		if !ok || size == 0 || int(size) > attr.max || writing && attr.set == nil {
			return nil, ssBadAttrib
		}

		access := vm.AccessWrite
		if writing {
			access = vm.AccessRead
		}

		if !env.accessible(buf, size, access) {
			return nil, ssAccVio
		}

		entries = append(entries, attributeEntry{size: size, attr: attr, addr: buf})
		addr += 8
	}

	return entries, ssNormal
}

// storeAttributes copies each entry's attribute from a into the
// program's buffer (IO$_ACCESS).
func (env *Environment) storeAttributes(entries []attributeEntry, a *rms.ACPAttributes) {
	for _, e := range entries {
		out := make([]byte, e.size)
		n := copy(out, e.attr.get(a))

		for i := n; i < len(out); i++ {
			out[i] = e.attr.fill
		}

		_ = env.mem.Store(env.cpu, e.addr, out)
	}
}

// attributeChange loads each entry's bytes from the program's buffer and
// returns the change that writes them (for rms's WriteAttributes and
// friends), or nil for an empty list. The bytes are read now, so the
// change can't fail on the program's memory.
func (env *Environment) attributeChange(entries []attributeEntry) (func(*rms.ACPAttributes), bool) {
	if len(entries) == 0 {
		return nil, true
	}

	values := make([][]byte, len(entries))

	for i, e := range entries {
		b, err := loadBytes(env, e.addr, int(e.size))
		if err != nil {
			return nil, false
		}

		values[i] = []byte(b)
	}

	return func(a *rms.ACPAttributes) {
		for i, e := range entries {
			// Start from the attribute's current bytes, so a short
			// write changes only its first bytes.
			cur := append([]byte(nil), e.attr.get(a)...)
			copy(cur, values[i])
			e.attr.set(a, cur)
		}
	}, true
}
