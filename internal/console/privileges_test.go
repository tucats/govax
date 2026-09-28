package console

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// TestPrivileges_assembledProgram runs testdata/asm/privileges.asm
// (docs/PHASE-26.md subtask 38): JPI$_CURPRIV, $SETPRV disabling and
// enabling CMKRNL around $CMKRNL, and $CREMBX without TMPMBX.
func TestPrivileges_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "privileges.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1", got)
	}

	quad := func(sym string) (uint32, uint32) {
		a, _ := c.Symbols.Get(sym)

		return getRange(t, c, a)
	}

	if lo, hi := quad("CURPRIV"); lo != 0xFFFFFFFF || hi != 0x7F {
		t.Errorf("CURPRIV %#x %#x, want all 39 privileges", lo, hi)
	}

	if lo, hi := quad("PRVPRV"); lo != 0xFFFFFFFF || hi != 0x7F {
		t.Errorf("PRVPRV %#x %#x, want all 39 privileges", lo, hi)
	}

	checks := []struct {
		sym  string
		want uint32
	}{
		{"CMK1", ssNoPriv},
		{"SETPRV2", ssNormal},
		{"CMK2", 0x77},
		{"MBX", ssNoPriv},
	}

	for _, ck := range checks {
		if got := word(ck.sym); got != ck.want {
			t.Errorf("%s = %#x, want %#x", ck.sym, got, ck.want)
		}
	}

	// The fixture ran as a console CALL, not RUN, so no image rundown:
	// TMPMBX is still off in the current mask, but not the permanent one.
	p := c.RTL.Process
	if p.CurrentPrivileges == p.ProcessPrivileges {
		t.Error("the temporary disable should be visible before rundown")
	}

	c.RTL.ImageRundown()

	if p.CurrentPrivileges != p.ProcessPrivileges {
		t.Error("rundown should restore the permanent privileges")
	}
}
