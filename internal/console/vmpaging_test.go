package console

import "testing"

// TestVMInitPagingQualifier: VMINIT/PAGING and /NOPAGING choose the
// paging mode (docs/PHASE-62 - VMS paging.md, Decision 1), and the System
// VMINIT builds gets it.
func TestVMInitPagingQualifier(t *testing.T) {
	c, _ := newTestConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	if err := d.Dispatch("vminit /p0=20 /p1=4116 /s0=120 /ksp=2 /paging"); err != nil {
		t.Fatalf("VMINIT/PAGING: %v", err)
	}

	if !c.Paging() || !c.ProcessOne().Sysgen.Paging {
		t.Error("VMINIT/PAGING left paging off")
	}

	if err := d.Dispatch("vminit /p0=20 /p1=20 /ksp=2 /nopaging"); err != nil {
		t.Fatalf("VMINIT/NOPAGING: %v", err)
	}

	if c.Paging() || c.ProcessOne().Sysgen.Paging {
		t.Error("VMINIT/NOPAGING left paging on")
	}
}
