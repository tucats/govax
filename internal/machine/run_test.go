package machine

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// TestRun_noExecuteLoadsAndFixesUpOnly exercises Console.Run's actual
// public signature end-to-end (load, fixup, driver build) without running
// the CPU at all, matching /NOEXECUTE.
func TestRun_noExecuteLoadsAndFixesUpOnly(t *testing.T) {
	c := newRunnableMachine(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.Run(exeFixturePath(t, "simple.exe"), RunOptions{NoExecute: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(c.images().ICBList) == 0 {
		t.Fatal("expected Run to have loaded at least the main image")
	}

	if c.images().ICBList[0].Flags&icbFixed == 0 {
		t.Error("expected the main image to be fixed up")
	}
}

// TestRun_everyMilestoneFixture is Phase 12/13's own named milestone check
// (docs/PHASE-13.md's Deliverables): put.exe, putc.exe, cli.exe, sieve.exe,
// and simple.exe must load and run "to whatever extent their actual code
// paths are covered by Phase 10's RTL surface -- a program calling an
// unregistered SHIM$/SYS$ entry is expected to report that clearly, not
// silently misbehave" -- so this doesn't demand a clean exit from every
// fixture, only that Run reaches a definite, reported outcome (a clean
// return/HALT, or a clear error) rather than hanging or panicking.
func TestRun_everyMilestoneFixture(t *testing.T) {
	for _, name := range []string{"put.exe", "putc.exe", "cli.exe", "sieve.exe", "simple.exe"} {
		t.Run(name, func(t *testing.T) {
			c := newBootableMachine(t)

			if _, _, err := c.Assemble(kernelPath(t)); err != nil {
				t.Fatalf("Assemble(kernel.asm): %v", err)
			}

			runKernelInitialize(t, c)

			if err := c.EnsureShims(); err != nil {
				t.Fatalf("ensureShims: %v", err)
			}

			mainICB, err := c.imageLoad(exeFixturePath(t, name), icbMain)
			if err != nil {
				t.Fatalf("imageLoad(%s): %v", name, err)
			}

			for _, dep := range c.images().ICBList {
				if err := c.imageFixup(dep); err != nil {
					t.Fatalf("imageFixup(%s dependency %s): %v", name, dep.Name, err)
				}
			}

			driverAddr, ok, err := c.BuildImageInitDriver(mainICB, false)
			if err != nil {
				t.Fatalf("BuildImageInitDriver(%s): %v", name, err)
			}

			if !ok {
				t.Fatalf("%s: no usable transfer address", name)
			}

			// sieve.exe is a genuine CPU-bound benchmark (sieve of
			// Eratosthenes up to 100,000) now that it actually runs its
			// real code (see docs/PHASE-20.md's progress log) rather than
			// faulting immediately as it did before that fix -- it needs
			// far more steps than the other, effectively-instant fixtures,
			// so it gets its own, much larger budget rather than raising
			// every fixture's cap to match.
			maxSteps := 2_000_000
			if name == "sieve.exe" {
				maxSteps = 60_000_000
			}

			runErr, hitCap := callBounded(t, c, driverAddr, maxSteps)
			if hitCap {
				t.Errorf("%s: did not reach a HALT/return within %d steps", name, maxSteps)
			}

			t.Logf("%s: terminating outcome: %v", name, runErr)
		})
	}
}

// TestRun_driverCallsExitHandlers: RUN's image driver calls $EXIT with
// the status main returns, as VMS's image activator does, so an image
// that just returns still has its exit handlers called
// (docs/PHASE-26.md subtask 19).
func TestRun_driverCallsExitHandlers(t *testing.T) {
	c := newBootableMachine(t)

	addr, _, err := c.Assemble(asmFixturePath(t, "exit_on_return.asm"))
	if err != nil {
		t.Fatalf("Assemble(exit_on_return.asm): %v", err)
	}

	driver, ok, err := c.BuildImageInitDriver(&ICB{Name: "EXITRET", Transfer: [4]uint32{addr}}, false)
	if err != nil || !ok {
		t.Fatalf("BuildImageInitDriver: ok=%v err=%v", ok, err)
	}

	c.Engine.SetModeStack(vax.User, false)

	if runErr, hitCap := callBounded(t, c, driver, 100_000); runErr != nil || hitCap {
		t.Fatalf("running the driver: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 7 {
		t.Errorf("R0 = %d, want main's status 7", got)
	}

	seen, ok := c.Symbols.Get("SEEN")
	if !ok {
		t.Fatal("no SEEN symbol")
	}

	if got, err := c.Mem.LoadLongword(c.CPU, seen); err != nil || got != 7 {
		t.Errorf("the exit handler saw status %d (%v), want 7: it wasn't called", got, err)
	}
}
