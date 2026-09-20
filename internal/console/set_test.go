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

// TestSetSymbol_pslInvalidatesProtectionOnModeChange matches
// console_set.c's own bare "SET PSL=value" case, which calls
// read_psl_bits() right after -- invalidating cached TB protection state
// if CurMod actually changed (docs/PHASE-21.md). The cached mapping
// itself must survive; only its verified access mode is reset.
func TestSetSymbol_pslInvalidatesProtectionOnModeChange(t *testing.T) {
	c := newRunnableConsole(t)

	if _, err := c.Mem.LoadLongword(c.CPU, 0x200); err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}

	entries := c.Mem.TBSnapshot()
	if len(entries) == 0 || !entries[0].ProtValid() {
		t.Fatalf("TBSnapshot() = %+v, want a populated, protection-valid entry first", entries)
	}

	// CurMod occupies PSL bits 25:24 -- switch it from Kernel (0) to User
	// (3), leaving every other bit as Init/VMInit left it.
	newPSL := uint32(c.CPU.PSL())&^(0x3<<24) | (3 << 24)

	if err := c.SetSymbol("PSL", newPSL); err != nil {
		t.Fatalf("SetSymbol: %v", err)
	}

	if got := c.CPU.PSL().CurMod(); got != vax.User {
		t.Fatalf("CurMod() = %v, want User (the SET PSL should have taken effect)", got)
	}

	entries = c.Mem.TBSnapshot()
	if len(entries) == 0 {
		t.Fatalf("TBSnapshot() empty after SET PSL, want the mapping to survive")
	}

	if entries[0].ProtValid() {
		t.Errorf("entries[0].ProtValid() = true after SET PSL changed CurMod, want false (forced recheck)")
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

func TestSetSymbolQualified_permanentSurvivesClearTemporary(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetSymbolQualified("PERMFOO", 1, true, false, false); err != nil {
		t.Fatalf("SetSymbolQualified: %v", err)
	}

	if err := c.SetSymbolQualified("TEMPFOO", 2, false, false, false); err != nil {
		t.Fatalf("SetSymbolQualified: %v", err)
	}

	c.Symbols.ClearTemporary()

	if _, ok := c.Symbols.Get("PERMFOO"); !ok {
		t.Error("expected PERMFOO (permanent) to survive ClearTemporary")
	}

	if _, ok := c.Symbols.Get("TEMPFOO"); ok {
		t.Error("expected TEMPFOO (not permanent) to be removed by ClearTemporary")
	}
}

func TestSetPSLField_bit(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetPSLField("T", 1); err != nil {
		t.Fatalf("SetPSLField: %v", err)
	}

	if !c.CPU.PSL().T() {
		t.Error("expected PSL.T set")
	}

	if err := c.SetPSLField("T", 0); err != nil {
		t.Fatalf("SetPSLField: %v", err)
	}

	if c.CPU.PSL().T() {
		t.Error("expected PSL.T cleared")
	}
}

func TestSetPSLField_ipl(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetPSLField("IPL", 15); err != nil {
		t.Fatalf("SetPSLField: %v", err)
	}

	if got := c.CPU.PSL().IPL(); got != 15 {
		t.Errorf("IPL() = %d, want 15", got)
	}

	if err := c.SetPSLField("IPL", 99); err == nil {
		t.Error("expected an error for IPL out of range")
	}
}

func TestSetPSLField_curModSwitchesStack(t *testing.T) {
	c, _ := newTestConsole(t)
	c.CPU.SetPR(vax.KSP, 0x1000)
	c.CPU.SetPR(vax.ESP, 0x2000)
	c.CPU.SetGPR(vax.SP, 0x1000)

	if err := c.SetPSLField("CUR_MOD", uint32(vax.Executive)); err != nil {
		t.Fatalf("SetPSLField: %v", err)
	}

	if got := c.CPU.PSL().CurMod(); got != vax.Executive {
		t.Errorf("CurMod() = %d, want Executive", got)
	}

	if got := c.CPU.GPR(vax.SP); got != 0x2000 {
		t.Errorf("SP = %#x, want ESP's own 0x2000 after switching to Executive", got)
	}
}

func TestSetMode(t *testing.T) {
	c, _ := newTestConsole(t)
	c.CPU.SetPR(vax.USP, 0x3000)

	if err := c.SetMode("USER"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}

	if got := c.CPU.PSL().CurMod(); got != vax.User {
		t.Errorf("CurMod() = %d, want User", got)
	}

	if err := c.SetMode("INTERRUPT"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}

	if !c.CPU.PSL().IS() {
		t.Error("expected PSL.IS set after SET MODE INTERRUPT")
	}

	if err := c.SetMode("BOGUS"); err == nil {
		t.Error("expected an error for an unknown mode")
	}
}

func TestSetVM_requiresKernelMode(t *testing.T) {
	c, _ := newTestConsole(t)
	psl := c.CPU.PSL()
	psl.SetCurMod(vax.User)
	c.CPU.SetPSL(psl)

	if err := c.SetVM(true); err == nil {
		t.Error("expected an error setting MAPEN outside kernel mode")
	}
}

func TestSetVM(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetVM(true); err != nil {
		t.Fatalf("SetVM(true): %v", err)
	}

	if c.CPU.PR(vax.MAPEN) != 1 {
		t.Errorf("MAPEN = %d, want 1", c.CPU.PR(vax.MAPEN))
	}

	if err := c.SetVM(false); err != nil {
		t.Fatalf("SetVM(false): %v", err)
	}

	if c.CPU.PR(vax.MAPEN) != 0 {
		t.Errorf("MAPEN = %d, want 0", c.CPU.PR(vax.MAPEN))
	}
}

func TestSetBase(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.SetBase(0x4000); err != nil {
		t.Fatalf("SetBase: %v", err)
	}

	if c.DepositAddr != 0x4000 {
		t.Errorf("DepositAddr = %#x, want 0x4000", c.DepositAddr)
	}
}

func TestSetVerboseVerifyNoVerbose(t *testing.T) {
	c, _ := newTestConsole(t)
	if !c.Verbose {
		t.Fatal("expected Verbose true by default (initialization.c's own CONSOLE_VERBOSE default)")
	}

	if err := c.SetVerify(); err != nil {
		t.Fatalf("SetVerify: %v", err)
	}

	if !c.Verify {
		t.Error("expected Verify set")
	}

	if !c.Verbose {
		t.Error("SetVerify must not touch Verbose")
	}

	if err := c.SetNoVerbose(); err != nil {
		t.Fatalf("SetNoVerbose: %v", err)
	}

	if c.Verbose || c.Verify {
		t.Errorf("Verbose=%v Verify=%v, want both false after SET NOVERBOSE", c.Verbose, c.Verify)
	}

	if err := c.SetVerbose(); err != nil {
		t.Fatalf("SetVerbose: %v", err)
	}

	if !c.Verbose {
		t.Error("expected Verbose set")
	}
}

func TestSetQuantum(t *testing.T) {
	c, buf := newTestConsole(t)
	buf.Reset()

	if err := c.SetQuantum(0); err != nil {
		t.Fatalf("SetQuantum: %v", err)
	}

	if !strings.Contains(buf.String(), "NOINTERRUPTS") {
		t.Errorf("output = %q, want the suspended-delivery message", buf.String())
	}

	current, initial := c.Engine.Quantum()
	if current != 0 || initial != 0 {
		t.Errorf("Quantum() = (%d, %d), want (0, 0)", current, initial)
	}

	buf.Reset()

	if err := c.SetQuantum(20); err != nil {
		t.Fatalf("SetQuantum: %v", err)
	}

	if !strings.Contains(buf.String(), "INTERRUPTS") {
		t.Errorf("output = %q, want the resumed-delivery message", buf.String())
	}
}

func TestSetPTE_roundTrip(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	addr := uint32(0x1000)
	if err := c.SetPTE(addr, "VALID", 1); err != nil {
		t.Fatalf("SetPTE VALID: %v", err)
	}

	if err := c.SetPTE(addr, "PFN", 0x42); err != nil {
		t.Fatalf("SetPTE PFN: %v", err)
	}

	_, _, pte, err := c.Mem.LookupPTE(c.CPU, addr)
	if err != nil {
		t.Fatalf("LookupPTE: %v", err)
	}

	if !pte.Valid() {
		t.Error("expected the valid bit set")
	}

	if pte.PFN() != 0x42 {
		t.Errorf("PFN() = %#x, want 0x42", pte.PFN())
	}
}

func TestSetPTE_badField(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if err := c.SetPTE(0x1000, "BOGUS", 1); err == nil {
		t.Error("expected an error for an unknown PTE field")
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
	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	buf.Reset()

	if err := c.ShowMemory(); err != nil {
		t.Fatalf("ShowMemory: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Virtual Memory (currently enabled)") {
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
	c.CPU.SetPR(vax.SBR, 0x11223344)
	c.DepositAddr = 0x00000200
	buf.Reset()

	if err := c.ShowCPU(); err != nil {
		t.Fatalf("ShowCPU: %v", err)
	}

	out := buf.String()

	for _, want := range []string{
		"Registers:", "PSL:", "Stack pointers:", "USP:", "ISP:",
		"Privileged registers:", "SBR", "11223344", "Next storage address is 00000200",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output = %q, want it to contain %q", out, want)
		}
	}
}
