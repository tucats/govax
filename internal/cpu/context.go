package cpu

import (
	"encoding/binary"
	"errors"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// This file is the VAX's process structure (Phase 43): the hardware process
// control block and the two instructions that save a process's registers
// into one and load them back out, SVPCTX and LDPCTX.
//
// A *process* is a program running with its own registers and its own
// address space (its own P0 and P1 page tables). One CPU runs many
// processes by taking turns: to switch, the operating system's scheduler
// saves the running process's registers somewhere, picks another process,
// and loads that one's registers. The VAX defines where "somewhere" is --
// a 96-byte block of memory per process, the *hardware process control
// block* (PCB) -- and gives the scheduler two instructions for the job.
// The PCBB privileged register holds the physical address of the current
// process's PCB.
//
// Everything here follows the VAX Architecture Reference Manual (EY-3459E,
// 1987), chapter 6, "Process Structure": figure 6.1 and table 6.1 for the
// layout, and the LDPCTX and SVPCTX pages' operation text, which the code
// below follows step by step. Where the manual says an outcome is
// UNDEFINED, its note 2 names the preferred implementation, a reserved-
// operand fault, and that is what govax does.
//
// govax keeps all four per-process stack pointers (KSP, ESP, SSP, USP) in
// privileged registers, as the manual's "internal registers for stack
// pointers" processors do. One detail of govax's register file matters
// throughout: while the CPU runs in some mode, that mode's stack pointer
// is live in SP (R14), and its privileged-register copy is stale until the
// mode changes (see handlefault.go's setModeStack, and REI in call.go).

// Offsets of the hardware PCB's fields, in bytes from its start (the
// manual's table 6.1).
const (
	PCBKSP    = 0x00 // kernel stack pointer
	PCBESP    = 0x04 // executive stack pointer
	PCBSSP    = 0x08 // supervisor stack pointer
	PCBUSP    = 0x0C // user stack pointer
	PCBR0     = 0x10 // R0; R1-R11, AP (R12), and FP (R13) follow, 4 bytes apart
	PCBAP     = 0x40 // argument pointer (R12)
	PCBFP     = 0x44 // frame pointer (R13)
	PCBPC     = 0x48 // program counter
	PCBPSL    = 0x4C // processor status longword
	PCBP0BR   = 0x50 // P0 base register
	PCBP0LRAS = 0x54 // P0 length register in bits 21:0, ASTLVL in bits 26:24
	PCBP1BR   = 0x58 // P1 base register
	PCBP1LRPM = 0x5C // P1 length register in bits 21:0, PME in bit 31

	// PCBSize is the hardware PCB's length: 24 longwords.
	PCBSize = 0x60
)

// The packed PCB longwords' fields (figure 6.1). "MBZ" is the manual's
// "must be zero": a PCB with a one there is UNDEFINED to LDPCTX.
const (
	pcbLengthMask  = 0x003FFFFF // P0LR/P1LR: bits 21:0
	pcbASTLVLShift = 24         // ASTLVL: bits 26:24
	pcbASTLVLMask  = 0x7
	pcbP0LRMBZ     = 0xF8C00000 // bits 31:27 and 23:22
	pcbPME         = 0x80000000 // bit 31
	pcbP1LRMBZ     = 0x7FC00000 // bits 30:22
)

// PCB is a hardware process control block's contents, unpacked: the
// register state of one process that isn't running (or, for the running
// process, its state as of the last SVPCTX, plus the memory-management
// values software keeps there).
type PCB struct {
	// SP holds the four per-process stack pointers, indexed by access
	// mode: KSP, ESP, SSP, USP (vax.Kernel..vax.User).
	SP [4]uint32

	// R holds R0-R13: R0-R11, then AP (R12) and FP (R13). SP (R14) is
	// the current mode's entry in SP above, and PC (R15) is PC below.
	R [14]uint32

	PC  uint32
	PSL vax.PSL

	// The memory-management registers that describe the process's
	// address space, its AST level, and its performance-monitor enable.
	// SVPCTX doesn't save these (the manual's SVPCTX note 1: they change
	// rarely, so software writes them into the PCB itself when it changes
	// them), but LDPCTX loads them.
	P0BR, P0LR uint32
	ASTLVL     uint32
	P1BR, P1LR uint32
	PME        bool
}

// pcbWords is a PCB as it is laid out in memory: 24 little-endian
// longwords.
type pcbWords [PCBSize / 4]uint32

// words packs p into its memory layout.
func (p *PCB) words() pcbWords {
	var w pcbWords

	copy(w[PCBKSP/4:], p.SP[:])
	copy(w[PCBR0/4:], p.R[:])
	w[PCBPC/4] = p.PC
	w[PCBPSL/4] = uint32(p.PSL)
	w[PCBP0BR/4] = p.P0BR
	w[PCBP0LRAS/4] = p.P0LR&pcbLengthMask | (p.ASTLVL&pcbASTLVLMask)<<pcbASTLVLShift
	w[PCBP1BR/4] = p.P1BR

	w[PCBP1LRPM/4] = p.P1LR & pcbLengthMask
	if p.PME {
		w[PCBP1LRPM/4] |= pcbPME
	}

	return w
}

// pcbFromWords unpacks a PCB's memory layout. Bits the layout has no room
// for (the MBZ fields) are dropped; check reports them.
func pcbFromWords(w pcbWords) PCB {
	var p PCB

	copy(p.SP[:], w[PCBKSP/4:])
	copy(p.R[:], w[PCBR0/4:])
	p.PC = w[PCBPC/4]
	p.PSL = vax.PSL(w[PCBPSL/4])
	p.P0BR = w[PCBP0BR/4]
	p.P0LR = w[PCBP0LRAS/4] & pcbLengthMask
	p.ASTLVL = w[PCBP0LRAS/4] >> pcbASTLVLShift & pcbASTLVLMask
	p.P1BR = w[PCBP1BR/4]
	p.P1LR = w[PCBP1LRPM/4] & pcbLengthMask
	p.PME = w[PCBP1LRPM/4]&pcbPME != 0

	return p
}

// check reports whether LDPCTX may load w, following the UNDEFINED tests in
// the manual's LDPCTX operation:
//
//   - P0BR must be a longword-aligned system-space (S0) address: the P0
//     page table lives in S0, which is the only region every process maps.
//     Bits 31:30 of an S0 address are binary 10.
//   - P1BR must be one too, after adding 2**23. P1's page table describes
//     the *top* of P1, so P1BR points 2**21 page-table entries (2**23
//     bytes) before the table's end, and only that end has to be in S0.
//   - The MBZ bits of the two packed longwords are zero, and ASTLVL is at
//     most 4 (4 means "no AST pending"; 5-7 are reserved).
func (w *pcbWords) check() error {
	inS0 := func(addr uint32) bool { return addr>>30 == 2 && addr&3 == 0 }

	switch {
	case !inS0(w[PCBP0BR/4]),
		w[PCBP0LRAS/4]&pcbP0LRMBZ != 0,
		w[PCBP0LRAS/4]>>pcbASTLVLShift&pcbASTLVLMask > 4,
		!inS0(w[PCBP1BR/4] + 1<<23),
		w[PCBP1LRPM/4]&pcbP1LRMBZ != 0:
		return &Fault{Code: ExcReservedOp}
	}

	return nil
}

// readPCBWords reads the PCB at physical address addr. The PCB is always
// reached by its physical address, whatever MAPEN says (the manual's
// LDPCTX note: "The PCB is located by the physical address in PCBB").
func readPCBWords(mem *vm.Memory, addr uint32) (pcbWords, error) {
	var (
		buf [PCBSize]byte
		w   pcbWords
	)

	if err := mem.LoadPhysical(addr, buf[:]); err != nil {
		return w, err
	}

	for i := range w {
		w[i] = binary.LittleEndian.Uint32(buf[i*4:])
	}

	return w, nil
}

// writePCBWords writes w's first n bytes (a multiple of 4) to the PCB at
// physical address addr.
func writePCBWords(mem *vm.Memory, addr uint32, w *pcbWords, n int) error {
	var buf [PCBSize]byte

	for i := 0; i < n/4; i++ {
		binary.LittleEndian.PutUint32(buf[i*4:], w[i])
	}

	return mem.StorePhysical(addr, buf[:n])
}

// ReadPCB returns the hardware PCB at physical address addr, as it is in
// memory (MBZ bits are ignored, not checked): for the console's and the
// scheduler's use.
func ReadPCB(mem *vm.Memory, addr uint32) (PCB, error) {
	w, err := readPCBWords(mem, addr)
	if err != nil {
		return PCB{}, err
	}

	return pcbFromWords(w), nil
}

// WritePCB writes all of p into the hardware PCB at physical address addr,
// the memory-management fields included: how software sets up a new
// process's PCB before its first LDPCTX.
func WritePCB(mem *vm.Memory, addr uint32, p *PCB) error {
	w := p.words()

	return writePCBWords(mem, addr, &w, PCBSize)
}

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x06, emulLdpctx) // LDPCTX
	reg(0x07, emulSvpctx) // SVPCTX
}

// emulSvpctx is SVPCTX, save process context. It is meant to run at the
// start of a scheduler entered by an interrupt, so the interrupted
// process's PC and PSL are the top two longwords of the stack: SVPCTX pops
// them into the PCB with the general registers and stack pointers. If it
// ran on the kernel stack, it then moves to the interrupt stack, which no
// process owns, so the scheduler has a stack that stays put while it
// changes processes underneath it.
func emulSvpctx(e *Engine, _ *Decoded) error {
	if e.cpu.PSL().CurMod() != vax.Kernel {
		return &Fault{Code: ExcPrivileged}
	}

	// Pop PC and PSL first, so that if the stack can't be read nothing has
	// been changed yet, and the access-violation fault restarts cleanly.
	sp := e.cpu.GPR(vax.SP)

	pc, err := e.mem.LoadLongword(e.cpu, sp)
	if err != nil {
		return err
	}

	psl, err := e.mem.LoadLongword(e.cpu, sp+4)
	if err != nil {
		return err
	}

	sp += 8

	p := e.registerContext()
	p.PC = pc
	p.PSL = vax.PSL(psl)

	cur := e.cpu.PSL()
	if !cur.IS() {
		// On the kernel stack (SVPCTX is privileged, so the mode is
		// kernel): SP, past the popped PC and PSL, is the kernel stack
		// pointer to save.
		p.SP[vax.Kernel] = sp
	}

	w := p.words()
	if err := writePCBWords(e.mem, e.cpu.PR(vax.PCBB), &w, PCBP0BR); err != nil {
		return err
	}

	if cur.IS() {
		e.cpu.SetGPR(vax.SP, sp)

		return nil
	}

	// Move to the interrupt stack. IPL is raised to at least 1, since
	// code on the interrupt stack must never be at IPL 0 (where a process
	// would run).
	e.cpu.SetPR(vax.KSP, sp)
	cur.SetIPL(max(1, cur.IPL()))
	cur.SetIS(true)
	e.cpu.SetPSL(cur)
	e.cpu.SetPR(vax.IPL, cur.IPL())
	e.cpu.SetGPR(vax.SP, e.cpu.PR(vax.ISP))

	return nil
}

// registerContext returns the CPU's general registers and per-process
// stack pointers as a PCB (with no PC, PSL, or memory-management fields).
// The stack pointers are the privileged registers' copies; the caller
// replaces the current mode's, which is stale (see this file's header).
func (e *Engine) registerContext() PCB {
	var p PCB

	for m := vax.Kernel; m <= vax.User; m++ {
		p.SP[m] = e.cpu.PR(vax.PrivReg(m))
	}

	for r := vax.R0; r <= vax.FP; r++ {
		p.R[r] = e.cpu.GPR(r)
	}

	return p
}

// emulLdpctx is LDPCTX, load process context: the second half of a
// process switch. Run on the interrupt stack once the scheduler has put
// the chosen process's PCB address in PCBB, it loads that process's
// registers and address space, moves to its kernel stack, and pushes its
// saved PSL and PC there, so the REI that must follow (the manual's note
// 3) resumes the process where it left off.
func emulLdpctx(e *Engine, _ *Decoded) error {
	psl := e.cpu.PSL()
	if psl.CurMod() != vax.Kernel {
		return &Fault{Code: ExcPrivileged}
	}

	// Off the interrupt stack, LDPCTX would overwrite the kernel stack
	// pointer of the code that's running it: UNDEFINED.
	if !psl.IS() {
		return &Fault{Code: ExcReservedOp}
	}

	p, err := e.readLoadablePCB()
	if err != nil {
		return err
	}

	// Keep the registers as they were, so that a kernel stack LDPCTX can't
	// push on (also UNDEFINED) can leave the machine as it found it and
	// fault.
	saved := *e.cpu

	e.loadContext(&p)

	sp := e.cpu.GPR(vax.SP) - 8

	err = e.mem.StoreLongword(e.cpu, sp+4, uint32(p.PSL))
	if err == nil {
		err = e.mem.StoreLongword(e.cpu, sp, p.PC)
	}

	if err != nil {
		*e.cpu = saved
		e.mem.InvalidateProcessTB()

		return &Fault{Code: ExcReservedOp}
	}

	e.cpu.SetGPR(vax.SP, sp)

	return nil
}

// readLoadablePCB reads the PCB that PCBB points at, checking it as
// LDPCTX does.
func (e *Engine) readLoadablePCB() (PCB, error) {
	w, err := readPCBWords(e.mem, e.cpu.PR(vax.PCBB))
	if err != nil {
		return PCB{}, err
	}

	if err := w.check(); err != nil {
		return PCB{}, err
	}

	return pcbFromWords(w), nil
}

// loadContext is LDPCTX's loading, from "invalidate per-process
// translation buffer entries" through the switch to the new kernel stack:
// it loads p's registers, address space, ASTLVL, and PME, saves the
// interrupt stack pointer, and leaves the CPU on p's kernel stack (still
// in kernel mode, at the same IPL). The CPU must be in kernel mode on the
// interrupt stack.
func (e *Engine) loadContext(p *PCB) {
	// The translation buffer caches page-table lookups. Those for P0 and
	// P1 addresses came from the old process's page tables and would be
	// wrong for the new one, so they must go; S0's stay, since every
	// process shares S0.
	e.mem.InvalidateProcessTB()

	for m := vax.Kernel; m <= vax.User; m++ {
		e.cpu.SetPR(vax.PrivReg(m), p.SP[m])
	}

	for r := vax.R0; r <= vax.FP; r++ {
		e.cpu.SetGPR(r, p.R[r])
	}

	e.cpu.SetPR(vax.P0BR, p.P0BR)
	e.cpu.SetPR(vax.P0LR, p.P0LR)
	e.cpu.SetPR(vax.ASTLVL, p.ASTLVL)
	e.cpu.SetPR(vax.P1BR, p.P1BR)
	e.cpu.SetPR(vax.P1LR, p.P1LR)

	pme := uint32(0)
	if p.PME {
		pme = 1
	}

	e.cpu.SetPR(vax.PME, pme)

	// Leave the interrupt stack for the new process's kernel stack.
	e.cpu.SetPR(vax.ISP, e.cpu.GPR(vax.SP))

	psl := e.cpu.PSL()
	psl.SetIS(false)
	e.cpu.SetPSL(psl)
	e.cpu.SetGPR(vax.SP, p.SP[vax.Kernel])
}

// ErrContextOnInterruptStack is SaveContext's error when the CPU is on the
// interrupt stack: interrupt-stack code belongs to no process, so there
// is no process to save.
var ErrContextOnInterruptStack = errors.New("cpu: cannot save a process context from the interrupt stack")

// SaveContext saves the running process's context into the PCB that PCBB
// points at, between two instructions, for a scheduler written in Go
// (Phase 44). It has the effect of the rescheduling interrupt and SVPCTX
// in the manual's RESCHED example: the PCB gets the process's general
// registers, its stack pointers, and the PC and PSL it will resume at
// (the next instruction's), and the CPU is left in kernel mode on the
// interrupt stack at IPL 3 (or higher, if it was higher), ready for
// LoadContext. As with SVPCTX, the PCB's memory-management fields aren't
// written.
func (e *Engine) SaveContext() error {
	psl := e.cpu.PSL()
	if psl.IS() {
		return ErrContextOnInterruptStack
	}

	p := e.registerContext()
	p.SP[psl.CurMod()] = e.cpu.GPR(vax.SP)
	p.PC = e.cpu.GPR(vax.PC)
	p.PSL = psl

	w := p.words()
	if err := writePCBWords(e.mem, e.cpu.PR(vax.PCBB), &w, PCBP0BR); err != nil {
		return err
	}

	// An interrupt's new PSL: kernel mode (current and previous), on the
	// interrupt stack, at the interrupt's IPL; every other bit clear.
	e.cpu.SetPR(vax.PrivReg(psl.CurMod()), e.cpu.GPR(vax.SP))

	var next vax.PSL

	next.SetCurMod(vax.Kernel)
	next.SetPrvMod(vax.Kernel)
	next.SetIS(true)
	next.SetIPL(max(3, psl.IPL()))
	e.cpu.SetPSL(next)
	e.cpu.SetPR(vax.IPL, next.IPL())
	e.cpu.SetGPR(vax.SP, e.cpu.PR(vax.ISP))

	if psl.CurMod() != vax.Kernel {
		e.mem.InvalidateProtection()
	}

	return nil
}

// LoadContext loads the process whose PCB PCBB points at and resumes it,
// between two instructions, for a scheduler written in Go: the effect of
// LDPCTX and the REI after it. It checks the PCB as LDPCTX does, and like
// LDPCTX it needs the CPU in kernel mode on the interrupt stack (as
// SaveContext leaves it), returning a reserved-operand or privileged-
// instruction *Fault otherwise, with nothing changed.
func (e *Engine) LoadContext() error {
	psl := e.cpu.PSL()

	switch {
	case psl.CurMod() != vax.Kernel:
		return &Fault{Code: ExcPrivileged}
	case !psl.IS():
		return &Fault{Code: ExcReservedOp}
	}

	p, err := e.readLoadablePCB()
	if err != nil {
		return err
	}

	e.loadContext(&p)

	// What REI does with the PC and PSL LDPCTX pushes: the kernel stack
	// pointer goes back to its register, and the process resumes in its
	// own mode, on that mode's stack.
	e.cpu.SetPR(vax.KSP, e.cpu.GPR(vax.SP))
	e.cpu.SetPSL(p.PSL)
	e.cpu.SetPR(vax.IPL, p.PSL.IPL())

	if p.PSL.IS() {
		e.cpu.SetGPR(vax.SP, e.cpu.PR(vax.ISP))
	} else {
		e.cpu.SetGPR(vax.SP, e.cpu.PR(vax.PrivReg(p.PSL.CurMod())))
	}

	e.cpu.SetGPR(vax.PC, p.PC)

	if p.PSL.CurMod() != vax.Kernel {
		e.mem.InvalidateProtection()
	}

	return nil
}
