package obj

import (
	"errors"
	"fmt"
	"strings"
)

// This file reads and writes the debug symbol table (DST) records that
// traceback (TBT) and debugger (DBG) records carry (docs/PHASE-29.md,
// subtask 10).
//
// A TBT or DBG record holds TIR commands, as a TIR record does, but the
// bytes they store don't go into the image: the linker gathers them into
// the image's debug symbol table, which the traceback facility and the
// debugger read (the VMS 5.0 Linker Utility Manual, 7.7 and 7.8). Those
// bytes are a stream of DST records, each:
//
//	+--------+--------+---------------------------+
//	| length |  type  |  fields (length-1 bytes)  |
//	+--------+--------+---------------------------+
//
// where length counts the bytes after itself. An address in a record (a
// routine's entry point, a psect's base) is stored by the linker: the
// object stacks it (STA_PL psect offset) and stores it (STO_PIDR), and the
// stream has four bytes there.
//
// The manuals don't give the records' layouts. govax works in a clean
// room, so they come from real VAX MACRO's objects instead: the 21
// fixtures' and the Phase 29 probe's (testdata/mar). Real MACRO's
// traceback is four kinds of record, which DSTType names; the debugger
// records (/DEBUG) have more (dbg.go, subtask 12). A DST record may
// start in one TBT record and end in the
// next, so a module's TBT records are one stream, and its DBG records
// another.

// DSTType is a DST record's type: the byte after its length.
type DSTType byte

// The DST record types real MACRO's traceback records hold, named by what
// they describe. The values are the ones its objects use; the names are
// govax's.
const (
	// DSTPsect describes a psect: a byte (0 in every real object), the
	// psect's base address, its name (counted), and its size (a
	// longword). There is one for each psect but the absolute one.
	DSTPsect DSTType = 0xB8
	// DSTModuleBegin starts a module's records: five bytes (0 in every
	// real object) and the module's name (counted).
	DSTModuleBegin DSTType = 0xBC
	// DSTModuleEnd ends a module's records. It has no fields.
	DSTModuleEnd DSTType = 0xBD
	// DSTRoutineBegin describes a routine (an .ENTRY): a byte (0 in every
	// real object), its entry address, and its name (counted).
	DSTRoutineBegin DSTType = 0xBE
)

// dstTypeNames are the names Dump gives the record types.
var dstTypeNames = map[DSTType]string{
	DSTPsect:        "psect",
	DSTModuleBegin:  "module begin",
	DSTModuleEnd:    "module end",
	DSTRoutineBegin: "routine begin",
}

func (t DSTType) String() string {
	if name, ok := dstTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("type %02X", byte(t))
}

// DSTRecord is one DST record.
type DSTRecord struct {
	Type DSTType
	// Data is the record's fields: its bytes after the type. Where the
	// linker stores an address, Data holds zeros.
	Data []byte
	// Addresses are the values the linker stores into Data, in order.
	Addresses []DSTAddress
}

// DSTAddress is a value the linker stores into a DST record: the TIR
// commands that stack it and the one that stores it, at Offset in the
// record's Data.
type DSTAddress struct {
	Offset   int
	Commands []Command
}

// storeSizes are how many bytes each store command a DST record may use
// stores.
var storeSizes = map[Op]int{
	opsByName["STO_SB"]:   1,
	opsByName["STO_B"]:    1,
	opsByName["STO_BD"]:   1,
	opsByName["STO_USB"]:  1,
	opsByName["STO_SW"]:   2,
	opsByName["STO_W"]:    2,
	opsByName["STO_WD"]:   2,
	opsByName["STO_USW"]:  2,
	opsByName["STO_L"]:    4,
	opsByName["STO_LD"]:   4,
	opsByName["STO_LI"]:   4,
	opsByName["STO_PIDR"]: 4,
}

// dstByte is one byte of a DST stream as decoded: the byte, the record
// (TBT or DBG) it came from, and, for the first byte of a stored value,
// the commands that store it.
type dstByte struct {
	b       byte
	record  int
	address []Command
	stored  bool // the byte is part of a stored value
}

// DecodeDST reads the DST records the commands of a module's TBT (or
// DBG) records store, in order: groups holds each record's commands. For
// each DST record it also returns the index in groups of the record it
// ends in.
func DecodeDST(groups [][]Command) ([]DSTRecord, []int, error) {
	var (
		stream  []dstByte
		pending []Command
	)

	for g, cmds := range groups {
		for _, c := range cmds {
			if c.Op == OpStoreImmediate {
				if len(pending) > 0 {
					return nil, nil, errors.New("a value stacked and never stored")
				}

				for _, b := range c.Data {
					stream = append(stream, dstByte{b: b, record: g})
				}

				continue
			}

			pending = append(pending, c)

			info := ops[c.Op]
			if info.push > 0 || info.pop == 0 {
				continue
			}

			n, ok := storeSizes[c.Op]
			if !ok {
				return nil, nil, fmt.Errorf("%s in a DST record", c.Op)
			}

			for i := 0; i < n; i++ {
				stream = append(stream, dstByte{record: g, stored: true})
			}

			stream[len(stream)-n].address = pending
			pending = nil
		}
	}

	if len(pending) > 0 {
		return nil, nil, errors.New("a value stacked and never stored")
	}

	var (
		recs []DSTRecord
		ends []int
	)

	for i := 0; i < len(stream); {
		if stream[i].stored {
			return nil, nil, fmt.Errorf("a stored value where DST record %d's length should be", len(recs)+1)
		}

		length := int(stream[i].b)
		end := i + 1 + length

		if length < 1 || end > len(stream) {
			return nil, nil, fmt.Errorf("DST record %d: length %d, %d bytes left", len(recs)+1, length, len(stream)-i-1)
		}

		if stream[i+1].stored {
			return nil, nil, fmt.Errorf("DST record %d: a stored value where its type should be", len(recs)+1)
		}

		rec := DSTRecord{Type: DSTType(stream[i+1].b)}

		for k := i + 2; k < end; k++ {
			if stream[k].address != nil {
				if k+storeSize(stream[k].address) > end {
					return nil, nil, fmt.Errorf("DST record %d: a stored value runs past its end", len(recs)+1)
				}

				rec.Addresses = append(rec.Addresses, DSTAddress{Offset: k - i - 2, Commands: stream[k].address})
			}

			rec.Data = append(rec.Data, stream[k].b)
		}

		recs = append(recs, rec)
		ends = append(ends, stream[end-1].record)
		i = end
	}

	return recs, ends, nil
}

// storeSize returns how many bytes the commands that end in a store
// command store.
func storeSize(cmds []Command) int {
	return storeSizes[cmds[len(cmds)-1].Op]
}

// EncodeDST returns the TIR commands that store recs: each record's
// length, type, and fields, as STORE IMMEDIATE commands of up to
// MaxImmediate bytes, and each address as its own commands. Bytes on
// either side of a record's end share a STORE IMMEDIATE, as real MACRO's
// do.
func EncodeDST(recs []DSTRecord) ([]Command, error) {
	var (
		out []Command
		imm []byte
	)

	flush := func() {
		for len(imm) > 0 {
			n := min(len(imm), MaxImmediate)
			out = append(out, Command{Op: OpStoreImmediate, Data: append([]byte(nil), imm[:n]...)})
			imm = imm[n:]
		}
	}

	for i, r := range recs {
		if len(r.Data)+1 > 0xFF {
			return nil, fmt.Errorf("DST record %d: %d bytes is too long", i+1, len(r.Data))
		}

		imm = append(imm, byte(len(r.Data)+1), byte(r.Type))
		at := 0

		for _, a := range r.Addresses {
			n := storeSize(a.Commands)
			if a.Offset < at || a.Offset+n > len(r.Data) {
				return nil, fmt.Errorf("DST record %d: an address at %d overlaps", i+1, a.Offset)
			}

			imm = append(imm, r.Data[at:a.Offset]...)
			flush()

			out = append(out, a.Commands...)
			at = a.Offset + n
		}

		imm = append(imm, r.Data[at:]...)
	}

	flush()

	return out, nil
}

// Name returns the counted name in a module begin, routine begin, psect,
// or symbol record (dbg.go), or "" for another record. Each has five
// bytes before it: a module begin's zeros, or a byte and the address or
// value.
func (r DSTRecord) Name() string {
	const at = 5

	switch {
	case r.Type != DSTModuleBegin && r.Type != DSTRoutineBegin && r.Type != DSTPsect && !isSymbolRecord(r.Type),
		at >= len(r.Data), at+1+int(r.Data[at]) > len(r.Data):
		return ""
	}

	return string(r.Data[at+1 : at+1+int(r.Data[at])])
}

// counted returns name as a counted string: its length, then its bytes.
func counted(name string) []byte {
	return append([]byte{byte(len(name))}, name...)
}

// addressCommands returns the commands that store the value stack pushes
// as an address: stack, then STO_PIDR.
func addressCommands(stack Command) []Command {
	return []Command{stack, {Op: OpStorePIDataRef}}
}

// DSTModuleBeginRecord returns the module begin record for the module
// name.
func DSTModuleBeginRecord(name string) DSTRecord {
	return DSTRecord{Type: DSTModuleBegin, Data: append(make([]byte, 5), counted(name)...)}
}

// DSTModuleEndRecord returns the module end record.
func DSTModuleEndRecord() DSTRecord {
	return DSTRecord{Type: DSTModuleEnd}
}

// DSTRoutineBeginRecord returns the routine begin record for the routine
// name, whose entry address offset in psect real MACRO stacks with
// STA_PL whatever its size.
func DSTRoutineBeginRecord(name string, psect uint16, offset uint32) DSTRecord {
	data := append(make([]byte, 5), counted(name)...)
	stack := Command{Op: OpStackPsectLong, Psect: psect, Value: offset}

	return DSTRecord{Type: DSTRoutineBegin, Data: data, Addresses: []DSTAddress{{Offset: 1, Commands: addressCommands(stack)}}}
}

// DSTNoCallRoutineRecord returns a routine begin record flagged
// DST$V_RTNBEG_NO_CALL (80): a routine entered by JSB or BSB. Real MACRO
// writes one, with no name, at the base of each psect whose code begins
// with no .ENTRY, when it writes debugger records
// (testdata/mar/dst/vax/dstln2.obj).
func DSTNoCallRoutineRecord(name string, psect uint16, offset uint32) DSTRecord {
	r := DSTRoutineBeginRecord(name, psect, offset)
	r.Data[0] = 0x80

	return r
}

// DSTPsectRecord returns the psect record for the psect name, number
// psect, of size bytes. Real MACRO stacks its base with STA_PB.
func DSTPsectRecord(name string, psect uint16, size uint32) DSTRecord {
	data := append(make([]byte, 5), counted(name)...)
	data = append(data, byte(size), byte(size>>8), byte(size>>16), byte(size>>24))
	stack := Command{Op: opsByName["STA_PB"], Psect: psect}

	return DSTRecord{Type: DSTPsect, Data: data, Addresses: []DSTAddress{{Offset: 1, Commands: addressCommands(stack)}}}
}

// FormatDST describes a DST record on a line.
func FormatDST(r DSTRecord) string {
	var b strings.Builder

	fmt.Fprintf(&b, "DST %s", r.Type)

	if name := r.Name(); name != "" {
		fmt.Fprintf(&b, " %q", name)
	}

	switch {
	case r.Type == DSTPsect && len(r.Data) >= 4:
		n := len(r.Data)
		fmt.Fprintf(&b, ", %d bytes", uint32(r.Data[n-4])|uint32(r.Data[n-3])<<8|uint32(r.Data[n-2])<<16|uint32(r.Data[n-1])<<24)
	case (dstTypeNames[r.Type] == "" || isSymbolRecord(r.Type) || r.Type == DSTSourceFile || r.Type == DSTLineNumbers) && len(r.Data) > 0:
		fmt.Fprintf(&b, ": % x", r.Data)
	}

	for _, a := range r.Addresses {
		parts := make([]string, len(a.Commands))
		for i, c := range a.Commands {
			parts[i] = FormatCommand(c)
		}

		fmt.Fprintf(&b, "; at %d: %s", a.Offset, strings.Join(parts, ", "))
	}

	return b.String()
}
