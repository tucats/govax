package console

import "testing"

// TestVMInitPagingQualifier: INITIALIZE/MEMORY/PAGING and /NOPAGING choose the
// paging mode (docs/planning/PHASE-62 - VMS paging.md, Decision 1), and the System
// INITIALIZE/MEMORY builds gets it.
func TestVMInitPagingQualifier(t *testing.T) {
	c, _ := newTestConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t))

	if err := d.Dispatch("initialize/memory /p0=20 /p1=4116 /s0=120 /ksp=2 /paging"); err != nil {
		t.Fatalf("INITIALIZE/MEMORY/PAGING: %v", err)
	}

	if !c.Paging() || !c.ProcessOne().Sysgen.Paging {
		t.Error("INITIALIZE/MEMORY/PAGING left paging off")
	}

	if err := d.Dispatch("initialize/memory /p0=20 /p1=20 /ksp=2 /nopaging"); err != nil {
		t.Fatalf("INITIALIZE/MEMORY/NOPAGING: %v", err)
	}

	if c.Paging() || c.ProcessOne().Sysgen.Paging {
		t.Error("INITIALIZE/MEMORY/NOPAGING left paging on")
	}
}
