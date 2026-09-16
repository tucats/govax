package cpu

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// interruptEngine returns a fresh Engine (via NewEngine, so it has the
// default quantum) with an SCB set up so a delivered interrupt has
// somewhere to go.
func interruptEngine(t *testing.T) *Engine {
	t.Helper()
	e := newEngine()
	e.cpu.SetGPR(vax.SP, 0x7000)
	e.cpu.SetPR(vax.KSP, 0x7000)
	return e
}

func TestInterruptImmediateDeliveryWhenUnmasked(t *testing.T) {
	e := interruptEngine(t)
	putVector(t, e, ExcConWrite, 0x400, 0)
	putBytes(t, e.cpu, e.mem, 0x400, 0x01) // NOP: Step delivers *and* decodes in the same call (matches vax.c's own fall-through), so the vector target needs a real, harmless instruction
	e.cpu.SetGPR(vax.PC, 0x1000)

	e.Interrupt(ExcConWrite, 20, 0)
	if !e.interruptPending {
		t.Fatal("expected Interrupt to admit immediately (unmasked, nothing pending)")
	}

	if err := e.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	// Step both delivers the interrupt (redirecting PC to the vector) and
	// decodes/executes the ISR's first instruction in the same call
	// (matching vax.c's own fall-through) -- so by the time Step returns,
	// PC has already advanced past that one-byte NOP.
	if got := e.cpu.GPR(vax.PC); got != 0x401 {
		t.Errorf("PC after delivery+one NOP = %#x, want 0x401", got)
	}
	if got := e.cpu.PSL().IPL(); got != 20 {
		t.Errorf("PSL.IPL() = %d, want 20", got)
	}
}

// TestInterruptDeliverySavesCurrentPCNotStale exercises the fixed-vs-C
// behavior documented on deliverPendingInterrupt: the interrupt fires from
// setPrivReg mid-instruction (as a real TXDB write would), and the PC
// pushed onto the stack for the ISR's REI to return to must be the *next*
// instruction's address, not the just-executed instruction's own start
// address.
func TestInterruptDeliverySavesCurrentPCNotStale(t *testing.T) {
	e := interruptEngine(t)
	putVector(t, e, ExcConWrite, 0x400, 0)

	// A one-byte instruction at 0x1000 that admits an interrupt as a side
	// effect (standing in for MTPR TXDB) followed by a second instruction
	// at 0x1001.
	inst := &Instruction{Name: "TESTINT", Opcode: Opcode{Function: 0x01}}
	e.table = newTable([]*Instruction{inst})
	handlerCalls := 0

	e.table.SetHandler(inst, func(eng *Engine, d *Decoded) error {
		handlerCalls++
		if handlerCalls == 1 {
			eng.Interrupt(ExcConWrite, 20, 0)
		}
		// The second call happens when Step decodes/executes the ISR's own
		// first instruction (reusing the same opcode byte at 0x400, purely
		// to keep this test's table minimal) -- by then the interrupt is
		// already masked at IPL 20, so this is a harmless no-op admission.
		return nil
	})

	putBytes(t, e.cpu, e.mem, 0x1000, 0x01)
	putBytes(t, e.cpu, e.mem, 0x400, 0x01)
	e.cpu.SetGPR(vax.PC, 0x1000)

	if err := e.Step(); err != nil { // executes the side-effecting instruction
		t.Fatalf("Step (side-effecting instruction): %v", err)
	}

	if err := e.Step(); err != nil { // delivers the interrupt, admitted above
		t.Fatalf("Step (interrupt delivery): %v", err)
	}

	if got := e.cpu.GPR(vax.PC); got != 0x401 {
		t.Fatalf("PC after delivery+one instruction = %#x, want 0x401", got)
	}

	savedPC, err := e.mem.LoadLongword(e.cpu, e.cpu.GPR(vax.SP))
	if err != nil {
		t.Fatalf("LoadLongword(saved PC): %v", err)
	}

	if savedPC != 0x1001 {
		t.Errorf("saved return PC = %#x, want 0x1001 (the next instruction, not the stale 0x1000)", savedPC)
	}
}

func TestInterruptDebugInterruptsTrace(t *testing.T) {
	e := interruptEngine(t)

	var buf bytes.Buffer

	e.cpu.SetDebugWriter(&buf)
	e.cpu.SetDebug(vax.DebugInterrupts)

	e.Interrupt(ExcConWrite, 20, 0) // unmasked: delivered immediately
	if !strings.Contains(buf.String(), "DEBUG(INTERRUPTS): set interrupt") {
		t.Errorf("output = %q, want an immediate-delivery trace line", buf.String())
	}

	buf.Reset()
	e.interruptPending = false // clear the delivery above so the next Interrupt call is masked by IPL, not by it
	e.SetQuantum(4)
	psl := e.cpu.PSL()
	psl.SetIPL(20)
	e.cpu.SetPSL(psl)
	e.Interrupt(ExcConRead, 20, 0) // masked: queued

	if !strings.Contains(buf.String(), "DEBUG(INTERRUPTS): queue interrupt") {
		t.Errorf("output = %q, want a queue trace line", buf.String())
	}

	buf.Reset()
	e.tickQuantum()
	if !strings.Contains(buf.String(), "DEBUG(INTERRUPTS): quantum; evaluating interrupt") {
		t.Errorf("output = %q, want a quantum-scan trace line", buf.String())
	}
}

func TestInterruptNoDebugTraceWhenFlagClear(t *testing.T) {
	e := interruptEngine(t)

	var buf bytes.Buffer

	e.cpu.SetDebugWriter(&buf)
	e.cpu.SetDebug(0)

	e.Interrupt(ExcConWrite, 20, 0)
	if buf.Len() != 0 {
		t.Errorf("output = %q, want no trace output with DebugInterrupts clear", buf.String())
	}
}

func TestDeliverConsoleByteDropsAndTracesWhenAlreadyPending(t *testing.T) {
	e := interruptEngine(t)

	var buf bytes.Buffer

	e.cpu.SetDebugWriter(&buf)
	e.cpu.SetDebug(vax.DebugKeyboard)

	e.DeliverConsoleByte('A')
	buf.Reset()
	e.DeliverConsoleByte('B') // RXCS<7> still set: dropped, not overwritten

	if got := e.cpu.PR(vax.RXDB); got != uint32('A') {
		t.Errorf("RXDB = %#x, want 'A' (second byte should have been dropped)", got)
	}
	if !strings.Contains(buf.String(), "KBD: hit, not read yet, ignoring") {
		t.Errorf("output = %q, want the ignoring trace line", buf.String())
	}
}

func TestInterruptMaskedByCurrentIPLIsQueuedNotDelivered(t *testing.T) {
	e := interruptEngine(t)
	e.SetQuantum(4)
	psl := e.cpu.PSL()
	psl.SetIPL(20)
	e.cpu.SetPSL(psl)

	e.Interrupt(ExcConWrite, 20, 0) // ipl == current IPL: masked
	if e.interruptPending {
		t.Fatal("expected Interrupt to queue, not deliver immediately, when ipl <= current IPL")
	}

	if len(e.iqueue) != 1 {
		t.Fatalf("len(iqueue) = %d, want 1", len(e.iqueue))
	}
	// A quantum==0 admission request (masked only by IPL, not deliberately
	// delayed) collapses the quantum countdown to 0 too, matching
	// interrupt()'s own "reset quantum evaluation now" -- so the very next
	// quantum boundary, not a full quantum period later, re-evaluates it.
	if got, _ := e.Quantum(); got != 0 {
		t.Fatalf("quantumCurrent = %d, want 0 (collapsed by the quantum==0 admission request)", got)
	}

	// Lower the IPL below the queued interrupt's own; the next quantum
	// boundary should now admit it.
	psl = e.cpu.PSL()
	psl.SetIPL(0)
	e.cpu.SetPSL(psl)

	e.tickQuantum()
	if !e.interruptPending {
		t.Fatal("expected the queued interrupt to be admitted once its IPL exceeds the (now-lowered) current IPL")
	}
}

// TestPendingInterruptsReportsBothHalves checks SHOW FAULT's own data
// source: PendingInterrupts must surface both the immediately-deliverable
// interrupt (if any) and everything still aging in the quantum queue.
func TestPendingInterruptsReportsBothHalves(t *testing.T) {
	e := interruptEngine(t)
	e.SetQuantum(4)

	pending, queued := e.PendingInterrupts()
	if pending != nil || len(queued) != 0 {
		t.Fatalf("expected no pending interrupts initially, got pending=%v queued=%v", pending, queued)
	}

	psl := e.cpu.PSL()
	psl.SetIPL(20)
	e.cpu.SetPSL(psl)
	e.Interrupt(ExcConWrite, 20, 0) // masked: queued, not delivered

	pending, queued = e.PendingInterrupts()
	if pending != nil {
		t.Fatalf("expected no immediately-pending interrupt, got %+v", pending)
	}

	if len(queued) != 1 || queued[0].Code != ExcConWrite || queued[0].IPL != 20 {
		t.Fatalf("queued = %+v, want one ExcConWrite entry at IPL 20", queued)
	}

	psl.SetIPL(0)
	e.cpu.SetPSL(psl)
	e.Interrupt(ExcConRead, 21, 0) // unmasked: delivered immediately

	pending, queued = e.PendingInterrupts()
	if pending == nil || pending.Code != ExcConRead || pending.IPL != 21 {
		t.Fatalf("pending = %+v, want ExcConRead at IPL 21", pending)
	}
	
	if len(queued) != 1 {
		t.Fatalf("expected the earlier queued entry to remain, got %+v", queued)
	}
}

// TestClearInterrupt checks CLEAR INTERRUPT <id>'s own selective removal:
// only matching-code entries in the queue are removed, everything else
// (a non-matching code, Engine's own immediately-pending interrupt) is
// left alone.
func TestClearInterrupt(t *testing.T) {
	e := interruptEngine(t)
	e.SetQuantum(4)
	psl := e.cpu.PSL()
	psl.SetIPL(20)
	e.cpu.SetPSL(psl)

	e.Interrupt(ExcConWrite, 20, 0)
	e.Interrupt(ExcConRead, 20, 0)
	e.Interrupt(ExcConWrite, 20, 0)

	if n := e.ClearInterrupt(ExcConWrite); n != 2 {
		t.Fatalf("ClearInterrupt(ExcConWrite) = %d, want 2", n)
	}

	_, queued := e.PendingInterrupts()
	if len(queued) != 1 || queued[0].Code != ExcConRead {
		t.Fatalf("queued = %+v, want only the ExcConRead entry left", queued)
	}
}

// TestClearAllInterrupts checks CLEAR INTERRUPT/ALL: both the queue and any
// immediately-pending interrupt are cleared.
func TestClearAllInterrupts(t *testing.T) {
	e := interruptEngine(t)
	e.SetQuantum(4)

	psl := e.cpu.PSL()
	psl.SetIPL(20)
	e.cpu.SetPSL(psl)
	e.Interrupt(ExcConWrite, 20, 0) // masked: queued

	psl.SetIPL(0)
	e.cpu.SetPSL(psl)
	e.Interrupt(ExcConRead, 21, 0) // unmasked: delivered immediately

	if n := e.ClearAllInterrupts(); n != 1 {
		t.Fatalf("ClearAllInterrupts() = %d, want 1 (queue length only)", n)
	}

	pending, queued := e.PendingInterrupts()
	if pending != nil || len(queued) != 0 {
		t.Fatalf("expected everything cleared, got pending=%v queued=%v", pending, queued)
	}
}

func TestInterruptQuantumDelayDefersEvenWhenUnmasked(t *testing.T) {
	e := interruptEngine(t)
	e.SetQuantum(5)

	e.Interrupt(ExcConRead, 20, 5) // fully unmasked (IPL 0), but a 5-tick delay requested
	if e.interruptPending {
		t.Fatal("expected a nonzero quantum delay to queue even when otherwise immediately admittable")
	}
	if len(e.iqueue) != 1 || e.iqueue[0].age != 1 {
		t.Fatalf("iqueue = %+v, want one entry with age 1 (5/5)", e.iqueue)
	}
}

func TestScanInterruptQueueTakesOnlyOnePerTick(t *testing.T) {
	e := interruptEngine(t)
	e.SetQuantum(1)

	e.iqueue = []*queuedInterrupt{
		{code: ExcConWrite, ipl: 20, age: 0},
		{code: ExcConRead, ipl: 20, age: 0},
	}

	e.tickQuantum()
	if !e.interruptPending {
		t.Fatal("expected one interrupt to be admitted")
	}
	if len(e.iqueue) != 1 {
		t.Errorf("len(iqueue) after one tick = %d, want 1 (the second entry stays queued)", len(e.iqueue))
	}
}

func TestTickIntervalClockFiresAtIPL22WhenRunAndIESet(t *testing.T) {
	e := interruptEngine(t)
	e.SetQuantum(1) // one tickQuantum call == one quantum boundary == one interval-clock tick
	// 0xFFFFFFFF - 0xFFFFFFFB == 4: it takes exactly 4 ticks (each ICR++) to
	// reach the 0xFFFFFFFF fire condition from this starting value.
	e.cpu.SetPR(vax.NICR, 0xFFFFFFFB)
	e.cpu.SetPR(vax.ICR, 0xFFFFFFFB)
	e.cpu.SetPR(vax.ICCS, iccsRun|deviceIE)

	for i := 0; i < 3; i++ {
		e.tickQuantum()
		if e.interruptPending {
			t.Fatalf("interval interrupt admitted after %d ticks, want 4", i+1)
		}
	}
	e.tickQuantum()
	if !e.interruptPending {
		t.Fatal("expected the interval-clock interrupt to be admitted on the 4th tick")
	}
	if e.interruptCode != ExcInterval || e.interruptIPL != 22 {
		t.Errorf("interruptCode/IPL = %#x/%d, want ExcInterval/22", e.interruptCode, e.interruptIPL)
	}
	if got := e.cpu.PR(vax.ICR); got != e.cpu.PR(vax.NICR) {
		t.Errorf("ICR = %#x after firing, want reloaded from NICR (%#x)", got, e.cpu.PR(vax.NICR))
	}
}

func TestTickIntervalClockNoOpWhenNotRunning(t *testing.T) {
	e := interruptEngine(t)
	e.SetQuantum(1)
	e.cpu.SetPR(vax.NICR, 0xFFFFFFFF)
	e.cpu.SetPR(vax.ICR, 0xFFFFFFFE)
	e.cpu.SetPR(vax.ICCS, deviceIE) // IE set, RUN clear

	for i := 0; i < 10; i++ {
		e.tickQuantum()
	}
	if e.interruptPending {
		t.Error("interval-clock interrupt admitted while ICCS<RUN> was clear")
	}
}
