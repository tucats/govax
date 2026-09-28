package console

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/rtl"

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

// TestNumtim_assembledProgram is docs/PHASE-26.md subtask 20's
// acceptance test: testdata/asm/numtim.asm converts an absolute and a
// delta time with $BINTIM, then breaks each down with $NUMTIM.
func TestNumtim_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "numtim.asm"))
	if err != nil {
		t.Fatalf("Assemble(numtim.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("numtim.asm has no entry address")
	}

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running numtim.asm: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %d, want 1 (every conversion succeeded)", got)
	}

	fields := func(name string) [7]uint16 {
		t.Helper()

		a, ok := c.Symbols.Get(name)
		if !ok {
			t.Fatalf("no %s symbol", name)
		}

		var out [7]uint16

		for i := range out {
			w, err := c.Mem.LoadWord(c.CPU, a+uint32(2*i))
			if err != nil {
				t.Fatal(err)
			}

			out[i] = w
		}

		return out
	}

	if got, want := fields("ABSFLD"), [7]uint16{2000, 2, 29, 23, 59, 59, 99}; got != want {
		t.Errorf("absolute time fields = %v, want %v", got, want)
	}

	if got, want := fields("DELFLD"), [7]uint16{0, 0, 5, 3, 18, 32, 7}; got != want {
		t.Errorf("delta time fields = %v, want %v", got, want)
	}
}

// TestJPIASTState_assembledProgram is docs/PHASE-26.md subtask 21's
// acceptance test: testdata/asm/jpi_ast_state.asm reads $GETJPI's AST
// and state items normally, with ASTs disabled, from inside an AST
// routine, and with a timer AST outstanding.
func TestJPIASTState_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "jpi_ast_state.asm"))
	if err != nil {
		t.Fatalf("Assemble(jpi_ast_state.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("jpi_ast_state.asm has no entry address")
	}

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running jpi_ast_state.asm: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %d, want 1 (every call succeeded)", got)
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

	checks := []struct {
		name string
		want uint32
		why  string
	}{
		{"ASTEN", 0xF, "every mode enabled"},
		{"ASTACT", 0, "no AST running"},
		{"ASTCNT", 24, "the whole quota"},
		{"STATE", 14, "SCH$C_CUR"},
		{"ASTENOFF", 0xE, "kernel disabled by $SETAST(0)"},
		{"ASTACTIN", 0x1, "a kernel AST running"},
		{"ASTCNTTMR", 23, "one timer AST outstanding"},
	}

	for _, ch := range checks {
		if got := word(ch.name); got != ch.want {
			t.Errorf("%s = %#x, want %#x (%s)", ch.name, got, ch.want, ch.why)
		}
	}
}

// TestModeSwitchAST_assembledProgram is docs/PHASE-26.md subtask 22's
// acceptance test: testdata/asm/mode_switch_ast.asm sets a timer from
// kernel mode, drops to user mode, and hibernates. The timer's kernel-mode
// AST is delivered by switching into kernel mode (with memory protection
// on, on the kernel stack), wakes the process, and returns to user mode.
func TestModeSwitchAST_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "mode_switch_ast.asm"))
	if err != nil {
		t.Fatalf("Assemble(mode_switch_ast.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("mode_switch_ast.asm has no entry address")
	}

	start := c.Engine.SystemTime()

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running mode_switch_ast.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("mode_switch_ast.asm didn't finish within 100,000 steps (the kernel AST never ended the user-mode $HIBER)")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1", got)
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

	if m, p := word("KMODE"), word("KPRV"); m != uint32(vax.Kernel) || p != uint32(vax.User) {
		t.Errorf("the AST ran in mode %d with previous mode %d; want kernel (0), user (3)", m, p)
	}

	if got := word("KPARAM"); got != 7 {
		t.Errorf("AST parameter = %d, want 7", got)
	}

	if got := word("UMODE"); got != uint32(vax.User) {
		t.Errorf("mode after the AST = %d, want user (3)", got)
	}

	if elapsed := c.Engine.SystemTime() - start; elapsed < 10*10_000 {
		t.Errorf("system time advanced %d, want at least 10ms (100000)", elapsed)
	}

	if n := c.RTL.PendingASTs(); n != 0 {
		t.Errorf("%d ASTs left, want none", n)
	}
}

// TestGetsyi_assembledProgram is docs/PHASE-26.md subtask 23's
// acceptance test: testdata/asm/getsyi.asm asks $GETSYIW for the VMS
// version and node name, then walks the cluster with a wildcard scan,
// finding one node.
func TestGetsyi_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "getsyi.asm"))
	if err != nil {
		t.Fatalf("Assemble(getsyi.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("getsyi.asm has no entry address")
	}

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running getsyi.asm: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %d, want 1 (every call behaved)", got)
	}

	read := func(name string, n int) string {
		t.Helper()

		a, ok := c.Symbols.Get(name)
		if !ok {
			t.Fatalf("no %s symbol", name)
		}

		buf := make([]byte, n)
		if err := c.Mem.Load(c.CPU, a, buf); err != nil {
			t.Fatal(err)
		}

		return string(buf)
	}

	if got := read("VERSION", 8); got != "V7.3    " {
		t.Errorf("SYI$_VERSION = %q, want \"V7.3    \"", got)
	}

	namlen := read("NAMLEN", 2)
	n := int(namlen[0]) | int(namlen[1])<<8

	if got := read("NODE", n); got != c.RTL.NodeName {
		t.Errorf("SYI$_NODENAME = %q (length %d), want %q", got, n, c.RTL.NodeName)
	}

	if nodes := read("NODES", 4); nodes[0] != 1 {
		t.Errorf("the wildcard scan visited %d nodes, want 1", nodes[0])
	}
}

// TestFAO_assembledProgram is docs/PHASE-26.md subtask 24's acceptance
// test: testdata/asm/fao.asm formats text with $FAO (parameters in the
// call) and $FAOL (parameters in a list), and writes each result to the
// terminal with $QIOW.
func TestFAO_assembledProgram(t *testing.T) {
	var out bytes.Buffer

	c := New(&out)

	if err := c.Init(8192 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(2048, 8192, 2048, 20, 0, 0, 0, 0); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	c.DefineDevice("TTA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassTT})

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "fao.asm"))
	if err != nil {
		t.Fatalf("Assemble(fao.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("fao.asm has no entry address")
	}

	out.Reset() // just the program's own output

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running fao.asm: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every call behaved)", got)
	}

	if want := "SYSTEM has 3 files at 000001F4\r\nBYTES:   1  22 255\r\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// TestPutmsg_assembledProgram is docs/PHASE-26.md subtask 25's
// acceptance test: testdata/asm/putmsg.asm reads a message with $GETMSG
// and writes message vectors with $PUTMSG, one through an action routine
// that lets only the first line be written.
func TestPutmsg_assembledProgram(t *testing.T) {
	var out bytes.Buffer

	c := New(&out)

	if err := c.Init(8192 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(2048, 8192, 2048, 20, 0, 0, 0, 0); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "putmsg.asm"))
	if err != nil {
		t.Fatalf("Assemble(putmsg.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("putmsg.asm has no entry address")
	}

	out.Reset()

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running putmsg.asm: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every call behaved)", got)
	}

	want := "%SYSTEM-F-ABORT, abort\n" +
		"%SYSTEM-F-ACCVIO, access violation, reason mask=04, virtual address=00000200, PC=00000300, PS=0000001B\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}

	a, ok := c.Symbols.Get("LINLEN")
	if !ok {
		t.Fatal("no LINLEN symbol")
	}

	if n, _ := c.Mem.LoadLongword(c.CPU, a); n != uint32(len("%SYSTEM-F-ABORT, abort")) {
		t.Errorf("the action routine saw a first line of %d characters, want %d", n, len("%SYSTEM-F-ABORT, abort"))
	}
}

// TestCmkrnl_assembledProgram is docs/PHASE-26.md subtask 26's acceptance
// test: testdata/asm/cmkrnl.asm, running in user mode with memory
// management on, calls routines of its own in kernel mode ($CMKRNL, one
// executing a privileged MFPR) and executive mode ($CMEXEC), and gets
// each routine's status back.
func TestCmkrnl_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "cmkrnl.asm"))
	if err != nil {
		t.Fatalf("Assemble(cmkrnl.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("cmkrnl.asm has no entry address")
	}

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running cmkrnl.asm: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (both services returned their routine's status)", got)
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

	if m, p := word("KMODE"), word("KPRV"); m != uint32(vax.Kernel) || p != uint32(vax.User) {
		t.Errorf("KRNL ran in mode %d with previous mode %d; want kernel (0), user (3)", m, p)
	}

	if got := word("KIPL"); got != 0 {
		t.Errorf("KRNL's MFPR read IPL %d, want 0", got)
	}

	if got := word("EMODE"); got != uint32(vax.Executive) {
		t.Errorf("EXEC ran in mode %d, want executive (1)", got)
	}

	if got := word("UMODE"); got != uint32(vax.User) {
		t.Errorf("mode afterwards = %d, want user (3)", got)
	}
}

// TestCtrlCAST_assembledProgram is docs/PHASE-26.md subtask 27's
// acceptance test: testdata/asm/ctrlc_ast.asm enables a CTRL/C AST and
// spins until it runs, then a CTRL/Y AST. The test "types" CTRL/C
// (Engine.Attention, as the host keyboard handler does) each time the
// program reaches a spin loop; instead of stopping the machine, each
// key is delivered as the program's AST.
func TestCtrlCAST_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	c.DefineDevice("TTA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassTT})

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "ctrlc_ast.asm"))
	if err != nil {
		t.Fatalf("Assemble(ctrlc_ast.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("ctrlc_ast.asm has no entry address")
	}

	symbol := func(name string) uint32 {
		t.Helper()

		a, ok := c.Symbols.Get(name)
		if !ok {
			t.Fatalf("no %s symbol", name)
		}

		return a
	}

	spins := []uint32{symbol("SPIN1"), symbol("SPIN2")}
	typed := 0

	if err := c.Engine.CallEntry(addr); err != nil {
		t.Fatal(err)
	}

	done := false

	for i := 0; i < 100_000 && !done; i++ {
		// Type CTRL/C the first time the program reaches each spin loop.
		if typed < len(spins) && c.CPU.GPR(vax.PC) == spins[typed] {
			c.Engine.Attention()
			typed++
		}

		switch err := c.Engine.Step(); {
		case err == nil:
		case errors.Is(err, cpu.ErrConsoleCallReturned):
			done = true
		default:
			t.Fatalf("step %d: %v (CTRL/C stopped the program instead of running its AST?)", i, err)
		}
	}

	if !done {
		t.Fatal("ctrlc_ast.asm didn't finish within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1", got)
	}

	for name, want := range map[string]uint32{"SEEN": 5, "YSEEN": 9} {
		if got, _ := c.Mem.LoadLongword(c.CPU, symbol(name)); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}

	if c.Engine.AttentionRequested() || c.RTL.AttentionASTs() != 0 {
		t.Error("a key or an AST request was left over")
	}
}

// TestAttentionKeys_matchRTL: the engine's attention keys are the RTL's.
func TestAttentionKeys_matchRTL(t *testing.T) {
	c := newBootableConsole(t)

	var _ cpu.AttentionHandler = c

	if cpu.AttentionCtrlC != rtl.AttentionCtrlC || cpu.AttentionCtrlY != rtl.AttentionCtrlY {
		t.Errorf("engine keys %#x/%#x, RTL keys %#x/%#x", cpu.AttentionCtrlC, cpu.AttentionCtrlY, rtl.AttentionCtrlC, rtl.AttentionCtrlY)
	}

	// With nothing enabled, neither key is taken.
	if c.HandleAttention(cpu.AttentionCtrlC) || c.HandleAttention(cpu.AttentionCtrlY) {
		t.Error("a key was taken with no AST enabled")
	}
}

// TestGetdvi_assembledProgram is docs/PHASE-26.md subtask 28's acceptance
// test: testdata/asm/getdvi.asm asks $GETDVIW about its terminal channel
// and $GETDVI (then $SYNCH) about SYS$OUTPUT.
func TestGetdvi_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	c.DefineDevice("TTA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassTT})

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "getdvi.asm"))
	if err != nil {
		t.Fatalf("Assemble(getdvi.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("getdvi.asm has no entry address")
	}

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running getdvi.asm: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %d, want 1 (every call behaved)", got)
	}

	read := func(name string, n int) []byte {
		t.Helper()

		a, ok := c.Symbols.Get(name)
		if !ok {
			t.Fatalf("no %s symbol", name)
		}

		buf := make([]byte, n)
		if err := c.Mem.Load(c.CPU, a, buf); err != nil {
			t.Fatal(err)
		}

		return buf
	}

	word := func(name string) int { b := read(name, 2); return int(b[0]) | int(b[1])<<8 }

	if got := string(read("NAME", word("NAMLEN"))); got != "_TTA0:" {
		t.Errorf("DVI$_DEVNAM = %q, want \"_TTA0:\"", got)
	}

	if got := string(read("FULL", word("FULLEN"))); got != "_"+c.RTL.NodeName+"$TTA0:" {
		t.Errorf("DVI$_FULLDEVNAM of SYS$OUTPUT = %q", got)
	}

	if got := read("CLASS", 1)[0]; got != byte(iodev.DeviceClassTT) {
		t.Errorf("DVI$_DEVCLASS = %d, want %d", got, iodev.DeviceClassTT)
	}

	if got := read("UNIT", 1)[0]; got != 0 {
		t.Errorf("DVI$_UNIT = %d, want 0", got)
	}

	if got := read("REFCNT", 1)[0]; got != 1 {
		t.Errorf("DVI$_REFCNT = %d, want 1 (the program's channel)", got)
	}
}

// TestMailbox_assembledProgram is docs/PHASE-26.md subtask 29's
// acceptance test: testdata/asm/mailbox.asm passes two messages through
// a mailbox, the second read waiting in $QIOW until a timer AST writes
// its message, then deletes the mailbox by deassigning its channels.
func TestMailbox_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "mailbox.asm"))
	if err != nil {
		t.Fatalf("Assemble(mailbox.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("mailbox.asm has no entry address")
	}

	start := c.Engine.SystemTime()

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running mailbox.asm: err=%v hitCap=%v", runErr, hitCap)
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %d, want 1 (every call behaved)", got)
	}

	read := func(name string, n int) string {
		t.Helper()

		a, ok := c.Symbols.Get(name)
		if !ok {
			t.Fatalf("no %s symbol", name)
		}

		buf := make([]byte, n)
		if err := c.Mem.Load(c.CPU, a, buf); err != nil {
			t.Fatal(err)
		}

		return string(buf)
	}

	if got := read("MSG1", 5); got != "first" {
		t.Errorf("first message = %q", got)
	}

	if got := read("MSG2", 6); got != "second" {
		t.Errorf("second message = %q", got)
	}

	if elapsed := c.Engine.SystemTime() - start; elapsed < 10*10_000 {
		t.Errorf("system time advanced %d, want at least 10ms: the read didn't wait for the timer", elapsed)
	}

	if _, found := c.Devices.Find("MBA1"); found || len(c.RTL.Mailboxes.All()) != 0 {
		t.Error("the temporary mailbox outlived its channels")
	}

	if n := c.RTL.PendingIO(); n != 0 {
		t.Errorf("%d I/O requests left pending", n)
	}
}

// TestProcessControl_assembledProgram is docs/PHASE-26.md subtask 30's
// acceptance test: testdata/asm/process_control.asm renames its process,
// sets its priority, and forces its own exit, which runs its exit
// handler and ends the image before the code after $FORCEX.
func TestProcessControl_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "process_control.asm"))
	if err != nil {
		t.Fatalf("Assemble(process_control.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("process_control.asm has no entry address")
	}

	start := c.RTL.Process.BasePriority

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running process_control.asm: err=%v hitCap=%v", runErr, hitCap)
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

	p := c.RTL.Process
	if p.Name != "WORKER" || p.BasePriority != 6 {
		t.Errorf("process %q priority %d; want WORKER, 6", p.Name, p.BasePriority)
	}

	if got := word("OLDPRI"); got != start {
		t.Errorf("OLDPRI = %d, want %d", got, start)
	}

	if got := word("SEEN"); got != 0x2C {
		t.Errorf("the exit handler saw %#x, want 0x2C", got)
	}

	if word("REACHED") != 0 {
		t.Error("the code after $FORCEX ran")
	}

	if got := c.CPU.GPR(vax.R0); got != 0x2C || p.ExitStatus != 0x2C {
		t.Errorf("R0 = %#x, exit status %#x; want 0x2C", got, p.ExitStatus)
	}
}
