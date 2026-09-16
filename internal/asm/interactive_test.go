package asm

import "testing"

// TestAssembleLineMatchesBatch feeds xor.asm to AssembleLine one line at a
// time (as the console's interactive REPL mode does, docs/PHASE-19.md) and
// checks the result is byte-identical to assembling the whole file in one
// Assemble call, confirming the two entry points share the same underlying
// per-statement engine.
func TestAssembleLineMatchesBatch(t *testing.T) {
	src := readFixture(t, "xor.asm")

	batch := New()
	want, err := batch.Assemble(src)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	interactive := New()

	var done bool

	for _, line := range splitLines(src) {
		d, err := interactive.AssembleLine(line)
		if err != nil {
			t.Fatalf("AssembleLine(%q): %v", line, err)
		}

		if d {
			done = true

			break
		}
	}

	if !done {
		t.Fatal("expected xor.asm's .end to report done=true")
	}

	got := interactive.Bytes()
	if len(got) != len(want) || string(got) != string(want) {
		t.Fatalf("interactive assembly diverged from batch assembly: got %d bytes, want %d", len(got), len(want))
	}
}

// TestAssembleLine_stopsAtEnd confirms a bare "END" (no dot) stops assembly
// exactly like a dotted ".END", and that no further statements are needed
// once done is reported.
func TestAssembleLine_stopsAtEnd(t *testing.T) {
	a := New()

	if done, err := a.AssembleLine("START: MOVL #1,R0"); err != nil || done {
		t.Fatalf("AssembleLine(MOVL) = done=%v, err=%v", done, err)
	}

	done, err := a.AssembleLine("END START")
	if err != nil {
		t.Fatalf("AssembleLine(END): %v", err)
	}
	if !done {
		t.Fatal("expected bare END to report done=true")
	}

	addr, ok := a.Entry()
	if !ok {
		t.Fatal("expected END START to record an entry address")
	}

	want, ok := a.symbols.find("START")
	if !ok || want.value != addr {
		t.Fatalf("entry address %#x doesn't match START's own value", addr)
	}
}

// TestAssembleLine_errorStaysInteractive confirms a statement error doesn't
// set done=true -- matching the reference tool's assemble(), which returns
// a bad statement's error without ever touching assembler_mode, so a typo
// doesn't kick the caller out of interactive mode.
func TestAssembleLine_errorStaysInteractive(t *testing.T) {
	a := New()

	done, err := a.AssembleLine("NOTANOPCODE R0,R1")
	if err == nil {
		t.Fatal("expected an error for an unrecognized opcode")
	}
	if done {
		t.Fatal("a statement error must not report done=true")
	}

	// The session should still be usable afterward.
	if done, err := a.AssembleLine("MOVL #1,R0"); err != nil || done {
		t.Fatalf("AssembleLine after error = done=%v, err=%v", done, err)
	}
}

// TestBeginInteractive_resetsStop confirms a fresh interactive session
// started after a previous one's own END doesn't immediately no-op on its
// first line, matching Assemble's own reset-on-entry for the same reason
// (internal/console/asm.go reuses one Assembler across several ASM
// commands).
func TestBeginInteractive_resetsStop(t *testing.T) {
	a := New()

	if _, err := a.AssembleLine("END"); err != nil {
		t.Fatalf("first END: %v", err)
	}

	a.BeginInteractive()

	done, err := a.AssembleLine("MOVL #1,R0")
	if err != nil {
		t.Fatalf("AssembleLine after BeginInteractive: %v", err)
	}
	if done {
		t.Fatal("a real statement right after BeginInteractive must not report done=true")
	}
}

// TestDeposit confirms Deposit() tracks the active location counter as
// statements are assembled, matching vax.console.deposit.
func TestDeposit(t *testing.T) {
	a := New()

	start := a.Deposit()

	if _, err := a.AssembleLine("LONG 1,2,3,4"); err != nil {
		t.Fatalf("AssembleLine: %v", err)
	}

	if got, want := a.Deposit(), start+16; got != want {
		t.Errorf("Deposit() = %#x, want %#x", got, want)
	}
}

// TestHasUnresolvedSymbols confirms a forward reference to a never-defined
// label is reported, and a real one isn't.
func TestHasUnresolvedSymbols(t *testing.T) {
	a := New()

	if _, err := a.AssembleLine("JMP UNDEFINED_LABEL"); err != nil {
		t.Fatalf("AssembleLine: %v", err)
	}

	if !a.HasUnresolvedSymbols() {
		t.Error("expected a forward reference to UNDEFINED_LABEL to be unresolved")
	}

	b := New()
	if _, err := b.AssembleLine("HERE: JMP HERE"); err != nil {
		t.Fatalf("AssembleLine: %v", err)
	}

	if b.HasUnresolvedSymbols() {
		t.Error("expected a self-referencing backward jump to have no unresolved symbols")
	}
}

func splitLines(s string) []string {
	var lines []string

	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}

	if start < len(s) {
		lines = append(lines, s[start:])
	}

	return lines
}
