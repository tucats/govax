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
	if !strings.Contains(buf.String(), "10000") { // 64K in hex
		t.Errorf("output = %q, want it to contain the memory size", buf.String())
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
	if err := c.ShowStack(StackESP); err != nil {
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
