package cpu

import (
	"errors"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vax"
)

// limitsEngine returns an Engine running a genuine infinite loop (NOP;
// BRB back to itself) at base -- exactly the "runaway program" shape
// -instruction-limit/-time-limit exist to bound.
func limitsEngine(t *testing.T) *Engine {
	t.Helper()
	e := newEngine()
	e.cpu.SetGPR(vax.PC, base)
	putBytes(t, e.cpu, e.mem, base,
		0x01,       // NOP
		0x11, 0xFD, // BRB base (displacement -3: back past both the NOP and this BRB)
	)
	return e
}

func TestEngineInstructionLimitStopsRunButLeavesStateIntact(t *testing.T) {
	e := limitsEngine(t)
	e.SetLimits(3, 0)
	e.BeginRun()

	for i := 0; i < 3; i++ {
		if err := e.Step(); err != nil {
			t.Fatalf("Step %d: %v", i, err)
		}
	}

	err := e.Step()
	if !errors.Is(err, ErrInstructionLimitExceeded) {
		t.Fatalf("Step 4 = %v, want ErrInstructionLimitExceeded", err)
	}

	// Refusing the 4th instruction must not have changed anything -- calling
	// Step again keeps refusing, not somehow recovering or corrupting PC.
	pc := e.cpu.GPR(vax.PC)
	if err := e.Step(); !errors.Is(err, ErrInstructionLimitExceeded) {
		t.Fatalf("Step 5 = %v, want ErrInstructionLimitExceeded again", err)
	}
	if got := e.cpu.GPR(vax.PC); got != pc {
		t.Errorf("PC changed from %#x to %#x across a refused Step", pc, got)
	}
}

func TestEngineInstructionLimitZeroMeansUnlimited(t *testing.T) {
	e := limitsEngine(t)
	e.SetLimits(0, 0)
	e.BeginRun()

	for i := 0; i < 1000; i++ {
		if err := e.Step(); err != nil {
			t.Fatalf("Step %d: %v (want no limit applied)", i, err)
		}
	}
}

func TestEngineBeginRunResetsInstructionCountBetweenRuns(t *testing.T) {
	e := limitsEngine(t)
	e.SetLimits(2, 0)

	e.BeginRun()
	for i := 0; i < 2; i++ {
		if err := e.Step(); err != nil {
			t.Fatalf("first run, step %d: %v", i, err)
		}
	}
	if err := e.Step(); !errors.Is(err, ErrInstructionLimitExceeded) {
		t.Fatalf("first run's 3rd step = %v, want ErrInstructionLimitExceeded", err)
	}

	// A fresh run (BeginRun again, as Console.Execute/Call/Step do at the
	// start of each command) must get its own full budget, not inherit the
	// first run's exhaustion.
	e.BeginRun()
	for i := 0; i < 2; i++ {
		if err := e.Step(); err != nil {
			t.Fatalf("second run, step %d: %v", i, err)
		}
	}
}

func TestEngineTimeLimitStopsRun(t *testing.T) {
	e := limitsEngine(t)
	e.SetLimits(0, 20*time.Millisecond)
	e.BeginRun()

	deadline := time.Now().Add(2 * time.Second)
	steps := 0

	for {
		if time.Now().After(deadline) {
			t.Fatal("time limit was never enforced within a generous real-time deadline")
		}

		err := e.Step()
		if err == nil {
			steps++

			continue
		}

		if errors.Is(err, ErrTimeLimitExceeded) {
			break
		}

		t.Fatalf("Step: %v", err)
	}
	
	if steps == 0 {
		t.Error("expected at least one instruction to execute before the time limit hit")
	}
}

func TestEngineAttentionStopsRunButLeavesStateIntact(t *testing.T) {
	e := limitsEngine(t)
	e.BeginRun()

	for i := 0; i < 3; i++ {
		if err := e.Step(); err != nil {
			t.Fatalf("Step %d: %v", i, err)
		}
	}

	e.Attention()

	if !e.AttentionRequested() {
		t.Fatal("AttentionRequested() = false right after Attention()")
	}

	err := e.Step()
	if !errors.Is(err, ErrAttention) {
		t.Fatalf("Step after Attention() = %v, want ErrAttention", err)
	}

	// Refusing the next instruction must not have changed anything --
	// calling Step again keeps refusing, matching the instruction-limit
	// case above, not somehow recovering or corrupting PC.
	pc := e.cpu.GPR(vax.PC)
	if err := e.Step(); !errors.Is(err, ErrAttention) {
		t.Fatalf("second Step = %v, want ErrAttention again", err)
	}
	if got := e.cpu.GPR(vax.PC); got != pc {
		t.Errorf("PC changed from %#x to %#x across a refused Step", pc, got)
	}
}

func TestEngineBeginRunClearsAttentionBetweenRuns(t *testing.T) {
	e := limitsEngine(t)
	e.BeginRun()
	e.Attention()

	if err := e.Step(); !errors.Is(err, ErrAttention) {
		t.Fatalf("Step = %v, want ErrAttention", err)
	}

	// A fresh run (BeginRun again, as Console.Execute/Call/Step do at the
	// start of each command) must not still be carrying a stale Attention
	// from before -- matching execute_vax's own `vax.halted = 0` at the
	// top of every run: a Ctrl-C pressed while idle at the prompt has no
	// lingering effect on the next command.
	e.BeginRun()

	if e.AttentionRequested() {
		t.Error("AttentionRequested() = true right after BeginRun()")
	}

	for i := 0; i < 5; i++ {
		if err := e.Step(); err != nil {
			t.Fatalf("Step %d after BeginRun: %v (want the stale Attention cleared)", i, err)
		}
	}
}

func TestEngineTimeLimitZeroMeansUnlimited(t *testing.T) {
	e := limitsEngine(t)
	e.SetLimits(0, 0)
	e.BeginRun()
	if !e.runDeadline.IsZero() {
		t.Error("runDeadline should stay zero (no time.Now() call) when no time limit is configured")
	}

	for i := 0; i < 1000; i++ {
		if err := e.Step(); err != nil {
			t.Fatalf("Step %d: %v (want no limit applied)", i, err)
		}
	}
}
