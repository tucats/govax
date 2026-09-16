package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// This file covers setPrivReg/emulMfpr's ICCS/RXCS/RXDB/TXCS/TXDB cases --
// docs/PHASE-14.md's interval-timer and console-I/O device modeling.

func TestEmulMtprIccsXfrReloadsIcrFromNicr(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetPR(vax.NICR, 0xFFFFFFF0)

	cpu.SetGPR(vax.R1, 0x10) // ICCS<XFR>
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.ICCS))...)

	if got := cpu.PR(vax.ICR); got != 0xFFFFFFF0 {
		t.Errorf("PR(ICR) = %#x, want 0xFFFFFFF0 (reloaded from NICR)", got)
	}
}

func TestEmulMtprIccsRunAndIEBits(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.R1, 0x41) // RUN | IE
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.ICCS))...)

	if got := cpu.PR(vax.ICCS); got&iccsRun == 0 || got&deviceIE == 0 {
		t.Errorf("PR(ICCS) = %#x, want RUN and IE both set", got)
	}

	cpu.SetGPR(vax.R1, 0) // clear RUN (IE bit copied as 0 too)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.ICCS))...)

	if got := cpu.PR(vax.ICCS); got&iccsRun != 0 {
		t.Errorf("PR(ICCS) = %#x, want RUN cleared", got)
	}
}

func TestEmulMtprIccsSglIncrementsClock(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetPR(vax.ICR, 5)

	cpu.SetGPR(vax.R1, 0x20) // ICCS<SGL>
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.ICCS))...)

	if got := cpu.PR(vax.ICR); got != 6 {
		t.Errorf("PR(ICR) = %d, want 6", got)
	}
}

func TestEmulMtprTxcsIEEnablesInterruptImmediately(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu

	cpu.SetGPR(vax.R1, 0x40) // TXCS<IE>
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TXCS))...)

	if got := cpu.PR(vax.TXCS); got != 0xC0 {
		t.Errorf("PR(TXCS) = %#x, want 0xC0 (RDY|IE)", got)
	}

	if !e.interruptPending {
		t.Fatal("expected enabling TXCS<IE> to admit EXC$CONWRITE immediately")
	}

	if e.interruptCode != ExcConWrite || e.interruptIPL != 20 {
		t.Errorf("interruptCode/IPL = %#x/%d, want ExcConWrite/20", e.interruptCode, e.interruptIPL)
	}
}

func TestEmulMtprTxdbWritesByteAndInterruptsWhenIESet(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	f := &fakeServices{}
	e.SetSystemServices(f)
	cpu.SetPR(vax.TXCS, 0xC0) // RDY|IE already set

	cpu.SetGPR(vax.R1, 'H')
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TXDB))...)

	if f.writtenByte != 'H' {
		t.Errorf("ConsoleWriteByte got %q, want 'H'", f.writtenByte)
	}

	if !e.interruptPending {
		t.Fatal("expected TXDB write with TXCS<IE> set to admit EXC$CONWRITE")
	}

	if e.interruptCode != ExcConWrite || e.interruptIPL != 20 {
		t.Errorf("interruptCode/IPL = %#x/%d, want ExcConWrite/20", e.interruptCode, e.interruptIPL)
	}
}

func TestEmulMtprTxdbNoInterruptWhenIEClear(t *testing.T) {
	e := kernelEngine()
	e.SetSystemServices(&fakeServices{})
	e.cpu.SetPR(vax.TXCS, 0x80) // RDY only

	e.cpu.SetGPR(vax.R1, 'x')
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TXDB))...)

	if e.interruptPending {
		t.Error("expected no interrupt when TXCS<IE> is clear")
	}

	if got := e.cpu.PR(vax.TXCS); got&0x80 == 0 {
		t.Errorf("PR(TXCS) = %#x, want RDY bit set", got)
	}
}

// TestEmulMtprTxdbWithNoServicesIsHarmless covers the register-only path
// (no Console wired up) -- existing procreg tests that predate Phase 14
// exercise MTPR/MFPR without SetSystemServices, and TXDB shouldn't panic.
func TestEmulMtprTxdbWithNoServicesIsHarmless(t *testing.T) {
	e := kernelEngine()
	e.cpu.SetGPR(vax.R1, 'x')
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.TXDB))...)
}

func TestEmulMtprRxcsReSignalsWhenIEAlreadySetAndDonPending(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetPR(vax.RXCS, 0x80|deviceIE) // DON pending, IE already on

	cpu.SetGPR(vax.R1, uint32(deviceIE)) // rewrite IE (still set)
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.RXCS))...)

	if len(e.iqueue) == 0 && !e.interruptPending {
		t.Fatal("expected re-writing RXCS with IE already set and DON pending to (re-)admit EXC$CONREAD")
	}
}

func TestEmulMtprRxcsNoSignalWhenIEWasNotAlreadySet(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetPR(vax.RXCS, 0x80) // DON pending, IE currently off

	cpu.SetGPR(vax.R1, uint32(deviceIE)) // just now enabling IE
	stepInstruction(t, e, mtprBytes(vax.R1, uint32(vax.RXCS))...)

	if e.interruptPending || len(e.iqueue) != 0 {
		t.Error("expected no admission when IE was not already set before this write")
	}

	if got := cpu.PR(vax.RXCS); got != 0x80|deviceIE {
		t.Errorf("PR(RXCS) = %#x, want DON|IE", got)
	}
}

func TestEmulMfprRxdbReadsLiveByteAndClearsDon(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	f := &fakeServices{readByte: 'Q'}
	e.SetSystemServices(f)
	cpu.SetPR(vax.RXCS, 0x80|deviceIE) // DON set, IE set
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)

	stepInstruction(t, e, mfprBytes(uint32(vax.RXDB), vax.R2)...)

	if got := cpu.GPR(vax.R2); got != 'Q' {
		t.Errorf("R2 = %q, want 'Q' (live byte from ConsoleReadByte)", got)
	}

	if got := cpu.PR(vax.RXCS); got != deviceIE {
		t.Errorf("PR(RXCS) = %#x, want IE only (DON cleared)", got)
	}
}

func TestDeliverConsoleByteAdmitsInterruptWhenIESet(t *testing.T) {
	e := interruptEngine(t)
	e.cpu.SetPR(vax.RXCS, deviceIE)

	e.DeliverConsoleByte('Z')

	if got := e.cpu.PR(vax.RXDB); got != 'Z' {
		t.Errorf("PR(RXDB) = %q, want 'Z'", got)
	}

	if got := e.cpu.PR(vax.RXCS); got&0x80 == 0 {
		t.Errorf("PR(RXCS) = %#x, want DON bit set", got)
	}

	if !e.interruptPending || e.interruptCode != ExcConRead {
		t.Errorf("expected an immediate ExcConRead admission, interruptPending=%v code=%#x", e.interruptPending, e.interruptCode)
	}
}

func TestDeliverConsoleByteNoInterruptWhenIEClear(t *testing.T) {
	e := interruptEngine(t)

	e.DeliverConsoleByte('Z')

	if e.interruptPending {
		t.Error("expected no interrupt admission when RXCS<IE> is clear")
	}
	
	if got := e.cpu.PR(vax.RXDB); got != 'Z' {
		t.Errorf("PR(RXDB) = %q, want 'Z' (byte still deposited)", got)
	}
}
