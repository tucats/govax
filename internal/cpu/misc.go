package cpu

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// This is the Go port of emul_misc.c's remaining instructions -- BUGL/BUGW,
// INDEX, BISPSW/BICPSW, PUSHR/POPR, BPT, PROBER/PROBEW, MOVPSL -- everything
// in that file not already claimed by an earlier phase (HALT/NOP in Phase 03/
// 04's internal/cpu/control.go, the six queue instructions in Phase 06's
// internal/cpu/queue.go).

// trapSubRng is TRAP_SUB_RNG (fpu.h), the second signal argument INDEX's
// subrange-check arithmetic fault carries.
const trapSubRng = 0x07

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x03, emulBpt)    // BPT
	reg(0x0A, emulIndex)  // INDEX
	reg(0x0C, emulProbe)  // PROBER
	reg(0x0D, emulProbe)  // PROBEW
	reg(0xB8, emulBitpsw) // BISPSW
	reg(0xB9, emulBitpsw) // BICPSW
	reg(0xBA, emulPopr)   // POPR
	reg(0xBB, emulPushr)  // PUSHR
	reg(0xDC, emulMovpsl) // MOVPSL

	extReg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Extended: 0xFF, Function: fn}), h)
	}
	extReg(0xFD, emulBug) // BUGL
	extReg(0xFE, emulBug) // BUGW
}

// emulBpt is BPT: unconditionally signals a breakpoint exception.
func emulBpt(e *Engine, d *Decoded) error {
	return &Fault{Code: ExcBreakpoint}
}

// emulBug is BUGL/BUGW: unconditionally signals a privileged-instruction
// fault carrying the instruction's implicit immediate code argument, port of
// emul_misc.c's emul_bug.
func emulBug(e *Engine, d *Decoded) error {
	return &Fault{Code: ExcPrivileged, Args: []uint32{uint32(d.Operands[0].Value)}}
}

// emulIndex is INDEX: computes an array-reference index, port of
// emul_misc.c's emul_index. The index arithmetic (out <- (in+subscript)*size)
// is computed before the range check, but -- matching the C source's early
// return -- only stored and only reflected in the condition codes once the
// subscript is confirmed in range; on the arithmetic fault, neither happens.
func emulIndex(e *Engine, d *Decoded) error {
	subscriptRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	lowRaw, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	highRaw, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	sizeRaw, err := d.Operands[3].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	inRaw, err := d.Operands[4].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	subscript := int32(uint32(subscriptRaw))
	low := int32(uint32(lowRaw))
	high := int32(uint32(highRaw))
	size := int32(uint32(sizeRaw))
	in := int32(uint32(inRaw))

	out := (in + subscript) * size

	if subscript < low || subscript > high {
		return &Fault{Code: ExcArithmetic, Args: []uint32{trapSubRng}}
	}

	psl := e.cpu.PSL()
	psl.SetN(out < 0)
	psl.SetZ(out == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return d.Operands[5].Store(e.cpu, e.mem, uint64(uint32(out)))
}

// emulBitpsw is BISPSW/BICPSW: sets or clears bits in the low byte of the
// PSW under user control, port of emul_misc.c's emul_bitpsw. Bits 8:15 of
// the mask operand must be zero, or a reserved-operand fault occurs.
func emulBitpsw(e *Engine, d *Decoded) error {
	maskRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	mask := uint16(maskRaw)
	if mask&0xFF00 != 0 {
		return &Fault{Code: ExcReservedOp}
	}

	longmask := uint32(mask) & 0xFF

	psl := uint32(e.cpu.PSL())
	if d.Opcode.Function == 0xB8 { // BISPSW
		psl |= longmask
	} else { // BICPSW
		psl &^= longmask
	}

	e.cpu.SetPSL(registers.PSL(psl))

	return nil
}

// emulPushr is PUSHR: pushes the registers named in mask (bits 0:14, R0-R14;
// the PC cannot be specified) onto the stack in descending register order,
// port of emul_misc.c's emul_pushr.
func emulPushr(e *Engine, d *Decoded) error {
	maskRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	mask := uint16(maskRaw)

	for n := 14; n >= 0; n-- {
		if mask&(1<<uint(n)) == 0 {
			continue
		}

		if err := push(e, e.cpu.GPR(registers.Reg(n))); err != nil {
			return err
		}
	}

	return nil
}

// emulPopr is POPR: pops the registers named in mask (bits 0:14) off the
// stack in ascending register order, port of emul_misc.c's emul_popr.
func emulPopr(e *Engine, d *Decoded) error {
	maskRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	mask := uint16(maskRaw)

	for n := 0; n <= 14; n++ {
		if mask&(1<<uint(n)) == 0 {
			continue
		}

		sp := e.cpu.GPR(registers.SP)

		v, err := e.mem.LoadLongword(e.cpu, sp)
		if err != nil {
			return err
		}

		e.cpu.SetGPR(registers.SP, sp+4)
		e.cpu.SetGPR(registers.Reg(n), v)
	}

	return nil
}

// emulProbe is PROBER/PROBEW: checks the read or write accessibility of the
// first and last byte of the base/length-described region, at the larger
// (less privileged) of the requested mode and the previous-mode PSL field,
// port of emul_misc.c's emul_probe.
//
// Matching the C source's VM_NOSIGNAL translation mode (see docs/PHASE-07.md's
// design notes), neither byte test raises a real fault regardless of why it
// failed -- an access violation and a translation-not-valid both simply mean
// "not accessible" here. The manual's own Note 4 describes a genuine
// Translation Not Valid exception as still possible in one specific
// (system-page-table-entry) case; emul_misc.c doesn't implement that
// distinction and neither does this port.
//
// Both byte tests read their page's PTE (vm.Memory.ProbeAccess) and check
// its protection field: as the architecture's PROBE, they never touch the
// page, so a page that isn't valid isn't paged in, and no fault is
// counted (docs/DEVIATIONS.md, Phase 66; testdata/pg66's case A3).
func emulProbe(e *Engine, d *Decoded) error {
	modeRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	lenRaw, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	base := d.Operands[2].Addr
	mode := int8(uint8(modeRaw))
	length := int16(uint16(lenRaw))

	savedPSL := e.cpu.PSL()
	if int(savedPSL.CurMod()) > int(mode) {
		mode = int8(savedPSL.PrvMod())
	}

	access := vm.AccessRead
	if d.Opcode.Function == 0x0D { // PROBEW
		access = vm.AccessWrite
	}

	testPSL := savedPSL
	testPSL.SetCurMod(registers.AccessMode(mode))
	e.cpu.SetPSL(testPSL)

	ok := e.mem.ProbeAccess(e.cpu, base, access)
	if ok {
		last := uint32(int32(base) + int32(length) - 1)
		ok = e.mem.ProbeAccess(e.cpu, last, access)
	}

	e.cpu.SetPSL(savedPSL)

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(!ok)
	psl.SetV(false)
	e.cpu.SetPSL(psl)
	
	return nil
}

// emulMovpsl is MOVPSL: writes the whole PSL to the destination operand,
// port of emul_misc.c's emul_movpsl. write_psl_bits (syncing a cached "wide"
// bitfield copy purely for C bitfield-access speed) has no Go equivalent to
// call -- see docs/PHASE-01.md's design notes -- since registers.PSL is already
// the single canonical representation.
func emulMovpsl(e *Engine, d *Decoded) error {
	return d.Operands[0].Store(e.cpu, e.mem, uint64(e.cpu.PSL()))
}
