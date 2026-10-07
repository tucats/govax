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
