package console

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// callBounded runs Console.Call's own logic with a hard step cap, so a
// milestone test against a real, uncontrolled program can't hang the test
// suite if it turns out to loop forever (e.g. waiting on device I/O this
// port doesn't simulate). Returns the terminating error (nil for a clean
// return/HALT) and whether the cap was hit.
func callBounded(t *testing.T, c *Console, addr uint32, maxSteps int) (err error, hitCap bool) {
	t.Helper()

	if err := c.Engine.CallEntry(addr); err != nil {
		return err, false
	}

	for i := 0; i < maxSteps; i++ {
		err := c.Engine.Step()
		if err == nil {
			continue
		}

		if errors.Is(err, cpu.ErrConsoleCallReturned) || errors.Is(err, cpu.ErrHalted) {
			return nil, false
		}

		return err, false
	}

	return nil, true
}

func TestDefaultRunInits(t *testing.T) {
	c := newRunnableConsole(t)
	if !c.DefaultRunInits() {
		t.Error("DefaultRunInits() = false, want true (DebugLibinit is on by default)")
	}

	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugLibinit)
	
	if c.DefaultRunInits() {
		t.Error("DefaultRunInits() = true, want false once DebugLibinit is cleared")
	}
}

func TestParseRunQualifier_defaultAndOverride(t *testing.T) {
	opts, rest := parseRunQualifier("foo.exe", true)
	if !opts.RunInits || rest != "foo.exe" {
		t.Errorf("parseRunQualifier(no qualifier, default=true) = %+v, %q, want RunInits=true", opts, rest)
	}

	opts, rest = parseRunQualifier("/NOINIT foo.exe", true)
	if opts.RunInits || rest != " foo.exe" {
		t.Errorf("parseRunQualifier(/NOINIT, default=true) = %+v, %q, want RunInits=false", opts, rest)
	}

	opts, rest = parseRunQualifier("/INIT foo.exe", false)
	if !opts.RunInits || rest != " foo.exe" {
		t.Errorf("parseRunQualifier(/INIT, default=false) = %+v, %q, want RunInits=true", opts, rest)
	}
}

func TestRun_debugImagesTrace(t *testing.T) {
	c := newRunnableConsole(t)
	c.CPU.SetDebug(vax.DebugImages)

	if err := c.Run(exeFixturePath(t, "simple.exe"), RunOptions{NoExecute: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := c.Out.(*bytes.Buffer).String()
	if !strings.Contains(out, "Main image is") {
		t.Errorf("output = %q, want a \"Main image is\" trace", out)
	}
}

// TestRun_noExecuteLoadsAndFixesUpOnly exercises Console.Run's actual
// public signature end-to-end (load, fixup, driver build) without running
// the CPU at all, matching /NOEXECUTE.
func TestRun_noExecuteLoadsAndFixesUpOnly(t *testing.T) {
	c := newRunnableConsole(t)
	if err := c.Run(exeFixturePath(t, "simple.exe"), RunOptions{NoExecute: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(c.ICBList) == 0 {
		t.Fatal("expected Run to have loaded at least the main image")
	}

	if c.ICBList[0].Flags&icbFixed == 0 {
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
			c := newRunnableConsole(t)

			if err := c.ensureShims(); err != nil {
				t.Fatalf("ensureShims: %v", err)
			}

			mainICB, err := c.imageLoad(exeFixturePath(t, name), icbMain)
			if err != nil {
				t.Fatalf("imageLoad(%s): %v", name, err)
			}

			for _, dep := range c.ICBList {
				if err := c.imageFixup(dep); err != nil {
					t.Fatalf("imageFixup(%s dependency %s): %v", name, dep.Name, err)
				}
			}

			driverAddr, ok, err := c.buildImageInitDriver(mainICB, false)
			if err != nil {
				t.Fatalf("buildImageInitDriver(%s): %v", name, err)
			}

			if !ok {
				t.Fatalf("%s: no usable transfer address", name)
			}

			runErr, hitCap := callBounded(t, c, driverAddr, 2_000_000)
			if hitCap {
				t.Errorf("%s: did not reach a HALT/return within 2,000,000 steps", name)
			}
			
			t.Logf("%s: terminating outcome: %v", name, runErr)
		})
	}
}
