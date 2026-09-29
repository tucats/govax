package console

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// TestRights_assembledProgram runs testdata/asm/rights.asm
// (docs/PHASE-26.md subtask 40): $ASCTOID, $FAO's !%I, and $IDTOASC,
// singly and as a listing.
func TestRights_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "rights.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a call failed?)", got)
	}

	text := func(buf, length string) string {
		t.Helper()

		a, _ := c.Symbols.Get(buf)
		n := word(length) & 0xFFFF
		b := make([]byte, n)

		if err := c.Mem.Load(c.CPU, a, b); err != nil {
			t.Fatal(err)
		}

		return string(b)
	}

	if got := word("ID"); got != 0x80000003 {
		t.Errorf("ID = %#x, want INTERACTIVE's %%X80000003", got)
	}

	if got := text("TEXT", "TEXTLEN"); got != "INTERACTIVE and [SYSTEM]" {
		t.Errorf("TEXT = %q", got)
	}

	if got := word("COUNT"); got != 7 {
		t.Errorf("COUNT = %d, want 7 identifiers", got)
	}

	// NAME holds the listing's last name, SYSTEM; the single translation
	// before it was checked through its status.
	if got := text("NAME", "NAMLEN"); got != "SYSTEM" {
		t.Errorf("NAME = %q", got)
	}
}
