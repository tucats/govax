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
	privNone   = AccessNone
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
// Several of set_priv_reg's register-specific cases have side effects this
// port doesn't model yet and are deliberately not replicated -- see
// docs/PHASE-07.md's open questions: IPL/SIRR's pending-interrupt delivery
// (both call interrupt(), the device-interrupt-queue admission routine
// Phase 03 already deferred to Phase 09) and ICCS/RXCS/TXCS/TXDB's
// console/clock device modeling (Phase 09 I/O) fall through to a plain
// register store here rather than replicating device behavior that doesn't
// exist yet. TBIA/TBIS are no-ops: Phase 02 never ported the translation-
// buffer cache (a pure 1999-era performance hack with no result-visible
// effect -- see internal/vm/translate.go's design notes), so there is
// nothing to invalidate.
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

	case vax.ASTLVL:
		if value > 4 {
			return &Fault{Code: ExcReservedOp}
		}
		e.cpu.SetPR(vax.ASTLVL, value)

	case vax.SIRR:
		value &= 0x0F
		e.cpu.SetPR(vax.SISR, e.cpu.PR(vax.SISR)|(1<<value))
		e.cpu.SetPR(vax.SIRR, value)

	case vax.TBIA, vax.TBIS:
		// No-op; see the doc comment above.

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

	// emul_mfpr.c clears RXCS's DON bit as a side effect of reading RXDB
	// (33); a console-device effect not modeled yet, see the doc comment on
	// setPrivReg.
	value := e.cpu.PR(vax.PrivReg(reg))
	return d.Operands[1].Store(e.cpu, e.mem, uint64(value))
}
