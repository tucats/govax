package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/vax"
)

// Phase 44's subtask 7: only process 1's outcomes end the console's run.
// A process other than process 1 whose image ends just stops; a HALT
// anywhere stops the machine, and names the process.

// startedLikeAnImage builds a process, as handBuiltProcess does, whose
// program starts on a call frame like the one RUN builds for an image
// (cpu.Engine.CallEntry): its saved PC and FP are cpu.SentinelReturn, so
// a RET from the program's outermost procedure, or $EXIT unwinding to
// it, ends its image, as process 1's RET ends a CALL. mode is the mode
// it runs in.
func startedLikeAnImage(t *testing.T, c *console.Console, code []byte, mode vax.AccessMode) *corevms.Environment {
	t.Helper()

	env := handBuiltProcess(t, c, code)

	var psl vax.PSL

	psl.SetCurMod(mode)
	psl.SetPrvMod(mode)

	start := corevms.InitialPCB(env.Space, env.Stacks, codeAddr, psl)

	// The frame: condition handler, entry mask and PSW (a CALLG frame
	// saving no registers), AP, then FP and PC, the sentinel.
	frame := start.SP[mode] - 20
	for i, v := range []uint32{0, 0, 0, cpu.SentinelReturn, cpu.SentinelReturn} {
		setLongword(t, c, env, frame+uint32(4*i), v)
	}

	start.SP[mode] = frame
	start.R[vax.FP] = frame

	if err := cpu.WritePCB(c.Mem, env.Stacks.PCBB, &start); err != nil {
		t.Fatal(err)
	}

	return env
}

// countThenReturn: count to 200 at dataAddr, then return.
const countThenReturn = `
loop:	incl	@#^X600
	cmpl	@#^X600, #200
	blssu	loop
	ret
`

// countThenExit: count to 5, then $EXIT with status 1.
const countThenExit = `
loop:	incl	@#^X600
	cmpl	@#^X600, #5
	blssu	loop
	pushl	#1
	calls	#1, @#sys$exit
`

// TestSchedulerRun_otherImageEnds: process 1's CALL runs to its own end
// although process 2's image ends first (its main routine returns, or it
// calls $EXIT); process 2 stops and is never scheduled again. Both with
// the console's own run loop and the debugger's.
func TestSchedulerRun_otherImageEnds(t *testing.T) {
	for _, tt := range []struct {
		name     string
		program  string
		debugger bool
	}{
		{"return, console", countThenReturn, false},
		{"$EXIT, console", countThenExit, false},
		{"return, debugger", countThenReturn, true},
		{"$EXIT, debugger", countThenExit, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Process 1 counts to 200 and returns: a procedure the
			// console CALLs. Process 2 ends its image after 5 or 200.
			mainCode, _ := assembleAt(t, "\t.word\t0\n"+countThenReturn)
			otherCode, _ := assembleAt(t, tt.program)

			c, _ := scheduledConsole(t, "7", mainCode)
			one := c.RTL
			two := startedLikeAnImage(t, c, otherCode, vax.User)

			if tt.debugger {
				debugger.Install(c, consoletest.DebugGrammar(t), nil)
			}

			// The CALL runs process 1 in user mode (scheduledConsole set
			// the PSL), from the entry mask at codeAddr.
			if err := c.Call(codeAddr, false); err != nil {
				t.Fatalf("CALL: %v", err)
			}

			if n := countOf(t, c, one); n != 200 {
				t.Errorf("process 1 counted %d, want 200 (its CALL ended early)", n)
			}

			if !two.Stopped || c.RTL.Current() == two {
				t.Errorf("process 2: stopped %v, current %v", two.Stopped, c.RTL.Current() == two)
			}

			if _, ok := two.Scheduler().Info(0); ok {
				t.Error("handle 0 in the scheduler")
			}

			for _, h := range two.Scheduler().Handles() {
				if uint32(h) == two.Process.PID {
					t.Error("process 2 is still in the scheduler")
				}
			}
		})
	}
}

// TestSchedulerRun_haltInOtherProcess: a HALT in process 2 (in kernel
// mode, where HALT is allowed) stops the machine and the console's run,
// and the stop names the process.
func TestSchedulerRun_haltInOtherProcess(t *testing.T) {
	haltCode, _ := assembleAt(t, "\thalt\n")

	c, out := scheduledConsole(t, longQuantum, counter())
	two := startedLikeAnImage(t, c, haltCode, vax.Kernel)

	if err := c.Execute(nil); err != nil {
		t.Fatalf("GO: %v", err)
	}

	if c.RTL.Current() != two || !c.Engine.Halted() {
		t.Errorf("current is process 2: %v; halted: %v", c.RTL.Current() == two, c.Engine.Halted())
	}

	if !strings.Contains(out.String(), "cpu halted at PC = 00000401 in process 00000302") {
		t.Errorf("the HALT doesn't name process 2:\n%s", out.String())
	}
}

// Phase 44's subtask 8: the debugger with several processes.

// debuggedPair boots with the scheduler on and a short quantum, both
// processes running counter() at the same P0 address, with a debugger
// installed and a breakpoint at bp, and runs (GO) to the first stop.
func debuggedPair(t *testing.T, bp uint32) (*console.Console, *debugger.Debugger, *corevms.Environment) {
	t.Helper()

	c, _ := scheduledConsole(t, "5", counter())
	two := handBuiltProcess(t, c, counter())

	db := debugger.Install(c, consoletest.DebugGrammar(t), nil)
	db.AddBreakpoint(bp)

	if err := c.Execute(nil); err != nil {
		t.Fatalf("GO: %v", err)
	}

	return c, db, two
}

// TestSchedulerDebug_breakpointsAreProcessOnes: a breakpoint stops
// process 1 only. Process 2, running the same code at the same address,
// passes it without stopping; each GO stops in process 1 at the
// breakpoint, one count on. The cases cover the boundaries where a
// switch happens: on the counter's INCL, where process 1 is often
// switched back in (the check must see process 1 there, not the process
// it replaces), and with a switch forced at each GO's first boundary
// (process 1, back at the breakpoint it was stopped at, must not stop
// again before running it).
func TestSchedulerDebug_breakpointsAreProcessOnes(t *testing.T) {
	for _, tt := range []struct {
		name   string
		bp     uint32
		first  uint32 // process 1's count at the first stop (GO runs the first instruction unchecked)
		forced bool
	}{
		{"at the BRB", codeAddr + 6, 1, false},
		{"at the INCL", codeAddr, 1, false},
		{"at the BRB, switching at once", codeAddr + 6, 1, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, db, two := debuggedPair(t, tt.bp)
			one := c.RTL

			for i := range 30 {
				if c.RTL.Current() != one || c.CPU.GPR(vax.PC) != tt.bp {
					t.Fatalf("stop %d: process %08X at %08X, want process 1 at the breakpoint",
						i, c.RTL.Current().Process.PID, c.CPU.GPR(vax.PC))
				}

				if got, want := countOf(t, c, one), tt.first+uint32(i); got != want {
					t.Fatalf("stop %d: process 1 counted %d, want %d", i, got, want)
				}

				if tt.forced {
					one.Scheduler().RequestReschedule()
					c.Engine.RequestReschedule()
				}

				if err := db.Dispatch("GO"); err != nil {
					t.Fatalf("GO: %v", err)
				}
			}

			if countOf(t, c, two) < 10 {
				t.Errorf("process 2 counted only %d: it should have run past the breakpoint", countOf(t, c, two))
			}
		})
	}
}

// TestSchedulerDebug_stepFreezes: a STEP runs process 1 alone, however
// many quanta it takes; GO lets process 2 run again.
func TestSchedulerDebug_stepFreezes(t *testing.T) {
	c, db, two := debuggedPair(t, codeAddr+6)
	one := c.RTL

	before, oneBefore := countOf(t, c, two), countOf(t, c, one)

	if err := db.Dispatch("STEP 40"); err != nil {
		t.Fatalf("STEP: %v", err)
	}

	if c.RTL.Current() != one || countOf(t, c, two) != before {
		t.Errorf("during STEP 40: process 2 counted %d to %d; current is process 1: %v",
			before, countOf(t, c, two), c.RTL.Current() == one)
	}

	// (How far 40 steps take it depends on the debugger's step mode;
	// what matters is that it moved and process 2 didn't.)
	if countOf(t, c, one) == oneBefore {
		t.Error("process 1 didn't move in 40 steps")
	}

	for range 10 {
		if err := db.Dispatch("GO"); err != nil {
			t.Fatalf("GO: %v", err)
		}
	}

	if countOf(t, c, two) == before {
		t.Error("process 2 never ran after the STEP")
	}
}

// TestSchedulerDebug_everyInstruction: with a breakpoint on both of the
// counter's instructions, each GO runs exactly one instruction of
// process 1, so the stops alternate between them. Process 1 is switched
// out and back in at every kind of boundary, and each time the check
// before its next instruction must be made for it, not for the process
// it replaced (or a stop is missed, and the same address comes twice).
func TestSchedulerDebug_everyInstruction(t *testing.T) {
	c, db, two := debuggedPair(t, codeAddr)
	db.AddBreakpoint(codeAddr + 6)

	one := c.RTL
	last := c.CPU.GPR(vax.PC)

	for i := range 60 {
		if err := db.Dispatch("GO"); err != nil {
			t.Fatalf("GO: %v", err)
		}

		pc := c.CPU.GPR(vax.PC)
		if c.RTL.Current() != one || pc == last {
			t.Fatalf("stop %d: process %08X at %08X after %08X: want process 1, at the other breakpoint",
				i, c.RTL.Current().Process.PID, pc, last)
		}

		last = pc
	}

	if countOf(t, c, two) < 10 {
		t.Errorf("process 2 counted only %d", countOf(t, c, two))
	}
}
