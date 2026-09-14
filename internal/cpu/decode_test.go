package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestDecodeInstructionNoOperands(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0x00) // HALT

	d, err := decodeInstruction(cpu, mem, instructionTable)
	if err != nil {
		t.Fatalf("decodeInstruction: %v", err)
	}
	if d.Instruction.Name != "HALT" {
		t.Errorf("Instruction.Name = %q, want HALT", d.Instruction.Name)
	}
	if d.NextPC != base+1 {
		t.Errorf("NextPC = %#x, want %#x", d.NextPC, base+1)
	}
}

func TestDecodeInstructionTwoOperands(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.PC, base)
	cpu.SetGPR(vax.R1, 0x1234)
	// MOVL R1,R2 : opcode 0xD0, src=Rn(1) mode 5 reg 1, dst=Rn mode 5 reg 2
	putBytes(t, cpu, mem, base, 0xD0, 0x51, 0x52)

	d, err := decodeInstruction(cpu, mem, instructionTable)
	if err != nil {
		t.Fatalf("decodeInstruction: %v", err)
	}
	if d.Instruction.Name != "MOVL" {
		t.Fatalf("Instruction.Name = %q, want MOVL", d.Instruction.Name)
	}
	if d.Operands[0].Kind != OperandRegister || d.Operands[0].Reg != vax.R1 {
		t.Errorf("Operands[0] = %+v, want Kind=Register Reg=R1", d.Operands[0])
	}
	if d.Operands[1].Kind != OperandRegister || d.Operands[1].Reg != vax.R2 {
		t.Errorf("Operands[1] = %+v, want Kind=Register Reg=R2", d.Operands[1])
	}
	if d.NextPC != base+3 {
		t.Errorf("NextPC = %#x, want %#x", d.NextPC, base+3)
	}
}

func TestDecodeInstructionExtendedOpcode(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.PC, base)
	// BUGL #12345678 : extended opcode 0xFF,0xFD, one OP_IM longword operand
	putBytes(t, cpu, mem, base, 0xFF, 0xFD)
	putLongword(t, cpu, mem, base+2, 0x12345678)

	d, err := decodeInstruction(cpu, mem, instructionTable)
	if err != nil {
		t.Fatalf("decodeInstruction: %v", err)
	}
	if d.Instruction.Name != "BUGL" {
		t.Fatalf("Instruction.Name = %q, want BUGL", d.Instruction.Name)
	}
	if d.Opcode != (Opcode{Extended: 0xFF, Function: 0xFD}) {
		t.Errorf("Opcode = %+v, want {FF FD}", d.Opcode)
	}
	if d.Operands[0].Kind != OperandImmediate || uint32(d.Operands[0].Value) != 0x12345678 {
		t.Errorf("Operands[0] = %+v, want Kind=Immediate Value=0x12345678", d.Operands[0])
	}
	if d.NextPC != base+6 {
		t.Errorf("NextPC = %#x, want %#x", d.NextPC, base+6)
	}
}

func TestDecodeInstructionUndefinedExtendedOpcodeFaults(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.PC, base)
	putBytes(t, cpu, mem, base, 0xFD, 0x00) // 0xFD-prefixed 0x00 is undefined

	_, err := decodeInstruction(cpu, mem, instructionTable)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcPrivileged {
		t.Fatalf("err = %v, want *Fault{Code: ExcPrivileged}", err)
	}
}

// TestDecodeInstructionQuadwordRegisterPair cross-checks decode against
// reference/AUDIT.md finding C2 (quadword register-pair reconstruction):
// C2's own live-test scenario was `MOVQ R2,@addr` / `MOVQ @addr,R4`
// round-tripping R2:R3 = 12345678:ABCDEF01 through memory and back into
// R4:R5 intact. That finding was confirmed fixed by the closed C-side audit
// purely via the LONGWORD-width fix (reference/eVAX's vax.reg[] became a
// real 4-byte-per-element array again, so decode_operand.c's mode==5 fast
// path — unchanged, no scale==8 special case — correctly spans Rn/Rn+1 by
// ordinary pointer arithmetic). This port has no pointer arithmetic to rely
// on, so Operand.Load/Store (internal/cpu/operandaccess.go) implement the
// Rn/Rn+1 pairing explicitly; this test exercises that path end-to-end
// starting from a real decoded MOVQ instruction, not a hand-built Operand.
func TestDecodeInstructionQuadwordRegisterPair(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.PC, base)
	cpu.SetGPR(vax.R2, 0xABCDEF01) // low longword of the quadword R2:R3
	cpu.SetGPR(vax.R3, 0x12345678) // high longword
	// MOVQ R2,R4 : opcode 0x7D, src mode 5 reg 2, dst mode 5 reg 4.
	putBytes(t, cpu, mem, base, 0x7D, 0x52, 0x54)

	d, err := decodeInstruction(cpu, mem, instructionTable)
	if err != nil {
		t.Fatalf("decodeInstruction: %v", err)
	}
	if d.Instruction.Name != "MOVQ" {
		t.Fatalf("Instruction.Name = %q, want MOVQ", d.Instruction.Name)
	}
	if d.Operands[0].Size != 8 || d.Operands[1].Size != 8 {
		t.Fatalf("Operand sizes = %d, %d, want 8, 8", d.Operands[0].Size, d.Operands[1].Size)
	}

	v, err := d.Operands[0].Load(cpu, mem)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := uint64(0x12345678ABCDEF01); v != want {
		t.Errorf("Load() = %#x, want %#x (R3:R2 combined)", v, want)
	}

	if err := d.Operands[1].Store(cpu, mem, v); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if cpu.GPR(vax.R4) != 0xABCDEF01 {
		t.Errorf("R4 (low) = %#x, want 0xABCDEF01", cpu.GPR(vax.R4))
	}
	if cpu.GPR(vax.R5) != 0x12345678 {
		t.Errorf("R5 (high) = %#x, want 0x12345678 (would be 0 under AUDIT.md's C2 bug)", cpu.GPR(vax.R5))
	}
}

func TestDecodeInstructionOperandFaultPropagates(t *testing.T) {
	cpu, mem := fixture()
	cpu.SetGPR(vax.PC, base)
	// MOVL S^#5,S^#5 : a short literal is illegal as a write (destination)
	// operand — decode_operand.c raises a reserved-addressing-mode fault.
	putBytes(t, cpu, mem, base, 0xD0, 0x05, 0x05)

	d, err := decodeInstruction(cpu, mem, instructionTable)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcReservedAddr {
		t.Fatalf("err = %v, want *Fault{Code: ExcReservedAddr}", err)
	}
	if d.Instruction == nil || d.Instruction.Name != "MOVL" {
		t.Errorf("Instruction = %+v, want MOVL still recorded despite the fault", d.Instruction)
	}
}
