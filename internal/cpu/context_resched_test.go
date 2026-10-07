package cpu_test

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// reschedSource is the VAX Architecture Reference Manual's RESCHED example
// (chapter 6, after SVPCTX) as a program: two processes take turns, each
// requesting the IPL 3 software interrupt (MTPR #3, #SIRR) to give the
// CPU up. The interrupt runs RESCHED on the outgoing process's kernel
// stack; RESCHED saves it with SVPCTX, points PCBB at the other process's
// PCB, and resumes that one with LDPCTX and REI. Its "run queue" is the
// one longword at OTHER, swapped each time. Each process writes two
// registers, one before and one after giving up the CPU, so the test can
// see that each kept its own. Memory mapping is off, so addresses are
// physical.
const reschedSource = `
resched:
	svpctx				; save the interrupted process
	mfpr	#^X10, r0		; PCBB: its PCB
	movl	@#other, r1		; the process to run
	movl	r0, @#other		; next time, the one just saved
	mtpr	r1, #^X10		; PCBB
	ldpctx				; load it
	rei				; and run it
other:	.long	0

proca:
	movl	#^XA1, r2
	mtpr	#3, #^X14		; SIRR: reschedule
	movl	#^XA2, r3
	mtpr	#3, #^X14
	halt

procb:
	movl	#^XB1, r2
	mtpr	#3, #^X14
	movl	#^XB2, r3
	mtpr	#3, #^X14
	brb	procb
`

// TestReschedProgram runs reschedSource: A runs, B runs, A finishes its
// second register and reschedules, B finishes its own, and A runs again
// to its HALT. Then both PCBs and both kernel stacks are checked.
func TestReschedProgram(t *testing.T) {
	const (
		origin = 0x1000
		scbb   = 0x0200
		pcbA   = 0xA000
		pcbB   = 0xA100
		kspA   = 0x8000
		kspB   = 0x7000
		isp    = 0x9000
	)

	a := asm.New(false)
	a.SetOrigin(origin)

	code, err := a.Assemble(reschedSource)
	if err != nil {
		t.Fatal(err)
	}

	sym := func(name string) uint32 {
		s, ok := a.Symbols().Get(name)
		if !ok {
			t.Fatalf("no symbol %s", name)
		}

		return s.Value
	}

	c, mem := vax.New(), vm.NewMemory(1<<20)
	if err := mem.StorePhysical(origin, code); err != nil {
		t.Fatal(err)
	}

	store := func(addr, v uint32) {
		if err := mem.StoreLongword(c, addr, v); err != nil {
			t.Fatal(err)
		}
	}

	// The IPL 3 software interrupt's SCB vector: RESCHED, on the kernel
	// stack (bit 0 clear).
	c.SetPR(vax.SCBB, scbb)
	store(scbb+uint32(cpu.ExcSoftware1)+2*4, sym("RESCHED"))
	store(sym("OTHER"), pcbB)

	// Both PCBs get valid memory-management fields, which SVPCTX never
	// writes. B's also gets where it starts: kernel mode, IPL 0, at PROCB.
	for _, p := range []struct {
		addr, ksp, pc uint32
	}{{pcbA, kspA, 0}, {pcbB, kspB, sym("PROCB")}} {
		pcb := cpu.PCB{
			PC: p.pc, P0BR: 0x80000000, P1BR: 0x80000000 - 1<<23, P1LR: 0x200000, ASTLVL: 4,
		}
		pcb.SP[vax.Kernel] = p.ksp

		if err := cpu.WritePCB(mem, p.addr, &pcb); err != nil {
			t.Fatal(err)
		}
	}

	// Process A is running, in kernel mode at IPL 0.
	c.SetPR(vax.PCBB, pcbA)
	c.SetPR(vax.ISP, isp)
	c.SetGPR(vax.SP, kspA)
	c.SetGPR(vax.PC, sym("PROCA"))

	e := cpu.NewEngine(c, mem)

	var steps int
	for err = nil; err == nil && steps < 100; steps++ {
		err = e.Step()
	}

	if !errors.Is(err, cpu.ErrHalted) {
		t.Fatalf("after %d steps: err = %v, want HALT", steps, err)
	}

	// A is running: its registers, its kernel stack back where it began.
	if c.GPR(vax.R2) != 0xA1 || c.GPR(vax.R3) != 0xA2 {
		t.Errorf("A's R2, R3 = %#x, %#x; want 0xA1, 0xA2", c.GPR(vax.R2), c.GPR(vax.R3))
	}

	if c.GPR(vax.SP) != kspA || c.PR(vax.ISP) != isp || c.PSL().IS() || c.PSL().IPL() != 0 {
		t.Errorf("SP %#x, ISP %#x, PSL %#x; want A's kernel stack at IPL 0",
			c.GPR(vax.SP), c.PR(vax.ISP), uint32(c.PSL()))
	}

	if got := c.PR(vax.PCBB); got != pcbA {
		t.Errorf("PCBB = %#x, want A's", got)
	}

	// B is saved: its registers, and the PC after its second MTPR (the
	// BRB), with its kernel stack pointer as it was before the interrupt.
	b, err := cpu.ReadPCB(mem, pcbB)
	if err != nil {
		t.Fatal(err)
	}

	if b.R[2] != 0xB1 || b.R[3] != 0xB2 {
		t.Errorf("B's R2, R3 = %#x, %#x; want 0xB1, 0xB2", b.R[2], b.R[3])
	}

	if want := uint32(origin + len(code) - 2); b.PC != want {
		t.Errorf("B's PC = %#x, want %#x (its BRB)", b.PC, want)
	}

	if b.SP[vax.Kernel] != kspB || b.PSL != 0 {
		t.Errorf("B's KSP, PSL = %#x, %#x; want %#x, 0", b.SP[vax.Kernel], uint32(b.PSL), kspB)
	}
}
