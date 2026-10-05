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
// The layouts were worked out from real MACRO's objects (FORTH's and the
// two probes' in testdata/mar/list and testdata/mar/dst), then checked
// against docs/DEBUG-RECORDS.md, a clean-room description of the DST
// format, whose names (DST$K_...) the comments give. The data type codes
// (DSC$K_DTYPE_...) and descriptors are the VAX calling standard's.

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
	// DSTDataASCIC is counted ASCII text, .ASCIC (DSC$K_DTYPE_AC, a
	// debugger extension code).
	DSTDataASCIC DSTType = 0x2D
	// DSTDataASCIZ is zero-terminated ASCII text, .ASCIZ
	// (DSC$K_DTYPE_AZ).
	DSTDataASCIZ DSTType = 0x2E
	// DSTLabel (DST$K_LABEL, 187) names a code address: real MACRO's
	// record for a label of code, a label no data follows, or a
	// relocatable assignment.
	DSTLabel DSTType = 0xBB
)

// The DST record types of the line-number information.
const (
	// DSTSourceFile (DST$K_SOURCE, 155) holds source correlation
	// commands: real MACRO's first declares the source file
	// (DSTSourceFileRecord), and a second maps the lines to its records
	// (DSTLineCountRecord).
	DSTSourceFile DSTType = 0x9B
	// DSTLineNumbers (DST$K_LINE_NUM, 185) holds line-number commands
	// (the Line* functions): a module's line-number records are one
	// stream of them, split across as many records as it needs.
	DSTLineNumbers DSTType = 0xB9
)

// What a symbol record's first byte (DST$B_VFLAGS) says its value is.
const (
	// SymbolValue (DST$K_VALKIND_LITERAL): the longword is the symbol's
	// value (a constant, or a DSTLabel symbol's address).
	SymbolValue byte = 0
	// SymbolAddress (DST$K_VALKIND_ADDR): the longword is the address of
	// the symbol's data.
	SymbolAddress byte = 1
	// SymbolDescriptorAddress (DST$K_VALKIND_DESC): the longword is the
	// address of a descriptor of the data (.ASCID's).
	SymbolDescriptorAddress byte = 2
	// symbolDescriptor (DST$K_VFLAGS_DSC): a descriptor of the data
	// follows the name, and the longword is its offset from the name's
	// count byte.
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
// file specification, creation date, size (the block holding its end of
// file, and the first free byte in that block), and RMS record format
// and organization (5, stream-LF and sequential, for a host file).
type SourceFile struct {
	Spec      string
	Created   time.Time
	EOFBlock  uint32
	FirstFree uint16
	Format    byte
}

// vmsTime is t as a VMS time (its wall-clock reading), or 0 for no
// time.
func vmsTime(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}

	return vmsdef.Time(t)
}

// maxSourceSpec is the longest file specification a source file's
// record has room for: 254 bytes, less the record's 32 others.
const maxSourceSpec = 254 - 32

// RecordFormatStreamLF is RMS's stream-LF record format, the one a
// source on the host is described as having.
const RecordFormatStreamLF = 5

// DSTSourceFileRecord returns the source correlation record real MACRO
// starts with, of these commands:
//
//   - DST$K_SRC_FORMFEED (10): a line of only a form feed is a line;
//   - DST$K_SRC_DECLFILE (01): the byte count after it, flags (0), the
//     file's ID (1), its creation date, end of file block, first free
//     byte, and record format, the counted file specification, and the
//     counted library module name (empty);
//   - DST$K_SRC_SETFILE 1, DST$K_SRC_SETREC_W 1, DST$K_SRC_SETLNUM_W 1
//     (02, 04, 06): line 1 is the file's record 1.
//
// A DST record holds at most 254 bytes after its type, so a specification
// longer than maxSourceSpec (a long host path) keeps its last
// maxSourceSpec characters (govax's choice).
func DSTSourceFileRecord(f SourceFile) DSTRecord {
	if len(f.Spec) > maxSourceSpec {
		f.Spec = f.Spec[len(f.Spec)-maxSourceSpec:]
	}

	data := []byte{0x10, 0x01, byte(20 + len(f.Spec)), 0x00, 0x01, 0x00}
	data = binary.LittleEndian.AppendUint64(data, vmsTime(f.Created))
	data = binary.LittleEndian.AppendUint32(data, f.EOFBlock)
	data = binary.LittleEndian.AppendUint16(data, f.FirstFree)
	data = append(data, f.Format)
	data = append(data, counted(f.Spec)...)
	data = append(data, 0x00, 0x02, 0x01, 0x00, 0x04, 0x01, 0x00, 0x06, 0x01, 0x00)

	return DSTRecord{Type: DSTSourceFile, Data: data}
}

// DSTLineCountRecord returns the source correlation record that maps the
// source's lines, one to one, to its records: DST$K_SRC_DEFLINES_B (0B)
// and a byte, or DST$K_SRC_DEFLINES_W (0A) and a word, as many as it
// takes.
func DSTLineCountRecord(lines int) DSTRecord {
	var data []byte

	for ; lines > 0xFFFF; lines -= 0xFFFF {
		data = append(data, 0x0A, 0xFF, 0xFF)
	}

	if lines <= 0xFF {
		data = append(data, 0x0B, byte(lines))
	} else {
		data = append(data, 0x0A, byte(lines), byte(lines>>8))
	}

	return DSTRecord{Type: DSTSourceFile, Data: data}
}

// The line-number table's commands (docs/DEBUG-RECORDS.md, section 13).
// The table is a series of rows, each a line and the address its code
// starts at. Real MACRO starts the table with LineStart. A segment (a run
// of code with no gap) starts with LineSetAddress and LineSetLine (to
// the line before its first), then LineAdvance(0) makes its first row.
// Each LineAdvance after that steps past a row's code to the next line,
// which LineSkip (lines with no code between) or LineSetLine (any other
// line) can move first. LineEnd ends a segment with its last row's size.

// LineSetAddress is DST$K_SET_ABS_PC (10) and the address, which stack
// (STA_PB, STA_PW, or STA_PL) gives: where a segment starts.
func LineSetAddress(stack Command) DSTItem {
	return DSTItem{Bytes: []byte{0x10}, Address: addressCommands(stack)}
}

// LineSetLine sets the line to n as real MACRO does: DST$K_SET_LINUM_B
// (13) and its low byte, then DST$K_INCR_LINUM_W (03) by the rest, as
// FORTH's 329 is 13 49 03 00 01.
func LineSetLine(n int) DSTItem {
	b := []byte{0x13, byte(n)}
	if rest := n &^ 0xFF; rest > 0 {
		b = append(b, LineSkip(rest).Bytes...)
	}

	return DSTItem{Bytes: b}
}

// LineSkip moves the line on by n lines with no code: DST$K_INCR_LINUM
// (02) and a byte, _W (03) and a word, or _L (18) and a longword.
func LineSkip(n int) DSTItem {
	switch {
	case n <= 0xFF:
		return DSTItem{Bytes: []byte{0x02, byte(n)}}
	case n <= 0xFFFF:
		return DSTItem{Bytes: []byte{0x03, byte(n), byte(n >> 8)}}
	}

	return DSTItem{Bytes: append([]byte{0x12}, longword(uint32(n))...)}
}

// LineAdvance steps past size bytes of code to the next line, making a
// row: a Delta-PC byte, -size (0 to -128), or DST$K_DELTA_PC_W (01) and a
// word, or _L (17) and a longword.
func LineAdvance(size uint32) DSTItem {
	switch {
	case size <= 128:
		return DSTItem{Bytes: []byte{byte(-int8(int(size)))}}
	case size <= 0xFFFF:
		return DSTItem{Bytes: []byte{0x01, byte(size), byte(size >> 8)}}
	}

	return DSTItem{Bytes: append([]byte{0x11}, longword(size)...)}
}

// LineEnd ends a segment whose last row has size bytes: DST$K_TERM (0E)
// and a byte, _W (0F) and a word, or _L (21) and a longword.
func LineEnd(size uint32) DSTItem {
	switch {
	case size <= 0xFF:
		return DSTItem{Bytes: []byte{0x0E, byte(size)}}
	case size <= 0xFFFF:
		return DSTItem{Bytes: []byte{0x0F, byte(size), byte(size >> 8)}}
	}

	return DSTItem{Bytes: append([]byte{0x15}, longword(size)...)}
}

// LineStart is what real MACRO starts the line-number table with:
// DST$K_SET_LINUM_B 0 (13 00).
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
