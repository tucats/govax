package cpu

// registerNames are the sixteen general registers' names as VAX MACRO
// writes them: R0 to R11, then AP (R12, the argument pointer), FP (R13,
// the frame pointer), SP (R14, the stack pointer), and PC (R15, the
// program counter).
var registerNames = [16]string{
	"R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7",
	"R8", "R9", "R10", "R11", "AP", "FP", "SP", "PC",
}

// RegisterName returns general register n's name ("R5", "AP", "PC"),
// for the assembler's messages and the disassembler's operands. Only the
// low four bits of n are used, as in an operand specifier's register
// field.
func RegisterName(n int) string {
	return registerNames[n&0x0F]
}
