package console

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// newBootableConsole is newRunnableConsole's own real-size counterpart,
// matching vax.init's actual `init ^d8192` / `vminit /p0=2048 /p1=8192
// /s0=2048 /ksp=20` boot parameters exactly rather than newRunnableConsole's
// own much smaller, lighter-weight sizing (chosen there purely for the
// image-load/fixup-only tests that share it, none of which ever run real
// user-mode code far enough to touch the top of a conventionally-sized P1
// stack). runKernelInitialize's own REI down to user mode does touch it
// (kernel.asm's real initial USP), so a test that calls both needs this
// larger console, not newRunnableConsole -- a small P1 leaves that address
// unmapped, faulting with an access violation before EXE$INITIALIZE ever
// reaches its own silent halt.
func newBootableConsole(t testing.TB) *Console {
	t.Helper()

	c := New(&bytes.Buffer{})
	if err := c.Init(8192 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(2048, 8192, 2048, 20, 0, 0, 0, 0); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	return c
}

// runKernelInitialize runs kernel.asm's own EXE$INITIALIZE routine to
// completion, matching vax.init's own "go exe$initialize" boot step: it
// enables the TXCS/RXCS console-device interrupt-on-ready bits and the
// interval timer, then fakes a REI down to user mode at IPL 0 "so
// interrupts will start happening" (kernel.asm's own comment) before
// halting silently. A test that runs a real .exe fixture touching
// kernel.asm's own hand-written, interrupt-driven LIB$PUT_OUTPUT (not the
// Go-native DECC$SHR print shims, which need none of this) hangs forever
// without it -- IPL stays wherever VMInit left it, above the console
// device's own IPL, so the TXCS-ready interrupt LIB$PUT_OUTPUT's per-
// character busy-wait depends on to advance past the first character can
// never be delivered. Entered via a raw PC set + Step loop, not
// Console.Call/Engine.CallEntry -- kernel.asm's own comment on this exact
// routine warns "you must GO this code, not CALL it", since (unlike a real
// `.ENTRY` procedure) it carries no register-save entry mask word for
// CallEntry to (mis)read as one.
func runKernelInitialize(t *testing.T, c *Console) {
	t.Helper()

	addr, ok := c.Symbols.Get("EXE$INITIALIZE")
	if !ok {
		t.Fatal("expected kernel.asm to define EXE$INITIALIZE")
	}

	c.CPU.SetGPR(vax.PC, addr)
	c.Engine.BeginRun()

	for i := 0; i < 100_000; i++ {
		err := c.Engine.Step()
		if err == nil {
			continue
		}

		if errors.Is(err, cpu.ErrHalted) {
			return
		}

		t.Fatalf("EXE$INITIALIZE: %v", err)
	}

	t.Fatal("EXE$INITIALIZE did not halt within 100,000 steps")
}

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

	if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

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

	if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

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
			c := newBootableConsole(t)

			if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
				t.Fatalf("Assemble(kernel.asm): %v", err)
			}

			runKernelInitialize(t, c)

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
