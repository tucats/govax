package obj

import (
	"encoding/binary"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// This file builds the DST records real MACRO's debugger (DBG) records
// hold (docs/PHASE-29.md, subtask 12): a symbol record for each symbol
// the module defines, the source file's record, and the line-number
// table that maps code addresses to source lines.
//
// As with traceback (dst.go), the manuals don't give the layouts, so they
// come from real MACRO's objects: FORTH's and the two probes' in
// testdata/mar/list and testdata/mar/dst. The data type codes, and the
// string and array descriptors some symbol records carry, are the VAX
// calling standard's (a public manual); the other names are govax's.

// The DST record types of a symbol record: the symbol's data type. A
// label's is the type of the data directive that follows it (.LONG's is
// L), a constant's is L, and DSTLabel is a symbol with none (a label of
// code, or one followed by no data).
const (
	// DSTDataLU is an unsigned longword: .ADDRESS and .BLKA.
	DSTDataLU DSTType = 0x04
	// DSTDataB is a byte: .BYTE, .SIGNED_BYTE, and .BLKB.
	DSTDataB DSTType = 0x06
	// DSTDataW is a word: .WORD, .SIGNED_WORD, and .BLKW.
	DSTDataW DSTType = 0x07
	// DSTDataL is a longword: .LONG, .BLKL, and every constant.
	DSTDataL DSTType = 0x08
	// DSTDataQ is a quadword: .QUAD and .BLKQ.
	DSTDataQ DSTType = 0x09
	// DSTDataF is F_floating: .F_FLOATING and .BLKF.
	DSTDataF DSTType = 0x0A
	// DSTDataD is D_floating: .D_FLOATING and .BLKD.
	DSTDataD DSTType = 0x0B
	// DSTDataT is text: .ASCII (with a string descriptor) and .ASCID
	// (the address of its descriptor).
	DSTDataT DSTType = 0x0E
	// DSTDataP is packed decimal: .PACKED, with a string descriptor.
	DSTDataP DSTType = 0x15
	// DSTDataO is an octaword: .OCTA and .BLKO.
	DSTDataO DSTType = 0x1A
	// DSTDataG is G_floating: .G_FLOATING and .BLKG.
	DSTDataG DSTType = 0x1B
	// DSTDataH is H_floating: .H_FLOATING and .BLKH.
	DSTDataH DSTType = 0x1C
	// DSTDataASCIC is a counted string, .ASCIC. The calling standard's
	// name for the code isn't confirmed; this is govax's.
	DSTDataASCIC DSTType = 0x2D
	// DSTDataASCIZ is a zero-terminated string, .ASCIZ (govax's name, as
	// for DSTDataASCIC).
	DSTDataASCIZ DSTType = 0x2E
	// DSTLabel is a symbol with no data type: a label of code, a label
	// no data follows, or a relocatable assignment.
	DSTLabel DSTType = 0xBB
)

// The DST record types of the line-number information.
const (
	// DSTSourceFile describes the source file (DSTSourceFileRecord), and
	// a second one holds its line count (DSTLineCountRecord).
	DSTSourceFile DSTType = 0x9B
	// DSTLineNumbers holds line-number commands (see the Line*
	// functions); the table runs on across as many as it needs.
	DSTLineNumbers DSTType = 0xB9
)

// What a symbol record's first byte says its value is.
const (
	// SymbolValue: the longword is the symbol's value (a constant, or a
	// DSTLabel symbol's address).
	SymbolValue byte = 0
	// SymbolAddress: the longword is the address of the symbol's data.
	SymbolAddress byte = 1
	// SymbolDescriptorAddress: the longword is the address of a
	// descriptor of the data (.ASCID's).
	SymbolDescriptorAddress byte = 2
	// symbolDescriptor: a descriptor of the data follows the name, and
	// the longword is how far after the longword it starts.
	symbolDescriptor byte = 0xFA
)

// Descriptor classes and array flags, as the calling standard defines
// them: a string descriptor (S), and an array's (A), whose flags real
// MACRO always sets to E0 (column-major order, with the coefficients and
// bounds blocks present).
const (
	descClassS  = 1
	descClassA  = 4
	arrayFlagsA = 0xE0
)

func init() {
	for t, name := range map[DSTType]string{
		DSTDataLU: "LU symbol", DSTDataB: "B symbol", DSTDataW: "W symbol", DSTDataL: "L symbol",
		DSTDataQ: "Q symbol", DSTDataF: "F symbol", DSTDataD: "D symbol", DSTDataT: "T symbol",
		DSTDataP: "P symbol", DSTDataO: "O symbol", DSTDataG: "G symbol", DSTDataH: "H symbol",
		DSTDataASCIC: "ASCIC symbol", DSTDataASCIZ: "ASCIZ symbol", DSTLabel: "label",
		DSTSourceFile: "source file", DSTLineNumbers: "line numbers",
	} {
		dstTypeNames[t] = name
	}
}

// isSymbolRecord reports whether t is a symbol record's type.
func isSymbolRecord(t DSTType) bool {
	switch t {
	case DSTDataLU, DSTDataB, DSTDataW, DSTDataL, DSTDataQ, DSTDataF, DSTDataD, DSTDataT,
		DSTDataP, DSTDataO, DSTDataG, DSTDataH, DSTDataASCIC, DSTDataASCIZ, DSTLabel:
		return true
	}

	return false
}

// longword returns v's four bytes, low byte first.
func longword(v uint32) []byte {
	return binary.LittleEndian.AppendUint32(nil, v)
}

// longAddress returns the commands that store an offset in a psect as an
// address, with STA_PL whatever its size, as a symbol record's is.
func longAddress(psect uint16, offset uint32) []Command {
	return addressCommands(Command{Op: OpStackPsectLong, Psect: psect, Value: offset})
}

// DSTValueRecord returns the record of a symbol whose value is the
// longword itself: a constant (of type DSTDataL) or a label in an
// absolute psect.
func DSTValueRecord(typ DSTType, name string, value uint32) DSTRecord {
	data := append([]byte{SymbolValue}, longword(value)...)

	return DSTRecord{Type: typ, Data: append(data, counted(name)...)}
}

// DSTAddressRecord returns the record of a symbol at an offset in a
// psect: what (SymbolValue for a DSTLabel symbol, SymbolAddress for a
// label of data, SymbolDescriptorAddress for .ASCID's) says what the
// address is.
func DSTAddressRecord(typ DSTType, name string, what byte, psect uint16, offset uint32) DSTRecord {
	data := append([]byte{what}, make([]byte, 4)...)

	return DSTRecord{
		Type: typ, Data: append(data, counted(name)...),
		Addresses: []DSTAddress{{Offset: 1, Commands: longAddress(psect, offset)}},
	}
}

// descriptorRecord returns a symbol record whose descriptor follows the
// name: desc, with the address of the data stored at each of its
// offsets at.
func descriptorRecord(typ DSTType, name string, psect uint16, offset uint32, desc []byte, at ...int) DSTRecord {
	cn := counted(name)
	data := append([]byte{symbolDescriptor}, longword(uint32(len(cn)))...)
	data = append(data, cn...)
	base := len(data)
	data = append(data, desc...)

	r := DSTRecord{Type: typ, Data: data}
	for _, a := range at {
		r.Addresses = append(r.Addresses, DSTAddress{Offset: base + a, Commands: longAddress(psect, offset)})
	}

	return r
}

// DSTStringRecord returns the record of a label of a string (.ASCII's, of
// type DSTDataT) or packed decimal (.PACKED's, DSTDataP), with a string
// descriptor: its length (a packed number's in digits), type, class S,
// and address.
func DSTStringRecord(typ DSTType, name string, length uint16, psect uint16, offset uint32) DSTRecord {
	desc := []byte{byte(length), byte(length >> 8), byte(typ), descClassS, 0, 0, 0, 0}

	return descriptorRecord(typ, name, psect, offset, desc, 4)
}

// DSTArrayRecord returns the record of a label of count elements of type
// typ, each size bytes (a list, or a .BLKx of more than one), with an
// array descriptor of one dimension: the element's size, type, and class
// A, the address, scale and digits (0), flags, the dimension count, the
// array's size in bytes, its virtual origin (the address again), the
// count, and the bounds 0 and count-1. Real MACRO writes the upper bound
// in a word, extended with zeros: .BLKB 0's is FFFF.
func DSTArrayRecord(typ DSTType, name string, size uint16, count uint32, psect uint16, offset uint32) DSTRecord {
	desc := []byte{byte(size), byte(size >> 8), byte(typ), descClassA, 0, 0, 0, 0, 0, 0, arrayFlagsA, 1}
	desc = append(desc, longword(uint32(size)*count)...)
	desc = append(desc, 0, 0, 0, 0)
	desc = append(desc, longword(count)...)
	desc = append(desc, longword(0)...)
	desc = append(desc, longword(uint32(uint16(count-1)))...)

	return descriptorRecord(typ, name, psect, offset, desc, 4, 16)
}

// SourceFile is what a source-file record says of the source: its full
// file specification, revision date, size (the block holding its end of
// file, and the first free byte in that block), and RMS record format
// (FAB$C_STMLF, 5, for a host file).
type SourceFile struct {
	Spec      string
	Revised   time.Time
	EOFBlock  uint32
	FirstFree uint16
	Format    byte
}

// revisedTime is t as a VMS time (its wall-clock reading), or 0 for no
// time.
func revisedTime(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}

	return vmsdef.Time(t)
}

// RecordFormatStreamLF is RMS's stream-LF record format, the one a
// source on the host is described as having.
const RecordFormatStreamLF = 5

// DSTSourceFileRecord returns the record describing the source file: 10
// 01, a word counting the bytes from itself through the specification,
// 01 00 (the file's number), the revision date, the end of file block
// (a longword) and first free byte (a word), the record format, the
// counted specification, then 00 02 01 00 04 01 00 06 01 00. What the
// fixed bytes mean isn't known; every real object has them.
func DSTSourceFileRecord(f SourceFile) DSTRecord {
	data := []byte{0x10, 0x01}
	data = binary.LittleEndian.AppendUint16(data, uint16(20+len(f.Spec)))
	data = append(data, 0x01, 0x00)
	data = binary.LittleEndian.AppendUint64(data, revisedTime(f.Revised))
	data = binary.LittleEndian.AppendUint32(data, f.EOFBlock)
	data = binary.LittleEndian.AppendUint16(data, f.FirstFree)
	data = append(data, f.Format)
	data = append(data, counted(f.Spec)...)
	data = append(data, 0x00, 0x02, 0x01, 0x00, 0x04, 0x01, 0x00, 0x06, 0x01, 0x00)

	return DSTRecord{Type: DSTSourceFile, Data: data}
}

// DSTLineCountRecord returns the record that ends the line-number
// information with the source's line count: 0B and a byte, or 0A and a
// word.
func DSTLineCountRecord(lines int) DSTRecord {
	if lines <= 0xFF {
		return DSTRecord{Type: DSTSourceFile, Data: []byte{0x0B, byte(lines)}}
	}

	return DSTRecord{Type: DSTSourceFile, Data: []byte{0x0A, byte(lines), byte(lines >> 8)}}
}

// The line-number table's commands. The table is a series of rows, each
// a line and the address its code starts at. A segment (a run of code
// with no gap) starts with LineSetAddress and LineSetLine (to the line
// before its first), then LineAdvance(0) makes its first row. Each
// LineAdvance after that steps past a row's code to the next line, which
// LineSkip (lines with no code between) or LineSetLine (any other line)
// can move first. LineEnd ends a segment with its last row's size.

// LineSetAddress is the command that starts a segment at a psect offset:
// 10 and the address, which stack (STA_PB, STA_PW, or STA_PL) gives.
func LineSetAddress(stack Command) DSTItem {
	return DSTItem{Bytes: []byte{0x10}, Address: addressCommands(stack)}
}

// LineSetLine sets the line to n: 13 and its low byte, then a skip of the
// rest (03 and a word), as FORTH's 329 is 13 49 03 00 01.
func LineSetLine(n int) DSTItem {
	b := []byte{0x13, byte(n)}
	if rest := n &^ 0xFF; rest > 0 {
		b = append(b, 0x03, byte(rest), byte(rest>>8))
	}

	return DSTItem{Bytes: b}
}

// LineSkip moves the line on by n lines with no code: 02 and a byte, or
// 03 and a word.
func LineSkip(n int) DSTItem {
	if n <= 0xFF {
		return DSTItem{Bytes: []byte{0x02, byte(n)}}
	}

	return DSTItem{Bytes: []byte{0x03, byte(n), byte(n >> 8)}}
}

// LineAdvance steps past size bytes of code to the next line, making a
// row: a negative byte, -size (0 for none), or 01 and a word. Where the
// byte form stops isn't confirmed; it's used up to 128.
func LineAdvance(size uint32) DSTItem {
	if size <= 128 {
		return DSTItem{Bytes: []byte{byte(-int8(int(size)))}}
	}

	return DSTItem{Bytes: []byte{0x01, byte(size), byte(size >> 8)}}
}

// LineEnd ends a segment whose last row has size bytes: 0E and a byte.
// The form for 256 bytes and more (0F and a word) isn't confirmed.
func LineEnd(size uint32) DSTItem {
	if size <= 0xFF {
		return DSTItem{Bytes: []byte{0x0E, byte(size)}}
	}

	return DSTItem{Bytes: []byte{0x0F, byte(size), byte(size >> 8)}}
}

// LineStart is the bytes the line-number table starts with: 13 00.
func LineStart() DSTItem {
	return DSTItem{Bytes: []byte{0x13, 0x00}}
}

// DSTItem is a piece of DST data: bytes, then, if Address isn't nil, an
// address the linker stores (four bytes in the DST).
type DSTItem struct {
	Bytes   []byte
	Address []Command
}

func (it DSTItem) size() int {
	if it.Address != nil {
		return len(it.Bytes) + 4
	}

	return len(it.Bytes)
}
