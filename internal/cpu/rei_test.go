package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// makePSL builds a PSL from its fields, for the REI tests: the current and
// previous modes, the IPL, and whether the interrupt stack is in use.
func makePSL(cur, prv registers.AccessMode, ipl uint32, is bool) registers.PSL {
	var p registers.PSL

	p.SetCurMod(cur)
	p.SetPrvMod(prv)
	p.SetIPL(ipl)
	p.SetIS(is)

	return p
}

// TestReiChecksPoppedPSL runs REI with each kind of PSL the architecture
// refuses (the 1987 VAX Architecture Reference Manual's REI operation;
// reiAllows) and some it accepts. A refused one is a reserved operand
// fault with nothing changed: not SP, not the PSL, not PC.
func TestReiChecksPoppedPSL(t *testing.T) {
	const (
		frame = 0x6050 // the stack, holding the PC and PSL REI pops
		pc    = 0x5000
		start = 0x4000 // where the REI is (any address will do)
	)

	kernelAt := func(ipl uint32) registers.PSL { return makePSL(registers.Kernel, registers.Kernel, ipl, false) }
	interrupt := makePSL(registers.Kernel, registers.Kernel, 20, true)
	user := makePSL(registers.User, registers.User, 0, false)
	exec := makePSL(registers.Executive, registers.Executive, 0, false)

	cases := []struct {
		name  string
		cur   registers.PSL // the CPU's PSL when REI runs
		next  registers.PSL // the PSL REI pops
		allow bool
	}{
		// Accepted.
		{"kernel to user", kernelAt(0), user, true},
		{"kernel to executive", kernelAt(0), exec, true},
		{"user to user", user, user, true},
		{"interrupt stack to kernel at a lower IPL", interrupt, kernelAt(3), true},
		{"interrupt stack to the interrupt stack at a lower IPL", interrupt, makePSL(registers.Kernel, registers.Kernel, 8, true), true},
		{"user, previous mode user, with the PSW set", kernelAt(0), user | 0xFF, true},
		{"user to user, with TP set", user, user | 1<<30, true},

		// Refused.
		{"user to kernel (inward)", user, kernelAt(0), false},
		{"user to executive (inward)", user, exec, false},
		{"executive to kernel (inward)", exec, kernelAt(0), false},
		{"previous mode more privileged than current", kernelAt(0), makePSL(registers.User, registers.Kernel, 0, false), false},
		{"IPL above 0 in user mode", kernelAt(4), makePSL(registers.User, registers.User, 4, false), false},
		{"IS set with IPL 0", interrupt, makePSL(registers.Kernel, registers.Kernel, 0, true), false},
		{"IS set outside kernel mode", interrupt, makePSL(registers.User, registers.User, 4, true), false},
		{"IS set from off the interrupt stack", kernelAt(20), makePSL(registers.Kernel, registers.Kernel, 8, true), false},
		{"IPL raised", kernelAt(3), kernelAt(8), false},
		{"a PSW must-be-zero bit", kernelAt(0), user | 1<<8, false},
		{"bit 21 set", kernelAt(0), user | 1<<21, false},
		{"bit 28 set", kernelAt(0), user | 1<<28, false},
		{"bit 29 set", kernelAt(0), user | 1<<29, false},
		{"compatibility mode", kernelAt(0), user | 1<<31, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem := fixture()
			e := NewEngine(cpu, mem)

			cpu.SetPSL(tc.cur)
			cpu.SetPR(registers.IPL, tc.cur.IPL())
			cpu.SetGPR(registers.SP, frame)
			cpu.SetGPR(registers.PC, start)
			putLongword(t, cpu, mem, frame, pc)
			putLongword(t, cpu, mem, frame+4, uint32(tc.next))

			err := emulRei(e, &Decoded{})

			if tc.allow {
				if err != nil {
					t.Fatalf("REI of %08X from %08X: %v, want it restored", uint32(tc.next), uint32(tc.cur), err)
				}

				if got := cpu.GPR(registers.PC); got != pc {
					t.Errorf("PC = %#x, want %#x", got, pc)
				}

				// TP is the only bit REI may add: it's kept if it was set.
				if got := cpu.PSL(); got != tc.next|tc.cur&(1<<30) {
					t.Errorf("PSL = %08X, want %08X", uint32(got), uint32(tc.next))
				}

				return
			}

			var f *Fault
			if !errors.As(err, &f) || f.Code != ExcReservedOp {
				t.Fatalf("REI of %08X from %08X: %v, want a reserved operand fault", uint32(tc.next), uint32(tc.cur), err)
			}

			if got := cpu.PSL(); got != tc.cur {
				t.Errorf("PSL = %08X after the fault, want %08X unchanged", uint32(got), uint32(tc.cur))
			}

			if got := cpu.GPR(registers.SP); got != frame {
				t.Errorf("SP = %#x after the fault, want %#x unchanged", got, frame)
			}

			if got := cpu.GPR(registers.PC); got != start {
				t.Errorf("PC = %#x after the fault, want %#x unchanged", got, start)
			}
		})
	}
}

// TestReiStaysOnInterruptStack: an REI from one interrupt level to a lower
// one restores a PSL with IS set, so the new SP is the interrupt stack's
// (what's left of it once the PC and PSL are popped), not the kernel
// stack's.
func TestReiStaysOnInterruptStack(t *testing.T) {
	const (
		frame = 0x6050
		ksp   = 0x7000
	)

	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	cpu.SetPSL(makePSL(registers.Kernel, registers.Kernel, 20, true))
	cpu.SetPR(registers.IPL, 20)
	cpu.SetPR(registers.KSP, ksp)
	cpu.SetGPR(registers.SP, frame)
	putLongword(t, cpu, mem, frame, 0x5000)
	putLongword(t, cpu, mem, frame+4, uint32(makePSL(registers.Kernel, registers.Kernel, 8, true)))

	if err := emulRei(e, &Decoded{}); err != nil {
		t.Fatalf("REI: %v", err)
	}

	if got := cpu.GPR(registers.SP); got != frame+8 {
		t.Errorf("SP = %#x, want the interrupt stack's %#x", got, frame+8)
	}

	if got := cpu.PR(registers.KSP); got != ksp {
		t.Errorf("KSP = %#x, want it untouched, %#x", got, ksp)
	}

	if psl := cpu.PSL(); !psl.IS() || psl.IPL() != 8 {
		t.Errorf("PSL = %08X, want IS set at IPL 8", uint32(psl))
	}
}
