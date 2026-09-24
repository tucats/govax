package asm

import (
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestPseudoRMSDEFNoMicrokernelRequired confirms .RMSDEF, unlike
// .P1VECTOR/.SHIM/.SCB/.REGION, needs no .MICROKERNEL — it deposits no
// bytes, only defines symbols, matching a real $FABDEF/$RABDEF/$RMSDEF
// .INCLUDE any ordinary program can use.
func TestPseudoRMSDEFNoMicrokernelRequired(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".RMSDEF"); err != nil {
		t.Fatalf("assemble: %v", err)
	}
}

// TestPseudoRMSDEFDefinesEveryConstantAndOffsetSymbol spot-checks a
// representative sample of .RMSDEF's own symbol table — every real name
// this phase's planning research confirmed — plus a full sweep confirming
// every internal/vmsdef.Constants entry and every FABFields/RABFields
// offset symbol resolved to its real value.
func TestPseudoRMSDEFDefinesEveryConstantAndOffsetSymbol(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".RMSDEF"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	want := map[string]uint32{
		"FAB$C_SEQ":   0,
		"FAB$C_FIX":   1,
		"FAB$K_BLN":   80,
		"RAB$C_SEQ":   0,
		"RAB$K_BLN":   68,
		"RMS$_NORMAL": 65537,
		"RMS$_EOF":    98938,
		"FAB$M_PUT":   1,
		"FAB$B_FAC":   22,
		"FAB$L_STS":   8,
		"FAB$L_FNA":   44,
		"RAB$L_RBF":   40,
		"RAB$L_FAB":   60,
		"RAB$B_RAC":   30,
	}

	for name, wantVal := range want {
		v, _, err := a.getSymbol(name, false, 0, fixNone)
		if err != nil {
			t.Errorf("%s: %v", name, err)

			continue
		}

		if v != wantVal {
			t.Errorf("%s = %d, want %d", name, v, wantVal)
		}
	}

	for name, wantVal := range vmsdef.Constants {
		v, _, err := a.getSymbol(name, false, 0, fixNone)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if v != wantVal {
			t.Fatalf("%s = %d, want %d", name, v, wantVal)
		}
	}

	for _, f := range vmsdef.FABFields {
		v, _, err := a.getSymbol(f.Symbol, false, 0, fixNone)
		if err != nil {
			t.Fatalf("%s: %v", f.Symbol, err)
		}

		if v != f.Offset {
			t.Fatalf("%s = %d, want %d", f.Symbol, v, f.Offset)
		}
	}

	for _, f := range vmsdef.RABFields {
		v, _, err := a.getSymbol(f.Symbol, false, 0, fixNone)
		if err != nil {
			t.Fatalf("%s: %v", f.Symbol, err)
		}

		if v != f.Offset {
			t.Fatalf("%s = %d, want %d", f.Symbol, v, f.Offset)
		}
	}
}

// TestPseudoRMSDEFIsIdempotent matches .P1VECTOR's own fixed idempotency
// issue (docs/PHASE-11.md) — running .RMSDEF twice, whether in one source or
// across two separate top-level Assemble calls on the same Assembler (the
// shape a persistent console session's asmSession actually produces), must
// not fail with a duplicate-symbol error.
func TestPseudoRMSDEFIsIdempotent(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".RMSDEF\n.RMSDEF\n"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	b := New(true)

	if _, err := b.Assemble(".RMSDEF\n"); err != nil {
		t.Fatalf("first assemble: %v", err)
	}

	if _, err := b.Assemble(".RMSDEF\n"); err != nil {
		t.Fatalf("second assemble: %v", err)
	}
}

// TestPseudoRMSDEFSymbolsAreNoBytes confirms .RMSDEF deposits nothing into
// the image — pure symbol-table definition, matching its own doc comment.
func TestPseudoRMSDEFSymbolsAreNoBytes(t *testing.T) {
	a := New(true)

	before := a.Bytes()

	if _, err := a.Assemble(".RMSDEF"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	after := a.Bytes()

	if len(before) != len(after) {
		t.Fatalf("Bytes() length changed: %d -> %d", len(before), len(after))
	}

	for i := range after {
		if after[i] != 0 {
			t.Fatalf("Bytes()[%d] = %#x, want 0 (no bytes deposited by .RMSDEF)", i, after[i])
		}
	}
}
