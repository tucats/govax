package dbgsym

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// The DST record types this reader interprets (docs/DEBUG-RECORDS.md,
// section 4.4). A record type from 1 up to maxDataType is a data symbol
// whose type is that DSC$K_DTYPE code (section 4.2). Others (blocks,
// types, fixups, which MACRO doesn't write) are counted in
// Program.Skipped.
const (
	typeContinuation = 173 // DST$K_CONTIN: the rest of the record before it
	typeEntry        = 181 // DST$K_ENTRY: a secondary entry point
	typePsect        = 184 // DST$K_PSECT
	typeLineNumbers  = 185 // DST$K_LINE_NUM
	typeLabelOrLit   = 186 // DST$K_LBLORLIT: a label or a constant (MACRO)
	typeLabel        = 187 // DST$K_LABEL
	typeModuleBegin  = 188 // DST$K_MODBEG
	typeModuleEnd    = 189 // DST$K_MODEND
	typeRoutineBegin = 190 // DST$K_RTNBEG
	typeRoutineEnd   = 191 // DST$K_RTNEND
	typeSource       = 155 // DST$K_SOURCE: source file correlation

	// maxDataType is the highest type code read as a data symbol. The
	// calling standard's codes run to 0x3F or so; the DST's own record
	// types start at 116 (DST$K_LOWEST).
	maxDataType = 115
)

// Value kinds and special flags of a data record's first byte
// (DST$B_VFLAGS, section 7.3 and 7.4).
const (
	valKindMask    = 0x03
	valKindLiteral = 0 // DST$K_VALKIND_LITERAL: the value is the constant
	valKindAddress = 1 // DST$K_VALKIND_ADDR: the value is the address
	valKindDesc    = 2 // DST$K_VALKIND_DESC: the value is a descriptor's address
	valKindReg     = 3 // DST$K_VALKIND_REG: the object is in a register

	vflagIndirect = 0x04 // DST$V_INDIRECT
	vflagDisp     = 0x08 // DST$V_DISP

	vflagsNoValue    = 128 // DST$K_VFLAGS_NOVAL
	vflagsDescriptor = 250 // DST$K_VFLAGS_DSC: a descriptor follows the name
)

// record is one DST record: its type and its fields (the bytes after the
// type), with any continuation records joined on.
type record struct {
	offset int // where it starts in the DST
	typ    byte
	data   []byte
}

var le = binary.LittleEndian

// errShort is a record too short for its fields.
var errShort = errors.New("record too short")

// readRecords splits a DST into its records. The stream ends at its end
// or at a zero length byte (the padding after the last record). A
// continuation record's fields are joined onto the record before it
// (section 16).
func readRecords(dst []byte) ([]record, error) {
	var out []record

	for i := 0; i < len(dst); {
		n := int(dst[i])
		if n == 0 {
			break
		}

		if i+1+n > len(dst) {
			return out, fmt.Errorf("DST record at %#x runs past the table's end", i)
		}

		r := record{offset: i, typ: dst[i+1], data: dst[i+2 : i+1+n]}

		if r.typ == typeContinuation && len(out) > 0 {
			last := &out[len(out)-1]
			last.data = append(append([]byte(nil), last.data...), r.data...)
		} else {
			out = append(out, r)
		}

		i += n + 1
	}

	return out, nil
}

// counted reads a counted string (a length byte, then that many
// characters) at b[at:], returning it and the offset after it.
func counted(b []byte, at int) (string, int, error) {
	if at >= len(b) || at+1+int(b[at]) > len(b) {
		return "", 0, errShort
	}

	n := int(b[at])

	return string(b[at+1 : at+1+n]), at + 1 + n, nil
}

// long reads a longword at b[at:].
func long(b []byte, at int) (uint32, error) {
	if at+4 > len(b) {
		return 0, errShort
	}

	return le.Uint32(b[at:]), nil
}

// addrNameRecord decodes the common layout of a routine begin, a label,
// a psect, and an entry point: a flags byte, an address, and a counted
// name; next is the offset after the name.
func addrNameRecord(r record) (flags byte, addr uint32, name string, next int, err error) {
	if len(r.data) < 6 {
		return 0, 0, "", 0, errShort
	}

	addr, _ = long(r.data, 1)
	name, next, err = counted(r.data, 5)

	return r.data[0], addr, name, next, err
}
