package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// fakeServices is a test double for SystemServices, recording its inputs and
// returning canned outputs.
type fakeServices struct {
	writtenByte byte
	readByte    byte

	consoleCmd string
	consoleRC  uint32

	dclR1, dclR2, dclR3 uint32
	dclPresentRC        uint32
	dclKeywordRC        uint32
	dclIntegerRC        uint32
	dclStringAddr       uint32
	dclStringOK         bool

	servicePC      uint32
	serviceRC      uint32
	serviceHandled bool
	serviceErr     error

	shimCode    uint32
	shimRC      uint32
	shimHandled bool
	shimErr     error

	quitRequested bool
}

func (f *fakeServices) ConsoleWriteByte(b byte) { f.writtenByte = b }
func (f *fakeServices) ConsoleReadByte() byte   { return f.readByte }

func (f *fakeServices) ConsoleCommand(cmd string) uint32 {
	f.consoleCmd = cmd

	return f.consoleRC
}

func (f *fakeServices) DCLPresent(r1, r2 uint32) uint32 {
	f.dclR1, f.dclR2 = r1, r2

	return f.dclPresentRC
}

func (f *fakeServices) DCLGetKeyword(r1, r2, r3 uint32) uint32 {
	f.dclR1, f.dclR2, f.dclR3 = r1, r2, r3

	return f.dclKeywordRC
}

func (f *fakeServices) DCLGetString(r1, r2 uint32) (uint32, bool) {
	f.dclR1, f.dclR2 = r1, r2

	return f.dclStringAddr, f.dclStringOK
}

func (f *fakeServices) DCLGetInteger(r1, r2 uint32) uint32 {
	f.dclR1, f.dclR2 = r1, r2

	return f.dclIntegerRC
}

func (f *fakeServices) SystemService(pc uint32) (uint32, bool, error) {
	f.servicePC = pc

	return f.serviceRC, f.serviceHandled, f.serviceErr
}

func (f *fakeServices) Shim(code uint32) (uint32, bool, error) {
	f.shimCode = code

	return f.shimRC, f.shimHandled, f.shimErr
}

func (f *fakeServices) RequestQuit() { f.quitRequested = true }

// xfcEngine returns an Engine with fakeServices installed as its hooks.
func xfcEngine() (*Engine, *fakeServices) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	f := &fakeServices{}
	e.SetSystemServices(f)

	return e, f
}

func TestEmulXfcConsoleWrite(t *testing.T) {
	e, f := xfcEngine()
	e.cpu.SetGPR(vax.R0, 0xFFFFFF41) // 'A', with garbage in the high bytes
	stepInstruction(t, e, 0xFC, xfcConsoleWrite)

	if f.writtenByte != 'A' {
		t.Errorf("writtenByte = %#x, want 'A'", f.writtenByte)
	}
}

func TestEmulXfcConsoleRead(t *testing.T) {
	e, f := xfcEngine()
	f.readByte = 'Z'

	e.cpu.SetGPR(vax.R0, 0xAABBCC11)
	stepInstruction(t, e, 0xFC, xfcConsoleRead)
	
	if got := e.cpu.GPR(vax.R0); got != 0xAABBCC5A {
		t.Errorf("R0 = %#x, want 0xaabbcc5a (only low byte replaced)", got)
	}
}

func TestEmulXfcConsoleCmd(t *testing.T) {
	e, f := xfcEngine()
	f.consoleRC = 42

	cmdAddr := uint32(0x2000)
	cmd := "SHOW VERSION"

	if err := e.mem.StoreWord(e.cpu, cmdAddr, uint16(len(cmd))); err != nil {
		t.Fatal(err)
	}

	if err := e.mem.Store(e.cpu, cmdAddr+2, []byte(cmd)); err != nil {
		t.Fatal(err)
	}

	e.cpu.SetGPR(vax.R0, cmdAddr)
	stepInstruction(t, e, 0xFC, xfcConsoleCmd)

	if f.consoleCmd != cmd {
		t.Errorf("consoleCmd = %q, want %q", f.consoleCmd, cmd)
	}

	if got := e.cpu.GPR(vax.R0); got != 42 {
		t.Errorf("R0 = %d, want 42", got)
	}
}

func TestEmulXfcConsoleCmdEmptyLengthIgnored(t *testing.T) {
	e, f := xfcEngine()
	cmdAddr := uint32(0x2000)

	if err := e.mem.StoreWord(e.cpu, cmdAddr, 0); err != nil {
		t.Fatal(err)
	}

	e.cpu.SetGPR(vax.R0, cmdAddr)
	stepInstruction(t, e, 0xFC, xfcConsoleCmd)

	if f.consoleCmd != "" {
		t.Errorf("consoleCmd = %q, want dispatch skipped for zero length", f.consoleCmd)
	}
}

func TestEmulXfcQuitEmulator(t *testing.T) {
	e, f := xfcEngine()
	psl := e.cpu.PSL()
	psl.SetCurMod(vax.Kernel)
	e.cpu.SetPSL(psl)

	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, 0xFC, xfcQuitEmulator)

	if err := e.Step(); !errors.Is(err, ErrHalted) {
		t.Errorf("Step() = %v, want ErrHalted", err)
	}

	if !f.quitRequested {
		t.Error("RequestQuit was not called, want the console asked to stop entirely")
	}
}

// TestEmulXfcQuitEmulatorFaultsOutsideKernelMode calls emulXfc directly,
// rather than through Engine.Step, for the same reason as
// TestEmulMtprRequiresKernelMode (procreg_test.go): raising the fault from
// User mode would drive HandleFault's User->Kernel mode switch, which sets
// MAPEN = 1 as a side effect (docs/DEVIATIONS.md), needing a page table this
// test doesn't otherwise care about.
func TestEmulXfcQuitEmulatorFaultsOutsideKernelMode(t *testing.T) {
	var f *Fault

	cpu, mem := fixture()
	psl := cpu.PSL()
	psl.SetCurMod(vax.User)
	cpu.SetPSL(psl)

	d := &Decoded{Operands: [6]Operand{
		{Kind: OperandImmediate, Value: xfcQuitEmulator, Size: 1},
	}}

	err := emulXfc(NewEngine(cpu, mem), d)
	if !errors.As(err, &f) {
		t.Fatalf("emulXfc err = %v, want *Fault", err)
	}

	if f.Code != ExcPrivileged {
		t.Errorf("fault code = %#x, want ExcPrivileged", f.Code)
	}
}

func TestEmulXfcHaltAndHaltSilent(t *testing.T) {
	for _, code := range []byte{xfcHaltSilent, xfcHalt} {
		e, f := xfcEngine()

		e.cpu.SetGPR(vax.PC, base)
		putBytes(t, e.cpu, e.mem, base, 0xFC, code)

		if err := e.Step(); !errors.Is(err, ErrHalted) {
			t.Errorf("code %#x: Step() = %v, want ErrHalted", code, err)
		}

		if f.quitRequested {
			t.Errorf("code %#x: RequestQuit was called, want only XFC$QUIT_EMULATION to stop the console", code)
		}
	}
}

func TestEmulXfcDCL(t *testing.T) {
	e, f := xfcEngine()
	e.cpu.SetGPR(vax.R1, 0x11)
	e.cpu.SetGPR(vax.R2, 0x22)
	e.cpu.SetGPR(vax.R3, 0x33)

	f.dclPresentRC = 1

	e.cpu.SetGPR(vax.R0, dclPresent)
	stepInstruction(t, e, 0xFC, xfcDCL)

	if got := e.cpu.GPR(vax.R0); got != 1 {
		t.Errorf("DCLPresent: R0 = %d, want 1", got)
	}

	if f.dclR1 != 0x11 || f.dclR2 != 0x22 {
		t.Errorf("DCLPresent args = (%#x,%#x), want (0x11,0x22)", f.dclR1, f.dclR2)
	}

	f.dclKeywordRC = 7

	e.cpu.SetGPR(vax.R0, dclGetKeyword)
	stepInstruction(t, e, 0xFC, xfcDCL)

	if got := e.cpu.GPR(vax.R0); got != 7 {
		t.Errorf("DCLGetKeyword: R0 = %d, want 7", got)
	}

	if f.dclR3 != 0x33 {
		t.Errorf("DCLGetKeyword r3 = %#x, want 0x33", f.dclR3)
	}

	f.dclIntegerRC = 99

	e.cpu.SetGPR(vax.R0, dclGetInteger)
	stepInstruction(t, e, 0xFC, xfcDCL)

	if got := e.cpu.GPR(vax.R0); got != 99 {
		t.Errorf("DCLGetInteger: R0 = %d, want 99", got)
	}
}

func TestEmulXfcDCLGetString(t *testing.T) {
	e, f := xfcEngine()

	f.dclStringOK = false

	e.cpu.SetGPR(vax.R0, dclGetString)
	stepInstruction(t, e, 0xFC, xfcDCL)

	if got := e.cpu.GPR(vax.R0); got != 0 {
		t.Errorf("no buffer available: R0 = %#x, want 0", got)
	}

	f.dclStringOK = true
	f.dclStringAddr = 0x4000

	e.cpu.SetGPR(vax.R0, dclGetString)
	stepInstruction(t, e, 0xFC, xfcDCL)

	if got := e.cpu.GPR(vax.R0); got != 0x4000 {
		t.Errorf("R0 = %#x, want 0x4000", got)
	}
}

func TestEmulXfcDCLUnknownSubfunctionFaults(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)

	e.SetSystemServices(&fakeServices{})
	putVector(t, e, ExcReservedOp, 0x300, 0)

	e.cpu.SetGPR(vax.R0, 99) // no such DCL subfunction
	stepInstruction(t, e, 0xFC, xfcDCL)

	if got := e.cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulXfcP1Vector(t *testing.T) {
	e, f := xfcEngine()
	f.serviceHandled = true
	f.serviceRC = 0x12345678

	// Model a real CALLS-reached P1-vector entry (p1_vector.c's p1_init): a
	// 2-byte zero procedure-entry mask at the vector's own address,
	// immediately followed by the 2-byte XFC instruction -- CALLS's own
	// frame-build logic is what actually reads and skips a mask during a
	// full dispatch (buildCallFrame), so this test starts PC just past it,
	// at the XFC itself, since it only exercises XFC's own handler.
	putBytes(t, e.cpu, e.mem, base, 0x00, 0x00)
	e.cpu.SetGPR(vax.PC, base+2)
	putBytes(t, e.cpu, e.mem, base+2, 0xFC, xfcP1Vector)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if f.servicePC != base {
		t.Errorf("SystemService called with pc=%#x, want %#x (the CALLS target -- the mask word's own address)", f.servicePC, base)
	}

	if got := e.cpu.GPR(vax.R0); got != 0x12345678 {
		t.Errorf("R0 = %#x, want 0x12345678", got)
	}
}

func TestEmulXfcP1VectorUnhandledFaults(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	
	f := &fakeServices{serviceHandled: false}
	e.SetSystemServices(f)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	stepInstruction(t, e, 0xFC, xfcP1Vector)

	if got := e.cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulXfcP1VectorPropagatesError(t *testing.T) {
	e, f := xfcEngine()
	wantErr := errors.New("boom")
	f.serviceErr = wantErr

	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, 0xFC, xfcP1Vector)

	if err := e.Step(); !errors.Is(err, wantErr) {
		t.Errorf("Step() = %v, want %v", err, wantErr)
	}
}

// TestEmulXfcP1VectorSetsR0EvenWhenHandledCallErrors matches call_service's
// own "vax.R0 = rc" happening unconditionally right after the native handler
// returns, before the caller's fetch loop next checks vax.halted -- a
// handled call that also requests a halt (e.g. an unrecognized SYS$CLI
// request) still leaves its status in R0.
func TestEmulXfcP1VectorSetsR0EvenWhenHandledCallErrors(t *testing.T) {
	e, f := xfcEngine()
	wantErr := errors.New("halt requested")
	f.serviceHandled = true
	f.serviceRC = 0xDEAD
	f.serviceErr = wantErr

	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base, 0xFC, xfcP1Vector)

	if err := e.Step(); !errors.Is(err, wantErr) {
		t.Errorf("Step() = %v, want %v", err, wantErr)
	}

	if got := e.cpu.GPR(vax.R0); got != 0xDEAD {
		t.Errorf("R0 = %#x, want 0xdead (set before the error propagated)", got)
	}
}

func TestEmulXfcShim(t *testing.T) {
	e, f := xfcEngine()
	f.shimHandled = true
	f.shimRC = 0xAB

	e.cpu.SetGPR(vax.R0, 14) // decc$gets, arbitrary code
	stepInstruction(t, e, 0xFC, xfcShim)

	if f.shimCode != 14 {
		t.Errorf("Shim called with code=%d, want 14", f.shimCode)
	}

	if got := e.cpu.GPR(vax.R0); got != 0xAB {
		t.Errorf("R0 = %#x, want 0xab", got)
	}
}

func TestEmulXfcShimUnhandledFaults(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	e.SetSystemServices(&fakeServices{shimHandled: false})
	putVector(t, e, ExcReservedOp, 0x300, 0)

	e.cpu.SetGPR(vax.R0, 250) // no such shim code
	stepInstruction(t, e, 0xFC, xfcShim)

	if got := e.cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulXfcVMProbeTranslationFailureSetsV(t *testing.T) {
	e, _ := xfcEngine()
	e.cpu.SetPR(vax.MAPEN, 0) // starting state to verify restoration below
	e.cpu.SetGPR(vax.R0, 0x1000)

	stepInstruction(t, e, 0xFC, xfcVMR)

	if !e.cpu.PSL().V() {
		t.Error("PSL<V> not set, want set for a failed translation (no page tables configured)")
	}

	if got := e.cpu.GPR(vax.R0); got != 0x1000 {
		t.Errorf("R0 = %#x, want unchanged (0x1000)", got)
	}

	if got := e.cpu.PR(vax.MAPEN); got != 0 {
		t.Errorf("MAPEN = %d, want restored to 0", got)
	}
}

func TestEmulXfcVMProbeWriteRestoresMapen(t *testing.T) {
	e, _ := xfcEngine()
	e.cpu.SetPR(vax.MAPEN, 0)
	// A zero-length P0 region (P0LR=0) makes page 8 (address 0x1000) out of
	// range, so forcing MAPEN=1 for the probe fails cleanly without needing
	// a real page table -- this only checks MAPEN is restored either way,
	// for the XFC$VMW selector specifically (XFC$VMR is covered above).
	e.cpu.SetGPR(vax.R0, 0x1000)

	stepInstruction(t, e, 0xFC, xfcVMW)

	if got := e.cpu.PR(vax.MAPEN); got != 0 {
		t.Errorf("MAPEN = %d, want restored to 0", got)
	}
}

func TestEmulXfcUnknownSelectorFaults(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcReservedOp, 0x300, 0)

	stepInstruction(t, e, 0xFC, 0x55) // no such XFC selector

	if got := e.cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (reserved-operand fault vector)", got)
	}
}

func TestEmulXfcFaultsWithoutServicesInstalled(t *testing.T) {
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	putVector(t, e, ExcPrivileged, 0x300, 0)

	stepInstruction(t, e, 0xFC, xfcConsoleWrite)

	if got := e.cpu.GPR(vax.PC); got != 0x300 {
		t.Errorf("PC = %#x, want 0x300 (privileged-instruction fault vector, no services installed)", got)
	}
}
