package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// TestFaultRestoresOperandRegisters: an instruction that faults hasn't
// happened, so its autoincrement and autodecrement are undone, and PC is at
// the instruction: MOVF of a reserved operand through each mode, and a register
// used by two operands. A trap's instruction has finished, and keeps them.
func TestFaultRestoresOperandRegisters(t *testing.T) {
	const (
		code = 0x1000
		data = 0x2000
	)

	cases := []struct {
		name string
		text []byte
	}{
		{"autoincrement", []byte{0x50, 0x82, 0x54}},          // MOVF (R2)+,R4
		{"autodecrement", []byte{0x50, 0x72, 0x54}},          // MOVF -(R2),R4
		{"autoincrement deferred", []byte{0x50, 0x93, 0x54}}, // MOVF @(R3)+,R4
		{"twice", []byte{0x41, 0x82, 0x82, 0x54}},            // ADDF3 (R2)+,(R2)+,R4
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := limitsEngine(t)
			putMem(t, e, code, c.text...)
			putMem(t, e, data-4, 0x00, 0x80, 0, 0, 0x00, 0x80, 0, 0, 0x00, 0x80, 0, 0)
			putMem(t, e, data+0x100, byte(data&0xFF), byte(data>>8), 0, 0)

			e.cpu.SetGPR(registers.PC, code)
			e.cpu.SetGPR(registers.R2, data)
			e.cpu.SetGPR(registers.R3, data+0x100)

			dec := &e.decoded[0]
			if err := decodeInstruction(e.cpu, e.mem, e.table, dec); err != nil {
				t.Fatal(err)
			}

			handler := e.table.HandlerFor(dec.Instruction)

			err := handler(e, dec)
			if err == nil {
				t.Fatal("no fault")
			}

			e.undoOperands(err)

			if got := e.cpu.GPR(registers.R2); got != data {
				t.Errorf("R2 = %#x, want %#x", got, data)
			}

			if got := e.cpu.GPR(registers.R3); got != data+0x100 {
				t.Errorf("R3 = %#x, want %#x", got, data+0x100)
			}
		})
	}

	t.Run("a trap keeps them", func(t *testing.T) {
		e := limitsEngine(t)
		e.cpu.SetGPR(registers.R2, data)
		e.cpu.NoteOperandRegister(registers.R2)
		e.cpu.SetGPR(registers.R2, data+4)

		e.undoOperands(&Fault{Code: ExcArithmetic, Trap: true})

		if got := e.cpu.GPR(registers.R2); got != data+4 {
			t.Errorf("R2 = %#x, want %#x", got, data+4)
		}
	})
}
