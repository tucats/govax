package disasm

import (
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/cpu"
)

// TestOperandFields checks the parts Disassemble records for each
// addressing mode. Each case is one instruction at address 0x1000, so a
// PC-relative operand's Target can be worked out by hand: the address of
// the byte after the displacement plus the displacement.
func TestOperandFields(t *testing.T) {
	const pc = 0x1000

	cases := []struct {
		name  string
		bytes []byte
		index int // which operand to check
		want  Operand
	}{
		{"short literal", []byte{0xD0, 0x05, 0x50}, 0,
			Operand{Mode: ModeLiteral, Register: -1, Index: -1, Value: 5}},
		{"register", []byte{0xD0, 0x05, 0x50}, 1,
			Operand{Mode: ModeRegister, Register: 0, Index: -1}},
		{"register deferred", []byte{0xD4, 0x63}, 0,
			Operand{Mode: ModeRegisterDeferred, Register: 3, Index: -1}},
		{"autodecrement", []byte{0xD4, 0x73}, 0,
			Operand{Mode: ModeAutodecrement, Register: 3, Index: -1}},
		{"autoincrement", []byte{0xD4, 0x83}, 0,
			Operand{Mode: ModeAutoincrement, Register: 3, Index: -1}},
		{"autoincrement deferred", []byte{0xD4, 0x93}, 0,
			Operand{Mode: ModeAutoincrementDeferred, Register: 3, Index: -1}},
		{"byte displacement, negative", []byte{0xD4, 0xAD, 0xFC}, 0,
			Operand{Mode: ModeDisplacement, Register: 13, Index: -1, Width: 1, Displacement: -4}},
		{"word displacement deferred", []byte{0xD4, 0xDC, 0x34, 0x12}, 0,
			Operand{Mode: ModeDisplacement, Deferred: true, Register: 12, Index: -1, Width: 2, Displacement: 0x1234}},
		{"immediate", []byte{0xD0, 0x8F, 0xE8, 0x03, 0x00, 0x00, 0x52}, 0,
			Operand{Mode: ModeImmediate, Register: -1, Index: -1, Width: 4, Value: 1000, Bytes: []byte{0xE8, 0x03, 0, 0}}},
		{"absolute", []byte{0xD4, 0x9F, 0x00, 0x02, 0x00, 0x00}, 0,
			Operand{Mode: ModeAbsolute, Register: -1, Index: -1, Target: 0x200, HasTarget: true}},
		// MOVL with displacement -0xE06 at 0x1000: the displacement ends at 0x1006,
		// so the target is 0x1006-0xE06 = 0x200.
		{"long relative", []byte{0xD0, 0xEF, 0xFA, 0xF1, 0xFF, 0xFF, 0x50}, 0,
			Operand{Mode: ModeRelative, Register: -1, Index: -1, Width: 4, Displacement: -0xE06, Target: 0x200, HasTarget: true}},
		{"word relative deferred", []byte{0xD0, 0xDF, 0x10, 0x00, 0x50}, 0,
			Operand{Mode: ModeRelative, Deferred: true, Register: -1, Index: -1, Width: 2, Displacement: 0x10, Target: 0x1014, HasTarget: true}},
		// MOVL L^TABLE[R4],R0: the index register is recorded, and the
		// base's fields are the operand's own.
		{"indexed relative", []byte{0xD0, 0x44, 0xEF, 0x00, 0x00, 0x00, 0x00, 0x50}, 0,
			Operand{Mode: ModeRelative, Register: -1, Index: 4, Width: 4, Target: 0x1007, HasTarget: true}},
		// BEQL .+7: the displacement ends at 0x1002.
		{"branch", []byte{0x13, 0x05}, 0,
			Operand{Mode: ModeBranch, Register: -1, Index: -1, Width: 1, Displacement: 5, Target: 0x1007, HasTarget: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := make(SliceReader, pc+len(tc.bytes))
			copy(r[pc:], tc.bytes)

			dec, err := Disassemble(r, pc)
			if err != nil {
				t.Fatal(err)
			}

			got := dec.Operands[tc.index]

			// The instruction table's fields are checked elsewhere.
			got.Access, got.Type, got.Size = 0, cpu.DataNone, 0

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("operand %d of % X:\n got %+v\nwant %+v", tc.index, tc.bytes, got, tc.want)
			}
		})
	}
}

// TestOperandSymbol checks that a Symbol is shown in place of the target
// address, with the operand's mode prefix kept.
func TestOperandSymbol(t *testing.T) {
	for _, tc := range []struct {
		bytes []byte
		want  string
	}{
		{[]byte{0xFB, 0x00, 0x9F, 0x00, 0x04, 0x00, 0x00}, "CALLS S^#0,@#START"},
		{[]byte{0xD0, 0xEF, 0x00, 0x00, 0x00, 0x00, 0x50}, "MOVL L^START,R0"},
		{[]byte{0xD0, 0xDF, 0x00, 0x00, 0x50}, "MOVL @W^START,R0"},
		{[]byte{0x11, 0x00}, "BRB START"},
	} {
		dec, err := Disassemble(SliceReader(tc.bytes), 0)
		if err != nil {
			t.Fatal(err)
		}

		for i := range dec.Operands {
			if dec.Operands[i].HasTarget {
				dec.Operands[i].Symbol = "START"
			}
		}

		if got := dec.String(); got != tc.want {
			t.Errorf("% X: got %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

// TestEntryMask checks a routine's mask word decodes as one.
func TestEntryMask(t *testing.T) {
	dec := EntryMask(SliceReader{0x1C, 0xC0}, 0, "SUB2")

	if !dec.IsMask || dec.Mask != 0xC01C || dec.Length != 2 {
		t.Fatalf("EntryMask = %+v", dec)
	}

	if got, want := dec.String(), ".ENTRY SUB2,^M<R2,R3,R4,DV,IV>"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}
