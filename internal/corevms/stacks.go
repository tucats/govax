package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// A process's stacks and hardware PCB (Phase 43).
//
// A VAX keeps one stack per access mode (kernel, executive, supervisor,
// user), and the CPU switches between them when the mode changes: a
// system service called from user mode runs on the kernel or executive
// stack, never on the user's. Each process needs all four, because a
// process that is switched out in the middle of a service leaves its
// frames on its own kernel stack, to be resumed later. Only the interrupt
// stack is shared: interrupts belong to no process.
//
// docs/MODE-STACKS.md describes process 1's layout, which VMINIT lays out
// in S0 below CONSOLE$SCRATCH. A new process gets the same layout in one
// run of S0 pool pages:
//
//	kernel stack       kernel pages, URKW (S0's default; see MODE-STACKS)
//	guard page         NA
//	executive stack    executive pages, EW
//	guard page         NA
//	supervisor stack   supervisor pages, SW
//
// and its user stack in its own P1, whose top is the same address in
// every process (0x7FE00000), as each P1 is private.
//
// The hardware PCB, the 96 bytes LDPCTX loads a process's registers from
// and SVPCTX saves them to (internal/cpu/context.go), gets a pool page of
// its own, kernel-only (KW). VMS keeps it in the process header, beside
// the process's page tables; a page apart is simpler and costs one page
// per process. PCBB holds its *physical* address: LDPCTX and SVPCTX run
// with the old process's mapping or none, so the PCB is never looked up
// through a page table.

// UserStackTop is every process's initial user stack pointer, in its P1.
const UserStackTop = 0x7FE00000

// ProcessStacks is a process's privileged stacks and hardware PCB.
type ProcessStacks struct {
	// KSP, ESP, and SSP are the initial kernel, executive, and supervisor
	// stack pointers (each its stack's last longword); USP the user's.
	KSP, ESP, SSP, USP uint32

	// Base is the S0 address of the pool run holding the three stacks;
	// zero for process 1, whose stacks are VMINIT's.
	Base uint32

	// PCB is the S0 address of the hardware PCB's page, and PCBB its
	// physical address (what the PCBB register holds while the process
	// is current). Zero until the process has one (EnsurePCB).
	PCB, PCBB uint32
}

// AdoptStacks returns process 1's stacks: VMINIT's, as the CPU's stack
// registers describe them. Process 1 has no PCB until EnsurePCB gives it
// one: VMINIT's layout has no room for it, and the pool's first pages
// are the microkernel's, claimed as ASM deposits it.
func AdoptStacks(c *vax.CPU) *ProcessStacks {
	return &ProcessStacks{
		KSP: c.PR(vax.KSP), ESP: c.PR(vax.ESP), SSP: c.PR(vax.SSP), USP: c.PR(vax.USP),
	}
}

// BuildStacks allocates a new process's kernel, executive, and
// supervisor stacks (with kernel, executive, and supervisor pages; each
// at least 1) and its PCB page from the S0 pool, charged to pid, with the
// layout and protections above.
func (sys *System) BuildStacks(pid, kernel, executive, supervisor uint32) (*ProcessStacks, error) {
	if kernel == 0 || executive == 0 || supervisor == 0 {
		return nil, fmt.Errorf("every stack needs at least a page (%d, %d, %d asked for)", kernel, executive, supervisor)
	}

	pages := kernel + 1 + executive + 1 + supervisor

	base, err := sys.AllocateS0(pages, pid, "privileged stacks")
	if err != nil {
		return nil, err
	}

	st := &ProcessStacks{Base: base, USP: UserStackTop}

	// Lay the run out from the bottom: each stack's pointer starts at its
	// last longword, and its guard page is just below it.
	va := base + kernel*pageSize
	st.KSP = va - 4

	type part struct {
		pages uint32
		prot  vm.Protection
		sp    *uint32
	}

	for _, p := range []part{{executive, vm.ProtEW, &st.ESP}, {supervisor, vm.ProtSW, &st.SSP}} {
		if err = sys.SetS0Protection(va, 1, vm.ProtNA); err != nil {
			break
		}

		va += pageSize

		if err = sys.SetS0Protection(va, p.pages, p.prot); err != nil {
			break
		}

		va += p.pages * pageSize
		*p.sp = va - 4
	}

	if err == nil {
		err = sys.allocatePCB(pid, st)
	}

	if err != nil {
		sys.s0.Free(base)

		return nil, err
	}

	return st, nil
}

// EnsurePCB gives env's process a hardware PCB page if it has none yet
// (process 1 starts without one; see AdoptStacks), and returns the PCB's
// physical address.
func (sys *System) EnsurePCB(env *Environment) (uint32, error) {
	if env.Stacks == nil {
		return 0, fmt.Errorf("process %08X has no stacks (VMINIT has not been run)", env.Process.PID)
	}

	if env.Stacks.PCB == 0 {
		if err := sys.allocatePCB(env.Process.PID, env.Stacks); err != nil {
			return 0, err
		}
	}

	return env.Stacks.PCBB, nil
}

// allocatePCB allocates st's PCB page, kernel-only, and finds its physical
// address.
func (sys *System) allocatePCB(pid uint32, st *ProcessStacks) error {
	va, err := sys.AllocateS0(1, pid, "hardware PCB")
	if err != nil {
		return err
	}

	pa, err := sys.mem.TranslateIn(sys.cpu, vm.AddressSpace{}, va, vm.AccessRead)
	if err == nil {
		err = sys.SetS0Protection(va, 1, vm.ProtKW)
	}

	if err != nil {
		sys.s0.Free(va)

		return err
	}

	st.PCB, st.PCBB = va, pa

	return nil
}

// InitialPCB is the hardware PCB a new process starts from: its stacks'
// initial pointers, its address space's registers, zeroed general
// registers, and pc and psl to start at. ASTLVL is 4, "no AST pending in
// any mode" (an AST is delivered when the current mode is at or below
// ASTLVL, and mode 4 doesn't exist). The first LDPCTX and REI for the
// process (or the Go switcher's LoadContext) load it.
func InitialPCB(space *ProcessSpace, st *ProcessStacks, pc uint32, psl vax.PSL) cpu.PCB {
	return cpu.PCB{
		SP:   [4]uint32{st.KSP, st.ESP, st.SSP, st.USP},
		PC:   pc,
		PSL:  psl,
		P0BR: space.P0BR, P0LR: space.P0LR,
		P1BR: space.P1BR, P1LR: space.P1LR,
		ASTLVL: 4,
	}
}
