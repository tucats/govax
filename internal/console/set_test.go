package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestSetSymbol_register(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetSymbol("R5", 0xCAFEBABE); err != nil {
		t.Fatalf("SetSymbol: %v", err)
	}
	if got := c.CPU.GPR(vax.R5); got != 0xCAFEBABE {
		t.Errorf("R5 = %#x, want 0xcafebabe", got)
	}
}

func TestSetSymbol_privilegedRegister(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetSymbol("SIRR", 5); err != nil {
		t.Fatalf("SetSymbol: %v", err)
	}
	if got := c.CPU.PR(vax.SIRR); got != 5 {
		t.Errorf("SIRR = %d, want 5", got)
	}
}

func TestSetSymbol_psl(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetSymbol("PSL", 0x001F0000); err != nil {
		t.Fatalf("SetSymbol: %v", err)
	}
	if got := c.CPU.PSL(); uint32(got) != 0x001F0000 {
		t.Errorf("PSL = %#x, want 0x1f0000", uint32(got))
	}
}

func TestSetSymbol_userSymbol(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetSymbol("FOOBAR", 0x1234); err != nil {
		t.Fatalf("SetSymbol: %v", err)
	}
	v, ok := c.Symbols.Get("FOOBAR")
	if !ok || v != 0x1234 {
		t.Errorf("FOOBAR = %#x, ok=%v; want 0x1234, true", v, ok)
	}
}

func TestSetRadix(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetRadix(8); err != nil {
		t.Fatalf("SetRadix: %v", err)
	}
	if c.Radix != 8 {
		t.Errorf("Radix = %d, want 8", c.Radix)
	}
	if err := c.SetRadix(2); err == nil {
		t.Error("expected error for invalid radix")
	}
}

func TestSetDebug_defaultFlags(t *testing.T) {
	c, _ := newTestConsole(t)
	if got := c.CPU.Debug(); got != vax.DebugDefault {
		t.Errorf("Debug() = %#x, want DebugDefault (%#x)", got, vax.DebugDefault)
	}
}

func TestSetDebug_setAndClear(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetDebug([]string{"VM", "RMS"}); err != nil {
		t.Fatalf("SetDebug: %v", err)
	}
	if !c.CPU.DebugEnabled(vax.DebugVM) || !c.CPU.DebugEnabled(vax.DebugRMS) {
		t.Errorf("Debug() = %#x, want VM and RMS set", c.CPU.Debug())
	}
	// The default REGISTERS/USERHALT/LIBINIT bits are untouched by setting
	// unrelated flags.
	if !c.CPU.DebugEnabled(vax.DebugUserHalt) {
		t.Errorf("Debug() = %#x, want USERHALT still set", c.CPU.Debug())
	}

	if err := c.SetDebug([]string{"NOUSERHALT"}); err != nil {
		t.Fatalf("SetDebug: %v", err)
	}
	if c.CPU.DebugEnabled(vax.DebugUserHalt) {
		t.Errorf("Debug() = %#x, want USERHALT cleared", c.CPU.Debug())
	}
	if !c.CPU.DebugEnabled(vax.DebugVM) {
		t.Errorf("Debug() = %#x, want VM still set", c.CPU.Debug())
	}
}

func TestSetDebug_bareSetsNativeDebugger(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetDebug(nil); err != nil {
		t.Fatalf("SetDebug: %v", err)
	}
	if !c.CPU.DebugEnabled(vax.DebugNative) {
		t.Errorf("Debug() = %#x, want DEBUG (native debugger) set", c.CPU.Debug())
	}
}

func TestSetDebug_invalidFlag(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetDebug([]string{"BOGUS"}); err == nil {
		t.Error("expected an error for an invalid SET DEBUG flag")
	}
}

func TestSetTrace(t *testing.T) {
	c, _ := newTestConsole(t)
	if c.Trace {
		t.Error("Trace = true, want false by default")
	}

	c.SetTrace(true)
	if !c.Trace {
		t.Error("Trace = false, want true after SetTrace(true)")
	}

	c.SetTrace(false)
	if c.Trace {
		t.Error("Trace = true, want false after SetTrace(false)")
	}
}

func TestShowRegisters(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetGPR(vax.R3, 0x11223344)
	buf.Reset()
	if err := c.ShowRegisters(); err != nil {
		t.Fatalf("ShowRegisters: %v", err)
	}
	if !strings.Contains(buf.String(), "11223344") {
		t.Errorf("output = %q, want it to contain the R3 value", buf.String())
	}
}

func TestShowPSL(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()
	if err := c.ShowPSL(); err != nil {
		t.Fatalf("ShowPSL: %v", err)
	}
	if !strings.Contains(buf.String(), "PSL") {
		t.Errorf("output = %q, want it to mention PSL", buf.String())
	}
}

func TestShowMemory(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()
	if err := c.ShowMemory(); err != nil {
		t.Fatalf("ShowMemory: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "0000FFFF") { // 64K - 1, the top physical address
		t.Errorf("output = %q, want it to contain the top physical address", out)
	}
	if !strings.Contains(out, "configuration is unknown") {
		t.Errorf("output = %q, want it to report VM as unconfigured before VMINIT", out)
	}
}

func TestShowMemory_afterVMInit(t *testing.T) {
	c, buf := newTestConsole(t)
	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2); err != nil {
		t.Fatalf("VMInit: %v", err)
	}
	buf.Reset()
	if err := c.ShowMemory(); err != nil {
		t.Fatalf("ShowMemory: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Virtual Memory (currently ENABLED)") {
		t.Errorf("output = %q, want MAPEN reported enabled", out)
	}
	if !strings.Contains(out, "physical pages mapped") {
		t.Errorf("output = %q, want a mapped/free page count", out)
	}
	for _, name := range []string{"P0 Region", "P1 Region", "S0 Region"} {
		if !strings.Contains(out, name) {
			t.Errorf("output = %q, want it to contain %q", out, name)
		}
	}
}

func TestShowSymbols(t *testing.T) {
	c, buf := newTestConsole(t)
	c.Symbols.Set("MYSYM", 0xABCD, SymbolUser)
	buf.Reset()
	if err := c.ShowSymbols(); err != nil {
		t.Fatalf("ShowSymbols: %v", err)
	}
	if !strings.Contains(buf.String(), "MYSYM") || !strings.Contains(buf.String(), "0000ABCD") {
		t.Errorf("output = %q, want it to contain MYSYM and its value", buf.String())
	}
}

func TestShowBreakpoints(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()
	if err := c.ShowBreakpoints(); err != nil {
		t.Fatalf("ShowBreakpoints: %v", err)
	}
	if !strings.Contains(buf.String(), "No breakpoints") {
		t.Errorf("output = %q, want a no-breakpoints message", buf.String())
	}

	c.AddBreakpoint(0x300)
	buf.Reset()
	if err := c.ShowBreakpoints(); err != nil {
		t.Fatalf("ShowBreakpoints: %v", err)
	}
	if !strings.Contains(buf.String(), "00000300") {
		t.Errorf("output = %q, want it to contain the breakpoint address", buf.String())
	}
}

func TestShowRadix(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()
	if err := c.ShowRadix(); err != nil {
		t.Fatalf("ShowRadix: %v", err)
	}
	if !strings.Contains(buf.String(), "16") {
		t.Errorf("output = %q, want it to contain the default radix 16", buf.String())
	}
}

func TestShowStack(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetPR(vax.ESP, 0x99887766)
	buf.Reset()
	if err := c.ShowStack(StackESP, false, 0, false); err != nil {
		t.Fatalf("ShowStack: %v", err)
	}
	if !strings.Contains(buf.String(), "99887766") {
		t.Errorf("output = %q, want it to contain the ESP value", buf.String())
	}
}

func TestShowCPU(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()
	if err := c.ShowCPU(); err != nil {
		t.Fatalf("ShowCPU: %v", err)
	}
	if !strings.Contains(buf.String(), "running") {
		t.Errorf("output = %q, want it to say running", buf.String())
	}
}
