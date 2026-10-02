package main

import (
	"log"
	"sort"
	"strings"
)

// spec is one instruction's operand list from manualOperands, decoded into
// the same shape as a parsed instruction_table.h entry (see field).
type spec struct {
	count  int
	scale  [6]int
	access [6]string // OP_RD, OP_WR, ... (instruction_table.h's names)
	types  [6]string // the manual's data-type letters
}

// specAccess maps the manual's access letter to instruction_table.h's
// access constant, which is what the rest of the generator works in.
var specAccess = map[byte]string{
	'r': "OP_RD",
	'w': "OP_WR",
	'm': "OP_MD",
	'a': "OP_AD",
	'v': "OP_VA",
	'b': "OP_BR",
	'i': "OP_IM",
}

// specSize maps the manual's data-type letter to the operand's size in
// bytes.
var specSize = map[byte]int{
	'b': 1, 'w': 2, 'l': 4, 'q': 8, 'o': 16,
	'f': 4, 'd': 8, 'g': 8, 'h': 16,
}

// floatTypes are the data-type letters of the four VAX floating formats.
var floatTypes = map[string]bool{"f": true, "d": true, "g": true, "h": true}

// dataTypeName maps a data-type letter to the cpu package's DataType
// constant (internal/cpu/datatype.go) that the generated table uses.
var dataTypeName = map[string]string{
	"":  "DataNone",
	"b": "DataByte",
	"w": "DataWord",
	"l": "DataLongword",
	"q": "DataQuadword",
	"o": "DataOctaword",
	"f": "DataFFloating",
	"d": "DataDFloating",
	"g": "DataGFloating",
	"h": "DataHFloating",
}

// parseSpec decodes one manualOperands value ("rl,rl,wl") for the named
// instruction, stopping the generator on anything malformed.
func parseSpec(name, text string) spec {
	var s spec

	// Every unused slot is OP_NL, as in instruction_table.h.
	for i := range s.access {
		s.access[i] = "OP_NL"
	}

	if text == "" {
		return s
	}

	ops := strings.Split(text, ",")
	if len(ops) > 6 {
		log.Fatalf("gen: %s has %d operands; the VAX has at most 6", name, len(ops))
	}

	for i, op := range ops {
		if len(op) != 2 {
			log.Fatalf("gen: %s operand %d %q isn't two letters", name, i, op)
		}

		access, ok := specAccess[op[0]]
		if !ok {
			log.Fatalf("gen: %s operand %d has unknown access letter %q", name, i, op[0])
		}

		size, ok := specSize[op[1]]
		if !ok {
			log.Fatalf("gen: %s operand %d has unknown data-type letter %q", name, i, op[1])
		}

		s.access[i] = access
		s.scale[i] = size
		s.types[i] = op[1:]
	}

	s.count = len(ops)

	return s
}

// shortLiteralType works out an instruction's short-literal type from its
// data types: OP_TYPE_FLOAT if any operand it reads is a floating type,
// otherwise OP_TYPE_INT.
//
// A short literal (addressing modes 0-3) packs a 6-bit constant into the
// operand specifier byte itself. For an integer operand it means the
// integer 0-63; for a floating operand it means one of 64 floating values
// (0.5 to 120, the cpu package's shortDouble table). Only operands that are
// read can be literals, so only they count. This reproduces every
// instruction_table.h entry's type (applyManualOperands checks), and gives
// the blank and missing entries theirs.
func shortLiteralType(s spec) string {
	for i := 0; i < s.count; i++ {
		if (s.access[i] == "OP_RD" || s.access[i] == "OP_MD") && floatTypes[s.types[i]] {
			return "OP_TYPE_FLOAT"
		}
	}

	return "OP_TYPE_INT"
}

// isFiller reports whether an instruction_table.h entry is a placeholder
// rather than an instruction: the reserved opcodes (RSVD_xx) and the three
// slots for the bytes that introduce two-byte opcodes (EXT_FD, EXT_FE,
// EXT_FF).
func isFiller(name string) bool {
	return strings.HasPrefix(name, "RSVD_") || strings.HasPrefix(name, "EXT_")
}

// sameOperands reports whether a parsed entry's operands match a spec's.
func sameOperands(f field, s spec) bool {
	return f.count == s.count && f.scale == s.scale && f.access == s.access
}

// applyManualOperands checks and completes the parsed table against
// manualOperands (see operands.go's file comment for the rules), and
// returns it with missingInstructions appended. It stops the generator on
// any disagreement it hasn't been told about, and on any entry in
// manualOperands, manualCorrections, or missingInstructions that goes
// unused, since that means one of those lists is stale.
func applyManualOperands(fields []field) []field {
	used := make(map[string]bool, len(manualOperands))
	corrected := make(map[string]bool, len(manualCorrections))

	for i, f := range fields {
		if isFiller(f.name) {
			if f.count != 0 {
				log.Fatalf("gen: filler entry %s has operands", f.name)
			}

			continue
		}

		text, ok := manualOperands[f.name]
		if !ok {
			log.Fatalf("gen: %s is in instruction_table.h but not in manualOperands", f.name)
		}

		used[f.name] = true
		s := parseSpec(f.name, text)

		switch {
		case sameOperands(f, s) && f.typ == shortLiteralType(s):
			// The C header agrees with the manual, short-literal type
			// included.

		case f.count == 0:
			// A blank entry: eVAX never implemented the instruction, so
			// its header row has no operands. Fill it in (below).

		case manualCorrections[f.name] != "":
			// A known error in the C header; the manual wins.
			corrected[f.name] = true

		default:
			log.Fatalf("gen: %s's operands in instruction_table.h (count %d, scale %v, access %v, %s) "+
				"disagree with manualOperands %q (%s); fix one, or add it to manualCorrections",
				f.name, f.count, f.scale, f.access, f.typ, text, shortLiteralType(s))
		}

		f.count, f.scale, f.access, f.types = s.count, s.scale, s.access, s.types
		f.typ = shortLiteralType(s)
		fields[i] = f
	}

	for name := range manualCorrections {
		if !corrected[name] {
			log.Fatalf("gen: manualCorrections names %s, but its header entry already agrees (or is missing)", name)
		}
	}

	for _, m := range missingInstructions {
		if used[m.name] {
			log.Fatalf("gen: missingInstructions names %s, which instruction_table.h already has", m.name)
		}

		text, ok := manualOperands[m.name]
		if !ok {
			log.Fatalf("gen: missingInstructions names %s, which has no manualOperands entry", m.name)
		}

		used[m.name] = true
		s := parseSpec(m.name, text)
		fields = append(fields, field{
			name: m.name, ext: m.ext, opcode: m.opc,
			count: s.count, scale: s.scale, access: s.access, types: s.types,
			typ: shortLiteralType(s),
		})
	}

	var unused []string

	for name := range manualOperands {
		if !used[name] {
			unused = append(unused, name)
		}
	}

	if len(unused) > 0 {
		sort.Strings(unused)
		log.Fatalf("gen: manualOperands has instructions that are in neither instruction_table.h "+
			"nor missingInstructions: %v", unused)
	}

	return fields
}

// dataTypeList renders an entry's six data-type letters as the Go
// constants for the generated table's DataType array.
func dataTypeList(types [6]string) string {
	names := make([]string, len(types))

	for i, t := range types {
		name, ok := dataTypeName[t]
		if !ok {
			log.Fatalf("gen: unknown data type %q", t)
		}

		names[i] = name
	}

	return strings.Join(names, ", ")
}
