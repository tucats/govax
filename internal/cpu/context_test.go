package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// Addresses for the process-context tests. Memory mapping is off in these
// tests (MAPEN is 0), so virtual and physical addresses are the same.
const (
	ctxPCB    = 0xA000 // the PCB PCBB points at
	ctxKSP    = 0x8000 // a kernel stack
	ctxISP    = 0x9000 // the interrupt stack
	ctxStack  = 0x7000 // where the tests put a PC/PSL pair for SVPCTX
	ctxSentry = 0xDEADBEEF
)

// validPCB returns a PCB LDPCTX accepts: P0BR at the start of S0 and P1BR
// 2**23 bytes below it, as govax's VMINIT lays process page tables out.
func validPCB() PCB {
	p := PCB{
		SP:     [4]uint32{ctxKSP, 0x6000, 0x5000, 0x4000},
		PC:     0x3000,
		PSL:    vax.PSL(0x03C00000), // user mode, previous mode user, IPL 0
		P0BR:   0x80000000,
		P0LR:   0x4000,
		ASTLVL: 4,
		P1BR:   0x80000000 - 1<<23,
		P1LR:   0x1FE000,
		PME:    true,
	}

	for i := range p.R {
		p.R[i] = 0x1000 + uint32(i)
	}

	return p
}

// faultCode returns err's exception code, failing the test if err isn't a
// *Fault.
func faultCode(t *testing.T, err error) Exception {
	t.Helper()

	var f *Fault
	if !errors.As(err, &f) {
		t.Fatalf("err = %v, want a *Fault", err)
	}

	return f.Code
}

// TestPCBLayout checks WritePCB puts each field at table 6.1's offset, with
// ASTLVL and PME packed beside P0LR and P1LR, and ReadPCB reads them back.
func TestPCBLayout(t *testing.T) {
	e := kernelEngine()
	p := validPCB()

	if err := WritePCB(e.mem, ctxPCB, &p); err != nil {
		t.Fatal(err)
	}

	want := map[uint32]uint32{
		PCBKSP: ctxKSP, PCBESP: 0x6000, PCBSSP: 0x5000, PCBUSP: 0x4000,
		PCBR0: 0x1000, PCBR0 + 11*4: 0x100B, PCBAP: 0x100C, PCBFP: 0x100D,
		PCBPC: 0x3000, PCBPSL: 0x03C00000,
		PCBP0BR: 0x80000000, PCBP0LRAS: 0x04004000,
		PCBP1BR: 0x7F800000, PCBP1LRPM: 0x801FE000,
	}

	for off, v := range want {
		if got := physLongword(t, e, ctxPCB+off); got != v {
			t.Errorf("PCB+%#02x = %#08x, want %#08x", off, got, v)
		}
	}

	got, err := ReadPCB(e.mem, ctxPCB)
	if err != nil {
		t.Fatal(err)
	}

	if got != p {
		t.Errorf("ReadPCB = %+v, want %+v", got, p)
	}
}

func physLongword(t *testing.T, e *Engine, addr uint32) uint32 {
	t.Helper()

	v, err := e.mem.LoadLongword(e.cpu, addr)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

// svpctxEngine is a kernel-mode engine with a PCB at ctxPCB whose
// memory-management longwords hold a sentinel (SVPCTX must leave them
// alone), registers R0-FP set to 0x500+n, the four mode stack pointers'
// registers set, and a PC/PSL pair at ctxStack.
func svpctxEngine(t *testing.T) *Engine {
	t.Helper()

	e := kernelEngine()
	e.cpu.SetPR(vax.PCBB, ctxPCB)

	for off := uint32(PCBP0BR); off < PCBSize; off += 4 {
		putLongword(t, e.cpu, e.mem, ctxPCB+off, ctxSentry)
	}

	for r := vax.R0; r <= vax.FP; r++ {
		e.cpu.SetGPR(r, 0x500+uint32(r))
	}

	e.cpu.SetPR(vax.KSP, 0x1111) // stale while the CPU runs on the kernel stack
	e.cpu.SetPR(vax.ESP, 0x6000)
	e.cpu.SetPR(vax.SSP, 0x5000)
	e.cpu.SetPR(vax.USP, 0x4000)
	e.cpu.SetPR(vax.ISP, ctxISP)

	putLongword(t, e.cpu, e.mem, ctxStack, 0x2468)       // PC
	putLongword(t, e.cpu, e.mem, ctxStack+4, 0x03C00004) // PSL: user mode, Z set
	e.cpu.SetGPR(vax.SP, ctxStack)

	return e
}

// TestSvpctxFromKernelStack follows SVPCTX's operation text from the
// kernel stack: registers and the popped PC and PSL go into the PCB, the
// kernel stack pointer saved is SP after the pops, the memory-management
// fields are untouched, and the CPU moves to the interrupt stack at IPL 1.
func TestSvpctxFromKernelStack(t *testing.T) {
	e := svpctxEngine(t)

	stepInstruction(t, e, 0x07) // SVPCTX

	p, err := ReadPCB(e.mem, ctxPCB)
	if err != nil {
		t.Fatal(err)
	}

	if want := [4]uint32{ctxStack + 8, 0x6000, 0x5000, 0x4000}; p.SP != want {
		t.Errorf("PCB stack pointers = %#x, want %#x", p.SP, want)
	}

	for r := range p.R {
		if want := 0x500 + uint32(r); p.R[r] != want {
			t.Errorf("PCB R%d = %#x, want %#x", r, p.R[r], want)
		}
	}

	if p.PC != 0x2468 || p.PSL != 0x03C00004 {
		t.Errorf("PCB PC/PSL = %#x/%#x, want 0x2468/0x3c00004", p.PC, uint32(p.PSL))
	}

	for off := uint32(PCBP0BR); off < PCBSize; off += 4 {
		if got := physLongword(t, e, ctxPCB+off); got != ctxSentry {
			t.Errorf("PCB+%#x = %#x, want it unchanged", off, got)
		}
	}

	psl := e.cpu.PSL()
	if !psl.IS() || psl.CurMod() != vax.Kernel || psl.IPL() != 1 || e.cpu.PR(vax.IPL) != 1 {
		t.Errorf("PSL = %#x (IPL register %d), want kernel mode on the interrupt stack at IPL 1",
			uint32(psl), e.cpu.PR(vax.IPL))
	}

	if got := e.cpu.GPR(vax.SP); got != ctxISP {
		t.Errorf("SP = %#x, want the interrupt stack's %#x", got, ctxISP)
	}

	if got := e.cpu.PR(vax.KSP); got != ctxStack+8 {
		t.Errorf("KSP = %#x, want %#x", got, ctxStack+8)
	}
}

// TestSvpctxKeepsHigherIPL checks the IPL is maximized with 1, not set to
// it.
func TestSvpctxKeepsHigherIPL(t *testing.T) {
	e := svpctxEngine(t)
	psl := e.cpu.PSL()
	psl.SetIPL(3)
	e.cpu.SetPSL(psl)
	e.cpu.SetPR(vax.IPL, 3)

	stepInstruction(t, e, 0x07)

	if got := e.cpu.PSL().IPL(); got != 3 {
		t.Errorf("IPL = %d, want 3", got)
	}
}

// TestSvpctxOnInterruptStack: already on the interrupt stack, SVPCTX saves
// the kernel stack pointer from its register and only pops the stack.
func TestSvpctxOnInterruptStack(t *testing.T) {
	e := svpctxEngine(t)
	e.cpu.SetPR(vax.KSP, ctxKSP)

	psl := e.cpu.PSL()
	psl.SetIS(true)
	psl.SetIPL(3)
	e.cpu.SetPSL(psl)
	e.cpu.SetPR(vax.IPL, 3)

	stepInstruction(t, e, 0x07)

	p, err := ReadPCB(e.mem, ctxPCB)
	if err != nil {
		t.Fatal(err)
	}

	if p.SP[vax.Kernel] != ctxKSP {
		t.Errorf("PCB KSP = %#x, want %#x", p.SP[vax.Kernel], ctxKSP)
	}

	if got := e.cpu.GPR(vax.SP); got != ctxStack+8 {
		t.Errorf("SP = %#x, want %#x (the pair popped, no stack switch)", got, ctxStack+8)
	}

	if got := e.cpu.PSL(); got != psl {
		t.Errorf("PSL = %#x, want %#x unchanged", uint32(got), uint32(psl))
	}
}

// TestContextInstructionsArePrivileged: both instructions need kernel
// mode. The handlers are called directly, as TestEmulMtprRequiresKernelMode
// does, so that no fault frame needs a stack.
func TestContextInstructionsArePrivileged(t *testing.T) {
	for _, h := range []struct {
		name string
		fn   Handler
	}{{"LDPCTX", emulLdpctx}, {"SVPCTX", emulSvpctx}} {
		for _, mode := range []vax.AccessMode{vax.Executive, vax.Supervisor, vax.User} {
			e := newEngine()
			psl := e.cpu.PSL()
			psl.SetCurMod(mode)
			psl.SetIS(false)
			e.cpu.SetPSL(psl)

			if got := faultCode(t, h.fn(e, &Decoded{})); got != ExcPrivileged {
				t.Errorf("%s in mode %d: fault %#x, want ExcPrivileged", h.name, mode, got)
			}
		}
	}
}

// ldpctxEngine is a kernel-mode engine on the interrupt stack, with
// validPCB at ctxPCB.
func ldpctxEngine(t *testing.T) (*Engine, PCB) {
	t.Helper()

	e := kernelEngine()
	p := validPCB()

	if err := WritePCB(e.mem, ctxPCB, &p); err != nil {
		t.Fatal(err)
	}

	e.cpu.SetPR(vax.PCBB, ctxPCB)

	psl := e.cpu.PSL()
	psl.SetIS(true)
	psl.SetIPL(3)
	e.cpu.SetPSL(psl)
	e.cpu.SetPR(vax.IPL, 3)
	e.cpu.SetGPR(vax.SP, ctxISP)

	return e, p
}

// TestLdpctxLoadsContext follows LDPCTX's operation text: every register
// and memory-management register is loaded, the interrupt stack pointer
// is saved, and the PCB's PSL and PC are pushed on the new kernel stack;
// the REI that follows then resumes the process in user mode.
func TestLdpctxLoadsContext(t *testing.T) {
	e, p := ldpctxEngine(t)
	_, _, flushes, pflushes := e.mem.TBStats()

	stepInstruction(t, e, 0x06) // LDPCTX

	// Only the process half of the translation buffer is flushed.
	if _, _, f, pf := e.mem.TBStats(); f != flushes || pf != pflushes+1 {
		t.Errorf("TB flushes, process flushes = %d, %d; want %d, %d", f, pf, flushes, pflushes+1)
	}

	for r := vax.R0; r <= vax.FP; r++ {
		if got := e.cpu.GPR(r); got != p.R[r] {
			t.Errorf("R%d = %#x, want %#x", r, got, p.R[r])
		}
	}

	regs := []struct {
		r    vax.PrivReg
		want uint32
	}{
		{vax.ESP, 0x6000}, {vax.SSP, 0x5000}, {vax.USP, 0x4000}, {vax.ISP, ctxISP},
		{vax.P0BR, p.P0BR}, {vax.P0LR, p.P0LR}, {vax.P1BR, p.P1BR}, {vax.P1LR, p.P1LR},
		{vax.ASTLVL, 4}, {vax.PME, 1},
	}
	for _, r := range regs {
		if got := e.cpu.PR(r.r); got != r.want {
			t.Errorf("privileged register %d = %#x, want %#x", r.r, got, r.want)
		}
	}

	if psl := e.cpu.PSL(); psl.IS() || psl.CurMod() != vax.Kernel || psl.IPL() != 3 {
		t.Errorf("PSL = %#x, want kernel mode on the kernel stack, IPL unchanged", uint32(psl))
	}

	if got := e.cpu.GPR(vax.SP); got != ctxKSP-8 {
		t.Fatalf("SP = %#x, want %#x", got, ctxKSP-8)
	}

	if pc, psl := physLongword(t, e, ctxKSP-8), physLongword(t, e, ctxKSP-4); pc != p.PC || psl != uint32(p.PSL) {
		t.Errorf("kernel stack holds PC %#x, PSL %#x; want %#x, %#x", pc, psl, p.PC, uint32(p.PSL))
	}

	stepInstruction(t, e, 0x02) // REI

	if got := e.cpu.GPR(vax.PC); got != p.PC {
		t.Errorf("PC after REI = %#x, want %#x", got, p.PC)
	}

	if got := e.cpu.PSL(); got != p.PSL {
		t.Errorf("PSL after REI = %#x, want %#x", uint32(got), uint32(p.PSL))
	}

	if got := e.cpu.GPR(vax.SP); got != 0x4000 {
		t.Errorf("SP after REI = %#x, want the user stack's 0x4000", got)
	}

	if got := e.cpu.PR(vax.KSP); got != ctxKSP {
		t.Errorf("KSP after REI = %#x, want %#x", got, ctxKSP)
	}
}

// TestLdpctxUndefined checks each UNDEFINED case of LDPCTX's operation
// text takes a reserved-operand fault and changes nothing.
func TestLdpctxUndefined(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *Engine, w *pcbWords)
	}{
		{"not on the interrupt stack", func(e *Engine, _ *pcbWords) {
			psl := e.cpu.PSL()
			psl.SetIS(false)
			e.cpu.SetPSL(psl)
		}},
		{"P0BR in P0", func(_ *Engine, w *pcbWords) { w[PCBP0BR/4] = 0x00001000 }},
		{"P0BR in S1", func(_ *Engine, w *pcbWords) { w[PCBP0BR/4] = 0xC0000000 }},
		{"P0BR unaligned", func(_ *Engine, w *pcbWords) { w[PCBP0BR/4] = 0x80000002 }},
		{"P0LR bit 31", func(_ *Engine, w *pcbWords) { w[PCBP0LRAS/4] |= 0x80000000 }},
		{"P0LR bit 27", func(_ *Engine, w *pcbWords) { w[PCBP0LRAS/4] |= 0x08000000 }},
		{"P0LR bit 22", func(_ *Engine, w *pcbWords) { w[PCBP0LRAS/4] |= 0x00400000 }},
		{"ASTLVL 5", func(_ *Engine, w *pcbWords) { w[PCBP0LRAS/4] = w[PCBP0LRAS/4]&^0x07000000 | 0x05000000 }},
		{"P1BR's table ends in P1", func(_ *Engine, w *pcbWords) { w[PCBP1BR/4] = 0x7F000000 }},
		{"P1BR unaligned", func(_ *Engine, w *pcbWords) { w[PCBP1BR/4] |= 1 }},
		{"P1LR bit 30", func(_ *Engine, w *pcbWords) { w[PCBP1LRPM/4] |= 0x40000000 }},
		{"P1LR bit 22", func(_ *Engine, w *pcbWords) { w[PCBP1LRPM/4] |= 0x00400000 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, p := ldpctxEngine(t)
			w := p.words()
			tc.setup(e, &w)

			if err := writePCBWords(e.mem, ctxPCB, &w, PCBSize); err != nil {
				t.Fatal(err)
			}

			before := *e.cpu

			if got := faultCode(t, emulLdpctx(e, &Decoded{})); got != ExcReservedOp {
				t.Errorf("fault %#x, want ExcReservedOp", got)
			}

			if *e.cpu != before {
				t.Error("registers changed")
			}
		})
	}
}

// TestLdpctxBadKernelStack: a kernel stack LDPCTX can't push on is
// UNDEFINED too, and the registers are restored before the fault.
func TestLdpctxBadKernelStack(t *testing.T) {
	e, p := ldpctxEngine(t)
	p.SP[vax.Kernel] = e.mem.Size() + 0x1000 // past the end of memory

	if err := WritePCB(e.mem, ctxPCB, &p); err != nil {
		t.Fatal(err)
	}

	before := *e.cpu

	if got := faultCode(t, emulLdpctx(e, &Decoded{})); got != ExcReservedOp {
		t.Errorf("fault %#x, want ExcReservedOp", got)
	}

	if *e.cpu != before {
		t.Error("registers changed")
	}
}

// TestSaveLoadContext checks the Go switcher's pair against the
// instructions: SaveContext stores what an interrupt and SVPCTX would,
// leaving the CPU on the interrupt stack at IPL 3, and LoadContext of the
// same PCB resumes exactly where the process was.
func TestSaveLoadContext(t *testing.T) {
	e := kernelEngine()
	e.cpu.SetPR(vax.PCBB, ctxPCB)

	p := validPCB()
	if err := WritePCB(e.mem, ctxPCB, &p); err != nil {
		t.Fatal(err)
	}

	// A user-mode process at IPL 0, its user stack live in SP.
	for r := vax.R0; r <= vax.FP; r++ {
		e.cpu.SetGPR(r, 0x700+uint32(r))
	}

	e.cpu.SetPR(vax.KSP, ctxKSP)
	e.cpu.SetPR(vax.ESP, 0x6000)
	e.cpu.SetPR(vax.SSP, 0x5000)
	e.cpu.SetPR(vax.USP, 0x1234) // stale
	e.cpu.SetPR(vax.ISP, ctxISP)
	e.cpu.SetGPR(vax.SP, 0x3FF0)
	e.cpu.SetGPR(vax.PC, 0x2222)
	e.cpu.SetPSL(vax.PSL(0x03C00008)) // user, N set
	e.cpu.SetPR(vax.IPL, 0)

	if err := e.SaveContext(); err != nil {
		t.Fatal(err)
	}

	saved, err := ReadPCB(e.mem, ctxPCB)
	if err != nil {
		t.Fatal(err)
	}

	if want := [4]uint32{ctxKSP, 0x6000, 0x5000, 0x3FF0}; saved.SP != want {
		t.Errorf("PCB stack pointers = %#x, want %#x", saved.SP, want)
	}

	if saved.PC != 0x2222 || saved.PSL != 0x03C00008 || saved.R[vax.FP] != 0x70D {
		t.Errorf("PCB PC/PSL/FP = %#x/%#x/%#x", saved.PC, uint32(saved.PSL), saved.R[vax.FP])
	}

	if saved.P0BR != p.P0BR || saved.P1LR != p.P1LR || !saved.PME {
		t.Error("SaveContext changed the PCB's memory-management fields")
	}

	if psl := e.cpu.PSL(); !psl.IS() || psl.CurMod() != vax.Kernel || psl.IPL() != 3 || e.cpu.GPR(vax.SP) != ctxISP {
		t.Errorf("after SaveContext PSL = %#x, SP = %#x; want the interrupt stack at IPL 3",
			uint32(psl), e.cpu.GPR(vax.SP))
	}

	if err := e.SaveContext(); !errors.Is(err, ErrContextOnInterruptStack) {
		t.Errorf("SaveContext on the interrupt stack: err = %v", err)
	}

	// Scribble on the registers, then load the process back.
	for r := vax.R0; r <= vax.FP; r++ {
		e.cpu.SetGPR(r, 0)
	}

	if err := e.LoadContext(); err != nil {
		t.Fatal(err)
	}

	for r := vax.R0; r <= vax.FP; r++ {
		if got := e.cpu.GPR(r); got != 0x700+uint32(r) {
			t.Errorf("R%d = %#x, want %#x", r, got, 0x700+uint32(r))
		}
	}

	if e.cpu.GPR(vax.PC) != 0x2222 || e.cpu.PSL() != 0x03C00008 || e.cpu.GPR(vax.SP) != 0x3FF0 {
		t.Errorf("PC/PSL/SP = %#x/%#x/%#x, want 0x2222/0x3c00008/0x3ff0",
			e.cpu.GPR(vax.PC), uint32(e.cpu.PSL()), e.cpu.GPR(vax.SP))
	}

	if e.cpu.PR(vax.KSP) != ctxKSP || e.cpu.PR(vax.ISP) != ctxISP || e.cpu.PR(vax.IPL) != 0 {
		t.Errorf("KSP/ISP/IPL = %#x/%#x/%d", e.cpu.PR(vax.KSP), e.cpu.PR(vax.ISP), e.cpu.PR(vax.IPL))
	}

	// Not on the interrupt stack now, so LoadContext refuses.
	if got := faultCode(t, e.LoadContext()); got != ExcPrivileged {
		t.Errorf("LoadContext in user mode: fault %#x, want ExcPrivileged", got)
	}
}
