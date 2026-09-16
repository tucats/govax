package console

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// newRunnableDispatcher wires a Dispatcher around a freshly VMINIT'd
// Console (newRunnableConsole, image_test.go), matching what a real govax
// session hands Dispatch -- interactive ASM mode (docs/PHASE-19.md) needs a
// live, translated address space to deposit into, exactly like the batch
// "ASM <file>" tests in asm_test.go.
func newRunnableDispatcher(t *testing.T) (*Dispatcher, *Console) {
	t.Helper()

	c := newRunnableConsole(t)
	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	return d, c
}

// TestDispatchASM_bareEntersInteractiveMode confirms a bare "ASM" command
// (no filename) puts the console into interactive assembler mode instead of
// erroring out, and that DISASM/EXAMINE-style commands are rejected as
// assembly statements while it's active -- matching console_dispatch.c's
// own hand-off to assemble() ahead of any verb-table lookup.
func TestDispatchASM_bareEntersInteractiveMode(t *testing.T) {
	d, c := newRunnableDispatcher(t)

	if c.InAssemblerMode() {
		t.Fatal("console should not start in assembler mode")
	}

	if err := d.Dispatch("ASM"); err != nil {
		t.Fatalf("Dispatch(ASM): %v", err)
	}

	if !c.InAssemblerMode() {
		t.Fatal("expected InAssemblerMode() true after a bare ASM command")
	}

	// "EXAMINE" is now just an (invalid) assembly statement, not a console
	// command -- matching the reference tool exactly (a line spelling a
	// command name is still fed to the assembler once assembler_mode is
	// on).
	if err := d.Dispatch("EXAMINE R0"); err == nil {
		t.Fatal("expected EXAMINE, read as an assembly statement, to fail")
	}

	if !c.InAssemblerMode() {
		t.Fatal("a bad statement must not exit assembler mode")
	}

	if err := d.Dispatch("END"); err != nil {
		t.Fatalf("Dispatch(END): %v", err)
	}

	if c.InAssemblerMode() {
		t.Fatal("expected InAssemblerMode() false after END")
	}
}

// TestDispatchASM_depositsAndAutoCallsEntry types a small routine in one
// line at a time and confirms: each statement deposits into live memory
// immediately (the label is resolvable mid-session), and "END <name>"
// auto-invokes CALL __ENTRY exactly like the batch ASM <file> form does.
func TestDispatchASM_depositsAndAutoCallsEntry(t *testing.T) {
	d, c := newRunnableDispatcher(t)

	for _, line := range []string{
		"ASM",
		".ENTRY START,^M<>",
		"        MOVL #7,R0",
		"        MOVL #3,R1",
		"        ADDL2 R1,R0",
		"        RET",
		"END START",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("Dispatch(%q): %v", line, err)
		}
	}

	if c.InAssemblerMode() {
		t.Fatal("expected InAssemblerMode() false after END START")
	}

	if got := c.CPU.GPR(vax.R0); got != 10 {
		t.Errorf("R0 = %d, want 10 (END START should auto-CALL the routine)", got)
	}

	if _, ok := c.Symbols.Get("START"); !ok {
		t.Fatal("expected START to be merged into Console.Symbols")
	}
}

// TestDispatchASM_depositAddrIntegration confirms a fresh interactive
// session starts depositing at Console.DepositAddr's *current* value, not
// the assembler package's own hardcoded default -- matching the reference
// tool, where EXAMINE/DEPOSIT/interactive-ASM all share one
// vax.console.deposit register (see docs/PHASE-19.md's design notes) -- and
// that DepositAddr is kept in sync afterward.
func TestDispatchASM_depositAddrIntegration(t *testing.T) {
	d, c := newRunnableDispatcher(t)

	c.DepositAddr = 0x4000

	if err := d.Dispatch("ASM"); err != nil {
		t.Fatalf("Dispatch(ASM): %v", err)
	}

	if err := d.Dispatch(".ENTRY START,^M<>"); err != nil {
		t.Fatalf("Dispatch(.ENTRY): %v", err)
	}

	if err := d.Dispatch("MOVL #9,R0"); err != nil {
		t.Fatalf("Dispatch(MOVL): %v", err)
	}

	addr, ok := c.Symbols.Get("START")
	if !ok {
		t.Fatal("expected START to be defined")
	}

	if addr != 0x4000 {
		t.Errorf("START = %#x, want %#x (should start at DepositAddr)", addr, 0x4000)
	}

	if c.DepositAddr <= 0x4000 {
		t.Errorf("DepositAddr = %#x, expected it to advance past 0x4000 after a statement", c.DepositAddr)
	}

	if err := d.Dispatch("RET"); err != nil {
		t.Fatalf("Dispatch(RET): %v", err)
	}

	if err := d.Dispatch("END"); err != nil {
		t.Fatalf("Dispatch(END): %v", err)
	}

	if err := c.Call(addr, false); err != nil {
		t.Fatalf("Call: %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 9 {
		t.Errorf("R0 = %d, want 9", got)
	}
}

// TestDispatchASM_sharedSessionWithBatchFile assembles testdata/asm/xor.asm
// (batch form) first, then enters interactive mode and references its
// "test" .entry label -- confirming the persistent asmSession (and its
// symbol table) is genuinely shared between the batch and interactive
// forms, matching machine.go's own doc comment on why "ASM <file>" reuses
// one Assembler across calls.
func TestDispatchASM_sharedSessionWithBatchFile(t *testing.T) {
	d, c := newRunnableDispatcher(t)

	if _, _, err := c.Assemble(asmFixturePath(t, "xor.asm")); err != nil {
		t.Fatalf("Assemble(xor.asm): %v", err)
	}

	for _, line := range []string{
		"ASM",
		"JMP TEST",
		"END",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("Dispatch(%q): %v", line, err)
		}
	}

	if c.InAssemblerMode() {
		t.Fatal("expected InAssemblerMode() false after END")
	}
}
