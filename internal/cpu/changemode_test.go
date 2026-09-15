package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// TestEmulChmxReturnsPastTheChmxInstruction regresses the bug documented on
// emulChmx's own doc comment: without setting e.instructionPC to the
// already-advanced current PC before faulting, Engine.raise resets PC back
// to the CHMx instruction's own start address (Step's generic,
// before-decode e.instructionPC, correct for every other synchronous fault
// but not this one), so RET/REI from the CHMK handler resumes at the CHMK
// instruction itself instead of the instruction after it -- an infinite
// re-trap loop the moment any caller issues CHMK a second time after the
// first one's handler returns (exactly LIB$PUT_ONE's own per-byte
// "chmk #EXE$PUT_CONSOLE" loop, found via Phase 14's own end-to-end test).
func TestEmulChmxReturnsPastTheChmxInstruction(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetGPR(vax.SP, 0x7000)
	cpu.SetPR(vax.KSP, 0x7000)

	// A trivial handler: CHMK pushes its one signal argument (the change-
	// mode code) below the PC/PSL frame REI expects -- discard it (MOVL
	// (SP)+,R0) before REI, matching what any real handler does before
	// returning.
	const handlerAddr = 0x900
	putVector(t, e, ExcChangeModeK, handlerAddr, 0)
	putBytes(t, cpu, e.mem, handlerAddr,
		0xD0, 0x8E, 0x50, // MOVL (SP)+,R0
		0x02, // REI
	)

	// Two back-to-back CHMK #0 instructions (2 bytes each: opcode + short-
	// literal operand), matching LIB$PUT_ONE_LOOP's own repeated
	// "chmk #EXE$PUT_CONSOLE" shape.
	const base = 0x1000
	putBytes(t, cpu, e.mem, base, 0xBC, 0x00, 0xBC, 0x00)
	cpu.SetGPR(vax.PC, base)

	if err := e.Step(); err != nil { // CHMK #1: decode/fault/deliver -- lands at the handler
		t.Fatalf("Step (first CHMK, fault delivery): %v", err)
	}
	if got := cpu.GPR(vax.PC); got != handlerAddr {
		t.Fatalf("PC after first CHMK's fault delivery = %#x, want handlerAddr %#x", got, uint32(handlerAddr))
	}
	if err := e.Step(); err != nil { // MOVL (SP)+,R0 (discard the signal arg)
		t.Fatalf("Step (first MOVL): %v", err)
	}
	if err := e.Step(); err != nil { // REI
		t.Fatalf("Step (first REI): %v", err)
	}
	if got := cpu.GPR(vax.PC); got != base+2 {
		t.Fatalf("PC after first CHMK's handler returns = %#x, want %#x (the instruction after CHMK, not CHMK's own address)", got, uint32(base+2))
	}

	if err := e.Step(); err != nil { // CHMK #2: must actually execute CHMK again, not re-trap the first one
		t.Fatalf("Step (second CHMK, fault delivery): %v", err)
	}
	if got := cpu.GPR(vax.PC); got != handlerAddr {
		t.Fatalf("PC after second CHMK's fault delivery = %#x, want handlerAddr %#x (confirms CHMK #2 actually executed, not a re-trap of CHMK #1)", got, uint32(handlerAddr))
	}
	if err := e.Step(); err != nil { // MOVL (SP)+,R0 (discard the signal arg)
		t.Fatalf("Step (second MOVL): %v", err)
	}
	if err := e.Step(); err != nil { // REI
		t.Fatalf("Step (second REI): %v", err)
	}
	if got := cpu.GPR(vax.PC); got != base+4 {
		t.Fatalf("PC after second CHMK's handler returns = %#x, want %#x", got, uint32(base+4))
	}
}
