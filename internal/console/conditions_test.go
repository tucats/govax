package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// runFixture assembles and runs one of testdata/asm's programs to
// completion on a booted console, returning a reader for its longword
// symbols.
func runFixture(t *testing.T, c *Console, name string) func(string) uint32 {
	t.Helper()

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, name))
	if err != nil {
		t.Fatalf("Assemble(%s): %v", name, err)
	}

	if !hasEntry {
		t.Fatalf("%s has no entry address", name)
	}

	if runErr, hitCap := callBounded(t, c, addr, 100_000); runErr != nil || hitCap {
		t.Fatalf("running %s: err=%v hitCap=%v", name, runErr, hitCap)
	}

	return func(sym string) uint32 {
		t.Helper()

		a, ok := c.Symbols.Get(sym)
		if !ok {
			t.Fatalf("no %s symbol", sym)
		}

		v, err := c.Mem.LoadLongword(c.CPU, a)
		if err != nil {
			t.Fatal(err)
		}

		return v
	}
}

// TestConditions_assembledProgram runs testdata/asm/conditions.asm
// (docs/PHASE-26.md subtask 31): an access violation that one frame's
// handler resignals and its caller's continues, and a subscript range
// trap.
func TestConditions_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "conditions.asm")

	checks := []struct {
		sym  string
		want uint32
	}{
		{"SUBCALLS", 1}, // SUB1's handler, once
		{"SUBDEPTH", 0}, // ... for SUB1's own frame
		{"DEPTH1", 1},   // MAINH, one frame up
		{"VA", 0},       // the address SUB1 read
		{"MASK", 2},     // the reason: a protection violation
		{"R0SEEN", 0x55},
		{"RET1", 2},
		{"DEPTH2", 1},
		{"RET2", 3},
	}

	for _, ck := range checks {
		if got := word(ck.sym); got != ck.want {
			t.Errorf("%s = %#x, want %#x", ck.sym, got, ck.want)
		}
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %#x, want main's 1", got)
	}

	if n := len(c.Out.(*bytes.Buffer).String()); n != 0 {
		t.Errorf("unexpected output %q", c.Out.(*bytes.Buffer).String())
	}
}

// TestConditionExit_assembledProgram runs testdata/asm/condition_exit.asm
// (docs/PHASE-26.md subtask 31): an access violation no handler takes is
// reported by the catch-all and ends the image through $EXIT.
func TestConditionExit_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "condition_exit.asm")

	const status = 0x1000000C // SS$_ACCVIO, message inhibited

	if got := word("SEEN"); got != status {
		t.Errorf("the exit handler saw %#x, want %#x", got, status)
	}

	if word("REACHED") != 0 {
		t.Error("the code after the access violation ran")
	}

	if got := c.CPU.GPR(vax.R0); got != status {
		t.Errorf("R0 = %#x, want %#x", got, status)
	}

	out := c.Out.(*bytes.Buffer).String()
	if !strings.HasPrefix(out, "%SYSTEM-F-ACCVIO, access violation, reason mask=") ||
		!strings.Contains(out, ", virtual address=00000000, PC=") {
		t.Errorf("output %q, want the ACCVIO message", out)
	}
}

// TestExceptionVectors_assembledProgram runs
// testdata/asm/exception_vectors.asm (docs/PHASE-26.md subtask 32):
// primary, secondary, and last-chance vectors set and cleared by
// $SETEXV.
func TestExceptionVectors_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "exception_vectors.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a $SETEXV failed?)", got)
	}

	checks := []struct {
		sym  string
		want uint32
	}{
		{"PRIMCALLS", 1},
		{"PRIMDEPTH", 0xFFFFFFFE}, // -2
		{"LASTDEPTH", 0xFFFFFFFD}, // -3
		{"SECDEPTH", 0xFFFFFFFF},  // -1
	}

	for _, ck := range checks {
		if got := word(ck.sym); got != ck.want {
			t.Errorf("%s = %#x, want %#x", ck.sym, got, ck.want)
		}
	}

	prim, _ := c.Symbols.Get("PRIM")
	if got := word("PRVHND"); got != prim {
		t.Errorf("PRVHND = %#x, want PRIM %#x", got, prim)
	}
}
