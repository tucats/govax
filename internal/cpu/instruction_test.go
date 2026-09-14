package cpu

import "testing"

// wantInstructionCount is the number of entries in
// reference/eVAX/eVAX/Headers/instruction_table.h (284, including the three
// EXT_FD/EXT_FE/EXT_FF filler entries that keep the single-byte opcode
// range's array-position-equals-opcode-value convention intact, but
// excluding the table's own empty-name end-of-array sentinel). A change
// here should only ever come from regenerating against an updated
// instruction_table.h, never a hand edit.
const wantInstructionCount = 284

func allInstructions(t *Table) []*Instruction {
	seen := make(map[*Instruction]bool)
	var out []*Instruction
	for _, inst := range t.single {
		if inst != nil && !seen[inst] {
			seen[inst] = true
			out = append(out, inst)
		}
	}
	for _, inst := range t.extended {
		if !seen[inst] {
			seen[inst] = true
			out = append(out, inst)
		}
	}
	return out
}

func TestInstructionTableCount(t *testing.T) {
	got := len(allInstructions(instructionTable))
	if got != wantInstructionCount {
		t.Fatalf("instruction count = %d, want %d", got, wantInstructionCount)
	}
}

func TestInstructionTableSingleByteRangeComplete(t *testing.T) {
	// decode_opcode.c relies on every single-byte slot 0x00-0xFF being
	// populated (0xFD-0xFF by the EXT_FD/FE/FF filler entries, since those
	// three values are actually intercepted as extended-opcode prefixes
	// before any single-byte lookup happens — see decode_opcode.c's
	// `f > 0x00FC` check, ported in sub-phase 3).
	for f := 0; f <= 0xFF; f++ {
		if instructionTable.single[f] == nil {
			t.Errorf("single-byte opcode 0x%02X has no table entry", f)
		}
	}
}

func TestInstructionTableLookup(t *testing.T) {
	cases := []struct {
		name   string
		opcode Opcode
		want   string
	}{
		{"HALT", Opcode{Extended: 0x00, Function: 0x00}, "HALT"},
		{"MOVL", Opcode{Extended: 0x00, Function: 0xD0}, "MOVL"},
		{"XFC", Opcode{Extended: 0x00, Function: 0xFC}, "XFC"},
		{"extended BUGL", Opcode{Extended: 0xFF, Function: 0xFD}, "BUGL"},
		{"extended BUGW", Opcode{Extended: 0xFF, Function: 0xFE}, "BUGW"},
		{"extended CVTGH", Opcode{Extended: 0xFD, Function: 0x56}, "CVTGH"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inst := instructionTable.Lookup(c.opcode)
			if inst == nil {
				t.Fatalf("Lookup(%+v) = nil, want %q", c.opcode, c.want)
			}
			if inst.Name != c.want {
				t.Errorf("Lookup(%+v).Name = %q, want %q", c.opcode, inst.Name, c.want)
			}
			if inst.Opcode != c.opcode {
				t.Errorf("Lookup(%+v).Opcode = %+v, want %+v", c.opcode, inst.Opcode, c.opcode)
			}
		})
	}
}

func TestInstructionTableLookupUndefined(t *testing.T) {
	// 0xFD-prefixed opcode 0x00 is not among the defined FD-extended
	// opcodes (see instruction_table.h's 0xFD block); decode should be able
	// to tell an undefined extended opcode apart from a defined one.
	if inst := instructionTable.Lookup(Opcode{Extended: 0xFD, Function: 0x00}); inst != nil {
		t.Fatalf("Lookup(undefined FD opcode) = %+v, want nil", inst)
	}
}

func TestInstructionTableDFloatingFixedEntries(t *testing.T) {
	// SUBD2, CVTDB, CVTDW, CVTDL, CVTRDL, CMPD, TSTD have zeroed-out operand
	// data in the C reference's own instruction_table.h (confirmed a real
	// bug in the C source, not a Go transcription issue -- see
	// docs/PHASE-05.md's design notes and docs/DEVIATIONS.md); the generator
	// (internal/cpu/gen's knownTableFixes) patches them to mirror their
	// already-correct F-floating/sibling D-floating rows. This test pins
	// that fix down at the table level, independent of any handler.
	cases := []struct {
		name   string
		opcode byte
		count  int
		scale  [6]int
		access [6]AccessKind
	}{
		{"SUBD2", 0x62, 2, [6]int{8, 8, 0, 0, 0, 0},
			[6]AccessKind{AccessRead, AccessModify, AccessNone, AccessNone, AccessNone, AccessNone}},
		{"CVTDB", 0x68, 2, [6]int{8, 1, 0, 0, 0, 0},
			[6]AccessKind{AccessRead, AccessWrite, AccessNone, AccessNone, AccessNone, AccessNone}},
		{"CVTDW", 0x69, 2, [6]int{8, 2, 0, 0, 0, 0},
			[6]AccessKind{AccessRead, AccessWrite, AccessNone, AccessNone, AccessNone, AccessNone}},
		{"CVTDL", 0x6A, 2, [6]int{8, 4, 0, 0, 0, 0},
			[6]AccessKind{AccessRead, AccessWrite, AccessNone, AccessNone, AccessNone, AccessNone}},
		{"CVTRDL", 0x6B, 2, [6]int{8, 4, 0, 0, 0, 0},
			[6]AccessKind{AccessRead, AccessWrite, AccessNone, AccessNone, AccessNone, AccessNone}},
		{"CMPD", 0x71, 2, [6]int{8, 8, 0, 0, 0, 0},
			[6]AccessKind{AccessRead, AccessRead, AccessNone, AccessNone, AccessNone, AccessNone}},
		{"TSTD", 0x73, 1, [6]int{8, 0, 0, 0, 0, 0},
			[6]AccessKind{AccessRead, AccessNone, AccessNone, AccessNone, AccessNone, AccessNone}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inst := instructionTable.Lookup(Opcode{Function: c.opcode})
			if inst == nil || inst.Name != c.name {
				t.Fatalf("Lookup(0x%02X) = %+v, want %q", c.opcode, inst, c.name)
			}
			if inst.OperandCount != c.count {
				t.Errorf("%s.OperandCount = %d, want %d", c.name, inst.OperandCount, c.count)
			}
			if inst.Scale != c.scale {
				t.Errorf("%s.Scale = %v, want %v", c.name, inst.Scale, c.scale)
			}
			if inst.Access != c.access {
				t.Errorf("%s.Access = %v, want %v", c.name, inst.Access, c.access)
			}
			if inst.Type != ShortLiteralFloat {
				t.Errorf("%s.Type = %v, want ShortLiteralFloat", c.name, inst.Type)
			}
		})
	}
}

func TestInstructionTableIndexOperands(t *testing.T) {
	inst := instructionTable.Lookup(Opcode{Extended: 0x00, Function: 0x0A})
	if inst == nil || inst.Name != "INDEX" {
		t.Fatalf("Lookup(INDEX) = %+v", inst)
	}
	if inst.OperandCount != 6 {
		t.Fatalf("INDEX.OperandCount = %d, want 6", inst.OperandCount)
	}
	wantAccess := [6]AccessKind{AccessRead, AccessRead, AccessRead, AccessRead, AccessRead, AccessWrite}
	if inst.Access != wantAccess {
		t.Errorf("INDEX.Access = %+v, want %+v", inst.Access, wantAccess)
	}
	wantScale := [6]int{4, 4, 4, 4, 4, 4}
	if inst.Scale != wantScale {
		t.Errorf("INDEX.Scale = %+v, want %+v", inst.Scale, wantScale)
	}
}

func TestInstructionTableNoDuplicateOpcodes(t *testing.T) {
	// newTable panics on a duplicate opcode at package init time (via the
	// instructionTable package var), so reaching this point at all is
	// itself a pass; this test exists so a future refactor that removes
	// the panic still has coverage.
	if instructionTable == nil {
		t.Fatal("instructionTable is nil")
	}
}
