package vax

// Reg indexes the general register file. R0-R15 are architecturally defined;
// R16 and up are reusable temporaries (see arch.h's T0-T5 mnemonics), carried
// over from the C source's oversized reg[] array so later phases that burn
// through temporaries during operand decode have the same room to work with.
type Reg int

// General register indices, matching arch.h's VAXREG/R*/PC/SP/FP/AP mnemonics.
// PC, SP, FP and AP are aliases for R15, R14, R13 and R12 respectively — same
// storage slot, not a separate copy.
const (
	R0 Reg = iota
	R1
	R2
	R3
	R4
	R5
	R6
	R7
	R8
	R9
	R10
	R11
	R12
	R13
	R14
	R15

	AP = R12
	FP = R13
	SP = R14
	PC = R15
)

// MaxReg is the highest valid Reg index, matching the C source's MAXREG (63):
// 16 architected registers plus 48 reusable temporaries.
const MaxReg = 63

// PrivReg indexes the privileged register file, addressed by number (as the
// MTPR/MFPR instructions will do starting in Phase 07).
type PrivReg int

// Privileged register indices for the architected subset, matching arch.h's
// preg[] mnemonics. Indices with no architected mnemonic are still valid
// PrivReg values (0..MaxPrivReg) but have no named constant here.
const (
	KSP    PrivReg = 0
	ESP    PrivReg = 1
	SSP    PrivReg = 2
	USP    PrivReg = 3
	ISP    PrivReg = 4
	P0BR   PrivReg = 8
	P0LR   PrivReg = 9
	P1BR   PrivReg = 10
	P1LR   PrivReg = 11
	SBR    PrivReg = 12
	SLR    PrivReg = 13
	PCBB   PrivReg = 16
	SCBB   PrivReg = 17
	IPL    PrivReg = 18
	ASTLVL PrivReg = 19
	SIRR   PrivReg = 20
	SISR   PrivReg = 21
	ICCS   PrivReg = 24
	NICR   PrivReg = 25
	ICR    PrivReg = 26
	TODR   PrivReg = 27
	RXCS   PrivReg = 32
	RXDB   PrivReg = 33
	TXCS   PrivReg = 34
	TXDB   PrivReg = 35
	TBDR   PrivReg = 36
	MAPEN  PrivReg = 56
	TBIA   PrivReg = 57
	TBIS   PrivReg = 58
	PMR    PrivReg = 61
	SID    PrivReg = 62
	TBCHK  PrivReg = 63
)

// MaxPrivReg is the highest valid PrivReg index, matching the C source's
// MAXPRIVREG (128).
const MaxPrivReg = 128

// CPU holds a VAX processor's register state: the general register file, the
// privileged register file, and the processor status longword. It is created
// with New and passed explicitly / receiver-bound — see docs/PLAN.md's
// locked-in state model.
type CPU struct {
	gpr [MaxReg + 1]uint32
	pr  [MaxPrivReg + 1]uint32
	psl PSL
}

// New returns a CPU with all registers and the PSL zeroed.
func New() *CPU {
	return &CPU{}
}

// Reset zeroes all registers and the PSL, equivalent to a freshly constructed CPU.
func (c *CPU) Reset() {
	*c = CPU{}
}

// GPR reads a general register.
func (c *CPU) GPR(r Reg) uint32 {
	return c.gpr[r]
}

// SetGPR writes a general register.
func (c *CPU) SetGPR(r Reg, v uint32) {
	c.gpr[r] = v
}

// PR reads a privileged register.
func (c *CPU) PR(r PrivReg) uint32 {
	return c.pr[r]
}

// SetPR writes a privileged register.
func (c *CPU) SetPR(r PrivReg, v uint32) {
	c.pr[r] = v
}
