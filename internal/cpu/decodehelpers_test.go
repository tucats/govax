package cpu

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// decodeInstructionValue is decodeInstruction for tests: it decodes into a
// fresh Decoded and returns it, with the error, the way decodeInstruction
// worked before it filled in its caller's Decoded (docs/PERFORMANCE.md,
// Study 1, R2). On an error the Decoded holds what was decoded so far.
func decodeInstructionValue(cpu *vax.CPU, mem *vm.Memory, table *Table) (Decoded, error) {
	var d Decoded

	err := decodeInstruction(cpu, mem, table, &d)

	return d, err
}

// decodeOperandValue is decodeOperand for tests, returning the Operand
// rather than filling one in.
func decodeOperandValue(cpu *vax.CPU, mem *vm.Memory, pc *uint32, access AccessKind, size int, dtype DataType, indexed bool) (Operand, error) {
	var op Operand

	err := decodeOperand(cpu, mem, pc, access, size, dtype, indexed, &op)

	return op, err
}
