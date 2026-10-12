package machine

import (
	"bytes"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// Test helpers shared by the machine's tests: machines ready to run
// programs, the paths of the kernel and the fixtures, and running a
// routine or an image a bounded number of steps. (internal/console's
// tests have their own, making consoles.)

func newTestMachine(t *testing.T) (*Machine, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	c := NewMachine(&buf)
	if err := c.Init(64 * 1024); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return c, &buf
}

// newBootableMachine is newRunnableMachine's own real-size counterpart,
// matching vax.init's actual `init ^d8192` / `vminit /p0=2048 /p1=8192
// /s0=2048 /ksp=20` boot parameters exactly rather than newRunnableMachine's
// own much smaller, lighter-weight sizing (chosen there purely for the
// image-load/fixup-only tests that share it, none of which ever run real
// user-mode code far enough to touch the top of a conventionally-sized P1
// stack). runKernelInitialize's own REI down to user mode does touch it
// (kernel.asm's real initial USP), so a test that calls both needs this
// larger console, not newRunnableMachine -- a small P1 leaves that address
// unmapped, faulting with an access violation before EXE$INITIALIZE ever
// reaches its own silent halt.
func newBootableMachine(t testing.TB) *Machine {
	t.Helper()

	c := NewMachine(&bytes.Buffer{})
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
func runKernelInitialize(t *testing.T, c *Machine) {
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

		if err.Error() == cpu.ErrHaltedSilent.Error() {
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
func callBounded(t *testing.T, c *Machine, addr uint32, maxSteps int) (err error, hitCap bool) {
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

// newRunnableMachine returns a Console with enough physical/virtual memory
// set up (INIT + VMINIT) and running in kernel mode to exercise imageLoad,
// matching console_run's own "switch to kernel mode" precondition. p1Pages
// (200) is bigger than this helper's own lightweight sizing used to be
// (100): most of its callers assemble kernel.asm, whose ".p1vector"
// statement (internal/asm/pseudo.go's pseudoP1Vector, no longer a no-op)
// deposits real trampolines across internal/vmsdef.P1VectorTable's fixed
// addresses, the lowest of which (0x7FFEDE00) sits 145 pages below the top
// of P1 space — below 145 pages, VMInit's own demand-paged P1 region
// (grows down from maxP1) doesn't reach that low, and depositing there
// access-violates. 200 leaves headroom rather than sizing exactly to the
// boundary.
func newRunnableMachine(t testing.TB) *Machine {
	t.Helper()

	c := NewMachine(&bytes.Buffer{})
	if err := c.Init(4096 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(2000, 200, 0, 4, 4, 4, 4, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	c.Debugger = &stepDebugger{c: c}

	return c
}

func exeFixturePath(t *testing.T, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "exe", name)
}

// kernelPath is the microkernel tests assemble: bootdata's kernel.asm, the
// one govax itself boots. (testdata/asm once held an older copy, without
// the arithmetic and translation-not-valid vectors; it was removed so no
// test can run against a kernel govax doesn't use.)
func kernelPath(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "bootdata", "files", "kernel.asm")
}

func asmFixturePath(t testing.TB, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "asm", name)
}

// dbgImagePath is a path to one of testdata/dbg/vax's images: Phase 41's
// probe, linked by real LINK (DBGDIS) and by govax (GVDBGDIS).
func dbgImagePath(t *testing.T, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "dbg", "vax", name)
}

// runStepped loads the image at path as RUN/STEP does, leaving it stopped
// at its first instruction, and clears the output so far.
func runStepped(t *testing.T, path string) (*Machine, *bytes.Buffer) {
	t.Helper()

	c := newRunnableMachine(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.Run(path, RunOptions{Debug: DebugOn}); err != nil {
		t.Fatalf("RUN/STEP %s: %v", filepath.Base(path), err)
	}

	buf := c.Hooks.Output().(*bytes.Buffer)
	buf.Reset()

	return c, buf
}
