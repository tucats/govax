package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_procreg.c: MTPR/MFPR and the privileged-
// register access-kind table (init_reg_access/set_priv_reg). LDPCTX/SVPCTX,
// also defined in emul_procreg.c, are out of this phase's scope -- see
// docs/PHASE-07.md's open questions.

// privAccess says whether a privileged register may be read, written, or
// both via MTPR/MFPR, matching emul_procreg.c's priv_reg_access[] (values
// OP_NL/OP_RD/OP_WR/OP_MD, reusing the same accessKind constants as operand
// access -- the C source does too).
type privAccess = AccessKind

const (
	privRead   = AccessRead
	privWrite  = AccessWrite
	privModify = AccessModify
)

// privRegAccessTable is the port of init_reg_access: every unlisted register
// defaults to privNone (no access), matching the C source's initial fill.
var privRegAccessTable = buildPrivRegAccessTable()

func buildPrivRegAccessTable() [vax.MaxPrivReg + 1]privAccess {
	var t [vax.MaxPrivReg + 1]privAccess

	t[0] = privModify  // KSP
	t[1] = privModify  // ESP
	t[2] = privModify  // SSP
	t[3] = privModify  // USP
	t[4] = privModify  // ISP
	t[8] = privModify  // P0BR
	t[9] = privModify  // P0LR
	t[10] = privModify // P1BR
	t[11] = privModify // P1LR
	t[12] = privModify // SBR
	t[13] = privModify // SLR
	t[16] = privModify // PCBB
	t[17] = privModify // SCBB
	t[18] = privModify // IPL
	t[19] = privModify // ASTLVL
	t[20] = privWrite  // SIRR
	t[21] = privModify // SISR
	t[23] = privModify // MCSR (11/750)
	t[24] = privModify // ICCS
	t[25] = privWrite  // NICR
	t[26] = privRead   // ICR
	t[27] = privModify // TODR
	t[28] = privModify // CSRS
	t[29] = privModify // CSRD
	t[30] = privModify // CSTS
	t[31] = privModify // CSTD
	t[32] = privModify // RXCS
	t[33] = privRead   // RXDB
	t[34] = privModify // TXCS
	t[35] = privWrite  // TXDB
	t[36] = privWrite  // TBDR (11/750)
	t[37] = privWrite  // CADR (11/750)
	t[38] = privModify // MCESR (11/750)
	t[39] = privModify // CAER (11/750)
	t[40] = privModify // ACCS
	t[41] = privModify // SAVISP
	t[42] = privModify // SAVPC
	t[43] = privModify // SAVPSL
	t[56] = privModify // MAPEN
	t[57] = privWrite  // TBIA
	t[58] = privWrite  // TBIS
	t[61] = privModify // PMR
	t[62] = privModify // SID
	t[63] = privWrite  // TBCHK

	return t
}

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0xDA, emulMtpr) // MTPR
	reg(0xDB, emulMfpr) // MFPR
}

// emulMtpr is MTPR, port of emul_procreg.c's emul_mtpr. The register-number
// operand is truncated to a 16-bit signed value before range-checking,
// matching set_priv_reg's `short reg` parameter (emul_mtpr.c passes its own
// longword-sized operand through a `(short)` cast) -- an asymmetry with
// MFPR's own register-number operand below, which the C source checks at
// full LONGWORD width. Replicated as read in the C source.
func emulMtpr(e *Engine, d *Decoded) error {
	valueRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	regRaw, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	reg := int(int16(uint16(regRaw)))

	return setPrivReg(e, reg, uint32(valueRaw))
}

// setPrivReg stores value into privileged register reg, applying the same
// bounds/mode/access checks and register-specific side effects as
// emul_procreg.c's set_priv_reg.
//
// IPL/SIRR's pending-interrupt delivery and ICCS/RXCS/TXCS/TXDB's console/
// clock device modeling, both previously deferred (see docs/PHASE-07.md's
// open questions) for lack of an interrupt-admission mechanism, are wired up
// as of Phase 14 -- see interrupt.go's Engine.Interrupt. TBIA/TBIS are wired
// up as of Phase 21 to the real translation-buffer cache internal/vm now
// maintains -- see docs/PHASE-21.md.
func setPrivReg(e *Engine, reg int, value uint32) error {
	if reg < 0 || reg > vax.MaxPrivReg {
		return &Fault{Code: ExcReservedOp}
	}

	if e.cpu.PSL().CurMod() != vax.Kernel {
		return &Fault{Code: ExcPrivileged}
	}

	access := privRegAccessTable[reg]
	if access != privModify && access != privWrite {
		return &Fault{Code: ExcReservedOp}
	}

	switch vax.PrivReg(reg) {
	case vax.IPL:
		value &= 0x1F
		psl := e.cpu.PSL()
		psl.SetIPL(value)
		e.cpu.SetPSL(psl)
		e.cpu.SetPR(vax.IPL, value)

		// Lowering the IPL can expose a software interrupt already latched
		// in SISR at a level now above the new IPL; admit each one, highest
		// level first, matching set_priv_reg's own `for (n = 15; n > value;
		// n--)` scan.
		for n := uint32(15); n > value; n-- {
			sisr := e.cpu.PR(vax.SISR)
			if sisr&(1<<n) != 0 {
				e.cpu.SetPR(vax.SISR, sisr&^(1<<n))
				e.Interrupt(ExcSoftware1+Exception((n-1)*4), n, 0)
			}
		}

	case vax.ASTLVL:
		if value > 4 {
			return &Fault{Code: ExcReservedOp}
		}

		e.cpu.SetPR(vax.ASTLVL, value)

	case vax.SIRR:
		value &= 0x0F
		if value > e.cpu.PSL().IPL() {
			// Higher priority than the current IPL: take it now rather than
			// merely latching SISR, matching set_priv_reg's own case 20.
			e.Interrupt(ExcSoftware1+Exception((value-1)*4), value, 0)
		} else {
			e.cpu.SetPR(vax.SISR, e.cpu.PR(vax.SISR)|(1<<value))
			e.cpu.SetPR(vax.SIRR, value)
		}

	case vax.ICCS:
		// Only these bits are settable via MTPR, matching set_priv_reg's own
		// mask; RUN/DON reflect current status rather than being freely
		// writable.
		value &= 0x800000F1
		iccs := e.cpu.PR(vax.ICCS)

		if value&iccsInt != 0 { // write to ICCS<INT>: clear it
			iccs &^= iccsInt
		}

		if value&iccsRun != 0 { // write to ICCS<RUN>: set it
			iccs |= iccsRun
		} else {
			iccs &^= iccsRun
		}

		if value&iccsXFR != 0 { // write to ICCS<XFR>: reload ICR from NICR
			e.cpu.SetPR(vax.ICR, e.cpu.PR(vax.NICR))
		}

		if value&iccsSGL != 0 { // write to ICCS<SGL>: increment the clock
			e.cpu.SetPR(vax.ICR, e.cpu.PR(vax.ICR)+1)
		}

		iccs = (iccs &^ deviceIE) | (value & deviceIE) // write to ICCS<IE>
		if value&iccsErr != 0 {                        // write to ICCS<ERR>: clear it
			iccs &^= iccsErr
		}

		e.cpu.SetPR(vax.ICCS, iccs)

	case vax.RXCS:
		// Only the IE bit is writable; DON is status-only.
		value &= deviceIE
		old := e.cpu.PR(vax.RXCS)
		wasIE := old & deviceIE
		rxcs := (old & 0x80) | value
		e.cpu.SetPR(vax.RXCS, rxcs)

		// If IE was already set and a byte is (still) waiting, (re-)signal
		// the interrupt -- matching set_priv_reg's own case 32, including
		// its one-tick delay.
		if wasIE != 0 && rxcs&0xC0 != 0 {
			e.Interrupt(ExcConRead, 20, 1)
		}

	case vax.TXCS:
		// Only the IE bit is writable; RDY is always set (a real console
		// transmitter completes instantly in this emulator).
		value &= deviceIE
		if value != 0 {
			e.cpu.SetPR(vax.TXCS, 0xC0)
			// Real VAX IPL for the console terminal is 20 (0x14); the C
			// reference's own two TXCS/TXDB call sites use the literal
			// 0x20 (32 decimal) here despite this same case's own comment
			// saying "IPL 20", and despite RXCS's IE-bit case just above
			// (and vax.c's own poll_keyboard) consistently using decimal
			// 20 for the same conceptual interrupt -- an internal
			// self-contradiction, not a deliberate ISA choice, so fixed
			// directly rather than replicated; see docs/DEVIATIONS.md.
			e.Interrupt(ExcConWrite, 20, 0)
		} else {
			e.cpu.SetPR(vax.TXCS, 0x80)
		}

	case vax.TXDB:
		value &= 0x7F
		if e.services != nil {
			e.services.ConsoleWriteByte(byte(value))
		}

		txcs := e.cpu.PR(vax.TXCS)
		wasIE := txcs & 0x60 // matches set_priv_reg's own (unusual) mask
		txcs |= 0x80
		e.cpu.SetPR(vax.TXCS, txcs)

		if wasIE != 0 {
			e.Interrupt(ExcConWrite, 20, 0) // see the IPL note on case TXCS above
		}

	case vax.TBIA:
		e.mem.InvalidateTB()

	case vax.TBIS:
		e.mem.InvalidatePage(value)

	default:
		e.cpu.SetPR(vax.PrivReg(reg), value)
	}

	return nil
}

// emulMfpr is MFPR, port of emul_procreg.c's emul_mfpr.
func emulMfpr(e *Engine, d *Decoded) error {
	regRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	reg := int(int32(uint32(regRaw)))
	if reg < 0 || reg > vax.MaxPrivReg {
		return &Fault{Code: ExcReservedOp}
	}

	if e.cpu.PSL().CurMod() != vax.Kernel {
		return &Fault{Code: ExcPrivileged}
	}

	access := privRegAccessTable[reg]
	if access != privModify && access != privRead {
		return &Fault{Code: ExcReservedOp}
	}

	if vax.PrivReg(reg) == vax.RXDB {
		// Pull a real byte from the console's own input source (the same
		// one XFC$CONSOLE_READ uses) as this read's side effect, rather
		// than returning whatever stale value happens to be sitting in the
		// register -- see docs/DEVIATIONS.md's poll_keyboard finding on why
		// the C reference's own RXDB was never reliably populated on its
		// target (Linux) platform.
		if e.services != nil {
			e.cpu.SetPR(vax.RXDB, uint32(e.services.ConsoleReadByte()))
		}
		// emul_mfpr.c's own "clear DON bit" side effect (`vax.RXCS &= 0x40`
		// -- clears every bit except IE, not just DON).
		e.cpu.SetPR(vax.RXCS, e.cpu.PR(vax.RXCS)&deviceIE)
	}

	value := e.cpu.PR(vax.PrivReg(reg))

	return d.Operands[1].Store(e.cpu, e.mem, uint64(value))
}
