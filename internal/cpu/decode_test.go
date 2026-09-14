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
