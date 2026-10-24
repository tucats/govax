package cpu

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// The trace rules (trace.go), case by case from the VAX Architecture
// Reference Manual's Table 5.3 and the rules of operation before it.

// Where these tests' handlers are. The trace handler is a lone REI, so a
// trace fault is taken and dismissed in two steps; the arithmetic and
// change mode handlers pop their one parameter, then REI.
const (
	traceVector   = 0x400
	paramVector   = 0x500
	reservedOpVec = 0x600
	tracedEnd     = 0x1F00 // a test program ends by reaching here
)

var (
	nop      = []byte{0x01}
	bispswT  = []byte{0xB8, 0x10} // BISPSW #^X10
	bicpswT  = []byte{0xB9, 0x10} // BICPSW #^X10
	addl2One = []byte{0xC0, 0x01, regMode(registers.R0)}
	chmkZero = []byte{0xBC, 0x00}
)

// jumpEnd is JMP @#tracedEnd, the end of a test program.
func jumpEnd() []byte {
	return []byte{0x17, 0x9F, tracedEnd & 0xFF, tracedEnd >> 8, 0, 0}
}

// traceHit is one trace fault: the PC and PSL its frame saved, and the
// PSL its handler started with.
type traceHit struct {
	pc, savedPSL uint32
	handlerPSL   registers.PSL
}

// newTraceEngine returns an engine in kernel mode with a stack, the trace
// handler, and the arithmetic and change-mode-to-kernel handlers, with the
// program code at base.
func newTraceEngine(t *testing.T, code ...[]byte) *Engine {
	t.Helper()

	e := newEngine()
	e.cpu.SetGPR(registers.SP, 0x7000)
	e.cpu.SetPR(registers.KSP, 0x7000)

	putVector(t, e, ExcTracePending, traceVector, 0)
	putVector(t, e, ExcArithmetic, paramVector, 0)
	putVector(t, e, ExcChangeModeK, paramVector, 0)
	putVector(t, e, ExcReservedOp, reservedOpVec, 0)
	putBytes(t, e.cpu, e.mem, traceVector, 0x02)             // REI
	putBytes(t, e.cpu, e.mem, paramVector, 0xD5, 0x8E, 0x02) // TSTL (SP)+; REI
	putBytes(t, e.cpu, e.mem, tracedEnd, 0x01)

	addr := uint32(base)
	for _, c := range code {
		putBytes(t, e.cpu, e.mem, addr, c...)
		addr += uint32(len(c))
	}

	e.cpu.SetGPR(registers.PC, base)

	return e
}

// setT sets PSL<T> directly, as an REI or a debugger's DEPOSIT would.
func setT(e *Engine) {
	psl := e.cpu.PSL()
	psl.SetT(true)
	e.cpu.SetPSL(psl)
}

// runTraced steps e until PC reaches tracedEnd with no trace pending (or
// a reserved operand fault's handler), recording each trace fault as its
// handler starts.
func runTraced(t *testing.T, e *Engine) []traceHit {
	t.Helper()

	var hits []traceHit

	for range 200 {
		pc := e.cpu.GPR(registers.PC)
		if pc == tracedEnd && !e.cpu.PSL().TP() || pc == reservedOpVec {
			return hits
		}

		if err := e.Step(); err != nil {
			t.Fatalf("Step at %#x: %v", pc, err)
		}

		if e.cpu.GPR(registers.PC) == traceVector {
			sp := e.cpu.GPR(registers.SP)
			hits = append(hits, traceHit{
				pc:         mustLongword(t, e.cpu, e.mem, sp),
				savedPSL:   mustLongword(t, e.cpu, e.mem, sp+4),
				handlerPSL: e.cpu.PSL(),
			})
		}
	}

	t.Fatal("the program didn't reach its end")

	return nil
}

// tracedPCs is the PCs of hits.
func tracedPCs(hits []traceHit) []uint32 {
	pcs := make([]uint32, len(hits))
	for i, h := range hits {
		pcs[i] = h.pc
	}

	return pcs
}

func checkPCs(t *testing.T, hits []traceHit, want ...uint32) {
	t.Helper()

	got := tracedPCs(hits)
	if len(got) != len(want) {
		t.Fatalf("traces at %#x, want %#x", got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("traces at %#x, want %#x", got, want)
		}
	}
}

// TestTraceEveryInstruction: with T set, a trace follows each instruction,
// its saved PC the next one's and its saved PSL T set and TP clear, and
// the handler runs with both clear.
func TestTraceEveryInstruction(t *testing.T) {
	e := newTraceEngine(t, nop, nop, nop, jumpEnd())
	setT(e)

	hits := runTraced(t, e)
	checkPCs(t, hits, base+1, base+2, base+3, tracedEnd)

	for _, h := range hits {
		saved := registers.PSL(h.savedPSL)
		if !saved.T() || saved.TP() {
			t.Errorf("trace at %#x: saved PSL %08X, want T set and TP clear", h.pc, h.savedPSL)
		}

		if h.handlerPSL.T() || h.handlerPSL.TP() {
			t.Errorf("trace at %#x: handler's PSL %08X, want T and TP clear", h.pc, uint32(h.handlerPSL))
		}
	}
}

// TestTraceNotWithoutT: no T, no traces.
func TestTraceNotWithoutT(t *testing.T) {
	e := newTraceEngine(t, nop, nop, jumpEnd())
	checkPCs(t, runTraced(t, e))
}

// TestTraceBISPSW: tracing begins with the second instruction after the
// BISPSW that sets T (the first trace follows the first instruction after
// it); after a BICPSW that clears T, the BICPSW itself is the last
// instruction traced.
func TestTraceBISPSW(t *testing.T) {
	e := newTraceEngine(t, bispswT, nop, nop, bicpswT, nop, jumpEnd())

	// BISPSW at base, NOPs at +2 and +3, BICPSW at +4, NOP at +6.
	checkPCs(t, runTraced(t, e), base+3, base+4, base+6)
}

// TestTraceFault: a traced instruction that faults saves its PSL with TP
// clear (the instruction runs again, and is traced then).
func TestTraceFault(t *testing.T) {
	bad := []byte{0xB8, 0x8F, 0x00, 0x01} // BISPSW #^X100: a reserved operand
	e := newTraceEngine(t, nop, bad)
	setT(e)

	checkPCs(t, runTraced(t, e), base+1)

	if pc := e.cpu.GPR(registers.PC); pc != reservedOpVec {
		t.Fatalf("PC %#x, want the reserved operand handler", pc)
	}

	sp := e.cpu.GPR(registers.SP)
	if pc := mustLongword(t, e.cpu, e.mem, sp); pc != base+1 {
		t.Errorf("fault's saved PC %#x, want the instruction's own %#x", pc, base+1)
	}

	saved := registers.PSL(mustLongword(t, e.cpu, e.mem, sp+4))
	if !saved.T() || saved.TP() {
		t.Errorf("fault's saved PSL %08X, want T set and TP clear", uint32(saved))
	}

	if e.cpu.PSL().T() || e.cpu.PSL().TP() {
		t.Errorf("fault handler's PSL %08X, want T and TP clear", uint32(e.cpu.PSL()))
	}
}

// TestTraceArithmeticTrap: a traced instruction that traps saves its PSL
// with TP set, so the trap's handler runs untraced and the trace comes
// after its REI, with the next instruction's PC.
func TestTraceArithmeticTrap(t *testing.T) {
	e := newTraceEngine(t, addl2One, jumpEnd())
	e.cpu.SetGPR(registers.R0, 0x7FFFFFFF)

	psl := e.cpu.PSL()
	psl.SetIV(true)
	psl.SetT(true)
	e.cpu.SetPSL(psl)

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}

	if pc := e.cpu.GPR(registers.PC); pc != paramVector {
		t.Fatalf("PC %#x, want the arithmetic trap's handler", pc)
	}

	saved := registers.PSL(mustLongword(t, e.cpu, e.mem, e.cpu.GPR(registers.SP)+8))
	if !saved.T() || !saved.TP() {
		t.Errorf("trap's saved PSL %08X, want T and TP set", uint32(saved))
	}

	// The handler's two instructions aren't traced; the next trace is the
	// ADDL2's, after the handler's REI.
	checkPCs(t, runTraced(t, e), base+3, tracedEnd)
}

// TestTraceCHMK: a CHMx's handler isn't traced, and the CHMx's trace comes
// after the handler's REI.
func TestTraceCHMK(t *testing.T) {
	e := newTraceEngine(t, chmkZero, nop, jumpEnd())
	setT(e)

	checkPCs(t, runTraced(t, e), base+2, base+3, tracedEnd)
}

// TestTraceCallsRet: CALLS saves T clear in the frame and leaves PSL<T>
// set, so the routine is traced; its RET restores T clear, and the RET is
// the last instruction traced. A T set in the frame's saved PSW resumes
// tracing after the instruction after the RET.
func TestTraceCallsRet(t *testing.T) {
	const routine = 0x1800

	calls := []byte{0xFB, 0x00, 0x9F, 0, 0, 0, 0} // CALLS #0,@#routine
	binary.LittleEndian.PutUint32(calls[3:], routine)

	run := func(t *testing.T, frameT bool) []traceHit {
		t.Helper()

		e := newTraceEngine(t, calls, nop, nop, jumpEnd())
		putBytes(t, e.cpu, e.mem, routine, 0x00, 0x00, 0x01, 0x04) // .WORD 0; NOP; RET
		setT(e)

		if !frameT {
			return runTraced(t, e)
		}

		// Step the CALLS (the first trace comes after the NOP in the
		// routine), then set T in the frame's saved PSW.
		if err := e.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}

		fp := e.cpu.GPR(registers.FP)
		mask := mustLongword(t, e.cpu, e.mem, fp+4)

		if mask&0x10 != 0 {
			t.Errorf("CALLS saved PSW %04X, want T clear", mask&0xFFFF)
		}

		putLongword(t, e.cpu, e.mem, fp+4, mask|0x10)

		return runTraced(t, e)
	}

	t.Run("RET ends tracing", func(t *testing.T) {
		checkPCs(t, run(t, false), routine+2, routine+3, base+7)
	})

	t.Run("T in the frame", func(t *testing.T) {
		checkPCs(t, run(t, true), routine+2, routine+3, base+7, base+8, base+9, tracedEnd)
	})
}

// TestTraceREI: the instruction after an REI is traced if T was set when
// the REI ran, or TP is set in the PSL it restores; T set in that PSL
// alone starts tracing an instruction later.
func TestTraceREI(t *testing.T) {
	const target = 0x1800

	// The REI restores PC target, in kernel mode, with T and TP as given.
	run := func(t *testing.T, tracedREI, restoredT, restoredTP bool) []uint32 {
		t.Helper()

		e := newTraceEngine(t, []byte{0x02}) // REI
		putBytes(t, e.cpu, e.mem, target, 0x01, 0x01)
		putBytes(t, e.cpu, e.mem, target+2, jumpEnd()...)

		restored := e.cpu.PSL()
		restored.SetT(restoredT)
		restored.SetTP(restoredTP)

		sp := uint32(0x6FF8)
		putLongword(t, e.cpu, e.mem, sp, target)
		putLongword(t, e.cpu, e.mem, sp+4, uint32(restored))
		e.cpu.SetGPR(registers.SP, sp)

		if tracedREI {
			setT(e)
		}

		return tracedPCs(runTraced(t, e))
	}

	cases := []struct {
		name                             string
		tracedREI, restoredT, restoredTP bool
		want                             []uint32
	}{
		{"untraced, nothing restored", false, false, false, nil},
		{"traced REI, T clear after", true, false, false, []uint32{target}},
		{"untraced REI, TP restored", false, false, true, []uint32{target}},
		{"untraced REI, T restored", false, true, false, []uint32{target + 1, target + 2, tracedEnd}},
		{"traced REI, T restored", true, true, false, []uint32{target, target + 1, target + 2, tracedEnd}},
		{"traced REI, T and TP restored", true, true, true, []uint32{target, target + 1, target + 2, tracedEnd}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, tc.tracedREI, tc.restoredT, tc.restoredTP)
			if len(got) != len(tc.want) {
				t.Fatalf("traces at %#x, want %#x", got, tc.want)
			}

			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("traces at %#x, want %#x", got, tc.want)
				}
			}
		})
	}
}

// TestTraceInterrupt: an interrupt taken between a traced instruction and
// the next saves the PSL with TP set, so its handler runs untraced and
// the trace comes after its REI, once.
func TestTraceInterrupt(t *testing.T) {
	e := newTraceEngine(t, nop, nop, jumpEnd())
	putVector(t, e, ExcConWrite, 0x700, 0)
	putBytes(t, e.cpu, e.mem, 0x700, 0x01, 0x02) // NOP; REI
	setT(e)

	if err := e.Step(); err != nil { // the first NOP; TP is now set
		t.Fatalf("Step: %v", err)
	}

	e.Interrupt(ExcConWrite, 20, 0)

	if err := e.Step(); err != nil { // the interrupt, and its NOP
		t.Fatalf("Step: %v", err)
	}

	saved := registers.PSL(mustLongword(t, e.cpu, e.mem, e.cpu.GPR(registers.SP)+4))
	if !saved.T() || !saved.TP() {
		t.Errorf("interrupt's saved PSL %08X, want T and TP set", uint32(saved))
	}

	checkPCs(t, runTraced(t, e), base+1, base+2, tracedEnd)
}

// TestTraceFaultBreakpoint: a fault breakpoint on the trace fault stops
// before delivery with the trace still pending (TP set, PC the instruction
// it comes before), and the next step delivers it without stopping again;
// the trace after that stops again. Clearing TP at the stop drops it.
func TestTraceFaultBreakpoint(t *testing.T) {
	e := newTraceEngine(t, nop, nop, nop, jumpEnd())
	e.SetFaultBreakpoint(ExcTracePending)
	setT(e)

	if err := e.Step(); err != nil { // the first NOP, TP ← T
		t.Fatalf("first NOP: %v", err)
	}

	var fb *FaultBreak

	stop := func(pc uint32) {
		t.Helper()

		if err := e.Step(); !errors.As(err, &fb) || fb.Code != ExcTracePending {
			t.Fatalf("Step = %v, want the trace fault's break", err)
		}

		if got := e.cpu.GPR(registers.PC); got != pc {
			t.Errorf("PC at the break = %#x, want %#x", got, pc)
		}

		if !e.cpu.PSL().TP() {
			t.Error("TP clear at the break, want the trace pending")
		}
	}

	stop(base + 1)

	// Delivered: the trace handler (an REI) is entered.
	if err := e.Step(); err != nil {
		t.Fatalf("delivering the held trace: %v", err)
	}

	if got := e.cpu.GPR(registers.PC); got != traceVector {
		t.Fatalf("PC after delivery = %#x, want the trace handler %#x", got, traceVector)
	}

	for range 2 { // the handler's REI, then the second NOP
		if err := e.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
	}

	stop(base + 2)

	psl := e.cpu.PSL()
	psl.SetTP(false)
	e.cpu.SetPSL(psl)

	if err := e.Step(); err != nil { // the third NOP, untraced before it
		t.Fatalf("third NOP: %v", err)
	}

	if got := e.cpu.GPR(registers.PC); got != base+3 {
		t.Errorf("PC after the dropped trace = %#x, want %#x", got, base+3)
	}
}
