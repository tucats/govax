package vax

import "io"

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
	KSP    PrivReg = 0  // KERNEL mode stack pointer value
	ESP    PrivReg = 1  // EXEC mode stack pointer value
	SSP    PrivReg = 2  // SUPERVISOR mode stack pointer value
	USP    PrivReg = 3  // USER mode stack pointer value
	ISP    PrivReg = 4  // INTERRUPT mode stack pointer value
	P0BR   PrivReg = 8  // P0 base address register
	P0LR   PrivReg = 9  // P0 length register
	P1BR   PrivReg = 10 // P1 base address register
	P1LR   PrivReg = 11 // P1 length register
	SBR    PrivReg = 12 // System base register
	SLR    PrivReg = 13 // System length register
	PCBB   PrivReg = 16 // Process Control Block base address
	SCBB   PrivReg = 17 // System Control Block base address
	IPL    PrivReg = 18 // Interrupt processing level
	ASTLVL PrivReg = 19 // Asynchronous System Trap (AST) level
	SIRR   PrivReg = 20 // Software Interrupt Request Register
	SISR   PrivReg = 21 // Software Interrupt Summary Register
	ICCS   PrivReg = 24 // Interval Clock Control and Status
	NICR   PrivReg = 25 // Next Interval Count Register
	ICR    PrivReg = 26 // Interval Count Register
	TODR   PrivReg = 27 // Time-of-Year Clock Register
	RXCS   PrivReg = 32 // Console Receive Control and Status
	RXDB   PrivReg = 33 // Console Receive Data Buffer
	TXCS   PrivReg = 34 // Console Transmit Control and Status
	TXDB   PrivReg = 35 // Console Transmit Data Buffer
	TBDR   PrivReg = 36 // Trnaslation Buffer Group Disable Register
	MAPEN  PrivReg = 56 // Virtual memory mapping enabled
	TBIA   PrivReg = 57 // Translation Buffer Invalidate All
	TBIS   PrivReg = 58 // Translation Buffer Invalidate Single
	PMR    PrivReg = 61 // Performance Monitoring Register
	SID    PrivReg = 62 // System Identification REgister
	TBCHK  PrivReg = 63 // Translation Buffer Check
)

// MaxPrivReg is the highest valid PrivReg index, matching the C source's
// MAXPRIVREG (128).
const MaxPrivReg = 128

// CPU holds a VAX processor's register state: the general register file, the
// privileged register file, and the processor status longword. It is created
// with New and passed explicitly / receiver-bound — see docs/PLAN.md's
// locked-in state model.
//
// debug/debugOut (see debug.go) are the C source's `vax.debug` and its
// implicit "trace to the same stream as console output" destination —
// carried on CPU, not a separate type, since CPU is already threaded (by
// value or by parameter) into every layer (internal/cpu, internal/vm,
// internal/rtl) that needs to check a DBG_* flag. See docs/PHASE-17.md.
type CPU struct {
	gpr [MaxReg + 1]uint32
	pr  [MaxPrivReg + 1]uint32
	psl PSL

	debug    DebugFlags
	debugOut io.Writer
}

// New returns a CPU with all registers and the PSL zeroed, and the debug
// flags set to DebugDefault, matching initialization.c's alloc_vax.
func New() *CPU {
	return &CPU{debug: DebugDefault}
}

// Reset zeroes all registers and the PSL, equivalent to a freshly constructed CPU.
func (c *CPU) Reset() {
	*c = CPU{debug: DebugDefault}
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
