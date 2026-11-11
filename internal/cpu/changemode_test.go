package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/registers"
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
	cpu.SetGPR(registers.SP, 0x7000)
	cpu.SetPR(registers.KSP, 0x7000)

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
	cpu.SetGPR(registers.PC, base)

	if err := e.Step(); err != nil { // CHMK #1: decode/fault/deliver -- lands at the handler
		t.Fatalf("Step (first CHMK, fault delivery): %v", err)
	}

	if got := cpu.GPR(registers.PC); got != handlerAddr {
		t.Fatalf("PC after first CHMK's fault delivery = %#x, want handlerAddr %#x", got, uint32(handlerAddr))
	}

	if err := e.Step(); err != nil { // MOVL (SP)+,R0 (discard the signal arg)
		t.Fatalf("Step (first MOVL): %v", err)
	}

	if err := e.Step(); err != nil { // REI
		t.Fatalf("Step (first REI): %v", err)
	}

	if got := cpu.GPR(registers.PC); got != base+2 {
		t.Fatalf("PC after first CHMK's handler returns = %#x, want %#x (the instruction after CHMK, not CHMK's own address)", got, uint32(base+2))
	}

	if err := e.Step(); err != nil { // CHMK #2: must actually execute CHMK again, not re-trap the first one
		t.Fatalf("Step (second CHMK, fault delivery): %v", err)
	}

	if got := cpu.GPR(registers.PC); got != handlerAddr {
		t.Fatalf("PC after second CHMK's fault delivery = %#x, want handlerAddr %#x (confirms CHMK #2 actually executed, not a re-trap of CHMK #1)", got, uint32(handlerAddr))
	}

	if err := e.Step(); err != nil { // MOVL (SP)+,R0 (discard the signal arg)
		t.Fatalf("Step (second MOVL): %v", err)
	}

	if err := e.Step(); err != nil { // REI
		t.Fatalf("Step (second REI): %v", err)
	}
	
	if got := cpu.GPR(registers.PC); got != base+4 {
		t.Fatalf("PC after second CHMK's handler returns = %#x, want %#x", got, uint32(base+4))
	}
}

// TestChmsFromKernelStaysInKernel: a change-mode trap goes to the more
// privileged of the instruction's mode and the current one (the
// architecture's CHM, "maximize privilege"). A CHMS from kernel mode is
// dispatched through CHMS's vector but taken in kernel mode, on the kernel
// stack, with the code, PC, and PSL pushed there.
func TestChmsFromKernelStaysInKernel(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetGPR(registers.SP, 0x7000)
	cpu.SetPR(registers.KSP, 0x7000)
	cpu.SetPR(registers.SSP, 0x6000)
	putVector(t, e, ExcChangeModeS, 0x900, 0)

	stepInstruction(t, e, 0xBE, 0x03) // CHMS #3

	if got := cpu.PSL().CurMod(); got != registers.Kernel {
		t.Errorf("mode %v after CHMS from kernel, want kernel", got)
	}

	if got := cpu.GPR(registers.SP); got != 0x7000-12 {
		t.Errorf("SP %#x, want %#x (three longwords on the kernel stack)", got, 0x7000-12)
	}

	if got := cpu.GPR(registers.PC); got != 0x900 {
		t.Errorf("PC %#x, want the CHMS vector, 0x900", got)
	}

	if code := mustLongword(t, cpu, e.mem, 0x7000-12); code != 3 {
		t.Errorf("code on the stack %d, want 3", code)
	}
}

// TestChmxPreviousModeWithinAMode: a change-mode trap that stays in the
// current mode records that mode as the previous one, as every exception
// does (PRV_MOD is the old CUR_MOD), whatever the PSL had there.
func TestChmxPreviousModeWithinAMode(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetGPR(registers.SP, 0x7000)
	cpu.SetPR(registers.KSP, 0x7000)

	psl := cpu.PSL()
	psl.SetPrvMod(registers.User) // as kernel code reached from user mode has it
	cpu.SetPSL(psl)

	putVector(t, e, ExcChangeModeK, 0x900, 0)
	stepInstruction(t, e, 0xBC, 0x01) // CHMK #1

	if got := cpu.PSL().PrvMod(); got != registers.Kernel {
		t.Errorf("previous mode %v after CHMK from kernel, want kernel", got)
	}

	if saved := registers.PSL(mustLongword(t, cpu, e.mem, 0x7000-4)); saved.PrvMod() != registers.User {
		t.Errorf("the saved PSL's previous mode %v, want user, as it was", saved.PrvMod())
	}
}

// TestChmxOnInterruptStackHalts: "illegal from I stack" (the architecture's
// CHM): the processor halts.
func TestChmxOnInterruptStackHalts(t *testing.T) {
	e := kernelEngine()
	cpu := e.cpu
	cpu.SetGPR(registers.SP, 0x7000)

	psl := cpu.PSL()
	psl.SetIS(true)
	cpu.SetPSL(psl)

	putVector(t, e, ExcChangeModeK, 0x900, 0)
	cpu.SetGPR(registers.PC, base)
	putBytes(t, cpu, e.mem, base, 0xBC, 0x01) // CHMK #1

	if err := e.Step(); !errors.Is(err, ErrHalted) {
		t.Errorf("Step = %v, want ErrHalted", err)
	}
}

// changeModeServices are fakeServices with change mode handlers ($DCLCMH).
type changeModeServices struct {
	fakeServices
	handlers map[registers.AccessMode]uint32
}

func (s *changeModeServices) ChangeModeHandler(mode registers.AccessMode) uint32 {
	return s.handlers[mode]
}

// TestChmuToDeclaredHandler: a CHMU whose SCB vector is the console's
// sentinel goes to the change mode handler the services name for user mode,
// delivered as the hardware delivers the trap: in user mode, on the user
// stack, the code on top, then the PC of the next instruction and the PSL.
// With none declared, it goes to the services' condition dispatch instead
// (no dispatcher here: the console's report).
func TestChmuToDeclaredHandler(t *testing.T) {
	e := newEngine()
	cpu := e.cpu
	services := &changeModeServices{handlers: map[registers.AccessMode]uint32{registers.User: 0xA00}}
	e.SetSystemServices(services)
	putVector(t, e, ExcChangeModeU, 0xFFFFFFFF, 0)

	psl := cpu.PSL()
	psl.SetCurMod(registers.User)
	psl.SetPrvMod(registers.User)
	cpu.SetPSL(psl)
	cpu.SetGPR(registers.SP, 0x5000)

	stepInstruction(t, e, 0xBF, 0x05) // CHMU #5

	if got := cpu.GPR(registers.PC); got != 0xA00 {
		t.Fatalf("PC %#x, want the handler, 0xA00", got)
	}

	if got := cpu.PSL().CurMod(); got != registers.User {
		t.Errorf("mode %v, want user", got)
	}

	sp := cpu.GPR(registers.SP)
	if sp != 0x5000-12 {
		t.Errorf("SP %#x, want %#x", sp, 0x5000-12)
	}

	if code, pc := mustLongword(t, cpu, e.mem, sp), mustLongword(t, cpu, e.mem, sp+4); code != 5 || pc != base+2 {
		t.Errorf("stack: code %d, PC %#x; want 5, %#x", code, pc, base+2)
	}

	delete(services.handlers, registers.User)
	cpu.SetGPR(registers.SP, 0x5000)

	cpu.SetGPR(registers.PC, base)
	if err := e.Step(); !errors.Is(err, ErrNoExceptionHandler) {
		t.Errorf("CHMU with no handler: Step = %v, want the console handler's report", err)
	}
}
