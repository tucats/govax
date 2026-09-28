package console

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
)

// TestProcessServices_assembledProgram is docs/PHASE-26.md's acceptance
// test for its first batch of services: testdata/asm/process_services.asm,
// assembled and run as real VAX code, calls $ADJSTK, $ADJWSL, $ALLOC, and
// $ASCEFC (with $SETEF/$READEF on the associated cluster) through their
// real P1-vector addresses. The program checks each result itself and
// leaves 1 in R0 only if all of them worked; this test then checks the
// emulated process and device state they left behind.
func TestProcessServices_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	tta0 := c.DefineDevice("TTA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassTT})

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "process_services.asm"))
	if err != nil {
		t.Fatalf("Assemble(process_services.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("process_services.asm has no entry address")
	}

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running process_services.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("process_services.asm didn't finish within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every service call worked)", got)
	}

	if got := c.CPU.PR(vax.USP); got != 0x4FF8 {
		t.Errorf("USP = %#x, want 0x4FF8 from $ADJSTK", got)
	}

	p := c.RTL.Process
	if p.WSLimit != p.WSDefault+10 {
		t.Errorf("WSLimit = %d, want WSDEFAULT+10 from $ADJWSL", p.WSLimit)
	}

	if !tta0.Allocated() || tta0.PID != p.PID {
		t.Errorf("TTA0 allocated=%v PID=%#x, want allocated to %#x", tta0.Allocated(), tta0.PID, p.PID)
	}

	cl := p.CommonClusters[0]
	if cl == nil || cl.Name != "CLUSTER" || cl.Flags != 2 {
		t.Errorf("common cluster 2 = %+v, want CLUSTER with flag 65 set", cl)
	}
}

// TestEventFlagWait_timerInterrupt is docs/PHASE-26.md subtask 9's
// acceptance test: testdata/asm/wait_timer.asm waits in $WAITFR for an
// event flag that only its interval-timer interrupt handler sets. The
// wait re-executes the service's XFC each step, the timer interrupt is
// taken in between, and its handler's $SETEF ends the wait.
func TestEventFlagWait_timerInterrupt(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "wait_timer.asm"))
	if err != nil {
		t.Fatalf("Assemble(wait_timer.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("wait_timer.asm has no entry address")
	}

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running wait_timer.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("wait_timer.asm didn't finish within 100,000 steps (the wait never ended)")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (the timer interrupt ended the wait)", got)
	}
}

// TestEventFlagWait_blocksUntilSet: a $WAITFR on a flag nothing sets keeps
// the program in the wait (PC on the service's XFC) for as many steps as
// it's given; setting the flag from outside then lets it finish.
func TestEventFlagWait_blocksUntilSet(t *testing.T) {
	c := newBootableConsole(t)

	addr, _, err := c.Assemble(asmFixturePath(t, "wait_timer.asm"))
	if err != nil {
		t.Fatalf("Assemble(wait_timer.asm): %v", err)
	}

	// Keep the timer from ever interrupting: ICCS is cleared after every
	// step, so nothing asynchronous can set the flag.
	if err := c.Engine.CallEntry(addr); err != nil {
		t.Fatal(err)
	}

	waitfr := uint32(0)

	for i := 0; i < 5_000; i++ {
		if err := c.Engine.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}

		c.CPU.SetPR(vax.ICCS, 0)

		if pc := c.CPU.GPR(vax.PC); pc >= 0x7FFE0000 {
			waitfr = pc
		}
	}

	if waitfr == 0 || c.CPU.GPR(vax.PC) != waitfr {
		t.Fatalf("PC = %#x after 5,000 steps, want the program parked in $WAITFR's P1-vector stub", c.CPU.GPR(vax.PC))
	}

	c.RTL.Process.LocalEventFlags[0] |= 1 << 3

	runErr, hitCap := continueBounded(c, 1_000)
	if runErr != nil || hitCap {
		t.Fatalf("after setting flag 3: err=%v hitCap=%v, want the program to finish", runErr, hitCap)
	}

	// The program notices no tick ran, so it reports failure -- which
	// confirms the wait really was ended from outside.
	if got := c.CPU.GPR(vax.R0); got != 0 {
		t.Errorf("R0 = %d, want 0 (no timer tick ended this wait)", got)
	}
}

// continueBounded steps an already started console call to completion,
// like callBounded without the CallEntry.
func continueBounded(c *Console, maxSteps int) (err error, hitCap bool) {
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

// TestTimerServices_assembledProgram is docs/PHASE-26.md subtask 11's
// acceptance test: testdata/asm/timer_services.asm sets, cancels, and
// waits on $SETIMR timers without any guest interrupt setup. The wait
// lasts until the engine's system time (one millisecond per interval-
// clock tick, deterministic in quantum mode) has advanced 50ms.
func TestTimerServices_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "timer_services.asm"))
	if err != nil {
		t.Fatalf("Assemble(timer_services.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("timer_services.asm has no entry address")
	}

	start := c.Engine.SystemTime()

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running timer_services.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("timer_services.asm didn't finish within 100,000 steps (the timer never fired)")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (the timer ended the wait; the cancelled one never fired)", got)
	}

	if elapsed := c.Engine.SystemTime() - start; elapsed < 50*10_000 {
		t.Errorf("system time advanced %d, want at least 50ms (500000): the wait ended early", elapsed)
	}
}

// TestHibernate_assembledProgram is docs/PHASE-26.md subtask 14's
// acceptance test: testdata/asm/hibernate.asm hibernates on an
// already-pending $WAKE, then three times on a repeating $SCHDWK wakeup,
// and cancels it. Each $HIBER that must sleep re-executes its XFC until
// the engine's system time reaches the next wakeup.
func TestHibernate_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "hibernate.asm"))
	if err != nil {
		t.Fatalf("Assemble(hibernate.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("hibernate.asm has no entry address")
	}

	start := c.Engine.SystemTime()

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running hibernate.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("hibernate.asm didn't finish within 100,000 steps (a $HIBER never woke)")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every service call worked)", got)
	}

	if elapsed := c.Engine.SystemTime() - start; elapsed < 50*10_000 {
		t.Errorf("system time advanced %d, want at least 50ms (500000): a $HIBER returned early", elapsed)
	}

	if n := c.RTL.PendingTimers(); n != 0 {
		t.Errorf("%d requests still queued, want 0 after $CANWAK", n)
	}
}

// TestASTDelivery_assembledProgram is docs/PHASE-26.md subtask 15's
// acceptance test: testdata/asm/ast_delivery.asm declares ASTs with
// $DCLAST, holds one back with $SETAST, and checks each ran when it
// should and left R0, R1, and R2 as they were. This test also checks the
// routine saw five arguments and that no AST is left queued or active.
func TestASTDelivery_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "ast_delivery.asm"))
	if err != nil {
		t.Fatalf("Assemble(ast_delivery.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("ast_delivery.asm has no entry address")
	}

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running ast_delivery.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("ast_delivery.asm didn't finish within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every AST ran when it should, registers intact)", got)
	}

	argcount, ok := c.Symbols.Get("ARGCOUNT")
	if !ok {
		t.Fatal("no ARGCOUNT symbol")
	}

	if n, err := c.Mem.LoadLongword(c.CPU, argcount); err != nil || n != 5 {
		t.Errorf("the AST routine's argument count = %d (%v), want 5", n, err)
	}

	if n := c.RTL.PendingASTs(); n != 0 {
		t.Errorf("%d ASTs still queued, want 0", n)
	}
}

// TestTimerAST_assembledProgram is docs/PHASE-26.md subtask 16's
// acceptance test: testdata/asm/timer_ast.asm uses ASTs from $SETIMR
// (ending a $HIBER through the AST routine's $WAKE, cancelled by $CANTIM,
// and interrupting a loop that calls no services) and from $GETJPIW.
func TestTimerAST_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "timer_ast.asm"))
	if err != nil {
		t.Fatalf("Assemble(timer_ast.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("timer_ast.asm has no entry address")
	}

	start := c.Engine.SystemTime()

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running timer_ast.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("timer_ast.asm didn't finish within 100,000 steps (an AST never arrived)")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every AST arrived as it should)", got)
	}

	// 30ms ($HIBER) + 30ms ($WAITFR) + 10ms (the loop).
	if elapsed := c.Engine.SystemTime() - start; elapsed < 70*10_000 {
		t.Errorf("system time advanced %d, want at least 70ms (700000)", elapsed)
	}

	pid, ok := c.Symbols.Get("PID")
	if !ok {
		t.Fatal("no PID symbol")
	}

	if got, err := c.Mem.LoadLongword(c.CPU, pid); err != nil || got != c.RTL.Process.PID {
		t.Errorf("$GETJPIW's JPI$_PID = %#x (%v), want %#x", got, err, c.RTL.Process.PID)
	}

	if n, m := c.RTL.PendingASTs(), c.RTL.PendingTimers(); n != 0 || m != 0 {
		t.Errorf("%d ASTs and %d timers left, want none", n, m)
	}
}

// TestTerminalQIO_assembledProgram is docs/PHASE-26.md subtask 17's
// acceptance test: testdata/asm/terminal_qio.asm reads a prompted line
// and writes to the terminal with $QIO/$QIOW. The console's input is the
// typed line; its output must show the prompt, the carriage-controlled
// greeting, and the name read back.
func TestTerminalQIO_assembledProgram(t *testing.T) {
	var out bytes.Buffer

	c := New(&out)
	c.In = strings.NewReader("Tom\n")

	if err := c.Init(8192 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(2048, 8192, 2048, 20, 0, 0, 0, 0); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	c.DefineDevice("TTA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassTT})

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "terminal_qio.asm"))
	if err != nil {
		t.Fatalf("Assemble(terminal_qio.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("terminal_qio.asm has no entry address")
	}

	out.Reset() // just the program's own output

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running terminal_qio.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("terminal_qio.asm didn't finish within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every $QIO did what it should)", got)
	}

	if want := "Name? \nHello, \rTom"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}

	if n := c.RTL.PendingASTs(); n != 0 {
		t.Errorf("%d ASTs left, want none", n)
	}
}

// TestSynch_assembledProgram is docs/PHASE-26.md subtask 18's acceptance
// test: testdata/asm/synch.asm's $SYNCH ignores a timer's "false alarm"
// on a shared event flag (its IOSB is still 0 at 10ms) and returns only
// when the stand-in request's AST writes the IOSB, at 30ms.
func TestSynch_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "synch.asm"))
	if err != nil {
		t.Fatalf("Assemble(synch.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("synch.asm has no entry address")
	}

	start := c.Engine.SystemTime()

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running synch.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("synch.asm didn't finish within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every $SYNCH returned when it should)", got)
	}

	// $SYNCH must not have returned at the 10ms false alarm.
	if elapsed := c.Engine.SystemTime() - start; elapsed < 30*10_000 {
		t.Errorf("system time advanced %d, want at least 30ms (300000)", elapsed)
	}

	if n, m := c.RTL.PendingASTs(), c.RTL.PendingTimers(); n != 0 || m != 0 {
		t.Errorf("%d ASTs and %d timers left, want none", n, m)
	}
}

// TestExitHandlers_assembledProgram is docs/PHASE-26.md subtask 19's
// acceptance test: testdata/asm/exit_handlers.asm, run in user mode,
// declares exit handlers, cancels one, and calls $EXIT, which calls the
// others newest first and then ends the image, returning to the console
// with the exit status in R0.
func TestExitHandlers_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "exit_handlers.asm"))
	if err != nil {
		t.Fatalf("Assemble(exit_handlers.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("exit_handlers.asm has no entry address")
	}

	// Run it as a user-mode image, on the user stack VMInit set up.
	c.Engine.SetModeStack(vax.User, false)

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running exit_handlers.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("exit_handlers.asm didn't finish within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 0x2C {
		t.Errorf("R0 = %#x, want the exit status 0x2C", got)
	}

	word := func(name string) uint32 {
		t.Helper()

		a, ok := c.Symbols.Get(name)
		if !ok {
			t.Fatalf("no %s symbol", name)
		}

		v, err := c.Mem.LoadLongword(c.CPU, a)
		if err != nil {
			t.Fatal(err)
		}

		return v
	}

	if o1, o2 := word("ORDER1"), word("ORDER2"); o2 != 1 || o1 != 2 {
		t.Errorf("handler order: HANDLER1 ran %d, HANDLER2 ran %d; want HANDLER2 first (1), then HANDLER1 (2)", o1, o2)
	}

	if s1, s2 := word("STATUS1"), word("STATUS2"); s1 != 0x2C || s2 != 0x2C {
		t.Errorf("statuses the handlers saw: %#x, %#x; want 0x2C", s1, s2)
	}

	if word("RAN3") != 0 {
		t.Error("the cancelled handler ran")
	}

	if word("RETURNED") != 0 {
		t.Error("$EXIT returned to its caller")
	}

	if n := c.RTL.ExitHandlers(vax.User); n != 0 || c.RTL.Process.ExitStatus != 0x2C {
		t.Errorf("%d user handlers left, ExitStatus %#x; want none and 0x2C", n, c.RTL.Process.ExitStatus)
	}
}
