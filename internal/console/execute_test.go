package console

import (
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

const (
	opHalt = 0x00
	opNop  = 0x01
)

func loadProgram(t *testing.T, c *Console, addr uint32, bytes ...byte) {
	t.Helper()

	for i, b := range bytes {
		if err := c.Deposit("", addr+uint32(i), SizeByte, uint32(b)); err != nil {
			t.Fatalf("Deposit: %v", err)
		}
	}
}

// TestExecute_xfcQuitEmulatorStopsConsole exercises XFC$QUIT_EMULATION
// (opcode 0xFC, selector 0x78) end to end through the console layer: it
// must both halt the CPU (Execute returns cleanly, like any HALT) and stop
// the console's own command loop (Running() goes false) -
// the two-part effect main.go's main() relies on (via Console.Running) to
// exit the whole program instead of just returning to the "VAX>" prompt.
func TestExecute_xfcQuitEmulatorStopsConsole(t *testing.T) {
	c, _ := newTestConsole(t)
	loadProgram(t, c, 0x200, 0xFC, 0x78) // XFC #XFC$QUIT_EMULATION

	if !c.Running() {
		t.Fatal("Running() = false before Execute, want true")
	}

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if c.Running() {
		t.Error("Running() = true after XFC$QUIT_EMULATION, want false")
	}
}

func TestExecute_runsUntilHalt(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, opNop, opNop, opHalt)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "HALT") {
		t.Errorf("output = %q, want a HALT message", buf.String())
	}

	if got := c.CPU.GPR(vax.PC); got != 0x204 {
		t.Errorf("PC after halt = %#x, want 0x204", got)
	}
}

// movR0Program is "MOVL #0x12345678, R0" followed by HALT: 0xD0 (MOVL),
// 0x8F (immediate-longword source), the 4 immediate bytes, 0x50 (register-
// direct destination, R0), then opHalt -- enough to exercise the
// instruction trace, the DebugRegisters changed-register dump (R0
// changes), and the DebugFullDisasm operand dump (one read, one write
// operand) all at once.
func movR0Program(t *testing.T, c *Console, addr uint32) {
	t.Helper()
	loadProgram(t, c, addr, 0xD0, 0x8F, 0x78, 0x56, 0x34, 0x12, 0x50, opHalt)
}

func TestExecute_tracesWhenTraceEnabled(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = true

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[KSP ") || !strings.Contains(out, "MOVL") {
		t.Errorf("output = %q, want a [KSP ...] MOVL trace line", out)
	}
}

func TestExecute_noTraceByDefault(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if strings.Contains(buf.String(), "MOVL") {
		t.Errorf("output = %q, want no MOVL trace line with Trace off", buf.String())
	}
}

func TestExecute_debugRegistersChangeDump(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = true
	c.CPU.SetDebug(c.CPU.Debug() | vax.DebugRegisters)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "R0:  12345678") {
		t.Errorf("output = %q, want a changed-R0 register dump line", buf.String())
	}
}

func TestExecute_noRegisterDumpWhenDebugRegistersClear(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = true
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugRegisters)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if strings.Contains(buf.String(), "R0:  12345678") {
		t.Errorf("output = %q, want no register dump with DebugRegisters clear", buf.String())
	}
}

func TestExecute_debugFullDisasmOperandDump(t *testing.T) {
	c, buf := newTestConsole(t)
	movR0Program(t, c, 0x200)
	c.Trace = true
	c.CPU.SetDebug(c.CPU.Debug() | vax.DebugFullDisasm)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "#0 read") {
		t.Errorf("output = %q, want an operand #0 read line", out)
	}

	if !strings.Contains(out, "#1 write") || !strings.Contains(out, "R0 = 12345678") {
		t.Errorf("output = %q, want an operand #1 write line naming R0 = 12345678", out)
	}
}

func TestCall_returnsCleanlyThroughSentinelFrame(t *testing.T) {
	c, _ := newTestConsole(t)

	// Procedure at 0x200: empty entry mask, CALLS a nested procedure at
	// 0x300, then RET.
	loadProgram(t, c, 0x200,
		0x00, 0x00, // entry mask: no registers saved
		0xFB, 0x00, 0x9F, 0x00, 0x03, 0x00, 0x00, // CALLS #0, @#0x300
		0x04, // RET
	)
	// Nested procedure at 0x300: empty entry mask, RET immediately.
	loadProgram(t, c, 0x300, 0x00, 0x00, 0x04)

	if err := c.Call(0x200, false); err != nil {
		t.Fatalf("Call: %v", err)
	}
}

func TestCall_tracesWhenTraceEnabled(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200,
		0x00, 0x00, // entry mask: no registers saved
		0xFB, 0x00, 0x9F, 0x00, 0x03, 0x00, 0x00, // CALLS #0, @#0x300
		0x04, // RET
	)
	loadProgram(t, c, 0x300, 0x00, 0x00, 0x04)
	c.Trace = true

	if err := c.Call(0x200, false); err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !strings.Contains(buf.String(), "[KSP ") {
		t.Errorf("output = %q, want [KSP ...] trace lines during CALL", buf.String())
	}
}

// TestExecute_stopsAtInstructionLimit exercises govax's own -instruction-limit
// flag (docs/PHASE-15.md's sub-phase 2): an infinite loop (NOP; BRB back to
// itself) must not hang Execute when a limit is configured -- it should stop
// cleanly, reporting the limit rather than propagating an error.
func TestExecute_stopsAtInstructionLimit(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200,
		opNop,
		0x11, 0xFD, // BRB base (displacement -3: back to the NOP)
	)
	c.Engine.SetLimits(5, 0)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("Execute: %v (want a clean stop, not an error)", err)
	}

	if !strings.Contains(buf.String(), "INSTRLIMIT") {
		t.Errorf("output = %q, want an instruction-limit message", buf.String())
	}
}

// TestExecute_beginRunGivesEachCommandAFreshBudget confirms a second Execute
// call gets its own full instruction budget rather than inheriting the
// first call's exhaustion (Engine.BeginRun's own contract).
func TestExecute_beginRunGivesEachCommandAFreshBudget(t *testing.T) {
	c, _ := newTestConsole(t)
	loadProgram(t, c, 0x200, opNop, 0x11, 0xFD)
	c.Engine.SetLimits(3, 0)

	addr := uint32(0x200)
	if err := c.Execute(&addr); err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	if err := c.Execute(&addr); err != nil {
		t.Fatalf("second Execute: %v (want a fresh budget, not immediate exhaustion)", err)
	}
}

// TestExecute_stopsOnAttention exercises Ctrl-C interrupting a running VAX
// program (cpu.Engine.Attention, wired up in main.go's own terminal
// plumbing -- see attention.go): once something else has called Attention
// mid-run, Execute's own Step loop must stop cleanly -- at the end of
// whatever instruction is currently in flight, not mid-instruction -- and
// report it via reportStopReason, rather than hang (this program is an
// infinite loop) or propagate an error. Attention is called from a
// separate goroutine, as its real caller (main.go's background
// terminal-reading goroutine) calls it, under its "safe to call from any
// goroutine" contract. A run's start (BeginRun) clears a CTRL/C typed
// before it, so the goroutine presses CTRL/C again every millisecond
// until Execute returns: one press lands after the run has begun,
// whichever goroutine is scheduled first. (A single press used to race
// BeginRun, and the run then ended at the instruction limit instead.)
// The instruction limit is a safety net against a hang.
func TestExecute_stopsOnAttention(t *testing.T) {
	c, buf := newTestConsole(t)
	loadProgram(t, c, 0x200,
		opNop,
		0x11, 0xFD, // BRB base (displacement -3: back to the NOP)
	)
	c.Engine.SetLimits(500_000_000, 0)

	done := make(chan struct{})
	pressed := make(chan struct{})

	go func() {
		defer close(pressed)

		for {
			c.Engine.Attention()

			select {
			case <-done:
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()

	addr := uint32(0x200)
	err := c.Execute(&addr)

	close(done)
	<-pressed

	if err != nil {
		t.Fatalf("Execute: %v (want a clean stop, not an error)", err)
	}

	if !strings.Contains(buf.String(), "ATTENTION") {
		t.Errorf("output = %q, want an attention/interrupt message", buf.String())
	}

	if strings.Contains(buf.String(), "INSTRLIMIT") {
		t.Errorf("output = %q, stopped via the instruction-limit safety net instead of Attention", buf.String())
	}
}

func TestExecute_requiresInit(t *testing.T) {
	c := New(&strings.Builder{})
	if err := c.Execute(nil); err == nil {
		t.Error("expected error before Init")
	}
}

// TestReportStop_attention is a direct, non-racy unit test of the
// message ReportStop prints for ErrAttention, complementing
// TestExecute_stopsOnAttention's own end-to-end (if inherently
// timing-dependent) coverage above.
func TestReportStopReason_attention(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := c.ReportStop(cpu.ErrAttention); err != nil {
		t.Fatalf("ReportStop(ErrAttention) = %v, want nil (a benign, reported stop)", err)
	}

	if !strings.Contains(buf.String(), "ATTENTION") {
		t.Errorf("output = %q, want an attention/interrupt message", buf.String())
	}
}
