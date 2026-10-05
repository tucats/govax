package asm

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/disasm"
)

// TestRoundTripEveryInstruction encodes every instruction in the CPU's
// table with simple operands, disassembles it, reassembles the text, and
// checks the bytes come back the same. It covers each instruction Phase 35
// added or gave operands to (G and H floating, octaword, packed decimal),
// which the assembler and disassembler both take from the same table.
//
// The operands are register-mode where a register is allowed: R0, R1,
// R2, ... in turn (byte 0x50+n, mode 5 in the high nibble and the register
// number in the low). An address or bit-field operand can't be a register
// (that's a reserved addressing mode for an address), so it is register
// deferred, "(Rn)" (0x60+n), instead. A branch displacement is zero, so the
// target is the next instruction.
func TestRoundTripEveryInstruction(t *testing.T) {
	for _, inst := range cpu.Instructions().All() {
		if skipRoundTrip(inst) {
			continue
		}

		t.Run(inst.Name, func(t *testing.T) {
			var want []byte

			if inst.Opcode.Extended != 0 {
				want = append(want, inst.Opcode.Extended)
			}

			want = append(want, inst.Opcode.Function)

			for i := 0; i < inst.OperandCount; i++ {
				switch inst.Access[i] {
				case cpu.AccessAddress, cpu.AccessVarField:
					want = append(want, 0x60+byte(i))
				case cpu.AccessBranch:
					want = append(want, make([]byte, inst.Scale[i])...)
				default:
					want = append(want, 0x50+byte(i))
				}
			}

			dec, err := disasm.Disassemble(disasm.SliceReader(want), 0)
			if err != nil {
				t.Fatalf("disasm.Disassemble(% X): %v", want, err)
			}

			if dec.Mnemonic != inst.Name {
				t.Fatalf("disasm.Disassemble(% X) found %s", want, dec.Mnemonic)
			}

			// The disassembly was made at address 0, so a branch's target
			// is an absolute address computed from 0; reassemble there too.
			got := assembleBytesAt(t, 0, dec.String())
			requireBytes(t, got, want...)
		})
	}
}

// skipRoundTrip names the table entries this test can't encode the simple
// way: the reserved and prefix placeholders (not instructions), CASEB/W/L
// (a table of branch displacements follows their operands), and XFC, BUGW,
// and BUGL (their data after the opcode isn't an operand specifier).
func skipRoundTrip(inst *cpu.Instruction) bool {
	if strings.HasPrefix(inst.Name, "RSVD_") || strings.HasPrefix(inst.Name, "EXT_") {
		return true
	}

	switch inst.Name {
	case "CASEB", "CASEW", "CASEL", "XFC", "BUGW", "BUGL":
		return true
	}

	return false
}

// TestFloatingAliases checks the alternative names for the instructions
// the floating types share with the integer ones of the same size: each
// assembles to the same bytes as the instruction it names.
func TestFloatingAliases(t *testing.T) {
	cases := []struct{ alias, real string }{
		{"CLRD R0", "CLRQ R0"},
		{"CLRG R0", "CLRQ R0"},
		{"CLRH R0", "CLRO R0"},
		{"MOVAD (R1),R0", "MOVAQ (R1),R0"},
		{"MOVAG (R1),R0", "MOVAQ (R1),R0"},
		{"MOVAH (R1),R0", "MOVAO (R1),R0"},
		{"PUSHAD (R1)", "PUSHAQ (R1)"},
		{"PUSHAG (R1)", "PUSHAQ (R1)"},
		{"PUSHAH (R1)", "PUSHAO (R1)"},
	}

	for _, c := range cases {
		t.Run(c.alias, func(t *testing.T) {
			want := assembleBytes(t, c.real)
			requireBytes(t, assembleBytes(t, c.alias), want...)
		})
	}

	// The octaword forms are two-byte opcodes: 0xFD, then 0x7C for CLRO.
	requireBytes(t, assembleBytes(t, "CLRH R0"), 0xFD, 0x7C, 0x50)
}
