package console

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/rtl"
	"github.com/tucats/govax/internal/vax"
)

func TestConsoleWriteByteAndReadByte(t *testing.T) {
	c, buf := newTestConsole(t)
	c.ConsoleWriteByte('A')
	if buf.String() != "A" {
		t.Errorf("output = %q, want \"A\"", buf.String())
	}

	c.In = strings.NewReader("Z")
	if got := c.ConsoleReadByte(); got != 'Z' {
		t.Errorf("ConsoleReadByte() = %q, want 'Z'", got)
	}
}

func TestConsoleReadByteNoInputSource(t *testing.T) {
	c, _ := newTestConsole(t)
	if got := c.ConsoleReadByte(); got != 0 {
		t.Errorf("ConsoleReadByte() = %d, want 0 (no In configured)", got)
	}
}

func TestConsoleCommandNoDispatcher(t *testing.T) {
	c, _ := newTestConsole(t)
	if got := c.ConsoleCommand("SHOW VERSION"); got != 1 {
		t.Errorf("ConsoleCommand() = %d, want 1 (no Dispatcher configured)", got)
	}
}

func TestConsoleCommandDispatches(t *testing.T) {
	d, c := newTestDispatcher(t)
	c.Dispatcher = d

	if got := c.ConsoleCommand("EXAMINE R0"); got != 0 {
		t.Errorf("ConsoleCommand(\"EXAMINE R0\") = %d, want 0", got)
	}
	if got := c.ConsoleCommand("NOT A REAL COMMAND AT ALL"); got != 1 {
		t.Errorf("ConsoleCommand(garbage) = %d, want 1", got)
	}
}

func TestConsoleSystemServiceDelegatesToRTL(t *testing.T) {
	c, _ := newTestConsole(t)

	// Build a real one-argument VAX argument list (SYS$SETEF's own shape)
	// at an arbitrary scratch address and point AP at it, matching how a
	// CALLS-based call sets up AP before the callee's XFC executes.
	ap := uint32(0x1000)
	if err := c.Mem.StoreLongword(c.CPU, ap, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.Mem.StoreLongword(c.CPU, ap+4, 3); err != nil {
		t.Fatal(err)
	}
	c.CPU.SetGPR(vax.AP, ap)

	// SYS$SETEF's real p1Vector address, matching how Engine's XFC handler
	// would compute and pass it.
	r0, handled, err := c.SystemService(0x7FFEE000)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("SYS$SETEF not handled")
	}
	if r0 != 1 {
		t.Errorf("r0 = %d, want 1 (ssNormal)", r0)
	}
}

func TestConsoleShimDelegatesToRTL(t *testing.T) {
	c, _ := newTestConsole(t)

	r0, handled, err := c.Shim(32) // DECC$TIME
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("DECC$TIME shim not handled")
	}
	if r0 == 0 {
		t.Error("r0 = 0, want a nonzero Unix timestamp")
	}
}

func TestConsoleReInitReplacesRTLEnvironment(t *testing.T) {
	c, _ := newTestConsole(t)
	first := c.RTL
	if err := c.Init(64 * 1024); err != nil {
		t.Fatal(err)
	}
	if c.RTL == first {
		t.Error("RTL environment unchanged after re-Init, want a fresh one bound to the new CPU/Memory")
	}
}

func TestTranslateHalt(t *testing.T) {
	if !errors.Is(translateHalt(rtl.ErrHalt), cpu.ErrHalted) {
		t.Error("translateHalt(rtl.ErrHalt) does not unwrap to cpu.ErrHalted")
	}
	other := errors.New("boom")
	if !errors.Is(translateHalt(other), other) {
		t.Error("translateHalt should pass through any other error unchanged")
	}
	if translateHalt(nil) != nil {
		t.Error("translateHalt(nil) should be nil")
	}
}
