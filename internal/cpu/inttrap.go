package cpu

// The integer overflow trap (VAX Architecture Reference Manual, 8.4.6 and
// appendix E.1.1). An integer instruction whose result doesn't fit its
// destination stores the result's low-order bits, sets N and Z from them,
// and sets V. If the PSL's IV bit (integer overflow enable) is set too, an
// arithmetic trap of type 1 follows once the instruction has completed:
// its result stored, its condition codes set, and, for the loop
// instructions, its branch taken or not. The PC the trap saves is the next
// instruction's, so a handler that continues goes on from there. IV is set
// by a procedure's entry mask (bit 14) and by BISPSW.
//
// V means integer overflow only for the instructions listed below, so
// Step checks V and IV after those alone (Instruction.TrapsOnOverflow).
// A few other integer conversions trap on their own: CVTPL, EMOD, and the
// floating-to-integer conversions (decimal.go, emod.go, cvtfloat.go).
//
// Divide by zero isn't IV's to enable: an integer divide with a zero
// divisor always traps, type 2 (8.5.1, E.1.2). DIV and EDIV raise it
// themselves (integerDivideTrap), which takes the place of the overflow
// check.

// integerOverflowOpcodes are the single-byte opcodes whose V reports an
// integer overflow.
var integerOverflowOpcodes = []byte{
	0x80, 0x81, 0xA0, 0xA1, 0xC0, 0xC1, // ADDB2, ADDB3, ADDW2, ADDW3, ADDL2, ADDL3
	0x82, 0x83, 0xA2, 0xA3, 0xC2, 0xC3, // SUBx2, SUBx3
	0x84, 0x85, 0xA4, 0xA5, 0xC4, 0xC5, // MULx2, MULx3
	0x86, 0x87, 0xA6, 0xA7, 0xC6, 0xC7, // DIVx2, DIVx3 (the largest negative / -1)
	0xD8, 0xD9, // ADWC, SBWC
	0x96, 0xB6, 0xD6, // INCB, INCW, INCL
	0x97, 0xB7, 0xD7, // DECB, DECW, DECL
	0x8E, 0xAE, 0xCE, // MNEGB, MNEGW, MNEGL (of the largest negative)
	0x58,       // ADAWI
	0x78, 0x79, // ASHL, ASHQ
	0x33, 0xF6, 0xF7, // CVTWB, CVTLB, CVTLW (the narrowing conversions)
	0x9D, 0x3D, 0xF1, // ACBB, ACBW, ACBL
	0xF2, 0xF3, // AOBLSS, AOBLEQ
	0xF4, 0xF5, // SOBGEQ, SOBGTR
	0x7B, // EDIV (a quotient too big for a longword)
}

func init() {
	for _, fn := range integerOverflowOpcodes {
		instructionTable.Lookup(Opcode{Function: fn}).TrapsOnOverflow = true
	}
}

// integerOverflowTrap returns the integer overflow trap if the instruction
// just executed, inst, is one whose V reports an integer overflow and both
// V and IV are set; otherwise nil.
func (e *Engine) integerOverflowTrap(inst *Instruction) error {
	psl := e.cpu.PSL()
	if !psl.V() || !psl.IV() || !inst.TrapsOnOverflow {
		return nil
	}

	return e.arithmeticTrap(trapIntOvf)
}

// integerDivideTrap is the integer divide-by-zero trap, for a DIV or EDIV
// whose divisor was zero, once it has stored its results.
func (e *Engine) integerDivideTrap() error {
	return e.arithmeticTrap(trapIntDiv)
}
