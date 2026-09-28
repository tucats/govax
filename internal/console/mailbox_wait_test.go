package console

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestMailboxWait_assembledProgram runs testdata/asm/mailbox_wait.asm
// (docs/PHASE-26.md subtask 37): a read attention AST, a writer waiting
// on a full mailbox for a timer AST to make room, and $SETRWM.
func TestMailboxWait_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "mailbox_wait.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a service failed?)", got)
	}

	text := func(sym string, n int) string {
		a, _ := c.Symbols.Get(sym)
		b := make([]byte, n)

		if err := c.Mem.Load(c.CPU, a, b); err != nil {
			t.Fatal(err)
		}

		return string(b)
	}

	if got := text("MSG1", 5); got != "hello" {
		t.Errorf("MSG1 %q: the read attention AST's read", got)
	}

	if got := text("MSG2", 16); got != "0123456789ABCDEF" {
		t.Errorf("MSG2 %q: the timer AST's read", got)
	}

	if got := text("MSG3", 4); got != "wxyz" {
		t.Errorf("MSG3 %q: the write that waited for room", got)
	}

	checks := []struct {
		sym  string
		want uint32
	}{
		{"RPARAM", 0x11},
		{"WSTAT", ssNormal},
		{"RWM1", ssWasClr},
		{"FULLST", vmsdef.SSConstants["SS$_MBFULL"] & 0xFFFF},
		{"RWM2", ssWasSet},
	}

	for _, ck := range checks {
		if got := word(ck.sym); got != ck.want {
			t.Errorf("%s = %#x, want %#x", ck.sym, got, ck.want)
		}
	}
}
