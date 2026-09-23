package asm

import (
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestPseudoP1VectorRequiresMicrokernel matches .SCB/.REGION/.SHIM's own
// gating: .P1VECTOR is meaningless outside a microkernel-style assembly.
func TestPseudoP1VectorRequiresMicrokernel(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".P1VECTOR"); err == nil {
		t.Fatal("expected an error assembling .P1VECTOR without .MICROKERNEL")
	}
}

// TestPseudoP1VectorIsIdempotent matches p1_init()'s own real behavior
// (asm_symbols.c's set_symbol only rejects a redefinition when the caller
// has separately raised ASM_UNIQUE beforehand, which set_symbol_direct's
// call sites in p1_init() never do): running .P1VECTOR twice against the
// same Assembler -- whether from one source ASMing ".P1VECTOR" twice, or
// (the real scenario this matters for) a persistent console session that
// already booted kernel.asm's own ".p1vector" line ASMing a second file
// that also has one, e.g. testdata/asm/rms_roundtrip.asm outside full
// console boot -- must not fail with a duplicate-symbol error.
func TestPseudoP1VectorIsIdempotent(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".MICROKERNEL\n.P1VECTOR\n.P1VECTOR\n"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	v, _, err := a.getSymbol("SYS$CREATE", false, 0, fixNone)
	if err != nil {
		t.Fatal(err)
	}

	if v != 0x7FFEE1C8 {
		t.Errorf("SYS$CREATE = %#x, want %#x", v, 0x7FFEE1C8)
	}

	// Also across two separate top-level Assemble calls on the same
	// Assembler -- the shape internal/console/asm.go's persistent
	// asmSession actually produces (one ASM <file> command per call).
	b := New(true)

	if _, err := b.Assemble(".MICROKERNEL\n.P1VECTOR\n"); err != nil {
		t.Fatalf("first assemble: %v", err)
	}

	if _, err := b.Assemble(".MICROKERNEL\n.P1VECTOR\n"); err != nil {
		t.Fatalf("second assemble: %v", err)
	}
}

// TestPseudoP1VectorDefinesSymbolsAndTrampolines assembles a bare
// ".microkernel" / ".p1vector" program and checks it did what p1_init()
// itself does: every SYS$xxx symbol defined at its real, fixed address,
// and a CALLS-compatible trampoline (mask word, XFC #XFC$P1VECTOR, RET)
// deposited there.
func TestPseudoP1VectorDefinesSymbolsAndTrampolines(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".MICROKERNEL\n.P1VECTOR\n"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	// docs/DEVIATIONS.md's "[Phase 11] Three P1-vector table entries land
	// close enough together..." entry: SYS$CLRAST_2/SYS$GL_ASTRET (both at
	// 0x7FFEE110) and SYS$GL_COMMON (0x7FFEE114) end up with their own
	// trailing RET byte clobbered by a later table entry's mask/XFC write —
	// a quirk inherited faithfully from the C reference's own p1_init(),
	// not a porting bug, so this test asserts the exact clobbered byte
	// rather than assuming every entry's trampoline is untouched.
	clobberedRET := map[string]byte{
		"SYS$CLRAST_2":  0x00, // SYS$GL_COMMON's mask word overwrites it
		"SYS$GL_ASTRET": 0x00, // ditto (shares SYS$CLRAST_2's address)
		"SYS$GL_COMMON": 0xFC, // SYS$SRCHANDLER's XFC opcode overwrites it
	}

	for _, e := range vmsdef.P1VectorTable {
		v, _, err := a.getSymbol(e.Name, false, 0, fixNone)
		if err != nil {
			t.Fatalf("%s: %v", e.Name, err)
		}

		if v != e.Addr {
			t.Fatalf("%s = %#x, want %#x", e.Name, v, e.Addr)
		}

		addr := e.Addr
		if !e.Jmp {
			if got := a.BytesRange(addr, addr+2); got[0] != 0 || got[1] != 0 {
				t.Fatalf("%s: mask word = %v, want zero", e.Name, got)
			}
		} else {
			addr -= 2
		}

		want := []byte{0xFC, 0x7A, 0x04}
		if b, clobbered := clobberedRET[e.Name]; clobbered {
			want[2] = b
		}

		got := a.BytesRange(addr+2, addr+5)
		if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
			t.Fatalf("%s: trampoline = % X, want % X", e.Name, got, want)
		}
	}

	// SYS$SRCHANDLER is the one Jmp entry — confirm it's SymLabel, not
	// SymEntry, matching p1_init()'s own SYM_LABEL/SYM_ENTRY split.
	sym, ok := a.symbols.find("SYS$SRCHANDLER")
	if !ok {
		t.Fatal("expected SYS$SRCHANDLER to be defined")
	}

	if sym.flags&SymEntry != 0 {
		t.Error("SYS$SRCHANDLER: expected SymLabel (Jmp entry), got SymEntry")
	}

	if sym.flags&SymLabel == 0 {
		t.Error("SYS$SRCHANDLER: expected SymLabel set")
	}

	base, _, err := a.getSymbol("EXE$P1_VECTOR_BASE", false, 0, fixNone)
	if err != nil {
		t.Fatal(err)
	}

	end, _, err := a.getSymbol("EXE$P1_VECTOR_END", false, 0, fixNone)
	if err != nil {
		t.Fatal(err)
	}

	wantMin, wantMax := uint32(0x7FFFFFFF), uint32(0)
	for _, e := range vmsdef.P1VectorTable {
		if e.Addr < wantMin {
			wantMin = e.Addr
		}

		if e.Addr > wantMax {
			wantMax = e.Addr
		}
	}

	if base != wantMin {
		t.Errorf("EXE$P1_VECTOR_BASE = %#x, want %#x", base, wantMin)
	}

	if end != wantMax {
		t.Errorf("EXE$P1_VECTOR_END = %#x, want %#x", end, wantMax)
	}

	rangeBase, rangeEnd, ok := a.P1VectorRange()
	if !ok {
		t.Fatal("expected P1VectorRange to report ok=true after .P1VECTOR")
	}

	if rangeBase != wantMin || rangeEnd != wantMax+5 {
		t.Errorf("P1VectorRange() = [%#x, %#x), want [%#x, %#x)", rangeBase, rangeEnd, wantMin, wantMax+5)
	}
}
